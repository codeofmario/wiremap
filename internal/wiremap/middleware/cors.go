package middleware

import (
	"github.com/gin-gonic/gin"
)

func CORS(devMode bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if devMode {
			c.Header("Access-Control-Allow-Origin", "http://localhost:5173")
		}
		c.Header("Access-Control-Allow-Methods", "GET, PUT, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
