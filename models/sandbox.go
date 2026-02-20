package models

// ResourceLimits defines CPU and memory constraints for a sandbox.
type ResourceLimits struct {
	Memory int64   `json:"memory"` // memory limit in MB (e.g. 512 = 512MB). Default: 1024 (1GB), Max: 8192 (8GB)
	CPUs   float64 `json:"cpus"`   // fractional CPU limit (e.g. 1.5). Default: 1.0, Max: 4.0
}

// CreateSandboxRequest is the body for POST /v1/sandboxes
type CreateSandboxRequest struct {
	Image     string          `json:"image" binding:"required"`
	Name      string          `json:"name"`
	Env       []string        `json:"env"`
	Cmd       []string        `json:"cmd"`
	Ports     []string        `json:"ports"`     // container ports to expose: ["80/tcp", "443/tcp"]
	Timeout   int             `json:"timeout"`   // seconds until auto-stop, 0 = default (900s)
	Resources *ResourceLimits `json:"resources"` // CPU/memory limits, nil = defaults (1GB RAM, 1 vCPU)
}

// CreateSandboxResponse is the response for POST /v1/sandboxes
type CreateSandboxResponse struct {
	ID    string            `json:"id"`
	Ports map[string]string `json:"ports"` // "80/tcp": "32768"
}

// ExecRequest is the body for POST /v1/sandboxes/:id/exec
type ExecRequest struct {
	Cmd []string `json:"cmd" binding:"required"`
}

// ExecResponse is the response for POST /v1/sandboxes/:id/exec
type ExecResponse struct {
	Output string `json:"output"`
}

// FileReadResponse is the response for GET /v1/sandboxes/:id/files
type FileReadResponse struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// FileWriteRequest is the body for PUT /v1/sandboxes/:id/files
type FileWriteRequest struct {
	Content string `json:"content" binding:"required"`
}

// FileListResponse is the response for GET /v1/sandboxes/:id/files/list
type FileListResponse struct {
	Path   string `json:"path"`
	Output string `json:"output"`
}

// RenewExpirationRequest is the body for POST /v1/sandboxes/:id/renew-expiration
type RenewExpirationRequest struct {
	Timeout int `json:"timeout" binding:"required"` // new TTL in seconds
}

// RenewExpirationResponse is the response for POST /v1/sandboxes/:id/renew-expiration
type RenewExpirationResponse struct {
	Status  string `json:"status"`
	Timeout int    `json:"timeout"`
}
