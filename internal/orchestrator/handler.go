package orchestrator

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler exposes orchestrator-specific HTTP endpoints (worker registration).
type Handler struct {
	registry *WorkerRegistry
}

// NewHandler creates an orchestrator Handler.
func NewHandler(registry *WorkerRegistry) *Handler {
	return &Handler{registry: registry}
}

type registerRequest struct {
	URL string `json:"url" binding:"required"`
}

type registerResponse struct {
	WorkerID string `json:"worker_id"`
}

// RegisterWorker handles POST /internal/v1/workers/register.
func (h *Handler) RegisterWorker(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "message": err.Error()})
		return
	}

	// Use the API key from the request header as the worker's auth key.
	apiKey := c.GetHeader("X-Worker-Key")

	id, err := h.registry.Register(req.URL, apiKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, registerResponse{WorkerID: id})
}

// DeregisterWorker handles DELETE /internal/v1/workers/:id.
func (h *Handler) DeregisterWorker(c *gin.Context) {
	id := c.Param("id")
	if err := h.registry.Deregister(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deregistered"})
}

// ListWorkers handles GET /internal/v1/workers.
func (h *Handler) ListWorkers(c *gin.Context) {
	workers := h.registry.All()
	c.JSON(http.StatusOK, gin.H{"workers": workers})
}

// RegisterRoutes attaches worker management endpoints.
func (h *Handler) RegisterRoutes(g *gin.RouterGroup) {
	w := g.Group("/workers")
	w.POST("/register", h.RegisterWorker)
	w.GET("", h.ListWorkers)
	w.DELETE("/:id", h.DeregisterWorker)
}
