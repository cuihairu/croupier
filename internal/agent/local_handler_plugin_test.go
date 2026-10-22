package agent

import (
	"context"
	"errors"
	"strings"
	"testing"

	agentlocal "github.com/cuihairu/croupier/internal/platform/agentlocal"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"google.golang.org/protobuf/proto"
)

// mockPluginFunctionManager is the dispatch-seam fake for the function
// adapter plugin path (todo.md:35).
type mockPluginFunctionManager struct {
	isPluginFunc bool
	resp         []byte
	err          error
	called       int
	lastID       string
	lastParams   []byte
}

func (m *mockPluginFunctionManager) IsPluginFunction(functionID string) bool { return m.isPluginFunc }

func (m *mockPluginFunctionManager) CallPluginFunction(ctx context.Context, functionID string, paramsJSON []byte) ([]byte, error) {
	m.called++
	m.lastID = functionID
	m.lastParams = paramsJSON
	return m.resp, m.err
}

// TestLocalHandler_HandleInvokePluginFunction pins the dispatch seam:
// a plugin-registered function is served by the plugin host locally —
// the params JSON payload reaches it and the result payload rides the
// standard InvokeResponse — while foreign ids never touch the plugin
// seam.
func TestLocalHandler_HandleInvokePluginFunction(t *testing.T) {
	invoke := func(t *testing.T, handler *LocalHandler, functionID string) ([]byte, error) {
		t.Helper()
		req := &sdkv1.InvokeRequest{FunctionId: functionID, Payload: []byte(`{"path":"/health"}`)}
		data, err := proto.Marshal(req)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		return handler.handleInvoke(context.Background(), data)
	}

	t.Run("plugin function dispatches locally", func(t *testing.T) {
		handler := NewLocalHandler(agentlocal.NewLocalStore(), "/tmp", "agent-1", nil)
		pfm := &mockPluginFunctionManager{isPluginFunc: true, resp: []byte(`{"status":200}`)}
		handler.SetPluginFunctionManager(pfm)

		resp, err := invoke(t, handler, "plugin.httpget.health")
		if err != nil {
			t.Fatalf("invoke: %v", err)
		}
		out := &sdkv1.InvokeResponse{}
		if err := proto.Unmarshal(resp, out); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if string(out.Payload) != `{"status":200}` {
			t.Fatalf("plugin result payload: %q", out.Payload)
		}
		if pfm.called != 1 || pfm.lastID != "plugin.httpget.health" || string(pfm.lastParams) != `{"path":"/health"}` {
			t.Fatalf("plugin seam inputs: called=%d id=%q params=%q", pfm.called, pfm.lastID, pfm.lastParams)
		}
	})

	t.Run("plugin function error propagates", func(t *testing.T) {
		handler := NewLocalHandler(agentlocal.NewLocalStore(), "/tmp", "agent-1", nil)
		pfm := &mockPluginFunctionManager{isPluginFunc: true, err: errors.New("function plugin call rejected: target_rejected: game interface answered 404")}
		handler.SetPluginFunctionManager(pfm)

		_, err := invoke(t, handler, "plugin.httpget.health")
		if err == nil || !strings.Contains(err.Error(), "target_rejected") {
			t.Fatalf("closed-set error must propagate verbatim: %v", err)
		}
	})

	t.Run("foreign id never touches plugin seam", func(t *testing.T) {
		handler := NewLocalHandler(agentlocal.NewLocalStore(), "/tmp", "agent-1", nil)
		pfm := &mockPluginFunctionManager{isPluginFunc: false}
		handler.SetPluginFunctionManager(pfm)

		// 未注册实例 → 走既有 pickInstance 失败路径，而非插件派发。
		_, err := invoke(t, handler, "game.func")
		if err == nil {
			t.Fatal("unregistered function must fail routing")
		}
		if pfm.called != 0 {
			t.Fatalf("foreign id must not dispatch into the plugin seam (called=%d)", pfm.called)
		}
	})
}
