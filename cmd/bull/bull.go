package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"runtime/debug"
	"time"

	thirdparty "github.com/gokrazy/bull/third_party"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/goregular"
)

const (
	bullPrefix    = "_bull/"
	bullURLPrefix = "/_bull/"
)

func defaultContentDir() string {
	if v := os.Getenv("BULL_CONTENT"); v != "" {
		return v
	}
	dir, _ := os.Getwd()
	// If Getwd failed, return the empty string.
	// That still means “current working directory”,
	// we just have no name to display to the user.
	return dir
}

func defaultEditor() string {
	if len(thirdparty.BullCodemirror) > 0 {
		return "codemirror"
	}
	return "textarea"
}

func runbull() error {
	info, ok := debug.ReadBuildInfo()
	mainVersion := info.Main.Version
	if !ok {
		mainVersion = "<runtime/debug.ReadBuildInfo failed>"
	}
	log.Printf("github.com/gokrazy/bull %s", mainVersion)

	listenAddr := flag.String("listen",
		"localhost:3333",
		"[host]:port listen address")

	contentDir := flag.String("content",
		defaultContentDir(),
		"content directory. bull considers each markdown file in this directory a page and will only serve files from this directory")

	bullStatic := flag.String("bull_static",
		"",
		"if non-empty, path to the bull static assets directory (useful when developing bull)")

	editor := flag.String("editor",
		defaultEditor(),
		"if empty, editing files is disabled (read-only mode). one of 'textarea' (HTML textarea) or 'codemirror' (CodeMirror JavaScript editor)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "bull is a minimalistic bullet journaling software.\n")
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  %s                         # serve the current directory\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -content=$HOME/keep     # serve ~/keep\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -listen=100.5.23.42:80  # serve on a Tailscale VPN IP\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "Command-line flags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	content, err := os.OpenRoot(*contentDir)
	if err != nil {
		return err
	}

	if _, err := content.Stat("_bull"); err == nil {
		log.Printf("NOTE: your _bull directory in %q will be shadowed by bull-internal handlers", *contentDir)
	}

	var static *os.Root
	if *bullStatic != "" {
		var err error
		static, err = os.OpenRoot(*bullStatic)
		if err != nil {
			return err
		}
	}

	// index for backlinks
	// TODO: index in the background, print how long it took when done
	// TODO: deal with permission denied errors.
	// add a test that ensures bull starts up even when files cannot be read
	idx, err := index(content)
	if err != nil {
		return err
	}

	bull := &bull{
		content:    content,
		contentDir: *contentDir,
		static:     static,
		idx:        idx,
		editor:     *editor,
	}
	if err := bull.init(); err != nil {
		return err
	}

	// TODO: serve a default favicon if there is none in the content directory

	// TODO: should the program work at non-rooted URLs?
	http.Handle("/{page...}", handleError(bull.render))
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
			http.ServeContent(w, r, basename, zeroModTime, bytes.NewReader(variant.content))
		})
	}
	{
		basename := "bull-codemirror.bundle.js"
		http.HandleFunc(bullURLPrefix+"js/"+basename,
			func(w http.ResponseWriter, r *http.Request) {
				http.ServeContent(w, r, basename, zeroModTime, bytes.NewReader(thirdparty.BullCodemirror))
			})
	}
	http.Handle(bullURLPrefix+"mostrecent", handleError(bull.mostrecent))
	http.Handle(bullURLPrefix+"buildinfo", handleError(bull.buildinfo))
	http.Handle(bullURLPrefix+"watch/{page...}", handleError(bull.watch))
	http.Handle(bullURLPrefix+"save/{page...}", handleError(bull.save))

	ln, err := net.Listen("tcp", *listenAddr)
	if err != nil {
		return err
	}
	log.Printf("serving content from %q on %s", *contentDir, ln.Addr())
	log.Printf("ready! now open %s", urlForListener(ln))
	return http.Serve(ln, nil)
}

func main() {
	if err := runbull(); err != nil {
		log.Fatal(err)
	}
}
