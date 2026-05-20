package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	defaultCaldavURL    = "https://caldav.feishu.cn"
	defaultEndpointPath = "/caldav2ics/feishu"
	defaultTimezone     = "Asia/Shanghai"
	defaultTimeout      = 15 * time.Second
)

type Config struct {
	CaldavURL          string
	CaldavUsername     string
	CaldavPassword     string
	CaldavPrincipalURL string
	CaldavCalendarHome string
	CaldavCalendarURL  string
	Timezone           string
	Timeout            time.Duration
	ListenAddr         string
	Debug              bool
}

func LoadConfig() (Config, error) {
	cfg := Config{
		CaldavURL:          defaultCaldavURL,
		CaldavUsername:     strings.TrimSpace(os.Getenv("CALDAV_USERNAME")),
		CaldavPassword:     strings.TrimSpace(os.Getenv("CALDAV_PASSWORD")),
		CaldavPrincipalURL: strings.TrimSpace(os.Getenv("CALDAV_PRINCIPAL_URL")),
		CaldavCalendarHome: strings.TrimSpace(os.Getenv("CALDAV_CALENDAR_HOME")),
		CaldavCalendarURL:  strings.TrimSpace(os.Getenv("CALDAV_CALENDAR_URL")),
		Timezone:           strings.TrimSpace(os.Getenv("TIMEZONE")),
	}
	if cfg.Timezone == "" {
		cfg.Timezone = defaultTimezone
	}

	timeoutStr := strings.TrimSpace(os.Getenv("CALDAV_TIMEOUT"))
	if timeoutStr == "" {
		cfg.Timeout = defaultTimeout
	} else {
		parsed, err := time.ParseDuration(timeoutStr)
		if err != nil {
			return cfg, fmt.Errorf("invalid CALDAV_TIMEOUT: %w", err)
		}
		cfg.Timeout = parsed
	}

	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "8080"
	}
	cfg.ListenAddr = ":" + port

	if cfg.CaldavUsername == "" || cfg.CaldavPassword == "" {
		return cfg, errors.New("CALDAV_USERNAME and CALDAV_PASSWORD are required")
	}

	debugStr := strings.TrimSpace(os.Getenv("CALDAV_DEBUG"))
	if debugStr != "" {
		switch strings.ToLower(debugStr) {
		case "1", "true", "yes", "y", "on":
			cfg.Debug = true
		}
	}
	return cfg, nil
}
