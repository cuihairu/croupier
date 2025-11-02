package httpserver

import (
    "encoding/json"
    "log"
    "log/slog"
    "net/http"

    "github.com/cuihairu/croupier/internal/function/descriptor"
    functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
    "context"
    "crypto/rand"
    "encoding/hex"
    "crypto/sha256"
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
    usersgorm "github.com/cuihairu/croupier/internal/infra/persistence/gorm/users"
    jwt "github.com/cuihairu/croupier/internal/auth/token"
    "github.com/cuihairu/croupier/internal/auth/otp"
    localv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/local/v1"
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
    "math"
    "gorm.io/gorm"
    gpostgres "gorm.io/driver/postgres"
    gmysql "gorm.io/driver/mysql"
    gsqlite "gorm.io/driver/sqlite"
    
    "net/url"
    obj "github.com/cuihairu/croupier/internal/objstore"
)

type Server struct {
    mux   *http.ServeMux
    descs []*descriptor.Descriptor
    descIndex map[string]*descriptor.Descriptor
    invoker FunctionInvoker
    audit *auditchain.Writer
    rbac  *rbac.Policy
    games *games.Repo
    reg   *registry.Store
    userRepo *usersgorm.Repo
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
    // per-function rate limiters and concurrency semaphores
    rl map[string]*rateLimiter
    conc map[string]chan struct{}
    // assignments: game_id|env -> []function_ids
    assignments map[string][]string
    assignmentsPath string
    // packs export auth requirement (optional via env)
    packsExportRequireAuth bool
    // games metadata via GORM (postgres preferred, else sqlite)
    gdb *gorm.DB
    
    obj obj.Store
    objConf obj.Config
}

type FunctionInvoker interface {
    Invoke(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.InvokeResponse, error)
    StartJob(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.StartJobResponse, error)
    StreamJob(ctx context.Context, req *functionv1.JobStreamRequest) (functionv1.FunctionService_StreamJobClient, error)
    CancelJob(ctx context.Context, req *functionv1.CancelJobRequest) (*functionv1.StartJobResponse, error)
}

func NewServer(descriptorDir string, invoker FunctionInvoker, audit *auditchain.Writer, policy *rbac.Policy, reg *registry.Store, jwtMgr *jwt.Manager, locator interface{ GetJobAddr(string) (string, bool) }, statsProv interface{ GetStats() map[string]*loadbalancer.AgentStats; GetPoolStats() *connpool.PoolStats }) (*Server, error) {
    descs, err := descriptor.LoadAll(descriptorDir)
    if err != nil { return nil, err }
    idx := map[string]*descriptor.Descriptor{}
    for _, d := range descs { idx[d.ID] = d }
    s := &Server{mux: http.NewServeMux(), descs: descs, descIndex: idx, invoker: invoker, audit: audit, rbac: policy, reg: reg, jwtMgr: jwtMgr, startedAt: time.Now(), locator: locator, statsProv: statsProv, fn: map[string]*fnMetrics{}, typeReg: pack.NewTypeRegistry(), packDir: descriptorDir, rl: map[string]*rateLimiter{}, conc: map[string]chan struct{}{}, assignments: map[string][]string{}, assignmentsPath: filepath.Join(descriptorDir, "assignments.json")}
    // approvals store: prefer Postgres via env DATABASE_URL, else in-memory
    dbURL := os.Getenv("DATABASE_URL")
    if dbURL != "" {
        // Prefer explicit scheme
        if strings.HasPrefix(dbURL, "postgres://") || strings.HasPrefix(dbURL, "postgresql://") || strings.HasPrefix(dbURL, "pgx://") {
            if st, err := appr.NewPGStore(dbURL); err == nil { s.approvals = st; log.Printf("[approvals] using postgres store") } else { log.Printf("[approvals] postgres disabled: %v; fallback to memory", err); s.approvals = appr.NewMemStore() }
        } else if strings.HasPrefix(dbURL, "sqlite://") || strings.HasPrefix(dbURL, "file:") || strings.HasSuffix(dbURL, ".db") || dbURL == ":memory:" {
            if st, err := appr.NewSQLiteStore(dbURL); err == nil { s.approvals = st; log.Printf("[approvals] using sqlite store") } else { log.Printf("[approvals] sqlite disabled: %v; fallback to memory", err); s.approvals = appr.NewMemStore() }
        } else {
            // unknown scheme
            log.Printf("[approvals] unknown DATABASE_URL scheme; using memory")
            s.approvals = appr.NewMemStore()
        }
    } else { s.approvals = appr.NewMemStore() }
    _ = s.typeReg.LoadFDSFromDir(descriptorDir)
    // load assignments (best-effort)
    if b, err := os.ReadFile(s.assignmentsPath); err == nil {
        _ = json.Unmarshal(b, &s.assignments)
    }
    // optional toggles via env
    // METRICS_PER_FUNCTION=true|false, METRICS_PER_GAME_DENIES=true|false
    if v := os.Getenv("METRICS_PER_FUNCTION"); v != "" {
        SetMetricsOptions(strings.EqualFold(v, "true") || v == "1" || strings.EqualFold(v, "yes"), metricsPerGameDenies.Load())
    }
    if v := os.Getenv("METRICS_PER_GAME_DENIES"); v != "" {
        SetMetricsOptions(metricsPerFunction.Load(), strings.EqualFold(v, "true") || v == "1" || strings.EqualFold(v, "yes"))
    }
    if v := os.Getenv("PACKS_EXPORT_REQUIRE_AUTH"); v != "" {
        s.packsExportRequireAuth = strings.EqualFold(v, "true") || v == "1" || strings.EqualFold(v, "yes")
    }
    // init GORM (prefer postgres)
    // DB initialization by config/env: DB_DRIVER=[postgres|mysql|sqlite|auto], DATABASE_URL as DSN
    sel := strings.ToLower(strings.TrimSpace(os.Getenv("DB_DRIVER")))
    dsn := os.Getenv("DATABASE_URL")
    openAuto := func() {
        if dsn != "" {
            if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") || strings.HasPrefix(dsn, "pgx://") {
                if db, err := gorm.Open(gpostgres.Open(dsn), &gorm.Config{}); err == nil { s.gdb = db; return }
            }
            if strings.HasPrefix(dsn, "mysql://") || strings.Contains(dsn, "@tcp(") {
                norm := normalizeMySQLDSN(dsn)
                if db, err := gorm.Open(gmysql.Open(norm), &gorm.Config{}); err == nil { s.gdb = db; return }
            }
        }
    }
    switch sel {
    case "postgres":
        if dsn != "" { if db, err := gorm.Open(gpostgres.Open(dsn), &gorm.Config{}); err == nil { s.gdb = db } }
    case "mysql":
        if dsn != "" { if db, err := gorm.Open(gmysql.Open(normalizeMySQLDSN(dsn)), &gorm.Config{}); err == nil { s.gdb = db } }
    case "sqlite":
        // allow explicit sqlite DSN; if empty, fallback below
        if dsn != "" { if db, err := gorm.Open(gsqlite.Open(dsn), &gorm.Config{}); err == nil { s.gdb = db } }
    case "", "auto":
        openAuto()
    default:
        openAuto()
    }
    if s.gdb == nil { // fallback sqlite local
        _ = os.MkdirAll("data", 0o755)
        fp := filepath.ToSlash(filepath.Join("data", "croupier.db"))
        dsn := "file:" + fp
        if db, err := gorm.Open(gsqlite.Open(dsn), &gorm.Config{}); err == nil { s.gdb = db }
    }
    if s.gdb != nil {
        // auto-migrate and prepare repos
        _ = games.AutoMigrate(s.gdb)
        s.games = games.NewRepo(s.gdb)
        _ = usersgorm.AutoMigrate(s.gdb)
        s.userRepo = usersgorm.New(s.gdb)
    }
    // init object storage (from env bridge STORAGE_*)
    s.objConf = obj.FromEnv()
    if s.objConf.Driver == "file" && s.objConf.BaseDir == "" {
        // default uploads dir for local
        s.objConf.BaseDir = filepath.Join("data", "uploads")
    }
    if s.objConf.Driver != "" {
        if err := obj.Validate(s.objConf); err == nil {
            switch strings.ToLower(s.objConf.Driver) {
            case "s3":
                if st, err := obj.OpenS3(context.Background(), s.objConf); err == nil { s.obj = st }
            case "file":
                if st, err := obj.OpenFile(context.Background(), s.objConf); err == nil { s.obj = st }
            case "oss":
                if st, err := obj.OpenOSS(context.Background(), s.objConf); err == nil { s.obj = st }
            case "cos":
                if st, err := obj.OpenCOS(context.Background(), s.objConf); err == nil { s.obj = st }
            }
        }
    }
    s.routes()
    return s, nil
}

// normalizeMySQLDSN converts mysql:// URIs to go-sql-driver DSN when needed and ensures parseTime.
func normalizeMySQLDSN(dsn string) string {
    // already native DSN
    if !strings.HasPrefix(dsn, "mysql://") {
        if !strings.Contains(strings.ToLower(dsn), "parsetime=") { // append parseTime if missing
            sep := "?"; if strings.Contains(dsn, "?") { sep = "&" }
            dsn = dsn + sep + "parseTime=true"
        }
        return dsn
    }
    // Convert mysql://user:pass@host:port/db?params -> user:pass@tcp(host:port)/db?params
    u, err := url.Parse(dsn)
    if err != nil { return dsn }
    user := ""; pass := ""
    if u.User != nil { user = u.User.Username(); pass, _ = u.User.Password() }
    host := u.Host
    db := strings.TrimPrefix(u.Path, "/")
    q := u.RawQuery
    if !strings.Contains(strings.ToLower(q), "parsetime=") { if q == "" { q = "parseTime=true" } else { q = q + "&parseTime=true" } }
    auth := user
    if pass != "" { auth = auth + ":" + pass }
    if auth != "" { auth = auth + "@" }
    return fmt.Sprintf("%stcp(%s)/%s?%s", auth, host, db, q)
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

// computePackETag returns a stable sha256 hex digest for current pack content under packDir.
// It walks manifest.json, descriptors/*.json, ui/*.json, web-plugin/*.js and any *.pb at root,
// hashing rel path and file bytes to build a content-addressed ETag.
func computePackETag(packDir string) string {
    h := sha256.New()
    writeFile := func(rel string) {
        p := filepath.Join(packDir, rel)
        b, err := os.ReadFile(p)
        if err != nil { return }
        _, _ = h.Write([]byte(rel))
        _, _ = h.Write([]byte{0})
        _, _ = h.Write(b)
        _, _ = h.Write([]byte{0})
    }
    // manifest
    writeFile("manifest.json")
    // descriptors and ui
    _ = filepath.Walk(filepath.Join(packDir, "descriptors"), func(path string, info os.FileInfo, err error) error {
        if err != nil || info == nil || info.IsDir() { return nil }
        if filepath.Ext(path) != ".json" { return nil }
        rel, _ := filepath.Rel(packDir, path)
        writeFile(rel)
        return nil
    })
    _ = filepath.Walk(filepath.Join(packDir, "ui"), func(path string, info os.FileInfo, err error) error {
        if err != nil || info == nil || info.IsDir() { return nil }
        if filepath.Ext(path) != ".json" { return nil }
        rel, _ := filepath.Rel(packDir, path)
        writeFile(rel)
        return nil
    })
    // web-plugin js
    _ = filepath.Walk(filepath.Join(packDir, "web-plugin"), func(path string, info os.FileInfo, err error) error {
        if err != nil || info == nil || info.IsDir() { return nil }
        if filepath.Ext(path) != ".js" { return nil }
        rel, _ := filepath.Rel(packDir, path)
        writeFile(rel)
        return nil
    })
    // root *.pb
    _ = filepath.Walk(packDir, func(path string, info os.FileInfo, err error) error {
        if err != nil || info == nil || info.IsDir() { return nil }
        if filepath.Dir(path) != packDir { return nil }
        if filepath.Ext(path) != ".pb" { return nil }
        rel, _ := filepath.Rel(packDir, path)
        writeFile(rel)
        return nil
    })
    sum := h.Sum(nil)
    return hex.EncodeToString(sum)
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
        if s.userRepo == nil || s.jwtMgr == nil { http.Error(w, "auth disabled", http.StatusServiceUnavailable); return }
        var in struct{ Username string `json:"username"`; Password string `json:"password"` }
        if err := json.NewDecoder(r.Body).Decode(&in); err != nil { http.Error(w, err.Error(), 400); return }
        ur, err := s.userRepo.Verify(r.Context(), in.Username, in.Password)
        if err != nil { http.Error(w, "unauthorized", http.StatusUnauthorized); return }
        // collect role names
        roles := []string{}
        if rs, err := s.userRepo.ListUserRoles(r.Context(), ur.ID); err == nil {
            for _, rr := range rs { roles = append(roles, rr.Name) }
        }
        tok, _ := s.jwtMgr.Sign(in.Username, roles, 8*time.Hour)
        _ = json.NewEncoder(w).Encode(struct{ Token string `json:"token"`; User any `json:"user"` }{Token: tok, User: struct{ Username string `json:"username"`; Roles []string `json:"roles"` }{in.Username, roles}})
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
        // approvals snapshot counters (best-effort)
        if s.approvals != nil {
            type cnt struct{ Pending, Approved, Rejected int }
            get := func(state string) int {
                items, total, err := s.approvals.List(appr.Filter{State: state}, appr.Page{Page: 1, Size: 1})
                _ = items
                if err != nil { return -1 }
                return total
            }
            out["approvals"] = map[string]int{
                "pending":  get("pending"),
                "approved": get("approved"),
                "rejected": get("rejected"),
            }
        }
        _ = json.NewEncoder(w).Encode(out)
    })

    // UI schema fetch: /api/ui_schema?id=<function_id>
    s.mux.HandleFunc("/api/ui_schema", func(w http.ResponseWriter, r *http.Request){
        addCORS(w, r)
        if r.Method != http.MethodGet { w.WriteHeader(http.StatusMethodNotAllowed); return }
        id := r.URL.Query().Get("id")
        if id == "" { http.Error(w, "missing id", 400); return }
        base := sanitize(id)
        schemaPath := filepath.Join(s.packDir, "ui", base+".schema.json")
        uiPath := filepath.Join(s.packDir, "ui", base+".uischema.json")
        var schema, uischema any
        if b, err := os.ReadFile(schemaPath); err == nil { _ = json.Unmarshal(b, &schema) }
        if b, err := os.ReadFile(uiPath); err == nil { _ = json.Unmarshal(b, &uischema) }
        _ = json.NewEncoder(w).Encode(map[string]any{"schema": schema, "uischema": uischema})
    })

    // Pack import: multipart/form-data with file=pack.tgz
    s.mux.HandleFunc("/api/packs/import", func(w http.ResponseWriter, r *http.Request){
        addCORS(w, r)
        if r.Method != http.MethodPost { w.WriteHeader(http.StatusMethodNotAllowed); return }
        if user, _, ok := s.auth(r); !ok { http.Error(w, "unauthorized", http.StatusUnauthorized); return } else {
            if s.rbac != nil && !(s.rbac.Can(user, "packs:import") || s.rbac.Can(user, "*")) { http.Error(w, "forbidden", http.StatusForbidden); return }
        }
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
    // Packs list: return current manifest and basic counts
    s.mux.HandleFunc("/api/packs/list", func(w http.ResponseWriter, r *http.Request){
        addCORS(w, r)
        if r.Method != http.MethodGet { w.WriteHeader(http.StatusMethodNotAllowed); return }
        maniPath := filepath.Join(s.packDir, "manifest.json")
        b, err := os.ReadFile(maniPath)
        if err != nil { http.Error(w, "manifest not found", http.StatusNotFound); return }
        var mani any
        _ = json.Unmarshal(b, &mani)
        // basic counts
        type counts struct{ Descriptors int `json:"descriptors"`; UISchema int `json:"ui_schema"` }
        c := counts{}
        _ = filepath.Walk(filepath.Join(s.packDir, "descriptors"), func(path string, info os.FileInfo, err error) error {
            if err != nil || info == nil || info.IsDir() { return nil }
            if filepath.Ext(path) == ".json" { c.Descriptors++ }
            return nil
        })
        _ = filepath.Walk(filepath.Join(s.packDir, "ui"), func(path string, info os.FileInfo, err error) error {
            if err != nil || info == nil || info.IsDir() { return nil }
            if filepath.Ext(path) == ".json" { c.UISchema++ }
            return nil
        })
        etag := computePackETag(s.packDir)
        _ = json.NewEncoder(w).Encode(map[string]any{"manifest": mani, "counts": c, "etag": etag, "export_auth_required": s.packsExportRequireAuth})
    })
    // Packs export: stream current pack (descriptors/ui/manifest/web-plugin/*.js and any *.pb) as tar.gz
    s.mux.HandleFunc("/api/packs/export", func(w http.ResponseWriter, r *http.Request){
        addCORS(w, r)
        if r.Method != http.MethodGet { w.WriteHeader(http.StatusMethodNotAllowed); return }
        if s.packsExportRequireAuth {
            if user, _, ok := s.auth(r); !ok { http.Error(w, "unauthorized", http.StatusUnauthorized); return } else {
                if s.rbac != nil && !(s.rbac.Can(user, "packs:export") || s.rbac.Can(user, "*")) { http.Error(w, "forbidden", http.StatusForbidden); return }
            }
        }
        if et := computePackETag(s.packDir); et != "" { w.Header().Set("ETag", et) }
        w.Header().Set("Content-Type", "application/gzip")
        w.Header().Set("Content-Disposition", "attachment; filename=pack.tgz")
        gz := gzip.NewWriter(w)
        defer gz.Close()
        tw := tar.NewWriter(gz)
        defer tw.Close()
        // write manifest.json if exists
        maniPath := filepath.Join(s.packDir, "manifest.json")
        if b, err := os.ReadFile(maniPath); err == nil {
            hdr := &tar.Header{Name: "manifest.json", Mode: 0644, Size: int64(len(b))}
            if err := tw.WriteHeader(hdr); err == nil { _, _ = tw.Write(b) }
        }
        // helper to add files under a dir with a prefix
        addDir := func(rel string) {
            base := filepath.Join(s.packDir, rel)
            _ = filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
                if err != nil || info == nil || info.IsDir() { return nil }
                // limit to json/js/pb files
                if !(filepath.Ext(path) == ".json" || filepath.Ext(path) == ".js" || filepath.Ext(path) == ".pb") { return nil }
                b, err := os.ReadFile(path)
                if err != nil { return nil }
                relPath, _ := filepath.Rel(s.packDir, path)
                hdr := &tar.Header{Name: filepath.ToSlash(relPath), Mode: 0644, Size: int64(len(b))}
                if err := tw.WriteHeader(hdr); err == nil { _, _ = tw.Write(b) }
                return nil
            })
        }
        addDir("descriptors")
        addDir("ui")
        addDir("web-plugin")
        // also include any *.pb at root
        _ = filepath.Walk(s.packDir, func(path string, info os.FileInfo, err error) error {
            if err != nil || info == nil || info.IsDir() { return nil }
            if filepath.Dir(path) != s.packDir { return nil }
            if filepath.Ext(path) != ".pb" { return nil }
            b, err := os.ReadFile(path)
            if err != nil { return nil }
            relPath, _ := filepath.Rel(s.packDir, path)
            hdr := &tar.Header{Name: filepath.ToSlash(relPath), Mode: 0644, Size: int64(len(b))}
            if err := tw.WriteHeader(hdr); err == nil { _, _ = tw.Write(b) }
            return nil
        })
    })
    // Packs reload: rescan packDir for descriptors and fds (useful after out-of-band changes)
    s.mux.HandleFunc("/api/packs/reload", func(w http.ResponseWriter, r *http.Request){
        addCORS(w, r)
        if r.Method != http.MethodPost { w.WriteHeader(http.StatusMethodNotAllowed); return }
        if user, _, ok := s.auth(r); !ok { http.Error(w, "unauthorized", http.StatusUnauthorized); return } else {
            if s.rbac != nil && !(s.rbac.Can(user, "packs:reload") || s.rbac.Can(user, "*")) { http.Error(w, "forbidden", http.StatusForbidden); return }
        }
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
    // Assignments management: GET list; POST set {game_id,env,functions}
    s.mux.HandleFunc("/api/assignments", func(w http.ResponseWriter, r *http.Request){
        addCORS(w, r)
        switch r.Method {
        case http.MethodGet:
            if user, _, ok := s.auth(r); !ok { http.Error(w, "unauthorized", http.StatusUnauthorized); return } else {
                if s.rbac != nil && !(s.rbac.Can(user, "assignments:read") || s.rbac.Can(user, "*")) { http.Error(w, "forbidden", http.StatusForbidden); return }
            }
            gid := r.URL.Query().Get("game_id")
            env := r.URL.Query().Get("env")
            s.mu.RLock()
            out := map[string][]string{}
            for k, v := range s.assignments {
                if gid != "" || env != "" {
                    parts := strings.SplitN(k, "|", 2)
                    ge := ""; if len(parts) > 1 { ge = parts[1] }
                    if (gid != "" && parts[0] != gid) || (env != "" && ge != env) { continue }
                }
                out[k] = append([]string{}, v...)
            }
            s.mu.RUnlock()
            _ = json.NewEncoder(w).Encode(struct{ Assignments map[string][]string `json:"assignments"` }{Assignments: out})
        case http.MethodPost:
            actor := ""
            if u, _, ok := s.auth(r); !ok { http.Error(w, "unauthorized", http.StatusUnauthorized); return } else {
                actor = u
                if s.rbac != nil && !(s.rbac.Can(u, "assignments:write") || s.rbac.Can(u, "*")) { http.Error(w, "forbidden", http.StatusForbidden); return }
            }
            var in struct{ GameID, Env string; Functions []string }
            if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.GameID == "" {
                http.Error(w, "bad request", http.StatusBadRequest); return }
            // validate function ids
            valid := make([]string, 0, len(in.Functions))
            unknown := []string{}
            for _, fid := range in.Functions {
                if _, ok := s.descIndex[fid]; ok { valid = append(valid, fid) } else { unknown = append(unknown, fid) }
            }
            key := in.GameID + "|" + in.Env
            s.mu.Lock(); s.assignments[key] = append([]string{}, valid...); s.mu.Unlock()
            b, _ := json.MarshalIndent(s.assignments, "", "  ")
            _ = os.WriteFile(s.assignmentsPath, b, 0o644)
            // audit
            if s.audit != nil {
                meta := map[string]string{
                    "game_env": key,
                    "game_id": in.GameID,
                    "env": in.Env,
                    "functions": strings.Join(valid, ","),
                }
                if len(unknown) > 0 { meta["unknown"] = strings.Join(unknown, ",") }
                if err := s.audit.Log("assignments.update", actor, key, meta); err != nil { atomic.AddInt64(&s.auditErrors, 1) }
            }
            _ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "unknown": unknown})
        default:
            w.WriteHeader(http.StatusMethodNotAllowed)
        }
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

    // Serve pack static (manifest.json, web-plugin/*)
    s.mux.Handle("/pack_static/", http.StripPrefix("/pack_static/", http.FileServer(http.Dir(s.packDir))))

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
        // ABAC allow_if expression (optional): descriptor.auth.allow_if
        if d := s.descIndex[in.FunctionID]; d != nil && d.Auth != nil {
            if expr, ok := d.Auth["allow_if"].(string); ok && expr != "" {
                userName, roles, _ := s.auth(r)
                ctx := policyContext{User: userName, Roles: roles, GameID: gameID, Env: env, FunctionID: in.FunctionID}
                if !evalAllowIf(expr, ctx) { http.Error(w, "forbidden", http.StatusForbidden); return }
            }
        }
        // rate limit and concurrency guards
        if d := s.descIndex[in.FunctionID]; d != nil && d.Semantics != nil {
            if v, ok := d.Semantics["rate_limit"].(string); ok && v != "" {
                rl := s.getRateLimiter(in.FunctionID, v)
                if rl != nil && !rl.Try() { http.Error(w, "rate limited", 429); return }
            }
            if v, ok := d.Semantics["concurrency"].(float64); ok && v > 0 {
                sem := s.getSemaphore(in.FunctionID, int(v))
                select {
                case sem <- struct{}{}:
                    defer func(){ <-sem }()
                default:
                    http.Error(w, "too many concurrent requests", 429)
                    return
                }
            }
        }
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
        var in struct{ ID string `json:"id"`; OTP string `json:"otp,omitempty"` }
        if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.ID == "" { http.Error(w, "missing id", 400); return }
        a, err := s.approvals.Approve(in.ID)
        if err != nil { http.Error(w, err.Error(), 409); return }
        // OTP check (optional): require OTP if function is high risk or descriptor.auth.require_otp == true and user has otp_secret
        if d := s.descIndex[a.FunctionID]; d != nil {
            needOTP := strings.EqualFold(d.Risk, "high")
            if auth := d.Auth; auth != nil {
                if v, ok := auth["require_otp"].(bool); ok && v { needOTP = true }
            }
            // Optional OTP support can be reintroduced when user store tracks OTP secrets in DB.
        }
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
        if user, _, ok := s.auth(r); !ok { http.Error(w, "unauthorized", http.StatusUnauthorized); return } else {
            if s.rbac != nil && !(s.rbac.Can(user, "audit:read") || s.rbac.Can(user, "*")) { http.Error(w, "forbidden", http.StatusForbidden); return }
        }
        if r.Method != http.MethodGet { w.WriteHeader(http.StatusMethodNotAllowed); return }
        gameID := r.URL.Query().Get("game_id")
        env := r.URL.Query().Get("env")
        actor := r.URL.Query().Get("actor")
        kind := r.URL.Query().Get("kind")
        // optional time range filters: start/end accept RFC3339 or unix seconds/milliseconds
        parseBound := func(qs string) (time.Time, bool) {
            v := strings.TrimSpace(r.URL.Query().Get(qs))
            if v == "" { return time.Time{}, false }
            // try RFC3339
            if t, err := time.Parse(time.RFC3339, v); err == nil { return t, true }
            // try integer seconds or ms
            var n int64
            if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
                if n > 1_000_000_000_000 { // likely ms
                    return time.Unix(0, n*int64(time.Millisecond)), true
                }
                return time.Unix(n, 0), true
            }
            return time.Time{}, false
        }
        startT, hasStart := parseBound("start")
        endT, hasEnd := parseBound("end")
        // pagination (optional): limit, offset or page+size
        limit := 200
        if qs := r.URL.Query().Get("limit"); qs != "" { fmt.Sscanf(qs, "%d", &limit) }
        if qs := r.URL.Query().Get("size"); qs != "" { fmt.Sscanf(qs, "%d", &limit) }
        if limit <= 0 { limit = 200 }
        offset := 0
        if qs := r.URL.Query().Get("offset"); qs != "" { fmt.Sscanf(qs, "%d", &offset) }
        if qs := r.URL.Query().Get("page"); qs != "" {
            var page int
            fmt.Sscanf(qs, "%d", &page)
            if page > 0 { offset = (page-1) * limit }
        }
        type resp struct{ Events []auditchain.Event `json:"events"`; Total int `json:"total"` }
        all := make([]auditchain.Event, 0, limit)
        // naive scan (reads entire logs/audit.log; ok for dev/demo)
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
                if hasStart && ev.Time.Before(startT) { continue }
                if hasEnd && ev.Time.After(endT) { continue }
                all = append(all, ev)
            }
        }
        // newest-first ordering
        for i, j := 0, len(all)-1; i < j; i, j = i+1, j-1 { all[i], all[j] = all[j], all[i] }
        total := len(all)
        // apply offset+limit window
        start := offset
        if start > total { start = total }
        end := start + limit
        if end > total { end = total }
        window := all[start:end]
        _ = json.NewEncoder(w).Encode(resp{Events: window, Total: total})
    })
    // Registry summary
    s.mux.HandleFunc("/api/registry", func(w http.ResponseWriter, r *http.Request) {
        addCORS(w, r)
        if user, _, ok := s.auth(r); !ok { http.Error(w, "unauthorized", http.StatusUnauthorized); return } else {
            if s.rbac != nil && !(s.rbac.Can(user, "registry:read") || s.rbac.Can(user, "*")) { http.Error(w, "forbidden", http.StatusForbidden); return }
        }
        if r.Method != http.MethodGet { w.WriteHeader(http.StatusMethodNotAllowed); return }
        type Agent struct{ AgentID, GameID, Env, RpcAddr string; Functions int; Healthy bool; ExpiresInSec int }
        type Function struct{ GameID, ID string; Agents int }
        type FuncCov struct{ Healthy int `json:"healthy"`; Total int `json:"total"` }
        type Coverage struct{ GameEnv string `json:"game_env"`; Functions map[string]FuncCov `json:"functions"`; Uncovered []string `json:"uncovered"` }
        var agents []Agent
        var functions []Function
        var coverage []Coverage
        if s.reg != nil {
            s.reg.Mu().RLock()
            now := time.Now()
            for _, a := range s.reg.AgentsUnsafe() {
                healthy := now.Before(a.ExpireAt)
                exp := int(time.Until(a.ExpireAt).Seconds())
                if exp < 0 { exp = 0 }
                agents = append(agents, Agent{AgentID: a.AgentID, GameID: a.GameID, Env: a.Env, RpcAddr: a.RPCAddr, Functions: len(a.Functions), Healthy: healthy, ExpiresInSec: exp})
            }
            fnCountAll := map[string]map[string]int{}
            fnCountHealthy := map[string]map[string]int{}
            for _, a := range s.reg.AgentsUnsafe() {
                isHealthy := now.Before(a.ExpireAt)
                for fid := range a.Functions {
                    if fnCountAll[a.GameID] == nil { fnCountAll[a.GameID] = map[string]int{} }
                    fnCountAll[a.GameID][fid]++
                    if isHealthy {
                        if fnCountHealthy[a.GameID] == nil { fnCountHealthy[a.GameID] = map[string]int{} }
                        fnCountHealthy[a.GameID][fid]++
                    }
                }
            }
            for gid, m := range fnCountHealthy {
                for fid, c := range m { functions = append(functions, Function{GameID: gid, ID: fid, Agents: c}) }
            }
            // assignments coverage: for each game|env key, compute agents covering its functions (by game only)
            for k, fns := range s.assignments {
                parts := strings.SplitN(k, "|", 2)
                gid := parts[0]
                cov := map[string]FuncCov{}
                uncovered := []string{}
                for _, fid := range fns {
                    h := fnCountHealthy[gid][fid]
                    t := fnCountAll[gid][fid]
                    cov[fid] = FuncCov{Healthy: h, Total: t}
                    if h == 0 { uncovered = append(uncovered, fid) }
                }
                coverage = append(coverage, Coverage{GameEnv: k, Functions: cov, Uncovered: uncovered})
            }
            s.reg.Mu().RUnlock()
        }
        _ = json.NewEncoder(w).Encode(struct{ Agents []Agent `json:"agents"`; Functions []Function `json:"functions"`; Assignments any `json:"assignments"`; Coverage []Coverage `json:"coverage"` }{Agents: agents, Functions: functions, Assignments: s.assignments, Coverage: coverage})
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
    // Games RESTful implemented below

    // Games RESTful APIs
    s.mux.HandleFunc("/api/games", func(w http.ResponseWriter, r *http.Request) {
        addCORS(w, r)
        user, _, ok := s.auth(r)
        if !ok { http.Error(w, "unauthorized", http.StatusUnauthorized); return }
        canRead := s.rbac == nil || s.rbac.Can(user, "games:read") || s.rbac.Can(user, "games:manage")
        canManage := s.rbac == nil || s.rbac.Can(user, "games:manage")
        switch r.Method {
        case http.MethodGet:
            if !canRead { http.Error(w, "forbidden", http.StatusForbidden); return }
            items, err := s.games.List(r.Context())
            if err != nil { http.Error(w, err.Error(), 500); return }
            _ = json.NewEncoder(w).Encode(struct{ Games []*games.Game `json:"games"` }{Games: items})
        case http.MethodPost:
            if !canManage { http.Error(w, "forbidden", http.StatusForbidden); return }
            var in struct{ ID uint; Name, Icon, Description string; Enabled bool }
            if err := json.NewDecoder(r.Body).Decode(&in); err != nil { http.Error(w, err.Error(), 400); return }
            if in.ID == 0 {
                g := &games.Game{Name: in.Name, Icon: in.Icon, Description: in.Description, Enabled: in.Enabled}
                if err := s.games.Create(r.Context(), g); err != nil { http.Error(w, err.Error(), 500); return }
                _ = json.NewEncoder(w).Encode(struct{ ID uint `json:"id"` }{ID: g.ID})
            } else {
                g, err := s.games.Get(r.Context(), in.ID)
                if err != nil { http.Error(w, err.Error(), 404); return }
                g.Name, g.Icon, g.Description, g.Enabled = in.Name, in.Icon, in.Description, in.Enabled
                if err := s.games.Update(r.Context(), g); err != nil { http.Error(w, err.Error(), 500); return }
                w.WriteHeader(http.StatusNoContent)
            }
        default:
            w.WriteHeader(http.StatusMethodNotAllowed)
        }
    })
    s.mux.HandleFunc("/api/games/", func(w http.ResponseWriter, r *http.Request) {
        addCORS(w, r)
        user, _, ok := s.auth(r)
        if !ok { http.Error(w, "unauthorized", http.StatusUnauthorized); return }
        rest := strings.TrimPrefix(r.URL.Path, "/api/games/")
        if strings.HasSuffix(rest, "/envs") {
            idStr := strings.TrimSuffix(strings.TrimSuffix(rest, "/envs"), "/")
            id64, _ := strconv.ParseUint(idStr, 10, 64)
            id := uint(id64)
            switch r.Method {
            case http.MethodGet:
                if s.rbac != nil && !(s.rbac.Can(user, "games:read") || s.rbac.Can(user, "games:manage")) { http.Error(w, "forbidden", http.StatusForbidden); return }
                envs, err := s.games.ListEnvs(r.Context(), id)
                if err != nil { http.Error(w, err.Error(), 500); return }
                _ = json.NewEncoder(w).Encode(struct{ Envs []string `json:"envs"` }{Envs: envs})
            case http.MethodPost:
                if s.rbac != nil && !s.rbac.Can(user, "games:manage") { http.Error(w, "forbidden", http.StatusForbidden); return }
                var in struct{ Env string }
                if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Env == "" { http.Error(w, "invalid env", 400); return }
                if err := s.games.AddEnv(r.Context(), id, in.Env); err != nil { http.Error(w, err.Error(), 500); return }
                w.WriteHeader(http.StatusNoContent)
            case http.MethodDelete:
                if s.rbac != nil && !s.rbac.Can(user, "games:manage") { http.Error(w, "forbidden", http.StatusForbidden); return }
                env := r.URL.Query().Get("env")
                if env == "" { http.Error(w, "missing env", 400); return }
                if err := s.games.RemoveEnv(r.Context(), id, env); err != nil { http.Error(w, err.Error(), 500); return }
                w.WriteHeader(http.StatusNoContent)
            default:
                w.WriteHeader(http.StatusMethodNotAllowed)
            }
            return
        }
        idStr := strings.TrimSuffix(rest, "/")
        id64, _ := strconv.ParseUint(idStr, 10, 64)
        id := uint(id64)
        switch r.Method {
        case http.MethodGet:
            if s.rbac != nil && !(s.rbac.Can(user, "games:read") || s.rbac.Can(user, "games:manage")) { http.Error(w, "forbidden", http.StatusForbidden); return }
            g, err := s.games.Get(r.Context(), id)
            if err != nil { http.Error(w, err.Error(), 404); return }
            _ = json.NewEncoder(w).Encode(g)
        case http.MethodPut:
            if s.rbac != nil && !s.rbac.Can(user, "games:manage") { http.Error(w, "forbidden", http.StatusForbidden); return }
            var in struct{ Name, Icon, Description string; Enabled bool }
            if err := json.NewDecoder(r.Body).Decode(&in); err != nil { http.Error(w, err.Error(), 400); return }
            g, err := s.games.Get(r.Context(), id)
            if err != nil { http.Error(w, err.Error(), 404); return }
            g.Name, g.Icon, g.Description, g.Enabled = in.Name, in.Icon, in.Description, in.Enabled
            if err := s.games.Update(r.Context(), g); err != nil { http.Error(w, err.Error(), 500); return }
            w.WriteHeader(http.StatusNoContent)
        case http.MethodDelete:
            if s.rbac != nil && !s.rbac.Can(user, "games:manage") { http.Error(w, "forbidden", http.StatusForbidden); return }
            if err := s.games.Delete(r.Context(), id); err != nil { http.Error(w, err.Error(), 500); return }
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

    // serve local uploads if using file driver
    if strings.ToLower(s.objConf.Driver) == "file" && s.objConf.BaseDir != "" {
        upfs := http.FileServer(http.Dir(s.objConf.BaseDir))
        s.mux.Handle("/uploads/", http.StripPrefix("/uploads/", upfs))
    }

    // basic upload + signed url endpoints
    s.mux.HandleFunc("/api/upload", func(w http.ResponseWriter, r *http.Request) {
        addCORS(w, r)
        user, _, ok := s.auth(r)
        if !ok { http.Error(w, "unauthorized", http.StatusUnauthorized); return }
        if s.obj == nil { http.Error(w, "storage not available", http.StatusServiceUnavailable); return }
        if r.Method != http.MethodPost { w.WriteHeader(http.StatusMethodNotAllowed); return }
        // RBAC: require uploads:write
        if s.rbac != nil && !s.rbac.Can(user, "uploads:write") { http.Error(w, "forbidden", http.StatusForbidden); return }
        // Size limit: 120MB
        constMax := int64(120 * 1024 * 1024)
        if cl := r.Header.Get("Content-Length"); cl != "" {
            if n, err := strconv.ParseInt(cl, 10, 64); err == nil && n > constMax {
                http.Error(w, "request too large", http.StatusRequestEntityTooLarge); return
            }
        }
        r.Body = http.MaxBytesReader(w, r.Body, constMax)
        if err := r.ParseMultipartForm(32 << 20); err != nil { http.Error(w, err.Error(), 400); return }
        f, fh, err := r.FormFile("file")
        if err != nil { http.Error(w, "missing file", 400); return }
        defer f.Close()
        // generate key
        ts := time.Now().UnixNano()
        name := fh.Filename
        key := fmt.Sprintf("%s/%d_%s", user, ts, name)
        // copy to temp file to support ReadSeeker
        tmp, err := os.CreateTemp("", "upload-*")
        if err != nil { http.Error(w, err.Error(), 500); return }
        defer os.Remove(tmp.Name())
        if _, err := io.Copy(tmp, f); err != nil { tmp.Close(); http.Error(w, err.Error(), 500); return }
        if _, err := tmp.Seek(0, io.SeekStart); err != nil { tmp.Close(); http.Error(w, err.Error(), 500); return }
        ct := fh.Header.Get("Content-Type")
        if err := s.obj.Put(r.Context(), key, tmp, fh.Size, ct); err != nil { tmp.Close(); http.Error(w, err.Error(), 500); return }
        _ = tmp.Close()
        url, _ := s.obj.SignedURL(r.Context(), key, "GET", s.objConf.SignedURLTTL)
        _ = json.NewEncoder(w).Encode(struct{ Key, URL string }{Key: key, URL: url})
    })

    s.mux.HandleFunc("/api/signed_url", func(w http.ResponseWriter, r *http.Request) {
        addCORS(w, r)
        if _, _, ok := s.auth(r); !ok { http.Error(w, "unauthorized", http.StatusUnauthorized); return }
        if s.obj == nil { http.Error(w, "storage not available", http.StatusServiceUnavailable); return }
        key := r.URL.Query().Get("key")
        if key == "" { http.Error(w, "missing key", 400); return }
        method := r.URL.Query().Get("op")
        if method == "" { method = "GET" }
        // parse expiry like 10m
        exp := s.objConf.SignedURLTTL
        if v := r.URL.Query().Get("ttl"); v != "" { if d, err := time.ParseDuration(v); err == nil { exp = d } }
        url, err := s.obj.SignedURL(r.Context(), key, method, exp)
        if err != nil { http.Error(w, err.Error(), 500); return }
        _ = json.NewEncoder(w).Encode(struct{ URL string `json:"url"` }{URL: url})
    })
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
        // Only extract descriptors/*.json, ui/*.json, manifest.json, web-plugin/* and any *.pb
        if strings.HasPrefix(hdr.Name, "descriptors/") || strings.HasPrefix(hdr.Name, "ui/") || strings.HasPrefix(hdr.Name, "web-plugin/") || hdr.Name == "manifest.json" || strings.HasSuffix(hdr.Name, ".pb") {
            // Normalize: strip leading "descriptors/" so files land directly under dest/
            name := filepath.FromSlash(hdr.Name)
            if strings.HasPrefix(name, "descriptors/") {
                name = strings.TrimPrefix(name, "descriptors/")
            }
            target := filepath.Join(dest, name)
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

// --- Simple ABAC policy evaluator (==,!=,&&,||, has_role('x')) ---
type policyContext struct { User string; Roles []string; GameID string; Env string; FunctionID string }
func hasRole(roles []string, want string) bool { for _, r := range roles { if r == want { return true } }; return false }
func evalAllowIf(expr string, ctx policyContext) bool {
    trim := strings.TrimSpace
    parseLit := func(s string) any {
        s = trim(s)
        if s == "true" { return true }; if s == "false" { return false }
        if (strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"")) || (strings.HasPrefix(s, "'") && strings.HasSuffix(s, "'")) { return s[1:len(s)-1] }
        if n, err := strconv.ParseFloat(s, 64); err == nil { return n }
        return s
    }
    get := func(path string) any {
        p := trim(path)
        switch p {
        case "user": return ctx.User
        case "game_id": return ctx.GameID
        case "env": return ctx.Env
        case "function_id": return ctx.FunctionID
        default:
            if strings.HasPrefix(p, "has_role(") && strings.HasSuffix(p, ")") {
                arg := strings.TrimSuffix(strings.TrimPrefix(p, "has_role("), ")")
                v := parseLit(arg)
                if s, ok := v.(string); ok { return hasRole(ctx.Roles, s) }
                return false
            }
            // unknown identifier -> empty
            return ""
        }
    }
    // Evaluate OR of AND terms
    orParts := strings.Split(expr, "||")
    for _, orp := range orParts {
        andOk := true
        for _, andp := range strings.Split(orp, "&&") {
            p := trim(andp)
            if p == "" { continue }
            // support lhs op rhs or function(bool)
            if strings.Contains(p, "==") || strings.Contains(p, "!=") {
                op := "=="; i := strings.Index(p, "=="); j := strings.Index(p, "!=")
                if j >= 0 && (i < 0 || j < i) { op = "!="; i = j }
                lhs := trim(p[:i]); rhs := trim(p[i+len(op):])
                lv := get(lhs); rv := parseLit(rhs)
                eq := false
                switch l := lv.(type) {
                case string:
                    if rs, ok := rv.(string); ok { eq = (l == rs) }
                case bool:
                    if rb, ok := rv.(bool); ok { eq = (l == rb) }
                case float64:
                    if rf, ok := rv.(float64); ok { eq = (math.Abs(l-rf) < 1e-9) }
                default:
                    eq = false
                }
                if (op == "==" && !eq) || (op == "!=" && eq) { andOk = false; break }
            } else {
                // bare function/identifier truthiness
                v := get(p)
                ok := false
                switch t := v.(type) {
                case bool: ok = t
                case string: ok = (t != "")
                default: ok = v != nil
                }
                if !ok { andOk = false; break }
            }
        }
        if andOk { return true }
    }
    return false
}

// --- Simple token bucket ---
type rateLimiter struct { cap int; tokens float64; last time.Time; rate float64; mu sync.Mutex }
func newRateLimiter(rps int) *rateLimiter { return &rateLimiter{cap: rps, tokens: float64(rps), last: time.Now(), rate: float64(rps)} }
func (r *rateLimiter) Try() bool {
    r.mu.Lock(); defer r.mu.Unlock()
    now := time.Now()
    dt := now.Sub(r.last).Seconds()
    r.tokens = math.Min(float64(r.cap), r.tokens + dt*r.rate)
    r.last = now
    if r.tokens >= 1 { r.tokens -= 1; return true }
    return false
}
func parseRPS(s string) int {
    s = strings.TrimSpace(s)
    if strings.HasSuffix(s, "rps") { s = strings.TrimSuffix(s, "rps") }
    if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil && n>0 { return n }
    return 0
}
func (s *Server) getRateLimiter(fid string, cfg string) *rateLimiter {
    rps := parseRPS(cfg)
    if rps <= 0 { return nil }
    s.mu.Lock(); defer s.mu.Unlock()
    rl := s.rl[fid]
    if rl == nil { rl = newRateLimiter(rps); s.rl[fid] = rl }
    return rl
}
func (s *Server) getSemaphore(fid string, n int) chan struct{} {
    if n <= 0 { return nil }
    s.mu.Lock(); defer s.mu.Unlock()
    sem := s.conc[fid]
    if sem == nil || cap(sem) != n { sem = make(chan struct{}, n); s.conc[fid] = sem }
    return sem
}

// sanitize converts an id into a filesystem-friendly base name (mirrors generator sanitize)
func sanitize(id string) string {
    out := strings.Map(func(r rune) rune {
        switch {
        case r >= 'a' && r <= 'z':
            return r
        case r >= 'A' && r <= 'Z':
            return r
        case r >= '0' && r <= '9':
            return r
        case r == '.' || r == '-' || r == '_':
            return r
        default:
            return '-'
        }
    }, id)
    return out
}
