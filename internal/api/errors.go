package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"open-sandbox/internal/docker"
)

// errorResponse is the standard error body returned by all API endpoints.
type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// badRequest writes a 400 response with code BAD_REQUEST and the provided message.
func badRequest(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, errorResponse{Code: "BAD_REQUEST", Message: msg})
}

// notFound writes a 404 response with code NOT_FOUND for the given resource name.
func notFound(c *gin.Context, resource string) {
	c.JSON(http.StatusNotFound, errorResponse{Code: "NOT_FOUND", Message: resource + " not found"})
}

// internalError writes a 500 response with code INTERNAL_ERROR.
// If err is docker.ErrNotFound it downgrades to a 404 notFound response instead.
func internalError(c *gin.Context, err error) {
	if errors.Is(err, docker.ErrNotFound) {
		notFound(c, "sandbox")
		return
	}
	c.JSON(http.StatusInternalServerError, errorResponse{Code: "INTERNAL_ERROR", Message: err.Error()})
}
