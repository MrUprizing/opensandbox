package api

import (
	"context"

	"github.com/moby/moby/api/types/container"
	"open-sandbox/models"
)

// DockerClient defines the sandbox operations used by the API handlers.
type DockerClient interface {
	List(ctx context.Context, all bool) ([]container.Summary, error)
	Create(ctx context.Context, req models.CreateSandboxRequest) (models.CreateSandboxResponse, error)
	Inspect(ctx context.Context, id string) (container.InspectResponse, error)
	Stop(ctx context.Context, id string) error
	Restart(ctx context.Context, id string) error
	Remove(ctx context.Context, id string) error
	Exec(ctx context.Context, id string, cmd []string) (string, error)
	ReadFile(ctx context.Context, id, path string) (string, error)
	WriteFile(ctx context.Context, id, path, content string) error
	DeleteFile(ctx context.Context, id, path string) error
	ListDir(ctx context.Context, id, path string) (string, error)
}
