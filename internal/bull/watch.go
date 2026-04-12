package bull

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

func initEventStream(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// Instruct nginx to disable buffering:
	w.Header().Set("X-Accel-Buffering", "no")

	w.WriteHeader(http.StatusOK)

	w.Write([]byte{'\n'})
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

func maybeNotify(ctx context.Context, notify chan<- struct{}, fileName string) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("fsnotify.NewWatcher: %v", err)
		return
	}
	go func() {
		defer w.Close()
		for {
			select {
			case <-ctx.Done():
				return

			case err, ok := <-w.Errors:
				if !ok {
					return // watcher closed
				}
				log.Printf("fsnotify error: %v", err)

			case e, ok := <-w.Events:
				if !ok {
					return // watcher closed
				}
				if e.Name == fileName {
					notify <- struct{}{}
				}
			}
		}
	}()
	// Watch the directory instead of the file itself:
	// Editors replace files with an updated version.
	dir := filepath.Dir(fileName)
	if err := w.Add(dir); err != nil {
		w.Close()
		log.Printf("fsnotify.Watch(%s): %v", dir, err)
		return
	}
}

func (b *bullServer) browseContentHash(dir, sortby, sortorder, directories string) (string, error) {
	md, err := b.browseContent(dir, sortby, sortorder, directories)
	if err != nil {
		return "", err
	}
	return hashSum(md), nil
}

func (b *bullServer) handleWatchBrowse(ctx context.Context, w http.ResponseWriter, flusher http.Flusher, r *http.Request) error {
	dir := r.FormValue("dir")
	sortby := r.FormValue("sort")
	sortorder := r.FormValue("sortorder")
	directories := r.FormValue("directories")
	rhash := r.FormValue("hash")

	w.Header().Set("Access-Control-Allow-Origin", "*")
	initEventStream(w)

	// Acquire the change channel before the hash check to avoid a
	// TOCTOU gap: any change that occurs during or after hashing
	// will be visible through this channel.
	contentChanged := b.contentChangedCh()

	// Catch up on changes that happened while disconnected
	// (e.g. during page reload), analogous to the hash check
	// in the regular page watcher.
	if rhash != "" {
		current, err := b.browseContentHash(dir, sortby, sortorder, directories)
		if err != nil {
			log.Printf("browseContentHash (initial): %v", err)
		} else if current != rhash {
			w.Write([]byte("data: {\"changed\":true}\n\n"))
			flusher.Flush()
			return nil
		}
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-contentChanged:
			// Re-acquire the channel for the next iteration before
			// doing any work, so we don't miss changes that occur
			// during hash computation.
			contentChanged = b.contentChangedCh()

			if rhash != "" {
				// TODO: browseContentHash walks the entire content
				// directory. Consider adding debounce or caching if
				// this becomes a bottleneck with large wikis.
				current, err := b.browseContentHash(dir, sortby, sortorder, directories)
				if err != nil {
					log.Printf("browseContentHash (watch loop): %v", err)
					// On error, notify the client to reload rather than
					// silently sitting idle.
				} else if current == rhash {
					continue // content didn't actually change for this view
				}
			}
			w.Write([]byte("data: {\"changed\":true}\n\n"))
			flusher.Flush()
			return nil // client reloads and reconnects
		}
	}
}

func (b *bullServer) handleWatch(w http.ResponseWriter, r *http.Request) error {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("BUG: ResponseWriter does not implement http.Flusher")
	}

	ctx := r.Context()

	pageName := pageFromURL(r)
	if pageName == bullPrefix+"browse" {
		return b.handleWatchBrowse(ctx, w, flusher, r)
	}

	possibilities := filesFromURL(r)
	lastb, err := b.readFirst(possibilities)
	if err != nil {
		return err
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	initEventStream(w)

	// Each watch request contains the page ContentHash() as a URL parameter,
	// so that we can immediately emit a change even when the client was
	// not connected during the time of the actual change.
	if rhash := r.FormValue("hash"); rhash != "" {
		if current := lastb.ContentHash(); current != rhash {
			w.Write([]byte("data: {\"changed\":true}\n\n"))
			flusher.Flush()
		}
	}

	notify := make(chan struct{})
	maybeNotify(ctx, notify, filepath.Join(b.contentDir, lastb.FileName))

	poller := time.NewTicker(1 * time.Second)
	defer poller.Stop()
	for {
		contentChanged := b.contentChangedCh()
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-poller.C:
		case <-notify:
		case <-contentChanged:
			// Backlinks or other content changed; re-check the page.
		}

		b, err := b.readFirst(possibilities)
		if err != nil {
			return err
		}
		if lastb.Content == b.Content {
			continue
		}
		lastb = b
		w.Write([]byte("data: {\"changed\":true}\n\n"))
		flusher.Flush()
	}
}
