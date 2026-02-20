package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"open-sandbox/models"
)

// Handler holds dependencies for all API handlers.
type Handler struct {
	docker DockerClient
}

// New creates a Handler with the given Docker client.
func New(d DockerClient) *Handler {
	return &Handler{docker: d}
}

// healthCheck handles GET /health.
// @Summary      Health check
// @Description  Returns the health status of the API and its Docker daemon connection.
// @Tags         system
// @Produce      json
// @Success      200  {object}  map[string]string  "status: healthy"
// @Failure      503  {object}  map[string]string  "status: unhealthy"
// @Router       /health [get]
func (h *Handler) healthCheck(c *gin.Context) {
	if err := h.docker.Ping(c.Request.Context()); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "unhealthy",
			"error":  err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "healthy"})
}

// listSandboxes handles GET /v1/sandboxes.
// @Summary      List sandboxes
// @Description  List all active sandboxes. Use ?all=true to include stopped containers.
// @Tags         sandboxes
// @Produce      json
// @Param        all  query  bool  false  "Include stopped containers"
// @Success      200  {object}  map[string]interface{}  "List of sandboxes"
// @Failure      500  {object}  ErrorResponse
// @Security     ApiKeyAuth
// @Router       /sandboxes [get]
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
// @Summary      Create a sandbox
// @Description  Create and start a new Docker container. Returns its ID and assigned host ports.
// @Tags         sandboxes
// @Accept       json
// @Produce      json
// @Param        body  body      models.CreateSandboxRequest  true  "Sandbox configuration"
// @Success      201   {object}  models.CreateSandboxResponse
// @Failure      400   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Security     ApiKeyAuth
// @Router       /sandboxes [post]
func (h *Handler) createSandbox(c *gin.Context) {
	var req models.CreateSandboxRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}

	if req.Timeout < 0 {
		badRequest(c, "timeout must be >= 0")
		return
	}
	if req.Resources != nil {
		if req.Resources.Memory < 0 {
			badRequest(c, "resources.memory must be >= 0")
			return
		}
		if req.Resources.CPUs < 0 {
			badRequest(c, "resources.cpus must be >= 0")
			return
		}
	}

	result, err := h.docker.Create(c.Request.Context(), req)
	if err != nil {
		internalError(c, err)
		return
	}

	c.JSON(http.StatusCreated, result)
}

// getSandbox handles GET /v1/sandboxes/:id.
// @Summary      Inspect a sandbox
// @Description  Returns full Docker inspect data for the sandbox.
// @Tags         sandboxes
// @Produce      json
// @Param        id   path      string  true  "Sandbox ID"
// @Success      200  {object}  map[string]interface{}  "Docker inspect response"
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Security     ApiKeyAuth
// @Router       /sandboxes/{id} [get]
func (h *Handler) getSandbox(c *gin.Context) {
	info, err := h.docker.Inspect(c.Request.Context(), c.Param("id"))
	if err != nil {
		internalError(c, err)
		return
	}

	c.JSON(http.StatusOK, info)
}

// stopSandbox handles POST /v1/sandboxes/:id/stop.
// @Summary      Stop a sandbox
// @Description  Gracefully stop a running sandbox.
// @Tags         sandboxes
// @Produce      json
// @Param        id   path      string  true  "Sandbox ID"
// @Success      200  {object}  map[string]string  "status: stopped"
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Security     ApiKeyAuth
// @Router       /sandboxes/{id}/stop [post]
func (h *Handler) stopSandbox(c *gin.Context) {
	if err := h.docker.Stop(c.Request.Context(), c.Param("id")); err != nil {
		internalError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "stopped"})
}

// restartSandbox handles POST /v1/sandboxes/:id/restart.
// @Summary      Restart a sandbox
// @Description  Restart a sandbox (stop + start).
// @Tags         sandboxes
// @Produce      json
// @Param        id   path      string  true  "Sandbox ID"
// @Success      200  {object}  map[string]string  "status: restarted"
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Security     ApiKeyAuth
// @Router       /sandboxes/{id}/restart [post]
func (h *Handler) restartSandbox(c *gin.Context) {
	if err := h.docker.Restart(c.Request.Context(), c.Param("id")); err != nil {
		internalError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "restarted"})
}

// deleteSandbox handles DELETE /v1/sandboxes/:id.
// @Summary      Delete a sandbox
// @Description  Force-remove a sandbox regardless of its state.
// @Tags         sandboxes
// @Param        id   path      string  true  "Sandbox ID"
// @Success      204  "No Content"
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Security     ApiKeyAuth
// @Router       /sandboxes/{id} [delete]
func (h *Handler) deleteSandbox(c *gin.Context) {
	if err := h.docker.Remove(c.Request.Context(), c.Param("id")); err != nil {
		internalError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// execSandbox handles POST /v1/sandboxes/:id/exec.
// @Summary      Execute a command
// @Description  Run an arbitrary command inside the sandbox and return combined stdout+stderr.
// @Tags         sandboxes
// @Accept       json
// @Produce      json
// @Param        id    path      string             true  "Sandbox ID"
// @Param        body  body      models.ExecRequest  true  "Command to execute"
// @Success      200   {object}  models.ExecResponse
// @Failure      400   {object}  ErrorResponse
// @Failure      404   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Security     ApiKeyAuth
// @Router       /sandboxes/{id}/exec [post]
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
// @Summary      Read a file
// @Description  Returns the content of a file at the given path inside the sandbox.
// @Tags         files
// @Produce      json
// @Param        id    path      string  true  "Sandbox ID"
// @Param        path  query     string  true  "File path inside the sandbox"
// @Success      200   {object}  models.FileReadResponse
// @Failure      400   {object}  ErrorResponse
// @Failure      404   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Security     ApiKeyAuth
// @Router       /sandboxes/{id}/files [get]
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
// @Summary      Write a file
// @Description  Write or overwrite a file inside the sandbox. Creates parent directories as needed.
// @Tags         files
// @Accept       json
// @Produce      json
// @Param        id    path      string                  true  "Sandbox ID"
// @Param        path  query     string                  true  "File path inside the sandbox"
// @Param        body  body      models.FileWriteRequest  true  "File content"
// @Success      200   {object}  map[string]string  "path and status"
// @Failure      400   {object}  ErrorResponse
// @Failure      404   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Security     ApiKeyAuth
// @Router       /sandboxes/{id}/files [put]
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
// @Summary      Delete a file
// @Description  Remove a file or directory (recursive) inside the sandbox.
// @Tags         files
// @Param        id    path      string  true  "Sandbox ID"
// @Param        path  query     string  true  "File path inside the sandbox"
// @Success      204  "No Content"
// @Failure      400   {object}  ErrorResponse
// @Failure      404   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Security     ApiKeyAuth
// @Router       /sandboxes/{id}/files [delete]
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
// @Summary      List a directory
// @Description  Returns the output of ls -la for the given directory. Defaults to root (/).
// @Tags         files
// @Produce      json
// @Param        id    path      string  true   "Sandbox ID"
// @Param        path  query     string  false  "Directory path (default: /)"
// @Success      200   {object}  models.FileListResponse
// @Failure      404   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Security     ApiKeyAuth
// @Router       /sandboxes/{id}/files/list [get]
func (h *Handler) listDir(c *gin.Context) {
	path := c.DefaultQuery("path", "/")

	output, err := h.docker.ListDir(c.Request.Context(), c.Param("id"), path)
	if err != nil {
		internalError(c, err)
		return
	}

	c.JSON(http.StatusOK, models.FileListResponse{Path: path, Output: output})
}

// pauseSandbox handles POST /v1/sandboxes/:id/pause.
// @Summary      Pause a sandbox
// @Description  Freeze all processes inside the sandbox.
// @Tags         sandboxes
// @Produce      json
// @Param        id   path      string  true  "Sandbox ID"
// @Success      200  {object}  map[string]string  "status: paused"
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Security     ApiKeyAuth
// @Router       /sandboxes/{id}/pause [post]
func (h *Handler) pauseSandbox(c *gin.Context) {
	if err := h.docker.Pause(c.Request.Context(), c.Param("id")); err != nil {
		internalError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "paused"})
}

// resumeSandbox handles POST /v1/sandboxes/:id/resume.
// @Summary      Resume a sandbox
// @Description  Resume a paused sandbox.
// @Tags         sandboxes
// @Produce      json
// @Param        id   path      string  true  "Sandbox ID"
// @Success      200  {object}  map[string]string  "status: resumed"
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Security     ApiKeyAuth
// @Router       /sandboxes/{id}/resume [post]
func (h *Handler) resumeSandbox(c *gin.Context) {
	if err := h.docker.Resume(c.Request.Context(), c.Param("id")); err != nil {
		internalError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "resumed"})
}

// renewExpiration handles POST /v1/sandboxes/:id/renew-expiration.
// @Summary      Renew sandbox expiration
// @Description  Reset the auto-stop timer for a sandbox.
// @Tags         sandboxes
// @Accept       json
// @Produce      json
// @Param        id    path      string                          true  "Sandbox ID"
// @Param        body  body      models.RenewExpirationRequest   true  "New timeout"
// @Success      200   {object}  models.RenewExpirationResponse
// @Failure      400   {object}  ErrorResponse
// @Failure      404   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Security     ApiKeyAuth
// @Router       /sandboxes/{id}/renew-expiration [post]
func (h *Handler) renewExpiration(c *gin.Context) {
	var req models.RenewExpirationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}

	if req.Timeout <= 0 {
		badRequest(c, "timeout must be > 0")
		return
	}

	if err := h.docker.RenewExpiration(c.Request.Context(), c.Param("id"), req.Timeout); err != nil {
		internalError(c, err)
		return
	}

	c.JSON(http.StatusOK, models.RenewExpirationResponse{Status: "renewed", Timeout: req.Timeout})
}
