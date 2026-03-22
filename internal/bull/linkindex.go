package bull

import (
	"context"
	"errors"
	"io/fs"
	"log"
	"maps"
	"os"
	"path"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yuin/goldmark/ast"
	"go.abhg.dev/goldmark/wikilink"
	"golang.org/x/sync/errgroup"
)

type idx struct {
	dirs, pages uint64
	// links maps from page name to target page names (forward index).
	links map[string][]string
	// backlinks maps from page name (e.g. index) to
	// page names that contain a link to that page (e.g. SETTINGS, projects, …).
	backlinks map[string][]string
}

func (b *bullServer) linkTargets(pg *page) ([]string, error) {
	var targets []string

	doc := b.parseMD(pg, pg.Content)
	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if wl, ok := n.(*wikilink.Node); ok {
			targets = append(targets, string(wl.Target))
		}
		if link, ok := n.(*ast.Link); ok {
			targets = append(targets, string(link.Destination))
		}
		return ast.WalkContinue, nil
	})

	slices.Sort(targets)
	return slices.Compact(targets), nil
}

type indexer struct {
	// config
	contentRoot *os.Root
	readModTime bool

	// state
	walkq       *queue
	readq       chan page
	dirs, pages atomic.Uint64
	pending     atomic.Int64
}

func newIndexer(content *os.Root) *indexer {
	return &indexer{
		contentRoot: content,
		walkq:       newQueue(),
		readq:       make(chan page),
	}
}

func (i *indexer) dirDiscovered() {
	i.pending.Add(1)
	i.dirs.Add(1)
}

func (i *indexer) dirWalked() {
	i.pending.Add(-1)
}

func (i *indexer) done() bool {
	return i.pending.Load() == 0
}

func (i *indexer) walkN(dir string) error {
	dirents, err := fs.ReadDir(i.contentRoot.FS(), dir)
	if err != nil {
		log.Printf("indexing %s failed: %v", dir, err)
		// intentionally do not error out the entire indexing
		// just because parts of a directory might be inaccessible.
		return nil
	}
	for _, dirent := range dirents {
		name := dirent.Name()
		if dirent.IsDir() {
			if name == ".git" {
				continue
			}
			i.dirDiscovered()
			// put each discovered directory into the walk queue
			// so that any goroutine can pick it up
			i.walkq.Push(path.Join(dir, name))
			continue
		}
		if !isMarkdown(name) {
			continue
		}
		i.pages.Add(1)
		fn := path.Join(dir, name)
		pg := page{
			PageName: file2page(fn),
			FileName: fn,
			// Content is empty; page not read yet
			// ModTime is empty
		}
		if i.readModTime {
			info, err := dirent.Info()
			if err != nil {
				return err
			}
			pg.ModTime = info.ModTime()
		}
		i.readq <- pg
	}
	return nil
}

func (i *indexer) walk() error {
	i.dirDiscovered()
	i.walkq.Push(".")

	ctx, canc := context.WithCancel(context.Background())
	defer canc()
	walkg, gctx := errgroup.WithContext(ctx)
	for range runtime.NumCPU() {
		walkg.Go(func() error {
			defer canc() // first exiting goroutine cancels all others
			for !i.done() {
				// There is some remaining work. Try to obtain the directory
				// from the walk queue, but time-box the attempt:
				popctx, popcanc := context.WithTimeout(gctx, 100*time.Millisecond)
				dir, err := i.walkq.PopOrWait(popctx)
				popcanc()
				if err != nil {
					if errors.Is(err, context.DeadlineExceeded) {
						// Some other goroutine might have raced us to picking
						// up the remaining work item. Retry to see if there is
						// more work (or exit otherwise).
						continue
					}
					return err
				}
				// fmt.Printf("walk %s\n", dir)
				if err := i.walkN(dir); err != nil {
					i.dirWalked()
					return err
				}
				i.dirWalked()
			}
			return nil
		})
	}
	if err := walkg.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	close(i.readq)
	return nil
}

func (b *bullServer) index() (*idx, error) {
	i := newIndexer(b.content)

	var (
		linksMu sync.Mutex
		links   = make(map[string][]string)
		readg   errgroup.Group
	)
	for range runtime.NumCPU() {
		readg.Go(func() error {
			linksN := make(map[string][]string)
			for pg := range i.readq {
				// fmt.Printf("reading %s\n", fn)
				pg, err := b.read(pg.FileName)
				if err != nil {
					return err
				}
				targets, err := b.linkTargets(pg)
				if err != nil {
					return err
				}
				linksN[pg.PageName] = targets
			}
			linksMu.Lock()
			defer linksMu.Unlock()
			maps.Copy(links, linksN)
			return nil
		})
	}
	if err := i.walk(); err != nil {
		return nil, err
	}
	if err := readg.Wait(); err != nil {
		return nil, err
	}
	return &idx{
		dirs:      i.dirs.Load(),
		pages:     i.pages.Load(),
		links:     links,
		backlinks: invertLinks(links),
	}, nil
}

func invertLinks(links map[string][]string) map[string][]string {
	backlinks := make(map[string][]string)
	for pageName, targets := range links {
		for _, target := range targets {
			backlinks[target] = append(backlinks[target], pageName)
		}
	}
	for _, linkers := range backlinks {
		slices.Sort(linkers)
	}
	return backlinks
}

func (b *bullServer) updateIndex(pageName string, newTargets []string) {
	b.idxMu.Lock()
	defer b.idxMu.Unlock()
	b.updateIndexLocked(pageName, newTargets)
}

func (b *bullServer) removeFromIndex(pageName string) {
	b.idxMu.Lock()
	defer b.idxMu.Unlock()
	b.removeFromIndexLocked(pageName)
}

// indexUpdate pairs a page name with its new link targets for batch operations.
type indexUpdate struct {
	pageName string
	targets  []string
}

// applyIndexBatchLocked applies multiple removals and updates in a single
// clone-patch-store cycle, avoiding the O(N * total_pages) cost of calling
// updateIndexLocked in a loop.
// Caller must hold b.idxMu.
func (b *bullServer) applyIndexBatchLocked(removals []string, updates []indexUpdate) {
	old := b.idx.Load()

	newLinks := make(map[string][]string, len(old.links)+len(updates))
	maps.Copy(newLinks, old.links)
	// Shallow clone: the []string value slices are shared with old.backlinks.
	// This is safe because removeFromSorted/insertIntoSorted never mutate
	// slices in place — they either return the original slice unchanged
	// or allocate a new one.
	newBacklinks := maps.Clone(old.backlinks)

	for _, pageName := range removals {
		oldTargets := newLinks[pageName]
		if len(oldTargets) == 0 {
			continue
		}
		delete(newLinks, pageName)
		patchBacklinks(newBacklinks, pageName, nil, oldTargets)
	}

	for _, u := range updates {
		oldTargets := newLinks[u.pageName]
		newLinks[u.pageName] = u.targets
		added, removed := diffSorted(oldTargets, u.targets)
		patchBacklinks(newBacklinks, u.pageName, added, removed)
	}

	b.idx.Store(&idx{
		dirs:      old.dirs,
		pages:     uint64(len(newLinks)),
		links:     newLinks,
		backlinks: newBacklinks,
	})
}

// removeFromIndexLocked removes a page from the index.
// Caller must hold b.idxMu.
func (b *bullServer) removeFromIndexLocked(pageName string) {
	old := b.idx.Load()
	oldTargets, ok := old.links[pageName]
	if !ok {
		return // already absent
	}

	newLinks := make(map[string][]string, len(old.links))
	maps.Copy(newLinks, old.links)
	delete(newLinks, pageName)

	added, removed := diffSorted(oldTargets, nil)
	// Shallow clone: the []string value slices are shared with old.backlinks.
	// This is safe because removeFromSorted/insertIntoSorted never mutate
	// slices in place — they either return the original slice unchanged
	// or allocate a new one.
	newBacklinks := maps.Clone(old.backlinks)
	patchBacklinks(newBacklinks, pageName, added, removed)

	b.idx.Store(&idx{
		dirs:      old.dirs,
		pages:     uint64(len(newLinks)),
		links:     newLinks,
		backlinks: newBacklinks,
	})
}

// updateIndexLocked updates a single page's links in the index.
// Caller must hold b.idxMu.
func (b *bullServer) updateIndexLocked(pageName string, newTargets []string) {
	old := b.idx.Load()
	// Shallow clone: the []string value slices are shared with old.links.
	// This is safe because we never mutate existing slices — we only
	// replace entire map entries.
	newLinks := make(map[string][]string, len(old.links)+1)
	maps.Copy(newLinks, old.links)

	oldTargets := old.links[pageName] // already sorted+deduped
	newLinks[pageName] = newTargets

	// Surgically patch backlinks: diff old vs new targets (both sorted),
	// then update only the affected backlink entries.
	added, removed := diffSorted(oldTargets, newTargets)
	// Shallow clone: the []string value slices are shared with old.backlinks.
	// This is safe because removeFromSorted/insertIntoSorted never mutate
	// slices in place — they either return the original slice unchanged
	// or allocate a new one.
	newBacklinks := maps.Clone(old.backlinks)
	patchBacklinks(newBacklinks, pageName, added, removed)

	b.idx.Store(&idx{
		dirs:      old.dirs,
		pages:     uint64(len(newLinks)),
		links:     newLinks,
		backlinks: newBacklinks,
	})
}

func patchBacklinks(backlinks map[string][]string, pageName string, added, removed []string) {
	for _, target := range removed {
		if bl := backlinks[target]; len(bl) > 0 {
			patched := removeFromSorted(bl, pageName)
			if len(patched) == 0 {
				delete(backlinks, target)
			} else {
				backlinks[target] = patched
			}
		}
	}
	for _, target := range added {
		backlinks[target] = insertIntoSorted(backlinks[target], pageName)
	}
}

// diffSorted returns elements present in b but not a (added),
// and elements present in a but not b (removed).
// Both inputs must be sorted and deduplicated.
func diffSorted(a, b []string) (added, removed []string) {
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] < b[j]:
			removed = append(removed, a[i])
			i++
		case a[i] > b[j]:
			added = append(added, b[j])
			j++
		default:
			i++
			j++
		}
	}
	for ; i < len(a); i++ {
		removed = append(removed, a[i])
	}
	for ; j < len(b); j++ {
		added = append(added, b[j])
	}
	return added, removed
}

// removeFromSorted returns a new slice with val removed from the sorted slice.
// It MUST always return a new slice (or the original unchanged) — never mutate
// s in place. Callers rely on COW safety: the previous idx value (and its
// backlink slices) is still visible to concurrent readers via atomic.Load.
func removeFromSorted(s []string, val string) []string {
	i, found := slices.BinarySearch(s, val)
	if !found {
		return s
	}
	result := make([]string, len(s)-1)
	copy(result, s[:i])
	copy(result[i:], s[i+1:])
	return result
}

// insertIntoSorted returns a new slice with val inserted into the sorted slice.
// It MUST always return a new slice (or the original unchanged) — never mutate
// s in place. Callers rely on COW safety: the previous idx value (and its
// backlink slices) is still visible to concurrent readers via atomic.Load.
func insertIntoSorted(s []string, val string) []string {
	i, found := slices.BinarySearch(s, val)
	if found {
		return s // already present
	}
	result := make([]string, len(s)+1)
	copy(result, s[:i])
	result[i] = val
	copy(result[i+1:], s[i:])
	return result
}
