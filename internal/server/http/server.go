package httpserver

import (
	"encoding/json"
	"log"
	"log/slog"
	"net/http"
    "net"

	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
    "time"

	auditchain "github.com/cuihairu/croupier/internal/audit/chain"
	"github.com/cuihairu/croupier/internal/auth/rbac"
	jwt "github.com/cuihairu/croupier/internal/auth/token"
	"github.com/cuihairu/croupier/internal/connpool"
	"github.com/cuihairu/croupier/internal/function/descriptor"
	msgsgorm "github.com/cuihairu/croupier/internal/infra/persistence/gorm/messages"
	usersgorm "github.com/cuihairu/croupier/internal/infra/persistence/gorm/users"
	certmonitor "github.com/cuihairu/croupier/internal/infra/monitoring/certificates"
	"github.com/cuihairu/croupier/internal/loadbalancer"
	pack "github.com/cuihairu/croupier/internal/pack"
	appr "github.com/cuihairu/croupier/internal/server/approvals"
	"github.com/cuihairu/croupier/internal/server/games"
	"github.com/cuihairu/croupier/internal/infra/persistence/gorm/support"
	"github.com/cuihairu/croupier/internal/server/registry"
	"github.com/cuihairu/croupier/internal/validation"
	entityvalidation "github.com/cuihairu/croupier/internal/validation"
	localv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/local/v1"
	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	gmysql "gorm.io/driver/mysql"
	gpostgres "gorm.io/driver/postgres"
	gsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"net/url"

	obj "github.com/cuihairu/croupier/internal/objstore"
    gin "github.com/gin-gonic/gin"
)

type Server struct {
	descs     []*descriptor.Descriptor
	descIndex map[string]*descriptor.Descriptor
	invoker   FunctionInvoker
	audit     *auditchain.Writer
	rbac      rbac.PolicyInterface
	games     *games.Repo
	reg       *registry.Store
	userRepo  *usersgorm.Repo
	msgRepo   *msgsgorm.Repo
	// message event subscribers (for SSE push)
	msgSubs          map[chan struct{}]struct{}
	jwtMgr           *jwt.Manager
	startedAt        time.Time
	invocations      int64
	invocationsError int64
	jobsStarted      int64
	jobsError        int64
	rbacDenied       int64
	auditErrors      int64
	locator          interface{ GetJobAddr(string) (string, bool) }
	statsProv        interface {
		GetStats() map[string]*loadbalancer.AgentStats
		GetPoolStats() *connpool.PoolStats
	}
	typeReg *pack.TypeRegistry
	packDir string
	mu      sync.RWMutex
	fn      map[string]*fnMetrics
	// approvals store (two-person rule)
	approvals appr.Store
	// per-function rate limiters and concurrency semaphores
	rl   map[string]*rateLimiter
    conc map[string]chan struct{}
    // dynamic rate limit rules (function-level MVP): function_id -> rps
    rateLimitRules map[string]int
    rateLimitsPath string
    // service-level rate limit: agent_id -> rps
    serviceRateRules map[string]int
    fnRulesAdv       []rateRuleAdv
    svcRulesAdv      []rateRuleAdv
    // recent jobs (in-memory)
    jobsMu    sync.Mutex
    jobs      map[string]*jobInfo
    jobsOrder []string
	// assignments: game_id|env -> []function_ids
	assignments     map[string][]string
	assignmentsPath string
	// packs export auth requirement (optional via env)
	packsExportRequireAuth bool
	// games metadata via GORM (postgres preferred, else sqlite)
	gdb *gorm.DB

	obj           obj.Store
	objConf       obj.Config
	httpSrv       *http.Server
	componentMgr  *pack.ComponentManager
	// monitoring stores
    certStore *certmonitor.Store

    // login rate limiting (in-memory): key = ip|username -> attempt times within window
    loginAttempts map[string][]time.Time
    loginMu       sync.Mutex

    // optional IP -> region resolver via HTTP endpoint
    geoIPHTTP     string
    geoIPTimeout  time.Duration
    ipRegionCache map[string]struct{ val string; exp time.Time }
    ipRegionMu    sync.RWMutex
}

// rateRuleAdv supports gray matching and percent rollout
type rateRuleAdv struct {
    Scope     string            `json:"scope"`
    Key       string            `json:"key"`
    LimitQPS  int               `json:"limit_qps"`
    Match     map[string]string `json:"match,omitempty"`
    Percent   int               `json:"percent,omitempty"`
}

// jobInfo captures recent job metadata for /api/ops/jobs
type jobInfo struct {
    ID         string    `json:"id"`
    FunctionID string    `json:"function_id"`
    Actor      string    `json:"actor"`
    GameID     string    `json:"game_id"`
    Env        string    `json:"env"`
    State      string    `json:"state"`
    StartedAt  time.Time `json:"started_at"`
    EndedAt    time.Time `json:"ended_at"`
    DurationMs int64     `json:"duration_ms"`
    Error      string    `json:"error"`
    RPCAddr    string    `json:"rpc_addr"`
    TraceID    string    `json:"trace_id"`
}

// ginAuthZ is a unified authorization middleware.
// It enforces RBAC via Casbin policy (path+method) when a CasbinPolicy is configured.
// If not using Casbin, it becomes a no-op (per-route checks remain effective).
func (s *Server) ginAuthZ() gin.HandlerFunc {
    // endpoints to skip (auth handled specially or should be public)
    skip := map[string]bool{
        "/api/auth/login":     true,
        "/api/auth/me":        true, // user info handled by route-level token check
        "/api/descriptors":    true, // keep backward-compatible public descriptors
        "/api/ui_schema":      true, // schema preview
        "/api/messages/stream": true, // SSE uses token query param; route does its own auth
        "/api/messages/unread_count": true, // route-level token check
        "/api/messages": true, // route-level token check
        "/api/agent/meta": true, // agent meta reports are token-gated separately
    }
    return func(c *gin.Context) {
        // Only guard API paths; let static and others pass
        p := c.Request.URL.Path
        if c.Request.Method == http.MethodOptions { // CORS preflight already handled
            c.Next()
            return
        }
        if !strings.HasPrefix(p, "/api/") || skip[p] || (p == "/api/packs/export" && !s.packsExportRequireAuth) {
            c.Next()
            return
        }
        // Only enforce when Casbin policy is active
        if _, ok := s.rbac.(*rbac.CasbinPolicy); !ok {
            c.Next()
            return
        }
        user, roles, ok := s.auth(c.Request)
        if !ok {
            // Unified JSON error
            s.respondError(c, http.StatusUnauthorized, "unauthorized", "unauthorized")
            c.Abort()
            return
        }
        if !s.canHTTP(user, roles, c.Request) {
            s.respondError(c, http.StatusForbidden, "forbidden", "forbidden")
            c.Abort()
            return
        }
        c.Next()
    }
}

// can checks permission for user or any of their roles.
func (s *Server) can(user string, roles []string, perm string) bool {
	if s.rbac == nil {
		return true
	}

	// Check direct user permissions
	if s.rbac.Can(user, perm) {
		log.Printf("[RBAC] ALLOWED: user %s has permission %s", user, perm)
		return true
	}

	// Check user with prefix
	userKey := "user:" + user
	if s.rbac.Can(userKey, perm) {
		log.Printf("[RBAC] ALLOWED: %s has permission %s", userKey, perm)
		return true
	}

	// Check role permissions
	for _, role := range roles {
		roleKey := "role:" + role
		if s.rbac.Can(roleKey, perm) {
			log.Printf("[RBAC] ALLOWED: user %s has permission %s via role %s", user, perm, roleKey)
			return true
		}
	}

	log.Printf("[RBAC] DENIED: user=%s, roles=%v, perm=%s", user, roles, perm)
	return false
}

// canHTTP checks HTTP request permission using the new interface
func (s *Server) canHTTP(user string, roles []string, r *http.Request) bool {
	if s.rbac == nil {
		return true
	}

	// Use the new CanHTTP method if available
	return s.rbac.CanHTTP(user, roles, r)
}

type FunctionInvoker interface {
	Invoke(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.InvokeResponse, error)
	StartJob(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.StartJobResponse, error)
	StreamJob(ctx context.Context, req *functionv1.JobStreamRequest) (functionv1.FunctionService_StreamJobClient, error)
	CancelJob(ctx context.Context, req *functionv1.CancelJobRequest) (*functionv1.StartJobResponse, error)
}

func NewServer(descriptorDir string, invoker FunctionInvoker, audit *auditchain.Writer, policy rbac.PolicyInterface, reg *registry.Store, jwtMgr *jwt.Manager, locator interface{ GetJobAddr(string) (string, bool) }, statsProv interface {
	GetStats() map[string]*loadbalancer.AgentStats
	GetPoolStats() *connpool.PoolStats
}) (*Server, error) {
	descs, err := descriptor.LoadAll(descriptorDir)
	if err != nil {
		return nil, err
	}
	idx := map[string]*descriptor.Descriptor{}
	for _, d := range descs {
		idx[d.ID] = d
	}
    s := &Server{descs: descs, descIndex: idx, invoker: invoker, audit: audit, rbac: policy, reg: reg, jwtMgr: jwtMgr, startedAt: time.Now(), locator: locator, statsProv: statsProv, fn: map[string]*fnMetrics{}, typeReg: pack.NewTypeRegistry(), packDir: descriptorDir, rl: map[string]*rateLimiter{}, conc: map[string]chan struct{}{}, assignments: map[string][]string{}, assignmentsPath: filepath.Join(descriptorDir, "assignments.json"), loginAttempts: map[string][]time.Time{}, ipRegionCache: map[string]struct{ val string; exp time.Time }{}, rateLimitRules: map[string]int{}, serviceRateRules: map[string]int{}, jobs: map[string]*jobInfo{}}
    // Init rate limits persistence path
    if v := strings.TrimSpace(os.Getenv("RATE_LIMITS_PATH")); v != "" {
        s.rateLimitsPath = v
    } else {
        _ = os.MkdirAll("data", 0o755)
        s.rateLimitsPath = filepath.Join("data", "rate_limits.json")
    }
	s.msgSubs = make(map[chan struct{}]struct{})
	// approvals store: prefer Postgres via env DATABASE_URL, else in-memory
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL != "" {
		// Prefer explicit scheme
		if strings.HasPrefix(dbURL, "postgres://") || strings.HasPrefix(dbURL, "postgresql://") || strings.HasPrefix(dbURL, "pgx://") {
			if st, err := appr.NewPGStore(dbURL); err == nil {
				s.approvals = st
				log.Printf("[approvals] using postgres store")
			} else {
				log.Printf("[approvals] postgres disabled: %v; fallback to memory", err)
				s.approvals = appr.NewMemStore()
			}
		} else if strings.HasPrefix(dbURL, "sqlite://") || strings.HasPrefix(dbURL, "file:") || strings.HasSuffix(dbURL, ".db") || dbURL == ":memory:" {
			if st, err := appr.NewSQLiteStore(dbURL); err == nil {
				s.approvals = st
				log.Printf("[approvals] using sqlite store")
			} else {
				log.Printf("[approvals] sqlite disabled: %v; fallback to memory", err)
				s.approvals = appr.NewMemStore()
			}
		} else {
			// unknown scheme
			log.Printf("[approvals] unknown DATABASE_URL scheme; using memory")
			s.approvals = appr.NewMemStore()
		}
	} else {
		s.approvals = appr.NewMemStore()
	}
    _ = s.typeReg.LoadFDSFromDir(descriptorDir)
    // Load persisted rate limits (best-effort)
    s.loadRateLimitsFromFile()
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
    // GeoIP resolver via HTTP endpoint (optional)
    if u := strings.TrimSpace(os.Getenv("GEOIP_HTTP_URL")); u != "" {
        s.geoIPHTTP = u
    }
    if tv := strings.TrimSpace(os.Getenv("GEOIP_TIMEOUT_MS")); tv != "" {
        if n, err := strconv.Atoi(tv); err == nil && n > 0 { s.geoIPTimeout = time.Duration(n) * time.Millisecond }
    }
    if s.geoIPTimeout == 0 { s.geoIPTimeout = 1500 * time.Millisecond }
	// init GORM (prefer postgres)
	// DB initialization by config/env: DB_DRIVER=[postgres|mysql|sqlite|auto], DATABASE_URL as DSN
	sel := strings.ToLower(strings.TrimSpace(os.Getenv("DB_DRIVER")))
	dsn := os.Getenv("DATABASE_URL")
	openAuto := func() {
		if dsn != "" {
			if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") || strings.HasPrefix(dsn, "pgx://") {
				if db, err := gorm.Open(gpostgres.Open(dsn), &gorm.Config{}); err == nil {
					s.gdb = db
					return
				}
			}
			if strings.HasPrefix(dsn, "mysql://") || strings.Contains(dsn, "@tcp(") {
				norm := normalizeMySQLDSN(dsn)
				if db, err := gorm.Open(gmysql.Open(norm), &gorm.Config{}); err == nil {
					s.gdb = db
					return
				}
			}
		}
	}
	switch sel {
	case "postgres":
		if dsn != "" {
			if db, err := gorm.Open(gpostgres.Open(dsn), &gorm.Config{}); err == nil {
				s.gdb = db
			}
		}
	case "mysql":
		if dsn != "" {
			if db, err := gorm.Open(gmysql.Open(normalizeMySQLDSN(dsn)), &gorm.Config{}); err == nil {
				s.gdb = db
			}
		}
	case "sqlite":
		// allow explicit sqlite DSN; if empty, fallback below
		if dsn != "" {
			if db, err := gorm.Open(gsqlite.Open(dsn), &gorm.Config{}); err == nil {
				s.gdb = db
			}
		}
	case "", "auto":
		openAuto()
	default:
		openAuto()
	}
	if s.gdb == nil { // fallback sqlite local
		_ = os.MkdirAll("data", 0o755)
		fp := filepath.ToSlash(filepath.Join("data", "croupier.db"))
		dsn := "file:" + fp
		if db, err := gorm.Open(gsqlite.Open(dsn), &gorm.Config{}); err == nil {
			s.gdb = db
		}
	}
	if s.gdb != nil {
		// auto-migrate and prepare repos
		_ = games.AutoMigrate(s.gdb)
		s.games = games.NewRepo(s.gdb)
		_ = usersgorm.AutoMigrate(s.gdb)
		_ = msgsgorm.AutoMigrate(s.gdb)
		s.userRepo = usersgorm.New(s.gdb)
		s.msgRepo = msgsgorm.NewRepo(s.gdb)
		// support system
		{
			_ = support.AutoMigrate(s.gdb)
		}
		// monitoring: certificates
		{
			cs := certmonitor.NewStore(s.gdb)
			_ = cs.AutoMigrate()
			s.certStore = cs
		}
		// optional: import legacy JSON when empty DB (DEV bootstrap)
		_ = s.importLegacyUsersIfEmpty()
		_ = s.importLegacyGamesIfEmpty()
		// rebuild in-memory RBAC policy from DB roles/permissions
		_ = s.buildPolicyFromDB()
		// seed defaults if empty (dev/first boot)
		_ = s.seedDefaultsIfEmpty()
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
				if st, err := obj.OpenS3(context.Background(), s.objConf); err == nil {
					s.obj = st
				}
			case "file":
				if st, err := obj.OpenFile(context.Background(), s.objConf); err == nil {
					s.obj = st
				}
			case "oss":
				if st, err := obj.OpenOSS(context.Background(), s.objConf); err == nil {
					s.obj = st
				}
			case "cos":
				if st, err := obj.OpenCOS(context.Background(), s.objConf); err == nil {
					s.obj = st
				}
			}
		}
	}
	// Initialize component manager for managing function components
	s.componentMgr = pack.NewComponentManager("data")
	if err := s.componentMgr.LoadRegistry(); err != nil {
		log.Printf("[components] failed to load registry: %v", err)
	}
	// All routes are defined in Gin engine now; no legacy mux routes
	return s, nil
}

// normalizeMySQLDSN converts mysql:// URIs to go-sql-driver DSN when needed and ensures parseTime.
func normalizeMySQLDSN(dsn string) string {
	// already native DSN
	if !strings.HasPrefix(dsn, "mysql://") {
		if !strings.Contains(strings.ToLower(dsn), "parsetime=") { // append parseTime if missing
			sep := "?"
			if strings.Contains(dsn, "?") {
				sep = "&"
			}
			dsn = dsn + sep + "parseTime=true"
		}
		return dsn
	}
	// Convert mysql://user:pass@host:port/db?params -> user:pass@tcp(host:port)/db?params
	u, err := url.Parse(dsn)
	if err != nil {
		return dsn
	}
	user := ""
	pass := ""
	if u.User != nil {
		user = u.User.Username()
		pass, _ = u.User.Password()
	}
	host := u.Host
	db := strings.TrimPrefix(u.Path, "/")
	q := u.RawQuery
	if !strings.Contains(strings.ToLower(q), "parsetime=") {
		if q == "" {
			q = "parseTime=true"
		} else {
			q = q + "&parseTime=true"
		}
	}
	auth := user
	if pass != "" {
		auth = auth + ":" + pass
	}
	if auth != "" {
		auth = auth + "@"
	}
	return fmt.Sprintf("%stcp(%s)/%s?%s", auth, host, db, q)
}

// metrics toggles and per-function structures
var metricsPerFunction atomic.Bool

// --- recent jobs helpers ---
func (s *Server) jobsAdd(id, fid, actor, gid, env, trace string) {
    if id == "" { return }
    ji := &jobInfo{ID: id, FunctionID: fid, Actor: actor, GameID: gid, Env: env, State: "running", StartedAt: time.Now(), TraceID: trace}
    if s.locator != nil {
        if addr, ok := s.locator.GetJobAddr(id); ok { ji.RPCAddr = addr }
    }
    s.jobsMu.Lock()
    if s.jobs == nil { s.jobs = map[string]*jobInfo{} }
    s.jobs[id] = ji
    s.jobsOrder = append(s.jobsOrder, id)
    if len(s.jobsOrder) > 1000 {
        oldest := s.jobsOrder[0]
        s.jobsOrder = s.jobsOrder[1:]
        delete(s.jobs, oldest)
    }
    s.jobsMu.Unlock()
}

func (s *Server) jobsSetState(id, state, errMsg string) {
    if id == "" { return }
    s.jobsMu.Lock()
    if s.jobs == nil { s.jobs = map[string]*jobInfo{} }
    ji := s.jobs[id]
    if ji == nil {
        ji = &jobInfo{ID: id, State: state}
        s.jobs[id] = ji
        s.jobsOrder = append(s.jobsOrder, id)
    }
    ji.State = state
    if errMsg != "" { ji.Error = errMsg }
    if s.locator != nil && ji.RPCAddr == "" {
        if addr, ok := s.locator.GetJobAddr(id); ok { ji.RPCAddr = addr }
    }
    if state == "succeeded" || state == "failed" || state == "canceled" {
        if ji.EndedAt.IsZero() { ji.EndedAt = time.Now() }
        if ji.StartedAt.IsZero() { ji.DurationMs = 0 } else { ji.DurationMs = ji.EndedAt.Sub(ji.StartedAt).Milliseconds() }
    }
    s.jobsMu.Unlock()
}

// pickFnRate chooses function-level QPS considering advanced rules with match and percent.
func (s *Server) pickFnRate(fid, gid, env, trace string) int {
    s.mu.RLock(); defer s.mu.RUnlock()
    best := 0
    bestScore := -1
    if len(s.fnRulesAdv) > 0 {
        for _, rr := range s.fnRulesAdv {
            if rr.Scope != "function" || rr.Key != fid { continue }
            sc := 0
            if rr.Match != nil {
                if v:=strings.TrimSpace(rr.Match["game_id"]); v != "" { if v!=gid { continue } ; sc++ }
                if v:=strings.TrimSpace(rr.Match["env"]); v != "" { if v!=env { continue } ; sc++ }
            }
            if sc > bestScore { best = rr.LimitQPS; bestScore = sc }
        }
        if best > 0 {
            // apply percent by hashing trace id
            pct := 100
            for _, rr := range s.fnRulesAdv { if rr.Scope=="function" && rr.Key==fid { pct = rr.Percent; if pct<=0 { pct=100 }; break } }
            if pct < 100 {
                sid := trace
                if sid == "" { sid = fid+"|"+gid+"|"+env }
                h := sha256.Sum256([]byte(sid))
                if int(h[0])%100 >= pct { return 0 }
            }
            return best
        }
    }
    // fallback simple map
    if v := s.rateLimitRules[fid]; v > 0 { return v }
    return 0
}
var metricsPerGameDenies atomic.Bool

func SetMetricsOptions(perFunction bool, perGameDenies bool) {
	metricsPerFunction.Store(perFunction)
	metricsPerGameDenies.Store(perGameDenies)
}

// Approval struct moved to approvals package

type histogram struct {
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
		if err != nil {
			return
		}
		_, _ = h.Write([]byte(rel))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write(b)
		_, _ = h.Write([]byte{0})
	}
	// manifest
	writeFile("manifest.json")
	// descriptors and ui
	_ = filepath.Walk(filepath.Join(packDir, "descriptors"), func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".json" {
			return nil
		}
		rel, _ := filepath.Rel(packDir, path)
		writeFile(rel)
		return nil
	})
	_ = filepath.Walk(filepath.Join(packDir, "ui"), func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".json" {
			return nil
		}
		rel, _ := filepath.Rel(packDir, path)
		writeFile(rel)
		return nil
	})
	// web-plugin js
	_ = filepath.Walk(filepath.Join(packDir, "web-plugin"), func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".js" {
			return nil
		}
		rel, _ := filepath.Rel(packDir, path)
		writeFile(rel)
		return nil
	})
	// root *.pb
	_ = filepath.Walk(packDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if filepath.Dir(path) != packDir {
			return nil
		}
		if filepath.Ext(path) != ".pb" {
			return nil
		}
		rel, _ := filepath.Rel(packDir, path)
		writeFile(rel)
		return nil
	})
	sum := h.Sum(nil)
	return hex.EncodeToString(sum)
}
func newHistogram() *histogram {
	b := []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}
	return &histogram{buckets: b, counts: make([]int64, len(b))}
}
func (h *histogram) observe(sec float64) {
	h.count++
	h.sum += sec
	for i, le := range h.buckets {
		if sec <= le {
			h.counts[i]++
			break
		}
	}
}
func (h *histogram) approxQuantile(q float64) float64 {
	if h.count == 0 {
		return 0
	}
	target := int64(float64(h.count) * q)
	if target <= 0 {
		target = 1
	}
	var cum int64
	for i, c := range h.counts {
		cum += c
		if cum >= target {
			return h.buckets[i]
		}
	}
	return h.buckets[len(h.buckets)-1]
}

type fnMetrics struct {
	invocations  int64
	errors       int64
	rbacDenied   int64
	hist         *histogram
	deniesByGame map[string]int64
}

func (s *Server) getFn(id string) *fnMetrics {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := s.fn[id]
	if m == nil {
		m = &fnMetrics{hist: newHistogram(), deniesByGame: map[string]int64{}}
		s.fn[id] = m
	}
	return m
}

// loggingResponseWriter wraps ResponseWriter to capture status and bytes.
type loggingResponseWriter struct {
	http.ResponseWriter
	status int
	nbytes int
}

func (w *loggingResponseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
func (w *loggingResponseWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.nbytes += n
	return n, err
}

// loggingHandler logs each HTTP request with status and latency, capturing panics.
func (s *Server) loggingHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lrw := &loggingResponseWriter{ResponseWriter: w}
		// recover panics
		defer func() {
			if rec := recover(); rec != nil {
				lrw.WriteHeader(http.StatusInternalServerError)
				slog.Error("http panic", "method", r.Method, "path", r.URL.Path, "panic", rec, "stack", string(debug.Stack()))
			}
			dur := time.Since(start)
			user, _, _ := s.auth(r) // best-effort
			lvl := slog.LevelInfo
			if lrw.status >= 500 {
				lvl = slog.LevelError
			} else if lrw.status >= 400 {
				lvl = slog.LevelWarn
			}
			slog.Log(r.Context(), lvl, "http",
				"method", r.Method,
				"path", r.URL.Path,
				"status", lrw.status,
				"bytes", lrw.nbytes,
				"remote", r.RemoteAddr,
				"user", user,
				"dur_ms", dur.Milliseconds(),
			)
		}()
		next.ServeHTTP(lrw, r)
	})
}

// gin middlewares
func (s *Server) ginCORS() gin.HandlerFunc {
    return func(c *gin.Context) {
        w := c.Writer
        r := c.Request
        // Read config from env; safe defaults for dev
        allowOrigins := strings.TrimSpace(os.Getenv("CORS_ALLOW_ORIGINS")) // e.g. "https://a.com,https://b.com" or "*"
        allowHeaders := strings.TrimSpace(os.Getenv("CORS_ALLOW_HEADERS"))
        allowMethods := strings.TrimSpace(os.Getenv("CORS_ALLOW_METHODS"))
        allowCreds := strings.EqualFold(strings.TrimSpace(os.Getenv("CORS_ALLOW_CREDENTIALS")), "true") || os.Getenv("CORS_ALLOW_CREDENTIALS") == "1"

        // Defaults
        if allowHeaders == "" { allowHeaders = "Content-Type, Authorization, X-Game-ID, X-Env" }
        if allowMethods == "" { allowMethods = "GET, POST, PUT, DELETE, OPTIONS" }

        originHdr := r.Header.Get("Origin")
        if allowOrigins == "*" || allowOrigins == "" {
            // Dev default; but when credentials are allowed, echo back concrete Origin per spec
            if allowCreds && originHdr != "" {
                w.Header().Set("Access-Control-Allow-Origin", originHdr)
                w.Header().Add("Vary", "Origin")
            } else {
                w.Header().Set("Access-Control-Allow-Origin", "*")
            }
        } else {
            // Match against allowlist
            allowed := map[string]struct{}{}
            for _, o := range strings.Split(allowOrigins, ",") {
                o = strings.TrimSpace(o)
                if o != "" { allowed[o] = struct{}{} }
            }
            if originHdr != "" {
                if _, ok := allowed[originHdr]; ok {
                    w.Header().Set("Access-Control-Allow-Origin", originHdr)
                    w.Header().Add("Vary", "Origin")
                }
            }
        }
        w.Header().Set("Access-Control-Allow-Headers", allowHeaders)
        w.Header().Set("Access-Control-Allow-Methods", allowMethods)
        if allowCreds { w.Header().Set("Access-Control-Allow-Credentials", "true") }
        if r.Method == http.MethodOptions {
            c.Status(http.StatusNoContent)
            c.Abort()
            return
        }
        c.Next()
    }
}

// ginReqID injects/propagates an X-Request-ID for traceability.
func (s *Server) ginReqID() gin.HandlerFunc {
    return func(c *gin.Context) {
        rid := c.Request.Header.Get("X-Request-ID")
        if strings.TrimSpace(rid) == "" {
            // 16-byte random hex id
            b := make([]byte, 16)
            if _, err := rand.Read(b); err == nil {
                rid = hex.EncodeToString(b)
            } else {
                rid = fmt.Sprintf("%d", time.Now().UnixNano())
            }
        }
        c.Set("reqid", rid)
        c.Writer.Header().Set("X-Request-ID", rid)
        c.Next()
    }
}

// respondError sends a unified JSON error body.
func (s *Server) respondError(c *gin.Context, status int, code, message string) {
    type errBody struct {
        Code      string `json:"code"`
        Message   string `json:"message"`
        RequestID string `json:"request_id,omitempty"`
    }
    // Map常见错误到中文，以便直接呈现；对于具体错误细节（如 err.Error()），保持原样
    zh := map[string]string{
        "unauthorized":        "未授权",
        "forbidden":           "无权限",
        "bad_request":         "请求参数无效",
        "internal_error":      "服务器内部错误",
        "not_found":           "资源不存在",
        "unavailable":         "服务不可用",
        "conflict":            "资源冲突",
        "rate_limited":        "请求过于频繁",
        "method_not_allowed":  "方法不被允许",
        "not_implemented":     "未实现",
        "bad_gateway":         "上游服务错误",
        "request_too_large":   "请求体过大",
    }
    // 仅在 message 看起来是占位提示或为空时，采用中文映射
    if v, ok := zh[code]; ok {
        switch strings.ToLower(strings.TrimSpace(message)) {
        case "", "unauthorized", "forbidden", "bad request", "internal error", "not found", "service unavailable", "conflict", "too many login attempts", "method not allowed", "not implemented", "bad gateway", "request too large", "invalid payload":
            message = v
        }
    }
    // 内部错误：若携带具体错误信息，则在前面增加中文前缀，便于用户理解
    if code == "internal_error" {
        m := strings.TrimSpace(message)
        if m != "" && !strings.HasPrefix(m, "服务器内部错误") {
            message = "服务器内部错误：" + m
        }
    }
    rid, _ := c.Get("reqid")
    c.JSON(status, errBody{Code: code, Message: message, RequestID: fmt.Sprint(rid)})
}

// require checks that the current request is authenticated and has any of the provided permissions.
// Returns (user, roles, true) on success; otherwise it writes the error and returns false.
func (s *Server) require(c *gin.Context, anyOf ...string) (string, []string, bool) {
    user, roles, ok := s.auth(c.Request)
    if !ok {
        s.respondError(c, http.StatusUnauthorized, "unauthorized", "unauthorized")
        return "", nil, false
    }
    if len(anyOf) == 0 {
        return user, roles, true
    }
    for _, p := range anyOf {
        if s.can(user, roles, p) {
            return user, roles, true
        }
    }
    s.respondError(c, http.StatusForbidden, "forbidden", "forbidden")
    return user, roles, false
}

// allowLogin performs simple in-memory rate limiting for login attempts per ip|username.
func (s *Server) allowLogin(ip, username string) bool {
    key := fmt.Sprintf("%s|%s", strings.TrimSpace(ip), strings.TrimSpace(username))
    now := time.Now()
    window := now.Add(-5 * time.Minute)
    s.loginMu.Lock()
    defer s.loginMu.Unlock()
    arr := s.loginAttempts[key]
    // keep only attempts within window
    kept := arr[:0]
    for _, t := range arr {
        if t.After(window) {
            kept = append(kept, t)
        }
    }
    if len(kept) >= 10 { // max 10 attempts per 5 minutes
        s.loginAttempts[key] = kept
        return false
    }
    kept = append(kept, now)
    s.loginAttempts[key] = kept
    return true
}

func (s *Server) ginLogger() gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()
        c.Next()
        dur := time.Since(start)
        user, _, _ := s.auth(c.Request)
        lvl := slog.LevelInfo
        st := c.Writer.Status()
		if st >= 500 {
			lvl = slog.LevelError
		} else if st >= 400 {
			lvl = slog.LevelWarn
		}
        rid, _ := c.Get("reqid")
        slog.Log(c, lvl, "http",
            "method", c.Request.Method,
            "path", c.Request.URL.Path,
            "status", st,
            "bytes", c.Writer.Size(),
            "remote", c.ClientIP(),
            "user", user,
            "reqid", rid,
            "dur_ms", dur.Milliseconds(),
        )
    }
}

// ipRegion returns a human-readable region string for an IP (e.g., "CN/Zhejiang/Hangzhou") using optional HTTP resolver.
// Results are cached in-memory with a short TTL; private/local addresses map to "内网".
func (s *Server) ipRegion(ip string) string {
    ip = cleanIP(strings.TrimSpace(ip))
    if ip == "" { return "" }
    // quick local/LAN detection
    if isLoopbackIP(ip) { return "本地" }
    if isLANIP(ip) { return "局域网" }
    // cache
    s.ipRegionMu.RLock()
    if ent, ok := s.ipRegionCache[ip]; ok && time.Now().Before(ent.exp) {
        s.ipRegionMu.RUnlock()
        return ent.val
    }
    s.ipRegionMu.RUnlock()
    // Optional offline DB via ip2location (enabled with build tag)
    if v := ip2locRegion(s, ip); v != "" {
        s.ipRegionMu.Lock()
        s.ipRegionCache[ip] = struct{ val string; exp time.Time }{val: v, exp: time.Now().Add(24 * time.Hour)}
        s.ipRegionMu.Unlock()
        return v
    }
    if s.geoIPHTTP == "" { return "" }
    // build URL
    url := s.geoIPHTTP
    if strings.Contains(url, "{{ip}}") || strings.Contains(url, "{ip}") {
        url = strings.ReplaceAll(url, "{{ip}}", ip)
        url = strings.ReplaceAll(url, "{ip}", ip)
    } else {
        if strings.Contains(url, "?") { url += "&ip=" + ip } else { url += "?ip=" + ip }
    }
    // do request with timeout
    ctx, cancel := context.WithTimeout(context.Background(), s.geoIPTimeout)
    defer cancel()
    req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
    resp, err := http.DefaultClient.Do(req)
    if err != nil || resp == nil || resp.Body == nil {
        return ""
    }
    defer resp.Body.Close()
    var m map[string]any
    if err := json.NewDecoder(resp.Body).Decode(&m); err != nil { return "" }
    pick := func(keys ...string) string {
        for _, k := range keys {
            if v, ok := m[k]; ok {
                if s, ok := v.(string); ok && strings.TrimSpace(s) != "" { return s }
            }
        }
        return ""
    }
    country := pick("country_name", "country")
    region := pick("region_name", "region", "province", "state")
    city := pick("city")
    parts := []string{}
    if country != "" { parts = append(parts, country) }
    if region != "" { parts = append(parts, region) }
    if city != "" { parts = append(parts, city) }
    res := strings.Join(parts, "/")
    if res == "" { return "" }
    // cache for 24h
    s.ipRegionMu.Lock()
    s.ipRegionCache[ip] = struct{ val string; exp time.Time }{val: res, exp: time.Now().Add(24 * time.Hour)}
    s.ipRegionMu.Unlock()
    return res
}

func isPrivateIP(ipStr string) bool {
    ip := net.ParseIP(ipStr)
    if ip == nil { return false }
    if ip.IsLoopback() { return true }
    // IPv4 ranges
    v4 := ip.To4()
    if v4 != nil {
        b0, b1 := v4[0], v4[1]
        switch {
        case b0 == 10:
            return true
        case b0 == 172 && b1 >= 16 && b1 <= 31:
            return true
        case b0 == 192 && b1 == 168:
            return true
        case b0 == 169 && b1 == 254:
            return true // link-local
        case b0 == 127:
            return true
        }
        return false
    }
    // IPv6 private: fc00::/7 and loopback ::1
    if ip.IsLoopback() { return true }
    if ip[0]&0xfe == 0xfc { return true }
    return false
}

func isLoopbackIP(ipStr string) bool {
    ip := net.ParseIP(cleanIP(ipStr))
    if ip == nil { return false }
    return ip.IsLoopback()
}

func isLANIP(ipStr string) bool {
    ip := net.ParseIP(cleanIP(ipStr))
    if ip == nil { return false }
    if ip.IsLoopback() { return false }
    // IPv4 private ranges and link-local
    if v4 := ip.To4(); v4 != nil {
        b0, b1 := v4[0], v4[1]
        switch {
        case b0 == 10:
            return true
        case b0 == 172 && b1 >= 16 && b1 <= 31:
            return true
        case b0 == 192 && b1 == 168:
            return true
        case b0 == 169 && b1 == 254:
            return true // link-local
        }
        return false
    }
    // IPv6 ULA fc00::/7 and link-local fe80::/10 considered local network
    // ULA: fc00::/7 => first 7 bits 1111110x, check top 7 bits via ip[0]&0xfe == 0xfc
    if ip[0]&0xfe == 0xfc { return true }
    // fe80::/10 => first 10 bits 1111111010, quick check high byte 0xfe and next high bits 0x80..0xbf
    if ip[0] == 0xfe && (ip[1]&0xc0) == 0x80 { return true }
    return false
}

// cleanIP normalizes an IP string by removing brackets, ports and zone identifiers.
func cleanIP(s string) string {
    t := strings.TrimSpace(s)
    if t == "" { return "" }
    // Trim IPv6 literal with port: [::1]:1234
    if strings.HasPrefix(t, "[") {
        if h, _, err := net.SplitHostPort(t); err == nil {
            return strings.Trim(h, "[]")
        }
        // no port, just [::1]
        t = strings.TrimPrefix(t, "[")
        if i := strings.IndexByte(t, ']'); i >= 0 { t = t[:i] }
        return t
    }
    // Strip zone id, e.g., fe80::1%lo0
    if i := strings.IndexByte(t, '%'); i >= 0 {
        t = t[:i]
    }
    // IPv4 with port: 127.0.0.1:8080
    if h, _, err := net.SplitHostPort(t); err == nil && net.ParseIP(h) != nil {
        return h
    }
    return t
}

// seedDefaultsIfEmpty creates default roles and an admin user when tables are empty.
func (s *Server) seedDefaultsIfEmpty() error {
	if s.userRepo == nil {
		return nil
	}
    // Ensure admin role exists
    var admin usersgorm.RoleRecord
    if err := s.gdb.Where("name = ?", "admin").First(&admin).Error; err != nil {
        admin = usersgorm.RoleRecord{Name: "admin", Description: "Administrator"}
        if err2 := s.gdb.Create(&admin).Error; err2 == nil {
            _ = s.userRepo.GrantRolePerm(context.Background(), admin.ID, "*")
        }
    }

    // Check if admin user exists and has password
    var adminUser usersgorm.UserAccount
    err := s.gdb.Where("username = ?", "admin").First(&adminUser).Error
    if err != nil {
        // No admin user, create one
        slog.Info("No admin user found, creating default admin user")
        u := &usersgorm.UserAccount{Username: "admin", DisplayName: "Administrator", Active: true}
        if err := s.userRepo.CreateUser(context.Background(), u); err == nil {
            _ = s.userRepo.SetPassword(context.Background(), u.ID, "admin")
            _ = s.userRepo.AddUserRole(context.Background(), u.ID, admin.ID)
            slog.Info("Default admin user created", "username", "admin", "password", "admin")
        } else {
            slog.Error("Failed to create default admin user", "error", err)
        }
    } else if adminUser.PasswordHash == "" {
        // Admin user exists but has no password
        slog.Info("Admin user exists but has no password, setting default password")
        if err := s.userRepo.SetPassword(context.Background(), adminUser.ID, "admin"); err == nil {
            // Ensure admin has admin role
            _ = s.userRepo.AddUserRole(context.Background(), adminUser.ID, admin.ID)
            slog.Info("Admin user password set", "username", "admin", "password", "admin")
        } else {
            slog.Error("Failed to set admin user password", "error", err)
        }
    }
	return nil
}

// importLegacyUsersIfEmpty imports users and roles from JSON files when DB tables are empty.
func (s *Server) importLegacyUsersIfEmpty() error {
	if s.gdb == nil || s.userRepo == nil {
		return nil
	}
	var cnt int64
    if err := s.gdb.Model(&usersgorm.UserAccount{}).Count(&cnt).Error; err != nil {
		return err
	}
	if cnt > 0 {
		return nil
	}
	// Try configs/users.json
	b, err := os.ReadFile("configs/users.json")
	if err != nil {
		slog.Info("import users skipped: configs/users.json not found")
		return nil
	}
    var in []struct {
        Username      string   `json:"username"`
        Roles         []string `json:"roles"`
        Perms         []string `json:"perms"`
        DisplayName   string   `json:"display_name"`
        Email         string   `json:"email"`
        Phone         string   `json:"phone"`
        OTPSecret     string   `json:"otp_secret"`
        Password      string   `json:"password"`        // optional plain password to set
        PasswordBcrypt string  `json:"password_bcrypt"` // optional pre-bcrypted hash (wins over Password)
        Active        *bool    `json:"active"`
    }
	if err := json.Unmarshal(b, &in); err != nil {
		slog.Warn("import users failed: json parse error", "error", err)
		return nil
	}
	slog.Info("importing users from json", "path", "configs/users.json", "count", len(in))
    for _, u := range in {
        act := true
        if u.Active != nil { act = *u.Active }
        ur := &usersgorm.UserAccount{Username: u.Username, DisplayName: u.DisplayName, Email: u.Email, Phone: u.Phone, Active: act, OTPSecret: u.OTPSecret}
        if err := s.userRepo.CreateUser(context.Background(), ur); err != nil {
            slog.Warn("create user failed", "username", u.Username, "error", err)
            continue
        }
        slog.Info("user created", "username", u.Username)
        // set password if provided
        if strings.TrimSpace(u.PasswordBcrypt) != "" {
            if err := s.gdb.WithContext(context.Background()).Model(&usersgorm.UserAccount{}).Where("id=?", ur.ID).Update("password_hash", u.PasswordBcrypt).Error; err != nil {
                slog.Warn("set bcrypt password failed", "username", u.Username, "error", err)
            } else {
                slog.Info("password (bcrypt) set", "username", u.Username)
            }
        } else if strings.TrimSpace(u.Password) != "" {
            if err := s.userRepo.SetPassword(context.Background(), ur.ID, u.Password); err != nil {
                slog.Warn("set password failed", "username", u.Username, "error", err)
            } else {
                slog.Info("password set", "username", u.Username)
            }
        } else if def := strings.TrimSpace(os.Getenv("CROUPIER_IMPORT_DEFAULT_PASSWORD")); def != "" {
            if err := s.userRepo.SetPassword(context.Background(), ur.ID, def); err != nil {
                slog.Warn("set default password failed", "username", u.Username, "error", err)
            } else {
                slog.Info("password set (default)", "username", u.Username)
            }
        } else {
            slog.Warn("user imported without password", "username", u.Username)
        }
		// attach roles
		for _, rn := range u.Roles {
			var role usersgorm.RoleRecord
			if err := s.gdb.Where("name = ?", rn).First(&role).Error; err != nil {
				role = usersgorm.RoleRecord{Name: rn}
				if err := s.gdb.Create(&role).Error; err != nil {
					slog.Warn("create role failed during import", "role", rn, "error", err)
					continue
				}
				slog.Info("role created during import", "role", rn)
			}
			if err := s.userRepo.AddUserRole(context.Background(), ur.ID, role.ID); err != nil {
				slog.Warn("attach role failed", "username", u.Username, "role", rn, "error", err)
			} else {
				slog.Debug("role attached", "username", u.Username, "role", rn)
			}
		}
		// attach direct perms via a per-user role: user:<username>
		if len(u.Perms) > 0 {
			rname := "user:" + u.Username
			var role usersgorm.RoleRecord
			if err := s.gdb.Where("name = ?", rname).First(&role).Error; err != nil {
				role = usersgorm.RoleRecord{Name: rname}
				if err := s.gdb.Create(&role).Error; err != nil {
					slog.Warn("create per-user role failed", "role", rname, "error", err)
				} else {
					slog.Info("per-user role created", "role", rname)
				}
			}
			if err := s.userRepo.AddUserRole(context.Background(), ur.ID, role.ID); err != nil {
				slog.Warn("attach per-user role failed", "username", u.Username, "role", rname, "error", err)
			}
			for _, p := range u.Perms {
				if err := s.userRepo.GrantRolePerm(context.Background(), role.ID, p); err != nil {
					slog.Warn("grant perm failed", "role", rname, "perm", p, "error", err)
				} else {
					slog.Debug("perm granted", "role", rname, "perm", p)
				}
			}
		}
	}
	slog.Info("users import completed")
	return nil
}

// importLegacyGamesIfEmpty imports allowed envs from configs/games.json by creating Game rows with Name=legacy game_id.
func (s *Server) importLegacyGamesIfEmpty() error {
	if s.gdb == nil || s.games == nil {
		return nil
	}
	var cnt int64
	if err := s.gdb.Model(&games.Game{}).Count(&cnt).Error; err != nil {
		return err
	}
	if cnt > 0 {
		return nil
	}
	b, err := os.ReadFile("configs/games.json")
	if err != nil {
		return nil
	}
	var data struct {
		Games []struct {
			GameID string `json:"game_id"`
			Env    string `json:"env"`
		} `json:"games"`
	}
	if err := json.Unmarshal(b, &data); err != nil {
		return nil
	}
	// group envs by legacy id
	m := map[string][]string{}
	for _, e := range data.Games {
		if e.GameID != "" {
			m[e.GameID] = append(m[e.GameID], e.Env)
		}
	}
	for name, envs := range m {
		g := &games.Game{Name: name, Enabled: true}
		if err := s.games.Create(context.Background(), g); err != nil {
			continue
		}
		for _, env := range envs {
			if env != "" {
				_ = s.games.AddEnv(context.Background(), g.ID, env)
			}
		}
	}
	return nil
}

// buildPolicyFromDB rebuilds in-memory RBAC policy using role_perms table.
func (s *Server) buildPolicyFromDB() error {
    if s.userRepo == nil {
        return nil
    }
    // When using Casbin policy (file-based), keep it as authoritative and do not override.
    if _, ok := s.rbac.(*rbac.CasbinPolicy); ok {
        return nil
    }
	snaps, err := s.userRepo.BuildPolicySnapshot(context.Background())
	if err != nil {
		return err
	}
	p := rbac.NewPolicy()
	for role, perms := range snaps {
		for _, perm := range perms {
			p.Grant("role:"+role, perm)
		}
	}
	s.rbac = p
	return nil
}

// extractPack extracts a tar.gz pack into dest directory; keeps descriptors and fds
func extractPack(archive, dest string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		// Only extract descriptors/*.json, ui/*.json, manifest.json, web-plugin/* and any *.pb
		if strings.HasPrefix(hdr.Name, "descriptors/") || strings.HasPrefix(hdr.Name, "ui/") || strings.HasPrefix(hdr.Name, "web-plugin/") || hdr.Name == "manifest.json" || strings.HasSuffix(hdr.Name, ".pb") {
			// Normalize: strip leading "descriptors/" so files land directly under dest/
			name := filepath.FromSlash(hdr.Name)
			if strings.HasPrefix(name, "descriptors/") {
				name = strings.TrimPrefix(name, "descriptors/")
			}
			target := filepath.Join(dest, name)
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			w, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
			if err != nil {
				return err
			}
			if _, err := io.Copy(w, tr); err != nil {
				w.Close()
				return err
			}
			w.Close()
		}
	}
	return nil
}

// ginEngine builds a Gin engine and mounts selected routes natively; the rest fall back to mux via NoRoute.
func (s *Server) ginEngine() *gin.Engine {
    // Default mode; use Recovery + custom logger + CORS
    r := gin.New()
    // Attach global CORS, logging and unified RBAC middleware (Casbin-based when available)
    r.Use(s.ginReqID(), s.ginCORS(), s.ginLogger(), s.ginAuthZ(), gin.Recovery())
	// Native Gin routes for performance-sensitive or upload endpoints
        r.POST("/api/upload", func(c *gin.Context) {
            user, _, ok := s.require(c, "uploads:write")
            if !ok { return }
            if s.obj == nil {
                slog.Error("upload storage not available")
                s.respondError(c, http.StatusServiceUnavailable, "unavailable", "storage not available")
                return
            }
		constMax := int64(120 * 1024 * 1024)
        if cl := c.Request.Header.Get("Content-Length"); cl != "" {
            if n, err := strconv.ParseInt(cl, 10, 64); err == nil && n > constMax {
                s.respondError(c, http.StatusRequestEntityTooLarge, "request_too_large", "request too large")
                return
            }
        }
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, constMax)
            if err := c.Request.ParseMultipartForm(32 << 20); err != nil { slog.Error("upload parse form", "error", err); s.respondError(c, http.StatusBadRequest, "bad_request", "invalid multipart form"); return }
		f, fh, err := c.Request.FormFile("file")
            if err != nil { slog.Error("upload file missing", "error", err); s.respondError(c, http.StatusBadRequest, "bad_request", "missing file"); return }
		defer f.Close()
		ts := time.Now().UnixNano()
		name := fh.Filename
		key := fmt.Sprintf("%s/%d_%s", user, ts, name)
		tmp, err := os.CreateTemp("", "upload-*")
            if err != nil { slog.Error("upload temp", "error", err); s.respondError(c, http.StatusInternalServerError, "internal_error", "create temp file failed"); return }
		defer os.Remove(tmp.Name())
            if _, err := io.Copy(tmp, f); err != nil { tmp.Close(); slog.Error("upload copy", "error", err); s.respondError(c, http.StatusInternalServerError, "internal_error", "upload copy failed"); return }
            if _, err := tmp.Seek(0, io.SeekStart); err != nil { tmp.Close(); slog.Error("upload seek", "error", err); s.respondError(c, http.StatusInternalServerError, "internal_error", "upload seek failed"); return }
		ct := fh.Header.Get("Content-Type")
            if err := s.obj.Put(c, key, tmp, fh.Size, ct); err != nil { tmp.Close(); slog.Error("upload put", "error", err, "user", user, "key", key, "size", fh.Size, "ct", ct); s.respondError(c, http.StatusInternalServerError, "internal_error", "upload store failed"); return }
		_ = tmp.Close()
		url, err := s.obj.SignedURL(c, key, "GET", s.objConf.SignedURLTTL)
		if err != nil {
			slog.Error("upload signed url", "error", err, "key", key)
		}
		s.JSON(c, http.StatusOK, gin.H{"Key": key, "URL": url})
	})
	// Games routes
        r.GET("/api/games", func(c *gin.Context) {
            _, _, ok := s.require(c, "games:read", "games:manage")
            if !ok { return }
            items, err := s.games.List(c)
            if err != nil { s.respondError(c, 500, "internal_error", "failed to list games"); return }
    // Normalize time fields to RFC3339 (seconds precision) and keys to snake_case for clients
    type gv struct {
        ID          uint   `json:"id"`
        Name        string `json:"name"`
        Icon        string `json:"icon"`
        Description string `json:"description"`
        Enabled     bool   `json:"enabled"`
        AliasName   string `json:"alias_name"`
        Homepage    string `json:"homepage"`
        Status      string `json:"status"`
        CreatedAt   string `json:"created_at"`
        UpdatedAt   string `json:"updated_at"`
    }
    out := make([]gv, 0, len(items))
    for _, g := range items {
        out = append(out, gv{
            ID: g.ID,
            Name: g.Name,
            Icon: g.Icon,
            Description: g.Description,
            Enabled: g.Enabled,
            AliasName: g.AliasName,
            Homepage: g.Homepage,
            Status: g.Status,
            CreatedAt: g.CreatedAt.Format(time.RFC3339),
            UpdatedAt: g.UpdatedAt.Format(time.RFC3339),
        })
    }
    s.JSON(c, 200, gin.H{"games": out})
	})
        r.POST("/api/games", func(c *gin.Context) {
            _, _, ok := s.require(c, "games:manage")
            if !ok { return }
            var in struct{
                ID          uint   `json:"id"`
                Name        string `json:"name"`
                Icon        string `json:"icon"`
                Description string `json:"description"`
                Enabled     bool   `json:"enabled"`
                Status      string `json:"status"`
                AliasName   string `json:"alias_name"`
                Homepage    string `json:"homepage"`
            }
            if err := c.BindJSON(&in); err != nil { s.respondError(c, 400, "bad_request", "invalid payload"); return }
        if in.ID == 0 {
            // Default status to 'dev' if not provided; validate known lifecycle values
            status := strings.TrimSpace(in.Status)
            switch status {
            case "", "dev", "test", "running", "online", "offline", "maintenance":
                if status == "" { status = "dev" }
            default:
                // Unknown status -> treat as dev
                status = "dev"
            }
            g := &games.Game{
                Name:        in.Name,
                Icon:        in.Icon,
                Description: in.Description,
                Enabled:     in.Enabled,
                Status:      status,
                AliasName:   in.AliasName,
                Homepage:    in.Homepage,
            }
            if err := s.games.Create(c, g); err != nil { s.respondError(c, 500, "internal_error", "failed to create game"); return }
            c.Header("Location", fmt.Sprintf("/api/games/%d", g.ID))
            s.JSON(c, 201, gin.H{"id": g.ID})
        } else {
            g, err := s.games.Get(c, in.ID)
            if err != nil { s.respondError(c, 404, "not_found", "not found"); return }
            if in.Name != "" { g.Name = in.Name }
            g.Icon = in.Icon
            g.Description = in.Description
            g.Enabled = in.Enabled
            if in.Status != "" { g.Status = in.Status }
            g.AliasName = in.AliasName
            g.Homepage = in.Homepage
            if err := s.games.Update(c, g); err != nil { s.respondError(c, 500, "internal_error", "failed to update game"); return }
            c.Status(204)
        }
        })
        r.GET("/api/games/:id", func(c *gin.Context) {
            _, _, ok := s.require(c, "games:read", "games:manage")
            if !ok { return }
            id64, _ := strconv.ParseUint(c.Param("id"), 10, 64)
        g, err := s.games.Get(c, uint(id64))
        if err != nil { s.respondError(c, 404, "not_found", err.Error()); return }
        s.JSON(c, 200, gin.H{
        "id": g.ID,
        "name": g.Name,
        "icon": g.Icon,
        "description": g.Description,
        "enabled": g.Enabled,
        "alias_name": g.AliasName,
        "homepage": g.Homepage,
        "status": g.Status,
        "created_at": g.CreatedAt.Format(time.RFC3339),
        "updated_at": g.UpdatedAt.Format(time.RFC3339),
    })
	})
        r.PUT("/api/games/:id", func(c *gin.Context) {
            _, _, ok := s.require(c, "games:manage")
            if !ok { return }
		id64, _ := strconv.ParseUint(c.Param("id"), 10, 64)
    var in struct{
        Name        string `json:"name"`
        Icon        string `json:"icon"`
        Description string `json:"description"`
        Enabled     bool   `json:"enabled"`
        Status      string `json:"status"`
        AliasName   string `json:"alias_name"`
        Homepage    string `json:"homepage"`
    }
            if err := c.BindJSON(&in); err != nil { s.respondError(c, 400, "bad_request", "invalid payload"); return }
            g, err := s.games.Get(c, uint(id64))
            if err != nil { s.respondError(c, 404, "not_found", "game not found"); return }
        if in.Name != "" { g.Name = in.Name }
        g.Icon, g.Description, g.Enabled = in.Icon, in.Description, in.Enabled
        if in.Status != "" { g.Status = in.Status }
        g.AliasName = in.AliasName
        g.Homepage = in.Homepage
            if err := s.games.Update(c, g); err != nil { s.respondError(c, 500, "internal_error", "failed to update game"); return }
            c.Status(204)
        })
        r.DELETE("/api/games/:id", func(c *gin.Context) {
            _, _, ok := s.require(c, "games:manage")
            if !ok { return }
            id64, _ := strconv.ParseUint(c.Param("id"), 10, 64)
            if err := s.games.Delete(c, uint(id64)); err != nil { s.respondError(c, 500, "internal_error", "failed to delete game"); return }
            c.Status(204)
        })
    r.GET("/api/games/:id/envs", func(c *gin.Context) {
        _, _, ok := s.require(c, "games:read", "games:manage")
        if !ok { return }
        id64, _ := strconv.ParseUint(c.Param("id"), 10, 64)
        envs, err := s.games.ListEnvRecords(c, uint(id64))
        if err != nil { s.respondError(c, 500, "internal_error", "failed to list envs"); return }
        // Map to lean response for FE
        type envOut struct{ ID uint `json:"id"`; Env string `json:"env"`; Description string `json:"description"` }
        out := make([]envOut, 0, len(envs))
        for _, e := range envs { out = append(out, envOut{ID: e.ID, Env: e.Env, Description: e.Description}) }
        s.JSON(c, 200, gin.H{"envs": out})
    })
    r.POST("/api/games/:id/envs", func(c *gin.Context) {
        _, _, ok := s.require(c, "games:manage")
        if !ok { return }
        id64, _ := strconv.ParseUint(c.Param("id"), 10, 64)
        var in struct{ Env, Description string }
        if err := c.BindJSON(&in); err != nil { s.respondError(c, 400, "bad_request", "invalid payload"); return }
        name := strings.TrimSpace(in.Env)
        if name == "" { s.respondError(c, 400, "bad_request", "invalid env"); return }
        if err := s.games.AddEnvWithDesc(c, uint(id64), name, strings.TrimSpace(in.Description)); err != nil { s.respondError(c, 500, "internal_error", "failed to add env"); return }
        c.Status(204)
    })
    r.PUT("/api/games/:id/envs", func(c *gin.Context) {
        _, _, ok := s.require(c, "games:manage")
        if !ok { return }
        id64, _ := strconv.ParseUint(c.Param("id"), 10, 64)
        var in struct{ OldEnv, Env, Description string }
        if err := c.BindJSON(&in); err != nil { s.respondError(c, 400, "bad_request", "invalid payload"); return }
        oldEnv := strings.TrimSpace(in.OldEnv)
        if oldEnv == "" { s.respondError(c, 400, "bad_request", "missing old_env"); return }
        if err := s.games.UpdateEnv(c, uint(id64), oldEnv, strings.TrimSpace(in.Env), strings.TrimSpace(in.Description)); err != nil { s.respondError(c, 500, "internal_error", "failed to update env"); return }
        c.Status(204)
    })
    r.DELETE("/api/games/:id/envs", func(c *gin.Context) {
        _, _, ok := s.require(c, "games:manage")
        if !ok { return }
        id64, _ := strconv.ParseUint(c.Param("id"), 10, 64)
        if idStr := c.Query("id"); idStr != "" {
            if idv, err := strconv.ParseUint(idStr, 10, 64); err == nil {
                if err := s.games.RemoveEnvByID(c, uint(idv)); err != nil { s.respondError(c, 500, "internal_error", "failed to delete env"); return }
                c.Status(204)
                return
            }
        }
        env := c.Query("env")
        if env == "" { s.respondError(c, 400, "bad_request", "missing env"); return }
        if err := s.games.RemoveEnv(c, uint(id64), env); err != nil { s.respondError(c, 500, "internal_error", "failed to delete env"); return }
        c.Status(204)
    })

        // Auth
        r.POST("/api/auth/login", func(c *gin.Context) {
            if c.Request.Method != http.MethodPost {
                s.respondError(c, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
                return
            }
            if s.userRepo == nil || s.jwtMgr == nil {
                s.respondError(c, http.StatusServiceUnavailable, "auth_disabled", "auth disabled")
                return
            }
            var in struct{ Username, Password string }
            if err := c.BindJSON(&in); err != nil {
                s.respondError(c, http.StatusBadRequest, "bad_request", "invalid payload")
                return
            }
            ip := c.ClientIP()
            if !s.allowLogin(ip, in.Username) {
                s.respondError(c, http.StatusTooManyRequests, "rate_limited", "too many login attempts")
                if s.audit != nil { _ = s.audit.Log("login_rate_limited", in.Username, "auth", map[string]string{"ip": ip, "ua": c.Request.Header.Get("User-Agent")}) }
                return
            }
            ur, err := s.userRepo.Verify(c, in.Username, in.Password)
            if err != nil {
                // Avoid disclosing whether user exists; audit failure
                if s.audit != nil { _ = s.audit.Log("login_fail", in.Username, "auth", map[string]string{"ip": ip, "ua": c.Request.Header.Get("User-Agent")}) }
                s.respondError(c, http.StatusUnauthorized, "unauthorized", "invalid credentials")
                return
            }
            roles := []string{}
            if rs, err := s.userRepo.ListUserRoles(c, ur.ID); err == nil {
                for _, rr := range rs {
                    roles = append(roles, rr.Name)
                }
            }
            tok, _ := s.jwtMgr.Sign(in.Username, roles, 8*time.Hour)
            // Audit login event (ip, user-agent)
            if s.audit != nil {
                ua := c.Request.Header.Get("User-Agent")
            _ = s.audit.Log("login", in.Username, "auth", map[string]string{"ip": c.ClientIP(), "ua": ua})
            }
            s.JSON(c, 200, gin.H{"token": tok, "user": gin.H{"username": in.Username, "roles": roles}})
        })
        r.GET("/api/auth/me", func(c *gin.Context) {
            user, roles, ok := s.auth(c.Request)
            if !ok { s.respondError(c, 401, "unauthorized", "unauthorized"); return }
            s.JSON(c, 200, gin.H{"username": user, "roles": roles})
        })

	// Descriptors
        r.GET("/api/descriptors", func(c *gin.Context) { s.JSON(c, 200, s.descs) })
	r.GET("/healthz", func(c *gin.Context) { c.String(200, "ok") })
    r.GET("/metrics", func(c *gin.Context) {
        w := c.Writer
        w.Header().Set("Content-Type", "text/plain; version=0.0.4")

		// Basic counters
		fmt.Fprintf(w, "# HELP croupier_uptime_seconds Time since server started\n")
		fmt.Fprintf(w, "# TYPE croupier_uptime_seconds gauge\n")
		fmt.Fprintf(w, "croupier_uptime_seconds %d\n", int(time.Since(s.startedAt).Seconds()))

		fmt.Fprintf(w, "# HELP croupier_invocations_total Total number of function invocations\n")
		fmt.Fprintf(w, "# TYPE croupier_invocations_total counter\n")
		fmt.Fprintf(w, "croupier_invocations_total %d\n", atomic.LoadInt64(&s.invocations))

		fmt.Fprintf(w, "# HELP croupier_invocations_error_total Total number of failed invocations\n")
		fmt.Fprintf(w, "# TYPE croupier_invocations_error_total counter\n")
		fmt.Fprintf(w, "croupier_invocations_error_total %d\n", atomic.LoadInt64(&s.invocationsError))

		fmt.Fprintf(w, "# HELP croupier_jobs_started_total Total number of jobs started\n")
		fmt.Fprintf(w, "# TYPE croupier_jobs_started_total counter\n")
		fmt.Fprintf(w, "croupier_jobs_started_total %d\n", atomic.LoadInt64(&s.jobsStarted))

		fmt.Fprintf(w, "# HELP croupier_jobs_error_total Total number of job errors\n")
		fmt.Fprintf(w, "# TYPE croupier_jobs_error_total counter\n")
		fmt.Fprintf(w, "croupier_jobs_error_total %d\n", atomic.LoadInt64(&s.jobsError))

		fmt.Fprintf(w, "# HELP croupier_rbac_denied_total Total number of RBAC denials\n")
		fmt.Fprintf(w, "# TYPE croupier_rbac_denied_total counter\n")
		fmt.Fprintf(w, "croupier_rbac_denied_total %d\n", atomic.LoadInt64(&s.rbacDenied))

		fmt.Fprintf(w, "# HELP croupier_audit_errors_total Total number of audit errors\n")
		fmt.Fprintf(w, "# TYPE croupier_audit_errors_total counter\n")
		fmt.Fprintf(w, "croupier_audit_errors_total %d\n", atomic.LoadInt64(&s.auditErrors))
	})

	// UI schema
        r.GET("/api/ui_schema", func(c *gin.Context) {
            id := c.Query("id")
            if id == "" { s.respondError(c, 400, "bad_request", "missing id"); return }
		base := sanitize(id)
		schemaPath := filepath.Join(s.packDir, "ui", base+".schema.json")
		uiPath := filepath.Join(s.packDir, "ui", base+".uischema.json")
		var schema, uischema any
		if b, err := os.ReadFile(schemaPath); err == nil {
			_ = json.Unmarshal(b, &schema)
		}
		if b, err := os.ReadFile(uiPath); err == nil {
			_ = json.Unmarshal(b, &uischema)
		}
		s.JSON(c, 200, gin.H{"schema": schema, "uischema": uischema})
	})

    // Packs management
        r.POST("/api/packs/import", func(c *gin.Context) {
            // require manage permission for packs
            _, _, ok := s.require(c, "packs:reload")
            if !ok { return }
            if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
            s.respondError(c, 400, "bad_request", "invalid multipart form"); return
            }
            f, hdr, err := c.Request.FormFile("file")
            if err != nil { s.respondError(c, 400, "bad_request", "missing file"); return }
            defer f.Close()
            tmpPath := filepath.Join(os.TempDir(), hdr.Filename)
            out, err := os.Create(tmpPath)
            if err != nil { s.respondError(c, 500, "internal_error", "create temp file failed"); return }
            if _, err := io.Copy(out, f); err != nil { out.Close(); s.respondError(c, 500, "internal_error", "save temp file failed"); return }
            _ = out.Close()
            if err := extractPack(tmpPath, s.packDir); err != nil { s.respondError(c, 500, "internal_error", "extract pack failed"); return }
		if descs, err := descriptor.LoadAll(s.packDir); err == nil {
			idx := map[string]*descriptor.Descriptor{}
			for _, d := range descs {
				idx[d.ID] = d
			}
			s.descs = descs
			s.descIndex = idx
		}
		_ = s.typeReg.LoadFDSFromDir(s.packDir)
		s.JSON(c, 200, gin.H{"ok": true})
	})
        r.GET("/api/packs/list", func(c *gin.Context) {
            maniPath := filepath.Join(s.packDir, "manifest.json")
            b, err := os.ReadFile(maniPath)
            if err != nil { s.respondError(c, 404, "not_found", "manifest not found"); return }
		var mani any
		_ = json.Unmarshal(b, &mani)
		type counts struct {
			Descriptors int `json:"descriptors"`
			UISchema    int `json:"ui_schema"`
		}
		cts := counts{}
		_ = filepath.Walk(filepath.Join(s.packDir, "descriptors"), func(path string, info os.FileInfo, err error) error {
			if err != nil || info == nil || info.IsDir() {
				return nil
			}
			if filepath.Ext(path) == ".json" {
				cts.Descriptors++
			}
			return nil
		})
		_ = filepath.Walk(filepath.Join(s.packDir, "ui"), func(path string, info os.FileInfo, err error) error {
			if err != nil || info == nil || info.IsDir() {
				return nil
			}
			if filepath.Ext(path) == ".json" {
				cts.UISchema++
			}
			return nil
		})
		etag := computePackETag(s.packDir)
		s.JSON(c, 200, gin.H{"manifest": mani, "counts": cts, "etag": etag, "export_auth_required": s.packsExportRequireAuth})
	})
        r.GET("/api/packs/export", func(c *gin.Context) {
            // When PACKS_EXPORT_REQUIRE_AUTH is enabled, unified RBAC middleware will handle authz.
        if et := computePackETag(s.packDir); et != "" {
            c.Writer.Header().Set("ETag", et)
        }
        c.Writer.Header().Set("Content-Type", "application/gzip")
		c.Writer.Header().Set("Content-Disposition", "attachment; filename=pack.tgz")
		gz := gzip.NewWriter(c.Writer)
		defer gz.Close()
		tw := tar.NewWriter(gz)
		defer tw.Close()
		filepath.Walk(s.packDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info == nil || info.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(s.packDir, path)
			if !(strings.HasPrefix(rel, "descriptors/") || strings.HasPrefix(rel, "ui/") || strings.HasPrefix(rel, "web-plugin/") || rel == "manifest.json" || filepath.Ext(rel) == ".pb") {
				return nil
			}
			hdr, _ := tar.FileInfoHeader(info, "")
			hdr.Name = filepath.ToSlash(rel)
			if err := tw.WriteHeader(hdr); err != nil {
				return err
			}
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			_, _ = io.Copy(tw, f)
			_ = f.Close()
			return nil
		})
	})
    r.POST("/api/packs/reload", func(c *gin.Context) {
        if descs, err := descriptor.LoadAll(s.packDir); err == nil {
            idx := map[string]*descriptor.Descriptor{}
            for _, d := range descs {
                idx[d.ID] = d
            }
            s.descs = descs
            s.descIndex = idx
        }
        _ = s.typeReg.LoadFDSFromDir(s.packDir)
        s.JSON(c, 200, gin.H{"ok": true})
    })

	// Function Components Management
        r.GET("/api/components", func(c *gin.Context) {
            _, _, ok := s.require(c, "components:read")
            if !ok { return }

		category := c.Query("category")
		var result any

		if category != "" {
			result = s.componentMgr.ListByCategory(category)
		} else {
			result = gin.H{
				"installed": s.componentMgr.ListInstalled(),
				"disabled":  s.componentMgr.ListDisabled(),
			}
		}

		s.JSON(c, 200, gin.H{"components": result})
	})

        r.POST("/api/components/install", func(c *gin.Context) {
            _, _, ok := s.require(c, "components:install")
            if !ok { return }

        if err := c.Request.ParseMultipartForm(32 << 20); err != nil { s.respondError(c, 400, "bad_request", err.Error()); return }

		f, hdr, err := c.Request.FormFile("file")
        if err != nil { s.respondError(c, 400, "bad_request", "missing file"); return }
		defer f.Close()

		tmpPath := filepath.Join(os.TempDir(), hdr.Filename)
		out, err := os.Create(tmpPath)
        if err != nil { s.respondError(c, 500, "internal_error", err.Error()); return }

        if _, err := io.Copy(out, f); err != nil { out.Close(); s.respondError(c, 500, "internal_error", err.Error()); return }
		_ = out.Close()

        if err := extractPack(tmpPath, "components/staging"); err != nil { s.respondError(c, 500, "internal_error", err.Error()); return }

        if err := s.componentMgr.InstallComponent("components/staging"); err != nil { s.respondError(c, 500, "internal_error", err.Error()); return }

		s.JSON(c, 200, gin.H{"ok": true})
	})

        r.DELETE("/api/components/:id", func(c *gin.Context) {
            _, _, ok := s.require(c, "components:uninstall")
            if !ok { return }

		componentID := c.Param("id")
        if err := s.componentMgr.UninstallComponent(componentID); err != nil { s.respondError(c, 500, "internal_error", err.Error()); return }

		s.JSON(c, 200, gin.H{"ok": true})
	})

        r.POST("/api/components/:id/enable", func(c *gin.Context) {
            _, _, ok := s.require(c, "components:manage")
            if !ok { return }

		componentID := c.Param("id")
        if err := s.componentMgr.EnableComponent(componentID); err != nil { s.respondError(c, 500, "internal_error", err.Error()); return }

		s.JSON(c, 200, gin.H{"ok": true})
	})

        r.POST("/api/components/:id/disable", func(c *gin.Context) {
            _, _, ok := s.require(c, "components:manage")
            if !ok { return }

		componentID := c.Param("id")
        if err := s.componentMgr.DisableComponent(componentID); err != nil { s.respondError(c, 500, "internal_error", err.Error()); return }

		s.JSON(c, 200, gin.H{"ok": true})
	})

	// Entity Management APIs
        r.GET("/api/entities", func(c *gin.Context) {
            _, _, ok := s.require(c, "entities:read")
            if !ok { return }

		// Load all entity definitions from components
		entities := []map[string]any{}

		// Scan all component directories for entity definitions
		componentsDir := "components"
		if _, err := os.Stat(componentsDir); os.IsNotExist(err) {
			s.JSON(c, 200, gin.H{"entities": entities})
			return
		}

            entries, err := os.ReadDir(componentsDir)
            if err != nil { s.respondError(c, 500, "internal_error", "read components failed"); return }

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			descriptorsDir := filepath.Join(componentsDir, entry.Name(), "descriptors")
			if _, err := os.Stat(descriptorsDir); os.IsNotExist(err) {
				continue
			}

			descriptorFiles, err := os.ReadDir(descriptorsDir)
			if err != nil {
				continue
			}

			for _, file := range descriptorFiles {
				if !strings.HasSuffix(file.Name(), ".entity.json") {
					continue
				}

				entityPath := filepath.Join(descriptorsDir, file.Name())
				entityData, err := os.ReadFile(entityPath)
				if err != nil {
					continue
				}

				var entity map[string]any
				if err := json.Unmarshal(entityData, &entity); err != nil {
					continue
				}

				// Add component info
				entity["component"] = entry.Name()
				entities = append(entities, entity)
			}
		}

		s.JSON(c, 200, gin.H{"entities": entities})
        })

        r.POST("/api/entities", func(c *gin.Context) {
            _, _, ok := s.require(c, "entities:create")
            if !ok { return }

		var entity map[string]any
            if err := c.BindJSON(&entity); err != nil { s.respondError(c, 400, "bad_request", "invalid payload"); return }

		// Validate required fields
		id, ok := entity["id"].(string)
            if !ok || id == "" { s.respondError(c, 400, "bad_request", "missing or invalid entity id"); return }

		entityType, ok := entity["type"].(string)
            if !ok || entityType != "entity" { s.respondError(c, 400, "bad_request", "type must be 'entity'"); return }

		// Determine target component
		component, ok := entity["component"].(string)
            if !ok || component == "" { s.respondError(c, 400, "bad_request", "missing component"); return }

		// Create entity file
		componentDir := filepath.Join("components", component)
		descriptorsDir := filepath.Join(componentDir, "descriptors")

            if err := os.MkdirAll(descriptorsDir, 0755); err != nil { s.respondError(c, 500, "internal_error", "mkdir failed"); return }

		// Remove component field from entity data before saving
		delete(entity, "component")

            entityData, err := json.MarshalIndent(entity, "", "  ")
            if err != nil { s.respondError(c, 500, "internal_error", "marshal failed"); return }

		entityFile := filepath.Join(descriptorsDir, id+".json")
            if err := os.WriteFile(entityFile, entityData, 0644); err != nil { s.respondError(c, 500, "internal_error", "write failed"); return }
            
            c.Header("Location", fmt.Sprintf("/api/entities/%s", id))
            s.JSON(c, 201, gin.H{"id": id, "created": true})
        })

        r.GET("/api/entities/:id", func(c *gin.Context) {
            _, _, ok := s.require(c, "entities:read")
            if !ok { return }

		entityID := c.Param("id")

		// Search for entity in all components
		componentsDir := "components"
            entries, err := os.ReadDir(componentsDir)
            if err != nil { s.respondError(c, 500, "internal_error", "read components failed"); return }

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			entityPath := filepath.Join(componentsDir, entry.Name(), "descriptors", entityID+".json")
			if _, err := os.Stat(entityPath); os.IsNotExist(err) {
				continue
			}

			entityData, err := os.ReadFile(entityPath)
			if err != nil {
				continue
			}

			var entity map[string]any
			if err := json.Unmarshal(entityData, &entity); err != nil {
				continue
			}

			// Add component info
			entity["component"] = entry.Name()
			s.JSON(c, 200, entity)
			return
		}

        s.respondError(c, 404, "not_found", "entity not found")
    })

    r.PUT("/api/entities/:id", func(c *gin.Context) {
        _, _, ok := s.require(c, "entities:update")
        if !ok { return }

		entityID := c.Param("id")
        var entity map[string]any
        if err := c.BindJSON(&entity); err != nil {
            s.respondError(c, 400, "bad_request", "invalid payload")
            return
        }

		// Find existing entity
		componentsDir := "components"
            entries, err := os.ReadDir(componentsDir)
            if err != nil { s.respondError(c, 500, "internal_error", "read components failed"); return }

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			entityPath := filepath.Join(componentsDir, entry.Name(), "descriptors", entityID+".json")
			if _, err := os.Stat(entityPath); os.IsNotExist(err) {
				continue
			}

			// Remove component field from entity data before saving
			delete(entity, "component")

			// Ensure ID matches
			entity["id"] = entityID

            entityData, err := json.MarshalIndent(entity, "", "  ")
            if err != nil {
                s.respondError(c, 500, "internal_error", err.Error())
                return
            }

            if err := os.WriteFile(entityPath, entityData, 0644); err != nil {
                s.respondError(c, 500, "internal_error", err.Error())
                return
            }

			s.JSON(c, 200, gin.H{"id": entityID, "updated": true})
			return
		}

            s.respondError(c, 404, "not_found", "entity not found")
        })

        r.DELETE("/api/entities/:id", func(c *gin.Context) {
            _, _, ok := s.require(c, "entities:delete")
            if !ok { return }

		entityID := c.Param("id")

		// Find and delete entity
		componentsDir := "components"
            entries, err := os.ReadDir(componentsDir)
            if err != nil { s.respondError(c, 500, "internal_error", "read components failed"); return }

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			entityPath := filepath.Join(componentsDir, entry.Name(), "descriptors", entityID+".json")
			if _, err := os.Stat(entityPath); os.IsNotExist(err) {
				continue
			}

                if err := os.Remove(entityPath); err != nil { s.respondError(c, 500, "internal_error", "delete failed"); return }

			s.JSON(c, 200, gin.H{"id": entityID, "deleted": true})
			return
		}

            s.respondError(c, 404, "not_found", "entity not found")
        })

        r.POST("/api/entities/validate", func(c *gin.Context) {
            _, _, ok := s.require(c, "entities:read")
            if !ok { return }
            
            var entity map[string]any
            if err := c.BindJSON(&entity); err != nil { s.respondError(c, 400, "bad_request", "invalid payload"); return }

		// Use the enhanced validation function
		errors := entityvalidation.ValidateEntityDefinition(entity)

		s.JSON(c, 200, gin.H{
			"valid":  len(errors) == 0,
			"errors": errors,
		})
	})

        r.POST("/api/entities/:id/preview", func(c *gin.Context) {
            _, _, ok := s.require(c, "entities:read")
            if !ok { return }

		entityID := c.Param("id")

		// Find entity
		componentsDir := "components"
            entries, err := os.ReadDir(componentsDir)
            if err != nil { s.respondError(c, 500, "internal_error", "read components failed"); return }

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			entityPath := filepath.Join(componentsDir, entry.Name(), "descriptors", entityID+".json")
			if _, err := os.Stat(entityPath); os.IsNotExist(err) {
				continue
			}

			entityData, err := os.ReadFile(entityPath)
			if err != nil {
				continue
			}

			var entity map[string]any
			if err := json.Unmarshal(entityData, &entity); err != nil {
				continue
			}

			// Generate ProTable configuration
			proTableConfig := map[string]any{
				"columns": []map[string]any{},
				"search": map[string]any{"placeholder": "Search " + entityID},
				"pagination": map[string]any{"pageSize": 20},
			}

			// Generate ProForm configuration
			proFormConfig := map[string]any{
				"layout": "vertical",
				"submitter": map[string]any{
					"searchConfig": map[string]any{"submitText": "Save"},
				},
			}

			// Extract schema properties for column/form generation
			if schema, ok := entity["schema"].(map[string]any); ok {
				if properties, ok := schema["properties"].(map[string]any); ok {
					columns := []map[string]any{}
					formItems := []map[string]any{}

					for fieldName, fieldDef := range properties {
						if fieldDefMap, ok := fieldDef.(map[string]any); ok {
							fieldType, _ := fieldDefMap["type"].(string)
							description, _ := fieldDefMap["description"].(string)

							// ProTable column
							column := map[string]any{
								"dataIndex": fieldName,
								"title":     description,
								"key":       fieldName,
							}

							// Add special handling based on field properties
							if searchable, ok := fieldDefMap["searchable"].(bool); ok && searchable {
								column["search"] = true
							}
							if sortable, ok := fieldDefMap["sortable"].(bool); ok && sortable {
								column["sorter"] = true
							}
							if filterable, ok := fieldDefMap["filterable"].(bool); ok && filterable {
								if enum, ok := fieldDefMap["enum"].([]any); ok {
									options := []map[string]any{}
									for _, v := range enum {
										if s, ok := v.(string); ok {
											options = append(options, map[string]any{
												"text": s, "value": s,
											})
										}
									}
									column["filters"] = options
								}
							}

							columns = append(columns, column)

							// ProForm item
							formItem := map[string]any{
								"name":  fieldName,
								"label": description,
								"rules": []map[string]any{},
							}

							// Determine form field type
							switch fieldType {
							case "string":
								if enum, ok := fieldDefMap["enum"].([]any); ok {
									formItem["valueType"] = "select"
									options := []map[string]any{}
									for _, v := range enum {
										if s, ok := v.(string); ok {
											options = append(options, map[string]any{
												"label": s, "value": s,
											})
										}
									}
									formItem["options"] = options
								} else if format, ok := fieldDefMap["format"].(string); ok {
									switch format {
									case "date":
										formItem["valueType"] = "date"
									case "date-time":
										formItem["valueType"] = "dateTime"
									case "email":
										formItem["valueType"] = "email"
									default:
										formItem["valueType"] = "text"
									}
								} else {
									formItem["valueType"] = "text"
								}
							case "integer", "number":
								formItem["valueType"] = "digit"
							case "boolean":
								formItem["valueType"] = "switch"
							default:
								formItem["valueType"] = "text"
							}

							// Add validation rules
							if required, ok := schema["required"].([]any); ok {
								for _, r := range required {
									if rs, ok := r.(string); ok && rs == fieldName {
                            formItem["rules"] = append(formItem["rules"].([]map[string]any), map[string]any{
                                "required": true,
                                // Primary UI language is zh-CN
                                "message":  "请输入 " + description,
                            })
										break
									}
								}
							}

							formItems = append(formItems, formItem)
						}
					}

					proTableConfig["columns"] = columns
					proFormConfig["items"] = formItems
				}
			}

			s.JSON(c, 200, gin.H{
				"entity":         entity,
				"proTableConfig": proTableConfig,
				"proFormConfig":  proFormConfig,
			})
			return
		}

        s.respondError(c, 404, "not_found", "entity not found")
    })

    // Schema validation endpoint
    r.POST("/api/schema/validate", func(c *gin.Context) {
        _, _, ok := s.require(c, "schema:validate")
        if !ok { return }

        var request struct {
            Schema map[string]any `json:"schema"`
        }
        if err := c.BindJSON(&request); err != nil {
            s.respondError(c, 400, "bad_request", "invalid payload")
            return
        }

		// Validate just the JSON Schema part
		errors := entityvalidation.ValidateEntityDefinition(map[string]any{
			"id":     "temp",
			"type":   "entity",
			"schema": request.Schema,
		})

		// Filter out errors that aren't schema-related
		schemaErrors := []string{}
		for _, err := range errors {
			if strings.Contains(err, "schema") {
				schemaErrors = append(schemaErrors, err)
			}
		}

		s.JSON(c, 200, gin.H{
			"valid":  len(schemaErrors) == 0,
			"errors": schemaErrors,
		})
	})

    // Assignments (in-memory)
    r.GET("/api/assignments", func(c *gin.Context) {
        if _, _, ok := s.require(c, "assignments:read", "assignments:write"); !ok { return }
        gid := c.Query("game_id")
        env := c.Query("env")
        s.mu.RLock()
        out := map[string][]string{}
        for k, v := range s.assignments {
			if gid != "" || env != "" {
				parts := strings.SplitN(k, "|", 2)
				ge := ""
				if len(parts) > 1 {
					ge = parts[1]
				}
				if (gid != "" && parts[0] != gid) || (env != "" && ge != env) {
					continue
				}
			}
			out[k] = append([]string{}, v...)
		}
		s.mu.RUnlock()
		s.JSON(c, 200, gin.H{"assignments": out})
	})
    r.POST("/api/assignments", func(c *gin.Context) {
        actor, _, ok := s.require(c, "assignments:write")
        if !ok { return }
        var in struct {
            GameID, Env string
            Functions   []string
        }
        if err := c.BindJSON(&in); err != nil || in.GameID == "" {
            s.respondError(c, 400, "bad_request", "bad request")
            return
        }
		valid := make([]string, 0, len(in.Functions))
		unknown := []string{}
		for _, fid := range in.Functions {
			if _, ok := s.descIndex[fid]; ok {
				valid = append(valid, fid)
			} else {
				unknown = append(unknown, fid)
			}
		}
		key := in.GameID + "|" + in.Env
		s.mu.Lock()
		s.assignments[key] = append([]string{}, valid...)
		s.mu.Unlock()
		b, _ := json.MarshalIndent(s.assignments, "", "  ")
		_ = os.WriteFile(s.assignmentsPath, b, 0o644)
            if s.audit != nil {
                meta := map[string]string{"game_env": key, "game_id": in.GameID, "env": in.Env, "functions": strings.Join(valid, ","), "ip": c.ClientIP()}
                if len(unknown) > 0 {
                    meta["unknown"] = strings.Join(unknown, ",")
                }
                if err := s.audit.Log("assignments.update", actor, key, meta); err != nil {
                    atomic.AddInt64(&s.auditErrors, 1)
                }
            }
		s.JSON(c, 200, gin.H{"ok": true, "unknown": unknown})
	})

        // Ant Design Pro demo stub
        r.Any("/api/rule", func(c *gin.Context) {
            if c.Request.Method == http.MethodGet {
                s.JSON(c, 200, gin.H{"data": []any{}, "total": 0, "success": true})
            } else if c.Request.Method == http.MethodPost {
                s.JSON(c, 200, gin.H{"success": true})
            } else {
                s.respondError(c, 405, "method_not_allowed", "method not allowed")
            }
        })

	// Users and Roles management
	// Me profile
        r.GET("/api/me/profile", func(c *gin.Context) {
            user, _, ok := s.auth(c.Request)
            if !ok { s.respondError(c, 401, "unauthorized", "unauthorized"); return }
            if s.userRepo == nil { s.respondError(c, 503, "unavailable", "user repo unavailable"); return }
            ur, err := s.userRepo.GetUserByUsername(c, user)
            if err != nil { s.respondError(c, 404, "not_found", "not found"); return }
		roles, _ := s.userRepo.ListUserRoles(c, ur.ID)
		rn := []string{}
		for _, r0 := range roles {
			rn = append(rn, r0.Name)
		}
		s.JSON(c, 200, gin.H{"username": ur.Username, "display_name": ur.DisplayName, "email": ur.Email, "phone": ur.Phone, "active": ur.Active, "roles": rn})
	})
        r.PUT("/api/me/profile", func(c *gin.Context) {
            user, _, ok := s.auth(c.Request)
            if !ok { s.respondError(c, 401, "unauthorized", "unauthorized"); return }
            if s.userRepo == nil { s.respondError(c, 503, "unavailable", "user repo unavailable"); return }
            ur, err := s.userRepo.GetUserByUsername(c, user)
            if err != nil { s.respondError(c, 404, "not_found", "not found"); return }
            var in struct{ DisplayName, Email, Phone string }
            if err := c.BindJSON(&in); err != nil { s.respondError(c, 400, "bad_request", "invalid payload"); return }
            ur.DisplayName, ur.Email, ur.Phone = in.DisplayName, in.Email, in.Phone
            if err := s.userRepo.UpdateUser(c, ur); err != nil { s.respondError(c, 500, "internal_error", "update failed"); return }
            c.Status(204)
        })
        r.POST("/api/me/password", func(c *gin.Context) {
            user, _, ok := s.auth(c.Request)
            if !ok { s.respondError(c, 401, "unauthorized", "unauthorized"); return }
            if s.userRepo == nil { s.respondError(c, 503, "unavailable", "user repo unavailable"); return }
            var in struct{ Current, Password string }
            if err := c.BindJSON(&in); err != nil || in.Password == "" { s.respondError(c, 400, "bad_request", "invalid payload"); return } // verify current password if set
            if _, err := s.userRepo.Verify(c, user, in.Current); err != nil { s.respondError(c, 401, "unauthorized", "invalid current password"); return }
            ur, err := s.userRepo.GetUserByUsername(c, user)
            if err != nil { s.respondError(c, 404, "not_found", "not found"); return }
            if err := s.userRepo.SetPassword(c, ur.ID, in.Password); err != nil { s.respondError(c, 500, "internal_error", "failed to set password"); return }
            c.Status(204)
        })

	// Messages (inbox)
        r.GET("/api/messages/unread_count", func(c *gin.Context) {
            user, _, ok := s.auth(c.Request)
            if !ok { s.respondError(c, 401, "unauthorized", "unauthorized"); return }
            if s.msgRepo == nil || s.userRepo == nil { s.respondError(c, 503, "unavailable", "repo unavailable"); return }
            ur, err := s.userRepo.GetUserByUsername(c, user)
            if err != nil { s.respondError(c, 404, "not_found", "not found"); return }
		// direct unread
		n1, err := s.msgRepo.UnreadCount(c, ur.ID)
            if err != nil { s.respondError(c, 500, "internal_error", err.Error()); return }
		// broadcast unread by roles
		rlist, _ := s.userRepo.ListUserRoles(c, ur.ID)
		roleNames := make([]string, 0, len(rlist))
		for _, r0 := range rlist {
			roleNames = append(roleNames, r0.Name)
		}
		n2, err := s.msgRepo.Broadcast().UnreadCount(c, ur.ID, roleNames)
        if err != nil {
            s.respondError(c, 500, "internal_error", err.Error())
            return
        }
		s.JSON(c, 200, gin.H{"count": n1 + n2})
	})
        r.GET("/api/messages", func(c *gin.Context) {
            user, _, ok := s.auth(c.Request)
            if !ok { s.respondError(c, 401, "unauthorized", "unauthorized"); return }
            if s.msgRepo == nil || s.userRepo == nil { s.respondError(c, 503, "unavailable", "repo unavailable"); return }
            ur, err := s.userRepo.GetUserByUsername(c, user)
            if err != nil { s.respondError(c, 404, "not_found", "not found"); return }
		status := strings.ToLower(c.DefaultQuery("status", "unread"))
		unreadOnly := (status == "unread")
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
		if page <= 0 {
			page = 1
		}
		if size <= 0 || size > 100 {
			size = 20
		}
		// fetch direct and broadcast then merge-sort in memory (MVP)
		// to approximate correct window, over-fetch up to page*size from each
		capN := page * size
		if capN < size {
			capN = size
		}
		dirItems, _, err := s.msgRepo.List(c, ur.ID, unreadOnly, capN, 0)
            if err != nil { s.respondError(c, 500, "internal_error", err.Error()); return }
		rlist, _ := s.userRepo.ListUserRoles(c, ur.ID)
		roleNames := make([]string, 0, len(rlist))
		for _, r0 := range rlist {
			roleNames = append(roleNames, r0.Name)
		}
		bItems, _, err := s.msgRepo.Broadcast().List(c, ur.ID, roleNames, unreadOnly, capN, 0)
        if err != nil {
            s.respondError(c, 500, "internal_error", err.Error())
            return
        }
		type item struct {
			ID                   uint
			Title, Content, Type string
			CreatedAt            time.Time
			Read                 bool
			Kind                 string
		}
		merged := make([]item, 0, len(dirItems)+len(bItems))
		for _, it := range dirItems {
			merged = append(merged, item{ID: it.ID, Title: it.Title, Content: it.Content, Type: it.Type, CreatedAt: it.CreatedAt, Read: it.ReadAt != nil, Kind: "direct"})
		}
		for _, it := range bItems {
			merged = append(merged, item{ID: it.ID, Title: it.Title, Content: it.Content, Type: it.Type, CreatedAt: it.CreatedAt, Read: it.Read, Kind: "broadcast"})
		}
		// sort DESC by CreatedAt
		sort.Slice(merged, func(i, j int) bool { return merged[i].CreatedAt.After(merged[j].CreatedAt) })
		total := len(merged)
		start := (page - 1) * size
		if start > total {
			start = total
		}
		endi := start + size
		if endi > total {
			endi = total
		}
		window := merged[start:endi]
		type M struct {
			ID        uint      `json:"id"`
			Title     string    `json:"title"`
			Content   string    `json:"content"`
			Type      string    `json:"type"`
			CreatedAt time.Time `json:"created_at"`
			Read      bool      `json:"read"`
			Kind      string    `json:"kind"`
		}
		out := make([]M, 0, len(window))
		for _, it := range window {
			out = append(out, M{ID: it.ID, Title: it.Title, Content: it.Content, Type: it.Type, CreatedAt: it.CreatedAt, Read: it.Read, Kind: it.Kind})
		}
		s.JSON(c, 200, gin.H{"messages": out, "total": total, "page": page, "size": size})
	})
        r.POST("/api/messages/read", func(c *gin.Context) {
            user, _, ok := s.auth(c.Request)
            if !ok { s.respondError(c, 401, "unauthorized", "unauthorized"); return }
            if s.msgRepo == nil || s.userRepo == nil { s.respondError(c, 503, "unavailable", "repo unavailable"); return }
            ur, err := s.userRepo.GetUserByUsername(c, user)
            if err != nil { s.respondError(c, 404, "not_found", "not found"); return }
            var in struct {
                IDs          []uint `json:"ids"`
                BroadcastIDs []uint `json:"broadcast_ids"`
            }
            if err := c.BindJSON(&in); err != nil { s.respondError(c, 400, "bad_request", "invalid payload"); return }
            if err := s.msgRepo.MarkRead(c, ur.ID, in.IDs); err != nil { s.respondError(c, 500, "internal_error", err.Error()); return }
            if len(in.BroadcastIDs) > 0 {
                if err := s.msgRepo.Broadcast().MarkRead(c, ur.ID, in.BroadcastIDs); err != nil { s.respondError(c, 500, "internal_error", err.Error()); return }
            }
            s.msgNotify()
            c.Status(204)
        })
        // Admin send message to a user (requires messages:send or users:manage/admin)
        r.POST("/api/messages", func(c *gin.Context) {
            actor, roles, ok := s.auth(c.Request)
            if !ok { s.respondError(c, 401, "unauthorized", "unauthorized"); return }
            if !(s.can(actor, roles, "messages:send") || s.can(actor, roles, "users:manage") || s.can(actor, roles, "admin")) { s.respondError(c, 403, "forbidden", "forbidden"); return }
            if s.msgRepo == nil || s.userRepo == nil { s.respondError(c, 503, "unavailable", "repo unavailable"); return }
            // Direct or broadcast based on body
            var raw map[string]any
            if err := c.BindJSON(&raw); err != nil { s.respondError(c, 400, "bad_request", "invalid payload"); return }
            if _, ok := raw["to_username"]; ok || raw["to_user_id"] != nil {
                // direct
                var in struct {
                    ToUsername           string
                    ToUserID             *uint
                    Title, Content, Type string
                }
                b, _ := json.Marshal(raw)
                _ = json.Unmarshal(b, &in)
                var toID uint
                if in.ToUserID != nil {
                    toID = *in.ToUserID
                } else if in.ToUsername != "" {
                    ur, err := s.userRepo.GetUserByUsername(c, in.ToUsername)
                    if err != nil { s.respondError(c, 404, "not_found", "user not found"); return }
                    toID = ur.ID
                } else {
                    s.respondError(c, 400, "bad_request", "missing recipient"); return
                }
                var fromID *uint
                if ur, err := s.userRepo.GetUserByUsername(c, actor); err == nil {
                    fromID = &ur.ID
                }
                m := &msgsgorm.MessageRecord{ToUserID: toID, FromUserID: fromID, Title: in.Title, Content: in.Content, Type: in.Type}
                if err := s.msgRepo.Create(c, m); err != nil { s.respondError(c, 500, "internal_error", err.Error()); return }
                if s.audit != nil {
                    _ = s.audit.Log("message_send", actor, fmt.Sprintf("user:%d", toID), map[string]string{"ip": c.ClientIP(), "title": in.Title})
                }
                s.msgNotify()
                s.JSON(c, 200, gin.H{"id": m.ID, "kind": "direct"})
                return
            }
            // broadcast
            var in2 struct {
                Title, Content, Type string
                Audience             struct {
                    All   bool
                    Roles []string
                }
            }
            b, _ := json.Marshal(raw)
            _ = json.Unmarshal(b, &in2)
            bm := &msgsgorm.BroadcastMessageRecord{Title: in2.Title, Content: in2.Content, Type: in2.Type}
            if in2.Audience.All {
                bm.Audience = "all"
            } else {
                bm.Audience = "roles"
            }
            if err := msgsgorm.NewBroadcastRepo(s.gdb).Create(c, bm, in2.Audience.Roles); err != nil { s.respondError(c, 500, "internal_error", err.Error()); return }
            if s.audit != nil {
                _ = s.audit.Log("message_broadcast", actor, strings.Join(in2.Audience.Roles, ","), map[string]string{"ip": c.ClientIP(), "title": in2.Title})
            }
            s.msgNotify()
            s.JSON(c, 200, gin.H{"id": bm.ID, "kind": "broadcast"})
        })

	// Messages unread-count SSE stream (auth via Authorization or token query for EventSource)
        r.GET("/api/messages/stream", func(c *gin.Context) {
            user, _, ok := s.auth(c.Request)
            if !ok {
                tok := c.Query("token")
                if tok != "" && s.jwtMgr != nil {
                    if u, roles, err := s.jwtMgr.Verify(tok); err == nil {
                        user, ok = u, true
                        _ = roles
                    }
                }
            }
            if !ok {
                s.respondError(c, 401, "unauthorized", "unauthorized"); return
            }
            // Setup SSE
            w := c.Writer
            w.Header().Set("Content-Type", "text/event-stream")
            w.Header().Set("Cache-Control", "no-cache")
            w.Header().Set("Connection", "keep-alive")
            flusher, okf := w.(http.Flusher)
            if !okf {
                s.respondError(c, 500, "internal_error", "stream unsupported"); return
            }
		// subscribe
		sub := s.msgAddSub()
		defer s.msgRemoveSub(sub)
		// helper to send current count
		send := func() bool {
			if s.msgRepo == nil || s.userRepo == nil {
				return false
			}
			ur, err := s.userRepo.GetUserByUsername(c, user)
			if err != nil {
				return false
			}
			n, err := s.msgRepo.UnreadCount(c, ur.ID)
			if err != nil {
				return false
			}
			fmt.Fprintf(w, "event: unread\n")
			fmt.Fprintf(w, "data: {\"count\": %d}\n\n", n)
			flusher.Flush()
			return true
		}
		// initial push
		_ = send()
		// loop
		notify := c.Request.Context().Done()
		for {
			select {
			case <-notify:
				return
			case <-sub:
				_ = send()
			}
		}
	})

        // Users list/create/update/delete (admin)
        r.GET("/api/users", func(c *gin.Context) {
            _, _, ok := s.require(c, "users:read", "users:manage")
            if !ok { return }
            if s.userRepo == nil { s.respondError(c, 503, "unavailable", "user repo unavailable"); return }
            users, err := s.userRepo.ListUsers(c)
            if err != nil { s.respondError(c, 500, "internal_error", "failed to list users"); return }
            // attach roles per user
            out := make([]map[string]any, 0, len(users))
            for _, u := range users {
                rlist, _ := s.userRepo.ListUserRoles(c, u.ID)
                rn := []string{}
                for _, r0 := range rlist {
                    rn = append(rn, r0.Name)
                }
                out = append(out, map[string]any{"id": u.ID, "username": u.Username, "display_name": u.DisplayName, "email": u.Email, "phone": u.Phone, "active": u.Active, "roles": rn})
            }
            s.JSON(c, 200, gin.H{"users": out})
        })
        r.POST("/api/users", func(c *gin.Context) {
            actor, _, ok := s.require(c, "users:manage")
            if !ok { return }
            if s.userRepo == nil { s.respondError(c, 503, "unavailable", "user repo unavailable"); return }
            var in struct {
                Username, DisplayName, Email, Phone, Password string
                Active                                        bool
                Roles                                         []string
            }
            if err := c.BindJSON(&in); err != nil || in.Username == "" { s.respondError(c, 400, "bad_request", "invalid payload"); return }
            u := &usersgorm.UserAccount{Username: in.Username, DisplayName: in.DisplayName, Email: in.Email, Phone: in.Phone, Active: in.Active}
            if err := s.userRepo.CreateUser(c, u); err != nil { s.respondError(c, 500, "internal_error", "failed to create user"); return }
            if in.Password != "" {
                _ = s.userRepo.SetPassword(c, u.ID, in.Password)
            } // attach roles
            if len(in.Roles) > 0 {
                var all []*usersgorm.RoleRecord
                all, _ = s.userRepo.ListRoles(c)
                name2id := map[string]uint{}
                for _, r0 := range all {
                    name2id[r0.Name] = r0.ID
                }
                for _, rn := range in.Roles {
                    if id, ok := name2id[rn]; ok {
                        _ = s.userRepo.AddUserRole(c, u.ID, id)
                    }
                }
            }
            _ = s.buildPolicyFromDB()
            if s.audit != nil {
                _ = s.audit.Log("user_create", actor, in.Username, map[string]string{"id": fmt.Sprintf("%d", u.ID), "ip": c.ClientIP()})
            }
            s.JSON(c, 200, gin.H{"id": u.ID})
        })
        r.PUT("/api/users/:id", func(c *gin.Context) {
            actor, _, ok := s.require(c, "users:manage")
            if !ok { return }
            if s.userRepo == nil { s.respondError(c, 503, "unavailable", "user repo unavailable"); return }
            id64, _ := strconv.ParseUint(c.Param("id"), 10, 64)
            var in struct {
                DisplayName, Email, Phone string
                Active                    *bool
                Roles                     []string
            }
            if err := c.BindJSON(&in); err != nil { s.respondError(c, 400, "bad_request", "invalid payload"); return } // load user
            var urec *usersgorm.UserAccount // we only have GetUserByUsername; use gorm directly
            var u usersgorm.UserAccount
            if err := s.gdb.First(&u, uint(id64)).Error; err != nil { s.respondError(c, 404, "not_found", "user not found"); return }
            urec = &u
            if in.DisplayName != "" {
                urec.DisplayName = in.DisplayName
            }
            if in.Email != "" || in.Email == "" {
                urec.Email = in.Email
            }
            if in.Phone != "" || in.Phone == "" {
                urec.Phone = in.Phone
            }
            if in.Active != nil {
                urec.Active = *in.Active
            }
            if err := s.userRepo.UpdateUser(c, urec); err != nil { s.respondError(c, 500, "internal_error", "failed to update user"); return }
            // update roles if provided
            var changedRoles bool
            if in.Roles != nil { // replace set
                current, _ := s.userRepo.ListUserRoles(c, urec.ID)
                cur := map[string]uint{}
                for _, r0 := range current {
                    cur[r0.Name] = r0.ID
                }
                all, _ := s.userRepo.ListRoles(c)
                name2id := map[string]uint{}
                for _, r0 := range all {
                    name2id[r0.Name] = r0.ID
                }
                want := map[string]uint{}
                for _, rn := range in.Roles {
                    if id, ok := name2id[rn]; ok {
                        want[rn] = id
                    }
                }
                // remove missing
                for rn, id := range cur {
                    if _, ok := want[rn]; !ok {
                        _ = s.userRepo.RemoveUserRole(c, urec.ID, id)
                        changedRoles = true
                    }
                }
                // add new
                for rn, id := range want {
                    if _, ok := cur[rn]; !ok {
                        _ = s.userRepo.AddUserRole(c, urec.ID, id)
                        changedRoles = true
                    }
                }
                _ = s.buildPolicyFromDB()
            }
            if s.audit != nil {
                meta := map[string]string{"id": fmt.Sprintf("%d", urec.ID), "ip": c.ClientIP()}
                if changedRoles { meta["roles_changed"] = "true" }
                _ = s.audit.Log("user_update", actor, urec.Username, meta)
            }
            c.Status(204)
        })
        r.DELETE("/api/users/:id", func(c *gin.Context) {
            actor, _, ok := s.require(c, "users:manage")
            if !ok { return }
            id64, _ := strconv.ParseUint(c.Param("id"), 10, 64)
            if err := s.userRepo.DeleteUser(c, uint(id64)); err != nil { s.respondError(c, 500, "internal_error", "failed to delete user"); return }
            if s.audit != nil {
                _ = s.audit.Log("user_delete", actor, fmt.Sprintf("%d", id64), map[string]string{"ip": c.ClientIP()})
            }
            c.Status(204)
        })
        r.POST("/api/users/:id/password", func(c *gin.Context) {
            actor, _, ok := s.require(c, "users:manage")
            if !ok { return }
            var in struct{ Password string }
            if err := c.BindJSON(&in); err != nil || in.Password == "" { s.respondError(c, 400, "bad_request", "invalid payload"); return }
            id64, _ := strconv.ParseUint(c.Param("id"), 10, 64)
            if err := s.userRepo.SetPassword(c, uint(id64), in.Password); err != nil { s.respondError(c, 500, "internal_error", "failed to set password"); return }
            if s.audit != nil {
                _ = s.audit.Log("user_set_password", actor, fmt.Sprintf("%d", id64), map[string]string{"ip": c.ClientIP()})
            }
            c.Status(204)
        })

        // User game scopes (assign which games a user can access)
        r.GET("/api/users/:id/games", func(c *gin.Context) {
            _, _, ok := s.require(c, "users:read", "users:manage")
            if !ok { return }
            if s.userRepo == nil { s.respondError(c, 503, "unavailable", "user repo unavailable"); return }
            id64, _ := strconv.ParseUint(c.Param("id"), 10, 64)
            ids, err := s.userRepo.ListUserGameIDs(c, uint(id64))
            if err != nil { s.respondError(c, 500, "internal_error", "failed to list user games"); return }
            s.JSON(c, 200, gin.H{"game_ids": ids})
        })
        r.PUT("/api/users/:id/games", func(c *gin.Context) {
            actor, _, ok := s.require(c, "users:manage")
            if !ok { return }
            if s.userRepo == nil { s.respondError(c, 503, "unavailable", "user repo unavailable"); return }
            id64, _ := strconv.ParseUint(c.Param("id"), 10, 64)
            var in struct{ GameIDs []uint `json:"game_ids"` }
            if err := c.BindJSON(&in); err != nil { s.respondError(c, 400, "bad_request", "invalid payload"); return }
            if err := s.userRepo.ReplaceUserGameIDs(c, uint(id64), in.GameIDs); err != nil { s.respondError(c, 500, "internal_error", "failed to set user games"); return }
            if s.audit != nil {
                _ = s.audit.Log("user_set_games", actor, fmt.Sprintf("%d", id64), map[string]string{"count": fmt.Sprintf("%d", len(in.GameIDs)), "ip": c.ClientIP()})
            }
            c.Status(204)
        })

	// Roles CRUD
        r.GET("/api/roles", func(c *gin.Context) {
            _, _, ok := s.require(c, "roles:read", "roles:manage")
            if !ok { return }
		var arr []*usersgorm.RoleRecord
		arr, _ = s.userRepo.ListRoles(c)
		out := make([]map[string]any, 0, len(arr))
		for _, r0 := range arr {
			perms, _ := s.userRepo.ListRolePerms(c, r0.ID)
			out = append(out, map[string]any{"id": r0.ID, "name": r0.Name, "description": r0.Description, "perms": perms})
		}
		s.JSON(c, 200, gin.H{"roles": out})
	})
        r.POST("/api/roles", func(c *gin.Context) {
            _, _, ok := s.require(c, "roles:manage")
            if !ok { return }
		var in struct {
			Name, Description string
			Perms             []string
		}
            if err := c.BindJSON(&in); err != nil || in.Name == "" { s.respondError(c, 400, "bad_request", "invalid payload"); return }
		rrec := &usersgorm.RoleRecord{Name: in.Name, Description: in.Description}
            if err := s.gdb.Create(rrec).Error; err != nil { s.respondError(c, 500, "internal_error", "failed to create role"); return }
		for _, p := range in.Perms {
			_ = s.userRepo.GrantRolePerm(c, rrec.ID, p)
		}
		_ = s.buildPolicyFromDB()
            c.Header("Location", fmt.Sprintf("/api/roles/%d", rrec.ID))
            s.JSON(c, 201, gin.H{"id": rrec.ID})
	})
        r.PUT("/api/roles/:id", func(c *gin.Context) {
            _, _, ok := s.require(c, "roles:manage")
            if !ok { return }
		id64, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var in struct{ Name, Description string }
            if err := c.BindJSON(&in); err != nil { s.respondError(c, 400, "bad_request", "invalid payload"); return }
		var rrec usersgorm.RoleRecord
            if err := s.gdb.First(&rrec, uint(id64)).Error; err != nil { s.respondError(c, 404, "not_found", "role not found"); return }
		if in.Name != "" {
			rrec.Name = in.Name
		}
		if in.Description != "" || in.Description == "" {
			rrec.Description = in.Description
		}
            if err := s.gdb.Save(&rrec).Error; err != nil { s.respondError(c, 500, "internal_error", "failed to update role"); return }
            _ = s.buildPolicyFromDB()
            c.Status(204)
        })
        r.DELETE("/api/roles/:id", func(c *gin.Context) {
            _, _, ok := s.require(c, "roles:manage")
            if !ok { return }
		id64, _ := strconv.ParseUint(c.Param("id"), 10, 64)
            if err := s.gdb.Delete(&usersgorm.RoleRecord{}, uint(id64)).Error; err != nil { s.respondError(c, 500, "internal_error", "failed to delete role"); return }
            _ = s.buildPolicyFromDB()
            c.Status(204)
        })
        r.PUT("/api/roles/:id/perms", func(c *gin.Context) {
            _, _, ok := s.require(c, "roles:manage")
            if !ok { return }
            id64, _ := strconv.ParseUint(c.Param("id"), 10, 64)
            var in struct{ Perms []string }
            if err := c.BindJSON(&in); err != nil { s.respondError(c, 400, "bad_request", "invalid payload"); return } // replace perms
		// clear existing perms
		_ = s.gdb.Where("role_id=?", uint(id64)).Delete(&usersgorm.RolePermRecord{}).Error
		for _, p := range in.Perms {
			_ = s.userRepo.GrantRolePerm(c, uint(id64), p)
		}
		_ = s.buildPolicyFromDB()
		c.Status(204)
	})

        // Function invoke
        r.POST("/api/invoke", func(c *gin.Context) {
            user, roles, ok := s.auth(c.Request)
            if !ok { s.respondError(c, 401, "unauthorized", "unauthorized"); return }
            gameID := c.Request.Header.Get("X-Game-ID")
            env := c.Request.Header.Get("X-Env")
            var in struct {
                FunctionID      string `json:"function_id"`
                Payload         any    `json:"payload"`
                IdempotencyKey  string `json:"idempotency_key"`
                Route           string `json:"route"`
                TargetServiceID string `json:"target_service_id"`
                HashKey         string `json:"hash_key"`
            }
            if err := c.BindJSON(&in); err != nil { s.respondError(c, 400, "bad_request", "invalid payload"); return }
            if d := s.descIndex[in.FunctionID]; d != nil {
                if ps := d.Params; ps != nil {
                    b, _ := json.Marshal(in.Payload)
                    if err := validation.ValidateJSON(ps, b); err != nil { s.respondError(c, 400, "bad_request", fmt.Sprintf("payload invalid: %v", err)); return }
                }
            }
		basePerm := "function:" + in.FunctionID
		if d := s.descIndex[in.FunctionID]; d != nil {
			if auth := d.Auth; auth != nil {
				if p, ok := auth["permission"].(string); ok && p != "" {
					basePerm = p
				}
			}
		}
		scopedOk := true
		if s.rbac != nil {
			scopedOk = false
			scoped := basePerm
			if gameID != "" {
				scoped = "game:" + gameID + ":" + basePerm
			}
			if s.can(user, roles, scoped) || s.can(user, roles, basePerm) || (gameID != "" && s.can(user, roles, "game:"+gameID+":*")) {
				scopedOk = true
			}
		}
            if !scopedOk { atomic.AddInt64(&s.rbacDenied, 1); s.respondError(c, 403, "forbidden", "forbidden"); return }
            if d := s.descIndex[in.FunctionID]; d != nil && d.Auth != nil {
                if expr, ok := d.Auth["allow_if"].(string); ok && expr != "" {
                    ctx := policyContext{User: user, Roles: roles, GameID: gameID, Env: env, FunctionID: in.FunctionID}
                    if !evalAllowIf(expr, ctx) { s.respondError(c, 403, "forbidden", "forbidden"); return }
                }
            }
            b, err := json.Marshal(in.Payload)
            if err != nil { s.respondError(c, 400, "bad_request", "invalid payload"); return }
            if in.IdempotencyKey == "" {
                in.IdempotencyKey = randHex(16)
            }
            traceID := randHex(8)
            masked := s.maskSnapshot(in.FunctionID, in.Payload)
            if err := s.audit.Log("invoke", user, in.FunctionID, map[string]string{"ip": c.ClientIP(), "trace_id": traceID, "game_id": gameID, "env": env, "payload_snapshot": masked}); err != nil {
                atomic.AddInt64(&s.auditErrors, 1)
            }
            meta := map[string]string{"trace_id": traceID}
		if gameID != "" {
			meta["game_id"] = gameID
		}
		if env != "" {
			meta["env"] = env
		}
		if in.Route != "" {
			meta["route"] = in.Route
		} else if d := s.descIndex[in.FunctionID]; d != nil {
			if sem := d.Semantics; sem != nil {
				if rv, ok := sem["route"].(string); ok && rv != "" {
					meta["route"] = rv
				}
			}
		}
            if rv, ok := meta["route"]; ok && rv != "lb" && rv != "broadcast" && rv != "targeted" && rv != "hash" { s.respondError(c, 400, "bad_request", "invalid route"); return }
            if in.HashKey != "" {
                meta["hash_key"] = in.HashKey
            }
            if meta["route"] == "hash" && meta["hash_key"] == "" { s.respondError(c, 400, "bad_request", "hash_key required for hash route"); return }
            // Dynamic function-level rate limit (supports gray by game/env and percent)
            if rps := s.pickFnRate(in.FunctionID, gameID, env, traceID); rps > 0 {
                if rl := s.getRateLimiter(in.FunctionID, fmt.Sprintf("%drps", rps)); rl != nil && !rl.Try() { s.respondError(c, 429, "rate_limited", "rate limited"); return }
            }
            if d := s.descIndex[in.FunctionID]; d != nil && d.Semantics != nil {
                if v, ok := d.Semantics["rate_limit"].(string); ok && v != "" {
                    rl := s.getRateLimiter(in.FunctionID, v)
                    if rl != nil && !rl.Try() { s.respondError(c, 429, "rate_limited", "rate limited"); return }
                }
                if v, ok := d.Semantics["concurrency"].(float64); ok && v > 0 {
                    sem := s.getSemaphore(in.FunctionID, int(v))
                    select {
                    case sem <- struct{}{}:
                        defer func() { <-sem }()
                    default:
                        s.respondError(c, 429, "rate_limited", "too many concurrent requests")
                        return
                    }
                }
            }
            resp, err := s.invoker.Invoke(c, &functionv1.InvokeRequest{FunctionId: in.FunctionID, IdempotencyKey: in.IdempotencyKey, Payload: b, Metadata: meta})
            if err != nil {
                atomic.AddInt64(&s.invocationsError, 1)
                slog.Error("invoke failed", "user", user, "function_id", in.FunctionID, "trace_id", traceID, "game_id", gameID, "env", env, "route", meta["route"], "error", err.Error())
                if strings.Contains(strings.ToLower(err.Error()), "rate limited") { s.respondError(c, 429, "rate_limited", "rate limited") } else { s.respondError(c, 500, "internal_error", "invoke failed") }
                return
            }
		slog.Info("invoke", "user", user, "function_id", in.FunctionID, "trace_id", traceID, "game_id", gameID, "env", env, "route", meta["route"])
		atomic.AddInt64(&s.invocations, 1)
		out := resp.GetPayload()
		if d := s.descIndex[in.FunctionID]; d != nil && d.Transport != nil {
			if tp, ok := d.Transport["proto"].(map[string]any); ok {
				if fqn, ok2 := tp["response_fqn"].(string); ok2 && fqn != "" && s.typeReg != nil {
					if j, err2 := s.typeReg.ProtoBinToJSON(fqn, out); err2 == nil {
						out = j
					}
				}
			}
		}
		if len(out) == 0 {
			c.Status(204)
			return
		}
		c.Data(200, "application/json", out)
	})

        r.POST("/api/start_job", func(c *gin.Context) {
            user, roles, ok := s.auth(c.Request)
            if !ok { s.respondError(c, 401, "unauthorized", "unauthorized"); return }
            gameID := c.Request.Header.Get("X-Game-ID")
            env := c.Request.Header.Get("X-Env")
            var in struct {
                FunctionID      string
                Payload         any
                IdempotencyKey  string
                Route           string
                TargetServiceID string
                HashKey         string
            }
            if err := c.BindJSON(&in); err != nil { s.respondError(c, 400, "bad_request", "invalid payload"); return }
            if d := s.descIndex[in.FunctionID]; d != nil {
                if ps := d.Params; ps != nil {
                    b, _ := json.Marshal(in.Payload)
                    if err := validation.ValidateJSON(ps, b); err != nil { s.respondError(c, 400, "bad_request", fmt.Sprintf("payload invalid: %v", err)); return }
                }
            }
		basePerm := "function:" + in.FunctionID
		if d := s.descIndex[in.FunctionID]; d != nil {
			if auth := d.Auth; auth != nil {
				if p, ok := auth["permission"].(string); ok && p != "" {
					basePerm = p
				}
			}
		}
		scopedOk := true
		if s.rbac != nil {
			scopedOk = false
			scoped := basePerm
			if gameID != "" {
				scoped = "game:" + gameID + ":" + basePerm
			}
			if s.can(user, roles, scoped) || s.can(user, roles, basePerm) || (gameID != "" && s.can(user, roles, "game:"+gameID+":*")) {
				scopedOk = true
			}
		}
            if !scopedOk { atomic.AddInt64(&s.rbacDenied, 1); s.respondError(c, 403, "forbidden", "forbidden"); return }
            b, _ := json.Marshal(in.Payload)
            if in.IdempotencyKey == "" {
                in.IdempotencyKey = randHex(16)
            }
            traceID := randHex(8)
            if err := s.audit.Log("start_job", user, in.FunctionID, map[string]string{"ip": c.ClientIP(), "trace_id": traceID, "game_id": gameID, "env": env}); err != nil {
                atomic.AddInt64(&s.auditErrors, 1)
            }
            meta := map[string]string{"trace_id": traceID}
            if gameID != "" {
                meta["game_id"] = gameID
            }
            if env != "" {
                meta["env"] = env
            }
            if in.Route != "" {
                meta["route"] = in.Route
            }
            resp, err := s.invoker.StartJob(c, &functionv1.InvokeRequest{FunctionId: in.FunctionID, IdempotencyKey: in.IdempotencyKey, Payload: b, Metadata: meta})
            if err != nil {
                atomic.AddInt64(&s.jobsError, 1)
                slog.Error("start_job failed", "user", user, "function_id", in.FunctionID, "trace_id", traceID, "game_id", gameID, "env", env, "route", in.Route, "error", err.Error())
                if strings.Contains(strings.ToLower(err.Error()), "rate limited") { s.respondError(c, 429, "rate_limited", "rate limited") } else { s.respondError(c, 500, "internal_error", "start_job failed") }
                return
            }
		slog.Info("start_job", "user", user, "function_id", in.FunctionID, "trace_id", traceID, "game_id", gameID, "env", env, "route", in.Route)
		atomic.AddInt64(&s.jobsStarted, 1)
            // record job meta
            if resp != nil {
                if jid := resp.GetJobId(); jid != "" { s.jobsAdd(jid, in.FunctionID, user, gameID, env, traceID) }
            }
		s.JSON(c, 200, resp)
	})

	// Approvals
        r.GET("/api/approvals", func(c *gin.Context) {
            _, _, ok := s.require(c, "approvals:read")
            if !ok { return }
            f := appr.Filter{State: c.Query("state"), FunctionID: c.Query("function_id"), GameID: c.Query("game_id"), Env: c.Query("env"), Actor: c.Query("actor"), Mode: c.Query("mode")}
            page := 1
            size := 20
            sort := c.Query("sort")
            if v := c.Query("page"); v != "" {
                if n, err := strconv.Atoi(v); err == nil && n > 0 {
                    page = n
                }
            }
            if v := c.Query("size"); v != "" {
                if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
                    size = n
                }
            }
            // Support completed_only: merge approved+rejected with correct paging
            completedOnly := strings.EqualFold(c.DefaultQuery("completed_only", "false"), "true") || c.DefaultQuery("completed_only", "") == "1"
            var items []*appr.Approval
            var total int
            if completedOnly && strings.TrimSpace(f.State) == "" {
                // Determine window
                if page <= 0 { page = 1 }
                if size <= 0 || size > 200 { size = 20 }
                need := page*size
                // Fetch approved and rejected up to need window each, then merge-sort by created_at desc
                aItems, aTotal, err1 := s.approvals.List(appr.Filter{State: "approved", FunctionID: f.FunctionID, GameID: f.GameID, Env: f.Env, Actor: f.Actor, Mode: f.Mode}, appr.Page{Page: 1, Size: need, Sort: sort})
                rItems, rTotal, err2 := s.approvals.List(appr.Filter{State: "rejected", FunctionID: f.FunctionID, GameID: f.GameID, Env: f.Env, Actor: f.Actor, Mode: f.Mode}, appr.Page{Page: 1, Size: need, Sort: sort})
                if err1 != nil || err2 != nil { s.respondError(c, 500, "internal_error", "failed to list approvals"); return }
                total = aTotal + rTotal
                // Merge two slices by CreatedAt desc by default (unless created_at_asc)
                desc := true
                if strings.ToLower(sort) == "created_at_asc" { desc = false }
                i, j := 0, 0
                combined := make([]*appr.Approval, 0, len(aItems)+len(rItems))
                for i < len(aItems) && j < len(rItems) {
                    if desc {
                        if aItems[i].CreatedAt.After(rItems[j].CreatedAt) { combined = append(combined, aItems[i]); i++ } else { combined = append(combined, rItems[j]); j++ }
                    } else {
                        if aItems[i].CreatedAt.Before(rItems[j].CreatedAt) { combined = append(combined, aItems[i]); i++ } else { combined = append(combined, rItems[j]); j++ }
                    }
                }
                for ; i < len(aItems); i++ { combined = append(combined, aItems[i]) }
                for ; j < len(rItems); j++ { combined = append(combined, rItems[j]) }
                // Page window
                start := (page-1)*size
                if start > len(combined) { start = len(combined) }
                end := start + size
                if end > len(combined) { end = len(combined) }
                items = combined[start:end]
            } else {
                var err error
                items, total, err = s.approvals.List(f, appr.Page{Page: page, Size: size, Sort: sort})
                if err != nil { s.respondError(c, 500, "internal_error", "failed to list approvals"); return }
            }
            // Optional risk filter: filter by descriptor risk when provided
            if wantRisk := strings.ToLower(strings.TrimSpace(c.Query("risk"))); wantRisk != "" {
                filtered := make([]*appr.Approval, 0, len(items))
                for _, a := range items {
                    if d := s.descIndex[a.FunctionID]; d != nil {
                        if strings.ToLower(strings.TrimSpace(d.Risk)) == wantRisk {
                            filtered = append(filtered, a)
                        }
                    }
                }
                items = filtered
                total = len(items)
            }
            type view struct{ ID, CreatedAt, Actor, FunctionID, IdempotencyKey, Route, TargetServiceID, HashKey, GameID, Env, State, Mode, ApproveIP, ApproveTime, ApproveIPRegion, RejectIP, RejectTime, RejectIPRegion string }
            withAudit := strings.EqualFold(c.DefaultQuery("with_audit", "false"), "true") || c.DefaultQuery("with_audit", "") == "1"
            ipBy := map[string][4]string{}
            if withAudit {
                want := map[string]struct{}{}
                for _, a := range items {
                    // 仅对非 pending 的记录回填审计信息，避免无谓扫描
                    if strings.EqualFold(a.State, "pending") { continue }
                    want[a.ID] = struct{}{}
                }
                if len(want) > 0 {
                    if f, err := os.Open("logs/audit.log"); err == nil {
                        sc := bufio.NewScanner(f)
                        for sc.Scan() {
                            var ev auditchain.Event
                            if err := json.Unmarshal([]byte(strings.TrimSpace(sc.Text())), &ev); err != nil { continue }
                            id := ev.Meta["approval_id"]
                            if id == "" { continue }
                            if _, ok := want[id]; !ok { continue }
                            cur := ipBy[id]
                            if ev.Kind == "approval_approve" {
                                cur[0], cur[1] = ev.Meta["ip"], ev.Time.Format(time.RFC3339)
                            } else if ev.Kind == "approval_reject" {
                                cur[2], cur[3] = ev.Meta["ip"], ev.Time.Format(time.RFC3339)
                            }
                            ipBy[id] = cur
                        }
                        _ = f.Close()
                    }
                }
            }
            out := make([]view, 0, len(items))
            for _, a := range items {
                ip := ipBy[a.ID]
                out = append(out, view{ID: a.ID, CreatedAt: a.CreatedAt.Format(time.RFC3339), Actor: a.Actor, FunctionID: a.FunctionID, IdempotencyKey: a.IdempotencyKey, Route: a.Route, TargetServiceID: a.TargetServiceID, HashKey: a.HashKey, GameID: a.GameID, Env: a.Env, State: a.State, Mode: a.Mode, ApproveIP: ip[0], ApproveTime: ip[1], ApproveIPRegion: s.ipRegion(ip[0]), RejectIP: ip[2], RejectTime: ip[3], RejectIPRegion: s.ipRegion(ip[2])})
            }
            s.JSON(c, 200, gin.H{"approvals": out, "total": total, "page": page, "size": size})
        })
        r.GET("/api/approvals/get", func(c *gin.Context) {
            _, _, ok := s.require(c, "approvals:read")
            if !ok { return }
            id := c.Query("id")
            if id == "" { s.respondError(c, 400, "bad_request", "missing id"); return }
            a, err := s.approvals.Get(id)
            if err != nil { s.respondError(c, 404, "not_found", "not found"); return }
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
            // Lookup recent approval audit events for this approval to enrich with IP and timestamps
            var approveIP, approveTS, rejectIP, rejectTS string
            if f, err := os.Open("logs/audit.log"); err == nil {
                sc := bufio.NewScanner(f)
                for sc.Scan() {
                    line := strings.TrimSpace(sc.Text())
                    if line == "" { continue }
                    var ev auditchain.Event
                    if err := json.Unmarshal([]byte(line), &ev); err != nil { continue }
                    if ev.Meta["approval_id"] != a.ID { continue }
                    if ev.Kind == "approval_approve" {
                        approveIP = ev.Meta["ip"]
                        approveTS = ev.Time.Format(time.RFC3339)
                    } else if ev.Kind == "approval_reject" {
                        rejectIP = ev.Meta["ip"]
                        rejectTS = ev.Time.Format(time.RFC3339)
                    }
                }
                _ = f.Close()
            }
            s.JSON(c, 200, gin.H{
                "id": a.ID, "created_at": a.CreatedAt.Format(time.RFC3339), "actor": a.Actor, "function_id": a.FunctionID,
                "idempotency_key": a.IdempotencyKey, "route": a.Route, "target_service_id": a.TargetServiceID, "hash_key": a.HashKey,
                "game_id": a.GameID, "env": a.Env, "state": a.State, "mode": a.Mode, "reason": a.Reason, "payload_preview": preview,
                "approve_ip": approveIP, "approve_time": approveTS, "approve_ip_region": s.ipRegion(approveIP),
                "reject_ip": rejectIP, "reject_time": rejectTS, "reject_ip_region": s.ipRegion(rejectIP),
            })
        })
        r.POST("/api/approvals/approve", func(c *gin.Context) {
            user, _, ok := s.require(c, "approvals:approve")
            if !ok { return }
            var in struct{ ID, OTP string }
            if err := c.BindJSON(&in); err != nil || in.ID == "" { s.respondError(c, 400, "bad_request", "missing id"); return }
            a, err := s.approvals.Approve(in.ID)
            if err != nil { s.respondError(c, 409, "conflict", err.Error()); return }
		meta := map[string]string{}
		if a.Route != "" {
			meta["route"] = a.Route
		}
		if a.HashKey != "" {
			meta["hash_key"] = a.HashKey
		}
		if a.TargetServiceID != "" {
			meta["target_service_id"] = a.TargetServiceID
		}
		if a.GameID != "" {
			meta["game_id"] = a.GameID
		}
		if a.Env != "" {
			meta["env"] = a.Env
		}
            if err := s.audit.Log("approval_approve", user, a.FunctionID, map[string]string{"approval_id": a.ID, "ip": c.ClientIP()}); err != nil {
                atomic.AddInt64(&s.auditErrors, 1)
            }
		switch a.Mode {
		case "invoke":
			resp, err := s.invoker.Invoke(c, &functionv1.InvokeRequest{FunctionId: a.FunctionID, IdempotencyKey: a.IdempotencyKey, Payload: a.Payload, Metadata: meta})
                if err != nil { s.respondError(c, 500, "internal_error", "invoke failed"); return }
			out := resp.GetPayload()
			if d := s.descIndex[a.FunctionID]; d != nil && d.Transport != nil {
				if tp, ok := d.Transport["proto"].(map[string]any); ok {
					if fqn, ok2 := tp["response_fqn"].(string); ok2 && fqn != "" && s.typeReg != nil {
						if j, err2 := s.typeReg.ProtoBinToJSON(fqn, out); err2 == nil {
							out = j
						}
					}
				}
			}
			if len(out) == 0 {
				c.Status(204)
				return
			}
			c.Data(200, "application/json", out)
		case "start_job":
			resp, err := s.invoker.StartJob(c, &functionv1.InvokeRequest{FunctionId: a.FunctionID, IdempotencyKey: a.IdempotencyKey, Payload: a.Payload, Metadata: meta})
                if err != nil { s.respondError(c, 500, "internal_error", "start_job failed"); return }
                s.JSON(c, 200, resp)
            default:
                s.respondError(c, 400, "bad_request", "unknown mode")
            }
        })
        r.POST("/api/approvals/reject", func(c *gin.Context) {
            user, _, ok := s.require(c, "approvals:reject")
            if !ok { return }
            var tmp map[string]any
            if err := c.BindJSON(&tmp); err != nil { s.respondError(c, 400, "bad_request", "bad request"); return }
            id, _ := tmp["id"].(string)
            reason, _ := tmp["reason"].(string)
            if id == "" { s.respondError(c, 400, "bad_request", "missing id"); return }
            a, err := s.approvals.Reject(id, reason)
            if err != nil { s.respondError(c, 409, "conflict", err.Error()); return }
            if err := s.audit.Log("approval_reject", user, a.FunctionID, map[string]string{"approval_id": a.ID, "reason": reason, "ip": c.ClientIP()}); err != nil {
                atomic.AddInt64(&s.auditErrors, 1)
            }
            c.Status(204)
        })

        // Stream job (SSE)
        r.POST("/api/cancel_job", func(c *gin.Context) {
            user, _, ok := s.require(c, "job:cancel")
            if !ok { return }
            var in struct {
                JobID string `json:"job_id"`
            }
            if err := c.BindJSON(&in); err != nil { s.respondError(c, 400, "bad_request", "invalid payload"); return }
            if in.JobID == "" { s.respondError(c, 400, "bad_request", "missing job_id"); return }
            _ = s.audit.Log("cancel_job", user, in.JobID, map[string]string{"ip": c.ClientIP()})
            if _, err := s.invoker.CancelJob(c, &functionv1.CancelJobRequest{JobId: in.JobID}); err != nil { s.respondError(c, 500, "internal_error", "cancel failed"); return }
            s.jobsSetState(in.JobID, "canceled", "")
            c.Status(204)
        })
        r.GET("/api/job_result", func(c *gin.Context) {
            if _, _, ok := s.auth(c.Request); !ok { s.respondError(c, 401, "unauthorized", "unauthorized"); return }
            if c.Request.Method != http.MethodGet {
                s.respondError(c, 405, "method_not_allowed", "method not allowed"); return
            }
            jobID := c.Query("id")
            if jobID == "" { s.respondError(c, 400, "bad_request", "missing id"); return }
            if s.locator != nil {
                addr, ok := s.locator.GetJobAddr(jobID)
                if !ok { s.respondError(c, 404, "not_found", "unknown job"); return }
                cc, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
                if err != nil { s.respondError(c, 502, "bad_gateway", err.Error()); return }
                defer cc.Close()
                cli := localv1.NewLocalControlServiceClient(cc)
                resp, err := cli.GetJobResult(c, &localv1.GetJobResultRequest{JobId: jobID})
                if err != nil { s.respondError(c, 502, "bad_gateway", err.Error()); return }
                if resp != nil {
                    st := strings.TrimSpace(resp.GetState())
                    if st == "" { st = "succeeded" }
                    s.jobsSetState(jobID, st, resp.GetError())
                }
                s.JSON(c, 200, resp)
                return
            }
            type jobFetcher interface {
                JobResult(ctx context.Context, jobID string) (string, []byte, string, error)
            }
            if jf, ok := s.invoker.(jobFetcher); ok {
                st, payload, errMsg, err := jf.JobResult(c, jobID)
                if err != nil { s.respondError(c, 502, "bad_gateway", err.Error()); return }
                if st == "" { st = "succeeded" }
                s.jobsSetState(jobID, st, errMsg)
                s.JSON(c, 200, gin.H{"state": st, "payload": payload, "error": errMsg})
                return
            }
            s.respondError(c, 501, "not_implemented", "job_result not available")
        })

    // Audit list
    r.GET("/api/audit", func(c *gin.Context) {
        _, _, ok := s.require(c, "audit:read")
        if !ok { return }
        gameID := c.Query("game_id")
        env := c.Query("env")
        actor := c.Query("actor")
        kind := c.Query("kind")
        kindsParam := strings.TrimSpace(c.Query("kinds"))
        kindSet := map[string]struct{}{}
        if kindsParam != "" {
            for _, k := range strings.Split(kindsParam, ",") {
                k = strings.TrimSpace(k)
                if k != "" { kindSet[k] = struct{}{} }
            }
        }
        ipf := strings.TrimSpace(c.Query("ip"))
        parseBound := func(v string) (time.Time, bool) {
			v = strings.TrimSpace(v)
			if v == "" {
				return time.Time{}, false
			}
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				return t, true
			}
			var n int64
			if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
				if n > 1_000_000_000_000 {
					return time.Unix(0, n*int64(time.Millisecond)), true
				}
				return time.Unix(n, 0), true
			}
			return time.Time{}, false
		}
		startT, _ := parseBound(c.Query("start"))
		endT, _ := parseBound(c.Query("end"))
		limit := 200
		if v := c.Query("limit"); v != "" {
			fmt.Sscanf(v, "%d", &limit)
		}
		if v := c.Query("size"); v != "" {
			fmt.Sscanf(v, "%d", &limit)
		}
        if limit <= 0 {
            limit = 200
        }
		offset := 0
		if v := c.Query("offset"); v != "" {
			fmt.Sscanf(v, "%d", &offset)
		}
		if v := c.Query("page"); v != "" {
			var p int
			fmt.Sscanf(v, "%d", &p)
			if p > 0 {
				offset = (p - 1) * limit
			}
		}
		type resp struct {
			Events []auditchain.Event `json:"events"`
			Total  int                `json:"total"`
		}
		all := make([]auditchain.Event, 0, limit)
		f, err := os.Open("logs/audit.log")
		if err == nil {
			defer f.Close()
			sc := bufio.NewScanner(f)
			for sc.Scan() {
				line := strings.TrimSpace(sc.Text())
				if line == "" {
					continue
				}
				var ev auditchain.Event
				if err := json.Unmarshal([]byte(line), &ev); err != nil {
					continue
				}
				if actor != "" && ev.Actor != actor {
					continue
				}
                if len(kindSet) > 0 {
                    if _, ok := kindSet[ev.Kind]; !ok { continue }
                } else if kind != "" && ev.Kind != kind {
                    continue
                }
                if gameID != "" && ev.Meta["game_id"] != gameID {
                    continue
                }
                if env != "" && ev.Meta["env"] != env {
                    continue
                }
                if ipf != "" && ev.Meta["ip"] != ipf {
                    continue
                }
				if !startT.IsZero() {
					if ev.Time.Before(startT) {
						continue
					}
				}
				if !endT.IsZero() {
					if ev.Time.After(endT) {
						continue
					}
				}
				all = append(all, ev)
			}
		}
        // Enrich with IP region when possible
        for idx := range all {
            if ip := strings.TrimSpace(all[idx].Meta["ip"]); ip != "" {
                if reg := s.ipRegion(ip); reg != "" { all[idx].Meta["ip_region"] = reg }
            }
        }
        for i, j := 0, len(all)-1; i < j; i, j = i+1, j-1 {
            all[i], all[j] = all[j], all[i]
        }
		total := len(all)
		start := offset
		if start > total {
			start = total
		}
		endi := start + limit
		if endi > total {
			endi = total
		}
		window := all[start:endi]
		s.JSON(c, 200, resp{Events: window, Total: total})
    })

    // Registry
    r.GET("/api/registry", func(c *gin.Context) {
        if _, _, ok := s.require(c, "registry:read"); !ok { return }
        type Agent struct {
            AgentID   string `json:"agent_id"`
            GameID    string `json:"game_id"`
            Env       string `json:"env"`
            RpcAddr   string `json:"rpc_addr"`
            IP        string `json:"ip"`
            Type      string `json:"type"`
            Version   string `json:"version"`
            Functions int    `json:"functions"`
            Healthy   bool   `json:"healthy"`
            ExpiresInSec int `json:"expires_in_sec"`
        }
		type Function struct {
			GameID, ID string
			Agents     int
		}
		type FuncCov struct {
			Healthy int `json:"healthy"`
			Total   int `json:"total"`
		}
		type Coverage struct {
			GameEnv   string             `json:"game_env"`
			Functions map[string]FuncCov `json:"functions"`
			Uncovered []string           `json:"uncovered"`
		}
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
                ip := ""
                if h, _, err := net.SplitHostPort(a.RPCAddr); err == nil { ip = h } else {
                    // fallback: strip suffix after ':' if any
                    if i := strings.LastIndex(a.RPCAddr, ":"); i > 0 { ip = a.RPCAddr[:i] } else { ip = a.RPCAddr }
                }
                agents = append(agents, Agent{
                    AgentID: a.AgentID,
                    GameID: a.GameID,
                    Env: a.Env,
                    RpcAddr: a.RPCAddr,
                    IP: ip,
                    Type: "agent",
                    Version: a.Version,
                    Functions: len(a.Functions),
                    Healthy: healthy,
                    ExpiresInSec: exp,
                })
            }
			fnCountAll := map[string]map[string]int{}
			fnCountHealthy := map[string]map[string]int{}
			for _, a := range s.reg.AgentsUnsafe() {
				isHealthy := now.Before(a.ExpireAt)
				for fid := range a.Functions {
					if fnCountAll[a.GameID] == nil {
						fnCountAll[a.GameID] = map[string]int{}
					}
					fnCountAll[a.GameID][fid]++
					if isHealthy {
						if fnCountHealthy[a.GameID] == nil {
							fnCountHealthy[a.GameID] = map[string]int{}
						}
						fnCountHealthy[a.GameID][fid]++
					}
				}
			}
			for gid, m := range fnCountHealthy {
				for fid, c2 := range m {
					functions = append(functions, Function{GameID: gid, ID: fid, Agents: c2})
				}
			}
			for k, fns := range s.assignments {
				parts := strings.SplitN(k, "|", 2)
				gid := parts[0]
				cov := map[string]FuncCov{}
				uncovered := []string{}
				for _, fid := range fns {
					h := fnCountHealthy[gid][fid]
					t := fnCountAll[gid][fid]
					cov[fid] = FuncCov{Healthy: h, Total: t}
					if h == 0 {
						uncovered = append(uncovered, fid)
					}
				}
				coverage = append(coverage, Coverage{GameEnv: k, Functions: cov, Uncovered: uncovered})
			}
			s.reg.Mu().RUnlock()
		}
		s.JSON(c, 200, gin.H{"agents": agents, "functions": functions, "assignments": s.assignments, "coverage": coverage})
	})

        // Function instances
        r.GET("/api/function_instances", func(c *gin.Context) {
            if _, _, ok := s.require(c, "registry:read"); !ok { return }
            gameID := c.Query("game_id")
            fid := c.Query("function_id")
		type Inst struct{ AgentID, ServiceID, Addr, Version string }
		var out []Inst
		if s.reg != nil {
			s.reg.Mu().RLock()
			for _, a := range s.reg.AgentsUnsafe() {
				if gameID != "" && a.GameID != gameID {
					continue
				}
				if fid != "" {
					if _, ok := a.Functions[fid]; !ok {
						continue
					}
				}
				cc, err := grpc.Dial(a.RPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
				if err != nil {
					continue
				}
				cli := localv1.NewLocalControlServiceClient(cc)
				resp, err := cli.ListLocal(c, &localv1.ListLocalRequest{})
				_ = cc.Close()
				if err != nil || resp == nil {
					continue
				}
				for _, lf := range resp.Functions {
					if fid != "" && lf.Id != fid {
						continue
					}
					for _, inst := range lf.Instances {
						out = append(out, Inst{AgentID: a.AgentID, ServiceID: inst.ServiceId, Addr: inst.Addr, Version: inst.Version})
					}
				}
			}
			s.reg.Mu().RUnlock()
		}
		s.JSON(c, 200, gin.H{"instances": out})
	})

        // Stream job events (SSE)
        r.GET("/api/stream_job", func(c *gin.Context) {
            jobID := c.Query("id")
            if jobID == "" { s.respondError(c, http.StatusBadRequest, "bad_request", "missing id"); return }
            c.Writer.Header().Set("Content-Type", "text/event-stream")
            c.Writer.Header().Set("Cache-Control", "no-cache")
            c.Writer.Header().Set("Connection", "keep-alive")
            flusher, ok := c.Writer.(http.Flusher)
            if !ok { s.respondError(c, http.StatusInternalServerError, "internal_error", "stream unsupported"); return }
            stream, err := s.invoker.StreamJob(c, &functionv1.JobStreamRequest{JobId: jobID})
            if err != nil { s.respondError(c, http.StatusInternalServerError, "internal_error", err.Error()); return }
            enc := jsonAPI.NewEncoder(c.Writer)
            for {
                ev, err := stream.Recv()
                if err != nil {
                    return
                }
			fmt.Fprintf(c.Writer, "event: %s\n", ev.GetType())
			fmt.Fprintf(c.Writer, "data: ")
			_ = enc.Encode(ev)
			fmt.Fprint(c.Writer, "\n")
			flusher.Flush()
			if ev.GetType() == "done" || ev.GetType() == "error" {
				return
			}
		}
	})

        // Signed URL
        r.GET("/api/signed_url", func(c *gin.Context) {
            if _, _, ok := s.auth(c.Request); !ok { s.respondError(c, 401, "unauthorized", "unauthorized"); return }
            if s.obj == nil { s.respondError(c, 503, "unavailable", "storage not available"); return }
            key := c.Query("key")
            if key == "" { s.respondError(c, 400, "bad_request", "missing key"); return }
            method := c.Query("op")
            if method == "" {
                method = "GET"
            }
            exp := s.objConf.SignedURLTTL
            if v := c.Query("ttl"); v != "" {
                if d, err := time.ParseDuration(v); err == nil {
                    exp = d
                }
            }
            url, err := s.obj.SignedURL(c, key, method, exp)
            if err != nil { s.respondError(c, 500, "internal_error", err.Error()); return }
            s.JSON(c, 200, gin.H{"url": url})
        })

	// Prom metrics text
	r.GET("/metrics.prom", func(c *gin.Context) {
		w := c.Writer
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
	})

		// Register optional feature routes
		if s.certStore != nil {
			s.addCertificateRoutes(r)
		}

        // Support system routes (tickets/faq/feedback)
        s.addSupportRoutes(r)
        // Ops routes (/api/ops/*)
        s.addOpsRoutes(r)
        // Analytics routes (/api/analytics/*)
        s.addAnalyticsRoutes(r)

	// Static files
	staticDir := "web/dist"
	if st, err := os.Stat(staticDir); err != nil || !st.IsDir() {
		staticDir = "web/static"
	}
	// NOTE: Gin 不允许根通配和前缀共存，这里将静态资源挂到 /static
	r.Static("/static", staticDir)
	// 根路径返回 index.html（若存在），便于 SPA 在生产环境直接托管
	if _, err := os.Stat(filepath.Join(staticDir, "index.html")); err == nil {
		r.GET("/", func(c *gin.Context) { c.File(filepath.Join(staticDir, "index.html")) })
		// 非 /api/* 的未知路由回退到 index.html，支持 SPA 刷新
		r.NoRoute(func(c *gin.Context) {
			p := c.Request.URL.Path
			if strings.HasPrefix(p, "/api/") || p == "/metrics" || strings.HasPrefix(p, "/metrics") {
				c.Status(http.StatusNotFound)
				return
			}
			c.File(filepath.Join(staticDir, "index.html"))
		})
	}
	if strings.ToLower(s.objConf.Driver) == "file" && s.objConf.BaseDir != "" {
		r.Static("/uploads", s.objConf.BaseDir)
	}
	// Serve pack static
	r.Static("/pack_static", s.packDir)
	return r
}

// loadRateLimitsFromFile loads dynamic function-level rate limit rules from JSON file into memory.
func (s *Server) loadRateLimitsFromFile() {
    path := strings.TrimSpace(s.rateLimitsPath)
    if path == "" { return }
    b, err := os.ReadFile(path)
    if err != nil { return }
    var in struct{ Rules []struct{ Scope, Key string; LimitQPS int; Match map[string]string; Percent int } }
    if err := json.Unmarshal(b, &in); err != nil { return }
    s.mu.Lock()
    s.fnRulesAdv = nil; s.svcRulesAdv = nil
    for _, r := range in.Rules {
        scope := strings.ToLower(strings.TrimSpace(r.Scope))
        k := strings.TrimSpace(r.Key)
        if k == "" || r.LimitQPS <= 0 { continue }
        if r.Percent <= 0 { r.Percent = 100 }
        rr := rateRuleAdv{Scope: scope, Key: k, LimitQPS: r.LimitQPS, Match: r.Match, Percent: r.Percent}
        switch scope {
        case "function":
            s.fnRulesAdv = append(s.fnRulesAdv, rr)
        case "service":
            s.svcRulesAdv = append(s.svcRulesAdv, rr)
        }
    }
    // Build simple maps for backward usage
    for _, rr := range s.fnRulesAdv { if len(rr.Match)==0 && rr.Percent>=100 { s.rateLimitRules[rr.Key] = rr.LimitQPS } }
    for _, rr := range s.svcRulesAdv { if len(rr.Match)==0 && rr.Percent>=100 { s.serviceRateRules[rr.Key] = rr.LimitQPS } }
    s.mu.Unlock()
}

// saveRateLimitsToFile persists dynamic rate limit rules to JSON file atomically.
func (s *Server) saveRateLimitsToFile() error {
    path := strings.TrimSpace(s.rateLimitsPath)
    if path == "" { return nil }
    s.mu.Lock()
    arr := make([]map[string]any, 0, len(s.rateLimitRules))
    if len(s.fnRulesAdv) > 0 || len(s.svcRulesAdv) > 0 {
        for _, rr := range s.fnRulesAdv {
            m := map[string]any{"scope":"function","key": rr.Key, "limit_qps": rr.LimitQPS}
            if len(rr.Match)>0 { m["match"] = rr.Match }
            if rr.Percent>0 && rr.Percent<100 { m["percent"] = rr.Percent }
            arr = append(arr, m)
        }
        for _, rr := range s.svcRulesAdv {
            m := map[string]any{"scope":"service","key": rr.Key, "limit_qps": rr.LimitQPS}
            if len(rr.Match)>0 { m["match"] = rr.Match }
            if rr.Percent>0 && rr.Percent<100 { m["percent"] = rr.Percent }
            arr = append(arr, m)
        }
    } else {
        for k, v := range s.rateLimitRules { arr = append(arr, map[string]any{"scope": "function", "key": k, "limit_qps": v}) }
        for k, v := range s.serviceRateRules { arr = append(arr, map[string]any{"scope": "service", "key": k, "limit_qps": v}) }
    }
    s.mu.Unlock()
    out := map[string]any{"rules": arr}
    b, err := json.MarshalIndent(out, "", "  ")
    if err != nil { return err }
    _ = os.MkdirAll(filepath.Dir(path), 0o755)
    tmp := path + ".tmp"
    if err := os.WriteFile(tmp, b, 0o644); err != nil { return err }
    return os.Rename(tmp, path)
}

func (s *Server) ListenAndServe(addr string) error {
	log.Printf("http api listening on %s", addr)
	// All routes are registered in Gin via ginEngine()
	s.httpSrv = &http.Server{Addr: addr, Handler: s.ginEngine()}
	err := s.httpSrv.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

// Shutdown gracefully shuts down the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpSrv != nil {
		return s.httpSrv.Shutdown(ctx)
	}
	return nil
}

// --- message SSE helpers ---
func (s *Server) msgAddSub() chan struct{} {
	ch := make(chan struct{}, 8)
	s.mu.Lock()
	if s.msgSubs == nil {
		s.msgSubs = make(map[chan struct{}]struct{})
	}
	s.msgSubs[ch] = struct{}{}
	s.mu.Unlock()
	return ch
}
func (s *Server) msgRemoveSub(ch chan struct{}) {
	s.mu.Lock()
	if s.msgSubs != nil {
		delete(s.msgSubs, ch)
	}
	s.mu.Unlock()
	close(ch)
}
func (s *Server) msgNotify() {
	s.mu.RLock()
	for ch := range s.msgSubs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
	s.mu.RUnlock()
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// addCORS is deprecated; CORS is handled by ginCORS middleware globally.
func addCORS(w http.ResponseWriter, r *http.Request) { /* no-op */ }

// auth extracts username and roles from Authorization: Bearer <token>
func (s *Server) auth(r *http.Request) (string, []string, bool) {
	authz := r.Header.Get("Authorization")
	if strings.HasPrefix(authz, "Bearer ") && s.jwtMgr != nil {
		tok := strings.TrimPrefix(authz, "Bearer ")
		user, roles, err := s.jwtMgr.Verify(tok)
		if err == nil {
			return user, roles, true
		}
	}
	return "", nil, false
}

// maskSnapshot masks sensitive fields in payload based on descriptor UI hints.
func (s *Server) maskSnapshot(fid string, payload any) string {
	// Build sensitive keys set
	sensitive := map[string]struct{}{
		"password": {}, "pass": {}, "secret": {}, "token": {}, "api_key": {}, "apikey": {}, "authorization": {}, "auth": {}, "key": {},
	}
	if d := s.descIndex[fid]; d != nil && d.UI != nil {
		// support ui.sensitive: ["field1", "nested.field2"]
		if raw, ok := d.UI["sensitive"]; ok {
			if arr, ok := raw.([]any); ok {
				for _, v := range arr {
					if s1, ok := v.(string); ok && s1 != "" {
						sensitive[strings.ToLower(s1)] = struct{}{}
					}
				}
			}
		}
	}
	// Work on a generic map clone
	var m any = payload
	// If payload is raw JSON bytes (from proxy), try decode
	if b, ok := payload.([]byte); ok {
		var tmp any
		if err := json.Unmarshal(b, &tmp); err == nil {
			m = tmp
		}
	}
	masked := maskAny(m, sensitive)
	out, err := json.Marshal(masked)
	if err != nil {
		return "{}"
	}
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
		for i, e := range t {
			out[i] = maskAny(e, sensitive)
		}
		return out
	default:
		return t
	}
}

// --- Simple ABAC policy evaluator (==,!=,&&,||, has_role('x')) ---
type policyContext struct {
	User       string
	Roles      []string
	GameID     string
	Env        string
	FunctionID string
}

func hasRole(roles []string, want string) bool {
	for _, r := range roles {
		if r == want {
			return true
		}
	}
	return false
}
func evalAllowIf(expr string, ctx policyContext) bool {
	trim := strings.TrimSpace
	parseLit := func(s string) any {
		s = trim(s)
		if s == "true" {
			return true
		}
		if s == "false" {
			return false
		}
		if (strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"")) || (strings.HasPrefix(s, "'") && strings.HasSuffix(s, "'")) {
			return s[1 : len(s)-1]
		}
		if n, err := strconv.ParseFloat(s, 64); err == nil {
			return n
		}
		return s
	}
	get := func(path string) any {
		p := trim(path)
		switch p {
		case "user":
			return ctx.User
		case "game_id":
			return ctx.GameID
		case "env":
			return ctx.Env
		case "function_id":
			return ctx.FunctionID
		default:
			if strings.HasPrefix(p, "has_role(") && strings.HasSuffix(p, ")") {
				arg := strings.TrimSuffix(strings.TrimPrefix(p, "has_role("), ")")
				v := parseLit(arg)
				if s, ok := v.(string); ok {
					return hasRole(ctx.Roles, s)
				}
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
			if p == "" {
				continue
			}
			// support lhs op rhs or function(bool)
			if strings.Contains(p, "==") || strings.Contains(p, "!=") {
				op := "=="
				i := strings.Index(p, "==")
				j := strings.Index(p, "!=")
				if j >= 0 && (i < 0 || j < i) {
					op = "!="
					i = j
				}
				lhs := trim(p[:i])
				rhs := trim(p[i+len(op):])
				lv := get(lhs)
				rv := parseLit(rhs)
				eq := false
				switch l := lv.(type) {
				case string:
					if rs, ok := rv.(string); ok {
						eq = (l == rs)
					}
				case bool:
					if rb, ok := rv.(bool); ok {
						eq = (l == rb)
					}
				case float64:
					if rf, ok := rv.(float64); ok {
						eq = (math.Abs(l-rf) < 1e-9)
					}
				default:
					eq = false
				}
				if (op == "==" && !eq) || (op == "!=" && eq) {
					andOk = false
					break
				}
			} else {
				// bare function/identifier truthiness
				v := get(p)
				ok := false
				switch t := v.(type) {
				case bool:
					ok = t
				case string:
					ok = (t != "")
				default:
					ok = v != nil
				}
				if !ok {
					andOk = false
					break
				}
			}
		}
		if andOk {
			return true
		}
	}
	return false
}

// --- Simple token bucket ---
type rateLimiter struct {
	cap    int
	tokens float64
	last   time.Time
	rate   float64
	mu     sync.Mutex
}

func newRateLimiter(rps int) *rateLimiter {
	return &rateLimiter{cap: rps, tokens: float64(rps), last: time.Now(), rate: float64(rps)}
}
func (r *rateLimiter) Try() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	dt := now.Sub(r.last).Seconds()
	r.tokens = math.Min(float64(r.cap), r.tokens+dt*r.rate)
	r.last = now
	if r.tokens >= 1 {
		r.tokens -= 1
		return true
	}
	return false
}
func parseRPS(s string) int {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "rps") {
		s = strings.TrimSuffix(s, "rps")
	}
	if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil && n > 0 {
		return n
	}
	return 0
}
func (s *Server) getRateLimiter(fid string, cfg string) *rateLimiter {
	rps := parseRPS(cfg)
	if rps <= 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rl := s.rl[fid]
	if rl == nil {
		rl = newRateLimiter(rps)
		s.rl[fid] = rl
	}
	return rl
}
func (s *Server) getSemaphore(fid string, n int) chan struct{} {
	if n <= 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sem := s.conc[fid]
	if sem == nil || cap(sem) != n {
		sem = make(chan struct{}, n)
		s.conc[fid] = sem
	}
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

// helper
func ifEmpty(s string, d string) string {
	if strings.TrimSpace(s) == "" {
		return d
	}
	return s
}
