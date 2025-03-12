package bull

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func (b *bullServer) save(w http.ResponseWriter, r *http.Request) error {
	if b.editor == "" {
		return httpError(http.StatusForbidden, fmt.Errorf("running in read-only mode (-editor= flag)"))
	}

	md := r.FormValue("markdown")
	if md == "" {
		return fmt.Errorf("markdown= parameter empty. to save an empty page, put at least a space")
	}

	// The HTML spec mandates that browser normalize line endings to \r\n:
	// https://html.spec.whatwg.org/multipage/form-control-infrastructure.html#multipart-form-data
	// We want to stick to UNIX line endings (\n) though:
	md = strings.ReplaceAll(md, "\r\n", "\n")

	pageName := pageFromURL(r)
	possibilities := page2files(pageName)

	// TODO(go1.25): use https://github.com/google/renameio/ to make writes
	// safer once Go 1.25 ships os.Root.Rename.

	var firstFn string
	for _, fn := range possibilities {
		_, err := b.content.Stat(fn)
		if err != nil {
			continue
		}
		firstFn = fn
		break
	}
	if firstFn == "" {
		firstFn = page2desired(pageName)
	}

	if err := mkdirAll(b.content, filepath.Dir(firstFn), 0755); err != nil {
		return err
	}
	f, err := b.content.OpenFile(firstFn, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write([]byte(md)); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	http.Redirect(w, r, b.root+pageName, http.StatusFound)
	return nil
}
