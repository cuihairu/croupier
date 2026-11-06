package openapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/internal/platform/provider"
)

// 运行时 function ID 注册与调用侧统一折叠为小写（control_handler 折叠），
// 而 methodMap 以 OpenAPI operationId 原样（可能含驼峰）为键。camelCase
// operationId（如 player.batchRecharge）经折叠后以小写调用，必须仍可路由。
func TestProviderCallFoldsLowercasedMethod(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/players/batch-recharge" && r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "affected": 2})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	p := NewProvider()
	_ = p.Init(context.Background(), provider.ProviderConfig{
		Enabled: true,
		Type:    "openapi",
		Config: map[string]interface{}{
			"baseUrl": server.URL,
			"methods": []interface{}{
				map[string]interface{}{
					"name":   "player.batchRecharge",
					"path":   "/players/batch-recharge",
					"method": "POST",
				},
			},
		},
	})

	resp, err := p.Call(context.Background(), "player.batchrecharge", []byte(`{"playerIds":["p-001","p-002"],"amount":100}`))
	if err != nil {
		t.Fatalf("Call(lowercased method) error = %v", err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(resp, &out); err != nil {
		t.Fatalf("response not json: %v", err)
	}
	if out["affected"] != float64(2) {
		t.Errorf("affected = %v, want 2", out["affected"])
	}

	// 精确原名调用保持不变。
	if _, err := p.Call(context.Background(), "player.batchRecharge", nil); err != nil {
		t.Errorf("Call(exact method) error = %v", err)
	}

	// 完全未注册的方法仍须报 MethodNotSupported。
	if _, err := p.Call(context.Background(), "player.unknown", nil); err == nil {
		t.Error("Call(unknown method) should fail")
	}
}
