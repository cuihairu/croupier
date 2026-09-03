package generator

import (
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Small helpers
// ---------------------------------------------------------------------------

func TestExtraRawInt(t *testing.T) {
	assert.Equal(t, 0, rawInt(nil))
	assert.Equal(t, 42, rawInt(json.RawMessage(`42`)))
	assert.Equal(t, 0, rawInt(json.RawMessage(`"abc"`)))
	assert.Equal(t, 7, rawInt(json.RawMessage(`7`)))
}

func TestExtraFallbackLabel(t *testing.T) {
	// F：humanize 语义——分隔符与 camelCase 拆词、首字母大写
	cases := map[string]string{
		"":            "",
		"   ":         "",
		"player_id":   "Player Id",
		"create-task": "Create Task",
		"createTask":  "Create Task",
		"a.b.c":       "A B C",
		"already ok":  "Already Ok",
		"ID":          "ID",
		"HTTPServer":  "HTTP Server",
		"player.ban":  "Player Ban",
	}
	for input, want := range cases {
		assert.Equal(t, want, fallbackLabel(input), "fallbackLabel(%q)", input)
	}
}

func TestExtraCompactPointers(t *testing.T) {
	out := compactPointers([]string{" /a ", "", "b", "/a", "/c"})
	assert.Equal(t, []string{"/a", "/c"}, out)
}

func TestExtraSchemaAtPointerAndDatasetItemSchemaAtPointer(t *testing.T) {
	raw := `{
		"type":"object",
		"properties":{
			"report":{"type":"array","items":{"type":"object","properties":{"amount":{"type":"number"},"day":{"type":"string","format":"date"}}}}
		}
	}`
	schema := map[string]json.RawMessage{}
	require.NoError(t, json.Unmarshal([]byte(raw), &schema))

	node, ok := schemaAtPointer(schema, "")
	assert.True(t, ok)
	assert.Equal(t, "object", schemaTypeFromObject(node))

	node, ok = schemaAtPointer(schema, "/report")
	assert.True(t, ok)
	assert.Equal(t, "array", schemaTypeFromObject(node))

	_, ok = schemaAtPointer(schema, "/missing")
	assert.False(t, ok)

	item := datasetItemSchemaAtPointer(spec.JSONSchema(raw), "/report")
	require.Len(t, item, 2)
	assert.Nil(t, datasetItemSchemaAtPointer(spec.JSONSchema(raw), "/nope"))
	assert.Nil(t, datasetItemSchemaAtPointer(spec.JSONSchema(`{"type":"object"}`), ""))
}

func TestExtraCollectionOutputHelpers(t *testing.T) {
	listSchema := spec.JSONSchema(`{
		"type":"object",
		"properties":{
			"list":{"type":"array"},
			"total":{"type":"integer"}
		}
	}`)
	assert.Equal(t, "/list", collectionOutputSource(listSchema, []string{"items", "list"}))

	arraySchema := spec.JSONSchema(`{"type":"array"}`)
	assert.Equal(t, "", collectionOutputSource(arraySchema, []string{"items"}))

	assigns := collectionOutputAssignments(listSchema, []string{"items", "list"}, "rows")
	require.Len(t, assigns, 2)
	assert.Equal(t, "rows", assigns[0].StateKey)
	assert.Equal(t, "total", assigns[1].StateKey)

	assert.Empty(t, collectionOutputAssignments(spec.JSONSchema(`{"type":"object","properties":{}}`), []string{"items"}, "rows"))
}

// ---------------------------------------------------------------------------
// defaultOutputAssignments / applyReportSemantic / buildResultFields
// ---------------------------------------------------------------------------

func TestExtraDefaultOutputAssignments(t *testing.T) {
	objectSchema := spec.JSONSchema(`{"type":"object","properties":{}}`)
	arrayRoot := spec.JSONSchema(`{"type":"array"}`)
	reportSchema := spec.JSONSchema(`{
		"type":"object",
		"properties":{"dataset":{"type":"array"}}
	}`)
	bareObject := spec.JSONSchema(`{"type":"object"}`)

	assert.Nil(t, defaultOutputAssignments(spec.BindingUsageQuery, nil))

	querySchema := spec.JSONSchema(`{
		"type":"object",
		"properties":{"items":{"type":"array"},"total":{"type":"integer"}}
	}`)
	queryAssigns := defaultOutputAssignments(spec.BindingUsageQuery, querySchema)
	require.Len(t, queryAssigns, 2)
	assert.Equal(t, "/items", queryAssigns[0].Source)
	assert.Equal(t, "total", queryAssigns[1].StateKey)

	queryArray := defaultOutputAssignments(spec.BindingUsageQuery, arrayRoot)
	require.Len(t, queryArray, 1)
	assert.Empty(t, queryArray[0].Source)

	reportAssigns := defaultOutputAssignments(spec.BindingUsageReport, reportSchema)
	require.Len(t, reportAssigns, 1)
	assert.Equal(t, spec.OutputShapeDataset, reportAssigns[0].Shape)

	assert.Len(t, defaultOutputAssignments(spec.BindingUsageReport, arrayRoot), 1)
	assert.Nil(t, defaultOutputAssignments(spec.BindingUsageReport, bareObject))

	actionAssigns := defaultOutputAssignments(spec.BindingUsageAction, objectSchema)
	require.Len(t, actionAssigns, 1)
	assert.Equal(t, spec.OutputShapeObject, actionAssigns[0].Shape)

	detailScalar := defaultOutputAssignments(spec.BindingUsageDetail, spec.JSONSchema(`{"type":"string"}`))
	require.Len(t, detailScalar, 1)
	assert.Equal(t, spec.OutputShapeScalar, detailScalar[0].Shape)

	assert.Nil(t, defaultOutputAssignments(spec.BindingUsageTaskStatus, objectSchema))
}

func TestExtraApplyReportSemantic(t *testing.T) {
	var nilBinding *spec.PageFunctionBinding
	applyReportSemantic(nilBinding, spec.ReportSemantic{Query: spec.FunctionRef{FunctionID: "q"}})

	binding := &spec.PageFunctionBinding{}
	applyReportSemantic(binding, spec.ReportSemantic{}) // empty query → untouched
	assert.Nil(t, binding.Selectors)

	applyReportSemantic(binding, spec.ReportSemantic{
		Query:       spec.FunctionRef{FunctionID: "r.query"},
		DatasetPath: "/data",
	})
	require.NotNil(t, binding.Selectors)
	require.Len(t, binding.Selectors.Output, 1)
	assert.Equal(t, "dataset", binding.Selectors.Output[0].StateKey)
	assert.Equal(t, "/data", binding.Selectors.Output[0].Source)
}

func TestExtraBuildResultFields(t *testing.T) {
	assert.Nil(t, buildResultFields(nil, "zh-CN"))
	scalar := buildResultFields(spec.JSONSchema(`{"type":"integer"}`), "zh-CN")
	require.Len(t, scalar, 1)
	assert.Equal(t, "result", scalar[0].Key)

	emptyProps := buildResultFields(spec.JSONSchema(`{"type":"object"}`), "zh-CN")
	assert.Nil(t, emptyProps)

	fields := buildResultFields(spec.JSONSchema(`{
		"type":"object",
		"properties":{"name":{"type":"string"},"count":{"type":"integer"}}
	}`), "zh-CN")
	require.Len(t, fields, 2)
	assert.Equal(t, "count", fields[0].Key)
	assert.Equal(t, "number", fields[0].DataType)
}

// ---------------------------------------------------------------------------
// Dataset from semantic + pointer builders
// ---------------------------------------------------------------------------

func TestExtraBuildDatasetSpecFromSemantic(t *testing.T) {
	outputSchema := spec.JSONSchema(`{
		"type":"object",
		"properties":{
			"data":{"type":"array","items":{
				"type":"object",
				"properties":{
					"day":{"type":"string","format":"date"},
					"income":{"type":"number"},
					"active":{"type":"boolean"}
				}
			}}
		}
	}`)

	t.Run("empty query function returns nil", func(t *testing.T) {
		assert.Nil(t, buildDatasetSpecFromSemantic(outputSchema, spec.ReportSemantic{}, "zh-CN"))
	})

	t.Run("missing dataset pointer returns nil", func(t *testing.T) {
		assert.Nil(t, buildDatasetSpecFromSemantic(outputSchema, spec.ReportSemantic{
			Query:       spec.FunctionRef{FunctionID: "r.query"},
			DatasetPath: "",
		}, "zh-CN"))
	})

	t.Run("non-array target returns nil", func(t *testing.T) {
		assert.Nil(t, buildDatasetSpecFromSemantic(outputSchema, spec.ReportSemantic{
			Query:       spec.FunctionRef{FunctionID: "r.query"},
			DatasetPath: "/unknown",
		}, "zh-CN"))
	})

	t.Run("incomplete dims or metrics returns nil", func(t *testing.T) {
		assert.Nil(t, buildDatasetSpecFromSemantic(outputSchema, spec.ReportSemantic{
			Query:       spec.FunctionRef{FunctionID: "r.query"},
			DatasetPath: "/data",
			Dimensions:  []string{"/day"},
			Metrics:     []string{"/missing"},
		}, "zh-CN"))
	})

	t.Run("full semantic builds dimensions and metrics", func(t *testing.T) {
		ds := buildDatasetSpecFromSemantic(outputSchema, spec.ReportSemantic{
			Query:       spec.FunctionRef{FunctionID: "r.query"},
			DatasetPath: "/data",
			Dimensions:  []string{" /day ", "", "dup", "/active", "/bogus"},
			Metrics:     []string{"/income", "/income", "/day"},
		}, "zh-CN")
		require.NotNil(t, ds)
		require.Len(t, ds.Dimensions, 2)
		assert.Equal(t, "day", ds.Dimensions[0].Key)
		assert.Equal(t, "date", ds.Dimensions[0].DataType)
		assert.Equal(t, "active", ds.Dimensions[1].Key)
		assert.Equal(t, "string", ds.Dimensions[1].DataType)
		require.Len(t, ds.Metrics, 1)
		assert.Equal(t, "income", ds.Metrics[0].Key)
		assert.Equal(t, "sum", ds.Metrics[0].AggType)
	})
}

func TestExtraBuildMetricsFromPointersSkipsNonNumeric(t *testing.T) {
	item := map[string]json.RawMessage{}
	require.NoError(t, json.Unmarshal([]byte(`{"properties":{"n":{"type":"integer"},"s":{"type":"string"}}}`), &item))

	metrics := buildMetricsFromPointers(item, []string{"/n", "/s", "/ghost"}, "zh-CN")
	require.Len(t, metrics, 1)
	assert.Equal(t, "n", metrics[0].Key)
}

// ---------------------------------------------------------------------------
// Quality helpers
// ---------------------------------------------------------------------------

func TestExtraQualityHelpers(t *testing.T) {
	op := spec.OperationSpec{FunctionID: "fn"}

	assert.Equal(t, spec.GeneratedPageQualityReady, qualityFromDiagnostics(nil))
	assert.Equal(t, spec.GeneratedPageQualityNeedsReview, qualityFromDiagnostics(
		[]spec.Diagnostic{{Severity: spec.SeverityWarning}}))
	assert.Equal(t, spec.GeneratedPageQualityNeedsReview, qualityFromDiagnostics(
		[]spec.Diagnostic{{Severity: spec.SeverityError}}))

	assert.Equal(t, spec.GeneratedPageQualityBasic, operationQuality(op, nil))
	assert.Equal(t, spec.GeneratedPageQualityNeedsReview,
		operationQuality(spec.OperationSpec{}, nil))
	assert.Equal(t, spec.GeneratedPageQualityNeedsReview, operationQuality(op,
		[]spec.Diagnostic{{Code: "json_schema_generation_subset_unsupported", Severity: spec.SeverityWarning}}))

	assert.Equal(t, spec.GeneratedPageQualityBasic, taskQuality(op, nil))
	assert.Equal(t, spec.GeneratedPageQualityNeedsReview, taskQuality(op,
		[]spec.Diagnostic{{Severity: spec.SeverityWarning}}))
	assert.Equal(t, spec.GeneratedPageQualityReady,
		taskQuality(spec.OperationSpec{}, []spec.Diagnostic{}))
}

// ---------------------------------------------------------------------------
// buildFormFields
// ---------------------------------------------------------------------------

func TestExtraBuildFormFields(t *testing.T) {
	assert.Nil(t, buildFormFields(nil, "zh-CN"))
	assert.Nil(t, buildFormFields(spec.JSONSchema(`{"type":"string"}`), "zh-CN"))
	assert.Nil(t, buildFormFields(spec.JSONSchema(`{"type":"object"}`), "zh-CN"))

	fields := buildFormFields(spec.JSONSchema(`{
		"type":"object",
		"required":["nickname"],
		"properties":{
			"nickname":{"type":"string","title":"昵称"},
			"bio":{"type":"string","format":"textarea"},
			"signature":{"type":"string","maxLength":200},
			"level":{"type":"integer"},
			"muted":{"type":"boolean"},
			"region":{"type":"string","enum":["cn","us"]}
		}
	}`), "zh-CN")

	byKey := map[string]spec.FormFieldSpec{}
	for _, f := range fields {
		byKey[f.Key] = f
	}
	assert.NotContains(t, byKey, "nickname", "titled fields keep their schema title and are skipped")
	assert.Equal(t, spec.FormWidgetTextArea, byKey["bio"].Widget)
	assert.Equal(t, spec.FormWidgetTextArea, byKey["signature"].Widget)
	assert.Equal(t, spec.FormWidgetNumber, byKey["level"].Widget)
	assert.Equal(t, spec.FormWidgetSwitch, byKey["muted"].Widget)
	assert.NotEmpty(t, byKey["region"].Placeholder)
	assert.NotEmpty(t, byKey["level"].Label["zh-CN"])
}
