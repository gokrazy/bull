// Package itasklist renders interactive task lists (like the goldmark
// extension.TaskList, but with interactive tick/un-tick capability).
package itasklist

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

var taskListRegexp = regexp.MustCompile(`^\[([\sxX])\]\s*`)

var TaskListKind = ast.NewNodeKind("itasklist")

type TaskListNode struct {
	ast.BaseInline

	// TODO: identify the tasklist (by line number?) to preserve scroll position
}

func (*TaskListNode) Kind() ast.NodeKind { return TaskListKind }

func (n *TaskListNode) Dump(src []byte, level int) {
	ast.DumpHelper(n, src, level, map[string]string{}, nil)
}

var _ ast.Node = (*TaskListNode)(nil)

var TaskItemKind = ast.NewNodeKind("itaskitem")

type TaskItemNode struct {
	ast.BaseInline

	IsChecked bool
	StartByte int
}

func (*TaskItemNode) Kind() ast.NodeKind { return TaskItemKind }

func (n *TaskItemNode) Dump(src []byte, level int) {
	ast.DumpHelper(n, src, level, map[string]string{
		"IsChecked": fmt.Sprint(n.IsChecked),
		"StartByte": strconv.Itoa(n.StartByte),
	}, nil)
}

var _ ast.Node = (*TaskItemNode)(nil)

type TaskItemParser struct{}

var trigger = []byte{'['}

func (p *TaskItemParser) Trigger() []byte { return trigger }

func lineNumber(source []byte, bytePos int) int {
	if bytePos > len(source) {
		bytePos = len(source)
	}
	// TODO: what about files with \r\n line endings? where do they get normalized?
	return bytes.Count(source[:bytePos], []byte{'\n'}) + 1
}

func (p *TaskItemParser) Parse(parent ast.Node, block text.Reader, _ parser.Context) ast.Node {
	// Given AST structure must be like
	// - List
	//   - ListItem         : parent.Parent
	//     - TextBlock      : parent
	//       (current line)
	if parent.Parent() == nil || parent.Parent().FirstChild() != parent {
		return nil
	}

	if parent.HasChildren() {
		return nil
	}
	if _, ok := parent.Parent().(*ast.ListItem); !ok {
		return nil
	}
	line, seg := block.PeekLine()
	m := taskListRegexp.FindSubmatchIndex(line)
	if m == nil {
		return nil
	}
	value := line[m[2]:m[3]][0]
	block.Advance(m[1])
	checked := value == 'x' || value == 'X'
	return &TaskItemNode{
		IsChecked: checked,
		StartByte: seg.Start,
	}
}

var _ parser.InlineParser = (*TaskItemParser)(nil)

type TaskListRenderer struct {
	URLBullPrefix string
	PageURLPath   string // already escaped with url.URL.EscapedPath
}

func (r *TaskListRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(TaskListKind, r.renderItasklist)
}

func (r *TaskListRenderer) renderItasklist(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		w.WriteString("<form class=\"itasklist\" action=\"" + r.URLBullPrefix + "_itasklist/" + r.PageURLPath + "\" method=\"POST\">\n")
	} else {
		w.WriteString("</form>\n")
	}

	return ast.WalkContinue, nil
}

var _ renderer.NodeRenderer = (*TaskListRenderer)(nil)

type TaskItemRenderer struct{}

func (r *TaskItemRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(TaskItemKind, r.renderItaskitem)
}

func (r *TaskItemRenderer) renderItaskitem(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*TaskItemNode)

	if n.IsChecked {
		w.WriteString(`<input checked="" type="checkbox"`)
	} else {
		w.WriteString(`<input type="checkbox"`)
	}
	line := lineNumber(source, n.StartByte)
	w.WriteString(` data-line="` + strconv.Itoa(line) + `">`)
	return ast.WalkContinue, nil
}

var _ renderer.NodeRenderer = (*TaskItemRenderer)(nil)

type Extender struct {
	URLBullPrefix string
	PageURLPath   string // already escaped with url.URL.EscapedPath
}

func (e *Extender) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(
		parser.WithInlineParsers(
			util.Prioritized(&TaskItemParser{}, 0),
		),
	)
	m.Renderer().AddOptions(
		renderer.WithNodeRenderers(
			util.Prioritized(&TaskItemRenderer{}, 500),
		),
	)
	m.Renderer().AddOptions(
		renderer.WithNodeRenderers(
			util.Prioritized(&TaskListRenderer{
				URLBullPrefix: e.URLBullPrefix,
				PageURLPath:   e.PageURLPath,
			}, 999),
		),
	)
}

var _ goldmark.Extender = (*Extender)(nil)
