package orchestrator_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"open-sandbox/internal/database"
	"open-sandbox/internal/orchestrator"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func orchestratorRouter() (*gin.Engine, *orchestrator.WorkerRegistry) {
	gin.SetMode(gin.TestMode)
	db := database.New(":memory:")
	repo := database.NewRepository(db)
	reg := orchestrator.NewRegistry(repo)
	h := orchestrator.NewHandler(reg)

	r := gin.New()
	g := r.Group("/internal/v1")
	h.RegisterRoutes(g)
	return r, reg
}

func TestHandler_RegisterWorker(t *testing.T) {
	r, _ := orchestratorRouter()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/v1/workers/register",
		strings.NewReader(`{"url":"http://10.0.0.1:9090"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Worker-Key", "secret")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp struct {
		WorkerID string `json:"worker_id"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.WorkerID)
	assert.Contains(t, resp.WorkerID, "wrk_")
}

func TestHandler_RegisterWorker_MissingURL(t *testing.T) {
	r, _ := orchestratorRouter()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/v1/workers/register",
		strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_ListWorkers(t *testing.T) {
	r, reg := orchestratorRouter()

	reg.Register("http://w1:9090", "k1")
	reg.Register("http://w2:9090", "k2")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/internal/v1/workers", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Workers []struct {
			ID  string `json:"ID"`
			URL string `json:"URL"`
		} `json:"workers"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp.Workers, 2)
}

func TestHandler_DeregisterWorker(t *testing.T) {
	r, reg := orchestratorRouter()

	id, _ := reg.Register("http://w1:9090", "k1")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/internal/v1/workers/"+id, nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Len(t, reg.All(), 0)
}
