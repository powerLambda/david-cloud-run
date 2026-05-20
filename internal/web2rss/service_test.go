package web2rss

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/powerLambda/david-cloud-run/internal/config"
	"gopkg.in/yaml.v3"
)

// Unit tests for web2rss extraction logic. Tests cover:
// - tokenized Feed43-style extraction (`extractItems`)
// - BuildFeedForURL end-to-end flow using httptest servers

func TestExtractItemsSimple(t *testing.T) {
	html := `<h2>Title A</h2><a href="/linkA">read</a><p>Content A</p>`
	// pattern: <h2>{%}</h2>{*}<a href="{%}">{*}</a>{%}<p>{%}</p>
	pattern := `<h2>{%}</h2>{*}<a href="{%}">{*}</a>{*}<p>{%}</p>`
	items, err := extractItems(html, pattern, `{%1}`, `{%2}`, `{%3}`)
	if err != nil {
		t.Fatalf("extract error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	it := items[0]
	if it.Title != "Title A" {
		t.Fatalf("title mismatch: %q", it.Title)
	}
}

func TestBuildFeedForURL_Handler(t *testing.T) {
	// create a test server that returns static HTML
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<h2>Title B</h2><a href="/b">link</a><p>Content B</p>`))
	}))
	defer ts.Close()

	sc := SourceConfig{URL: ts.URL, Pattern: `<h2>{%}</h2>{*}<a href="{%}">{*}</a>{*}<p>{%}</p>`, TitleTemplate: `{%1}`, LinkTemplate: `{%2}`, ContentTemplate: `{%3}`, FeedTitle: "T", FeedLink: ts.URL, FeedDesc: "D"}
	sources := map[string]SourceConfig{ts.URL: sc}
	svc := NewService(config.Config{Timeout: 5 * time.Second}, sources)
	feed, err := svc.BuildFeedForURL(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("BuildFeedForURL error: %v", err)
	}
	if feed == nil || len(feed.Items) == 0 {
		t.Fatalf("expected feed items")
	}
}

func TestBuildFeedForURL_MultipleItems(t *testing.T) {
	// create a test server that returns HTML with multiple items
	html := `<!doctype html><html><body>
	<div class="item"><a href="/l1">link</a><h2>Title 1</h2><div>Content 1</div></div>
	<div class="item"><a href="/l2">link</a><h2>Title 2</h2><div>Content 2</div></div>
	</body></html>`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(html))
	}))
	defer ts.Close()

	// prepare SourcesFile and encode to YAML to simulate sources.yaml
	sc := SourceConfig{URL: ts.URL, Pattern: `<div class="item">{*}<a href="{%}">{*}</a>{*}<h2>{%}</h2>{*}<div>{%}</div>{*}</div>`, TitleTemplate: `{%2}`, LinkTemplate: `{%1}`, ContentTemplate: `{%3}`, FeedTitle: "T", FeedLink: ts.URL, FeedDesc: "D"}
	sf := SourcesFile{Sources: []SourceConfig{sc}}
	var buf strings.Builder
	enc := yaml.NewEncoder(&buf)
	if err := enc.Encode(sf); err != nil {
		t.Fatalf("yaml encode error: %v", err)
	}
	enc.Close()
	sources, err := LoadSources(strings.NewReader(buf.String()))
	if err != nil {
		t.Fatalf("LoadSources error: %v\nYAML:\n%s", err, buf.String())
	}
	svc := NewService(config.Config{Timeout: 5 * time.Second}, sources)
	feed, err := svc.BuildFeedForURL(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("BuildFeedForURL error: %v", err)
	}
	if feed == nil || len(feed.Items) != 2 {
		t.Fatalf("expected 2 feed items, got %v", feed)
	}
	rss, err := feed.ToRss()
	if err != nil {
		t.Fatalf("ToRss error: %v", err)
	}
	if !strings.Contains(rss, "Title 1") || !strings.Contains(rss, "/l1") || !strings.Contains(rss, "Content 1") {
		t.Fatalf("first item not present in rss: %s", rss)
	}
	if !strings.Contains(rss, "Title 2") || !strings.Contains(rss, "/l2") || !strings.Contains(rss, "Content 2") {
		t.Fatalf("second item not present in rss: %s", rss)
	}
	itemBlocks := regexp.MustCompile(`(?s)<item>(.*?)</item>`).FindAllStringSubmatch(rss, -1)
	if len(itemBlocks) != 2 {
		t.Fatalf("expected 2 item blocks, got %d: %s", len(itemBlocks), rss)
	}
	firstPub := regexp.MustCompile(`(?s)<pubDate>([^<]+)</pubDate>`).FindStringSubmatch(itemBlocks[0][1])
	secondPub := regexp.MustCompile(`(?s)<pubDate>([^<]+)</pubDate>`).FindStringSubmatch(itemBlocks[1][1])
	if len(firstPub) < 2 || len(secondPub) < 2 {
		t.Fatalf("missing item pubDate(s): %s", rss)
	}
	first, err := time.Parse(time.RFC1123Z, firstPub[1])
	if err != nil {
		t.Fatalf("parse first pubDate: %v", err)
	}
	second, err := time.Parse(time.RFC1123Z, secondPub[1])
	if err != nil {
		t.Fatalf("parse second pubDate: %v", err)
	}
	if diff := first.Sub(second); diff != time.Hour {
		t.Fatalf("expected pubDates to differ by 1h, got %v", diff)
	}
}
