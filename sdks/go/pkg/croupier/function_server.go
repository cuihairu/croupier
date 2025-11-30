package croupier

import (
	"context"
	"fmt"
	"sync"
	"time"

	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type jobState struct {
	events chan *functionv1.JobEvent
	cancel context.CancelFunc
}

// functionServer exposes FunctionService over the registered Go handlers.
type functionServer struct {
	functionv1.UnimplementedFunctionServiceServer

	handlers map[string]FunctionHandler

	mu   sync.RWMutex
	jobs map[string]*jobState
}

func newFunctionServer(handlers map[string]FunctionHandler) *functionServer {
	return &functionServer{
		handlers: handlers,
		jobs:     make(map[string]*jobState),
	}
}

func (s *functionServer) handler(id string) (FunctionHandler, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	h, ok := s.handlers[id]
	return h, ok
}

func (s *functionServer) Invoke(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.InvokeResponse, error) {
	if req == nil || req.GetFunctionId() == "" {
		return nil, status.Error(codes.InvalidArgument, "function_id is required")
	}
	handler, ok := s.handler(req.GetFunctionId())
	if !ok {
		return nil, status.Errorf(codes.NotFound, "function %s not registered", req.GetFunctionId())
	}
	payload, err := handler(ctx, req.GetPayload())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "handler failed: %v", err)
	}
	return &functionv1.InvokeResponse{Payload: payload}, nil
}

func (s *functionServer) StartJob(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.StartJobResponse, error) {
	if req == nil || req.GetFunctionId() == "" {
		return nil, status.Error(codes.InvalidArgument, "function_id is required")
	}
	handler, ok := s.handler(req.GetFunctionId())
	if !ok {
		return nil, status.Errorf(codes.NotFound, "function %s not registered", req.GetFunctionId())
	}

	jobID := fmt.Sprintf("%s-%d", req.GetFunctionId(), time.Now().UnixNano())
	jobCtx, cancel := context.WithCancel(context.Background())
	state := &jobState{
		events: make(chan *functionv1.JobEvent, 4),
		cancel: cancel,
	}

	s.mu.Lock()
	s.jobs[jobID] = state
	s.mu.Unlock()

	state.events <- &functionv1.JobEvent{Type: "started", Message: "job started"}

	go s.runJob(jobID, jobCtx, handler, req.GetPayload(), state.events)

	return &functionv1.StartJobResponse{JobId: jobID}, nil
}

func (s *functionServer) runJob(jobID string, jobCtx context.Context, handler FunctionHandler, payload []byte, ch chan *functionv1.JobEvent) {
	defer func() {
		close(ch)
		s.mu.Lock()
		delete(s.jobs, jobID)
		s.mu.Unlock()
	}()

	result, err := handler(jobCtx, payload)
	if err != nil {
		if jobCtx.Err() != nil {
			ch <- &functionv1.JobEvent{
				Type:    "cancelled",
				Message: jobCtx.Err().Error(),
			}
		} else {
			ch <- &functionv1.JobEvent{
				Type:    "error",
				Message: err.Error(),
			}
		}
		return
	}

	ch <- &functionv1.JobEvent{
		Type:    "completed",
		Payload: result,
		Message: "job completed",
	}
}

func (s *functionServer) StreamJob(req *functionv1.JobStreamRequest, stream functionv1.FunctionService_StreamJobServer) error {
	if req == nil || req.GetJobId() == "" {
		return status.Error(codes.InvalidArgument, "job_id is required")
	}

	s.mu.RLock()
	state, ok := s.jobs[req.GetJobId()]
	s.mu.RUnlock()

	if !ok {
		return status.Errorf(codes.NotFound, "job %s not found", req.GetJobId())
	}

	for event := range state.events {
		if err := stream.Send(event); err != nil {
			return err
		}
	}
	return nil
}

func (s *functionServer) CancelJob(ctx context.Context, req *functionv1.CancelJobRequest) (*functionv1.StartJobResponse, error) {
	if req == nil || req.GetJobId() == "" {
		return nil, status.Error(codes.InvalidArgument, "job_id is required")
	}

	s.mu.Lock()
	state, ok := s.jobs[req.GetJobId()]
	s.mu.Unlock()

	if ok && state.cancel != nil {
		state.cancel()
	}

	return &functionv1.StartJobResponse{JobId: req.GetJobId()}, nil
}
