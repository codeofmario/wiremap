package handler

import (
	"net/http"
	"strings"

	apperrors "github.com/codeofmario/wiremap/internal/wiremap/errors"
	"github.com/codeofmario/wiremap/internal/wiremap/service"
	"github.com/gin-gonic/gin"
)

type KubeBrowserHandler struct {
	service service.KubeBrowserService
}

func NewKubeBrowserHandler(service service.KubeBrowserService) *KubeBrowserHandler {
	return &KubeBrowserHandler{service: service}
}

func (h *KubeBrowserHandler) Kinds(c *gin.Context) {
	kinds, err := h.service.Kinds(c.Request.Context(), c.Param("cluster"))
	if err != nil {
		apperrors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, kinds)
}

func (h *KubeBrowserHandler) List(c *gin.Context) {
	objects, err := h.service.List(c.Request.Context(), c.Param("cluster"),
		c.Query("group"), c.Query("version"), c.Query("resource"), c.Query("namespace"), c.Query("continue"))
	if err != nil {
		apperrors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, objects)
}

// Counts takes the kinds to count as a comma-separated "group/Kind" list, e.g. kinds=/Pod,apps/Deployment.
func (h *KubeBrowserHandler) Counts(c *gin.Context) {
	var kinds []string
	if raw := c.Query("kinds"); raw != "" {
		kinds = strings.Split(raw, ",")
	}
	counts, err := h.service.Counts(c.Request.Context(), c.Param("cluster"), c.Query("namespace"), kinds)
	if err != nil {
		apperrors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, counts)
}
