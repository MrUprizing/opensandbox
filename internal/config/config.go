package config

import (
	"flag"
	"os"
	"strings"
)

// Config holds all-in-one application configuration.
type Config struct {
	Addr       string   // HTTP listen address, e.g. ":8080"
	APIKey     string   // API key for authentication (env API_KEY). Empty = auth disabled.
	ProxyAddrs []string // Reverse proxy listen addresses, e.g. [":80", ":3000"]
	BaseDomain string   // Base domain for subdomain routing, e.g. "localhost"
}

// PrimaryProxyAddr returns the first proxy address, used for generating URLs.
func (c *Config) PrimaryProxyAddr() string {
	if len(c.ProxyAddrs) == 0 {
		return ":80"
	}
	return c.ProxyAddrs[0]
}

// Load parses flags and env vars for the all-in-one binary.
func Load() *Config {
	addr := flag.String("addr", envOrDefault("ADDR", ":8080"), "HTTP listen address")
	proxyAddr := flag.String("proxy-addr", envOrDefault("PROXY_ADDR", ":80,:3000"), "Comma-separated proxy listen addresses (first is used for URL generation)")
	baseDomain := flag.String("base-domain", envOrDefault("BASE_DOMAIN", "localhost"), "Base domain for subdomain routing")
	flag.Parse()

	return &Config{
		Addr:       *addr,
		APIKey:     os.Getenv("API_KEY"),
		ProxyAddrs: parseAddrs(*proxyAddr),
		BaseDomain: *baseDomain,
	}
}

// WorkerConfig holds configuration for the worker binary.
type WorkerConfig struct {
	Addr            string // Worker HTTP listen address (default ":9090")
	APIKey          string // Shared API key for orchestrator ↔ worker auth
	OrchestratorURL string // Orchestrator URL for self-registration
	HostIP          string // Bind IP for container ports ("0.0.0.0" or "127.0.0.1")
}

// LoadWorker parses flags and env vars for the worker binary.
func LoadWorker() *WorkerConfig {
	addr := flag.String("addr", envOrDefault("WORKER_ADDR", ":9090"), "Worker HTTP listen address")
	orchestratorURL := flag.String("orchestrator-url", envOrDefault("ORCHESTRATOR_URL", ""), "Orchestrator URL for self-registration")
	hostIP := flag.String("host-ip", envOrDefault("HOST_IP", "0.0.0.0"), "Bind IP for container ports")
	flag.Parse()

	return &WorkerConfig{
		Addr:            *addr,
		APIKey:          os.Getenv("WORKER_API_KEY"),
		OrchestratorURL: *orchestratorURL,
		HostIP:          *hostIP,
	}
}

// OrchestratorConfig holds configuration for the orchestrator binary.
type OrchestratorConfig struct {
	Addr       string   // API HTTP listen address (default ":8080")
	APIKey     string   // Public API key
	WorkerKey  string   // Shared key for worker ↔ orchestrator auth
	ProxyAddrs []string // Proxy listen addresses
	BaseDomain string   // Base domain for proxy subdomains
}

// PrimaryProxyAddr returns the first proxy address, used for generating URLs.
func (c *OrchestratorConfig) PrimaryProxyAddr() string {
	if len(c.ProxyAddrs) == 0 {
		return ":80"
	}
	return c.ProxyAddrs[0]
}

// LoadOrchestrator parses flags and env vars for the orchestrator binary.
func LoadOrchestrator() *OrchestratorConfig {
	addr := flag.String("addr", envOrDefault("ADDR", ":8080"), "API HTTP listen address")
	proxyAddr := flag.String("proxy-addr", envOrDefault("PROXY_ADDR", ":80,:3000"), "Comma-separated proxy listen addresses")
	baseDomain := flag.String("base-domain", envOrDefault("BASE_DOMAIN", "localhost"), "Base domain for subdomain routing")
	flag.Parse()

	return &OrchestratorConfig{
		Addr:       *addr,
		APIKey:     os.Getenv("API_KEY"),
		WorkerKey:  os.Getenv("WORKER_API_KEY"),
		ProxyAddrs: parseAddrs(*proxyAddr),
		BaseDomain: *baseDomain,
	}
}

// parseAddrs splits a comma-separated list of addresses and trims whitespace.
func parseAddrs(raw string) []string {
	parts := strings.Split(raw, ",")
	addrs := make([]string, 0, len(parts))
	for _, p := range parts {
		if a := strings.TrimSpace(p); a != "" {
			addrs = append(addrs, a)
		}
	}
	return addrs
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
