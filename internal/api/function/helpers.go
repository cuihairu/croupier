package function

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/cluster"
	"github.com/cuihairu/croupier/internal/common/errorx"
	logicfunction "github.com/cuihairu/croupier/internal/logic/function"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/dispatch"
	functioninvoke "github.com/cuihairu/croupier/internal/platform/functioninvoke"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/policy"
	"github.com/cuihairu/croupier/internal/svc"
)

// Function management implementations

func functionsList(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionsListRequest) (*FunctionsListResponse, error) {
	scope := currentFunctionScope(ctx)
	gameID := strings.TrimSpace(req.GameId)
	if scope.GameID != "" {
		gameID = scope.GameID
	}
	logicResp, err := logicfunction.NewFunctionsListLogic(ctx, svcCtx).FunctionsList(&logicfunction.FunctionsListRequest{
		Page:     1,
		PageSize: 10000,
		GameId:   gameID,
		Resource: req.Resource,
		Status:   req.Status,
	})
	if err != nil {
		return nil, err
	}

	dbItems, _, err := svcCtx.FunctionModel.List(ctx, model.ListFunctionsOptions{
		PaginationOptions: model.NewPagination(1, 10000),
		GameID:            gameID,
	})
	if err != nil {
		return nil, err
	}
	dbIndex := make(map[string]model.Function, len(dbItems))
	for _, item := range dbItems {
		dbIndex[item.FunctionID] = item
	}

	items := make([]Function, 0, len(logicResp.Items))
	for _, fn := range logicResp.Items {
		dbFn, ok := dbIndex[fn.ID]
		resource := fn.Resource
		version := fn.Version
		specFormat := fn.SpecFormat
		openAPISpec := rawJSONFromAny(fn.OpenAPISpec)
		description := fn.Description
		createdAt := ""
		updatedAt := ""
		if ok {
			if resource == "" {
				resource = getStringFromMetadata(dbFn.Metadata, "resource")
			}
			if version == "" {
				version = getStringFromMetadata(dbFn.Metadata, "version")
			}
			if specFormat == "" {
				specFormat = getStringFromMetadata(dbFn.Metadata, "spec_format")
			}
			if len(openAPISpec) == 0 {
				openAPISpec = rawJSONFromAny(jsonValueFromMetadata(dbFn.Metadata, "openapi_spec"))
			}
			if description == "" {
				description = dbFn.Description
			}
			createdAt = utils.FormatTimestamp(dbFn.CreatedAt)
			updatedAt = utils.FormatTimestamp(dbFn.UpdatedAt)
		}
		items = append(items, Function{
			Id:          fn.ID,
			Name:        fn.Name,
			Description: description,
			Resource:    resource,
			GameId:      fn.GameId,
			Status:      fn.Status,
			Version:     version,
			Instances:   fn.Instances,
			SpecFormat:  specFormat,
			Tags:        firstNonEmptySlice(getStringSliceFromMetadata(dbFn.Metadata, "tags"), fn.Tags),
			Summary:     firstNonEmptyMap(getLocalizedTextFromMetadata(dbFn.Metadata, "summary"), fn.Summary),
			OpenAPISpec: openAPISpec,
			CreatedAt:   createdAt,
			UpdatedAt:   updatedAt,
		})
	}

	return &FunctionsListResponse{
		Items: items,
		Total: logicResp.Total,
		Page:  logicResp.Page,
		Size:  logicResp.Size,
	}, nil
}

func functionsPending(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionsPendingRequest) (*FunctionsPendingResponse, error) {
	// Implementation would query pending functions
	return &FunctionsPendingResponse{
		Items: []PendingFunction{},
	}, nil
}

func functionDetail(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionDetailRequest) (*FunctionDetailResponse, error) {
	logicResp, err := logicfunction.NewFunctionDetailLogic(ctx, svcCtx).FunctionDetail(&logicfunction.FunctionDetailRequest{
		ID: req.ID,
	})
	if err != nil {
		return nil, err
	}

	return &FunctionDetailResponse{
		Function: Function{
			Id:          logicResp.Function.ID,
			Name:        logicResp.Function.Name,
			Description: logicResp.Function.Description,
			Resource:    logicResp.Function.Resource,
			GameId:      logicResp.Function.GameId,
			Status:      logicResp.Function.Status,
			Version:     logicResp.Function.Version,
			Instances:   logicResp.Function.Instances,
			SpecFormat:  logicResp.Function.SpecFormat,
			OpenAPISpec: rawJSONFromBytes(logicResp.Function.OpenAPISpec),
			CreatedAt:   logicResp.Function.CreatedAt,
			UpdatedAt:   logicResp.Function.UpdatedAt,
		},
		Descriptor: FunctionDescriptor{
			Input:  rawJSONFromBytes(logicResp.Descriptor.Input),
			Output: rawJSONFromBytes(logicResp.Descriptor.Output),
			Schema: rawJSONFromBytes(logicResp.Descriptor.Schema),
		},
	}, nil
}

func functionAnalytics(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionAnalyticsRequest) (*FunctionAnalyticsResponse, error) {
	stats, err := logicfunction.NewFunctionAnalyticsLogic(ctx, svcCtx).FunctionAnalytics(
		&logicfunction.FunctionAnalyticsRequest{ID: req.ID},
	)
	if err != nil {
		return nil, err
	}
	return &FunctionAnalyticsResponse{
		TotalCalls:     stats.TotalCalls,
		FailedCalls:    stats.FailedCalls,
		SuccessRate:    stats.SuccessRate,
		AvgLatency:     stats.AvgLatency,
		CallsToday:     stats.CallsToday,
		CallsThisWeek:  stats.CallsThisWeek,
		CallsThisMonth: stats.CallsThisMonth,
	}, nil
}

func functionCopy(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionCopyRequest) (*FunctionCopyResponse, error) {
	// Implementation would copy function to target env
	return &FunctionCopyResponse{
		FunctionId: req.ID,
		NewId:      "",
	}, nil
}

func functionDelete(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionDeleteRequest) error {
	return svcCtx.FunctionModel.DeleteFunction(ctx, req.FunctionId)
}

func functionDisable(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionDisableRequest) error {
	return setFunctionEnabled(ctx, svcCtx, req.FunctionId, 0)
}

func functionEnable(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionEnableRequest) error {
	return setFunctionEnabled(ctx, svcCtx, req.FunctionId, 1)
}

// setFunctionEnabled flips a function's enabled status by its external
// function_id. The previous implementation passed the numeric primary key 0
// (a placeholder that matches no row), so enable/disable silently did
// nothing while reporting success.
func setFunctionEnabled(ctx context.Context, svcCtx *svc.ServiceContext, functionID string, status int) error {
	functionID = strings.TrimSpace(functionID)
	if functionID == "" {
		return errorx.NewBadRequest("functionId is required")
	}
	fn, err := svcCtx.FunctionModel.FindByFunctionID(ctx, functionID)
	if err != nil {
		return fmt.Errorf("find function %s: %w", functionID, err)
	}
	if err := svcCtx.FunctionModel.Update(ctx, fn.ID, map[string]interface{}{"status": status}); err != nil {
		return fmt.Errorf("update function %s status: %w", functionID, err)
	}
	return nil
}

// fetchFunctionHistoryPage delegates to the history logic; it is a package
// seam letting tests exercise nil-item handling without a full logic stack.
var fetchFunctionHistoryPage = func(ctx context.Context, svcCtx *svc.ServiceContext, req *logicfunction.FunctionHistoryRequest) ([]logicfunction.FunctionHistoryItem, int, error) {
	return logicfunction.NewFunctionHistoryLogic(ctx, svcCtx).FunctionHistoryPaged(req)
}

func functionHistory(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionHistoryRequest) (*FunctionHistoryResponse, error) {
	// 历史默认取最近一页；limit/offset 由 query 传入
	limit := req.Limit
	if limit <= 0 {
		limit = functionHistoryDefaultLimit
	}
	if limit > functionHistoryMaxLimit {
		limit = functionHistoryMaxLimit
	}
	offset := req.Offset
	if offset < 0 {
		offset = 0
	}
	items, total, err := fetchFunctionHistoryPage(ctx, svcCtx, &logicfunction.FunctionHistoryRequest{ID: req.ID, Limit: limit, Offset: offset})
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []logicfunction.FunctionHistoryItem{}
	}
	result := make([]FunctionHistoryItem, 0, len(items))
	for _, item := range items {
		result = append(result, FunctionHistoryItem{
			ID: item.ID, Action: item.Action, Operator: item.Operator,
			Timestamp: item.Timestamp, Details: rawJSONFromBytes(item.Details),
		})
	}
	return &FunctionHistoryResponse{Items: result, Total: total}, nil
}

// --- 统一执行链薄壳（internal/platform/functioninvoke）---
//
// functionInvoke 的完整执行链（scope、禁用、分配、策略审批、审计、执行
// 留痕）已上收为唯一实现 functioninvoke.Invoke，HTTP 入口与 logic 侧薄壳
// 共用同一实现。本包只保留 API DTO 适配与既有用例依赖的私有 helper 转发。

func functionInvoke(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionInvokeRequest) (*FunctionInvokeResponse, error) {
	shared := functioninvokeRequestFromDTO(req)
	resp, err := functioninvoke.Invoke(ctx, svcCtx, shared)
	// 统一链会把 GameID/Env 覆写为 ctx scope 值并归一化 Route；同步回 DTO
	// 保持收口前入口行为（调用方可读到生效 scope）。
	req.GameID, req.Env, req.Route = shared.GameID, shared.Env, shared.Route
	if err != nil {
		// 派发失败同样返回可追踪的空响应（与收口前行为一致）。
		if resp != nil {
			return functionInvokeResponseFromUnified(resp), err
		}
		return nil, err
	}
	return functionInvokeResponseFromUnified(resp), nil
}

func functioninvokeRequestFromDTO(req *FunctionInvokeRequest) *functioninvoke.Request {
	if req == nil {
		return &functioninvoke.Request{}
	}
	return &functioninvoke.Request{
		ID:              req.ID,
		Params:          req.Params,
		Payload:         req.Payload,
		GameID:          req.GameID,
		Env:             req.Env,
		Mode:            req.Mode,
		Route:           req.Route,
		TargetServiceID: req.TargetServiceID,
		HashKey:         req.HashKey,
		TimeoutMs:       req.TimeoutMs,
		Metadata:        req.Metadata,
	}
}

func functionInvokeResponseFromUnified(resp *functioninvoke.Response) *FunctionInvokeResponse {
	if resp == nil {
		return nil
	}
	out := &FunctionInvokeResponse{
		TaskId:            resp.TaskId,
		Result:            resp.Result,
		ApprovalID:        resp.ApprovalID,
		ApprovalRequired:  resp.ApprovalRequired,
		ApprovalWorkflow:  resp.ApprovalWorkflow,
		TraceID:           resp.TraceID,
		ExecutionMetadata: resp.ExecutionMetadata,
	}
	if resp.Broadcast != nil {
		items := make([]BroadcastAgentItem, 0, len(resp.Broadcast.Results))
		for _, item := range resp.Broadcast.Results {
			items = append(items, BroadcastAgentItem{AgentID: item.AgentID, Result: item.Result, Error: item.Error})
		}
		out.Broadcast = &BroadcastResult{
			Total:   resp.Broadcast.Total,
			Success: resp.Broadcast.Success,
			Failure: resp.Broadcast.Failure,
			Results: items,
		}
	}
	return out
}

func validateInvokeInput(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionInvokeRequest, payload []byte) error {
	return functioninvoke.ValidateInvokeInput(ctx, svcCtx, functioninvokeRequestFromDTO(req), payload)
}

func validateInvokeRoute(req *FunctionInvokeRequest) error {
	if req == nil {
		return errorx.NewBadRequest("invoke request is required")
	}
	shared := functioninvokeRequestFromDTO(req)
	if err := functioninvoke.ValidateInvokeRoute(shared); err != nil {
		return err
	}
	req.Route = shared.Route
	return nil
}

func cloneMetadata(metadata map[string]string) map[string]string {
	return functioninvoke.CloneMetadata(metadata)
}

func isApprovedContinuation(metadata map[string]string) bool {
	return functioninvoke.IsApprovedContinuation(metadata)
}

func isPageSnapshotGoverned(metadata map[string]string) bool {
	return functioninvoke.IsPageSnapshotGoverned(metadata)
}

func effectiveTimeoutMs(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionInvokeRequest) int {
	return functioninvoke.EffectiveTimeoutMs(ctx, svcCtx, functioninvokeRequestFromDTO(req))
}

func declaredTimeoutMs(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionInvokeRequest) int {
	return functioninvoke.DeclaredTimeoutMs(ctx, svcCtx, functioninvokeRequestFromDTO(req))
}

func auditFunctionInvoke(ctx context.Context, svcCtx *svc.ServiceContext, functionID string, admin *model.Admin, userRoles []string, functionPolicy *policy.Policy, invokeErr error, durationMs int64) {
	functioninvoke.AuditFunctionInvoke(ctx, svcCtx, functionID, admin, userRoles, functionPolicy, invokeErr, durationMs)
}

func createFunctionApproval(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionInvokeRequest, payload []byte, admin *model.Admin, functionPolicy *policy.Policy) (string, error) {
	return functioninvoke.CreateFunctionApproval(ctx, svcCtx, functioninvokeRequestFromDTO(req), payload, admin, functionPolicy)
}

func approvalNotifyRecipients(ctx context.Context, svcCtx *svc.ServiceContext) ([]string, error) {
	return functioninvoke.ApprovalNotifyRecipients(ctx, svcCtx)
}

func dedupeStrings(in []string) []string {
	return functioninvoke.DedupeStrings(in)
}

func auditApprovalCreated(ctx context.Context, svcCtx *svc.ServiceContext, functionID string, admin *model.Admin, userRoles []string, approvalID string, functionPolicy *policy.Policy) {
	functioninvoke.AuditApprovalCreated(ctx, svcCtx, functionID, admin, userRoles, approvalID, functionPolicy)
}

func functionPublish(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionPublishRequest) (*FunctionPublishResponse, error) {
	// Implementation would publish function to approval queue
	return &FunctionPublishResponse{
		ApprovalId: "",
		Published:  true,
	}, nil
}

// Instance management implementations

// sessionMatchesScope 判断会话是否落入 scope（空 scope 全通过）。
func sessionMatchesScope(scope svc.GameScope, gameID, env string) bool {
	if scope.GameID == "" {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(gameID), scope.GameID) &&
		strings.EqualFold(strings.TrimSpace(env), scope.Env)
}

// remoteAgentSnapshot 是归属表兜底出的远端 agent 会话快照及其 owner 实例。
type remoteAgentSnapshot struct {
	sess          *reg.AgentSession
	ownerInstance string
}

// clusterOwnerRecords 读归属表全集。集群未启用（Cluster/ListAgentOwners
// 未装配）或读失败返回 nil，instances 视图回落本地语义。
func clusterOwnerRecords(ctx context.Context, svcCtx *svc.ServiceContext) []cluster.AgentOwnerRecord {
	if svcCtx.Cluster == nil || svcCtx.Cluster.ListAgentOwners == nil {
		return nil
	}
	recs, err := svcCtx.Cluster.ListAgentOwners(ctx)
	if err != nil {
		return nil
	}
	return recs
}

// clusterSelfInstanceID 返回本实例的集群身份（未装配返回空）。
func clusterSelfInstanceID(svcCtx *svc.ServiceContext) string {
	if svcCtx.Cluster == nil {
		return ""
	}
	return svcCtx.Cluster.InstanceID
}

// remoteOwnerMap 从归属表构建 agentID → 对端实例标注：本地 registry 里
// 由对端持有的 DB 快照（refreshRemoteSnapshots 回灌的）标真归属，本实例
// 自持的排除（前端渲染「本实例」）。selfInstanceID 为空（未装配集群身份）
// 时无法区分自持，整体不标注。
func remoteOwnerMap(owners []cluster.AgentOwnerRecord, selfInstanceID string) map[string]string {
	if len(owners) == 0 || selfInstanceID == "" {
		return nil
	}
	out := make(map[string]string, len(owners))
	for _, rec := range owners {
		if rec.InstanceID == selfInstanceID {
			continue
		}
		out[rec.AgentID] = rec.InstanceID
	}
	return out
}

// remoteAgentSnapshots 返回「归属表活跃但本地 registry 未持有」的 agent
// 会话快照——集群模式下 instances 视图的跨实例兜底来源：本地 registry 只
// 含连到本实例的 agent，对端实例持有的 agent 以共享归属表
// （cluster_agent_owners，TTL 内即活跃）为在线全集，明细从共享
// agent_sessions 快照表读取（Providers/Functions 已 JSON 落库）。归属表
// 不可达或快照行缺失（owner 已 Release 的竞态）时静默回落本地视图，
// 读路径不升级成 5xx。非集群部署（Cluster/DB 未装配）恒返回空。
// owners 由调用方一次读出共用（本地条目标注也要用，避免双查归属表）。
func remoteAgentSnapshots(ctx context.Context, svcCtx *svc.ServiceContext, scope svc.GameScope, localAgentIDs map[string]struct{}, owners []cluster.AgentOwnerRecord) []remoteAgentSnapshot {
	if svcCtx.Cluster == nil || svcCtx.Cluster.ListAgentOwners == nil || svcCtx.DB == nil {
		return nil
	}
	missing := make([]string, 0, len(owners))
	for _, rec := range owners {
		if _, ok := localAgentIDs[rec.AgentID]; ok {
			continue
		}
		if !sessionMatchesScope(scope, rec.GameID, rec.Env) {
			continue
		}
		missing = append(missing, rec.AgentID)
	}
	if len(missing) == 0 {
		return nil
	}
	sessions, err := reg.NewAgentSessionModel(svcCtx.DB).LoadActiveSessionsByAgentIDs(ctx, missing)
	if err != nil {
		return nil
	}
	ownerByAgent := make(map[string]string, len(owners))
	for _, rec := range owners {
		ownerByAgent[rec.AgentID] = rec.InstanceID
	}
	out := make([]remoteAgentSnapshot, 0, len(sessions))
	// 设计债清理：sessions 来自 LoadActiveSessionsByAgentIDs，其元素由
	// toDomainSession 产出（err!=nil 时被跳过不 append），恒非 nil，
	// 原 nil 防御分支不可达已删除。
	for _, sess := range sessions {
		out = append(out, remoteAgentSnapshot{sess: sess, ownerInstance: ownerByAgent[sess.AgentID]})
	}
	return out
}

// instanceItemForFunction 判断会话是否注册了目标函数并产出条目。
func instanceItemForFunction(sess *reg.AgentSession, scope svc.GameScope, functionID, ownerInstance string) (FunctionInstance, bool) {
	if sess == nil || !sessionMatchesScope(scope, sess.GameID, sess.Env) {
		return FunctionInstance{}, false
	}
	if _, ok := sess.Functions[functionID]; ok {
		return FunctionInstance{
			AgentId:       sess.AgentID,
			AgentName:     sess.AgentID, // Use AgentID as name since AgentName doesn't exist
			Status:        "active",
			UpdatedAt:     utils.FormatTimestamp(sess.LastSeen),
			OwnerInstance: ownerInstance,
		}, true
	}
	return FunctionInstance{}, false
}

// appendInstanceSummaries 把单个 agent 会话的函数注册展开为实例条目：
// provider 会话优先（携带 ServiceID/SDK/addr 明细），未被任何 provider
// 声明的函数仍以 agent 级条目兜底。scope 不匹配的会话/provider 跳过。
func appendInstanceSummaries(instances []FunctionInstanceSummary, sess *reg.AgentSession, scope svc.GameScope, ownerInstance string) []FunctionInstanceSummary {
	if sess == nil || !sessionMatchesScope(scope, sess.GameID, sess.Env) {
		return instances
	}
	claimed := map[string]struct{}{}
	for i := range sess.Providers {
		prov := sess.Providers[i]
		seen := map[string]struct{}{}
		for _, fid := range prov.FunctionIDs {
			if _, ok := seen[fid]; ok {
				continue
			}
			seen[fid] = struct{}{}
			if scope.GameID != "" && !strings.EqualFold(strings.TrimSpace(prov.GameID), scope.GameID) {
				continue
			}
			instances = append(instances, FunctionInstanceSummary{
				FunctionID:    fid,
				AgentID:       sess.AgentID,
				AgentName:     sess.AgentID,
				ServiceID:     prov.ProviderID,
				Addr:          prov.Addr,
				Version:       prov.Version,
				SDKName:       prov.SDKName,
				SDKLang:       prov.SDKLanguage,
				SDKVersion:    prov.SDKVersion,
				AgentVersion:  sess.Version,
				GameID:        sess.GameID,
				Env:           sess.Env,
				Status:        "active",
				UpdatedAt:     utils.FormatTimestamp(sess.LastSeen),
				OwnerInstance: ownerInstance,
			})
			claimed[fid] = struct{}{}
		}
	}
	for fid := range sess.Functions {
		if _, ok := claimed[fid]; ok {
			continue
		}
		instances = append(instances, FunctionInstanceSummary{
			FunctionID:    fid,
			AgentID:       sess.AgentID,
			AgentName:     sess.AgentID,
			AgentVersion:  sess.Version,
			GameID:        sess.GameID,
			Env:           sess.Env,
			Status:        "active",
			UpdatedAt:     utils.FormatTimestamp(sess.LastSeen),
			OwnerInstance: ownerInstance,
		})
	}
	return instances
}

func functionInstances(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionInstancesRequest) (*FunctionInstancesResponse, error) {
	scope := currentFunctionScope(ctx)
	store := svcCtx.RegistryStore
	if store == nil {
		return &FunctionInstancesResponse{Items: []FunctionInstance{}}, nil
	}

	// 本地条目在读锁内构建（会话对象与 registry 并发写共享，不能锁外
	// 访问）；归属表在锁前一次读出——本地快照中被对端持有的条目也按
	// 归属表标注（refreshRemoteSnapshots 回灌的 DB 快照不是本实例直连），
	// 集群兜底 IO（归属表/快照表）在锁外。
	owners := clusterOwnerRecords(ctx, svcCtx)
	ownerOf := remoteOwnerMap(owners, clusterSelfInstanceID(svcCtx))
	items := []FunctionInstance{}
	localAgentIDs := map[string]struct{}{}
	store.Mu().RLock()
	for _, sess := range store.AgentsUnsafe() {
		if sess == nil {
			continue
		}
		localAgentIDs[sess.AgentID] = struct{}{}
		if item, ok := instanceItemForFunction(sess, scope, req.ID, ownerOf[sess.AgentID]); ok {
			items = append(items, item)
		}
	}
	store.Mu().RUnlock()

	for _, snap := range remoteAgentSnapshots(ctx, svcCtx, scope, localAgentIDs, owners) {
		if item, ok := instanceItemForFunction(snap.sess, scope, req.ID, snap.ownerInstance); ok {
			items = append(items, item)
		}
	}

	return &FunctionInstancesResponse{
		Items: items,
	}, nil
}

func functionInstancesAll(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionInstancesAllRequest) (*FunctionInstancesAllResponse, error) {
	scope := currentFunctionScope(ctx)
	store := svcCtx.RegistryStore
	if store == nil {
		return &FunctionInstancesAllResponse{Instances: []FunctionInstanceSummary{}, Total: 0}, nil
	}

	// 同 functionInstances：本地条目持读锁构建（对端持有的 DB 快照按
	// 归属表标注），集群兜底 IO 在锁外。
	owners := clusterOwnerRecords(ctx, svcCtx)
	ownerOf := remoteOwnerMap(owners, clusterSelfInstanceID(svcCtx))
	instances := []FunctionInstanceSummary{}
	localAgentIDs := map[string]struct{}{}
	store.Mu().RLock()
	for _, sess := range store.AgentsUnsafe() {
		if sess == nil {
			continue
		}
		localAgentIDs[sess.AgentID] = struct{}{}
		instances = appendInstanceSummaries(instances, sess, scope, ownerOf[sess.AgentID])
	}
	store.Mu().RUnlock()

	for _, snap := range remoteAgentSnapshots(ctx, svcCtx, scope, localAgentIDs, owners) {
		instances = appendInstanceSummaries(instances, snap.sess, scope, snap.ownerInstance)
	}

	sort.Slice(instances, func(i, j int) bool {
		if instances[i].FunctionID != instances[j].FunctionID {
			return instances[i].FunctionID < instances[j].FunctionID
		}
		return instances[i].ServiceID < instances[j].ServiceID
	})

	return &FunctionInstancesAllResponse{
		Instances: instances,
		Total:     len(instances),
	}, nil
}

// Permission management implementations

func functionPermissions(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionPermissionsRequest) (*FunctionPermissionsResponse, error) {
	perms, err := svcCtx.FunctionModel.ListPermissions(ctx, req.ID)
	if err != nil {
		return nil, err
	}

	items := make([]FunctionPermission, 0, len(perms))
	for _, p := range perms {
		var roles []string
		if len(p.Roles) > 0 {
			roles = parseRolesFromJSON(p.Roles)
		}
		var actions []string
		if p.Actions != nil {
			actions = parseActionsFromJSON(p.Actions)
		}
		items = append(items, FunctionPermission{
			Resource: p.Resource,
			Actions:  actions,
			Roles:    roles,
		})
	}

	return &FunctionPermissionsResponse{
		Items: items,
	}, nil
}

func functionPermissionsUpdate(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionPermissionsUpdateRequest) error {
	// Convert API permissions to model permissions
	var modelPerms []model.FunctionPermission
	for _, p := range req.Permissions {
		rolesJSON, _ := json.Marshal(p.Roles)
		actionsJSON, _ := json.Marshal(p.Actions)
		modelPerms = append(modelPerms, model.FunctionPermission{
			FunctionID: req.ID,
			Resource:   p.Resource,
			Roles:      rolesJSON,
			Actions:    actionsJSON,
		})
	}
	return svcCtx.FunctionModel.ReplacePermissions(ctx, req.ID, modelPerms)
}

func functionWarnings(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionWarningsRequest) (*FunctionWarningsResponse, error) {
	scope := currentFunctionScope(ctx)

	if svcCtx.RegistryStore == nil {
		return &FunctionWarningsResponse{Items: []FunctionWarningItem{}}, nil
	}

	filter := reg.RegistrationWarningFilter{
		GameID:     scope.GameID,
		Env:        scope.Env,
		FunctionID: req.FunctionID,
		AgentID:    req.AgentID,
		Code:       req.Code,
		Limit:      req.Limit,
	}

	warnings := svcCtx.RegistryStore.ListRegistrationWarnings(filter)

	items := make([]FunctionWarningItem, 0, len(warnings))
	for _, w := range warnings {
		items = append(items, FunctionWarningItem{
			Key:        w.Key,
			GameID:     w.GameID,
			Env:        w.Env,
			AgentID:    w.AgentID,
			FunctionID: w.FunctionID,
			Version:    w.Version,
			Code:       w.Code,
			Message:    w.Message,
			Count:      w.Count,
			FirstSeen:  w.FirstSeen.Format(time.RFC3339),
			LastSeen:   w.LastSeen.Format(time.RFC3339),
		})
	}

	return &FunctionWarningsResponse{
		Items: items,
	}, nil
}

// Descriptors implementations

func descriptors(ctx context.Context, svcCtx *svc.ServiceContext, req *DescriptorsRequest) (*DescriptorsResponse, error) {
	scope := currentFunctionScope(ctx)
	gameID := strings.TrimSpace(req.GameId)
	if scope.GameID != "" {
		gameID = scope.GameID
	}
	descs, err := svcCtx.FunctionModel.ListDescriptors(ctx, gameID)
	if err != nil {
		return nil, err
	}

	items := make([]Descriptor, 0, len(descs))
	for _, d := range descs {
		items = append(items, Descriptor{
			Id:          d.FunctionID,
			Name:        "", // FunctionDescriptor doesn't have Name
			Description: "", // FunctionDescriptor doesn't have Description
			Input:       rawJSONFromAny(d.Input),
			Output:      rawJSONFromAny(d.Output),
		})
	}

	return &DescriptorsResponse{
		Items: items,
	}, nil
}

func currentFunctionScope(ctx context.Context) svc.GameScope {
	return svc.GameScopeFromContext(ctx)
}

// Batch operations implementations

func batchCopyFunctions(ctx context.Context, svcCtx *svc.ServiceContext, req *BatchCopyFunctionsRequest) (*BatchCopyFunctionsResponse, error) {
	results := make([]FunctionCopyResponse, 0, len(req.Functions))
	for _, f := range req.Functions {
		result, _ := functionCopy(ctx, svcCtx, &f)
		if result != nil {
			results = append(results, *result)
		}
	}
	return &BatchCopyFunctionsResponse{
		Results: results,
	}, nil
}

func batchDeleteFunctions(ctx context.Context, svcCtx *svc.ServiceContext, req *BatchDeleteFunctionsRequest) (*BatchDeleteFunctionsResponse, error) {
	deleted := []string{}
	failed := []string{}

	for _, id := range req.FunctionIds {
		if err := functionDelete(ctx, svcCtx, &FunctionDeleteRequest{FunctionId: id}); err != nil {
			failed = append(failed, id)
		} else {
			deleted = append(deleted, id)
		}
	}

	return &BatchDeleteFunctionsResponse{
		Deleted: deleted,
		Failed:  failed,
	}, nil
}

func batchUpdateFunctions(ctx context.Context, svcCtx *svc.ServiceContext, req *BatchUpdateFunctionsRequest) (*BatchUpdateFunctionsResponse, error) {
	resp, err := logicfunction.NewBatchUpdateFunctionsLogic(ctx, svcCtx).BatchUpdateFunctions(&logicfunction.BatchUpdateFunctionsRequest{
		FunctionIds: req.FunctionIds,
		Enabled:     req.Enabled,
	})
	if err != nil {
		return nil, err
	}
	return &BatchUpdateFunctionsResponse{
		Updated: resp.Updated,
		Failed:  resp.Failed,
	}, nil
}

// Helper functions

// getStringFromMetadata gets a string value from metadata map
func getStringFromMetadata(metadata map[string]interface{}, key string) string {
	if metadata == nil {
		return ""
	}
	if val, ok := metadata[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// getIntFromMetadata gets an int value from metadata map
func getIntFromMetadata(metadata map[string]interface{}, key string) int {
	if metadata == nil {
		return 0
	}
	if val, ok := metadata[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case float64:
			return int(v)
		case string:
			// Try to parse string as int
			var i int
			if _, err := fmt.Sscanf(v, "%d", &i); err == nil {
				return i
			}
		}
	}
	return 0
}

// parseRolesFromJSON parses roles from JSON array or comma-separated string
func parseRolesFromJSON(data model.JSON) []string {
	if len(data) == 0 {
		return []string{}
	}
	// Try to parse as JSON array
	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		return arr
	}
	// Try to parse as comma-separated string
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		if str != "" {
			return strings.Split(str, ",")
		}
	}
	return []string{}
}

// parseActionsFromJSON parses actions from JSON array
func parseActionsFromJSON(data model.JSON) []string {
	if len(data) == 0 {
		return []string{}
	}
	// Try to parse as JSON array
	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		return arr
	}
	// Try to parse as comma-separated string
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		if str != "" {
			return strings.Split(str, ",")
		}
	}
	return []string{}
}

func jsonValueFromMetadata(metadata map[string]interface{}, key string) interface{} {
	if metadata == nil {
		return nil
	}
	if val, ok := metadata[key]; ok {
		return val
	}
	return nil
}

// getStringSliceFromMetadata gets a string slice value from metadata map
func getStringSliceFromMetadata(metadata map[string]interface{}, key string) []string {
	if metadata == nil {
		return []string{}
	}
	if val, ok := metadata[key]; ok {
		switch v := val.(type) {
		case []string:
			return v
		case []interface{}:
			result := make([]string, 0, len(v))
			for _, item := range v {
				if str, ok := item.(string); ok {
					result = append(result, str)
				}
			}
			return result
		}
	}
	return []string{}
}

// getBoolFromMetadata gets a bool value from metadata map
func getBoolFromMetadata(metadata map[string]interface{}, key string) bool {
	if metadata == nil {
		return false
	}
	if val, ok := metadata[key]; ok {
		switch v := val.(type) {
		case bool:
			return v
		case string:
			return v == "true" || v == "1"
		case int:
			return v != 0
		case float64:
			return v != 0
		}
	}
	return false
}

func invokePayload(req *FunctionInvokeRequest) []byte {
	if req == nil {
		return []byte("null")
	}
	return functioninvoke.InvokePayload(functioninvokeRequestFromDTO(req))
}

func rawJSONFromAny(value interface{}) json.RawMessage {
	return functioninvoke.RawJSONFromAny(value)
}

func rawJSONFromBytes(value []byte) json.RawMessage {
	return functioninvoke.RawJSONFromBytes(value)
}

func policyRiskFromContract(risk string) policy.RiskLevel {
	return functioninvoke.PolicyRiskFromContract(risk)
}

func resolvePolicyRisk(ctx context.Context, svcCtx *svc.ServiceContext, functionID, gameID, env string) (policy.RiskLevel, error) {
	return functioninvoke.ResolvePolicyRisk(ctx, svcCtx, functionID, gameID, env)
}

// ResolvePolicyRisk exposes resolvePolicyRisk to the approve path
// (internal/api/approval step-up gate, OPEN-ISSUES #75): 高危审批门槛必须与
// 调用路径共用同一风险判定口径，禁止各包私抄解析逻辑。来源故障向上传播，
// 调用侧按 fail-closed 处理（step-up 门槛视作高危）。
func ResolvePolicyRisk(ctx context.Context, svcCtx *svc.ServiceContext, functionID, gameID, env string) (policy.RiskLevel, error) {
	return functioninvoke.ResolvePolicyRisk(ctx, svcCtx, functionID, gameID, env)
}

func enforceFunctionPolicy(ctx context.Context, svcCtx *svc.ServiceContext, functionID, gameID, env string, userRoles []string) (*policy.Policy, error) {
	return functioninvoke.EnforceFunctionPolicy(ctx, svcCtx, functionID, gameID, env, userRoles)
}

func matchAnyRole(userRoles []string, allowedRoles []string) bool {
	return functioninvoke.MatchAnyRole(userRoles, allowedRoles)
}

func buildBroadcastResponse(b *dispatch.BroadcastInvocation) *FunctionInvokeResponse {
	return functionInvokeResponseFromUnified(functioninvoke.BuildBroadcastResponse(b))
}

// getLocalizedTextFromMetadata gets a localized text map from metadata
func getLocalizedTextFromMetadata(metadata map[string]interface{}, key string) map[string]string {
	if metadata == nil {
		return nil
	}
	val, ok := metadata[key]
	if !ok {
		return nil
	}
	switch v := val.(type) {
	case map[string]string:
		return v
	case map[string]interface{}:
		result := make(map[string]string, len(v))
		for k, item := range v {
			if str, ok := item.(string); ok {
				result[k] = str
			}
		}
		return result
	}
	return nil
}

// firstNonEmptySlice returns the first non-empty slice, or nil if both are empty.
func firstNonEmptySlice(a, b []string) []string {
	if len(a) > 0 {
		return a
	}
	return b
}

// firstNonEmptyMap returns the first non-empty map, or nil if both are empty.
func firstNonEmptyMap(a, b map[string]string) map[string]string {
	if len(a) > 0 {
		return a
	}
	return b
}

// functionWarningFilterOptions 按 #34 提供注册警告过滤下拉的服务端聚合选项：
// 当前 scope 警告全集（与 functionWarnings 同源、不过滤）下 distinct
// 函数 ID / Agent 及各自警告条数，供 Warnings 页两个下拉一次取全。
func functionWarningFilterOptions(ctx context.Context, svcCtx *svc.ServiceContext) (*FunctionWarningFilterOptionsResponse, error) {
	scope := currentFunctionScope(ctx)

	resp := &FunctionWarningFilterOptionsResponse{
		Functions: []WarningFilterOptionItem{},
		Agents:    []WarningFilterOptionItem{},
	}
	if svcCtx.RegistryStore == nil {
		return resp, nil
	}

	warnings := svcCtx.RegistryStore.ListRegistrationWarnings(reg.RegistrationWarningFilter{
		GameID: scope.GameID,
		Env:    scope.Env,
		Limit:  0, // 聚合用全集，不限条数
	})

	funcCounts := map[string]int64{}
	agentCounts := map[string]int64{}
	for _, w := range warnings {
		if v := strings.TrimSpace(w.FunctionID); v != "" {
			funcCounts[v]++
		}
		if v := strings.TrimSpace(w.AgentID); v != "" {
			agentCounts[v]++
		}
	}
	resp.Functions = optionItemsFromCounts(funcCounts)
	resp.Agents = optionItemsFromCounts(agentCounts)
	return resp, nil
}

func optionItemsFromCounts(counts map[string]int64) []WarningFilterOptionItem {
	items := make([]WarningFilterOptionItem, 0, len(counts))
	for value, count := range counts {
		items = append(items, WarningFilterOptionItem{Value: value, Label: value, Count: count})
	}
	// 稳定序：条数降序、同数按 value 字典序（测试与展示均可预期）
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count != items[j].Count {
			return items[i].Count > items[j].Count
		}
		return items[i].Value < items[j].Value
	})
	return items
}
