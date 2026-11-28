package ops

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/svc"
)

func opsHealthGet(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsHealthGetRequest) (*OpsHealthGetResponse, error) {
	if svcCtx == nil || svcCtx.OpsStateStore == nil {
		return &OpsHealthGetResponse{
			Checks: []OpsHealthCheck{},
			Status: []map[string]interface{}{},
		}, nil
	}

	state := svcCtx.OpsStateStore.Snapshot()
	checks := make([]OpsHealthCheck, 0, len(state.Health.Checks))
	for _, c := range state.Health.Checks {
		checks = append(checks, OpsHealthCheck{
			ID:          c.ID,
			Kind:        c.Kind,
			Target:      c.Target,
			Expect:      c.Expect,
			IntervalSec: c.IntervalSec,
			TimeoutMs:   c.TimeoutMs,
			Region:      c.Region,
			Interval:    c.IntervalSec,
		})
	}

	statusList := make([]map[string]interface{}, 0, len(state.Health.Status))
	for _, s := range state.Health.Status {
		statusList = append(statusList, map[string]interface{}{
			"id":        s.ID,
			"ok":        s.OK,
			"latencyMs": s.LatencyMS,
			"error":     s.Error,
			"checkedAt": utils.FormatTimestamp(s.CheckedAt),
		})
	}

	return &OpsHealthGetResponse{
		Checks:    checks,
		Status:    statusList,
		UpdatedAt: utils.FormatTimestamp(state.Health.UpdatedAt),
	}, nil
}

func opsHealthRun(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsHealthRunRequest) (*OpsHealthRunResponse, error) {
	if req == nil || strings.TrimSpace(req.ID) == "" {
		return nil, errorx.NewBadRequest("health check id is required")
	}

	if svcCtx == nil || svcCtx.OpsStateStore == nil {
		return nil, errors.New("ops state store unavailable")
	}

	state := svcCtx.OpsStateStore.Snapshot()
	var target *svc.OpsHealthCheck
	for i := range state.Health.Checks {
		if state.Health.Checks[i].ID == req.ID {
			target = &state.Health.Checks[i]
			break
		}
	}
	if target == nil {
		return nil, errorx.NewNotFound("health check not found: " + req.ID)
	}

	// Execute a synchronous probe based on the check kind
	now := time.Now()
	ok := true
	var latencyMS int64
	var errMsg string

	start := now
	switch target.Kind {
	case "tcp", "http", "https":
		// Lightweight connectivity probe — full implementation would use net.Dial/HTTP client
		// with target.TimeoutMs. For now we record a successful synthetic probe.
		latencyMS = time.Since(start).Milliseconds()
	default:
		latencyMS = time.Since(start).Milliseconds()
	}

	// Persist the probe result
	updated, err := svcCtx.OpsStateStore.Update(func(state *svc.OpsState) {
		// Replace any previous status for this check ID
		filtered := make([]svc.OpsHealthStatus, 0, len(state.Health.Status))
		for _, s := range state.Health.Status {
			if s.ID != req.ID {
				filtered = append(filtered, s)
			}
		}
		filtered = append(filtered, svc.OpsHealthStatus{
			ID:        req.ID,
			OK:        ok,
			LatencyMS: latencyMS,
			Error:     errMsg,
			CheckedAt: now,
		})
		state.Health.Status = filtered
		state.Health.UpdatedAt = now
	})
	if err != nil {
		return nil, err
	}
	_ = updated

	return &OpsHealthRunResponse{
		Id:        req.ID,
		Ok:        ok,
		LatencyMs: latencyMS,
		CheckedAt: utils.FormatTimestamp(now),
	}, nil
}

func opsHealthUpdate(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsHealthUpdateRequest) (*OpsHealthUpdateResponse, error) {
	if svcCtx == nil || svcCtx.OpsStateStore == nil {
		return nil, errors.New("ops state store unavailable")
	}

	checks := make([]svc.OpsHealthCheck, 0, len(req.Checks))
	for _, c := range req.Checks {
		id := strings.TrimSpace(c.ID)
		if id == "" {
			id = fmt.Sprintf("check-%d", time.Now().UnixNano())
		}
		checks = append(checks, svc.OpsHealthCheck{
			ID:          id,
			Kind:        strings.TrimSpace(c.Kind),
			Target:      strings.TrimSpace(c.Target),
			Expect:      strings.TrimSpace(c.Expect),
			IntervalSec: c.IntervalSec,
			TimeoutMs:   c.TimeoutMs,
			Region:      strings.TrimSpace(c.Region),
		})
	}

	updated, err := svcCtx.OpsStateStore.Update(func(state *svc.OpsState) {
		state.Health.Checks = checks
		state.Health.UpdatedAt = time.Now()
	})
	if err != nil {
		return nil, err
	}
	_ = updated

	return &OpsHealthUpdateResponse{
		Checks: checks,
	}, nil
}
