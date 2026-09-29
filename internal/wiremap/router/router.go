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
	kubeClusterHandler *handler.KubeClusterHandler,
	kubeResourceHandler *handler.KubeResourceHandler,
	kubePodHandler *handler.KubePodHandler,
	kubeBrowserHandler *handler.KubeBrowserHandler,
	kubeActionHandler *handler.KubeActionHandler,
	hub *ws.Hub,
	kubeHub *ws.KubeHub,
) *gin.Engine {
	r := gin.Default()
	r.MaxMultipartMemory = 1 << 20 // 1 MB
	r.Use(middleware.Security())
	r.Use(middleware.CORS(settings.DevMode))
	r.Use(middleware.SameOrigin(settings.DevMode))

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

		k8s := api.Group("/k8s/clusters")
		{
			k8s.GET("", kubeClusterHandler.List)
			k8s.GET("/:cluster/topology", kubeClusterHandler.Topology)
			k8s.GET("/:cluster/kinds", kubeBrowserHandler.Kinds)
			k8s.GET("/:cluster/objects", kubeBrowserHandler.List)
			k8s.GET("/:cluster/counts", kubeBrowserHandler.Counts)
			k8s.GET("/:cluster/resources/:kind/:name", kubeResourceHandler.Get)
			k8s.PUT("/:cluster/resources/:kind/:name", kubeActionHandler.Update)
			k8s.DELETE("/:cluster/resources/:kind/:name", kubeActionHandler.Delete)
			k8s.POST("/:cluster/resources/:kind/:name/scale", kubeActionHandler.Scale)
			k8s.POST("/:cluster/resources/:kind/:name/restart", kubeActionHandler.Restart)
			k8s.POST("/:cluster/resources/:kind/:name/trigger", kubeActionHandler.Trigger)
			k8s.POST("/:cluster/resources/:kind/:name/suspend", kubeActionHandler.Suspend)
			k8s.POST("/:cluster/nodes/:name/cordon", kubeActionHandler.Cordon)
			k8s.POST("/:cluster/nodes/:name/drain", kubeActionHandler.Drain)
			k8s.GET("/:cluster/namespaces/:namespace/pods/:name", kubePodHandler.Inspect)
		}
	}

	api.GET("/hosts", func(c *gin.Context) {
		hosts := pool.Hosts()
		result := make([]gin.H, 0, len(hosts))
		for _, h := range hosts {
			result = append(result, gin.H{"name": h.Name, "connected": pool.IsConnected(h.Name)})
		}
		c.JSON(200, result)
	})

	r.GET("/ws", func(c *gin.Context) {
		hub.HandleWS(c.Writer, c.Request)
	})

	r.GET("/ws/exec/:id", func(c *gin.Context) {
		hub.HandleExecWS(c.Writer, c.Request, c.Param("id"), c.DefaultQuery("host", ""))
	})

	r.GET("/ws/k8s", func(c *gin.Context) {
		kubeHub.HandleWS(c.Writer, c.Request)
	})

	r.GET("/ws/k8s/exec/:cluster/:namespace/:pod", func(c *gin.Context) {
		kubeHub.HandleExecWS(c.Writer, c.Request, c.Param("cluster"), c.Param("namespace"), c.Param("pod"), c.Query("container"))
	})

	middleware.ServeFrontend(r, settings)

	return r
}
