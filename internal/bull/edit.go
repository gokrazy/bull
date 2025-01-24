package bull

import (
	"fmt"
	"net/http"
	"os"
	"time"

	thirdparty "github.com/gokrazy/bull/third_party"
)

func (b *bullServer) edit(w http.ResponseWriter, r *http.Request) error {
	if b.editor == "" {
		return httpError(http.StatusForbidden, fmt.Errorf("running in read-only mode (-editor= flag)"))
	}

	possibilities := filesFromURL(r)
	pg, err := b.readFirst(possibilities)
	if err != nil {
		if os.IsNotExist(err) {
			// It is not an error if a page does not exist,
			// the edit handler can be used to create a page.
			pageName := pageFromURL(r)
			pg = &page{
				Exists:   false,
				FileName: page2desired(pageName),
				PageName: pageName,
				Content:  "",          // file does not exist
				ModTime:  time.Time{}, // file does not exist
			}
		} else {
			return err
		}
	}

	return b.executeTemplate(w, "edit.html.tmpl", struct {
		URLPrefix            string
		URLBullPrefix        string
		RequestPath          string
		ReadOnly             bool
		Title                string
		Page                 *page
		MarkdownContent      string
		StaticHash           func(string) string
		StaticHashCodeMirror func() string
	}{
		URLPrefix:     b.root,
		URLBullPrefix: b.URLBullPrefix(),
		RequestPath:   r.URL.EscapedPath(),
		Title:         "edit: " + insideOutTitle(pg.FileName, b.contentDir),
		Page:          pg,
		// For editing, we need to use the page contents as stored on disk,
		// without any customization post-processing.
		MarkdownContent: pg.DiskContent,
		StaticHash:      b.staticHash,
		StaticHashCodeMirror: func() string {
			return hashSum(thirdparty.BullCodemirror)
		},
	})
}
