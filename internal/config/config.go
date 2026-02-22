package config

import (
	"flag"
	"os"
)

// Config holds all application configuration.
type Config struct {
	Addr       string // HTTP listen address, e.g. ":8080"
	APIKey     string // API key for authentication (env API_KEY). Empty = auth disabled.
	ProxyAddr  string // Reverse proxy listen address, e.g. ":3000"
	BaseDomain string // Base domain for subdomain routing, e.g. "localhost"
}

// Load parses flags and env vars. Flags take precedence over env vars.
func Load() *Config {
	addr := flag.String("addr", envOrDefault("ADDR", ":8080"), "HTTP listen address")
	proxyAddr := flag.String("proxy-addr", envOrDefault("PROXY_ADDR", ":3000"), "Reverse proxy listen address")
	baseDomain := flag.String("base-domain", envOrDefault("BASE_DOMAIN", "localhost"), "Base domain for subdomain routing")
	flag.Parse()

	return &Config{
		Addr:       *addr,
		APIKey:     os.Getenv("API_KEY"),
		ProxyAddr:  *proxyAddr,
		BaseDomain: *baseDomain,
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
