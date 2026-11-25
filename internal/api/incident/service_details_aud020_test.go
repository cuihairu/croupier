package incident

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/require"
)

// AUD-020: incident detail updates must use a database-serializable JSON type.
//
// The model's Details column is a JSON type (datatypes.JSONMap); UpdateIncident
// previously stuffed a bare map straight into the updates map -- gorm's map
// update bypasses the column type's serializer, so ordinary detail updates
// always failed with: sql: unsupported type map[string]interface {}. This
// test covers save and read-back for a normal object, nested values, and an
// empty object.
func TestUpdateIncident_DetailsJSONRoundtrip(t *testing.T) {
	s := setupService(t)
	ctx := context.Background()
	qa := mustCategory(t, s, model.IncidentCatQA)
	inc, _, err := s.CreateIncident(ctx, &IncidentCreateRequest{
		Title: "详情", CategoryID: qa.ID,
		Details: map[string]interface{}{"reason": "first"},
	}, "tester")
	require.NoError(t, err)
	require.Equal(t, "first", inc.Details["reason"])

	// Normal object plus nested values (a list and an object).
	upd, err := s.UpdateIncident(ctx, inc.ID, &IncidentUpdateRequest{
		Details: map[string]interface{}{
			"reason": "second",
			"steps":  []interface{}{"step-1", "step-2"},
			"meta":   map[string]interface{}{"owner": "ops"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "second", upd.Details["reason"])

	steps, ok := upd.Details["steps"].([]interface{})
	require.True(t, ok, "list value should round-trip")
	require.Equal(t, []interface{}{"step-1", "step-2"}, steps)

	nested, ok := upd.Details["meta"].(map[string]interface{})
	require.True(t, ok, "nested object should round-trip")
	require.Equal(t, "ops", nested["owner"])

	// Independent read-back (not the Update return value; go through GetIncident
	// to re-query the database).
	got, err := s.GetIncident(ctx, inc.ID)
	require.NoError(t, err)
	require.Equal(t, "second", got.Details["reason"])
	require.Len(t, got.Details, 3)

	// Empty object: saved as {} and read back; it must not be silently dropped
	// or raise an error.
	upd, err = s.UpdateIncident(ctx, inc.ID, &IncidentUpdateRequest{Details: map[string]interface{}{}})
	require.NoError(t, err)
	require.NotNil(t, upd.Details)
	require.Empty(t, upd.Details)
	require.Len(t, upd.Details, 0)

	got, err = s.GetIncident(ctx, inc.ID)
	require.NoError(t, err)
	require.NotNil(t, got.Details)
	require.Len(t, got.Details, 0)
}
