package tunnel

import (
    "errors"
    "log"
    "sync"
    "time"

    tunnelv1 "github.com/your-org/croupier/gen/go/croupier/tunnel/v1"
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
}

type Server struct {
    tunnelv1.UnimplementedTunnelServiceServer
    mu sync.RWMutex
    agents map[string]*edgeConn // agent_id -> conn
    pendMu sync.Mutex
    pending map[string]*pending // request_id -> chan
}

func NewServer() *Server { return &Server{agents: map[string]*edgeConn{}, pending: map[string]*pending{}} }

func (s *Server) Open(stream tunnelv1.TunnelService_OpenServer) error {
    // expect Hello first
    hello, err := stream.Recv()
    if err != nil || hello == nil || hello.Type != "hello" || hello.Hello == nil { return errors.New("bad hello") }
    h := hello.Hello
    conn := &edgeConn{agentID: h.AgentId, gameID: h.GameId, env: h.Env, srv: stream}
    s.mu.Lock(); s.agents[h.AgentId] = conn; s.mu.Unlock()
    log.Printf("tunnel: agent connected id=%s game=%s env=%s", h.AgentId, h.GameId, h.Env)
    // reader loop for results
    for {
        msg, err := stream.Recv()
        if err != nil { break }
        if msg == nil { continue }
        if msg.Type == "result" && msg.Result != nil {
            s.pendMu.Lock()
            if p := s.pending[msg.Result.RequestId]; p != nil {
                select { case p.ch <- msg.Result: default: }
            }
            s.pendMu.Unlock()
        }
    }
    // cleanup
    s.mu.Lock(); delete(s.agents, h.AgentId); s.mu.Unlock()
    return nil
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
    // wait result with timeout
    select {
    case res := <-ch:
        return res, nil
    case <-time.After(5 * time.Second):
        return nil, errors.New("tunnel invoke timeout")
    }
}

