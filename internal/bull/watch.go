package bull

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

func initEventStream(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// Instruct nginx to disable buffering:
	w.Header().Set("X-Accel-Buffering", "no")

	w.WriteHeader(http.StatusOK)

	w.Write([]byte{'\n'})
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

func maybeNotify(ctx context.Context, notify chan<- struct{}, fileName string) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("fsnotify.NewWatcher: %v", err)
		return
	}
	go func() {
		defer w.Close()
		for {
			select {
			case <-ctx.Done():
				return

			case err, ok := <-w.Errors:
				if !ok {
					return // watcher closed
				}
				log.Printf("fsnotify error: %v", err)

			case e, ok := <-w.Events:
				if !ok {
					return // watcher closed
				}
				if e.Name == fileName {
					notify <- struct{}{}
				}
			}
		}
	}()
	// Watch the directory instead of the file itself:
	// Editors replace files with an updated version.
	dir := filepath.Dir(fileName)
	if err := w.Add(dir); err != nil {
		w.Close()
		log.Printf("fsnotify.Watch(%s): %v", dir, err)
		return
	}
}

func (b *bullServer) watch(w http.ResponseWriter, r *http.Request) error {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("BUG: ResponseWriter does not implement http.Flusher")
	}

	ctx := r.Context()

	possibilities := filesFromURL(r)
	lastb, err := b.readFirst(possibilities)
	if err != nil {
		return err
	}

	initEventStream(w)

	// Each watch request contains the page ContentHash() as a URL parameter,
	// so that we can immediately emit a change even when the client was
	// not connected during the time of the actual change.
	if rhash := r.FormValue("hash"); rhash != "" {
		if current := lastb.ContentHash(); current != rhash {
			w.Write([]byte("data: {\"changed\":true}\n\n"))
			flusher.Flush()
		}
	}

	notify := make(chan struct{})
	maybeNotify(ctx, notify, filepath.Join(b.contentDir, lastb.FileName))

	poller := time.NewTicker(1 * time.Second)
	defer poller.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-poller.C:
		case <-notify:
		}

		b, err := b.readFirst(possibilities)
		if err != nil {
			return err
		}
		if lastb.Content == b.Content {
			continue
		}
		lastb = b
		w.Write([]byte("data: {\"changed\":true}\n\n"))
		flusher.Flush()
	}
}
