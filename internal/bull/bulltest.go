package bull

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/tools/txtar"
)

type bullTestServer struct {
	tmp string
	srv *httptest.Server
}

func newTestServer(t *testing.T) *bullTestServer {
	tmp := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmp, "foo.md"), []byte("hello\nworld\n"), 0o666); err != nil {
		t.Fatal(err)
	}

	content, err := os.OpenRoot(tmp)
	if err != nil {
		t.Fatal(err)
	}

	cs, err := loadContentSettings(content)
	if err != nil {
		t.Fatal(err)
	}

	idxReady := make(chan struct{})
	close(idxReady)
	bull := &bullServer{
		root:            "/",
		content:         content,
		contentDir:      tmp,
		contentSettings: cs,
		editor:          "textarea",
		contentChanged:  make(chan struct{}),
		idxReady:        idxReady,
	}
	bull.idx.Store(&idx{})
	if err := bull.init(); err != nil {
		t.Fatal(err)
	}
	urlBullPrefix := bull.URLBullPrefix()
	mux := http.NewServeMux()
	// TODO: unify handler registration between cmdserve.go and here
	mux.Handle("GET /{page...}", handleError(bull.handleRender))
	mux.Handle("POST "+urlBullPrefix+"save/{page...}", handleError(bull.save))
	mux.Handle("POST "+urlBullPrefix+"upload/{page...}", handleError(bull.upload))
	return &bullTestServer{
		tmp: tmp,
		srv: httptest.NewServer(mux),
	}
}

func (ts *bullTestServer) unpackTxTar(contents []byte) error {
	archive := txtar.Parse(contents)
	for _, f := range archive.Files {
		fn := filepath.Join(ts.tmp, f.Name)
		if err := os.WriteFile(fn, f.Data, 0o666); err != nil {
			return err
		}
	}
	return nil
}
