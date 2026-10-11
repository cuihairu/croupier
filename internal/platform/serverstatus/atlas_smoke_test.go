// atlas 本地冒烟（atlas 对齐验收：设计 §4「真实 API 对齐 + env 门控冒烟」）：
// 起真 atlas（memory store）后设 CROUPIER_ATLAS_SMOKE_BASE_URL 运行，验证
// AtlasProvider 对真实 discovery API 的路径参数定位/status 枚举判定/清单
// 解析全链。未设环境变量一律跳过（CI 与常规单测不依赖 atlas 进程）。
// 判定值与 discovery 原始响应逐字段比对（不假设运维侧当前状态）。
//
// 目标服务器可由测试自备（推荐）：再设 CROUPIER_ATLAS_SMOKE_REGISTRY_BASE_URL
// 指向 registry 服务，测试先 register+heartbeat（幂等，重注册可把 offline 老
// 服务器拉回 rotation——atlas 侧 API 源服务器 offline 后仅靠心跳回不来）。
// 不设则沿用清单里已有服务器（须处于 discovery 可见状态）。
//
// 本地复跑（atlas 侧 memory store 全默认即可，无需 postgres/redis）：
//
//	cd ~/workspaces/atlas && go build -o /tmp/atlasd ./cmd/atlas
//	ATLAS_HTTP_ADDR=:18091 ATLAS_REGISTRY_ADDR=:18092 ATLAS_ADMIN_ADDR=:18093 \
//	  ATLAS_GRPC_ADDR=:18094 ATLAS_STORE=memory /tmp/atlasd &
//	CROUPIER_ATLAS_SMOKE_BASE_URL=http://127.0.0.1:18091 \
//	  CROUPIER_ATLAS_SMOKE_REGISTRY_BASE_URL=http://127.0.0.1:18092 \
//	  go test ./internal/platform/serverstatus/ -run TestAtlasSmoke -v
//	# 切维护态冒烟（可选）：
//	curl -s -X POST http://127.0.0.1:18093/v1/admin/servers/srv-smoke/maintenance -d '{}'
//	# 再跑一次同用例，adapter 判定须随 discovery 翻为 maintenance。
package serverstatus

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestAtlasSmoke(t *testing.T) {
	base := os.Getenv("CROUPIER_ATLAS_SMOKE_BASE_URL")
	if base == "" {
		t.Skip("CROUPIER_ATLAS_SMOKE_BASE_URL not set; local atlas smoke skipped")
	}
	serverID := os.Getenv("CROUPIER_ATLAS_SMOKE_SERVER_ID")
	if serverID == "" {
		serverID = "srv-smoke"
	}
	if registry := os.Getenv("CROUPIER_ATLAS_SMOKE_REGISTRY_BASE_URL"); registry != "" {
		setupSmokeServer(t, registry, serverID)
	}

	p := NewAtlasProvider(Options{
		BaseURL: base,
		Token:   os.Getenv("CROUPIER_ATLAS_SMOKE_TOKEN"),
		Timeout: 3 * time.Second,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 清单：discovery /servers 包装 {"servers":[...]}，须含目标服务器。
	rows, err := p.ListServers(ctx)
	if err != nil {
		t.Fatalf("ListServers: %v", err)
	}
	found := false
	for _, r := range rows {
		if r.AgentID == serverID {
			found = true
		}
	}
	if !found {
		t.Fatalf("ListServers = %+v, want server %q (offline/starting servers are hidden; set CROUPIER_ATLAS_SMOKE_REGISTRY_BASE_URL for self-contained setup)", rows, serverID)
	}

	// 单服务器：适配器判定须与 discovery 原始 status 枚举一致。
	raw := map[string]interface{}{}
	resp, err := http.Get(base + "/v1/discovery/servers/" + serverID)
	if err != nil {
		t.Fatalf("raw discovery: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("raw discovery http %d: %s", resp.StatusCode, body)
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("raw discovery decode: %v", err)
	}
	status, _ := raw["status"].(string)

	st, err := p.GetServerStatus(ctx, ServerRef{AgentID: serverID})
	if err != nil {
		t.Fatalf("GetServerStatus: %v", err)
	}
	if st.Source != ProviderAtlas {
		t.Fatalf("source = %q, want atlas", st.Source)
	}
	if want := status == "maintenance"; st.InMaintenance != want {
		t.Fatalf("InMaintenance = %v, want %v (discovery status = %q)", st.InMaintenance, want, status)
	}
}

// setupSmokeServer 幂等备好冒烟目标：register（starting）后立刻 heartbeat
// （首次有效心跳自动升 online；须在老化窗口内完成，迟到即 suspect）。
func setupSmokeServer(t *testing.T, registry, serverID string) {
	t.Helper()
	client := &http.Client{Timeout: 3 * time.Second}
	post := func(path string, body map[string]interface{}) {
		raw, _ := json.Marshal(body)
		resp, err := client.Post(registry+path, "application/json", bytes.NewReader(raw))
		if err != nil {
			t.Fatalf("smoke setup %s: %v", path, err)
		}
		data, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			t.Fatalf("smoke setup %s: http %d: %s", path, resp.StatusCode, data)
		}
	}
	post("/v1/registry/servers/register", map[string]interface{}{
		"server_id": serverID,
		"name":      "smoke",
		"type":      "gameserver",
		"region":    "local",
		"endpoint":  map[string]interface{}{"host": "127.0.0.1", "port": 7000},
	})
	post("/v1/registry/servers/"+serverID+"/heartbeat", map[string]interface{}{
		"status": "online",
	})
}
