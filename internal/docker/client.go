package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/containerd/errdefs"
	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	moby "github.com/moby/moby/client"
	"open-sandbox/internal/database"
	"open-sandbox/models"
)

// Client wraps the Docker SDK and exposes sandbox operations.
type Client struct {
	cli    *moby.Client
	repo   *database.Repository
	timers sync.Map // map[containerID]*timerEntry
}

// timerEntry holds a timer and a cancel channel to avoid goroutine leaks.
type timerEntry struct {
	timer     *time.Timer
	cancel    chan struct{}
	expiresAt time.Time
}

// defaultTimeout is applied when no timeout is specified (15 minutes).
const defaultTimeout = 900

// Default resource limits (1 vCPU, 1GB RAM)
const (
	defaultMemoryMB = 1024 // 1GB
	defaultCPUs     = 1.0  // 1 vCPU
)

// Maximum resource limits (4 vCPU, 8GB RAM)
const (
	maxMemoryMB = 8192 // 8GB
	maxCPUs     = 4.0  // 4 vCPU
)

var (
	once       sync.Once
	mobyClient *moby.Client
)

// New creates a Docker Client with the given repository.
// The underlying Docker connection is a singleton (created once),
// but each Client gets its own repository.
func New(repo *database.Repository) *Client {
	once.Do(func() {
		cli, err := moby.NewClientWithOpts(moby.FromEnv, moby.WithAPIVersionNegotiation())
		if err != nil {
			panic(err)
		}
		mobyClient = cli
	})
	return &Client{cli: mobyClient, repo: repo}
}

// Ping checks connectivity with the Docker daemon.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.cli.Ping(ctx, moby.PingOptions{})
	return err
}

// List returns all sandboxes tracked in the database, enriched with live
// state from Docker. Stopped containers are always included.
func (c *Client) List(ctx context.Context) ([]models.SandboxSummary, error) {
	// Fetch all persisted sandboxes from the database.
	dbSandboxes, err := c.repo.FindAll()
	if err != nil {
		return nil, err
	}
	if len(dbSandboxes) == 0 {
		return []models.SandboxSummary{}, nil
	}

	// Fetch all containers (including stopped) to build a lookup map.
	result, err := c.cli.ContainerList(ctx, moby.ContainerListOptions{All: true})
	if err != nil {
		return nil, err
	}

	type containerInfo struct {
		Name   string
		Image  string
		Status string
		State  string
		Ports  map[string]string
	}
	lookup := make(map[string]containerInfo, len(result.Items))
	for _, item := range result.Items {
		ports := make(map[string]string)
		for _, p := range item.Ports {
			if p.PublicPort > 0 {
				ports[portKey(p.PrivatePort, p.Type)] = portValue(p.PublicPort)
			}
		}
		lookup[item.ID] = containerInfo{
			Name:   containerName(item.Names),
			Image:  item.Image,
			Status: item.Status,
			State:  string(item.State),
			Ports:  ports,
		}
	}

	summaries := make([]models.SandboxSummary, 0, len(dbSandboxes))
	for _, db := range dbSandboxes {
		s := models.SandboxSummary{
			ID:    db.ID,
			Name:  db.Name,
			Image: db.Image,
			Ports: map[string]string(db.Ports),
		}

		// Enrich with live Docker state if the container still exists.
		if info, ok := lookup[db.ID]; ok {
			s.Name = info.Name
			s.Image = info.Image
			s.Status = info.Status
			s.State = info.State
			if len(info.Ports) > 0 {
				s.Ports = info.Ports
			}
		} else {
			s.Status = "removed"
			s.State = "removed"
		}

		// Attach expiration info if tracked.
		if entry := c.getTimerEntry(db.ID); entry != nil {
			ea := entry.expiresAt
			s.ExpiresAt = &ea
		}

		summaries = append(summaries, s)
	}

	return summaries, nil
}

// Create creates and starts a sandbox. Docker assigns host ports automatically.
// Applies optional resource limits and schedules auto-stop with a default TTL of 15 minutes.
// Returns ErrImageNotFound if the image does not exist locally.
func (c *Client) Create(ctx context.Context, req models.CreateSandboxRequest) (models.CreateSandboxResponse, error) {
	// Verify image exists locally
	exists, err := c.ImageExists(ctx, req.Image)
	if err != nil {
		return models.CreateSandboxResponse{}, err
	}
	if !exists {
		return models.CreateSandboxResponse{}, ErrImageNotFound
	}

	cfg := &container.Config{
		Image:        req.Image,
		Env:          req.Env,
		ExposedPorts: buildExposedPorts(req.Ports),
	}
	if len(req.Cmd) > 0 {
		cfg.Cmd = req.Cmd
	}

	hostCfg := &container.HostConfig{PublishAllPorts: true}

	// Apply resource limits (defaults: 1GB RAM, 1 vCPU)
	memory := int64(defaultMemoryMB)
	cpus := defaultCPUs
	if req.Resources != nil {
		if req.Resources.Memory > 0 {
			memory = req.Resources.Memory
		}
		if req.Resources.CPUs > 0 {
			cpus = req.Resources.CPUs
		}
	}
	hostCfg.Resources = container.Resources{
		Memory:   memory * 1024 * 1024, // MB to bytes
		NanoCPUs: int64(cpus * 1e9),
	}

	result, err := c.cli.ContainerCreate(ctx, moby.ContainerCreateOptions{
		Config:     cfg,
		HostConfig: hostCfg,
		Name:       req.Name,
	})
	if err != nil {
		return models.CreateSandboxResponse{}, err
	}

	if _, err := c.cli.ContainerStart(ctx, result.ID, moby.ContainerStartOptions{}); err != nil {
		return models.CreateSandboxResponse{}, err
	}

	// Schedule auto-stop. Default 15 min if not specified.
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	c.scheduleStop(result.ID, timeout)

	// Inspect to get Docker-assigned host ports.
	info, err := c.cli.ContainerInspect(ctx, result.ID, moby.ContainerInspectOptions{})
	if err != nil {
		return models.CreateSandboxResponse{}, err
	}

	ports := extractPorts(info.Container.NetworkSettings.Ports)

	// Persist sandbox (fire-and-forget: log errors, don't block).
	if err := c.repo.Save(database.Sandbox{
		ID:    result.ID,
		Name:  req.Name,
		Image: req.Image,
		Ports: database.JSONMap(ports),
	}); err != nil {
		log.Printf("database: failed to persist sandbox %s: %v", result.ID, err)
	}

	return models.CreateSandboxResponse{
		ID:    result.ID,
		Ports: ports,
	}, nil
}

// Inspect returns a curated view of a sandbox.
func (c *Client) Inspect(ctx context.Context, id string) (models.SandboxDetail, error) {
	result, err := c.cli.ContainerInspect(ctx, id, moby.ContainerInspectOptions{})
	if err != nil {
		return models.SandboxDetail{}, wrapNotFound(err)
	}

	info := result.Container
	detail := models.SandboxDetail{
		ID:      info.ID,
		Name:    strings.TrimPrefix(info.Name, "/"),
		Image:   info.Config.Image,
		Status:  string(info.State.Status),
		Running: info.State.Running,
		Ports:   extractPorts(info.NetworkSettings.Ports),
		Resources: models.ResourceLimits{
			Memory: info.HostConfig.Memory / (1024 * 1024), // bytes to MB
			CPUs:   float64(info.HostConfig.NanoCPUs) / 1e9,
		},
		StartedAt:  info.State.StartedAt,
		FinishedAt: info.State.FinishedAt,
	}

	if entry := c.getTimerEntry(id); entry != nil {
		ea := entry.expiresAt
		detail.ExpiresAt = &ea
	}

	return detail, nil
}

// Stop stops a running sandbox and cancels its expiration timer.
func (c *Client) Stop(ctx context.Context, id string) error {
	c.cancelTimer(id)
	_, err := c.cli.ContainerStop(ctx, id, moby.ContainerStopOptions{})
	return wrapNotFound(err)
}

// Restart restarts a sandbox and returns the new port mappings.
// It cancels any existing timer and schedules a fresh one with the default timeout.
func (c *Client) Restart(ctx context.Context, id string) (models.RestartResponse, error) {
	c.cancelTimer(id)

	if _, err := c.cli.ContainerRestart(ctx, id, moby.ContainerRestartOptions{}); err != nil {
		return models.RestartResponse{}, wrapNotFound(err)
	}

	// Re-schedule auto-stop with the default timeout.
	c.scheduleStop(id, defaultTimeout)

	// Inspect to get the new ports.
	info, err := c.cli.ContainerInspect(ctx, id, moby.ContainerInspectOptions{})
	if err != nil {
		return models.RestartResponse{}, wrapNotFound(err)
	}

	var expiresAt *time.Time
	if entry := c.getTimerEntry(id); entry != nil {
		ea := entry.expiresAt
		expiresAt = &ea
	}

	ports := extractPorts(info.Container.NetworkSettings.Ports)

	// Update persisted ports after restart (they may change).
	if dbErr := c.repo.UpdatePorts(id, database.JSONMap(ports)); dbErr != nil {
		log.Printf("database: failed to update ports for sandbox %s: %v", id, dbErr)
	}

	return models.RestartResponse{
		Status:    "restarted",
		Ports:     ports,
		ExpiresAt: expiresAt,
	}, nil
}

// Remove removes a sandbox forcefully and cancels its expiration timer.
// If the container no longer exists in Docker, it still cleans up the DB record.
func (c *Client) Remove(ctx context.Context, id string) error {
	c.cancelTimer(id)
	_, err := c.cli.ContainerRemove(ctx, id, moby.ContainerRemoveOptions{Force: true})
	if err != nil && !errdefs.IsNotFound(err) {
		return err
	}

	if dbErr := c.repo.Delete(id); dbErr != nil {
		log.Printf("database: failed to delete sandbox %s: %v", id, dbErr)
	}
	return nil
}

// Pause pauses a running sandbox (freezes all processes).
func (c *Client) Pause(ctx context.Context, id string) error {
	_, err := c.cli.ContainerPause(ctx, id, moby.ContainerPauseOptions{})
	return wrapNotFound(err)
}

// Resume unpauses a paused sandbox.
func (c *Client) Resume(ctx context.Context, id string) error {
	_, err := c.cli.ContainerUnpause(ctx, id, moby.ContainerUnpauseOptions{})
	return wrapNotFound(err)
}

// RenewExpiration resets the auto-stop timer for a sandbox.
func (c *Client) RenewExpiration(ctx context.Context, id string, timeout int) error {
	// Verify the sandbox exists.
	if _, err := c.cli.ContainerInspect(ctx, id, moby.ContainerInspectOptions{}); err != nil {
		return wrapNotFound(err)
	}

	c.cancelTimer(id)
	c.scheduleStop(id, timeout)
	return nil
}

// Logs returns the container log stream. The caller must close the returned ReadCloser.
// The stream is multiplexed (8-byte header per frame) when the container is not using a TTY.
func (c *Client) Logs(ctx context.Context, id string, opts models.LogsOptions) (io.ReadCloser, error) {
	tail := "100"
	if opts.Tail > 0 {
		tail = fmt.Sprintf("%d", opts.Tail)
	} else if opts.Tail == 0 && opts.Follow {
		tail = "0" // follow from now, no history
	}

	rc, err := c.cli.ContainerLogs(ctx, id, moby.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     opts.Follow,
		Timestamps: opts.Timestamps,
		Tail:       tail,
	})
	if err != nil {
		return nil, wrapNotFound(err)
	}
	return rc, nil
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

// PullImage pulls a Docker image from a registry and waits for completion.
// It reads the JSON message stream to detect errors that the Docker daemon
// reports inline (e.g. "no matching manifest for linux/amd64").
func (c *Client) PullImage(ctx context.Context, image string) error {
	resp, err := c.cli.ImagePull(ctx, image, moby.ImagePullOptions{})
	if err != nil {
		return err
	}
	defer resp.Close()

	for msg, err := range resp.JSONMessages(ctx) {
		if err != nil {
			return err
		}
		if msg.Error != nil {
			return fmt.Errorf("pull %s: %s", image, msg.Error.Message)
		}
	}

	// Verify the image actually exists locally after pull.
	if exists, err := c.ImageExists(ctx, image); err != nil {
		return err
	} else if !exists {
		return fmt.Errorf("pull %s: image not available after pull", image)
	}

	return nil
}

// ListImages returns all locally available Docker images.
func (c *Client) ListImages(ctx context.Context) ([]models.ImageSummary, error) {
	result, err := c.cli.ImageList(ctx, moby.ImageListOptions{})
	if err != nil {
		return nil, err
	}

	images := make([]models.ImageSummary, 0, len(result.Items))
	for _, item := range result.Items {
		images = append(images, models.ImageSummary{
			ID:   item.ID,
			Tags: item.RepoTags,
			Size: item.Size,
		})
	}
	return images, nil
}

// ImageExists checks if an image exists locally.
func (c *Client) ImageExists(ctx context.Context, image string) (bool, error) {
	_, err := c.cli.ImageInspect(ctx, image)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Shutdown cancels all pending timers and stops tracked containers.
// Called during graceful shutdown to prevent orphaned containers.
func (c *Client) Shutdown(ctx context.Context) {
	c.timers.Range(func(key, value any) bool {
		id := key.(string)
		entry := value.(*timerEntry)
		entry.timer.Stop()
		close(entry.cancel)
		c.timers.Delete(id)
		c.cli.ContainerStop(ctx, id, moby.ContainerStopOptions{})
		return true
	})
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
		return "", wrapNotFound(err)
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

// scheduleStop creates a timer that auto-stops the sandbox after the given seconds.
// Uses a cancel channel so cancelTimer can cleanly terminate the goroutine.
func (c *Client) scheduleStop(id string, seconds int) {
	d := time.Duration(seconds) * time.Second
	timer := time.NewTimer(d)
	cancel := make(chan struct{})

	c.timers.Store(id, &timerEntry{
		timer:     timer,
		cancel:    cancel,
		expiresAt: time.Now().Add(d),
	})

	go func() {
		select {
		case <-timer.C:
			c.timers.Delete(id)
			c.cli.ContainerStop(context.Background(), id, moby.ContainerStopOptions{})
		case <-cancel:
			// Timer was cancelled; stop it and drain the channel if needed.
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
		}
	}()
}

// cancelTimer stops and removes the expiration timer for a sandbox.
func (c *Client) cancelTimer(id string) {
	if v, ok := c.timers.LoadAndDelete(id); ok {
		entry := v.(*timerEntry)
		close(entry.cancel)
	}
}

// getTimerEntry returns the timer entry for a sandbox, or nil if not tracked.
func (c *Client) getTimerEntry(id string) *timerEntry {
	if v, ok := c.timers.Load(id); ok {
		return v.(*timerEntry)
	}
	return nil
}

// wrapNotFound converts Docker "not found" errors to ErrNotFound.
func wrapNotFound(err error) error {
	if err == nil {
		return nil
	}
	if errdefs.IsNotFound(err) {
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

// containerName extracts a clean name from Docker's name list (removes leading /).
func containerName(names []string) string {
	if len(names) == 0 {
		return ""
	}
	return strings.TrimPrefix(names[0], "/")
}

// portKey builds a port key like "3000/tcp".
func portKey(port uint16, proto string) string {
	if proto == "" {
		proto = "tcp"
	}
	return portValue(port) + "/" + proto
}

// portValue converts a uint16 port to its string representation.
func portValue(port uint16) string {
	return fmt.Sprintf("%d", port)
}
