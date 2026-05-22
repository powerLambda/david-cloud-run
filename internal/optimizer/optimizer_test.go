package optimizer

import (
	"context"
	"encoding/json"
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

	// wildcard to match any baseID/tableID in the path
	mux.HandleFunc("/open-apis/bitable/", func(w http.ResponseWriter, r *http.Request) {
		resp := searchResp{
			Code: 0,
			Data: searchData{
				Items: []record{
					{
						RecordID: "recvkf0zJLIAg9",
						Fields: map[string][]FieldValue{
							"证券名称": {{Text: "A50ETF广发", Type: "text"}},
							"证券代码": {{Text: "159136_SZ", Type: "text"}},
						},
					},
					{
						RecordID: "recvkgjTF1Aw6U",
						Fields: map[string][]FieldValue{
							"证券名称": {{Text: "易方达恒生科技ETF联结(QDII)A", Type: "text"}},
							"证券代码": {{Text: "013308", Type: "text"}},
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
	if items[0]["证券代码"] != "159136_SZ" {
		t.Errorf("证券代码: got %q, want %q", items[0]["证券代码"], "159136_SZ")
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
	if items[0]["证券代码"] != "159136_SZ" {
		t.Errorf("items[0] 证券代码: got %q, want %q", items[0]["证券代码"], "159136_SZ")
	}
}
