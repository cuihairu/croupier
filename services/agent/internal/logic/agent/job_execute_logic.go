// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/services/agent/internal/svc"
	"github.com/cuihairu/croupier/services/agent/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type JobExecuteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewJobExecuteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *JobExecuteLogic {
	return &JobExecuteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *JobExecuteLogic) JobExecute(req *types.JobExecuteRequest) (*types.JobExecuteResponse, error) {
	if req == nil {
		return nil, errors.New("missing request payload")
	}
	if l.svcCtx.Core == nil {
		return nil, errors.New("agent core is not running")
	}

	jobID := strings.TrimSpace(req.JobId)
	if jobID == "" {
		jobID = fmt.Sprintf("job-%d", time.Now().UnixNano())
	}

	functionID := strings.TrimSpace(req.FunctionId)
	if functionID == "" {
		return nil, errors.New("function_id 不能为空")
	}
	if strings.TrimSpace(l.svcCtx.LocalGRPCAddr) == "" {
		return nil, errors.New("local gRPC core address not configured")
	}

	payload, err := json.Marshal(map[string]interface{}{
		"inputs":  svc.CloneInterfaceMap(req.Inputs),
		"options": svc.CloneInterfaceMap(req.Options),
	})
	if err != nil {
		return nil, err
	}

	dialCtx, cancel := context.WithTimeout(l.ctx, 3*time.Second)
	defer cancel()

	cc, err := grpc.DialContext(dialCtx, l.svcCtx.LocalGRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		l.svcCtx.Core.Store().SetJobResult(jobID, "failed", nil, err.Error())
		return &types.JobExecuteResponse{Success: false, JobId: jobID, Status: "failed"}, nil
	}
	defer cc.Close()

	callCtx, callCancel := context.WithTimeout(l.ctx, 10*time.Second)
	defer callCancel()

	cli := sdkv1.NewInvokerServiceClient(cc)
	resp, err := cli.Invoke(callCtx, &sdkv1.InvokeRequest{
		FunctionId:     functionID,
		IdempotencyKey: jobID,
		Payload:        payload,
		Metadata: map[string]string{
			"game_id": strings.TrimSpace(req.GameId),
			"env":     strings.TrimSpace(req.Env),
		},
	})
	if err != nil {
		l.svcCtx.Core.Store().SetJobResult(jobID, "failed", nil, err.Error())
		return &types.JobExecuteResponse{Success: false, JobId: jobID, Status: "failed"}, nil
	}

	l.svcCtx.Core.Store().SetJobResult(jobID, "completed", resp.GetPayload(), "")
	return &types.JobExecuteResponse{Success: true, JobId: jobID, Status: "completed"}, nil
}
