// Package linkify renders links as URLs.
package linkify

import (
	"bytes"
	"fmt"
	"unicode"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

var Kind = ast.NewNodeKind("linkify")

type Node struct {
	ast.BaseInline

	Link              []byte
	withSchema        bool
	leadingWhitespace bool
}

func (*Node) Kind() ast.NodeKind { return Kind }

func (n *Node) Dump(src []byte, level int) {
	ast.DumpHelper(n, src, level, map[string]string{
		"Link": string(n.Link),
	}, nil)
}

var _ ast.Node = (*Node)(nil)

type Parser struct{}

// trigger is unicode.IsSpace
var trigger = []byte{'\t', '\n', '\v', '\f', '\r', ' ', 0x85, 0xA0}

func (p *Parser) Trigger() []byte { return trigger }

func isURL(word []byte) (any bool, full bool) {
	for _, schema := range [][]byte{[]byte("http://"), []byte("https://")} {
		idx := bytes.Index(word, schema)
		if idx == -1 {
			continue
		}
		if idx == 0 {
			return true, true // easy case: word starts with a schema
		}
		// prevent false positive: word contains a URL, but not at the start
		return false, false
	}
	// Is this word vaguely URL-shaped? Does it contain at least one dot
	// (hostnames need at least one dot), which is not at the very end?
	if idx := bytes.IndexByte(word, '.'); idx > -1 && idx < len(word)-1 {
		// is the dot followed by a well-known TLD?
		afterDot := word[idx+1:]
		wellKnownTLD := bytes.HasPrefix(afterDot, []byte("dev"))
		if !wellKnownTLD {
			return false, false
		}
		// verify left of the dot is only valid hostname characters
		beforeDot := word[:idx]
		for _, c := range beforeDot {
			switch {
			case 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z':
			case '0' <= c && c <= '9':
			case c == '-' || c == '.':
			default:
				return false, false
			}
		}
		return true, false
	}
	return false, false
}

func (p *Parser) Parse(_ ast.Node, block text.Reader, _ parser.Context) ast.Node {
	line, seg := block.PeekLine()
	if len(line) == 0 {
		return nil
	}
	// Trim leading space
	skip := 0
	for skip < len(line) && util.IsSpace(line[skip]) {
		skip++
	}
	line = line[skip:]

	// find the next whitespace character or end of line
	stop := 0
	if idx := bytes.IndexFunc(line, unicode.IsSpace); idx > -1 {
		stop += idx
	} else {
		stop += len(line)
	}
	word := line[:stop]
	url, withSchema := isURL(word)
	if !url {
		return nil
	}

	seg = seg.WithStart(seg.Start + skip)
	seg = seg.WithStop(seg.Start + stop)
	n := Node{
		Link:              block.Value(seg),
		withSchema:        withSchema,
		leadingWhitespace: skip > 0,
	}
	n.AppendChild(&n, ast.NewTextSegment(seg))
	block.Advance(skip + seg.Len())
	return &n
}

var _ parser.InlineParser = (*Parser)(nil)

type Renderer struct{}

func (r *Renderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(Kind, r.renderLinkify)
}

func (r *Renderer) renderLinkify(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n, ok := node.(*Node)
	if !ok {
		return ast.WalkStop, fmt.Errorf("unexpected node %T, expected *linkify.Node", node)
	}

	if entering {
		if n.leadingWhitespace {
			w.WriteString(" ")
		}
		if !n.withSchema {
			w.WriteString(`<a href="http://` + string(n.Link) + `">`)
		} else {
			w.WriteString(`<a href="` + string(n.Link) + `">`)
		}
	} else {
		w.WriteString("</a>")
	}

	return ast.WalkContinue, nil
}

var _ renderer.NodeRenderer = (*Renderer)(nil)

type Extender struct{}

func (e *Extender) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(
		parser.WithInlineParsers(
			util.Prioritized(&Parser{}, 999),
		),
	)
	m.Renderer().AddOptions(
		renderer.WithNodeRenderers(
			util.Prioritized(&Renderer{}, 999),
		),
	)
}

var _ goldmark.Extender = (*Extender)(nil)
