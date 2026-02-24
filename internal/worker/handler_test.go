package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"open-sandbox/internal/docker"
	"open-sandbox/models"
)

// stubDocker implements DockerClient for testing.
type stubDocker struct {
	pingErr       error
	listResult    []models.SandboxSummary
	createResult  models.CreateSandboxResponse
	createErr     error
	inspectResult models.SandboxDetail
	inspectErr    error
	startResult   models.RestartResponse
	startErr      error
	stopErr       error
	restartResult models.RestartResponse
	restartErr    error
	removeErr     error
	pauseErr      error
	resumeErr     error
	renewErr      error
	statsResult   models.SandboxStats
	statsErr      error
	execResult    models.CommandDetail
	execErr       error
	getCmd        models.CommandDetail
	getCmdErr     error
	listCmds      []models.CommandDetail
	listCmdsErr   error
	killResult    models.CommandDetail
	killErr       error
	logsResult    models.CommandLogsResponse
	logsErr       error
	waitResult    models.CommandDetail
	waitErr       error
	readResult    string
	readErr       error
	writeErr      error
	deleteFileErr error
	listDirResult string
	listDirErr    error
	pullErr       error
	removeImgErr  error
	inspectImg    models.ImageDetail
	inspectImgErr error
	listImgs      []models.ImageSummary
	listImgsErr   error
}

func (s *stubDocker) Ping(ctx context.Context) error { return s.pingErr }
func (s *stubDocker) List(ctx context.Context) ([]models.SandboxSummary, error) {
	return s.listResult, nil
}
func (s *stubDocker) Create(ctx context.Context, req models.CreateSandboxRequest) (models.CreateSandboxResponse, error) {
	return s.createResult, s.createErr
}
func (s *stubDocker) Inspect(ctx context.Context, id string) (models.SandboxDetail, error) {
	return s.inspectResult, s.inspectErr
}
func (s *stubDocker) Start(ctx context.Context, id string) (models.RestartResponse, error) {
	return s.startResult, s.startErr
}
func (s *stubDocker) Stop(ctx context.Context, id string) error { return s.stopErr }
func (s *stubDocker) Restart(ctx context.Context, id string) (models.RestartResponse, error) {
	return s.restartResult, s.restartErr
}
func (s *stubDocker) Remove(ctx context.Context, id string) error { return s.removeErr }
func (s *stubDocker) Pause(ctx context.Context, id string) error  { return s.pauseErr }
func (s *stubDocker) Resume(ctx context.Context, id string) error { return s.resumeErr }
func (s *stubDocker) RenewExpiration(ctx context.Context, id string, timeout int) error {
	return s.renewErr
}
func (s *stubDocker) ExecCommand(ctx context.Context, sandboxID string, req models.ExecCommandRequest) (models.CommandDetail, error) {
	return s.execResult, s.execErr
}
func (s *stubDocker) GetCommand(ctx context.Context, sandboxID, cmdID string) (models.CommandDetail, error) {
	return s.getCmd, s.getCmdErr
}
func (s *stubDocker) ListCommands(ctx context.Context, sandboxID string) ([]models.CommandDetail, error) {
	return s.listCmds, s.listCmdsErr
}
func (s *stubDocker) KillCommand(ctx context.Context, sandboxID, cmdID string, signal int) (models.CommandDetail, error) {
	return s.killResult, s.killErr
}
func (s *stubDocker) StreamCommandLogs(ctx context.Context, sandboxID, cmdID string) (io.ReadCloser, io.ReadCloser, error) {
	stdout := io.NopCloser(strings.NewReader("hello\n"))
	stderr := io.NopCloser(strings.NewReader(""))
	return stdout, stderr, s.logsErr
}
func (s *stubDocker) GetCommandLogs(ctx context.Context, sandboxID, cmdID string) (models.CommandLogsResponse, error) {
	return s.logsResult, s.logsErr
}
func (s *stubDocker) WaitCommand(ctx context.Context, sandboxID, cmdID string) (models.CommandDetail, error) {
	return s.waitResult, s.waitErr
}
func (s *stubDocker) Stats(ctx context.Context, id string) (models.SandboxStats, error) {
	return s.statsResult, s.statsErr
}
func (s *stubDocker) ReadFile(ctx context.Context, id, path string) (string, error) {
	return s.readResult, s.readErr
}
func (s *stubDocker) WriteFile(ctx context.Context, id, path, content string) error {
	return s.writeErr
}
func (s *stubDocker) DeleteFile(ctx context.Context, id, path string) error {
	return s.deleteFileErr
}
func (s *stubDocker) ListDir(ctx context.Context, id, path string) (string, error) {
	return s.listDirResult, s.listDirErr
}
func (s *stubDocker) PullImage(ctx context.Context, image string) error { return s.pullErr }
func (s *stubDocker) RemoveImage(ctx context.Context, id string, force bool) error {
	return s.removeImgErr
}
func (s *stubDocker) InspectImage(ctx context.Context, id string) (models.ImageDetail, error) {
	return s.inspectImg, s.inspectImgErr
}
func (s *stubDocker) ListImages(ctx context.Context) ([]models.ImageSummary, error) {
	return s.listImgs, s.listImgsErr
}

func setupRouter(stub *stubDocker) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHandler(stub)
	g := r.Group("/internal/v1")
	h.RegisterRoutes(g)
	return r
}

// --- Health ---

func TestHealth_OK(t *testing.T) {
	r := setupRouter(&stubDocker{})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/internal/v1/health", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestHealth_Unhealthy(t *testing.T) {
	r := setupRouter(&stubDocker{pingErr: docker.ErrNotRunning})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/internal/v1/health", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

// --- Sandbox CRUD ---

func TestCreateSandbox_OK(t *testing.T) {
	stub := &stubDocker{
		createResult: models.CreateSandboxResponse{ID: "abc123", Name: "test-box"},
	}
	r := setupRouter(stub)
	body, _ := json.Marshal(models.CreateSandboxRequest{Image: "node:24"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/internal/v1/sandboxes", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateSandbox_BadRequest(t *testing.T) {
	r := setupRouter(&stubDocker{})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/internal/v1/sandboxes", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestInspectSandbox_OK(t *testing.T) {
	stub := &stubDocker{
		inspectResult: models.SandboxDetail{ID: "abc", Name: "test"},
	}
	r := setupRouter(stub)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/internal/v1/sandboxes/abc", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestInspectSandbox_NotFound(t *testing.T) {
	stub := &stubDocker{inspectErr: docker.ErrNotFound}
	r := setupRouter(stub)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/internal/v1/sandboxes/missing", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestDeleteSandbox_OK(t *testing.T) {
	r := setupRouter(&stubDocker{})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/internal/v1/sandboxes/abc", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
}

func TestDeleteSandbox_NotFound(t *testing.T) {
	stub := &stubDocker{removeErr: docker.ErrNotFound}
	r := setupRouter(stub)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/internal/v1/sandboxes/missing", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

// --- Lifecycle ---

func TestStartSandbox_OK(t *testing.T) {
	r := setupRouter(&stubDocker{startResult: models.RestartResponse{Status: "started"}})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/internal/v1/sandboxes/abc/start", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestStartSandbox_AlreadyRunning(t *testing.T) {
	r := setupRouter(&stubDocker{startErr: docker.ErrAlreadyRunning})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/internal/v1/sandboxes/abc/start", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

func TestStopSandbox_OK(t *testing.T) {
	r := setupRouter(&stubDocker{})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/internal/v1/sandboxes/abc/stop", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestStopSandbox_AlreadyStopped(t *testing.T) {
	r := setupRouter(&stubDocker{stopErr: docker.ErrAlreadyStopped})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/internal/v1/sandboxes/abc/stop", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

func TestPauseSandbox_OK(t *testing.T) {
	r := setupRouter(&stubDocker{})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/internal/v1/sandboxes/abc/pause", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestResumeSandbox_OK(t *testing.T) {
	r := setupRouter(&stubDocker{})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/internal/v1/sandboxes/abc/resume", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestRestartSandbox_OK(t *testing.T) {
	r := setupRouter(&stubDocker{restartResult: models.RestartResponse{Status: "restarted"}})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/internal/v1/sandboxes/abc/restart", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// --- Stats ---

func TestStats_OK(t *testing.T) {
	stub := &stubDocker{statsResult: models.SandboxStats{CPU: 12.5}}
	r := setupRouter(stub)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/internal/v1/sandboxes/abc/stats", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// --- Commands ---

func TestExecCommand_OK(t *testing.T) {
	stub := &stubDocker{execResult: models.CommandDetail{ID: "cmd_1"}}
	r := setupRouter(stub)
	body, _ := json.Marshal(models.ExecCommandRequest{Command: "ls"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/internal/v1/sandboxes/abc/cmd", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestExecCommand_NotRunning(t *testing.T) {
	stub := &stubDocker{execErr: docker.ErrNotRunning}
	r := setupRouter(stub)
	body, _ := json.Marshal(models.ExecCommandRequest{Command: "ls"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/internal/v1/sandboxes/abc/cmd", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

func TestListCommands_OK(t *testing.T) {
	stub := &stubDocker{listCmds: []models.CommandDetail{{ID: "cmd_1"}}}
	r := setupRouter(stub)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/internal/v1/sandboxes/abc/cmd", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestKillCommand_OK(t *testing.T) {
	stub := &stubDocker{killResult: models.CommandDetail{ID: "cmd_1"}}
	r := setupRouter(stub)
	body, _ := json.Marshal(models.KillCommandRequest{Signal: 15})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/internal/v1/sandboxes/abc/cmd/cmd_1/kill", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestKillCommand_AlreadyFinished(t *testing.T) {
	stub := &stubDocker{killErr: docker.ErrCommandFinished}
	r := setupRouter(stub)
	body, _ := json.Marshal(models.KillCommandRequest{Signal: 9})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/internal/v1/sandboxes/abc/cmd/cmd_1/kill", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

func TestGetCommandLogs_OK(t *testing.T) {
	stub := &stubDocker{logsResult: models.CommandLogsResponse{Stdout: "output"}}
	r := setupRouter(stub)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/internal/v1/sandboxes/abc/cmd/cmd_1/logs", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// --- Files ---

func TestReadFile_OK(t *testing.T) {
	stub := &stubDocker{readResult: "file content"}
	r := setupRouter(stub)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/internal/v1/sandboxes/abc/files?path=/app/main.go", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestReadFile_MissingPath(t *testing.T) {
	r := setupRouter(&stubDocker{})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/internal/v1/sandboxes/abc/files", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestWriteFile_OK(t *testing.T) {
	r := setupRouter(&stubDocker{})
	body, _ := json.Marshal(models.FileWriteRequest{Content: "hello"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/internal/v1/sandboxes/abc/files?path=/app/main.go", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestWriteFile_MissingPath(t *testing.T) {
	r := setupRouter(&stubDocker{})
	body, _ := json.Marshal(models.FileWriteRequest{Content: "hello"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/internal/v1/sandboxes/abc/files", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestDeleteFile_OK(t *testing.T) {
	r := setupRouter(&stubDocker{})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/internal/v1/sandboxes/abc/files?path=/app/tmp.txt", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
}

func TestDeleteFile_MissingPath(t *testing.T) {
	r := setupRouter(&stubDocker{})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/internal/v1/sandboxes/abc/files", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestListDir_OK(t *testing.T) {
	stub := &stubDocker{listDirResult: "file1\nfile2\n"}
	r := setupRouter(stub)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/internal/v1/sandboxes/abc/files/list", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// --- Images ---

func TestListImages_OK(t *testing.T) {
	stub := &stubDocker{listImgs: []models.ImageSummary{{ID: "sha256:abc"}}}
	r := setupRouter(stub)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/internal/v1/images", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestInspectImage_OK(t *testing.T) {
	stub := &stubDocker{inspectImg: models.ImageDetail{ID: "sha256:abc"}}
	r := setupRouter(stub)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/internal/v1/images/sha256:abc", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestInspectImage_NotFound(t *testing.T) {
	stub := &stubDocker{inspectImgErr: docker.ErrImageNotFound}
	r := setupRouter(stub)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/internal/v1/images/missing", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestPullImage_OK(t *testing.T) {
	r := setupRouter(&stubDocker{})
	body, _ := json.Marshal(models.ImagePullRequest{Image: "node:24"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/internal/v1/images/pull", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteImage_OK(t *testing.T) {
	r := setupRouter(&stubDocker{})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/internal/v1/images/sha256:abc", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
}

// --- Error mapping ---

func TestMapError_Timeout(t *testing.T) {
	stub := &stubDocker{inspectErr: context.DeadlineExceeded}
	r := setupRouter(stub)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/internal/v1/sandboxes/abc", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusRequestTimeout {
		t.Fatalf("expected 408, got %d", w.Code)
	}
}

func TestMapError_CommandNotFound(t *testing.T) {
	stub := &stubDocker{getCmdErr: docker.ErrCommandNotFound}
	r := setupRouter(stub)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/internal/v1/sandboxes/abc/cmd/missing", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

// --- Renew Expiration ---

func TestRenewExpiration_OK(t *testing.T) {
	r := setupRouter(&stubDocker{})
	body, _ := json.Marshal(models.RenewExpirationRequest{Timeout: 600})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/internal/v1/sandboxes/abc/renew-expiration", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
