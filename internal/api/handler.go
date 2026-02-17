package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"open-sandbox/internal/docker"
	"open-sandbox/models"
)

// Handler holds dependencies for all API handlers.
type Handler struct {
	docker *docker.Client
}

// New creates a Handler with the given Docker client.
func New(d *docker.Client) *Handler {
	return &Handler{docker: d}
}

// listSandboxes handles GET /v1/sandboxes.
// Accepts ?all=true to include stopped containers.
func (h *Handler) listSandboxes(c *gin.Context) {
	all := c.Query("all") == "true"

	items, err := h.docker.List(c.Request.Context(), all)
	if err != nil {
		internalError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"sandboxes": items})
}

// createSandbox handles POST /v1/sandboxes.
// Creates and starts a container; returns its ID and assigned host ports.
func (h *Handler) createSandbox(c *gin.Context) {
	var req models.CreateSandboxRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}

	result, err := h.docker.Create(c.Request.Context(), req)
	if err != nil {
		internalError(c, err)
		return
	}

	c.JSON(http.StatusCreated, result)
}

// getSandbox handles GET /v1/sandboxes/:id.
// Returns full Docker inspect data for the sandbox.
func (h *Handler) getSandbox(c *gin.Context) {
	info, err := h.docker.Inspect(c.Request.Context(), c.Param("id"))
	if err != nil {
		internalError(c, err)
		return
	}

	c.JSON(http.StatusOK, info)
}

// stopSandbox handles POST /v1/sandboxes/:id/stop.
// Gracefully stops a running sandbox.
func (h *Handler) stopSandbox(c *gin.Context) {
	if err := h.docker.Stop(c.Request.Context(), c.Param("id")); err != nil {
		internalError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "stopped"})
}

// restartSandbox handles POST /v1/sandboxes/:id/restart.
// Restarts a sandbox (stop + start).
func (h *Handler) restartSandbox(c *gin.Context) {
	if err := h.docker.Restart(c.Request.Context(), c.Param("id")); err != nil {
		internalError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "restarted"})
}

// deleteSandbox handles DELETE /v1/sandboxes/:id.
// Force-removes the sandbox regardless of its state. Returns 204 on success.
func (h *Handler) deleteSandbox(c *gin.Context) {
	if err := h.docker.Remove(c.Request.Context(), c.Param("id")); err != nil {
		internalError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// execSandbox handles POST /v1/sandboxes/:id/exec.
// Runs an arbitrary command inside the sandbox and returns combined stdout+stderr.
func (h *Handler) execSandbox(c *gin.Context) {
	var req models.ExecRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}

	output, err := h.docker.Exec(c.Request.Context(), c.Param("id"), req.Cmd)
	if err != nil {
		internalError(c, err)
		return
	}

	c.JSON(http.StatusOK, models.ExecResponse{Output: output})
}

// readFile handles GET /v1/sandboxes/:id/files?path=<path>.
// Returns the content of the file at the given path inside the sandbox.
func (h *Handler) readFile(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		badRequest(c, "path query param is required")
		return
	}

	content, err := h.docker.ReadFile(c.Request.Context(), c.Param("id"), path)
	if err != nil {
		internalError(c, err)
		return
	}

	c.JSON(http.StatusOK, models.FileReadResponse{Path: path, Content: content})
}

// writeFile handles PUT /v1/sandboxes/:id/files?path=<path>.
// Writes or overwrites a file inside the sandbox; creates parent dirs as needed.
func (h *Handler) writeFile(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		badRequest(c, "path query param is required")
		return
	}

	var req models.FileWriteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}

	if err := h.docker.WriteFile(c.Request.Context(), c.Param("id"), path, req.Content); err != nil {
		internalError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"path": path, "status": "written"})
}

// deleteFile handles DELETE /v1/sandboxes/:id/files?path=<path>.
// Removes a file or directory (recursive) inside the sandbox. Returns 204 on success.
func (h *Handler) deleteFile(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		badRequest(c, "path query param is required")
		return
	}

	if err := h.docker.DeleteFile(c.Request.Context(), c.Param("id"), path); err != nil {
		internalError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// listDir handles GET /v1/sandboxes/:id/files/list?path=<path>.
// Returns the output of `ls -la` for the given directory. Defaults to "/".
func (h *Handler) listDir(c *gin.Context) {
	path := c.DefaultQuery("path", "/")

	output, err := h.docker.ListDir(c.Request.Context(), c.Param("id"), path)
	if err != nil {
		internalError(c, err)
		return
	}

	c.JSON(http.StatusOK, models.FileListResponse{Path: path, Output: output})
}
