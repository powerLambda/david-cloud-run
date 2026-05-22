# CalDAV to ICS for Cloud Run

This service exposes a public HTTP endpoint that queries Feishu CalDAV and returns an ICS feed for Google Calendar subscription.

## Endpoints

- `GET /caldav2ics/feishu` returns the ICS feed
- `GET /healthz` returns `ok`

## Environment Variables

Required:

- `CALDAV_USERNAME`
- `CALDAV_PASSWORD`
- `FEISHU_APP_ID` (for optimizer module)
- `FEISHU_APP_SECRET` (for optimizer module)

The CalDAV server is fixed to `https://caldav.feishu.cn`.

Optional:

- `CALDAV_PRINCIPAL_URL` (override principal path or URL)
- `CALDAV_CALENDAR_HOME` (override calendar home path or URL)
- `CALDAV_CALENDAR_URL` (override default calendar path or URL)
- `CALDAV_TIMEOUT` (default `15s`)
- `ENDPOINT_PATH` (default `/caldav2ics/feishu`)
- `TIMEZONE` (default `Asia/Shanghai`)
- `CALDAV_DEBUG` (`1`/`true`/`yes` to enable debug logging)
- `PORT` (default `8080`)
- `CALDAV_E2E` (set to `1` to enable the CalDAV e2e test)

## Local Run

```bash
export CALDAV_USERNAME="your-username"
export CALDAV_PASSWORD="your-password"

go run .
```

Test:

```bash
curl -v http://localhost:8080/caldav2ics/feishu
```

Tests:

```bash
go test ./...
CALDAV_E2E=1 CALDAV_USERNAME=... CALDAV_PASSWORD=... go test ./internal/caldav2ics -run TestCalDAV2ICSE2E
```

## Cloud Run Deploy

```bash
gcloud run deploy caldav2ics \
  --source . \
  --region asia-east1 \
  --allow-unauthenticated \
  --set-secrets="CALDAV_USERNAME=CALDAV_USERNAME:latest,CALDAV_PASSWORD=CALDAV_PASSWORD:latest,FEISHU_APP_ID=FEISHU_APP_ID:latest,FEISHU_APP_SECRET=FEISHU_APP_SECRET:latest"
```
 
## Web2RSS

`web2rss` 模块根据 per-URL 的 `sources.yaml` 配置抓取页面并生成 RSS。配置使用 Feed43 风格的 token pattern：

- Feed43 风格的 token pattern（示例：`<h2>{%}</h2>{*}<a href="{%}">{*}</a>{*}<p>{%}</p>`），模板使用 `{%1}`, `{%2}` 等引用捕获组。

示例 `internal/web2rss/sources.sample.yaml` 已包含注释说明；你也可以参考 `internal/web2rss/sources.yaml` 中的 LanceDB 示例。

测试与验证：

- 带开关的对照 e2e（使用 rsseverything 作为参考源，比较前 10 条 item 的 title/link/description）：
```bash
WEB2RSS_E2E=1 go test ./internal/web2rss -run TestRSSEverythingCompare -v
```

- 可重复运行的验证脚本：
```bash
./scripts/repeat_e2e.sh 5
```

注意：e2e 依赖外部站点（rsseverything 与 lancedb），我在测试中增加了 HTML 标签剥离、实体解码、空白归一化和特定尾注过滤来提高对齐的稳健性，但长期稳定性依赖于目标站点结构的变化。

Webcal subscription URL:

```
webcal://YOUR_CLOUD_RUN_HOST/caldav2ics/feishu
```
