package api

import "github.com/gin-gonic/gin"

// RegisterHealthCheck attaches the /v1/health endpoint directly to the engine (no auth).
func (h *Handler) RegisterHealthCheck(r *gin.Engine) {
	r.GET("/v1/health", h.healthCheck)
}

// RegisterRoutes attaches all sandbox routes to the given router group.
func (h *Handler) RegisterRoutes(v1 *gin.RouterGroup) {
	sb := v1.Group("/sandboxes")
	sb.GET("", h.listSandboxes)
	sb.POST("", h.createSandbox)
	sb.GET("/:id", h.getSandbox)
	sb.DELETE("/:id", h.deleteSandbox)
	sb.POST("/:id/stop", h.stopSandbox)
	sb.POST("/:id/restart", h.restartSandbox)
	sb.POST("/:id/pause", h.pauseSandbox)
	sb.POST("/:id/resume", h.resumeSandbox)
	sb.POST("/:id/renew-expiration", h.renewExpiration)
	sb.POST("/:id/exec", h.execSandbox)
	sb.GET("/:id/files", h.readFile)
	sb.PUT("/:id/files", h.writeFile)
	sb.DELETE("/:id/files", h.deleteFile)
	sb.GET("/:id/files/list", h.listDir)

	img := v1.Group("/images")
	img.GET("", h.listImages)
	img.POST("/pull", h.pullImage)
}
