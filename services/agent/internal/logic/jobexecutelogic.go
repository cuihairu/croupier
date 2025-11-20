package logic

import (
	"context"
	"fmt"
	"time"

	"github.com/cuihairu/croupier/services/agent/internal/svc"
	"github.com/cuihairu/croupier/services/agent/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
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

func (l *JobExecuteLogic) JobExecute(req *types.JobExecuteRequest) (resp *types.JobExecuteResponse, err error) {
	logx.Infof("Job execution request: jobId=%s, functionId=%s, gameId=%s, env=%s",
		req.JobId, req.FunctionId, req.GameId, req.Env)

	// Validate required fields
	if req.JobId == "" || req.FunctionId == "" || req.GameId == "" || req.Env == "" {
		return &types.JobExecuteResponse{
			Success: false,
			JobId:   req.JobId,
			Status:  "failed",
		}, nil
	}

	// Check if function exists
	functionKey := fmt.Sprintf("%s:%s:%s", req.GameId, req.Env, req.FunctionId)
	functionInfo, exists := l.svcCtx.AgentStore.GetFunction(functionKey)
	if !exists {
		return &types.JobExecuteResponse{
			Success: false,
			JobId:   req.JobId,
			Status:  "failed",
		}, nil
	}

	// Create job object
	job := &svc.Job{
		ID:          req.JobId,
		FunctionID:  req.FunctionId,
		GameID:      req.GameId,
		Env:         req.Env,
		Inputs:      req.Inputs,
		Options:     req.Options,
		Status:      "pending",
		Progress:    0,
		StartTime:   time.Now().Format(time.RFC3339),
		Retries:     0,
		MaxRetries:  l.svcCtx.Config.Job.Retries,
	}

	// Store job info
	job.FunctionInfo = functionInfo

	// Try to create job
	if !l.svcCtx.JobManager.CreateJob(job) {
		return &types.JobExecuteResponse{
			Success: false,
			JobId:   req.JobId,
			Status:  "rejected",
		}, nil
	}

	// Start job execution asynchronously
	go l.executeJob(job)

	logx.Infof("Job accepted and started: %s", req.JobId)

	return &types.JobExecuteResponse{
		Success: true,
		JobId:   req.JobId,
		Status:  "accepted",
	}, nil
}

func (l *JobExecuteLogic) executeJob(job *svc.Job) {
	logx.Infof("Starting job execution: %s", job.ID)

	// Update job status to running
	l.svcCtx.JobManager.UpdateJobStatus(job.ID, "running")

	// Simulate job execution (in real implementation, this would call the actual function)
	l.simulateJobExecution(job)
}

func (l *JobExecuteLogic) simulateJobExecution(job *svc.Job) {
	// This is a placeholder for actual function execution
	// In a real implementation, this would:
	// 1. Load the function implementation
	// 2. Execute with provided inputs
	// 3. Handle timeouts and errors
	// 4. Update progress
	// 5. Return results

	logx.Infof("Simulating execution of job: %s", job.ID)

	// Update progress
	job.Progress = 25
	l.svcCtx.JobManager.UpdateJobStatus(job.ID, "running")

	// Simulate work
	time.Sleep(1 * time.Second)

	job.Progress = 50
	l.svcCtx.JobManager.UpdateJobStatus(job.ID, "running")

	time.Sleep(1 * time.Second)

	job.Progress = 75
	l.svcCtx.JobManager.UpdateJobStatus(job.ID, "running")

	time.Sleep(1 * time.Second)

	// Complete job
	job.Progress = 100
	job.Status = "completed"
	job.EndTime = time.Now().Format(time.RFC3339)
	job.Result = map[string]interface{}{
		"output": fmt.Sprintf("Function %s executed successfully", job.FunctionID),
		"execution_time": "3s",
	}

	l.svcCtx.JobManager.UpdateJobStatus(job.ID, "completed")

	logx.Infof("Job completed successfully: %s", job.ID)
}