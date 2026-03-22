package bull

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/google/renameio/v2"
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
	pf, err := renameio.NewPendingFile(firstFn, renameio.WithRoot(b.content))
	if err != nil {
		return err
	}
	defer pf.Cleanup()
	if _, err := pf.Write([]byte(md)); err != nil {
		return err
	}
	if err := pf.CloseAtomicallyReplace(); err != nil {
		return err
	}

	// Update backlink index
	pg, err := b.read(firstFn)
	if err != nil {
		log.Printf("index update after save: read: %v", err)
	} else {
		targets, err := b.linkTargets(pg)
		if err != nil {
			log.Printf("index update after save: linkTargets: %v", err)
		} else {
			b.updateIndex(pg.PageName, targets)
		}
	}
	b.notifyContentChanged()

	http.Redirect(w, r, b.root+pageName, http.StatusFound)
	return nil
}
