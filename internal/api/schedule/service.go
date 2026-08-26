// Package schedule 提供 cron 定时任务的 REST API。
package schedule

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	scheduler "github.com/cuihairu/croupier/internal/tasks/scheduler"
	"gorm.io/datatypes"
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
	// cron 合法性前置校验（快速失败）。
	if _, err := scheduler.ParseCron(req.CronExpr); err != nil {
		return nil, errorx.NewBadRequest("cron 表达式无效: " + err.Error())
	}
	scope, err := resolveScheduleScope(ctx, req.GameID, req.Env)
	if err != nil {
		return nil, err
	}
	actor := currentActor(ctx)
	created, err := m.Create(ctx, model.CreateScheduleInput{
		Name:       req.Name,
		CronExpr:   req.CronExpr,
		GameID:     scope.GameID,
		Env:        scope.Env,
		FunctionID: req.FunctionID,
		Payload:    datatypes.JSON(req.Payload),
		MaxFailed:  req.MaxFailedRuns,
		Actor:      actor,
	})
	if err != nil {
		return nil, err
	}
	// 立即计算下次触发时间。
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
	TaskRunID string `json:"taskRunId"`
}

// TriggerNow 立即触发一次（不计入触发槽幂等，走 dispatch 直发）。
func (s *Service) TriggerNow(ctx context.Context, id uint) (*TriggerNowResponse, error) {
	m := s.model()
	if m == nil || s.svcCtx.Dispatcher == nil {
		return nil, errorx.NewBadRequest("调度未初始化")
	}
	sch, err := m.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	meta := map[string]string{
		"game_id":     sch.GameID,
		"env":         sch.Env,
		"schedule_id": strconv.FormatUint(uint64(sch.ID), 10),
		"actor":       currentActor(ctx),
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
	return m.Delete(ctx, id)
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
