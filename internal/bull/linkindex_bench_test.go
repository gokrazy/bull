package bull

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"slices"
	"testing"
	"time"
)

var gardenDir = flag.String("garden", "", "path to a content directory for index benchmarks")

func TestIndexPerformance(t *testing.T) {
	if *gardenDir == "" {
		t.Skip("-garden not set")
	}

	content, err := os.OpenRoot(*gardenDir)
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
		contentDir:      *gardenDir,
		contentSettings: cs,
		contentChanged:  make(chan struct{}),
		idxReady:        idxReady,
	}
	b.idx.Store(&idx{links: make(map[string][]string)})
	if err := b.init(); err != nil {
		t.Fatal(err)
	}

	// 1. Full index build
	idx, err := b.index()
	if err != nil {
		t.Fatal(err)
	}
	b.idx.Store(idx)
	t.Logf("full index: pages=%d, links map entries=%d", idx.pages, len(idx.links))

	// Pick the page with the most links for single-page measurements.
	var somePage string
	var someTargets []string
	for p, tgts := range idx.links {
		if len(tgts) > len(someTargets) {
			somePage = p
			someTargets = tgts
		}
	}
	if somePage == "" {
		t.Fatal("no page with links found")
	}

	const N = 100

	// 2. Single updateIndex (clone + invertLinks + store)
	t.Run("updateIndex", func(t *testing.T) {
		b.updateIndex(somePage, someTargets) // warm up
		start := time.Now()
		for range N {
			b.updateIndex(somePage, someTargets)
		}
		t.Logf("%v per call (%d pages)", time.Since(start)/N, len(idx.links))
	})

	// 3. invertLinks alone
	t.Run("invertLinks", func(t *testing.T) {
		invertLinks(idx.links) // warm up
		start := time.Now()
		for range N {
			invertLinks(idx.links)
		}
		t.Logf("%v per call (%d pages)", time.Since(start)/N, len(idx.links))
	})

	// 4. linkTargets (parse markdown + walk AST)
	t.Run("linkTargets", func(t *testing.T) {
		pg, err := b.read(somePage + ".md")
		if err != nil {
			for _, ext := range []string{".md", ".markdown"} {
				pg, err = b.read(somePage + ext)
				if err == nil {
					break
				}
			}
		}
		if err != nil {
			t.Skipf("could not read page %q: %v", somePage, err)
		}
		b.linkTargets(pg) // warm up
		start := time.Now()
		for range N {
			b.linkTargets(pg)
		}
		t.Logf("%v per call (page %q, %d bytes)", time.Since(start)/N, somePage, len(pg.Content))
	})

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	t.Logf("heap in use: %.1f MB", float64(m.HeapInuse)/1024/1024)
}

func BenchmarkUpdateIndex(b *testing.B) {
	if *gardenDir == "" {
		b.Skip("-garden not set")
	}

	content, err := os.OpenRoot(*gardenDir)
	if err != nil {
		b.Fatal(err)
	}
	cs, err := loadContentSettings(content)
	if err != nil {
		b.Fatal(err)
	}
	idxReady := make(chan struct{})
	close(idxReady)
	bull := &bullServer{
		root:            "/",
		content:         content,
		contentDir:      *gardenDir,
		contentSettings: cs,
		contentChanged:  make(chan struct{}),
		idxReady:        idxReady,
	}
	bull.idx.Store(&idx{links: make(map[string][]string)})
	if err := bull.init(); err != nil {
		b.Fatal(err)
	}
	idx, err := bull.index()
	if err != nil {
		b.Fatal(err)
	}
	bull.idx.Store(idx)

	// Pick the page with the most links for deterministic selection.
	var somePage string
	var someTargets []string
	for p, tgts := range idx.links {
		if len(tgts) > len(someTargets) {
			somePage = p
			someTargets = tgts
		}
	}

	b.Run(fmt.Sprintf("pages=%d/clone-only", len(idx.links)), func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			bull.updateIndex(somePage, someTargets)
		}
	})

	b.Run(fmt.Sprintf("pages=%d/patch", len(idx.links)), func(b *testing.B) {
		// Alternate target set: drop the first target, add a synthetic one.
		altTargets := make([]string, len(someTargets))
		copy(altTargets, someTargets)
		if len(altTargets) > 0 {
			altTargets[0] = "zzz-synthetic-target"
		}
		slices.Sort(altTargets)
		targets := [2][]string{someTargets, altTargets}
		b.ReportAllocs()
		i := 0
		for b.Loop() {
			bull.updateIndex(somePage, targets[i%2])
			i++
		}
	})
}

func BenchmarkInvertLinks(b *testing.B) {
	if *gardenDir == "" {
		b.Skip("-garden not set")
	}

	content, err := os.OpenRoot(*gardenDir)
	if err != nil {
		b.Fatal(err)
	}
	cs, err := loadContentSettings(content)
	if err != nil {
		b.Fatal(err)
	}
	idxReady := make(chan struct{})
	close(idxReady)
	bull := &bullServer{
		root:            "/",
		content:         content,
		contentDir:      *gardenDir,
		contentSettings: cs,
		contentChanged:  make(chan struct{}),
		idxReady:        idxReady,
	}
	bull.idx.Store(&idx{links: make(map[string][]string)})
	if err := bull.init(); err != nil {
		b.Fatal(err)
	}
	idx, err := bull.index()
	if err != nil {
		b.Fatal(err)
	}

	b.Run(fmt.Sprintf("pages=%d", len(idx.links)), func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			invertLinks(idx.links)
		}
	})
}
