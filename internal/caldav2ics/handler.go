package caldav2ics

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/powerLambda/david-cloud-run/internal/config"
)

type Client interface {
	FetchCalendarData(ctx context.Context) ([]string, error)
}

type Handler struct {
	cfg             config.Config
	client          Client
	buildCalendarFn func(ctx context.Context) ([]byte, error)
}

func NewHandler(cfg config.Config, client Client) *Handler {
	return &Handler{cfg: cfg, client: client}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.Timeout)
	defer cancel()

	ics, err := h.buildCalendar(ctx)
	if err != nil {
		log.Printf("caldav2ics error: %v", err)
		http.Error(w, "caldav fetch failed", http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Content-Disposition", "inline; filename=calendar.ics")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(ics)

	log.Printf("caldav2ics ok duration=%s", time.Since(start))
}

func (h *Handler) buildCalendar(ctx context.Context) ([]byte, error) {
	if h.buildCalendarFn != nil {
		return h.buildCalendarFn(ctx)
	}
	loc, err := time.LoadLocation(h.cfg.Timezone)
	if err != nil {
		return nil, err
	}

	// keep timezone lookup for parity, but do not use time-window filtering
	_ = loc

	calendarData, err := h.client.FetchCalendarData(ctx)
	if err != nil {
		return nil, err
	}

	return BuildICS(h.cfg.Timezone, calendarData), nil
}
