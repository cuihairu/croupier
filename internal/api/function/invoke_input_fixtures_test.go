package function

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"gorm.io/datatypes"

	"github.com/cuihairu/croupier/internal/validation/validationtest"
)

// SUI-016: the HTTP function invoke entry asserts the shared fixture table
// through its own validateInvokeInput wrapper, so the verdict seen by a
// console operator and the one seen by a direct API caller cannot drift.
func TestValidateInvokeInputSharedFixtures(t *testing.T) {
	for _, fixture := range validationtest.SchemaValidationFixtures() {
		t.Run(fixture.Name, func(t *testing.T) {
			f := newInvokeFixture(t)
			if fixture.Schema != nil {
				raw, err := json.Marshal(fixture.Schema)
				if err != nil {
					t.Fatalf("marshal fixture schema: %v", err)
				}
				var jsonMap datatypes.JSONMap
				if err := json.Unmarshal(raw, &jsonMap); err != nil {
					t.Fatalf("unmarshal fixture schema: %v", err)
				}
				upsertInputContract(t, f, "demo.grant", jsonMap)
			}
			err := validateInvokeInput(context.Background(), f.svcCtx,
				&FunctionInvokeRequest{ID: "demo.grant"}, []byte(fixture.Payload))
			switch {
			case fixture.WantDefect:
				if err == nil {
					t.Fatalf("expected defect rejection, got nil")
				}
			case fixture.WantPass:
				if err != nil {
					t.Fatalf("expected pass, got %v", err)
				}
			default:
				if err == nil {
					t.Fatalf("expected violation, got nil")
				}
				if fixture.WantField != "" && !strings.Contains(err.Error(), fixture.WantField) {
					t.Fatalf("expected field %s in message, got %v", fixture.WantField, err.Error())
				}
			}
		})
	}
}
