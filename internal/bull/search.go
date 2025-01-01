package bull

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"runtime"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"
)

func (b *bullServer) search(w http.ResponseWriter, r *http.Request) error {
	// TODO: implement server-rendered version for non-javascript
	const pageName = bullPrefix + "search"
	return b.executeTemplate(w, "search.html.tmpl", struct {
		Title       string
		RequestPath string
		Page        *page
		ReadOnly    bool
	}{
		Title:       "search",
		RequestPath: r.URL.EscapedPath(),
		Page: &page{
			PageName: pageName,
			FileName: page2desired(pageName),
		},
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

type match struct {
	Type          string   `json:"type"`
	PageName      string   `json:"page_name"`
	MatchingLines []string `json:"matching_lines"`
}

func (b *bullServer) searchAPI(w http.ResponseWriter, r *http.Request) error {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("BUG: ResponseWriter does not implement http.Flusher")
	}

	query := r.FormValue("q")
	log.Printf("searching for query %q", query)

	ctx := r.Context()

	initEventStream(w)

	i := newIndexer(b.content)

	var (
		results = make(chan []byte)
		readg   errgroup.Group
		streamg sync.WaitGroup
	)
	streamg.Add(1)
	go func() {
		defer streamg.Done()
		for result := range results {
			w.Write(append(append([]byte("data: "), result...), '\n', '\n'))
			flusher.Flush()
		}
	}()
	for range runtime.NumCPU() {
		readg.Go(func() error {
			for pg := range i.readq {
				if err := ctx.Err(); err != nil {
					return err
				}
				// fmt.Printf("reading %s\n", pg.FileName)
				pg, err := read(b.content, pg.FileName)
				if err != nil {
					// TODO: send an error result
					log.Printf("read: %v", err)
					continue
				}
				matches := grep(pg.Content, query)
				matches = append(matches, grep(pg.PageName, query)...)
				if len(matches) == 0 {
					continue
				}
				b, err := json.Marshal(match{
					Type:          "result",
					PageName:      pg.PageName,
					MatchingLines: matches,
				})
				if err != nil {
					return err // BUG
				}
				results <- b
			}
			return nil
		})
	}
	if err := i.walk(); err != nil {
		return err
	}
	if err := readg.Wait(); err != nil {
		return err
	}
	// Synchronize with the streaming goroutine to ensure it no longer tries to
	// use the ResponseWriter by the time this handler returns.
	close(results)
	streamg.Wait()

	// TODO: stream progress every once in a while

	w.Write([]byte(`data: {"type":"done"}` + "\n\n"))
	flusher.Flush()
	return nil
}
