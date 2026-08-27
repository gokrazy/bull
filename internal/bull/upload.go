package bull

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/google/renameio/v2"
)

// upload handles file uploads, for example triggered by paste or drag&drop on
// the CodeMirror editor (if enabled).
func (b *bullServer) upload(w http.ResponseWriter, r *http.Request) error {
	if b.editor == "" {
		return httpError(http.StatusForbidden, fmt.Errorf("running in read-only mode (-editor= flag)"))
	}

	// Find the directory of the page for which the attachment is uploaded.
	pageName := pageFromURL(r)
	dir := path.Dir(page2desired(pageName))

	_, err := b.content.Stat(dir)
	if os.IsNotExist(err) {
		if err := mkdirAll(b.content, dir, 0o777); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	q := r.URL.Query()
	filename := q.Get("filename")
	if filename == "" {
		return httpError(http.StatusBadRequest, fmt.Errorf("no ?filename query parameter found"))
	}
	destFnBase := path.Join(dir, filename)
	ext := path.Ext(destFnBase)
	if ext == "" {
		return httpError(http.StatusBadRequest, fmt.Errorf("invalid filename: no extension"))
	}
	lastMod, err := strconv.ParseInt(q.Get("lastmodified"), 0, 64)
	if err != nil {
		return httpError(http.StatusBadRequest, fmt.Errorf("invalid lastmodified: %v", err))
	}
	mtime := time.Unix(lastMod, 0)
	destFn := destFnBase

	// The following code is written for the following concerns:
	//
	// 1. We prevent overwriting existing files by creating a new file
	//    (exclusively) with a new name until we grab an available name.
	//
	// 2. We prevent incomplete uploads from going unnoticed by keeping the
	//    destination file at 0 bytes and atomically renaming over it.
	//    The renameio rename replaces the file.
	var lastErr error
	for attempt := 1; attempt < 100; attempt++ {
		if attempt > 1 {
			destFn = fmt.Sprintf("%s_%03d%s", strings.TrimSuffix(destFnBase, ext), attempt, ext)
		}
		f, err := b.content.OpenFile(destFn, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
		if err != nil {
			if os.IsExist(err) {
				// Some other upload raced us for this file, try another name.
				lastErr = err
				continue
			}
			return err
		}
		// Name claimed, close the file.
		lastErr = f.Close()
		break
	}
	if lastErr != nil {
		return lastErr
	}

	success := false
	defer func() {
		if success {
			return
		}
		b.content.Remove(destFn)
	}()

	pf, err := renameio.NewPendingFile(destFn, renameio.WithRoot(b.content), renameio.WithPermissions(0666))
	if err != nil {
		return err
	}
	defer pf.Cleanup()
	sz, err := io.Copy(pf, r.Body)
	if err != nil {
		return err
	}
	if err := pf.CloseAtomicallyReplace(); err != nil {
		return err
	}
	success = true
	if err := b.content.Chtimes(destFn, mtime, mtime); err != nil {
		return err
	}

	log.Printf("%s: uploaded %s (bytes: %d)", r.URL.Path, destFn, sz)

	jsonb, err := json.Marshal(struct {
		Path string `json:"path"`
		Size int64  `json:"size"`
	}{
		Path: destFn,
		Size: sz,
	})
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(jsonb)
	return err
}
