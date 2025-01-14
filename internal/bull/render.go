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
	"time"

	"github.com/gokrazy/bull/internal/assets"
	"github.com/gokrazy/bull/internal/hashtag"
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
	// TODO: Set up autolinking for bare URLs (like golang.org):
	// https://github.com/yuin/goldmark/blob/master/README.md#linkify-extension
	var parserOpts []parser.Option
	parserOpts = append(parserOpts, parser.WithAutoHeadingID())
	var rendererOpts []renderer.Option
	if b.contentSettings.HardWraps {
		// Turn newlines into <br>.
		rendererOpts = append(rendererOpts, html.WithHardWraps())
	}
	return goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			&wikilink.Extender{
				Resolver: &resolver{
					root:        b.root,
					contentRoot: b.content,
				},
			},
			&hashtag.Extender{
				URLBullPrefix: b.URLBullPrefix(),
			},
		),
		goldmark.WithParserOptions(parserOpts...),
		goldmark.WithRendererOptions(rendererOpts...),
	)
}

func (b *bullServer) renderMD(md string) ast.Node {
	converter := b.converter()
	p := converter.Parser()
	return p.Parse(text.NewReader([]byte(md)))
}

func (b *bullServer) render(md string) string {
	var buf bytes.Buffer

	converter := b.converter()
	if err := converter.Convert([]byte(md), &buf); err != nil {
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

func (b *bullServer) renderBullMarkdown(w http.ResponseWriter, r *http.Request, basename string, buf bytes.Buffer) error {
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

func (b *bullServer) renderMarkdown(w http.ResponseWriter, r *http.Request, pg *page, md []byte) error {
	html := b.render(string(md))
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
	}{
		URLPrefix:     b.root,
		URLBullPrefix: b.URLBullPrefix(),
		RequestPath:   r.URL.EscapedPath(),
		ReadOnly:      b.editor == "",
		Title:         pg.Abs(b.contentDir),
		Page:          pg,
		Content:       template.HTML(html),
		ContentHash:   pg.ContentHash(),
		StaticHash:    b.staticHash,
	})
}
