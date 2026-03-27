package bull

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func newTestBull(t *testing.T, files map[string]string) *bullServer {
	t.Helper()
	tmp := t.TempDir()
	for name, content := range files {
		dir := filepath.Dir(filepath.Join(tmp, name))
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(tmp, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	content, err := os.OpenRoot(tmp)
	if err != nil {
		t.Fatal(err)
	}
	cs, err := loadContentSettings(content)
	if err != nil {
		t.Fatal(err)
	}
	idxReady := make(chan struct{})
	close(idxReady)
	b := &bullServer{
		root:            "/",
		content:         content,
		contentDir:      tmp,
		contentSettings: cs,
		contentChanged:  make(chan struct{}),
		idxReady:        idxReady,
	}
	b.idx.Store(&idx{links: make(map[string][]string)})
	if err := b.init(); err != nil {
		t.Fatal(err)
	}
	return b
}

func TestDiffSorted(t *testing.T) {
	for _, tt := range []struct {
		name           string
		a, b           []string
		added, removed []string
	}{
		{name: "both empty"},
		{name: "identical", a: []string{"x", "y"}, b: []string{"x", "y"}},
		{name: "disjoint", a: []string{"a", "b"}, b: []string{"c", "d"}, added: []string{"c", "d"}, removed: []string{"a", "b"}},
		{name: "overlapping", a: []string{"a", "b", "c"}, b: []string{"b", "c", "d"}, added: []string{"d"}, removed: []string{"a"}},
		{name: "a empty", b: []string{"x", "y"}, added: []string{"x", "y"}},
		{name: "b empty", a: []string{"x", "y"}, removed: []string{"x", "y"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			added, removed := diffSorted(tt.a, tt.b)
			if diff := cmp.Diff(tt.added, added); diff != "" {
				t.Errorf("added mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.removed, removed); diff != "" {
				t.Errorf("removed mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestRemoveFromSorted(t *testing.T) {
	for _, tt := range []struct {
		name string
		s    []string
		val  string
		want []string
	}{
		{name: "empty slice", val: "x", want: nil},
		{name: "not found", s: []string{"a", "b", "c"}, val: "d", want: []string{"a", "b", "c"}},
		{name: "first", s: []string{"a", "b", "c"}, val: "a", want: []string{"b", "c"}},
		{name: "middle", s: []string{"a", "b", "c"}, val: "b", want: []string{"a", "c"}},
		{name: "last", s: []string{"a", "b", "c"}, val: "c", want: []string{"a", "b"}},
		{name: "single element", s: []string{"x"}, val: "x", want: []string{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := removeFromSorted(tt.s, tt.val)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestInsertIntoSorted(t *testing.T) {
	for _, tt := range []struct {
		name string
		s    []string
		val  string
		want []string
	}{
		{name: "empty slice", val: "x", want: []string{"x"}},
		{name: "already present", s: []string{"a", "b", "c"}, val: "b", want: []string{"a", "b", "c"}},
		{name: "insert at start", s: []string{"b", "c"}, val: "a", want: []string{"a", "b", "c"}},
		{name: "insert in middle", s: []string{"a", "c"}, val: "b", want: []string{"a", "b", "c"}},
		{name: "insert at end", s: []string{"a", "b"}, val: "c", want: []string{"a", "b", "c"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := insertIntoSorted(tt.s, tt.val)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestUpdateIndex(t *testing.T) {
	b := newTestBull(t, map[string]string{
		"alpha.md": "see [[beta]]",
		"beta.md":  "hello",
	})

	// Build initial index.
	idx, err := b.index()
	if err != nil {
		t.Fatal(err)
	}
	b.idx.Store(idx)

	if got := b.idx.Load().backlinks["beta"]; len(got) != 1 || got[0] != "alpha" {
		t.Fatalf("initial backlinks[beta] = %v, want [alpha]", got)
	}

	// Simulate editing alpha to link to gamma instead of beta.
	if err := os.WriteFile(
		filepath.Join(b.contentDir, "alpha.md"),
		[]byte("see [[gamma]]"), 0644); err != nil {
		t.Fatal(err)
	}
	pg, err := b.read("alpha.md")
	if err != nil {
		t.Fatal(err)
	}
	targets, err := b.linkTargets(pg)
	if err != nil {
		t.Fatal(err)
	}
	b.updateIndex(pg.PageName, targets)

	cur := b.idx.Load()
	if got := cur.backlinks["beta"]; len(got) != 0 {
		t.Errorf("after update: backlinks[beta] = %v, want []", got)
	}
	if diff := cmp.Diff([]string{"alpha"}, cur.backlinks["gamma"]); diff != "" {
		t.Errorf("after update: backlinks[gamma] mismatch (-want +got):\n%s", diff)
	}
}

func TestUpdateIndexPageCount(t *testing.T) {
	b := newTestBull(t, map[string]string{
		"one.md": "[[two]]",
		"two.md": "hello",
	})

	idx, err := b.index()
	if err != nil {
		t.Fatal(err)
	}
	b.idx.Store(idx)

	if got, want := b.idx.Load().pages, uint64(2); got != want {
		t.Fatalf("initial pages = %d, want %d", got, want)
	}

	// Add a new page.
	if err := os.WriteFile(
		filepath.Join(b.contentDir, "three.md"),
		[]byte("new page"), 0644); err != nil {
		t.Fatal(err)
	}
	pg, err := b.read("three.md")
	if err != nil {
		t.Fatal(err)
	}
	targets, err := b.linkTargets(pg)
	if err != nil {
		t.Fatal(err)
	}
	b.updateIndex(pg.PageName, targets)

	if got, want := b.idx.Load().pages, uint64(3); got != want {
		t.Errorf("after add: pages = %d, want %d", got, want)
	}

	// Remove it.
	b.removeFromIndex("three")

	if got, want := b.idx.Load().pages, uint64(2); got != want {
		t.Errorf("after remove: pages = %d, want %d", got, want)
	}
}

func TestUpdateIndexRemoveAndReadd(t *testing.T) {
	b := newTestBull(t, map[string]string{
		"a.md": "[[b]]",
		"b.md": "[[a]]",
	})

	idx, err := b.index()
	if err != nil {
		t.Fatal(err)
	}
	b.idx.Store(idx)

	// Remove page a → backlinks for b should clear.
	b.removeFromIndex("a")
	if got := b.idx.Load().backlinks["b"]; len(got) != 0 {
		t.Errorf("after remove a: backlinks[b] = %v, want []", got)
	}

	// Re-add a with same links → backlinks restored.
	b.updateIndex("a", []string{"b"})
	if diff := cmp.Diff([]string{"a"}, b.idx.Load().backlinks["b"]); diff != "" {
		t.Errorf("after re-add a: backlinks[b] mismatch (-want +got):\n%s", diff)
	}
}

func TestApplyIndexBatch(t *testing.T) {
	// Exercise the rename path: remove old page + add new page + update linkers
	// in a single batch, then verify backlinks are consistent.
	b := newTestBull(t, map[string]string{
		"old.md":    "some content [[other]]",
		"linker.md": "see [[old]]",
		"other.md":  "hello",
	})

	idx, err := b.index()
	if err != nil {
		t.Fatal(err)
	}
	b.idx.Store(idx)

	// Verify initial state.
	if diff := cmp.Diff([]string{"linker"}, b.idx.Load().backlinks["old"]); diff != "" {
		t.Fatalf("initial backlinks[old] mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"old"}, b.idx.Load().backlinks["other"]); diff != "" {
		t.Fatalf("initial backlinks[other] mismatch (-want +got):\n%s", diff)
	}

	// Simulate a rename: old → new.
	// The batch removes "old", adds "new" (with same targets), and updates
	// "linker" to point to "new" instead of "old".
	removals := []string{"old"}
	updates := []indexUpdate{
		{"new", []string{"other"}},  // renamed page keeps its links
		{"linker", []string{"new"}}, // linker now points to new name
	}
	b.idxMu.Lock()
	b.applyIndexBatchLocked(removals, updates)
	b.idxMu.Unlock()

	snap := b.idx.Load()

	// "old" should be gone from the index entirely.
	if got := snap.links["old"]; got != nil {
		t.Errorf("links[old] = %v, want nil", got)
	}
	if got := snap.backlinks["old"]; len(got) != 0 {
		t.Errorf("backlinks[old] = %v, want []", got)
	}

	// "new" should have the renamed page's links and a backlink from linker.
	if diff := cmp.Diff([]string{"other"}, snap.links["new"]); diff != "" {
		t.Errorf("links[new] mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"linker"}, snap.backlinks["new"]); diff != "" {
		t.Errorf("backlinks[new] mismatch (-want +got):\n%s", diff)
	}

	// "other" should now have a backlink from "new" (not "old").
	if diff := cmp.Diff([]string{"new"}, snap.backlinks["other"]); diff != "" {
		t.Errorf("backlinks[other] mismatch (-want +got):\n%s", diff)
	}

	// Page count: started with 3, removed 1, added 1 → still 3.
	if got, want := snap.pages, uint64(3); got != want {
		t.Errorf("pages = %d, want %d", got, want)
	}
}

func TestConcurrentIndex(t *testing.T) {
	b := newTestBull(t, map[string]string{
		"a.md": "[[b]] [[c]]",
		"b.md": "[[c]]",
	})
	initial, err := b.index()
	if err != nil {
		t.Fatal(err)
	}
	b.idx.Store(initial)

	var wg sync.WaitGroup

	// Concurrent readers.
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 200 {
				snap := b.idx.Load()
				_ = snap.pages
				_ = snap.links["a"]
				_ = snap.backlinks["c"]
			}
		}()
	}

	// Concurrent writers: updateIndex.
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 100 {
				targets := []string{"b"}
				if i%2 == 0 {
					targets = []string{"b", "c", "d"}
				}
				b.updateIndex("a", targets)
			}
		}()
	}

	// Concurrent writers: removeFromIndex + re-add.
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				b.removeFromIndex("b")
				b.updateIndex("b", []string{"c"})
			}
		}()
	}

	wg.Wait()

	snap := b.idx.Load()
	if got, want := snap.pages, uint64(len(snap.links)); got != want {
		t.Errorf("pages = %d, len(links) = %d", got, want)
	}
}
