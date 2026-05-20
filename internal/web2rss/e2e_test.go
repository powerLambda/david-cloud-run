package web2rss

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/powerLambda/david-cloud-run/internal/config"
	"gopkg.in/yaml.v3"
)

func TestE2E_Handler_WithSourcesFile(t *testing.T) {
	// content server
	html := `<!doctype html><html><body>
    <div class="item"><a href="/a1">link</a><h2>ETitle 1</h2><div>EContent 1</div></div>
    <div class="item"><a href="/a2">link</a><h2>ETitle 2</h2><div>EContent 2</div></div>
    </body></html>`
	contentSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(html))
	}))
	defer contentSrv.Close()

	// create a temp sources.yaml file
	tmpFile, err := os.CreateTemp("", "sources-*.yaml")
	if err != nil {
		t.Fatalf("tmp file create: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	sc := SourceConfig{
		URL:             contentSrv.URL,
		Pattern:         `<div class="item">{*}<a href="{%}">{*}</a>{*}<h2>{%}</h2>{*}<div>{%}</div>{*}</div>`,
		TitleTemplate:   `{%2}`,
		LinkTemplate:    `{%1}`,
		ContentTemplate: `{%3}`,
		FeedTitle:       "E T",
		FeedLink:        contentSrv.URL,
		FeedDesc:        "E D",
	}
	sf := SourcesFile{Sources: []SourceConfig{sc}}
	enc := yaml.NewEncoder(tmpFile)
	if err := enc.Encode(sf); err != nil {
		t.Fatalf("yaml encode: %v", err)
	}
	enc.Close()
	// ensure file is written
	if err := tmpFile.Sync(); err != nil {
		t.Fatalf("sync: %v", err)
	}
	_ = tmpFile.Close()

	// build service from that file
	f, err := os.Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("open tmp: %v", err)
	}
	sources, err := LoadSources(f)
	_ = f.Close()
	if err != nil {
		t.Fatalf("LoadSources: %v", err)
	}

	svc := NewService(config.Config{Timeout: 5 * time.Second}, sources)
	encodedURL := url.QueryEscape(contentSrv.URL)
	req := httptest.NewRequest(http.MethodGet, "/web2rss?url="+encodedURL, nil)
	req.RequestURI = "/web2rss?url=" + encodedURL
	rec := httptest.NewRecorder()
	NewHandler(config.Config{Timeout: 5 * time.Second}, svc).ServeHTTP(rec, req)
	resp := rec.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status not ok: %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/rss+xml; charset=utf-8" {
		t.Fatalf("unexpected content-type: %q", ct)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	s := string(body)
	if !strings.Contains(s, "ETitle 1") || !strings.Contains(s, "/a1") || !strings.Contains(s, "EContent 1") {
		t.Fatalf("feed missing first item: %s", s)
	}
	if !strings.Contains(s, "ETitle 2") || !strings.Contains(s, "/a2") || !strings.Contains(s, "EContent 2") {
		t.Fatalf("feed missing second item: %s", s)
	}
}
