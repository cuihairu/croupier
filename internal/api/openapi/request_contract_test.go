package openapi

import (
	"os"
	"testing"

	driveropenapi "github.com/cuihairu/croupier/internal/drivers/openapi"
	"github.com/cuihairu/croupier/internal/platform/provider"
	"github.com/stretchr/testify/require"
)

const requestFixture = "../../../testdata/openapi/sui-request.yaml"

func TestSourceRequestContract(t *testing.T) {
	raw, err := os.ReadFile(requestFixture)
	require.NoError(t, err)
	s := setupOpenAPITestService(t)
	source, err := s.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{
		Name: "Request fixture", Spec: raw,
	})
	require.NoError(t, err)
	stored, err := s.GetSource(openAPITestContext(), &OpenAPISourceGetRequest{SourceID: source.Source.SourceID})
	require.NoError(t, err)
	update, ok := findSourceOperation(stored.Source.Operations, "updatePlayer")
	require.True(t, ok)
	require.NotNil(t, update.Request)
	require.Len(t, update.Request.Parameters, 4)
	path, query, header, cookie := update.Request.Parameters[0], update.Request.Parameters[1], update.Request.Parameters[2], update.Request.Parameters[3]
	require.Equal(t, "id", path.Name)
	require.Equal(t, "path", path.In)
	require.True(t, path.Required)
	require.Equal(t, "simple", path.Style)
	require.Equal(t, "#/paths/~1players~1{id}/parameters/0", path.Origin.Pointer)
	require.Equal(t, "filter", query.Name)
	require.Equal(t, "query", query.In)
	require.Equal(t, "form", query.Style)
	require.NotNil(t, query.Explode)
	require.False(t, *query.Explode)
	require.JSONEq(t, `{"type":"string"}`, string(query.Schema))
	require.Equal(t, "#/paths/~1players~1{id}/post/parameters/0", query.Origin.Pointer)
	require.Equal(t, "X-Tenant", header.Name)
	require.Equal(t, "header", header.In)
	require.True(t, header.Required)
	require.Equal(t, "cookie", cookie.In)
	require.True(t, *cookie.Explode)
	require.True(t, update.Request.Body.Required)
	require.Equal(t, "#/paths/~1players~1{id}/post/requestBody", update.Request.Body.Origin.Pointer)
	require.JSONEq(t, `{"type":"object","additionalProperties":false,"minProperties":1,"required":["filter"],"properties":{"filter":{"type":"integer"}}}`, string(update.Request.Body.Content["application/json"].Schema))

	bulk, ok := findSourceOperation(stored.Source.Operations, "bulkPlayers")
	require.True(t, ok)
	require.JSONEq(t, `{"type":"array","minItems":1,"items":{"type":"integer"}}`, string(bulk.Request.Body.Content["application/json"].Schema))
}

func TestSourceAndProviderRequestContractParity(t *testing.T) {
	raw, err := os.ReadFile(requestFixture)
	require.NoError(t, err)
	p := driveropenapi.NewProvider()
	require.NoError(t, p.Init(t.Context(), provider.ProviderConfig{
		Config: map[string]any{"openapiSpec": requestFixture},
	}))
	methods := p.GetMethodDetails()
	source, err := setupOpenAPITestService(t).CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{Spec: raw})
	require.NoError(t, err)
	require.Len(t, methods, len(source.Source.Operations))
	for _, op := range source.Source.Operations {
		require.Contains(t, methods, op.OperationID)
		require.NotNil(t, methods[op.OperationID].Request)
		require.Equal(t, op.Request, methods[op.OperationID].Request, op.OperationID)
	}
}

func TestLegacySourceBindingRereadAndReimport(t *testing.T) {
	raw, err := os.ReadFile(requestFixture)
	require.NoError(t, err)
	s := setupOpenAPITestService(t)
	ctx := openAPITestContext()
	created, err := s.CreateSource(ctx, &OpenAPISourceCreateRequest{Spec: raw})
	require.NoError(t, err)
	sourceID := created.Source.SourceID
	// Recreate the previous storage format before binding these operations.
	model := s.svcCtx.OpenAPISourceModel
	row, err := model.FindByScopeAndSourceID(ctx, "demo-game", "development", sourceID)
	require.NoError(t, err)
	operations := created.Source.Operations
	for i := range operations {
		operations[i].Request = nil
	}
	require.NoError(t, row.SetOperations(operations))
	require.NoError(t, row.SetDiagnostics(nil))
	require.NoError(t, model.Update(ctx, row))

	for operation, function := range map[string]string{"createCompatible": "player.list", "updatePlayer": "player.get"} {
		_, err := s.CreateBinding(ctx, &OpenAPISourceBindingCreateRequest{
			SourceID: sourceID, OperationID: operation, FunctionID: function, Kind: "provider",
		})
		require.NoError(t, err)
	}
	before := contractsByFunctionID(t, s)
	compatibleSchema := `{"type":"object","properties":{"name":{"type":"string"},"scope":{"type":"string"}},"required":["name"]}`
	require.JSONEq(t, compatibleSchema, string(before["player.list"].InputSchema))

	reread, err := s.GetSource(ctx, &OpenAPISourceGetRequest{SourceID: sourceID})
	require.NoError(t, err)
	legacy, ok := findSourceOperation(reread.Source.Operations, "updatePlayer")
	require.True(t, ok)
	require.True(t, legacy.Bound)
	require.NotNil(t, legacy.Request)
	require.Equal(t, "player.get", legacy.FunctionID)
	require.Equal(t, before["player.get"].InputSchema, contractsByFunctionID(t, s)["player.get"].InputSchema)
	oldDiagnostics, err := s.SourceDiagnostics(ctx, &OpenAPISourceGetRequest{SourceID: sourceID})
	require.NoError(t, err)

	reimported, err := s.UpdateSource(ctx, &OpenAPISourceUpdateRequest{SourceID: sourceID, Spec: raw})
	require.NoError(t, err)
	for _, want := range []struct{ code, field string }{
		{driveropenapi.FlatInputCollision, "/properties/filter"},
		{driveropenapi.FlatInputConstraint, "/minProperties"},
		{driveropenapi.FlatInputConstraint, "/additionalProperties"},
	} {
		found := false
		for _, diagnostic := range oldDiagnostics.Diagnostics {
			if diagnostic.Code == want.code && diagnostic.Field == "#/paths/~1players~1{id}/post/requestBody/content/application~1json/schema"+want.field {
				require.Equal(t, "updatePlayer", diagnostic.FunctionID)
				require.Contains(t, reimported.Source.Diagnostics, diagnostic)
				found = true
			}
		}
		require.True(t, found, "missing diagnostic for %s", want.field)
	}
	after := contractsByFunctionID(t, s)
	require.JSONEq(t, compatibleSchema, string(after["player.list"].InputSchema))
	require.JSONEq(t, `{
		"type":"object","additionalProperties":false,"required":["path","header","body"],
		"properties":{
			"path":{"type":"object","additionalProperties":false,"required":["id"],"properties":{"id":{"type":"string"}}},
			"query":{"type":"object","additionalProperties":false,"properties":{"filter":{"type":"string"}}},
			"header":{"type":"object","additionalProperties":false,"required":["X-Tenant"],"properties":{"X-Tenant":{"type":"string"}}},
			"cookie":{"type":"object","additionalProperties":false,"properties":{"session":{"type":"string"}}},
			"body":{"type":"object","additionalProperties":false,"minProperties":1,"required":["filter"],"properties":{"filter":{"type":"integer"}}}
		}
	}`, string(after["player.get"].InputSchema))
}
