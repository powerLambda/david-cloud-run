package optimizer

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"unicode"
)

// parseTencentBody extracts the current price from a Tencent stock API response.
// Response format: v_sz159136="name~code~code~1.040~...";
// Field index [3] (0-based, split by ~) is the current price.
func parseTencentBody(body string) (float64, error) {
	open := strings.IndexByte(body, '"')
	if open < 0 {
		return 0, fmt.Errorf("tencent: missing opening quote in %q", body)
	}
	close := strings.LastIndexByte(body, '"')
	if close <= open {
		return 0, fmt.Errorf("tencent: missing closing quote in %q", body)
	}
	parts := strings.Split(body[open+1:close], "~")
	if len(parts) < 4 {
		return 0, fmt.Errorf("tencent: need at least 4 fields, got %d", len(parts))
	}
	return strconv.ParseFloat(strings.TrimSpace(parts[3]), 64)
}

// parseEastmoneyBody extracts the unit net value (单位净值) from an Eastmoney fund response.
// The value appears immediately after the first "bold'>" and before the next "<".
func parseEastmoneyBody(body string) (float64, error) {
	const marker = "bold'>"
	idx := strings.Index(body, marker)
	if idx < 0 {
		return 0, fmt.Errorf("eastmoney: marker %q not found", marker)
	}
	rest := body[idx+len(marker):]
	end := strings.IndexByte(rest, '<')
	if end < 0 {
		return 0, fmt.Errorf("eastmoney: closing tag not found after marker")
	}
	return strconv.ParseFloat(strings.TrimSpace(rest[:end]), 64)
}

func fetchTencentPrice(ctx context.Context, client *http.Client, code string) (float64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://qt.gtimg.cn/q="+code, nil)
	if err != nil {
		return 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("tencent: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("tencent read: %w", err)
	}
	return parseTencentBody(string(body))
}

func fetchEastmoneyPrice(ctx context.Context, client *http.Client, code string) (float64, error) {
	url := "https://fundf10.eastmoney.com/F10DataApi.aspx?type=lsjz&code=" + code + "&page=1&per=1"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("eastmoney: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("eastmoney read: %w", err)
	}
	return parseEastmoneyBody(string(body))
}

// fetchPrice routes to the correct price API based on the security code format.
// Codes starting with "sz" or "sh" are exchange-listed securities (Tencent API).
// All-digit codes are mutual funds (Eastmoney API).
func fetchPrice(ctx context.Context, client *http.Client, code string) (float64, error) {
	if strings.HasPrefix(code, "sz") || strings.HasPrefix(code, "sh") {
		return fetchTencentPrice(ctx, client, code)
	}
	if isAllDigits(code) {
		return fetchEastmoneyPrice(ctx, client, code)
	}
	return 0, fmt.Errorf("unrecognized code format: %q", code)
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}
