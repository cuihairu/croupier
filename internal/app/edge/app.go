package edge

import (
	"context"
	"fmt"
	"strings"
	"sync"

	ctrl "github.com/cuihairu/croupier/internal/platform/control"
	dispatch "github.com/cuihairu/croupier/internal/platform/dispatch"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	jobv1 "github.com/cuihairu/croupier/pkg/pb/croupier/edge/job/v1"
	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
	serverv1 "github.com/cuihairu/croupier/pkg/pb/croupier/server/v1"
	tunnelv1 "github.com/cuihairu/croupier/pkg/pb/croupier/tunnel/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// App assembles gRPC services for Edge process.
type App struct {
	ctrl        *ctrl.Server
	dispatcher  *dispatch.Dispatcher
	metricsLock sync.RWMutex
	tunnelStats map[string]int64
}

func New(registry *reg.Store) *App {
	if registry == nil {
		registry = reg.NewStore()
	}
	return &App{
		ctrl:        ctrl.NewServer(registry),
		dispatcher:  dispatch.NewDispatcher(registry),
		tunnelStats: map[string]int64{},
	}
}

// RegisterGRPC registers gRPC services on the given server.
func (a *App) RegisterGRPC(s *grpc.Server) {
	serverv1.RegisterControlServiceServer(s, a.ctrl)
	tunnelv1.RegisterTunnelServiceServer(s, &TunnelServer{dispatcher: a.dispatcher, metrics: a})
	functionv1.RegisterFunctionServiceServer(s, &FunctionServer{dispatcher: a.dispatcher})
	jobv1.RegisterJobServiceServer(s, &JobServer{dispatcher: a.dispatcher})
}

// MetricsMap exposes aggregated metrics.
func (a *App) MetricsMap() map[string]any {
	a.metricsLock.RLock()
	defer a.metricsLock.RUnlock()
	store := a.dispatcher.Store()
	store.Mu().RLock()
	agents := len(store.AgentsUnsafe())
	store.Mu().RUnlock()
	return map[string]any{
		"tunnel_reconnects": a.tunnelStats["reconnects"],
		"active_agents":     agents,
	}
}

// FunctionServer proxies requests to live agents via dispatcher.
type FunctionServer struct {
	functionv1.UnimplementedFunctionServiceServer
	dispatcher *dispatch.Dispatcher
}

func (s *FunctionServer) Invoke(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.InvokeResponse, error) {
	if s.dispatcher == nil {
		return nil, status.Error(codes.Unavailable, "dispatcher not ready")
	}
	return s.dispatcher.InvokeRequest(ctx, cloneInvokeRequest(req))
}

func (s *FunctionServer) StartJob(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.StartJobResponse, error) {
	if s.dispatcher == nil {
		return nil, status.Error(codes.Unavailable, "dispatcher not ready")
	}
	return s.dispatcher.StartJobRequest(ctx, cloneInvokeRequest(req))
}

func (s *FunctionServer) CancelJob(ctx context.Context, req *functionv1.CancelJobRequest) (*functionv1.StartJobResponse, error) {
	if s.dispatcher == nil {
		return nil, status.Error(codes.Unavailable, "dispatcher not ready")
	}
	if err := s.dispatcher.CancelJob(ctx, req.GetJobId()); err != nil {
		return nil, err
	}
	return &functionv1.StartJobResponse{JobId: req.GetJobId()}, nil
}

func (s *FunctionServer) StreamJob(req *functionv1.JobStreamRequest, stream functionv1.FunctionService_StreamJobServer) error {
	if s.dispatcher == nil {
		return status.Error(codes.Unavailable, "dispatcher not ready")
	}
	_, err := s.dispatcher.StreamJobRealtime(stream.Context(), req.GetJobId(), func(evt *functionv1.JobEvent) bool {
		_ = stream.Send(evt)
		return true
	})
	return err
}

// JobServer exposes job helpers for tunnel/API consumption.
type JobServer struct {
	jobv1.UnimplementedJobServiceServer
	dispatcher *dispatch.Dispatcher
}

func (s *JobServer) GetJobResult(ctx context.Context, req *jobv1.GetJobResultRequest) (*jobv1.GetJobResultResponse, error) {
	if s.dispatcher == nil {
		return nil, status.Error(codes.Unavailable, "dispatcher not ready")
	}
	events, done, err := s.dispatcher.StreamJob(ctx, req.GetJobId())
	if err != nil {
		return nil, err
	}
	state := "running"
	if done {
		state = "completed"
		if len(events) > 0 {
			state = strings.ToLower(events[len(events)-1].GetType())
		}
	}
	return &jobv1.GetJobResultResponse{
		State:   state,
		Payload: lastPayload(events),
	}, nil
}

// TunnelServer implements streaming tunnel logic bridging to dispatcher.
type TunnelServer struct {
	tunnelv1.UnimplementedTunnelServiceServer
	dispatcher *dispatch.Dispatcher
	metrics    *App
}

func (s *TunnelServer) Open(stream tunnelv1.TunnelService_OpenServer) error {
	if s.dispatcher == nil {
		return status.Error(codes.Unavailable, "dispatcher not ready")
	}
	ctx := stream.Context()
	var agentID string
	for {
		msg, err := stream.Recv()
		if err != nil {
			return err
		}
		switch {
		case msg.GetHello() != nil:
			agentID = msg.GetHello().GetAgentId()
			s.bumpMetric("reconnects")
		case msg.GetInvoke() != nil:
			frame := msg.GetInvoke()
			resp, err := s.dispatcher.InvokeRequest(ctx, &functionv1.InvokeRequest{
				FunctionId:     frame.GetFunctionId(),
				IdempotencyKey: frame.GetIdempotencyKey(),
				Payload:        frame.GetPayload(),
				Metadata:       frame.GetMetadata(),
			})
			out := &tunnelv1.TunnelMessage{
				Type: "result",
				Result: &tunnelv1.ResultFrame{
					RequestId: frame.GetRequestId(),
					Payload:   nil,
				},
			}
			if err != nil {
				out.Result.Error = err.Error()
			} else {
				out.Result.Payload = resp.GetPayload()
			}
			if err := stream.Send(out); err != nil {
				return err
			}
		case msg.GetStart() != nil:
			frame := msg.GetStart()
			resp, err := s.dispatcher.StartJobRequest(ctx, &functionv1.InvokeRequest{
				FunctionId:     frame.GetFunctionId(),
				IdempotencyKey: frame.GetIdempotencyKey(),
				Payload:        frame.GetPayload(),
				Metadata:       frame.GetMetadata(),
			})
			out := &tunnelv1.TunnelMessage{
				Type: "start_result",
				StartR: &tunnelv1.StartJobResult{
					RequestId: frame.GetRequestId(),
				},
			}
			if err != nil {
				out.StartR.Error = err.Error()
			} else {
				out.StartR.JobId = resp.GetJobId()
				go s.streamJobEvents(ctx, stream, resp.GetJobId())
			}
			if err := stream.Send(out); err != nil {
				return err
			}
		case msg.GetCancel() != nil:
			jobID := msg.GetCancel().GetJobId()
			err := s.dispatcher.CancelJob(ctx, jobID)
			statusMsg := &tunnelv1.TunnelMessage{
				Type: "job_event",
				JobEvt: &tunnelv1.JobEventFrame{
					JobId:   jobID,
					Type:    "canceled",
					Message: "cancel requested",
				},
			}
			if err != nil {
				statusMsg.JobEvt.Message = fmt.Sprintf("cancel failed: %v", err)
			}
			if err := stream.Send(statusMsg); err != nil {
				return err
			}
		case msg.GetListReq() != nil:
			req := msg.GetListReq()
			ids := s.dispatcher.ListFunctionAgents(req.GetFunctionId())
			if err := stream.Send(&tunnelv1.TunnelMessage{
				Type: "list_res",
				ListRes: &tunnelv1.ListLocalResponse{
					RequestId:  req.GetRequestId(),
					FunctionId: req.GetFunctionId(),
					ServiceIds: ids,
				},
			}); err != nil {
				return err
			}
		case msg.GetJobResReq() != nil:
			req := msg.GetJobResReq()
			events, done, err := s.dispatcher.StreamJob(ctx, req.GetJobId())
			resp := &tunnelv1.TunnelMessage{
				Type: "job_res",
				JobResRes: &tunnelv1.GetJobResultResponse{
					RequestId: req.GetRequestId(),
				},
			}
			if err != nil {
				resp.JobResRes.Error = err.Error()
			} else {
				resp.JobResRes.State = jobState(events, done)
				resp.JobResRes.Payload = lastPayload(events)
				for _, evt := range events {
					_ = stream.Send(&tunnelv1.TunnelMessage{
						Type:   "job_event",
						JobEvt: toJobFrame(req.GetJobId(), evt),
					})
				}
			}
			if err := stream.Send(resp); err != nil {
				return err
			}
		default:
			if err := stream.Send(&tunnelv1.TunnelMessage{
				Type: "error",
				Result: &tunnelv1.ResultFrame{
					Error: "unsupported message",
				},
			}); err != nil {
				return err
			}
		}
		if agentID == "" {
			agentID = "unknown"
		}
	}
}

func (s *TunnelServer) bumpMetric(key string) {
	if s.metrics == nil {
		return
	}
	s.metrics.metricsLock.Lock()
	s.metrics.tunnelStats[key]++
	s.metrics.metricsLock.Unlock()
}

func (s *TunnelServer) streamJobEvents(ctx context.Context, stream tunnelv1.TunnelService_OpenServer, jobID string) {
	if jobID == "" {
		return
	}
	_, _ = s.dispatcher.StreamJobRealtime(ctx, jobID, func(evt *functionv1.JobEvent) bool {
		_ = stream.Send(&tunnelv1.TunnelMessage{
			Type:   "job_event",
			JobEvt: toJobFrame(jobID, evt),
		})
		return true
	})
}

func cloneInvokeRequest(req *functionv1.InvokeRequest) *functionv1.InvokeRequest {
	if req == nil {
		return &functionv1.InvokeRequest{}
	}
	if cloned, ok := proto.Clone(req).(*functionv1.InvokeRequest); ok {
		return cloned
	}
	return &functionv1.InvokeRequest{}
}

func jobState(events []*functionv1.JobEvent, done bool) string {
	if !done {
		return "running"
	}
	if len(events) == 0 {
		return "completed"
	}
	return strings.ToLower(events[len(events)-1].GetType())
}

func lastPayload(events []*functionv1.JobEvent) []byte {
	for i := len(events) - 1; i >= 0; i-- {
		if evt := events[i]; evt != nil && len(evt.GetPayload()) > 0 {
			return evt.GetPayload()
		}
	}
	return nil
}

func toJobFrame(jobID string, evt *functionv1.JobEvent) *tunnelv1.JobEventFrame {
	if evt == nil {
		return nil
	}
	return &tunnelv1.JobEventFrame{
		JobId:    jobID,
		Type:     evt.GetType(),
		Message:  evt.GetMessage(),
		Progress: evt.GetProgress(),
		Payload:  evt.GetPayload(),
	}
}
