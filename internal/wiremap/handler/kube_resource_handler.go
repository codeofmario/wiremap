package handler

import (
	"net/http"

	apperrors "github.com/codeofmario/wiremap/internal/wiremap/errors"
	"github.com/codeofmario/wiremap/internal/wiremap/service"
	"github.com/gin-gonic/gin"
)

type KubeResourceHandler struct {
	service service.KubeResourceService
}

func NewKubeResourceHandler(service service.KubeResourceService) *KubeResourceHandler {
	return &KubeResourceHandler{service: service}
}

func (h *KubeResourceHandler) Get(c *gin.Context) {
	resource, err := h.service.Get(c.Request.Context(), c.Param("cluster"), c.Query("group"), c.Param("kind"), c.Query("namespace"), c.Param("name"))
	if err != nil {
		apperrors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resource)
}
