package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/moby/moby/api/types/container"
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
	list            func(bool) ([]container.Summary, error)
	create          func(models.CreateSandboxRequest) (models.CreateSandboxResponse, error)
	inspect         func(string) (container.InspectResponse, error)
	stop            func(string) error
	restart         func(string) error
	remove          func(string) error
	pause           func(string) error
	resume          func(string) error
	renewExpiration func(string, int) error
	exec            func(string, []string) (string, error)
	readFile        func(string, string) (string, error)
	writeFile       func(string, string, string) error
	deleteFile      func(string, string) error
	listDir         func(string, string) (string, error)
}

func (s *stub) Ping(_ context.Context) error {
	if s.ping != nil {
		return s.ping()
	}
	return nil
}
func (s *stub) List(_ context.Context, all bool) ([]container.Summary, error) {
	return s.list(all)
}
func (s *stub) Create(_ context.Context, r models.CreateSandboxRequest) (models.CreateSandboxResponse, error) {
	return s.create(r)
}
func (s *stub) Inspect(_ context.Context, id string) (container.InspectResponse, error) {
	return s.inspect(id)
}
func (s *stub) Stop(_ context.Context, id string) error    { return s.stop(id) }
func (s *stub) Restart(_ context.Context, id string) error { return s.restart(id) }
func (s *stub) Remove(_ context.Context, id string) error  { return s.remove(id) }
func (s *stub) Pause(_ context.Context, id string) error   { return s.pause(id) }
func (s *stub) Resume(_ context.Context, id string) error  { return s.resume(id) }
func (s *stub) RenewExpiration(_ context.Context, id string, timeout int) error {
	return s.renewExpiration(id, timeout)
}
func (s *stub) Exec(_ context.Context, id string, cmd []string) (string, error) {
	return s.exec(id, cmd)
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
		list: func(bool) ([]container.Summary, error) {
			return []container.Summary{{ID: "abc123"}}, nil
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
		inspect: func(string) (container.InspectResponse, error) {
			return container.InspectResponse{}, docker.ErrNotFound
		},
	})

	w := do(r, "GET", "/v1/sandboxes/nope", nil)
	assert.Equal(t, 404, w.Code)
	assert.Contains(t, w.Body.String(), "NOT_FOUND")
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
		restart: func(string) error { return nil },
	})

	w := do(r, "POST", "/v1/sandboxes/abc123/restart", nil)
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "restarted")
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
		list: func(bool) ([]container.Summary, error) {
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
		list: func(bool) ([]container.Summary, error) {
			return []container.Summary{}, nil
		},
	}, "sk-test-123")

	w := do(r, "GET", "/v1/sandboxes", nil)
	assert.Equal(t, 401, w.Code)
	assert.Contains(t, w.Body.String(), "UNAUTHORIZED")
}

func TestApiKeyAuth_WrongKey(t *testing.T) {
	r := newAuthRouter(&stub{
		list: func(bool) ([]container.Summary, error) {
			return []container.Summary{}, nil
		},
	}, "sk-test-123")

	w := doWithAuth(r, "GET", "/v1/sandboxes", nil, "sk-wrong")
	assert.Equal(t, 401, w.Code)
	assert.Contains(t, w.Body.String(), "UNAUTHORIZED")
}

func TestApiKeyAuth_CorrectKey(t *testing.T) {
	r := newAuthRouter(&stub{
		list: func(bool) ([]container.Summary, error) {
			return []container.Summary{{ID: "abc123"}}, nil
		},
	}, "sk-test-123")

	w := doWithAuth(r, "GET", "/v1/sandboxes", nil, "sk-test-123")
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "abc123")
}

func TestNoAuth_WorksWithoutMiddleware(t *testing.T) {
	r := newRouter(&stub{
		list: func(bool) ([]container.Summary, error) {
			return []container.Summary{{ID: "abc123"}}, nil
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

	w := do(r, "GET", "/health", nil)
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "healthy")
}

func TestHealthCheck_Unhealthy(t *testing.T) {
	r := newRouter(&stub{
		ping: func() error { return errors.New("daemon unreachable") },
	})

	w := do(r, "GET", "/health", nil)
	assert.Equal(t, 503, w.Code)
	assert.Contains(t, w.Body.String(), "unhealthy")
	assert.Contains(t, w.Body.String(), "daemon unreachable")
}

func TestHealthCheck_NoAuthRequired(t *testing.T) {
	r := newAuthRouter(&stub{
		ping: func() error { return nil },
	}, "sk-test-123")

	// Health check should work without auth header.
	w := do(r, "GET", "/health", nil)
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "healthy")
}
