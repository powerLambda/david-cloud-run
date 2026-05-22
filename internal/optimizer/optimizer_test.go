package optimizer

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// newMockFeishu starts a test server that simulates the Feishu token + Bitable search endpoints.
func newMockFeishu(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/open-apis/auth/v3/tenant_access_token/internal", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(tokenResp{Code: 0, TenantAccessToken: "t-mock"})
	})

	// wildcard to match any baseID/tableID/recordID in the path
	mux.HandleFunc("/open-apis/bitable/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			// update record — return success
			_ = json.NewEncoder(w).Encode(updateResp{Code: 0, Msg: "success"})
			return
		}
		// POST — search records
		resp := searchResp{
			Code: 0,
			Data: searchData{
				Items: []record{
					{
						RecordID: "recvkf0zJLIAg9",
						Fields: map[string]json.RawMessage{
							"证券名称": json.RawMessage(`[{"text":"A50ETF广发","type":"text"}]`),
							"证券代码": json.RawMessage(`[{"text":"sz159136","type":"text"}]`),
						},
					},
					{
						RecordID: "recvkgjTF1Aw6U",
						Fields: map[string]json.RawMessage{
							"证券名称": json.RawMessage(`[{"text":"易方达恒生科技ETF联结(QDII)A","type":"text"}]`),
							"证券代码": json.RawMessage(`[{"text":"013308","type":"text"}]`),
						},
					},
				},
				Total: 2,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	return httptest.NewServer(mux)
}

func TestFetchPortfolioCode_FieldExtraction(t *testing.T) {
	ts := newMockFeishu(t)
	defer ts.Close()

	orig := feishuBaseURL
	feishuBaseURL = ts.URL
	defer func() { feishuBaseURL = orig }()

	items, err := FetchPortfolioCode(context.Background(), Config{AppID: "test", AppSecret: "secret"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0]["证券名称"] != "A50ETF广发" {
		t.Errorf("证券名称: got %q, want %q", items[0]["证券名称"], "A50ETF广发")
	}
	if items[0]["证券代码"] != "sz159136" {
		t.Errorf("证券代码: got %q, want %q", items[0]["证券代码"], "sz159136")
	}
	if items[0]["record_id"] != "recvkf0zJLIAg9" {
		t.Errorf("record_id: got %q, want %q", items[0]["record_id"], "recvkf0zJLIAg9")
	}
	if items[1]["证券代码"] != "013308" {
		t.Errorf("items[1] 证券代码: got %q, want %q", items[1]["证券代码"], "013308")
	}
}

func TestHandler_MethodNotAllowed(t *testing.T) {
	h := &Handler{cfg: Config{AppID: "x", AppSecret: "y"}}
	req := httptest.NewRequest(http.MethodPost, "/optimizer/refresh-portfolio-price", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestFetchPortfolioCodeE2E(t *testing.T) {
	if os.Getenv("OPTIMIZER_E2E") != "1" {
		t.Skip("set OPTIMIZER_E2E=1 to run")
	}
	cfg := LoadConfig()
	items, err := FetchPortfolioCode(context.Background(), cfg)
	if err != nil {
		t.Fatalf("E2E error: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected non-empty items")
	}
	t.Logf("total items: %d", len(items))
	for i, item := range items {
		t.Logf("items[%d] = %v", i, item)
	}
	if items[0]["证券名称"] != "A50ETF广发" {
		t.Errorf("items[0] 证券名称: got %q, want %q", items[0]["证券名称"], "A50ETF广发")
	}
	if items[0]["证券代码"] != "sz159136" {
		t.Errorf("items[0] 证券代码: got %q, want %q", items[0]["证券代码"], "sz159136")
	}
	if items[0]["record_id"] == "" {
		t.Error("items[0] record_id should be non-empty")
	}
}

// TestUpdatePortfolioPriceE2E runs the full pipeline:
// FetchPortfolioCode → QueryPortfolioPrice → UpdatePortfolioPrice.
// After the update it re-reads the bitable and verifies the "当前市价" column
// matches the prices that were written.
func TestUpdatePortfolioPriceE2E(t *testing.T) {
	if os.Getenv("OPTIMIZER_E2E") != "1" {
		t.Skip("set OPTIMIZER_E2E=1 to run")
	}
	ctx := context.Background()
	cfg := LoadConfig()

	// Step 1: fetch codes + record_ids from bitable
	items, err := FetchPortfolioCode(ctx, cfg)
	if err != nil {
		t.Fatalf("FetchPortfolioCode: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("no items returned from bitable")
	}

	// Step 2: query current prices
	priced, err := QueryPortfolioPrice(ctx, items)
	if err != nil {
		t.Fatalf("QueryPortfolioPrice: %v", err)
	}
	for _, p := range priced {
		t.Logf("price: %s (%s) record=%s price=%.4f", p.Name, p.Code, p.RecordID, p.Price)
		if p.RecordID == "" {
			t.Errorf("missing record_id for %s", p.Code)
		}
	}

	// Step 3: write prices back to bitable
	if err := UpdatePortfolioPrice(ctx, cfg, priced); err != nil {
		t.Fatalf("UpdatePortfolioPrice: %v", err)
	}
	t.Logf("UpdatePortfolioPrice completed for %d records", len(priced))

	// Step 4: re-read bitable and verify "当前市价" matches what was written
	httpCli := &http.Client{}
	token, err := fetchToken(ctx, httpCli, cfg.AppID, cfg.AppSecret)
	if err != nil {
		t.Fatalf("re-read fetchToken: %v", err)
	}
	records, err := searchRecords(ctx, httpCli, token, bitableBaseID, bitableTableID, searchReqBody{
		ViewID:          bitableViewID,
		FieldNames:      []string{"证券代码", fieldCurrentPrice},
		AutomaticFields: false,
	})
	if err != nil {
		t.Fatalf("re-read searchRecords: %v", err)
	}

	storedByCode := make(map[string]float64, len(records))
	for _, r := range records {
		code := ""
		if raw, ok := r.Fields["证券代码"]; ok {
			code = fieldText(raw)
		}
		if raw, ok := r.Fields[fieldCurrentPrice]; ok {
			var f float64
			if err := json.Unmarshal(raw, &f); err == nil {
				storedByCode[code] = f
			}
		}
	}

	for _, p := range priced {
		stored, ok := storedByCode[p.Code]
		if !ok {
			t.Errorf("code %q not found in re-read", p.Code)
			continue
		}
		if math.Abs(stored-p.Price) > 0.0001 {
			t.Errorf("code %q: bitable=%.4f, written=%.4f — mismatch", p.Code, stored, p.Price)
		} else {
			t.Logf("verified %s: bitable=%.4f == written=%.4f ✓", p.Code, stored, p.Price)
		}
	}
}
