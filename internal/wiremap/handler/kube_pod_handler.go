package handler

import (
	"net/http"

	apperrors "github.com/codeofmario/wiremap/internal/wiremap/errors"
	"github.com/codeofmario/wiremap/internal/wiremap/service"
	"github.com/gin-gonic/gin"
)

type KubePodHandler struct {
	service service.KubePodService
}

func NewKubePodHandler(service service.KubePodService) *KubePodHandler {
	return &KubePodHandler{service: service}
}

func (h *KubePodHandler) Inspect(c *gin.Context) {
	pod, err := h.service.Inspect(c.Request.Context(), c.Param("cluster"), c.Param("namespace"), c.Param("name"))
	if err != nil {
		apperrors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, pod)
}
