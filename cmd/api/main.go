package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"open-sandbox/internal/api"
	"open-sandbox/internal/config"
	"open-sandbox/internal/database"
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

	db := database.New("sandbox.db")
	repo := database.NewRepository(db)
	dc := docker.New(repo)

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

	h := api.New(dc)
	h.RegisterHealthCheck(r)
	h.RegisterRoutes(v1)

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))

	// Graceful shutdown: listen for SIGINT/SIGTERM, then stop tracked containers.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	srv := &http.Server{Addr: cfg.Addr, Handler: r}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down: stopping tracked sandboxes...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dc.Shutdown(shutdownCtx)
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("server shutdown: %v", err)
	}

	log.Println("server stopped")
}
