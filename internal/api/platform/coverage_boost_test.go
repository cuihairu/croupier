package platform

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 死分支实证：Service 三个方法把错误经 resp.Code 表达、恒返回 nil error，
// handler.go 中 Call/ListPlatforms/ListMethods 的 err != nil 分支不可达。
func TestService_NeverReturnsError_Matrix(t *testing.T) {
	store := reg.NewStore()
	require.NoError(t, store.UpsertAgent(&reg.AgentSession{
		AgentID:   "boost-agent",
		Addr:      "127.0.0.1:19091",
		ExpireAt:  time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{"external.boost.ping": {Enabled: true}},
	}))
	s := NewService(&svc.ServiceContext{RegistryStore: store})
	ctx := context.Background()

	callResp, err := s.Call(ctx, &CallPlatformRequest{Platform: "boost", Method: "ping"})
	assert.NoError(t, err)
	require.NotNil(t, callResp)
	assert.Equal(t, 503, callResp.Code, "no dispatcher wired -> 503 via resp.Code, not error")

	platformsResp, err := s.ListPlatforms(ctx)
	assert.NoError(t, err)
	require.NotNil(t, platformsResp)
	assert.Equal(t, 200, platformsResp.Code)
	assert.Len(t, platformsResp.Platforms, 1)
	assert.Equal(t, "boost", platformsResp.Platforms[0].Name)
	assert.True(t, platformsResp.Platforms[0].Enabled)
	assert.Equal(t, "extension", platformsResp.Platforms[0].Source)

	methodsResp, err := s.ListMethods(ctx, "boost")
	assert.NoError(t, err)
	require.NotNil(t, methodsResp)
	assert.Equal(t, 200, methodsResp.Code)
	assert.Equal(t, []string{"ping"}, methodsResp.Methods)
	assert.Equal(t, "extension", methodsResp.Source)
}

// platform 参数为纯空白 → TrimSpace 后为空 → 400。
func TestService_ListMethods_WhitespacePlatform(t *testing.T) {
	s := NewService(&svc.ServiceContext{})
	resp, err := s.ListMethods(context.Background(), "   ")
	require.NoError(t, err)
	assert.Equal(t, 400, resp.Code)
	assert.Empty(t, resp.Methods)
}

// registry 中同名方法重复注册（跨 agent）只保留一份。
func TestService_ListMethods_DedupAcrossAgents(t *testing.T) {
	store := reg.NewStore()
	for _, id := range []string{"a1", "a2"} {
		require.NoError(t, store.UpsertAgent(&reg.AgentSession{
			AgentID:  id,
			Addr:     "127.0.0.1:19091",
			ExpireAt: time.Now().Add(time.Minute),
			Functions: map[string]reg.FunctionMeta{
				"external.dedup.ping": {Enabled: true},
				"external.dedup.PING": {Enabled: true}, // ToLower 归一后重复
			},
		}))
	}
	s := NewService(&svc.ServiceContext{RegistryStore: store})
	resp, err := s.ListMethods(context.Background(), "dedup")
	require.NoError(t, err)
	assert.Equal(t, 200, resp.Code)
	assert.Len(t, resp.Methods, 1)
}

// 禁用的函数不参与方法发现。
func TestService_ListMethods_SkipsDisabledFunctions(t *testing.T) {
	store := reg.NewStore()
	require.NoError(t, store.UpsertAgent(&reg.AgentSession{
		AgentID:  "disabled-agent",
		Addr:     "127.0.0.1:19091",
		ExpireAt: time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{
			"external.dis.off":  {Enabled: false},
			"external.dis.live": {Enabled: true},
		},
	}))
	s := NewService(&svc.ServiceContext{RegistryStore: store})

	resp, err := s.ListMethods(context.Background(), "dis")
	require.NoError(t, err)
	assert.Equal(t, []string{"live"}, resp.Methods)

	platforms, err := s.ListPlatforms(context.Background())
	require.NoError(t, err)
	require.Len(t, platforms.Platforms, 1)
	assert.Equal(t, []string{"live"}, platforms.Platforms[0].Methods)
}

// 过期 agent 仍提供函数发现（registry 视图），但健康统计归零不在此路径断言。
func TestService_Call_RequestFieldStringified(t *testing.T) {
	s := NewService(&svc.ServiceContext{})
	resp, err := s.Call(context.Background(), &CallPlatformRequest{
		Platform: "p",
		Method:   "m",
		Request:  `{"a":1}`,
	})
	require.NoError(t, err)
	assert.Equal(t, 503, resp.Code)
	assert.Equal(t, "extension", resp.Source)
}

// 路由别名 List/Methods 与 ListPlatforms/ListMethods 行为一致。
func TestHandler_AliasRoutes_List_Methods(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := reg.NewStore()
	require.NoError(t, store.UpsertAgent(&reg.AgentSession{
		AgentID:   "alias-agent",
		Addr:      "127.0.0.1:19091",
		ExpireAt:  time.Now().Add(time.Minute),
		Functions: map[string]reg.FunctionMeta{"external.alias.ping": {Enabled: true}},
	}))
	h := NewHandler(NewService(&svc.ServiceContext{RegistryStore: store}))

	r := gin.New()
	r.GET("/platforms", h.List)
	r.GET("/platforms/:platform/methods", h.Methods)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/platforms", nil))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "alias")

	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/platforms/alias/methods", nil))
	assert.Equal(t, http.StatusOK, rec2.Code)
	assert.Contains(t, rec2.Body.String(), "ping")
}

// Handler.Call 请求体缺少必填字段时由 Service 层返回 400（字段校验走 resp.Code）。
func TestHandler_Call_MissingFieldsViaService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(NewService(&svc.ServiceContext{}))
	r := gin.New()
	r.POST("/call", h.Call)

	for _, body := range []string{
		`{"platform":"","method":"m"}`,
		`{"platform":"p","method":""}`,
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/call", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "body %s", body)
	}
}

// failingPlatformService 经 platformServiceAPI 缝隙注入故障：三个端点必须以
// 500 internal_error 响应并携带注入的错误消息。
type failingPlatformService struct {
	err error
}

func (f failingPlatformService) Call(_ context.Context, _ *CallPlatformRequest) (*CallPlatformResponse, error) {
	return nil, f.err
}

func (f failingPlatformService) ListPlatforms(_ context.Context) (*ListPlatformsResponse, error) {
	return nil, f.err
}

func (f failingPlatformService) ListMethods(_ context.Context, _ string) (*ListPlatformMethodsResponse, error) {
	return nil, f.err
}

func TestHandler_ServiceErrorBranches(t *testing.T) {
	gin.SetMode(gin.TestMode)
	boom := errors.New("platform service boom")
	h := &Handler{service: failingPlatformService{err: boom}}

	r := gin.New()
	r.POST("/call", h.Call)
	r.GET("/platforms", h.ListPlatforms)
	r.GET("/platforms/:platform/methods", h.ListMethods)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/call", strings.NewReader(`{"platform":"p","method":"m"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "internal_error")
	assert.Contains(t, rec.Body.String(), boom.Error())

	for _, path := range []string{"/platforms", "/platforms/p/methods"} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		assert.Equal(t, http.StatusInternalServerError, rec.Code, "path %s", path)
		assert.Contains(t, rec.Body.String(), "internal_error", "path %s", path)
		assert.Contains(t, rec.Body.String(), boom.Error(), "path %s", path)
	}
}

// 真实发现结果（registry 解析 + bindings 归一）恒不含空白方法名，
// ListMethods 的空白跳过分支仅在 discoverFn 缝隙注入时可触达：
// 空白条目必须被跳过，非空方法正常返回。
func TestService_ListMethods_SkipsBlankMethodEntries(t *testing.T) {
	s := NewService(&svc.ServiceContext{})
	s.discoverFn = func(_ context.Context) map[string][]string {
		return map[string][]string{"gapfix": {"real_method", "   ", ""}}
	}

	resp, err := s.ListMethods(context.Background(), "gapfix")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 200, resp.Code)
	assert.Equal(t, []string{"real_method"}, resp.Methods)
	assert.Equal(t, "extension", resp.Source)

	platforms, err := s.ListPlatforms(context.Background())
	require.NoError(t, err)
	require.Len(t, platforms.Platforms, 1)
	assert.Equal(t, "gapfix", platforms.Platforms[0].Name)
}

// 零值构造（未经 NewService 绑定缝隙）必须回退到真实发现逻辑，行为不变。
func TestService_ZeroValueFallsBackToRealDiscovery(t *testing.T) {
	s := &Service{}
	resp, err := s.ListMethods(context.Background(), "any")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 404, resp.Code)
	assert.Empty(t, resp.Methods)

	platforms, err := s.ListPlatforms(context.Background())
	require.NoError(t, err)
	assert.Empty(t, platforms.Platforms)
}
