// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsServicesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取服务列表
func NewOpsServicesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsServicesLogic {
	return &OpsServicesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsServicesLogic) OpsServices(_ *types.OpsServicesRequest) (*types.OpsServicesResponse, error) {
	if _, _, err := utils.RequireAnyPermission(l.ctx, l.svcCtx, "无权查看服务列表", "admin:all", "ops:read", "registry:read"); err != nil {
		return nil, err
	}

	services := make([]types.OpsServiceItem, 0)

	// Add server itself
	serverAddr := fmt.Sprintf("%s:%d", l.svcCtx.Config.Host, l.svcCtx.Config.Port)
	if l.svcCtx.Config.Host == "0.0.0.0" {
		serverAddr = fmt.Sprintf("localhost:%d", l.svcCtx.Config.Port)
	}
	services = append(services, types.OpsServiceItem{
		ID:      "server",
		Name:    "croupier-server",
		Type:    "server",
		Status:  "running",
		Address: serverAddr,
		Version: l.svcCtx.ServerVersion,
		Labels:  map[string]string{},
	})

	// Add agents from registry
	if store := l.svcCtx.RegistryStore; store != nil {
		store.Mu().RLock()
		for _, sess := range store.AgentsUnsafe() {
			if sess == nil || strings.TrimSpace(sess.AgentID) == "" {
				continue
			}

			ttl, healthy := ttlAndHealth(sess)
			status := "healthy"
			if ttl <= 0 {
				status = "expired"
			} else if !healthy {
				status = "unhealthy"
			}

			labels := sess.Labels
			if labels == nil {
				labels = make(map[string]string)
			}

			services = append(services, types.OpsServiceItem{
				ID:             sess.AgentID,
				Name:           sess.AgentID,
				Type:           "agent",
				Status:         status,
				Address:        sess.RPCAddr,
				GameID:         sess.GameID,
				Env:            sess.Env,
				Version:        sess.Version,
				Region:         sess.Region,
				Zone:           sess.Zone,
				Labels:         labels,
				FunctionsCount: utils.CountEnabledFunctions(sess.Functions),
				LastSeen:       sess.ExpireAt.Add(-time.Second * 30).Format(time.RFC3339),
			})
		}
		store.Mu().RUnlock()
	}

	return &types.OpsServicesResponse{
		Services: services,
		Total:    len(services),
	}, nil
}

func ttlAndHealth(sess *registry.AgentSession) (int, bool) {
	if sess == nil || sess.ExpireAt.IsZero() {
		return 0, false
	}
	ttl := int(time.Until(sess.ExpireAt).Seconds())
	if ttl < 0 {
		ttl = 0
	}
	return ttl, ttl > 0
}
