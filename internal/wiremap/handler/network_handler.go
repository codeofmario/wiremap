package handler

import (
	"net/http"

	apperrors "github.com/codeofmario/wiremap/internal/wiremap/errors"
	"github.com/codeofmario/wiremap/internal/wiremap/service"
	"github.com/gin-gonic/gin"
)

type NetworkHandler struct {
	service service.NetworkService
}

func NewNetworkHandler(service service.NetworkService) *NetworkHandler {
	return &NetworkHandler{service: service}
}

func (h *NetworkHandler) List(c *gin.Context) {
	hostID := c.DefaultQuery("host", "")

	networks, err := h.service.List(c.Request.Context(), hostID)
	if err != nil {
		apperrors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, networks)
}

func (h *NetworkHandler) Inspect(c *gin.Context) {
	id := c.Param("id")
	hostID := c.DefaultQuery("host", "")

	network, err := h.service.Inspect(c.Request.Context(), hostID, id)
	if err != nil {
		apperrors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, network)
}
