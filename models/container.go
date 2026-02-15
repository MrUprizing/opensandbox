package models

// CreateContainerRequest is the body for POST /containers
type CreateContainerRequest struct {
	Image string   `json:"image" binding:"required"`
	Name  string   `json:"name"`
	Env   []string `json:"env"`
	Cmd   []string `json:"cmd"`
	Ports []string `json:"ports"` // container ports to expose: ["80/tcp", "443/tcp"]
}

// ExecRequest is the body for POST /containers/:id/exec
type ExecRequest struct {
	Cmd []string `json:"cmd" binding:"required"`
}
