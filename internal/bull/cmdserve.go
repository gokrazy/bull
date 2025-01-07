package bull

import (
	"bytes"
	"flag"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gokrazy/bull/internal/assets"
	thirdparty "github.com/gokrazy/bull/third_party"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/goregular"
)

const serveUsage = `
serve - serve markdown pages

Example:
  % bull                                # serve the current directory
  % bull --content ~/keep serve         # serve ~/keep
  % bull serve --listen=100.5.23.42:80  # serve on a Tailscale VPN IP
`

func defaultEditor() string {
	if len(thirdparty.BullCodemirror) > 0 {
		return "codemirror"
	}
	return "textarea"
}

func (c *Customization) serve(args []string) error {
	fset := flag.NewFlagSet("serve", flag.ExitOnError)
	fset.Usage = usage(fset, serveUsage)

	listenAddr := fset.String("listen",
		"localhost:3333",
		"[host]:port listen address")

	bullStatic := fset.String("bull_static",
		"",
		"if non-empty, path to the bull static assets directory (useful when developing bull)")

	editor := fset.String("editor",
		defaultEditor(),
		"if empty, editing files is disabled (read-only mode). one of 'textarea' (HTML textarea) or 'codemirror' (CodeMirror JavaScript editor)")

	if err := fset.Parse(args); err != nil {
		return err
	}
	content, err := os.OpenRoot(*contentDir)
	if err != nil {
		return err
	}

	// Best effort check: does not correctly identify whether the content
	// directory and home directory are truly identical, just checks
	// whether their names are the same.
	if filepath.Clean(content.Name()) == filepath.Clean(os.Getenv("HOME")) {
		log.Printf("WARNING: You are running bull in your home directory, which may contain many files. You might want to start bull in a smaller directory of markdown files (or set the -content flag).")
	}

	if _, err := content.Stat("_bull"); err == nil {
		log.Printf("NOTE: your _bull directory in %q will not be served; it will be shadowed by bull-internal handlers", *contentDir)
	}

	cs, err := loadContentSettings(content)
	if err != nil {
		return err
	}

	var static *os.Root
	if *bullStatic != "" {
		var err error
		static, err = os.OpenRoot(*bullStatic)
		if err != nil {
			return err
		}
	}

	bull := &bullServer{
		customization:   c,
		content:         content,
		contentDir:      *contentDir,
		contentSettings: cs,
		static:          static,
		editor:          *editor,
	}
	if err := bull.init(); err != nil {
		return err
	}

	// index for backlinks
	// TODO: index in the background, print how long it took when done
	// TODO: deal with permission denied errors.
	// add a test that ensures bull starts up even when files cannot be read
	start := time.Now()
	log.Printf("indexing all pages (markdown files) in %s (for backlinks)", content.Name())
	idx, err := bull.index()
	if err != nil {
		return err
	}
	bull.idx = idx
	log.Printf("discovered in %.2fs: directories: %d, pages: %d, links: %d", time.Since(start).Seconds(), idx.dirs, idx.pages, len(idx.backlinks))

	// TODO: serve a default favicon if there is none in the content directory

	// TODO: should the program work at non-rooted URLs?
	http.Handle("/{page...}", handleError(bull.handleRender))
	http.Handle(bullURLPrefix+"edit/{page...}", handleError(bull.edit))

	var zeroModTime time.Time
	for _, variant := range []struct {
		name    string
		content []byte
	}{
		{"regular", goregular.TTF},
		{"bold", gobold.TTF},
		{"mono", gomono.TTF},
	} {
		basename := "go" + variant.name + ".ttf"
		http.HandleFunc(bullURLPrefix+"gofont/"+basename, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "public, max-age=604800, immutable")
			w.Header().Set("Expires", time.Now().Add(7*24*time.Hour).Format(http.TimeFormat))
			http.ServeContent(w, r, basename, zeroModTime, bytes.NewReader(variant.content))
		})
	}
	{
		basename := "bull-codemirror.bundle.js"
		http.HandleFunc(bullURLPrefix+"js/"+basename,
			func(w http.ResponseWriter, r *http.Request) {
				// TODO: set cache headers and include cache buster in html.tmpl
				http.ServeContent(w, r, basename, zeroModTime, bytes.NewReader(thirdparty.BullCodemirror))
			})
		var assetsFS fs.FS = assets.FS
		if static != nil {
			assetsFS = static.FS()
		}
		http.Handle(bullURLPrefix+"js/", http.StripPrefix(bullURLPrefix, http.FileServerFS(assetsFS)))
	}
	http.Handle("GET "+bullURLPrefix+"browse", handleError(bull.browse))
	http.Handle("GET "+bullURLPrefix+"buildinfo", handleError(bull.buildinfo))
	http.Handle("GET "+bullURLPrefix+"watch/{page...}", handleError(bull.watch))
	http.Handle("POST "+bullURLPrefix+"save/{page...}", handleError(bull.save))
	http.Handle("GET "+bullURLPrefix+"search", handleError(bull.search))
	http.Handle("GET "+bullURLPrefix+"_search", handleError(bull.searchAPI))

	ln, err := net.Listen("tcp", *listenAddr)
	if err != nil {
		return err
	}
	log.Printf("serving content from %q on %s", *contentDir, ln.Addr())
	log.Printf("ready! now open %s", urlForListener(ln))
	return http.Serve(ln, nil)
}
