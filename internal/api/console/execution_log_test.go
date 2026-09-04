package console

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/executionlog"
	"github.com/cuihairu/croupier/internal/svc"
)

// R2：页面绑定执行写入 execution_logs（审批类执行除外）。
func attachExecutionLogWriter(t *testing.T, svcCtx *svc.ServiceContext) *executionlog.Writer {
	t.Helper()
	w := executionlog.NewWriter(svcCtx.DB, executionlog.Config{
		Enabled:       true,
		FlushInterval: 50 * time.Millisecond,
	})
	svcCtx.ExecutionLogWriter = w
	w.Run(context.Background())
	t.Cleanup(w.Stop)
	return w
}

func TestExecuteBindingWritesExecutionLog(t *testing.T) {
	service, ctx, _ := newConsoleTestServiceWithAudit(t, "function:invoke", "player:query")
	w := attachExecutionLogWriter(t, service.svcCtx)

	inputSchema := `{"type":"object","properties":{"keyword":{"type":"string"}}}`
	outputSchema := `{"type":"object","properties":{"items":{"type":"array"}}}`
	require.NoError(t, seedConsolePublishedPageWithSchema(service.svcCtx, ctx, inputSchema, outputSchema))
	caller := &fakeConsoleSessionCaller{payload: []byte(`{"items":[],"total":0}`)}
	service.svcCtx.Dispatcher.SetSessionResolver(fakeConsoleSessionResolver{caller: caller})

	_, err := service.ExecuteBinding(ctx, &ConsoleExecuteBindingRequest{
		PageKey:   "player.manage",
		BindingID: "player.query",
		Context: ConsoleBindingExecutionContext{
			Form: json.RawMessage(`{"keyword":"k1"}`),
		},
	})
	require.NoError(t, err)
	w.Stop()

	var items []model.ExecutionLog
	require.NoError(t, service.svcCtx.DB.Find(&items).Error)
	require.Len(t, items, 1)
	assert.Equal(t, executionlog.SourcePage, items[0].Source)
	assert.Equal(t, "player.query", items[0].FunctionID)
	assert.Equal(t, "console_tester", items[0].Actor)
	assert.Equal(t, executionlog.StatusOK, items[0].Status)
	assert.Equal(t, "player.manage", items[0].PageKey)
	assert.Equal(t, "player.query", items[0].BindingID)
	require.True(t, strings.Contains(string(items[0].RequestPayload), "k1"))
	require.True(t, strings.Contains(string(items[0].ResponseBody), "items"))
}

func TestExecuteBindingFailureWritesErrorLog(t *testing.T) {
	service, ctx, _ := newConsoleTestServiceWithAudit(t, "function:invoke", "player:query")
	w := attachExecutionLogWriter(t, service.svcCtx)

	inputSchema := `{"type":"object","properties":{"keyword":{"type":"string"}}}`
	outputSchema := `{"type":"object","properties":{"items":{"type":"array"}}}`
	require.NoError(t, seedConsolePublishedPageWithSchema(service.svcCtx, ctx, inputSchema, outputSchema))
	caller := &fakeConsoleSessionCaller{err: assert.AnError}
	service.svcCtx.Dispatcher.SetSessionResolver(fakeConsoleSessionResolver{caller: caller})

	_, err := service.ExecuteBinding(ctx, &ConsoleExecuteBindingRequest{
		PageKey:   "player.manage",
		BindingID: "player.query",
		Context: ConsoleBindingExecutionContext{
			Form: json.RawMessage(`{"keyword":"k1"}`),
		},
	})
	require.Error(t, err)
	w.Stop()

	var items []model.ExecutionLog
	require.NoError(t, service.svcCtx.DB.Find(&items).Error)
	require.Len(t, items, 1)
	assert.Equal(t, executionlog.StatusFail, items[0].Status)
	require.True(t, strings.Contains(string(items[0].ResponseBody), "error"))
}
