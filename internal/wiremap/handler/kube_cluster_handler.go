package handler

import (
	"fmt"
	"net/http"

	"github.com/codeofmario/wiremap/internal/wiremap/dto"
	apperrors "github.com/codeofmario/wiremap/internal/wiremap/errors"
	"github.com/codeofmario/wiremap/internal/wiremap/service"
	"github.com/gin-gonic/gin"
)

type KubeClusterHandler struct {
	clusterService  service.KubeClusterService
	topologyService service.KubeTopologyService
}

func NewKubeClusterHandler(clusterService service.KubeClusterService, topologyService service.KubeTopologyService) *KubeClusterHandler {
	return &KubeClusterHandler{clusterService: clusterService, topologyService: topologyService}
}

func (h *KubeClusterHandler) List(c *gin.Context) {
	c.JSON(http.StatusOK, h.clusterService.List())
}

// Topology returns one drill-down level of the cluster, selected by the `level` query param.
func (h *KubeClusterHandler) Topology(c *gin.Context) {
	ctx := c.Request.Context()
	cluster := c.Param("cluster")
	namespace := c.Query("namespace")
	name := c.Query("name")

	var graph *dto.KubeGraphDto
	var err error

	switch level := c.DefaultQuery("level", service.LevelCluster); level {
	case service.LevelCluster:
		graph, err = h.topologyService.Cluster(ctx, cluster, c.DefaultQuery("lens", service.LensNamespaces))
	case service.LevelNamespace:
		graph, err = h.topologyService.Namespace(ctx, cluster, namespace)
	case service.LevelObject:
		graph, err = h.topologyService.Object(ctx, cluster, namespace, c.Query("group"), c.Query("kind"), name)
	case service.LevelNode:
		graph, err = h.topologyService.Node(ctx, cluster, name)
	default:
		err = apperrors.BadRequest(fmt.Sprintf("unknown level %q", level))
	}

	if err != nil {
		apperrors.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, graph)
}
