package openapi

import (
	"testing"

	"github.com/cuihairu/croupier/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// unboundSpecFixture 两个操作：user.list（无绑定 → unbound）与
// listPlayers（可显式绑定到运行时 player.list → bound 路径）。
func unboundSpecFixture() map[string]interface{} {
	return map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":   "Unbound API",
			"version": "1.0.0",
		},
		"paths": map[string]interface{}{
			"/users": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "user.list",
					"summary":     "List users",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Success",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{"type": "object"},
								},
							},
						},
					},
				},
				"post": map[string]interface{}{
					"operationId": "user.create",
					"summary":     "Create user",
					"responses": map[string]interface{}{
						"201": map[string]interface{}{"description": "Created"},
					},
				},
			},
			"/players": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "listPlayers",
					"summary":     "List players",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{"description": "Success"},
					},
				},
			},
		},
	}
}

func contractsByFunctionID(t *testing.T, s *Service) map[string]*model.FunctionContract {
	t.Helper()
	contracts, err := model.NewFunctionContractModel(s.svcCtx.DB).ListByScope(
		openAPITestContext(), "demo-game", "development")
	require.NoError(t, err)
	out := make(map[string]*model.FunctionContract, len(contracts))
	for _, contract := range contracts {
		out[contract.FunctionID] = contract
	}
	return out
}

// T4/D4 验收：上传文档（无 agent 参与）后契约直接落库且 executionState
// =unbound，functionId 为 operationId 的确定性映射。
func TestService_CreateSourceGeneratesUnboundContracts(t *testing.T) {
	t.Parallel()

	service := setupOpenAPITestService(t)
	resp, err := service.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{
		Name: "Unbound API",
		Spec: rawSpec(t, unboundSpecFixture()),
	})
	require.NoError(t, err)
	require.NotEmpty(t, resp.Source.SourceID)

	contracts := contractsByFunctionID(t, service)
	// camelCase operationId（listPlayers）归一为平台 functionId 字符集。
	for _, functionID := range []string{"user.list", "user.create", "listplayers"} {
		contract, ok := contracts[functionID]
		require.True(t, ok, "operation %s should materialize a contract", functionID)
		assert.Equal(t, "unbound", contract.ExecutionState, "%s must be unbound material", functionID)
		assert.Equal(t, "openapi", contract.Source)
	}
}

// T4 验收：重复上传幂等——CreateSource 后 UpdateSource 同文档，契约不
// 重建、不翻状态、数量不变。
func TestService_UpdateSourceUnboundContractsIdempotent(t *testing.T) {
	t.Parallel()

	service := setupOpenAPITestService(t)
	created, err := service.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{
		Name: "Unbound API",
		Spec: rawSpec(t, unboundSpecFixture()),
	})
	require.NoError(t, err)

	_, err = service.UpdateSource(openAPITestContext(), &OpenAPISourceUpdateRequest{
		SourceID: created.Source.SourceID,
		Spec:     rawSpec(t, unboundSpecFixture()),
	})
	require.NoError(t, err)

	contracts := contractsByFunctionID(t, service)
	assert.Len(t, contracts, 3)
	for functionID, contract := range contracts {
		assert.Equal(t, "unbound", contract.ExecutionState, "%s stays unbound on re-upload", functionID)
	}
}

// T4 验收：已存在 bound 契约的 operation 不降级——显式绑定（provider）
// 路径产出的 bound 契约在上传管线重放后保持 bound，未绑定操作仍 unbound。
func TestService_UpdateSourceDoesNotDowngradeBoundContract(t *testing.T) {
	t.Parallel()

	service := setupOpenAPITestService(t)
	created, err := service.CreateSource(openAPITestContext(), &OpenAPISourceCreateRequest{
		Name: "Unbound API",
		Spec: rawSpec(t, unboundSpecFixture()),
	})
	require.NoError(t, err)

	// listPlayers 显式绑定到运行时已注册的 player.list（agent fixture）。
	_, err = service.CreateBinding(openAPITestContext(), &OpenAPISourceBindingCreateRequest{
		SourceID:    created.Source.SourceID,
		OperationID: "listPlayers",
		Kind:        "provider",
		FunctionID:  "player.list",
	})
	require.NoError(t, err)

	contracts := contractsByFunctionID(t, service)
	require.NotNil(t, contracts["player.list"])
	require.Equal(t, "bound", contracts["player.list"].ExecutionState,
		"explicit binding path materializes a bound contract")

	_, err = service.UpdateSource(openAPITestContext(), &OpenAPISourceUpdateRequest{
		SourceID: created.Source.SourceID,
		Spec:     rawSpec(t, unboundSpecFixture()),
	})
	require.NoError(t, err)

	contracts = contractsByFunctionID(t, service)
	assert.Equal(t, "bound", contracts["player.list"].ExecutionState,
		"bound contract must not be downgraded by upload pipeline")
	assert.Equal(t, "unbound", contracts["user.list"].ExecutionState)
	assert.Equal(t, "unbound", contracts["user.create"].ExecutionState)
	assert.NotContains(t, contracts, "listplayers",
		"绑定后先前上传在 operationId 名下的 unbound 物料已被取代，重放时清理")
}
