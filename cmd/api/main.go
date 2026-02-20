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

// @securityDefinitions.apikey  ApiKeyAuth
// @in                          header
// @name                        Authorization
// @description                 Enter "Bearer {your-api-key}"

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

	v1 := r.Group("/v1")
	if cfg.APIKey != "" {
		v1.Use(api.APIKeyAuth(cfg.APIKey))
	}

	h := api.New(docker.New())
	h.RegisterHealthCheck(r)
	h.RegisterRoutes(v1)

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))

	r.Run(cfg.Addr)
}
