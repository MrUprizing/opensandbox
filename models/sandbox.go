package models

// CreateSandboxRequest is the body for POST /v1/sandboxes
type CreateSandboxRequest struct {
	Image string   `json:"image" binding:"required"`
	Name  string   `json:"name"`
	Env   []string `json:"env"`
	Cmd   []string `json:"cmd"`
	Ports []string `json:"ports"` // container ports to expose: ["80/tcp", "443/tcp"]
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
