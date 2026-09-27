// MetaOptions / SdkStats handler 层回归（覆盖率巡检：此前 MetaOptions
// handler 0%、service 75%）。
//
// 已知不可达的防御分支（不强行造路径）：① SdkStats 的 ShouldBindQuery
// 错误分支——请求结构体只有 string 字段，HTTP 层无法触发绑定失败；
// ② service.MetaOptions 的 items==nil→[] 归一——store 内存/DB 两条聚合
// 路径均恒返非空切片（groupMetaOptions 恒 make），nil 仅来自 nil store
// （该情形 service 已提前报错）。
package provider

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
)

// metaOptionsBody 是 GET /api/v1/providers/meta-options 的响应体。
type metaOptionsBody struct {
	Items []registry.ProviderMetaKeyOption `json:"items"`
}

// seedMetaProvider 向 DB-less registry 写入一个带元数据的 provider 注册。
func seedMetaProvider(t *testing.T, store *registry.Store, agentID, gameID, env, providerID string, metadata map[string]string) {
	t.Helper()
	err := store.UpsertAgent(&registry.AgentSession{
		AgentID: agentID,
		GameID:  gameID,
		Env:     env,
		Providers: []registry.ProviderSession{
			{ProviderID: providerID, Metadata: metadata},
		},
	})
	if err != nil {
		t.Fatalf("UpsertAgent(%s): %v", agentID, err)
	}
}

// MetaOptions HTTP 层（#2/#11 覆盖缺口，此前 0%）：scoped 聚合返回 200 +
// 去重键值；其他 scope 的键值不得泄漏；无数据时 items 为 [] 而非 null。
func TestMetaOptionsHandlerReturnsScopedAggregation(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	store := registry.NewStore()
	seedMetaProvider(t, store, "agent-1", "demo", "dev", "svc-1",
		map[string]string{"serverId": "s-1"})
	seedMetaProvider(t, store, "agent-2", "demo", "dev", "svc-2",
		map[string]string{"serverId": "s-2", "region": "cn-north"})
	// 其他 scope：不得泄漏进 demo/dev 聚合
	seedMetaProvider(t, store, "agent-3", "other", "prod", "svc-3",
		map[string]string{"serverId": "s-other"})

	h := NewHandler(NewService(&svc.ServiceContext{RegistryStore: store}))
	ctx, rec := newProviderTestContext(http.MethodGet, "/api/v1/providers/meta-options", "")
	req := ctx.Request.WithContext(svc.WithGameScope(
		ctx.Request.Context(), svc.GameScope{GameID: "demo", Env: "dev"}))
	ctx.Request = req

	h.MetaOptions(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body metaOptionsBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	byKey := map[string]map[string]int{}
	for _, key := range body.Items {
		byKey[key.Key] = map[string]int{}
		for _, v := range key.Values {
			byKey[key.Key][v.Value] = v.Count
		}
	}
	serverID, ok := byKey["serverId"]
	if !ok {
		t.Fatalf("missing serverId key, got %v", body.Items)
	}
	if serverID["s-1"] != 1 || serverID["s-2"] != 1 {
		t.Fatalf("serverId values = %v, want s-1:1 s-2:1", serverID)
	}
	if _, leak := byKey["region"]; !leak {
		// region 键只在 svc-2 上，demo/dev scope 内应存在
		t.Fatalf("missing region key in scope aggregation, got %v", body.Items)
	}
	for _, key := range body.Items {
		for _, v := range key.Values {
			if v.Value == "s-other" {
				t.Fatalf("other-scope value s-other leaked: %v", body.Items)
			}
		}
	}
}

// scope 无任何实例元数据时，聚合为空数组（service 的 nil→[] 归一分支）。
func TestMetaOptionsHandlerEmptyScopeYieldsEmptyItems(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{RegistryStore: registry.NewStore()}))
	ctx, rec := newProviderTestContext(http.MethodGet, "/api/v1/providers/meta-options", "")
	ctx.Request = ctx.Request.WithContext(svc.WithGameScope(
		ctx.Request.Context(), svc.GameScope{GameID: "empty", Env: "dev"}))

	h.MetaOptions(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body metaOptionsBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Items == nil || len(body.Items) != 0 {
		t.Fatalf("items = %v, want empty non-nil slice", body.Items)
	}
}

// ctx 无 scope（内部直调）：保持全量行为（service 既有语义），仍 200。
func TestMetaOptionsHandlerWithoutScopeStillSucceeds(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	store := registry.NewStore()
	seedMetaProvider(t, store, "agent-1", "demo", "dev", "svc-1",
		map[string]string{"serverId": "s-1"})

	h := NewHandler(NewService(&svc.ServiceContext{RegistryStore: store}))
	ctx, rec := newProviderTestContext(http.MethodGet, "/api/v1/providers/meta-options", "")

	h.MetaOptions(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body metaOptionsBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Items) == 0 {
		t.Fatalf("items = %v, want unscoped full aggregation", body.Items)
	}
}

// ensureRegistryStore 在 nil store 时报错：handler 走 response.Error。
// 注：不加 t.Parallel，与 store 相关的全局 mock 隔离。
func TestMetaOptionsHandlerNilStoreReturnsError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}))
	ctx, rec := newProviderTestContext(http.MethodGet, "/api/v1/providers/meta-options", "")

	h.MetaOptions(ctx)

	if rec.Code == http.StatusOK {
		t.Fatalf("nil store must not succeed, body = %s", rec.Body.String())
	}
	var errBody struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &errBody); err != nil || errBody.Error == "" {
		t.Fatalf("body = %s, want unified error object", rec.Body.String())
	}
}

// SdkStats handler 的 service 错误分支（nil store → response.Error）。
func TestSdkStatsHandlerNilStoreReturnsError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}))
	ctx, rec := newProviderTestContext(http.MethodGet, "/api/v1/providers/sdk-stats", "")

	h.SdkStats(ctx)

	if rec.Code == http.StatusOK {
		t.Fatalf("nil store must not succeed, body = %s", rec.Body.String())
	}
}

// 空库 + 无 scope：store 返回 nil 时的归一分支（items=[]）。
func TestMetaOptionsHandlerEmptyUnscopedYieldsEmptyItems(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{RegistryStore: registry.NewStore()}))
	ctx, rec := newProviderTestContext(http.MethodGet, "/api/v1/providers/meta-options", "")

	h.MetaOptions(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body metaOptionsBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Items == nil || len(body.Items) != 0 {
		t.Fatalf("items = %v, want empty non-nil slice", body.Items)
	}
}
