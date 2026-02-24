package worker

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// APIKeyAuth returns a middleware that validates the X-Worker-Key header.
func APIKeyAuth(key string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := strings.TrimSpace(c.GetHeader("X-Worker-Key"))
		if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(key)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    "UNAUTHORIZED",
				"message": "invalid or missing worker api key",
			})
			return
		}
		c.Next()
	}
}
