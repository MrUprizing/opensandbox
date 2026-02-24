package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"open-sandbox/internal/api"
	"open-sandbox/internal/config"
	"open-sandbox/internal/database"
	"open-sandbox/internal/orchestrator"
	"open-sandbox/internal/proxy"
	"open-sandbox/internal/worker"
)

func main() {
	cfg := config.LoadOrchestrator()

	db := database.New("sandbox.db")
	repo := database.NewRepository(db)

	// Worker registry (loads active workers from DB).
	registry := orchestrator.NewRegistry(repo)

	// RemoteDockerClient — implements DockerClient via HTTP to workers.
	dc := orchestrator.NewRemoteClient(registry, repo)

	// --- Reverse proxy (multi-listen) ---
	proxyServer := proxy.New(cfg.BaseDomain, repo)
	proxyHandler := proxyServer.Handler()

	var proxySrvs []*http.Server
	for _, addr := range cfg.ProxyAddrs {
		srv := &http.Server{Addr: addr, Handler: proxyHandler}
		proxySrvs = append(proxySrvs, srv)
		go func(a string) {
			log.Printf("proxy listening on %s (domain: *.%s)", a, cfg.BaseDomain)
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("proxy listen %s: %v", a, err)
			}
		}(addr)
	}
	log.Printf("proxy URLs via %s", strings.Join(cfg.ProxyAddrs, ", "))

	// --- API server ---
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	// Public API (same as all-in-one, but backed by RemoteDockerClient).
	v1 := r.Group("/v1")
	if cfg.APIKey != "" {
		v1.Use(api.APIKeyAuth(cfg.APIKey))
	}

	h := api.New(dc, cfg.BaseDomain, cfg.PrimaryProxyAddr())
	h.RegisterHealthCheck(r)
	h.RegisterRoutes(v1)

	// Internal API for worker registration.
	internal := r.Group("/internal/v1")
	if cfg.WorkerKey != "" {
		internal.Use(worker.APIKeyAuth(cfg.WorkerKey))
	}

	oh := orchestrator.NewHandler(registry)
	oh.RegisterRoutes(internal)

	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    "NOT_FOUND",
			"message": "route not found",
		})
	})

	// Graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	srv := &http.Server{Addr: cfg.Addr, Handler: r}

	go func() {
		log.Printf("orchestrator API listening on %s", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("api listen: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down orchestrator...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, ps := range proxySrvs {
		if err := ps.Shutdown(shutdownCtx); err != nil {
			log.Printf("proxy shutdown %s: %v", ps.Addr, err)
		}
	}
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("api shutdown: %v", err)
	}

	log.Println("orchestrator stopped")
}
