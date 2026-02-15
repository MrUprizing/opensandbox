package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"open-sandbox/models"
	"open-sandbox/sandbox"
)

// ListSandboxes handles GET /v1/sandboxes
func ListSandboxes(c *gin.Context) {
	all := c.Query("all") == "true"

	items, err := sandbox.List(c.Request.Context(), all)
	if err != nil {
		internalError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"sandboxes": items})
}

// CreateSandbox handles POST /v1/sandboxes
func CreateSandbox(c *gin.Context) {
	var req models.CreateSandboxRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}

	result, err := sandbox.Create(c.Request.Context(), req)
	if err != nil {
		internalError(c, err)
		return
	}

	c.JSON(http.StatusCreated, result)
}

// GetSandbox handles GET /v1/sandboxes/:id
func GetSandbox(c *gin.Context) {
	id := c.Param("id")

	info, err := sandbox.Inspect(c.Request.Context(), id)
	if err != nil {
		internalError(c, err)
		return
	}

	c.JSON(http.StatusOK, info)
}

// StopSandbox handles POST /v1/sandboxes/:id/stop
func StopSandbox(c *gin.Context) {
	id := c.Param("id")

	if err := sandbox.Stop(c.Request.Context(), id); err != nil {
		internalError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "stopped"})
}

// RestartSandbox handles POST /v1/sandboxes/:id/restart
func RestartSandbox(c *gin.Context) {
	id := c.Param("id")

	if err := sandbox.Restart(c.Request.Context(), id); err != nil {
		internalError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "restarted"})
}

// DeleteSandbox handles DELETE /v1/sandboxes/:id
func DeleteSandbox(c *gin.Context) {
	id := c.Param("id")

	if err := sandbox.Remove(c.Request.Context(), id); err != nil {
		internalError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// ExecSandbox handles POST /v1/sandboxes/:id/exec
func ExecSandbox(c *gin.Context) {
	id := c.Param("id")

	var req models.ExecRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}

	output, err := sandbox.Exec(c.Request.Context(), id, req.Cmd)
	if err != nil {
		internalError(c, err)
		return
	}

	c.JSON(http.StatusOK, models.ExecResponse{Output: output})
}
