// 覆盖目标（组 J）：provider.go discoverMethodsFromSpec 的 HTTP 响应体读取
// 失败分支（403.3,404.17 与 404.17,406.4）。
//
// 构造方式：httptest 服务声明 Content-Length 大于实际写入字节数后直接返回，
// 客户端 io.ReadAll 会确定性得到 unexpected EOF（无需真实网络故障）。
package openapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/internal/platform/provider"
	"github.com/stretchr/testify/require"
)

func TestDiscoverMethodsReadAllTruncatedBodyV10(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1024")
		// 声明 1024 字节但只写一小段：连接关闭后客户端读取到 unexpected EOF。
		_, _ = w.Write([]byte(`{"openapi":"3.0.3","info":{"title":"t","version":"1"`))
	}))
	defer srv.Close()

	p := NewProvider()
	require.NoError(t, p.Init(context.Background(), provider.ProviderConfig{
		Enabled: true,
		Config:  map[string]interface{}{"baseUrl": "http://example.com"},
	}))

	err := p.discoverMethodsFromSpec(context.Background(), srv.URL+"/spec.json")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unexpected EOF")
}
