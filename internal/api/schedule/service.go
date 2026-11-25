// Package schedule provides the REST API for cron-scheduled tasks.
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
	// Up-front cron validity check (fail fast).
	if _, err := scheduler.ParseCron(req.CronExpr); err != nil {
		return nil, errorx.NewBadRequest("cron 表达式无效: " + err.Error())
	}
	scope, err := resolveScheduleScope(ctx, req.GameID, req.Env)
	if err != nil {
		return nil, err
	}
	// Definition-time admission (ARCH-024): Create only checks that the function is
	// currently schedulable (disabled/allocation/input contract); it does not load a
	// principal and does not run policy approval. Authorization and policy approval
	// are re-granted on every trigger, which avoids manufacturing approval noise at
	// definition time and avoids freezing the creator's transient permissions into
	// the schedule.
	if _, err := functioninvoke.GovernInvocability(ctx, s.svcCtx, &functioninvoke.Request{
		ID:      req.FunctionID,
		GameID:  scope.GameID,
		Env:     scope.Env,
		Mode:    "async",
		Payload: []byte(req.Payload),
	}); err != nil {
		return nil, err
	}
	actor := currentActor(ctx)
	created, err := m.Create(ctx, model.CreateScheduleInput{
		Name:       req.Name,
		CronExpr:   req.CronExpr,
		GameID:     scope.GameID,
		Env:        scope.Env,
		FunctionID: req.FunctionID,
		Payload:    model.JSON(req.Payload),
		MaxFailed:  req.MaxFailedRuns,
		Actor:      actor,
	})
	if err != nil {
		return nil, err
	}
	// Compute the next trigger time immediately.
	if spec, err := scheduler.ParseCron(created.CronExpr); err == nil {
		next := spec.Next(time.Now())
		if err := m.UpdateSchedule(ctx, created.ID, map[string]interface{}{"next_triggered_at": next}); err == nil {
			created.NextTriggeredAt = &next
		}
	}
	return &CreateResponse{Item: buildItem(created)}, nil
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
		// Recompute the next trigger time when resuming.
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
	TaskRunID string `json:"taskRunId"`
	// ApprovalID/ApprovalWorkflow are returned when policy requires approval (ARCH-024,
	// structurally identical to the functionInvoke/task Start approval response); the
	// task is not dispatched.
	ApprovalID       string `json:"approvalId,omitempty"`
	ApprovalWorkflow string `json:"approvalWorkflow,omitempty"`
}

// TriggerNow fires the schedule once immediately (not counted against trigger-slot
// idempotency; it dispatches directly).
// A manual trigger is an execution entry point (ARCH-024): it shares the same
// execution admission chain as functionInvoke / /tasks Start -- disabled status,
// allocation, principal, policy approval and input contract are all consistent on
// every run, with no un-governed bare dispatch.
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
	// Execution admission (ARCH-024 unified governance): shares the Govern chain with
	// functionInvoke and /tasks Start. The Human principal is resolved by the ctx
	// auth middleware; when policy requires approval, a pending-approval response is
	// returned and nothing is dispatched.
	governed, err := functioninvoke.Govern(ctx, s.svcCtx, &functioninvoke.Request{
		ID:      sch.FunctionID,
		GameID:  sch.GameID,
		Env:     sch.Env,
		Mode:    "async",
		Payload: []byte(sch.Payload.String()),
	}, functioninvoke.Subject{Kind: functioninvoke.SubjectHuman})
	if err != nil {
		return nil, err
	}
	if governed.ApprovalRequired {
		return &TriggerNowResponse{
			ApprovalID:       governed.ApprovalID,
			ApprovalWorkflow: governed.ApprovalWorkflow,
		}, nil
	}
	meta := map[string]string{
		"gameId":     sch.GameID,
		"env":        sch.Env,
		"scheduleId": strconv.FormatUint(uint64(sch.ID), 10),
		"actor":      governed.Admin.Username,
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
	req := utils.BuildInvokeRequest(sch.FunctionID, []byte(sch.Payload.String()), meta)
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
