package main

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
	"strings"
	"time"
)

type CalDAVClient struct {
	httpClient   *http.Client
	baseURL      string
	username     string
	password     string
	principalURL string
	calendarHome string
	calendarURL  string
	debug        bool
}

type calendarCandidate struct {
	URL  string
	Name string
}

const maxMultiGetHrefs = 50

func NewCalDAVClient(cfg Config) *CalDAVClient {
	clientTimeout := cfg.Timeout + (5 * time.Second)
	return &CalDAVClient{
		httpClient:   &http.Client{Timeout: clientTimeout},
		baseURL:      cfg.CaldavURL,
		username:     cfg.CaldavUsername,
		password:     cfg.CaldavPassword,
		principalURL: cfg.CaldavPrincipalURL,
		calendarHome: cfg.CaldavCalendarHome,
		calendarURL:  cfg.CaldavCalendarURL,
		debug:        cfg.Debug,
	}
}

func (c *CalDAVClient) debugf(format string, args ...any) {
	if c.debug {
		log.Printf(format, args...)
	}
}

func (c *CalDAVClient) FetchCalendarData(ctx context.Context, start, end time.Time) ([]string, error) {
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
		c.debugf("caldav: trying calendar name=%s url=%s", name, calendar.URL)
		results, err := c.calendarQuery(ctx, calendar.URL, start, end)
		if err != nil {
			lastErr = err
			c.debugf("caldav: calendar query failed name=%s url=%s err=%v", name, calendar.URL, err)
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

func (c *CalDAVClient) resolveCalendars(ctx context.Context) ([]calendarCandidate, error) {
	if c.calendarURL != "" {
		c.debugf("caldav: using calendar URL override: %s", c.calendarURL)
		return []calendarCandidate{{URL: c.calendarURL, Name: "override"}}, nil
	}

	principalURL := c.principalURL
	if principalURL == "" {
		found, err := c.findCurrentUserPrincipal(ctx, c.baseURL)
		if err != nil {
			return nil, err
		}
		principalURL = found
		c.debugf("caldav: discovered principal URL: %s", principalURL)
	} else {
		c.debugf("caldav: using principal URL override: %s", principalURL)
	}

	calendarHome := c.calendarHome
	if calendarHome == "" {
		found, err := c.findCalendarHomeSet(ctx, principalURL)
		if err != nil {
			return nil, err
		}
		calendarHome = found
		c.debugf("caldav: discovered calendar home set: %s", calendarHome)
	} else {
		c.debugf("caldav: using calendar home override: %s", calendarHome)
	}

	calendars, err := c.findCalendars(ctx, calendarHome)
	if err != nil {
		return nil, err
	}
	return calendars, nil
}

func (c *CalDAVClient) findCurrentUserPrincipal(ctx context.Context, base string) (string, error) {
	body := `<?xml version="1.0" encoding="UTF-8"?>
<d:propfind xmlns:d="DAV:">
  <d:prop>
    <d:current-user-principal/>
    <d:principal-URL/>
  </d:prop>
</d:propfind>`

	data, err := c.propfind(ctx, base, "0", body)
	if err != nil {
		return "", err
	}

	ms, err := parseMultiStatus(data)
	if err != nil {
		return "", err
	}

	for _, resp := range ms.Responses {
		for _, propstat := range resp.Propstats {
			if !isOKStatus(propstat.Status) {
				continue
			}
			if propstat.Prop.CurrentUserPrincipal != nil && propstat.Prop.CurrentUserPrincipal.Href != "" {
				return resolveHref(base, propstat.Prop.CurrentUserPrincipal.Href)
			}
			if propstat.Prop.PrincipalURL != nil && propstat.Prop.PrincipalURL.Href != "" {
				return resolveHref(base, propstat.Prop.PrincipalURL.Href)
			}
		}
	}

	return "", errors.New("caldav: current-user-principal not found")
}

func (c *CalDAVClient) findCalendarHomeSet(ctx context.Context, principal string) (string, error) {
	body := `<?xml version="1.0" encoding="UTF-8"?>
<d:propfind xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav">
  <d:prop>
    <c:calendar-home-set/>
  </d:prop>
</d:propfind>`

	data, err := c.propfind(ctx, principal, "0", body)
	if err != nil {
		return "", err
	}

	ms, err := parseMultiStatus(data)
	if err != nil {
		return "", err
	}

	for _, resp := range ms.Responses {
		for _, propstat := range resp.Propstats {
			if !isOKStatus(propstat.Status) {
				continue
			}
			if propstat.Prop.CalendarHomeSet != nil && propstat.Prop.CalendarHomeSet.Href != "" {
				return resolveHref(principal, propstat.Prop.CalendarHomeSet.Href)
			}
		}
	}

	return "", errors.New("caldav: calendar-home-set not found")
}

func (c *CalDAVClient) findCalendars(ctx context.Context, homeSet string) ([]calendarCandidate, error) {
	body := `<?xml version="1.0" encoding="UTF-8"?>
<d:propfind xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav">
  <d:prop>
    <d:displayname/>
    <d:resourcetype/>
  </d:prop>
</d:propfind>`

	data, err := c.propfind(ctx, homeSet, "1", body)
	if err != nil {
		return nil, err
	}

	ms, err := parseMultiStatus(data)
	if err != nil {
		return nil, err
	}
	c.debugf("caldav: calendar home set responses=%d", len(ms.Responses))

	var calendars []calendarCandidate
	for _, resp := range ms.Responses {
		for _, propstat := range resp.Propstats {
			if !isOKStatus(propstat.Status) {
				continue
			}
			if propstat.Prop.ResourceType.Calendar != nil {
				resolved, err := resolveHref(homeSet, resp.Href)
				if err != nil {
					return nil, err
				}
				name := strings.TrimSpace(propstat.Prop.DisplayName)
				if name == "" {
					name = "(no displayname)"
				}
				c.debugf("caldav: calendar collection displayname=%s href=%s", name, resolved)
				calendars = append(calendars, calendarCandidate{URL: resolved, Name: name})
			}
		}
	}
	if len(calendars) == 0 {
		return nil, errors.New("caldav: no calendar collection found")
	}
	return calendars, nil
}

func (c *CalDAVClient) calendarQuery(ctx context.Context, calendarURL string, start, end time.Time) ([]string, error) {
	startStr := formatTimeUTC(start)
	endStr := formatTimeUTC(end)
	results, hrefs, err := c.calendarQueryWithRange(ctx, calendarURL, startStr, endStr)
	if err != nil {
		return nil, err
	}
	if len(results) > 0 {
		return results, nil
	}
	if len(hrefs) > 0 {
		c.debugf("caldav: no calendar-data; retrying multiget hrefs=%d mode=utc", len(hrefs))
		return c.calendarMultiGet(ctx, calendarURL, hrefs, startStr, endStr)
	}
	return nil, nil
}
func (c *CalDAVClient) calendarQueryWithRange(ctx context.Context, calendarURL, startStr, endStr string) ([]string, []string, error) {
	c.debugf("caldav: querying calendar %s range=%s..%s", calendarURL, startStr, endStr)
	body := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<c:calendar-query xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav">
  <d:prop>
    <d:getetag/>
    <c:calendar-data>
      <c:expand start="%s" end="%s"/>
    </c:calendar-data>
  </d:prop>
  <c:filter>
    <c:comp-filter name="VCALENDAR">
      <c:comp-filter name="VEVENT">
        <c:time-range start="%s" end="%s"/>
      </c:comp-filter>
    </c:comp-filter>
  </c:filter>
</c:calendar-query>`, startStr, endStr, startStr, endStr)

	data, status, err := c.report(ctx, calendarURL, "1", body)
	if err != nil {
		return nil, nil, err
	}
	if status != http.StatusMultiStatus && status != http.StatusOK {
		return nil, nil, fmt.Errorf("caldav: calendar-query failed with status %d", status)
	}

	ms, err := parseMultiStatus(data)
	if err != nil {
		return nil, nil, err
	}
	c.debugf("caldav: calendar-query responses=%d", len(ms.Responses))
	hrefs := collectHrefs(ms.Responses)

	var results []string
	for _, resp := range ms.Responses {
		for _, propstat := range resp.Propstats {
			if !isOKStatus(propstat.Status) {
				continue
			}
			if strings.TrimSpace(propstat.Prop.CalendarData) != "" {
				results = append(results, propstat.Prop.CalendarData)
			}
		}
	}
	c.debugf("caldav: calendar-query returned %d item(s)", len(results))

	return results, hrefs, nil
}

func (c *CalDAVClient) calendarMultiGet(ctx context.Context, calendarURL string, hrefs []string, startStr, endStr string) ([]string, error) {
	hrefs = uniqueHrefs(hrefs)
	if len(hrefs) == 0 {
		return nil, nil
	}
	if len(hrefs) > maxMultiGetHrefs {
		c.debugf("caldav: multiget truncating hrefs from %d to %d", len(hrefs), maxMultiGetHrefs)
		hrefs = hrefs[:maxMultiGetHrefs]
	}

	var builder strings.Builder
	builder.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<c:calendar-multiget xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav">
  <d:prop>
    <d:getetag/>
`)
	builder.WriteString(fmt.Sprintf("    <c:calendar-data>\n      <c:expand start=\"%s\" end=\"%s\"/>\n    </c:calendar-data>\n", startStr, endStr))
	builder.WriteString("  </d:prop>\n")
	for _, href := range hrefs {
		builder.WriteString("  <d:href>")
		builder.WriteString(escapeXMLText(href))
		builder.WriteString("</d:href>\n")
	}
	builder.WriteString("</c:calendar-multiget>")

	data, status, err := c.report(ctx, calendarURL, "0", builder.String())
	if err != nil {
		return nil, err
	}
	if status != http.StatusMultiStatus && status != http.StatusOK {
		return nil, fmt.Errorf("caldav: calendar-multiget failed with status %d", status)
	}

	ms, err := parseMultiStatus(data)
	if err != nil {
		return nil, err
	}
	c.debugf("caldav: calendar-multiget responses=%d", len(ms.Responses))

	var results []string
	for _, resp := range ms.Responses {
		for _, propstat := range resp.Propstats {
			if !isOKStatus(propstat.Status) {
				continue
			}
			if strings.TrimSpace(propstat.Prop.CalendarData) != "" {
				results = append(results, propstat.Prop.CalendarData)
			}
		}
	}
	c.debugf("caldav: calendar-multiget returned %d item(s)", len(results))

	return results, nil
}

func (c *CalDAVClient) propfind(ctx context.Context, targetURL, depth, body string) ([]byte, error) {
	data, status, err := c.doRequest(ctx, "PROPFIND", targetURL, depth, body)
	if err != nil {
		return nil, err
	}
	if status != http.StatusMultiStatus && status != http.StatusOK {
		return nil, fmt.Errorf("caldav: propfind failed with status %d", status)
	}
	return data, nil
}

func (c *CalDAVClient) report(ctx context.Context, targetURL, depth, body string) ([]byte, int, error) {
	return c.doRequest(ctx, "REPORT", targetURL, depth, body)
}

func (c *CalDAVClient) doRequest(ctx context.Context, method, targetURL, depth, body string) ([]byte, int, error) {
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, method, targetURL, bytes.NewBufferString(body))
	if err != nil {
		return nil, 0, err
	}
	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("Depth", depth)
	req.Header.Set("Content-Type", "application/xml; charset=utf-8")
	req.Header.Set("Accept", "application/xml")
	req.Header.Set("User-Agent", "caldav2ics/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.debugf("caldav: %s %s depth=%s error=%v", method, targetURL, depth, err)
		return nil, 0, err
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	c.debugf("caldav: %s %s depth=%s status=%d duration=%s", method, targetURL, depth, resp.StatusCode, time.Since(start))
	if resp.StatusCode >= 400 {
		return nil, resp.StatusCode, fmt.Errorf("caldav: %s", readBodySnippet(payload))
	}
	return payload, resp.StatusCode, nil
}

func parseMultiStatus(data []byte) (multiStatus, error) {
	var ms multiStatus
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.Strict = false
	if err := decoder.Decode(&ms); err != nil {
		return ms, fmt.Errorf("caldav: failed to parse multistatus: %w", err)
	}
	return ms, nil
}

func isOKStatus(status string) bool {
	return strings.Contains(status, " 200 ")
}

func resolveHref(base, href string) (string, error) {
	if href == "" {
		return "", errors.New("caldav: empty href")
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	refURL, err := url.Parse(href)
	if err != nil {
		return "", err
	}
	return baseURL.ResolveReference(refURL).String(), nil
}

func formatTimeUTC(t time.Time) string {
	return t.UTC().Format("20060102T150405Z")
}

func readBodySnippet(data []byte) string {
	const max = 300
	trimmed := strings.TrimSpace(string(data))
	if len(trimmed) > max {
		return trimmed[:max] + "..."
	}
	return trimmed
}

func collectHrefs(responses []response) []string {
	var hrefs []string
	for _, resp := range responses {
		href := strings.TrimSpace(resp.Href)
		if href == "" {
			continue
		}
		if strings.HasSuffix(href, "/") {
			continue
		}
		hrefs = append(hrefs, href)
	}
	return hrefs
}

func uniqueHrefs(hrefs []string) []string {
	seen := make(map[string]struct{}, len(hrefs))
	var out []string
	for _, href := range hrefs {
		if href == "" {
			continue
		}
		if _, ok := seen[href]; ok {
			continue
		}
		seen[href] = struct{}{}
		out = append(out, href)
	}
	return out
}

func escapeXMLText(input string) string {
	if input == "" {
		return ""
	}
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(input))
	return buf.String()
}

type multiStatus struct {
	Responses []response `xml:"response"`
}

type response struct {
	Href      string     `xml:"href"`
	Propstats []propstat `xml:"propstat"`
}

type propstat struct {
	Prop   prop   `xml:"prop"`
	Status string `xml:"status"`
}

type prop struct {
	CurrentUserPrincipal *href        `xml:"current-user-principal"`
	PrincipalURL         *href        `xml:"principal-URL"`
	CalendarHomeSet      *href        `xml:"calendar-home-set"`
	CalendarData         string       `xml:"calendar-data"`
	ResourceType         resourceType `xml:"resourcetype"`
	DisplayName          string       `xml:"displayname"`
}

type href struct {
	Href string `xml:"href"`
}

type resourceType struct {
	Calendar *struct{} `xml:"calendar"`
}
