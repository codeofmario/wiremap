package router

import (
	"github.com/codeofmario/wiremap/internal/wiremap/config"
	"github.com/codeofmario/wiremap/internal/wiremap/docker"
	"github.com/codeofmario/wiremap/internal/wiremap/handler"
	"github.com/codeofmario/wiremap/internal/wiremap/middleware"
	"github.com/codeofmario/wiremap/internal/wiremap/ws"
	"github.com/gin-gonic/gin"
)

func InitRoutes(
	settings *config.Settings,
	pool *docker.ClientPool,
	containerHandler *handler.ContainerHandler,
	networkHandler *handler.NetworkHandler,
	filesystemHandler *handler.FilesystemHandler,
	hub *ws.Hub,
) *gin.Engine {
	r := gin.Default()
	r.MaxMultipartMemory = 1 << 20 // 1 MB
	r.Use(middleware.Security())
	r.Use(middleware.CORS(settings.DevMode))

	api := r.Group("/api")
	{
		containers := api.Group("/containers")
		{
			containers.GET("", containerHandler.List)
			containers.GET("/:id", containerHandler.Inspect)
			containers.PUT("/:id/env", containerHandler.UpdateEnv)
			containers.GET("/:id/fs", filesystemHandler.ListDir)
			containers.GET("/:id/fs/read", filesystemHandler.ReadFile)
			containers.PUT("/:id/fs/write", filesystemHandler.WriteFile)
		}

		networks := api.Group("/networks")
		{
			networks.GET("", networkHandler.List)
			networks.GET("/:id", networkHandler.Inspect)
		}
	}

	api.GET("/hosts", func(c *gin.Context) {
		hosts := pool.Hosts()
		result := make([]gin.H, 0, len(hosts))
		for _, h := range hosts {
			result = append(result, gin.H{"name": h.Name})
		}
		c.JSON(200, result)
	})

	r.GET("/ws", func(c *gin.Context) {
		hub.HandleWS(c.Writer, c.Request)
	})

	r.GET("/ws/exec/:id", func(c *gin.Context) {
		hub.HandleExecWS(c.Writer, c.Request, c.Param("id"), c.DefaultQuery("host", ""))
	})

	middleware.ServeFrontend(r, settings)

	return r
}
