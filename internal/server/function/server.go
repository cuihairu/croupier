package function

import (
    "context"
    "errors"
    "fmt"
    "log/slog"
    "sync"
    "time"

    functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
    "github.com/cuihairu/croupier/internal/connpool"
    "github.com/cuihairu/croupier/internal/jobs"
    "github.com/cuihairu/croupier/internal/loadbalancer"
    "github.com/cuihairu/croupier/internal/server/registry"

    "google.golang.org/grpc"
    localv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/local/v1"
    "google.golang.org/grpc/credentials/insecure"
)

// Server implements FunctionService at Server side, routing calls to agents.
type Server struct {
    functionv1.UnimplementedFunctionServiceServer
    store       *registry.Store
    jobs        *jobs.Router
    balancer    loadbalancer.LoadBalancer
    connPool    connpool.ConnectionPool
    stats       loadbalancer.StatsCollector
    healthCheck loadbalancer.HealthChecker
    // per-agent rate limit (ops: service-level QPS)
    rlMu        sync.Mutex
    rateLookup  func(agentID string) int
    agentRL     map[string]*agentLimiter
}

// ServerConfig holds configuration for the function server
type ServerConfig struct {
    // LoadBalancerStrategy defines which load balancing strategy to use
    // Options: "round_robin", "weighted_round_robin", "least_connections", "consistent_hash"
    LoadBalancerStrategy string
    // ConnectionPool configuration
    ConnPoolConfig *connpool.PoolConfig
    // HealthCheck interval
    HealthCheckInterval time.Duration
}

func NewServer(store *registry.Store, config *ServerConfig) *Server {
    if config == nil {
        config = &ServerConfig{
            LoadBalancerStrategy: "round_robin",
            ConnPoolConfig:       connpool.DefaultPoolConfig(),
            HealthCheckInterval:  30 * time.Second,
        }
    }

    // Initialize components
    stats := loadbalancer.NewDefaultStatsCollector()
    healthCheck := loadbalancer.NewDefaultHealthChecker(config.HealthCheckInterval)
    connPool := connpool.NewConnectionPool(config.ConnPoolConfig)

    // Initialize load balancer based on strategy
    var balancer loadbalancer.LoadBalancer
    switch config.LoadBalancerStrategy {
    case "weighted_round_robin":
        balancer = loadbalancer.NewWeightedRoundRobinBalancer(healthCheck)
    case "least_connections":
        balancer = loadbalancer.NewLeastConnectionsBalancer(stats, healthCheck)
    case "consistent_hash":
        balancer = loadbalancer.NewConsistentHashBalancer(150, healthCheck)
    default: // "round_robin"
        balancer = loadbalancer.NewRoundRobinBalancer(healthCheck)
    }

    return &Server{
        store:       store,
        jobs:        jobs.NewRouter(),
        balancer:    balancer,
        connPool:    connPool,
        stats:       stats,
        healthCheck: healthCheck,
        agentRL:     map[string]*agentLimiter{},
    }
}

// agentLimiter is a simple token bucket per agent
type agentLimiter struct {
    cap    int
    tokens float64
    last   time.Time
    rate   float64
}

func newAgentLimiter(rps int) *agentLimiter { return &agentLimiter{cap: rps, tokens: float64(rps), last: time.Now(), rate: float64(rps)} }
func (r *agentLimiter) try() bool {
    now := time.Now()
    dt := now.Sub(r.last).Seconds()
    if dt > 0 {
        r.tokens += dt * r.rate
        if r.tokens > float64(r.cap) { r.tokens = float64(r.cap) }
        r.last = now
    }
    if r.tokens >= 1 {
        r.tokens -= 1
        return true
    }
    return false
}

// SetServiceRateLookup configures per-agent QPS provider used to enforce service-level rate limits.
func (s *Server) SetServiceRateLookup(fn func(agentID string) int) { s.rlMu.Lock(); defer s.rlMu.Unlock(); s.rateLookup = fn }

func (s *Server) getAgentLimiter(agentID string) *agentLimiter {
    s.rlMu.Lock()
    defer s.rlMu.Unlock()
    if s.rateLookup == nil { return nil }
    rps := s.rateLookup(agentID)
    if rps <= 0 { return nil }
    rl := s.agentRL[agentID]
    if rl == nil || rl.cap != rps {
        rl = newAgentLimiter(rps)
        s.agentRL[agentID] = rl
    }
    return rl
}

func (s *Server) pickAgent(fid, gameID, hashKey string) (*registry.AgentSession, error) {
    // Get candidates for this function and game
    cands := s.store.AgentsForFunctionScoped(gameID, fid, true)
    if len(cands) == 0 {
        return nil, errors.New("no agent available")
    }

    // Use load balancer to pick the best agent
    ctx := context.Background()
    agent, err := s.balancer.Pick(ctx, cands, hashKey)
    if err != nil {
        return nil, fmt.Errorf("load balancer failed to pick agent: %w", err)
    }

    return agent, nil
}

func (s *Server) getConnection(ctx context.Context, agentAddr string) (*grpc.ClientConn, error) {
    // Use connection pool to get/create connection
    conn, err := s.connPool.Get(ctx, agentAddr)
    if err != nil {
        return nil, fmt.Errorf("failed to get connection: %w", err)
    }
    return conn, nil
}

func (s *Server) recordStats(agentID string, start time.Time, success bool) {
    duration := time.Since(start)
    s.stats.RecordRequest(agentID, duration, success)
}

func (s *Server) Invoke(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.InvokeResponse, error) {
    start := time.Now()
    var gameID, hashKey, route, target string
    if req.Metadata != nil {
        gameID = req.Metadata["game_id"]
        hashKey = req.Metadata["hash_key"]
        route = req.Metadata["route"]
        target = req.Metadata["target_service_id"]
    }

    var agent *registry.AgentSession
    var err error
    if route == "targeted" && target != "" {
        // Find the agent that actually hosts the target service id
        agent, err = s.findAgentForTarget(ctx, req.GetFunctionId(), gameID, target)
        if err != nil {
            return nil, err
        }
    } else {
        // Pick agent using load balancer
        agent, err = s.pickAgent(req.GetFunctionId(), gameID, hashKey)
        if err != nil {
            return nil, err
        }
    }

    // Service-level rate limit (if configured)
    if rl := s.getAgentLimiter(agent.AgentID); rl != nil && !rl.try() {
        s.balancer.UpdateHealth(agent.AgentID, true)
        return nil, fmt.Errorf("rate limited")
    }
    // Increment active connections for stats
    s.stats.IncrementActiveConns(agent.AgentID)
    defer s.stats.DecrementActiveConns(agent.AgentID)

    // Get connection from pool
    conn, err := s.getConnection(ctx, agent.RPCAddr)
    if err != nil {
        s.recordStats(agent.AgentID, start, false)
        s.balancer.UpdateHealth(agent.AgentID, false)
        return nil, fmt.Errorf("dial agent %s: %w", agent.AgentID, err)
    }

    // Create client
    cli := functionv1.NewFunctionServiceClient(conn)

    // Log request
    trace := ""
    if req.Metadata != nil {
        trace = req.Metadata["trace_id"]
    }
    slog.Info("route invoke", "function_id", req.GetFunctionId(), "agent_id", agent.AgentID, "rpc_addr", agent.RPCAddr, "trace_id", trace, "idempotency_key", req.GetIdempotencyKey(), "strategy", s.balancer.Name(), "route", route, "hash_key", hashKey)

    // Make the call
    resp, err := cli.Invoke(ctx, req)
    success := err == nil

    // Record stats and update health
    s.recordStats(agent.AgentID, start, success)
    s.balancer.UpdateHealth(agent.AgentID, success)

    return resp, err
}

func (s *Server) StartJob(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.StartJobResponse, error) {
    start := time.Now()
    var gameID, hashKey, route, target string
    if req.Metadata != nil {
        gameID = req.Metadata["game_id"]
        hashKey = req.Metadata["hash_key"]
        route = req.Metadata["route"]
        target = req.Metadata["target_service_id"]
    }
    var agent *registry.AgentSession
    var err error
    if route == "targeted" && target != "" {
        agent, err = s.findAgentForTarget(ctx, req.GetFunctionId(), gameID, target)
        if err != nil { return nil, err }
    } else {
        agent, err = s.pickAgent(req.GetFunctionId(), gameID, hashKey)
        if err != nil { return nil, err }
    }

    // Service-level rate limit (if configured)
    if rl := s.getAgentLimiter(agent.AgentID); rl != nil && !rl.try() {
        s.balancer.UpdateHealth(agent.AgentID, true)
        return nil, fmt.Errorf("rate limited")
    }
    // Increment active connections for stats
    s.stats.IncrementActiveConns(agent.AgentID)
    defer s.stats.DecrementActiveConns(agent.AgentID)

    // Get connection from pool
    conn, err := s.getConnection(ctx, agent.RPCAddr)
    if err != nil {
        s.recordStats(agent.AgentID, start, false)
        s.balancer.UpdateHealth(agent.AgentID, false)
        return nil, fmt.Errorf("dial agent %s: %w", agent.AgentID, err)
    }

    // Create client
    cli := functionv1.NewFunctionServiceClient(conn)

    // Log request
    trace := ""
    if req.Metadata != nil {
        trace = req.Metadata["trace_id"]
    }
    slog.Info("route start_job", "function_id", req.GetFunctionId(), "agent_id", agent.AgentID, "rpc_addr", agent.RPCAddr, "trace_id", trace, "idempotency_key", req.GetIdempotencyKey(), "strategy", s.balancer.Name(), "route", route, "hash_key", hashKey)

    // Make the call
    resp, err := cli.StartJob(ctx, req)
    success := err == nil

    // Record stats and update health
    s.recordStats(agent.AgentID, start, success)
    s.balancer.UpdateHealth(agent.AgentID, success)

    // Store job mapping if successful
    if err == nil {
        s.jobs.Set(resp.GetJobId(), agent.RPCAddr)
    }

    return resp, err
}

// findAgentForTarget scans candidate agents for a function in a given game scope
// and returns the one that exposes the target service_id, by querying the agent's
// LocalControl service. This avoids routing targeted requests to the wrong agent.
func (s *Server) findAgentForTarget(ctx context.Context, fid, gameID, targetServiceID string) (*registry.AgentSession, error) {
    cands := s.store.AgentsForFunctionScoped(gameID, fid, true)
    if len(cands) == 0 { return nil, errors.New("no agent available") }
    // short timeout for discovery per agent
    for _, a := range cands {
        // Best-effort insecure dial to agent's local control (DEV topology)
        cc, err := grpc.DialContext(ctx, a.RPCAddr,
            grpc.WithTransportCredentials(insecure.NewCredentials()),
            grpc.WithDefaultCallOptions(grpc.CallContentSubtype("json")))
        if err != nil { continue }
        cli := localv1.NewLocalControlServiceClient(cc)
        dctx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
        resp, err := cli.ListLocal(dctx, &localv1.ListLocalRequest{})
        cancel()
        _ = cc.Close()
        if err != nil || resp == nil { continue }
        for _, lf := range resp.Functions {
            if lf.Id != fid { continue }
            for _, inst := range lf.Instances {
                if inst.ServiceId == targetServiceID { return a, nil }
            }
        }
    }
    return nil, fmt.Errorf("target service not found: %s", targetServiceID)
}

func (s *Server) StreamJob(req *functionv1.JobStreamRequest, stream functionv1.FunctionService_StreamJobServer) error {
    rpcAddr, ok := s.jobs.Get(req.GetJobId())
    if !ok {
        return errors.New("unknown job")
    }

    // Get connection from pool
    conn, err := s.getConnection(stream.Context(), rpcAddr)
    if err != nil {
        return fmt.Errorf("dial agent: %w", err)
    }

    cli := functionv1.NewFunctionServiceClient(conn)

    // Fan-out events from agent to caller
    agentStream, err := cli.StreamJob(stream.Context(), req)
    if err != nil {
        return err
    }

    for {
        ev, err := agentStream.Recv()
        if err != nil {
            return err
        }
        if err := stream.Send(ev); err != nil {
            return err
        }
        if ev.GetType() == "done" || ev.GetType() == "error" {
            return nil
        }
    }
}

func (s *Server) CancelJob(ctx context.Context, req *functionv1.CancelJobRequest) (*functionv1.StartJobResponse, error) {
    rpcAddr, ok := s.jobs.Get(req.GetJobId())
    if !ok {
        return nil, errors.New("unknown job")
    }

    // Get connection from pool
    conn, err := s.getConnection(ctx, rpcAddr)
    if err != nil {
        return nil, fmt.Errorf("dial agent: %w", err)
    }

    cli := functionv1.NewFunctionServiceClient(conn)
    return cli.CancelJob(ctx, req)
}

// JobLocator interface for HTTP layer to resolve job_id -> agent address
func (s *Server) GetJobAddr(jobID string) (string, bool) {
    addr, ok := s.jobs.Get(jobID)
    return addr, ok
}

// Close cleans up server resources
func (s *Server) Close() error {
    if s.connPool != nil {
        return s.connPool.Close()
    }
    return nil
}

// GetStats returns current load balancer statistics
func (s *Server) GetStats() map[string]*loadbalancer.AgentStats {
    if s.stats != nil {
        return s.stats.GetAllStats()
    }
    return nil
}

// GetPoolStats returns connection pool statistics
func (s *Server) GetPoolStats() *connpool.PoolStats {
    if s.connPool != nil {
        return s.connPool.Stats()
    }
    return nil
}

// Implement client-like helper to satisfy httpserver.FunctionInvoker
func (s *Server) StreamJobClient(ctx context.Context, req *functionv1.JobStreamRequest) (functionv1.FunctionService_StreamJobClient, error) {
    rpcAddr, ok := s.jobs.Get(req.GetJobId())
    if !ok {
        return nil, errors.New("unknown job")
    }

    // Get connection from pool
    conn, err := s.getConnection(ctx, rpcAddr)
    if err != nil {
        return nil, err
    }

    // Note: caller must manage connection lifecycle when using this method
    cli := functionv1.NewFunctionServiceClient(conn)
    return cli.StreamJob(ctx, req)
}

// clientAdapter wraps Server to expose client-style StreamJob for httpserver.
type clientAdapter struct{ s *Server }

func (a *clientAdapter) Invoke(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.InvokeResponse, error) {
    return a.s.Invoke(ctx, req)
}

func (a *clientAdapter) StartJob(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.StartJobResponse, error) {
    return a.s.StartJob(ctx, req)
}

func (a *clientAdapter) StreamJob(ctx context.Context, req *functionv1.JobStreamRequest) (functionv1.FunctionService_StreamJobClient, error) {
    return a.s.StreamJobClient(ctx, req)
}

func (a *clientAdapter) CancelJob(ctx context.Context, req *functionv1.CancelJobRequest) (*functionv1.StartJobResponse, error) {
    return a.s.CancelJob(ctx, req)
}

func NewClientAdapter(s *Server) *clientAdapter { return &clientAdapter{s: s} }

// Expose server for stats when needed (HTTP metrics)
func (a *clientAdapter) S() *Server { return a.s }

// Optional stats provider interface for HTTP metrics
func (a *clientAdapter) GetStats() map[string]*loadbalancer.AgentStats { return a.s.GetStats() }
func (a *clientAdapter) GetPoolStats() *connpool.PoolStats { return a.s.GetPoolStats() }
func (a *clientAdapter) SetServiceRateLookup(fn func(string) int) { a.s.SetServiceRateLookup(fn) }
