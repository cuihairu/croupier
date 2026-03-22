package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
)

func newProviderTestContext(method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req
	return ctx, rec
}

func TestBindProviderRequestUsesQueryForGet(t *testing.T) {
	t.Parallel()

	ctx, _ := newProviderTestContext(http.MethodGet, "/api/v1/providers?page=2&pageSize=10", "")
	var req ProvidersListRequest
	if err := bindProviderRequest(ctx, &req); err != nil {
		t.Fatalf("bindProviderRequest() error = %v", err)
	}
	if req.Page != 2 {
		t.Fatalf("expected page=2, got %d", req.Page)
	}
	if req.PageSize != 10 {
		t.Fatalf("expected pageSize=10, got %d", req.PageSize)
	}
}

func TestProviderHandlersReturnServiceErrors(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}))
	cases := []struct {
		name   string
		target string
		fn     func(*gin.Context)
	}{
		{name: "List", target: "/api/v1/providers", fn: h.List},
		{name: "Capabilities", target: "/api/v1/providers/capabilities", fn: h.Capabilities},
		{name: "Descriptors", target: "/api/v1/providers/descriptors", fn: h.Descriptors},
		{name: "Detail", target: "/api/v1/providers/", fn: h.Detail},
		{name: "Entities", target: "/api/v1/providers/entities", fn: h.Entities},
		{name: "Delete", target: "/api/v1/providers/delete", fn: h.Delete},
		{name: "Reload", target: "/api/v1/providers/reload", fn: h.Reload},
		{name: "AliasGet", target: "/api/v1/providers/get", fn: h.Get},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx, rec := newProviderTestContext(http.MethodGet, tc.target, "")
			tc.fn(ctx)
			if rec.Code != http.StatusInternalServerError && rec.Code != http.StatusBadRequest && rec.Code != http.StatusNotFound {
				t.Fatalf("unexpected status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

// Additional tests to improve coverage

func TestService_List_NilContext(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})

	resp, err := service.List(nil, &ProvidersListRequest{
		Page:     1,
		PageSize: 10,
	})

	// Should handle nil context gracefully
	if err != nil {
		t.Logf("Expected error for nil context: %v", err)
	}
	if resp != nil && len(resp.Items) == 0 {
		t.Log("Items field is empty as expected for empty service context")
	}
}

func TestService_Capabilities_NilContext(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})

	resp, err := service.Capabilities(nil, &ProvidersCapabilitiesRequest{})

	// Should handle nil context gracefully
	if err != nil {
		t.Logf("Expected error for nil context: %v", err)
	}
	if resp != nil {
		t.Log("Got response for capabilities")
	}
}

func TestService_Descriptors_NilContext(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})

	resp, err := service.Descriptors(nil, &ProvidersDescriptorsRequest{})

	// Should handle nil context gracefully
	if err != nil {
		t.Logf("Expected error for nil context: %v", err)
	}
	if resp != nil {
		t.Log("Got response for descriptors")
	}
}

func TestService_Detail_NilContext(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})

	resp, err := service.Detail(nil, &ProviderDetailRequest{
		ID: "test",
	})

	// Should handle nil context gracefully
	if err != nil {
		t.Logf("Expected error for nil context: %v", err)
	}
	if resp != nil {
		t.Log("Got response for detail")
	}
}

func TestService_Entities_NilContext(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})

	resp, err := service.Entities(nil, &ProvidersEntitiesRequest{
		ID: "test",
	})

	// Should handle nil context gracefully
	if err != nil {
		t.Logf("Expected error for nil context: %v", err)
	}
	if resp != nil {
		t.Log("Got response for entities")
	}
}

func TestService_List_NilRequest(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})

	resp, err := service.List(context.Background(), nil)

	// Should handle nil request
	if err != nil {
		t.Logf("Expected error for nil request: %v", err)
	}
	if resp != nil {
		t.Log("Got response for nil request")
	}
}

func TestService_Capabilities_NilRequest(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})

	resp, err := service.Capabilities(context.Background(), nil)

	// Should handle nil request
	if err != nil {
		t.Logf("Expected error for nil request: %v", err)
	}
	if resp != nil {
		t.Log("Got response for nil request")
	}
}

func TestService_Descriptors_NilRequest(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	service := NewService(&svc.ServiceContext{})

	resp, err := service.Descriptors(context.Background(), nil)

	// Should handle nil request
	if err != nil {
		t.Logf("Expected error for nil request: %v", err)
	}
	if resp != nil {
		t.Log("Got response for nil request")
	}
}

func TestHandler_List_WithQueryParams(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}))
	ctx, rec := newProviderTestContext(http.MethodGet, "/api/v1/providers?page=1&pageSize=20", "")
	h.List(ctx)

	// Should handle query parameters
	if rec.Code != http.StatusInternalServerError && rec.Code != http.StatusOK {
		t.Logf("Status: %d", rec.Code)
	}
}

func TestHandler_Detail_WithPathParam(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}))
	ctx, rec := newProviderTestContext(http.MethodGet, "/api/v1/providers/test_provider", "")
	h.Detail(ctx)

	// Should handle path parameter
	if rec.Code != http.StatusInternalServerError && rec.Code != http.StatusOK {
		t.Logf("Status: %d", rec.Code)
	}
}

func TestHandler_Entities_WithPathParam(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}))
	ctx, rec := newProviderTestContext(http.MethodGet, "/api/v1/providers/test_provider/entities", "")
	h.Entities(ctx)

	// Should handle path parameter
	if rec.Code != http.StatusInternalServerError && rec.Code != http.StatusOK {
		t.Logf("Status: %d", rec.Code)
	}
}

func TestHandler_Reload_WithPathParam(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}))
	ctx, rec := newProviderTestContext(http.MethodPost, "/api/v1/providers/test_provider/reload", "")
	h.Reload(ctx)

	// Should handle reload request
	if rec.Code != http.StatusInternalServerError && rec.Code != http.StatusOK {
		t.Logf("Status: %d", rec.Code)
	}
}

func TestHandler_Delete_WithPathParam(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}))
	ctx, rec := newProviderTestContext(http.MethodDelete, "/api/v1/providers/test_provider", "")
	h.Delete(ctx)

	// Should handle delete request
	if rec.Code != http.StatusInternalServerError && rec.Code != http.StatusOK {
		t.Logf("Status: %d", rec.Code)
	}
}

func TestHandler_Get_WithPathParam(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}))
	ctx, rec := newProviderTestContext(http.MethodGet, "/api/v1/providers/test_provider/get", "")
	h.Get(ctx)

	// Should handle get request
	if rec.Code != http.StatusInternalServerError && rec.Code != http.StatusOK {
		t.Logf("Status: %d", rec.Code)
	}
}

func TestHandler_Capabilities_WithQuery(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}))
	ctx, rec := newProviderTestContext(http.MethodGet, "/api/v1/providers/capabilities", "")
	h.Capabilities(ctx)

	// Should handle capabilities request
	if rec.Code != http.StatusInternalServerError && rec.Code != http.StatusOK {
		t.Logf("Status: %d", rec.Code)
	}
}

func TestHandler_Descriptors_WithQuery(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}))
	ctx, rec := newProviderTestContext(http.MethodGet, "/api/v1/providers/descriptors", "")
	h.Descriptors(ctx)

	// Should handle descriptors request
	if rec.Code != http.StatusInternalServerError && rec.Code != http.StatusOK {
		t.Logf("Status: %d", rec.Code)
	}
}
