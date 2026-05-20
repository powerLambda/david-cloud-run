package main

import (
	"log"
	"net/http"
	"os"
	"time"
	_ "time/tzdata"

	"github.com/powerLambda/david-cloud-run/internal/caldav2ics"
	"github.com/powerLambda/david-cloud-run/internal/config"
	"github.com/powerLambda/david-cloud-run/internal/web2rss"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	client, err := caldav2ics.NewClient(cfg)
	if err != nil {
		log.Fatalf("caldav client error: %v", err)
	}

	// load web2rss sources from default file path (fail-fast)
	sourcesPath := "./internal/web2rss/sources.yaml"
	sf, err := os.Open(sourcesPath)
	if err != nil {
		log.Fatalf("failed to open web2rss sources (%s): %v", sourcesPath, err)
	}
	defer sf.Close()
	sources, err := web2rss.LoadSources(sf)
	if err != nil {
		log.Fatalf("failed to load web2rss sources: %v", err)
	}
	webSvc := web2rss.NewService(cfg, sources)

	mux := http.NewServeMux()
	mux.Handle("/caldav2ics/feishu", caldav2ics.NewHandler(cfg, client))
	mux.Handle("/web2rss", web2rss.NewHandler(cfg, webSvc))
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
