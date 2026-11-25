package openapi

import (
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SUI-011 through the real upload path: a request the executor cannot send
// lands on the unbound contract row, so the projection (and therefore the
// default proposal quality gate) sees the same finding the source detail shows.
func TestService_CreateSourceStoresRequestCompatibilityOnContract(t *testing.T) {
	service := setupOpenAPITestService(t)
	specDoc := map[string]interface{}{
		"openapi": "3.0.3",
		"info":    map[string]interface{}{"title": "Upload", "version": "1"},
		"paths": map[string]interface{}{
			"/avatars": map[string]interface{}{
				"post": map[string]interface{}{
					"operationId": "uploadAvatar",
					"responses": map[string]interface{}{
						"201": map[string]interface{}{
							"description": "Created",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{"type": "object"},
								},
							},
						},
					},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"multipart/form-data": map[string]interface{}{
								"schema": map[string]interface{}{"type": "object"},
							},
						},
					},
				},
			},
		},
	}
	created, err := service.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{
		Name: "Upload",
		Spec: rawSpec(t, specDoc),
	})
	require.NoError(t, err)
	require.NotEmpty(t, created.Source.SourceID)

	contracts := contractsByFunctionID(t, service)
	contract, ok := contracts["uploadavatar"]
	require.True(t, ok, "the operation must materialize a contract")

	var diagnostics []spec.Diagnostic
	require.NoError(t, json.Unmarshal([]byte(contract.Diagnostics), &diagnostics))
	found := false
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == spec.DiagSchemaCompatibilityManual && diagnostic.Field == "$.body.content" {
			found = true
		}
	}
	assert.True(t, found, "expected the manual media type finding on the contract row, got %+v", diagnostics)

	// The generated default proposal must not be directly executable either.
	var proposal model.PageProposal
	require.NoError(t, service.svcCtx.DB.Unscoped().
		Where("game_id = ? AND env = ? AND proposal_key = ?", "demo-game", "development", "operation:uploadavatar").
		First(&proposal).Error)
	assert.Equal(t, "needs_review", proposal.Quality,
		"an operation the executor cannot send must not get a directly executable default proposal")
}
