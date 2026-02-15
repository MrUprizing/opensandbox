package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"open-sandbox/handlers"
)

func main() {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	// 404 handler
	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    "NOT_FOUND",
			"message": "route not found",
		})
	})

	v1 := r.Group("/v1")
	{
		sb := v1.Group("/sandboxes")
		sb.GET("", handlers.ListSandboxes)
		sb.POST("", handlers.CreateSandbox)
		sb.GET("/:id", handlers.GetSandbox)
		sb.DELETE("/:id", handlers.DeleteSandbox)
		sb.POST("/:id/stop", handlers.StopSandbox)
		sb.POST("/:id/restart", handlers.RestartSandbox)
		sb.POST("/:id/exec", handlers.ExecSandbox)
	}

	r.Run(":8080")
}
