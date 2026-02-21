package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"open-sandbox/internal/api"
	"open-sandbox/internal/docker"
	"open-sandbox/models"
)

func init() { gin.SetMode(gin.TestMode) }

// stub implements api.DockerClient for testing without a real Docker daemon.
// Each field is an optional function — set only what the test needs, leave the rest nil.
// If a nil method is called unexpectedly the test will panic, making the gap obvious.
type stub struct {
	ping            func() error
	list            func() ([]models.SandboxSummary, error)
	create          func(models.CreateSandboxRequest) (models.CreateSandboxResponse, error)
	inspect         func(string) (models.SandboxDetail, error)
	stop            func(string) error
	restart         func(string) (models.RestartResponse, error)
	remove          func(string) error
	pause           func(string) error
	resume          func(string) error
	renewExpiration func(string, int) error
	exec            func(string, []string) (string, error)
	logs            func(string, models.LogsOptions) (io.ReadCloser, error)
	stats           func(string) (models.SandboxStats, error)
	readFile        func(string, string) (string, error)
	writeFile       func(string, string, string) error
	deleteFile      func(string, string) error
	listDir         func(string, string) (string, error)
	pullImage       func(string) error
	listImages      func() ([]models.ImageSummary, error)
}

func (s *stub) Ping(_ context.Context) error {
	if s.ping != nil {
		return s.ping()
	}
	return nil
}
func (s *stub) List(_ context.Context) ([]models.SandboxSummary, error) {
	return s.list()
}
func (s *stub) Create(_ context.Context, r models.CreateSandboxRequest) (models.CreateSandboxResponse, error) {
	return s.create(r)
}
func (s *stub) Inspect(_ context.Context, id string) (models.SandboxDetail, error) {
	return s.inspect(id)
}
func (s *stub) Stop(_ context.Context, id string) error { return s.stop(id) }
func (s *stub) Restart(_ context.Context, id string) (models.RestartResponse, error) {
	return s.restart(id)
}
func (s *stub) Remove(_ context.Context, id string) error { return s.remove(id) }
func (s *stub) Pause(_ context.Context, id string) error  { return s.pause(id) }
func (s *stub) Resume(_ context.Context, id string) error { return s.resume(id) }
func (s *stub) RenewExpiration(_ context.Context, id string, timeout int) error {
	return s.renewExpiration(id, timeout)
}
func (s *stub) Exec(_ context.Context, id string, cmd []string) (string, error) {
	return s.exec(id, cmd)
}
func (s *stub) Logs(_ context.Context, id string, opts models.LogsOptions) (io.ReadCloser, error) {
	if s.logs != nil {
		return s.logs(id, opts)
	}
	return io.NopCloser(bytes.NewReader(nil)), nil
}
func (s *stub) Stats(_ context.Context, id string) (models.SandboxStats, error) {
	if s.stats != nil {
		return s.stats(id)
	}
	return models.SandboxStats{}, nil
}
func (s *stub) ReadFile(_ context.Context, id, path string) (string, error) {
	return s.readFile(id, path)
}
func (s *stub) WriteFile(_ context.Context, id, path, content string) error {
	return s.writeFile(id, path, content)
}
func (s *stub) DeleteFile(_ context.Context, id, path string) error { return s.deleteFile(id, path) }
func (s *stub) ListDir(_ context.Context, id, path string) (string, error) {
	return s.listDir(id, path)
}
func (s *stub) PullImage(_ context.Context, image string) error {
	if s.pullImage != nil {
		return s.pullImage(image)
	}
	return nil
}
func (s *stub) ListImages(_ context.Context) ([]models.ImageSummary, error) {
	if s.listImages != nil {
		return s.listImages()
	}
	return []models.ImageSummary{}, nil
}

// newRouter builds a Gin engine with all sandbox routes registered for the given client.
func newRouter(d api.DockerClient) *gin.Engine {
	r := gin.New()
	h := api.New(d)
	h.RegisterHealthCheck(r)
	h.RegisterRoutes(r.Group("/v1"))
	return r
}

// newAuthRouter builds a Gin engine with API key auth enabled on /v1.
func newAuthRouter(d api.DockerClient, key string) *gin.Engine {
	r := gin.New()
	h := api.New(d)
	h.RegisterHealthCheck(r)
	v1 := r.Group("/v1")
	v1.Use(api.APIKeyAuth(key))
	h.RegisterRoutes(v1)
	return r
}

// do fires an HTTP request against the router and returns the recorded response.
// body is JSON-encoded when non-nil.
func do(r *gin.Engine, method, url string, body any) *httptest.ResponseRecorder {
	var b bytes.Buffer
	if body != nil {
		json.NewEncoder(&b).Encode(body)
	}
	req, _ := http.NewRequest(method, url, &b)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// doWithAuth fires an HTTP request with a Bearer token.
func doWithAuth(r *gin.Engine, method, url string, body any, token string) *httptest.ResponseRecorder {
	var b bytes.Buffer
	if body != nil {
		json.NewEncoder(&b).Encode(body)
	}
	req, _ := http.NewRequest(method, url, &b)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// ── Tests ───────────────────────────────────────────────────────────────────

func TestListSandboxes(t *testing.T) {
	r := newRouter(&stub{
		list: func() ([]models.SandboxSummary, error) {
			return []models.SandboxSummary{{ID: "abc123", Name: "test", Image: "nginx", Status: "running", State: "running"}}, nil
		},
	})

	w := do(r, "GET", "/v1/sandboxes", nil)
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "abc123")
}

func TestCreateSandbox(t *testing.T) {
	r := newRouter(&stub{
		create: func(req models.CreateSandboxRequest) (models.CreateSandboxResponse, error) {
			return models.CreateSandboxResponse{ID: "abc123", Ports: map[string]string{"3000/tcp": "32768"}}, nil
		},
	})

	w := do(r, "POST", "/v1/sandboxes", map[string]any{"image": "nextjs-docker:latest"})
	assert.Equal(t, 201, w.Code)
	assert.Contains(t, w.Body.String(), "abc123")
}

func TestCreateSandbox_MissingImage(t *testing.T) {
	r := newRouter(&stub{})

	w := do(r, "POST", "/v1/sandboxes", map[string]any{"name": "test"})
	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "BAD_REQUEST")
}

func TestGetSandbox_NotFound(t *testing.T) {
	r := newRouter(&stub{
		inspect: func(string) (models.SandboxDetail, error) {
			return models.SandboxDetail{}, docker.ErrNotFound
		},
	})

	w := do(r, "GET", "/v1/sandboxes/nope", nil)
	assert.Equal(t, 404, w.Code)
	assert.Contains(t, w.Body.String(), "NOT_FOUND")
}

func TestGetSandbox_ReturnsDetail(t *testing.T) {
	r := newRouter(&stub{
		inspect: func(id string) (models.SandboxDetail, error) {
			return models.SandboxDetail{
				ID:      id,
				Name:    "my-sandbox",
				Image:   "nginx:latest",
				Status:  "running",
				Running: true,
				Ports:   map[string]string{"80/tcp": "32770"},
				Resources: models.ResourceLimits{
					Memory: 1024,
					CPUs:   1.0,
				},
				StartedAt: "2026-02-21T02:00:18Z",
			}, nil
		},
	})

	w := do(r, "GET", "/v1/sandboxes/abc123", nil)
	assert.Equal(t, 200, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "my-sandbox")
	assert.Contains(t, body, "32770")
	assert.Contains(t, body, "running")
	// Should NOT contain raw Docker inspect noise
	assert.NotContains(t, body, "HostConfig")
	assert.NotContains(t, body, "GraphDriver")
}

func TestDeleteSandbox(t *testing.T) {
	r := newRouter(&stub{
		remove: func(string) error { return nil },
	})

	w := do(r, "DELETE", "/v1/sandboxes/abc123", nil)
	assert.Equal(t, 204, w.Code)
}

func TestStopSandbox(t *testing.T) {
	r := newRouter(&stub{
		stop: func(string) error { return nil },
	})

	w := do(r, "POST", "/v1/sandboxes/abc123/stop", nil)
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "stopped")
}

func TestRestartSandbox(t *testing.T) {
	r := newRouter(&stub{
		restart: func(string) (models.RestartResponse, error) {
			return models.RestartResponse{
				Status: "restarted",
				Ports:  map[string]string{"3000/tcp": "32775"},
			}, nil
		},
	})

	w := do(r, "POST", "/v1/sandboxes/abc123/restart", nil)
	assert.Equal(t, 200, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "restarted")
	assert.Contains(t, body, "32775")
	assert.Contains(t, body, "ports")
}

func TestRestartSandbox_NotFound(t *testing.T) {
	r := newRouter(&stub{
		restart: func(string) (models.RestartResponse, error) {
			return models.RestartResponse{}, docker.ErrNotFound
		},
	})

	w := do(r, "POST", "/v1/sandboxes/nope/restart", nil)
	assert.Equal(t, 404, w.Code)
	assert.Contains(t, w.Body.String(), "NOT_FOUND")
}

func TestExecSandbox(t *testing.T) {
	r := newRouter(&stub{
		exec: func(id string, cmd []string) (string, error) {
			return "src\npackage.json\n", nil
		},
	})

	w := do(r, "POST", "/v1/sandboxes/abc123/exec", map[string]any{"cmd": []string{"ls", "/app"}})
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "src")
}

func TestExecSandbox_MissingCmd(t *testing.T) {
	r := newRouter(&stub{})

	w := do(r, "POST", "/v1/sandboxes/abc123/exec", map[string]any{})
	assert.Equal(t, 400, w.Code)
}

func TestReadFile(t *testing.T) {
	r := newRouter(&stub{
		readFile: func(id, path string) (string, error) {
			return "hello world", nil
		},
	})

	w := do(r, "GET", "/v1/sandboxes/abc123/files?path=/app/page.tsx", nil)
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "hello world")
}

func TestReadFile_MissingPath(t *testing.T) {
	r := newRouter(&stub{})

	w := do(r, "GET", "/v1/sandboxes/abc123/files", nil)
	assert.Equal(t, 400, w.Code)
}

func TestWriteFile(t *testing.T) {
	r := newRouter(&stub{
		writeFile: func(id, path, content string) error { return nil },
	})

	w := do(r, "PUT", "/v1/sandboxes/abc123/files?path=/app/page.tsx", map[string]any{"content": "hello"})
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "written")
}

func TestDeleteFile(t *testing.T) {
	r := newRouter(&stub{
		deleteFile: func(id, path string) error { return nil },
	})

	w := do(r, "DELETE", "/v1/sandboxes/abc123/files?path=/app/page.tsx", nil)
	assert.Equal(t, 204, w.Code)
}

func TestListDir(t *testing.T) {
	r := newRouter(&stub{
		listDir: func(id, path string) (string, error) {
			return "page.tsx\nlayout.tsx\n", nil
		},
	})

	w := do(r, "GET", "/v1/sandboxes/abc123/files/list?path=/app/src", nil)
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "page.tsx")
}

func TestInternalError(t *testing.T) {
	r := newRouter(&stub{
		list: func() ([]models.SandboxSummary, error) {
			return nil, errors.New("daemon unreachable")
		},
	})

	w := do(r, "GET", "/v1/sandboxes", nil)
	assert.Equal(t, 500, w.Code)
	assert.Contains(t, w.Body.String(), "INTERNAL_ERROR")
}

func TestCreateSandbox_WithResourcesAndTimeout(t *testing.T) {
	var captured models.CreateSandboxRequest
	r := newRouter(&stub{
		create: func(req models.CreateSandboxRequest) (models.CreateSandboxResponse, error) {
			captured = req
			return models.CreateSandboxResponse{ID: "abc123", Ports: map[string]string{"3000/tcp": "32768"}}, nil
		},
	})

	w := do(r, "POST", "/v1/sandboxes", map[string]any{
		"image":   "nextjs-docker:latest",
		"timeout": 3600,
		"resources": map[string]any{
			"memory": 512,
			"cpus":   1.5,
		},
	})
	assert.Equal(t, 201, w.Code)
	assert.Equal(t, 3600, captured.Timeout)
	assert.NotNil(t, captured.Resources)
	assert.Equal(t, int64(512), captured.Resources.Memory)
	assert.Equal(t, 1.5, captured.Resources.CPUs)
}

func TestCreateSandbox_NegativeTimeout(t *testing.T) {
	r := newRouter(&stub{})

	w := do(r, "POST", "/v1/sandboxes", map[string]any{
		"image":   "nextjs-docker:latest",
		"timeout": -1,
	})
	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "BAD_REQUEST")
}

func TestCreateSandbox_NegativeMemory(t *testing.T) {
	r := newRouter(&stub{})

	w := do(r, "POST", "/v1/sandboxes", map[string]any{
		"image":     "nextjs-docker:latest",
		"resources": map[string]any{"memory": -1, "cpus": 1.0},
	})
	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "BAD_REQUEST")
}

func TestCreateSandbox_NegativeCPUs(t *testing.T) {
	r := newRouter(&stub{})

	w := do(r, "POST", "/v1/sandboxes", map[string]any{
		"image":     "nextjs-docker:latest",
		"resources": map[string]any{"memory": 512, "cpus": -0.5},
	})
	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "BAD_REQUEST")
}

func TestCreateSandbox_ExceedsMaxMemory(t *testing.T) {
	r := newRouter(&stub{})

	w := do(r, "POST", "/v1/sandboxes", map[string]any{
		"image":     "nextjs-docker:latest",
		"resources": map[string]any{"memory": 9000, "cpus": 1.0},
	})
	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "BAD_REQUEST")
	assert.Contains(t, w.Body.String(), "8192")
}

func TestCreateSandbox_ExceedsMaxCPUs(t *testing.T) {
	r := newRouter(&stub{})

	w := do(r, "POST", "/v1/sandboxes", map[string]any{
		"image":     "nextjs-docker:latest",
		"resources": map[string]any{"memory": 1024, "cpus": 5.0},
	})
	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "BAD_REQUEST")
	assert.Contains(t, w.Body.String(), "4.0")
}

func TestCreateSandbox_DefaultResources(t *testing.T) {
	var captured models.CreateSandboxRequest
	r := newRouter(&stub{
		create: func(req models.CreateSandboxRequest) (models.CreateSandboxResponse, error) {
			captured = req
			return models.CreateSandboxResponse{ID: "test123"}, nil
		},
	})

	// Create without specifying resources
	w := do(r, "POST", "/v1/sandboxes", map[string]any{
		"image": "nextjs-docker:latest",
	})
	assert.Equal(t, 201, w.Code)
	// Resources should be nil in the request (defaults applied in Docker client)
	assert.Nil(t, captured.Resources)
}

func TestPauseSandbox(t *testing.T) {
	r := newRouter(&stub{
		pause: func(string) error { return nil },
	})

	w := do(r, "POST", "/v1/sandboxes/abc123/pause", nil)
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "paused")
}

func TestPauseSandbox_NotFound(t *testing.T) {
	r := newRouter(&stub{
		pause: func(string) error { return docker.ErrNotFound },
	})

	w := do(r, "POST", "/v1/sandboxes/nope/pause", nil)
	assert.Equal(t, 404, w.Code)
	assert.Contains(t, w.Body.String(), "NOT_FOUND")
}

func TestResumeSandbox(t *testing.T) {
	r := newRouter(&stub{
		resume: func(string) error { return nil },
	})

	w := do(r, "POST", "/v1/sandboxes/abc123/resume", nil)
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "resumed")
}

func TestResumeSandbox_NotFound(t *testing.T) {
	r := newRouter(&stub{
		resume: func(string) error { return docker.ErrNotFound },
	})

	w := do(r, "POST", "/v1/sandboxes/nope/resume", nil)
	assert.Equal(t, 404, w.Code)
	assert.Contains(t, w.Body.String(), "NOT_FOUND")
}

func TestRenewExpiration(t *testing.T) {
	var capturedID string
	var capturedTimeout int
	r := newRouter(&stub{
		renewExpiration: func(id string, timeout int) error {
			capturedID = id
			capturedTimeout = timeout
			return nil
		},
	})

	w := do(r, "POST", "/v1/sandboxes/abc123/renew-expiration", map[string]any{"timeout": 7200})
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "renewed")
	assert.Contains(t, w.Body.String(), "7200")
	assert.Equal(t, "abc123", capturedID)
	assert.Equal(t, 7200, capturedTimeout)
}

func TestRenewExpiration_NotFound(t *testing.T) {
	r := newRouter(&stub{
		renewExpiration: func(string, int) error { return docker.ErrNotFound },
	})

	w := do(r, "POST", "/v1/sandboxes/nope/renew-expiration", map[string]any{"timeout": 3600})
	assert.Equal(t, 404, w.Code)
	assert.Contains(t, w.Body.String(), "NOT_FOUND")
}

func TestRenewExpiration_MissingTimeout(t *testing.T) {
	r := newRouter(&stub{})

	w := do(r, "POST", "/v1/sandboxes/abc123/renew-expiration", map[string]any{})
	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "BAD_REQUEST")
}

func TestRenewExpiration_NegativeTimeout(t *testing.T) {
	r := newRouter(&stub{})

	w := do(r, "POST", "/v1/sandboxes/abc123/renew-expiration", map[string]any{"timeout": -1})
	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "BAD_REQUEST")
}

func TestRenewExpiration_ZeroTimeout(t *testing.T) {
	r := newRouter(&stub{})

	w := do(r, "POST", "/v1/sandboxes/abc123/renew-expiration", map[string]any{"timeout": 0})
	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "BAD_REQUEST")
}

// ── API Key Auth Tests ──────────────────────────────────────────────────────

func TestApiKeyAuth_NoHeader(t *testing.T) {
	r := newAuthRouter(&stub{
		list: func() ([]models.SandboxSummary, error) {
			return []models.SandboxSummary{}, nil
		},
	}, "sk-test-123")

	w := do(r, "GET", "/v1/sandboxes", nil)
	assert.Equal(t, 401, w.Code)
	assert.Contains(t, w.Body.String(), "UNAUTHORIZED")
}

func TestApiKeyAuth_WrongKey(t *testing.T) {
	r := newAuthRouter(&stub{
		list: func() ([]models.SandboxSummary, error) {
			return []models.SandboxSummary{}, nil
		},
	}, "sk-test-123")

	w := doWithAuth(r, "GET", "/v1/sandboxes", nil, "sk-wrong")
	assert.Equal(t, 401, w.Code)
	assert.Contains(t, w.Body.String(), "UNAUTHORIZED")
}

func TestApiKeyAuth_CorrectKey(t *testing.T) {
	r := newAuthRouter(&stub{
		list: func() ([]models.SandboxSummary, error) {
			return []models.SandboxSummary{{ID: "abc123"}}, nil
		},
	}, "sk-test-123")

	w := doWithAuth(r, "GET", "/v1/sandboxes", nil, "sk-test-123")
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "abc123")
}

func TestNoAuth_WorksWithoutMiddleware(t *testing.T) {
	r := newRouter(&stub{
		list: func() ([]models.SandboxSummary, error) {
			return []models.SandboxSummary{{ID: "abc123"}}, nil
		},
	})

	w := do(r, "GET", "/v1/sandboxes", nil)
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "abc123")
}

// ── Health Check Tests ──────────────────────────────────────────────────────

func TestHealthCheck_Healthy(t *testing.T) {
	r := newRouter(&stub{
		ping: func() error { return nil },
	})

	w := do(r, "GET", "/v1/health", nil)
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "healthy")
}

func TestHealthCheck_Unhealthy(t *testing.T) {
	r := newRouter(&stub{
		ping: func() error { return errors.New("daemon unreachable") },
	})

	w := do(r, "GET", "/v1/health", nil)
	assert.Equal(t, 503, w.Code)
	assert.Contains(t, w.Body.String(), "unhealthy")
	assert.Contains(t, w.Body.String(), "daemon unreachable")
}

func TestHealthCheck_NoAuthRequired(t *testing.T) {
	r := newAuthRouter(&stub{
		ping: func() error { return nil },
	}, "sk-test-123")

	// Health check should work without auth header.
	w := do(r, "GET", "/v1/health", nil)
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "healthy")
}

func TestPullImage(t *testing.T) {
	var capturedImage string
	r := newRouter(&stub{
		pullImage: func(image string) error {
			capturedImage = image
			return nil
		},
	})

	w := do(r, "POST", "/v1/images/pull", map[string]any{
		"image": "nginx:latest",
	})
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "pulled")
	assert.Contains(t, w.Body.String(), "nginx:latest")
	assert.Equal(t, "nginx:latest", capturedImage)
}

func TestPullImage_MissingImage(t *testing.T) {
	r := newRouter(&stub{})

	w := do(r, "POST", "/v1/images/pull", map[string]any{})
	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "BAD_REQUEST")
}

func TestPullImage_Error(t *testing.T) {
	r := newRouter(&stub{
		pullImage: func(string) error {
			return errors.New("registry unreachable")
		},
	})

	w := do(r, "POST", "/v1/images/pull", map[string]any{
		"image": "nginx:latest",
	})
	assert.Equal(t, 500, w.Code)
	assert.Contains(t, w.Body.String(), "INTERNAL_ERROR")
	assert.Contains(t, w.Body.String(), "registry unreachable")
}

func TestCreateSandbox_ImageNotFound(t *testing.T) {
	r := newRouter(&stub{
		create: func(models.CreateSandboxRequest) (models.CreateSandboxResponse, error) {
			return models.CreateSandboxResponse{}, docker.ErrImageNotFound
		},
	})

	w := do(r, "POST", "/v1/sandboxes", map[string]any{
		"image": "nonexistent:latest",
	})
	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "BAD_REQUEST")
	assert.Contains(t, w.Body.String(), "image not found locally")
	assert.Contains(t, w.Body.String(), "/v1/images/pull")
}

// ── Logs Tests ──────────────────────────────────────────────────────────────

// muxFrame builds a Docker multiplexed log frame (8-byte header + payload).
func muxFrame(streamType byte, data string) []byte {
	payload := []byte(data)
	frame := make([]byte, 8+len(payload))
	frame[0] = streamType
	frame[4] = byte(len(payload) >> 24)
	frame[5] = byte(len(payload) >> 16)
	frame[6] = byte(len(payload) >> 8)
	frame[7] = byte(len(payload))
	copy(frame[8:], payload)
	return frame
}

func TestGetLogs_Snapshot(t *testing.T) {
	// Build a multiplexed stream with stdout frames.
	var buf bytes.Buffer
	buf.Write(muxFrame(1, "line one\n"))
	buf.Write(muxFrame(1, "line two\n"))
	data := buf.Bytes()

	r := newRouter(&stub{
		logs: func(id string, opts models.LogsOptions) (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(data)), nil
		},
	})

	w := do(r, "GET", "/v1/sandboxes/abc123/logs", nil)
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "line one")
	assert.Contains(t, w.Body.String(), "line two")
}

func TestGetLogs_NotFound(t *testing.T) {
	r := newRouter(&stub{
		logs: func(string, models.LogsOptions) (io.ReadCloser, error) {
			return nil, docker.ErrNotFound
		},
	})

	w := do(r, "GET", "/v1/sandboxes/nope/logs", nil)
	assert.Equal(t, 404, w.Code)
	assert.Contains(t, w.Body.String(), "NOT_FOUND")
}

func TestGetLogs_DefaultTail(t *testing.T) {
	var capturedOpts models.LogsOptions
	r := newRouter(&stub{
		logs: func(id string, opts models.LogsOptions) (io.ReadCloser, error) {
			capturedOpts = opts
			return io.NopCloser(bytes.NewReader(nil)), nil
		},
	})

	do(r, "GET", "/v1/sandboxes/abc123/logs", nil)
	// Default tail is 0 (Go zero value) — the Docker client defaults to "100".
	assert.Equal(t, 0, capturedOpts.Tail)
	assert.False(t, capturedOpts.Follow)
	assert.False(t, capturedOpts.Timestamps)
}

func TestGetLogs_CustomTail(t *testing.T) {
	var capturedOpts models.LogsOptions
	r := newRouter(&stub{
		logs: func(id string, opts models.LogsOptions) (io.ReadCloser, error) {
			capturedOpts = opts
			return io.NopCloser(bytes.NewReader(nil)), nil
		},
	})

	do(r, "GET", "/v1/sandboxes/abc123/logs?tail=50&timestamps=true", nil)
	assert.Equal(t, 50, capturedOpts.Tail)
	assert.True(t, capturedOpts.Timestamps)
}

func TestGetLogs_Follow_SSE(t *testing.T) {
	// Create a stream that sends a few lines then closes.
	var buf bytes.Buffer
	buf.Write(muxFrame(1, "log line 1\n"))
	buf.Write(muxFrame(1, "log line 2\n"))
	data := buf.Bytes()

	r := newRouter(&stub{
		logs: func(id string, opts models.LogsOptions) (io.ReadCloser, error) {
			assert.True(t, opts.Follow)
			return io.NopCloser(bytes.NewReader(data)), nil
		},
	})

	w := do(r, "GET", "/v1/sandboxes/abc123/logs?follow=true", nil)
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/event-stream")
	assert.Contains(t, w.Body.String(), "log line 1")
	assert.Contains(t, w.Body.String(), "log line 2")
}

func TestGetLogs_NegativeTail(t *testing.T) {
	r := newRouter(&stub{})

	w := do(r, "GET", "/v1/sandboxes/abc123/logs?tail=-5", nil)
	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "BAD_REQUEST")
}

// ── Stats Tests ─────────────────────────────────────────────────────────────

func TestGetStats_OK(t *testing.T) {
	r := newRouter(&stub{
		stats: func(id string) (models.SandboxStats, error) {
			return models.SandboxStats{
				CPU: 25.5,
				Memory: models.MemoryUsage{
					Usage:   512 * 1024 * 1024,
					Limit:   1024 * 1024 * 1024,
					Percent: 50.0,
				},
				PIDs: 12,
			}, nil
		},
	})

	w := do(r, "GET", "/v1/sandboxes/abc123/stats", nil)
	assert.Equal(t, 200, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "25.5")
	assert.Contains(t, body, "50")
	assert.Contains(t, body, `"pids":12`)
}

func TestGetStats_NotFound(t *testing.T) {
	r := newRouter(&stub{
		stats: func(string) (models.SandboxStats, error) {
			return models.SandboxStats{}, docker.ErrNotFound
		},
	})

	w := do(r, "GET", "/v1/sandboxes/nope/stats", nil)
	assert.Equal(t, 404, w.Code)
	assert.Contains(t, w.Body.String(), "NOT_FOUND")
}

func TestGetStats_Error(t *testing.T) {
	r := newRouter(&stub{
		stats: func(string) (models.SandboxStats, error) {
			return models.SandboxStats{}, errors.New("daemon error")
		},
	})

	w := do(r, "GET", "/v1/sandboxes/abc123/stats", nil)
	assert.Equal(t, 500, w.Code)
	assert.Contains(t, w.Body.String(), "INTERNAL_ERROR")
}
