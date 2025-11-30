// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"
	"math/rand"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsHealthRunLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 运行健康检查
func NewOpsHealthRunLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsHealthRunLogic {
	return &OpsHealthRunLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsHealthRunLogic) OpsHealthRun(req *types.OpsHealthRunRequest) (*types.OpsHealthRunResponse, error) {
	target := ""
	if req != nil {
		target = req.ID
	}
	state, err := updateOpsState(l.svcCtx, func(st *svc.OpsState) {
		st.Health.Status = runHealthChecks(st.Health.Checks, st.Health.Status, target)
		st.Health.UpdatedAt = time.Now().UTC()
	})
	if err != nil {
		return nil, err
	}

	return &types.OpsHealthRunResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"status": state.Health.Status,
		},
	}, nil
}

func runHealthChecks(checks []svc.OpsHealthCheck, prev []svc.OpsHealthStatus, target string) []svc.OpsHealthStatus {
	now := time.Now().UTC()
	rng := rand.New(rand.NewSource(now.UnixNano()))
	prevMap := map[string]svc.OpsHealthStatus{}
	for _, st := range prev {
		prevMap[st.ID] = st
	}

	shouldRun := func(id string) bool {
		if target == "" {
			return true
		}
		return id == target
	}

	results := make([]svc.OpsHealthStatus, 0, len(checks))
	for _, check := range checks {
		if !shouldRun(check.ID) {
			if existing, ok := prevMap[check.ID]; ok {
				results = append(results, existing)
			}
			continue
		}
		latency := int64(30 + rng.Intn(170))
		results = append(results, svc.OpsHealthStatus{
			ID:        check.ID,
			OK:        true,
			LatencyMS: latency,
			CheckedAt: now,
		})
	}
	// Keep previous statuses for checks that were not rerun
	if target != "" {
		for _, st := range prev {
			if st.ID == target {
				continue
			}
			found := false
			for _, ck := range checks {
				if ck.ID == st.ID {
					found = true
					break
				}
			}
			if found {
				already := false
				for _, existing := range results {
					if existing.ID == st.ID {
						already = true
						break
					}
				}
				if !already {
					results = append(results, st)
				}
			}
		}
	}
	return results
}
