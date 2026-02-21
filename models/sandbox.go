package models

import "time"

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

// SandboxSummary is a concise view of a sandbox for list endpoints.
type SandboxSummary struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Image     string            `json:"image"`
	Status    string            `json:"status"`
	State     string            `json:"state"`
	Ports     map[string]string `json:"ports"`
	ExpiresAt *time.Time        `json:"expires_at,omitempty"`
}

// SandboxDetail is the full inspect response with only relevant fields.
type SandboxDetail struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Image      string            `json:"image"`
	Status     string            `json:"status"`
	Running    bool              `json:"running"`
	Ports      map[string]string `json:"ports"`
	Resources  ResourceLimits    `json:"resources"`
	StartedAt  string            `json:"started_at"`
	FinishedAt string            `json:"finished_at"`
	ExpiresAt  *time.Time        `json:"expires_at,omitempty"`
}

// RestartResponse is the response for POST /v1/sandboxes/:id/restart
type RestartResponse struct {
	Status    string            `json:"status"`
	Ports     map[string]string `json:"ports"`
	ExpiresAt *time.Time        `json:"expires_at,omitempty"`
}

// ExecRequest is the body for POST /v1/sandboxes/:id/exec
type ExecRequest struct {
	Cmd []string `json:"cmd" binding:"required"`
}

// ExecResult holds the separated output from a command execution inside a sandbox.
// Used internally by the Docker client and returned directly by the API.
type ExecResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
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

// ImagePullRequest is the body for POST /v1/images/pull
type ImagePullRequest struct {
	Image string `json:"image" binding:"required"` // image name with optional tag (e.g. "nginx:latest")
}

// ImagePullResponse is the response for POST /v1/images/pull
type ImagePullResponse struct {
	Status string `json:"status"`
	Image  string `json:"image"`
}

// LogsOptions configures container log retrieval.
type LogsOptions struct {
	Tail       int  `form:"tail"`       // last N lines (default 100, 0 = all)
	Follow     bool `form:"follow"`     // stream in real-time via SSE
	Timestamps bool `form:"timestamps"` // include timestamp per line
}

// SandboxStats is a curated snapshot of container resource usage.
type SandboxStats struct {
	CPU    float64     `json:"cpu_percent"` // CPU usage percentage
	Memory MemoryUsage `json:"memory"`      // memory usage and limit
	PIDs   uint64      `json:"pids"`        // number of running processes
}

// MemoryUsage holds memory consumption details.
type MemoryUsage struct {
	Usage   uint64  `json:"usage"`   // bytes currently used
	Limit   uint64  `json:"limit"`   // bytes limit
	Percent float64 `json:"percent"` // usage / limit * 100
}

// ImageDetail is the inspect response for a single Docker image.
type ImageDetail struct {
	ID           string   `json:"id"`
	Tags         []string `json:"tags"`
	Size         int64    `json:"size"`         // bytes
	Created      string   `json:"created"`      // RFC3339
	Architecture string   `json:"architecture"` // e.g. "amd64"
	OS           string   `json:"os"`           // e.g. "linux"
}

// ImageSummary is a concise view of a local Docker image.
type ImageSummary struct {
	ID   string   `json:"id"`
	Tags []string `json:"tags"`
	Size int64    `json:"size"` // bytes
}
