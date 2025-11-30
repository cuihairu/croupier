// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsHealthUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新健康检查配置
func NewOpsHealthUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsHealthUpdateLogic {
	return &OpsHealthUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsHealthUpdateLogic) OpsHealthUpdate(req *types.OpsHealthUpdateRequest) (*types.OpsHealthUpdateResponse, error) {
	if req == nil {
		return nil, errors.New("请求体不能为空")
	}

	checks := make([]svc.OpsHealthCheck, 0, len(req.Checks))
	seen := map[string]struct{}{}
	for _, check := range req.Checks {
		id := strings.TrimSpace(check.ID)
		if id == "" {
			return nil, errors.New("健康检查ID不能为空")
		}
		if _, ok := seen[id]; ok {
			return nil, fmt.Errorf("健康检查ID重复: %s", id)
		}
		seen[id] = struct{}{}
		kind := strings.TrimSpace(check.Kind)
		target := strings.TrimSpace(check.Target)
		if kind == "" || target == "" {
			return nil, fmt.Errorf("检查 %s 类型和目标不能为空", id)
		}
		interval := check.IntervalSec
		if interval <= 0 {
			interval = 60
		}
		timeout := check.TimeoutMs
		if timeout <= 0 {
			timeout = 1000
		}

		checks = append(checks, svc.OpsHealthCheck{
			ID:          id,
			Kind:        kind,
			Target:      target,
			Expect:      strings.TrimSpace(check.Expect),
			IntervalSec: interval,
			TimeoutMs:   timeout,
			Region:      strings.TrimSpace(check.Region),
		})
	}

	state, err := updateOpsState(l.svcCtx, func(st *svc.OpsState) {
		st.Health.Checks = checks
		st.Health.UpdatedAt = time.Now().UTC()
		// prune status entries not in updated checks
		valid := map[string]struct{}{}
		for _, ck := range checks {
			valid[ck.ID] = struct{}{}
		}
		status := make([]svc.OpsHealthStatus, 0, len(st.Health.Status))
		for _, stItem := range st.Health.Status {
			if _, ok := valid[stItem.ID]; ok {
				status = append(status, stItem)
			}
		}
		st.Health.Status = status
	})
	if err != nil {
		return nil, err
	}

	return &types.OpsHealthUpdateResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"checks": state.Health.Checks,
			"status": state.Health.Status,
		},
	}, nil
}
