package api

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMCPMetadataLoggerPreservesRequestBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(MCPMetadataLogger())
	r.POST("/v1/mcp", func(c *gin.Context) {
		b, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		c.Data(http.StatusOK, "application/json", b)
	})

	body := []byte(`{"jsonrpc":"2.0","method":"tools/list"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if w.Body.String() != string(body) {
		t.Fatalf("handler received modified body: %q", w.Body.String())
	}
}

func TestExtractMCPMethodsIgnoresUnknownShapes(t *testing.T) {
	if got := extractMCPMethods([]byte(`{"jsonrpc":"2.0"}`)); got != "" {
		t.Fatalf("extractMCPMethods without method = %q, want empty", got)
	}

	if got := extractMCPMethods([]byte(`[{"foo":"bar"}]`)); got != "" {
		t.Fatalf("extractMCPMethods unknown batch = %q, want empty", got)
	}
}
