// 覆盖目标：evaluateRule 的指标缺失分支、fire 的 alert 落库失败与 notify 分发、
// ExtractMetric 的非法磁盘路径。
package alertrule

import (
	"context"
	"errors"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	notify "github.com/cuihairu/croupier/internal/service/notify"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func openAlertruleDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/alertrule-v8.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	return db
}

func TestV8EvaluateRule_MetricMissing(t *testing.T) {
	e, db := setup(t)
	mustCreateRule(t, model.NewAlertRuleModel(db), &model.AlertRule{
		Name: "missing-disk", Metric: "disk./data.usedPercent", Operator: "gt",
		Threshold: 10, ForCount: 1, Enabled: true,
	})

	// 上报中无任何磁盘 → 指标缺失，视为未命中（计数不增加、不产生告警）。
	e.EvaluateAgent(context.Background(), "a1", report(95, 0, 0))
	assert.EqualValues(t, 0, alertCount(t, db))
}

func TestV8Fire_AlertCreateError(t *testing.T) {
	db := openAlertruleDB(t)
	e := New(model.NewAlertRuleModel(db), model.NewAlertModel(db), nil)
	mustCreateRule(t, model.NewAlertRuleModel(db), &model.AlertRule{
		Name: "cpu", Metric: "cpu.usagePercent", Operator: "gt",
		Threshold: 1, ForCount: 1, Enabled: true,
	})

	require.NoError(t, db.Callback().Create().Before("gorm:create").Register("v8:cfail", func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "alerts" {
			_ = tx.AddError(errors.New("injected alert create failure"))
		}
	}))

	// alert 落库失败仅告警日志，规则状态仍推进（不 panic）。
	e.EvaluateAgent(context.Background(), "a1", report(95, 0, 0))
}

func TestV8Fire_WithNotifyDispatch(t *testing.T) {
	db := openAlertruleDB(t)
	e := New(model.NewAlertRuleModel(db), model.NewAlertModel(db), notify.New(nil, nil))
	mustCreateRule(t, model.NewAlertRuleModel(db), &model.AlertRule{
		Name: "cpu", Metric: "cpu.usagePercent", Operator: "gt",
		Threshold: 1, ForCount: 1, Enabled: true, Level: model.AlertRuleLevelWarning,
	})

	e.EvaluateAgent(context.Background(), "a1", report(95, 0, 0))
	assert.EqualValues(t, 1, alertCount(t, db))
}

func TestV8ExtractMetric_InvalidDiskPath(t *testing.T) {
	r := report(50, 0, 0)
	_, ok := ExtractMetric("disk.nofield", r)
	assert.False(t, ok)

	_, ok = ExtractMetric("disk.", r)
	assert.False(t, ok)

	_, ok = ExtractMetric("disk..usedPercent", r)
	assert.False(t, ok)
}
