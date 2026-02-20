package api

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// APIKeyAuth returns a middleware that validates the Authorization: Bearer <key> header.
func APIKeyAuth(key string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		token, found := strings.CutPrefix(header, "Bearer ")
		if !found || subtle.ConstantTimeCompare([]byte(token), []byte(key)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    "UNAUTHORIZED",
				"message": "invalid or missing api key",
			})
			return
		}
		c.Next()
	}
}
