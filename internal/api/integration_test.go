package api_test

import (
	"encoding/json"
	"net/http"

	"testing"

	"open-sandbox/internal/api"
	"open-sandbox/internal/database"
	"open-sandbox/internal/docker"
	"open-sandbox/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// realRouter builds a Gin engine wired to the real Docker daemon.
func realRouter() *gin.Engine {
	db := database.New(":memory:")
	repo := database.NewRepository(db)
	r := gin.New()
	h := api.New(docker.New(repo))
	h.RegisterHealthCheck(r)
	h.RegisterRoutes(r.Group("/v1"))
	return r
}

func TestIntegration_FullLifecycle(t *testing.T) {
	r := realRouter()

	// 1. Create a sandbox using a lightweight image (assumes nextjs-docker:latest is already available locally).
	w := do(r, "POST", "/v1/sandboxes", map[string]any{
		"image":   "nextjs-docker:latest",
		"cmd":     []string{"sleep", "300"},
		"timeout": 60,
	})
	require.Equal(t, http.StatusCreated, w.Code, "create should return 201: %s", w.Body.String())

	var created models.CreateSandboxResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	require.NotEmpty(t, created.ID)
	id := created.ID

	// Cleanup: always remove the sandbox at the end.
	defer func() {
		do(r, "DELETE", "/v1/sandboxes/"+id, nil)
	}()

	// 2. List sandboxes — our container should be there.
	w = do(r, "GET", "/v1/sandboxes", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), id[:12])

	// 3. Inspect the sandbox — should return curated fields.
	w = do(r, "GET", "/v1/sandboxes/"+id, nil)
	assert.Equal(t, http.StatusOK, w.Code)

	var detail models.SandboxDetail
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &detail))
	assert.Equal(t, id, detail.ID)
	assert.True(t, detail.Running)
	assert.NotNil(t, detail.ExpiresAt, "sandbox should have an expiration time")

	// 4. Exec a command inside the sandbox.
	w = do(r, "POST", "/v1/sandboxes/"+id+"/exec", map[string]any{
		"cmd": []string{"echo", "hello"},
	})
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "hello")

	// 5. Write a file.
	w = do(r, "PUT", "/v1/sandboxes/"+id+"/files?path=/tmp/test.txt", map[string]any{
		"content": "integration-test",
	})
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "written")

	// 6. Read the file back.
	w = do(r, "GET", "/v1/sandboxes/"+id+"/files?path=/tmp/test.txt", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "integration-test")

	// 7. List directory.
	w = do(r, "GET", "/v1/sandboxes/"+id+"/files/list?path=/tmp", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "test.txt")

	// 8. Delete the file.
	w = do(r, "DELETE", "/v1/sandboxes/"+id+"/files?path=/tmp/test.txt", nil)
	assert.Equal(t, http.StatusNoContent, w.Code)

	// 9. Pause the sandbox.
	w = do(r, "POST", "/v1/sandboxes/"+id+"/pause", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "paused")

	// 10. Resume the sandbox.
	w = do(r, "POST", "/v1/sandboxes/"+id+"/resume", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "resumed")

	// 11. Renew expiration.
	w = do(r, "POST", "/v1/sandboxes/"+id+"/renew-expiration", map[string]any{
		"timeout": 120,
	})
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "renewed")

	// 12. Restart the sandbox — should return new ports and expiration.
	w = do(r, "POST", "/v1/sandboxes/"+id+"/restart", nil)
	assert.Equal(t, http.StatusOK, w.Code)

	var restarted models.RestartResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &restarted))
	assert.Equal(t, "restarted", restarted.Status)
	assert.NotNil(t, restarted.Ports, "restart should return port mappings")
	assert.NotNil(t, restarted.ExpiresAt, "restart should return expiration time")

	// 13. Stop the sandbox.
	w = do(r, "POST", "/v1/sandboxes/"+id+"/stop", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "stopped")

	// 14. Delete the sandbox.
	w = do(r, "DELETE", "/v1/sandboxes/"+id, nil)
	assert.Equal(t, http.StatusNoContent, w.Code)

	// 15. Inspect deleted sandbox should return 404.
	w = do(r, "GET", "/v1/sandboxes/"+id, nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestIntegration_NotFound(t *testing.T) {
	r := realRouter()

	endpoints := []struct {
		method string
		url    string
		body   any
	}{
		{"GET", "/v1/sandboxes/nonexistent", nil},
		{"POST", "/v1/sandboxes/nonexistent/stop", nil},
		{"POST", "/v1/sandboxes/nonexistent/restart", nil},
		{"POST", "/v1/sandboxes/nonexistent/pause", nil},
		{"POST", "/v1/sandboxes/nonexistent/resume", nil},
		{"DELETE", "/v1/sandboxes/nonexistent", nil},
		{"POST", "/v1/sandboxes/nonexistent/renew-expiration", map[string]any{"timeout": 60}},
		{"POST", "/v1/sandboxes/nonexistent/exec", map[string]any{"cmd": []string{"echo"}}},
	}

	for _, e := range endpoints {
		w := do(r, e.method, e.url, e.body)
		assert.Equal(t, http.StatusNotFound, w.Code, "%s %s should return 404", e.method, e.url)
	}
}

func TestIntegration_DefaultResourceLimits(t *testing.T) {
	r := realRouter()

	// Create a sandbox without specifying resource limits
	w := do(r, "POST", "/v1/sandboxes", map[string]any{
		"image":   "nextjs-docker:latest",
		"cmd":     []string{"sleep", "60"},
		"timeout": 60,
	})
	require.Equal(t, http.StatusCreated, w.Code, "create should return 201: %s", w.Body.String())

	var created models.CreateSandboxResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	require.NotEmpty(t, created.ID)
	id := created.ID

	defer func() {
		do(r, "DELETE", "/v1/sandboxes/"+id, nil)
	}()

	// Inspect the sandbox to verify default resource limits.
	w = do(r, "GET", "/v1/sandboxes/"+id, nil)
	assert.Equal(t, http.StatusOK, w.Code)

	var detail models.SandboxDetail
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &detail))

	// Verify defaults: 1GB RAM, 1 vCPU
	assert.Equal(t, int64(1024), detail.Resources.Memory, "Default memory should be 1024 MB")
	assert.Equal(t, 1.0, detail.Resources.CPUs, "Default CPUs should be 1.0")
}

func TestIntegration_ImagePull(t *testing.T) {
	r := realRouter()

	// Test pulling a lightweight image
	testImage := "busybox:1.36.1"

	w := do(r, "POST", "/v1/images/pull", map[string]any{
		"image": testImage,
	})
	require.Equal(t, http.StatusOK, w.Code, "pull should return 200: %s", w.Body.String())
	assert.Contains(t, w.Body.String(), "pulled")
	assert.Contains(t, w.Body.String(), testImage)

	var response models.ImagePullResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "pulled", response.Status)
	assert.Equal(t, testImage, response.Image)
}
