package storage

import (
	"mime/multipart"

	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

// openMultipartFileHeader 是 multipart.FileHeader.Open 的测试缝隙：
// gin 的 c.FormFile 内部（Request.FormFile）已对同一 FileHeader 先执行
// 一次 Open，真实请求中 handler 侧第二次 Open 失败不可达（内存态恒成功、
// 磁盘态需临时文件在两次同步调用间消失），仅测试可注入故障验证。
var openMultipartFileHeader = func(fh *multipart.FileHeader) (multipart.File, error) {
	return fh.Open()
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
		file, err = openMultipartFileHeader(fileHeader)
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
