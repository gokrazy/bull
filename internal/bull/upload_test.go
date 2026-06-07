package bull

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestUpload(t *testing.T) {
	const brokenPNG = `not actually a PNG, just multiple words in a trenchcoat`

	testsrv := newTestServer(t)
	cl := testsrv.srv.Client()

	type uploadResp struct {
		Path string `json:"path"`
	}
	upload := func(page, filename string, mtime int64) string {
		u, err := url.Parse(testsrv.srv.URL)
		if err != nil {
			t.Fatal(err)
		}
		u.Path += "/_bull/upload/" + page
		q := u.Query()
		q.Set("filename", filename)
		q.Set("lastmodified", strconv.FormatInt(mtime, 10))
		u.RawQuery = q.Encode()
		resp, err := cl.Post(u.String(), "image/png", bytes.NewReader([]byte(brokenPNG)))
		if err != nil {
			t.Fatal(err)
		}
		if got, want := resp.StatusCode, http.StatusOK; got != want {
			t.Errorf("POST %s: unexpected HTTP status: got %v, want %v", resp.Request.URL, resp.Status, want)
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var jsonr uploadResp
		if err := json.Unmarshal(body, &jsonr); err != nil {
			t.Fatal(err)
		}
		if jsonr.Path == "" {
			t.Fatalf(`invalid upload response: empty "path" in %q`, string(body))
		}
		return jsonr.Path
	}

	{
		mtime := time.Now()
		remotePath := upload("index", "image.png", mtime.Unix())
		got, err := os.ReadFile(filepath.Join(testsrv.tmp, remotePath))
		if err != nil {
			t.Fatalf("reading uploaded file from testserver path: %v", err)
		}
		if !bytes.Equal(got, []byte(brokenPNG)) {
			t.Fatalf("uploaded file does not match: %q != %q", string(got), string(brokenPNG))
		}
		st, err := os.Stat(filepath.Join(testsrv.tmp, remotePath))
		if err != nil {
			t.Fatalf("Stat'ing uploaded file from testserver path: %v", err)
		}
		if got, want := st.ModTime().Truncate(time.Second), mtime.Truncate(time.Second); got != want {
			t.Fatalf("mtime incorrect after upload: got %v, want %v", got, want)
		}
	}

	// verify uploading more files with the same name results in different files
	{
		remotePath := upload("index", "image.png", time.Now().Unix())
		if got, want := remotePath, "image_002.png"; got != want {
			t.Errorf("unexpected filename for second image.png: got %q, want %q", got, want)
		}
	}

	// verify upload works into not-yet-existant directory
	{
		_ = upload("does/not/exist", "image.png", time.Now().Unix())
		got, err := os.ReadFile(filepath.Join(testsrv.tmp, "does/not/image.png"))
		if err != nil {
			t.Fatalf("reading uploaded file from testserver path: %v", err)
		}
		if !bytes.Equal(got, []byte(brokenPNG)) {
			t.Fatalf("uploaded file does not match: %q != %q", string(got), string(brokenPNG))
		}
	}
}
