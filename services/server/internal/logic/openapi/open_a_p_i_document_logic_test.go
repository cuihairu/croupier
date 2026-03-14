package openapi

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/getkin/kin-openapi/openapi3"
)

func TestOpenAPIDocumentBuild(t *testing.T) {
	store := registry.NewStore()
	err := store.UpsertOpenAPI("player.ban", &openapi3.Operation{
		OperationID: "player.ban",
		Summary:     "Ban player",
		Responses: openapi3.NewResponses(
			openapi3.WithName("200", openapi3.NewResponse()),
		),
	})
	if err != nil {
		t.Fatalf("UpsertOpenAPI failed: %v", err)
	}

	logic := NewOpenAPIDocumentLogic(context.Background(), &svc.ServiceContext{
		RegistryStore: store,
	})
	resp, err := logic.OpenAPIDocument(&types.OpenAPIDocumentRequest{})
	if err != nil {
		t.Fatalf("OpenAPIDocument failed: %v", err)
	}
	spec, ok := resp.Spec.(*openapi3.T)
	if !ok {
		t.Fatalf("unexpected spec type: %T", resp.Spec)
	}
	if spec.OpenAPI != "3.0.3" {
		t.Fatalf("unexpected openapi version: %v", spec.OpenAPI)
	}
	if len(spec.Paths.Map()) == 0 {
		t.Fatalf("expected non-empty paths")
	}
}
