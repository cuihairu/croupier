package storage

import (
	"mime/multipart"

	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// SignedUrl handles the request to get a signed URL
func (h *Handler) SignedUrl(c *gin.Context) {
	var req SignedUrlRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.SignedUrl(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// ListObjects handles the request to list objects
func (h *Handler) ListObjects(c *gin.Context) {
	var req ListObjectsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.ListObjects(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// UploadObject handles the request to upload an object
func (h *Handler) UploadObject(c *gin.Context) {
	var req UploadObjectRequest
	if err := c.ShouldBind(&req); err != nil {
		response.Error(c, err)
		return
	}

	var file multipart.File
	fileHeader, err := c.FormFile("file")
	if err == nil && fileHeader != nil {
		file, err = fileHeader.Open()
		if err != nil {
			response.Error(c, err)
			return
		}
		defer file.Close()

		req.File = file
		req.Size = fileHeader.Size
		req.OriginalName = fileHeader.Filename
		if req.Path == "" {
			req.Path = fileHeader.Filename
		}
		if req.ContentType == "" {
			req.ContentType = fileHeader.Header.Get("Content-Type")
		}
	}

	resp, err := h.service.UploadObject(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// DeleteObject handles the request to delete an object
func (h *Handler) DeleteObject(c *gin.Context) {
	var req DeleteObjectRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.DeleteObject(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// BatchDeleteObjects handles the request to batch delete objects
func (h *Handler) BatchDeleteObjects(c *gin.Context) {
	var req BatchDeleteObjectsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.BatchDeleteObjects(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// CreateDirectory handles the request to create a directory
func (h *Handler) CreateDirectory(c *gin.Context) {
	var req CreateDirectoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.CreateDirectory(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// RenameDirectory handles the request to rename a directory
func (h *Handler) RenameDirectory(c *gin.Context) {
	var req RenameDirectoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.RenameDirectory(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// SignedURL alias for route compatibility
func (h *Handler) SignedURL(c *gin.Context) {
	h.SignedUrl(c)
}
