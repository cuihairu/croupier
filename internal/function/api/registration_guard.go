package api

import (
	"fmt"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/function/registrationguard"
)

// validateRegistrationBoundary enforces the function registration contract at
// the HTTP boundary: only stable key, schema, resource/operation/capability/
// execution/risk/permission/approval fields are accepted. Presentation,
// navigation, or page-composition fields (menu, labels, columns, pagination,
// mapping, component trees, page routes, ...) are rejected with a structured
// 400 error naming the offending field instead of being silently ignored.
func validateRegistrationBoundary(extensions map[string]string, inputSchema, outputSchema string) error {
	if violation, ok := registrationguard.FindPresentationViolation(extensions, inputSchema, outputSchema); ok {
		return presentationFieldError(violation.Location, violation.Field)
	}
	return nil
}

func presentationFieldError(location, field string) error {
	return errorx.NewBadRequestWithDetails(
		fmt.Sprintf("presentation field %q is not allowed in function registration; registration only accepts the executable capability contract", field),
		map[string]any{
			"field":    field,
			"location": location,
		},
	)
}
