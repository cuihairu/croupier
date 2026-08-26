// Database monitoring API (docs/research/db-monitoring-design.md P1).
package dbmon

import (
	"context"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

type Service struct {
	svcCtx *svc.ServiceContext
}

// NewService creates a dbmon service.
func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// ListSources returns registered sources with masked DSNs.
func (s *Service) ListSources(ctx context.Context) (*SourceListResponse, error) {
	items, err := s.svcCtx.DBSourceModel.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]DBSource, 0, len(items))
	for i := range items {
		out = append(out, buildSourceDTO(&items[i]))
	}
	return &SourceListResponse{Items: out}, nil
}

// CreateSource registers a database source.
func (s *Service) CreateSource(ctx context.Context, req *SourceUpsertRequest) (*SourceResponse, error) {
	src := &model.DBSource{
		Name: req.Name, Driver: strings.ToLower(req.Driver), Kind: strings.ToLower(strings.TrimSpace(req.Kind)),
		DSN: req.DSN, GameID: strings.TrimSpace(req.GameID), Env: strings.TrimSpace(req.Env),
		Enabled: true, Sort: req.Sort, CreatedBy: currentUsername(ctx),
	}
	if src.Kind == "" {
		src.Kind = model.DBSourceKindSelf
	}
	if err := model.ValidateDBSource(src); err != nil {
		return nil, errorx.NewBadRequest(err.Error())
	}
	if err := s.svcCtx.DBSourceModel.Create(ctx, src); err != nil {
		return nil, err
	}
	return &SourceResponse{DBSource: buildSourceDTO(src)}, nil
}

// UpdateSource applies a partial update. DSN updates are validated too.
func (s *Service) UpdateSource(ctx context.Context, req *SourceUpdateRequest) (*SourceResponse, error) {
	id, err := utils.ParseUintID(req.ID, "数据源 ID")
	if err != nil {
		return nil, err
	}
	existing, err := s.svcCtx.DBSourceModel.FindOne(ctx, id)
	if err != nil {
		return nil, err
	}
	updates := map[string]interface{}{}
	if v := strings.TrimSpace(req.Name); v != "" {
		updates["name"] = v
	}
	if v := strings.TrimSpace(strings.ToLower(req.Driver)); v != "" {
		updates["driver"] = v
	}
	if v := strings.TrimSpace(req.Kind); v != "" {
		updates["kind"] = v
	}
	if req.DSN != nil && strings.TrimSpace(*req.DSN) != "" {
		updates["dsn"] = strings.TrimSpace(*req.DSN)
	}
	if req.GameID != nil {
		updates["game_id"] = strings.TrimSpace(*req.GameID)
	}
	if req.Env != nil {
		updates["env"] = strings.TrimSpace(*req.Env)
	}
	if req.Sort != nil {
		updates["sort"] = *req.Sort
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if len(updates) == 0 {
		return nil, errorx.NewBadRequest("请提供需要更新的字段")
	}
	merged := *existing
	applyUpdates(&merged, updates)
	if err := model.ValidateDBSource(&merged); err != nil {
		return nil, errorx.NewBadRequest(err.Error())
	}
	if err := s.svcCtx.DBSourceModel.Update(ctx, id, updates); err != nil {
		return nil, err
	}
	updated, err := s.svcCtx.DBSourceModel.FindOne(ctx, id)
	if err != nil {
		return nil, err
	}
	return &SourceResponse{DBSource: buildSourceDTO(updated)}, nil
}

// DeleteSource removes a registration.
func (s *Service) DeleteSource(ctx context.Context, req *SourceDeleteRequest) error {
	id, err := utils.ParseUintID(req.ID, "数据源 ID")
	if err != nil {
		return err
	}
	return s.svcCtx.DBSourceModel.Delete(ctx, id)
}

// ProbeAll runs the health probe against every enabled source.
// Sources whose probe finds lock waits above threshold or a rising deadlock
// counter raise alerts into the ops alert center (best-effort).
func (s *Service) ProbeAll(ctx context.Context) (*ProbeAllResponse, error) {
	items, err := s.svcCtx.DBSourceModel.List(ctx)
	if err != nil {
		return nil, err
	}
	results := make([]ProbeResult, 0, len(items))
	for i := range items {
		src := &items[i]
		if !src.Enabled {
			continue
		}
		res, perr := s.ProbeOne(ctx, src)
		if perr != nil {
			res = &ProbeResult{SourceID: src.ID, Name: src.Name, Driver: src.Driver, Kind: src.Kind, Error: perr.Error()}
		}
		s.raiseAlertsIfNeeded(ctx, src, res)
		results = append(results, *res)
	}
	return &ProbeAllResponse{Results: results}, nil
}

// ProbeOne probes one registered source by id.
func (s *Service) ProbeOne(ctx context.Context, src *model.DBSource) (*ProbeResult, error) {
	dsn := resolveDSN(src)
	if dsn == "" {
		return nil, fmt.Errorf("source %d has no DSN", src.ID)
	}
	return Probe(ctx, src, dsn)
}

// resolveDSN returns the stored DSN verbatim (registered sources carry their
// own read-only DSN; masking only applies to API responses).
func resolveDSN(src *model.DBSource) string { return strings.TrimSpace(src.DSN) }

// raiseAlertsIfNeeded fires/resolves alerts per design thresholds.
func (s *Service) raiseAlertsIfNeeded(ctx context.Context, src *model.DBSource, res *ProbeResult) {
	if s.svcCtx == nil || s.svcCtx.AlertModel == nil || res.Error != "" {
		return
	}
	lockThreshold := src.LockWaitWarn
	if lockThreshold <= 0 {
		lockThreshold = 5
	}
	alertID := fmt.Sprintf("dbmon:%d", src.ID)

	status := "ok"
	level := ""
	msg := ""
	if len(res.LockWaits) > lockThreshold {
		status = "firing"
		level = "critical"
		msg = fmt.Sprintf("数据库锁等待过多：%s 有 %d 条等待（阈值 %d）", src.Name, len(res.LockWaits), lockThreshold)
	} else if res.Connections != nil && res.Connections.Max > 0 &&
		res.Connections.Current*100/res.Connections.Max >= warnConnRatio(src.ConnWarnRatio) {
		status = "firing"
		level = "warning"
		msg = fmt.Sprintf("数据库连接水位高：%s %d/%d", src.Name, res.Connections.Current, res.Connections.Max)
	}

	var existing *model.Alert
	existing, findErr := s.svcCtx.AlertModel.FindByAlertID(ctx, alertID)

	switch {
	case status == "firing" && (findErr != nil || existing == nil):
		_ = s.svcCtx.AlertModel.Create(ctx, &model.Alert{
			AlertID: alertID,
			Type:    "db_monitor",
			Level:   level,
			Message: msg,
			Source:  "dbmon",
			Status:  "firing",
			Details: map[string]interface{}{
				"sourceId":   src.ID,
				"lockWaits":  len(res.LockWaits),
				"thresholds": map[string]int{"lockWait": lockThreshold},
			},
		})
	case status == "ok" && findErr == nil && existing != nil && existing.Status == "firing":
		_ = s.svcCtx.AlertModel.UpdateStatus(ctx, existing.ID, "resolved")
	case status == "firing":
		// already firing; dedup
	default:
		_ = msg
	}
}

func warnConnRatio(configured int) int {
	if configured <= 0 {
		return 80
	}
	return configured
}

func applyUpdates(dst *model.DBSource, updates map[string]interface{}) {
	if v, ok := updates["name"].(string); ok {
		dst.Name = v
	}
	if v, ok := updates["driver"].(string); ok {
		dst.Driver = v
	}
	if v, ok := updates["kind"].(string); ok {
		dst.Kind = v
	}
	if v, ok := updates["dsn"].(string); ok {
		dst.DSN = v
	}
	if v, ok := updates["game_id"].(string); ok {
		dst.GameID = v
	}
	if v, ok := updates["env"].(string); ok {
		dst.Env = v
	}
}

func currentUsername(ctx context.Context) string {
	if name, err := utils.CurrentUsername(ctx); err == nil && name != "" {
		return name
	}
	return "system"
}
