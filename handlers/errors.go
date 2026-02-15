package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// errorResponse is the standard error body for all API errors.
type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func badRequest(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, errorResponse{
		Code:    "BAD_REQUEST",
		Message: msg,
	})
}

func notFound(c *gin.Context, resource string) {
	c.JSON(http.StatusNotFound, errorResponse{
		Code:    "NOT_FOUND",
		Message: resource + " not found",
	})
}

func internalError(c *gin.Context, err error) {
	// Surface Docker's "No such container" as 404
	if strings.Contains(err.Error(), "No such container") {
		notFound(c, "sandbox")
		return
	}
	c.JSON(http.StatusInternalServerError, errorResponse{
		Code:    "INTERNAL_ERROR",
		Message: err.Error(),
	})
}
