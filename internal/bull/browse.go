package bull

import (
	"bytes"
	"cmp"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
)

type browse struct {
	urlPrefix   string
	dir         string
	sortby      string
	sortorder   string
	directories string
	pages       []page
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
			slices.SortStableFunc(br.pages, func(a, b page) int {
				return cmp.Or(
					b.ModTime.Compare(a.ModTime),        // descending modtime
					cmp.Compare(a.PageName, b.PageName), // ascending tiebreaker
				)
			})
		} else {
			slices.SortStableFunc(br.pages, func(a, b page) int {
				return cmp.Or(
					a.ModTime.Compare(b.ModTime),        // ascending modtime
					cmp.Compare(a.PageName, b.PageName), // ascending tiebreaker
				)
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

func browseTableLine(name string, modTime time.Time, now time.Time) string {
	ts := modTime.Format("2006-01-02 15:04:05")
	ts += " • " + modTime.Format("Mon")
	if ago := now.Sub(modTime); ago >= 0 && ago < 24*time.Hour {
		hours := int(ago.Hours())
		minutes := int(ago.Minutes()) % 60
		seconds := int(ago.Seconds()) % 60
		if hours > 0 {
			ts += fmt.Sprintf(" • %dh %dm ago", hours, minutes)
		} else if minutes > 0 {
			ts += fmt.Sprintf(" • %dm %ds ago", minutes, seconds)
		} else {
			ts += fmt.Sprintf(" • %ds ago", seconds)
		}
	}
	return fmt.Sprintf("| %s | %s |\n", name, ts)
}

func (br *browse) browseTable() []string {
	now := time.Now()
	dirs := br.dirs()
	lines := make([]string, 0, len(br.pages))
	for _, pg := range br.pages {
		if dir, latest, skip := skip(dirs, pg.PageName); skip {
			if !latest.IsZero() {
				// This is the first time we encounter a page within this
				// directory, so produce a table line for the directory.
				name := fmt.Sprintf("[%s/](%s)", dir, br.browseDirLink(dir))
				lines = append(lines, browseTableLine(name, latest, now))
				dirs[dir] = time.Time{} // still present, but printed
			}
			if br.directories == "expand" {
				// keep going
			} else {
				continue
			}
		}

		lines = append(lines, browseTableLine("[["+pg.PageName+"]]", pg.ModTime, now))
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
	// one reading goroutine is sufficient, we only collect metadata
	readg.Go(func() {
		for pg := range i.readq {
			pages = append(pages, pg)
		}
	})
	if err := i.walk(); err != nil {
		return err
	}
	readg.Wait()

	br := browse{
		urlPrefix:   b.URLBullPrefix(),
		dir:         r.FormValue("dir"),
		sortby:      r.FormValue("sort"),
		sortorder:   r.FormValue("sortorder"),
		directories: r.FormValue("directories"),
		pages:       pages,
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

	fmt.Fprintf(&buf, "subdirectories: ")
	if br.directories == "expand" {
		fmt.Fprintf(&buf, "[collapse](%sbrowse?dir=%s&sort=%s&sortorder=%s&directories=) • **expand**\n", br.urlPrefix, url.QueryEscape(br.dir), br.sortby, br.sortorder)
	} else {
		fmt.Fprintf(&buf, "**collapse** • [expand](%sbrowse?dir=%s&sort=%s&sortorder=%s&directories=expand)\n", br.urlPrefix, url.QueryEscape(br.dir), br.sortby, br.sortorder)
	}

	fmt.Fprintf(&buf, "| page name [↑](%[1]sbrowse?dir=%[2]s&sort=pagename) [↓](%[1]sbrowse?dir=%[2]s&sort=pagename&sortorder=desc) | last modified [↑](%[1]sbrowse?dir=%[2]s&sort=modtime) [↓](%[1]sbrowse?dir=%[2]s&sort=modtime&sortorder=desc) |\n", br.urlPrefix, url.QueryEscape(br.dir))
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
