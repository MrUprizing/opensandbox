package worker

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"open-sandbox/internal/docker"
	"open-sandbox/models"
)

// DockerClient defines the sandbox operations used by the worker handlers.
// Identical to api.DockerClient — duplicated to avoid import cycles.
type DockerClient interface {
	Ping(ctx context.Context) error
	List(ctx context.Context) ([]models.SandboxSummary, error)
	Create(ctx context.Context, req models.CreateSandboxRequest) (models.CreateSandboxResponse, error)
	Inspect(ctx context.Context, id string) (models.SandboxDetail, error)
	Start(ctx context.Context, id string) (models.RestartResponse, error)
	Stop(ctx context.Context, id string) error
	Restart(ctx context.Context, id string) (models.RestartResponse, error)
	Remove(ctx context.Context, id string) error
	Pause(ctx context.Context, id string) error
	Resume(ctx context.Context, id string) error
	RenewExpiration(ctx context.Context, id string, timeout int) error
	ExecCommand(ctx context.Context, sandboxID string, req models.ExecCommandRequest) (models.CommandDetail, error)
	GetCommand(ctx context.Context, sandboxID, cmdID string) (models.CommandDetail, error)
	ListCommands(ctx context.Context, sandboxID string) ([]models.CommandDetail, error)
	KillCommand(ctx context.Context, sandboxID, cmdID string, signal int) (models.CommandDetail, error)
	StreamCommandLogs(ctx context.Context, sandboxID, cmdID string) (io.ReadCloser, io.ReadCloser, error)
	GetCommandLogs(ctx context.Context, sandboxID, cmdID string) (models.CommandLogsResponse, error)
	WaitCommand(ctx context.Context, sandboxID, cmdID string) (models.CommandDetail, error)
	Stats(ctx context.Context, id string) (models.SandboxStats, error)
	ReadFile(ctx context.Context, id, path string) (string, error)
	WriteFile(ctx context.Context, id, path, content string) error
	DeleteFile(ctx context.Context, id, path string) error
	ListDir(ctx context.Context, id, path string) (string, error)
	PullImage(ctx context.Context, image string) error
	RemoveImage(ctx context.Context, id string, force bool) error
	InspectImage(ctx context.Context, id string) (models.ImageDetail, error)
	ListImages(ctx context.Context) ([]models.ImageSummary, error)
}

// Handler holds worker handler dependencies.
type Handler struct {
	docker DockerClient
}

// NewHandler creates a worker Handler.
func NewHandler(d DockerClient) *Handler {
	return &Handler{docker: d}
}

// --- Error helpers ---

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func mapError(c *gin.Context, err error) {
	if errors.Is(err, docker.ErrNotFound) {
		c.JSON(http.StatusNotFound, errorResponse{Code: "NOT_FOUND", Message: "sandbox not found"})
		return
	}
	if errors.Is(err, docker.ErrImageNotFound) {
		c.JSON(http.StatusBadRequest, errorResponse{Code: "BAD_REQUEST", Message: "image not found locally"})
		return
	}
	if errors.Is(err, docker.ErrAlreadyRunning) || errors.Is(err, docker.ErrAlreadyStopped) ||
		errors.Is(err, docker.ErrAlreadyPaused) || errors.Is(err, docker.ErrNotPaused) ||
		errors.Is(err, docker.ErrNotRunning) || errors.Is(err, docker.ErrCommandFinished) {
		c.JSON(http.StatusConflict, errorResponse{Code: "CONFLICT", Message: err.Error()})
		return
	}
	if errors.Is(err, docker.ErrCommandNotFound) {
		c.JSON(http.StatusNotFound, errorResponse{Code: "NOT_FOUND", Message: "command not found"})
		return
	}
	if errors.Is(err, context.DeadlineExceeded) {
		c.JSON(http.StatusRequestTimeout, errorResponse{Code: "TIMEOUT", Message: "operation timed out"})
		return
	}
	c.JSON(http.StatusInternalServerError, errorResponse{Code: "INTERNAL_ERROR", Message: err.Error()})
}

// --- Sandbox handlers ---

func (h *Handler) health(c *gin.Context) {
	if err := h.docker.Ping(c.Request.Context()); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "healthy"})
}

func (h *Handler) listSandboxes(c *gin.Context) {
	items, err := h.docker.List(c.Request.Context())
	if err != nil {
		mapError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"sandboxes": items})
}

func (h *Handler) createSandbox(c *gin.Context) {
	var req models.CreateSandboxRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Code: "BAD_REQUEST", Message: err.Error()})
		return
	}
	result, err := h.docker.Create(c.Request.Context(), req)
	if err != nil {
		mapError(c, err)
		return
	}
	c.JSON(http.StatusCreated, result)
}

func (h *Handler) inspectSandbox(c *gin.Context) {
	info, err := h.docker.Inspect(c.Request.Context(), c.Param("id"))
	if err != nil {
		mapError(c, err)
		return
	}
	c.JSON(http.StatusOK, info)
}

func (h *Handler) deleteSandbox(c *gin.Context) {
	if err := h.docker.Remove(c.Request.Context(), c.Param("id")); err != nil {
		mapError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) startSandbox(c *gin.Context) {
	result, err := h.docker.Start(c.Request.Context(), c.Param("id"))
	if err != nil {
		mapError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) stopSandbox(c *gin.Context) {
	if err := h.docker.Stop(c.Request.Context(), c.Param("id")); err != nil {
		mapError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "stopped"})
}

func (h *Handler) restartSandbox(c *gin.Context) {
	result, err := h.docker.Restart(c.Request.Context(), c.Param("id"))
	if err != nil {
		mapError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) pauseSandbox(c *gin.Context) {
	if err := h.docker.Pause(c.Request.Context(), c.Param("id")); err != nil {
		mapError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "paused"})
}

func (h *Handler) resumeSandbox(c *gin.Context) {
	if err := h.docker.Resume(c.Request.Context(), c.Param("id")); err != nil {
		mapError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "resumed"})
}

func (h *Handler) renewExpiration(c *gin.Context) {
	var req models.RenewExpirationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Code: "BAD_REQUEST", Message: err.Error()})
		return
	}
	if err := h.docker.RenewExpiration(c.Request.Context(), c.Param("id"), req.Timeout); err != nil {
		mapError(c, err)
		return
	}
	c.JSON(http.StatusOK, models.RenewExpirationResponse{Status: "renewed", Timeout: req.Timeout})
}

func (h *Handler) getStats(c *gin.Context) {
	stats, err := h.docker.Stats(c.Request.Context(), c.Param("id"))
	if err != nil {
		mapError(c, err)
		return
	}
	c.JSON(http.StatusOK, stats)
}

// --- Command handlers ---

func (h *Handler) execCommand(c *gin.Context) {
	var req models.ExecCommandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Code: "BAD_REQUEST", Message: err.Error()})
		return
	}
	cmd, err := h.docker.ExecCommand(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		mapError(c, err)
		return
	}

	if c.Query("wait") == "true" {
		h.streamWait(c, c.Param("id"), cmd.ID)
		return
	}
	c.JSON(http.StatusOK, models.CommandResponse{Command: cmd})
}

func (h *Handler) listCommands(c *gin.Context) {
	cmds, err := h.docker.ListCommands(c.Request.Context(), c.Param("id"))
	if err != nil {
		mapError(c, err)
		return
	}
	c.JSON(http.StatusOK, models.CommandListResponse{Commands: cmds})
}

func (h *Handler) getCommand(c *gin.Context) {
	cmd, err := h.docker.GetCommand(c.Request.Context(), c.Param("id"), c.Param("cmdId"))
	if err != nil {
		mapError(c, err)
		return
	}
	if c.Query("wait") == "true" {
		h.streamWait(c, c.Param("id"), c.Param("cmdId"))
		return
	}
	c.JSON(http.StatusOK, models.CommandResponse{Command: cmd})
}

func (h *Handler) killCommand(c *gin.Context) {
	var req models.KillCommandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Code: "BAD_REQUEST", Message: err.Error()})
		return
	}
	cmd, err := h.docker.KillCommand(c.Request.Context(), c.Param("id"), c.Param("cmdId"), req.Signal)
	if err != nil {
		mapError(c, err)
		return
	}
	c.JSON(http.StatusOK, models.CommandResponse{Command: cmd})
}

func (h *Handler) getCommandLogs(c *gin.Context) {
	sandboxID := c.Param("id")
	cmdID := c.Param("cmdId")

	if c.Query("stream") == "true" {
		h.streamLogs(c, sandboxID, cmdID)
		return
	}
	logs, err := h.docker.GetCommandLogs(c.Request.Context(), sandboxID, cmdID)
	if err != nil {
		mapError(c, err)
		return
	}
	c.JSON(http.StatusOK, logs)
}

// --- File handlers ---

func (h *Handler) readFile(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, errorResponse{Code: "BAD_REQUEST", Message: "path query param is required"})
		return
	}
	content, err := h.docker.ReadFile(c.Request.Context(), c.Param("id"), path)
	if err != nil {
		mapError(c, err)
		return
	}
	c.JSON(http.StatusOK, models.FileReadResponse{Path: path, Content: content})
}

func (h *Handler) writeFile(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, errorResponse{Code: "BAD_REQUEST", Message: "path query param is required"})
		return
	}
	var req models.FileWriteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Code: "BAD_REQUEST", Message: err.Error()})
		return
	}
	if err := h.docker.WriteFile(c.Request.Context(), c.Param("id"), path, req.Content); err != nil {
		mapError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"path": path, "status": "written"})
}

func (h *Handler) deleteFile(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, errorResponse{Code: "BAD_REQUEST", Message: "path query param is required"})
		return
	}
	if err := h.docker.DeleteFile(c.Request.Context(), c.Param("id"), path); err != nil {
		mapError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) listDir(c *gin.Context) {
	path := c.DefaultQuery("path", "/")
	output, err := h.docker.ListDir(c.Request.Context(), c.Param("id"), path)
	if err != nil {
		mapError(c, err)
		return
	}
	c.JSON(http.StatusOK, models.FileListResponse{Path: path, Output: output})
}

// --- Image handlers ---

func (h *Handler) listImages(c *gin.Context) {
	images, err := h.docker.ListImages(c.Request.Context())
	if err != nil {
		mapError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"images": images})
}

func (h *Handler) inspectImage(c *gin.Context) {
	detail, err := h.docker.InspectImage(c.Request.Context(), c.Param("id"))
	if err != nil {
		mapError(c, err)
		return
	}
	c.JSON(http.StatusOK, detail)
}

func (h *Handler) pullImage(c *gin.Context) {
	var req models.ImagePullRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Code: "BAD_REQUEST", Message: err.Error()})
		return
	}
	if err := h.docker.PullImage(c.Request.Context(), req.Image); err != nil {
		mapError(c, err)
		return
	}
	c.JSON(http.StatusOK, models.ImagePullResponse{Status: "pulled", Image: req.Image})
}

func (h *Handler) deleteImage(c *gin.Context) {
	force := c.Query("force") == "true"
	if err := h.docker.RemoveImage(c.Request.Context(), c.Param("id"), force); err != nil {
		mapError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// --- Streaming helpers ---

func (h *Handler) streamLogs(c *gin.Context, sandboxID, cmdID string) {
	stdoutR, stderrR, err := h.docker.StreamCommandLogs(c.Request.Context(), sandboxID, cmdID)
	if err != nil {
		mapError(c, err)
		return
	}
	defer stdoutR.Close()
	defer stderrR.Close()

	c.Header("Content-Type", "application/x-ndjson")
	c.Status(http.StatusOK)
	flusher, _ := c.Writer.(http.Flusher)
	enc := json.NewEncoder(c.Writer)

	type logLine struct {
		Type string `json:"type"`
		Data string `json:"data"`
	}

	lines := make(chan logLine, 64)
	readStream := func(r io.ReadCloser, streamType string) {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			lines <- logLine{Type: streamType, Data: scanner.Text() + "\n"}
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); readStream(stdoutR, "stdout") }()
	go func() { defer wg.Done(); readStream(stderrR, "stderr") }()
	go func() { wg.Wait(); close(lines) }()

	for line := range lines {
		if c.IsAborted() {
			return
		}
		enc.Encode(line)
		if flusher != nil {
			flusher.Flush()
		}
	}
}

func (h *Handler) streamWait(c *gin.Context, sandboxID, cmdID string) {
	c.Header("Content-Type", "application/x-ndjson")
	c.Status(http.StatusOK)
	flusher, _ := c.Writer.(http.Flusher)
	enc := json.NewEncoder(c.Writer)

	cmd, err := h.docker.GetCommand(c.Request.Context(), sandboxID, cmdID)
	if err != nil {
		return
	}
	enc.Encode(models.CommandResponse{Command: cmd})
	if flusher != nil {
		flusher.Flush()
	}

	cmd, err = h.docker.WaitCommand(c.Request.Context(), sandboxID, cmdID)
	if err != nil {
		return
	}
	enc.Encode(models.CommandResponse{Command: cmd})
	if flusher != nil {
		flusher.Flush()
	}
}
