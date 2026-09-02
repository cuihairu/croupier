// 覆盖目标：alert 包 handler 错误分支（service 错误 / bind 错误）、
// rules.go buildRuleItem LastFiredAt 分支与 rulesModel nil 分支、
// service.go 静默 ID 越界、extension silences 追加/移除/损坏配置等路径。
package alert

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	extensioninstallation "github.com/cuihairu/croupier/internal/core/extension/installation"
	"github.com/cuihairu/croupier/internal/model"
	extensiongorm "github.com/cuihairu/croupier/internal/repo/gorm/extension"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// newAlertErrRouter 构造一个空模型 svcCtx 的路由，用于触发 handler 错误分支。
func newAlertErrRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	s := NewService(&svc.ServiceContext{})
	h := NewHandler(s)
	r := gin.New()
	r.GET("/alerts", h.List)
	r.POST("/alerts/:id/silence", h.Silence)
	r.GET("/silences", h.SilencesList)
	r.DELETE("/silences/:id", h.SilenceDelete)
	r.GET("/alerts/rules", h.RulesList)
	r.PUT("/alerts/rules/:id", h.RulesUpdate)
	r.DELETE("/alerts/rules/:id", h.RulesDelete)
	return r
}

func alertDo(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestAlertHandler_ServiceErrorBranches(t *testing.T) {
	r := newAlertErrRouter(t)

	t.Run("list without alert model returns 500", func(t *testing.T) {
		rec := alertDo(r, http.MethodGet, "/alerts", "")
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("silences list without model returns 500", func(t *testing.T) {
		rec := alertDo(r, http.MethodGet, "/silences", "")
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("silence without model returns 500", func(t *testing.T) {
		rec := alertDo(r, http.MethodPost, "/alerts/a-1/silence", `{"duration":10}`)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("rules list without rule model returns 500", func(t *testing.T) {
		rec := alertDo(r, http.MethodGet, "/alerts/rules", "")
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("rules update malformed json returns 400", func(t *testing.T) {
		rec := alertDo(r, http.MethodPut, "/alerts/rules/1", `{`)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("rules delete without rule model returns 500", func(t *testing.T) {
		rec := alertDo(r, http.MethodDelete, "/alerts/rules/1", "")
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func TestSilenceDelete_HugeIDFormatRejected(t *testing.T) {
	db := alertNewMemDB(t)
	s := NewService(&svc.ServiceContext{AlertModel: model.NewAlertModel(db)})

	// 超出 uint64 的数字在 ParseUint 即失败。
	err := s.SilenceDelete(context.Background(), &SilenceDeleteRequest{ID: "18446744073709551616"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "静默ID格式不正确")
}

func TestRulesModel_NilSvcCtx(t *testing.T) {
	s := &Service{}
	assert.Nil(t, s.rulesModel())
}

func TestBuildRuleItem_LastFiredAtSet(t *testing.T) {
	now := time.Now()
	item := buildRuleItem(&model.AlertRule{
		Model: gorm.Model{ID: 7}, Name: "n", Metric: "cpu.usagePercent", LastFiredAt: &now,
	})
	assert.Equal(t, uint(7), item.ID)
	assert.Equal(t, now.UTC().Format(time.RFC3339), item.LastFiredAt)
}

// ---- extension silences 相关 ----

type alertExtEnv struct {
	db        *gorm.DB
	svc       *extensioninstallation.Service
	installed *model.ExtensionInstallation
	service   *Service
}

func newAlertExtEnv(t *testing.T, config map[string]any) *alertExtEnv {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ExtensionInstallation{}, &model.ExtensionEvent{}, &model.ExtensionRuntimeBinding{}))
	repos := extensiongorm.NewBundle(db)
	installationSvc := extensioninstallation.NewService(repos.Installation, repos.Event, repos.Binding)
	if config == nil {
		config = map[string]any{}
	}
	installed, err := installationSvc.Install(context.Background(), extensioninstallation.InstallRequest{
		ExtensionID:    officialAlertingID,
		ReleaseVersion: "1.0.0",
		ScopeType:      "system",
		ScopeID:        "global",
		TargetType:     "agent_group",
		TargetID:       "default",
		Config:         config,
		Operator:       "tester",
	})
	require.NoError(t, err)
	return &alertExtEnv{
		db:        db,
		svc:       installationSvc,
		installed: installed,
		service: NewService(&svc.ServiceContext{
			Extensions: &svc.ExtensionServices{Installation: installationSvc},
		}),
	}
}

func TestAppendAlertingSilenceToExtension_UpdatesExisting(t *testing.T) {
	env := newAlertExtEnv(t, map[string]any{
		"silences": []map[string]any{
			{"id": "42", "alertType": "old", "matchers": map[string]any{}, "startAt": "", "endAt": "", "createdBy": "old-user"},
		},
	})

	now := time.Now()
	err := env.service.appendAlertingSilenceToExtension(context.Background(), "cpu", &model.AlertSilence{
		Model:          gorm.Model{ID: 42, CreatedAt: now},
		CreatedBy:      "alice",
		DurationMinute: 5,
		ExpiresAt:      now.Add(5 * time.Minute),
	})
	require.NoError(t, err)

	items, ok, err := env.service.loadAlertingSilencesFromExtension(context.Background())
	require.NoError(t, err)
	require.True(t, ok)
	require.Len(t, items, 1)
	assert.Equal(t, "cpu", items[0].AlertType)
	assert.Equal(t, "alice", items[0].CreatedBy)
	assert.NotEmpty(t, items[0].EndAt)
}

func TestAppendAlertingSilenceToExtension_AppendsNew(t *testing.T) {
	env := newAlertExtEnv(t, map[string]any{})

	err := env.service.appendAlertingSilenceToExtension(context.Background(), "mem", &model.AlertSilence{
		Model: gorm.Model{ID: 7}, CreatedBy: "bob", DurationMinute: 10,
	})
	require.NoError(t, err)

	items, ok, err := env.service.loadAlertingSilencesFromExtension(context.Background())
	require.NoError(t, err)
	require.True(t, ok)
	require.Len(t, items, 1)
	assert.Equal(t, "7", items[0].Id)
	assert.Equal(t, "mem", items[0].AlertType)
}

func TestAppendAlertingSilenceToExtension_LoadError(t *testing.T) {
	env := newAlertExtEnv(t, nil)
	require.NoError(t, env.db.Model(&model.ExtensionInstallation{}).
		Where("id = ?", env.installed.ID).
		Update("config_json", "not-json").Error)

	err := env.service.appendAlertingSilenceToExtension(context.Background(), "cpu", &model.AlertSilence{Model: gorm.Model{ID: 1}})
	require.Error(t, err)
}

func TestRemoveAlertingSilenceFromExtension_RemovesEntry(t *testing.T) {
	env := newAlertExtEnv(t, map[string]any{
		"silences": []map[string]any{
			{"id": "1", "alertType": "cpu", "matchers": map[string]any{}},
			{"id": "2", "alertType": "mem", "matchers": map[string]any{}},
		},
	})

	require.NoError(t, env.service.removeAlertingSilenceFromExtension(context.Background(), "1"))
	items, ok, err := env.service.loadAlertingSilencesFromExtension(context.Background())
	require.NoError(t, err)
	require.True(t, ok)
	require.Len(t, items, 1)
	assert.Equal(t, "2", items[0].Id)

	// 删除不存在的 ID：原样保留。
	require.NoError(t, env.service.removeAlertingSilenceFromExtension(context.Background(), "999"))
	items, _, err = env.service.loadAlertingSilencesFromExtension(context.Background())
	require.NoError(t, err)
	require.Len(t, items, 1)

	// 无活跃安装时为 no-op。
	emptySvc := NewService(&svc.ServiceContext{})
	require.NoError(t, emptySvc.removeAlertingSilenceFromExtension(context.Background(), "1"))
}

func TestRemoveAlertingSilenceFromExtension_LoadError(t *testing.T) {
	env := newAlertExtEnv(t, nil)
	require.NoError(t, env.db.Model(&model.ExtensionInstallation{}).
		Where("id = ?", env.installed.ID).
		Update("config_json", "{invalid").Error)

	err := env.service.removeAlertingSilenceFromExtension(context.Background(), "1")
	require.Error(t, err)
}

func TestLoadAlertingSilencesFromExtension_NoSilencesKey(t *testing.T) {
	env := newAlertExtEnv(t, map[string]any{"other": "value"})
	items, ok, err := env.service.loadAlertingSilencesFromExtension(context.Background())
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Nil(t, items)
}

func TestFindActiveAlertingInstallation_UninstalledSkipped(t *testing.T) {
	env := newAlertExtEnv(t, nil)

	// Status = uninstalled → 跳过。
	require.NoError(t, env.db.Model(&model.ExtensionInstallation{}).
		Where("id = ?", env.installed.ID).Update("status", "uninstalled").Error)
	_, ok, err := env.service.findActiveAlertingInstallation(context.Background())
	require.NoError(t, err)
	assert.False(t, ok)

	// DesiredState = uninstalled → 跳过。
	require.NoError(t, env.db.Model(&model.ExtensionInstallation{}).
		Where("id = ?", env.installed.ID).
		Updates(map[string]any{"status": "active", "desired_state": "uninstalled"}).Error)
	_, ok, err = env.service.findActiveAlertingInstallation(context.Background())
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestFindActiveAlertingInstallation_ListError(t *testing.T) {
	env := newAlertExtEnv(t, nil)
	sqlDB, err := env.db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	_, _, err = env.service.findActiveAlertingInstallation(context.Background())
	require.Error(t, err)
}

func TestSilencesList_ExtensionConfigUsed(t *testing.T) {
	env := newAlertExtEnv(t, map[string]any{
		"silences": []map[string]any{
			{"id": "s-9", "alertType": "disk", "matchers": map[string]any{"k": "v"}, "createdBy": "bob"},
		},
	})
	resp, err := env.service.SilencesList(context.Background(), &SilencesListRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "s-9", resp.Items[0].Id)
	assert.Equal(t, "disk", resp.Items[0].AlertType)
}

func TestSilencesList_DatabaseError(t *testing.T) {
	db := alertNewMemDB(t)
	am := model.NewAlertModel(db)
	require.NoError(t, am.Create(context.Background(), &model.Alert{AlertID: "a", Type: "t"}))
	s := NewService(&svc.ServiceContext{AlertModel: am})

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	_, err = s.SilencesList(context.Background(), &SilencesListRequest{})
	require.Error(t, err)
}

// ---- 小工具 ----

func alertNewMemDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Alert{}, &model.AlertSilence{}))
	return db
}
