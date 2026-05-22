package optimizer

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"
)

type Handler struct {
	cfg    Config
	cfgErr error // non-nil if AppID/AppSecret absent; returned as 503 on each request
}

// NewHandler constructs the handler. Never fails. If FEISHU_APP_ID or
// FEISHU_APP_SECRET are absent, the error is deferred and returned as 503.
func NewHandler() *Handler {
	cfg := LoadConfig()
	var cfgErr error
	if cfg.AppID == "" || cfg.AppSecret == "" {
		cfgErr = errors.New("FEISHU_APP_ID and FEISHU_APP_SECRET are required")
	}
	return &Handler{cfg: cfg, cfgErr: cfgErr}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	items, err := FetchPortfolioCode(ctx, h.cfg)
	if err != nil {
		log.Printf("optimizer error fetching codes: %v", err)
		http.Error(w, "failed to fetch portfolio", http.StatusBadGateway)
		return
	}

	priced, err := QueryPortfolioPrice(ctx, items)
	if err != nil {
		log.Printf("optimizer error fetching prices: %v", err)
		http.Error(w, "failed to fetch prices", http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(priced)

	log.Printf("optimizer ok items=%d duration=%s", len(priced), time.Since(start))
}

func writeJSONError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
