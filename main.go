package main

import (
	"github.com/gin-gonic/gin"
	"open-sandbox/handlers"
)

func main() {
	r := gin.Default()

	c := r.Group("/containers")
	{
		c.GET("", handlers.ListContainers)
		c.POST("", handlers.CreateContainer)
		c.GET("/:id", handlers.InspectContainer)
		c.POST("/:id/stop", handlers.StopContainer)
		c.POST("/:id/restart", handlers.RestartContainer)
		c.DELETE("/:id", handlers.RemoveContainer)
		c.POST("/:id/exec", handlers.ExecContainer)
	}

	r.Run(":8080")
}
