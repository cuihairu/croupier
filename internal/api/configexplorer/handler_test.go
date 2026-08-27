package configexplorer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/configsource"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var explorerSeq uint64

func newTestEnv(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	name := fmt.Sprintf("explorer_%d", atomic.AddUint64(&explorerSeq, 1))
	db, err := gorm.Open(gsqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))

	versionModel := model.NewConfigVersionModel(db)
	configsource.SetCroupierVersionModel(versionModel)
	svcCtx := &svc.ServiceContext{
		ConfigSourceBindingModel: model.NewConfigSourceBindingModel(db),
		ConfigVersionModel:       versionModel,
		AuditService:             audit.NewAuditService(audit.NewInMemoryAuditStore(), nil),
	}
	h := NewHandler(NewService(svcCtx))

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), "username", "tester")
		c.Request = c.Request.WithContext(ctx)
	})
	g := r.Group("/api/v1/config-explorer")
	g.GET("/sources", h.ListBindings)
	g.POST("/sources", h.UpsertBinding)
	g.DELETE("/sources/:id", h.DeleteBinding)
	g.GET("/tree", h.List)
	g.GET("/file", h.Read)
	g.PUT("/file", h.Write)
	return r, db
}

func doJSON(r *gin.Engine, method, target, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func doReq(r *gin.Engine, method, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec
}

func seedBinding(t *testing.T, db *gorm.DB, gameID, env, name, typ, config string) uint {
	t.Helper()
	b := &model.ConfigSourceBinding{GameID: gameID, Env: env, Name: name, Type: typ, Config: config}
	require.NoError(t, db.Create(b).Error)
	return b.ID
}

type bindingItems struct {
	Items []BindingDTO `json:"items"`
}

func TestListBindings_Empty(t *testing.T) {
	r, _ := newTestEnv(t)
	rec := doReq(r, http.MethodGet, "/api/v1/config-explorer/sources?gameId=demo&env=prod")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp bindingItems
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Empty(t, resp.Items)
}

func TestListBindings_MasksSecrets(t *testing.T) {
	r, db := newTestEnv(t)
	seedBinding(t, db, "demo", "prod", "redis-src", model.ConfigSourceTypeRedis,
		`{"addr":"1.2.3.4:6379","password":"hunter2","dsn":"user:secret@tcp(10.0.0.2)/cfg","prefix":"cfg:"}`)
	seedBinding(t, db, "other", "prod", "别的游戏", model.ConfigSourceTypeGit, `{}`)

	rec := doReq(r, http.MethodGet, "/api/v1/config-explorer/sources?gameId=demo&env=prod")
	require.Equal(t, http.StatusOK, rec.Code)
	var resp bindingItems
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Items, 1)

	cfg := map[string]string{}
	require.NoError(t, json.Unmarshal([]byte(resp.Items[0].Config), &cfg))
	assert.Equal(t, "******", cfg["password"])
	assert.Equal(t, "user:******@tcp(10.0.0.2)/cfg", cfg["dsn"])
	assert.Equal(t, "cfg:", cfg["prefix"])
	assert.True(t, resp.Items[0].Writable)
	assert.NotEmpty(t, resp.Items[0].CreatedAt)
}

func TestUpsertBinding_CreateAndUpdate_MaskedRoundTrip(t *testing.T) {
	r, db := newTestEnv(t)

	// 创建。
	rec := doJSON(r, http.MethodPost, "/api/v1/config-explorer/sources",
		`{"gameId":"demo","env":"prod","name":"redis-src","type":"redis","config":"{\"addr\":\"1.2.3.4:6379\",\"password\":\"hunter2\",\"dsn\":\"user:secret@tcp(10.0.0.2)/cfg\"}"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// 读取脱敏后的 config，改 addr 与 DSN host 后写回。
	var resp bindingItems
	rec = doReq(r, http.MethodGet, "/api/v1/config-explorer/sources?gameId=demo&env=prod")
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Items, 1)
	id := resp.Items[0].ID

	update := fmt.Sprintf(`{"id":%d,"gameId":"hacked","env":"hacked","name":"redis-renamed","type":"redis","config":"{\"addr\":\"5.6.7.8:6379\",\"password\":\"******\",\"dsn\":\"user:******@tcp(10.0.0.9)/cfg\"}"}`, id)
	rec = doJSON(r, http.MethodPost, "/api/v1/config-explorer/sources", update)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// DB 侧：脱敏占位被还原为旧凭据；game/env 不可改；其余字段更新。
	var stored model.ConfigSourceBinding
	require.NoError(t, db.First(&stored, id).Error)
	assert.Equal(t, "demo", stored.GameID)
	assert.Equal(t, "prod", stored.Env)
	assert.Equal(t, "redis-renamed", stored.Name)
	cfg := map[string]string{}
	require.NoError(t, json.Unmarshal([]byte(stored.Config), &cfg))
	assert.Equal(t, "hunter2", cfg["password"])
	assert.Equal(t, "user:secret@tcp(10.0.0.9)/cfg", cfg["dsn"])
	assert.Equal(t, "5.6.7.8:6379", cfg["addr"])
}

func TestUpsertBinding_Errors(t *testing.T) {
	r, _ := newTestEnv(t)

	// 非法类型。
	rec := doJSON(r, http.MethodPost, "/api/v1/config-explorer/sources",
		`{"gameId":"demo","env":"prod","name":"bad","type":"ftp","config":"{}"}`)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid config source type")

	// 缺字段。
	rec = doJSON(r, http.MethodPost, "/api/v1/config-explorer/sources",
		`{"gameId":"demo","env":"prod","type":"redis"}`)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "gameId/env/name required")

	// 空 body。
	rec = doJSON(r, http.MethodPost, "/api/v1/config-explorer/sources", "")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDeleteBinding(t *testing.T) {
	r, db := newTestEnv(t)
	id := seedBinding(t, db, "demo", "prod", "redis-src", model.ConfigSourceTypeRedis, `{}`)

	rec := doReq(r, http.MethodDelete, fmt.Sprintf("/api/v1/config-explorer/sources/%d", id))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"ok":true}`, rec.Body.String())

	var total int64
	require.NoError(t, db.Model(&model.ConfigSourceBinding{}).Count(&total).Error)
	assert.Equal(t, int64(0), total)

	rec = doReq(r, http.MethodDelete, "/api/v1/config-explorer/sources/abc")
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	rec = doReq(r, http.MethodDelete, "/api/v1/config-explorer/sources/99999")
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestTreeAndRead_CroupierSource(t *testing.T) {
	r, db := newTestEnv(t)
	ctx := context.Background()
	versionModel := model.NewConfigVersionModel(db)
	_, err := versionModel.CreateWithMeta(ctx, model.ConfigVersionPayload{
		Key: "shop.items", Content: `{"gold":100}`, Format: "json",
		GameID: "demo", Env: "prod", Namespace: model.ConfigNamespaceGameplay,
	}, "tester")
	require.NoError(t, err)
	_, err = versionModel.CreateWithMeta(ctx, model.ConfigVersionPayload{
		Key: "flags.maintenance", Content: "false", Format: "json",
		GameID: "demo", Env: "prod", Namespace: model.ConfigNamespaceRuntime,
	}, "tester")
	require.NoError(t, err)
	id := seedBinding(t, db, "demo", "prod", "croupier-src", model.ConfigSourceTypeCroupier, `{}`)

	// 根目录 = namespace。
	rec := doReq(r, http.MethodGet, fmt.Sprintf("/api/v1/config-explorer/tree?sourceId=%d", id))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var tree struct {
		Items []EntryDTO `json:"items"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &tree))
	require.Len(t, tree.Items, 2)
	assert.Equal(t, "gameplay", tree.Items[0].Name)
	assert.True(t, tree.Items[0].Dir)
	assert.Equal(t, "runtime", tree.Items[1].Name)

	// 子目录 = key 文件。
	rec = doReq(r, http.MethodGet, fmt.Sprintf("/api/v1/config-explorer/tree?sourceId=%d&dir=gameplay", id))
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &tree))
	require.Len(t, tree.Items, 1)
	assert.Equal(t, "shop.items.json", tree.Items[0].Name)
	assert.Equal(t, "gameplay/shop.items.json", tree.Items[0].Path)
	assert.NotEmpty(t, tree.Items[0].ModTime)

	// 读取：文本直出。
	rec = doReq(r, http.MethodGet, fmt.Sprintf("/api/v1/config-explorer/file?sourceId=%d&path=gameplay/shop.items.json", id))
	require.Equal(t, http.StatusOK, rec.Code)
	var file FileResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &file))
	assert.Equal(t, "json", file.Format)
	assert.Equal(t, `{"gold":100}`, file.Text)
	assert.Empty(t, file.Base64)
	assert.True(t, file.Writable)
	assert.Equal(t, int64(len(`{"gold":100}`)), file.Size)

	// 不存在的 key。
	rec = doReq(r, http.MethodGet, fmt.Sprintf("/api/v1/config-explorer/file?sourceId=%d&path=gameplay/nope.json", id))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "config not found")

	// 非法 sourceId。
	rec = doReq(r, http.MethodGet, "/api/v1/config-explorer/tree?sourceId=abc")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	rec = doReq(r, http.MethodGet, "/api/v1/config-explorer/file?sourceId=0")
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// 不存在的 source。
	rec = doReq(r, http.MethodGet, "/api/v1/config-explorer/tree?sourceId=99999")
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "source not found")
}

func TestWrite_CroupierSource_BumpsVersion(t *testing.T) {
	r, db := newTestEnv(t)
	ctx := context.Background()
	versionModel := model.NewConfigVersionModel(db)
	created, err := versionModel.CreateWithMeta(ctx, model.ConfigVersionPayload{
		Key: "shop.items", Content: `{"gold":100}`, Format: "json",
		GameID: "demo", Env: "prod", Namespace: model.ConfigNamespaceGameplay,
	}, "tester")
	require.NoError(t, err)
	id := seedBinding(t, db, "demo", "prod", "croupier-src", model.ConfigSourceTypeCroupier, `{}`)

	rec := doJSON(r, http.MethodPut, "/api/v1/config-explorer/file",
		fmt.Sprintf(`{"sourceId":%d,"path":"gameplay/shop.items.json","content":"{\"gold\":200}","reason":"应急调价"}`, id))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"ok":true}`, rec.Body.String())

	// 新版本落库，内容生效。
	var versions []model.ConfigVersion
	require.NoError(t, db.Where("key = ?", "shop.items").Order("version ASC").Find(&versions).Error)
	require.Len(t, versions, 2)
	assert.Equal(t, created.Version+1, versions[1].Version)
	assert.Equal(t, `{"gold":200}`, versions[1].Value)
	assert.Equal(t, "应急调价", versions[1].Message)
	assert.Equal(t, "config-explorer", versions[1].CreatedBy)

	// 读回新内容。
	rec = doReq(r, http.MethodGet, fmt.Sprintf("/api/v1/config-explorer/file?sourceId=%d&path=gameplay/shop.items.json", id))
	require.Equal(t, http.StatusOK, rec.Code)
	var after FileResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &after))
	assert.Equal(t, `{"gold":200}`, after.Text)
}

func TestWrite_ReadOnlySourceRejected(t *testing.T) {
	r, db := newTestEnv(t)
	id := seedBinding(t, db, "demo", "prod", "git-src", model.ConfigSourceTypeGit,
		`{"repoUrl":"https://example.com/repo.git"}`)

	rec := doJSON(r, http.MethodPut, "/api/v1/config-explorer/file",
		fmt.Sprintf(`{"sourceId":%d,"path":"conf/app.yaml","content":"x: 1","reason":"应急"}`, id))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "read-only")
}

func TestWrite_Validation(t *testing.T) {
	r, db := newTestEnv(t)
	id := seedBinding(t, db, "demo", "prod", "croupier-src", model.ConfigSourceTypeCroupier, `{}`)

	// 缺 reason。
	rec := doJSON(r, http.MethodPut, "/api/v1/config-explorer/file",
		fmt.Sprintf(`{"sourceId":%d,"path":"gameplay/k.json","content":"1"}`, id))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "reason required")

	// 缺 path。
	rec = doJSON(r, http.MethodPut, "/api/v1/config-explorer/file",
		fmt.Sprintf(`{"sourceId":%d,"reason":"x"}`, id))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "sourceId/path required")
}
