package main

import (
	"sort"
	"strings"
)

func BuildICS(timezone string, calendarData []string) []byte {
	tzBlocks := map[string]string{}
	var eventBlocks []string

	for _, data := range calendarData {
		foundTZ, foundEvents := extractBlocks(data)
		for key, block := range foundTZ {
			if _, exists := tzBlocks[key]; !exists {
				tzBlocks[key] = block
			}
		}
		eventBlocks = append(eventBlocks, foundEvents...)
	}

	var tzKeys []string
	for key := range tzBlocks {
		tzKeys = append(tzKeys, key)
	}
	sort.Strings(tzKeys)

	var builder strings.Builder
	builder.WriteString("BEGIN:VCALENDAR\r\n")
	builder.WriteString("PRODID:-//david-cloud-run//caldav2ics//EN\r\n")
	builder.WriteString("VERSION:2.0\r\n")
	builder.WriteString("CALSCALE:GREGORIAN\r\n")
	builder.WriteString("X-WR-TIMEZONE:" + timezone + "\r\n")

	for _, key := range tzKeys {
		builder.WriteString(tzBlocks[key])
	}
	for _, block := range eventBlocks {
		builder.WriteString(block)
	}
	builder.WriteString("END:VCALENDAR\r\n")

	return []byte(builder.String())
}

func extractBlocks(data string) (map[string]string, []string) {
	lines := splitLines(data)
	zblocks := map[string]string{}
	var events []string

	var buffer []string
	inEvent := false
	inTimezone := false
	currentTZID := ""

	for _, raw := range lines {
		line := strings.TrimRight(raw, "\r")
		if strings.HasPrefix(line, "BEGIN:VTIMEZONE") {
			inTimezone = true
			currentTZID = ""
			buffer = []string{line}
			continue
		}
		if strings.HasPrefix(line, "BEGIN:VEVENT") {
			inEvent = true
			buffer = []string{line}
			continue
		}

		if inTimezone {
			buffer = append(buffer, line)
			if strings.HasPrefix(line, "TZID:") && currentTZID == "" {
				currentTZID = strings.TrimPrefix(line, "TZID:")
			}
			if strings.HasPrefix(line, "END:VTIMEZONE") {
				block := joinBlock(buffer)
				key := currentTZID
				if key == "" {
					key = block
				}
				zblocks[key] = block
				inTimezone = false
			}
			continue
		}

		if inEvent {
			buffer = append(buffer, line)
			if strings.HasPrefix(line, "END:VEVENT") {
				events = append(events, joinBlock(buffer))
				inEvent = false
			}
			continue
		}
	}

	return zblocks, events
}

func splitLines(data string) []string {
	if data == "" {
		return nil
	}
	return strings.Split(data, "\n")
}

func joinBlock(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\r\n") + "\r\n"
}
