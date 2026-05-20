package web2rss

import (
	"context"
	"log"
	"net/http"

	"github.com/powerLambda/david-cloud-run/internal/config"
)

type Handler struct {
	cfg config.Config
	svc *Service
}

func NewHandler(cfg config.Config, svc *Service) *Handler {
	return &Handler{cfg: cfg, svc: svc}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	page := r.URL.Query().Get("url")
	if page == "" {
		http.Error(w, "missing url param", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.Timeout)
	defer cancel()

	feed, err := h.svc.BuildFeedForURL(ctx, page)
	if err != nil {
		log.Printf("web2rss error: %v", err)
		http.Error(w, "failed to build feed", http.StatusBadGateway)
		return
	}
	rss, err := feed.ToRss()
	if err != nil {
		log.Printf("web2rss rss encode error: %v", err)
		http.Error(w, "rss encode failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(rss))
}
