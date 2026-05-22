# Plan: Optimizer — Fetch Current Prices from External APIs

## Context

The optimizer endpoint already fetches security names and codes from Feishu Bitable via `FetchPortfolioCode`. This plan adds `QueryPortfolioPrice` on top: it calls `FetchPortfolioCode` then enriches each item with the current market price from an external API, and the HTTP handler is updated to call `QueryPortfolioPrice`.

Codes returned from the real Feishu Bitable are already in the correct format:
- Starting with `sz` or `sh` → Tencent stock API
- Pure digits → Eastmoney fund API

(The `"159136_SZ"` format in existing mock test data is a proxy artefact and must be corrected.)

---

## External APIs

| Code format | API | URL pattern | Price field |
|---|---|---|---|
| starts with `sz` or `sh` | Tencent `qt.gtimg.cn` | `https://qt.gtimg.cn/q=<code>` | split by `~`, index `[3]` |
| pure digits | Eastmoney `fundf10` | `https://fundf10.eastmoney.com/F10DataApi.aspx?type=lsjz&code=<code>&page=1&per=1` | text after first `bold'>`, before next `<` |

Test values: `sz159136` → `1.040`; `013841` → `2.6432`

---

## Files to change

| File | Action |
|---|---|
| `internal/optimizer/price.go` | **New** — `parseTencentBody`, `parseEastmoneyBody`, `fetchTencentPrice`, `fetchEastmoneyPrice`, `fetchPrice` |
| `internal/optimizer/price_test.go` | **New** — E2E tests only (no mocks) |
| `internal/optimizer/types.go` | Add `PortfolioItemWithPrice` struct |
| `internal/optimizer/optimizer.go` | Add `QueryPortfolioPrice` (calls `FetchPortfolioCode` + price loop) |
| `internal/optimizer/http.go` | Call `QueryPortfolioPrice` instead of `FetchPortfolioCode` |
| `internal/optimizer/optimizer_test.go` | Fix mock data (`"159136_SZ"` → `"sz159136"`); update assertions |

---

## Implementation

### `types.go` — new struct

```go
type PortfolioItemWithPrice struct {
    Name  string  `json:"name"`
    Code  string  `json:"code"`
    Price float64 `json:"price"`
}
```

---

### `price.go` — new file

**`parseTencentBody(body string) (float64, error)`**
- Response: `v_sz159136="股票名~代码~代码~1.040~…";`
- Find opening `"` and closing `"` → extract inner string → split by `~` → index `[3]` → `strconv.ParseFloat`

**`parseEastmoneyBody(body string) (float64, error)`**
- Find first `bold'>` → slice text up to next `<` → `strconv.ParseFloat`

**`fetchTencentPrice(ctx, client *http.Client, code string) (float64, error)`**
- GET `https://qt.gtimg.cn/q=` + code → `io.ReadAll` → `parseTencentBody`

**`fetchEastmoneyPrice(ctx, client *http.Client, code string) (float64, error)`**
- GET `https://fundf10.eastmoney.com/F10DataApi.aspx?type=lsjz&code=` + code + `&page=1&per=1` → `io.ReadAll` → `parseEastmoneyBody`

**`fetchPrice(ctx, client *http.Client, code string) (float64, error)`**
- `strings.HasPrefix(code, "sz") || strings.HasPrefix(code, "sh")` → `fetchTencentPrice`
- all-digit code → `fetchEastmoneyPrice`
- else → error (unknown format)

---

### `optimizer.go` — add `QueryPortfolioPrice`

The two functions are independent — `QueryPortfolioPrice` accepts `[]PortfolioItem` directly so each can be called and tested in isolation.

```go
// QueryPortfolioPrice enriches a slice of PortfolioItems with current market prices.
// It is independent of FetchPortfolioCode and can be called with any []PortfolioItem.
func QueryPortfolioPrice(ctx context.Context, items []PortfolioItem) ([]PortfolioItemWithPrice, error) {
    httpCli := &http.Client{}
    result := make([]PortfolioItemWithPrice, 0, len(items))
    for _, item := range items {
        code := item["证券代码"]
        price, err := fetchPrice(ctx, httpCli, code)
        if err != nil {
            log.Printf("optimizer: price fetch error for %q: %v", code, err)
        }
        result = append(result, PortfolioItemWithPrice{
            Name:  item["证券名称"],
            Code:  code,
            Price: price,
        })
    }
    return result, nil
}
```

`FetchPortfolioCode` is unchanged (signature and return type stay as `[]PortfolioItem`).

---

### `http.go` — orchestrate both functions

```go
items, err := FetchPortfolioCode(ctx, h.cfg)
// handle err ...
priced, err := QueryPortfolioPrice(ctx, items)
// handle err ...
json.NewEncoder(w).Encode(priced)
```

Response shape becomes `[{"name":…,"code":…,"price":…}]`.

---

### `optimizer_test.go` — fix mock data

Correct the hardcoded mock `Fields` to match real bitable format:
- `"证券代码": {{Text: "159136_SZ"}}` → `"证券代码": {{Text: "sz159136"}}`
- Update assertions that compare against `"159136_SZ"` to `"sz159136"`

---

### `price_test.go` — E2E only

```go
// TestParseTencentBody — pure string unit test, no network
// TestParseEastmoneyBody — pure string unit test, no network
// TestQueryPortfolioPriceE2E — guarded by OPTIMIZER_E2E=1, hits real Feishu + price APIs
```

The two parse functions are pure (no I/O), so they are unit-tested inline with canned strings. Network-dependent functions rely on E2E.

---

## Verification

```bash
# Parse unit tests (no network, no env vars)
go test ./internal/optimizer/... -v -run 'TestParse'

# E2E — requires FEISHU_APP_ID, FEISHU_APP_SECRET, no proxy
OPTIMIZER_E2E=1 go test ./internal/optimizer/... -v -run 'TestQueryPortfolioPriceE2E|TestFetchPortfolioCodeE2E' -timeout 60s
```
