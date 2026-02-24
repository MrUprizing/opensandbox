package proxy

import (
	"fmt"
	"net/url"

	"open-sandbox/internal/database"
)

// resolve looks up the sandbox by name and returns the target URL.
// In all-in-one mode: http://127.0.0.1:{hostPort}
// In orchestrator mode: http://{workerIP}:{hostPort}
func (s *Server) resolve(name string) (*url.URL, error) {
	// Check cache first.
	if target, ok := s.cache.get(name); ok {
		return target, nil
	}

	// DB lookup.
	sb, err := s.repo.FindByName(name)
	if err != nil {
		return nil, fmt.Errorf("lookup failed: %w", err)
	}
	if sb == nil {
		return nil, fmt.Errorf("not found")
	}

	// Resolve the host port for the main port.
	hostPort, err := resolveHostPort(sb)
	if err != nil {
		return nil, err
	}

	// Determine the host: worker IP if assigned, otherwise 127.0.0.1.
	host := "127.0.0.1"
	if sb.WorkerID != "" {
		w, wErr := s.repo.FindWorkerByID(sb.WorkerID)
		if wErr == nil && w != nil {
			if ip := extractHost(w.URL); ip != "" {
				host = ip
			}
		}
	}

	target := &url.URL{
		Scheme: "http",
		Host:   host + ":" + hostPort,
	}

	s.cache.set(name, target)
	return target, nil
}

// extractHost extracts the host (IP or hostname) from a URL string.
// "http://10.0.0.2:9090" → "10.0.0.2"
func extractHost(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	h := u.Hostname()
	// Don't use 0.0.0.0 as a target — it means "all interfaces" on the worker,
	// but we need the actual worker IP. In practice the orchestrator should have
	// the real IP from the worker's registration URL.
	if h == "0.0.0.0" {
		return ""
	}
	return h
}

// resolveHostPort returns the Docker-assigned host port for the sandbox's port.
// If Port is not set but there is exactly one port in the map, it uses that.
func resolveHostPort(sb *database.Sandbox) (string, error) {
	if sb.Port != "" {
		hp, ok := sb.Ports[sb.Port]
		if !ok {
			return "", fmt.Errorf("port %q not found in port map %v", sb.Port, sb.Ports)
		}
		return hp, nil
	}

	// Fallback: use the only port if there is exactly one.
	if len(sb.Ports) == 1 {
		for _, hp := range sb.Ports {
			return hp, nil
		}
	}

	return "", fmt.Errorf("no port configured and sandbox has %d ports", len(sb.Ports))
}
