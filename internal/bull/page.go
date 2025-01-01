package bull

import (
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// A page is the logical unit of content that bull works with.
//
// Pages are either backed by files inside the content directory.
// or generated on the fly by bull itself (those have a FileName
// starting with _bull/).
type page struct {
	Exists   bool   // whether the page exists on disk (false for error pages)
	PageName string // relative to content directory, no .md suffix
	FileName string // relative to content directory, with .md suffix
	// Content is intentionally a string (immutable) instead of a []byte.
	Content string
	ModTime time.Time
}

func (p *page) AvailableAt(encodedPath string) bool {
	if p.PageName == "index" {
		return encodedPath == "/" || encodedPath == "/index"
	}
	return encodedPath == p.URLPath()
}

func (p *page) IsGenerated() bool {
	return strings.HasPrefix(p.PageName, bullPrefix)
}

func (p *page) URLPath() string {
	return "/" + url.PathEscape(p.PageName)
}

var homeDir = os.Getenv("HOME")

func (p *page) Abs(contentDir string) string {
	// We join the page filename with contentDir even for generated pages so
	// that the title of a tab is sufficient to identify the bull content
	// directory. In other words, /home/michael/keep/_bull/mostrecent is a
	// more useful title compared to _bull/mostrecent.
	abs := filepath.Join(contentDir, p.FileName)
	// Replace $HOME with ~ for brevity: ~/keep/_bull/mostrecent
	prefix := homeDir + string(filepath.Separator)
	if strings.HasPrefix(abs, prefix) {
		return "~/" + strings.TrimPrefix(abs, prefix)
	}
	return abs
}

func isMarkdown(file string) bool {
	return strings.HasSuffix(file, ".md") ||
		strings.HasSuffix(file, ".markdown")
}

func file2page(file string) string {
	if strings.HasSuffix(file, ".md") {
		return strings.TrimSuffix(file, ".md")
	}
	if strings.HasSuffix(file, ".markdown") {
		return strings.TrimSuffix(file, ".markdown")
	}
	return file // not a markdown file
}

func page2files(page string) []string {
	return []string{
		page2desired(page),
		page + ".markdown", // also accepted
	}
}

func page2desired(page string) string { return page + ".md" }

func (b *bullServer) readFirst(possibilities []string) (*page, error) {
	var firstErr error
	for _, fn := range possibilities {
		pg, err := b.read(fn)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		return pg, nil
	}
	return nil, firstErr
}

func (b *bullServer) read(file string) (*page, error) {
	f, err := b.content.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	content, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	if b.customization != nil {
		if apr := b.customization.AfterPageRead; apr != nil {
			content = apr(content)
		}
	}
	return &page{
		Exists:   true, // read from disk
		PageName: file2page(file),
		FileName: file,
		Content:  string(content),
		ModTime:  fi.ModTime(),
	}, nil
}

// pageFromURL returns the requested content page name from the HTTP request.
//
// Special case: an empty page name (URL /) resolves to index.
func pageFromURL(r *http.Request) string {
	page := strings.TrimPrefix(r.PathValue("page"), "/")
	if page == "" {
		page = "index"
	}
	return page
}

// filesFromURL takes the page name from the HTTP request
// and returns the possible content file names.
//
// Special case: an empty page name (URL /) resolves to index.
func filesFromURL(r *http.Request) []string {
	return page2files(pageFromURL(r))
}
