package api

import (
	"context"

	"open-sandbox/models"
)

// DockerClient defines the sandbox operations used by the API handlers.
type DockerClient interface {
	Ping(ctx context.Context) error
	List(ctx context.Context) ([]models.SandboxSummary, error)
	Create(ctx context.Context, req models.CreateSandboxRequest) (models.CreateSandboxResponse, error)
	Inspect(ctx context.Context, id string) (models.SandboxDetail, error)
	Stop(ctx context.Context, id string) error
	Restart(ctx context.Context, id string) (models.RestartResponse, error)
	Remove(ctx context.Context, id string) error
	Pause(ctx context.Context, id string) error
	Resume(ctx context.Context, id string) error
	RenewExpiration(ctx context.Context, id string, timeout int) error
	Exec(ctx context.Context, id string, cmd []string) (string, error)
	ReadFile(ctx context.Context, id, path string) (string, error)
	WriteFile(ctx context.Context, id, path, content string) error
	DeleteFile(ctx context.Context, id, path string) error
	ListDir(ctx context.Context, id, path string) (string, error)
	PullImage(ctx context.Context, image string) error
	ListImages(ctx context.Context) ([]models.ImageSummary, error)
}
