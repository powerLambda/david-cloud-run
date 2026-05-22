package optimizer

import (
	"context"
	"os"
	"testing"
)

func TestParseTencentBody(t *testing.T) {
	// Real response format from https://qt.gtimg.cn/q=sz159136
	// Field index [3] (0-based, split by ~) is the current price.
	body := `v_sz159136="44~A50ETF广发~159136~1.040~1.050~1.039~...";`
	price, err := parseTencentBody(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if price != 1.040 {
		t.Errorf("got %v, want 1.040", price)
	}
}

func TestParseEastmoneyBody(t *testing.T) {
	// Real response fragment from https://fundf10.eastmoney.com/F10DataApi.aspx?type=lsjz&code=013841&page=1&per=1
	// Unit net value (单位净值) appears after the first "bold'>" and before the next "<".
	body := `{"Data":{"LSJZList":[{"FSRQ":"2024-05-20","DWJZ":"2.6432","JZZZL":"0.12"}]},...}<td>2024-05-20</td><td class='bold'>2.6432</td>`
	price, err := parseEastmoneyBody(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if price != 2.6432 {
		t.Errorf("got %v, want 2.6432", price)
	}
}

func TestQueryPortfolioPriceE2E(t *testing.T) {
	if os.Getenv("OPTIMIZER_E2E") != "1" {
		t.Skip("set OPTIMIZER_E2E=1 to run")
	}
	items := []PortfolioItem{
		{"证券名称": "A50ETF广发", "证券代码": "sz159136"},
		{"证券名称": "某基金", "证券代码": "013841"},
	}
	priced, err := QueryPortfolioPrice(context.Background(), items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(priced) != 2 {
		t.Fatalf("expected 2 items, got %d", len(priced))
	}
	for _, p := range priced {
		t.Logf("%s (%s): %.4f", p.Name, p.Code, p.Price)
		if p.Price <= 0 {
			t.Errorf("expected positive price for %q, got %v", p.Code, p.Price)
		}
	}
	// Tencent: sz159136 expected ~1.040 (may vary day to day, just check positive)
	if priced[0].Price <= 0 {
		t.Errorf("sz159136 price should be positive, got %v", priced[0].Price)
	}
	// Eastmoney: 013841 expected ~2.6432
	if priced[1].Price <= 0 {
		t.Errorf("013841 price should be positive, got %v", priced[1].Price)
	}
}
