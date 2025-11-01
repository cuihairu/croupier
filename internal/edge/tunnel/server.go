package tunnel

import (
    "errors"
    "log/slog"
    "sync"
    "sync/atomic"
    "time"

    tunnelv1 "github.com/cuihairu/croupier/gen/go/croupier/tunnel/v1"
    functionv1 "github.com/cuihairu/croupier/gen/go/croupier/function/v1"
)

type pending struct{
    ch chan *tunnelv1.ResultFrame
    created time.Time
}

type edgeConn struct{
    agentID string
    gameID string
    env string
    srv tunnelv1.TunnelService_OpenServer
    last time.Time
}

type Server struct {
    tunnelv1.UnimplementedTunnelServiceServer
    mu sync.RWMutex
    agents map[string]*edgeConn // agent_id -> conn
    pendMu sync.Mutex
    pending map[string]*pending // request_id -> chan (invoke/start)
    lpMu sync.Mutex
    pendingList map[string]chan *tunnelv1.ListLocalResponse // request_id -> chan (list_local)
    jrMu sync.Mutex
    pendingJobRes map[string]chan *tunnelv1.GetJobResultResponse // request_id -> chan (job_result)
    evMu sync.RWMutex
    subs map[string][]chan *functionv1.JobEvent // job_id -> subscribers
    jobsMu sync.RWMutex
    jobs map[string]string // job_id -> agent_id
    resMu sync.RWMutex
    results map[string]JobResult // job_id -> result snapshot
    connects int64
    disconnects int64
    invokes int64
    starts int64
    events int64
    cancels int64
}

func NewServer() *Server { return &Server{agents: map[string]*edgeConn{}, pending: map[string]*pending{}, pendingList: map[string]chan *tunnelv1.ListLocalResponse{}, pendingJobRes: map[string]chan *tunnelv1.GetJobResultResponse{}, subs: map[string][]chan *functionv1.JobEvent{}, jobs: map[string]string{}, results: map[string]JobResult{}} }

func (s *Server) Open(stream tunnelv1.TunnelService_OpenServer) error {
    // expect Hello first
    hello, err := stream.Recv()
    if err != nil || hello == nil || hello.Type != "hello" || hello.Hello == nil { return errors.New("bad hello") }
    h := hello.Hello
    conn := &edgeConn{agentID: h.AgentId, gameID: h.GameId, env: h.Env, srv: stream, last: time.Now()}
    s.mu.Lock(); s.agents[h.AgentId] = conn; s.mu.Unlock()
    slog.Info("tunnel connected", "agent_id", h.AgentId, "game_id", h.GameId, "env", h.Env)
    atomic.AddInt64(&s.connects, 1)
    // reader loop for results
    for {
        msg, err := stream.Recv()
        if err != nil { break }
        if msg == nil { continue }
        switch msg.Type {
        case "result":
            if msg.Result != nil {
                s.pendMu.Lock()
                if p := s.pending[msg.Result.RequestId]; p != nil {
                    select { case p.ch <- msg.Result: default: }
                }
                s.pendMu.Unlock()
            }
        case "start_job_result":
            if msg.StartR != nil {
                s.pendMu.Lock()
                if p := s.pending[msg.StartR.RequestId]; p != nil {
                    // overload result via pending: reuse ResultFrame error/payload not used here
                    rf := &tunnelv1.ResultFrame{RequestId: msg.StartR.RequestId, Payload: []byte(msg.StartR.JobId), Error: msg.StartR.Error}
                    select { case p.ch <- rf: default: }
                }
                s.pendMu.Unlock()
            }
        case "job_event":
            if msg.JobEvt != nil {
                ev := msg.JobEvt
                s.evMu.RLock()
                arr := s.subs[ev.JobId]
                s.evMu.RUnlock()
                je := &functionv1.JobEvent{Type: ev.Type, Message: ev.Message, Progress: ev.Progress, Payload: ev.Payload}
                for _, ch := range arr { select { case ch <- je: default: } }
                atomic.AddInt64(&s.events, 1)
                if ev.Type == "done" || ev.Type == "error" {
                    // close and remove subscribers
                    s.evMu.Lock()
                    for _, ch := range s.subs[ev.JobId] { close(ch) }
                    delete(s.subs, ev.JobId)
                    s.evMu.Unlock()
                    // drop job->agent mapping
                    s.jobsMu.Lock(); delete(s.jobs, ev.JobId); s.jobsMu.Unlock()
                    // save final result snapshot
                    st := JobResult{State: ev.Type}
                    if ev.Type == "done" { st.Payload = ev.Payload } else { st.Error = ev.Message }
                    s.resMu.Lock(); s.results[ev.JobId] = st; s.resMu.Unlock()
                }
            }
        case "heartbeat":
            s.mu.Lock(); if c := s.agents[h.AgentId]; c != nil { c.last = time.Now() }; s.mu.Unlock()
        case "list_local_res":
            if msg.ListRes != nil {
                s.lpMu.Lock()
                if ch := s.pendingList[msg.ListRes.RequestId]; ch != nil { select { case ch <- msg.ListRes: default: } }
                s.lpMu.Unlock()
            }
        case "get_job_result_res":
            if msg.JobResRes != nil {
                s.jrMu.Lock()
                if ch := s.pendingJobRes[msg.JobResRes.RequestId]; ch != nil { select { case ch <- msg.JobResRes: default: } }
                s.jrMu.Unlock()
            }
        }
    }
    // cleanup
    s.mu.Lock(); delete(s.agents, h.AgentId); s.mu.Unlock(); atomic.AddInt64(&s.disconnects,1)
    return nil
}

// JobResult holds final status
type JobResult struct { State string; Payload []byte; Error string }

func (s *Server) GetCachedJobResult(jobID string) (JobResult, bool) {
    s.resMu.RLock(); defer s.resMu.RUnlock()
    st, ok := s.results[jobID]
    return st, ok
}

func (s *Server) InvokeViaTunnel(agentID, requestID, functionID string, idem string, payload []byte, meta map[string]string) (*tunnelv1.ResultFrame, error) {
    s.mu.RLock(); conn := s.agents[agentID]; s.mu.RUnlock()
    if conn == nil { return nil, errors.New("agent not connected") }
    // register pending
    ch := make(chan *tunnelv1.ResultFrame, 1)
    s.pendMu.Lock(); s.pending[requestID] = &pending{ch: ch, created: time.Now()}; s.pendMu.Unlock()
    defer func(){ s.pendMu.Lock(); delete(s.pending, requestID); s.pendMu.Unlock() }()
    // send invoke
    msg := &tunnelv1.TunnelMessage{Type:"invoke", Invoke: &tunnelv1.InvokeFrame{RequestId: requestID, FunctionId: functionID, IdempotencyKey: idem, Payload: payload, Metadata: meta}}
    if err := conn.srv.Send(msg); err != nil { return nil, err }
    atomic.AddInt64(&s.invokes, 1)
    // wait result with timeout
    select {
    case res := <-ch:
        return res, nil
    case <-time.After(5 * time.Second):
        return nil, errors.New("tunnel invoke timeout")
    }
}

func (s *Server) StartJobViaTunnel(agentID, requestID, functionID string, idem string, payload []byte, meta map[string]string) (string, error) {
    s.mu.RLock(); conn := s.agents[agentID]; s.mu.RUnlock()
    if conn == nil { return "", errors.New("agent not connected") }
    ch := make(chan *tunnelv1.ResultFrame, 1)
    s.pendMu.Lock(); s.pending[requestID] = &pending{ch: ch, created: time.Now()}; s.pendMu.Unlock()
    defer func(){ s.pendMu.Lock(); delete(s.pending, requestID); s.pendMu.Unlock() }()
    msg := &tunnelv1.TunnelMessage{Type:"start_job", Start: &tunnelv1.StartJobFrame{RequestId: requestID, FunctionId: functionID, IdempotencyKey: idem, Payload: payload, Metadata: meta}}
    if err := conn.srv.Send(msg); err != nil { return "", err }
    atomic.AddInt64(&s.starts, 1)
    select {
    case res := <-ch:
        if res.Error != "" { return "", errors.New(res.Error) }
        jobID := string(res.Payload)
        // record job->agent mapping
        s.jobsMu.Lock(); s.jobs[jobID] = agentID; s.jobsMu.Unlock()
        return jobID, nil
    case <-time.After(5 * time.Second):
        return "", errors.New("tunnel start_job timeout")
    }
}

func (s *Server) SubscribeJob(jobID string) <-chan *functionv1.JobEvent {
    ch := make(chan *functionv1.JobEvent, 16)
    s.evMu.Lock(); s.subs[jobID] = append(s.subs[jobID], ch); s.evMu.Unlock()
    return ch
}

func (s *Server) CancelJobViaTunnel(jobID string) error {
    s.jobsMu.RLock(); agentID := s.jobs[jobID]; s.jobsMu.RUnlock()
    s.mu.RLock(); conn := s.agents[agentID]; s.mu.RUnlock()
    if conn == nil { return errors.New("agent not connected") }
    msg := &tunnelv1.TunnelMessage{Type:"cancel_job", Cancel: &tunnelv1.CancelJobFrame{JobId: jobID}}
    atomic.AddInt64(&s.cancels, 1)
    return conn.srv.Send(msg)
}

// GetJobAgent returns the agent id for a running job if known.
func (s *Server) GetJobAgent(jobID string) (string, bool) {
    s.jobsMu.RLock(); defer s.jobsMu.RUnlock()
    id, ok := s.jobs[jobID]
    return id, ok
}

// ListLocalViaTunnel requests the agent to list service_ids for a function via tunnel.
func (s *Server) ListLocalViaTunnel(agentID, requestID, functionID string) ([]string, error) {
    s.mu.RLock(); conn := s.agents[agentID]; s.mu.RUnlock()
    if conn == nil { return nil, errors.New("agent not connected") }
    ch := make(chan *tunnelv1.ListLocalResponse, 1)
    s.lpMu.Lock(); s.pendingList[requestID] = ch; s.lpMu.Unlock()
    defer func(){ s.lpMu.Lock(); delete(s.pendingList, requestID); s.lpMu.Unlock() }()
    // send request
    msg := &tunnelv1.TunnelMessage{Type: "list_local_req", ListReq: &tunnelv1.ListLocalRequest{RequestId: requestID, FunctionId: functionID}}
    if err := conn.srv.Send(msg); err != nil { return nil, err }
    // await response
    select {
    case res := <-ch:
        if res.Error != "" { return nil, errors.New(res.Error) }
        return res.ServiceIds, nil
    case <-time.After(3 * time.Second):
        return nil, errors.New("tunnel list_local timeout")
    }
}

func (s *Server) GetJobResultViaTunnel(agentID, requestID, jobID string) (JobResult, error) {
    s.mu.RLock(); conn := s.agents[agentID]; s.mu.RUnlock()
    if conn == nil { return JobResult{}, errors.New("agent not connected") }
    ch := make(chan *tunnelv1.GetJobResultResponse, 1)
    s.jrMu.Lock(); s.pendingJobRes[requestID] = ch; s.jrMu.Unlock()
    defer func(){ s.jrMu.Lock(); delete(s.pendingJobRes, requestID); s.jrMu.Unlock() }()
    msg := &tunnelv1.TunnelMessage{Type: "get_job_result_req", JobResReq: &tunnelv1.GetJobResultRequest{RequestId: requestID, JobId: jobID}}
    if err := conn.srv.Send(msg); err != nil { return JobResult{}, err }
    select {
    case res := <-ch:
        return JobResult{State: res.State, Payload: res.Payload, Error: res.Error}, nil
    case <-time.After(3 * time.Second):
        return JobResult{}, errors.New("tunnel get_job_result timeout")
    }
}

// Metrics helpers
func (s *Server) ConnCount() int { s.mu.RLock(); defer s.mu.RUnlock(); return len(s.agents) }
func (s *Server) PendingCount() int { s.pendMu.Lock(); defer s.pendMu.Unlock(); return len(s.pending) }
func (s *Server) JobsCount() int { s.jobsMu.RLock(); defer s.jobsMu.RUnlock(); return len(s.jobs) }
func (s *Server) MetricsMap() map[string]any {
    s.mu.RLock(); conns := len(s.agents); s.mu.RUnlock()
    s.pendMu.Lock(); pend := len(s.pending); s.pendMu.Unlock()
    s.jobsMu.RLock(); jobs := len(s.jobs); s.jobsMu.RUnlock()
    return map[string]any{
        "tunnel_agents": conns,
        "tunnel_pending": pend,
        "tunnel_jobs": jobs,
        "tunnel_connects_total": atomic.LoadInt64(&s.connects),
        "tunnel_disconnects_total": atomic.LoadInt64(&s.disconnects),
        "tunnel_invokes_total": atomic.LoadInt64(&s.invokes),
        "tunnel_starts_total": atomic.LoadInt64(&s.starts),
        "tunnel_events_total": atomic.LoadInt64(&s.events),
        "tunnel_cancels_total": atomic.LoadInt64(&s.cancels),
    }
}
