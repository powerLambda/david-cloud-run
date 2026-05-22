# Plan: Optimizer Stage 3 — Write Current Prices Back to Feishu Bitable

## Context

Stages 1 and 2 are done: `FetchPortfolioCode` reads securities from Feishu Bitable, and `QueryPortfolioPrice` enriches them with current prices. Stage 3 writes the prices back to the "当前市价" column in the same Bitable table, using each record's `record_id` as the update key.

Key note from the user: `FetchPortfolioCode` must also return `record_id` per record so the update step can target the correct row.

---

## Flow after stage 3

```
FetchPortfolioCode  →  []PortfolioItem (with "record_id" key)
        ↓
QueryPortfolioPrice →  []PortfolioItemWithPrice (RecordID field)
        ↓
UpdatePortfolioPrice → PATCH each bitable record's "当前市价" field
        ↓
HTTP handler returns []PortfolioItemWithPrice as verification
```

---

## Feishu Bitable Update API

```
PATCH https://open.feishu.cn/open-apis/bitable/v1/apps/{app_token}/tables/{table_id}/records/{record_id}
Authorization: Bearer {token}
Content-Type: application/json

{"fields": {"当前市价": <float64>}}
```

Response: `{"code": 0, "msg": "success", "data": {"record": {...}}}`

---

## Files to change

| File | Action |
|---|---|
| `internal/optimizer/config.go` | Add `fieldCurrentPrice = "当前市价"` constant |
| `internal/optimizer/types.go` | Add `RecordID` to `PortfolioItemWithPrice`; add `updateResp` type |
| `internal/optimizer/optimizer.go` | `FetchPortfolioCode`: store `record_id` into item map; `QueryPortfolioPrice`: copy RecordID to WithPrice; add `UpdatePortfolioPrice` |
| `internal/optimizer/client.go` | Add `updateRecord` (low-level PATCH call) |
| `internal/optimizer/http.go` | Add `UpdatePortfolioPrice` call after `QueryPortfolioPrice` |
| `internal/optimizer/optimizer_test.go` | Update mock & assertions for `record_id` in `FetchPortfolioCode`; add E2E for full pipeline |

---

## Implementation

### `config.go` — add constant

```go
const fieldCurrentPrice = "当前市价"
```

---

### `types.go` — update struct and add response type

```go
type PortfolioItemWithPrice struct {
    RecordID string  `json:"record_id"`
    Name     string  `json:"name"`
    Code     string  `json:"code"`
    Price    float64 `json:"price"`
}

type updateResp struct {
    Code int    `json:"code"`
    Msg  string `json:"msg"`
}
```

---

### `optimizer.go` — three changes

**1. `FetchPortfolioCode`: include `record_id` in each PortfolioItem**

```go
item["record_id"] = r.RecordID
```

**2. `QueryPortfolioPrice`: pass RecordID through**

```go
result = append(result, PortfolioItemWithPrice{
    RecordID: item["record_id"],
    Name:     item["证券名称"],
    Code:     code,
    Price:    price,
})
```

**3. New `UpdatePortfolioPrice`**

```go
func UpdatePortfolioPrice(ctx context.Context, cfg Config, items []PortfolioItemWithPrice) error {
    httpCli := &http.Client{}
    token, err := fetchToken(ctx, httpCli, cfg.AppID, cfg.AppSecret)
    if err != nil {
        return err
    }
    for _, item := range items {
        fields := map[string]interface{}{fieldCurrentPrice: item.Price}
        if err := updateRecord(ctx, httpCli, token, bitableBaseID, bitableTableID, item.RecordID, fields); err != nil {
            log.Printf("optimizer: update error for record %q (%s): %v", item.RecordID, item.Code, err)
        }
    }
    return nil
}
```

---

### `client.go` — add `updateRecord`

```go
func updateRecord(ctx context.Context, httpCli *http.Client, token, baseID, tableID, recordID string, fields map[string]interface{}) error {
    body, _ := json.Marshal(map[string]interface{}{"fields": fields})
    url := fmt.Sprintf("%s/open-apis/bitable/v1/apps/%s/tables/%s/records/%s",
        feishuBaseURL, baseID, tableID, recordID)
    req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(body))
    if err != nil {
        return err
    }
    req.Header.Set("Content-Type", "application/json; charset=utf-8")
    req.Header.Set("Authorization", "Bearer "+token)
    resp, err := httpCli.Do(req)
    if err != nil {
        return fmt.Errorf("updateRecord: %w", err)
    }
    defer resp.Body.Close()
    var ur updateResp
    if err := json.NewDecoder(resp.Body).Decode(&ur); err != nil {
        return err
    }
    if ur.Code != 0 {
        return fmt.Errorf("feishu update error: code=%d msg=%s", ur.Code, ur.Msg)
    }
    return nil
}
```

---

### `http.go` — add UpdatePortfolioPrice step

```go
if err := UpdatePortfolioPrice(ctx, h.cfg, priced); err != nil {
    log.Printf("optimizer error updating bitable: %v", err)
    // non-fatal: still return priced data so the caller can see what was fetched
}
```

The handler returns `priced` (the `[]PortfolioItemWithPrice`) as before — the caller can use it to verify the update.

---

### `optimizer_test.go` — updates

- Mock server: add handler for `PATCH /open-apis/bitable/` returning `{"code":0}`
- `TestFetchPortfolioCode_FieldExtraction`: assert `items[0]["record_id"] == "recvkf0zJLIAg9"`
- E2E test `TestFetchPortfolioCodeE2E`: assert `items[0]["record_id"]` is non-empty

---

## Verification

```bash
# Unit tests (no network)
go test ./internal/optimizer/... -v -run 'TestFetchPortfolioCode_FieldExtraction|TestHandler'

# Full E2E pipeline: fetch codes → prices → write to bitable → verify bitable has updated values
OPTIMIZER_E2E=1 go test ./internal/optimizer/... -v -run 'TestFetchPortfolioCodeE2E|TestQueryPortfolioPriceE2E' -timeout 60s
```

E2E result verification: after `UpdatePortfolioPrice` completes, re-read the bitable row via `FetchPortfolioCode` (expanding field names to include "当前市价") and confirm the stored value matches the price returned by `QueryPortfolioPrice`.
