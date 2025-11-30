// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package edge

import (
	"context"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/services/edge/internal/svc"
	"github.com/cuihairu/croupier/services/edge/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type TunnelListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewTunnelListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TunnelListLogic {
	return &TunnelListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TunnelListLogic) TunnelList(req *types.TunnelListRequest) (*types.TunnelListResponse, error) {
	if req == nil {
		req = &types.TunnelListRequest{}
	}

	state := l.svcCtx.State
	state.Mu.RLock()
	records := make([]*svc.TunnelRecord, 0, len(state.Tunnels))
	for _, t := range state.Tunnels {
		if req.AgentId != "" && !strings.EqualFold(t.AgentID, req.AgentId) {
			continue
		}
		if req.Status != "" && !strings.EqualFold(t.Status, req.Status) {
			continue
		}
		records = append(records, t)
	}
	state.Mu.RUnlock()

	sort.Slice(records, func(i, j int) bool {
		return records[i].LastActive.After(records[j].LastActive)
	})

	page := req.Page
	if page <= 0 {
		page = 1
	}
	size := req.Size
	if size <= 0 {
		size = 20
	}
	total := len(records)

	start := (page - 1) * size
	if start > total {
		start = total
	}
	end := start + size
	if end > total {
		end = total
	}

	items := make([]types.TunnelStatusResponse, 0, end-start)
	for _, t := range records[start:end] {
		items = append(items, types.TunnelStatusResponse{
			TunnelId:    t.ID,
			Status:      t.Status,
			Protocol:    t.Protocol,
			RemoteAddr:  t.RemoteAddr,
			LocalAddr:   t.LocalAddr,
			Connections: t.Connections,
			BytesIn:     t.BytesIn,
			BytesOut:    t.BytesOut,
			CreatedAt:   formatTime(t.CreatedAt),
			LastActive:  formatTime(t.LastActive),
		})
	}

	return &types.TunnelListResponse{
		Tunnels: items,
		Total:   int64(total),
		Page:    page,
		Size:    size,
	}, nil
}
