// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package storage

import (
	"mime"
	"net/http"
	"path/filepath"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/logic/storage"
	"github.com/cuihairu/croupier/services/server/internal/svc"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// 上传对象
func UploadObjectHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 解析 multipart/form-data
		err := r.ParseMultipartForm(32 << 20) // 32MB max memory
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, errorx.NewBadRequest("解析 multipart 表单失败"))
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, errorx.NewBadRequest("获取上传文件失败"))
			return
		}
		defer file.Close()

		// 获取路径参数（如果有）
		path := r.FormValue("path")
		if path == "" {
			// 如果没有 path 字段，使用文件名
			path = header.Filename
		}

		// 检测 Content-Type
		contentType := detectContentTypeFromHeader(header.Filename, header.Header.Get("Content-Type"))

		// 创建 logic 并调用
		l := storage.NewUploadObjectLogic(r.Context(), svcCtx)
		resp, err := l.UploadObjectWithFile(file, path, header.Size, contentType)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}

// 辅助函数：从 HTTP header 检测 Content-Type
func detectContentTypeFromHeader(filename string, headerContentType string) string {
	if headerContentType != "" && headerContentType != "application/octet-stream" {
		return headerContentType
	}
	ext := filepath.Ext(filename)
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		return "application/octet-stream"
	}
	return contentType
}
