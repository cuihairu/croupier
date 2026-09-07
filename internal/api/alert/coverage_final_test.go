package alert

import (
	"context"
	"errors"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// 注入式 gorm 回调：按发生序号让目标操作失败。

func injectQueryFailure(t *testing.T, db *gorm.DB, occurrence int) {
	t.Helper()
	var n int
	require.NoError(t, db.Callback().Query().Before("gorm:query").
		Register("test.fail.query", func(tx *gorm.DB) {
			n++
			if n >= occurrence {
				tx.AddError(errors.New("injected query failure"))
			}
		}))
	t.Cleanup(func() { _ = db.Callback().Query().Remove("test.fail.query") })
}

func injectCreateFailure(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Callback().Create().Before("gorm:create").
		Register("test.fail.create", func(tx *gorm.DB) {
			tx.AddError(errors.New("injected create failure"))
		}))
	t.Cleanup(func() { _ = db.Callback().Create().Remove("test.fail.create") })
}

func injectUpdateFailure(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Callback().Update().Before("gorm:update").
		Register("test.fail.update", func(tx *gorm.DB) {
			tx.AddError(errors.New("injected update failure"))
		}))
	t.Cleanup(func() { _ = db.Callback().Update().Remove("test.fail.update") })
}

func injectDeleteFailure(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Callback().Delete().Before("gorm:delete").
		Register("test.fail.delete", func(tx *gorm.DB) {
			tx.AddError(errors.New("injected delete failure"))
		}))
	t.Cleanup(func() { _ = db.Callback().Delete().Remove("test.fail.delete") })
}

// ---- service.go ----

func TestFinalService_List_ModelError(t *testing.T) {
	db := alertNewMemDB(t)
	s := NewService(&svc.ServiceContext{AlertModel: model.NewAlertModel(db)})
	injectQueryFailure(t, db, 1)

	resp, err := s.List(context.Background(), &AlertsListRequest{})
	require.Error(t, err)
	assert.Nil(t, resp)
}

func TestFinalService_Silence_CreateSilenceError(t *testing.T) {
	db := alertNewMemDB(t)
	am := model.NewAlertModel(db)
	s := NewService(&svc.ServiceContext{AlertModel: am})
	require.NoError(t, am.Create(context.Background(), &model.Alert{AlertID: "a-1", Type: "cpu"}))
	injectCreateFailure(t, db)

	err := s.Silence(context.Background(), &AlertSilenceRequest{ID: "a-1", Duration: 5})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "injected create failure")
}

func TestFinalService_SilencesList_FindByIDsError(t *testing.T) {
	db := alertNewMemDB(t)
	am := model.NewAlertModel(db)
	s := NewService(&svc.ServiceContext{AlertModel: am})
	require.NoError(t, am.CreateSilence(context.Background(), &model.AlertSilence{AlertID: 1, DurationMinute: 5}))
	// ListSilences 是第 1 个 query，FindByIDs 是第 2 个。
	injectQueryFailure(t, db, 2)

	resp, err := s.SilencesList(context.Background(), &SilencesListRequest{})
	require.Error(t, err)
	assert.Nil(t, resp)
}

func TestFinalService_SilencesList_ExtensionLoadError(t *testing.T) {
	env := newAlertExtEnv(t, nil)
	require.NoError(t, env.db.Model(&model.ExtensionInstallation{}).
		Where("id = ?", env.installed.ID).
		Update("config_json", "{broken").Error)

	resp, err := env.service.SilencesList(context.Background(), &SilencesListRequest{})
	require.Error(t, err)
	assert.Nil(t, resp)
}

func TestFinalService_SilenceDelete_ModelError(t *testing.T) {
	db := alertNewMemDB(t)
	s := NewService(&svc.ServiceContext{AlertModel: model.NewAlertModel(db)})
	injectDeleteFailure(t, db)

	err := s.SilenceDelete(context.Background(), &SilenceDeleteRequest{ID: "1"})
	require.Error(t, err)
}

func TestFinalService_LoadSilences_BadShapeError(t *testing.T) {
	env := newAlertExtEnv(t, map[string]any{"silences": "not-an-array"})

	items, ok, err := env.service.loadAlertingSilencesFromExtension(context.Background())
	require.Error(t, err)
	assert.False(t, ok)
	assert.Nil(t, items)
}

// ---- rules.go ----

func TestFinalRules_ListModelError(t *testing.T) {
	_, s, db := newAlertRulesEnv(t)
	injectQueryFailure(t, db, 1)

	resp, err := s.RulesList(context.Background(), &RulesListRequest{})
	require.Error(t, err)
	assert.Nil(t, resp)
}

func TestFinalRules_CreateModelError(t *testing.T) {
	_, s, db := newAlertRulesEnv(t)
	injectCreateFailure(t, db)

	resp, err := s.RulesCreate(context.Background(), &RuleCreateRequest{
		Name: "n", Metric: "cpu.usagePercent", Operator: "gt", Threshold: 90,
	})
	require.Error(t, err)
	assert.Nil(t, resp)
}

func TestFinalRules_UpdateValidLevel(t *testing.T) {
	_, s, _ := newAlertRulesEnv(t)

	created, err := s.RulesCreate(context.Background(), &RuleCreateRequest{
		Name: "n", Metric: "cpu.usagePercent", Operator: "gt",
	})
	require.NoError(t, err)

	level := model.AlertRuleLevelCritical
	resp, err := s.RulesUpdate(context.Background(), created.Item.ID, &RuleUpdateRequest{Level: &level})
	require.NoError(t, err)
	assert.Equal(t, model.AlertRuleLevelCritical, resp.Item.Level)
}

func TestFinalRules_UpdateModelError(t *testing.T) {
	_, s, db := newAlertRulesEnv(t)
	created, err := s.RulesCreate(context.Background(), &RuleCreateRequest{
		Name: "n", Metric: "cpu.usagePercent", Operator: "gt",
	})
	require.NoError(t, err)
	injectUpdateFailure(t, db)

	resp, err := s.RulesUpdate(context.Background(), created.Item.ID, &RuleUpdateRequest{
		Name: strPtrAlert("new-name"),
	})
	require.Error(t, err)
	assert.Nil(t, resp)
}

func TestFinalRules_UpdateReloadError(t *testing.T) {
	_, s, db := newAlertRulesEnv(t)
	created, err := s.RulesCreate(context.Background(), &RuleCreateRequest{
		Name: "n", Metric: "cpu.usagePercent", Operator: "gt",
	})
	require.NoError(t, err)
	// FindByID(#1) 成功，Update 成功，FindByID(#2) 注错。
	injectQueryFailure(t, db, 2)

	resp, err := s.RulesUpdate(context.Background(), created.Item.ID, &RuleUpdateRequest{
		Name: strPtrAlert("new-name"),
	})
	require.Error(t, err)
	assert.Nil(t, resp)
}

func strPtrAlert(v string) *string { return &v }
