package orchestrator_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"open-sandbox/internal/database"
	"open-sandbox/internal/orchestrator"
	"open-sandbox/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockWorker creates an httptest.Server that mimics the worker internal API.
func mockWorker(t *testing.T) *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /internal/v1/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
	})

	mux.HandleFunc("POST /internal/v1/sandboxes", func(w http.ResponseWriter, r *http.Request) {
		var req models.CreateSandboxRequest
		json.NewDecoder(r.Body).Decode(&req)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(models.CreateSandboxResponse{
			ID:    "abc123",
			Name:  req.Name,
			Ports: []string{"3000/tcp"},
		})
	})

	mux.HandleFunc("GET /internal/v1/sandboxes/{id}", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(models.SandboxDetail{
			ID:      r.PathValue("id"),
			Name:    "test-sandbox",
			Image:   "node:24",
			Status:  "running",
			Running: true,
		})
	})

	mux.HandleFunc("DELETE /internal/v1/sandboxes/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("POST /internal/v1/sandboxes/{id}/stop", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "stopped"})
	})

	mux.HandleFunc("POST /internal/v1/sandboxes/{id}/start", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(models.RestartResponse{
			Status: "started",
			Ports:  []string{"3000/tcp"},
		})
	})

	mux.HandleFunc("POST /internal/v1/sandboxes/{id}/restart", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(models.RestartResponse{
			Status: "restarted",
			Ports:  []string{"3000/tcp"},
		})
	})

	mux.HandleFunc("POST /internal/v1/sandboxes/{id}/pause", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "paused"})
	})

	mux.HandleFunc("POST /internal/v1/sandboxes/{id}/resume", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "resumed"})
	})

	mux.HandleFunc("GET /internal/v1/sandboxes/{id}/stats", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(models.SandboxStats{CPU: 1.5, PIDs: 10})
	})

	mux.HandleFunc("GET /internal/v1/sandboxes/{id}/cmd", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(models.CommandListResponse{Commands: []models.CommandDetail{}})
	})

	mux.HandleFunc("POST /internal/v1/sandboxes/{id}/cmd", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(models.CommandResponse{
			Command: models.CommandDetail{ID: "cmd_abc", Name: "ls", SandboxID: r.PathValue("id")},
		})
	})

	mux.HandleFunc("GET /internal/v1/sandboxes/{id}/files", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(models.FileReadResponse{
			Path: r.URL.Query().Get("path"), Content: "hello",
		})
	})

	mux.HandleFunc("PUT /internal/v1/sandboxes/{id}/files", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "written"})
	})

	mux.HandleFunc("DELETE /internal/v1/sandboxes/{id}/files", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("GET /internal/v1/sandboxes/{id}/files/list", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(models.FileListResponse{
			Path: r.URL.Query().Get("path"), Output: "drwxr-xr-x 2 root root 4096 Jan 1 00:00 .",
		})
	})

	mux.HandleFunc("GET /internal/v1/images", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"images": []models.ImageSummary{
				{ID: "sha256:abc", Tags: []string{"node:24"}, Size: 100},
			},
		})
	})

	mux.HandleFunc("GET /internal/v1/images/{id}", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(models.ImageDetail{
			ID: "sha256:abc", Tags: []string{"node:24"}, Size: 100,
		})
	})

	mux.HandleFunc("POST /internal/v1/images/pull", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(models.ImagePullResponse{Status: "pulled"})
	})

	mux.HandleFunc("DELETE /internal/v1/images/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	return httptest.NewServer(mux)
}

// setupClient creates a RemoteDockerClient with a mock worker registered.
func setupClient(t *testing.T, workerURL string) (*orchestrator.RemoteDockerClient, *database.Repository) {
	db := database.New(":memory:")
	repo := database.NewRepository(db)
	reg := orchestrator.NewRegistry(repo)

	// Register mock worker.
	workerID, err := reg.Register(workerURL, "test-key")
	require.NoError(t, err)

	// Pre-create a sandbox record so workerForSandbox lookups work.
	repo.Save(database.Sandbox{
		ID:       "abc123",
		Name:     "test-sandbox",
		Image:    "node:24",
		WorkerID: workerID,
	})

	client := orchestrator.NewRemoteClient(reg, repo)
	return client, repo
}

func TestRemoteClient_Ping(t *testing.T) {
	srv := mockWorker(t)
	defer srv.Close()
	client, _ := setupClient(t, srv.URL)

	err := client.Ping(context.Background())
	assert.NoError(t, err)
}

func TestRemoteClient_Create(t *testing.T) {
	srv := mockWorker(t)
	defer srv.Close()
	client, repo := setupClient(t, srv.URL)

	resp, err := client.Create(context.Background(), models.CreateSandboxRequest{
		Image: "node:24",
		Ports: []string{"3000"},
	})
	require.NoError(t, err)
	assert.Equal(t, "abc123", resp.ID)
	assert.NotEmpty(t, resp.Name)

	// Verify persisted in orchestrator DB.
	sb, err := repo.FindByID("abc123")
	require.NoError(t, err)
	require.NotNil(t, sb)
	assert.NotEmpty(t, sb.WorkerID)
}

func TestRemoteClient_Inspect(t *testing.T) {
	srv := mockWorker(t)
	defer srv.Close()
	client, _ := setupClient(t, srv.URL)

	detail, err := client.Inspect(context.Background(), "abc123")
	require.NoError(t, err)
	assert.Equal(t, "abc123", detail.ID)
	assert.True(t, detail.Running)
}

func TestRemoteClient_InspectNotFound(t *testing.T) {
	srv := mockWorker(t)
	defer srv.Close()
	client, _ := setupClient(t, srv.URL)

	_, err := client.Inspect(context.Background(), "nonexistent")
	assert.Error(t, err)
}

func TestRemoteClient_Stop(t *testing.T) {
	srv := mockWorker(t)
	defer srv.Close()
	client, _ := setupClient(t, srv.URL)

	err := client.Stop(context.Background(), "abc123")
	assert.NoError(t, err)
}

func TestRemoteClient_Start(t *testing.T) {
	srv := mockWorker(t)
	defer srv.Close()
	client, _ := setupClient(t, srv.URL)

	resp, err := client.Start(context.Background(), "abc123")
	require.NoError(t, err)
	assert.Equal(t, "started", resp.Status)
}

func TestRemoteClient_Restart(t *testing.T) {
	srv := mockWorker(t)
	defer srv.Close()
	client, _ := setupClient(t, srv.URL)

	resp, err := client.Restart(context.Background(), "abc123")
	require.NoError(t, err)
	assert.Equal(t, "restarted", resp.Status)
}

func TestRemoteClient_Remove(t *testing.T) {
	srv := mockWorker(t)
	defer srv.Close()
	client, repo := setupClient(t, srv.URL)

	err := client.Remove(context.Background(), "abc123")
	require.NoError(t, err)

	// Verify cleaned up from DB.
	sb, _ := repo.FindByID("abc123")
	assert.Nil(t, sb)
}

func TestRemoteClient_Pause(t *testing.T) {
	srv := mockWorker(t)
	defer srv.Close()
	client, _ := setupClient(t, srv.URL)

	err := client.Pause(context.Background(), "abc123")
	assert.NoError(t, err)
}

func TestRemoteClient_Resume(t *testing.T) {
	srv := mockWorker(t)
	defer srv.Close()
	client, _ := setupClient(t, srv.URL)

	err := client.Resume(context.Background(), "abc123")
	assert.NoError(t, err)
}

func TestRemoteClient_Stats(t *testing.T) {
	srv := mockWorker(t)
	defer srv.Close()
	client, _ := setupClient(t, srv.URL)

	stats, err := client.Stats(context.Background(), "abc123")
	require.NoError(t, err)
	assert.Equal(t, 1.5, stats.CPU)
	assert.Equal(t, uint64(10), stats.PIDs)
}

func TestRemoteClient_ExecCommand(t *testing.T) {
	srv := mockWorker(t)
	defer srv.Close()
	client, _ := setupClient(t, srv.URL)

	cmd, err := client.ExecCommand(context.Background(), "abc123", models.ExecCommandRequest{
		Command: "ls",
	})
	require.NoError(t, err)
	assert.Equal(t, "cmd_abc", cmd.ID)
}

func TestRemoteClient_ListCommands(t *testing.T) {
	srv := mockWorker(t)
	defer srv.Close()
	client, _ := setupClient(t, srv.URL)

	cmds, err := client.ListCommands(context.Background(), "abc123")
	require.NoError(t, err)
	assert.NotNil(t, cmds)
}

func TestRemoteClient_ReadFile(t *testing.T) {
	srv := mockWorker(t)
	defer srv.Close()
	client, _ := setupClient(t, srv.URL)

	content, err := client.ReadFile(context.Background(), "abc123", "/app/index.js")
	require.NoError(t, err)
	assert.Equal(t, "hello", content)
}

func TestRemoteClient_WriteFile(t *testing.T) {
	srv := mockWorker(t)
	defer srv.Close()
	client, _ := setupClient(t, srv.URL)

	err := client.WriteFile(context.Background(), "abc123", "/app/index.js", "console.log('hi')")
	assert.NoError(t, err)
}

func TestRemoteClient_DeleteFile(t *testing.T) {
	srv := mockWorker(t)
	defer srv.Close()
	client, _ := setupClient(t, srv.URL)

	err := client.DeleteFile(context.Background(), "abc123", "/app/index.js")
	assert.NoError(t, err)
}

func TestRemoteClient_ListDir(t *testing.T) {
	srv := mockWorker(t)
	defer srv.Close()
	client, _ := setupClient(t, srv.URL)

	output, err := client.ListDir(context.Background(), "abc123", "/app")
	require.NoError(t, err)
	assert.Contains(t, output, "drwxr-xr-x")
}

func TestRemoteClient_PullImage(t *testing.T) {
	srv := mockWorker(t)
	defer srv.Close()
	client, _ := setupClient(t, srv.URL)

	err := client.PullImage(context.Background(), "node:24")
	assert.NoError(t, err)
}

func TestRemoteClient_RemoveImage(t *testing.T) {
	srv := mockWorker(t)
	defer srv.Close()
	client, _ := setupClient(t, srv.URL)

	err := client.RemoveImage(context.Background(), "sha256:abc", false)
	assert.NoError(t, err)
}

func TestRemoteClient_InspectImage(t *testing.T) {
	srv := mockWorker(t)
	defer srv.Close()
	client, _ := setupClient(t, srv.URL)

	detail, err := client.InspectImage(context.Background(), "sha256:abc")
	require.NoError(t, err)
	assert.Equal(t, "sha256:abc", detail.ID)
}

func TestRemoteClient_ListImages(t *testing.T) {
	srv := mockWorker(t)
	defer srv.Close()
	client, _ := setupClient(t, srv.URL)

	images, err := client.ListImages(context.Background())
	require.NoError(t, err)
	assert.Len(t, images, 1)
	assert.Equal(t, "sha256:abc", images[0].ID)
}

func TestRemoteClient_ListImagesFromWorker(t *testing.T) {
	srv := mockWorker(t)
	defer srv.Close()
	db := database.New(":memory:")
	repo := database.NewRepository(db)
	reg := orchestrator.NewRegistry(repo)
	workerID, _ := reg.Register(srv.URL, "key")
	client := orchestrator.NewRemoteClient(reg, repo)

	images, err := client.ListImagesFromWorkers(context.Background(), workerID)
	require.NoError(t, err)
	assert.Len(t, images, 1)
}

func TestRemoteClient_ListImagesDeduplicates(t *testing.T) {
	// Two workers returning the same image.
	srv1 := mockWorker(t)
	defer srv1.Close()
	srv2 := mockWorker(t)
	defer srv2.Close()

	db := database.New(":memory:")
	repo := database.NewRepository(db)
	reg := orchestrator.NewRegistry(repo)
	reg.Register(srv1.URL, "k1")
	reg.Register(srv2.URL, "k2")
	client := orchestrator.NewRemoteClient(reg, repo)

	images, err := client.ListImages(context.Background())
	require.NoError(t, err)
	// Both workers return sha256:abc, should be deduplicated.
	assert.Len(t, images, 1)
}

func TestRemoteClient_PullImage_PartialFailure(t *testing.T) {
	// One working worker, one broken worker.
	srv := mockWorker(t)
	defer srv.Close()

	brokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"code": "INTERNAL_ERROR", "message": "disk full"})
	}))
	defer brokenSrv.Close()

	db := database.New(":memory:")
	repo := database.NewRepository(db)
	reg := orchestrator.NewRegistry(repo)
	reg.Register(srv.URL, "k1")
	reg.Register(brokenSrv.URL, "k2")
	client := orchestrator.NewRemoteClient(reg, repo)

	// Should succeed because at least one worker succeeded.
	err := client.PullImage(context.Background(), "node:24")
	assert.NoError(t, err)
}

func TestRemoteClient_PullImage_AllFail(t *testing.T) {
	brokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"code": "INTERNAL_ERROR", "message": "disk full"})
	}))
	defer brokenSrv.Close()

	db := database.New(":memory:")
	repo := database.NewRepository(db)
	reg := orchestrator.NewRegistry(repo)
	reg.Register(brokenSrv.URL, "k1")
	client := orchestrator.NewRemoteClient(reg, repo)

	err := client.PullImage(context.Background(), "node:24")
	assert.Error(t, err)
}

func TestRemoteClient_NoWorkers(t *testing.T) {
	db := database.New(":memory:")
	repo := database.NewRepository(db)
	reg := orchestrator.NewRegistry(repo)
	client := orchestrator.NewRemoteClient(reg, repo)

	err := client.Ping(context.Background())
	assert.ErrorIs(t, err, orchestrator.ErrNoWorkers)

	_, err = client.Create(context.Background(), models.CreateSandboxRequest{Image: "node:24"})
	assert.ErrorIs(t, err, orchestrator.ErrNoWorkers)

	err = client.PullImage(context.Background(), "node:24")
	assert.ErrorIs(t, err, orchestrator.ErrNoWorkers)
}
