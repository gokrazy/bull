package bull

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

func (b *bullServer) opensearch(w http.ResponseWriter, r *http.Request) error {
	cache(w)
	w.Header().Set("Content-Type", "application/opensearchdescription+xml")
	absoluteContentDir := (&page{
		FileName: "index",
	}).Abs(b.contentDir)
	// TODO: -trusted_proxies flag
	trustedProxy := strings.HasPrefix(r.RemoteAddr, "[::1]:") ||
		strings.HasPrefix(r.RemoteAddr, "127.0.0.1:")

	host := r.Header.Get("X-Forwarded-Host")
	if host == "" || !trustedProxy {
		host = r.Host
	}

	proto := r.Header.Get("X-Forwarded-Proto")
	if proto == "" || !trustedProxy {
		proto = "http"
	}

	return b.executeTextTemplate(w, "opensearch.xml.tmpl", struct {
		URLBullPrefix      string
		Host               string
		Proto              string
		AbsoluteContentDir string
	}{
		URLBullPrefix:      b.URLBullPrefix(),
		Host:               host,
		Proto:              proto,
		AbsoluteContentDir: absoluteContentDir,
	})

}

func (b *bullServer) suggest(w http.ResponseWriter, r *http.Request) error {
	query := r.FormValue("q")
	if query == "" {
		return httpError(http.StatusBadRequest, fmt.Errorf("empty q= parameter not allowed"))
	}
	if len(query) < 2 {
		return httpError(http.StatusBadRequest, fmt.Errorf("minimum query length: 2 characters"))
	}

	ctx := r.Context()
	start := time.Now()
	results, err := b.internalsearch(ctx, query, nil)
	if err != nil {
		return err
	}
	log.Printf("search for query %q done in %v, now streaming results", query, time.Since(start))

	suggestions := make([]string, len(results))
	for idx, result := range results {
		suggestions[idx] = result.PageName
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode([]interface{}{
		query,
		suggestions,
	}); err != nil {
		return fmt.Errorf("encoding response: %v", err)
	}
	io.Copy(w, &buf)
	return nil
}
