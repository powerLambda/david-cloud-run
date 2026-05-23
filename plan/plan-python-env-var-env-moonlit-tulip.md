# Plan: Add Local .env File Support

## Context

The project currently reads all environment variables via `os.Getenv()` at runtime. When running locally for testing, developers must manually export all required secrets before starting the server. This is inconvenient and error-prone.

The goal is to support a local `.env` file (same pattern as Python's `python-dotenv`) so developers can store real credentials locally and have them auto-loaded. The `.env` file must never be committed to git.

## Approach

Use `github.com/joho/godotenv` — the standard Go dotenv library. Load `.env` at the very start of `main()` before any config is read. Only load if the file exists (silent no-op in production/Cloud Run where the file won't be present).

## Changes

### 1. Add dependency — `go.mod` / `go.sum`
Run:
```
go get github.com/joho/godotenv
```

### 2. `main.go` — load `.env` at startup

Add at the top of `main()`, before `config.LoadConfig()`:

```go
import "github.com/joho/godotenv"

func main() {
    // Load .env if present (local dev only; no-op in production)
    if _, err := os.Stat(".env"); err == nil {
        if err := godotenv.Load(); err != nil {
            log.Fatalf("error loading .env file: %v", err)
        }
        log.Println("loaded .env file")
    }

    cfg, err := config.LoadConfig()
    // ... rest unchanged
}
```

`godotenv.Load()` does **not** overwrite env vars that are already set, so Cloud Run-injected vars always take precedence.

### 3. `.gitignore` — new file

```
.env
.env.*
```

### 4. `.env.example` — new file (committed, shows format only)

```
# Copy to .env and fill in real values (never commit .env)
CALDAV_USERNAME=your_username
CALDAV_PASSWORD=your_password
FEISHU_APP_ID=your_app_id
FEISHU_APP_SECRET=your_app_secret
```

## Critical Files

- [main.go](main.go) — add godotenv load at startup
- [go.mod](go.mod) — new dependency
- [.gitignore](.gitignore) — new file
- [.env.example](.env.example) — new file (safe to commit)

## Verification

1. Create a `.env` file with real credentials locally
2. Run `go run .` — the server should start and log `loaded .env file`
3. Verify env vars are active (e.g., `/caldav2ics/feishu` or `/optimizer/refresh-portfolio-price` responds correctly)
4. Confirm `git status` does not show `.env` as a tracked file
5. Deploy to Cloud Run — no `.env` file will exist there, so startup continues normally
