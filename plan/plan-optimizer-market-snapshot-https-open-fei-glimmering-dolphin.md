# Plan: Send Market Snapshot to Feishu Group via Custom Bot Webhook

## Context

The `/optimizer/market-snapshot` endpoint already takes a screenshot of dapanyuntu.com, uploads it to Feishu IM, and returns the `image_key`. This plan extends it to also push that image to a configured Feishu group via a custom bot webhook (rich text / post message), so the snapshot is automatically delivered to the group chat after each capture.

## Files to Modify

1. `internal/optimizer/config.go` — add `SnapshotWebhookURL` field
2. `internal/optimizer/snapshot_types.go` — add bot webhook request/response types
3. `internal/optimizer/snapshot.go` — add `sendMarketSnapshotToGroup()`, call it from `TakeMarketSnapshot()`

## Implementation

### 1. `config.go` — add field and env var

Add `SnapshotWebhookURL string` to `Config` struct, loaded from `FEISHU_SNAPSHOT_WEBHOOK_URL`.

```go
type Config struct {
    AppID              string
    AppSecret          string
    BrowserlessURL     string
    BrowserlessToken   string
    SnapshotWebhookURL string  // new: FEISHU_SNAPSHOT_WEBHOOK_URL
}

func LoadConfig() Config {
    return Config{
        ...existing fields...
        SnapshotWebhookURL: strings.TrimSpace(os.Getenv("FEISHU_SNAPSHOT_WEBHOOK_URL")),
    }
}
```

No change to validation in `snapshot_http.go` — webhook URL is optional. If not set, the step is skipped.

### 2. `snapshot_types.go` — add rich text / post message types

Feishu custom bot rich text format (per docs https://open.feishu.cn/document/client-docs/bot-v3/add-custom-bot#f62e72d5):

```go
type botPostMsg struct {
    MsgType string         `json:"msg_type"`
    Content botPostContent `json:"content"`
}

type botPostContent struct {
    Post botPostLangs `json:"post"`
}

type botPostLangs struct {
    ZhCn botPostBody `json:"zh_cn"`
}

type botPostBody struct {
    Title   string              `json:"title"`
    Content [][]botPostImgElem  `json:"content"`
}

type botPostImgElem struct {
    Tag      string `json:"tag"`
    ImageKey string `json:"image_key"`
}

// Custom bot webhook returns StatusCode/StatusMessage (not code/msg like regular API)
type botPostResp struct {
    StatusCode    int    `json:"StatusCode"`
    StatusMessage string `json:"StatusMessage"`
}
```

### 3. `snapshot.go` — new function + call site

Add `sendMarketSnapshotToGroup()`:

```go
func sendMarketSnapshotToGroup(ctx context.Context, httpCli *http.Client, webhookURL, imageKey string, t time.Time) error {
    title := t.Format("2006-01-02 15:04") + " A股大盘云图"
    msg := botPostMsg{
        MsgType: "post",
        Content: botPostContent{
            Post: botPostLangs{
                ZhCn: botPostBody{
                    Title:   title,
                    Content: [][]botPostImgElem{{
                        {Tag: "img", ImageKey: imageKey},
                    }},
                },
            },
        },
    }
    body, err := json.Marshal(msg)
    if err != nil {
        return err
    }
    req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
    if err != nil {
        return err
    }
    req.Header.Set("Content-Type", "application/json; charset=utf-8")

    resp, err := httpCli.Do(req)
    if err != nil {
        return fmt.Errorf("feishu webhook: %w", err)
    }
    defer resp.Body.Close()

    var wr botPostResp
    if err := json.NewDecoder(resp.Body).Decode(&wr); err != nil {
        return err
    }
    if wr.StatusCode != 0 {
        return fmt.Errorf("feishu webhook error: code=%d msg=%s", wr.StatusCode, wr.StatusMessage)
    }
    return nil
}
```

In `TakeMarketSnapshot()`, after the existing `log.Printf("market snapshot: uploaded image_key=%s", imageKey)` line, add:

```go
if cfg.SnapshotWebhookURL != "" {
    if err := sendMarketSnapshotToGroup(ctx, httpCli, cfg.SnapshotWebhookURL, imageKey, time.Now()); err != nil {
        log.Printf("market snapshot: send to group failed: %v", err)
    } else {
        log.Printf("market snapshot: sent to group via webhook")
    }
}
```

Webhook failure is non-fatal — the snapshot image_key is still returned to the caller.

## Configuration

Add env var to `.env` (dev) and Cloud Run service config:

```
FEISHU_SNAPSHOT_WEBHOOK_URL=https://open.feishu.cn/open-apis/bot/v2/hook/74b89eb0-40d3-41ba-a128-f4daca68854d
```

## Rich Text Message Result

Title: `2026-05-26 10:30 A股大盘云图`  
Content: the uploaded screenshot image

## Verification

1. Set `FEISHU_SNAPSHOT_WEBHOOK_URL` in `.env`
2. Run `go build ./...` to verify compilation
3. `curl http://localhost:PORT/optimizer/market-snapshot` — should return `{"image_key": "..."}` and the image should appear in the configured Feishu group
4. If webhook URL is unset, the endpoint still works (no group message, no error)
