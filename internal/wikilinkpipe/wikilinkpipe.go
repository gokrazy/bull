// Package wikilinkpipe is a goldmark extension that lets authors put a
// literal '|' inside a [[Target|Label]] wikilink that lives inside a GFM
// table cell, by writing '\|' (the standard GFM table-cell pipe escape).
//
// goldmark's GFM table parser splits a row at unescaped '|' before any inline
// parser runs, so a bare [[Target|Label]] inside a cell gets chopped in half.
// '\|' tells the table parser not to split — but the wikilink inline parser
// (go.abhg.dev/goldmark/wikilink) does not honor backslash escapes, so the
// resulting Target keeps a trailing '\' and the label keeps any escaped
// '\|' verbatim. This extension repairs both as a parser AST transform.
//
// This is a known ecosystem-wide problem with no upstream fix: Obsidian,
// Dendron, and other GFM+wikilink consumers all settled on the same '\|'
// escape convention. A wikilink-aware GFM table parser would resolve this
// at the root, but no upstream provides one yet. References:
//
//	https://forum.obsidian.md/t/wikilink-pipe-alias-inside-markdown-tables-parsed-as-table-delimiter-instead-of-link-alias/113141
//	https://github.com/dendronhq/dendron/issues/408
//	https://github.com/yuin/goldmark/issues/164 (escaped-pipe rendering, related)
//	https://github.com/yuin/goldmark/issues/157 (wikilink-syntax extension question, no upstream support)
package wikilinkpipe

import (
	"bytes"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	"go.abhg.dev/goldmark/wikilink"
)

// Extender registers the wikilink pipe-escape repair transformer.
type Extender struct{}

// Extend implements goldmark.Extender.
func (Extender) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(parser.WithASTTransformers(
		util.Prioritized(&transformer{}, 999),
	))
}

type transformer struct{}

// Transform implements parser.ASTTransformer.
func (*transformer) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	source := reader.Source()
	var wikilinks []*wikilink.Node
	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if wl, ok := n.(*wikilink.Node); ok {
			wikilinks = append(wikilinks, wl)
		}
		return ast.WalkContinue, nil
	})
	for _, wl := range wikilinks {
		repair(wl, source)
	}
}

func repair(wl *wikilink.Node, source []byte) {
	// Only strip a trailing '\' from Target/Fragment when the wikilink had
	// an actual '|' separator. The wikilink parser sets the label segment
	// to start AFTER the '|' byte; without a separator, the label segment
	// covers the whole content (Target=Label) and the byte before it is
	// '[' (the second '[' of "[["). Otherwise a target that legitimately
	// ends in '\' (e.g. an unusual page name) would be silently truncated.
	if first, ok := wl.FirstChild().(*ast.Text); ok {
		seg := first.Segment
		hasSeparator := seg.Start > 0 && seg.Start <= len(source) && source[seg.Start-1] == '|'
		if hasSeparator {
			if t := wl.Target; len(t) > 1 && t[len(t)-1] == '\\' {
				wl.Target = t[:len(t)-1]
			}
			if f := wl.Fragment; len(f) > 1 && f[len(f)-1] == '\\' {
				wl.Fragment = f[:len(f)-1]
			}
		}
	}

	// Replace every *ast.Text descendant of the wikilink whose source bytes
	// contain '\|' with an ast.String holding the unescaped bytes. Collect
	// first, mutate after — avoids surprising the walker's iteration.
	var texts []*ast.Text
	ast.Walk(wl, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if t, ok := n.(*ast.Text); ok {
			texts = append(texts, t)
		}
		return ast.WalkContinue, nil
	})
	for _, t := range texts {
		seg := t.Segment
		if seg.Start < 0 || seg.Stop > len(source) || seg.Start > seg.Stop {
			continue
		}
		chunk := source[seg.Start:seg.Stop]
		if !bytes.Contains(chunk, []byte{'\\', '|'}) {
			continue
		}
		label := bytes.ReplaceAll(chunk, []byte{'\\', '|'}, []byte{'|'})
		parent := t.Parent()
		if parent == nil {
			continue
		}
		parent.ReplaceChild(parent, t, ast.NewString(label))
	}
}
