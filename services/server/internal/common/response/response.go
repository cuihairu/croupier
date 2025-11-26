package response

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// Success 成功响应（自动序列化为 JSON）
// 使用 HTTP 200 OK
func Success(w http.ResponseWriter, data interface{}) {
	httpx.OkJson(w, data)
}

// Created 创建成功响应
// 使用 HTTP 201 Created
func Created(w http.ResponseWriter, data interface{}) {
	w.WriteHeader(http.StatusCreated)
	httpx.WriteJson(w, http.StatusCreated, data)
}

// NoContent 无内容响应（如删除成功）
// 使用 HTTP 204 No Content
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// Error 错误响应（自动设置正确的 HTTP 状态码）
// go-zero 的 httpx.Error 会自动识别 CodeError 并设置状态码
func Error(w http.ResponseWriter, r *http.Request, err error) {
	httpx.ErrorCtx(r.Context(), w, err)
}

// 工具函数：快速创建常见响应

// List 列表响应（带分页）
type ListResponse struct {
	Items interface{} `json:"items"`
	Total int64       `json:"total"`
	Page  int         `json:"page"`
	Size  int         `json:"pageSize"`
}

func SuccessList(w http.ResponseWriter, items interface{}, total int64, page, size int) {
	Success(w, ListResponse{
		Items: items,
		Total: total,
		Page:  page,
		Size:  size,
	})
}
