package bull

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"sync"
	"time"
)

type browse struct {
	urlPrefix string
	dir       string
	sortby    string
	sortorder string
	pages     []page
}

func (br *browse) prefix() string {
	return br.dir + "/"
}

// dirs find directories with at least one page
// and returns a map of directory to the latest modtime
// of the pages the directory contains.
func (br *browse) dirs() map[string]time.Time {
	seen := make(map[string]time.Time)
	for _, pg := range br.pages {
		pgdir := path.Dir(pg.PageName)
		if pgdir == br.dir {
			// do not hide any entries in the directory we are listing
			continue
		}
		latest, ok := seen[pgdir]
		if ok && latest.After(pg.ModTime) {
			continue
		}
		seen[pgdir] = pg.ModTime
	}
	return seen
}

func (br *browse) maybeFilterFilePrefix() {
	if br.dir == "" {
		return // nothing to filter
	}
	prefix := br.prefix()
	filtered := make([]page, 0, len(br.pages))
	for _, page := range br.pages {
		if !strings.HasPrefix(page.FileName, prefix) {
			continue
		}
		filtered = append(filtered, page)
	}
	br.pages = filtered
}

func (br *browse) sortPages() error {
	if br.sortorder != "desc" &&
		br.sortorder != "asc" &&
		br.sortorder != "" {
		return fmt.Errorf("unknown sortorder %q", br.sortorder)
	}

	switch br.sortby {
	case "modtime":
		if br.sortorder == "desc" {
			sort.SliceStable(br.pages, func(i, j int) bool {
				return br.pages[i].ModTime.After(br.pages[j].ModTime)
			})
		} else {
			sort.SliceStable(br.pages, func(i, j int) bool {
				return br.pages[i].ModTime.Before(br.pages[j].ModTime)
			})
		}

	case "pagename", "":
		if br.sortorder == "desc" {
			sort.SliceStable(br.pages, func(i, j int) bool {
				return br.pages[i].PageName >= br.pages[j].PageName
			})
		} else {
			sort.SliceStable(br.pages, func(i, j int) bool {
				return br.pages[i].PageName < br.pages[j].PageName
			})
		}

	default:
		return fmt.Errorf("unknown ?sort parameter")
	}

	return nil
}

// skip returns the earliest seen parent of the directory.
func skip(seen map[string]time.Time, rel string) (string, time.Time, bool) {
	var chomped string
	for {
		idx := strings.IndexByte(rel, '/')
		if idx == -1 {
			return "", time.Time{}, false
		}
		head := rel[:idx]
		component := chomped + head
		if latest, ok := seen[component]; ok {
			return component, latest, true
		}
		chomped += head + "/"
		rel = rel[idx+1:]
	}
}

func (br *browse) browseDirLink(dir string) string {
	q := url.Values{
		"dir":       []string{dir},
		"sort":      []string{br.sortby},
		"sortorder": []string{br.sortorder},
	}
	return (&url.URL{
		Path:     br.urlPrefix + "browse",
		RawQuery: q.Encode(),
	}).String()
}

func browseTableLine(name string, modTime time.Time) string {
	return fmt.Sprintf("| %s | %s |\n",
		name,
		modTime.Format("2006-01-02 15:04:05 Z07:00"))
}

func (br *browse) browseTable() []string {
	dirs := br.dirs()
	lines := make([]string, 0, len(br.pages))
	for _, pg := range br.pages {
		if dir, latest, skip := skip(dirs, pg.PageName); skip {
			if !latest.IsZero() {
				// This is the first time we encounter a page within this
				// directory, so produce a table line for the directory.
				name := fmt.Sprintf("[%s/](%s)", dir, br.browseDirLink(dir))
				lines = append(lines, browseTableLine(name, latest))
				dirs[dir] = time.Time{} // still present, but printed
			}
			continue
		}

		lines = append(lines, browseTableLine("[["+pg.PageName+"]]", pg.ModTime))
	}
	return lines
}

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

	br := browse{
		urlPrefix: b.URLBullPrefix(),
		dir:       r.FormValue("dir"),
		sortby:    r.FormValue("sort"),
		sortorder: r.FormValue("sortorder"),
		pages:     pages,
	}
	br.maybeFilterFilePrefix()
	if err := br.sortPages(); err != nil {
		return err
	}

	var buf bytes.Buffer
	if br.dir != "" {
		fmt.Fprintf(&buf, "# directory browser: %s\n", br.dir)
	} else {
		fmt.Fprintf(&buf, "# directory browser\n")
	}
	fmt.Fprintf(&buf, "| page name [↑](%[1]s/browse?dir=%[2]s&sort=pagename) [↓](%[1]s/browse?dir=%[2]s&sort=pagename&sortorder=desc) | last modified [↑](%[1]s/browse?dir=%[2]s&sort=modtime) [↓](%[1]s/browse?dir=%[2]s&sort=modtime&sortorder=desc) |\n", br.urlPrefix, br.dir)
	fmt.Fprintf(&buf, "|-----------|---------------|\n")
	// TODO: link to .. if dir != ""
	for _, line := range br.browseTable() {
		buf.Write([]byte(line))
	}
	pg := &page{
		Class:    "bull_gen_browse",
		Exists:   true,
		PageName: br.dir,
		FileName: page2desired(br.dir),
		Content:  buf.String(),
		ModTime:  time.Now(),
	}
	if pg.PageName == "" {
		pg.PageName = bullPrefix + "browse"
	}
	return b.renderMarkdown(w, r, pg, buf.Bytes())
}
