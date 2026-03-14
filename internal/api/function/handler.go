package function

import (
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/gin-gonic/gin"
)

func bindFunctionRequest(c *gin.Context, req interface{}) error {
	if c.Request.Method == "GET" {
		return c.ShouldBindQuery(req)
	}
	return c.ShouldBindJSON(req)
}

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Function management handlers

func (h *Handler) FunctionsList(c *gin.Context) {
	var req FunctionsListRequest
	if err := bindFunctionRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.FunctionsList(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) FunctionsPending(c *gin.Context) {
	var req FunctionsPendingRequest
	if err := bindFunctionRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.FunctionsPending(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) FunctionDetail(c *gin.Context) {
	var req FunctionDetailRequest
	if err := bindFunctionRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.FunctionDetail(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) FunctionAnalytics(c *gin.Context) {
	var req FunctionAnalyticsRequest
	if err := bindFunctionRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.FunctionAnalytics(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) FunctionCopy(c *gin.Context) {
	var req FunctionCopyRequest
	if err := bindFunctionRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.FunctionCopy(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) FunctionDelete(c *gin.Context) {
	var req FunctionDeleteRequest
	if err := bindFunctionRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	if err := h.service.FunctionDelete(c.Request.Context(), &req); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{})
}

func (h *Handler) FunctionDisable(c *gin.Context) {
	var req FunctionDisableRequest
	if err := bindFunctionRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	if err := h.service.FunctionDisable(c.Request.Context(), &req); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{})
}

func (h *Handler) FunctionEnable(c *gin.Context) {
	var req FunctionEnableRequest
	if err := bindFunctionRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	if err := h.service.FunctionEnable(c.Request.Context(), &req); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{})
}

func (h *Handler) FunctionHistory(c *gin.Context) {
	var req FunctionHistoryRequest
	if err := bindFunctionRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.FunctionHistory(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) FunctionInvoke(c *gin.Context) {
	var req FunctionInvokeRequest
	if err := bindFunctionRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.FunctionInvoke(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) FunctionPublish(c *gin.Context) {
	var req FunctionPublishRequest
	if err := bindFunctionRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.FunctionPublish(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) FunctionRoute(c *gin.Context) {
	var req FunctionRouteRequest
	if err := bindFunctionRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.FunctionRoute(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) FunctionRouteUpdate(c *gin.Context) {
	var req FunctionRouteUpdateRequest
	if err := bindFunctionRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.FunctionRouteUpdate(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Instance management handlers

func (h *Handler) FunctionInstances(c *gin.Context) {
	var req FunctionInstancesRequest
	if err := bindFunctionRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.FunctionInstances(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) FunctionInstancesAll(c *gin.Context) {
	var req FunctionInstancesAllRequest
	if err := bindFunctionRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.FunctionInstancesAll(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Permission management handlers

func (h *Handler) FunctionPermissions(c *gin.Context) {
	var req FunctionPermissionsRequest
	if err := bindFunctionRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.FunctionPermissions(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) FunctionPermissionsUpdate(c *gin.Context) {
	var req FunctionPermissionsUpdateRequest
	if err := bindFunctionRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	if err := h.service.FunctionPermissionsUpdate(c.Request.Context(), &req); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{})
}

// UI configuration handlers

func (h *Handler) FunctionUI(c *gin.Context) {
	var req FunctionUIRequest
	if err := bindFunctionRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.FunctionUI(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) FunctionUIUpdate(c *gin.Context) {
	var req FunctionUIUpdateRequest
	if err := bindFunctionRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	if err := h.service.FunctionUIUpdate(c.Request.Context(), &req); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{})
}

func (h *Handler) FunctionUIHistory(c *gin.Context) {
	var req FunctionUIHistoryRequest
	if err := bindFunctionRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.FunctionUIHistory(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) FunctionUIRollback(c *gin.Context) {
	var req FunctionUIRollbackRequest
	if err := bindFunctionRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	if err := h.service.FunctionUIRollback(c.Request.Context(), &req); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{})
}

func (h *Handler) FunctionWarnings(c *gin.Context) {
	var req FunctionWarningsRequest
	if err := bindFunctionRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.FunctionWarnings(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Descriptors handlers

func (h *Handler) Descriptors(c *gin.Context) {
	var req DescriptorsRequest
	if err := bindFunctionRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.Descriptors(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Batch operations handlers

func (h *Handler) BatchCopyFunctions(c *gin.Context) {
	var req BatchCopyFunctionsRequest
	if err := bindFunctionRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.BatchCopyFunctions(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) BatchDeleteFunctions(c *gin.Context) {
	var req BatchDeleteFunctionsRequest
	if err := bindFunctionRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.BatchDeleteFunctions(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) BatchUpdateFunctions(c *gin.Context) {
	var req BatchUpdateFunctionsRequest
	if err := bindFunctionRequest(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	resp, err := h.service.BatchUpdateFunctions(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// Alias methods for route compatibility

func (h *Handler) List(c *gin.Context) {
	h.FunctionsList(c)
}

func (h *Handler) Detail(c *gin.Context) {
	h.FunctionDetail(c)
}

func (h *Handler) Delete(c *gin.Context) {
	h.FunctionDelete(c)
}

func (h *Handler) Enable(c *gin.Context) {
	h.FunctionEnable(c)
}

func (h *Handler) Disable(c *gin.Context) {
	h.FunctionDisable(c)
}

func (h *Handler) Copy(c *gin.Context) {
	h.FunctionCopy(c)
}

func (h *Handler) Invoke(c *gin.Context) {
	h.FunctionInvoke(c)
}

func (h *Handler) Publish(c *gin.Context) {
	h.FunctionPublish(c)
}

func (h *Handler) Instances(c *gin.Context) {
	h.FunctionInstances(c)
}

func (h *Handler) InstancesAll(c *gin.Context) {
	h.FunctionInstancesAll(c)
}

func (h *Handler) Permissions(c *gin.Context) {
	h.FunctionPermissions(c)
}

func (h *Handler) PermissionsUpdate(c *gin.Context) {
	h.FunctionPermissionsUpdate(c)
}

func (h *Handler) UI(c *gin.Context) {
	h.FunctionUI(c)
}

func (h *Handler) UIUpdate(c *gin.Context) {
	h.FunctionUIUpdate(c)
}

func (h *Handler) UIHistory(c *gin.Context) {
	h.FunctionUIHistory(c)
}

func (h *Handler) UIRollback(c *gin.Context) {
	h.FunctionUIRollback(c)
}

func (h *Handler) Route(c *gin.Context) {
	h.FunctionRoute(c)
}

func (h *Handler) RouteUpdate(c *gin.Context) {
	h.FunctionRouteUpdate(c)
}

func (h *Handler) History(c *gin.Context) {
	h.FunctionHistory(c)
}

func (h *Handler) Analytics(c *gin.Context) {
	h.FunctionAnalytics(c)
}

func (h *Handler) Warnings(c *gin.Context) {
	h.FunctionWarnings(c)
}

func (h *Handler) Pending(c *gin.Context) {
	h.FunctionsPending(c)
}

func (h *Handler) BatchUpdate(c *gin.Context) {
	h.BatchUpdateFunctions(c)
}

func (h *Handler) BatchCopy(c *gin.Context) {
	h.BatchCopyFunctions(c)
}

func (h *Handler) BatchDelete(c *gin.Context) {
	h.BatchDeleteFunctions(c)
}
