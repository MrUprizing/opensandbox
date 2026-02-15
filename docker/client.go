package docker

import (
	"bytes"
	"context"
	"io"
	"sync"

	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	moby "github.com/moby/moby/client"
	"open-sandbox/models"
)

var (
	once     sync.Once
	instance *moby.Client
)

// Client returns the singleton Docker client.
func Client() *moby.Client {
	once.Do(func() {
		c, err := moby.NewClientWithOpts(moby.FromEnv, moby.WithAPIVersionNegotiation())
		if err != nil {
			panic(err)
		}
		instance = c
	})
	return instance
}

// CreateResult holds the result of creating a container.
type CreateResult struct {
	ID    string            `json:"id"`
	Ports map[string]string `json:"ports"` // "80/tcp": "32768"
}

// List returns all containers. Set all=true to include stopped ones.
func List(ctx context.Context, all bool) ([]container.Summary, error) {
	result, err := Client().ContainerList(ctx, moby.ContainerListOptions{All: all})
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

// Create creates and starts a container. Docker assigns host ports automatically.
// Returns the container ID and the assigned host ports.
func Create(ctx context.Context, req models.CreateContainerRequest) (CreateResult, error) {
	cli := Client()

	// Build ExposedPorts so Docker knows which ports the container uses
	exposedPorts := buildExposedPorts(req.Ports)

	cfg := &container.Config{
		Image:        req.Image,
		Env:          req.Env,
		ExposedPorts: exposedPorts,
	}
	if len(req.Cmd) > 0 {
		cfg.Cmd = req.Cmd
	}

	result, err := cli.ContainerCreate(ctx, moby.ContainerCreateOptions{
		Config: cfg,
		HostConfig: &container.HostConfig{
			// PublishAllPorts lets Docker pick available host ports automatically
			PublishAllPorts: true,
		},
		Name: req.Name,
	})
	if err != nil {
		return CreateResult{}, err
	}

	if _, err := cli.ContainerStart(ctx, result.ID, moby.ContainerStartOptions{}); err != nil {
		return CreateResult{}, err
	}

	// Inspect to get the assigned host ports
	info, err := cli.ContainerInspect(ctx, result.ID, moby.ContainerInspectOptions{})
	if err != nil {
		return CreateResult{}, err
	}

	return CreateResult{
		ID:    result.ID,
		Ports: extractPorts(info.Container.NetworkSettings.Ports),
	}, nil
}

// Inspect returns detailed info about a container.
func Inspect(ctx context.Context, id string) (container.InspectResponse, error) {
	result, err := Client().ContainerInspect(ctx, id, moby.ContainerInspectOptions{})
	return result.Container, err
}

// Stop stops a running container.
func Stop(ctx context.Context, id string) error {
	_, err := Client().ContainerStop(ctx, id, moby.ContainerStopOptions{})
	return err
}

// Restart restarts a container.
func Restart(ctx context.Context, id string) error {
	_, err := Client().ContainerRestart(ctx, id, moby.ContainerRestartOptions{})
	return err
}

// Remove removes a container.
func Remove(ctx context.Context, id string) error {
	_, err := Client().ContainerRemove(ctx, id, moby.ContainerRemoveOptions{Force: true})
	return err
}

// Exec runs a command inside a container and returns the combined output.
func Exec(ctx context.Context, id string, cmd []string) (string, error) {
	cli := Client()

	execResult, err := cli.ExecCreate(ctx, id, moby.ExecCreateOptions{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          cmd,
	})
	if err != nil {
		return "", err
	}

	attached, err := cli.ExecAttach(ctx, execResult.ID, moby.ExecAttachOptions{})
	if err != nil {
		return "", err
	}
	defer attached.Close()

	var stdout, stderr bytes.Buffer
	if _, err := stdcopy.StdCopy(&stdout, &stderr, attached.Reader); err != nil && err != io.EOF {
		return "", err
	}

	return stdout.String() + stderr.String(), nil
}

// buildExposedPorts converts ["80/tcp", "443/tcp"] to network.PortSet.
func buildExposedPorts(ports []string) network.PortSet {
	if len(ports) == 0 {
		return nil
	}
	set := make(network.PortSet)
	for _, p := range ports {
		parsed, err := network.ParsePort(p)
		if err != nil {
			continue
		}
		set[parsed] = struct{}{}
	}
	return set
}

// extractPorts converts network.PortMap to a simple map["80/tcp"]"32768".
func extractPorts(pm network.PortMap) map[string]string {
	out := make(map[string]string)
	for port, bindings := range pm {
		if len(bindings) > 0 {
			out[port.String()] = bindings[0].HostPort
		}
	}
	return out
}
