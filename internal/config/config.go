package config

import (
	"flag"
	"os"
)

// Config holds all application configuration.
type Config struct {
	Addr   string // HTTP listen address, e.g. ":8080"
	APIKey string // API key for authentication (env API_KEY). Empty = auth disabled.
}

// Load parses flags and env vars. Flags take precedence over env vars.
func Load() *Config {
	addr := flag.String("addr", envOrDefault("ADDR", ":8080"), "HTTP listen address")
	flag.Parse()

	return &Config{
		Addr:   *addr,
		APIKey: os.Getenv("API_KEY"),
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
