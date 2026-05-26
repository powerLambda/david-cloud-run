package optimizer

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"
)

type SnapshotHandler struct {
	cfg    Config
	cfgErr error
}

// NewSnapshotHandler constructs the handler. Never fails. If required env vars are
// absent, the error is deferred and returned as 503 on each request.
func NewSnapshotHandler() *SnapshotHandler {
	cfg := LoadConfig()
	var cfgErr error
	switch {
	case cfg.AppID == "" || cfg.AppSecret == "":
		cfgErr = errors.New("FEISHU_APP_ID and FEISHU_APP_SECRET are required")
	case cfg.BrowserlessURL == "" || cfg.BrowserlessToken == "":
		cfgErr = errors.New("BROWSERLESS_URL and BROWSERLESS_TOKEN are required")
	}
	return &SnapshotHandler{cfg: cfg, cfgErr: cfgErr}
}

func (h *SnapshotHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if h.cfgErr != nil {
		writeJSONError(w, h.cfgErr.Error(), http.StatusServiceUnavailable)
		return
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), snapshotTimeout)
	defer cancel()

	imageKey, err := TakeMarketSnapshot(ctx, h.cfg)
	if err != nil {
		log.Printf("market snapshot error: %v", err)
		writeJSONError(w, "failed to take market snapshot", http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"image_key": imageKey})

	log.Printf("market snapshot ok image_key=%s duration=%s", imageKey, time.Since(start))
}
