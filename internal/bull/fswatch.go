package bull

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/fsnotify/fsnotify"
)

func (b *bullServer) watchContent(ctx context.Context) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	var watchCount, watchErrors int
	if err := fs.WalkDir(b.content.FS(), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Printf("fswatch: walk %s: %v", path, err)
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		if d.Name() == ".git" {
			return fs.SkipDir
		}
		if err := w.Add(filepath.Join(b.contentDir, path)); err != nil {
			log.Printf("fswatch: watch %s: %v", path, err)
			watchErrors++
		} else {
			watchCount++
		}
		return nil
	}); err != nil {
		log.Printf("fswatch: adding watches: %v", err)
	}
	if watchErrors > 0 {
		log.Printf("fswatch: watching %d directories (%d errors — you may need to increase fs.inotify.max_user_watches)", watchCount, watchErrors)
	} else {
		log.Printf("fswatch: watching %d directories", watchCount)
	}

	go func() {
		if err := b.watchContentLoop(ctx, w); err != nil {
			log.Printf("fswatch: event channel closed unexpectedly")
		}
	}()

	return nil
}

func (b *bullServer) watchContentLoop(ctx context.Context, w *fsnotify.Watcher) error {
	defer w.Close()

	// Debounce notification: coalesce rapid events (e.g. git pull)
	// into a single notification after 100ms of quiet.
	var debounceTimer *time.Timer
	resetDebounce := func() {
		if debounceTimer != nil {
			debounceTimer.Stop()
		}
		// Fires on the timer goroutine; safe because notifyContentChanged is goroutine-safe.
		debounceTimer = time.AfterFunc(100*time.Millisecond, func() {
			b.notifyContentChanged()
		})
	}

	for {
		select {
		case <-ctx.Done():
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			return nil

		case event, ok := <-w.Events:
			if !ok {
				return fmt.Errorf("fswatch: event channel closed unexpectedly")
			}
			if b.handleContentEvent(w, event) {
				resetDebounce()
			}

		case err, ok := <-w.Errors:
			if !ok {
				return fmt.Errorf("fswatch: error channel closed unexpectedly")
			}
			if errors.Is(err, fsnotify.ErrEventOverflow) {
				log.Printf("fswatch: event queue overflowed, rebuilding index")
				if idx, err := b.index(); err == nil {
					b.idxMu.Lock()
					b.idx.Store(idx)
					b.idxMu.Unlock()
					b.notifyContentChanged()
				} else {
					log.Printf("fswatch: re-index after overflow: %v", err)
				}
			} else {
				log.Printf("fswatch: %v", err)
			}
		}
	}
}

// handleContentEvent processes a single fsnotify event.
// It returns true if the index was updated (caller should notify).
func (b *bullServer) handleContentEvent(w *fsnotify.Watcher, event fsnotify.Event) bool {
	name := event.Name

	rel, err := filepath.Rel(b.contentDir, name)
	if err != nil {
		log.Printf("fswatch: unexpected path %q: %v", name, err)
		return false
	}
	rel = filepath.ToSlash(rel)

	if !filepath.IsLocal(rel) {
		return false // prevent path traversal
	}

	// For removed/renamed paths, try to remove the watch
	// (handles directory renames/deletes that would leak watches).
	// Errors are expected here: fsnotify watches directories, not files,
	// and inotify auto-removes watches for deleted inodes on Linux.
	if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
		w.Remove(name)
	}

	// For new directories, add them to the watcher, then scan for
	// files that may have been created before the watch was established.
	// Use os.Lstat (not fs.Stat) to avoid following symlinks, consistent
	// with fs.WalkDir which also does not follow symlinks.
	if event.Has(fsnotify.Create) {
		info, err := os.Lstat(filepath.Join(b.contentDir, rel))
		if err == nil && info.IsDir() {
			if filepath.Base(rel) == ".git" {
				return false
			}
			if err := w.Add(filepath.Join(b.contentDir, rel)); err != nil {
				log.Printf("fswatch: watch %s: %v", rel, err)
			}
			return b.scanNewDir(w, rel)
		}
	}

	// Only process markdown files.
	if !isMarkdown(rel) {
		return false
	}

	pageName := file2page(rel)

	switch {
	case event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename):
		// Reading idx outside the lock is a benign TOCTOU: worst case we call
		// removeFromIndex redundantly (it rechecks under the lock).
		if old := b.idx.Load().links[pageName]; old == nil {
			return false // already absent from the index
		}
		b.removeFromIndex(pageName)
		return true

	case event.Has(fsnotify.Create) || event.Has(fsnotify.Write):
		pg, err := b.read(rel)
		if err != nil {
			log.Printf("fswatch: read %s: %v", rel, err)
			return false
		}
		targets, err := b.linkTargets(pg)
		if err != nil {
			log.Printf("fswatch: linkTargets %s: %v", rel, err)
			return false
		}
		// Reading idx outside the lock is a benign TOCTOU: worst case we call
		// updateIndex redundantly (it rechecks and stores an identical snapshot).
		if slices.Equal(b.idx.Load().links[pageName], targets) {
			return false // index already up to date (e.g. save already applied)
		}
		b.updateIndex(pageName, targets)
		return true
	}
	return false
}

// scanNewDir indexes any markdown files (and subdirectories) already present
// in a newly created directory, closing the race between mkdir and w.Add.
// Uses a two-phase approach: walk+parse outside the lock, then apply all
// updates under a single lock acquisition.
func (b *bullServer) scanNewDir(w *fsnotify.Watcher, dir string) bool {
	type indexEntry struct {
		pageName string
		targets  []string
	}

	// Phase 1: walk and parse (no lock held).
	var entries []indexEntry
	if err := fs.WalkDir(b.content.FS(), dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Printf("fswatch: scanNewDir walk %s: %v", p, err)
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return fs.SkipDir
			}
			if p != dir {
				if err := w.Add(filepath.Join(b.contentDir, p)); err != nil {
					log.Printf("fswatch: watch %s: %v", p, err)
				}
			}
			return nil
		}
		if !isMarkdown(p) {
			return nil
		}
		pg, err := b.read(p)
		if err != nil {
			log.Printf("fswatch: scanNewDir read %s: %v", p, err)
			return nil
		}
		targets, err := b.linkTargets(pg)
		if err != nil {
			log.Printf("fswatch: scanNewDir linkTargets %s: %v", p, err)
			return nil
		}
		entries = append(entries, indexEntry{pageName: file2page(p), targets: targets})
		return nil
	}); err != nil {
		log.Printf("fswatch: scanNewDir walk %s: %v", dir, err)
	}

	if len(entries) == 0 {
		return false
	}

	// Phase 2: apply all updates in a single clone-patch-store cycle.
	updates := make([]indexUpdate, len(entries))
	for idx, entry := range entries {
		updates[idx] = indexUpdate(entry)
	}
	b.idxMu.Lock()
	defer b.idxMu.Unlock()
	b.applyIndexBatchLocked(nil, updates)
	return true
}
