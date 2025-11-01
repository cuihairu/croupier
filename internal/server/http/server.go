package httpserver

import (
    "encoding/json"
    "log"
    "log/slog"
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
    "sync"
    users "github.com/cuihairu/croupier/internal/auth/users"
    jwt "github.com/cuihairu/croupier/internal/auth/token"
    localv1 "github.com/cuihairu/croupier/gen/go/croupier/agent/local/v1"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    "github.com/cuihairu/croupier/internal/loadbalancer"
    "github.com/cuihairu/croupier/internal/connpool"
    common "github.com/cuihairu/croupier/internal/cli/common"
    pack "github.com/cuihairu/croupier/internal/pack"
    appr "github.com/cuihairu/croupier/internal/server/approvals"
    "path/filepath"
    "io"
    "archive/tar"
    "compress/gzip"
    "strconv"
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
    invocationsError int64
    jobsStarted int64
    jobsError int64
    rbacDenied int64
    auditErrors int64
    locator interface{ GetJobAddr(string) (string, bool) }
    statsProv interface{
        GetStats() map[string]*loadbalancer.AgentStats
        GetPoolStats() *connpool.PoolStats
    }
    typeReg *pack.TypeRegistry
    packDir string
    mu sync.RWMutex
    fn map[string]*fnMetrics
    // approvals store (two-person rule)
    approvals appr.Store
}

type FunctionInvoker interface {
    Invoke(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.InvokeResponse, error)
    StartJob(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.StartJobResponse, error)
    StreamJob(ctx context.Context, req *functionv1.JobStreamRequest) (functionv1.FunctionService_StreamJobClient, error)
    CancelJob(ctx context.Context, req *functionv1.CancelJobRequest) (*functionv1.StartJobResponse, error)
}

func NewServer(descriptorDir string, invoker FunctionInvoker, audit *auditchain.Writer, policy *rbac.Policy, gamesStore *games.Store, reg *registry.Store, userStore *users.Store, jwtMgr *jwt.Manager, locator interface{ GetJobAddr(string) (string, bool) }, statsProv interface{ GetStats() map[string]*loadbalancer.AgentStats; GetPoolStats() *connpool.PoolStats }) (*Server, error) {
    descs, err := descriptor.LoadAll(descriptorDir)
    if err != nil { return nil, err }
    idx := map[string]*descriptor.Descriptor{}
    for _, d := range descs { idx[d.ID] = d }
    s := &Server{mux: http.NewServeMux(), descs: descs, descIndex: idx, invoker: invoker, audit: audit, rbac: policy, games: gamesStore, reg: reg, userStore: userStore, jwtMgr: jwtMgr, startedAt: time.Now(), locator: locator, statsProv: statsProv, fn: map[string]*fnMetrics{}, typeReg: pack.NewTypeRegistry(), packDir: descriptorDir}
    // approvals store: prefer Postgres via env DATABASE_URL, else in-memory
    dbURL := os.Getenv("DATABASE_URL")
    if dbURL != "" {
        if st, err := appr.NewPGStore(dbURL); err == nil {
            s.approvals = st
            log.Printf("[approvals] using postgres store")
        } else {
            log.Printf("[approvals] postgres disabled: %v; fallback to memory", err)
            s.approvals = appr.NewMemStore()
        }
    } else {
        s.approvals = appr.NewMemStore()
    }
    _ = s.typeReg.LoadFDSFromDir(descriptorDir)
    s.routes()
    return s, nil
}

// metrics toggles and per-function structures
var metricsPerFunction atomic.Bool
var metricsPerGameDenies atomic.Bool

func SetMetricsOptions(perFunction bool, perGameDenies bool) {
    metricsPerFunction.Store(perFunction)
    metricsPerGameDenies.Store(perGameDenies)
}

// Approval struct moved to approvals package

type histogram struct{
    buckets []float64
    counts  []int64
    sum     float64
    count   int64
}
func newHistogram() *histogram {
    b := []float64{0.005,0.01,0.025,0.05,0.1,0.25,0.5,1,2.5,5,10}
    return &histogram{buckets: b, counts: make([]int64, len(b))}
}
func (h *histogram) observe(sec float64) {
    h.count++
    h.sum += sec
    for i, le := range h.buckets {
        if sec <= le { h.counts[i]++; break }
    }
}
func (h *histogram) approxQuantile(q float64) float64 {
    if h.count == 0 { return 0 }
    target := int64(float64(h.count) * q)
    if target <= 0 { target = 1 }
    var cum int64
    for i, c := range h.counts {
        cum += c
        if cum >= target { return h.buckets[i] }
    }
    return h.buckets[len(h.buckets)-1]
}

type fnMetrics struct{
    invocations int64
    errors int64
    rbacDenied int64
    hist *histogram
    deniesByGame map[string]int64
}
func (s *Server) getFn(id string) *fnMetrics {
    s.mu.Lock(); defer s.mu.Unlock()
    m := s.fn[id]
    if m == nil { m = &fnMetrics{hist: newHistogram(), deniesByGame: map[string]int64{}}; s.fn[id] = m }
    return m
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
        out := map[string]any{
            "uptime_sec": int(time.Since(s.startedAt).Seconds()),
            "invocations_total": atomic.LoadInt64(&s.invocations),
            "invocations_error_total": atomic.LoadInt64(&s.invocationsError),
            "jobs_started_total": atomic.LoadInt64(&s.jobsStarted),
            "jobs_error_total": atomic.LoadInt64(&s.jobsError),
            "rbac_denied_total": atomic.LoadInt64(&s.rbacDenied),
            "audit_errors_total": atomic.LoadInt64(&s.auditErrors),
        }
        if s.statsProv != nil {
            out["lb_stats"] = s.statsProv.GetStats()
            out["conn_pool"] = s.statsProv.GetPoolStats()
        }
        out["logs"] = common.GetLogCounters()
        _ = json.NewEncoder(w).Encode(out)
    })

    // Pack import: multipart/form-data with file=pack.tgz
    s.mux.HandleFunc("/api/packs/import", func(w http.ResponseWriter, r *http.Request){
        if r.Method != http.MethodPost { w.WriteHeader(http.StatusMethodNotAllowed); return }
        if err := r.ParseMultipartForm(32 << 20); err != nil { http.Error(w, err.Error(), 400); return }
        f, hdr, err := r.FormFile("file")
        if err != nil { http.Error(w, "missing file", 400); return }
        defer f.Close()
        // Save to temp and extract
        tmpPath := filepath.Join(os.TempDir(), hdr.Filename)
        out, err := os.Create(tmpPath)
        if err != nil { http.Error(w, err.Error(), 500); return }
        if _, err := io.Copy(out, f); err != nil { out.Close(); http.Error(w, err.Error(), 500); return }
        out.Close()
        if err := extractPack(tmpPath, s.packDir); err != nil { http.Error(w, err.Error(), 500); return }
        // Reload descriptors and FDS
        descs, err := descriptor.LoadAll(s.packDir)
        if err == nil {
            idx := map[string]*descriptor.Descriptor{}
            for _, d := range descs { idx[d.ID] = d }
            s.descs = descs; s.descIndex = idx
        }
        _ = s.typeReg.LoadFDSFromDir(s.packDir)
        _ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
    })

    // Prometheus text exposition (basic)
    s.mux.HandleFunc("/metrics.prom", func(w http.ResponseWriter, r *http.Request){
        w.Header().Set("Content-Type", "text/plain; version=0.0.4")
        fmt.Fprintf(w, "# TYPE croupier_invocations_total counter\n")
        fmt.Fprintf(w, "croupier_invocations_total %d\n", atomic.LoadInt64(&s.invocations))
        fmt.Fprintf(w, "# TYPE croupier_invocations_error_total counter\n")
        fmt.Fprintf(w, "croupier_invocations_error_total %d\n", atomic.LoadInt64(&s.invocationsError))
        fmt.Fprintf(w, "# TYPE croupier_jobs_started_total counter\n")
        fmt.Fprintf(w, "croupier_jobs_started_total %d\n", atomic.LoadInt64(&s.jobsStarted))
        fmt.Fprintf(w, "# TYPE croupier_jobs_error_total counter\n")
        fmt.Fprintf(w, "croupier_jobs_error_total %d\n", atomic.LoadInt64(&s.jobsError))
        fmt.Fprintf(w, "# TYPE croupier_rbac_denied_total counter\n")
        fmt.Fprintf(w, "croupier_rbac_denied_total %d\n", atomic.LoadInt64(&s.rbacDenied))
        fmt.Fprintf(w, "# TYPE croupier_audit_errors_total counter\n")
        fmt.Fprintf(w, "croupier_audit_errors_total %d\n", atomic.LoadInt64(&s.auditErrors))
        lc := common.GetLogCounters()
        fmt.Fprintf(w, "# TYPE croupier_logs_total counter\n")
        fmt.Fprintf(w, "croupier_logs_total{level=\"debug\"} %d\n", lc["debug"])
        fmt.Fprintf(w, "croupier_logs_total{level=\"info\"} %d\n", lc["info"])
        fmt.Fprintf(w, "croupier_logs_total{level=\"warn\"} %d\n", lc["warn"])
        fmt.Fprintf(w, "croupier_logs_total{level=\"error\"} %d\n", lc["error"])
    })

    // Ant Design Pro demo stubs (for template pages)
    // GET /api/rule -> return empty rule list; POST /api/rule -> no-op
    s.mux.HandleFunc("/api/rule", func(w http.ResponseWriter, r *http.Request) {
        addCORS(w, r)
        switch r.Method {
        case http.MethodGet:
            type RuleItem struct{
                Key int `json:"key"`
                Name string `json:"name"`
                Desc string `json:"desc"`
                Status int `json:"status"`
                UpdatedAt string `json:"updatedAt"`
                CreatedAt string `json:"createdAt"`
                Progress int `json:"progress"`
            }
            _ = json.NewEncoder(w).Encode(struct{
                Data   []RuleItem `json:"data"`
                Total  int       `json:"total"`
                Success bool     `json:"success"`
            }{Data: []RuleItem{}, Total: 0, Success: true})
        case http.MethodPost:
            _ = json.NewEncoder(w).Encode(struct{ Success bool `json:"success"` }{Success: true})
        default:
            w.WriteHeader(http.StatusMethodNotAllowed)
        }
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
            HashKey string `json:"hash_key"`
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
        // audit with masked snapshot
        masked := s.maskSnapshot(in.FunctionID, in.Payload)
        if err := s.audit.Log("invoke", user, in.FunctionID, map[string]string{"ip": r.RemoteAddr, "trace_id": traceID, "game_id": gameID, "env": env, "payload_snapshot": masked}); err != nil { atomic.AddInt64(&s.auditErrors,1) }
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
        if rv, ok := meta["route"]; ok && rv != "lb" && rv != "broadcast" && rv != "targeted" && rv != "hash" {
            http.Error(w, "invalid route", 400); return
        }
        if in.HashKey != "" { meta["hash_key"] = in.HashKey }
        if meta["route"] == "hash" && meta["hash_key"] == "" { http.Error(w, "hash_key required for hash route", 400); return }
        if in.TargetServiceID != "" { meta["target_service_id"] = in.TargetServiceID }
        if meta["route"] == "targeted" && meta["target_service_id"] == "" { http.Error(w, "target_service_id required for targeted route", 400); return }
        // transport-aware encode: JSON -> pb-bin if descriptor.transport.proto.request_fqn is set
        if d := s.descIndex[in.FunctionID]; d != nil && d.Transport != nil {
            if tp, ok := d.Transport["proto"].(map[string]any); ok {
                if fqn, ok2 := tp["request_fqn"].(string); ok2 && fqn != "" && s.typeReg != nil {
                    if pb, err2 := s.typeReg.JSONToProtoBin(fqn, b); err2 == nil { b = pb } else { slog.Warn("encode proto failed; fallback json", "function_id", in.FunctionID, "error", err2.Error()) }
                }
            }
        }
        // if two_person_rule enabled, store approval and return 202 pending
        if d := s.descIndex[in.FunctionID]; d != nil {
            if auth := d.Auth; auth != nil {
                if tpr, ok := auth["two_person_rule"].(bool); ok && tpr {
                    id := randHex(12)
                    _ = s.approvals.Create(&appr.Approval{ID: id, CreatedAt: time.Now(), Actor: user, FunctionID: in.FunctionID, Payload: b, IdempotencyKey: in.IdempotencyKey, Route: meta["route"], TargetServiceID: meta["target_service_id"], HashKey: meta["hash_key"], GameID: gameID, Env: env, State: "pending", Mode: "invoke"})
                    w.WriteHeader(http.StatusAccepted)
                    _ = json.NewEncoder(w).Encode(map[string]any{"approval_id": id, "state": "pending"})
                    return
                }
            }
        }
        resp, err := s.invoker.Invoke(r.Context(), &functionv1.InvokeRequest{FunctionId: in.FunctionID, IdempotencyKey: in.IdempotencyKey, Payload: b, Metadata: meta})
        if err != nil {
            atomic.AddInt64(&s.invocationsError,1)
            slog.Error("invoke failed", "user", user, "function_id", in.FunctionID, "trace_id", traceID, "game_id", gameID, "env", env, "route", meta["route"], "error", err.Error())
            http.Error(w, err.Error(), 500); return
        }
        slog.Info("invoke", "user", user, "function_id", in.FunctionID, "trace_id", traceID, "game_id", gameID, "env", env, "route", meta["route"]) 
        atomic.AddInt64(&s.invocations, 1)
        // decode pb-bin -> JSON if response_fqn present
        out := resp.GetPayload()
        if d := s.descIndex[in.FunctionID]; d != nil && d.Transport != nil {
            if tp, ok := d.Transport["proto"].(map[string]any); ok {
                if fqn, ok2 := tp["response_fqn"].(string); ok2 && fqn != "" && s.typeReg != nil {
                    if j, err2 := s.typeReg.ProtoBinToJSON(fqn, out); err2 == nil { out = j }
                }
            }
        }
        if len(out) == 0 { w.WriteHeader(204); return }
        w.Header().Set("Content-Type", "application/json")
        _, _ = w.Write(out)
    })
    s.mux.HandleFunc("/api/start_job", func(w http.ResponseWriter, r *http.Request) {
        addCORS(w, r)
        if r.Method != http.MethodPost { w.WriteHeader(http.StatusMethodNotAllowed); return }
        user, _, ok := s.auth(r)
        if !ok { http.Error(w, "unauthorized", http.StatusUnauthorized); return }
        gameID := r.Header.Get("X-Game-ID")
        env := r.Header.Get("X-Env")
        var in struct{
            FunctionID string `json:"function_id"`
            Payload any `json:"payload"`
            IdempotencyKey string `json:"idempotency_key"`
            Route string `json:"route"`
            TargetServiceID string `json:"target_service_id"`
            HashKey string `json:"hash_key"`
        }
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
        if !scopedOk { atomic.AddInt64(&s.rbacDenied,1); http.Error(w, "forbidden", http.StatusForbidden); return }
        b, _ := json.Marshal(in.Payload)
        if in.IdempotencyKey == "" { in.IdempotencyKey = randHex(16) }
        traceID := randHex(8)
        if err := s.audit.Log("start_job", user, in.FunctionID, map[string]string{"ip": r.RemoteAddr, "trace_id": traceID, "game_id": gameID, "env": env}); err != nil { atomic.AddInt64(&s.auditErrors,1) }
        meta := map[string]string{"trace_id": traceID}
        if gameID != "" { meta["game_id"] = gameID }
        if env != "" { meta["env"] = env }
        // route selection: request override > descriptor.semantics.route
        if in.Route != "" { meta["route"] = in.Route } else if d := s.descIndex[in.FunctionID]; d != nil {
            if sem := d.Semantics; sem != nil {
                if rv, ok := sem["route"].(string); ok && rv != "" { meta["route"] = rv }
            }
        }
        // validate route
        if rv, ok := meta["route"]; ok && rv != "lb" && rv != "broadcast" && rv != "targeted" && rv != "hash" {
            http.Error(w, "invalid route", 400); return
        }
        if in.HashKey != "" { meta["hash_key"] = in.HashKey }
        if meta["route"] == "hash" && meta["hash_key"] == "" { http.Error(w, "hash_key required for hash route", 400); return }
        if in.TargetServiceID != "" { meta["target_service_id"] = in.TargetServiceID }
        if meta["route"] == "targeted" && meta["target_service_id"] == "" { http.Error(w, "target_service_id required for targeted route", 400); return }
        // transport-aware encode
        if d := s.descIndex[in.FunctionID]; d != nil && d.Transport != nil {
            if tp, ok := d.Transport["proto"].(map[string]any); ok {
                if fqn, ok2 := tp["request_fqn"].(string); ok2 && fqn != "" && s.typeReg != nil {
                    if pb, err2 := s.typeReg.JSONToProtoBin(fqn, b); err2 == nil { b = pb } else { slog.Warn("encode proto failed; fallback json", "function_id", in.FunctionID, "error", err2.Error()) }
                }
            }
        }
        // two_person_rule for start_job
        if d := s.descIndex[in.FunctionID]; d != nil {
            if auth := d.Auth; auth != nil {
                if tpr, ok := auth["two_person_rule"].(bool); ok && tpr {
                    id := randHex(12)
                    _ = s.approvals.Create(&appr.Approval{ID: id, CreatedAt: time.Now(), Actor: user, FunctionID: in.FunctionID, Payload: b, IdempotencyKey: in.IdempotencyKey, Route: meta["route"], TargetServiceID: meta["target_service_id"], HashKey: meta["hash_key"], GameID: gameID, Env: env, State: "pending", Mode: "start_job"})
                    w.WriteHeader(http.StatusAccepted)
                    _ = json.NewEncoder(w).Encode(map[string]any{"approval_id": id, "state": "pending"})
                    return
                }
            }
        }
        resp, err := s.invoker.StartJob(r.Context(), &functionv1.InvokeRequest{FunctionId: in.FunctionID, IdempotencyKey: in.IdempotencyKey, Payload: b, Metadata: meta})
        if err != nil {
            atomic.AddInt64(&s.jobsError,1)
            slog.Error("start_job failed", "user", user, "function_id", in.FunctionID, "trace_id", traceID, "game_id", gameID, "env", env, "route", in.Route, "error", err.Error())
            http.Error(w, err.Error(), 500); return
        }
        slog.Info("start_job", "user", user, "function_id", in.FunctionID, "trace_id", traceID, "game_id", gameID, "env", env, "route", in.Route)
        atomic.AddInt64(&s.jobsStarted, 1)
        _ = json.NewEncoder(w).Encode(resp)
    })
    // approvals endpoints
    s.mux.HandleFunc("/api/approvals", func(w http.ResponseWriter, r *http.Request) {
        addCORS(w, r)
        user, _, ok := s.auth(r)
        if !ok { http.Error(w, "unauthorized", http.StatusUnauthorized); return }
        if s.rbac != nil && !(s.rbac.Can(user, "approvals:read") || s.rbac.Can(user, "*")) { http.Error(w, "forbidden", http.StatusForbidden); return }
        if r.Method != http.MethodGet { w.WriteHeader(http.StatusMethodNotAllowed); return }
        // filters + pagination
        f := appr.Filter{
            State: r.URL.Query().Get("state"),
            FunctionID: r.URL.Query().Get("function_id"),
            GameID: r.URL.Query().Get("game_id"),
            Env: r.URL.Query().Get("env"),
            Actor: r.URL.Query().Get("actor"),
            Mode: r.URL.Query().Get("mode"),
        }
        page := 1; size := 20; sort := r.URL.Query().Get("sort")
        if v := r.URL.Query().Get("page"); v != "" { if n, err := strconv.Atoi(v); err == nil && n > 0 { page = n } }
        if v := r.URL.Query().Get("size"); v != "" { if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 { size = n } }
        items, total, err := s.approvals.List(f, appr.Page{Page: page, Size: size, Sort: sort})
        if err != nil { http.Error(w, err.Error(), 500); return }
        type view struct{ ID, CreatedAt, Actor, FunctionID, IdempotencyKey, Route, TargetServiceID, HashKey, GameID, Env, State, Mode string }
        out := make([]view, 0, len(items))
        for _, a := range items {
            out = append(out, view{
                ID: a.ID,
                CreatedAt: a.CreatedAt.Format(time.RFC3339),
                Actor: a.Actor,
                FunctionID: a.FunctionID,
                IdempotencyKey: a.IdempotencyKey,
                Route: a.Route,
                TargetServiceID: a.TargetServiceID,
                HashKey: a.HashKey,
                GameID: a.GameID,
                Env: a.Env,
                State: a.State,
                Mode: a.Mode,
            })
        }
        _ = json.NewEncoder(w).Encode(struct{
            Approvals []view `json:"approvals"`
            Total int `json:"total"`
            Page int `json:"page"`
            Size int `json:"size"`
        }{Approvals: out, Total: total, Page: page, Size: size})
    })
    // approval detail
    s.mux.HandleFunc("/api/approvals/get", func(w http.ResponseWriter, r *http.Request) {
        addCORS(w, r)
        user, _, ok := s.auth(r)
        if !ok { http.Error(w, "unauthorized", http.StatusUnauthorized); return }
        if s.rbac != nil && !(s.rbac.Can(user, "approvals:read") || s.rbac.Can(user, "*")) { http.Error(w, "forbidden", http.StatusForbidden); return }
        if r.Method != http.MethodGet { w.WriteHeader(http.StatusMethodNotAllowed); return }
        id := r.URL.Query().Get("id")
        if id == "" { http.Error(w, "missing id", 400); return }
        a, err := s.approvals.Get(id)
        if err != nil { http.Error(w, "not found", 404); return }
        // try build masked preview
        var preview string
        if d := s.descIndex[a.FunctionID]; d != nil && d.Transport != nil {
            if tp, ok := d.Transport["proto"].(map[string]any); ok {
                if fqn, ok2 := tp["request_fqn"].(string); ok2 && fqn != "" && s.typeReg != nil {
                    if j, err2 := s.typeReg.ProtoBinToJSON(fqn, a.Payload); err2 == nil {
                        preview = s.maskSnapshot(a.FunctionID, j)
                    }
                }
            }
        }
        if preview == "" {
            preview = s.maskSnapshot(a.FunctionID, a.Payload)
        }
        _ = json.NewEncoder(w).Encode(map[string]any{
            "id": a.ID,
            "created_at": a.CreatedAt.Format(time.RFC3339),
            "actor": a.Actor,
            "function_id": a.FunctionID,
            "idempotency_key": a.IdempotencyKey,
            "route": a.Route,
            "target_service_id": a.TargetServiceID,
            "hash_key": a.HashKey,
            "game_id": a.GameID,
            "env": a.Env,
            "state": a.State,
            "mode": a.Mode,
            "reason": a.Reason,
            "payload_preview": preview,
        })
    })
    s.mux.HandleFunc("/api/approvals/approve", func(w http.ResponseWriter, r *http.Request) {
        addCORS(w, r)
        user, _, ok := s.auth(r)
        if !ok { http.Error(w, "unauthorized", http.StatusUnauthorized); return }
        if r.Method != http.MethodPost { w.WriteHeader(http.StatusMethodNotAllowed); return }
        if s.rbac != nil && !(s.rbac.Can(user, "approvals:approve") || s.rbac.Can(user, "*")) { http.Error(w, "forbidden", http.StatusForbidden); return }
        var in struct{ ID string `json:"id"` }
        if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.ID == "" { http.Error(w, "missing id", 400); return }
        a, err := s.approvals.Approve(in.ID)
        if err != nil { http.Error(w, err.Error(), 409); return }
        // build meta from stored approval
        meta := map[string]string{}
        if a.Route != "" { meta["route"] = a.Route }
        if a.HashKey != "" { meta["hash_key"] = a.HashKey }
        if a.TargetServiceID != "" { meta["target_service_id"] = a.TargetServiceID }
        if a.GameID != "" { meta["game_id"] = a.GameID }
        if a.Env != "" { meta["env"] = a.Env }
        // audit
        if err := s.audit.Log("approval_approve", user, a.FunctionID, map[string]string{"approval_id": a.ID}); err != nil { atomic.AddInt64(&s.auditErrors,1) }
        // execute
        switch a.Mode {
        case "invoke":
            resp, err := s.invoker.Invoke(r.Context(), &functionv1.InvokeRequest{FunctionId: a.FunctionID, IdempotencyKey: a.IdempotencyKey, Payload: a.Payload, Metadata: meta})
            if err != nil { http.Error(w, err.Error(), 500); return }
            // decode response if needed
            out := resp.GetPayload()
            if d := s.descIndex[a.FunctionID]; d != nil && d.Transport != nil {
                if tp, ok := d.Transport["proto"].(map[string]any); ok {
                    if fqn, ok2 := tp["response_fqn"].(string); ok2 && fqn != "" && s.typeReg != nil {
                        if j, err2 := s.typeReg.ProtoBinToJSON(fqn, out); err2 == nil { out = j }
                    }
                }
            }
            if len(out) == 0 { w.WriteHeader(204); return }
            w.Header().Set("Content-Type", "application/json")
            _, _ = w.Write(out)
        case "start_job":
            resp, err := s.invoker.StartJob(r.Context(), &functionv1.InvokeRequest{FunctionId: a.FunctionID, IdempotencyKey: a.IdempotencyKey, Payload: a.Payload, Metadata: meta})
            if err != nil { http.Error(w, err.Error(), 500); return }
            _ = json.NewEncoder(w).Encode(resp)
        default:
            http.Error(w, "unknown mode", 400)
        }
    })
    s.mux.HandleFunc("/api/approvals/reject", func(w http.ResponseWriter, r *http.Request) {
        addCORS(w, r)
        user, _, ok := s.auth(r)
        if !ok { http.Error(w, "unauthorized", http.StatusUnauthorized); return }
        if r.Method != http.MethodPost { w.WriteHeader(http.StatusMethodNotAllowed); return }
        if s.rbac != nil && !(s.rbac.Can(user, "approvals:reject") || s.rbac.Can(user, "*")) { http.Error(w, "forbidden", http.StatusForbidden); return }
        var in struct{ ID, Reason string `json:"id" json:"reason"` }
        // decode flexible
        dec := json.NewDecoder(r.Body)
        var tmp map[string]any
        if err := dec.Decode(&tmp); err != nil { http.Error(w, "bad request", 400); return }
        id, _ := tmp["id"].(string)
        reason, _ := tmp["reason"].(string)
        if id == "" { http.Error(w, "missing id", 400); return }
        a, err := s.approvals.Reject(id, reason)
        if err != nil { http.Error(w, err.Error(), 409); return }
        // audit
        if err := s.audit.Log("approval_reject", user, a.FunctionID, map[string]string{"approval_id": a.ID, "reason": reason}); err != nil { atomic.AddInt64(&s.auditErrors,1) }
        w.WriteHeader(http.StatusNoContent)
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

    // Query job result/status (best-effort; in edge-forward mode may be unavailable)
    s.mux.HandleFunc("/api/job_result", func(w http.ResponseWriter, r *http.Request) {
        addCORS(w, r)
        if _, _, ok := s.auth(r); !ok { http.Error(w, "unauthorized", http.StatusUnauthorized); return }
        if r.Method != http.MethodGet { w.WriteHeader(http.StatusMethodNotAllowed); return }
        jobID := r.URL.Query().Get("id")
        if jobID == "" { http.Error(w, "missing id", 400); return }
        if s.locator != nil {
            addr, ok := s.locator.GetJobAddr(jobID)
            if !ok { http.Error(w, "unknown job", 404); return }
            // dial agent local control to query job result
            cc, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
            if err != nil { http.Error(w, err.Error(), 502); return }
            defer cc.Close()
            cli := localv1.NewLocalControlServiceClient(cc)
            resp, err := cli.GetJobResult(r.Context(), &localv1.GetJobResultRequest{JobId: jobID})
            if err != nil { http.Error(w, err.Error(), 502); return }
            _ = json.NewEncoder(w).Encode(resp)
            return
        }
        // Edge-forward mode: try invoker extension if available
        type jobFetcher interface{ JobResult(ctx context.Context, jobID string) (string, []byte, string, error) }
        if jf, ok := s.invoker.(jobFetcher); ok {
            st, payload, errMsg, err := jf.JobResult(r.Context(), jobID)
            if err != nil { http.Error(w, err.Error(), 502); return }
            _ = json.NewEncoder(w).Encode(struct{ State string `json:"state"`; Payload []byte `json:"payload,omitempty"`; Error string `json:"error,omitempty"` }{State: st, Payload: payload, Error: errMsg})
            return
        }
        http.Error(w, "job_result not available", http.StatusNotImplemented)
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

// extractPack extracts a tar.gz pack into dest directory; keeps descriptors and fds
func extractPack(archive, dest string) error {
    f, err := os.Open(archive)
    if err != nil { return err }
    defer f.Close()
    gz, err := gzip.NewReader(f)
    if err != nil { return err }
    defer gz.Close()
    tr := tar.NewReader(gz)
    for {
        hdr, err := tr.Next()
        if err == io.EOF { break }
        if err != nil { return err }
        // Only extract descriptors/*.json, ui/*.json and any *.pb
        if strings.HasPrefix(hdr.Name, "descriptors/") || strings.HasPrefix(hdr.Name, "ui/") || strings.HasSuffix(hdr.Name, ".pb") {
            target := filepath.Join(dest, filepath.FromSlash(hdr.Name))
            if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil { return err }
            w, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
            if err != nil { return err }
            if _, err := io.Copy(w, tr); err != nil { w.Close(); return err }
            w.Close()
        }
    }
    return nil
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

// maskSnapshot masks sensitive fields in payload based on descriptor UI hints.
func (s *Server) maskSnapshot(fid string, payload any) string {
    // Build sensitive keys set
    sensitive := map[string]struct{}{
        "password":{}, "pass":{}, "secret":{}, "token":{}, "api_key":{}, "apikey":{}, "authorization":{}, "auth":{}, "key":{},
    }
    if d := s.descIndex[fid]; d != nil && d.UI != nil {
        // support ui.sensitive: ["field1", "nested.field2"]
        if raw, ok := d.UI["sensitive"]; ok {
            if arr, ok := raw.([]any); ok {
                for _, v := range arr {
                    if s1, ok := v.(string); ok && s1 != "" { sensitive[strings.ToLower(s1)] = struct{}{} }
                }
            }
        }
    }
    // Work on a generic map clone
    var m any = payload
    // If payload is raw JSON bytes (from proxy), try decode
    if b, ok := payload.([]byte); ok {
        var tmp any
        if err := json.Unmarshal(b, &tmp); err == nil { m = tmp }
    }
    masked := maskAny(m, sensitive)
    out, err := json.Marshal(masked)
    if err != nil { return "{}" }
    return string(out)
}

// maskAny recursively masks maps/slices for keys present in sensitive set.
func maskAny(v any, sensitive map[string]struct{}) any {
    switch t := v.(type) {
    case map[string]any:
        mm := map[string]any{}
        for k, val := range t {
            if _, ok := sensitive[strings.ToLower(k)]; ok {
                mm[k] = "***"
                continue
            }
            mm[k] = maskAny(val, sensitive)
        }
        return mm
    case []any:
        out := make([]any, len(t))
        for i, e := range t { out[i] = maskAny(e, sensitive) }
        return out
    default:
        return t
    }
}
