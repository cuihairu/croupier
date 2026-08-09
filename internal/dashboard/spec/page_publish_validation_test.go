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
