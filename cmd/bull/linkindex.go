package main

import (
	"context"
	"errors"
	"io/fs"
	"log"
	"os"
	"path"
	"runtime"
	"slices"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yuin/goldmark/ast"
	"go.abhg.dev/goldmark/wikilink"
	"golang.org/x/sync/errgroup"
)

type idx struct {
	dirs, pages uint64
	// backlinks maps from page name (e.g. index) to
	// page names that contain a link to that page (e.g. SETTINGS, projects, …).
	backlinks map[string][]string
}

func linkTargets(contentRoot *os.Root, pg *page) ([]string, error) {
	var targets []string

	doc := renderMD(contentRoot, pg.Content)
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

	sort.Strings(targets)
	return slices.Compact(targets), nil
}

type indexer struct {
	contentRoot *os.Root
	walkq       *queue
	readq       chan string
	dirs, pages atomic.Uint64
	pending     atomic.Int64
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
		i.readq <- path.Join(dir, name)
	}
	return nil
}

func index(contentRoot *os.Root) (*idx, error) {
	i := &indexer{
		contentRoot: contentRoot,
		walkq:       newQueue(),
		readq:       make(chan string),
	}
	i.dirDiscovered()
	i.walkq.Push(".")

	var (
		linksMu sync.Mutex
		links   = make(map[string][]string)
	)
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
				defer popcanc()
				dir, err := i.walkq.PopOrWait(popctx)
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
	var readg errgroup.Group
	for range runtime.NumCPU() {
		readg.Go(func() error {
			linksN := make(map[string][]string)
			for fn := range i.readq {
				// fmt.Printf("reading %s\n", fn)
				pg, err := read(contentRoot, fn)
				if err != nil {
					return err
				}
				targets, err := linkTargets(contentRoot, pg)
				if err != nil {
					return err
				}
				linksN[pg.PageName] = targets
			}
			linksMu.Lock()
			defer linksMu.Unlock()
			for k, v := range linksN {
				links[k] = v
			}
			return nil
		})
	}
	if err := walkg.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		return nil, err
	}
	close(i.readq)
	if err := readg.Wait(); err != nil {
		return nil, err
	}
	// Invert the index
	// TODO: do we need to check if the target exists?
	// or is it sufficient that we do not query it because we never render it?
	backlinks := make(map[string][]string)
	for pageName, targets := range links {
		for _, target := range targets {
			backlinks[target] = append(backlinks[target], pageName)
		}
	}
	for pageName, linkers := range backlinks {
		sort.Strings(linkers)
		backlinks[pageName] = linkers
	}
	return &idx{
		dirs:      i.dirs.Load(),
		pages:     i.pages.Load(),
		backlinks: backlinks,
	}, nil
}
