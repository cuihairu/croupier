package functioninvoke

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/api/assignment"
	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/dispatch"
	"github.com/cuihairu/croupier/internal/platform/executionlog"
	"github.com/cuihairu/croupier/internal/platform/functioncontract"
	"github.com/cuihairu/croupier/internal/policy"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/cuihairu/croupier/internal/telemetry"
	"go.opentelemetry.io/otel/attribute"
)

// Invoke 是函数调用的唯一执行链（原 internal/api/function functionInvoke
// 逐行平移，语义不变）：scope 以 ctx 中间件解析值为准 → 路由语义前置校验 →
// 禁用拦截 → 分配闸门 → 遥测 → 策略审批/审计 → 输入契约校验 → 派发 →
// 审计与执行留痕。
func Invoke(ctx context.Context, svcCtx *svc.ServiceContext, req *Request) (*Response, error) {
	scope, err := svc.CurrentScope(ctx)
	if err != nil {
		return nil, errorx.NewBadRequest("游戏环境 scope 缺失")
	}
	// Scope is resolved and authorized by middleware. Request body values are
	// ignored so they can never redirect an invocation to another agent scope.
	req.GameID = scope.GameID
	req.Env = scope.Env
	// 路由语义前置校验（400 级）：targeted 缺 targetServiceId / hash 缺
	// hashKey / broadcast 配 async 属客户端错误，应在 policy 与 dispatch
	// 之前拦截，而不是静默回落 lb 或静默走 async。
	if err := ValidateInvokeRoute(req); err != nil {
		return nil, err
	}
	// 禁用拦截（E2）：dashboard 禁用按钮写 functions.status，但本路径从不
	// 回读，禁用函数照常执行。守卫在遥测/审计/执行留痕之前——被拒的调用
	// 不是一次执行，不产生失败执行记录污染成功率口径。
	if err := utils.EnsureFunctionEnabled(ctx, svcCtx, strings.TrimSpace(req.ID)); err != nil {
		return nil, err
	}
	// 分配闸门（BUG-032）：scope 存在分配记录时是执行白名单，未分配函数
	// 403 function_not_assigned（无记录默认开放）。与 EnsureFunctionEnabled
	// 同位、/tasks Start 共用，防止绕过。
	if err := assignment.EnsureFunctionAssigned(ctx, svcCtx, strings.TrimSpace(req.ID), req.GameID, req.Env); err != nil {
		return nil, err
	}
	startedAt := time.Now()
	var spanErr error
	if svcCtx != nil && svcCtx.Telemetry != nil {
		nextCtx, span := svcCtx.Telemetry.StartSpan(ctx, "function.invoke",
			attribute.String("function.id", req.ID),
			attribute.String("function.mode", strings.TrimSpace(req.Mode)),
			attribute.String("function.route", strings.TrimSpace(req.Route)),
			attribute.String("game.id", strings.TrimSpace(req.GameID)),
			attribute.String("game.env", strings.TrimSpace(req.Env)),
		)
		// 归流桥：函数调用是分析管道的核心业务事件（T12 断点④——
		// 此前 bridge 建好但发送端从未接线，事件从未进过 ClickHouse）
		svcCtx.Telemetry.BridgeFunctionCall(nextCtx, "function.call",
			attribute.String("function.id", req.ID),
			attribute.String("function.mode", strings.TrimSpace(req.Mode)),
			attribute.String("function.route", strings.TrimSpace(req.Route)),
			attribute.String("game.id", strings.TrimSpace(req.GameID)),
			attribute.String("game.env", strings.TrimSpace(req.Env)),
		)
		defer func() {
			eventType := "function.call.success"
			completionAttrs := []attribute.KeyValue{
				attribute.String("function.id", req.ID),
				attribute.String("game.id", strings.TrimSpace(req.GameID)),
				attribute.String("game.env", strings.TrimSpace(req.Env)),
			}
			if spanErr != nil {
				eventType = "function.call.error"
				completionAttrs = append(completionAttrs, attribute.String("error.message", spanErr.Error()))
			}
			svcCtx.Telemetry.BridgeFunctionCall(nextCtx, eventType, completionAttrs...)
			svcCtx.Telemetry.EndSpan(span, startedAt, spanErr)
		}()
		ctx = nextCtx
	}

	admin, roles, err := utils.LoadCurrentAdmin(ctx, svcCtx)
	if err != nil {
		spanErr = err
		return nil, err
	}
	_, err = utils.PermissionIDsFromRoles(ctx, svcCtx, roles)
	if err != nil {
		spanErr = err
		return nil, err
	}

	approvedContinuation := IsApprovedContinuation(req.Metadata)
	pageSnapshotGoverned := IsPageSnapshotGoverned(req.Metadata)
	var functionPolicy *policy.Policy
	// Apply function policy checks
	if svcCtx.PolicyManager != nil && !approvedContinuation && !pageSnapshotGoverned {
		roleNames := utils.RoleNamesFromModels(roles)
		functionPolicy, err = EnforceFunctionPolicy(ctx, svcCtx, req.ID, strings.TrimSpace(req.GameID), strings.TrimSpace(req.Env), roleNames)
		if err != nil {
			spanErr = err
			return nil, err
		}
	}

	payload := InvokePayload(req)

	// 输入契约前置校验：缺 required 参数属客户端错误，应在 dispatch 前
	// 400 并指明缺失字段，而不是透传 agent 后以 5xx 路由错误收场。
	if err := ValidateInvokeInput(ctx, svcCtx, req, payload); err != nil {
		spanErr = err
		return nil, err
	}

	// Check if approval is required
	if functionPolicy != nil && functionPolicy.RequireApproval && svcCtx.ApprovalsStore != nil {
		// Create approval request instead of executing directly
		approvalID, err := CreateFunctionApproval(ctx, svcCtx, req, payload, admin, functionPolicy)
		if err != nil {
			spanErr = fmt.Errorf("failed to create approval request: %w", err)
			return nil, spanErr
		}

		// Log approval creation if audit is enabled
		if svcCtx.AuditService != nil && functionPolicy.RequireAudit {
			AuditApprovalCreated(ctx, svcCtx, req.ID, admin, utils.RoleNamesFromModels(roles), approvalID, functionPolicy)
		}

		// Return approval response
		return &Response{
			TaskId:            "",
			ApprovalID:        approvalID,
			ApprovalRequired:  true,
			ApprovalWorkflow:  functionPolicy.ApprovalWorkflow,
			Result:            nil,
			TraceID:           telemetry.TraceIDFromContext(ctx),
			ExecutionMetadata: CloneMetadata(req.Metadata),
		}, nil
	}

	metadata := map[string]string{
		"async": "false",
	}
	for key, value := range req.Metadata {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		metadata[key] = value
	}
	if req.Mode == "async" {
		metadata["async"] = "true"
	}
	// 同步调用预算（毫秒）→ 约定键 timeout_ms：契约声明是上限，调用方
	// 显式值不得突破契约（声明即契约）；两者皆无则不注入（各层全局默认）。
	if req.Mode != "async" {
		if effective := EffectiveTimeoutMs(ctx, svcCtx, req); effective > 0 {
			metadata["timeoutMs"] = strconv.Itoa(effective)
		}
	}
	if gameID := strings.TrimSpace(req.GameID); gameID != "" {
		metadata["gameId"] = gameID
	}
	if env := strings.TrimSpace(req.Env); env != "" {
		metadata["env"] = env
	}
	if route := strings.TrimSpace(req.Route); route != "" {
		metadata["route"] = route
	}
	if targetServiceID := strings.TrimSpace(req.TargetServiceID); targetServiceID != "" {
		metadata["targetServiceId"] = targetServiceID
	}
	if hashKey := strings.TrimSpace(req.HashKey); hashKey != "" {
		metadata["hashKey"] = hashKey
	}

	// 记录操作者
	if admin != nil {
		metadata["actor"] = admin.Username
	}
	metadata = telemetry.InjectContext(ctx, metadata)

	var result *Response
	var invokeErr error

	if req.Mode == "async" {
		taskResp, err := svcCtx.Dispatcher.StartTaskRequest(ctx, utils.BuildInvokeRequest(req.ID, payload, metadata))
		if err != nil {
			invokeErr = err
		} else {
			result = &Response{
				TaskId:            taskResp.GetTaskId(),
				Result:            nil,
				TraceID:           telemetry.TraceIDFromContext(ctx),
				ExecutionMetadata: CloneMetadata(metadata),
			}
		}
	} else if strings.EqualFold(strings.TrimSpace(req.Route), "broadcast") {
		broadcast, err := svcCtx.Dispatcher.InvokeBroadcast(ctx, utils.BuildInvokeRequest(req.ID, payload, metadata))
		if err != nil {
			invokeErr = err
		} else {
			result = BuildBroadcastResponse(broadcast)
			result.TraceID = telemetry.TraceIDFromContext(ctx)
			result.ExecutionMetadata = CloneMetadata(metadata)
		}
	} else {
		resp, err := svcCtx.Dispatcher.InvokeRequest(ctx, utils.BuildInvokeRequest(req.ID, payload, metadata))
		if err != nil {
			invokeErr = err
		} else {
			result = &Response{
				TraceID:           telemetry.TraceIDFromContext(ctx),
				ExecutionMetadata: CloneMetadata(metadata),
			}
			if resp != nil && len(resp.GetPayload()) > 0 {
				result.Result = RawJSONFromBytes(resp.GetPayload())
			}
		}
	}

	// Audit logging: log function invocation if policy requires audit
	if svcCtx.AuditService != nil && functionPolicy != nil && functionPolicy.RequireAudit {
		invokeDurationMs := time.Since(startedAt).Milliseconds()
		AuditFunctionInvoke(ctx, svcCtx, req.ID, admin, utils.RoleNamesFromModels(roles), functionPolicy, invokeErr, invokeDurationMs)
	}

	// 执行留痕（R1）：payload 级请求/响应异步落库（与 require_audit 解耦）；
	// 页面绑定执行的内部 invoke 由 page 源记录承担，此处跳过避免双写
	if svcCtx.ExecutionLogWriter != nil && !executionlog.Skipped(ctx) {
		invokeDurationMs := time.Since(startedAt).Milliseconds()
		var actor string
		if admin != nil {
			actor = admin.Username
		}
		status := executionlog.StatusOK
		var responseBody interface{}
		if invokeErr != nil {
			status = executionlog.StatusFail
			responseBody = map[string]string{"error": invokeErr.Error()}
		} else if result != nil && len(result.Result) > 0 {
			responseBody = json.RawMessage(result.Result)
		}
		scope := svc.GameScopeFromContext(ctx)
		svcCtx.ExecutionLogWriter.Log(executionlog.Entry{
			GameID:     scope.GameID,
			Env:        scope.Env,
			Source:     executionlog.SourceInvoke,
			FunctionID: req.ID,
			Actor:      actor,
			Route:      strings.TrimSpace(req.Route),
			Status:     status,
			DurationMs: invokeDurationMs,
			TraceID:    telemetry.TraceIDFromContext(ctx),
			Request:    json.RawMessage(payload),
			Response:   responseBody,
		})
	}

	if invokeErr != nil {
		spanErr = invokeErr
		return &Response{
			TaskId:  "",
			Result:  nil,
			TraceID: telemetry.TraceIDFromContext(ctx),
		}, invokeErr
	}

	return result, nil
}

// ValidateInvokeInput 在 dispatch 前按函数 descriptor 的 inputSchema 校验
// payload。无 descriptor 或 schema 不可编译时透传；可编译契约则执行完整的
// JSON Schema 校验，避免 HTTP API 与 logic 入口各自解释同一份契约。
func ValidateInvokeInput(ctx context.Context, svcCtx *svc.ServiceContext, req *Request, payload []byte) error {
	if svcCtx == nil || svcCtx.FunctionModel == nil {
		return nil
	}
	functionID := strings.TrimSpace(req.ID)
	if functionID == "" {
		return nil
	}
	descs, err := svcCtx.FunctionModel.ListDescriptors(ctx, functionID)
	if err != nil || len(descs) == 0 {
		return nil
	}
	return functioncontract.ValidateInput(descs[0].Input, payload) // ListDescriptors 按版本倒序，取最新契约
}

// ValidateInvokeRoute enforces routing semantics before policy and dispatch.
// Both production entries (HTTP and the logic thin shell) share this gate so
// targeted calls can never silently fall back to load balancing.
func ValidateInvokeRoute(req *Request) error {
	if req == nil {
		return errorx.NewBadRequest("invoke request is required")
	}

	route := strings.ToLower(strings.TrimSpace(req.Route))
	if route == "" {
		route = "lb"
	}
	switch route {
	case "lb":
	case "targeted":
		if strings.TrimSpace(req.TargetServiceID) == "" {
			return errorx.NewBadRequest("target_service_id is required for route=targeted")
		}
	case "hash":
		if strings.TrimSpace(req.HashKey) == "" {
			return errorx.NewBadRequest("hash_key is required for route=hash")
		}
	case "broadcast":
		mode := strings.ToLower(strings.TrimSpace(req.Mode))
		if mode == "async" || mode == "task" || mode == "start_task" {
			return errorx.NewBadRequest("route=broadcast is only supported for synchronous invoke")
		}
	default:
		return errorx.NewBadRequest("invalid route " + route)
	}
	req.Route = route
	return nil
}

func CloneMetadata(metadata map[string]string) map[string]string {
	if len(metadata) == 0 {
		return nil
	}
	out := make(map[string]string, len(metadata))
	for key, value := range metadata {
		out[key] = value
	}
	return out
}

func IsApprovedContinuation(metadata map[string]string) bool {
	if metadata == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(metadata["approvalBypass"]), "approved")
}

// IsPageSnapshotGoverned 报告调用是否由页面快照治理（页面级门禁已前置执行，
// 函数级 policy 审批闸须跳过以免二次审批）。两种治理来源：
//   - "validated"：已发布页面快照（审批续跑回读快照版本校验新鲜度）；
//   - "draft"：草稿预览执行（页面无快照，审批续跑按契约版本校验新鲜度）。
func IsPageSnapshotGoverned(metadata map[string]string) bool {
	if metadata == nil {
		return false
	}
	governance := strings.TrimSpace(metadata["pageSnapshotGovernance"])
	return strings.EqualFold(governance, "validated") || strings.EqualFold(governance, "draft")
}

// EffectiveTimeoutMs 计算同步调用的注入预算：
//
//	explicit = req.TimeoutMs（调用方显式声明）
//	declared = 契约 FunctionContract.TimeoutMs（函数契约声明）
//
// 语义（声明即契约）：显式值不得突破契约上限 → effective = min(explicit,
// declared)（两者皆有）；只有契约 → declared；只有显式 → explicit；
// 都无 → 0（不注入，各层沿用全局默认）。契约读取失败按未声明处理
// （查询问题不应阻断调用——超时预算是优化项不是准入项）。
func EffectiveTimeoutMs(ctx context.Context, svcCtx *svc.ServiceContext, req *Request) int {
	explicit := req.TimeoutMs
	if explicit < 0 {
		explicit = 0
	}
	declared := DeclaredTimeoutMs(ctx, svcCtx, req)
	switch {
	case declared > 0 && explicit > 0:
		return min(explicit, declared)
	case declared > 0:
		return declared
	default:
		return explicit
	}
}

// DeclaredTimeoutMs 读取函数契约声明的同步预算（毫秒）；无契约/无声明/
// 读取失败 → 0。
func DeclaredTimeoutMs(ctx context.Context, svcCtx *svc.ServiceContext, req *Request) int {
	if svcCtx == nil || svcCtx.DB == nil {
		return 0
	}
	gameID := strings.TrimSpace(req.GameID)
	env := strings.TrimSpace(req.Env)
	if gameID == "" || env == "" {
		return 0
	}
	contract, err := model.NewFunctionContractModel(svcCtx.DB).
		FindByScopeAndFunctionID(ctx, gameID, env, strings.TrimSpace(req.ID))
	if err != nil || contract == nil {
		return 0
	}
	return int(contract.TimeoutMs)
}

// InvokePayload 取调用载荷：payload 优先、params 次之、空对象兜底。
func InvokePayload(req *Request) []byte {
	if req == nil {
		return []byte("null")
	}
	if len(req.Payload) > 0 {
		return append([]byte(nil), req.Payload...)
	}
	if len(req.Params) > 0 {
		return append([]byte(nil), req.Params...)
	}
	return []byte("{}")
}

// BuildBroadcastResponse aggregates per-agent outcomes from a broadcast
// invocation. The legacy Result field is populated with the first successful
// response so existing clients that don't know about Broadcast keep working.
func BuildBroadcastResponse(b *dispatch.BroadcastInvocation) *Response {
	if b == nil {
		return &Response{Broadcast: &BroadcastResult{}}
	}

	out := &Response{
		Broadcast: &BroadcastResult{
			Total:   b.Total,
			Success: len(b.Successes),
			Failure: len(b.Failures),
			Results: make([]BroadcastAgentItem, 0, b.Total),
		},
	}

	for _, s := range b.Successes {
		item := BroadcastAgentItem{AgentID: s.AgentID}
		if s.Response != nil && len(s.Response.GetPayload()) > 0 {
			item.Result = RawJSONFromBytes(s.Response.GetPayload())
			if out.Result == nil {
				out.Result = item.Result
			}
		}
		out.Broadcast.Results = append(out.Broadcast.Results, item)
	}

	for _, f := range b.Failures {
		out.Broadcast.Results = append(out.Broadcast.Results, BroadcastAgentItem{
			AgentID: f.AgentID,
			Error:   f.Err.Error(),
		})
	}

	return out
}
