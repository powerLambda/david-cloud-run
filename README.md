# CalDAV to ICS for Cloud Run

This service exposes a public HTTP endpoint that queries Feishu CalDAV and returns an ICS feed for Google Calendar subscription.

## Endpoints

- `GET /caldav2ics/feishu` returns the ICS feed
- `GET /healthz` returns `ok`

## Environment Variables

Required:

- `CALDAV_USERNAME`
- `CALDAV_PASSWORD`

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
  --set-secrets CALDAV_USERNAME=projects/PROJECT_ID/secrets/CALDAV_USERNAME:latest,CALDAV_PASSWORD=projects/PROJECT_ID/secrets/CALDAV_PASSWORD:latest
```

Webcal subscription URL:

```
webcal://YOUR_CLOUD_RUN_HOST/caldav2ics/feishu
```
