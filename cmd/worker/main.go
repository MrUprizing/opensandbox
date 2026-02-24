package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"open-sandbox/internal/config"
	"open-sandbox/internal/docker"
	"open-sandbox/internal/worker"
)

func main() {
	cfg := config.LoadWorker()

	dc := docker.New(docker.WithHostIP(cfg.HostIP))

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	internal := r.Group("/internal/v1")
	if cfg.APIKey != "" {
		internal.Use(worker.APIKeyAuth(cfg.APIKey))
	}

	h := worker.NewHandler(dc)
	h.RegisterRoutes(internal)

	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    "NOT_FOUND",
			"message": "route not found",
		})
	})

	// Graceful shutdown context.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Self-register with orchestrator if configured.
	var workerID string
	if cfg.OrchestratorURL != "" && cfg.APIKey != "" {
		var err error
		workerID, err = registerWorker(cfg)
		if err != nil {
			log.Fatalf("worker registration failed: %v", err)
		}
		log.Printf("registered with orchestrator as worker %s", workerID)
	} else {
		log.Println("no orchestrator configured, running standalone")
	}

	srv := &http.Server{Addr: cfg.Addr, Handler: r}

	go func() {
		log.Printf("worker listening on %s (host_ip: %s)", cfg.Addr, cfg.HostIP)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("worker listen: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down worker...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Deregister from orchestrator.
	if workerID != "" && cfg.OrchestratorURL != "" {
		if err := deregisterWorker(cfg, workerID); err != nil {
			log.Printf("worker deregistration failed: %v", err)
		} else {
			log.Printf("deregistered worker %s", workerID)
		}
	}

	dc.Shutdown(shutdownCtx)
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("worker shutdown: %v", err)
	}

	log.Println("worker stopped")
}

// registerWorker calls POST {orchestratorURL}/internal/v1/workers/register.
func registerWorker(cfg *config.WorkerConfig) (string, error) {
	body, _ := json.Marshal(map[string]string{
		"url": workerURL(cfg),
	})

	req, err := http.NewRequest(http.MethodPost, cfg.OrchestratorURL+"/internal/v1/workers/register", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Worker-Key", cfg.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("connect to orchestrator: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("orchestrator returned %d", resp.StatusCode)
	}

	var result struct {
		WorkerID string `json:"worker_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	return result.WorkerID, nil
}

// deregisterWorker calls DELETE {orchestratorURL}/internal/v1/workers/{id}.
func deregisterWorker(cfg *config.WorkerConfig, workerID string) error {
	req, err := http.NewRequest(http.MethodDelete, cfg.OrchestratorURL+"/internal/v1/workers/"+workerID, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Worker-Key", cfg.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("orchestrator returned %d", resp.StatusCode)
	}
	return nil
}

// workerURL builds the worker's externally reachable URL from its config.
func workerURL(cfg *config.WorkerConfig) string {
	// If addr starts with ":", prepend the hostname or default to the host IP.
	addr := cfg.Addr
	if addr[0] == ':' {
		addr = cfg.HostIP + addr
	}
	return "http://" + addr
}
