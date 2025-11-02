package jobserver

import (
    "context"
    jobv1 "github.com/cuihairu/croupier/pkg/pb/croupier/edge/job/v1"
    "github.com/cuihairu/croupier/internal/edge/tunnel"
    "fmt"
    "time"
)

type Server struct{
    jobv1.UnimplementedJobServiceServer
    tun *tunnel.Server
}

func New(t *tunnel.Server) *Server { return &Server{tun: t} }

func (s *Server) GetJobResult(ctx context.Context, req *jobv1.GetJobResultRequest) (*jobv1.GetJobResultResponse, error) {
    if req.JobId == "" { return &jobv1.GetJobResultResponse{State: "unknown", Error: "missing job_id"}, nil }
    // prefer cached
    if st, ok := s.tun.GetCachedJobResult(req.JobId); ok {
        return &jobv1.GetJobResultResponse{State: st.State, Payload: st.Payload, Error: st.Error}, nil
    }
    // fallback: ask agent via tunnel using jobs mapping
    if agentID, ok := s.tun.GetJobAgent(req.JobId); ok {
        rid := fmt.Sprintf("jr-%d", time.Now().UnixNano())
        st, err := s.tun.GetJobResultViaTunnel(agentID, rid, req.JobId)
        if err == nil {
            return &jobv1.GetJobResultResponse{State: st.State, Payload: st.Payload, Error: st.Error}, nil
        }
    }
    return &jobv1.GetJobResultResponse{State: "running"}, nil
}

// Extend tunnel server with a helper; patch added below.
