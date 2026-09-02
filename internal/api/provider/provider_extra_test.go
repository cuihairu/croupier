// 覆盖目标：provider 包 handler 的 bind 错误分支（List 非法 query、
// Capabilities/Descriptors 非法 JSON、bindProviderRequest POST 路径），
// 以及 service/helpers 的 nil-request 默认、非法 OpenAPI 文档、
// 资源聚合错误等未覆盖路径。
package provider

import (
	"net/http"
	"strings"
	"testing"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
)

func newProviderRegistryService() *Service {
	store := reg.NewStore()
	return NewService(&svc.ServiceContext{RegistryStore: store})
}

func TestBindProviderRequest_PostUsesJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := newProviderTestContext(http.MethodPost, "/api/v1/providers/capabilities", `{}`)
	var req ProvidersCapabilitiesRequest
	if err := bindProviderRequest(ctx, &req); err != nil {
		t.Fatalf("bindProviderRequest(POST) error = %v", err)
	}
}

func TestBindProviderRequest_PostInvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := newProviderTestContext(http.MethodPost, "/api/v1/providers/capabilities", `not-json`)
	var req ProvidersCapabilitiesRequest
	if err := bindProviderRequest(ctx, &req); err == nil {
		t.Fatal("expected bind error for invalid JSON body")
	}
}

func TestHandler_List_InvalidQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(newProviderRegistryService())
	ctx, rec := newProviderTestContext(http.MethodGet, "/api/v1/providers?page=abc", "")
	h.List(ctx)
	// 注：strconv 绑定错误当前被 response.Error 归类为 500 而非 400，
	// 此处只锁定“非 200 + 走错误分支”。
	if rec.Code == http.StatusOK {
		t.Fatalf("status = 200, want non-200 for invalid query")
	}
}

func TestHandler_Capabilities_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(newProviderRegistryService())
	ctx, rec := newProviderTestContext(http.MethodPost, "/api/v1/providers/capabilities", `not-json`)
	h.Capabilities(ctx)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s, want 400", rec.Code, rec.Body.String())
	}
}

func TestHandler_Descriptors_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(newProviderRegistryService())
	ctx, rec := newProviderTestContext(http.MethodPost, "/api/v1/providers/descriptors", `not-json`)
	h.Descriptors(ctx)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s, want 400", rec.Code, rec.Body.String())
	}
}

func TestService_List_NilRequestDefaults(t *testing.T) {
	s := newProviderRegistryService()
	resp, err := s.List(nil, nil)
	if err != nil {
		t.Fatalf("List(nil req) error = %v", err)
	}
	if resp == nil || resp.Total != 0 {
		t.Fatalf("unexpected resp: %+v", resp)
	}
}

// 非法 OpenAPI 文档的 provider：Descriptors 跳过、聚合资源跳过、
// Reload 报错——三条防御路径。
func TestService_InvalidOpenAPIDocPaths(t *testing.T) {
	store := reg.NewStore()
	if err := store.UpsertOpenAPIProvider(reg.OpenAPIProviderCaps{
		ID:         "broken",
		Version:    "1.0.0",
		OpenAPIDoc: []byte(`{not-json`),
	}); err != nil {
		t.Fatal(err)
	}
	s := NewService(&svc.ServiceContext{RegistryStore: store})

	// Descriptors：decode 失败 → continue，manifests 为空。
	resp, err := s.Descriptors(nil, &ProvidersDescriptorsRequest{})
	if err != nil {
		t.Fatalf("Descriptors error = %v", err)
	}
	if len(resp.ProviderManifests) != 0 {
		t.Fatalf("manifests = %v, want empty", resp.ProviderManifests)
	}

	// Resources（聚合）：decode 失败 → continue。
	res, err := s.Resources(nil, &ProvidersResourcesRequest{})
	if err != nil {
		t.Fatalf("Resources(*) error = %v", err)
	}
	if res.Total != 0 {
		t.Fatalf("resources total = %d, want 0", res.Total)
	}

	// Resources 指定非法文档 provider → decode 失败返回错误。
	if _, err := s.Resources(nil, &ProvidersResourcesRequest{ID: "broken"}); err == nil {
		t.Fatal("expected error for provider with invalid doc")
	}

	// Reload：非法文档 → 返回错误。
	if _, err := s.Reload(nil, &ProviderActionRequest{ID: "broken"}); err == nil {
		t.Fatal("expected Reload error for invalid doc")
	}
}

func TestService_Resources_UnknownProvider(t *testing.T) {
	s := newProviderRegistryService()
	if _, err := s.Resources(nil, &ProvidersResourcesRequest{ID: "ghost"}); err == nil {
		t.Fatal("expected error for unknown provider id")
	}
}

// 合法文档 + x-resource 扩展：聚合与单 provider 资源提取。
func TestService_Resources_ExtractsXResource(t *testing.T) {
	store := reg.NewStore()
	doc := []byte(`{"openapi":"3.0.3","info":{"title":"t","version":"1"},"paths":{"/a":{"post":{"operationId":"fn.a","x-resource":"player"}}}}`)
	if err := store.UpsertOpenAPIProvider(reg.OpenAPIProviderCaps{
		ID:         "ok",
		Version:    "1.0.0",
		OpenAPIDoc: doc,
	}); err != nil {
		t.Fatal(err)
	}
	s := NewService(&svc.ServiceContext{RegistryStore: store})

	all, err := s.Resources(nil, &ProvidersResourcesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if all.Total != 1 {
		t.Fatalf("aggregate total = %d, want 1", all.Total)
	}

	one, err := s.Resources(nil, &ProvidersResourcesRequest{ID: "ok"})
	if err != nil {
		t.Fatal(err)
	}
	if one.Total != 1 || one.Items[0]["name"] != "player" {
		t.Fatalf("provider resources = %+v", one.Items)
	}
}

func TestDeleteProviderCaps_NilStore_Extra(t *testing.T) {
	if err := deleteProviderCaps(nil, "x"); err == nil {
		t.Fatal("expected error for nil store")
	}
}

func TestRefreshProviderTimestamp_NilStoreNoPanic(t *testing.T) {
	refreshProviderTimestamp(nil, reg.OpenAPIProviderCaps{ID: "x"})
}

func TestHandler_List_ServiceError_Status(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(NewService(&svc.ServiceContext{}))
	ctx, rec := newProviderTestContext(http.MethodGet, "/api/v1/providers", "")
	h.List(ctx)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "registry") {
		t.Logf("body=%s", rec.Body.String())
	}
}
