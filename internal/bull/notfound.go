package bull

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
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
		return fmt.Sprintf("Check out the [directory browser](%sbrowse) pages for a list of pages.", b.URLBullPrefix()), nil
	}

	return fmt.Sprintf(`bull did not find any pages (markdown files) in content directory
%q (non-markdown files: %d)

bull works with pages, so maybe you would like to:

* Start bull with a different content directory (`+"`--content`"+` flag).
* [Create an index page](%sedit/index) to get started from scratch.
`, b.contentDir, len(dirents), b.URLBullPrefix()), nil
}

func (b *bullServer) renderNotFound(w http.ResponseWriter, r *http.Request) error {
	pageName := pageFromURL(r)
	var buf bytes.Buffer
	fmt.Fprintf(&buf, `# Error: page %q not found

To create this page <a href="%sedit/%s">click here</a> or press <kbd>Ctrl/Meta</kbd> + <kbd>E</kbd>.
	`, pageName, b.URLBullPrefix(), url.PathEscape(pageName))
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
