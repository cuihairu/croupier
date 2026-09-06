package platform

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/core/extension/installation"
	"github.com/cuihairu/croupier/internal/model"
	dispatch "github.com/cuihairu/croupier/internal/platform/dispatch"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	extensiongorm "github.com/cuihairu/croupier/internal/repo/gorm/extension"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/cuihairu/croupier/internal/transport"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

// ---- fake session plumbing：让 Dispatcher.Invoke 走通成功路径 ----

type v8SessionCaller struct {
	mu       sync.Mutex
	respBody []byte
	err      error
}

func (f *v8SessionCaller) Call(ctx context.Context, msgID uint32, reqBody []byte) (uint32, []byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return 0, nil, f.err
	}
	return msgID, f.respBody, nil
}

type v8SessionResolver struct {
	callers map[string]transport.SessionCaller
}

func (f *v8SessionResolver) ResolveAgentConn(agentID string) (transport.SessionCaller, bool) {
	caller, ok := f.callers[agentID]
	return caller, ok
}

func newInvokeSuccessDispatcher(t *testing.T, payload []byte) *dispatch.Dispatcher {
	t.Helper()
	store := reg.NewStore()
	require.NoError(t, store.UpsertAgent(&reg.AgentSession{
		AgentID:  "v8-agent",
		ExpireAt: time.Now().Add(time.Hour),
		Functions: map[string]reg.FunctionMeta{
			"external.v8plat.v8method": {Enabled: true},
		},
	}))
	d := dispatch.NewDispatcher(store)
	respBody, err := proto.Marshal(&sdkv1.InvokeResponse{Payload: payload})
	require.NoError(t, err)
	d.SetSessionResolver(&v8SessionResolver{callers: map[string]transport.SessionCaller{
		"v8-agent": &v8SessionCaller{respBody: respBody},
	}})
	return d
}

// Service.Call：Dispatcher.Invoke 成功且返回合法 JSON → Response 为解析结果
func TestServiceCallDispatcherSuccessJSON_V8(t *testing.T) {
	d := newInvokeSuccessDispatcher(t, []byte(`{"ok":true,"count":3}`))
	service := NewService(&svc.ServiceContext{Dispatcher: d})
	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "v8plat",
		Method:   "v8method",
		Request:  `{"q":"1"}`,
	})
	require.NoError(t, err)
	assert.Equal(t, 200, resp.Code)
	assert.Equal(t, "extension", resp.Source)
	m, ok := resp.Response.(map[string]interface{})
	require.True(t, ok, "response should be a JSON object, got %T", resp.Response)
	assert.Equal(t, true, m["ok"])
	assert.Equal(t, float64(3), m["count"])
}

// Service.Call：Dispatcher.Invoke 成功且返回空 payload → Response 为 nil
func TestServiceCallDispatcherSuccessEmptyPayload_V8(t *testing.T) {
	d := newInvokeSuccessDispatcher(t, nil)
	service := NewService(&svc.ServiceContext{Dispatcher: d})
	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "v8plat",
		Method:   "v8method",
	})
	require.NoError(t, err)
	assert.Equal(t, 200, resp.Code)
	assert.Equal(t, "extension", resp.Source)
	assert.Nil(t, resp.Response)
}

// Service.Call：Dispatcher.Invoke 成功但 payload 非法 JSON → 回退为字符串
func TestServiceCallDispatcherSuccessNonJSONPayload_V8(t *testing.T) {
	d := newInvokeSuccessDispatcher(t, []byte("not-json"))
	service := NewService(&svc.ServiceContext{Dispatcher: d})
	resp, err := service.Call(context.Background(), &CallPlatformRequest{
		Platform: "v8plat",
		Method:   "v8method",
	})
	require.NoError(t, err)
	assert.Equal(t, 200, resp.Code)
	assert.Equal(t, "not-json", resp.Response)
}

// Handler.Call：Dispatcher.Invoke 成功 → writeCallResponse 成功分支输出 payload
func TestHandlerCallDispatcherSuccess_V8(t *testing.T) {
	gin.SetMode(gin.TestMode)
	d := newInvokeSuccessDispatcher(t, []byte(`{"ok":true}`))
	service := NewService(&svc.ServiceContext{Dispatcher: d})
	r := gin.New()
	h := NewHandler(service)
	r.POST("/api/v1/platform/call", h.Call)

	body := []byte(`{"platform":"v8plat","method":"v8method","request":"{}"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/platform/call", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"ok":true`)
	assert.Contains(t, rec.Body.String(), `"source":"extension"`)
}

// writeCallResponse：resp.Code >= 400 → CodeError 分支
func TestWriteCallResponseErrorCode_V8(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	writeCallResponse(c, &CallPlatformResponse{Code: 502, Message: "bad gateway"})
	assert.Equal(t, http.StatusBadGateway, rec.Code)
}

// writeListPlatformsResponse：resp.Code >= 400 → CodeError 分支
func TestWriteListPlatformsResponseErrorCode_V8(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	writeListPlatformsResponse(c, &ListPlatformsResponse{Code: 500, Message: "boom"})
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// writeListMethodsResponse：resp 为 nil → 500
func TestWriteListMethodsResponseNil_V8(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	writeListMethodsResponse(c, nil)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// discoverExternalPlatforms：agents map 中存在 nil session → 跳过
func TestDiscoverExternalPlatformsNilAgentEntry_V8(t *testing.T) {
	store := reg.NewStore()
	require.NoError(t, store.UpsertAgent(&reg.AgentSession{
		AgentID:  "live-agent",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.v8.m1": {Enabled: true},
		},
	}))
	store.Mu().Lock()
	store.AgentsUnsafe()["nil-agent"] = nil
	store.Mu().Unlock()

	service := NewService(&svc.ServiceContext{RegistryStore: store})
	got := service.discoverExternalPlatforms(context.Background())
	assert.Len(t, got["v8"], 1)
}

// ListMethods：EqualFold 判定不同但 ToLower 归一相同（U+0130 'İ' → 'i'）
// 的重复方法名触发 addMethods 的 merged 去重分支
func TestListMethodsFoldCaseDedup_V8(t *testing.T) {
	store := reg.NewStore()
	require.NoError(t, store.UpsertAgent(&reg.AgentSession{
		AgentID:  "a1",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.v8.alpha": {Enabled: true},
			"external.v8.İ":     {Enabled: true}, // ToLower → "i"
			"external.v8.i":     {Enabled: true},
		},
	}))
	service := NewService(&svc.ServiceContext{RegistryStore: store})
	resp, err := service.ListMethods(context.Background(), "v8")
	require.NoError(t, err)
	assert.Equal(t, 200, resp.Code)
	// alpha + (i/İ 去重后 1 个) = 2
	assert.Len(t, resp.Methods, 2)
}

// ---- installation bindings 发现路径 ----

type v8InstallFixture struct {
	db       *gorm.DB
	service  *Service
	injectFn func()
}

func newV8InstallFixture(t *testing.T) *v8InstallFixture {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open("file:v8mem?mode=memory"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ExtensionInstallation{}, &model.ExtensionRuntimeBinding{}))

	installationRepo := extensiongorm.NewInstallationRepo(db)
	bindingRepo := extensiongorm.NewBindingRepo(db)
	eventRepo := extensiongorm.NewEventRepo(db)
	installationService := installation.NewService(installationRepo, eventRepo, bindingRepo)

	svcCtx := &svc.ServiceContext{
		Extensions: &svc.ExtensionServices{Installation: installationService},
	}
	return &v8InstallFixture{db: db, service: NewService(svcCtx)}
}

func (f *v8InstallFixture) createInstallation(t *testing.T, key, extensionID, status, desiredState string) *model.ExtensionInstallation {
	t.Helper()
	item := &model.ExtensionInstallation{
		InstallationKey: key,
		ExtensionID:     extensionID,
		ReleaseVersion:  "1.0.0",
		ScopeType:       "global",
		ScopeID:         "default",
		TargetType:      "server",
		TargetID:        "default",
		Status:          status,
		DesiredState:    desiredState,
		Enabled:         true,
		InstalledBy:     "test",
		InstalledAtUnix: time.Now().Unix(),
	}
	require.NoError(t, f.db.Create(item).Error)
	return item
}

func (f *v8InstallFixture) createBinding(t *testing.T, installationID uint, provider string) {
	t.Helper()
	require.NoError(t, f.db.Create(&model.ExtensionRuntimeBinding{
		InstallationID: installationID,
		BindingType:    "provider",
		BindingKey:     provider,
		SpecJSON:       model.JSON([]byte(`{"provider":"` + provider + `","operations":["do_work"]}`)),
		Status:         "active",
	}).Error)
}

// 非 external-platform 扩展安装记录被跳过（IsExternalPlatformExtensionID false）
func TestDiscoverSkipsNonExternalExtension_V8(t *testing.T) {
	f := newV8InstallFixture(t)
	item := f.createInstallation(t, "plain-ext", "plain-ext", "installed", "installed")
	f.createBinding(t, item.ID, "plainprovider")

	got := f.service.discoverExternalPlatforms(context.Background())
	assert.NotContains(t, got, "plainprovider")
}

// status=uninstalled / desiredState=uninstalled 均被跳过
func TestDiscoverSkipsUninstalledStatus_V8(t *testing.T) {
	f := newV8InstallFixture(t)
	byStatus := f.createInstallation(t, "uninst-1", "vendor.external-platform", "uninstalled", "installed")
	f.createBinding(t, byStatus.ID, "bystatus")
	byDesired := f.createInstallation(t, "uninst-2", "another.external-platform", "installed", "Uninstalled ")
	f.createBinding(t, byDesired.ID, "bydesired")

	got := f.service.discoverExternalPlatforms(context.Background())
	assert.NotContains(t, got, "bystatus")
	assert.NotContains(t, got, "bydesired")
}

// ListBindings 出错时跳过该安装的 bindings
func TestDiscoverSkipsBindingsError_V8(t *testing.T) {
	f := newV8InstallFixture(t)
	item := f.createInstallation(t, "ok-ext", "vendor.external-platform", "installed", "installed")
	f.createBinding(t, item.ID, "okprovider")

	require.NoError(t, f.db.Callback().Query().Before("gorm:query").Register("v8:binding_err", func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "extension_runtime_bindings" {
			_ = tx.AddError(errors.New("injected binding query failure"))
		}
	}))
	t.Cleanup(func() {
		_ = f.db.Callback().Query().Remove("v8:binding_err")
	})

	got := f.service.discoverExternalPlatforms(context.Background())
	assert.NotContains(t, got, "okprovider")
}

// 正常 external-platform 安装 + bindings 被发现并合并
func TestDiscoverIncludesExternalExtensionBindings_V8(t *testing.T) {
	f := newV8InstallFixture(t)
	item := f.createInstallation(t, "ok-ext", "vendor.external-platform", "installed", "installed")
	f.createBinding(t, item.ID, "vendorplat")

	got := f.service.discoverExternalPlatforms(context.Background())
	assert.Equal(t, []string{"do_work"}, got["vendorplat"])
}
