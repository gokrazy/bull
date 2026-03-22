package bull

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

func (b *bullServer) rename(w http.ResponseWriter, r *http.Request) error {
	if b.editor == "" {
		return httpError(http.StatusForbidden, fmt.Errorf("running in read-only mode (-editor= flag)"))
	}
	possibilities := filesFromURL(r)
	pg, err := b.readFirst(possibilities)
	if err != nil {
		return err
	}
	pg.Exists = false // do not add page title in page template

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "# Rename page %q\n", pg.PageName)
	fmt.Fprintf(&buf, `<form action="%s_rename/%s" method="post" class="bull_rename">`, b.URLBullPrefix(), pg.URLPath())
	fmt.Fprintf(&buf, `<label for="bull_newname">New name:</label>`)
	fmt.Fprintf(&buf, `<input id="bull_newname" type="text" name="newname" value="%s" autofocus="autofocus" onfocus="this.select()">`, pg.PageName)
	fmt.Fprintf(&buf, `<br>`)
	fmt.Fprintf(&buf, `<input type="submit" value="Rename and update links">`)
	fmt.Fprintf(&buf, `</form>`)

	pg.Content = buf.String()
	return b.renderMarkdown(w, r, pg, buf.Bytes())
}

func (b *bullServer) renameAPI(w http.ResponseWriter, r *http.Request) error {
	if b.editor == "" {
		return httpError(http.StatusForbidden, fmt.Errorf("running in read-only mode (-editor= flag)"))
	}
	src := r.PathValue("page")
	dest := r.FormValue("newname")
	log.Printf("renaming page=%q to newname=%q", src, dest)

	if !filepath.IsLocal(dest) {
		return httpError(http.StatusBadRequest, fmt.Errorf("invalid destination path: %q", dest))
	}
	if !filepath.IsLocal(src) {
		return httpError(http.StatusBadRequest, fmt.Errorf("invalid source path: %q", src))
	}

	currentIdx := b.idx.Load()

	possibilities := page2files(src)
	if isMarkdown(src) {
		possibilities = []string{src}
	}
	pg, err := b.readFirst(possibilities)
	if err != nil {
		return err
	}
	destPage := file2page(dest)
	if !isMarkdown(dest) {
		destPage = dest
		dest = page2desired(dest)
	}

	// Note: there is a TOCTOU race between this check and os.Rename below.
	if destPg, err := b.readFirst(page2files(destPage)); err == nil {
		return httpError(http.StatusConflict,
			fmt.Errorf("destination page %q already exists (see /%s)", destPage, destPg.URLPath()))
	}

	log.Printf("mv %q %q", pg.FileName, dest)

	oldpath := filepath.Join(b.contentDir, pg.FileName)
	newpath := filepath.Join(b.contentDir, dest)
	if err := mkdirAll(b.content, filepath.Dir(dest), 0755); err != nil {
		return err
	}
	if err := os.Rename(oldpath, newpath); err != nil {
		return err
	}

	linkers := currentIdx.backlinks[pg.PageName]
	log.Printf("# backlinks: %d", len(linkers))
	for _, linker := range linkers {
		linkerpg, err := b.readFirst(page2files(linker))
		if err != nil {
			log.Printf("  not found: %v", err)
			continue
		}

		log.Printf(`replace [[%s]] → [[%s]] in %s`, pg.PageName, destPage, linkerpg.FileName)

		if err := b.replaceLinks(linkerpg.FileName, pg.PageName, destPage); err != nil {
			log.Printf("  failed: %v", err)
		}
	}

	// Read the renamed page and compute targets before modifying the index,
	// so that a read failure does not leave the index with a hole.
	newPg, err := b.read(dest)
	if err != nil {
		return err
	}
	newTargets, err := b.linkTargets(newPg)
	if err != nil {
		return fmt.Errorf("index update after rename: %v", err)
	}

	// Pre-read all linker pages outside the lock to avoid holding idxMu
	// during disk I/O.
	type linkerUpdate struct {
		pageName string
		targets  []string
	}
	var linkerUpdates []linkerUpdate
	for _, linker := range linkers {
		linkerpg, err := b.readFirst(page2files(linker))
		if err != nil {
			log.Printf("rename: re-index linker %s: %v", linker, err)
			continue
		}
		targets, err := b.linkTargets(linkerpg)
		if err != nil {
			log.Printf("rename: linkTargets for linker %s: %v", linker, err)
			continue
		}
		linkerUpdates = append(linkerUpdates, linkerUpdate{linkerpg.PageName, targets})
	}

	// Update index atomically: single clone-patch-store cycle
	// to prevent fswatch from interleaving partial state.
	updates := make([]indexUpdate, 0, 1+len(linkerUpdates))
	updates = append(updates, indexUpdate{destPage, newTargets})
	for _, lu := range linkerUpdates {
		updates = append(updates, indexUpdate(lu))
	}
	b.idxMu.Lock()
	b.applyIndexBatchLocked([]string{pg.PageName}, updates)
	b.idxMu.Unlock()
	// Notify outside idxMu to maintain consistent lock ordering
	// (idxMu is never held when acquiring contentChangedMu).
	b.notifyContentChanged()

	http.Redirect(w, r, b.root+newPg.URLPath(), http.StatusFound)

	return nil
}
