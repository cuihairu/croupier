// Coverage target: the bind error branches of the provider package handlers
// (List with an invalid query, Capabilities/Descriptors with invalid JSON, the
// bindProviderRequest POST path), plus previously uncovered service/helpers
// paths such as the nil-request default, invalid OpenAPI documents, and
// resource aggregation errors.
package provider

import (
	"context"
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
	// Note: a strconv binding error is currently classified by response.Error
	// as 500 rather than 400, so here we only assert "non-200 + error branch
	// taken".
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
	resp, err := s.List(context.TODO(), nil)
	if err != nil {
		t.Fatalf("List(nil req) error = %v", err)
	}
	if resp == nil || resp.Total != 0 {
		t.Fatalf("unexpected resp: %+v", resp)
	}
}

// A provider with an invalid OpenAPI document: Descriptors skips it,
// aggregated resources skip it, and Reload errors out -- three defensive
// paths.
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

	// Descriptors: decode fails -> continue, manifests is empty.
	resp, err := s.Descriptors(context.TODO(), &ProvidersDescriptorsRequest{})
	if err != nil {
		t.Fatalf("Descriptors error = %v", err)
	}
	if len(resp.ProviderManifests) != 0 {
		t.Fatalf("manifests = %v, want empty", resp.ProviderManifests)
	}

	// Resources (aggregated): decode fails -> continue.
	res, err := s.Resources(context.TODO(), &ProvidersResourcesRequest{})
	if err != nil {
		t.Fatalf("Resources(*) error = %v", err)
	}
	if res.Total != 0 {
		t.Fatalf("resources total = %d, want 0", res.Total)
	}

	// Resources with the invalid-document provider specified -> decode fails
	// and returns an error.
	if _, err := s.Resources(context.TODO(), &ProvidersResourcesRequest{ID: "broken"}); err == nil {
		t.Fatal("expected error for provider with invalid doc")
	}

	// Reload: invalid document -> returns an error.
	if _, err := s.Reload(context.TODO(), &ProviderActionRequest{ID: "broken"}); err == nil {
		t.Fatal("expected Reload error for invalid doc")
	}
}

func TestService_Resources_UnknownProvider(t *testing.T) {
	s := newProviderRegistryService()
	if _, err := s.Resources(context.TODO(), &ProvidersResourcesRequest{ID: "ghost"}); err == nil {
		t.Fatal("expected error for unknown provider id")
	}
}

// A valid document + x-resource extension: aggregated and single-provider
// resource extraction.
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

	all, err := s.Resources(context.TODO(), &ProvidersResourcesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if all.Total != 1 {
		t.Fatalf("aggregate total = %d, want 1", all.Total)
	}

	one, err := s.Resources(context.TODO(), &ProvidersResourcesRequest{ID: "ok"})
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
