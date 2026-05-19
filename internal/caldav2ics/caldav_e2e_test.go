package caldav2ics

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/powerLambda/david-cloud-run/internal/config"
)

// TestCalDAV2ICSE2E is an end-to-end integration test that contacts a real
// CalDAV server. It runs only when CALDAV_E2E=1 and requires valid
// CALDAV_USERNAME and CALDAV_PASSWORD environment variables. The test
// asserts the handler returns a syntactically valid ICS calendar document.
func TestCalDAV2ICSE2E(t *testing.T) {
	if os.Getenv("CALDAV_E2E") != "1" {
		t.Skip("set CALDAV_E2E=1 to run")
	}
	if os.Getenv("CALDAV_USERNAME") == "" || os.Getenv("CALDAV_PASSWORD") == "" {
		t.Skip("CALDAV_USERNAME and CALDAV_PASSWORD are required for e2e")
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

	h := NewHandler(cfg, client)
	req := httptest.NewRequest(http.MethodGet, cfg.EndpointPath, nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "BEGIN:VCALENDAR") || !strings.Contains(body, "END:VCALENDAR") {
		t.Fatalf("expected ICS calendar, got: %q", body)
	}
}
