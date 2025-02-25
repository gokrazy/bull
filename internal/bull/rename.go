package bull

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func (b *bullServer) rename(w http.ResponseWriter, r *http.Request) error {
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
	src := r.PathValue("page")
	dest := r.FormValue("newname")
	log.Printf("renaming page=%q to newname=%q", src, dest)

	start := time.Now()
	log.Printf("indexing all pages (markdown files) in %s", b.content.Name())
	idx, err := b.index()
	if err != nil {
		return err
	}
	log.Printf("discovered in %.2fs: directories: %d, pages: %d, links: %d", time.Since(start).Seconds(), idx.dirs, idx.pages, len(idx.backlinks))

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

	log.Printf("mv %q %q", pg.FileName, dest)

	oldpath := filepath.Join(b.contentDir, pg.FileName)
	newpath := filepath.Join(b.contentDir, dest)
	if err := mkdirAll(b.content, filepath.Dir(dest), 0755); err != nil {
		return err
	}
	if err := os.Rename(oldpath, newpath); err != nil {
		return err
	}

	log.Printf("# backlinks: %d", len(idx.backlinks[pg.PageName]))
	for _, linker := range idx.backlinks[pg.PageName] {
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

	destPg, err := b.read(dest)
	if err != nil {
		return err
	}
	http.Redirect(w, r, b.root+destPg.URLPath(), http.StatusFound)

	return nil
}
