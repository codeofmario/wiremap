package middleware

import (
	"net/http"

	apperrors "github.com/codeofmario/wiremap/internal/wiremap/errors"
	"github.com/gin-gonic/gin"
)

// SameOrigin rejects state-changing requests sent by other websites. Simple
// cross-origin POSTs skip the CORS preflight, so without this any page open in
// the user's browser could trigger actions against the local API. Requests
// without an Origin header (curl, scripts) are not from a browser and pass.
func SameOrigin(devMode bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}

		origin := c.Request.Header.Get("Origin")
		host := c.Request.Host
		allowed := origin == "" || origin == "http://"+host || origin == "https://"+host ||
			(devMode && origin == "http://localhost:5173")
		if !allowed {
			apperrors.HandleError(c, apperrors.Forbidden("cross-origin requests are not allowed"))
			c.Abort()
			return
		}
		c.Next()
	}
}
