package job

import (
	"context"
	"encoding/json"
	"strings"

	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/svc"
)

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// List retrieves a paginated list of jobs
func (s *Service) List(ctx context.Context, req *JobListRequest) (*JobListResponse, error) {
	// TODO: Implement actual job listing from dispatcher
	return &JobListResponse{
		Jobs:  []JobItem{},
		Total: 0,
	}, nil
}

// Start starts a new job
func (s *Service) Start(ctx context.Context, req *JobStartRequest) (*JobStartResponse, error) {
	functionID, err := utils.ValidateFunctionID(req.FunctionID)
	if err != nil {
		return nil, err
	}

	if _, err := s.svcCtx.FunctionModel.FindByFunctionID(ctx, functionID); err != nil {
		return nil, err
	}

	payload, err := json.Marshal(req.Params)
	if err != nil {
		return nil, err
	}

	jobID, err := s.svcCtx.Dispatcher.StartJob(ctx, functionID, payload)
	if err != nil {
		return nil, err
	}

	return &JobStartResponse{
		JobID: jobID,
	}, nil
}

// Cancel cancels a job
func (s *Service) Cancel(ctx context.Context, req *JobCancelRequest) error {
	jobID := strings.TrimSpace(req.ID)
	if jobID == "" {
		return errorx.NewBadRequest("任务ID不能为空")
	}

	return s.svcCtx.Dispatcher.CancelJob(ctx, jobID)
}

// Result retrieves the result of a job
func (s *Service) Result(ctx context.Context, req *JobResultRequest) (*JobResultResponse, error) {
	jobID := strings.TrimSpace(req.ID)
	if jobID == "" {
		return nil, errorx.NewBadRequest("任务ID不能为空")
	}

	events, done, err := s.svcCtx.Dispatcher.StreamJob(ctx, jobID)
	if err != nil {
		return nil, err
	}

	// Convert []*sdkv1.JobEvent to []map[string]interface{}
	convertedEvents := make([]map[string]interface{}, 0, len(events))
	for _, evt := range events {
		if evt == nil {
			continue
		}
		convertedEvents = append(convertedEvents, map[string]interface{}{
			"type":     evt.Type,
			"message":  evt.Message,
			"progress": evt.GetProgress(),
			"payload":  evt.Payload,
		})
	}

	return &JobResultResponse{
		JobID:  jobID,
		Done:   done,
		Events: convertedEvents,
	}, nil
}

// Stream streams job events in real-time
func (s *Service) Stream(ctx context.Context, req *StreamJobRequest) (*StreamJobResponse, error) {
	jobID := strings.TrimSpace(req.JobID)
	if jobID == "" {
		return nil, errorx.NewBadRequest("任务ID不能为空")
	}

	result := make([]map[string]interface{}, 0)
	done, err := s.svcCtx.Dispatcher.StreamJobRealtime(ctx, jobID, func(evt *sdkv1.JobEvent) bool {
		if evt == nil {
			return true
		}
		result = append(result, map[string]interface{}{
			"type":     evt.Type,
			"message":  evt.Message,
			"progress": evt.GetProgress(),
			"payload":  evt.Payload,
		})
		return true
	})

	if err != nil {
		return nil, err
	}

	return &StreamJobResponse{
		Events: result,
		Done:   done,
	}, nil
}
