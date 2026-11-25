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

const (
	officialApprovalID = "official.approval"
	approvalRecordsKey = "approvals"

	// twoPersonPolicy is the governance policy key (policy.Policy.ApprovalWorkflow
	// / contract approval.policyKey): it requires two distinct authorized
	// subjects, so a requester may not self-approve.
	twoPersonPolicy = "two_person"
	// approvalPermissionKey is the minimum permission surface required for
	// approval actions (approve/reject);
	// it is the same permission key as the casbin approvals:write rule on
	// POST /api/v1/approvals.
	approvalPermissionKey = "approvals:write"
)

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
		// mine=true forces filtering by the currently logged-in user and ignores
		// the actor in the request (prevents out-of-scope enumeration);
		// when no username is present an empty set is returned (fail-safe), since
		// degrading to the full list would leak other people's requests
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

// rejectSelfApproval enforces the two-person rule: a requester may not review
// their own request.
// It is relaxed when approval.allowSelfApprove=true (an explicit choice of
// single-admin deployments);
// both the blocked and allowed outcomes are written to the hash-chain audit as
// approval.self_approval_blocked for traceability.
//
// AUD-038: the two_person policy enforces two distinct authorized subjects at
// runtime. The policy takes precedence over the global self-approval switch —
// even with allowSelfApprove=true, requests for two_person functions still
// forbid the same subject from approving/rejecting, otherwise the "two-person
// rule" would be merely descriptive data.
func (s *Service) rejectSelfApproval(ctx context.Context, operator string, record *approvals.Approval, action string) error {
	if s == nil || record == nil {
		return nil
	}
	if !strings.EqualFold(operator, strings.TrimSpace(record.Actor)) {
		return nil
	}
	outcome := "blocked"
	if s.svcCtx.Config.Approval.AllowSelfApprove {
		if approvalPolicyKey(record) == twoPersonPolicy {
			outcome = "blocked_by_two_person"
		} else {
			outcome = "allowed_by_config"
		}
	}
	s.logSelfApprovalAudit(ctx, operator, record, action, outcome)
	if outcome == "allowed_by_config" {
		return nil
	}
	return errorx.NewForbidden("申请人不能审批自己的申请 (self_approval_forbidden)")
}

// approvalPolicyKey reads the governance policy key of an approval record. At
// creation time (CreateFunctionApproval) the call chain writes it into
// approvals.ApprovalPolicyMetaKey; legacy records without the key return "",
// the two_person check does not hit, and the global self-approval switch
// governs as before.
func approvalPolicyKey(record *approvals.Approval) string {
	if record == nil {
		return ""
	}
	return strings.TrimSpace(record.Metadata[approvals.ApprovalPolicyMetaKey])
}

// requireApprovalPermission AUD-038: an approval action must hold the explicit
// approval permission (approvals:write); a login state plus scope alone is not
// enough. Previously any logged-in user (including low-privilege roles without
// approval permission) could approve/reject high-risk requests; such accounts
// are now rejected.
//
// Skipped when the permission layer is not wired (unit tests or minimal
// contexts where PermissionService/AdminModel are nil):
// the production ServiceContext always wires both (NewServiceContext
// initializes them together), and permission enforcement is covered separately
// by this method and permission-layer unit tests.
func (s *Service) requireApprovalPermission(ctx context.Context, operator string) error {
	if s == nil || s.svcCtx == nil || s.svcCtx.PermissionService == nil || s.svcCtx.AdminModel == nil {
		return nil
	}
	if strings.TrimSpace(operator) == "" {
		return errorx.NewUnauthorized("审批操作需要登录态")
	}
	_, _, err := utils.RequireAnyPermission(ctx, s.svcCtx,
		"无权审批该申请（需要审批权限）", "admin:all", approvalPermissionKey)
	return err
}

func (s *Service) logSelfApprovalAudit(ctx context.Context, operator string, record *approvals.Approval, action, outcome string) {
	if s.svcCtx == nil || s.svcCtx.AuditService == nil {
		return
	}
	_, auditErr := s.svcCtx.AuditService.Log(ctx, audit.AuditEventType("approval.self_approval_blocked"),
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
	if auditErr != nil {
		s.svcCtx.AuditService.ReportWriteGap(ctx, audit.AuditEventType("approval.self_approval_blocked"), auditErr)
	}
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
	// AUD-038: check the explicit approval permission (approvals:write) before
	// approving.
	if err := s.requireApprovalPermission(ctx, operator); err != nil {
		return nil, err
	}
	if err := s.rejectSelfApproval(ctx, operator, existing, "approve"); err != nil {
		return nil, err
	}
	// Step-up TOTP for high-risk approvals (OPEN-ISSUES #75): validate first
	// and only then land the approved state; when validation fails the approval
	// stays pending with no continuation or notification side effects. Web and
	// Mobile have long sent otp as designed while the server silently dropped
	// it — this is the server-side landing point for the three-part
	// confirmation.
	stepUp, err := s.enforceStepUpOTP(ctx, operator, existing, req.OTP)
	if err != nil {
		return nil, err
	}

	record, err := s.svcCtx.ApprovalsStore.Approve(strings.TrimSpace(req.ID), operator)
	if err != nil {
		if existing, ok := approvals.AsAlreadyReviewed(err); ok {
			// ARCH-001 D2/D4: a CAS failure means this request did not win the
			// transition (duplicate/race).
			// Never trigger continuation/audit/notification side effects; return
			// 409 plus the current terminal state.
			return nil, errorx.NewConflictWithCode("approval_already_reviewed",
				"审批已被处理，不能重复批准", map[string]any{
					"state":      existing.State,
					"approver":   existing.Approver,
					"reviewedAt": existing.ReviewedAt,
				})
		}
		return nil, err
	}
	// The approval decision lands on the audit chain immediately (before the
	// continuation: a failed continuation is still the fact "was approved").
	// Previously approvals only wrote to the extension event stream and
	// notifications; there were never any approval.approved/rejected rows in
	// audit_records, so the operation-logs kind=approval_* filter was always
	// empty — jumping to "view audit (approve/reject)" from the approval detail
	// page would inevitably find nobody there (docs/BUGS.md BUG-024).
	s.recordApprovalAudit(ctx, audit.EventApprovalApproved, operator, record,
		map[string]interface{}{"stepUp": stepUp})
	claimToken, err := approvals.NewClaimToken()
	if err != nil {
		return nil, fmt.Errorf("generate approval claim token: %w", err)
	}
	payloadDigest := approvals.PayloadDigest(record)
	record, err = s.svcCtx.ApprovalsStore.ClaimForDispatch(record.ID, claimToken, payloadDigest)
	if err != nil {
		if claimed, ok := approvals.AsAlreadyReviewed(err); ok {
			return nil, errorx.NewConflictWithCode("approval_already_claimed",
				"审批续跑已被领取，不能重复执行", map[string]any{
					"state":       claimed.State,
					"claimStatus": claimed.ClaimStatus,
					"claimToken":  claimed.ClaimToken,
					"claimedAt":   claimed.ClaimedAt,
				})
		}
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
		"审批 "+record.ID+"（"+record.FunctionID+"）已由 "+record.Approver+" 通过。")

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
	// AUD-038: check the explicit approval permission (approvals:write) before
	// rejecting.
	if err := s.requireApprovalPermission(ctx, operator); err != nil {
		return nil, err
	}
	if err := s.rejectSelfApproval(ctx, operator, existing, "reject"); err != nil {
		return nil, err
	}

	record, err := s.svcCtx.ApprovalsStore.Reject(strings.TrimSpace(req.ID), strings.TrimSpace(req.Reason), operator)
	if err != nil {
		if existing, ok := approvals.AsAlreadyReviewed(err); ok {
			return nil, errorx.NewConflictWithCode("approval_already_reviewed",
				"审批已被处理，不能重复拒绝", map[string]any{
					"state":      existing.State,
					"approver":   existing.Approver,
					"reviewedAt": existing.ReviewedAt,
				})
		}
		return nil, err
	}
	s.recordApprovalAudit(ctx, audit.EventApprovalRejected, operator, record)
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

// recordApprovalAudit writes the approval terminal state to the hash-chain
// audit (audit_records, actor_id = reviewer, game_id/env landed as columns
// with the record), so the operation-logs actor+kind=approval_approve/reject
// filter can find the real approval actions — the data source behind "view
// audit (approve/reject)" on the approval detail page.
// Write failures are only swallowed: auditing is a side channel and must never
// block the approval main flow.
// The variadic extraDetails parameter of recordApprovalAudit lets call sites
// add business details (such as the step-up validation tier); they are merged
// into details and then landed on the audit row together.
func (s *Service) recordApprovalAudit(ctx context.Context, eventType audit.AuditEventType, operator string, record *approvals.Approval, extraDetails ...map[string]interface{}) {
	if s == nil || s.svcCtx == nil || s.svcCtx.AuditService == nil || record == nil {
		return
	}
	details := map[string]interface{}{
		"approvalId": record.ID,
		"functionId": record.FunctionID,
		"actor":      record.Actor,
		"operator":   operator,
		"gameId":     record.GameID,
		"env":        record.Env,
	}
	for _, extra := range extraDetails {
		for k, v := range extra {
			details[k] = v
		}
	}
	if record.Reason != "" {
		details["reason"] = record.Reason
	}
	if record.ResultKind != "" {
		details["resultKind"] = record.ResultKind
	}
	if record.TaskID != "" {
		details["taskId"] = record.TaskID
	}
	// WithResourceID replaces Resource wholesale, so it must be called before
	// WithGameID,
	// otherwise the game_id/env columns get wiped out (scope filtering would no
	// longer find the row).
	_, auditErr := s.svcCtx.AuditService.Log(ctx, eventType,
		audit.WithActorID(operator, "admin", operator),
		audit.WithResourceID("approval", record.ID),
		audit.WithGameID(record.GameID, record.Env),
		audit.WithDetails(details),
	)
	if auditErr != nil {
		// The side effect has already run (the approval terminal state has
		// landed): report the gap (WARN + rate-limited
		// alert). It must not be silently swallowed, nor answered with 5xx
		// implying "the approval was not approved/rejected".
		s.svcCtx.AuditService.ReportWriteGap(ctx, eventType, auditErr)
	}
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
	// json.Indent only errors on invalid JSON; the previous line's
	// json.Unmarshal(a.Payload) already succeeded,
	// so a.Payload is valid JSON, Indent always succeeds, and the error branch
	// is unreachable and has been removed.
	_ = json.Indent(&buf, a.Payload, "", "  ")
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
	if record.ClaimToken != "" {
		metadata["approvalClaimToken"] = strings.TrimSpace(record.ClaimToken)
	}
	if record.PayloadDigest != "" {
		metadata["approvalPayloadDigest"] = strings.TrimSpace(record.PayloadDigest)
	}
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
	// FunctionInvoke contract: when err == nil the response must be non-nil
	// (the async/broadcast/sync
	// success paths all build a non-empty &FunctionInvokeResponse), so the
	// resp==nil branch is unreachable and has been removed.
	result := approvalContinuationResult{Triggered: true, Kind: "sync"}
	if resp.TaskId != "" {
		result.Kind = "task"
		result.TaskID = resp.TaskId
		return result, nil
	}
	result.Result = resp.Result
	return result, nil
}

func (s *Service) ensurePageApprovalStillFresh(ctx context.Context, record *approvals.Approval, metadata map[string]string) error {
	switch strings.ToLower(strings.TrimSpace(metadata["pageSnapshotGovernance"])) {
	case "validated":
		return s.ensurePublishedPageApprovalStillFresh(ctx, record, metadata)
	case "draft":
		return s.ensureDraftPageApprovalStillFresh(ctx, record, metadata)
	default:
		return nil
	}
}

// ensureDraftPageApprovalStillFresh checks the freshness of an approval
// created by a draft preview execution when it is being continued:
// a draft has no published snapshot to read back, so freshness is judged as
// "the contract is still present and its version unchanged" — if the function
// contract is re-registered during the approval (version change or the function
// disappears), continuation is refused and the draft must be preview-executed
// and submitted for approval again.
func (s *Service) ensureDraftPageApprovalStillFresh(ctx context.Context, record *approvals.Approval, metadata map[string]string) error {
	functionID := strings.TrimSpace(record.FunctionID)
	if functionID == "" {
		functionID = strings.TrimSpace(metadata["function_id"])
	}
	if functionID == "" {
		return errors.New("draft page approval metadata is incomplete")
	}
	functions, err := loadApprovalFunctionSpecs(ctx, s.svcCtx, record)
	if err != nil {
		return err
	}
	fn, ok := functions[functionID]
	if !ok {
		return fmt.Errorf("draft page binding is stale: function %s no longer registered", functionID)
	}
	snapshotVersion := strings.TrimSpace(metadata["snapshot_function_ver"])
	if snapshotVersion != "" && strings.TrimSpace(fn.Version) != snapshotVersion {
		return fmt.Errorf("draft page binding is stale: contract version %s != snapshot %s", strings.TrimSpace(fn.Version), snapshotVersion)
	}
	return nil
}

func (s *Service) ensurePublishedPageApprovalStillFresh(ctx context.Context, record *approvals.Approval, metadata map[string]string) error {
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
	// raw comes from a value taken out of a json.Unmarshal into map[string]any,
	// so its type can only be a JSON primitive type (nil/bool/float64/string/
	// []any/map[string]any); marshalling it again always succeeds, the error
	// branch is unreachable and has been removed.
	data, _ := json.Marshal(raw)
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

// filterApprovalSummaries filters by actor/functionId/state (the extension
// installation path, with semantics consistent with ApprovalsStore.Filter).
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
