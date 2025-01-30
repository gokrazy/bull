package bull

import (
	"bytes"
	"fmt"
	"net/http"
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
	fmt.Fprintf(w, "TODO: rename from %q to %q", r.PathValue("page"), r.FormValue("newname"))
	return nil
}
