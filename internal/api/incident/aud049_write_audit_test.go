package incident

import (
	"context"
	"fmt"
	"testing"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// aud049IncidentEnv 组装带审计内存存储的事故服务（AUD-049）。
func aud049IncidentEnv(t *testing.T) (*Service, *audit.InMemoryAuditStore) {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1)
	}
	require.NoError(t, db.AutoMigrate(&model.IncidentCategory{}, &model.Incident{}, &model.Bug{}))
	require.NoError(t, model.SeedIncidentCategories(context.Background(), db))
	store := audit.NewInMemoryAuditStore()
	svcCtx := &svc.ServiceContext{DB: db, AuditService: audit.NewAuditService(store, nil)}
	return NewService(svcCtx), store
}

func aud049IncidentRecords(t *testing.T, store *audit.InMemoryAuditStore, event audit.AuditEventType) []*audit.AuditRecord {
	t.Helper()
	records, _, err := store.List(audit.AuditFilter{EventType: []audit.AuditEventType{event}}, audit.AuditPage{Page: 1, PageSize: 50})
	require.NoError(t, err)
	return records
}

// InMemoryAuditStore.List 底层是 map（遍历顺序随机），断言按 outcome 子集
// 过滤，不依赖顺序。
func aud049ByOutcome(records []*audit.AuditRecord, outcome string) []*audit.AuditRecord {
	var out []*audit.AuditRecord
	for _, r := range records {
		if r.Outcome == outcome {
			out = append(out, r)
		}
	}
	return out
}

// 类别增删改、事故登记、状态流转成功留痕；跨 scope 的更新/流转落 denied。
func TestAud049_IncidentWriteAudit(t *testing.T) {
	s, store := aud049IncidentEnv(t)
	ctx := context.Background()

	cat, err := s.CreateCategory(ctx, &CategoryUpsertRequest{Name: strPtr("审计"), Slug: "aud049"})
	require.NoError(t, err)
	cats := aud049IncidentRecords(t, store, audit.EventIncidentCategory)
	require.Len(t, cats, 1)
	assert.Equal(t, "success", cats[0].Outcome)
	assert.Equal(t, "category_create", cats[0].Details["action"])
	assert.Equal(t, "aud049", cats[0].Details["slug"])
	assert.Equal(t, "incident", cats[0].Resource.Type)

	created, _, err := s.CreateIncident(ctx, &IncidentCreateRequest{
		Title: "aud049-incident", CategoryID: cat.ID, Severity: model.IncidentSeverityWarning,
		GameID: "game-a", Env: "prod",
	}, "alice")
	require.NoError(t, err)
	creates := aud049IncidentRecords(t, store, audit.EventIncidentCreate)
	require.Len(t, creates, 1)
	assert.Equal(t, "success", creates[0].Outcome)
	assert.Equal(t, fmt.Sprint(created.ID), creates[0].Resource.ID)
	assert.Equal(t, "game-a", creates[0].Details["gameId"])

	foreign := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "game-b", Env: "prod"})
	_, err = s.UpdateIncident(foreign, created.ID, &IncidentUpdateRequest{Title: strPtr("越权改")})
	require.Error(t, err)
	updates := aud049IncidentRecords(t, store, audit.EventIncidentUpdate)
	require.Len(t, updates, 1)
	assert.Equal(t, "denied", updates[0].Outcome)
	assert.Equal(t, "update", updates[0].Details["action"])

	// 跨 scope 流转 → denied 且带目标状态（便于回查操作意图）。
	_, err = s.TransitionIncident(foreign, created.ID, &IncidentStatusRequest{Status: "acknowledged"})
	require.Error(t, err)
	transitions := aud049IncidentRecords(t, store, audit.EventIncidentTransition)
	require.Len(t, transitions, 1)
	assert.Equal(t, "denied", transitions[0].Outcome)
	assert.Equal(t, "acknowledged", transitions[0].Details["to"])

	// 同 scope 流转 → success，"to" 记录目标状态（两侧均为请求态字符串，
	// 按 outcome 子集断言，不依赖 List 的随机遍历顺序）。
	matching := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "game-a", Env: "prod"})
	_, err = s.TransitionIncident(matching, created.ID, &IncidentStatusRequest{Status: "acknowledged"})
	require.NoError(t, err)
	transitions = aud049IncidentRecords(t, store, audit.EventIncidentTransition)
	require.Len(t, transitions, 2)
	deniedTransitions := aud049ByOutcome(transitions, "denied")
	require.Len(t, deniedTransitions, 1)
	assert.Equal(t, "acknowledged", deniedTransitions[0].Details["to"])
	okTransitions := aud049ByOutcome(transitions, "success")
	require.Len(t, okTransitions, 1)
	assert.Equal(t, "acknowledged", okTransitions[0].Details["to"])
	assert.Equal(t, "game-a", okTransitions[0].Details["gameId"])
}
