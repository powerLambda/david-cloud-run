package web2rss

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"net/url"

	"github.com/gorilla/feeds"
	"github.com/powerLambda/david-cloud-run/internal/config"
	"gopkg.in/yaml.v3"
)

type SourceConfig struct {
	URL             string `yaml:"url"`
	Pattern         string `yaml:"pattern"`
	TitleTemplate   string `yaml:"title_template"`
	LinkTemplate    string `yaml:"link_template"`
	ContentTemplate string `yaml:"content_template"`
	FeedTitle       string `yaml:"feed_title"`
	FeedLink        string `yaml:"feed_link"`
	FeedDesc        string `yaml:"feed_description"`
}

type SourcesFile struct {
	Sources []SourceConfig `yaml:"sources"`
}

type Service struct {
	cfg     config.Config
	sources map[string]SourceConfig
}

func LoadSources(r io.Reader) (map[string]SourceConfig, error) {
	var sf SourcesFile
	dec := yaml.NewDecoder(r)
	if err := dec.Decode(&sf); err != nil {
		return nil, err
	}
	m := map[string]SourceConfig{}
	for _, s := range sf.Sources {
		s.Pattern = strings.TrimSpace(s.Pattern)
		s.TitleTemplate = strings.TrimSpace(s.TitleTemplate)
		s.LinkTemplate = strings.TrimSpace(s.LinkTemplate)
		s.ContentTemplate = strings.TrimSpace(s.ContentTemplate)
		if s.URL == "" || s.Pattern == "" {
			return nil, fmt.Errorf("invalid source: missing url or pattern")
		}
		m[s.URL] = s
	}
	return m, nil
}

// NewService constructs the web2rss service with config and per-URL sources.
// The service holds loaded source configs used by BuildFeedForURL.
func NewService(cfg config.Config, sources map[string]SourceConfig) *Service {
	return &Service{cfg: cfg, sources: sources}
}

// compile pattern into regex; convert Feed43-like markers to capture groups
func compilePattern(p string) (*regexp.Regexp, error) {
	// Build regex by scanning pattern and replacing tokens:
	// {%} -> (.*?) capture group
	// {* } -> .* (non-capturing wildcard)
	var b strings.Builder
	for i := 0; i < len(p); {
		if strings.HasPrefix(p[i:], "{%}") {
			b.WriteString("(?s:(.*?))")
			i += 3
			continue
		}
		if strings.HasPrefix(p[i:], "{*}") {
			b.WriteString("(?s:.*?)")
			i += 3
			continue
		}
		// escape single rune
		r := p[i]
		b.WriteString(regexp.QuoteMeta(string(r)))
		i++
	}
	reStr := b.String()
	re, err := regexp.Compile(reStr)
	if err != nil {
		return nil, err
	}
	return re, nil
}

// extractItems applies pattern regex repeatedly to source and builds items
func extractItems(src string, pattern string, titleT, linkT, contentT string) ([]*feeds.Item, error) {
	re, err := compilePattern(pattern)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	var items []*feeds.Item
	matches := re.FindAllStringSubmatch(src, -1)
	if len(matches) == 0 {
		return items, nil
	}
	for i, m := range matches {
		// m[0] is full match, m[1...] are groups
		fill := func(t string) string {
			out := t
			// replace {%N} with group N
			reTpl := regexp.MustCompile(`\{\%(\d+)\}`)
			out = reTpl.ReplaceAllStringFunc(out, func(s string) string {
				sub := reTpl.FindStringSubmatch(s)
				if len(sub) < 2 {
					return ""
				}
				idx := 0
				fmt.Sscanf(sub[1], "%d", &idx)
				if idx >= 1 && idx < len(m) {
					return strings.TrimSpace(m[idx])
				}
				return ""
			})
			return out
		}

		title := fill(titleT)
		link := fill(linkT)
		content := fill(contentT)
		items = append(items, &feeds.Item{
			Title:       title,
			Link:        &feeds.Link{Href: link},
			Description: content,
			Created:     now.Add(-time.Duration(i) * time.Hour),
		})
	}
	return items, nil
}
func (s *Service) BuildFeedForURL(ctx context.Context, pageURL string) (*feeds.Feed, error) {
	sc, ok := s.sources[pageURL]
	if !ok {
		return nil, errors.New("source not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: s.cfg.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	// 1) Try Feed43-style tokenized extraction
	items, err := extractItems(string(b), sc.Pattern, sc.TitleTemplate, sc.LinkTemplate, sc.ContentTemplate)
	if err != nil {
		return nil, err
	}
	feed := &feeds.Feed{Title: sc.FeedTitle, Link: &feeds.Link{Href: sc.FeedLink}, Description: sc.FeedDesc, Created: time.Now()}
	feed.Items = items
	// normalize relative links to absolute using pageURL as base
	for _, it := range feed.Items {
		if it.Link == nil || it.Link.Href == "" {
			continue
		}
		// resolve relative -> absolute; if already absolute, keep as-is
		resolved := it.Link.Href
		if !strings.HasPrefix(resolved, "http://") && !strings.HasPrefix(resolved, "https://") {
			baseURL, err := url.Parse(pageURL)
			if err == nil {
				if ref, err := url.Parse(it.Link.Href); err == nil {
					resolved = baseURL.ResolveReference(ref).String()
				}
			}
		}
		it.Link.Href = resolved
		// set GUID to the item's absolute URL (strongly recommended for RSS2)
		it.Id = resolved
	}
	return feed, nil
}
