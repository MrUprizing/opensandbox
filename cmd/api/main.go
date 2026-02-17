package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"open-sandbox/internal/api"
	"open-sandbox/internal/config"
	"open-sandbox/internal/docker"
)

func main() {
	cfg := config.Load()

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    "NOT_FOUND",
			"message": "route not found",
		})
	})

	h := api.New(docker.New())
	h.RegisterRoutes(r.Group("/v1"))

	r.Run(cfg.Addr)
}
