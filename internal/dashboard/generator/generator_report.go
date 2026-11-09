package generator

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

// GenerateReportPageForOperation creates a report candidate. Without dataset,
// dimension, metric, and chart semantics it must be reviewed before publication.
func GenerateReportPageForOperation(op spec.OperationSpec, opts GenerateOptions) spec.GeneratedPageSpec {
	opts = normalizeOptions(opts)
	resourceKey := strings.TrimSpace(op.ResourceKey)
	pageKey := operationPageKey(op, opts)
	binding := pageBinding(op, "report", spec.BindingUsageReport, spec.PageExecutionModeSync)
	fn := opts.Functions[op.FunctionID]
	applySelectors(&binding, fn)
	diags := assessBaseCandidate(op)
	diags = append(diags, schemaSubsetDiagnostics(op.FunctionID, "inputSchema", fn.InputSchema)...)
	diags = append(diags, schemaSubsetDiagnostics(op.FunctionID, "outputSchema", fn.OutputSchema)...)
	locale := opts.DefaultLocale
	title := localizedTitle(op, pageKey, locale, opts)
	reportSemantic, hasReportSemantic := opts.ReportSemantics[strings.TrimSpace(op.FunctionID)]
	validSemantic := hasReportSemantic && reportSemanticComplete(op, reportSemantic)
	var dataset *spec.DatasetSpec
	if validSemantic {
		dataset = buildDatasetSpecFromSemantic(fn.OutputSchema, reportSemantic, locale)
		applyReportSemantic(&binding, reportSemantic)
	}
	charts := buildChartSpecs(dataset, title, locale)
	if !validSemantic || dataset == nil || len(dataset.Dimensions) == 0 || len(dataset.Metrics) == 0 {
		diags = append(diags, diagnostic(
			"report_dataset_missing",
			spec.SeverityWarning,
			"report capability requires dataset, dimension, and metric semantics before it can be safely published",
			op.FunctionID,
			"capability",
		))
		dataset = &spec.DatasetSpec{
			Dimensions: []spec.DimensionSpec{},
			Metrics:    []spec.MetricSpec{},
		}
		charts = nil
	}

	return spec.GeneratedPageSpec{
		PageSpec: spec.PageSpec{
			PageKey:     pageKey,
			Type:        spec.PageTypeReport,
			ResourceKey: resourceKey,
			Title:       title,
			Category:    categoryForOperation(op.FunctionID),
			Navigation: &spec.NavigationSpec{
				Title: title,
			},
			Report: &spec.ReportPageSpec{
				QueryForm:  buildFormPresentation(op, opts),
				Dataset:    dataset,
				Charts:     charts,
				Exportable: true,
			},
			Bindings: []spec.PageFunctionBinding{binding},
		},
		Quality:     qualityFromDiagnostics(diags),
		Diagnostics: diags,
	}
}

func reportSemanticComplete(op spec.OperationSpec, semantic spec.ReportSemantic) bool {
	return strings.TrimSpace(semantic.Query.FunctionID) == strings.TrimSpace(op.FunctionID) &&
		strings.TrimSpace(semantic.DatasetPath) != "" &&
		len(compactPointers(semantic.Dimensions)) > 0 &&
		len(compactPointers(semantic.Metrics)) > 0
}

func applyReportSemantic(binding *spec.PageFunctionBinding, reportSemantic spec.ReportSemantic) {
	if binding == nil || strings.TrimSpace(reportSemantic.Query.FunctionID) == "" {
		return
	}
	if binding.Selectors == nil {
		binding.Selectors = &spec.BindingSelectors{}
	}
	binding.Selectors.Output = []spec.OutputAssignment{{
		StateKey: "dataset",
		Source:   strings.TrimSpace(reportSemantic.DatasetPath),
		Shape:    spec.OutputShapeDataset,
	}}
}

func buildResultFields(outputSchema spec.JSONSchema, locale string) []spec.ResultFieldSpec {
	if len(outputSchema) == 0 {
		return nil
	}
	root := parseJSONObject(jsonRaw(outputSchema))
	if schemaTypeFromObject(root) != "object" {
		return []spec.ResultFieldSpec{{
			Key:      "result",
			Title:    spec.LocalizedText{locale: "结果"},
			DataType: dataTypeFromSchema(root),
		}}
	}
	properties := objectProperty(root, "properties")
	if len(properties) == 0 {
		return nil
	}
	keys := sortedRawMapKeys(properties)
	fields := make([]spec.ResultFieldSpec, 0, len(keys))
	for _, key := range keys {
		fields = append(fields, spec.ResultFieldSpec{
			Key:      key,
			Title:    spec.LocalizedText{locale: fallbackLabel(key)},
			DataType: dataTypeFromSchema(parseJSONObject(properties[key])),
		})
	}
	return fields
}

func buildDatasetSpec(outputSchema spec.JSONSchema, locale string) *spec.DatasetSpec {
	itemSchema := datasetItemSchema(outputSchema)
	if len(itemSchema) == 0 {
		return nil
	}
	properties := objectProperty(itemSchema, "properties")
	if len(properties) == 0 {
		return nil
	}
	keys := sortedRawMapKeys(properties)
	dataset := &spec.DatasetSpec{
		Dimensions: []spec.DimensionSpec{},
		Metrics:    []spec.MetricSpec{},
	}
	for _, key := range keys {
		prop := parseJSONObject(properties[key])
		switch schemaTypeFromObject(prop) {
		case "integer", "number":
			dataset.Metrics = append(dataset.Metrics, spec.MetricSpec{
				Key:      key,
				Title:    spec.LocalizedText{locale: fallbackLabel(key)},
				DataType: "number",
				AggType:  "sum",
			})
		case "string":
			dimensionType := "string"
			if rawString(prop["format"]) == "date" || rawString(prop["format"]) == "date-time" {
				dimensionType = "date"
			}
			dataset.Dimensions = append(dataset.Dimensions, spec.DimensionSpec{
				Key:      key,
				Title:    spec.LocalizedText{locale: fallbackLabel(key)},
				DataType: dimensionType,
			})
		case "boolean":
			dataset.Dimensions = append(dataset.Dimensions, spec.DimensionSpec{
				Key:      key,
				Title:    spec.LocalizedText{locale: fallbackLabel(key)},
				DataType: "string",
			})
		}
	}
	if len(dataset.Dimensions) == 0 || len(dataset.Metrics) == 0 {
		return nil
	}
	return dataset
}

func buildDatasetSpecFromSemantic(outputSchema spec.JSONSchema, report spec.ReportSemantic, locale string) *spec.DatasetSpec {
	if strings.TrimSpace(report.Query.FunctionID) == "" || len(outputSchema) == 0 {
		return nil
	}
	itemSchema := datasetItemSchemaAtPointer(outputSchema, strings.TrimSpace(report.DatasetPath))
	if len(itemSchema) == 0 {
		return nil
	}
	dimensions := buildDimensionsFromPointers(itemSchema, report.Dimensions, locale)
	metrics := buildMetricsFromPointers(itemSchema, report.Metrics, locale)
	if len(dimensions) == 0 || len(metrics) == 0 {
		return nil
	}
	return &spec.DatasetSpec{
		Dimensions: dimensions,
		Metrics:    metrics,
	}
}

func buildDimensionsFromPointers(itemSchema map[string]json.RawMessage, pointers []string, locale string) []spec.DimensionSpec {
	out := make([]spec.DimensionSpec, 0, len(pointers))
	for _, pointer := range compactPointers(pointers) {
		prop, ok := schemaAtPointer(itemSchema, pointer)
		if !ok {
			continue
		}
		key := keyFromPointer(pointer)
		dataType := dataTypeFromSchema(prop)
		if dataType == "datetime" {
			dataType = "date"
		}
		if dataType != "string" && dataType != "number" && dataType != "date" {
			dataType = "string"
		}
		out = append(out, spec.DimensionSpec{
			Key:      key,
			Title:    spec.LocalizedText{locale: fallbackLabel(key)},
			DataType: dataType,
		})
	}
	return out
}

func buildMetricsFromPointers(itemSchema map[string]json.RawMessage, pointers []string, locale string) []spec.MetricSpec {
	out := make([]spec.MetricSpec, 0, len(pointers))
	for _, pointer := range compactPointers(pointers) {
		prop, ok := schemaAtPointer(itemSchema, pointer)
		if !ok {
			continue
		}
		if schemaTypeFromObject(prop) != "number" && schemaTypeFromObject(prop) != "integer" {
			continue
		}
		key := keyFromPointer(pointer)
		out = append(out, spec.MetricSpec{
			Key:      key,
			Title:    spec.LocalizedText{locale: fallbackLabel(key)},
			DataType: "number",
			AggType:  "sum",
		})
	}
	return out
}

func datasetItemSchemaAtPointer(outputSchema spec.JSONSchema, pointer string) map[string]json.RawMessage {
	root := parseJSONObject(jsonRaw(outputSchema))
	node := root
	if pointer != "" {
		for _, token := range pointerTokens(pointer) {
			properties := objectProperty(node, "properties")
			if len(properties) == 0 {
				return nil
			}
			node = parseJSONObject(properties[token])
			if len(node) == 0 {
				return nil
			}
		}
	}
	if schemaTypeFromObject(node) != "array" {
		return nil
	}
	return objectProperty(node, "items")
}

func schemaAtPointer(root map[string]json.RawMessage, pointer string) (map[string]json.RawMessage, bool) {
	if pointer == "" {
		return root, len(root) > 0
	}
	node := root
	for _, token := range pointerTokens(pointer) {
		properties := objectProperty(node, "properties")
		if len(properties) == 0 {
			return nil, false
		}
		node = parseJSONObject(properties[token])
		if len(node) == 0 {
			return nil, false
		}
	}
	return node, true
}

func compactPointers(pointers []string) []string {
	out := make([]string, 0, len(pointers))
	seen := map[string]struct{}{}
	for _, pointer := range pointers {
		pointer = strings.TrimSpace(pointer)
		if pointer == "" || !strings.HasPrefix(pointer, "/") {
			continue
		}
		if _, ok := seen[pointer]; ok {
			continue
		}
		seen[pointer] = struct{}{}
		out = append(out, pointer)
	}
	return out
}

func pointerTokens(pointer string) []string {
	if pointer == "" {
		return nil
	}
	parts := strings.Split(strings.TrimPrefix(pointer, "/"), "/")
	for i, part := range parts {
		parts[i] = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
	}
	return parts
}

func keyFromPointer(pointer string) string {
	tokens := pointerTokens(pointer)
	if len(tokens) == 0 {
		return "value"
	}
	return tokens[len(tokens)-1]
}

func buildChartSpecs(dataset *spec.DatasetSpec, title spec.LocalizedText, locale string) []spec.ChartSpec {
	if dataset == nil || len(dataset.Dimensions) == 0 || len(dataset.Metrics) == 0 {
		return nil
	}
	return []spec.ChartSpec{{
		Type:   "line",
		Title:  title,
		XField: dataset.Dimensions[0].Key,
		YField: dataset.Metrics[0].Key,
	}}
}

func datasetItemSchema(outputSchema spec.JSONSchema) map[string]json.RawMessage {
	if len(outputSchema) == 0 {
		return nil
	}
	root := parseJSONObject(jsonRaw(outputSchema))
	if schemaTypeFromObject(root) == "array" {
		return objectProperty(root, "items")
	}
	properties := objectProperty(root, "properties")
	for _, key := range []string{"dataset", "items", "rows", "data"} {
		prop := objectProperty(properties, key)
		if schemaTypeFromObject(prop) != "array" {
			continue
		}
		if items := objectProperty(prop, "items"); len(items) > 0 {
			return items
		}
	}
	return nil
}

func collectionOutputAssignments(outputSchema spec.JSONSchema, arrayKeys []string, stateKey string) []spec.OutputAssignment {
	source := collectionOutputSource(outputSchema, arrayKeys)
	if source == "" && schemaRootType(outputSchema) != "array" {
		return nil
	}
	assignments := []spec.OutputAssignment{{
		StateKey: stateKey,
		Source:   source,
		Shape:    spec.OutputShapeCollection,
	}}
	if totalSource := scalarPropertySource(outputSchema, []string{"total", "count", "total_count"}); totalSource != "" {
		assignments = append(assignments, spec.OutputAssignment{
			StateKey: "total",
			Source:   totalSource,
			Shape:    spec.OutputShapeScalar,
		})
	}
	return assignments
}

func collectionOutputSource(outputSchema spec.JSONSchema, keys []string) string {
	if schemaRootType(outputSchema) == "array" {
		return ""
	}
	root := parseJSONObject(jsonRaw(outputSchema))
	properties := objectProperty(root, "properties")
	for _, key := range keys {
		prop := objectProperty(properties, key)
		if schemaTypeFromObject(prop) == "array" {
			return "/" + escapeJSONPointerToken(key)
		}
	}
	return ""
}

func scalarPropertySource(outputSchema spec.JSONSchema, keys []string) string {
	root := parseJSONObject(jsonRaw(outputSchema))
	properties := objectProperty(root, "properties")
	for _, key := range keys {
		prop := objectProperty(properties, key)
		switch schemaTypeFromObject(prop) {
		case "integer", "number":
			return "/" + escapeJSONPointerToken(key)
		}
	}
	return ""
}

func outputShapeForSchema(outputSchema spec.JSONSchema) spec.OutputResultShape {
	switch schemaRootType(outputSchema) {
	case "array":
		return spec.OutputShapeCollection
	case "object":
		return spec.OutputShapeObject
	default:
		return spec.OutputShapeScalar
	}
}

func schemaRootType(outputSchema spec.JSONSchema) string {
	return schemaTypeFromObject(parseJSONObject(jsonRaw(outputSchema)))
}

func schemaTypeFromObject(obj map[string]json.RawMessage) string {
	return rawString(obj["type"])
}

func jsonRaw(schema spec.JSONSchema) json.RawMessage {
	return json.RawMessage(schema)
}

func sortedRawMapKeys(input map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func rawInt(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	var value int
	if err := json.Unmarshal(raw, &value); err != nil {
		return 0
	}
	return value
}

func dataTypeFromSchema(obj map[string]json.RawMessage) string {
	switch schemaTypeFromObject(obj) {
	case "integer", "number":
		return "number"
	case "boolean":
		return "boolean"
	case "array":
		return "array"
	case "object":
		return "object"
	case "string":
		if rawString(obj["format"]) == "date-time" {
			return "datetime"
		}
		if rawString(obj["format"]) == "date" {
			return "date"
		}
		return "string"
	default:
		return "string"
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if v := strings.TrimSpace(value); v != "" {
			return v
		}
	}
	return ""
}

func fallbackLabel(value string) string {
	return HumanizeKey(value)
}

func flushWord(words *[]string, current *strings.Builder) {
	if current.Len() == 0 {
		return
	}
	*words = append(*words, current.String())
	current.Reset()
}
