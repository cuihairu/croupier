package openapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/platform/provider"
	"github.com/stretchr/testify/require"
)

// 能力矩阵 #14（Batch A）回归：重试等待由线性 (i+1)s 改恒定 1s，重试后
// 成功的链路不因退避改写而断（真实等待 ≤ RetryCount×1s，有界）。
func TestProviderCallRetryConstantBackOffSucceedsAfter5xx(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&hits, 1) <= 2 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	p := NewProvider()
	require.NoError(t, p.Init(context.Background(), provider.ProviderConfig{
		Enabled: true,
		Config: map[string]interface{}{
			"baseUrl":    srv.URL,
			"retryCount": 2,
			"methods": []interface{}{
				map[string]interface{}{"name": "get_x", "path": "/x", "method": "GET"},
			},
		},
	}))
	p.httpClient = srv.Client()

	start := time.Now()
	data, err := p.Call(context.Background(), "get_x", nil)
	require.NoError(t, err)
	require.NotNil(t, data)
	require.EqualValues(t, 3, atomic.LoadInt32(&hits))
	// 恒定 1s：两次重试等待 ≥2s（线性旧实现为 1s+2s=3s，不回assert上界——
	// 只验下界证明等待确实发生，避免慢机抖动假红）。
	require.GreaterOrEqual(t, time.Since(start), 2*time.Second)
}
