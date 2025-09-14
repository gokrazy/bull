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
	"strings"
	"time"

	"github.com/gokrazy/bull/internal/assets"
	"github.com/gokrazy/bull/internal/codemirror"
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
	if len(codemirror.BullCodemirror) > 0 {
		return "codemirror"
	}
	return "textarea"
}

func cache(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "public, max-age=604800, immutable")
	w.Header().Set("Expires", time.Now().Add(7*24*time.Hour).Format(http.TimeFormat))
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

	root := fset.String("root",
		"/",
		"under which path should bull serve its handlers? useful for serving under a non-root location, e.g. https://michael.stapelberg.ch/garden/")

	watch := fset.String("watch",
		"",
		"whether pages should watch for updates and reload automatically. one of 'true', 'false' or 'workaround' (default 'true' or 'workaround' if available). the 'workaround' setting picks a random hostname like watchXYZ.localhost to work around the 6 connection limit applied when accessing bull via localhost (HTTP/1), which only works with systemd-resolved")

	if err := fset.Parse(args); err != nil {
		return err
	}

	if *root == "" {
		*root = "/"
	}
	if !strings.HasSuffix(*root, "/") {
		*root += "/"
	}

	if *watch == "" && strings.HasPrefix(*listenAddr, "localhost:") {
		addrs, err := net.LookupHost("watchbull.localhost")
		if err != nil {
			log.Printf("NOTE: Browsers will not allow more than 6 concurrently visible tabs when listening on localhost (HTTP/1) and using -watch=true (default). If this bothers you, front bull with Caddy, Tailscale or similar to use HTTP/2 (which needs HTTPS), install systemd-resolve for -watch=workaround or disable watching pages with -watch=false.")
			*watch = "true"
		} else if len(addrs) > 0 {
			*watch = "workaround"
			log.Printf("(enabling -watch=workaround to work around EventSource connection limit)")
		}
	}

	content, err := os.OpenRoot(*contentDir)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		// Interpret the user starting bull with a certain --content flag to
		// mean that the directory should be created if it does not exist.
		if err := os.MkdirAll(*contentDir, 0755); err != nil {
			return err
		}
		content, err = os.OpenRoot(*contentDir)
		if err != nil {
			return err
		}
	}

	// Best effort check: does not correctly identify whether the content
	// directory and home directory are truly identical, just checks
	// whether their names are the same.
	if filepath.Clean(content.Name()) == filepath.Clean(os.Getenv("HOME")) {
		log.Printf("WARNING: You are running bull in your home directory, which may contain many files. You might want to start bull in a smaller directory of markdown files (or set the --content flag).")
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
		root:            *root,
		watch:           *watch,
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

	urlBullPrefix := bull.URLBullPrefix()
	http.Handle(bull.root+"{page...}", handleError(bull.handleRender))
	http.Handle(urlBullPrefix+"edit/{page...}", handleError(bull.edit))

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
		http.HandleFunc(urlBullPrefix+"gofont/"+basename, func(w http.ResponseWriter, r *http.Request) {
			cache(w)
			http.ServeContent(w, r, basename, zeroModTime, bytes.NewReader(variant.content))
		})
	}
	{
		basename := "bull-codemirror.bundle.js"
		http.HandleFunc(urlBullPrefix+"js/"+basename,
			func(w http.ResponseWriter, r *http.Request) {
				cache(w)
				http.ServeContent(w, r, basename, zeroModTime, bytes.NewReader(codemirror.BullCodemirror))
			})
		var assetsFS fs.FS = assets.FS
		if static != nil {
			assetsFS = static.FS()
		}
		handleStaticFile := http.StripPrefix(urlBullPrefix,
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				cache(w)
				http.FileServerFS(assetsFS).ServeHTTP(w, r)
			}))
		http.Handle(urlBullPrefix+"js/", handleStaticFile)
		http.Handle(urlBullPrefix+"css/", handleStaticFile)
		http.Handle(urlBullPrefix+"opensearch.xml", http.StripPrefix(urlBullPrefix, handleError(bull.opensearch)))
	}
	http.Handle("GET "+urlBullPrefix+"browse", handleError(bull.browse))
	http.Handle("GET "+urlBullPrefix+"buildinfo", handleError(bull.buildinfo))
	http.Handle("GET "+urlBullPrefix+"watch/{page...}", handleError(bull.handleWatch))
	http.Handle("POST "+urlBullPrefix+"save/{page...}", handleError(bull.save))
	http.Handle("POST "+urlBullPrefix+"upload/{page...}", handleError(bull.upload))
	http.Handle("GET "+urlBullPrefix+"suggest", handleError(bull.suggest))
	http.Handle("GET "+urlBullPrefix+"search", handleError(bull.search))
	http.Handle("GET "+urlBullPrefix+"_search", handleError(bull.searchAPI))
	http.Handle("GET "+urlBullPrefix+"rename/{page...}", handleError(bull.rename))
	http.Handle("POST "+urlBullPrefix+"_rename/{page...}", handleError(bull.renameAPI))

	ln, err := net.Listen("tcp", *listenAddr)
	if err != nil {
		return err
	}
	log.Printf("serving content from %q on %s", *contentDir, ln.Addr())
	log.Printf("ready! now open %s", urlForListener(ln))
	return http.Serve(ln, nil)
}
