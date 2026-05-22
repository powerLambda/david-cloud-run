# Plan: Portfolio Optimizer Module

## Context

Add a new `internal/optimizer` package that authenticates with Feishu (Lark) and reads records from a Bitable (Base) table, exposing the result via `GET /optimizer/refresh-portfolio-price`. This is a read-only, on-demand REST endpoint — no startup work, no CalDAV dependency. The module follows the same three-layer pattern as `web2rss` (config → client → handler).

---

## Feishu API Contract (from official docs)

### Token
`POST https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal`
```json
// request
{"app_id": "cli_xxx", "app_secret": "xxx"}
// response
{"code": 0, "msg": "ok", "tenant_access_token": "t-xxx", "expire": 7200}
```

### Search Records
`POST https://open.feishu.cn/open-apis/bitable/v1/apps/{app_token}/tables/{table_id}/records/search`
```
Authorization: Bearer <tenant_access_token>
Content-Type: application/json
```
Request body (hardcoded):
```json
{
  "view_id": "vewgMuvngQ",
  "field_names": ["证券名称", "证券代码"],
  "automatic_fields": false
}
```
Response — field values are `[{"text": "...", "type": "text"}]` arrays:
```json
{
  "code": 0,
  "data": {
    "has_more": false,
    "items": [
      {
        "record_id": "recvkf0zJLIAg9",
        "fields": {
          "证券代码": [{"text": "159136_SZ", "type": "text"}],
          "证券名称": [{"text": "A50ETF广发", "type": "text"}]
        }
      }
    ],
    "total": 12
  },
  "msg": "success"
}
```

---

## Configuration

### Environment Variables (only credentials remain)

| Variable            | Description       | Required |
|---------------------|-------------------|----------|
| `FEISHU_APP_ID`     | Feishu app ID     | Yes      |
| `FEISHU_APP_SECRET` | Feishu app secret | Yes      |

### Constants (hardcoded in `internal/optimizer/config.go`)

```go
const (
    bitableBaseID  = "<FEISHU_BASE_ID_VALUE>"   // Bitable app token
    bitableTableID = "<FEISHU_TABLE_ID_VALUE>"  // table ID
    bitableViewID  = "vewgMuvngQ"               // view ID
    requestTimeout = 15 * time.Second
)

var portfolioFieldNames = []string{"证券名称", "证券代码"}
```

---

## Files to Create

### `internal/optimizer/config.go`
```go
type Config struct {
    AppID     string
    AppSecret string
}

func LoadConfig() Config  // reads only FEISHU_APP_ID and FEISHU_APP_SECRET
```

### `internal/optimizer/types.go`
```go
type tokenReq  struct { AppID string `json:"app_id"`; AppSecret string `json:"app_secret"` }
type tokenResp struct { Code int; Msg string; TenantAccessToken string `json:"tenant_access_token"` }

type searchReqBody struct {
    ViewID          string   `json:"view_id,omitempty"`
    FieldNames      []string `json:"field_names,omitempty"`
    AutomaticFields bool     `json:"automatic_fields"`
}

// Each field value is a []FieldValue e.g. [{"text":"159136_SZ","type":"text"}]
type FieldValue struct {
    Text string `json:"text"`
    Type string `json:"type"`
}

type searchResp struct {
    Code int        `json:"code"`
    Msg  string     `json:"msg"`
    Data searchData `json:"data"`
}

type searchData struct {
    HasMore bool     `json:"has_more"`
    Items   []record `json:"items"`
    Total   int      `json:"total"`
}

type record struct {
    RecordID string                  `json:"record_id"`
    Fields   map[string][]FieldValue `json:"fields"`
}

// PortfolioItem holds extracted text values keyed by field name
// e.g. {"证券名称": "A50ETF广发", "证券代码": "159136_SZ"}
type PortfolioItem map[string]string
```

### `internal/optimizer/client.go`
```go
func fetchToken(ctx context.Context, httpCli *http.Client, appID, appSecret string) (string, error)
// POST https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal

func searchRecords(ctx context.Context, httpCli *http.Client, token, baseID, tableID string, body searchReqBody) ([]record, error)
// POST https://open.feishu.cn/open-apis/bitable/v1/apps/{baseID}/tables/{tableID}/records/search
// Returns error if code != 0
```

### `internal/optimizer/optimizer.go`
```go
// FetchPortfolioCode authenticates to Feishu, queries Bitable using hardcoded
// view + field constants, and returns one PortfolioItem per record.
func FetchPortfolioCode(ctx context.Context, cfg Config) ([]PortfolioItem, error)
```

Implementation:
1. Call `fetchToken(ctx, httpCli, cfg.AppID, cfg.AppSecret)`
2. Build `searchReqBody{ViewID: bitableViewID, FieldNames: portfolioFieldNames, AutomaticFields: false}`
3. Call `searchRecords(ctx, httpCli, token, bitableBaseID, bitableTableID, body)`
4. For each record: build `PortfolioItem` — extract `fields[name][0].Text` for each field name (skip if slice empty)
5. Return `[]PortfolioItem{}` (not nil) when empty

### `internal/optimizer/http.go`
```go
type Handler struct {
    cfg    Config
    cfgErr error  // non-nil if AppID/AppSecret missing; returns 503 per request
}

// NewHandler never fails; credentials absence deferred to request time.
func NewHandler() *Handler

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request)
// 1. Reject non-GET → 405
// 2. Return 503 JSON if h.cfgErr != nil
// 3. context.WithTimeout(r.Context(), requestTimeout)
// 4. Call FetchPortfolioCode → error → log + 502; success → 200 JSON array
```

Response: `Content-Type: application/json; charset=utf-8`
```json
[
  {"证券名称": "A50ETF广发", "证券代码": "159136_SZ"},
  {"证券名称": "易方达恒生科技ETF联结(QDII)A", "证券代码": "013308"}
]
```

### `internal/optimizer/optimizer_test.go`
Essential tests only:
- `TestFetchPortfolioCode_FieldExtraction` — unit test with `httptest.NewServer` mocking Feishu API; verifies `[{"text":"159136_SZ","type":"text"}]` → `"159136_SZ"` and correct JSON response shape
- `TestHandler_MethodNotAllowed` — POST → 405

E2E test gated by `OPTIMIZER_E2E=1`:
```go
func TestFetchPortfolioCodeE2E(t *testing.T) {
    if os.Getenv("OPTIMIZER_E2E") != "1" { t.Skip("set OPTIMIZER_E2E=1 to run") }
    cfg := LoadConfig()
    items, err := FetchPortfolioCode(context.Background(), cfg)
    require.NoError(t, err)
    assert.NotEmpty(t, items)
    // assert known records present
    assert.Equal(t, 12, len(items))
    assert.Equal(t, "A50ETF广发", items[0]["证券名称"])
    assert.Equal(t, "159136_SZ", items[0]["证券代码"])
}
```

---

## Files to Modify

### `main.go` — 2 lines added
```go
// import block:
"github.com/powerLambda/david-cloud-run/internal/optimizer"

// after mux creation:
mux.Handle("/optimizer/refresh-portfolio-price", optimizer.NewHandler())
```

### `tests/optimizer.http` — new file
```http
### Refresh portfolio prices
GET http://localhost:8080/optimizer/refresh-portfolio-price

### Method not allowed → 405
POST http://localhost:8080/optimizer/refresh-portfolio-price
```

---

## Dependencies

No new Go module dependencies — pure stdlib (`net/http`, `encoding/json`, `context`, `time`, `os`, `strings`).

---

## Verification

1. **Build check**:
   ```bash
   go build ./...
   ```

2. **Unit tests** (no credentials needed):
   ```bash
   go test ./internal/optimizer/...
   ```

3. **E2E — real Feishu API query** (requires credentials):
   ```bash
   OPTIMIZER_E2E=1 FEISHU_APP_ID=<app_id> FEISHU_APP_SECRET=<app_secret> \
     go test ./internal/optimizer/... -run E2E -v
   ```
   Expected output:
   ```
   --- PASS: TestFetchPortfolioCodeE2E
   items[0] = {"证券名称":"A50ETF广发","证券代码":"159136_SZ"}
   items[1] = {"证券名称":"易方达恒生科技ETF联结(QDII)A","证券代码":"013308"}
   ... (12 total)
   ```

4. **Live smoke test**:
   ```bash
   FEISHU_APP_ID=<app_id> FEISHU_APP_SECRET=<app_secret> go run . &
   curl http://localhost:8080/optimizer/refresh-portfolio-price
   # Expected: JSON array of 12 portfolio items with 证券名称 + 证券代码
   ```
