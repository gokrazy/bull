package main

import (
	"net/http"
	"time"
)

func (b *bull) watch(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	possibilities := filesFromURL(r)
	lastb, err := readFirst(b.content, possibilities)
	if err != nil {
		return err
	}

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

	// TODO(performance): inotify fast path?

	poller := time.NewTicker(1 * time.Second)
	defer poller.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-poller.C:
			b, err := readFirst(b.content, possibilities)
			if err != nil {
				return err
			}
			if lastb.Content == b.Content {
				continue
			}
			lastb = b
			w.Write([]byte("data: {\"changed\":true}\n\n"))
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
	}
}
