package bull

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type uploadResponse struct {
	File      string `json:"file"`
	SavedPath string `json:"savedPath"`
}

// upload Saves a file to <page>-<timestamp>-<original filename>
func (b *bullServer) upload(w http.ResponseWriter, r *http.Request) error {
	if b.editor == "" {
		return httpError(http.StatusForbidden, fmt.Errorf("running in read-only mode (-editor= flag)"))
	}

	// Limit the size of the memory to 10MB
	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		return err
	}

	file, handler, err := r.FormFile("file")
	if err != nil {
		log.Printf("error reading file: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	defer file.Close()

	pageName := pageFromURL(r)
	savedFileName := fmt.Sprintf("%s-%d-%s", pageName, time.Now().UnixMilli(), handler.Filename)

	if err := mkdirAll(b.content, filepath.Dir(savedFileName), 0755); err != nil {
		return err
	}
	f, err := b.content.OpenFile(savedFileName, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, file); err != nil {
		return err
	}

	resp := uploadResponse{
		File:      handler.Filename,
		SavedPath: savedFileName,
	}
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, err = w.Write(data)
	return err
}
