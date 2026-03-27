package handler

import (
	"net/http"

	apperrors "github.com/codeofmario/wiremap/internal/wiremap/errors"
	"github.com/codeofmario/wiremap/internal/wiremap/service"
	"github.com/gin-gonic/gin"
)

type ContainerHandler struct {
	service service.ContainerService
}

func NewContainerHandler(service service.ContainerService) *ContainerHandler {
	return &ContainerHandler{service: service}
}

func (h *ContainerHandler) List(c *gin.Context) {
	hostID := c.DefaultQuery("host", "")

	containers, err := h.service.List(c.Request.Context(), hostID)
	if err != nil {
		apperrors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, containers)
}

func (h *ContainerHandler) Inspect(c *gin.Context) {
	id := c.Param("id")
	hostID := c.DefaultQuery("host", "")

	container, err := h.service.Inspect(c.Request.Context(), hostID, id)
	if err != nil {
		apperrors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, container)
}

func (h *ContainerHandler) UpdateEnv(c *gin.Context) {
	id := c.Param("id")

	var body struct {
		Env []string `json:"env"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "env array is required"})
		return
	}

	hostID := c.DefaultQuery("host", "")

	newID, err := h.service.UpdateEnv(c.Request.Context(), hostID, id, body.Env)
	if err != nil {
		apperrors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"id": newID, "message": "Container recreated with updated environment"})
}
