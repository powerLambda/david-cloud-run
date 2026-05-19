package caldav2ics

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-webdav"
	"github.com/emersion/go-webdav/caldav"
	"github.com/powerLambda/david-cloud-run/internal/config"
)

type CalDAVClient struct {
	client        *caldav.Client
	httpClient    webdav.HTTPClient
	baseURL       string
	principalPath string
	calendarHome  string
	calendarPath  string
	debug         bool
}

type calendarCandidate struct {
	Path string
	Name string
}

type userAgentHTTPClient struct {
	c         webdav.HTTPClient
	userAgent string
}

func (c *userAgentHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	return c.c.Do(req)
}

func NewClient(cfg config.Config) (*CalDAVClient, error) {
	clientTimeout := cfg.Timeout + (5 * time.Second)
	httpClient := &http.Client{Timeout: clientTimeout}
	authClient := webdav.HTTPClientWithBasicAuth(httpClient, cfg.CaldavUsername, cfg.CaldavPassword)
	// Sanitize server responses (some servers return unquoted/getetag values).
	sanitClient := &sanitizeETagHTTPClient{c: authClient}
	uaClient := &userAgentHTTPClient{c: sanitClient, userAgent: "caldav2ics/1.0"}

	caldavClient, err := caldav.NewClient(uaClient, cfg.CaldavURL)
	if err != nil {
		return nil, err
	}

	principalPath, err := normalizePath(cfg.CaldavURL, cfg.CaldavPrincipalURL)
	if err != nil {
		return nil, err
	}
	calendarHome, err := normalizePath(cfg.CaldavURL, cfg.CaldavCalendarHome)
	if err != nil {
		return nil, err
	}
	calendarPath, err := normalizePath(cfg.CaldavURL, cfg.CaldavCalendarURL)
	if err != nil {
		return nil, err
	}

	return &CalDAVClient{
		client:        caldavClient,
		httpClient:    uaClient,
		baseURL:       cfg.CaldavURL,
		principalPath: principalPath,
		calendarHome:  calendarHome,
		calendarPath:  calendarPath,
		debug:         cfg.Debug,
	}, nil
}

// sanitizeETagHTTPClient fixes malformed <getetag> contents in XML multistatus
// responses by ensuring the inner value is quoted. It operates on XML bodies
// and updates Content-Length accordingly.
type sanitizeETagHTTPClient struct {
	c webdav.HTTPClient
}

func (s *sanitizeETagHTTPClient) Do(req *http.Request) (*http.Response, error) {
	// Perform the underlying request first, then inspect and possibly
	// sanitize the response body. We only modify XML multistatus or REPORT
	// responses and we make a best-effort fix to `<getetag>` contents so the
	// downstream `go-webdav` libraries can parse ETags correctly.
	resp, err := s.c.Do(req)
	if err != nil {
		return nil, err
	}

	ct := resp.Header.Get("Content-Type")
	// Read body first (we'll restore it regardless).
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(resp.Body)
	resp.Body.Close()
	if err != nil {
		return resp, nil
	}
	body := buf.String()

	// Only sanitize for REPORT responses or 207 Multi-Status XML responses.
	if !(req.Method == "REPORT" || resp.StatusCode == http.StatusMultiStatus) || (!strings.Contains(ct, "xml") && !strings.Contains(ct, "application/xml")) {
		// restore original body
		resp.Body = io.NopCloser(strings.NewReader(body))
		return resp, nil
	}

	// Quote/normalize getetag values: replace inner value with normalized quote form.
	re := regexp.MustCompile(`(?i)<(?:[a-z0-9]+:)?getetag>([^<]*)</(?:[a-z0-9]+:)?getetag>`)
	fixed := re.ReplaceAllStringFunc(body, func(m string) string {
		sub := re.FindStringSubmatch(m)
		if len(sub) < 2 {
			return m
		}
		v := sub[1]
		nv := normalizeETag(v)
		return strings.Replace(m, sub[1], nv, 1)
	})

	// Replace body and update Content-Length
	resp.Body = io.NopCloser(strings.NewReader(fixed))
	resp.ContentLength = int64(len(fixed))
	resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(fixed)))
	return resp, nil
}

// normalizeETag ensures an ETag string is properly quoted. Handles weak ETags (W/...).
func normalizeETag(raw string) string {
	v := strings.TrimSpace(raw)
	if v == "" {
		return v
	}
	// Handle weak ETag prefix
	if strings.HasPrefix(strings.ToUpper(v), "W/") {
		inner := strings.TrimSpace(v[2:])
		inner = strings.Trim(inner, "\"")
		return "W/\"" + inner + "\""
	}
	// Already quoted?
	if strings.HasPrefix(v, "\"") && strings.HasSuffix(v, "\"") {
		return v
	}
	// Otherwise quote the value
	return "\"" + strings.Trim(v, "\"") + "\""
}

func (c *CalDAVClient) debugf(format string, args ...any) {
	if c.debug {
		log.Printf(format, args...)
	}
}

func (c *CalDAVClient) FetchCalendarData(ctx context.Context) ([]string, error) {
	calendars, err := c.resolveCalendars(ctx)
	if err != nil {
		return nil, err
	}
	if len(calendars) == 0 {
		return nil, nil
	}

	var lastErr error
	for _, calendar := range calendars {
		name := strings.TrimSpace(calendar.Name)
		if name == "" {
			name = "(no displayname)"
		}
		c.debugf("caldav: trying calendar name=%s path=%s", name, calendar.Path)
		results, err := c.queryCalendar(ctx, calendar.Path)
		if err != nil {
			lastErr = err
			c.debugf("caldav: calendar query failed name=%s path=%s err=%v", name, calendar.Path, err)
			continue
		}
		if len(results) > 0 {
			return results, nil
		}
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, nil
}

// resolveCalendars discovers which calendar collections to query.
//
// Steps:
//  1. If `calendarPath` override is provided in config, return it immediately.
//  2. Otherwise discover the current user's principal with `FindCurrentUserPrincipal`.
//  3. Discover the calendar home set with `FindCalendarHomeSet` (unless overridden).
//  4. List calendars under the calendar home with `FindCalendars` and return
//     a list of candidate calendar collection paths and display names.
//
// This method intentionally avoids network retries or complex fallbacks; callers
// handle trying multiple candidates.
func (c *CalDAVClient) resolveCalendars(ctx context.Context) ([]calendarCandidate, error) {
	if c.calendarPath != "" {
		c.debugf("caldav: using calendar path override: %s", c.calendarPath)
		return []calendarCandidate{{Path: c.calendarPath, Name: "override"}}, nil
	}

	principalPath := c.principalPath
	if principalPath == "" {
		found, err := c.client.FindCurrentUserPrincipal(ctx)
		if err != nil {
			return nil, err
		}
		principalPath = found
		c.debugf("caldav: discovered principal path: %s", principalPath)
	} else {
		c.debugf("caldav: using principal path override: %s", principalPath)
	}

	calendarHome := c.calendarHome
	if calendarHome == "" {
		found, err := c.client.FindCalendarHomeSet(ctx, principalPath)
		if err != nil {
			return nil, err
		}
		calendarHome = found
		c.debugf("caldav: discovered calendar home set: %s", calendarHome)
	} else {
		c.debugf("caldav: using calendar home override: %s", calendarHome)
	}

	calendars, err := c.client.FindCalendars(ctx, calendarHome)
	if err != nil {
		return nil, err
	}

	var candidates []calendarCandidate
	for _, calendar := range calendars {
		name := strings.TrimSpace(calendar.Name)
		if name == "" {
			name = "(no displayname)"
		}
		c.debugf("caldav: calendar collection displayname=%s path=%s", name, calendar.Path)
		candidates = append(candidates, calendarCandidate{Path: calendar.Path, Name: name})
	}
	if len(candidates) == 0 {
		return nil, errors.New("caldav: no calendar collection found")
	}
	return candidates, nil
}

// queryCalendar collects VEVENT resources for a single calendar collection.
//
// Logic:
//  1. Issue a CalDAV REPORT (calendar-query) to list resource HREFs in the
//     collection (we intentionally do not rely on server-side calendar-data
//     being present in the REPORT responses).
//  2. Normalize returned HREFs to paths and call `MultiGetCalendar` to fetch
//     full calendar object data for those paths. This handles servers that
//     return href/getetag but not calendar-data in REPORT responses.
//  3. Encode the returned calendar objects into ICS strings and return them.
//
// The function returns an empty slice (nil) when no events are found.
func (c *CalDAVClient) queryCalendar(ctx context.Context, calendarPath string) ([]string, error) {
	// Use library's QueryCalendar first (REPORT via library). Some servers
	// may omit calendar-data in the REPORT response; when that happens the
	// returned CalendarObjects will have nil Data. We detect those cases and
	// fetch missing objects with MultiGetCalendar, leveraging the library's
	// implementations instead of manual HTTP parsing.

	q := &caldav.CalendarQuery{
		CompRequest: caldav.CalendarCompRequest{},
		CompFilter: caldav.CompFilter{
			Name:  "VCALENDAR",
			Comps: []caldav.CompFilter{{Name: "VEVENT"}},
		},
	}

	c.debugf("caldav: queryCalendar QUERY calendar=%s", calendarPath)
	objs, err := c.client.QueryCalendar(ctx, calendarPath, q)
	if err != nil {
		// Some CalDAV servers return a 404 for the calendar-data prop in
		// the REPORT response. In that case, fall back to issuing a raw
		// REPORT to collect hrefs and then call MultiGetCalendar.
		if isCalendarDataNotFound(err) {
			c.debugf("caldav: QueryCalendar returned calendar-data 404, falling back to REPORT+MultiGet")
			hrefs, rerr := c.fetchHrefsFromReport(ctx, calendarPath)
			if rerr != nil {
				return nil, rerr
			}
			if len(hrefs) == 0 {
				return nil, nil
			}
			paths := make([]string, 0, len(hrefs))
			for _, href := range hrefs {
				path, perr := normalizePath(c.baseURL, href)
				if perr != nil {
					continue
				}
				paths = append(paths, path)
			}
			if len(paths) == 0 {
				return nil, nil
			}
			mg := &caldav.CalendarMultiGet{Paths: paths, CompRequest: caldav.CalendarCompRequest{}}
			c.debugf("caldav: calendar-multiget paths=%d (from report fallback)", len(paths))
			objects, merr := c.client.MultiGetCalendar(ctx, calendarPath, mg)
			if merr != nil {
				return nil, merr
			}
			data, _, merr := encodeCalendarObjects(objects)
			return data, merr
		}
		return nil, err
	}

	// Collect paths that need full data via MultiGet.
	var need []string
	var have []caldav.CalendarObject
	for _, o := range objs {
		if o.Data == nil {
			if o.Path != "" {
				need = append(need, o.Path)
			}
			continue
		}
		have = append(have, o)
	}

	// If some objects are missing data, retrieve them with MultiGet.
	if len(need) > 0 {
		c.debugf("caldav: calendar-multiget paths=%d (from query)", len(need))
		mg := &caldav.CalendarMultiGet{Paths: need, CompRequest: caldav.CalendarCompRequest{}}
		fetched, merr := c.client.MultiGetCalendar(ctx, calendarPath, mg)
		if merr != nil {
			return nil, merr
		}
		// Merge fetched results with already-complete objects.
		have = append(have, fetched...)
	}

	data, _, merr := encodeCalendarObjects(have)
	return data, merr
}

// encodeCalendarObjects converts caldav.CalendarObject entries into their
// textual ICS representation. It returns the encoded calendar strings and a
// list of object paths that were missing data (object.Data == nil).
func encodeCalendarObjects(objects []caldav.CalendarObject) ([]string, []string, error) {
	var out []string
	var missing []string
	for _, object := range objects {
		if object.Data == nil {
			if object.Path != "" {
				missing = append(missing, object.Path)
			}
			continue
		}
		var buf bytes.Buffer
		if err := ical.NewEncoder(&buf).Encode(object.Data); err != nil {
			return nil, nil, err
		}
		out = append(out, buf.String())
	}
	return out, missing, nil
}

// normalizePath resolves a possibly-relative or absolute href into a path
// suitable for use with the CalDAV client. If `raw` is an absolute URL,
// the URL's Path component is returned. If `raw` is a relative reference,
// it is resolved against `baseURL`.
func normalizePath(baseURL, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		parsed, err := url.Parse(raw)
		if err != nil {
			return "", err
		}
		return parsed.Path, nil
	}
	if strings.HasPrefix(raw, "/") {
		return raw, nil
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	ref, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	resolved := base.ResolveReference(ref)
	if resolved.Path == "" {
		return "", fmt.Errorf("caldav: invalid path override: %s", raw)
	}
	return resolved.Path, nil
}

// Below we keep a minimal, targeted REPORT fallback for servers that
// return a property-level 404 for <calendar-data> in REPORT responses.
// The primary flow uses the library's QueryCalendar API; when that API
// fails with a calendar-data 404 we issue a lightweight REPORT that
// requests only DAV:getetag (to collect HREFs) and then call
// MultiGetCalendar for the collected paths.

type multistatus struct {
	Responses []struct {
		Href string `xml:"href"`
	} `xml:"response"`
}

// fetchHrefsFromReport issues a minimal calendar-query REPORT that asks
// for DAV:getetag only, and returns the list of hrefs from the multistatus
// response. The request is intentionally small to avoid depending on
// server-side calendar-data expansion.
func (c *CalDAVClient) fetchHrefsFromReport(ctx context.Context, calendarPath string) ([]string, error) {
	// Build a minimal calendar-query REPORT body requesting only getetag.
	reportBody := `<?xml version="1.0" encoding="utf-8"?>
<c:calendar-query xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav">
  <d:prop>
	<d:getetag/>
  </d:prop>
  <c:filter>
	<c:comp-filter name="VCALENDAR">
	  <c:comp-filter name="VEVENT"/>
	</c:comp-filter>
  </c:filter>
</c:calendar-query>`

	// Construct request against the calendar collection URL.
	// Normalize calendarPath into a URL we can call. If calendarPath is
	// already absolute, use it as-is; otherwise resolve against baseURL.
	target := calendarPath
	if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
		target = strings.TrimRight(c.baseURL, "/") + calendarPath
	}

	req, err := http.NewRequestWithContext(ctx, "REPORT", target, strings.NewReader(reportBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Depth", "1")
	req.Header.Set("Content-Type", "application/xml; charset=utf-8")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMultiStatus && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("caldav: report request returned status %d", resp.StatusCode)
	}

	var ms multistatus
	dec := xml.NewDecoder(resp.Body)
	if err := dec.Decode(&ms); err != nil && err != io.EOF {
		return nil, err
	}

	var hrefs []string
	for _, r := range ms.Responses {
		h := strings.TrimSpace(r.Href)
		if h != "" {
			hrefs = append(hrefs, h)
		}
	}
	return hrefs, nil
}

// isCalendarDataNotFound returns true when an error indicates the server
// reported that the <calendar-data> property was not found (property 404).
func isCalendarDataNotFound(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "calendar-data") && strings.Contains(s, "404")
}
