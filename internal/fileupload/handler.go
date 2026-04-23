package fileupload

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/r0lm0/go-saas-api/internal/platform/logger"
)

// Handler provides HTTP handlers for file uploads.
type Handler struct {
	service *Service
	log     logger.Logger
}

// NewHandler creates a new file upload handler.
func NewHandler(service *Service, log logger.Logger) *Handler {
	return &Handler{service: service, log: log}
}

// RegisterRoutes registers file upload endpoints.
func (h *Handler) RegisterRoutes(r *gin.Engine) {
	files := r.Group("/files")
	{
		files.POST("", h.Upload)
		files.GET("", h.List)
		files.GET("/:id", h.Get)
		files.GET("/:id/download", h.Download)
		files.DELETE("/:id", h.Delete)
	}
}

// Upload handles POST /files.
func (h *Handler) Upload(c *gin.Context) {
	tenantIDStr := c.GetHeader("X-Tenant-ID")
	userIDStr := c.GetHeader("X-User-ID")
	if tenantIDStr == "" || userIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing X-Tenant-ID or X-User-ID"})
		return
	}
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant_id"})
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
		return
	}

	fh, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing file field"})
		return
	}

	file, err := fh.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot open file"})
		return
	}
	defer file.Close()

	upload, err := h.service.Save(c.Request.Context(), tenantID, userID, fh.Filename, fh.Header.Get("Content-Type"), fh.Size, file)
	if err != nil {
		h.log.Error("upload failed", logger.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, upload)
}

// List handles GET /files.
func (h *Handler) List(c *gin.Context) {
	tenantIDStr := c.GetHeader("X-Tenant-ID")
	userIDStr := c.GetHeader("X-User-ID")
	if tenantIDStr == "" || userIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing X-Tenant-ID or X-User-ID"})
		return
	}
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant_id"})
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
		return
	}

	limit := 20
	offset := 0
	if l := c.Query("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}
	if o := c.Query("offset"); o != "" {
		fmt.Sscanf(o, "%d", &offset)
	}
	if limit > 100 {
		limit = 100
	}

	uploads, err := h.service.ListByUser(c.Request.Context(), tenantID, userID, limit, offset)
	if err != nil {
		h.log.Error("list uploads failed", logger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list files"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"files": uploads})
}

// Get handles GET /files/:id.
func (h *Handler) Get(c *gin.Context) {
	tenantIDStr := c.GetHeader("X-Tenant-ID")
	if tenantIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing X-Tenant-ID"})
		return
	}
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant_id"})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	upload, err := h.service.Get(c.Request.Context(), tenantID, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}

	c.JSON(http.StatusOK, upload)
}

// Download handles GET /files/:id/download.
func (h *Handler) Download(c *gin.Context) {
	tenantIDStr := c.GetHeader("X-Tenant-ID")
	if tenantIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing X-Tenant-ID"})
		return
	}
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant_id"})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	upload, err := h.service.Get(c.Request.Context(), tenantID, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}

	reader, err := h.service.Open(upload)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot open file"})
		return
	}
	defer reader.Close()

	c.Header("Content-Disposition", "attachment; filename=\""+upload.OriginalName+"\"")
	c.DataFromReader(http.StatusOK, upload.SizeBytes, upload.ContentType, reader, nil)
}

// Delete handles DELETE /files/:id.
func (h *Handler) Delete(c *gin.Context) {
	tenantIDStr := c.GetHeader("X-Tenant-ID")
	if tenantIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing X-Tenant-ID"})
		return
	}
	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant_id"})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.service.Delete(c.Request.Context(), tenantID, id); err != nil {
		h.log.Error("delete file failed", logger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete file"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "file deleted"})
}
