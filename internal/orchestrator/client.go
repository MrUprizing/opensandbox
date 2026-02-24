package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"

	"open-sandbox/internal/database"
	"open-sandbox/internal/docker"
	"open-sandbox/models"
)

// RemoteDockerClient implements the DockerClient interface by forwarding
// operations to worker nodes over HTTP.
type RemoteDockerClient struct {
	registry *WorkerRegistry
	repo     *database.Repository
	http     *http.Client
}

// NewRemoteClient creates a RemoteDockerClient.
func NewRemoteClient(registry *WorkerRegistry, repo *database.Repository) *RemoteDockerClient {
	return &RemoteDockerClient{
		registry: registry,
		repo:     repo,
		http:     &http.Client{},
	}
}

// --- Helpers ---

// workerForSandbox looks up which worker owns a sandbox.
func (c *RemoteDockerClient) workerForSandbox(sandboxID string) (WorkerInfo, error) {
	sb, err := c.repo.FindByID(sandboxID)
	if err != nil {
		return WorkerInfo{}, err
	}
	if sb == nil {
		return WorkerInfo{}, docker.ErrNotFound
	}
	if sb.WorkerID == "" {
		return WorkerInfo{}, fmt.Errorf("sandbox %s has no worker assigned", sandboxID)
	}
	return c.registry.Lookup(sb.WorkerID)
}

// doJSON sends a JSON request to a worker and decodes the response.
func (c *RemoteDockerClient) doJSON(ctx context.Context, w WorkerInfo, method, path string, body any, out any) error {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, w.URL+"/internal/v1"+path, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("X-Worker-Key", w.APIKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("worker %s: %w", w.ID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return c.mapWorkerError(resp)
	}

	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

// doNoContent sends a request expecting 204 No Content.
func (c *RemoteDockerClient) doNoContent(ctx context.Context, w WorkerInfo, method, path string, body any) error {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, w.URL+"/internal/v1"+path, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("X-Worker-Key", w.APIKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("worker %s: %w", w.ID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return c.mapWorkerError(resp)
	}
	return nil
}

// doStream opens a streaming connection to a worker and returns the response body.
func (c *RemoteDockerClient) doStream(ctx context.Context, w WorkerInfo, method, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, w.URL+"/internal/v1"+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Worker-Key", w.APIKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("worker %s: %w", w.ID, err)
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		return nil, c.mapWorkerError(resp)
	}
	return resp, nil
}

// mapWorkerError converts a worker error response to a sentinel error.
func (c *RemoteDockerClient) mapWorkerError(resp *http.Response) error {
	var errResp struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	json.NewDecoder(resp.Body).Decode(&errResp)

	switch resp.StatusCode {
	case http.StatusNotFound:
		if errResp.Message == "command not found" {
			return docker.ErrCommandNotFound
		}
		return docker.ErrNotFound
	case http.StatusConflict:
		// Map back to sentinel errors based on message.
		switch errResp.Message {
		case docker.ErrAlreadyRunning.Error():
			return docker.ErrAlreadyRunning
		case docker.ErrAlreadyStopped.Error():
			return docker.ErrAlreadyStopped
		case docker.ErrAlreadyPaused.Error():
			return docker.ErrAlreadyPaused
		case docker.ErrNotPaused.Error():
			return docker.ErrNotPaused
		case docker.ErrNotRunning.Error():
			return docker.ErrNotRunning
		case docker.ErrCommandFinished.Error():
			return docker.ErrCommandFinished
		}
		return fmt.Errorf("conflict: %s", errResp.Message)
	case http.StatusBadRequest:
		if errResp.Message == "image not found locally" {
			return docker.ErrImageNotFound
		}
		return fmt.Errorf("bad request: %s", errResp.Message)
	default:
		return fmt.Errorf("worker error %d: %s", resp.StatusCode, errResp.Message)
	}
}

// --- DockerClient implementation ---

func (c *RemoteDockerClient) Ping(ctx context.Context) error {
	workers := c.registry.All()
	if len(workers) == 0 {
		return ErrNoWorkers
	}
	// Ping first worker as health indicator.
	return c.doJSON(ctx, workers[0], http.MethodGet, "/health", nil, nil)
}

func (c *RemoteDockerClient) List(ctx context.Context) ([]models.SandboxSummary, error) {
	workers := c.registry.All()
	if len(workers) == 0 {
		return []models.SandboxSummary{}, nil
	}

	type result struct {
		items []models.SandboxSummary
		err   error
	}

	ch := make(chan result, len(workers))
	for _, w := range workers {
		go func(w WorkerInfo) {
			var resp struct {
				Sandboxes []models.SandboxSummary `json:"sandboxes"`
			}
			err := c.doJSON(ctx, w, http.MethodGet, "/sandboxes", nil, &resp)
			ch <- result{items: resp.Sandboxes, err: err}
		}(w)
	}

	var all []models.SandboxSummary
	for range workers {
		r := <-ch
		if r.err != nil {
			log.Printf("orchestrator: list from worker failed: %v", r.err)
			continue
		}
		all = append(all, r.items...)
	}
	return all, nil
}

func (c *RemoteDockerClient) Create(ctx context.Context, req models.CreateSandboxRequest) (models.CreateSandboxResponse, error) {
	// Pick a worker via round-robin.
	w, err := c.registry.Next()
	if err != nil {
		return models.CreateSandboxResponse{}, err
	}

	// Pre-generate a unique name.
	name := docker.GenerateUniqueName(func(n string) bool {
		sb, _ := c.repo.FindByName(n)
		return sb != nil
	})
	req.Name = name

	var resp models.CreateSandboxResponse
	if err := c.doJSON(ctx, w, http.MethodPost, "/sandboxes", req, &resp); err != nil {
		return models.CreateSandboxResponse{}, err
	}

	// Persist sandbox in orchestrator DB with worker_id.
	if err := c.repo.Save(database.Sandbox{
		ID:       resp.ID,
		Name:     resp.Name,
		Image:    req.Image,
		Ports:    database.JSONMap(portsToMap(resp.Ports)),
		Port:     mainPort(req.Ports),
		WorkerID: w.ID,
	}); err != nil {
		log.Printf("orchestrator: failed to persist sandbox %s: %v", resp.ID, err)
	}

	return resp, nil
}

func (c *RemoteDockerClient) Inspect(ctx context.Context, id string) (models.SandboxDetail, error) {
	w, err := c.workerForSandbox(id)
	if err != nil {
		return models.SandboxDetail{}, err
	}
	var resp models.SandboxDetail
	if err := c.doJSON(ctx, w, http.MethodGet, "/sandboxes/"+id, nil, &resp); err != nil {
		return models.SandboxDetail{}, err
	}
	return resp, nil
}

func (c *RemoteDockerClient) Start(ctx context.Context, id string) (models.RestartResponse, error) {
	w, err := c.workerForSandbox(id)
	if err != nil {
		return models.RestartResponse{}, err
	}
	var resp models.RestartResponse
	if err := c.doJSON(ctx, w, http.MethodPost, "/sandboxes/"+id+"/start", nil, &resp); err != nil {
		return models.RestartResponse{}, err
	}
	// Update ports in DB.
	c.repo.UpdatePorts(id, database.JSONMap(portsToMap(resp.Ports)))
	return resp, nil
}

func (c *RemoteDockerClient) Stop(ctx context.Context, id string) error {
	w, err := c.workerForSandbox(id)
	if err != nil {
		return err
	}
	return c.doJSON(ctx, w, http.MethodPost, "/sandboxes/"+id+"/stop", nil, nil)
}

func (c *RemoteDockerClient) Restart(ctx context.Context, id string) (models.RestartResponse, error) {
	w, err := c.workerForSandbox(id)
	if err != nil {
		return models.RestartResponse{}, err
	}
	var resp models.RestartResponse
	if err := c.doJSON(ctx, w, http.MethodPost, "/sandboxes/"+id+"/restart", nil, &resp); err != nil {
		return models.RestartResponse{}, err
	}
	// Update ports in DB.
	c.repo.UpdatePorts(id, database.JSONMap(portsToMap(resp.Ports)))
	return resp, nil
}

func (c *RemoteDockerClient) Remove(ctx context.Context, id string) error {
	w, err := c.workerForSandbox(id)
	if err != nil {
		return err
	}
	if err := c.doNoContent(ctx, w, http.MethodDelete, "/sandboxes/"+id, nil); err != nil {
		return err
	}
	// Clean up DB.
	c.repo.DeleteCommandsBySandbox(id)
	c.repo.Delete(id)
	return nil
}

func (c *RemoteDockerClient) Pause(ctx context.Context, id string) error {
	w, err := c.workerForSandbox(id)
	if err != nil {
		return err
	}
	return c.doJSON(ctx, w, http.MethodPost, "/sandboxes/"+id+"/pause", nil, nil)
}

func (c *RemoteDockerClient) Resume(ctx context.Context, id string) error {
	w, err := c.workerForSandbox(id)
	if err != nil {
		return err
	}
	return c.doJSON(ctx, w, http.MethodPost, "/sandboxes/"+id+"/resume", nil, nil)
}

func (c *RemoteDockerClient) RenewExpiration(ctx context.Context, id string, timeout int) error {
	w, err := c.workerForSandbox(id)
	if err != nil {
		return err
	}
	return c.doJSON(ctx, w, http.MethodPost, "/sandboxes/"+id+"/renew-expiration",
		models.RenewExpirationRequest{Timeout: timeout}, nil)
}

func (c *RemoteDockerClient) Stats(ctx context.Context, id string) (models.SandboxStats, error) {
	w, err := c.workerForSandbox(id)
	if err != nil {
		return models.SandboxStats{}, err
	}
	var resp models.SandboxStats
	if err := c.doJSON(ctx, w, http.MethodGet, "/sandboxes/"+id+"/stats", nil, &resp); err != nil {
		return models.SandboxStats{}, err
	}
	return resp, nil
}

// --- Commands ---

func (c *RemoteDockerClient) ExecCommand(ctx context.Context, sandboxID string, req models.ExecCommandRequest) (models.CommandDetail, error) {
	w, err := c.workerForSandbox(sandboxID)
	if err != nil {
		return models.CommandDetail{}, err
	}
	var resp models.CommandResponse
	if err := c.doJSON(ctx, w, http.MethodPost, "/sandboxes/"+sandboxID+"/cmd", req, &resp); err != nil {
		return models.CommandDetail{}, err
	}
	return resp.Command, nil
}

func (c *RemoteDockerClient) GetCommand(ctx context.Context, sandboxID, cmdID string) (models.CommandDetail, error) {
	w, err := c.workerForSandbox(sandboxID)
	if err != nil {
		return models.CommandDetail{}, err
	}
	var resp models.CommandResponse
	if err := c.doJSON(ctx, w, http.MethodGet, "/sandboxes/"+sandboxID+"/cmd/"+cmdID, nil, &resp); err != nil {
		return models.CommandDetail{}, err
	}
	return resp.Command, nil
}

func (c *RemoteDockerClient) ListCommands(ctx context.Context, sandboxID string) ([]models.CommandDetail, error) {
	w, err := c.workerForSandbox(sandboxID)
	if err != nil {
		return nil, err
	}
	var resp models.CommandListResponse
	if err := c.doJSON(ctx, w, http.MethodGet, "/sandboxes/"+sandboxID+"/cmd", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Commands, nil
}

func (c *RemoteDockerClient) KillCommand(ctx context.Context, sandboxID, cmdID string, signal int) (models.CommandDetail, error) {
	w, err := c.workerForSandbox(sandboxID)
	if err != nil {
		return models.CommandDetail{}, err
	}
	var resp models.CommandResponse
	if err := c.doJSON(ctx, w, http.MethodPost, "/sandboxes/"+sandboxID+"/cmd/"+cmdID+"/kill",
		models.KillCommandRequest{Signal: signal}, &resp); err != nil {
		return models.CommandDetail{}, err
	}
	return resp.Command, nil
}

func (c *RemoteDockerClient) StreamCommandLogs(ctx context.Context, sandboxID, cmdID string) (io.ReadCloser, io.ReadCloser, error) {
	w, err := c.workerForSandbox(sandboxID)
	if err != nil {
		return nil, nil, err
	}
	resp, err := c.doStream(ctx, w, http.MethodGet, "/sandboxes/"+sandboxID+"/cmd/"+cmdID+"/logs?stream=true")
	if err != nil {
		return nil, nil, err
	}
	// Worker streams interleaved ND-JSON. Return the body as a single reader for both.
	// The public API handler will pipe it directly to the client.
	return resp.Body, io.NopCloser(bytes.NewReader(nil)), nil
}

func (c *RemoteDockerClient) GetCommandLogs(ctx context.Context, sandboxID, cmdID string) (models.CommandLogsResponse, error) {
	w, err := c.workerForSandbox(sandboxID)
	if err != nil {
		return models.CommandLogsResponse{}, err
	}
	var resp models.CommandLogsResponse
	if err := c.doJSON(ctx, w, http.MethodGet, "/sandboxes/"+sandboxID+"/cmd/"+cmdID+"/logs", nil, &resp); err != nil {
		return models.CommandLogsResponse{}, err
	}
	return resp, nil
}

func (c *RemoteDockerClient) WaitCommand(ctx context.Context, sandboxID, cmdID string) (models.CommandDetail, error) {
	w, err := c.workerForSandbox(sandboxID)
	if err != nil {
		return models.CommandDetail{}, err
	}
	// Use ?wait=true to block on worker side.
	resp, err := c.doStream(ctx, w, http.MethodGet, "/sandboxes/"+sandboxID+"/cmd/"+cmdID+"?wait=true")
	if err != nil {
		return models.CommandDetail{}, err
	}
	defer resp.Body.Close()

	// Read ND-JSON lines; the last one has the final state.
	var last models.CommandResponse
	dec := json.NewDecoder(resp.Body)
	for dec.More() {
		if err := dec.Decode(&last); err != nil {
			break
		}
	}
	return last.Command, nil
}

// --- Files ---

func (c *RemoteDockerClient) ReadFile(ctx context.Context, id, path string) (string, error) {
	w, err := c.workerForSandbox(id)
	if err != nil {
		return "", err
	}
	var resp models.FileReadResponse
	if err := c.doJSON(ctx, w, http.MethodGet, "/sandboxes/"+id+"/files?path="+url.QueryEscape(path), nil, &resp); err != nil {
		return "", err
	}
	return resp.Content, nil
}

func (c *RemoteDockerClient) WriteFile(ctx context.Context, id, path, content string) error {
	w, err := c.workerForSandbox(id)
	if err != nil {
		return err
	}
	return c.doJSON(ctx, w, http.MethodPut, "/sandboxes/"+id+"/files?path="+url.QueryEscape(path),
		models.FileWriteRequest{Content: content}, nil)
}

func (c *RemoteDockerClient) DeleteFile(ctx context.Context, id, path string) error {
	w, err := c.workerForSandbox(id)
	if err != nil {
		return err
	}
	return c.doNoContent(ctx, w, http.MethodDelete, "/sandboxes/"+id+"/files?path="+url.QueryEscape(path), nil)
}

func (c *RemoteDockerClient) ListDir(ctx context.Context, id, path string) (string, error) {
	w, err := c.workerForSandbox(id)
	if err != nil {
		return "", err
	}
	var resp models.FileListResponse
	if err := c.doJSON(ctx, w, http.MethodGet, "/sandboxes/"+id+"/files/list?path="+url.QueryEscape(path), nil, &resp); err != nil {
		return "", err
	}
	return resp.Output, nil
}

// --- Images ---

func (c *RemoteDockerClient) PullImage(ctx context.Context, image string) error {
	workers := c.registry.All()
	if len(workers) == 0 {
		return ErrNoWorkers
	}

	// Pull on all workers in parallel. Collect per-worker errors.
	type pullResult struct {
		workerID string
		err      error
	}
	ch := make(chan pullResult, len(workers))
	for _, w := range workers {
		go func(w WorkerInfo) {
			err := c.doJSON(ctx, w, http.MethodPost, "/images/pull",
				models.ImagePullRequest{Image: image}, nil)
			ch <- pullResult{workerID: w.ID, err: err}
		}(w)
	}

	var firstErr error
	var failed int
	for range workers {
		r := <-ch
		if r.err != nil {
			failed++
			log.Printf("orchestrator: pull %s on worker %s failed: %v", image, r.workerID, r.err)
			if firstErr == nil {
				firstErr = r.err
			}
		}
	}

	// Fail only if ALL workers failed.
	if failed == len(workers) {
		return firstErr
	}
	if failed > 0 {
		log.Printf("orchestrator: pull %s succeeded on %d/%d workers", image, len(workers)-failed, len(workers))
	}
	return nil
}

func (c *RemoteDockerClient) RemoveImage(ctx context.Context, id string, force bool) error {
	workers := c.registry.All()
	if len(workers) == 0 {
		return ErrNoWorkers
	}

	path := "/images/" + url.PathEscape(id)
	if force {
		path += "?force=true"
	}

	type removeResult struct {
		workerID string
		err      error
	}
	ch := make(chan removeResult, len(workers))
	for _, w := range workers {
		go func(w WorkerInfo) {
			err := c.doNoContent(ctx, w, http.MethodDelete, path, nil)
			ch <- removeResult{workerID: w.ID, err: err}
		}(w)
	}

	var firstErr error
	var failed int
	for range workers {
		r := <-ch
		if r.err != nil {
			failed++
			log.Printf("orchestrator: remove image %s on worker %s failed: %v", id, r.workerID, r.err)
			if firstErr == nil {
				firstErr = r.err
			}
		}
	}

	if failed == len(workers) {
		return firstErr
	}
	return nil
}

func (c *RemoteDockerClient) InspectImage(ctx context.Context, id string) (models.ImageDetail, error) {
	workers := c.registry.All()
	path := "/images/" + url.PathEscape(id)
	for _, w := range workers {
		var resp models.ImageDetail
		if err := c.doJSON(ctx, w, http.MethodGet, path, nil, &resp); err == nil {
			return resp, nil
		}
	}
	return models.ImageDetail{}, docker.ErrImageNotFound
}

// ListImages returns images from all workers (or a single worker if workerID is specified).
// Results are merged and deduplicated by image ID.
func (c *RemoteDockerClient) ListImages(ctx context.Context) ([]models.ImageSummary, error) {
	return c.ListImagesFromWorkers(ctx, "")
}

// ListImagesFromWorkers lists images from a specific worker or all workers.
func (c *RemoteDockerClient) ListImagesFromWorkers(ctx context.Context, workerID string) ([]models.ImageSummary, error) {
	var targets []WorkerInfo
	if workerID != "" {
		w, err := c.registry.Lookup(workerID)
		if err != nil {
			return nil, err
		}
		targets = []WorkerInfo{w}
	} else {
		targets = c.registry.All()
	}

	if len(targets) == 0 {
		return []models.ImageSummary{}, nil
	}

	type result struct {
		items []models.ImageSummary
		err   error
	}
	ch := make(chan result, len(targets))
	for _, w := range targets {
		go func(w WorkerInfo) {
			var resp struct {
				Images []models.ImageSummary `json:"images"`
			}
			err := c.doJSON(ctx, w, http.MethodGet, "/images", nil, &resp)
			ch <- result{items: resp.Images, err: err}
		}(w)
	}

	// Merge and deduplicate by ID.
	seen := make(map[string]bool)
	var all []models.ImageSummary
	for range targets {
		r := <-ch
		if r.err != nil {
			log.Printf("orchestrator: list images from worker failed: %v", r.err)
			continue
		}
		for _, img := range r.items {
			if !seen[img.ID] {
				seen[img.ID] = true
				all = append(all, img)
			}
		}
	}
	return all, nil
}

// --- Helpers ---

// portsToMap converts ["3000/tcp", "8080/tcp"] to a map for DB storage.
// The orchestrator doesn't know host ports at this point; just stores the keys.
func portsToMap(ports []string) map[string]string {
	m := make(map[string]string, len(ports))
	for _, p := range ports {
		m[p] = ""
	}
	return m
}

// mainPort returns the first port from a list, or empty.
func mainPort(ports []string) string {
	if len(ports) > 0 {
		return ports[0]
	}
	return ""
}
