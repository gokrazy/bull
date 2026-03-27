package bull

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// graphAnalysis runs the same orphan/broken-link logic as the graph command
// against a pre-built index, returning the structured output.
func graphAnalysis(idx *idx) graphOutput {
	totalLinks := 0
	for _, targets := range idx.links {
		totalLinks += len(targets)
	}

	orphans := make([]string, 0)
	for pageName := range idx.links {
		if pageName == "index" {
			continue
		}
		if len(idx.backlinks[pageName]) == 0 {
			orphans = append(orphans, pageName)
		}
	}

	broken := make([]brokenLink, 0)
	for source, targets := range idx.links {
		for _, target := range targets {
			if containsScheme(target) {
				continue
			}
			if i := indexByte(target, '#'); i >= 0 {
				target = target[:i]
			}
			if target == "" {
				continue
			}
			if _, exists := idx.links[target]; !exists {
				broken = append(broken, brokenLink{Source: source, Target: target})
			}
		}
	}

	pages := make(map[string]graphPage, len(idx.links))
	for pageName, targets := range idx.links {
		outgoing := targets
		if outgoing == nil {
			outgoing = make([]string, 0)
		}
		incoming := idx.backlinks[pageName]
		if incoming == nil {
			incoming = make([]string, 0)
		}
		pages[pageName] = graphPage{
			Outgoing: outgoing,
			Incoming: incoming,
		}
	}

	return graphOutput{
		Pages:       pages,
		Orphans:     orphans,
		BrokenLinks: broken,
		Stats: graphStats{
			TotalPages:      int(idx.pages),
			TotalLinks:      totalLinks,
			OrphanCount:     len(orphans),
			BrokenLinkCount: len(broken),
		},
	}
}

// containsScheme checks for "://" in the target (same as graph command).
func containsScheme(s string) bool {
	for i := 0; i+2 < len(s); i++ {
		if s[i] == ':' && s[i+1] == '/' && s[i+2] == '/' {
			return true
		}
	}
	return false
}

// indexByte returns the index of the first instance of c in s, or -1.
func indexByte(s string, c byte) int {
	for i := range len(s) {
		if s[i] == c {
			return i
		}
	}
	return -1
}

func TestGraphEmptyGarden(t *testing.T) {
	b := newTestBull(t, map[string]string{})

	idx, err := b.index()
	if err != nil {
		t.Fatal(err)
	}

	out := graphAnalysis(idx)

	if out.Stats.TotalPages != 0 {
		t.Errorf("TotalPages = %d, want 0", out.Stats.TotalPages)
	}
	if len(out.Orphans) != 0 {
		t.Errorf("Orphans = %v, want []", out.Orphans)
	}
	if len(out.BrokenLinks) != 0 {
		t.Errorf("BrokenLinks = %v, want []", out.BrokenLinks)
	}
}

func TestGraphOrphans(t *testing.T) {
	b := newTestBull(t, map[string]string{
		"index.md":  "welcome [[linked]]",
		"linked.md": "hello",
		"orphan.md": "nobody links here",
	})

	idx, err := b.index()
	if err != nil {
		t.Fatal(err)
	}

	out := graphAnalysis(idx)

	if diff := cmp.Diff([]string{"orphan"}, out.Orphans); diff != "" {
		t.Errorf("Orphans mismatch (-want +got):\n%s", diff)
	}
	if out.Stats.OrphanCount != 1 {
		t.Errorf("OrphanCount = %d, want 1", out.Stats.OrphanCount)
	}
}

func TestGraphIndexExcludedFromOrphans(t *testing.T) {
	b := newTestBull(t, map[string]string{
		"index.md": "the root page",
	})

	idx, err := b.index()
	if err != nil {
		t.Fatal(err)
	}

	out := graphAnalysis(idx)

	if len(out.Orphans) != 0 {
		t.Errorf("index should not be an orphan, got Orphans = %v", out.Orphans)
	}
}

func TestGraphBrokenLinks(t *testing.T) {
	b := newTestBull(t, map[string]string{
		"index.md":  "see [[missing]] and [[exists]]",
		"exists.md": "hello",
	})

	idx, err := b.index()
	if err != nil {
		t.Fatal(err)
	}

	out := graphAnalysis(idx)

	want := []brokenLink{{Source: "index", Target: "missing"}}
	if diff := cmp.Diff(want, out.BrokenLinks); diff != "" {
		t.Errorf("BrokenLinks mismatch (-want +got):\n%s", diff)
	}
}

func TestGraphExternalLinksNotBroken(t *testing.T) {
	b := newTestBull(t, map[string]string{
		"index.md": "[ext](https://example.com) and [ext2](http://foo.bar)",
	})

	idx, err := b.index()
	if err != nil {
		t.Fatal(err)
	}

	out := graphAnalysis(idx)

	if len(out.BrokenLinks) != 0 {
		t.Errorf("external links should not be broken, got %v", out.BrokenLinks)
	}
}

func TestGraphFragmentLinksStripped(t *testing.T) {
	b := newTestBull(t, map[string]string{
		"index.md": "see [section](other#heading) and [anchor](#local)",
		"other.md": "hello",
	})

	idx, err := b.index()
	if err != nil {
		t.Fatal(err)
	}

	out := graphAnalysis(idx)

	if len(out.BrokenLinks) != 0 {
		t.Errorf("fragment links should not be broken, got %v", out.BrokenLinks)
	}
}

func TestGraphStats(t *testing.T) {
	b := newTestBull(t, map[string]string{
		"index.md": "[[a]] [[b]]",
		"a.md":     "[[b]]",
		"b.md":     "leaf",
	})

	idx, err := b.index()
	if err != nil {
		t.Fatal(err)
	}

	out := graphAnalysis(idx)

	if out.Stats.TotalPages != 3 {
		t.Errorf("TotalPages = %d, want 3", out.Stats.TotalPages)
	}
	if out.Stats.TotalLinks != 3 {
		t.Errorf("TotalLinks = %d, want 3", out.Stats.TotalLinks)
	}
}

func TestGraphJSONNullFreedom(t *testing.T) {
	// Verify that JSON output never contains null for array fields.
	b := newTestBull(t, map[string]string{
		"index.md": "no links here",
	})

	idx, err := b.index()
	if err != nil {
		t.Fatal(err)
	}

	out := graphAnalysis(idx)

	data, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}

	// Parse back into generic structure to check for nulls.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}

	// orphans and broken_links should be [] not null.
	for _, field := range []string{"orphans", "broken_links"} {
		val := string(raw[field])
		if val == "null" {
			t.Errorf("JSON field %q is null, want empty array []", field)
		}
	}

	// Per-page outgoing/incoming should also be [] not null.
	var pages map[string]map[string]json.RawMessage
	if err := json.Unmarshal(raw["pages"], &pages); err != nil {
		t.Fatal(err)
	}
	for pageName, page := range pages {
		for _, field := range []string{"outgoing", "incoming"} {
			val := string(page[field])
			if val == "null" {
				t.Errorf("JSON pages[%q].%s is null, want empty array []", pageName, field)
			}
		}
	}
}
