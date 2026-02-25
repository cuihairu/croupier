// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package function

import (
	"context"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
)

type FunctionInstancesAllLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取所有函数实例
func NewFunctionInstancesAllLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionInstancesAllLogic {
	return &FunctionInstancesAllLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionInstancesAllLogic) FunctionInstancesAll() (map[string]interface{}, error) {
	if store := l.svcCtx.RegistryStore; store != nil {
		out := make([]map[string]interface{}, 0)
		store.Mu().RLock()
		for _, sess := range store.AgentsUnsafe() {
			if sess == nil || strings.TrimSpace(sess.AgentID) == "" {
				continue
			}
			agentHealthy := time.Until(sess.ExpireAt) > 0
			agentLastSeen := sess.LastSeen
			if agentLastSeen.IsZero() && !sess.ExpireAt.IsZero() {
				agentLastSeen = sess.ExpireAt.Add(-30 * time.Second)
			}
			for _, p := range sess.Providers {
				if len(p.FunctionIDs) == 0 {
					continue
				}
				for _, fid := range p.FunctionIDs {
					fid = strings.TrimSpace(fid)
					if fid == "" {
						continue
					}
					out = append(out, map[string]interface{}{
						"function_id": fid,
						"agent_id":    sess.AgentID,
						"provider_id": p.ProviderID,
						"addr":        p.Addr,
						"version":     p.Version,
						"last_seen":   agentLastSeen.Format(time.RFC3339),
						"healthy":     agentHealthy,
						"game_id":     sess.GameID,
						"env":         sess.Env,
					})
				}
			}
		}
		store.Mu().RUnlock()
		return map[string]interface{}{"instances": out}, nil
	}

	return map[string]interface{}{"instances": []map[string]interface{}{}}, nil
}
