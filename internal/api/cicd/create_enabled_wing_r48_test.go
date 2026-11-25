package cicd_test

// Create's explicit enabled pointer wing (coverage sweep R48: the `row.Enabled =
// *req.Enabled` at service.go:219 is the only unowned uncovered block in the full
// profile - the remaining 31 statements across 21 files all line up with the
// round 3-28 registration/avoidance ledger).
//
// Locked contract:
// - `"enabled": true` as an explicit pointer -> the :219 assignment wing runs; true is
//   not the bool zero value, so the gorm:"default:true" column-dropping behavior is not
//   triggered and the INSERT really lands;
// - Control group: an absent key (nil pointer) skips the assignment wing, and the
//   landed form equals the explicit true (the default), so both wings behave the same
//   but the code paths diverge;
// - Known defect boundary (outside this case's assertion surface): an explicit false is
//   dropped at the driver level by model.CicdIntegration.Enabled's gorm:"default:true"
//   ("created as disabled" is unreachable) - see service_paths_test.go
//   TestCicdService_CreateEnabledFalseIsDroppedKnownDefect (registered with t.Skip;
//   the fix needs a *bool model field + a numbered migration, a separate batch).
import (
	"net/http"
	"testing"

	"github.com/cuihairu/croupier/internal/api/cicd"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCicdService_CreateExplicitEnabledWings(t *testing.T) {
	ctxSvc, db := setupCtx(t)
	r := newCicdRouter(cicd.NewService(ctxSvc))

	// Explicit true pointer -> the :219 assignment wing runs
	w := doCicd(r, http.MethodPost, "/cicd/integrations",
		`{"kind":"jenkins","name":"enabled-true","endpoint":"http://ci.internal","enabled":true}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"enabled":true`)

	var row model.CicdIntegration
	require.NoError(t, db.Where("name = ?", "enabled-true").First(&row).Error)
	assert.True(t, row.Enabled, "显式 true 应真实落库（非零值，不涉 default 丢弃缺陷）")

	// Control: an absent key (nil pointer) skips the assignment wing; the default true
	// lands in the same shape
	w = doCicd(r, http.MethodPost, "/cicd/integrations",
		`{"kind":"jenkins","name":"enabled-nil","endpoint":"http://ci.internal"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"enabled":true`)

	// Pitfall avoidance: GORM First(&row) folds the struct's existing primary key into
	// the condition - the second query must use a fresh variable
	var rowNil model.CicdIntegration
	require.NoError(t, db.Where("name = ?", "enabled-nil").First(&rowNil).Error)
	assert.True(t, rowNil.Enabled, "键缺省走构造器默认 true")
}
