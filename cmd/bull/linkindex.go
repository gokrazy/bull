package main

import (
	"io/fs"
	"os"
	"slices"
	"sort"

	"github.com/yuin/goldmark/ast"
	"go.abhg.dev/goldmark/wikilink"
)

type idx struct {
	// backlinks maps from page name (e.g. index) to
	// page names that contain a link to that page (e.g. SETTINGS, projects, …).
	backlinks map[string][]string
}

func linkTargets(contentRoot *os.Root, pg *page) ([]string, error) {
	var targets []string

	doc := renderMD(contentRoot, pg.Content)
	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if wl, ok := n.(*wikilink.Node); ok {
			targets = append(targets, string(wl.Target))
		}
		if link, ok := n.(*ast.Link); ok {
			targets = append(targets, string(link.Destination))
		}
		return ast.WalkContinue, nil
	})

	sort.Strings(targets)
	return slices.Compact(targets), nil
}

func index(contentRoot *os.Root) (*idx, error) {
	links := make(map[string][]string)
	err := fs.WalkDir(contentRoot.FS(), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !isMarkdown(path) {
			return nil
		}

		pg, err := read(contentRoot, path)
		if err != nil {
			return err
		}

		targets, err := linkTargets(contentRoot, pg)
		if err != nil {
			return err
		}
		links[pg.PageName] = targets

		return nil
	})
	if err != nil {
		return nil, err
	}
	// Invert the index
	// TODO: do we need to check if the target exists?
	// or is it sufficient that we do not query it because we never render it?
	backlinks := make(map[string][]string)
	for pageName, targets := range links {
		for _, target := range targets {
			backlinks[target] = append(backlinks[target], pageName)
		}
	}
	for pageName, linkers := range backlinks {
		sort.Strings(linkers)
		backlinks[pageName] = linkers
	}
	return &idx{
		backlinks: backlinks,
	}, nil
}
