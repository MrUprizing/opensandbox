package worker

import "github.com/gin-gonic/gin"

// RegisterRoutes attaches all internal worker API routes to the given router group.
// All routes are prefixed with /internal/v1.
func (h *Handler) RegisterRoutes(g *gin.RouterGroup) {
	// Sandboxes
	sb := g.Group("/sandboxes")
	sb.GET("", h.listSandboxes)
	sb.POST("", h.createSandbox)
	sb.GET("/:id", h.inspectSandbox)
	sb.DELETE("/:id", h.deleteSandbox)
	sb.POST("/:id/start", h.startSandbox)
	sb.POST("/:id/stop", h.stopSandbox)
	sb.POST("/:id/restart", h.restartSandbox)
	sb.POST("/:id/pause", h.pauseSandbox)
	sb.POST("/:id/resume", h.resumeSandbox)
	sb.POST("/:id/renew-expiration", h.renewExpiration)
	sb.GET("/:id/stats", h.getStats)

	// Commands
	sb.POST("/:id/cmd", h.execCommand)
	sb.GET("/:id/cmd", h.listCommands)
	sb.GET("/:id/cmd/:cmdId", h.getCommand)
	sb.POST("/:id/cmd/:cmdId/kill", h.killCommand)
	sb.GET("/:id/cmd/:cmdId/logs", h.getCommandLogs)

	// Files
	sb.GET("/:id/files", h.readFile)
	sb.PUT("/:id/files", h.writeFile)
	sb.DELETE("/:id/files", h.deleteFile)
	sb.GET("/:id/files/list", h.listDir)

	// Images
	img := g.Group("/images")
	img.GET("", h.listImages)
	img.GET("/:id", h.inspectImage)
	img.POST("/pull", h.pullImage)
	img.DELETE("/:id", h.deleteImage)

	// Health (no auth — registered outside the auth group in main)
	g.GET("/health", h.health)
}
