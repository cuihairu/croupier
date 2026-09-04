package approval

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/api/function"
	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/common/errorx"
	extensioninstallation "github.com/cuihairu/croupier/internal/core/extension/installation"
	"github.com/cuihairu/croupier/internal/dashboard/freshness"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	contractsvc "github.com/cuihairu/croupier/internal/service"
	"github.com/cuihairu/croupier/internal/svc"
)

const officialApprovalID = "official.approval"
const approvalRecordsKey = "approvals"

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// List retrieves a paginated list of approvals
func (s *Service) List(ctx context.Context, req *ApprovalsListRequest) (*ApprovalsListResponse, error) {
	if req == nil {
		req = &ApprovalsListRequest{}
	}
	page := req.Page
	if page <= 0 {
		page = 1
	}
	size := req.PageSize
	if size <= 0 {
		size = 20
	}
	actorFilter := strings.TrimSpace(req.Actor)
	if req.Mine {
		// mine=true 强制按当前登录用户过滤，忽略请求中的 actor（防越权枚举）；
		// 用户名缺失时返回空集（fail-safe），退化为全量会泄露他人申请
		actorFilter = currentApprovalActor(ctx)
		if actorFilter == "" {
			return &ApprovalsListResponse{
				Approvals: []ApprovalSummary{},
				Page:      page,
				Size:      size,
			}, nil
		}
	}
	scope := svc.GameScopeFromContext(ctx)
	if items, ok, err := s.loadApprovalsFromExtensionInstallation(ctx); err != nil {
		return nil, err
	} else if ok {
		list, total := paginateApprovalSummaries(filterApprovalSummaries(
			toApprovalSummaries(filterApprovalItemsByScope(items, scope)),
			actorFilter, req.FunctionID, req.Status), page, size)
		return &ApprovalsListResponse{
			Approvals: list,
			Total:     int64(total),
			Page:      page,
			Size:      size,
		}, nil
	}
	if s.svcCtx.ApprovalsStore == nil {
		return nil, errors.New("approvals store unavailable")
	}

	filter := approvals.Filter{
		State:      strings.TrimSpace(req.Status),
		FunctionID: strings.TrimSpace(req.FunctionID),
		Actor:      actorFilter,
		GameID:     scope.GameID,
		Env:        scope.Env,
	}
	items, total, err := s.svcCtx.ApprovalsStore.List(filter, approvals.Page{
		Page: page,
		Size: size,
	})
	if err != nil {
		return nil, err
	}

	list := make([]ApprovalSummary, 0, len(items))
	for _, item := range items {
		list = append(list, buildApprovalSummary(item))
	}

	return &ApprovalsListResponse{
		Approvals: list,
		Total:     int64(total),
		Page:      page,
		Size:      size,
	}, nil
}

func currentApprovalActor(ctx context.Context) string {
	username, err := utils.CurrentUsername(ctx)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(username)
}

// rejectSelfApproval 落实两人规则：申请人不能审批自己的申请。
// approval.allowSelfApprove=true 时放开（单管理员部署的显式选择）；
// 拦截与放行均写哈希链审计 approval.self_approval_blocked 供追溯。
func (s *Service) rejectSelfApproval(ctx context.Context, operator string, record *approvals.Approval, action string) error {
	if s == nil || record == nil {
		return nil
	}
	if !strings.EqualFold(operator, strings.TrimSpace(record.Actor)) {
		return nil
	}
	outcome := "blocked"
	if s.svcCtx.Config.Approval.AllowSelfApprove {
		outcome = "allowed_by_config"
	}
	s.logSelfApprovalAudit(ctx, operator, record, action, outcome)
	if outcome != "blocked" {
		return nil
	}
	return errorx.NewForbidden("申请人不能审批自己的申请 (self_approval_forbidden)")
}

func (s *Service) logSelfApprovalAudit(ctx context.Context, operator string, record *approvals.Approval, action, outcome string) {
	if s.svcCtx == nil || s.svcCtx.AuditService == nil {
		return
	}
	_, _ = s.svcCtx.AuditService.Log(ctx, audit.AuditEventType("approval.self_approval_blocked"),
		audit.WithActorID(operator, "admin", operator),
		audit.WithResourceID("approval", record.ID),
		audit.WithDetails(map[string]interface{}{
			"approvalId": record.ID,
			"functionId": record.FunctionID,
			"actor":      record.Actor,
			"operator":   operator,
			"action":     action,
			"outcome":    outcome,
			"gameId":     record.GameID,
			"env":        record.Env,
		}),
	)
}

// Get retrieves details of a specific approval
func (s *Service) Get(ctx context.Context, req *ApprovalGetRequest) (*ApprovalGetResponse, error) {
	if req == nil || strings.TrimSpace(req.ID) == "" {
		return nil, errors.New("id 不能为空")
	}
	scope := svc.GameScopeFromContext(ctx)
	if items, ok, err := s.loadApprovalsFromExtensionInstallation(ctx); err != nil {
		return nil, err
	} else if ok {
		id := strings.TrimSpace(req.ID)
		for _, item := range items {
			if strings.TrimSpace(item.ID) == id {
				if err := requireApprovalScope(scope, item.GameID, item.Env); err != nil {
					return nil, err
				}
				return &ApprovalGetResponse{Approval: item}, nil
			}
		}
		return nil, errors.New("not found")
	}
	if s.svcCtx.ApprovalsStore == nil {
		return nil, errors.New("approvals store unavailable")
	}

	approval, err := s.svcCtx.ApprovalsStore.Get(strings.TrimSpace(req.ID))
	if err != nil {
		return nil, err
	}
	if err := requireApprovalScope(scope, approval.GameID, approval.Env); err != nil {
		return nil, err
	}

	detail := buildApprovalDetail(approval)

	return &ApprovalGetResponse{
		Approval: detail,
	}, nil
}

// Approve approves an approval
func (s *Service) Approve(ctx context.Context, req *ApprovalApproveRequest) (*ApprovalApproveResponse, error) {
	if s.svcCtx.ApprovalsStore == nil {
		return nil, errors.New("approvals store unavailable")
	}
	if req == nil || strings.TrimSpace(req.ID) == "" {
		return nil, errors.New("id 不能为空")
	}
	scope := svc.GameScopeFromContext(ctx)
	existing, err := s.svcCtx.ApprovalsStore.Get(strings.TrimSpace(req.ID))
	if err != nil {
		return nil, err
	}
	if err := requireApprovalScope(scope, existing.GameID, existing.Env); err != nil {
		return nil, err
	}
	operator := currentApprovalActor(ctx)
	if operator == "" {
		return nil, errorx.NewUnauthorized("审批操作需要登录态")
	}
	if err := s.rejectSelfApproval(ctx, operator, existing, "approve"); err != nil {
		return nil, err
	}

	record, err := s.svcCtx.ApprovalsStore.Approve(strings.TrimSpace(req.ID), operator)
	if err != nil {
		return nil, err
	}
	continuation, err := s.continueApprovedFunction(ctx, record)
	if err != nil {
		record.Reason = "approved but continuation failed: " + err.Error()
		if updated, updateErr := s.svcCtx.ApprovalsStore.Update(record); updateErr == nil {
			record = updated
		}
		_ = s.upsertApprovalToExtension(ctx, buildApprovalDetail(record))
		return nil, err
	}
	record.ResultKind = continuation.Kind
	record.TaskID = continuation.TaskID
	record.Result = continuation.Result
	if updated, updateErr := s.svcCtx.ApprovalsStore.Update(record); updateErr == nil {
		record = updated
	}
	_ = s.upsertApprovalToExtension(ctx, buildApprovalDetail(record))
	_ = s.recordApprovalEvent(ctx, "approvals_approve", "approval approved",
		fmt.Sprintf(`{"approvalId":"%s"}`, record.ID),
	)
	s.notifyApprovalEvent(ctx, "approval.approved", record,
		"审批已通过: "+record.FunctionID,
		"审批 "+record.ID+"（"+record.FunctionID+"）已由 "+record.Actor+" 通过。")

	return &ApprovalApproveResponse{
		ID:           record.ID,
		State:        record.State,
		TaskID:       continuation.TaskID,
		Result:       continuation.Result,
		ResultKind:   continuation.Kind,
		Continuation: continuation.Triggered,
	}, nil
}

// Reject rejects an approval
func (s *Service) Reject(ctx context.Context, req *ApprovalRejectRequest) (*ApprovalRejectResponse, error) {
	if s.svcCtx.ApprovalsStore == nil {
		return nil, errors.New("approvals store unavailable")
	}
	if req == nil || strings.TrimSpace(req.ID) == "" {
		return nil, errors.New("id 不能为空")
	}
	if strings.TrimSpace(req.Reason) == "" {
		return nil, errors.New("reason 不能为空")
	}
	scope := svc.GameScopeFromContext(ctx)
	existing, err := s.svcCtx.ApprovalsStore.Get(strings.TrimSpace(req.ID))
	if err != nil {
		return nil, err
	}
	if err := requireApprovalScope(scope, existing.GameID, existing.Env); err != nil {
		return nil, err
	}
	operator := currentApprovalActor(ctx)
	if operator == "" {
		return nil, errorx.NewUnauthorized("审批操作需要登录态")
	}
	if err := s.rejectSelfApproval(ctx, operator, existing, "reject"); err != nil {
		return nil, err
	}

	record, err := s.svcCtx.ApprovalsStore.Reject(strings.TrimSpace(req.ID), strings.TrimSpace(req.Reason), operator)
	if err != nil {
		return nil, err
	}
	_ = s.upsertApprovalToExtension(ctx, buildApprovalDetail(record))
	_ = s.recordApprovalEvent(ctx, "approvals_reject", "approval rejected",
		fmt.Sprintf(`{"approvalId":"%s"}`, record.ID),
	)
	s.notifyApprovalEvent(ctx, "approval.rejected", record,
		"审批已拒绝: "+record.FunctionID,
		"审批 "+record.ID+"（"+record.FunctionID+"）已被拒绝："+record.Reason)

	return &ApprovalRejectResponse{
		ID:     record.ID,
		State:  record.State,
		Reason: record.Reason,
	}, nil
}

func currentApprovalScope(ctx context.Context) (svc.GameScope, error) {
	scope, err := svc.CurrentScope(ctx)
	if err != nil {
		return svc.GameScope{}, errorx.NewBadRequest("游戏环境 scope 缺失")
	}
	return scope, nil
}

func requireApprovalScope(scope svc.GameScope, gameID, env string) error {
	if scope.GameID == "" && scope.Env == "" {
		return nil
	}
	if !strings.EqualFold(scope.GameID, strings.TrimSpace(gameID)) ||
		!strings.EqualFold(scope.Env, strings.TrimSpace(env)) {
		return errorx.NewForbidden("无权访问该审批")
	}
	return nil
}

func filterApprovalItemsByScope(items []Approval, scope svc.GameScope) []Approval {
	filtered := make([]Approval, 0, len(items))
	for _, item := range items {
		if requireApprovalScope(scope, item.GameID, item.Env) == nil {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// Helper functions

func buildApprovalSummary(a *approvals.Approval) ApprovalSummary {
	if a == nil {
		return ApprovalSummary{}
	}
	approver := strings.TrimSpace(a.Approver)
	return ApprovalSummary{
		ID:              a.ID,
		CreatedAt:       utils.FormatTimestamp(a.CreatedAt),
		UpdatedAt:       utils.FormatTimestamp(a.UpdatedAt),
		Actor:           a.Actor,
		FunctionID:      a.FunctionID,
		GameID:          a.GameID,
		Env:             a.Env,
		State:           strings.ToLower(strings.TrimSpace(a.State)),
		Mode:            defaultString(a.Mode, "invoke"),
		Route:           a.Route,
		IdempotencyKey:  a.IdempotencyKey,
		TargetServiceID: a.TargetServiceID,
		HashKey:         a.HashKey,
		Reason:          a.Reason,
		Continuation:    strings.TrimSpace(a.ResultKind) != "" || strings.TrimSpace(a.TaskID) != "" || len(a.Result) > 0,
		ResultKind:      a.ResultKind,
		TaskID:          a.TaskID,
		Result:          string(a.Result),
		Approver:        approver,
		ReviewedAt:      formatReviewedAt(a.ReviewedAt),
		ReviewedByOther: approver != "" && !strings.EqualFold(approver, strings.TrimSpace(a.Actor)),
	}
}

func formatReviewedAt(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return utils.FormatTimestamp(*t)
}

func buildApprovalDetail(a *approvals.Approval) Approval {
	summary := buildApprovalSummary(a)
	payload, preview := decodeApprovalPayload(a)
	return Approval{
		ID:              summary.ID,
		CreatedAt:       summary.CreatedAt,
		UpdatedAt:       summary.UpdatedAt,
		Actor:           summary.Actor,
		FunctionID:      summary.FunctionID,
		GameID:          summary.GameID,
		Env:             summary.Env,
		State:           summary.State,
		Mode:            summary.Mode,
		Route:           summary.Route,
		IdempotencyKey:  summary.IdempotencyKey,
		TargetServiceID: summary.TargetServiceID,
		HashKey:         summary.HashKey,
		Reason:          summary.Reason,
		Continuation:    summary.Continuation,
		ResultKind:      summary.ResultKind,
		TaskID:          summary.TaskID,
		Result:          summary.Result,
		Approver:        summary.Approver,
		ReviewedAt:      summary.ReviewedAt,
		ReviewedByOther: summary.ReviewedByOther,
		Payload:         payload,
		PayloadPreview:  preview,
	}
}

func decodeApprovalPayload(a *approvals.Approval) (map[string]interface{}, string) {
	if a == nil || len(a.Payload) == 0 {
		return nil, ""
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(a.Payload, &payload); err != nil {
		return nil, string(a.Payload)
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, a.Payload, "", "  "); err != nil {
		return payload, string(a.Payload)
	}
	return payload, buf.String()
}

func cloneApprovalMetadata(metadata map[string]string) map[string]string {
	out := make(map[string]string, len(metadata)+4)
	for key, value := range metadata {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		out[key] = value
	}
	return out
}

type approvalContinuationResult struct {
	Triggered bool
	Kind      string
	TaskID    string
	Result    json.RawMessage
}

func (s *Service) continueApprovedFunction(ctx context.Context, record *approvals.Approval) (approvalContinuationResult, error) {
	if record == nil {
		return approvalContinuationResult{}, errors.New("approval record is required")
	}
	if strings.TrimSpace(record.FunctionID) == "" {
		return approvalContinuationResult{}, nil
	}
	metadata := cloneApprovalMetadata(record.Metadata)
	metadata["approvalBypass"] = "approved"
	metadata["approvalId"] = strings.TrimSpace(record.ID)
	metadata["approvalActor"] = strings.TrimSpace(record.Actor)
	metadata["runtime_api"] = "approval.continue"
	if err := s.ensurePageApprovalStillFresh(ctx, record, metadata); err != nil {
		return approvalContinuationResult{}, err
	}
	resp, err := function.NewService(s.svcCtx).FunctionInvoke(ctx, &function.FunctionInvokeRequest{
		ID:              strings.TrimSpace(record.FunctionID),
		Payload:         json.RawMessage(record.Payload),
		GameID:          strings.TrimSpace(record.GameID),
		Env:             strings.TrimSpace(record.Env),
		Mode:            strings.TrimSpace(record.Mode),
		Route:           strings.TrimSpace(record.Route),
		TargetServiceID: strings.TrimSpace(record.TargetServiceID),
		HashKey:         strings.TrimSpace(record.HashKey),
		Metadata:        metadata,
	})
	if err != nil {
		return approvalContinuationResult{}, fmt.Errorf("continue approved function: %w", err)
	}
	result := approvalContinuationResult{Triggered: true, Kind: "sync"}
	if resp == nil {
		return result, nil
	}
	if resp.TaskID != "" || resp.TaskId != "" {
		result.Kind = "task"
		result.TaskID = resp.TaskID
		if result.TaskID == "" {
			result.TaskID = resp.TaskId
		}
		return result, nil
	}
	result.Result = resp.Result
	return result, nil
}

func (s *Service) ensurePageApprovalStillFresh(ctx context.Context, record *approvals.Approval, metadata map[string]string) error {
	if !strings.EqualFold(strings.TrimSpace(metadata["pageSnapshotGovernance"]), "validated") {
		return nil
	}
	if s == nil || s.svcCtx == nil || s.svcCtx.PublishedPageSpecModel == nil {
		return errors.New("published page model unavailable")
	}
	pageKey := strings.TrimSpace(metadata["page_key"])
	bindingID := strings.TrimSpace(metadata["binding_id"])
	version, err := strconv.Atoi(strings.TrimSpace(metadata["publish_version"]))
	if pageKey == "" || bindingID == "" || version <= 0 || err != nil {
		return errors.New("page approval metadata is incomplete")
	}
	published, err := s.svcCtx.PublishedPageSpecModel.FindByScopePageKeyAndVersion(ctx, strings.TrimSpace(record.GameID), strings.TrimSpace(record.Env), pageKey, version)
	if err != nil {
		return fmt.Errorf("published page snapshot not found: %w", err)
	}
	if !published.Active {
		return errors.New("published page snapshot is no longer active")
	}
	pageSpec, contracts := parseApprovalPublishedSnapshot(*published)
	binding, ok := findApprovalBinding(pageSpec.Bindings, bindingID)
	if !ok {
		return errors.New("published page binding not found")
	}
	contract, ok := findApprovalContract(contracts, bindingID)
	if !ok {
		return errors.New("published page binding contract snapshot missing")
	}
	functions, err := loadApprovalFunctionSpecs(ctx, s.svcCtx, record)
	if err != nil {
		return err
	}
	diags := freshness.EvaluateBinding(binding, contract, functions)
	if len(diags) > 0 {
		return fmt.Errorf("published page binding is stale: %s", approvalBindingFreshnessStatuses(diags))
	}
	return nil
}

func parseApprovalPublishedSnapshot(published model.PublishedPageSpec) (spec.PageSpec, []spec.BindingContractSnapshot) {
	var pageSpec spec.PageSpec
	if strings.TrimSpace(published.SpecJSON) != "" {
		_ = json.Unmarshal([]byte(published.SpecJSON), &pageSpec)
	}
	var contracts []spec.BindingContractSnapshot
	if strings.TrimSpace(published.BindingContractsJSON) != "" {
		_ = json.Unmarshal([]byte(published.BindingContractsJSON), &contracts)
	}
	return pageSpec, contracts
}

func findApprovalBinding(bindings []spec.PageFunctionBinding, bindingID string) (spec.PageFunctionBinding, bool) {
	for _, binding := range bindings {
		if strings.TrimSpace(binding.ID) == bindingID {
			return binding, true
		}
	}
	return spec.PageFunctionBinding{}, false
}

func findApprovalContract(contracts []spec.BindingContractSnapshot, bindingID string) (spec.BindingContractSnapshot, bool) {
	for _, contract := range contracts {
		if strings.TrimSpace(contract.BindingID) == bindingID {
			return contract, true
		}
	}
	return spec.BindingContractSnapshot{}, false
}

func loadApprovalFunctionSpecs(ctx context.Context, svcCtx *svc.ServiceContext, record *approvals.Approval) (map[string]spec.FunctionSpec, error) {
	if svcCtx == nil || svcCtx.DB == nil {
		return nil, errors.New("function contract database unavailable")
	}
	if record == nil {
		return nil, errors.New("approval record unavailable")
	}
	gameID := strings.TrimSpace(record.GameID)
	env := strings.TrimSpace(record.Env)
	if gameID == "" || env == "" {
		return nil, errors.New("approval scope is incomplete")
	}
	return contractsvc.FunctionSpecsByScope(ctx, model.NewFunctionContractModel(svcCtx.DB), gameID, env)
}

func approvalBindingFreshnessStatuses(diags []spec.BindingFreshnessDiagnostic) string {
	statuses := make([]string, 0, len(diags))
	for _, diag := range diags {
		if diag.Status != "" {
			statuses = append(statuses, string(diag.Status))
		}
	}
	return strings.Join(statuses, ",")
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func (s *Service) findActiveApprovalInstallation(ctx context.Context) (*model.ExtensionInstallation, bool, error) {
	if s == nil || s.svcCtx == nil || s.svcCtx.Extensions == nil || s.svcCtx.Extensions.Installation == nil {
		return nil, false, nil
	}
	items, _, err := s.svcCtx.Extensions.Installation.List(ctx, extensioninstallation.ListQuery{
		ExtensionID: officialApprovalID,
		Limit:       50,
		Offset:      0,
	})
	if err != nil {
		return nil, false, err
	}
	for i := range items {
		item := items[i]
		if strings.EqualFold(strings.TrimSpace(item.Status), "uninstalled") ||
			strings.EqualFold(strings.TrimSpace(item.DesiredState), "uninstalled") {
			continue
		}
		return &item, true, nil
	}
	return nil, false, nil
}

func (s *Service) recordApprovalEvent(ctx context.Context, eventType, message, payload string) error {
	item, ok, err := s.findActiveApprovalInstallation(ctx)
	if err != nil || !ok || item == nil {
		return err
	}
	operator := "system"
	if username, userErr := utils.CurrentUsername(ctx); userErr == nil && strings.TrimSpace(username) != "" {
		operator = strings.TrimSpace(username)
	}
	return s.svcCtx.Extensions.Installation.RecordEvent(ctx, item.ID, eventType, "info", message, operator, payload)
}

func (s *Service) loadApprovalsFromExtensionInstallation(ctx context.Context) ([]Approval, bool, error) {
	item, ok, err := s.findActiveApprovalInstallation(ctx)
	if err != nil || !ok || item == nil {
		return nil, false, err
	}
	config := map[string]any{}
	if len(bytes.TrimSpace(item.ConfigJSON)) > 0 {
		if err := json.Unmarshal(item.ConfigJSON, &config); err != nil {
			return nil, false, err
		}
	}
	raw, exists := config[approvalRecordsKey]
	if !exists || raw == nil {
		return nil, false, nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, false, err
	}
	items := []Approval{}
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, false, err
	}
	return items, true, nil
}

func (s *Service) saveApprovalsToExtensionInstallation(ctx context.Context, items []Approval) error {
	item, ok, err := s.findActiveApprovalInstallation(ctx)
	if err != nil || !ok || item == nil {
		return err
	}
	config := map[string]any{}
	if len(bytes.TrimSpace(item.ConfigJSON)) > 0 {
		_ = json.Unmarshal(item.ConfigJSON, &config)
	}
	config[approvalRecordsKey] = items
	secretRefs := map[string]string{}
	if len(bytes.TrimSpace(item.SecretRefsJSON)) > 0 {
		_ = json.Unmarshal(item.SecretRefsJSON, &secretRefs)
	}
	operator := "system"
	if username, userErr := utils.CurrentUsername(ctx); userErr == nil && strings.TrimSpace(username) != "" {
		operator = strings.TrimSpace(username)
	}
	return s.svcCtx.Extensions.Installation.UpdateConfig(ctx, item.ID, config, secretRefs, operator)
}

func (s *Service) upsertApprovalToExtension(ctx context.Context, current Approval) error {
	if strings.TrimSpace(current.ID) == "" {
		return nil
	}
	items, _, err := s.loadApprovalsFromExtensionInstallation(ctx)
	if err != nil {
		return err
	}
	id := strings.TrimSpace(current.ID)
	for i := range items {
		if strings.TrimSpace(items[i].ID) == id {
			items[i] = current
			return s.saveApprovalsToExtensionInstallation(ctx, items)
		}
	}
	items = append(items, current)
	return s.saveApprovalsToExtensionInstallation(ctx, items)
}

func toApprovalSummaries(items []Approval) []ApprovalSummary {
	out := make([]ApprovalSummary, 0, len(items))
	for _, item := range items {
		out = append(out, ApprovalSummary{
			ID:              item.ID,
			CreatedAt:       item.CreatedAt,
			UpdatedAt:       item.UpdatedAt,
			Actor:           item.Actor,
			FunctionID:      item.FunctionID,
			GameID:          item.GameID,
			Env:             item.Env,
			State:           item.State,
			Mode:            item.Mode,
			Route:           item.Route,
			IdempotencyKey:  item.IdempotencyKey,
			TargetServiceID: item.TargetServiceID,
			HashKey:         item.HashKey,
			Reason:          item.Reason,
		})
	}
	return out
}

func filterApprovalSummariesByState(items []ApprovalSummary, state string) []ApprovalSummary {
	if state == "" {
		return items
	}
	out := make([]ApprovalSummary, 0, len(items))
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item.State), state) {
			out = append(out, item)
		}
	}
	return out
}

// filterApprovalSummaries 按 actor/functionId/state 过滤（extension 安装路径，
// 与 ApprovalsStore.Filter 语义一致）。
func filterApprovalSummaries(items []ApprovalSummary, actor, functionID, state string) []ApprovalSummary {
	out := items
	if actor = strings.TrimSpace(actor); actor != "" {
		out = filterApprovalSummariesByActor(out, actor)
	}
	if functionID = strings.TrimSpace(functionID); functionID != "" {
		filtered := make([]ApprovalSummary, 0, len(out))
		for _, item := range out {
			if strings.EqualFold(strings.TrimSpace(item.FunctionID), functionID) {
				filtered = append(filtered, item)
			}
		}
		out = filtered
	}
	return filterApprovalSummariesByState(out, state)
}

func filterApprovalSummariesByActor(items []ApprovalSummary, actor string) []ApprovalSummary {
	out := make([]ApprovalSummary, 0, len(items))
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item.Actor), actor) {
			out = append(out, item)
		}
	}
	return out
}

func paginateApprovalSummaries(items []ApprovalSummary, page, size int) ([]ApprovalSummary, int) {
	total := len(items)
	if size <= 0 {
		size = 20
	}
	if page <= 0 {
		page = 1
	}
	start := (page - 1) * size
	if start >= total {
		return []ApprovalSummary{}, total
	}
	end := start + size
	if end > total {
		end = total
	}
	return items[start:end], total
}
