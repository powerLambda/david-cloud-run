# Plan: Market Snapshot Handler (Browserless + Feishu Image Upload)

## Context
Add a new HTTP endpoint `/optimizer/market-snapshot` to the optimizer package. On each request it:
1. Calls a remote browserless container's REST screenshot API to capture https://dapanyuntu.com/
2. Receives JPEG image bytes directly (compressed via browserless quality option)
3. Fetches a Feishu tenant token, then uploads the image via Feishu IM image API
4. Returns `{"image_key": "img_xxx"}` to the caller

Credentials come from two new env vars (`BROWSERLESS_URL`, `BROWSERLESS_TOKEN`) following the same `os.Getenv` + `strings.TrimSpace` pattern as existing Feishu vars.

---

## Files to Create / Modify

| File | Action |
|------|--------|
| `internal/optimizer/config.go` | Extend `Config` struct and `LoadConfig()` |
| `internal/optimizer/snapshot_types.go` | New – request/response types for browserless + Feishu image API |
| `internal/optimizer/snapshot.go` | New – `TakeMarketSnapshot()` core logic |
| `internal/optimizer/snapshot_http.go` | New – `SnapshotHandler` struct + `NewSnapshotHandler()` + `ServeHTTP` |
| `main.go` | Register `/optimizer/market-snapshot` route |
| `.env.example` | Document the two new env vars |

---

## Step-by-step Implementation

### 1. `internal/optimizer/config.go` – Extend Existing Config

`AppID`/`AppSecret` already exist — only add the two browserless fields:
```go
// existing fields unchanged
type Config struct {
    AppID            string
    AppSecret        string
    BrowserlessURL   string // NEW: e.g. https://chrome.browserless.io
    BrowserlessToken string // NEW: browserless API token
}

func LoadConfig() Config {
    return Config{
        AppID:            strings.TrimSpace(os.Getenv("FEISHU_APP_ID")),     // unchanged
        AppSecret:        strings.TrimSpace(os.Getenv("FEISHU_APP_SECRET")), // unchanged
        BrowserlessURL:   strings.TrimSpace(os.Getenv("BROWSERLESS_URL")),   // new
        BrowserlessToken: strings.TrimSpace(os.Getenv("BROWSERLESS_TOKEN")), // new
    }
}
```

The existing `NewHandler()` is unaffected — it already calls `LoadConfig()` and ignores the two new fields.
`NewSnapshotHandler()` calls the same `LoadConfig()` and uses all four fields.

Add a snapshot-specific timeout constant:
```go
const snapshotTimeout = 60 * time.Second
```

### 2. `internal/optimizer/snapshot_types.go` – New Types

```go
package optimizer

// browserless REST screenshot request body
type screenshotReq struct {
    URL     string            `json:"url"`
    Options screenshotOptions `json:"options"`
}

type screenshotOptions struct {
    Type    string `json:"type"`    // "jpeg"
    Quality int    `json:"quality"` // 0–100
}

// Feishu IM image upload response
type feishuImageResp struct {
    Code int             `json:"code"`
    Msg  string          `json:"msg"`
    Data feishuImageData `json:"data"`
}

type feishuImageData struct {
    ImageKey string `json:"image_key"`
}
```

### 3. `internal/optimizer/snapshot.go` – Core Logic

Two functions:

**`takeScreenshot(ctx, httpCli, browserlessURL, token) ([]byte, error)`**
- POST `{browserlessURL}/screenshot?token={token}`
- Body: `{"url": "https://dapanyuntu.com/", "options": {"type": "jpeg", "quality": 75}}`
- Returns raw JPEG bytes from response body

**`uploadFeishuImage(ctx, httpCli, feishuToken string, imageData []byte) (string, error)`**
- Uses `mime/multipart` to build a multipart/form-data body with:
  - field `image_type` = `"msg"`
  - file field `image` with filename `snapshot.jpg`
- POST `feishuBaseURL + "/open-apis/im/v1/images"`
- Header: `Authorization: Bearer {feishuToken}`
- Decodes `feishuImageResp`, returns `data.image_key`

**`TakeMarketSnapshot(ctx, cfg) (string, error)`** (public, for tests)
- Creates shared `*http.Client{}`
- Calls `fetchToken(ctx, httpCli, cfg.AppID, cfg.AppSecret)` (reuse existing)
- Calls `takeScreenshot(ctx, httpCli, cfg.BrowserlessURL, cfg.BrowserlessToken)`
- Calls `uploadFeishuImage(ctx, httpCli, feishuToken, imageBytes)`
- Returns `image_key`

### 4. `internal/optimizer/snapshot_http.go` – HTTP Handler

```go
type SnapshotHandler struct {
    cfg    Config
    cfgErr error
}

func NewSnapshotHandler() *SnapshotHandler {
    cfg := LoadConfig()
    var cfgErr error
    // All four fields are required; check all together so one 503 covers both concerns
    switch {
    case cfg.AppID == "" || cfg.AppSecret == "":
        cfgErr = errors.New("FEISHU_APP_ID and FEISHU_APP_SECRET are required")
    case cfg.BrowserlessURL == "" || cfg.BrowserlessToken == "":
        cfgErr = errors.New("BROWSERLESS_URL and BROWSERLESS_TOKEN are required")
    }
    return &SnapshotHandler{cfg: cfg, cfgErr: cfgErr}
}

func (h *SnapshotHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // GET only
    // validate cfgErr → 503
    // context with snapshotTimeout
    // imageKey, err := TakeMarketSnapshot(ctx, h.cfg)
    // → 502 on error
    // → 200 JSON {"image_key": imageKey}
}
```

### 5. `main.go` – Register Route

```go
mux.Handle("/optimizer/market-snapshot", optimizer.NewSnapshotHandler())
```

### 6. `.env.example` – Document New Vars

```
BROWSERLESS_URL=https://your-browserless-host
BROWSERLESS_TOKEN=your_browserless_token
```

---

## Reused Existing Code

- `fetchToken()` in `client.go:14` — reused as-is for Feishu auth
- `feishuBaseURL` var in `client.go:12` — reused for image upload URL
- `writeJSONError()` in `http.go:69` — reused for error responses
- `LoadConfig()` pattern in `config.go:24` — extended, not replaced
- `requestTimeout` / `snapshotTimeout` pattern — add alongside existing constant

---

## Verification

1. Set env vars in `.env`: `FEISHU_APP_ID`, `FEISHU_APP_SECRET`, `BROWSERLESS_URL`, `BROWSERLESS_TOKEN`
2. `go build ./...` — confirms no compile errors
3. `go run . &` then `curl http://localhost:PORT/optimizer/market-snapshot`
4. Check response contains `{"image_key": "img_..."}` with a valid key
5. Paste the image_key into a Feishu message to confirm the image loads correctly
6. Test error path: unset `BROWSERLESS_TOKEN`, verify handler returns 503 with error JSON
