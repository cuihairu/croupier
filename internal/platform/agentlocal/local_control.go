package agentlocal

import (
	"context"
	"log"
	"time"

	localv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/local/v1"
)

// Server implements LocalControlService for local game servers to register with Agent.
type Server struct {
	localv1.UnimplementedLocalControlServiceServer
	store *LocalStore
}

func NewServer(store *LocalStore) *Server {
	if store == nil {
		store = NewLocalStore()
	}
	return &Server{store: store}
}

func (s *Server) RegisterLocal(ctx context.Context, in *localv1.RegisterLocalRequest) (*localv1.RegisterLocalResponse, error) {
	log.Printf("[agentlocal] DEBUG: RegisterLocal RPC received from %s", in.GetServiceId())
	s.store.Register(in.GetServiceId(), in.GetRpcAddr(), in.GetVersion(), in.GetFunctions())
	return &localv1.RegisterLocalResponse{SessionId: in.GetServiceId() + ":" + time.Now().Format("150405")}, nil
}

func (s *Server) Heartbeat(ctx context.Context, in *localv1.HeartbeatRequest) (*localv1.HeartbeatResponse, error) {
	s.store.Heartbeat(in.GetServiceId())
	return &localv1.HeartbeatResponse{}, nil
}

func (s *Server) ListLocal(ctx context.Context, in *localv1.ListLocalRequest) (*localv1.ListLocalResponse, error) {
	snap := s.store.List()
	out := &localv1.ListLocalResponse{}
	for fid, arr := range snap {
		fn := &localv1.LocalFunction{Id: fid}
		for _, it := range arr {
			fn.Instances = append(fn.Instances, &localv1.LocalInstance{ServiceId: it.ServiceID, Addr: it.Addr, Version: it.Version, LastSeen: it.LastSeen.Format(time.RFC3339)})
		}
		out.Functions = append(out.Functions, fn)
	}
	return out, nil
}

func (s *Server) GetJobResult(ctx context.Context, in *localv1.GetJobResultRequest) (*localv1.GetJobResultResponse, error) {
	if in == nil || in.GetJobId() == "" {
		return &localv1.GetJobResultResponse{
			State: "failed",
			Error: "job_id is required",
		}, nil
	}

	result, exists := s.store.GetJobResult(in.GetJobId())
	if !exists {
		return &localv1.GetJobResultResponse{
			State: "not_found",
			Error: "job not found",
		}, nil
	}

	return &localv1.GetJobResultResponse{
		State:   result.State,
		Payload: result.Payload,
		Error:   result.Error,
	}, nil
}
