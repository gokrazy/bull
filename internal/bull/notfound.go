package bull

import (
	"bytes"
	"fmt"
	"net/http"
	"time"
)

func (b *bullServer) indexNotFound() (string, error) {
	f, err := b.content.Open(".")
	if err != nil {
		return "", err
	}
	dirents, err := f.ReadDir(-1)
	if err != nil {
		return "", err
	}
	hasPages := false
	for _, dirent := range dirents {
		if isMarkdown(dirent.Name()) {
			hasPages = true
			break
		}
	}
	if hasPages {
		return "Check out the [most recent](/_bull/mostrecent) pages for a list of pages.", nil
	}

	return fmt.Sprintf(`bull did not find any pages (markdown files) in content directory
%q (non-markdown files: %d)

bull works with pages, so maybe you would like to:

* Start bull with a different content directory (`+"`-content`"+` flag).
* [Create an index page](/_bull/edit/index) to get started from scratch.
`, b.contentDir, len(dirents)), nil
}

func (b *bullServer) renderNotFound(w http.ResponseWriter, r *http.Request) error {
	pageName := pageFromURL(r)
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "# Error: page %q not found\n", pageName)
	if pageName == "index" {
		nf, err := b.indexNotFound()
		if err != nil {
			return err
		}
		buf.WriteString(nf)
	}

	pg := &page{
		Exists:   false,
		FileName: page2desired(pageName),
		PageName: pageName,
		Content:  buf.String(),
		ModTime:  time.Now(),
	}
	w.WriteHeader(http.StatusNotFound)
	return b.renderMarkdown(w, r, pg, buf.Bytes())
}
