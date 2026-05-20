## Plan: Add web2rss Service

Add a new web2rss module parallel to caldav2ics that reads per-URL pattern configs from a YAML file, fetches the page, applies a Feed43-style item search pattern with capture templates for title/link/content, and returns RSS via gorilla/feeds. Keep CalDAV credentials required at startup (fail if missing), and add tests (unit + gated live e2e) plus README updates.

**Steps**
1. Extend configuration loading to support a web2rss config path (default repo file), while keeping the existing CalDAV credential requirement (service fails startup if missing).
2. Define a YAML schema for web2rss sources: source URL, item search pattern, title/link/content templates referencing capture groups (e.g., {%2}), and feed metadata (title/link/description). Add parsing + validation (pattern contains captures; template indices in range).
3. Implement a pattern compiler that converts the item search pattern into a non-greedy regex, extracts capture groups for each match, and builds items by applying the three templates (title/link/content); include basic normalization (trim + HTML entity unescape). *parallel with step 4*
4. Implement the web2rss handler that validates the url query param, looks up the source config, fetches the page with timeout, applies extraction, and renders RSS via gorilla/feeds with appropriate headers. *parallel with step 3*
5. Add tests under internal/web2rss: pattern/template unit tests with static HTML strings; an optional live e2e test (gated by WEB2RSS_E2E=1) that fetches a real URL and compares against a JSON expected-items file you provide.
6. Update README with the new endpoint, required/optional env vars, and the config file format plus guidance for adding new pages and adjusting patterns.
7. Update go.mod/go.sum for gorilla/feeds (and YAML parser dependency).

**Relevant files**
- main.go — register the new web2rss module alongside caldav2ics
- internal/config/config.go — add web2rss config path and keep CalDAV credential requirement
- internal/web2rss/ — new module, handler, pattern logic, and tests
- internal/caldav2ics — reference for module/handler patterns
- internal/modules/module.go — module registration pattern
- README.md — document new endpoint and configuration
- go.mod — add gorilla/feeds and YAML parser

**Verification**
1. Run unit tests for web2rss pattern/template extraction and handler validation.
2. Run live e2e test with WEB2RSS_E2E=1 and the expected JSON file you provide.
3. Manual curl: GET /web2rss/url?url=... returns valid RSS XML with expected items.

**Decisions**
- Use YAML config in repo for URL-to-pattern mapping and feed metadata.
- Use item templates with numeric placeholders (e.g., titleTemplate: {%2}) to map capture groups in any order.
- Keep relative links as-is (no base resolution) per your preference.
- Keep CalDAV creds required at startup (fail if missing).

**Further Considerations**
1. If you want different fetch headers (User-Agent/cookies), we can extend per-source config later.