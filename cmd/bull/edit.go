package main

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

func (b *bull) edit(w http.ResponseWriter, r *http.Request) error {
	if b.editor == "" {
		return httpError(http.StatusForbidden, fmt.Errorf("running in read-only mode (-editor= flag)"))
	}

	possibilities := filesFromURL(r)
	pg, err := readFirst(b.content, possibilities)
	if err != nil {
		if os.IsNotExist(err) {
			// It is not an error if a page does not exist,
			// the edit handler can be used to create a page.
			pageName := pageFromURL(r)
			pg = &page{
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
		RequestPath     string
		ReadOnly        bool
		AbsolutePath    string
		Page            *page
		MarkdownContent string
	}{
		RequestPath:     r.URL.EscapedPath(),
		AbsolutePath:    pg.Abs(b.contentDir),
		Page:            pg,
		MarkdownContent: pg.Content,
	})
}
