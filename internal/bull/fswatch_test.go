package bull

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fsnotify/fsnotify"
	"github.com/google/go-cmp/cmp"
)

func TestHandleContentEventWrite(t *testing.T) {
	b := newTestBull(t, map[string]string{
		"alpha.md": "see [[beta]]",
		"beta.md":  "hello",
	})
	idx, err := b.index()
	if err != nil {
		t.Fatal(err)
	}
	b.idx.Store(idx)

	// Overwrite alpha to link to gamma instead.
	if err := os.WriteFile(filepath.Join(b.contentDir, "alpha.md"), []byte("see [[gamma]]"), 0644); err != nil {
		t.Fatal(err)
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	updated := b.handleContentEvent(w, fsnotify.Event{
		Name: filepath.Join(b.contentDir, "alpha.md"),
		Op:   fsnotify.Write,
	})
	if !updated {
		t.Fatal("expected index update, got false")
	}

	cur := b.idx.Load()
	if got := cur.backlinks["beta"]; len(got) != 0 {
		t.Errorf("backlinks[beta] = %v, want []", got)
	}
	if diff := cmp.Diff([]string{"alpha"}, cur.backlinks["gamma"]); diff != "" {
		t.Errorf("backlinks[gamma] mismatch (-want +got):\n%s", diff)
	}
}

func TestHandleContentEventRemove(t *testing.T) {
	b := newTestBull(t, map[string]string{
		"alpha.md": "see [[beta]]",
		"beta.md":  "hello",
	})
	idx, err := b.index()
	if err != nil {
		t.Fatal(err)
	}
	b.idx.Store(idx)

	w, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	updated := b.handleContentEvent(w, fsnotify.Event{
		Name: filepath.Join(b.contentDir, "alpha.md"),
		Op:   fsnotify.Remove,
	})
	if !updated {
		t.Fatal("expected index update, got false")
	}

	cur := b.idx.Load()
	if got := cur.backlinks["beta"]; len(got) != 0 {
		t.Errorf("backlinks[beta] = %v, want []", got)
	}
	if got := cur.links["alpha"]; got != nil {
		t.Errorf("links[alpha] = %v, want nil", got)
	}
}

func TestHandleContentEventNonMarkdown(t *testing.T) {
	b := newTestBull(t, map[string]string{
		"alpha.md": "hello",
	})
	idx, err := b.index()
	if err != nil {
		t.Fatal(err)
	}
	b.idx.Store(idx)

	// Write a non-markdown file.
	if err := os.WriteFile(filepath.Join(b.contentDir, "photo.jpg"), []byte("fake"), 0644); err != nil {
		t.Fatal(err)
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	updated := b.handleContentEvent(w, fsnotify.Event{
		Name: filepath.Join(b.contentDir, "photo.jpg"),
		Op:   fsnotify.Write,
	})
	if updated {
		t.Fatal("non-markdown file should not trigger index update")
	}
}

func TestHandleContentEventDedup(t *testing.T) {
	b := newTestBull(t, map[string]string{
		"alpha.md": "see [[beta]]",
		"beta.md":  "hello",
	})
	idx, err := b.index()
	if err != nil {
		t.Fatal(err)
	}
	b.idx.Store(idx)

	// Write the same content — targets unchanged.
	w, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	updated := b.handleContentEvent(w, fsnotify.Event{
		Name: filepath.Join(b.contentDir, "alpha.md"),
		Op:   fsnotify.Write,
	})
	if updated {
		t.Fatal("unchanged targets should not trigger index update")
	}
}

func TestHandleContentEventCreateDir(t *testing.T) {
	b := newTestBull(t, map[string]string{
		"alpha.md": "hello",
	})
	idx, err := b.index()
	if err != nil {
		t.Fatal(err)
	}
	b.idx.Store(idx)

	// Create a subdirectory with a markdown file already in it.
	subdir := filepath.Join(b.contentDir, "sub")
	if err := os.Mkdir(subdir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subdir, "page.md"), []byte("see [[alpha]]"), 0644); err != nil {
		t.Fatal(err)
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	updated := b.handleContentEvent(w, fsnotify.Event{
		Name: filepath.Join(b.contentDir, "sub"),
		Op:   fsnotify.Create,
	})
	if !updated {
		t.Fatal("expected index update from new directory scan, got false")
	}

	cur := b.idx.Load()
	if diff := cmp.Diff([]string{"sub/page"}, cur.backlinks["alpha"]); diff != "" {
		t.Errorf("backlinks[alpha] mismatch (-want +got):\n%s", diff)
	}
}
