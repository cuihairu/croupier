package openapi

import (
	"os"
	"testing"

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
