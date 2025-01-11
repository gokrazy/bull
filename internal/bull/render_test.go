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
