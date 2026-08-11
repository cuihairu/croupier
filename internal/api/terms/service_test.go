package terms

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var termsServiceDBSeq uint64

func newTermsServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := fmt.Sprintf("terms_svc_%d", atomic.AddUint64(&termsServiceDBSeq, 1))
	db, err := gorm.Open(gsqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	return db
}

func newTermsService(db *gorm.DB) *Service {
	svcCtx := &svc.ServiceContext{TermDictModel: model.NewTermDictionaryModel(db)}
	return NewService(svcCtx)
}

func TestService_List_NilRequest(t *testing.T) {
	db := newTermsServiceTestDB(t)
	svc := newTermsService(db)

	resp, err := svc.List(context.Background(), nil)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Empty(t, resp.Items)
}

func TestService_List_Empty(t *testing.T) {
	db := newTermsServiceTestDB(t)
	svc := newTermsService(db)

	resp, err := svc.List(context.Background(), &TermsListRequest{Domain: "resource"})
	require.NoError(t, err)
	assert.Empty(t, resp.Items)
}

func TestService_List_InvalidDomain(t *testing.T) {
	db := newTermsServiceTestDB(t)
	svc := newTermsService(db)

	_, err := svc.List(context.Background(), &TermsListRequest{Domain: "entity"})
	assert.Error(t, err)
}

func TestService_Upsert_NilRequest(t *testing.T) {
	db := newTermsServiceTestDB(t)
	svc := newTermsService(db)

	_, err := svc.Upsert(context.Background(), nil)
	assert.Error(t, err)
}

func TestService_Upsert_InvalidDomain(t *testing.T) {
	db := newTermsServiceTestDB(t)
	svc := newTermsService(db)

	_, err := svc.Upsert(context.Background(), &TermUpsertRequest{Domain: "entity"})
	assert.Error(t, err)
}

func TestService_Upsert_Success(t *testing.T) {
	db := newTermsServiceTestDB(t)
	svc := newTermsService(db)

	resp, err := svc.Upsert(context.Background(), &TermUpsertRequest{
		Domain:    "resource",
		TermKey:   "player",
		Alias:     "玩家",
		DisplayZh: "玩家",
		DisplayEn: "Player",
		Order:     1,
	})
	require.NoError(t, err)
	assert.True(t, resp.Ok)
}

func TestService_Delete_NilRequest(t *testing.T) {
	db := newTermsServiceTestDB(t)
	svc := newTermsService(db)

	_, err := svc.Delete(context.Background(), nil)
	assert.Error(t, err)
}

func TestService_Delete_InvalidDomain(t *testing.T) {
	db := newTermsServiceTestDB(t)
	svc := newTermsService(db)

	_, err := svc.Delete(context.Background(), &TermDeleteRequest{Domain: "entity", Alias: "test"})
	assert.Error(t, err)
}

func TestService_Delete_Success(t *testing.T) {
	db := newTermsServiceTestDB(t)
	svc := newTermsService(db)

	// Create a term first
	_, err := svc.Upsert(context.Background(), &TermUpsertRequest{
		Domain:  "resource",
		TermKey: "player",
		Alias:   "玩家",
	})
	require.NoError(t, err)

	// Delete it
	resp, err := svc.Delete(context.Background(), &TermDeleteRequest{
		Domain: "resource",
		Alias:  "玩家",
	})
	require.NoError(t, err)
	assert.True(t, resp.Ok)
}

func TestService_Upsert_UpdateExisting(t *testing.T) {
	db := newTermsServiceTestDB(t)
	svc := newTermsService(db)

	// Create
	_, err := svc.Upsert(context.Background(), &TermUpsertRequest{
		Domain:  "resource",
		TermKey: "player",
		Alias:   "玩家",
	})
	require.NoError(t, err)

	// Update
	resp, err := svc.Upsert(context.Background(), &TermUpsertRequest{
		Domain:    "resource",
		TermKey:   "player",
		Alias:     "玩家",
		DisplayZh: "游戏玩家",
		DisplayEn: "Game Player",
	})
	require.NoError(t, err)
	assert.True(t, resp.Ok)

	// Verify update
	listResp, err := svc.List(context.Background(), &TermsListRequest{Domain: "resource"})
	require.NoError(t, err)
	require.Len(t, listResp.Items, 1)
	assert.Equal(t, "游戏玩家", listResp.Items[0].DisplayZh)
	assert.Equal(t, "Game Player", listResp.Items[0].DisplayEn)
}
