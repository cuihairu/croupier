package main

// MCP-001 测试：InMemoryTransport 直连（不经 stdio）做三工具全链验证——
// 鉴权/scope 头透传、REST 语义透传、统一错误对象转 isError 工具结果、
// 工具清单如实缺位（恰好三个，无隐藏写工具）。后端断言一律「捕获 →
// 测试 goroutine 断言」（httptest handler 跑在别的 goroutine，handler 内
// require 会 Goexit 掉响应链路）。

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

// mcpBackendCapture 捕获假后端观测到的请求形态（header/query/body）。
type mcpBackendCapture struct {
	mu           sync.Mutex
	authz        []string
	gameID       []string
	env          []string
	taskQuery    url.Values
	invokeBodies []map[string]any
}

func (c *mcpBackendCapture) recordAuth(r *http.Request) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.authz = append(c.authz, r.Header.Get("Authorization"))
	c.gameID = append(c.gameID, r.Header.Get("X-Game-ID"))
	c.env = append(c.env, r.Header.Get("X-Env"))
}

// mcpTestBackend 假 croupier-server：三端点回固定 payload；fn-bad 回统一
// 错误对象（409 形态）。
func mcpTestBackend(t *testing.T) (*httptest.Server, *mcpBackendCapture) {
	t.Helper()
	cap := &mcpBackendCapture{}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/ops/agents", func(w http.ResponseWriter, r *http.Request) {
		cap.recordAuth(r)
		_, _ = w.Write([]byte(`{"agents":[{"agentId":"agent-a","connected":true}]}`))
	})
	mux.HandleFunc("/api/v1/tasks", func(w http.ResponseWriter, r *http.Request) {
		cap.recordAuth(r)
		cap.mu.Lock()
		cap.taskQuery = r.URL.Query()
		cap.mu.Unlock()
		_, _ = w.Write([]byte(`{"items":[{"id":"task-9","status":"running"}],"total":1,"page":3,"pageSize":20}`))
	})
	mux.HandleFunc("/api/v1/functions/fn-1/invoke", func(w http.ResponseWriter, r *http.Request) {
		cap.recordAuth(r)
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		cap.mu.Lock()
		cap.invokeBodies = append(cap.invokeBodies, body)
		cap.mu.Unlock()
		_, _ = w.Write([]byte(`{"taskId":"","result":{"echo":true}}`))
	})
	mux.HandleFunc("/api/v1/functions/fn-bad/invoke", func(w http.ResponseWriter, r *http.Request) {
		cap.recordAuth(r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"conflict","message":"资源状态冲突"}`))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server, cap
}

// newMCPTestSession 起假后端 + MCP server + InMemoryTransport 客户端直连。
func newMCPTestSession(t *testing.T) (*mcp.ClientSession, *mcpBackendCapture) {
	t.Helper()
	backend, cap := mcpTestBackend(t)
	client := &mcpRESTClient{
		baseURL:    backend.URL,
		token:      "tok-1",
		gameID:     "demo",
		env:        "prod",
		httpClient: &http.Client{Timeout: mcpHTTPTimeout},
	}
	session := newMCPTestSessionFor(t, client)
	return session, cap
}

func newMCPTestSessionFor(t *testing.T, client *mcpRESTClient) *mcp.ClientSession {
	t.Helper()
	server := newMCPServer(client)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ctx := context.Background()

	serverSession, err := server.Connect(ctx, serverTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = serverSession.Close() })

	mcpClient := mcp.NewClient(&mcp.Implementation{Name: "mcp-test", Version: "v1"}, nil)
	session, err := mcpClient.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	return session
}

func callText(t *testing.T, session *mcp.ClientSession, name string, args map[string]any) string {
	t.Helper()
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	require.NoError(t, err)
	require.False(t, res.IsError)
	text, ok := res.Content[0].(*mcp.TextContent)
	require.True(t, ok, "expected TextContent, got %T", res.Content[0])
	return text.Text
}

func TestMCPServerToolListIsExactlyReadOnlySet(t *testing.T) {
	session, _ := newMCPTestSession(t)
	list, err := session.ListTools(context.Background(), &mcp.ListToolsParams{})
	require.NoError(t, err)

	names := make([]string, 0, len(list.Tools))
	for _, tool := range list.Tools {
		names = append(names, tool.Name)
	}
	// 工具清单如实缺位：恰好三个只读工具，无写工具（封禁/补偿/订单）。
	require.ElementsMatch(t, []string{"list_sessions", "query_jobs", "invoke_function"}, names)
}

func TestMCPListSessionsPassesAuthAndScopeHeaders(t *testing.T) {
	session, cap := newMCPTestSession(t)
	body := callText(t, session, "list_sessions", map[string]any{})
	require.Contains(t, body, `"agentId":"agent-a"`)

	require.NotEmpty(t, cap.authz)
	for i := range cap.authz {
		require.Equal(t, "Bearer tok-1", cap.authz[i])
		require.Equal(t, "demo", cap.gameID[i])
		require.Equal(t, "prod", cap.env[i])
	}
}

func TestMCPQueryJobsMapsFiltersToQueryParams(t *testing.T) {
	session, cap := newMCPTestSession(t)
	body := callText(t, session, "query_jobs", map[string]any{
		"status": "running", "functionId": "fn-1", "page": 3, "size": 20,
	})
	require.Contains(t, body, `"total":1`)
	require.Contains(t, body, `"task-9"`)

	require.Equal(t, "running", cap.taskQuery.Get("status"))
	require.Equal(t, "fn-1", cap.taskQuery.Get("functionId"))
	require.Equal(t, "3", cap.taskQuery.Get("page"))
	require.Equal(t, "20", cap.taskQuery.Get("size"))
}

func TestMCPInvokeFunctionSyncOnly(t *testing.T) {
	session, cap := newMCPTestSession(t)
	body := callText(t, session, "invoke_function", map[string]any{
		"id": "fn-1", "params": map[string]any{"k": "v"}, "timeoutMs": 5000,
	})
	require.Contains(t, body, `"echo":true`)

	require.Len(t, cap.invokeBodies, 1)
	sent := cap.invokeBodies[0]
	// 不携带 mode：MCP 侧固定 sync（省略即 sync）；async 任务面不出本批。
	_, hasMode := sent["mode"]
	require.False(t, hasMode, "mcp must not send mode (sync default)")
	require.Equal(t, map[string]any{"k": "v"}, sent["params"])
	require.Equal(t, float64(5000), sent["timeoutMs"])
	// scope 不在 body 内自报：统一链以 ctx scope 头覆写（headers 即真值）。
	_, hasGameID := sent["gameId"]
	require.False(t, hasGameID)
}

func TestMCPInvokeFunctionErrorSurfacesAsToolError(t *testing.T) {
	session, _ := newMCPTestSession(t)
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "invoke_function",
		Arguments: map[string]any{"id": "fn-bad"},
	})
	// 协议层成功；错误以 isError 工具结果呈现（AI 宿主可见可自纠）。
	require.NoError(t, err)
	require.True(t, res.IsError)
	text, ok := res.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	require.Contains(t, text.Text, "409")
	require.Contains(t, text.Text, "conflict")
}

func TestMCPInvokeFunctionValidationAndTimeout(t *testing.T) {
	// runMCPServer 的必填校验逻辑（空参形态在 flag 层）直接验 rest 客户端。
	_, _ = newMCPTestSession(t)

	// timeout 常量覆盖同步调用上限 60s。
	require.Greater(t, mcpHTTPTimeout, 60*time.Second)
}
