package bull

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
)

func (b *bullServer) search(w http.ResponseWriter, r *http.Request) error {
	// TODO: implement server-rendered version for non-javascript,
	// i.e. POST /_bull/search handler
	const pageName = bullPrefix + "search"
	return b.executeTemplate(w, "search.html.tmpl", struct {
		URLPrefix     string
		URLBullPrefix string
		Title         string
		RequestPath   string
		Page          *page
		ReadOnly      bool
		Query         string
		StaticHash    func(string) string
	}{
		URLPrefix:     b.root,
		URLBullPrefix: b.URLBullPrefix(),
		Title:         "search",
		RequestPath:   r.URL.EscapedPath(),
		Page: &page{
			PageName: pageName,
			FileName: page2desired(pageName),
		},
		Query:      r.FormValue("q"),
		StaticHash: b.staticHash,
	})
}

func grep(content, query string) []string {
	queryl := strings.ToLower(query)
	var matches []string
	for _, line := range strings.Split(content, "\n") {
		if strings.Contains(strings.ToLower(line), queryl) {
			matches = append(matches, line)
		}
	}
	return matches
}

type progressUpdate struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type match struct {
	Type          string   `json:"type"`
	PageName      string   `json:"page_name"`
	MatchingLines []string `json:"matching_lines"`
	score         float64
}

func (b *bullServer) internalsearch(ctx context.Context, query string, progress chan<- progressUpdate) ([]match, error) {
	log.Printf("searching for query %q", query)

	i := newIndexer(b.content)

	var (
		resultsMu sync.Mutex
		results   []match

		readg     errgroup.Group
		progressg sync.WaitGroup

		filesRead atomic.Uint64
	)
	progressCtx, progressCanc := context.WithCancel(ctx)
	defer progressCanc()
	if progress != nil {
		progressg.Add(1)
		go func() {
			defer progressg.Done()
			for {
				select {
				case <-progressCtx.Done():
					return
				case <-time.After(1 * time.Second):
					progress <- progressUpdate{
						Type:    "progress",
						Message: fmt.Sprintf("searched through %d files", filesRead.Load()),
					}
				}
			}
		}()
	}
	for range runtime.NumCPU() {
		readg.Go(func() error {
			for pg := range i.readq {
				if err := ctx.Err(); err != nil {
					return err
				}
				// fmt.Printf("reading %s\n", pg.FileName)
				pg, err := b.read(pg.FileName)
				if err != nil {
					// TODO: send an error result
					log.Printf("read: %v", err)
					continue
				}
				filesRead.Add(1)
				matches := grep(pg.Content, query)
				nameMatches := grep(pg.PageName, query)
				matches = append(matches, nameMatches...)
				if len(matches) == 0 {
					continue
				}
				var score float64
				if len(nameMatches) > 0 {
					if pg.PageName == query {
						score = 1 // exact page match
					} else if strings.HasPrefix(pg.PageName, query) {
						score = 0.9 // prefix match
					} else {
						score = 0.5 // name match
					}
				}
				m := match{
					Type:          "result",
					PageName:      pg.PageName,
					MatchingLines: matches,
					score:         score,
				}
				resultsMu.Lock()
				results = append(results, m)
				resultsMu.Unlock()
			}
			return nil
		})
	}
	if err := i.walk(); err != nil {
		return nil, err
	}
	if err := readg.Wait(); err != nil {
		return nil, err
	}
	// Synchronize with the progress update goroutine to ensure it no longer
	// tries to use the ResponseWriter by the time this handler returns.
	progressCanc()
	progressg.Wait()

	sort.SliceStable(results, func(i, j int) bool {
		ri := results[i]
		rj := results[j]
		if ri.score == rj.score {
			// both are the same kind of match
			return ri.PageName < rj.PageName
		}
		return ri.score > rj.score
	})

	return results, nil
}

func (b *bullServer) searchAPI(w http.ResponseWriter, r *http.Request) error {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("BUG: ResponseWriter does not implement http.Flusher")
	}

	query := r.FormValue("q")
	if query == "" {
		return httpError(http.StatusBadRequest, fmt.Errorf("empty q= parameter not allowed"))
	}
	if len(query) < 2 {
		return httpError(http.StatusBadRequest, fmt.Errorf("minimum query length: 2 characters"))
	}

	ctx := r.Context()
	initEventStream(w)

	progress := make(chan progressUpdate)
	defer close(progress)
	go func() {
		for update := range progress {
			b, err := json.Marshal(update)
			if err != nil {
				log.Print(err)
				return
			}
			if err := ctx.Err(); err != nil {
				return
			}
			w.Write(append(append([]byte("data: "), b...), '\n', '\n'))
			flusher.Flush()
		}
	}()

	start := time.Now()
	results, err := b.internalsearch(ctx, query, progress)
	if err != nil {
		return err
	}
	log.Printf("search for query %q done in %v, now streaming results", query, time.Since(start))

	// stream search results
	for _, result := range results {
		b, err := json.Marshal(result)
		if err != nil {
			return err
		}
		w.Write(append(append([]byte("data: "), b...), '\n', '\n'))
	}

	w.Write([]byte(`data: {"type":"done"}` + "\n\n"))
	flusher.Flush()
	return nil
}
