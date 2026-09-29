package handler

import (
	"net/http"

	apperrors "github.com/codeofmario/wiremap/internal/wiremap/errors"
	"github.com/codeofmario/wiremap/internal/wiremap/service"
	"github.com/gin-gonic/gin"
)

type KubeActionHandler struct {
	actions service.KubeActionService
	nodes   service.KubeNodeActionService
}

func NewKubeActionHandler(actions service.KubeActionService, nodes service.KubeNodeActionService) *KubeActionHandler {
	return &KubeActionHandler{actions: actions, nodes: nodes}
}

type updateRequest struct {
	YAML string `json:"yaml" binding:"required"`
}

type scaleRequest struct {
	Replicas *int32 `json:"replicas" binding:"required"`
}

type toggleRequest struct {
	Value *bool `json:"value" binding:"required"`
}

// objectRef reads the target object from /clusters/:cluster/resources/:kind/:name?group=&namespace=
func objectRef(c *gin.Context) service.ObjectRef {
	return service.ObjectRef{
		Cluster:   c.Param("cluster"),
		Group:     c.Query("group"),
		Kind:      c.Param("kind"),
		Namespace: c.Query("namespace"),
		Name:      c.Param("name"),
	}
}

func (h *KubeActionHandler) Update(c *gin.Context) {
	var req updateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apperrors.HandleError(c, apperrors.BadRequest("yaml is required"))
		return
	}
	if err := h.actions.Update(c.Request.Context(), objectRef(c), req.YAML); err != nil {
		apperrors.HandleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *KubeActionHandler) Delete(c *gin.Context) {
	if err := h.actions.Delete(c.Request.Context(), objectRef(c)); err != nil {
		apperrors.HandleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *KubeActionHandler) Scale(c *gin.Context) {
	var req scaleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apperrors.HandleError(c, apperrors.BadRequest("replicas is required"))
		return
	}
	if err := h.actions.Scale(c.Request.Context(), objectRef(c), *req.Replicas); err != nil {
		apperrors.HandleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *KubeActionHandler) Restart(c *gin.Context) {
	if err := h.actions.Restart(c.Request.Context(), objectRef(c)); err != nil {
		apperrors.HandleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *KubeActionHandler) Trigger(c *gin.Context) {
	job, err := h.actions.TriggerCronJob(c.Request.Context(), objectRef(c))
	if err != nil {
		apperrors.HandleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"job": job})
}

func (h *KubeActionHandler) Suspend(c *gin.Context) {
	var req toggleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apperrors.HandleError(c, apperrors.BadRequest("value is required"))
		return
	}
	if err := h.actions.SetSuspended(c.Request.Context(), objectRef(c), *req.Value); err != nil {
		apperrors.HandleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *KubeActionHandler) Cordon(c *gin.Context) {
	var req toggleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apperrors.HandleError(c, apperrors.BadRequest("value is required"))
		return
	}
	if err := h.nodes.SetUnschedulable(c.Request.Context(), c.Param("cluster"), c.Param("name"), *req.Value); err != nil {
		apperrors.HandleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *KubeActionHandler) Drain(c *gin.Context) {
	refused, err := h.nodes.Drain(c.Request.Context(), c.Param("cluster"), c.Param("name"))
	if err != nil {
		apperrors.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"refused": refused})
}
