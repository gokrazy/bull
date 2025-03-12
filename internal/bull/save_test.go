package bull

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestSave(t *testing.T) {
	tmp := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmp, "foo.md"), []byte("hello\nworld\n"), 0644); err != nil {
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

	bull := &bullServer{
		root:            "/",
		content:         content,
		contentDir:      tmp,
		contentSettings: cs,
		idx:             &idx{},
		editor:          "textarea",
	}
	if err := bull.init(); err != nil {
		t.Fatal(err)
	}
	// TODO: refactor the registration between cmdserve.go and here
	urlBullPrefix := bull.URLBullPrefix()
	mux := http.NewServeMux()
	mux.Handle("GET /{page...}", handleError(bull.handleRender))
	mux.Handle("POST "+urlBullPrefix+"save/{page...}", handleError(bull.save))

	testsrv := httptest.NewServer(mux)
	cl := testsrv.Client()

	// Ensure we use \n line endings, see issue #15.
	{
		form := url.Values{}
		form.Set("markdown", "hello\r\nworld\r\nanother line\r\n")
		req, err := http.NewRequest("POST", testsrv.URL+"/_bull/save/foo", strings.NewReader(form.Encode()))
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		if err != nil {
			t.Fatal(err)
		}
		resp, err := cl.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := resp.StatusCode, http.StatusOK; got != want {
			t.Errorf("POST /_bull/save/foo: unexpected HTTP status: got %v, want %v", resp.Status, want)
		}
	}
	got, err := os.ReadFile(filepath.Join(tmp, "foo.md"))
	if err != nil {
		t.Fatal(err)
	}
	want := []byte("hello\nworld\nanother line\n")
	if !bytes.Equal(got, want) {
		t.Errorf("unexpected content after saving /foo: diff (-want +got):\n%s", cmp.Diff(want, got))
	}
}
