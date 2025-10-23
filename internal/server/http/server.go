package httpserver

import (
    "encoding/json"
    "log"
    "net/http"

    "github.com/cuihairu/croupier/internal/function/descriptor"
    functionv1 "github.com/cuihairu/croupier/gen/go/croupier/function/v1"
    "context"
    "crypto/rand"
    "encoding/hex"
    "fmt"
    "github.com/cuihairu/croupier/internal/validation"
    auditchain "github.com/cuihairu/croupier/internal/audit/chain"
    "github.com/cuihairu/croupier/internal/auth/rbac"
    "os"
    "github.com/cuihairu/croupier/internal/server/games"
    "github.com/cuihairu/croupier/internal/server/registry"
    "bufio"
    "strings"
    "time"
    "sync/atomic"
    users "github.com/cuihairu/croupier/internal/auth/users"
    jwt "github.com/cuihairu/croupier/internal/auth/token"
    localv1 "github.com/cuihairu/croupier/gen/go/croupier/agent/local/v1"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
)

type Server struct {
    mux   *http.ServeMux
    descs []*descriptor.Descriptor
    descIndex map[string]*descriptor.Descriptor
    invoker FunctionInvoker
    audit *auditchain.Writer
    rbac  *rbac.Policy
    games *games.Store
    reg   *registry.Store
    userStore *users.Store
    jwtMgr    *jwt.Manager
    startedAt time.Time
    invocations int64
    jobsStarted int64
}

type FunctionInvoker interface {
    Invoke(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.InvokeResponse, error)
    StartJob(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.StartJobResponse, error)
    StreamJob(ctx context.Context, req *functionv1.JobStreamRequest) (functionv1.FunctionService_StreamJobClient, error)
    CancelJob(ctx context.Context, req *functionv1.CancelJobRequest) (*functionv1.StartJobResponse, error)
}

func NewServer(descriptorDir string, invoker FunctionInvoker, audit *auditchain.Writer, policy *rbac.Policy, gamesStore *games.Store, reg *registry.Store, userStore *users.Store, jwtMgr *jwt.Manager) (*Server, error) {
    descs, err := descriptor.LoadAll(descriptorDir)
    if err != nil { return nil, err }
    idx := map[string]*descriptor.Descriptor{}
    for _, d := range descs { idx[d.ID] = d }
    s := &Server{mux: http.NewServeMux(), descs: descs, descIndex: idx, invoker: invoker, audit: audit, rbac: policy, games: gamesStore, reg: reg, userStore: userStore, jwtMgr: jwtMgr, startedAt: time.Now()}
    s.routes()
    return s, nil
}

func (s *Server) routes() {
    // Auth endpoints
    s.mux.HandleFunc("/api/auth/login", func(w http.ResponseWriter, r *http.Request) {
        addCORS(w, r)
        if r.Method != http.MethodPost { w.WriteHeader(http.StatusMethodNotAllowed); return }
        if s.userStore == nil || s.jwtMgr == nil { http.Error(w, "auth disabled", http.StatusServiceUnavailable); return }
        var in struct{ Username string `json:"username"`; Password string `json:"password"` }
        if err := json.NewDecoder(r.Body).Decode(&in); err != nil { http.Error(w, err.Error(), 400); return }
        u, err := s.userStore.Verify(in.Username, in.Password)
        if err != nil { http.Error(w, "unauthorized", http.StatusUnauthorized); return }
        tok, _ := s.jwtMgr.Sign(u.Username, u.Roles, 8*time.Hour)
        _ = json.NewEncoder(w).Encode(struct{ Token string `json:"token"`; User any `json:"user"` }{Token: tok, User: struct{ Username string `json:"username"`; Roles []string `json:"roles"` }{u.Username, u.Roles}})
    })
    s.mux.HandleFunc("/api/auth/me", func(w http.ResponseWriter, r *http.Request) {
        addCORS(w, r)
        user, roles, ok := s.auth(r)
        if !ok { http.Error(w, "unauthorized", http.StatusUnauthorized); return }
        _ = json.NewEncoder(w).Encode(struct{ Username string `json:"username"`; Roles []string `json:"roles"` }{user, roles})
    })
    s.mux.HandleFunc("/api/descriptors", func(w http.ResponseWriter, r *http.Request) {
        addCORS(w, r)
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(s.descs)
    })
    s.mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        _, _ = w.Write([]byte("ok"))
    })
    s.mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
        addCORS(w, r)
        _ = json.NewEncoder(w).Encode(map[string]any{
            "uptime_sec": int(time.Since(s.startedAt).Seconds()),
            "invocations_total": s.invocations,
            "jobs_started_total": s.jobsStarted,
        })
    })
    s.mux.HandleFunc("/api/invoke", func(w http.ResponseWriter, r *http.Request) {
        addCORS(w, r)
        if r.Method != http.MethodPost { w.WriteHeader(http.StatusMethodNotAllowed); return }
        user, _, ok := s.auth(r)
        if !ok { http.Error(w, "unauthorized", http.StatusUnauthorized); return }
        gameID := r.Header.Get("X-Game-ID")
        env := r.Header.Get("X-Env")
        var in struct{
            FunctionID string `json:"function_id"`
            Payload    any    `json:"payload"`
            IdempotencyKey string `json:"idempotency_key"`
            Route string `json:"route"`
            TargetServiceID string `json:"target_service_id"`
        }
        if err := json.NewDecoder(r.Body).Decode(&in); err != nil { http.Error(w, err.Error(), 400); return }
        // schema validation (best-effort)
        if d := s.descIndex[in.FunctionID]; d != nil {
            if ps := d.Params; ps != nil {
                b, _ := json.Marshal(in.Payload)
                if err := validation.ValidateJSON(ps, b); err != nil { http.Error(w, fmt.Sprintf("payload invalid: %v", err), 400); return }
            }
        }
        // rbac check (scoped)
        basePerm := "function:" + in.FunctionID
        if d := s.descIndex[in.FunctionID]; d != nil {
            if auth := d.Auth; auth != nil {
                if p, ok := auth["permission"].(string); ok && p != "" { basePerm = p }
            }
        }
        scopedOk := false
        if s.rbac != nil {
            scoped := basePerm
            if gameID != "" { scoped = "game:" + gameID + ":" + basePerm }
            // by username or by role
            if s.rbac.Can(user, scoped) || s.rbac.Can(user, basePerm) || (gameID != "" && s.rbac.Can(user, "game:"+gameID+":*")) || s.rbac.Can(user, "*") {
                scopedOk = true
            } else {
                _, roles, _ := s.auth(r)
                for _, role := range roles {
                    if s.rbac.Can("role:"+role, scoped) || s.rbac.Can("role:"+role, basePerm) { scopedOk = true; break }
                }
            }
        } else { scopedOk = true }
        if !scopedOk { http.Error(w, "forbidden", http.StatusForbidden); return }
        b, err := json.Marshal(in.Payload)
        if err != nil { http.Error(w, err.Error(), 400); return }
        if in.IdempotencyKey == "" { in.IdempotencyKey = randHex(16) }
        traceID := randHex(8)
        _ = s.audit.Log("invoke", user, in.FunctionID, map[string]string{"ip": r.RemoteAddr, "trace_id": traceID, "game_id": gameID, "env": env})
        meta := map[string]string{"trace_id": traceID}
        if gameID != "" { meta["game_id"] = gameID }
        if env != "" { meta["env"] = env }
        // route selection: request override > descriptor.semantics.route
        if in.Route != "" { meta["route"] = in.Route } else if d := s.descIndex[in.FunctionID]; d != nil {
            if sem := d.Semantics; sem != nil {
                if rv, ok := sem["route"].(string); ok && rv != "" { meta["route"] = rv }
            }
        }
        // validate route value
        if rv, ok := meta["route"]; ok && rv != "lb" && rv != "broadcast" && rv != "targeted" {
            http.Error(w, "invalid route", 400); return
        }
        if in.TargetServiceID != "" { meta["target_service_id"] = in.TargetServiceID }
        if meta["route"] == "targeted" && meta["target_service_id"] == "" { http.Error(w, "target_service_id required for targeted route", 400); return }
        resp, err := s.invoker.Invoke(r.Context(), &functionv1.InvokeRequest{FunctionId: in.FunctionID, IdempotencyKey: in.IdempotencyKey, Payload: b, Metadata: meta})
        if err != nil { http.Error(w, err.Error(), 500); return }
        atomic.AddInt64(&s.invocations, 1)
        w.Header().Set("Content-Type", "application/json")
        if len(resp.GetPayload()) == 0 {
            w.WriteHeader(204); return
        }
        _, _ = w.Write(resp.GetPayload())
    })
    s.mux.HandleFunc("/api/start_job", func(w http.ResponseWriter, r *http.Request) {
        addCORS(w, r)
        if r.Method != http.MethodPost { w.WriteHeader(http.StatusMethodNotAllowed); return }
        user, _, ok := s.auth(r)
        if !ok { http.Error(w, "unauthorized", http.StatusUnauthorized); return }
        gameID := r.Header.Get("X-Game-ID")
        env := r.Header.Get("X-Env")
        var in struct{ FunctionID string `json:"function_id"`; Payload any `json:"payload"`; IdempotencyKey string `json:"idempotency_key"` }
        if err := json.NewDecoder(r.Body).Decode(&in); err != nil { http.Error(w, err.Error(), 400); return }
        // validate
        if d := s.descIndex[in.FunctionID]; d != nil {
            if ps := d.Params; ps != nil {
                b, _ := json.Marshal(in.Payload)
                if err := validation.ValidateJSON(ps, b); err != nil { http.Error(w, fmt.Sprintf("payload invalid: %v", err), 400); return }
            }
        }
        // rbac check (scoped)
        basePerm := "function:" + in.FunctionID
        if d := s.descIndex[in.FunctionID]; d != nil {
            if auth := d.Auth; auth != nil {
                if p, ok := auth["permission"].(string); ok && p != "" { basePerm = p }
            }
        }
        scopedOk := false
        if s.rbac != nil {
            scoped := basePerm
            if gameID != "" { scoped = "game:" + gameID + ":" + basePerm }
            if s.rbac.Can(user, scoped) || s.rbac.Can(user, basePerm) || (gameID != "" && s.rbac.Can(user, "game:"+gameID+":*")) || s.rbac.Can(user, "*") {
                scopedOk = true
            } else {
                _, roles, _ := s.auth(r)
                for _, role := range roles {
                    if s.rbac.Can("role:"+role, scoped) || s.rbac.Can("role:"+role, basePerm) { scopedOk = true; break }
                }
            }
        } else { scopedOk = true }
        if !scopedOk { http.Error(w, "forbidden", http.StatusForbidden); return }
        b, _ := json.Marshal(in.Payload)
        if in.IdempotencyKey == "" { in.IdempotencyKey = randHex(16) }
        traceID := randHex(8)
        _ = s.audit.Log("start_job", user, in.FunctionID, map[string]string{"ip": r.RemoteAddr, "trace_id": traceID, "game_id": gameID, "env": env})
        meta := map[string]string{"trace_id": traceID}
        if gameID != "" { meta["game_id"] = gameID }
        if env != "" { meta["env"] = env }
        resp, err := s.invoker.StartJob(r.Context(), &functionv1.InvokeRequest{FunctionId: in.FunctionID, IdempotencyKey: in.IdempotencyKey, Payload: b, Metadata: meta})
        if err != nil { http.Error(w, err.Error(), 500); return }
        atomic.AddInt64(&s.jobsStarted, 1)
        _ = json.NewEncoder(w).Encode(resp)
    })
    s.mux.HandleFunc("/api/stream_job", func(w http.ResponseWriter, r *http.Request) {
        addCORS(w, r)
        jobID := r.URL.Query().Get("id")
        if jobID == "" { http.Error(w, "missing id", 400); return }
        w.Header().Set("Content-Type", "text/event-stream")
        w.Header().Set("Cache-Control", "no-cache")
        w.Header().Set("Connection", "keep-alive")
        flusher, ok := w.(http.Flusher)
        if !ok { http.Error(w, "stream unsupported", 500); return }
        ctx := r.Context()
        stream, err := s.invoker.StreamJob(ctx, &functionv1.JobStreamRequest{JobId: jobID})
        if err != nil { http.Error(w, err.Error(), 500); return }
        enc := json.NewEncoder(w)
        for {
            ev, err := stream.Recv()
            if err != nil { return }
            fmt.Fprintf(w, "event: %s\n", ev.GetType())
            fmt.Fprintf(w, "data: ")
            _ = enc.Encode(ev)
            fmt.Fprint(w, "\n")
            flusher.Flush()
            if ev.GetType() == "done" || ev.GetType() == "error" { return }
        }
    })
    s.mux.HandleFunc("/api/cancel_job", func(w http.ResponseWriter, r *http.Request) {
        addCORS(w, r)
        if r.Method != http.MethodPost { w.WriteHeader(http.StatusMethodNotAllowed); return }
        user, _, ok := s.auth(r)
        if !ok { http.Error(w, "unauthorized", http.StatusUnauthorized); return }
        var in struct{ JobID string `json:"job_id"` }
        if err := json.NewDecoder(r.Body).Decode(&in); err != nil { http.Error(w, err.Error(), 400); return }
        if in.JobID == "" { http.Error(w, "missing job_id", 400); return }
        if s.rbac != nil && !s.rbac.Can(user, "job:cancel") { http.Error(w, "forbidden", http.StatusForbidden); return }
        _ = s.audit.Log("cancel_job", user, in.JobID, map[string]string{"ip": r.RemoteAddr})
        if _, err := s.invoker.CancelJob(r.Context(), &functionv1.CancelJobRequest{JobId: in.JobID}); err != nil {
            http.Error(w, err.Error(), 500); return
        }
        w.WriteHeader(204)
    })

    // Audit list (simple JSONL reader with filters)
    s.mux.HandleFunc("/api/audit", func(w http.ResponseWriter, r *http.Request) {
        addCORS(w, r)
        if _, _, ok := s.auth(r); !ok { http.Error(w, "unauthorized", http.StatusUnauthorized); return }
        if r.Method != http.MethodGet { w.WriteHeader(http.StatusMethodNotAllowed); return }
        gameID := r.URL.Query().Get("game_id")
        env := r.URL.Query().Get("env")
        actor := r.URL.Query().Get("actor")
        kind := r.URL.Query().Get("kind")
        limit := 200
        if qs := r.URL.Query().Get("limit"); qs != "" { fmt.Sscanf(qs, "%d", &limit) }
        type resp struct{ Events []auditchain.Event `json:"events"` }
        var events []auditchain.Event
        // naive scan
        f, err := os.Open("logs/audit.log")
        if err == nil {
            defer f.Close()
            sc := bufio.NewScanner(f)
            for sc.Scan() {
                line := sc.Text()
                if strings.TrimSpace(line) == "" { continue }
                var ev auditchain.Event
                if err := json.Unmarshal([]byte(line), &ev); err != nil { continue }
                // filters
                if gameID != "" && ev.Meta["game_id"] != gameID { continue }
                if env != "" && ev.Meta["env"] != env { continue }
                if actor != "" && ev.Actor != actor { continue }
                if kind != "" && ev.Kind != kind { continue }
                events = append(events, ev)
                if len(events) > limit*2 { // basic cap
                    events = events[len(events)-limit:]
                }
            }
        }
        // tail limit
        if len(events) > limit { events = events[len(events)-limit:] }
        // sort by time ascending (already append order)
        _ = json.NewEncoder(w).Encode(resp{Events: events})
    })
    // Registry summary
    s.mux.HandleFunc("/api/registry", func(w http.ResponseWriter, r *http.Request) {
        addCORS(w, r)
        if _, _, ok := s.auth(r); !ok { http.Error(w, "unauthorized", http.StatusUnauthorized); return }
        if r.Method != http.MethodGet { w.WriteHeader(http.StatusMethodNotAllowed); return }
        type Agent struct{ AgentID, GameID, Env, RpcAddr string; Functions int }
        type Function struct{ GameID, ID string; Agents int }
        var agents []Agent
        var functions []Function
        if s.reg != nil {
            s.reg.Mu().RLock()
            for _, a := range s.reg.AgentsUnsafe() {
                agents = append(agents, Agent{AgentID: a.AgentID, GameID: a.GameID, Env: a.Env, RpcAddr: a.RPCAddr, Functions: len(a.Functions)})
            }
            fnCount := map[string]map[string]int{}
            for _, a := range s.reg.AgentsUnsafe() {
                for fid := range a.Functions {
                    if fnCount[a.GameID] == nil { fnCount[a.GameID] = map[string]int{} }
                    fnCount[a.GameID][fid]++
                }
            }
            for gid, m := range fnCount {
                for fid, c := range m { functions = append(functions, Function{GameID: gid, ID: fid, Agents: c}) }
            }
            s.reg.Mu().RUnlock()
        }
        _ = json.NewEncoder(w).Encode(struct{ Agents []Agent `json:"agents"`; Functions []Function `json:"functions"` }{Agents: agents, Functions: functions})
    })
    // Function instances across agents (targeted routing aid)
    s.mux.HandleFunc("/api/function_instances", func(w http.ResponseWriter, r *http.Request) {
        addCORS(w, r)
        if _, _, ok := s.auth(r); !ok { http.Error(w, "unauthorized", http.StatusUnauthorized); return }
        if r.Method != http.MethodGet { w.WriteHeader(http.StatusMethodNotAllowed); return }
        gameID := r.URL.Query().Get("game_id")
        fid := r.URL.Query().Get("function_id")
        type Inst struct{ AgentID, ServiceID, Addr, Version string }
        var out []Inst
        if s.reg != nil {
            s.reg.Mu().RLock()
            for _, a := range s.reg.AgentsUnsafe() {
                if gameID != "" && a.GameID != gameID { continue }
                if fid != "" { if _, ok := a.Functions[fid]; !ok { continue } }
                // dial agent local control and list
                cc, err := grpc.Dial(a.RPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
                if err != nil { continue }
                cli := localv1.NewLocalControlServiceClient(cc)
                resp, err := cli.ListLocal(r.Context(), &localv1.ListLocalRequest{})
                _ = cc.Close()
                if err != nil || resp == nil { continue }
                for _, lf := range resp.Functions {
                    if fid != "" && lf.Id != fid { continue }
                    for _, inst := range lf.Instances {
                        out = append(out, Inst{AgentID: a.AgentID, ServiceID: inst.ServiceId, Addr: inst.Addr, Version: inst.Version})
                    }
                }
            }
            s.reg.Mu().RUnlock()
        }
        _ = json.NewEncoder(w).Encode(struct{ Instances []Inst `json:"instances"` }{Instances: out})
    })
    // Game whitelist management
    s.mux.HandleFunc("/api/games", func(w http.ResponseWriter, r *http.Request) {
        addCORS(w, r)
        user, _, ok := s.auth(r)
        if !ok { http.Error(w, "unauthorized", http.StatusUnauthorized); return }
        switch r.Method {
        case http.MethodGet:
            _ = json.NewEncoder(w).Encode(struct{ Games []games.Entry `json:"games"` }{Games: s.games.List()})
        case http.MethodPost:
            if s.rbac != nil && !s.rbac.Can(user, "games:manage") { http.Error(w, "forbidden", http.StatusForbidden); return }
            var in games.Entry
            if err := json.NewDecoder(r.Body).Decode(&in); err != nil { http.Error(w, err.Error(), 400); return }
            if in.GameID == "" { http.Error(w, "missing game_id", 400); return }
            s.games.Add(in.GameID, in.Env)
            _ = s.games.Save()
            w.WriteHeader(http.StatusNoContent)
        default:
            w.WriteHeader(http.StatusMethodNotAllowed)
        }
    })

    // Static files: prefer production build at web/dist, fallback to web/static
    staticDir := "web/dist"
    if st, err := os.Stat(staticDir); err != nil || !st.IsDir() {
        staticDir = "web/static"
    }
    fs := http.FileServer(http.Dir(staticDir))
    s.mux.Handle("/", fs)
}

func (s *Server) ListenAndServe(addr string) error {
    log.Printf("http api listening on %s", addr)
    return http.ListenAndServe(addr, s.mux)
}

func randHex(n int) string {
    b := make([]byte, n)
    _, _ = rand.Read(b)
    return hex.EncodeToString(b)
}

func addCORS(w http.ResponseWriter, r *http.Request) {
    // Very simple CORS for dev
    w.Header().Set("Access-Control-Allow-Origin", "*")
    w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Game-ID, X-Env")
    w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
    if r.Method == http.MethodOptions { w.WriteHeader(http.StatusNoContent) }
}

// auth extracts username and roles from Authorization: Bearer <token>
func (s *Server) auth(r *http.Request) (string, []string, bool) {
    authz := r.Header.Get("Authorization")
    if strings.HasPrefix(authz, "Bearer ") && s.jwtMgr != nil {
        tok := strings.TrimPrefix(authz, "Bearer ")
        user, roles, err := s.jwtMgr.Verify(tok)
        if err == nil { return user, roles, true }
    }
    return "", nil, false
}
