package bull

import (
	"bytes"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestSave(t *testing.T) {
	testsrv := newTestServer(t)
	cl := testsrv.srv.Client()

	// Ensure we use \n line endings, see issue #15.
	{
		form := url.Values{}
		form.Set("markdown", "hello\r\nworld\r\nanother line\r\n")
		req, err := http.NewRequest("POST", testsrv.srv.URL+"/_bull/save/foo", strings.NewReader(form.Encode()))
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
	got, err := os.ReadFile(filepath.Join(testsrv.tmp, "foo.md"))
	if err != nil {
		t.Fatal(err)
	}
	want := []byte("hello\nworld\nanother line\n")
	if !bytes.Equal(got, want) {
		t.Errorf("unexpected content after saving /foo: diff (-want +got):\n%s", cmp.Diff(want, got))
	}
}
