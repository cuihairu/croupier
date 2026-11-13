package main

// mcp 子命令（MCP-001）：stdio MCP server，暴露三个只读工具——
// list_sessions / query_jobs / invoke_function——全部转发到运行中的
// croupier-server REST API（Bearer JWT + X-Game-ID/X-Env scope 头）。
// 工具语义即 REST 语义，本进程不直连 DB、不另立真值。
//
// 边界诚实：游戏侧写操作（封禁/补偿/订单等）不在本批——等 GAME-009/010
// 凭据协议定稿后再引入，工具清单如实缺位，不以假工具补位。invoke_function
// 固定 sync 模式（同步结果直读；异步任务创建属任务面，非本批工具）。
//
// 部署形态：AI 宿主 spawn `croupier-server mcp --server-url ... --token ...`，
// MCP 协议走 stdin/stdout，日志与错误走 stderr（stdio 协议不容 stdout 污染）。

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "运行只读 MCP stdio server（list_sessions / query_jobs / invoke_function）",
	Long: `运行 Model Context Protocol stdio server，向 AI 宿主暴露只读运维工具。

工具清单（本批全量，无隐藏工具）：
  - list_sessions：列出已注册 agent 会话（连接状态/版本/承载函数）
  - query_jobs：分页查询任务（可按状态/函数过滤）
  - invoke_function：同步调用已注册函数（走既有注册调用链；游戏侧可能有副作用）

游戏侧写操作（封禁/补偿/订单）不在本批：等 GAME-009/010 凭据协议定稿后再
引入，不以假工具占位。

连接参数：
  --server-url   croupier-server 基地址（必填）
  --token        操作者 Bearer JWT（必填）
  --game-id      X-Game-ID scope 头（工具全部走此 scope）
  --env          X-Env scope 头`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMCPServer()
	},
}

var (
	mcpServerURL string
	mcpToken     string
	mcpGameID    string
	mcpEnvName   string
)

func init() {
	// 默认值取环境变量：AI 宿主的 MCP 配置普遍只写 command + env。
	mcpCmd.Flags().StringVar(&mcpServerURL, "server-url", os.Getenv("CROUPIER_MCP_SERVER_URL"),
		"croupier-server 基地址，如 http://127.0.0.1:18780（env: CROUPIER_MCP_SERVER_URL）")
	mcpCmd.Flags().StringVar(&mcpToken, "token", os.Getenv("CROUPIER_MCP_TOKEN"),
		"操作者 Bearer JWT（env: CROUPIER_MCP_TOKEN）")
	mcpCmd.Flags().StringVar(&mcpGameID, "game-id", os.Getenv("CROUPIER_MCP_GAME_ID"),
		"X-Game-ID scope 头；工具调用全部落此 scope（env: CROUPIER_MCP_GAME_ID）")
	mcpCmd.Flags().StringVar(&mcpEnvName, "env", os.Getenv("CROUPIER_MCP_ENV"),
		"X-Env scope 头（env: CROUPIER_MCP_ENV）")
	rootCmd.AddCommand(mcpCmd)
}

func runMCPServer() error {
	baseURL := strings.TrimRight(strings.TrimSpace(mcpServerURL), "/")
	if baseURL == "" {
		return fmt.Errorf("--server-url（或 CROUPIER_MCP_SERVER_URL）必填")
	}
	token := strings.TrimSpace(mcpToken)
	if token == "" {
		return fmt.Errorf("--token（或 CROUPIER_MCP_TOKEN）必填")
	}

	client := &mcpRESTClient{
		baseURL: baseURL,
		token:   token,
		gameID:  strings.TrimSpace(mcpGameID),
		env:     strings.TrimSpace(mcpEnvName),
		// 65s 覆盖同步调用上限 60s（timeoutMs clamp 上界）+ 转发开销。
		httpClient: &http.Client{Timeout: mcpHTTPTimeout},
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return newMCPServer(client).Run(ctx, &mcp.StdioTransport{})
}

// newMCPServer 装配三个只读工具。测试复用：InMemoryTransport 直连验证，
// 不经 stdio。
func newMCPServer(client *mcpRESTClient) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "croupier-server-mcp",
		Version: Version,
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_sessions",
		Description: "List registered agent sessions (connection status, version, served functions) for the configured game scope. Read-only passthrough of GET /api/v1/ops/agents.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ mcpListSessionsArgs) (*mcp.CallToolResult, any, error) {
		raw, err := client.do(ctx, http.MethodGet, "/api/v1/ops/agents", nil, nil)
		if err != nil {
			return nil, nil, err
		}
		return mcpTextResult(raw), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "query_jobs",
		Description: "Page through task records (id, status, progress, function, agent, timings) with optional status/functionId filters. Read-only passthrough of GET /api/v1/tasks.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args mcpQueryJobsArgs) (*mcp.CallToolResult, any, error) {
		raw, err := client.do(ctx, http.MethodGet, "/api/v1/tasks", args.query(), nil)
		if err != nil {
			return nil, nil, err
		}
		return mcpTextResult(raw), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "invoke_function",
		Description: "Invoke a registered game function synchronously through the existing registry dispatch chain (POST /api/v1/functions/{id}/invoke, mode=sync only). NOTE: this executes the function on the game side and may cause game-side effects; async task creation is intentionally not exposed.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args mcpInvokeFunctionArgs) (*mcp.CallToolResult, any, error) {
		body := map[string]any{}
		if args.Params != nil {
			body["params"] = args.Params
		}
		if args.TimeoutMs > 0 {
			body["timeoutMs"] = args.TimeoutMs
		}
		raw, err := client.do(ctx, http.MethodPost,
			"/api/v1/functions/"+url.PathEscape(args.ID)+"/invoke", nil, body)
		if err != nil {
			return nil, nil, err
		}
		return mcpTextResult(raw), nil, nil
	})

	return server
}

// ---- 工具入参（jsonschema tag 即字段描述，SDK 自动生成 input schema）----

// mcpListSessionsArgs 无入参：scope 由连接参数决定（AI 宿主不能跳 scope）。
type mcpListSessionsArgs struct{}

type mcpQueryJobsArgs struct {
	Status     string `json:"status,omitempty" jsonschema:"filter by task status (e.g. running, succeeded, failed); empty = all"`
	FunctionID string `json:"functionId,omitempty" jsonschema:"filter by function contract id; empty = all"`
	Page       int    `json:"page,omitempty" jsonschema:"1-based page number; default 1"`
	Size       int    `json:"size,omitempty" jsonschema:"page size; default 20"`
}

func (a mcpQueryJobsArgs) query() url.Values {
	q := url.Values{}
	if v := strings.TrimSpace(a.Status); v != "" {
		q.Set("status", v)
	}
	if v := strings.TrimSpace(a.FunctionID); v != "" {
		q.Set("functionId", v)
	}
	if a.Page > 0 {
		q.Set("page", fmt.Sprintf("%d", a.Page))
	}
	if a.Size > 0 {
		q.Set("size", fmt.Sprintf("%d", a.Size))
	}
	return q
}

type mcpInvokeFunctionArgs struct {
	ID        string         `json:"id" jsonschema:"registered function contract id"`
	Params    map[string]any `json:"params,omitempty" jsonschema:"function input params object; validated server-side against the function input schema"`
	TimeoutMs int            `json:"timeoutMs,omitempty" jsonschema:"sync call timeout budget in ms, clamped to [1000,60000]"`
}

// ---- REST 薄客户端 ----

const mcpHTTPTimeout = 65 * time.Second

// mcpRESTClient 是 croupier-server REST API 的转发客户端：鉴权头 + scope
// 头 + 统一错误对象解析。不做响应字段的二次建模——工具输出即 REST payload
// 原文（JSON text content），避免 MCP 侧漂移出第二份契约。
type mcpRESTClient struct {
	baseURL    string
	token      string
	gameID     string
	env        string
	httpClient *http.Client
}

// mcpAPIError 是 REST 统一错误对象（API Response Contract）。
type mcpAPIError struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func (c *mcpRESTClient) do(ctx context.Context, method, path string, query url.Values, body any) (json.RawMessage, error) {
	var reqBody io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encode request body: %w", err)
		}
		reqBody = strings.NewReader(string(encoded))
	}

	u := c.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if c.gameID != "" {
		req.Header.Set("X-Game-ID", c.gameID)
	}
	if c.env != "" {
		req.Header.Set("X-Env", c.env)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("croupier REST request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read croupier REST response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, mcpRESTError(resp.StatusCode, raw)
	}
	return json.RawMessage(raw), nil
}

// mcpRESTError 把统一错误对象转成 error（SDK 自动落成 isError 工具结果，
// AI 宿主可见可自纠）。
func mcpRESTError(status int, raw []byte) error {
	apiErr := &mcpAPIError{}
	if err := json.Unmarshal(raw, apiErr); err == nil && apiErr.Error != "" {
		return fmt.Errorf("croupier REST %d: %s (code=%s)", status, apiErr.Message, apiErr.Error)
	}
	return fmt.Errorf("croupier REST %d: %s", status, strings.TrimSpace(string(raw)))
}

// mcpTextResult 把 REST payload 原文放进 text content（不二次建模）。
func mcpTextResult(raw json.RawMessage) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(raw)}},
	}
}
