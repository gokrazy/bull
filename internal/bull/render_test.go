package bull

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/tools/txtar"
)

var hrefRe = regexp.MustCompile(`href="([^"]+)"`)

func TestRender(t *testing.T) {
	tmp := t.TempDir()
	archive := txtar.Parse([]byte(`
-- test.md --
hello world

This page is #genius and should receive an award!

This line ends in a #endinghashtag

[markdown link](/maybe%20consider%3F)

wiki link: [[maybe consider?]]

full URL: https://github.com/gokrazy/bull

full insecure URL: http://localhost:3333/

naked URL: go.dev/cl/1234

-- maybe consider?.md --
something profound
`))
	for _, f := range archive.Files {
		fn := filepath.Join(tmp, f.Name)
		if err := os.WriteFile(fn, f.Data, 0644); err != nil {
			t.Fatal(err)
		}
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
	}
	if err := bull.init(); err != nil {
		t.Fatal(err)
	}

	// TODO: refactor the registration between bull.go and here
	mux := http.NewServeMux()
	mux.Handle("/{page...}", handleError(bull.handleRender))

	testsrv := httptest.NewServer(mux)
	cl := testsrv.Client()
	{
		resp, err := cl.Get(testsrv.URL + "/")
		if err != nil {
			t.Fatal(err)
		}
		if got, want := resp.StatusCode, http.StatusNotFound; got != want {
			t.Errorf("GET /: unexpected HTTP status: got %v, want %v", resp.Status, want)
		}
	}
	{
		resp, err := cl.Get(testsrv.URL + "/test")
		if err != nil {
			t.Fatal(err)
		}
		if got, want := resp.StatusCode, http.StatusOK; got != want {
			t.Fatalf("GET /: unexpected HTTP status: got %v, want %v", resp.Status, want)
		}
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		if want := "hello world"; !strings.Contains(string(b), want) {
			t.Errorf("GET /: response does not contain %q", want)
		}
		submatches := hrefRe.FindAllStringSubmatch(string(b), -1)
		targets := make(map[string]bool)
		for _, submatch := range submatches {
			if len(submatch) < 2 {
				continue
			}
			targets[submatch[1]] = true
		}

		if want := "/_bull/search?q=%23genius"; !targets[want] {
			t.Errorf("GET /: response does not link to %q", want)
		}

		if want := "/_bull/search?q=%23endinghashtag"; !targets[want] {
			t.Errorf("GET /: response does not link to %q", want)
		}

		if want := "https://github.com/gokrazy/bull"; !targets[want] {
			t.Errorf("GET /: response does not link to %q", want)
		}

		if want := "http://localhost:3333/"; !targets[want] {
			t.Errorf("GET /: response does not link to %q", want)
		}

		if want := "http://go.dev/cl/1234"; !targets[want] {
			t.Errorf("GET /: response does not link to %q", want)
		}

		want := "/maybe%20consider%3F"
		if !targets[want] {
			t.Errorf("GET /: response does not link to %q", want)
		}

		// Verify there are no other variants of this link
		for key := range targets {
			if key == want || !strings.HasPrefix(key, "/maybe") {
				continue
			}
			t.Fatalf("GET /: response links to %q, which does not seem correctly escaped", key)
		}

		// Verify the link works
		resp, err = cl.Get(testsrv.URL + want)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := resp.StatusCode, http.StatusOK; got != want {
			t.Fatalf("GET /: unexpected HTTP status: got %v, want %v", resp.Status, want)
		}
		b, err = io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		if want := "something profound"; !strings.Contains(string(b), want) {
			t.Errorf("GET /: response does not contain %q", want)
		}

	}
}

func TestLinkify(t *testing.T) {
	tmp := t.TempDir()
	archive := txtar.Parse([]byte("\n" +
		"-- test.md --\n" +
		"* e.g. Emacs with TRAMP: `emacs /ssh:scan2drive:/perm/keep/index.md` ([braucht ein /bin in gokrazy](https://github.com/gokrazy/tools/commit/37e2f95c5cfc58554405cc615c5da8e4899b071a))\n" +
		"* relocatable! e.g. serve opaque-preview with `--root=/go.dev/` and `--root=/protobuf.dev/`\n"))
	for _, f := range archive.Files {
		fn := filepath.Join(tmp, f.Name)
		if err := os.WriteFile(fn, f.Data, 0644); err != nil {
			t.Fatal(err)
		}
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
	}
	if err := bull.init(); err != nil {
		t.Fatal(err)
	}

	// TODO: refactor the registration between bull.go and here
	mux := http.NewServeMux()
	mux.Handle("/{page...}", handleError(bull.handleRender))

	testsrv := httptest.NewServer(mux)
	cl := testsrv.Client()
	{
		resp, err := cl.Get(testsrv.URL + "/")
		if err != nil {
			t.Fatal(err)
		}
		if got, want := resp.StatusCode, http.StatusNotFound; got != want {
			t.Errorf("GET /: unexpected HTTP status: got %v, want %v", resp.Status, want)
		}
	}
	{
		req, err := http.NewRequest("GET", testsrv.URL+"/test?format=markdown", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Accept", "text/markdown")
		resp, err := cl.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := resp.StatusCode, http.StatusOK; got != want {
			t.Fatalf("GET /: unexpected HTTP status: got %v, want %v", resp.Status, want)
		}
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}

		want := `<ul>
<li>e.g. Emacs with TRAMP: <code>emacs /ssh:scan2drive:/perm/keep/index.md</code> (<a href="https://github.com/gokrazy/tools/commit/37e2f95c5cfc58554405cc615c5da8e4899b071a">braucht ein /bin in gokrazy</a>)</li>
<li>relocatable! e.g. serve opaque-preview with <code>--root=/go.dev/</code> and <code>--root=/protobuf.dev/</code></li>
</ul>
`
		got := string(b)
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("unexpected Body: diff (-want +got):\n%s", diff)
		}
	}
}
