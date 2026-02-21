package api_test

import (
	"encoding/json"
	"net/http"

	"testing"

	"open-sandbox/internal/api"
	"open-sandbox/internal/docker"
	"open-sandbox/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() { gin.SetMode(gin.TestMode) }

// realRouter builds a Gin engine wired to the real Docker daemon.
func realRouter() *gin.Engine {
	r := gin.New()
	api.New(docker.New()).RegisterRoutes(r.Group("/v1"))
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

	// 3. Inspect the sandbox.
	w = do(r, "GET", "/v1/sandboxes/"+id, nil)
	assert.Equal(t, http.StatusOK, w.Code)

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

	// 12. Restart the sandbox.
	w = do(r, "POST", "/v1/sandboxes/"+id+"/restart", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "restarted")

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

	// Create a sandbox without specifying resource limits (assumes nextjs-docker:latest is already available locally)
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

	// Inspect the sandbox to verify default resource limits
	w = do(r, "GET", "/v1/sandboxes/"+id, nil)
	assert.Equal(t, http.StatusOK, w.Code)

	var inspect map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &inspect))

	// Verify HostConfig.Memory = 1GB (1024 * 1024 * 1024 bytes)
	hostConfig := inspect["HostConfig"].(map[string]any)
	memory := int64(hostConfig["Memory"].(float64))
	assert.Equal(t, int64(1024*1024*1024), memory, "Default memory should be 1GB")

	// Verify HostConfig.NanoCpus = 1 CPU (1e9 nanocpus)
	nanoCPUs := int64(hostConfig["NanoCpus"].(float64))
	assert.Equal(t, int64(1e9), nanoCPUs, "Default CPUs should be 1 vCPU")
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
