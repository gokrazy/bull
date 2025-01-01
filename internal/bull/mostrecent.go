package bull

import (
	"bytes"
	"fmt"
	"net/http"
	"sort"
	"sync"
)

func (b *bullServer) mostrecent(w http.ResponseWriter, r *http.Request) error {
	// walk the entire content directory
	i := newIndexer(b.content)
	i.readModTime = true // required for sorting by most recent
	var (
		pages []page
		readg sync.WaitGroup
	)
	readg.Add(1)
	// one reading goroutine is sufficient, we only collect metadata
	go func() {
		defer readg.Done()
		for pg := range i.readq {
			pages = append(pages, pg)
		}
	}()
	if err := i.walk(); err != nil {
		return err
	}
	readg.Wait()

	sort.SliceStable(pages, func(i, j int) bool {
		return pages[i].ModTime.After(pages[j].ModTime)
	})
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "# most recent\n")
	fmt.Fprintf(&buf, "| file name | last modified |\n")
	fmt.Fprintf(&buf, "|-----------|---------------|\n")
	for _, pg := range pages {
		fmt.Fprintf(&buf, "| [[%s]] | %s |\n",
			pg.PageName,
			pg.ModTime.Format("2006-01-02 15:04:05 Z07:00"))
	}
	return b.renderBullMarkdown(w, r, "mostrecent", buf)
}
