package caldav2ics

import (
	"strings"
	"testing"
)

// TestBuildICSDedupTimezoneAndEvents verifies that BuildICS merges identical
// VTIMEZONE blocks and preserves multiple VEVENT entries from multiple
// input calendars while adding an X-WR-TIMEZONE header.
func TestBuildICSDedupTimezoneAndEvents(t *testing.T) {
	cal1 := "BEGIN:VCALENDAR\n" +
		"BEGIN:VTIMEZONE\n" +
		"TZID:Asia/Shanghai\n" +
		"END:VTIMEZONE\n" +
		"BEGIN:VEVENT\n" +
		"SUMMARY:One\n" +
		"DTSTART:20250101T000000Z\n" +
		"DTEND:20250101T010000Z\n" +
		"END:VEVENT\n" +
		"END:VCALENDAR\n"

	cal2 := "BEGIN:VCALENDAR\n" +
		"BEGIN:VTIMEZONE\n" +
		"TZID:Asia/Shanghai\n" +
		"END:VTIMEZONE\n" +
		"BEGIN:VEVENT\n" +
		"SUMMARY:Two\n" +
		"DTSTART:20250102T000000Z\n" +
		"DTEND:20250102T010000Z\n" +
		"END:VEVENT\n" +
		"END:VCALENDAR\n"

	out := string(BuildICS("Asia/Shanghai", []string{cal1, cal2}))

	if !strings.Contains(out, "BEGIN:VCALENDAR\r\n") {
		t.Fatalf("expected calendar header, got: %q", out)
	}
	if !strings.Contains(out, "X-WR-TIMEZONE:Asia/Shanghai\r\n") {
		t.Fatalf("expected X-WR-TIMEZONE, got: %q", out)
	}
	if strings.Count(out, "BEGIN:VTIMEZONE") != 1 {
		t.Fatalf("expected one timezone block, got: %d", strings.Count(out, "BEGIN:VTIMEZONE"))
	}
	if strings.Count(out, "BEGIN:VEVENT") != 2 {
		t.Fatalf("expected two events, got: %d", strings.Count(out, "BEGIN:VEVENT"))
	}
	if !strings.Contains(out, "SUMMARY:One\r\n") || !strings.Contains(out, "SUMMARY:Two\r\n") {
		t.Fatalf("expected both events, got: %q", out)
	}
}
