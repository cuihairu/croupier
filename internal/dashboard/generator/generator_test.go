package generator

import (
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateForResourceCreatesEntityPageOnlyFromExplicitTableContract(t *testing.T) {
	resource := spec.ResourceSpec{
		Key:    "player",
		Labels: spec.LocalizedText{"zh-CN": "玩家"},
		Operations: []spec.OperationSpec{
			{
				FunctionID:  "player.list",
				ResourceKey: "player",
				Operation:   "list",
				Enabled:     true,
				PageContract: &spec.PageContract{
					Version:       "page-contract:1",
					InputMapping:  raw(`{"page":"values.page","pageSize":"values.pageSize"}`),
					OutputMapping: raw(`{"stateKey":"players"}`),
					Pagination: &spec.PagePaginationContract{
						PageField:     "page",
						PageSizeField: "pageSize",
						ItemsPath:     "items",
						TotalPath:     "total",
					},
					Table: &spec.PageTableContract{
						Columns: []spec.PageTableColumnContract{
							{Key: "id", Title: spec.LocalizedText{"zh-CN": "玩家ID"}, ValuePath: "id"},
							{Key: "name", Title: spec.LocalizedText{"zh-CN": "昵称"}, ValuePath: "profile.name"},
						},
					},
				},
			},
			{
				FunctionID:  "player.ban",
				ResourceKey: "player",
				Operation:   "ban",
				Risk:        spec.RiskDanger,
				Enabled:     true,
				PageContract: &spec.PageContract{
					Version:       "page-contract:1",
					InputMapping:  raw(`{"targetId":"row.id","note":"values.note"}`),
					OutputMapping: raw(`{"stateKey":"banResult"}`),
				},
			},
		},
	}

	pages := GenerateForResource(resource, GenerateOptions{
		DefaultLocale: "zh-CN",
		Functions: map[string]spec.FunctionSpec{
			"player.list": {
				ID:                 "player.list",
				InputFormilySchema: spec.FormilySchema(`{"type":"object","properties":{"keyword":{"type":"string","x-component":"Input"}}}`),
			},
		},
	})

	require.Len(t, pages, 1)
	page := pages[0]
	assert.Equal(t, "player.manage", page.PageKey)
	assert.Equal(t, spec.PageTypeEntity, page.Type)
	assert.Equal(t, "ready", page.Quality)
	assert.Equal(t, "玩家", page.Title["zh-CN"])
	assert.Len(t, page.Bindings, 2)
	assertBinding(t, page.Bindings, "player.query", spec.BindingUsageQuery)
	assertBinding(t, page.Bindings, "player.ban", spec.BindingUsageAction)
	assert.Contains(t, string(page.Schema), `"x-component":"DataTable"`)
	assert.Contains(t, string(page.Schema), `"bindingId":"player.query"`)
	assert.Contains(t, string(page.Schema), `"rowActions"`)
	assert.Contains(t, string(page.Schema), `"targetId":"row.id"`)
	assert.Contains(t, string(page.Schema), `"note":"values.note"`)
	assert.NotContains(t, string(page.Schema), `"functionId"`)
}

func TestGenerateForResourceDoesNotAddEntityActionWithoutOutputMapping(t *testing.T) {
	resource := spec.ResourceSpec{
		Key:    "player",
		Labels: spec.LocalizedText{"zh-CN": "玩家"},
		Operations: []spec.OperationSpec{
			{
				FunctionID:   "player.list",
				ResourceKey:  "player",
				Operation:    "list",
				Enabled:      true,
				PageContract: tablePageContract(),
			},
			{
				FunctionID:  "player.ban",
				ResourceKey: "player",
				Operation:   "ban",
				Risk:        spec.RiskDanger,
				Enabled:     true,
				PageContract: &spec.PageContract{
					Version:      "page-contract:1",
					InputMapping: raw(`{"targetId":"row.id"}`),
				},
			},
		},
	}

	pages := GenerateForResource(resource, DefaultGenerateOptions())

	require.Len(t, pages, 2)
	entityPage := pages[0]
	assert.Equal(t, "needs_review", entityPage.Quality)
	assertDiagnostic(t, entityPage.Diagnostics, "entity_action_output_mapping_missing")
	assertBinding(t, entityPage.Bindings, "player.query", spec.BindingUsageQuery)
	assert.NotContains(t, string(entityPage.Schema), `"bindingId":"player.ban"`)

	actionPage := pages[1]
	assert.Equal(t, "needs_review", actionPage.Quality)
	assert.Equal(t, "player.ban", actionPage.PageKey)
	assertDiagnostic(t, actionPage.Diagnostics, "binding_output_mapping_missing")
}

func TestGenerateForResourceKeepsStandaloneOperationOutsideEntityPage(t *testing.T) {
	resource := spec.ResourceSpec{
		Key: "mail",
		Operations: []spec.OperationSpec{
			{
				FunctionID:  "mail.send",
				ResourceKey: "mail",
				Operation:   "send",
				Enabled:     true,
				PageContract: &spec.PageContract{
					Version:       "page-contract:1",
					InputMapping:  raw(`{"target":"values.target","content":"values.content"}`),
					OutputMapping: raw(`{"stateKey":"mailSendResult"}`),
				},
			},
		},
	}

	pages := GenerateForResource(resource, DefaultGenerateOptions())

	require.Len(t, pages, 1)
	page := pages[0]
	assert.Equal(t, "mail.send", page.PageKey)
	assert.Equal(t, spec.PageTypeOperation, page.Type)
	assert.Equal(t, "ready", page.Quality)
	assert.Contains(t, string(page.Schema), `"x-component":"QueryForm"`)
	assert.Contains(t, string(page.Schema), `"x-component":"ResultPanel"`)
	assert.NotContains(t, string(page.Schema), `"x-component":"DataTable"`)
}

func TestGenerateForOperationCreatesTaskPageFromTaskContract(t *testing.T) {
	page := GenerateForOperation(spec.OperationSpec{
		FunctionID:  "reward.batchGrant",
		ResourceKey: "reward",
		Operation:   "batchGrant",
		Enabled:     true,
		PageContract: &spec.PageContract{
			Version:       "page-contract:1",
			ExecutionMode: spec.PageExecutionModeTask,
			InputMapping:  raw(`{"segment":"values.segment","rewardId":"values.rewardId"}`),
			OutputMapping: raw(`{"stateKey":"rewardTask"}`),
			Task: &spec.PageTaskContract{
				TaskIDPath: "taskId",
				StatusPath: "status",
				EventsPath: "events",
				ResultPath: "result",
			},
		},
	}, DefaultGenerateOptions())

	assert.Equal(t, "reward.batchGrant", page.PageKey)
	assert.Equal(t, spec.PageTypeTask, page.Type)
	assert.Equal(t, "ready", page.Quality)
	require.Len(t, page.Bindings, 1)
	assert.Equal(t, spec.BindingUsageTask, page.Bindings[0].Usage)
	assert.Equal(t, spec.PageExecutionModeTask, page.Bindings[0].Execution.Mode)
	assert.Contains(t, string(page.Schema), `"x-component":"TaskTimeline"`)
	assert.Contains(t, string(page.Schema), `"x-component":"ResultPanel"`)
}

func TestGenerateForOperationCreatesReportPageFromReportContract(t *testing.T) {
	page := GenerateForOperation(spec.OperationSpec{
		FunctionID:  "analytics.retention",
		ResourceKey: "analytics",
		Operation:   "retention",
		Enabled:     true,
		PageContract: &spec.PageContract{
			Version:       "page-contract:1",
			InputMapping:  raw(`{"startDate":"values.startDate","endDate":"values.endDate"}`),
			OutputMapping: raw(`{"stateKey":"retentionReport"}`),
			Report: &spec.PageReportContract{
				ChartType:    "line",
				CategoryPath: "cohorts.date",
				SeriesPath:   "cohorts.series",
				ValuePath:    "rate",
			},
		},
	}, DefaultGenerateOptions())

	assert.Equal(t, "analytics.retention", page.PageKey)
	assert.Equal(t, spec.PageTypeReport, page.Type)
	assert.Equal(t, "ready", page.Quality)
	require.Len(t, page.Bindings, 1)
	assert.Equal(t, spec.BindingUsageReport, page.Bindings[0].Usage)
	assert.Contains(t, string(page.Schema), `"x-component":"ChartPanel"`)
	assert.Contains(t, string(page.Schema), `"chartType":"line"`)
	assert.NotContains(t, string(page.Schema), `"x-component":"DataTable"`)
}

func TestGenerateForOperationMarksMissingContractNeedsReview(t *testing.T) {
	page := GenerateForOperation(spec.OperationSpec{
		FunctionID: "cache.refresh",
		Operation:  "refresh",
		Enabled:    true,
	}, DefaultGenerateOptions())

	assert.Equal(t, spec.PageTypeOperation, page.Type)
	assert.Equal(t, "needs_review", page.Quality)
	assertDiagnostic(t, page.Diagnostics, "resource_missing")
	assertDiagnostic(t, page.Diagnostics, "page_contract_missing")
	assert.Contains(t, string(page.Schema), `"x-component":"QueryForm"`)
	assert.NotContains(t, string(page.Schema), `"x-component":"DataTable"`)
}

func TestInferPageTypeUsesOnlyContractShape(t *testing.T) {
	assert.Equal(t, spec.PageTypeOperation, InferPageType([]spec.OperationSpec{{FunctionID: "player.list", Operation: "list"}}))
	assert.Equal(t, spec.PageTypeEntity, InferPageType([]spec.OperationSpec{{PageContract: tablePageContract()}}))
	assert.Equal(t, spec.PageTypeTask, InferPageType([]spec.OperationSpec{{PageContract: &spec.PageContract{Version: "page-contract:1", ExecutionMode: spec.PageExecutionModeTask}}}))
	assert.Equal(t, spec.PageTypeReport, InferPageType([]spec.OperationSpec{{PageContract: &spec.PageContract{Version: "page-contract:1", Report: &spec.PageReportContract{ChartType: "line"}}}}))
}

func tablePageContract() *spec.PageContract {
	return &spec.PageContract{
		Version:       "page-contract:1",
		InputMapping:  raw(`{"page":"values.page","pageSize":"values.pageSize"}`),
		OutputMapping: raw(`{"stateKey":"rows"}`),
		Pagination: &spec.PagePaginationContract{
			PageField:     "page",
			PageSizeField: "pageSize",
			ItemsPath:     "items",
			TotalPath:     "total",
		},
		Table: &spec.PageTableContract{
			Columns: []spec.PageTableColumnContract{{Key: "id", Title: spec.LocalizedText{"zh-CN": "ID"}, ValuePath: "id"}},
		},
	}
}

func raw(value string) json.RawMessage {
	return json.RawMessage(value)
}

func assertBinding(t *testing.T, bindings []spec.PageFunctionBinding, id string, usage spec.PageBindingUsage) {
	t.Helper()
	for _, binding := range bindings {
		if binding.ID == id {
			assert.Equal(t, usage, binding.Usage)
			return
		}
	}
	t.Fatalf("binding %s not found in %#v", id, bindings)
}

func assertDiagnostic(t *testing.T, diagnostics []spec.Diagnostic, code string) {
	t.Helper()
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return
		}
	}
	t.Fatalf("diagnostic %s not found in %#v", code, diagnostics)
}
