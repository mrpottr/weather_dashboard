package middleware

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

/*
Package middleware provides HTTP transport security, panic isolation, and token context binding.
Applying these handlers globally guarantees uniform header defense and graceful recovery across all API routes.
*/

// JSONRecover intercepts unexpected runtime panics and maps them to sanitized JSON errors to prevent internal stack leakages.
func JSONRecover() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic recovered: %v", rec)
				c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorResponse{
					Error: "An unexpected server error occurred. Please try again.",
					Code:  "internal_error",
				})
			}
		}()
		c.Next()
	}
}

// SecurityHeaders injects defense-in-depth HTTP headers to mitigate MIME-sniffing, cross-site framing, and unauthorized peripheral access.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		c.Next()
	}
}
