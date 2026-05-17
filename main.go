package main

import (
	"context"
	"log"
	"net/http"
	"time"
	_ "time/tzdata"
)

type Service struct {
	cfg    Config
	client *CalDAVClient
}

func main() {
	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	service := &Service{
		cfg:    cfg,
		client: NewCalDAVClient(cfg),
	}

	mux := http.NewServeMux()
	mux.HandleFunc(cfg.EndpointPath, service.handleICS)
	mux.HandleFunc("/healthz", handleHealth)

	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
	}

	log.Printf("listening on %s", cfg.ListenAddr)
	log.Fatal(server.ListenAndServe())
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *Service) handleICS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.Timeout)
	defer cancel()

	ics, err := s.buildCalendar(ctx)
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

func (s *Service) buildCalendar(ctx context.Context) ([]byte, error) {
	loc, err := time.LoadLocation(s.cfg.Timezone)
	if err != nil {
		return nil, err
	}

	now := time.Now().In(loc)
	start := now.AddDate(0, -1, 0)
	end := now.AddDate(0, 1, 0)

	calendarData, err := s.client.FetchCalendarData(ctx, start, end)
	if err != nil {
		return nil, err
	}

	return BuildICS(s.cfg.Timezone, calendarData), nil
}
