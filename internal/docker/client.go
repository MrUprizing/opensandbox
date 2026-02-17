package docker

import (
	"bytes"
	"context"
	"io"
	"strings"
	"sync"

	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	moby "github.com/moby/moby/client"
	"open-sandbox/models"
)

// Client wraps the Docker SDK and exposes sandbox operations.
type Client struct {
	cli *moby.Client
}

var (
	once     sync.Once
	instance *Client
)

// New returns the singleton Docker Client.
// Panics on connection failure (unrecoverable at startup).
func New() *Client {
	once.Do(func() {
		cli, err := moby.NewClientWithOpts(moby.FromEnv, moby.WithAPIVersionNegotiation())
		if err != nil {
			panic(err)
		}
		instance = &Client{cli: cli}
	})
	return instance
}

// List returns all sandboxes. Set all=true to include stopped ones.
func (c *Client) List(ctx context.Context, all bool) ([]container.Summary, error) {
	result, err := c.cli.ContainerList(ctx, moby.ContainerListOptions{All: all})
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

// Create creates and starts a sandbox. Docker assigns host ports automatically.
func (c *Client) Create(ctx context.Context, req models.CreateSandboxRequest) (models.CreateSandboxResponse, error) {
	cfg := &container.Config{
		Image:        req.Image,
		Env:          req.Env,
		ExposedPorts: buildExposedPorts(req.Ports),
	}
	if len(req.Cmd) > 0 {
		cfg.Cmd = req.Cmd
	}

	result, err := c.cli.ContainerCreate(ctx, moby.ContainerCreateOptions{
		Config:     cfg,
		HostConfig: &container.HostConfig{PublishAllPorts: true},
		Name:       req.Name,
	})
	if err != nil {
		return models.CreateSandboxResponse{}, err
	}

	if _, err := c.cli.ContainerStart(ctx, result.ID, moby.ContainerStartOptions{}); err != nil {
		return models.CreateSandboxResponse{}, err
	}

	// Inspect to get Docker-assigned host ports.
	info, err := c.cli.ContainerInspect(ctx, result.ID, moby.ContainerInspectOptions{})
	if err != nil {
		return models.CreateSandboxResponse{}, err
	}

	return models.CreateSandboxResponse{
		ID:    result.ID,
		Ports: extractPorts(info.Container.NetworkSettings.Ports),
	}, nil
}

// Inspect returns detailed info about a sandbox.
func (c *Client) Inspect(ctx context.Context, id string) (container.InspectResponse, error) {
	result, err := c.cli.ContainerInspect(ctx, id, moby.ContainerInspectOptions{})
	if err != nil {
		return container.InspectResponse{}, wrapNotFound(err)
	}
	return result.Container, nil
}

// Stop stops a running sandbox.
func (c *Client) Stop(ctx context.Context, id string) error {
	_, err := c.cli.ContainerStop(ctx, id, moby.ContainerStopOptions{})
	return wrapNotFound(err)
}

// Restart restarts a sandbox.
func (c *Client) Restart(ctx context.Context, id string) error {
	_, err := c.cli.ContainerRestart(ctx, id, moby.ContainerRestartOptions{})
	return wrapNotFound(err)
}

// Remove removes a sandbox forcefully.
func (c *Client) Remove(ctx context.Context, id string) error {
	_, err := c.cli.ContainerRemove(ctx, id, moby.ContainerRemoveOptions{Force: true})
	return wrapNotFound(err)
}

// Exec runs a command inside a sandbox and returns combined stdout+stderr.
func (c *Client) Exec(ctx context.Context, id string, cmd []string) (string, error) {
	return c.execWithStdin(ctx, id, cmd, nil)
}

// ReadFile reads the content of a file inside a sandbox.
func (c *Client) ReadFile(ctx context.Context, id, path string) (string, error) {
	return c.Exec(ctx, id, []string{"cat", path})
}

// WriteFile writes content to a file inside a sandbox (creates parent dirs as needed).
func (c *Client) WriteFile(ctx context.Context, id, path, content string) error {
	if _, err := c.Exec(ctx, id, []string{"sh", "-c", "mkdir -p $(dirname '" + path + "')"}); err != nil {
		return err
	}
	_, err := c.execWithStdin(ctx, id, []string{"sh", "-c", "cat > '" + path + "'"}, strings.NewReader(content))
	return err
}

// DeleteFile deletes a file or directory inside a sandbox.
func (c *Client) DeleteFile(ctx context.Context, id, path string) error {
	_, err := c.Exec(ctx, id, []string{"rm", "-rf", path})
	return err
}

// ListDir lists the contents of a directory inside a sandbox.
func (c *Client) ListDir(ctx context.Context, id, path string) (string, error) {
	return c.Exec(ctx, id, []string{"ls", "-la", path})
}

// execWithStdin runs a command with optional stdin.
func (c *Client) execWithStdin(ctx context.Context, id string, cmd []string, stdin io.Reader) (string, error) {
	attachStdin := stdin != nil
	execResult, err := c.cli.ExecCreate(ctx, id, moby.ExecCreateOptions{
		AttachStdin:  attachStdin,
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          cmd,
	})
	if err != nil {
		return "", err
	}

	attached, err := c.cli.ExecAttach(ctx, execResult.ID, moby.ExecAttachOptions{})
	if err != nil {
		return "", err
	}
	defer attached.Close()

	if stdin != nil {
		if _, err := io.Copy(attached.Conn, stdin); err != nil {
			return "", err
		}
		attached.CloseWrite()
	}

	var stdout, stderr bytes.Buffer
	if _, err := stdcopy.StdCopy(&stdout, &stderr, attached.Reader); err != nil && err != io.EOF {
		return "", err
	}

	return stdout.String() + stderr.String(), nil
}

// wrapNotFound converts Docker "No such container" errors to ErrNotFound.
func wrapNotFound(err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "No such container") {
		return ErrNotFound
	}
	return err
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

// extractPorts converts network.PortMap to map["80/tcp"]"32768".
func extractPorts(pm network.PortMap) map[string]string {
	out := make(map[string]string)
	for port, bindings := range pm {
		if len(bindings) > 0 {
			out[port.String()] = bindings[0].HostPort
		}
	}
	return out
}
