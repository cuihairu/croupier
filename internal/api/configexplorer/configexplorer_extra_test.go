// 覆盖目标：configexplorer 包 handler 的 ListBindings/Write 错误分支、
// service 的 UpsertBinding nil 请求与 Get/Update/Delete 失败路径、
// source() 适配器构建失败、Read 二进制格式 base64 输出、
// mergeMaskedConfig 非法 JSON 回退等未覆盖路径。
package configexplorer

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/configsource"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var extraSvcCtxSeq uint64

// newTestEnvServiceContext 构造仅含配置源依赖的 ServiceContext
// （与 handler_test 的 newTestEnv 同构，供直接调 service 方法的用例使用）。
func newTestEnvServiceContext(t *testing.T) *svc.ServiceContext {
	t.Helper()
	name := fmt.Sprintf("explorer_svc_%d", atomic.AddUint64(&extraSvcCtxSeq, 1))
	db, err := gorm.Open(gsqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	versionModel := model.NewConfigVersionModel(db)
	configsource.SetCroupierVersionModel(versionModel)
	return &svc.ServiceContext{
		DB:                       db,
		ConfigSourceBindingModel: model.NewConfigSourceBindingModel(db),
		ConfigVersionModel:       versionModel,
		AuditService:             audit.NewAuditService(audit.NewInMemoryAuditStore(), nil),
	}
}

func TestListBindings_DatabaseError(t *testing.T) {
	r, db := newTestEnv(t)
	require.NoError(t, db.Migrator().DropTable(&model.ConfigSourceBinding{}))

	rec := doReq(r, http.MethodGet, "/api/v1/config-explorer/sources?gameId=demo&env=prod")
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestWrite_InvalidJSON(t *testing.T) {
	r, _ := newTestEnv(t)
	rec := doJSON(r, http.MethodPut, "/api/v1/config-explorer/file", `not-json`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestWrite_SourceNotFound(t *testing.T) {
	r, _ := newTestEnv(t)
	rec := doJSON(r, http.MethodPut, "/api/v1/config-explorer/file",
		`{"sourceId":99999,"path":"gameplay/k.json","content":"1","reason":"x"}`)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "source not found")
}

func TestUpsertBinding_NilRequest(t *testing.T) {
	s := NewService(newTestEnvServiceContext(t))
	_, err := s.UpsertBinding(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

func TestUpsertBinding_UpdateMissingRecord(t *testing.T) {
	r, _ := newTestEnv(t)
	// id>0 但记录不存在 → Get 报错（404 语义）。
	rec := doJSON(r, http.MethodPost, "/api/v1/config-explorer/sources",
		`{"id":99999,"gameId":"demo","env":"prod","name":"ghost","type":"redis","config":"{}"}`)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestUpsertBinding_UpdateFails(t *testing.T) {
	r, db := newTestEnv(t)
	id := seedBinding(t, db, "demo", "prod", "redis-src", model.ConfigSourceTypeRedis, `{}`)
	// 触发 BEFORE UPDATE 失败：Update 走 Updates，用触发器拦截。
	require.NoError(t, db.Exec("CREATE TRIGGER block_binding_update BEFORE UPDATE ON config_source_bindings BEGIN SELECT RAISE(ABORT, 'no update'); END").Error)

	rec := doJSON(r, http.MethodPost, "/api/v1/config-explorer/sources",
		`{"id":`+itoa(id)+`,"gameId":"demo","env":"prod","name":"redis-x","type":"redis","config":"{}"}`)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestDeleteBinding_SoftDeleteViaUpdate(t *testing.T) {
	r, db := newTestEnv(t)
	id := seedBinding(t, db, "demo", "prod", "redis-src", model.ConfigSourceTypeRedis, `{}`)
	// gorm 软删除走 UPDATE：BEFORE UPDATE 触发器可拦截删除路径
	require.NoError(t, db.Exec("CREATE TRIGGER block_binding_delete BEFORE UPDATE ON config_source_bindings BEGIN SELECT RAISE(ABORT, 'no update'); END").Error)

	rec := doReq(r, http.MethodDelete, "/api/v1/config-explorer/sources/"+itoa(id))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// 适配器构建失败：croupier 源 Config 非法 JSON → New 报错 → source() 失败。
func TestSource_InvalidAdapterConfig(t *testing.T) {
	svcCtx := newTestEnvServiceContext(t)
	s := NewService(svcCtx)
	b := &model.ConfigSourceBinding{GameID: "demo", Env: "prod", Name: "bad", Type: model.ConfigSourceTypeCroupier, Config: `{not-json`}
	require.NoError(t, svcCtx.DB.Create(b).Error)

	if _, err := s.List(context.Background(), b.ID, ""); err == nil {
		t.Fatal("expected adapter build error for invalid croupier config")
	}
	if _, err := s.Read(context.Background(), b.ID, "gameplay/k.json"); err == nil {
		t.Fatal("expected adapter build error on read path")
	}
}

// 二进制格式文件：Read 返回 base64 而非 text。
func TestRead_BinaryFormatBase64(t *testing.T) {
	r, db := newTestEnv(t)
	ctx := context.Background()
	versionModel := model.NewConfigVersionModel(db)
	_, err := versionModel.CreateWithMeta(ctx, model.ConfigVersionPayload{
		Key: "asset.bin", Content: "\x00\x01\x02binary", Format: "bin",
		GameID: "demo", Env: "prod", Namespace: model.ConfigNamespaceGameplay,
	}, "tester")
	require.NoError(t, err)
	id := seedBinding(t, db, "demo", "prod", "croupier-src", model.ConfigSourceTypeCroupier, `{}`)

	rec := doReq(r, http.MethodGet, "/api/v1/config-explorer/file?sourceId="+itoa(id)+"&path=gameplay/asset.bin")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"base64"`)
	assert.NotContains(t, rec.Body.String(), `"text":"`)
}

func TestMergeMaskedConfig_InvalidOldJSON(t *testing.T) {
	got := mergeMaskedConfig(`{not-json`, `{"password":"new"}`)
	assert.Equal(t, `{"password":"new"}`, got)
}

func TestMergeMaskedConfig_InvalidNewJSON(t *testing.T) {
	got := mergeMaskedConfig(`{"password":"old"}`, `{not-json`)
	assert.Equal(t, `{"password":"old"}`, got)
}

func itoa(v uint) string {
	return ui64toa(uint64(v))
}

func ui64toa(v uint64) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
