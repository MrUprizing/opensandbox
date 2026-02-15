package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"open-sandbox/docker"
	"open-sandbox/models"
)

// ListContainers handles GET /containers
func ListContainers(c *gin.Context) {
	all := c.Query("all") == "true"

	containers, err := docker.List(c.Request.Context(), all)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, containers)
}

// CreateContainer handles POST /containers
func CreateContainer(c *gin.Context) {
	var req models.CreateContainerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := docker.Create(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, result)
}

// InspectContainer handles GET /containers/:id
func InspectContainer(c *gin.Context) {
	id := c.Param("id")

	info, err := docker.Inspect(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, info)
}

// StopContainer handles POST /containers/:id/stop
func StopContainer(c *gin.Context) {
	id := c.Param("id")

	if err := docker.Stop(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "stopped"})
}

// RestartContainer handles POST /containers/:id/restart
func RestartContainer(c *gin.Context) {
	id := c.Param("id")

	if err := docker.Restart(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "restarted"})
}

// RemoveContainer handles DELETE /containers/:id
func RemoveContainer(c *gin.Context) {
	id := c.Param("id")

	if err := docker.Remove(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}
