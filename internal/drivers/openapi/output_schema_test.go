package openapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/cuihairu/croupier/internal/platform/provider"
	"github.com/stretchr/testify/require"
)

// 能力矩阵 #4（批次 C）：InvokeResponse 响应 schema 校验——提取（正反）、
// 无 schema 兼容放行、编译失败降级（不破坏既有调用）、数据不符硬失败。

func TestExtractOutputSchema_Picks200BeforeOther2xx(t *testing.T) {
	root := map[string]interface{}{
		"paths": map[string]interface{}{},
	}
	methodObj := map[string]interface{}{
		"responses": map[string]interface{}{
			"204": map[string]interface{}{
				"content": map[string]interface{}{
					"application/json": map[string]interface{}{
						"schema": map[string]interface{}{"type": "string"},
					},
				},
			},
			"200": map[string]interface{}{
				"content": map[string]interface{}{
					"application/json": map[string]interface{}{
						"schema": map[string]interface{}{"type": "object"},
					},
				},
			},
		},
	}
	out := extractOutputSchema(methodObj, root)
	require.JSONEq(t, `{"type":"object"}`, out)
}

func TestExtractOutputSchema_RefResolved(t *testing.T) {
	root := map[string]interface{}{
		"components": map[string]interface{}{
			"schemas": map[string]interface{}{
				"User": map[string]interface{}{
					"type":     "object",
					"required": []interface{}{"id"},
					"properties": map[string]interface{}{
						"id": map[string]interface{}{"type": "integer"},
					},
				},
			},
		},
	}
	methodObj := map[string]interface{}{
		"responses": map[string]interface{}{
			"200": map[string]interface{}{
				"content": map[string]interface{}{
					"application/json": map[string]interface{}{
						"schema": map[string]interface{}{"$ref": "#/components/schemas/User"},
					},
				},
			},
		},
	}
	out := extractOutputSchema(methodObj, root)
	require.JSONEq(t,
		`{"type":"object","required":["id"],"properties":{"id":{"type":"integer"}}}`, out)
}

func TestExtractOutputSchema_Swagger20Shape(t *testing.T) {
	root := map[string]interface{}{}
	methodObj := map[string]interface{}{
		"responses": map[string]interface{}{
			"200": map[string]interface{}{
				"schema": map[string]interface{}{"type": "array"},
			},
		},
	}
	require.JSONEq(t, `{"type":"array"}`, extractOutputSchema(methodObj, root))
}

func TestExtractOutputSchema_NoJSONSuccessResponse(t *testing.T) {
	root := map[string]interface{}{}
	// 无 responses / 只有非 2xx / 2xx 无 JSON 内容 → 空串（跳过校验）。
	require.Empty(t, extractOutputSchema(map[string]interface{}{}, root))
	require.Empty(t, extractOutputSchema(map[string]interface{}{
		"responses": map[string]interface{}{
			"400":     map[string]interface{}{"description": "bad"},
			"default": map[string]interface{}{"description": "err"},
		},
	}, root))
	require.Empty(t, extractOutputSchema(map[string]interface{}{
		"responses": map[string]interface{}{
			"200": map[string]interface{}{
				"content": map[string]interface{}{
					"text/plain": map[string]interface{}{
						"schema": map[string]interface{}{"type": "string"},
					},
				},
			},
		},
	}, root))
}

// outputSpec 三个方法：get_user（有 $ref schema）、get_raw（无 response
// schema）、get_weird（schema 关键字类型非法 → 编译失败降级）。
const outputSpec = `{
  "openapi": "3.0.3",
  "info": {"title": "t", "version": "1.0.0"},
  "paths": {
    "/api/user": {
      "get": {
        "operationId": "get_user",
        "responses": {
          "200": {"description": "ok", "content": {"application/json": {"schema": {"$ref": "#/components/schemas/User"}}}}
        }
      }
    },
    "/api/raw": {
      "get": {
        "operationId": "get_raw",
        "responses": {"200": {"description": "ok"}}
      }
    },
    "/api/weird": {
      "get": {
        "operationId": "get_weird",
        "responses": {
          "200": {"description": "ok", "content": {"application/json": {"schema": {"type": "bogus"}}}}
        }
      }
    }
  },
  "components": {"schemas": {"User": {
    "type": "object",
    "required": ["id"],
    "properties": {"id": {"type": "integer"}}
  }}}
}`

// newOutputValidationProvider 起一个同时服务 spec 与业务 API 的测试服务器，
// 按 operationId 分发响应体。
func newOutputValidationProvider(t *testing.T, bodyFor func(method string) string) (*Provider, *int32) {
	t.Helper()
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/openapi.json" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(outputSpec))
			return
		}
		atomic.AddInt32(&hits, 1) // 只计业务 API 命中，spec 拉取不计
		switch r.URL.Path {
		case "/api/user":
			_, _ = w.Write([]byte(bodyFor("get_user")))
		case "/api/raw":
			_, _ = w.Write([]byte(bodyFor("get_raw")))
		case "/api/weird":
			_, _ = w.Write([]byte(bodyFor("get_weird")))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	p := NewProvider()
	require.NoError(t, p.Init(context.Background(), provider.ProviderConfig{
		Enabled: true,
		Config: map[string]interface{}{
			"baseUrl":      srv.URL,
			"openapiSpecs": []interface{}{srv.URL + "/openapi.json"},
		},
	}))
	p.httpClient = srv.Client()
	return p, &hits
}

func TestCall_ResponseValidationPasses(t *testing.T) {
	p, _ := newOutputValidationProvider(t, func(method string) string {
		if method == "get_weird" {
			return `{"ok":true}`
		}
		return `{"id": 42}`
	})
	data, err := p.Call(context.Background(), "get_user", nil)
	require.NoError(t, err)
	require.JSONEq(t, `{"id":42}`, string(data))
}

func TestCall_ResponseValidationRejectsInvalidData(t *testing.T) {
	p, hits := newOutputValidationProvider(t, func(method string) string {
		return `{"id": "not-an-int"}`
	})
	_, err := p.Call(context.Background(), "get_user", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "response validation failed for get_user")
	// 单次 HTTP 交付（校验失败发生在交付之后，不触发重试）。
	require.EqualValues(t, 1, atomic.LoadInt32(hits))
}

func TestCall_NoResponseSchemaSkipsValidation(t *testing.T) {
	// get_raw 未声明 response schema：任意返回体原样放行（兼容既有 provider）。
	p, _ := newOutputValidationProvider(t, func(method string) string {
		return `whatever-not-json`
	})
	data, err := p.Call(context.Background(), "get_raw", nil)
	require.NoError(t, err)
	require.Equal(t, `whatever-not-json`, string(data))
}

func TestCall_UncompilableSchemaDegradesToSkip(t *testing.T) {
	// spec 关键字类型非法（type: bogus）→ 编译失败走告警降级，调用不被破坏。
	p, _ := newOutputValidationProvider(t, func(method string) string {
		return `{"whatever": 1}`
	})
	data, err := p.Call(context.Background(), "get_weird", nil)
	require.NoError(t, err)
	require.JSONEq(t, `{"whatever":1}`, string(data))
}

func TestCall_ResponseValidationCompiledOnce(t *testing.T) {
	p, _ := newOutputValidationProvider(t, func(method string) string {
		return `{"id": 1}`
	})
	for i := 0; i < 3; i++ {
		_, err := p.Call(context.Background(), "get_user", nil)
		require.NoError(t, err)
	}
	_, ok := p.outputValidators.Load("get_user")
	require.True(t, ok, "compiled validator must be cached after first Call")
}
