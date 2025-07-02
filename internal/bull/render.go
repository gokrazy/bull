package bull

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/gokrazy/bull/internal/assets"
	"github.com/gokrazy/bull/internal/hashtag"
	"github.com/gokrazy/bull/internal/linkify"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"go.abhg.dev/goldmark/wikilink"
)

type resolver struct {
	root        string
	contentRoot *os.Root
}

func (r *resolver) ResolveWikilink(n *wikilink.Node) (destination []byte, err error) {
	// Wiki links (like [[target page name]]) are always resolved
	// to their corresponding (escaped) URL, regardless of whether
	// the page exists on disk or not.
	//
	// This allows creating pages by linking to them, following the link,
	// then clicking Create page in the top menu bar.
	return append([]byte(r.root), []byte(url.PathEscape(string(n.Target)))...), nil
}

func (b *bullServer) converter() goldmark.Markdown {
	var parserOpts []parser.Option
	parserOpts = append(parserOpts, parser.WithAutoHeadingID())
	var rendererOpts []renderer.Option
	if b.contentSettings.HardWraps {
		// Turn newlines into <br>.
		rendererOpts = append(rendererOpts, html.WithHardWraps())
	}
	// Allow inline HTML e.g. for the page rename form.
	rendererOpts = append(rendererOpts, html.WithUnsafe())
	extensions := []goldmark.Extender{
		// extension.GFM is defined as
		// Linkify, Table, Strikethrough and TaskList
		// We need to pass custom options to Linkify.
		&linkify.Extender{},
		extension.Table,
		extension.Strikethrough,
		extension.TaskList,
		&wikilink.Extender{
			Resolver: &resolver{
				root:        b.root,
				contentRoot: b.content,
			},
		},
		&hashtag.Extender{
			URLBullPrefix: b.URLBullPrefix(),
		},
	}
	if b.customization != nil {
		extensions = append(extensions, b.customization.GoldmarkExtensions...)
	}
	return goldmark.New(
		goldmark.WithExtensions(extensions...),
		goldmark.WithParserOptions(parserOpts...),
		goldmark.WithRendererOptions(rendererOpts...),
	)
}

func (b *bullServer) renderMD(md string) ast.Node {
	converter := b.converter()
	source := []byte(md)
	doc := converter.Parser().Parse(text.NewReader(source))

	// Make the URL protocol default to http:// for naked links
	// like go.dev/cl/641655
	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if al, ok := n.(*ast.AutoLink); ok && al.Protocol == nil {
			u := string(al.URL(source))
			if !strings.HasPrefix(u, "http://") &&
				!strings.HasPrefix(u, "https://") {
				al.Protocol = []byte("http")
			}
		}
		return ast.WalkContinue, nil
	})

	return doc
}

func (b *bullServer) render(md string) string {
	var buf bytes.Buffer

	doc := b.renderMD(md)
	converter := b.converter()
	err := converter.Renderer().Render(&buf, []byte(md), doc)
	if err != nil {
		// TODO: error page template
		return "goldmark.Convert: " + err.Error()
	}
	return buf.String()
}

func (b *bullServer) serveStaticFile(w http.ResponseWriter, r *http.Request) error {
	staticFn := pageFromURL(r)
	f, err := b.content.Open(staticFn)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	if st.IsDir() {
		q := url.Values{
			"dir": []string{staticFn},
		}
		target := (&url.URL{
			Path:     b.URLBullPrefix() + "browse",
			RawQuery: q.Encode(),
		}).String()
		http.Redirect(w, r, target, http.StatusFound)
		return nil
	}
	http.ServeContent(w, r, staticFn, st.ModTime(), f)
	return nil
}

func (b *bullServer) handleRender(w http.ResponseWriter, r *http.Request) error {
	possibilities := filesFromURL(r)
	pg, err := b.readFirst(possibilities)
	switch {
	case err == nil:
		// The requested page exists!
		return b.renderWithBacklinks(w, r, pg)

	case os.IsNotExist(err):
		// The requested page does not exist.
		//
		// Maybe this request is not for a markdown page,
		// but for a file that should be statically served,
		// like an included image?
		err = b.serveStaticFile(w, r)
		if os.IsNotExist(err) {
			// Neither a page nor a static file exists
			// with this name. Render a not found error.
			return b.renderNotFound(w, r)
		}
		return err

	default:
		return err
	}
}

func (b *bullServer) renderWithBacklinks(w http.ResponseWriter, r *http.Request, pg *page) error {
	// Extend the content with backlinks (wb).
	wb := []byte(pg.Content)

	if linkers := b.idx.backlinks[pg.PageName]; len(linkers) > 0 {
		wb = append(wb, []byte(`
# backlinks

`)...)
		for _, linker := range linkers {
			wb = append(wb, []byte(fmt.Sprintf("* [[%s]]\n", file2page(linker)))...)
		}
	}

	return b.renderMarkdown(w, r, pg, wb)
}

func (b *bullServer) renderBullMarkdown(w http.ResponseWriter, r *http.Request, basename string, buf *bytes.Buffer) error {
	pageName := bullPrefix + basename
	pg := &page{
		Exists:   true,
		PageName: pageName,
		FileName: page2desired(pageName),
		Content:  buf.String(),
		ModTime:  time.Now(),
	}
	return b.renderMarkdown(w, r, pg, buf.Bytes())
}

func (b *bullServer) staticHash(path string) string {
	var assetsFS fs.FS = assets.FS
	if b.static != nil {
		assetsFS = b.static.FS()
	}
	f, err := assetsFS.Open(path)
	if err != nil {
		return fmt.Sprintf("BUG: %v", err)
	}
	defer f.Close()
	h := quickhash()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Sprintf("Copy: %v", err)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func insideOutTitle(fn, contentDir string) string {
	contentDir = briefHome(contentDir)
	components := strings.Split(fn, string(os.PathSeparator))
	slices.Reverse(components)
	if len(components) > 0 {
		components[0] = file2page(components[0])
	}
	return strings.Join(components, " ← ") + " ← " + contentDir
}

func (b *bullServer) renderMarkdown(w http.ResponseWriter, r *http.Request, pg *page, md []byte) error {
	html := b.render(string(md))
	if accept := r.Header.Get("Accept"); accept != "" {
		// TODO(go1.25): use net/http content negotiation if available:
		// https://github.com/golang/go/issues/19307
		// (No big deal, we mostly use Accept headers for testing.)
		posMarkdown := strings.Index(accept, "text/markdown")
		posHTML := strings.Index(accept, "text/html")
		if posMarkdown > -1 &&
			(posHTML == -1 || posMarkdown < posHTML) {
			w.Write([]byte(html))
			return nil
		}
	}
	return b.executeTemplate(w, "page.html.tmpl", struct {
		URLPrefix     string
		URLBullPrefix string
		RequestPath   string
		ReadOnly      bool
		Title         string
		Page          *page
		Content       template.HTML
		ContentHash   string
		StaticHash    func(string) string
		Watch         string
	}{
		URLPrefix:     b.root,
		URLBullPrefix: b.URLBullPrefix(),
		RequestPath:   r.URL.EscapedPath(),
		ReadOnly:      b.editor == "",
		Title:         insideOutTitle(pg.FileName, b.contentDir),
		Page:          pg,
		Content:       template.HTML(html),
		ContentHash:   pg.ContentHash(),
		StaticHash:    b.staticHash,
		Watch:         b.watch,
	})
}
