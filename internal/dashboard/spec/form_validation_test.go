package spec

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const formValidationSchema = `{
	"type": "object",
	"properties": {
		"name": {"type": "string"},
		"count": {"type": "integer"},
		"flag": {"type": "boolean"},
		"tags": {"type": "array", "items": {"type": "object", "properties": {"label": {"type": "string"}}}},
		"address": {"type": "object", "properties": {"city": {"type": "string"}}},
		"a/b": {"type": "string"},
		"a~b": {"type": "string"},
		"nullableName": {"type": ["string", "null"]},
		"payload": {"type": "object"},
		"list": {"type": "array"}
	}
}`

func formValidationPage(form *FormPresentationSpec) PageSpec {
	return PageSpec{
		Type: PageTypeOperation,
		Operation: &OperationPageSpec{
			Form: form,
		},
	}
}

func formValidationCodes(diags []Diagnostic) []string {
	codes := make([]string, 0, len(diags))
	for _, diag := range diags {
		codes = append(codes, diag.Code)
	}
	return codes
}

func TestValidateFormPresentationsDiagnostics(t *testing.T) {
	tests := []struct {
		name  string
		form  FormPresentationSpec
		codes []string
		field string
	}{
		{
			name: "unknown field",
			form: FormPresentationSpec{
				JSONSchema: JSONSchema(formValidationSchema),
				Fields:     []FormFieldSpec{{Key: "missing", Widget: FormWidgetInput, Width: 12}},
			},
			codes: []string{"form_field_unknown"},
			field: "operation.form.fields[0].key",
		},
		{
			name: "dotted name is literal key, not a path",
			form: FormPresentationSpec{
				JSONSchema: JSONSchema(formValidationSchema),
				Fields:     []FormFieldSpec{{Key: "address.city", Widget: FormWidgetInput}},
			},
			codes: []string{"form_field_unknown"},
		},
		{
			name: "empty field key",
			form: FormPresentationSpec{
				JSONSchema: JSONSchema(formValidationSchema),
				Fields:     []FormFieldSpec{{Key: "  ", Widget: FormWidgetInput}},
			},
			codes: []string{"form_field_key_missing"},
		},
		{
			name: "duplicate field",
			form: FormPresentationSpec{
				JSONSchema: JSONSchema(formValidationSchema),
				Fields: []FormFieldSpec{
					{Key: "name", Widget: FormWidgetInput},
					{Key: "name", Widget: FormWidgetInput},
				},
			},
			codes: []string{"form_field_duplicate"},
			field: "operation.form.fields[1].key",
		},
		{
			name: "group key missing",
			form: FormPresentationSpec{
				JSONSchema: JSONSchema(formValidationSchema),
				Fields:     []FormFieldSpec{{Key: "name", Widget: FormWidgetInput}},
				Groups:     []FormGroupSpec{{Fields: []string{"name"}}},
			},
			codes: []string{"form_group_key_missing"},
		},
		{
			name: "duplicate group key",
			form: FormPresentationSpec{
				JSONSchema: JSONSchema(formValidationSchema),
				Fields:     []FormFieldSpec{{Key: "name", Widget: FormWidgetInput}, {Key: "flag", Widget: FormWidgetSwitch}},
				Groups: []FormGroupSpec{
					{Key: "g", Fields: []string{"name"}},
					{Key: "g", Fields: []string{"flag"}},
				},
			},
			codes: []string{"form_group_key_duplicate"},
		},
		{
			name: "group field not in schema",
			form: FormPresentationSpec{
				JSONSchema: JSONSchema(formValidationSchema),
				Fields:     []FormFieldSpec{{Key: "name", Widget: FormWidgetInput}},
				Groups:     []FormGroupSpec{{Key: "g", Fields: []string{"ghost"}}},
			},
			codes: []string{"form_group_field_unknown"},
		},
		{
			name: "group field not declared in form.fields",
			form: FormPresentationSpec{
				JSONSchema: JSONSchema(formValidationSchema),
				Groups:     []FormGroupSpec{{Key: "g", Fields: []string{"name"}}},
			},
			codes: []string{"form_group_field_unknown"},
		},
		{
			name: "field assigned to two groups",
			form: FormPresentationSpec{
				JSONSchema: JSONSchema(formValidationSchema),
				Fields:     []FormFieldSpec{{Key: "name", Widget: FormWidgetInput}, {Key: "flag", Widget: FormWidgetSwitch}},
				Groups: []FormGroupSpec{
					{Key: "g1", Fields: []string{"name"}},
					{Key: "g2", Fields: []string{"name"}},
				},
			},
			codes: []string{"form_group_field_conflict"},
		},
		{
			name: "duplicate field inside group",
			form: FormPresentationSpec{
				JSONSchema: JSONSchema(formValidationSchema),
				Fields:     []FormFieldSpec{{Key: "name", Widget: FormWidgetInput}},
				Groups:     []FormGroupSpec{{Key: "g", Fields: []string{"name", "name"}}},
			},
			codes: []string{"form_group_field_duplicate"},
		},
		{
			name: "invalid widget",
			form: FormPresentationSpec{
				JSONSchema: JSONSchema(formValidationSchema),
				Fields:     []FormFieldSpec{{Key: "name", Widget: FormWidget("NoSuchWidget")}},
			},
			codes: []string{"form_widget_invalid"},
			field: "operation.form.fields[0].widget",
		},
		{
			name: "incompatible widget",
			form: FormPresentationSpec{
				JSONSchema: JSONSchema(formValidationSchema),
				Fields:     []FormFieldSpec{{Key: "name", Widget: FormWidgetNumber}},
			},
			codes: []string{"form_widget_incompatible"},
		},
		{
			name: "invalid width negative",
			form: FormPresentationSpec{
				JSONSchema: JSONSchema(formValidationSchema),
				Fields:     []FormFieldSpec{{Key: "name", Widget: FormWidgetInput, Width: -1}},
			},
			codes: []string{"form_width_invalid"},
			field: "operation.form.fields[0].width",
		},
		{
			name: "invalid width above 12",
			form: FormPresentationSpec{
				JSONSchema: JSONSchema(formValidationSchema),
				Fields:     []FormFieldSpec{{Key: "name", Widget: FormWidgetInput, Width: 13}},
			},
			codes: []string{"form_width_invalid"},
		},
		{
			name: "zero width is valid",
			form: FormPresentationSpec{
				JSONSchema: JSONSchema(formValidationSchema),
				Fields:     []FormFieldSpec{{Key: "name", Widget: FormWidgetInput, Width: 0}},
			},
			codes: []string{},
		},
		{
			name: "declared-only DateRange is valid",
			form: FormPresentationSpec{
				JSONSchema: JSONSchema(formValidationSchema),
				Fields:     []FormFieldSpec{{Key: "name", Widget: FormWidgetDateRange}},
			},
			codes: []string{},
		},
		{
			name: "declared-only RichText is valid",
			form: FormPresentationSpec{
				JSONSchema: JSONSchema(formValidationSchema),
				Fields:     []FormFieldSpec{{Key: "name", Widget: FormWidgetRichText}},
			},
			codes: []string{},
		},
		{
			name: "nullable type array accepts string widget",
			form: FormPresentationSpec{
				JSONSchema: JSONSchema(formValidationSchema),
				Fields:     []FormFieldSpec{{Key: "nullableName", Widget: FormWidgetInput}},
			},
			codes: []string{},
		},
		{
			name: "no properties skips field resolution",
			form: FormPresentationSpec{
				JSONSchema: JSONSchema(`{"type":"array","items":{"type":"string"}}`),
				Fields:     []FormFieldSpec{{Key: "anything", Widget: FormWidgetInput}},
			},
			codes: []string{},
		},
		{
			name: "non-object schema skips field resolution",
			form: FormPresentationSpec{
				JSONSchema: JSONSchema(`"scalar"`),
				Fields:     []FormFieldSpec{{Key: "anything", Widget: FormWidgetInput}},
			},
			codes: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := ValidateFormPresentations(formValidationPage(&tt.form))
			assert.Equal(t, tt.codes, formValidationCodes(diags))
			for _, diag := range diags {
				assert.Equal(t, SeverityError, diag.Severity)
				assert.True(t, strings.HasPrefix(diag.Field, "operation.form."), diag.Field)
			}
			if tt.field != "" {
				require.NotEmpty(t, diags)
				assert.Equal(t, tt.field, diags[0].Field)
			}
		})
	}
}

func TestValidateFormPresentationsPointerPaths(t *testing.T) {
	form := FormPresentationSpec{
		JSONSchema: JSONSchema(formValidationSchema),
		Fields: []FormFieldSpec{
			{Key: "/address/city", Widget: FormWidgetInput, Width: 12},
			{Key: "/tags/-/label", Widget: FormWidgetInput, Width: 12},
			{Key: "/a~1b", Widget: FormWidgetInput, Width: 12},
			{Key: "/a~0b", Widget: FormWidgetInput, Width: 12},
		},
		Groups: []FormGroupSpec{{
			Key:    "location",
			Fields: []string{"/address/city", "/tags/-/label", "/a~1b", "/a~0b"},
		}},
	}
	assert.Empty(t, ValidateFormPresentations(formValidationPage(&form)))

	// A pointer with a segment missing from properties is diagnosed with a reason.
	form.Fields = []FormFieldSpec{{Key: "/address/zipcode", Widget: FormWidgetInput}}
	form.Groups = nil
	diags := ValidateFormPresentations(formValidationPage(&form))
	require.Len(t, diags, 1)
	assert.Equal(t, "form_field_unknown", diags[0].Code)
	assert.Contains(t, diags[0].Message, "zipcode")

	// A pointer traversing an array whose items do not declare properties.
	form.Fields = []FormFieldSpec{{Key: "/list/-/name", Widget: FormWidgetInput}}
	diags = ValidateFormPresentations(formValidationPage(&form))
	require.Len(t, diags, 1)
	assert.Equal(t, "form_field_unknown", diags[0].Code)
}

func TestValidateFormPresentationsAllLocations(t *testing.T) {
	bad := func() *FormPresentationSpec {
		return &FormPresentationSpec{
			JSONSchema: JSONSchema(formValidationSchema),
			Fields:     []FormFieldSpec{{Key: "no-such-field", Widget: FormWidgetInput}},
		}
	}
	tests := []struct {
		name  string
		page  PageSpec
		field string
	}{
		{
			name:  "resource.createForm",
			page:  PageSpec{Type: PageTypeResource, Resource: &ResourcePageSpec{CreateForm: bad()}},
			field: "resource.createForm.fields[0].key",
		},
		{
			name:  "resource.updateForm",
			page:  PageSpec{Type: PageTypeResource, Resource: &ResourcePageSpec{UpdateForm: bad()}},
			field: "resource.updateForm.fields[0].key",
		},
		{
			name: "resource.actions[i].form",
			page: PageSpec{Type: PageTypeResource, Resource: &ResourcePageSpec{
				Actions: []ActionSpec{{Key: "ban", Form: bad()}},
			}},
			field: "resource.actions[0].form.fields[0].key",
		},
		{
			name:  "operation.form",
			page:  formValidationPage(bad()),
			field: "operation.form.fields[0].key",
		},
		{
			name:  "task.form",
			page:  PageSpec{Type: PageTypeTask, Task: &TaskPageSpec{Form: bad()}},
			field: "task.form.fields[0].key",
		},
		{
			name:  "report.queryForm",
			page:  PageSpec{Type: PageTypeReport, Report: &ReportPageSpec{QueryForm: bad()}},
			field: "report.queryForm.fields[0].key",
		},
		{
			name: "composite.sections[i].form",
			page: PageSpec{Type: PageTypeComposite, Composite: &CompositePageSpec{
				Sections: []CompositeSection{{Key: "s1", Form: bad()}},
			}},
			field: "composite.sections[0].form.fields[0].key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := ValidateFormPresentations(tt.page)
			require.Len(t, diags, 1)
			assert.Equal(t, "form_field_unknown", diags[0].Code)
			assert.Equal(t, tt.field, diags[0].Field)
		})
	}
}

func TestValidateFormPresentationsValidFullFixture(t *testing.T) {
	form := FormPresentationSpec{
		JSONSchema: JSONSchema(formValidationSchema),
		Fields: []FormFieldSpec{
			{Key: "name", Widget: FormWidgetInput, Width: 12},
			{Key: "count", Widget: FormWidgetNumber, Width: 6},
			{Key: "flag", Widget: FormWidgetSwitch, Width: 6},
			{Key: "tags", Widget: FormWidgetArray, Width: 12},
			{Key: "address", Widget: FormWidgetObject, Width: 12},
			{Key: "/address/city", Widget: FormWidgetInput, Width: 12},
			{Key: "/tags/-/label", Widget: FormWidgetInput, Width: 12},
			{Key: "payload", Widget: FormWidgetKeyValue, Width: 12},
			{Key: "list", Widget: FormWidgetMultiSelect, Width: 12},
		},
		Groups: []FormGroupSpec{
			{Key: "basic", Fields: []string{"name", "count", "flag"}},
			{Key: "complex", Fields: []string{"/address/city", "/tags/-/label", "payload", "list"}},
		},
	}
	page := PageSpec{
		Type: PageTypeResource,
		Resource: &ResourcePageSpec{
			CreateForm: &form,
			UpdateForm: &form,
			Actions: []ActionSpec{
				{Key: "row", Form: &form},
				{Key: "second", Form: nil},
			},
		},
		Operation: &OperationPageSpec{Form: &form},
		Task:      &TaskPageSpec{Form: &form},
		Report:    &ReportPageSpec{QueryForm: &form},
		Composite: &CompositePageSpec{Sections: []CompositeSection{{Key: "s", Form: &form}}},
	}
	assert.Empty(t, ValidateFormPresentations(page))
}

func TestValidatePublishablePageShapeSurfacesFormDiagnostics(t *testing.T) {
	page := PageSpec{
		Type: PageTypeOperation,
		Operation: &OperationPageSpec{
			Form: &FormPresentationSpec{
				JSONSchema: JSONSchema(formValidationSchema),
				Fields: []FormFieldSpec{
					{Key: "name", Widget: FormWidgetNumber, Width: 13},
				},
			},
		},
		Bindings: []PageFunctionBinding{{ID: "main", Usage: BindingUsageAction}},
	}
	diags := ValidatePublishablePageShape(page)
	codes := formValidationCodes(diags)
	assert.Contains(t, codes, "form_widget_incompatible")
	assert.Contains(t, codes, "form_width_invalid")
	for _, diag := range diags {
		if strings.HasPrefix(diag.Code, "form_") {
			assert.Equal(t, SeverityError, diag.Severity)
		}
	}
}
