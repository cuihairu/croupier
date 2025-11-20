package logic

import (
	"context"
	"time"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsServicesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpsServicesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsServicesLogic {
	return &OpsServicesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsServicesLogic) OpsServices() (*types.OpsServicesResponse, error) {
	store := l.svcCtx.RegistryStore
	if store == nil {
		return &types.OpsServicesResponse{Services: []types.OpsService{}}, nil
	}
	now := time.Now()
	var services []types.OpsService
	store.Mu().RLock()
	for _, agent := range store.AgentsUnsafe() {
		if agent == nil {
			continue
		}
		expSec := int(time.Until(agent.ExpireAt).Seconds())
		if expSec < 0 {
			expSec = 0
		}
		service := types.OpsService{
			AgentId:        agent.AgentID,
			GameId:         agent.GameID,
			Env:            agent.Env,
			RpcAddr:        agent.RPCAddr,
			Ip:             hostFromAddr(agent.RPCAddr),
			Region:         agent.Region,
			Zone:           agent.Zone,
			Labels:         agent.Labels,
			Type:           "agent",
			Version:        agent.Version,
			Functions:      len(agent.Functions),
			Healthy:        now.Before(agent.ExpireAt),
			ExpiresInSec:   expSec,
			ActiveConns:    0,
			TotalRequests:  0,
			FailedRequests: 0,
			ErrorRate:      0,
			AvgLatencyMs:   0,
			LastSeen:       "",
			Qps1m:          0,
			QpsLimit:       0,
		}
		services = append(services, service)
	}
	store.Mu().RUnlock()
	return &types.OpsServicesResponse{Services: services}, nil
}
