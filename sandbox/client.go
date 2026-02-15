package sandbox

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

// client returns the singleton Docker client.
func client() *moby.Client {
	once.Do(func() {
		c, err := moby.NewClientWithOpts(moby.FromEnv, moby.WithAPIVersionNegotiation())
		if err != nil {
			panic(err)
		}
		instance = c
	})
	return instance
}

// List returns all sandboxes. Set all=true to include stopped ones.
func List(ctx context.Context, all bool) ([]container.Summary, error) {
	result, err := client().ContainerList(ctx, moby.ContainerListOptions{All: all})
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

// Create creates and starts a sandbox. Docker assigns host ports automatically.
func Create(ctx context.Context, req models.CreateSandboxRequest) (models.CreateSandboxResponse, error) {
	cli := client()

	cfg := &container.Config{
		Image:        req.Image,
		Env:          req.Env,
		ExposedPorts: buildExposedPorts(req.Ports),
	}
	if len(req.Cmd) > 0 {
		cfg.Cmd = req.Cmd
	}

	result, err := cli.ContainerCreate(ctx, moby.ContainerCreateOptions{
		Config: cfg,
		HostConfig: &container.HostConfig{
			PublishAllPorts: true,
		},
		Name: req.Name,
	})
	if err != nil {
		return models.CreateSandboxResponse{}, err
	}

	if _, err := cli.ContainerStart(ctx, result.ID, moby.ContainerStartOptions{}); err != nil {
		return models.CreateSandboxResponse{}, err
	}

	// Inspect to get Docker-assigned host ports
	info, err := cli.ContainerInspect(ctx, result.ID, moby.ContainerInspectOptions{})
	if err != nil {
		return models.CreateSandboxResponse{}, err
	}

	return models.CreateSandboxResponse{
		ID:    result.ID,
		Ports: extractPorts(info.Container.NetworkSettings.Ports),
	}, nil
}

// Inspect returns detailed info about a sandbox.
func Inspect(ctx context.Context, id string) (container.InspectResponse, error) {
	result, err := client().ContainerInspect(ctx, id, moby.ContainerInspectOptions{})
	return result.Container, err
}

// Stop stops a running sandbox.
func Stop(ctx context.Context, id string) error {
	_, err := client().ContainerStop(ctx, id, moby.ContainerStopOptions{})
	return err
}

// Restart restarts a sandbox.
func Restart(ctx context.Context, id string) error {
	_, err := client().ContainerRestart(ctx, id, moby.ContainerRestartOptions{})
	return err
}

// Remove removes a sandbox forcefully.
func Remove(ctx context.Context, id string) error {
	_, err := client().ContainerRemove(ctx, id, moby.ContainerRemoveOptions{Force: true})
	return err
}

// Exec runs a command inside a sandbox and returns the combined stdout+stderr.
func Exec(ctx context.Context, id string, cmd []string) (string, error) {
	cli := client()

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
