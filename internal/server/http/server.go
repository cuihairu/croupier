package httpserver

import (
    "encoding/json"
    "log"
    "net/http"

    "github.com/your-org/croupier/internal/function/descriptor"
    functionv1 "github.com/your-org/croupier/gen/go/croupier/function/v1"
    "context"
    "crypto/rand"
    "encoding/hex"
    "fmt"
    "github.com/your-org/croupier/internal/validation"
    auditchain "github.com/your-org/croupier/internal/audit/chain"
    "github.com/your-org/croupier/internal/auth/rbac"
    "os"
    "github.com/your-org/croupier/internal/server/games"
)

type Server struct {
    mux   *http.ServeMux
    descs []*descriptor.Descriptor
    descIndex map[string]*descriptor.Descriptor
    invoker FunctionInvoker
    audit *auditchain.Writer
    rbac  *rbac.Policy
    games *games.Store
}

type FunctionInvoker interface {
    Invoke(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.InvokeResponse, error)
    StartJob(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.StartJobResponse, error)
    StreamJob(ctx context.Context, req *functionv1.JobStreamRequest) (functionv1.FunctionService_StreamJobClient, error)
    CancelJob(ctx context.Context, req *functionv1.CancelJobRequest) (*functionv1.StartJobResponse, error)
}

func NewServer(descriptorDir string, invoker FunctionInvoker, audit *auditchain.Writer, policy *rbac.Policy, gamesStore *games.Store) (*Server, error) {
    descs, err := descriptor.LoadAll(descriptorDir)
    if err != nil { return nil, err }
    idx := map[string]*descriptor.Descriptor{}
    for _, d := range descs { idx[d.ID] = d }
    s := &Server{mux: http.NewServeMux(), descs: descs, descIndex: idx, invoker: invoker, audit: audit, rbac: policy, games: gamesStore}
    s.routes()
    return s, nil
}

func (s *Server) routes() {
    s.mux.HandleFunc("/api/descriptors", func(w http.ResponseWriter, r *http.Request) {
        addCORS(w, r)
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(s.descs)
    })
    s.mux.HandleFunc("/api/invoke", func(w http.ResponseWriter, r *http.Request) {
        addCORS(w, r)
        if r.Method != http.MethodPost { w.WriteHeader(http.StatusMethodNotAllowed); return }
        user := r.Header.Get("X-User"); if user == "" { user = "user:dev" }
        gameID := r.Header.Get("X-Game-ID")
        env := r.Header.Get("X-Env")
        var in struct{
            FunctionID string `json:"function_id"`
            Payload    any    `json:"payload"`
            IdempotencyKey string `json:"idempotency_key"`
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
            if s.rbac.Can(user, scoped) || s.rbac.Can(user, basePerm) || (gameID != "" && s.rbac.Can(user, "game:"+gameID+":*")) || s.rbac.Can(user, "*") {
                scopedOk = true
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
        resp, err := s.invoker.Invoke(r.Context(), &functionv1.InvokeRequest{FunctionId: in.FunctionID, IdempotencyKey: in.IdempotencyKey, Payload: b, Metadata: meta})
        if err != nil { http.Error(w, err.Error(), 500); return }
        w.Header().Set("Content-Type", "application/json")
        if len(resp.GetPayload()) == 0 {
            w.WriteHeader(204); return
        }
        _, _ = w.Write(resp.GetPayload())
    })
    s.mux.HandleFunc("/api/start_job", func(w http.ResponseWriter, r *http.Request) {
        addCORS(w, r)
        if r.Method != http.MethodPost { w.WriteHeader(http.StatusMethodNotAllowed); return }
        user := r.Header.Get("X-User"); if user == "" { user = "user:dev" }
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
        user := r.Header.Get("X-User"); if user == "" { user = "user:dev" }
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
    // Game whitelist management
    s.mux.HandleFunc("/api/games", func(w http.ResponseWriter, r *http.Request) {
        addCORS(w, r)
        user := r.Header.Get("X-User"); if user == "" { user = "user:dev" }
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
    w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-User, X-Game-ID, X-Env")
    w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
    if r.Method == http.MethodOptions { w.WriteHeader(http.StatusNoContent) }
}
