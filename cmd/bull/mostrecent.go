package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"net/http"
	"sort"
)

func (b *bull) mostrecent(w http.ResponseWriter, r *http.Request) error {
	// walk the entire content directory
	var pages []*page
	err := fs.WalkDir(b.content.FS(), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !isMarkdown(path) {
			return nil
		}

		// save path and modtime for sorting
		info, err := d.Info()
		if err != nil {
			return err
		}
		pages = append(pages, &page{
			PageName: file2page(path),
			FileName: path,
			Content:  "", // intentionally left blank
			ModTime:  info.ModTime(),
		})
		return nil
	})
	if err != nil {
		return err
	}
	sort.Slice(pages, func(i, j int) bool {
		return pages[i].ModTime.After(pages[j].ModTime)
	})
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "# most recent\n")
	fmt.Fprintf(&buf, "| file name | last modified |\n")
	fmt.Fprintf(&buf, "|-----------|---------------|\n")
	for _, pg := range pages {
		fmt.Fprintf(&buf, "| [[%s]] | %s |\n",
			pg.PageName,
			pg.ModTime.Format("2006-01-02 15:04:05 Z07:00"))
	}
	return b.renderBullMarkdown(w, r, "mostrecent", buf)
}
