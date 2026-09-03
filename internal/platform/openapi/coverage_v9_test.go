// 覆盖目标：converter 扩展字段与校验错误分支、ValidateSpec 加载失败、
// Init/extractConfig/buildMethodMap 错误路径、discoverMethodsFromSpec HTTP/YAML 错误、
// parseOpenAPISpec 合并解析错误、Call 限流/构造请求/重试取消分支、
// filterConsumedParams 全分支。
package openapi

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/platform/provider"
	"github.com/stretchr/testify/require"
)

func TestToOpenAPIOperation_RiskOnlyExtensionV9(t *testing.T) {
	c := NewProviderConverter()
	op, err := c.ToOpenAPIOperation(&FunctionDescriptor{
		ID:   "op.risk",
		Risk: "high",
	})
	require.NoError(t, err)
	require.Equal(t, "high", op.Extensions["x-risk"])
	_, hasResource := op.Extensions["x-resource"]
	require.False(t, hasResource)
}

func TestToOpenAPIOperation_OperationOnlyExtensionV9(t *testing.T) {
	c := NewProviderConverter()
	op, err := c.ToOpenAPIOperation(&FunctionDescriptor{
		ID:        "op.operation",
		Operation: "write",
	})
	require.NoError(t, err)
	require.Equal(t, "write", op.Extensions["x-operation"])
	require.Empty(t, op.Extensions["x-resource"])
	require.Empty(t, op.Extensions["x-risk"])
}

func TestToOpenAPIOperation_InvalidOperationV9(t *testing.T) {
	c := NewProviderConverter()
	_, err := c.ToOpenAPIOperation(&FunctionDescriptor{
		Summary: "missing operation id",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid operation")
}

func TestValidatorValidateSpecInvalidDataV9(t *testing.T) {
	v := NewValidator()
	_, err := v.ValidateSpec([]byte(`{not valid json`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to load OpenAPI spec")
}

func newOpenapiProviderWithConfigV9(t *testing.T, cfg map[string]interface{}) *Provider {
	t.Helper()
	p := NewProvider()
	require.NoError(t, p.Init(context.Background(), provider.ProviderConfig{
		Enabled: true,
		Type:    "openapi",
		Config:  cfg,
	}))
	return p
}

func TestProviderInitExtractConfigMarshalErrorV9(t *testing.T) {
	p := NewProvider()
	err := p.Init(context.Background(), provider.ProviderConfig{
		Enabled: true,
		Config:  map[string]interface{}{"bad": func() {}},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to extract config")

	// 直接调用 extractConfig 覆盖同一错误分支
	p2 := NewProvider()
	p2.openapiConfig = &Config{}
	require.Error(t, p2.extractConfig(map[string]interface{}{"bad": func() {}}))
}

func TestProviderBuildMethodMapSkipsUnnamedV9(t *testing.T) {
	p := newOpenapiProviderWithConfigV9(t, map[string]interface{}{
		"baseUrl": "http://example.com",
		"methods": []interface{}{
			map[string]interface{}{"name": "", "path": "/x", "method": "GET"},
			map[string]interface{}{"name": "ok", "path": "/y", "method": "GET"},
		},
	})
	require.Equal(t, []string{"ok"}, p.SupportedMethods())
}

func TestProviderInitSpecsDiscoveryErrorV9(t *testing.T) {
	p := NewProvider()
	err := p.Init(context.Background(), provider.ProviderConfig{
		Enabled: true,
		Config: map[string]interface{}{
			"baseUrl":      "http://example.com",
			"openapiSpecs": []interface{}{"/nonexistent/spec-a.json"},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to discover methods from /nonexistent/spec-a.json")
}

func TestProviderInitSingleSpecDiscoveryErrorV9(t *testing.T) {
	p := NewProvider()
	err := p.Init(context.Background(), provider.ProviderConfig{
		Enabled: true,
		Config: map[string]interface{}{
			"baseUrl":     "http://example.com",
			"openapiSpec": "/nonexistent/spec-b.json",
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to discover methods")
}

func TestDiscoverMethodsBadRequestURLV9(t *testing.T) {
	p := NewProvider()
	p.openapiConfig = &Config{}
	p.httpClient = &http.Client{}
	require.Error(t, p.discoverMethodsFromSpec(context.Background(), "http://a b/spec.json"))
}

func TestDiscoverMethodsUnreachableServerV9(t *testing.T) {
	p := NewProvider()
	p.openapiConfig = &Config{}
	p.httpClient = &http.Client{Timeout: 2 * time.Second}
	err := p.discoverMethodsFromSpec(context.Background(), "http://127.0.0.1:1/spec.json")
	require.Error(t, err)
}

func TestDiscoverMethodsNon200StatusV9(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	defer srv.Close()

	p := NewProvider()
	p.openapiConfig = &Config{}
	p.httpClient = srv.Client()
	err := p.discoverMethodsFromSpec(context.Background(), srv.URL+"/spec.json")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to fetch OpenAPI spec")
}

func TestDiscoverMethodsInvalidYAMLV9(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	require.NoError(t, os.WriteFile(path, []byte("key: [unclosed"), 0o644))

	p := NewProvider()
	p.openapiConfig = &Config{}
	err := p.discoverMethodsFromSpec(context.Background(), path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to convert YAML to JSON")
}

func TestParseOpenAPISpecMergeUnmarshalErrorsV9(t *testing.T) {
	validSpec := []byte(`{"openapi":"3.0.3","info":{"title":"t","version":"1"},"paths":{}}`)

	// 已有文档不是合法 JSON
	p := NewProvider()
	p.openapiDoc = json.RawMessage(`{"paths":`)
	err := p.parseOpenAPISpec(validSpec)
	require.Error(t, err)

	// 新 spec 不是合法 JSON（已有文档合法）
	p2 := NewProvider()
	p2.openapiDoc = json.RawMessage(`{"paths":{}}`)
	err = p2.parseOpenAPISpec([]byte(`{bad`))
	require.Error(t, err)

	// 无已有文档时 spec 非法 JSON
	p3 := NewProvider()
	err = p3.parseOpenAPISpec([]byte(`{bad`))
	require.Error(t, err)
}

// errorLimiterV9 总是返回 ctx 错误的限流器，用于覆盖 Call 的限流失败分支。
type errorLimiterV9 struct{}

func (errorLimiterV9) Wait(ctx context.Context) error { return ctx.Err() }

func TestProviderCallRateLimiterContextCanceledV9(t *testing.T) {
	p := NewProvider()
	require.NoError(t, p.Init(context.Background(), provider.ProviderConfig{
		Enabled: true,
		Config:  map[string]interface{}{"baseUrl": "http://example.com"},
	}))

	cctx, cancel := context.WithCancel(context.Background())
	cancel()
	p.rateLimiter = errorLimiterV9{}
	_, err := p.Call(cctx, "any_method", nil)
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
}

func TestProviderCallBuildRequestErrorV9(t *testing.T) {
	p := newOpenapiProviderWithConfigV9(t, map[string]interface{}{
		"baseUrl": "http://example.com",
		"methods": []interface{}{
			map[string]interface{}{"name": "bad", "path": "/x", "method": "BAD METHOD"},
		},
	})

	_, err := p.Call(context.Background(), "bad", []byte(`{}`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to build request")

	// 直接调用 buildRequest 覆盖同一错误分支
	_, err = p.buildRequest(context.Background(), p.methodMap["bad"], nil)
	require.Error(t, err)
}

func TestProviderCallDefaultHeadersV9(t *testing.T) {
	var gotHeader string
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("X-Custom-V9")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})
	defer srv.Close()

	p := NewProvider()
	require.NoError(t, p.Init(context.Background(), provider.ProviderConfig{
		Enabled: true,
		Config: map[string]interface{}{
			"baseUrl": srv.URL,
			"headers": map[string]interface{}{"X-Custom-V9": "header-value"},
			"methods": []interface{}{
				map[string]interface{}{"name": "get_x", "path": "/x", "method": "GET"},
			},
		},
	}))
	p.httpClient = srv.Client()

	_, err := p.Call(context.Background(), "get_x", nil)
	require.NoError(t, err)
	require.Equal(t, "header-value", gotHeader)
}

func TestProviderCallRetryContextCanceledV9(t *testing.T) {
	p := NewProvider()
	require.NoError(t, p.Init(context.Background(), provider.ProviderConfig{
		Enabled: true,
		Config: map[string]interface{}{
			"baseUrl":    "http://127.0.0.1:1",
			"retryCount": 1,
			"methods": []interface{}{
				map[string]interface{}{"name": "get_x", "path": "/x", "method": "GET"},
			},
		},
	}))

	cctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := p.Call(cctx, "get_x", nil)
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
}

func TestFilterConsumedParamsV9(t *testing.T) {
	// 空 reqData 原样返回
	reqData := map[string]interface{}{}
	require.Equal(t, reqData, filterConsumedParams([]ParameterMapping{{Name: "a"}}, reqData))

	// 无参数映射：consumed 为空，原样返回
	reqData = map[string]interface{}{"k": "v"}
	require.Equal(t, reqData, filterConsumedParams(nil, reqData))

	// From 为空时回退到 Name
	reqData = map[string]interface{}{"id": "1", "extra": "e"}
	filtered := filterConsumedParams([]ParameterMapping{{Name: "id", In: "path"}}, reqData)
	require.NotContains(t, filtered, "id")
	require.Equal(t, "e", filtered["extra"])

	// 显式 From 优先于 Name
	reqData = map[string]interface{}{"src": "s", "keep": "k"}
	filtered = filterConsumedParams([]ParameterMapping{{Name: "other", From: "src", In: "query"}}, reqData)
	require.NotContains(t, filtered, "src")
	require.Equal(t, "k", filtered["keep"])

	// Name 与 From 均为空的映射不消费任何字段
	reqData = map[string]interface{}{"k": "v"}
	filtered = filterConsumedParams([]ParameterMapping{{Name: "", From: "", In: "header"}}, reqData)
	require.Equal(t, "v", filtered["k"])
}

func TestProviderGetMethodDetailsExtensionsV9(t *testing.T) {
	p := newOpenapiProviderWithConfigV9(t, map[string]interface{}{
		"baseUrl": "http://example.com",
		"methods": []interface{}{
			map[string]interface{}{
				"name":        "extended",
				"path":        "/e",
				"method":      "POST",
				"x-resource":  "res",
				"x-risk":      "high",
				"x-operation": "write",
			},
		},
	})
	details := p.GetMethodDetails()["extended"]
	require.NotNil(t, details)
	require.Equal(t, "res", details.Resource)
	require.Equal(t, "high", details.Risk)
	require.Equal(t, "write", details.Operation)
	require.True(t, strings.Contains(strings.Join([]string{details.Name}, ","), "extended"))
}
