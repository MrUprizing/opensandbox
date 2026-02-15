package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"open-sandbox/docker"
	"open-sandbox/models"
)

// ExecContainer handles POST /containers/:id/exec
func ExecContainer(c *gin.Context) {
	id := c.Param("id")

	var req models.ExecRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	output, err := docker.Exec(c.Request.Context(), id, req.Cmd)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"output": output})
}
