package platform

import (
	"context"
	"net/http"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

// platformServiceAPI 是 Handler 对 Service 的最小接口缝隙：Service 三个方法
// 把错误经 resp.Code 表达、恒返回 nil error，err != nil 分支仅在测试注入
// 故障实现时可触达。
type platformServiceAPI interface {
	Call(ctx context.Context, req *CallPlatformRequest) (*CallPlatformResponse, error)
	ListPlatforms(ctx context.Context) (*ListPlatformsResponse, error)
	ListMethods(ctx context.Context, platform string) (*ListPlatformMethodsResponse, error)
}

type Handler struct {
	service platformServiceAPI
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Call handles the request to call a platform method
func (h *Handler) Call(c *gin.Context) {
	var req CallPlatformRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.Call(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	writeCallResponse(c, resp)
}

// ListPlatforms handles the request to list platforms
func (h *Handler) ListPlatforms(c *gin.Context) {
	resp, err := h.service.ListPlatforms(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	writeListPlatformsResponse(c, resp)
}

// ListMethods handles the request to list platform methods
func (h *Handler) ListMethods(c *gin.Context) {
	platform := c.Param("platform")
	resp, err := h.service.ListMethods(c.Request.Context(), platform)
	if err != nil {
		response.Error(c, err)
		return
	}
	writeListMethodsResponse(c, resp)
}

// Alias methods for route compatibility

func (h *Handler) List(c *gin.Context) {
	h.ListPlatforms(c)
}

func (h *Handler) Methods(c *gin.Context) {
	h.ListMethods(c)
}

func writeCallResponse(c *gin.Context, resp *CallPlatformResponse) {
	if resp == nil {
		response.InternalServerError(c, "platform response is nil")
		return
	}
	if resp.Code >= http.StatusBadRequest {
		response.Error(c, &errorx.CodeError{
			Code:    resp.Code,
			Message: resp.Message,
		})
		return
	}
	response.Success(c, CallPlatformPayload{
		Response: resp.Response,
		Source:   resp.Source,
	})
}

func writeListPlatformsResponse(c *gin.Context, resp *ListPlatformsResponse) {
	if resp == nil {
		response.InternalServerError(c, "platform response is nil")
		return
	}
	if resp.Code >= http.StatusBadRequest {
		response.Error(c, &errorx.CodeError{
			Code:    resp.Code,
			Message: resp.Message,
		})
		return
	}
	response.Success(c, ListPlatformsPayload{
		Platforms: resp.Platforms,
	})
}

func writeListMethodsResponse(c *gin.Context, resp *ListPlatformMethodsResponse) {
	if resp == nil {
		response.InternalServerError(c, "platform response is nil")
		return
	}
	if resp.Code >= http.StatusBadRequest {
		response.Error(c, &errorx.CodeError{
			Code:    resp.Code,
			Message: resp.Message,
		})
		return
	}
	response.Success(c, ListPlatformMethodsPayload{
		Methods: resp.Methods,
		Source:  resp.Source,
	})
}
