package bull

import (
	"fmt"
	"net/http"
	"time"
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

	// TODO(performance): inotify fast path?

	poller := time.NewTicker(1 * time.Second)
	defer poller.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-poller.C:
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
}
