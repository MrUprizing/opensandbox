package worker

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupAuthRouter(key string) *gin.Engine {
	r := gin.New()
	r.Use(APIKeyAuth(key))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r
}

func TestAPIKeyAuth_ValidKey(t *testing.T) {
	r := setupAuthRouter("secret-key")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Worker-Key", "secret-key")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAPIKeyAuth_MissingHeader(t *testing.T) {
	r := setupAuthRouter("secret-key")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAPIKeyAuth_WrongKey(t *testing.T) {
	r := setupAuthRouter("secret-key")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Worker-Key", "wrong-key")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAPIKeyAuth_WhitespaceKey(t *testing.T) {
	r := setupAuthRouter("secret-key")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Worker-Key", "  secret-key  ")
	r.ServeHTTP(w, req)

	// trimmed key should match
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (trimmed), got %d", w.Code)
	}
}

func TestAPIKeyAuth_EmptyKey(t *testing.T) {
	r := setupAuthRouter("secret-key")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Worker-Key", "")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}
