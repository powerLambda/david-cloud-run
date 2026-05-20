package web2rss

import (
	"context"
	"html"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/powerLambda/david-cloud-run/internal/config"
)

func normalizeForCompare(s string) string {
	s = strings.TrimSpace(s)
	s = html.UnescapeString(s)
	// strip HTML tags
	tagRe := regexp.MustCompile(`<[^>]+>`)
	s = tagRe.ReplaceAllString(s, " ")
	// remove common rsseverything footer marker
	footerRe := regexp.MustCompile(`(?is)--\s*Delivered by.*`)
	s = footerRe.ReplaceAllString(s, " ")
	re := regexp.MustCompile(`\s+`)
	s = re.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	return s
}

func runRSSEverythingCompare(t *testing.T, pageURL, expectedFeedURL string) {
	t.Helper()
	if os.Getenv("WEB2RSS_E2E") != "1" {
		t.Skip("set WEB2RSS_E2E=1 to run")
	}

	// parse expected feed from rsseverything
	// We use `gofeed` to parse the reference RSS so both sides are compared
	// using a consistent parsing/normalization strategy.
	parser := gofeed.NewParser()
	expFeed, err := parser.ParseURL(expectedFeedURL)
	if err != nil {
		t.Fatalf("failed to parse expected feed: %v", err)
	}

	// build service using local sources.yaml in package dir
	f, err := os.Open("sources.yaml")
	if err != nil {
		t.Fatalf("open sources.yaml: %v", err)
	}
	srcs, err := LoadSources(f)
	_ = f.Close()
	if err != nil {
		t.Fatalf("LoadSources: %v", err)
	}

	svc := NewService(config.Config{Timeout: 10 * time.Second}, srcs)

	feed, err := svc.BuildFeedForURL(context.Background(), pageURL)
	if err != nil {
		t.Fatalf("BuildFeedForURL error: %v", err)
	}

	// compare up to first 10 items
	want := 10
	if len(expFeed.Items) < want {
		want = len(expFeed.Items)
	}
	if len(feed.Items) < want {
		t.Fatalf("generated feed has %d items, expected at least %d", len(feed.Items), want)
	}

	for i := 0; i < want; i++ {
		exp := expFeed.Items[i]
		got := feed.Items[i]

		nExpTitle := normalizeForCompare(exp.Title)
		nGotTitle := normalizeForCompare(got.Title)
		if nExpTitle != nGotTitle {
			t.Fatalf("item %d title mismatch:\nexpected: %q\nactual:   %q", i+1, nExpTitle, nGotTitle)
		}
		nExpLink := normalizeForCompare(exp.Link)
		nGotLink := normalizeForCompare(got.Link.Href)
		if nExpLink != nGotLink {
			t.Fatalf("item %d link mismatch:\nexpected: %q\nactual:   %q", i+1, nExpLink, nGotLink)
		}
		nExpDesc := normalizeForCompare(exp.Description)
		nGotDesc := normalizeForCompare(got.Description)
		if nExpDesc != nGotDesc {
			t.Fatalf("item %d description mismatch:\nexpected: %q\nactual:   %q", i+1, nExpDesc, nGotDesc)
		}
	}
}

// TestRSSEverythingCompare_Lancedb is a gated e2e that compares BuildFeedForURL output
// against rsseverything's feed (https://rsseverything.com/feed/1538.xml).
// Set WEB2RSS_E2E=1 to run.
func TestRSSEverythingCompare_Lancedb(t *testing.T) {
	runRSSEverythingCompare(t, "https://www.lancedb.com/blog", "https://rsseverything.com/feed/1538.xml")
}

// TestRSSEverythingCompare_Flink compares the Flink blog output against
// rsseverything's feed (https://rsseverything.com/feed/1542.xml).
// Set WEB2RSS_E2E=1 to run.
func TestRSSEverythingCompare_Flink(t *testing.T) {
	runRSSEverythingCompare(t, "https://flink.apache.org/posts/", "https://rsseverything.com/feed/1542.xml")
}
