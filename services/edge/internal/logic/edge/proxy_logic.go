// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package edge

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/edge/internal/svc"
	"github.com/cuihairu/croupier/services/edge/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProxyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewProxyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProxyLogic {
	return &ProxyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ProxyLogic) Proxy(req *types.ProxyRequest) (*types.ProxyResponse, error) {
	if req == nil || strings.TrimSpace(req.TunnelId) == "" {
		return nil, errors.New("tunnel_id 不能为空")
	}

	state := l.svcCtx.State
	state.Mu.Lock()
	tunnel := state.Tunnels[req.TunnelId]
	if tunnel == nil {
		state.Mu.Unlock()
		return nil, errors.New("tunnel not found")
	}
	bodyLen := len(req.Body)
	tunnel.BytesIn += int64(bodyLen)
	tunnel.BytesOut += int64(bodyLen)
	tunnel.LastActive = time.Now()
	state.Mu.Unlock()

	responseHeaders := map[string]string{
		"content-type": "application/json",
	}
	if len(req.Headers) > 0 {
		for k, v := range req.Headers {
			responseHeaders[k] = v
		}
	}

	body := fmt.Sprintf("proxied via tunnel %s: %s %s", req.TunnelId, strings.ToUpper(req.Method), req.Path)

	return &types.ProxyResponse{
		Status:  200,
		Headers: responseHeaders,
		Body:    body,
	}, nil
}
