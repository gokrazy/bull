package bull

import (
	"bytes"
	"html/template"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/gokrazy/bull"
	"github.com/gokrazy/bull/internal/assets"
)

type bullServer struct {
	customization *Customization
	// content is a directory tree.
	content         *os.Root
	contentDir      string // only for <title>
	contentSettings bull.ContentSettings
	static          *os.Root // static assets (for development)
	idx             *idx
	editor          string
	root            string
}

func (b *bullServer) URLBullPrefix() string {
	return b.root + bullPrefix
}

// Initialize this bull server: ensure embedded templates can be parsed,
// or (if -bull_static is not empty) specified directory contains assets.
func (b *bullServer) init() error {
	if _, err := b.templates(); err != nil {
		return err
	}
	return nil
}

func tmplFromFS(fs fs.FS) (*template.Template, error) {
	return template.New("").Funcs(template.FuncMap{
		"hasPrefix": strings.HasPrefix,
	}).ParseFS(fs, "*.html.tmpl")
}

var staticOnce = sync.OnceValues(func() (*template.Template, error) {
	return tmplFromFS(assets.FS)
})

func (b *bullServer) templates() (*template.Template, error) {
	if b.static != nil {
		return tmplFromFS(b.static.FS())
	}
	return staticOnce()
}

func (b *bullServer) executeTemplate(w http.ResponseWriter, basename string, tmpldata any) error {
	tmpls, err := b.templates()
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := tmpls.ExecuteTemplate(&buf, basename, tmpldata); err != nil {
		return err
	}
	if _, err := io.Copy(w, &buf); err != nil {
		return err
	}
	return nil
}

func urlForListener(ln net.Listener) string {
	host := *ln.Addr().(*net.TCPAddr)
	switch {
	case bytes.Equal(host.IP, net.IPv4zero):
		// TODO: why is there no net.IPv4loopback?
		host.IP = net.ParseIP("127.0.0.1")
	case bytes.Equal(host.IP, net.IPv6zero):
		host.IP = net.IPv6loopback
	}
	return (&url.URL{
		Scheme: "http",
		Host:   host.String(),
	}).String()
}
