package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"open-sandbox/internal/api"
	"open-sandbox/internal/config"
	"open-sandbox/internal/docker"

	_ "open-sandbox/docs"
)

// @title           Open Sandbox API
// @version         1.0
// @description     Docker sandbox orchestrator REST API. Create, manage, and execute commands inside isolated Docker containers.

// @host      localhost:8080
// @BasePath  /v1

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

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))

	r.Run(cfg.Addr)
}
