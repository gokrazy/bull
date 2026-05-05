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

subdirectory wiki link: [[best/stuff]]

full URL: https://github.com/gokrazy/bull

full insecure URL: http://localhost:3333/

naked URL: go.dev/cl/1234

URL in parens: (https://example.com/in-parens)

URL with trailing period: https://example.com/trailing.

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
	idxReady := make(chan struct{})
	close(idxReady)
	bull := &bullServer{
		root:            "/",
		content:         content,
		contentDir:      tmp,
		contentSettings: cs,
		idxReady:        idxReady,
	}
	bull.idx.Store(&idx{})
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

		if want := "https://example.com/in-parens"; !targets[want] {
			t.Errorf("GET /: response does not link to %q", want)
		}

		if want := "https://example.com/trailing"; !targets[want] {
			t.Errorf("GET /: response does not link to %q", want)
		}

		if want := "/best/stuff"; !targets[want] {
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

func TestWikilinkLabels(t *testing.T) {
	tests := []struct {
		name        string
		md          string
		wantHTML    string
		mustContain []string
		mustNotHave []string
	}{
		{
			name:     "paragraph",
			md:       "wiki link with label: [[SomePage|the label]]\n",
			wantHTML: `<p>wiki link with label: <a href="/SomePage">the label</a></p>` + "\n",
		},
		{
			// Authors must escape '|' as '\|' inside table cells (the same
			// convention GFM already requires for any literal pipe).
			name: "in_table_single_cell",
			md: "| Projekt |\n" +
				"|---|\n" +
				"| [[DIY/fimo-figuren\\|Figuren aus FIMO]] |\n",
			mustContain: []string{
				`<a href="/DIY/fimo-figuren">Figuren aus FIMO</a>`,
			},
			mustNotHave: []string{
				`[[DIY/fimo-figuren`,
				`Figuren aus FIMO]]`,
			},
		},
		{
			name: "in_table_two_links_one_row",
			md: "| A | B |\n" +
				"|---|---|\n" +
				"| [[Foo\\|F]] | [[Bar\\|B]] |\n",
			mustContain: []string{
				`<a href="/Foo">F</a>`,
				`<a href="/Bar">B</a>`,
			},
			mustNotHave: []string{`[[Foo`, `[[Bar`},
		},
		{
			name: "in_table_with_fragment",
			md: "| col |\n" +
				"|---|\n" +
				"| [[page#frag\\|the label]] |\n",
			// Note: bull's resolver currently drops Fragment (pre-existing
			// limitation in render.go's resolver, unrelated to this fix).
			// What we own here: the cell must not be split, target must be
			// "page", label must be "the label".
			mustContain: []string{
				`<a href="/page">the label</a>`,
			},
			mustNotHave: []string{`[[page`, `the label]]`},
		},
		{
			name: "in_table_embed",
			md: "| col |\n" +
				"|---|\n" +
				"| ![[img\\|alt text]] |\n",
			mustContain: []string{`alt text`},
			mustNotHave: []string{`[[img`, `alt text]]`},
		},
		{
			name: "fenced_code_in_table_unchanged",
			md: "| col |\n" +
				"|---|\n" +
				"| normal cell |\n" +
				"\n" +
				"```\n" +
				"[[Foo|Bar]]\n" +
				"```\n",
			mustContain: []string{`[[Foo|Bar]]`},
		},
		{
			name:        "inline_code_unchanged",
			md:          "Look at `[[Foo|Bar]]` here.\n",
			mustContain: []string{`<code>[[Foo|Bar]]</code>`},
		},
		{
			name: "label_with_escaped_pipe_in_table",
			md: "| col |\n" +
				"|---|\n" +
				"| [[Foo\\|a\\|b]] |\n",
			// Both pipes are escaped: target "Foo", label "a|b".
			mustContain: []string{`<a href="/Foo">a|b</a>`},
		},
		{
			name:     "escaped_pipe_in_paragraph",
			md:       "see [[SomePage\\|the label]]\n",
			wantHTML: `<p>see <a href="/SomePage">the label</a></p>` + "\n",
		},
		{
			// A wikilink whose target legitimately ends in '\' (no '|'
			// separator at all) must keep the trailing backslash — the
			// table-escape strip should not fire when there is no label.
			name: "target_with_trailing_backslash_no_label",
			md:   "look at [[Foo\\]] please\n",
			mustContain: []string{
				`href="/Foo%5C"`,
			},
		},
		{
			// '\\|' = literal backslash, then escaped pipe-as-separator.
			// Target should be "Foo\" (one literal backslash), label "Bar".
			name: "literal_backslash_before_escape_in_table",
			md: "| col |\n" +
				"|---|\n" +
				"| [[Foo\\\\|Bar]] |\n",
			mustContain: []string{
				`href="/Foo%5C"`,
				`>Bar</a>`,
			},
		},
		{
			// Embed target/label are correctly extracted from the
			// table cell. (bull's wikilink renderer currently emits
			// embeds as plain anchors, not <img> — pre-existing.)
			name: "embed_in_table_target_and_label",
			md: "| col |\n" +
				"|---|\n" +
				"| ![[img\\|alt text]] |\n",
			mustContain: []string{
				`href="/img"`,
				`alt text`,
			},
			mustNotHave: []string{`%5C`, `\|`, `[[img`},
		},
		{
			// Characterization: a bare '|' inside a table cell still
			// breaks the wikilink (the GFM table parser splits the cell
			// before any inline parser runs). Authors must use '\|'.
			name: "bare_pipe_in_table_breaks",
			md: "| A | B |\n" +
				"|---|---|\n" +
				"| [[Foo|Bar]] x |\n",
			mustNotHave: []string{`<a href="/Foo">Bar</a>`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmp := t.TempDir()
			if err := os.WriteFile(filepath.Join(tmp, "test.md"), []byte(tt.md), 0644); err != nil {
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
				idxReady:        idxReady,
			}
			bull.idx.Store(&idx{})
			if err := bull.init(); err != nil {
				t.Fatal(err)
			}
			mux := http.NewServeMux()
			mux.Handle("GET /{page...}", handleError(bull.handleRender))
			testsrv := httptest.NewServer(mux)
			req, err := http.NewRequest("GET", testsrv.URL+"/test", nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Accept", "text/markdown")
			resp, err := testsrv.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := resp.StatusCode, http.StatusOK; got != want {
				t.Fatalf("unexpected HTTP status: got %v, want %v", resp.Status, want)
			}
			b, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			got := string(b)
			t.Logf("rendered HTML:\n%s", got)
			if tt.wantHTML != "" {
				if diff := cmp.Diff(tt.wantHTML, got); diff != "" {
					t.Errorf("unexpected Body: diff (-want +got):\n%s", diff)
				}
			}
			for _, want := range tt.mustContain {
				if !strings.Contains(got, want) {
					t.Errorf("response does not contain %q", want)
				}
			}
			for _, unwanted := range tt.mustNotHave {
				if strings.Contains(got, unwanted) {
					t.Errorf("response unexpectedly contains %q", unwanted)
				}
			}
		})
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
	idxReady := make(chan struct{})
	close(idxReady)
	bull := &bullServer{
		root:            "/",
		content:         content,
		contentDir:      tmp,
		contentSettings: cs,
		idxReady:        idxReady,
	}
	bull.idx.Store(&idx{})
	if err := bull.init(); err != nil {
		t.Fatal(err)
	}

	// TODO: refactor the registration between cmdserve.go and here
	mux := http.NewServeMux()
	mux.Handle("GET /{page...}", handleError(bull.handleRender))

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
