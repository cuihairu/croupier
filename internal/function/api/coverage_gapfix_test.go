package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/function/registry"
	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// faultStore 在真实 registry.Store 之上注入可配置的 Filter/Register 故障，
// 用于覆盖 registry.Store 恒返回 nil error / handler 已前置拦截而不可达的错误分支。
type faultStore struct {
	*registry.Store
	filterErr   error
	registerErr error
}

func (f *faultStore) Filter(ctx context.Context, filter *functionv1.FunctionFilter) ([]*functionv1.FunctionMetadata, error) {
	if f.filterErr != nil {
		return nil, f.filterErr
	}
	return f.Store.Filter(ctx, filter)
}

func (f *faultStore) Register(ctx context.Context, metadata *functionv1.FunctionMetadata) error {
	if f.registerErr != nil {
		return f.registerErr
	}
	return f.Store.Register(ctx, metadata)
}

func newFaultService(filterErr, registerErr error) *Service {
	return &Service{store: &faultStore{Store: registry.NewStore(), filterErr: filterErr, registerErr: registerErr}}
}

func newGapfixRouter(service *Service) *gin.Engine {
	handler := NewHandler(service)
	router := gin.New()
	functions := router.Group("/api/v1/metadata/functions")
	{
		functions.GET("", handler.ListFunctions)
		functions.POST("", handler.RegisterFunction)
	}
	return router
}

// Service.List 的错误分支：store.Filter 故障时错误以 "filter functions:"
// 前缀包装返回。
func TestGapfixService_List_FilterError(t *testing.T) {
	service := newFaultService(errors.New("store filter boom"), nil)

	result, err := service.List(testCtx(), &ListOptions{})
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "filter functions")
	assert.Contains(t, err.Error(), "store filter boom")
}

// ListFunctions handler 的错误分支：service.List 失败按非 errorx 错误
// 以 500 internal_error 兜底，message 透传底层错误文本。
func TestGapfixHandler_ListFunctions_ServiceError(t *testing.T) {
	router := newGapfixRouter(newFaultService(errors.New("store filter boom"), nil))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/metadata/functions?page=1&pageSize=10", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	body := map[string]interface{}{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "internal_error", body["error"])
	msg, _ := body["message"].(string)
	assert.Contains(t, msg, "filter functions")
}

// RegisterFunction handler 的错误分支：service.Register 失败映射为
// 409 conflict，message 透传底层错误文本。
func TestGapfixHandler_RegisterFunction_ServiceError(t *testing.T) {
	router := newGapfixRouter(newFaultService(nil, errors.New("store register boom")))

	body := `{"id":"player.seam","name":"Seam"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/metadata/functions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusConflict, w.Code)
	respBody := map[string]interface{}{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &respBody))
	assert.Equal(t, "conflict", respBody["error"])
	msg, _ := respBody["message"].(string)
	assert.Contains(t, msg, "store register boom")
}
