// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"
	"time"

	localv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/local/v1"
	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type FunctionInstancesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewFunctionInstancesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionInstancesLogic {
	return &FunctionInstancesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionInstancesLogic) FunctionInstances(req *types.FunctionInstancesRequest) (resp *types.FunctionInstancesResponse, err error) {
	result := make([]types.FunctionInstance, 0)
	store := l.svcCtx.RegistryStore
	if store == nil {
		return &types.FunctionInstancesResponse{Instances: result}, nil
	}

	type agentSnapshot struct {
		id        string
		gameID    string
		rpcAddr   string
		functions map[string]struct{}
	}

	var agents []agentSnapshot
	store.Mu().RLock()
	for _, agent := range store.AgentsUnsafe() {
		if agent == nil {
			continue
		}
		if req.GameId != "" && agent.GameID != req.GameId {
			continue
		}
		if req.FunctionId != "" {
			if _, ok := agent.Functions[req.FunctionId]; !ok {
				continue
			}
		}
		fnSet := make(map[string]struct{}, len(agent.Functions))
		for fid := range agent.Functions {
			fnSet[fid] = struct{}{}
		}
		agents = append(agents, agentSnapshot{
			id:        agent.AgentID,
			gameID:    agent.GameID,
			rpcAddr:   agent.RPCAddr,
			functions: fnSet,
		})
	}
	store.Mu().RUnlock()

	for _, ag := range agents {
		if req.FunctionId != "" {
			if _, ok := ag.functions[req.FunctionId]; !ok {
				continue
			}
		}
		dialCtx, cancel := context.WithTimeout(l.ctx, 5*time.Second)
		conn, err := grpc.DialContext(dialCtx, ag.rpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		cancel()
		if err != nil {
			logx.WithContext(l.ctx).Errorf("dial agent %s: %v", ag.id, err)
			continue
		}
		func() {
			defer conn.Close()
			client := localv1.NewLocalControlServiceClient(conn)
			callCtx, cancel := context.WithTimeout(l.ctx, 5*time.Second)
			defer cancel()
			resp, err := client.ListLocal(callCtx, &localv1.ListLocalRequest{})
			if err != nil || resp == nil {
				if err != nil {
					logx.WithContext(l.ctx).Errorf("list local from agent %s: %v", ag.id, err)
				}
				return
			}
			for _, lf := range resp.Functions {
				if lf == nil {
					continue
				}
				if req.FunctionId != "" && lf.GetId() != req.FunctionId {
					continue
				}
				for _, inst := range lf.Instances {
					if inst == nil {
						continue
					}
					result = append(result, types.FunctionInstance{
						AgentId:   ag.id,
						ServiceId: inst.GetServiceId(),
						Addr:      inst.GetAddr(),
						Version:   inst.GetVersion(),
					})
				}
			}
		}()
	}

	return &types.FunctionInstancesResponse{Instances: result}, nil
}
