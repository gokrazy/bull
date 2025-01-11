// Package hashtag renders text like #foo as a link to the bull search page.
package hashtag

import (
	"bytes"
	"fmt"
	"net/url"
	"unicode"
	"unicode/utf8"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

var Kind = ast.NewNodeKind("hashtag")

type Node struct {
	ast.BaseInline

	Tag []byte
}

func (*Node) Kind() ast.NodeKind { return Kind }

func (n *Node) Dump(src []byte, level int) {
	ast.DumpHelper(n, src, level, map[string]string{
		"Tag": string(n.Tag),
	}, nil)
}

var _ ast.Node = (*Node)(nil)

type Parser struct{}

var trigger = []byte{'#'}

func (p *Parser) Trigger() []byte { return trigger }

// TODO: consider relaxing the requirements for what hashtags contain
// TODO: how about some quoting syntax to allow even for whitespace?
func endOfHashtag(r rune) bool {
	return !unicode.IsLetter(r) &&
		!unicode.IsDigit(r) &&
		r != '_' && r != '-' && r != '/'
}

func (p *Parser) Parse(_ ast.Node, block text.Reader, _ parser.Context) ast.Node {
	line, seg := block.PeekLine()
	if len(line) == 0 || line[0] != '#' {
		return nil
	}
	stop := 1
	line = line[1:]

	// Hashtag must start with a letter.
	first, size := utf8.DecodeRune(line)
	if !unicode.IsLetter(first) {
		return nil
	}
	stop += size
	line = line[size:]

	if idx := bytes.IndexFunc(line, endOfHashtag); idx > -1 {
		stop += idx
	} else {
		stop += len(line)
	}
	seg = seg.WithStop(seg.Start + stop)

	n := Node{
		Tag: block.Value(seg),
	}
	n.AppendChild(&n, ast.NewTextSegment(seg))
	block.Advance(seg.Len())
	return &n
}

var _ parser.InlineParser = (*Parser)(nil)

type Renderer struct {
	urlBullPrefix string
}

func (r *Renderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(Kind, r.renderHashtag)
}

func (r *Renderer) renderHashtag(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n, ok := node.(*Node)
	if !ok {
		return ast.WalkStop, fmt.Errorf("unexpected node %T, expected *hashtag.Node", node)
	}

	if entering {
		q := url.Values{
			"q": []string{string(n.Tag)},
		}
		searchLink := (&url.URL{
			Path:     r.urlBullPrefix + "search",
			RawQuery: q.Encode(),
		}).String()
		w.WriteString(`<span class="bull_hashtag"><a href="` + searchLink + `">`)
	} else {
		w.WriteString("</a></span>")
	}

	return ast.WalkContinue, nil
}

var _ renderer.NodeRenderer = (*Renderer)(nil)

type Extender struct {
	URLBullPrefix string
}

func (e *Extender) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(
		parser.WithInlineParsers(
			util.Prioritized(&Parser{}, 999),
		),
	)
	m.Renderer().AddOptions(
		renderer.WithNodeRenderers(
			util.Prioritized(&Renderer{
				urlBullPrefix: e.URLBullPrefix,
			}, 999),
		),
	)
}

var _ goldmark.Extender = (*Extender)(nil)
