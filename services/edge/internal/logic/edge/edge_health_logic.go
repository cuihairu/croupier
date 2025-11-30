// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package edge

import (
	"context"
	"runtime"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/edge/internal/svc"
	"github.com/cuihairu/croupier/services/edge/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type EdgeHealthLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewEdgeHealthLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EdgeHealthLogic {
	return &EdgeHealthLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *EdgeHealthLogic) EdgeHealth(req *types.EdgeHealthRequest) (*types.EdgeHealthResponse, error) {
	state := l.svcCtx.State
	state.Mu.RLock()
	active := 0
	agents := map[string]struct{}{}
	for _, t := range state.Tunnels {
		if strings.EqualFold(t.Status, "active") {
			active++
		}
		if t.AgentID != "" {
			agents[t.AgentID] = struct{}{}
		}
	}
	state.Mu.RUnlock()

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	load := map[string]float64{
		"cpu":    0.0,
		"memory": float64(mem.Alloc) / (1024 * 1024),
		"tunnel": float64(active),
	}

	return &types.EdgeHealthResponse{
		Status:    "ok",
		Uptime:    l.svcCtx.Uptime().Nanoseconds() / int64(time.Second),
		Version:   l.svcCtx.Config.Log.ServiceName,
		Tunnels:   int64(active),
		Agents:    int64(len(agents)),
		Load:      load,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}, nil
}
