package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"open-sandbox/internal/docker"
)

// ErrorResponse is the standard error body returned by all API endpoints.
type ErrorResponse struct {
	Code    string `json:"code" example:"BAD_REQUEST"`
	Message string `json:"message" example:"image is required"`
}

// badRequest writes a 400 response with code BAD_REQUEST and the provided message.
func badRequest(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, ErrorResponse{Code: "BAD_REQUEST", Message: msg})
}

// notFound writes a 404 response with code NOT_FOUND for the given resource name.
func notFound(c *gin.Context, resource string) {
	c.JSON(http.StatusNotFound, ErrorResponse{Code: "NOT_FOUND", Message: resource + " not found"})
}

// internalError writes a 500 response with code INTERNAL_ERROR.
// If err is docker.ErrNotFound it downgrades to a 404 notFound response instead.
// If err is docker.ErrImageNotFound it downgrades to a 400 badRequest response.
func internalError(c *gin.Context, err error) {
	if errors.Is(err, docker.ErrNotFound) {
		notFound(c, "sandbox")
		return
	}
	if errors.Is(err, docker.ErrImageNotFound) {
		badRequest(c, "image not found locally, use POST /v1/images/pull to download it first")
		return
	}
	c.JSON(http.StatusInternalServerError, ErrorResponse{Code: "INTERNAL_ERROR", Message: err.Error()})
}
