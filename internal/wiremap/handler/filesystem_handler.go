package handler

import (
	"net/http"
	"path/filepath"
	"strings"

	apperrors "github.com/codeofmario/wiremap/internal/wiremap/errors"
	"github.com/codeofmario/wiremap/internal/wiremap/service"
	"github.com/gin-gonic/gin"
)

type FilesystemHandler struct {
	service service.FilesystemService
}

func NewFilesystemHandler(service service.FilesystemService) *FilesystemHandler {
	return &FilesystemHandler{service: service}
}

func validatePath(path string) error {
	if path == "" || path == "/" {
		return nil
	}

	if !filepath.IsAbs(path) {
		return apperrors.BadRequest("path must be absolute")
	}

	cleaned := filepath.Clean(path)
	if strings.Contains(cleaned, "..") {
		return apperrors.BadRequest("path traversal is not allowed")
	}

	return nil
}

func (h *FilesystemHandler) ListDir(c *gin.Context) {
	containerID := c.Param("id")
	path := c.DefaultQuery("path", "/")
	hostID := c.DefaultQuery("host", "")

	if err := validatePath(path); err != nil {
		apperrors.HandleError(c, err)
		return
	}

	entries, err := h.service.ListDir(c.Request.Context(), hostID, containerID, path)
	if err != nil {
		apperrors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, entries)
}

func (h *FilesystemHandler) ReadFile(c *gin.Context) {
	containerID := c.Param("id")
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "path is required"})
		return
	}

	if err := validatePath(path); err != nil {
		apperrors.HandleError(c, err)
		return
	}

	hostID := c.DefaultQuery("host", "")

	content, err := h.service.ReadFile(c.Request.Context(), hostID, containerID, path)
	if err != nil {
		apperrors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, content)
}

func (h *FilesystemHandler) WriteFile(c *gin.Context) {
	containerID := c.Param("id")

	var body struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "path and content are required"})
		return
	}

	if err := validatePath(body.Path); err != nil {
		apperrors.HandleError(c, err)
		return
	}

	hostID := c.DefaultQuery("host", "")

	if err := h.service.WriteFile(c.Request.Context(), hostID, containerID, body.Path, body.Content); err != nil {
		apperrors.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}
