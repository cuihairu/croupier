// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsSilenceDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除静默规则
func NewOpsSilenceDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsSilenceDeleteLogic {
	return &OpsSilenceDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsSilenceDeleteLogic) OpsSilenceDelete(req *types.OpsAlertSilenceDeleteRequest) (*types.OpsSilenceDeleteResponse, error) {
	if req == nil {
		return nil, errors.New("请求体不能为空")
	}
	id := strings.TrimSpace(req.ID)
	if id == "" {
		return nil, errors.New("静默ID不能为空")
	}

	state, err := updateOpsState(l.svcCtx, func(st *svc.OpsState) {
		next := make([]svc.OpsSilenceEntry, 0, len(st.Alerts.Silences))
		for _, silence := range st.Alerts.Silences {
			if silence.ID == id {
				continue
			}
			next = append(next, silence)
		}
		st.Alerts.Silences = next
		st.Alerts.UpdatedAt = time.Now().UTC()
	})
	if err != nil {
		return nil, err
	}

	return &types.OpsSilenceDeleteResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"id":        id,
			"silences":  len(state.Alerts.Silences),
			"updatedAt": utils.FormatTimestamp(state.Alerts.UpdatedAt),
		},
	}, nil
}
