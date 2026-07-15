package middleware

import (
	"github.com/gin-gonic/gin"
)

// CORSMiddleware returns a CORS middleware. Since the production deployment
// uses same-origin (external NGINX proxy), CORS is mainly for development
// where the Vite dev server runs on a different port.
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")
		// Note: Allow-Credentials is intentionally omitted because it is incompatible
		// with Allow-Origin: * per the CORS specification. Authentication uses Bearer
		// tokens via the Authorization header, not cookies.

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
