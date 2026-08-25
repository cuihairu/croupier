package service

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"gorm.io/datatypes"
)

func TestNormalizeLocalizedText(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]string
		expected map[string]string
	}{
		{"nil", nil, nil},
		{"empty", map[string]string{}, nil},
		{"all whitespace", map[string]string{"zh-CN": "  ", "en-US": "  "}, nil},
		{"zh-CN passthrough", map[string]string{"zh-CN": "玩家"}, map[string]string{"zh-CN": "玩家"}},
		{"en-US passthrough", map[string]string{"en-US": "player"}, map[string]string{"en-US": "player"}},
		{"zh alias", map[string]string{"zh": "玩家"}, map[string]string{"zh-CN": "玩家"}},
		{"zh-cn lower", map[string]string{"zh-cn": "玩家"}, map[string]string{"zh-CN": "玩家"}},
		{"zh_cn underscore", map[string]string{"zh_cn": "玩家"}, map[string]string{"zh-CN": "玩家"}},
		{"en alias", map[string]string{"en": "player"}, map[string]string{"en-US": "player"}},
		{"en-us lower", map[string]string{"en-us": "player"}, map[string]string{"en-US": "player"}},
		{"en_us underscore", map[string]string{"en_us": "player"}, map[string]string{"en-US": "player"}},
		{"custom locale", map[string]string{"ja": "プレイヤー"}, map[string]string{"ja": "プレイヤー"}},
		{"trim spaces", map[string]string{"zh-CN": "  玩家  "}, map[string]string{"zh-CN": "玩家"}},
		{"mixed", map[string]string{"zh-CN": "玩家", "en-US": "player", "ja": "プレイヤー"}, map[string]string{"zh-CN": "玩家", "en-US": "player", "ja": "プレイヤー"}},
		{"empty value removed", map[string]string{"zh-CN": ""}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeLocalizedText(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHasDefaultLocale(t *testing.T) {
	tests := []struct {
		name     string
		labels   map[string]string
		expected bool
	}{
		{"nil", nil, false},
		{"empty", map[string]string{}, false},
		{"no zh-CN", map[string]string{"en-US": "player"}, false},
		{"zh-CN empty", map[string]string{"zh-CN": ""}, false},
		{"zh-CN whitespace", map[string]string{"zh-CN": "  "}, false},
		{"zh-CN present", map[string]string{"zh-CN": "玩家"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, hasDefaultLocale(tt.labels))
		})
	}
}

func TestLocalizedTextEqual(t *testing.T) {
	tests := []struct {
		name     string
		left     map[string]string
		right    map[string]string
		expected bool
	}{
		{"both nil", nil, nil, true},
		{"both empty", map[string]string{}, map[string]string{}, true},
		{"equal", map[string]string{"zh-CN": "玩家"}, map[string]string{"zh-CN": "玩家"}, true},
		{"equal with aliases", map[string]string{"zh": "玩家"}, map[string]string{"zh-CN": "玩家"}, true},
		{"different values", map[string]string{"zh-CN": "玩家"}, map[string]string{"zh-CN": "角色"}, false},
		{"different keys", map[string]string{"zh-CN": "玩家"}, map[string]string{"en-US": "玩家"}, false},
		{"different lengths", map[string]string{"zh-CN": "玩家", "en-US": "player"}, map[string]string{"zh-CN": "玩家"}, false},
		{"whitespace equal", map[string]string{"zh-CN": " 玩家 "}, map[string]string{"zh-CN": "玩家"}, true},
		{"nil vs empty", nil, map[string]string{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, localizedTextEqual(tt.left, tt.right))
		})
	}
}

func TestBindingRequiresOutputSelectors(t *testing.T) {
	tests := []struct {
		name     string
		binding  spec.PageFunctionBinding
		pageType spec.PageType
		expected bool
	}{
		{"query on resource", spec.PageFunctionBinding{Usage: spec.BindingUsageQuery}, spec.PageTypeResource, true},
		{"query on non-resource", spec.PageFunctionBinding{Usage: spec.BindingUsageQuery}, spec.PageTypeOperation, false},
		{"report on report", spec.PageFunctionBinding{Usage: spec.BindingUsageReport}, spec.PageTypeReport, true},
		{"report on non-report", spec.PageFunctionBinding{Usage: spec.BindingUsageReport}, spec.PageTypeResource, false},
		{"task status on task", spec.PageFunctionBinding{Usage: spec.BindingUsageTaskStatus}, spec.PageTypeTask, true},
		{"task events on task", spec.PageFunctionBinding{Usage: spec.BindingUsageTaskEvents}, spec.PageTypeTask, true},
		{"task result on task", spec.PageFunctionBinding{Usage: spec.BindingUsageTaskResult}, spec.PageTypeTask, true},
		{"task status on non-task", spec.PageFunctionBinding{Usage: spec.BindingUsageTaskStatus}, spec.PageTypeResource, false},
		{"unknown usage", spec.PageFunctionBinding{Usage: "unknown"}, spec.PageTypeResource, false},
		{"empty usage", spec.PageFunctionBinding{}, spec.PageTypeResource, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page := spec.PageSpec{Type: tt.pageType}
			assert.Equal(t, tt.expected, bindingRequiresOutputSelectors(tt.binding, page))
		})
	}
}

func TestSchemaHasFields(t *testing.T) {
	tests := []struct {
		name     string
		raw      spec.JSONSchema
		expected bool
	}{
		{"nil", nil, false},
		{"empty", spec.JSONSchema{}, false},
		{"invalid json", spec.JSONSchema(`{invalid`), true},
		{"no properties no required", spec.JSONSchema(`{"type":"object"}`), false},
		{"with properties", spec.JSONSchema(`{"properties":{"name":{"type":"string"}}}`), true},
		{"with required", spec.JSONSchema(`{"required":["name"]}`), true},
		{"empty properties", spec.JSONSchema(`{"properties":{}}`), false},
		{"empty required", spec.JSONSchema(`{"required":[]}`), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, schemaHasFields(tt.raw))
		})
	}
}

func TestDigestJSON(t *testing.T) {
	tests := []struct {
		name  string
		raw   datatypes.JSON
		empty bool
	}{
		{"nil", nil, true},
		{"empty", datatypes.JSON{}, true},
		{"valid", datatypes.JSON(`{"key":"value"}`), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := digestJSON(tt.raw)
			if tt.empty {
				assert.Empty(t, result)
			} else {
				assert.Len(t, result, 64) // SHA-256 hex
			}
		})
	}
}

func TestProposalComparableDigest(t *testing.T) {
	tests := []struct {
		name     string
		proposal *model.PageProposal
		isEmpty  bool
	}{
		{"nil", nil, true},
		{"with key", &model.PageProposal{ProposalKey: "test"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := proposalComparableDigest(tt.proposal)
			if tt.isEmpty {
				assert.Empty(t, result)
			} else {
				assert.Len(t, result, 64)
			}
		})
	}
}

func TestPreserveGeneratedProposalStatus(t *testing.T) {
	tests := []struct {
		name     string
		status   dbenum.ProposalStatus
		expected dbenum.ProposalStatus
	}{
		{"accepted", dbenum.ProposalStatusAccepted, dbenum.ProposalStatusAccepted},
		{"rejected", dbenum.ProposalStatusRejected, dbenum.ProposalStatusRejected},
		{"pending", dbenum.ProposalStatusPending, dbenum.ProposalStatusPending},
		{"unknown", dbenum.ProposalStatus(42), dbenum.ProposalStatusPending},
		{"empty", dbenum.ProposalStatus(0), dbenum.ProposalStatusPending},
		{"generating", dbenum.ProposalStatus(7), dbenum.ProposalStatusPending},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, preserveGeneratedProposalStatus(tt.status))
		})
	}
}

func TestIsValidProposalPageType(t *testing.T) {
	tests := []struct {
		name     string
		pageType spec.PageType
		expected bool
	}{
		{"resource", spec.PageTypeResource, true},
		{"operation", spec.PageTypeOperation, true},
		{"task", spec.PageTypeTask, true},
		{"report", spec.PageTypeReport, true},
		{"unknown", "unknown", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isValidProposalPageType(tt.pageType))
		})
	}
}

func TestActorFromContext(t *testing.T) {
	// Without username in context, should return "system"
	result := actorFromContext(context.Background())
	assert.Equal(t, "system", result)
}

func TestPageShapeMatchesType(t *testing.T) {
	tests := []struct {
		name     string
		page     spec.PageSpec
		expected bool
	}{
		{"resource with resource", spec.PageSpec{Type: spec.PageTypeResource, Resource: &spec.ResourcePageSpec{}}, true},
		{"resource without resource", spec.PageSpec{Type: spec.PageTypeResource}, false},
		{"operation with operation", spec.PageSpec{Type: spec.PageTypeOperation, Operation: &spec.OperationPageSpec{}}, true},
		{"operation without operation", spec.PageSpec{Type: spec.PageTypeOperation}, false},
		{"task with task", spec.PageSpec{Type: spec.PageTypeTask, Task: &spec.TaskPageSpec{}}, true},
		{"task without task", spec.PageSpec{Type: spec.PageTypeTask}, false},
		{"report with report", spec.PageSpec{Type: spec.PageTypeReport, Report: &spec.ReportPageSpec{}}, true},
		{"report without report", spec.PageSpec{Type: spec.PageTypeReport}, false},
		{"unknown type", spec.PageSpec{Type: "unknown"}, false},
		{"empty type", spec.PageSpec{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, pageShapeMatchesType(tt.page))
		})
	}
}

func TestIsValidBindingUsage(t *testing.T) {
	tests := []struct {
		name     string
		usage    spec.PageBindingUsage
		expected bool
	}{
		{"query", spec.BindingUsageQuery, true},
		{"detail", spec.BindingUsageDetail, true},
		{"action", spec.BindingUsageAction, true},
		{"task", spec.BindingUsageTask, true},
		{"task status", spec.BindingUsageTaskStatus, true},
		{"task events", spec.BindingUsageTaskEvents, true},
		{"task result", spec.BindingUsageTaskResult, true},
		{"task cancel", spec.BindingUsageTaskCancel, true},
		{"task retry", spec.BindingUsageTaskRetry, true},
		{"report", spec.BindingUsageReport, true},
		{"unknown", "unknown", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isValidBindingUsage(tt.usage))
		})
	}
}

func TestIsValidExecutionMode(t *testing.T) {
	tests := []struct {
		name     string
		mode     spec.PageExecutionMode
		expected bool
	}{
		{"sync", spec.PageExecutionModeSync, true},
		{"task", spec.PageExecutionModeTask, true},
		{"unknown", "unknown", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isValidExecutionMode(tt.mode))
		})
	}
}

func TestExecutionModeForFunctionSpec(t *testing.T) {
	tests := []struct {
		name     string
		fn       spec.FunctionSpec
		expected spec.PageExecutionMode
	}{
		{"sync", spec.FunctionSpec{Execution: spec.FunctionExecutionSync}, spec.PageExecutionModeSync},
		{"task", spec.FunctionSpec{Execution: spec.FunctionExecutionTask}, spec.PageExecutionModeTask},
		{"empty", spec.FunctionSpec{}, spec.PageExecutionModeSync},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, executionModeForFunctionSpec(tt.fn))
		})
	}
}

func TestValidateAcceptedPageSpec(t *testing.T) {
	tests := []struct {
		name     string
		gameID   string
		env      string
		proposal *model.PageProposal
		page     spec.PageSpec
		wantErr  bool
	}{
		{"valid", "game1", "prod", &model.PageProposal{PageKey: "test"}, spec.PageSpec{
			PageKey:  "test",
			Type:     spec.PageTypeResource,
			Title:    spec.LocalizedText{"zh-CN": "玩家"},
			Category: spec.PageCategorySpec{Key: "player", Labels: spec.LocalizedText{"zh-CN": "玩家"}},
			Resource: &spec.ResourcePageSpec{},
			Bindings: []spec.PageFunctionBinding{{ID: "b1", FunctionID: "player.list", Usage: spec.BindingUsageQuery, Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync}}},
		}, false},
		{"missing gameID", "", "prod", &model.PageProposal{PageKey: "test"}, spec.PageSpec{
			PageKey:  "test",
			Type:     spec.PageTypeResource,
			Title:    spec.LocalizedText{"zh-CN": "玩家"},
			Category: spec.PageCategorySpec{Key: "player", Labels: spec.LocalizedText{"zh-CN": "玩家"}},
			Resource: &spec.ResourcePageSpec{},
			Bindings: []spec.PageFunctionBinding{{ID: "b1", FunctionID: "player.list", Usage: spec.BindingUsageQuery, Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync}}},
		}, true},
		{"missing env", "game1", "", &model.PageProposal{PageKey: "test"}, spec.PageSpec{
			PageKey:  "test",
			Type:     spec.PageTypeResource,
			Title:    spec.LocalizedText{"zh-CN": "玩家"},
			Category: spec.PageCategorySpec{Key: "player", Labels: spec.LocalizedText{"zh-CN": "玩家"}},
			Resource: &spec.ResourcePageSpec{},
			Bindings: []spec.PageFunctionBinding{{ID: "b1", FunctionID: "player.list", Usage: spec.BindingUsageQuery, Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync}}},
		}, true},
		{"missing pageKey", "game1", "prod", &model.PageProposal{PageKey: "test"}, spec.PageSpec{
			Type:  spec.PageTypeResource,
			Title: spec.LocalizedText{"zh-CN": "玩家"},
		}, true},
		{"pageKey mismatch", "game1", "prod", &model.PageProposal{PageKey: "other"}, spec.PageSpec{
			PageKey:  "test",
			Type:     spec.PageTypeResource,
			Title:    spec.LocalizedText{"zh-CN": "玩家"},
			Category: spec.PageCategorySpec{Key: "player", Labels: spec.LocalizedText{"zh-CN": "玩家"}},
			Resource: &spec.ResourcePageSpec{},
			Bindings: []spec.PageFunctionBinding{{ID: "b1", FunctionID: "player.list", Usage: spec.BindingUsageQuery, Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync}}},
		}, true},
		{"invalid type", "game1", "prod", &model.PageProposal{PageKey: "test"}, spec.PageSpec{
			PageKey:  "test",
			Type:     "unknown",
			Title:    spec.LocalizedText{"zh-CN": "玩家"},
			Category: spec.PageCategorySpec{Key: "player", Labels: spec.LocalizedText{"zh-CN": "玩家"}},
			Resource: &spec.ResourcePageSpec{},
			Bindings: []spec.PageFunctionBinding{{ID: "b1", FunctionID: "player.list", Usage: spec.BindingUsageQuery, Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync}}},
		}, true},
		{"missing title", "game1", "prod", &model.PageProposal{PageKey: "test"}, spec.PageSpec{
			PageKey:  "test",
			Type:     spec.PageTypeResource,
			Category: spec.PageCategorySpec{Key: "player", Labels: spec.LocalizedText{"zh-CN": "玩家"}},
			Resource: &spec.ResourcePageSpec{},
			Bindings: []spec.PageFunctionBinding{{ID: "b1", FunctionID: "player.list", Usage: spec.BindingUsageQuery, Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync}}},
		}, true},
		{"missing category key", "game1", "prod", &model.PageProposal{PageKey: "test"}, spec.PageSpec{
			PageKey:  "test",
			Type:     spec.PageTypeResource,
			Title:    spec.LocalizedText{"zh-CN": "玩家"},
			Category: spec.PageCategorySpec{Labels: spec.LocalizedText{"zh-CN": "玩家"}},
			Resource: &spec.ResourcePageSpec{},
			Bindings: []spec.PageFunctionBinding{{ID: "b1", FunctionID: "player.list", Usage: spec.BindingUsageQuery, Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync}}},
		}, true},
		{"missing category labels", "game1", "prod", &model.PageProposal{PageKey: "test"}, spec.PageSpec{
			PageKey:  "test",
			Type:     spec.PageTypeResource,
			Title:    spec.LocalizedText{"zh-CN": "玩家"},
			Category: spec.PageCategorySpec{Key: "player"},
			Resource: &spec.ResourcePageSpec{},
			Bindings: []spec.PageFunctionBinding{{ID: "b1", FunctionID: "player.list", Usage: spec.BindingUsageQuery, Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync}}},
		}, true},
		{"no bindings", "game1", "prod", &model.PageProposal{PageKey: "test"}, spec.PageSpec{
			PageKey:  "test",
			Type:     spec.PageTypeResource,
			Title:    spec.LocalizedText{"zh-CN": "玩家"},
			Category: spec.PageCategorySpec{Key: "player", Labels: spec.LocalizedText{"zh-CN": "玩家"}},
			Resource: &spec.ResourcePageSpec{},
		}, true},
		{"shape mismatch", "game1", "prod", &model.PageProposal{PageKey: "test"}, spec.PageSpec{
			PageKey:  "test",
			Type:     spec.PageTypeResource,
			Title:    spec.LocalizedText{"zh-CN": "玩家"},
			Category: spec.PageCategorySpec{Key: "player", Labels: spec.LocalizedText{"zh-CN": "玩家"}},
			Bindings: []spec.PageFunctionBinding{{ID: "b1", FunctionID: "player.list", Usage: spec.BindingUsageQuery, Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync}}},
		}, true},
		{"binding missing id", "game1", "prod", &model.PageProposal{PageKey: "test"}, spec.PageSpec{
			PageKey:  "test",
			Type:     spec.PageTypeResource,
			Title:    spec.LocalizedText{"zh-CN": "玩家"},
			Category: spec.PageCategorySpec{Key: "player", Labels: spec.LocalizedText{"zh-CN": "玩家"}},
			Resource: &spec.ResourcePageSpec{},
			Bindings: []spec.PageFunctionBinding{{FunctionID: "player.list", Usage: spec.BindingUsageQuery, Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync}}},
		}, true},
		{"binding missing functionId", "game1", "prod", &model.PageProposal{PageKey: "test"}, spec.PageSpec{
			PageKey:  "test",
			Type:     spec.PageTypeResource,
			Title:    spec.LocalizedText{"zh-CN": "玩家"},
			Category: spec.PageCategorySpec{Key: "player", Labels: spec.LocalizedText{"zh-CN": "玩家"}},
			Resource: &spec.ResourcePageSpec{},
			Bindings: []spec.PageFunctionBinding{{ID: "b1", Usage: spec.BindingUsageQuery, Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync}}},
		}, true},
		{"binding invalid usage", "game1", "prod", &model.PageProposal{PageKey: "test"}, spec.PageSpec{
			PageKey:  "test",
			Type:     spec.PageTypeResource,
			Title:    spec.LocalizedText{"zh-CN": "玩家"},
			Category: spec.PageCategorySpec{Key: "player", Labels: spec.LocalizedText{"zh-CN": "玩家"}},
			Resource: &spec.ResourcePageSpec{},
			Bindings: []spec.PageFunctionBinding{{ID: "b1", FunctionID: "player.list", Usage: "unknown", Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync}}},
		}, true},
		{"binding invalid execution mode", "game1", "prod", &model.PageProposal{PageKey: "test"}, spec.PageSpec{
			PageKey:  "test",
			Type:     spec.PageTypeResource,
			Title:    spec.LocalizedText{"zh-CN": "玩家"},
			Category: spec.PageCategorySpec{Key: "player", Labels: spec.LocalizedText{"zh-CN": "玩家"}},
			Resource: &spec.ResourcePageSpec{},
			Bindings: []spec.PageFunctionBinding{{ID: "b1", FunctionID: "player.list", Usage: spec.BindingUsageQuery, Execution: spec.PageBindingExecution{Mode: "unknown"}}},
		}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAcceptedPageSpec(tt.gameID, tt.env, tt.proposal, tt.page)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestStaleDiagnosticsForDraft(t *testing.T) {
	// Test empty bindings
	page := spec.PageSpec{Bindings: []spec.PageFunctionBinding{}}
	functions := map[string]spec.FunctionSpec{}
	result := staleDiagnosticsForDraft(page, functions)
	assert.Empty(t, result)

	// Test with missing function
	page2 := spec.PageSpec{
		Bindings: []spec.PageFunctionBinding{
			{ID: "b1", FunctionID: "player.list", Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync}},
		},
	}
	functions2 := map[string]spec.FunctionSpec{}
	result2 := staleDiagnosticsForDraft(page2, functions2)
	assert.Len(t, result2, 1)
	assert.Equal(t, spec.BindingFreshnessFunctionMissing, result2[0].Status)

	// Test with execution mode mismatch
	page3 := spec.PageSpec{
		Bindings: []spec.PageFunctionBinding{
			{ID: "b2", FunctionID: "player.ban", Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync}},
		},
	}
	functions3 := map[string]spec.FunctionSpec{
		"player.ban": {ID: "player.ban", Execution: spec.FunctionExecutionTask},
	}
	result3 := staleDiagnosticsForDraft(page3, functions3)
	assert.Len(t, result3, 1)
	assert.Equal(t, spec.BindingFreshnessExecutionModeStale, result3[0].Status)

	// Test with matching execution mode
	page4 := spec.PageSpec{
		Bindings: []spec.PageFunctionBinding{
			{ID: "b3", FunctionID: "player.get", Execution: spec.PageBindingExecution{Mode: spec.PageExecutionModeSync}},
		},
	}
	functions4 := map[string]spec.FunctionSpec{
		"player.get": {ID: "player.get", Execution: spec.FunctionExecutionSync},
	}
	result4 := staleDiagnosticsForDraft(page4, functions4)
	assert.Empty(t, result4)
}

func TestStringSliceFromJSON(t *testing.T) {
	// Test nil
	result := stringSliceFromJSON(nil)
	assert.Nil(t, result)

	// Test empty
	result2 := stringSliceFromJSON([]byte{})
	assert.Nil(t, result2)

	// Test valid JSON array
	result3 := stringSliceFromJSON([]byte(`["a","b","c"]`))
	assert.Equal(t, []string{"a", "b", "c"}, result3)

	// Test invalid JSON
	result4 := stringSliceFromJSON([]byte(`invalid`))
	assert.Nil(t, result4)
}

func TestNormalizeJSONMap(t *testing.T) {
	// Test nil
	result := normalizeJSONMap(nil)
	assert.Nil(t, result)

	// Test empty
	result2 := normalizeJSONMap(map[string]interface{}{})
	assert.Nil(t, result2)

	// Test with values
	values := map[string]interface{}{
		"zh-CN": "玩家",
		"en-US": "player",
	}
	result3 := normalizeJSONMap(values)
	assert.Equal(t, "玩家", result3["zh-CN"])
	assert.Equal(t, "player", result3["en-US"])

	// Test with non-string value
	values2 := map[string]interface{}{
		"zh-CN": 123,
	}
	result4 := normalizeJSONMap(values2)
	assert.Nil(t, result4)
}
