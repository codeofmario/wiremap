package middleware

import (
	"io/fs"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/codeofmario/wiremap/internal/wiremap/config"
	"github.com/codeofmario/wiremap/web"
	"github.com/gin-gonic/gin"
)

func ServeFrontend(r *gin.Engine, settings *config.Settings) {
	if settings.DevMode {
		serveDevProxy(r)
	} else {
		serveEmbedded(r)
	}
}

func serveDevProxy(r *gin.Engine) {
	target, _ := url.Parse("http://localhost:5173")
	proxy := httputil.NewSingleHostReverseProxy(target)

	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api") || c.Request.URL.Path == "/ws" {
			return
		}
		proxy.ServeHTTP(c.Writer, c.Request)
	})
}

func serveEmbedded(r *gin.Engine) {
	distFS, err := fs.Sub(web.DistFS, "dist")
	if err != nil {
		panic("failed to load embedded frontend: " + err.Error())
	}

	fileServer := http.FileServer(http.FS(distFS))

	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		if strings.HasPrefix(path, "/api") || path == "/ws" {
			return
		}

		// Try to serve the file directly
		f, err := distFS.(fs.ReadFileFS).ReadFile(strings.TrimPrefix(path, "/"))
		if err == nil && len(f) > 0 {
			fileServer.ServeHTTP(c.Writer, c.Request)
			return
		}

		// SPA fallback: serve index.html
		c.Request.URL.Path = "/"
		fileServer.ServeHTTP(c.Writer, c.Request)
	})
}
