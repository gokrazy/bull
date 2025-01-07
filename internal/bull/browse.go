package bull

import (
	"bytes"
	"fmt"
	"net/http"
	"sort"
	"sync"
)

func (b *bullServer) browse(w http.ResponseWriter, r *http.Request) error {
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
	sortorder := r.FormValue("sortorder")
	if sortorder != "desc" &&
		sortorder != "asc" &&
		sortorder != "" {
		return fmt.Errorf("unknown sortorder %q", sortorder)
	}

	switch r.FormValue("sort") {
	case "modtime":
		if sortorder == "desc" {
			sort.SliceStable(pages, func(i, j int) bool {
				return pages[i].ModTime.After(pages[j].ModTime)
			})
		} else {
			sort.SliceStable(pages, func(i, j int) bool {
				return pages[i].ModTime.Before(pages[j].ModTime)
			})
		}

	case "pagename", "":
		if sortorder == "desc" {
			sort.SliceStable(pages, func(i, j int) bool {
				return pages[i].PageName >= pages[j].PageName
			})
		} else {
			sort.SliceStable(pages, func(i, j int) bool {
				return pages[i].PageName < pages[j].PageName
			})
		}

	default:
		return fmt.Errorf("unknown ?sort parameter")
	}

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "# page browser\n")
	fmt.Fprintf(&buf, "| file name [↑](/_bull/browse?sort=pagename) [↓](/_bull/browse?sort=pagename&sortorder=desc) | last modified [↑](/_bull/browse?sort=modtime) [↓](/_bull/browse?sort=modtime&sortorder=desc) |\n")
	fmt.Fprintf(&buf, "|-----------|---------------|\n")
	for _, pg := range pages {
		fmt.Fprintf(&buf, "| [[%s]] | %s |\n",
			pg.PageName,
			pg.ModTime.Format("2006-01-02 15:04:05 Z07:00"))
	}
	return b.renderBullMarkdown(w, r, "browse", buf)
}
