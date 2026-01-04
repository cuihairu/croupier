package function_calls

import (
	"context"
	"encoding/json"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type FunctionCallRerunLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewFunctionCallRerunLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionCallRerunLogic {
	return &FunctionCallRerunLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionCallRerunLogic) FunctionCallRerun(req *types.FunctionCallRerunRequest) (*types.FunctionCallRerunResponse, error) {
	// Permission check
	_, roles, err := utils.LoadCurrentAdmin(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}
	roleNames := utils.RoleNamesFromModels(roles)
	permIDs, err := utils.PermissionIDsFromRoles(l.ctx, l.svcCtx, roles)
	if err != nil {
		return nil, err
	}
	if !utils.HasAdminRole(roleNames) && !utils.HasPermissionID(permIDs, "function_calls:rerun") && !utils.HasPermissionID(permIDs, "*") {
		return nil, errorx.NewForbidden("无权重新执行调用")
	}

	// Find original call
	original, err := l.svcCtx.JobHistoryModel.FindByJobID(l.ctx, req.ID)
	if err != nil {
		return nil, errorx.NewNotFound("原始调用记录不存在")
	}

	// Get current admin info
	admin, _, err := utils.LoadCurrentAdmin(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}

	// Determine payload to use
	var payload []byte
	if req.Payload != nil {
		// Use new payload from request
		payload, _ = json.Marshal(req.Payload)
	} else if original.Payload != nil {
		// Use original payload
		payload = original.Payload
	} else {
		payload = []byte("{}")
	}

	// Extract game_id and env from metadata or original
	gameID := original.GameID
	env := original.Env
	if gameID == "" {
		// Try to get from metadata
		if original.Metadata != nil {
			var meta map[string]string
			if err := json.Unmarshal(original.Metadata, &meta); err == nil {
				gameID = meta["game_id"]
				env = meta["env"]
			}
		}
	}

	// Create metadata for new job
	metadata := map[string]string{
		"rerun_of":    original.JobID,
		"rerun_by":    admin.Username,
		"rerun_at":    time.Now().Format(time.RFC3339),
		"game_id":     gameID,
		"env":         env,
		"original_id": req.ID,
	}
	metadataBytes, _ := json.Marshal(metadata)

	// Record job start in history
	now := time.Now()
	newJobHistory := &model.JobHistory{
		JobID:       generateJobID(),
		FunctionID:  original.FunctionID,
		GameID:      gameID,
		Env:         env,
		ActorID:     admin.Username,
		ActorType:   "admin",
		Status:      "running",
		Payload:     payload,
		Metadata:    metadataBytes,
		RetryCount:  original.RetryCount + 1,
		ParentJobID: original.JobID,
		StartedAt:   &now,
	}

	// Insert into history
	if err := l.svcCtx.JobHistoryModel.Insert(l.ctx, newJobHistory); err != nil {
		logx.Errorf("Failed to insert job history for rerun: %v", err)
		return nil, errorx.NewInternalError("创建重新执行记录失败")
	}

	// Execute via dispatcher
	if l.svcCtx.Dispatcher == nil {
		// Update status to failed if no dispatcher
		_ = l.svcCtx.JobHistoryModel.UpdateStatus(l.ctx, newJobHistory.JobID, "failed", &now, nil, "调度器未初始化", 0)
		return nil, errorx.NewInternalError("调度器未初始化")
	}

	// Prepare invoke request
	invokeReq := &invokeRequest{
		FunctionID: original.FunctionID,
		Payload:    payload,
		Metadata:   metadata,
	}

	// Start the job
	jobID, err := l.svcCtx.Dispatcher.StartJob(l.ctx, invokeReq.FunctionID, invokeReq.Payload)
	if err != nil {
		// Update status to failed
		_ = l.svcCtx.JobHistoryModel.UpdateStatus(l.ctx, newJobHistory.JobID, "failed", &now, nil, err.Error(), 0)
		return nil, errorx.NewInternalError("启动任务失败: " + err.Error())
	}

	// Update job ID to the actual one from dispatcher
	_ = l.svcCtx.JobHistoryModel.DeleteByJobID(l.ctx, newJobHistory.JobID) // Remove temp record
	newJobHistory.JobID = jobID
	_ = l.svcCtx.JobHistoryModel.Insert(l.ctx, newJobHistory) // Re-insert with correct JobID

	return &types.FunctionCallRerunResponse{JobID: jobID}, nil
}

func generateJobID() string {
	return "job_" + time.Now().Format("20060102150405") + "_" + randomString(8)
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}

type invokeRequest struct {
	FunctionID string
	Payload    []byte
	Metadata   map[string]string
}
