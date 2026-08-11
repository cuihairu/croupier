package resource

import (
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestApprovalPolicyFromJSONMap(t *testing.T) {
	tests := []struct {
		name   string
		values map[string]interface{}
		want   spec.ApprovalPolicy
	}{
		{"nil", nil, spec.ApprovalPolicy{}},
		{"empty", map[string]interface{}{}, spec.ApprovalPolicy{}},
		{"required with policyKey", map[string]interface{}{"required": true, "policyKey": "admin"}, spec.ApprovalPolicy{Required: true, PolicyKey: "admin"}},
		{"required with snake_case", map[string]interface{}{"required": true, "policy_key": "admin"}, spec.ApprovalPolicy{Required: true, PolicyKey: "admin"}},
		{"not required", map[string]interface{}{"required": false}, spec.ApprovalPolicy{Required: false}},
		{"policyKey takes precedence", map[string]interface{}{"policyKey": "camel", "policy_key": "snake"}, spec.ApprovalPolicy{PolicyKey: "camel"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := approvalPolicyFromJSONMap(tt.values)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestLocalizedFromJSONMap(t *testing.T) {
	tests := []struct {
		name     string
		values   map[string]interface{}
		fallback string
		wantLen  int
	}{
		{"nil", nil, "", 0},
		{"empty", map[string]interface{}{}, "", 0},
		{"with values", map[string]interface{}{"zh-CN": "玩家", "en-US": "player"}, "", 2},
		{"with fallback", nil, "player", 2},
		{"empty fallback", nil, "", 0},
		{"non-string value ignored", map[string]interface{}{"zh-CN": 123}, "", 0},
		{"whitespace key ignored", map[string]interface{}{"  ": "value"}, "", 0},
		{"whitespace value ignored", map[string]interface{}{"zh-CN": "  "}, "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := localizedFromJSONMap(tt.values, tt.fallback)
			assert.Len(t, result, tt.wantLen)
		})
	}
}

func TestDiagnosticsFromJSON_Resource(t *testing.T) {
	tests := []struct {
		name     string
		raw      []byte
		fallback string
		wantLen  int
	}{
		{"nil", nil, "", 0},
		{"empty", []byte{}, "", 0},
		{"invalid", []byte(`{invalid`), "fn1", 1},
		{"valid", []byte(`[{"code":"test","severity":"info","message":"ok"}]`), "", 1},
		{"fallback applied", []byte(`[{"code":"test","severity":"info","message":"ok"}]`), "fn1", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := diagnosticsFromJSON(tt.raw, tt.fallback)
			assert.Len(t, result, tt.wantLen)
			if tt.wantLen == 1 && tt.fallback != "" && len(tt.raw) > 0 && tt.raw[0] == '[' {
				// Check fallback was applied to empty FunctionID
				assert.Equal(t, tt.fallback, result[0].FunctionID)
			}
		})
	}
}

func TestResourceDiagnostics(t *testing.T) {
	tests := []struct {
		name      string
		contracts []*model.FunctionContract
		semantics *model.CapabilitySemantics
		wantLen   int
	}{
		{"nil semantics nil contracts", nil, nil, 0},
		{"with semantics", nil, &model.CapabilitySemantics{}, 0},
		{"with contracts", []*model.FunctionContract{
			{FunctionID: "player.getList"},
		}, nil, 0},
		{"with nil contract", []*model.FunctionContract{nil}, nil, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resourceDiagnostics(tt.contracts, tt.semantics)
			assert.Len(t, result, tt.wantLen)
		})
	}
}
