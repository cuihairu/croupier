package cicd

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// aud049WebhookEnv assembles a webhook test environment with an in-memory audit store
// (AUD-049).
func aud049WebhookEnv(t *testing.T) (*Service, *audit.InMemoryAuditStore, uint) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1)
	}
	require.NoError(t, db.AutoMigrate(&model.CicdIntegration{}, &model.CicdBuild{}))
	store := audit.NewInMemoryAuditStore()
	svcCtx := &svc.ServiceContext{
		DB:                   db,
		CicdIntegrationModel: model.NewCicdIntegrationModel(db),
		CicdBuildModel:       model.NewCicdBuildModel(db),
		AuditService:         audit.NewAuditService(store, nil),
	}
	row := &model.CicdIntegration{
		GameID: "demo", Env: "prod", Kind: "generic", Name: "aud049-ci",
		Endpoint: "https://ci.example.com", Token: "aud049-token", Enabled: true,
	}
	require.NoError(t, db.Create(row).Error)
	return NewService(svcCtx), store, row.ID
}

func aud049WebhookRecords(t *testing.T, store *audit.InMemoryAuditStore, event audit.AuditEventType) []*audit.AuditRecord {
	t.Helper()
	records, _, err := store.List(audit.AuditFilter{EventType: []audit.AuditEventType{event}}, audit.AuditPage{Page: 1, PageSize: 50})
	require.NoError(t, err)
	return records
}

// InMemoryAuditStore.List is backed by a map (random iteration order), so denied records
// are asserted as subsets by reason, without depending on order.
func aud049WebhookByReason(records []*audit.AuditRecord, reason string) []*audit.AuditRecord {
	var out []*audit.AuditRecord
	for _, r := range records {
		if got, _ := r.Details["reason"].(string); got == reason {
			out = append(out, r)
		}
	}
	return out
}

// Successful write-back -> cicd.webhook_ingest / success; token mismatch -> denied;
// scope conflict -> denied; all three records share one exit (LogWrite), distinguished
// by outcome.
func TestAud049_WebhookAuditSuccessAndDenied(t *testing.T) {
	s, store, id := aud049WebhookEnv(t)
	ctx := context.Background()

	resp, err := s.IngestWebhook(ctx, id, "aud049-token", &WebhookPayload{
		Pipeline: "deploy", ExternalID: "b-1", Status: "success",
	})
	require.NoError(t, err)
	assert.Equal(t, "success", resp.Build.Status)

	ingest := aud049WebhookRecords(t, store, audit.EventCicdWebhookIngest)
	require.Len(t, ingest, 1)
	assert.Equal(t, "success", ingest[0].Outcome)
	assert.Equal(t, "cicd-webhook", ingest[0].Actor.ID)
	assert.Equal(t, "cicd_build", ingest[0].Resource.Type)
	assert.Equal(t, "deploy", ingest[0].Details["pipeline"])

	_, err = s.IngestWebhook(ctx, id, "wrong-token", &WebhookPayload{Pipeline: "deploy", ExternalID: "b-2", Status: "success"})
	require.Error(t, err)

	// Scope injection: the payload declares a scope the integration did not configure
	// -> rejected and recorded.
	_, err = s.IngestWebhook(ctx, id, "aud049-token", &WebhookPayload{Pipeline: "deploy", ExternalID: "b-3", Status: "success", GameID: "other-game"})
	require.Error(t, err)

	denied := aud049WebhookRecords(t, store, audit.EventCicdWebhookDenied)
	require.Len(t, denied, 2)
	tokenDenied := aud049WebhookByReason(denied, "token_mismatch")
	require.Len(t, tokenDenied, 1)
	assert.Equal(t, "denied", tokenDenied[0].Outcome)
	scopeDenied := aud049WebhookByReason(denied, "scope_conflict")
	require.Len(t, scopeDenied, 1)
	assert.Equal(t, "denied", scopeDenied[0].Outcome)
}
