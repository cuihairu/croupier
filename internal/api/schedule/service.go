// Package schedule 提供 cron 定时任务的 REST API。
package schedule

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/functioninvoke"
	"github.com/cuihairu/croupier/internal/svc"
	scheduler "github.com/cuihairu/croupier/internal/tasks/scheduler"
)

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

func (s *Service) model() *model.TaskScheduleModel {
	if s.svcCtx == nil || s.svcCtx.TaskScheduleModel == nil {
		return nil
	}
	return s.svcCtx.TaskScheduleModel
}

// ---------- DTO ----------

type ScheduleItem struct {
	ID                  uint   `json:"id"`
	Name                string `json:"name"`
	CronExpr            string `json:"cronExpr"`
	GameID              string `json:"gameId"`
	Env                 string `json:"env"`
	FunctionID          string `json:"functionId"`
	Status              string `json:"status"`
	MaxFailedRuns       int    `json:"maxFailedRuns"`
	ConsecutiveFailures int    `json:"consecutiveFailures"`
	LastRunID           string `json:"lastRunId,omitempty"`
	Actor               string `json:"actor,omitempty"`
	NextTriggerAt       string `json:"nextTriggerAt,omitempty"`
	LastTriggerAt       string `json:"lastTriggerAt,omitempty"`
	CreatedAt           string `json:"createdAt"`
	UpdatedAt           string `json:"updatedAt"`
}

func buildItem(s *model.TaskSchedule) ScheduleItem {
	item := ScheduleItem{
		ID:                  s.ID,
		Name:                s.Name,
		CronExpr:            s.CronExpr,
		GameID:              s.GameID,
		Env:                 s.Env,
		FunctionID:          s.FunctionID,
		Status:              s.Status,
		MaxFailedRuns:       s.MaxFailedRuns,
		ConsecutiveFailures: s.ConsecutiveFailures,
		LastRunID:           s.LastRunID,
		Actor:               s.Actor,
		CreatedAt:           s.CreatedAt.Format(time.RFC3339),
		UpdatedAt:           s.UpdatedAt.Format(time.RFC3339),
	}
	if s.NextTriggeredAt != nil {
		item.NextTriggerAt = s.NextTriggeredAt.Format(time.RFC3339)
	}
	if s.LastTriggeredAt != nil {
		item.LastTriggerAt = s.LastTriggeredAt.Format(time.RFC3339)
	}
	return item
}

type ListRequest struct {
	Page     int    `form:"page"`
	PageSize int    `form:"pageSize"`
	GameID   string `form:"gameId"`
	Env      string `form:"env"`
	Status   string `form:"status"`
}

type ListResponse struct {
	Items []ScheduleItem `json:"items"`
	Total int64          `json:"total"`
}

type CreateRequest struct {
	Name          string          `json:"name" binding:"required"`
	CronExpr      string          `json:"cronExpr" binding:"required"`
	FunctionID    string          `json:"functionId" binding:"required"`
	GameID        string          `json:"gameId"`
	Env           string          `json:"env"`
	Payload       json.RawMessage `json:"payload"`
	MaxFailedRuns int             `json:"maxFailedRuns"`
}

type CreateResponse struct {
	Item ScheduleItem `json:"item"`
	// ApprovalRequired 政策要求审批时返回（调度创建为 paused，批准续跑后激活）。
	ApprovalRequired bool   `json:"approvalRequired,omitempty"`
	ApprovalID       string `json:"approvalId,omitempty"`
}

type UpdateStatusRequest struct {
	Status string `json:"status" binding:"required"` // active | paused
}

// ---------- Handlers ----------

func (s *Service) List(ctx context.Context, req *ListRequest) (*ListResponse, error) {
	m := s.model()
	if m == nil {
		return nil, errorx.NewBadRequest("调度模型未初始化")
	}
	items, total, err := m.List(ctx, model.ListSchedulesOptions{
		Page:     req.Page,
		PageSize: req.PageSize,
		GameID:   svc.ResolveGameID(ctx, req.GameID),
		Env:      svc.ResolveEnv(ctx, req.Env),
		Status:   strings.TrimSpace(req.Status),
	})
	if err != nil {
		return nil, err
	}
	out := make([]ScheduleItem, 0, len(items))
	for i := range items {
		out = append(out, buildItem(&items[i]))
	}
	return &ListResponse{Items: out, Total: total}, nil
}

func (s *Service) Create(ctx context.Context, req *CreateRequest) (*CreateResponse, error) {
	m := s.model()
	if m == nil {
		return nil, errorx.NewBadRequest("调度模型未初始化")
	}
	// cron 合法性前置校验（快速失败）。
	if _, err := scheduler.ParseCron(req.CronExpr); err != nil {
		return nil, errorx.NewBadRequest("cron 表达式无效: " + err.Error())
	}
	scope, err := resolveScheduleScope(ctx, req.GameID, req.Env)
	if err != nil {
		return nil, err
	}
	// 人类操作员 + scope 授权。
	admin, roles, err := utils.LoadCurrentAdmin(ctx, s.svcCtx)
	if err != nil {
		return nil, err
	}
	roleNames := utils.RoleNamesFromModels(roles)
	if err := utils.RequireGameEnvScope(ctx, s.svcCtx, admin.ID, roleNames, scope.GameID, scope.Env); err != nil {
		return nil, err
	}
	payload := []byte(req.Payload)

	// 统一治理闸门（ARCH-024）：注册时验证函数可执行（禁用/分配/函数权限/政策/
	// 输入契约）。政策要求审批 → 调度创建为 paused（待批准），批准续跑后激活
	// （approvalBypass=approved 快照）。未要求审批 → 直接 active。
	gate, err := functioninvoke.NewGovernance(s.svcCtx).Gate(ctx, functioninvoke.Subject{Admin: admin, Roles: roles}, functioninvoke.GateInput{
		FunctionID: req.FunctionID,
		GameID:     scope.GameID,
		Env:        scope.Env,
		Payload:    payload,
	})
	if err != nil {
		return nil, err
	}
	status := model.ScheduleStatusActive
	approvalID := ""
	if gate.RequireApproval && s.svcCtx.ApprovalsStore != nil {
		status = model.ScheduleStatusPaused
	}
	created, err := m.Create(ctx, model.CreateScheduleInput{
		Name:       req.Name,
		CronExpr:   req.CronExpr,
		GameID:     scope.GameID,
		Env:        scope.Env,
		FunctionID: req.FunctionID,
		Payload:    model.JSON(req.Payload),
		MaxFailed:  req.MaxFailedRuns,
		Actor:      admin.Username,
		Status:     status,
	})
	if err != nil {
		return nil, err
	}
	// 立即计算下次触发时间（active 才触发）。
	if spec, err := scheduler.ParseCron(created.CronExpr); err == nil {
		next := spec.Next(time.Now())
		if err := m.UpdateSchedule(ctx, created.ID, map[string]interface{}{"next_triggered_at": next}); err == nil {
			created.NextTriggeredAt = &next
		}
	}
	// 政策要求审批：创建审批请求（approvalTarget=schedule，续跑时激活调度），
	// 调度保持 paused。审批创建失败 → 删除调度回滚（不留半截 paused 调度）。
	if status == model.ScheduleStatusPaused {
		approvalID, err = functioninvoke.CreateFunctionApproval(ctx, s.svcCtx, &functioninvoke.Request{
			ID:       req.FunctionID,
			GameID:   scope.GameID,
			Env:      scope.Env,
			Payload:  payload,
			Mode:     "async",
			Metadata: map[string]string{"approvalTarget": "schedule", "scheduleId": strconv.FormatUint(uint64(created.ID), 10)},
		}, payload, admin, gate.FunctionPolicy)
		if err != nil {
			_ = m.Delete(ctx, created.ID)
			return nil, err
		}
		// 记录待批准引用（approvalBypass 快照在批准续跑时写入）。
		_ = m.UpdateSchedule(ctx, created.ID, map[string]interface{}{
			"metadata": scheduleMetadataWithApproval(created.Metadata, approvalID),
		})
	}
	return &CreateResponse{
		Item:             buildItem(created),
		ApprovalRequired: status == model.ScheduleStatusPaused,
		ApprovalID:       approvalID,
	}, nil
}

func (s *Service) SetStatus(ctx context.Context, id uint, status string) (ScheduleItem, error) {
	m := s.model()
	if m == nil {
		return ScheduleItem{}, errorx.NewBadRequest("调度模型未初始化")
	}
	if err := s.requireScheduleScope(ctx, id); err != nil {
		return ScheduleItem{}, err
	}
	switch status {
	case model.ScheduleStatusActive:
		// 恢复时重算下次触发。
		sch, err := m.FindByID(ctx, id)
		if err != nil {
			return ScheduleItem{}, err
		}
		if spec, err := scheduler.ParseCron(sch.CronExpr); err == nil {
			next := spec.Next(time.Now())
			if err := m.UpdateSchedule(ctx, id, map[string]interface{}{"next_triggered_at": next}); err != nil {
				return ScheduleItem{}, err
			}
		}
		if err := m.SetStatus(ctx, id, model.ScheduleStatusActive); err != nil {
			return ScheduleItem{}, err
		}
	case model.ScheduleStatusPaused:
		if err := m.UpdateSchedule(ctx, id, map[string]interface{}{
			"status": model.ScheduleStatusPaused, "next_triggered_at": nil,
		}); err != nil {
			return ScheduleItem{}, err
		}
	default:
		return ScheduleItem{}, errorx.NewBadRequest("status 仅支持 active/paused（dead_letter 恢复也用 active）")
	}
	updated, err := m.FindByID(ctx, id)
	if err != nil {
		return ScheduleItem{}, err
	}
	return buildItem(updated), nil
}

type TriggerNowResponse struct {
	TaskRunID string `json:"taskRunId,omitempty"`
	// ApprovalRequired 政策要求审批时返回（不派发，需先批准续跑）。
	ApprovalRequired bool   `json:"approvalRequired,omitempty"`
	ApprovalID       string `json:"approvalId,omitempty"`
}

// TriggerNow 立即触发一次（不计入触发槽幂等，走 dispatch 直发）。
//
// ARCH-024：与 /tasks Start 同一治理闸门——禁用/分配/函数权限/政策审批/输入契约
// 一致执行。政策要求审批时创建审批请求而非派发（高危函数经手动调度也不能绕过
// 审批）。
func (s *Service) TriggerNow(ctx context.Context, id uint) (*TriggerNowResponse, error) {
	m := s.model()
	if m == nil || s.svcCtx.Dispatcher == nil {
		return nil, errorx.NewBadRequest("调度未初始化")
	}
	if err := s.requireScheduleScope(ctx, id); err != nil {
		return nil, err
	}
	sch, err := m.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	// 人类操作员 + scope 授权。
	admin, roles, err := utils.LoadCurrentAdmin(ctx, s.svcCtx)
	if err != nil {
		return nil, err
	}
	roleNames := utils.RoleNamesFromModels(roles)
	if err := utils.RequireGameEnvScope(ctx, s.svcCtx, admin.ID, roleNames, sch.GameID, sch.Env); err != nil {
		return nil, err
	}
	payload := []byte(sch.Payload.String())
	meta := map[string]string{
		"gameId":     sch.GameID,
		"env":        sch.Env,
		"scheduleId": strconv.FormatUint(uint64(sch.ID), 10),
		"actor":      admin.Username,
	}
	var extra map[string]string
	if len(sch.Metadata) > 0 {
		_ = json.Unmarshal(sch.Metadata, &extra)
	}
	for k, v := range extra {
		if _, ok := meta[k]; !ok {
			meta[k] = v
		}
	}

	// 统一治理闸门（ARCH-024）：含调度携带的批准快照（approvalBypass）时豁免
	// 政策闸（站立授权），enable/assign/perm/input 仍 live 执行。
	gate, err := functioninvoke.NewGovernance(s.svcCtx).Gate(ctx, functioninvoke.Subject{Admin: admin, Roles: roles}, functioninvoke.GateInput{
		FunctionID: sch.FunctionID,
		GameID:     sch.GameID,
		Env:        sch.Env,
		Payload:    payload,
		Metadata:   meta,
	})
	if err != nil {
		return nil, err
	}
	if gate.RequireApproval && s.svcCtx.ApprovalsStore != nil {
		approvalID, err := functioninvoke.CreateFunctionApproval(ctx, s.svcCtx, &functioninvoke.Request{
			ID:       sch.FunctionID,
			GameID:   sch.GameID,
			Env:      sch.Env,
			Payload:  payload,
			Mode:     "async",
			Metadata: meta,
		}, payload, admin, gate.FunctionPolicy)
		if err != nil {
			return nil, err
		}
		return &TriggerNowResponse{ApprovalID: approvalID, ApprovalRequired: true}, nil
	}

	req := utils.BuildInvokeRequest(sch.FunctionID, payload, meta)
	resp, err := s.svcCtx.Dispatcher.StartTaskRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	return &TriggerNowResponse{TaskRunID: resp.GetTaskId()}, nil
}

type RunLogItem struct {
	ID        uint   `json:"id"`
	TaskRunID string `json:"taskRunId,omitempty"`
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
	Slot      string `json:"slot"`
	CreatedAt string `json:"createdAt"`
}

type RunLogsResponse struct {
	Items []RunLogItem `json:"items"`
	Total int64        `json:"total"`
}

func (s *Service) RunLogs(ctx context.Context, id uint, page, pageSize int) (*RunLogsResponse, error) {
	m := s.model()
	if m == nil {
		return nil, errorx.NewBadRequest("调度模型未初始化")
	}
	if err := s.requireScheduleScope(ctx, id); err != nil {
		return nil, err
	}
	logs, total, err := m.ListRunLogs(ctx, id, page, pageSize)
	if err != nil {
		return nil, err
	}
	items := make([]RunLogItem, 0, len(logs))
	for i := range logs {
		items = append(items, RunLogItem{
			ID:        logs[i].ID,
			TaskRunID: logs[i].TaskRunID,
			Status:    logs[i].Status,
			Message:   logs[i].Message,
			Slot:      logs[i].Slot.Format(time.RFC3339),
			CreatedAt: logs[i].CreatedAt.Format(time.RFC3339),
		})
	}
	return &RunLogsResponse{Items: items, Total: total}, nil
}

func (s *Service) Delete(ctx context.Context, id uint) error {
	m := s.model()
	if m == nil {
		return errorx.NewBadRequest("调度模型未初始化")
	}
	if err := s.requireScheduleScope(ctx, id); err != nil {
		return err
	}
	return m.Delete(ctx, id)
}

func (s *Service) requireScheduleScope(ctx context.Context, id uint) error {
	m := s.model()
	if m == nil {
		return errorx.NewBadRequest("调度模型未初始化")
	}
	schedule, err := m.FindByID(ctx, id)
	if err != nil {
		return err
	}
	scope := svc.GameScopeFromContext(ctx)
	if scope.GameID != "" && scope.GameID != schedule.GameID {
		return errorx.NewNotFound(fmt.Sprintf("schedule %d not found in game %s", id, scope.GameID))
	}
	if scope.Env != "" && scope.Env != schedule.Env {
		return errorx.NewNotFound(fmt.Sprintf("schedule %d not found in env %s", id, scope.Env))
	}
	return nil
}

// ---------- helpers ----------

func resolveScheduleScope(ctx context.Context, gameID, env string) (svc.GameScope, error) {
	gameID = strings.TrimSpace(gameID)
	env = strings.TrimSpace(env)
	if gameID == "" || env == "" {
		scope := svc.GameScopeFromContext(ctx)
		if scope.GameID == "" || scope.Env == "" {
			return scope, errorx.NewBadRequest("缺少 gameId/env（请求体或 X-Game-ID/X-Env 头）")
		}
		return scope, nil
	}
	return svc.GameScope{GameID: gameID, Env: env}, nil
}

func currentActor(ctx context.Context) string {
	if v, ok := ctx.Value("username").(string); ok {
		return v
	}
	return ""
}

// scheduleMetadataWithApproval 把待批准引用（approvalId）合并进调度 metadata，
// 供批准续跑与 UI 展示（批准快照 approvalBypass 在续跑激活时写入）。
func scheduleMetadataWithApproval(metadata model.JSON, approvalID string) model.JSON {
	var m map[string]string
	if len(metadata) > 0 {
		_ = json.Unmarshal(metadata, &m)
	}
	if m == nil {
		m = map[string]string{}
	}
	m["approvalId"] = approvalID
	out, _ := json.Marshal(m)
	return out
}
