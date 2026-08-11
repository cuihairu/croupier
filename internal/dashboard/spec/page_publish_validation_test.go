package spec

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidatePublishablePageShapeRejectsInvalidPageVariant(t *testing.T) {
	tests := []struct {
		name string
		page PageSpec
		code string
	}{
		{
			name: "missing page body",
			page: PageSpec{Type: PageTypeOperation},
			code: "page_variant_invalid",
		},
		{
			name: "multiple page bodies",
			page: PageSpec{Type: PageTypeOperation, Operation: &OperationPageSpec{}, Resource: &ResourcePageSpec{}},
			code: "page_variant_invalid",
		},
		{
			name: "body does not match type",
			page: PageSpec{Type: PageTypeOperation, Resource: &ResourcePageSpec{}},
			code: "page_variant_type_mismatch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diagnostics := ValidatePublishablePageShape(tt.page)
			assert.NotEmpty(t, diagnostics)
			assert.Equal(t, tt.code, diagnostics[0].Code)
		})
	}
}

func TestValidatePublishablePageShapeAcceptsSingleMatchingVariant(t *testing.T) {
	diagnostics := ValidatePublishablePageShape(PageSpec{
		Type:      PageTypeOperation,
		Operation: &OperationPageSpec{},
		Bindings: []PageFunctionBinding{{
			ID: "main", Usage: BindingUsageAction,
		}},
	})
	assert.Empty(t, diagnostics)
}

func TestValidatePublishableOperationPageRequiresActionConfirmBinding(t *testing.T) {
	page := PageSpec{
		Type: PageTypeOperation,
		Operation: &OperationPageSpec{
			Confirm: &ConfirmActionSpec{BindingID: "missing"},
		},
		Bindings: []PageFunctionBinding{{ID: "main", Usage: BindingUsageAction}},
	}

	diagnostics := ValidatePublishablePageShape(page)
	assert.Len(t, diagnostics, 1)
	assert.Equal(t, "operation_confirm_binding_invalid", diagnostics[0].Code)

	page.Operation.Confirm.BindingID = "main"
	assert.Empty(t, ValidatePublishablePageShape(page))
}

func TestValidatePublishableResultView(t *testing.T) {
	tests := []struct {
		name     string
		result   *ResultViewSpec
		field    string
		wantDiag int
	}{
		{
			name:     "nil result view",
			result:   nil,
			field:    "report.resultView",
			wantDiag: 0,
		},
		{
			name:     "empty fields",
			result:   &ResultViewSpec{Fields: []ResultFieldSpec{}},
			field:    "report.resultView",
			wantDiag: 0,
		},
		{
			name: "missing key",
			result: &ResultViewSpec{
				Fields: []ResultFieldSpec{
					{Key: "", Title: LocalizedText{"zh-CN": "标题"}, DataType: "string"},
				},
			},
			field:    "report.resultView",
			wantDiag: 1,
		},
		{
			name: "duplicate keys",
			result: &ResultViewSpec{
				Fields: []ResultFieldSpec{
					{Key: "name", Title: LocalizedText{"zh-CN": "名称1"}, DataType: "string"},
					{Key: "name", Title: LocalizedText{"zh-CN": "名称2"}, DataType: "string"},
				},
			},
			field:    "report.resultView",
			wantDiag: 1,
		},
		{
			name: "missing title locale",
			result: &ResultViewSpec{
				Fields: []ResultFieldSpec{
					{Key: "name", Title: LocalizedText{}, DataType: "string"},
				},
			},
			field:    "report.resultView",
			wantDiag: 1,
		},
		{
			name: "missing data type",
			result: &ResultViewSpec{
				Fields: []ResultFieldSpec{
					{Key: "name", Title: LocalizedText{"zh-CN": "名称"}},
				},
			},
			field:    "report.resultView",
			wantDiag: 1,
		},
		{
			name: "valid field",
			result: &ResultViewSpec{
				Fields: []ResultFieldSpec{
					{Key: "name", Title: LocalizedText{"zh-CN": "名称"}, DataType: "string"},
				},
			},
			field:    "report.resultView",
			wantDiag: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := validatePublishableResultView(tt.result, tt.field)
			assert.Len(t, diags, tt.wantDiag)
		})
	}
}
