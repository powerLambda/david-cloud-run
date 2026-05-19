package main

import (
	"log"
	"net/http"
	"time"
	_ "time/tzdata"

	"github.com/powerLambda/david-cloud-run/internal/caldav2ics"
	"github.com/powerLambda/david-cloud-run/internal/config"
	"github.com/powerLambda/david-cloud-run/internal/modules"
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

	mux := http.NewServeMux()
	modules.Register(mux, caldav2ics.NewModule(cfg, client))
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
