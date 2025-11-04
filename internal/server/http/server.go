package httpserver

import (
	"encoding/json"
	"log"
	"log/slog"
	"net/http"

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
	"github.com/cuihairu/croupier/internal/loadbalancer"
	pack "github.com/cuihairu/croupier/internal/pack"
	appr "github.com/cuihairu/croupier/internal/server/approvals"
	"github.com/cuihairu/croupier/internal/server/games"
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
	rbac      *rbac.Policy
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
}

// can checks permission for user or any of their roles.
func (s *Server) can(user string, roles []string, perm string) bool {
	if s.rbac == nil {
		return true
	}
	if s.rbac.Can(user, perm) || s.rbac.Can(user, "*") {
		return true
	}
	for _, r := range roles {
		rn := "role:" + r
		if s.rbac.Can(rn, perm) || s.rbac.Can(rn, "*") {
			return true
		}
	}
	return false
}

type FunctionInvoker interface {
	Invoke(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.InvokeResponse, error)
	StartJob(ctx context.Context, req *functionv1.InvokeRequest) (*functionv1.StartJobResponse, error)
	StreamJob(ctx context.Context, req *functionv1.JobStreamRequest) (functionv1.FunctionService_StreamJobClient, error)
	CancelJob(ctx context.Context, req *functionv1.CancelJobRequest) (*functionv1.StartJobResponse, error)
}

func NewServer(descriptorDir string, invoker FunctionInvoker, audit *auditchain.Writer, policy *rbac.Policy, reg *registry.Store, jwtMgr *jwt.Manager, locator interface{ GetJobAddr(string) (string, bool) }, statsProv interface {
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
	s := &Server{descs: descs, descIndex: idx, invoker: invoker, audit: audit, rbac: policy, reg: reg, jwtMgr: jwtMgr, startedAt: time.Now(), locator: locator, statsProv: statsProv, fn: map[string]*fnMetrics{}, typeReg: pack.NewTypeRegistry(), packDir: descriptorDir, rl: map[string]*rateLimiter{}, conc: map[string]chan struct{}{}, assignments: map[string][]string{}, assignmentsPath: filepath.Join(descriptorDir, "assignments.json")}
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
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Game-ID, X-Env")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			c.Status(http.StatusNoContent)
			c.Abort()
			return
		}
		c.Next()
	}
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
		slog.Log(c, lvl, "http",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", st,
			"bytes", c.Writer.Size(),
			"remote", c.Request.RemoteAddr,
			"user", user,
			"dur_ms", dur.Milliseconds(),
		)
	}
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
	r.Use(s.ginCORS(), s.ginLogger(), gin.Recovery())
	// Native Gin routes for performance-sensitive or upload endpoints
	r.POST("/api/upload", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		user, roles, ok := s.auth(c.Request)
		if !ok {
			c.String(http.StatusUnauthorized, "unauthorized")
			return
		}
		if s.obj == nil {
			slog.Error("upload storage not available")
			c.String(http.StatusServiceUnavailable, "storage not available")
			return
		}
		if !s.can(user, roles, "uploads:write") {
			slog.Warn("upload forbidden", "user", user)
			c.String(http.StatusForbidden, "forbidden")
			return
		}
		constMax := int64(120 * 1024 * 1024)
		if cl := c.Request.Header.Get("Content-Length"); cl != "" {
			if n, err := strconv.ParseInt(cl, 10, 64); err == nil && n > constMax {
				c.String(http.StatusRequestEntityTooLarge, "request too large")
				return
			}
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, constMax)
		if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
			slog.Error("upload parse form", "error", err)
			c.String(http.StatusBadRequest, err.Error())
			return
		}
		f, fh, err := c.Request.FormFile("file")
		if err != nil {
			slog.Error("upload file missing", "error", err)
			c.String(http.StatusBadRequest, "missing file")
			return
		}
		defer f.Close()
		ts := time.Now().UnixNano()
		name := fh.Filename
		key := fmt.Sprintf("%s/%d_%s", user, ts, name)
		tmp, err := os.CreateTemp("", "upload-*")
		if err != nil {
			slog.Error("upload temp", "error", err)
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		defer os.Remove(tmp.Name())
		if _, err := io.Copy(tmp, f); err != nil {
			tmp.Close()
			slog.Error("upload copy", "error", err)
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		if _, err := tmp.Seek(0, io.SeekStart); err != nil {
			tmp.Close()
			slog.Error("upload seek", "error", err)
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		ct := fh.Header.Get("Content-Type")
		if err := s.obj.Put(c, key, tmp, fh.Size, ct); err != nil {
			tmp.Close()
			slog.Error("upload put", "error", err, "user", user, "key", key, "size", fh.Size, "ct", ct)
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		_ = tmp.Close()
		url, err := s.obj.SignedURL(c, key, "GET", s.objConf.SignedURLTTL)
		if err != nil {
			slog.Error("upload signed url", "error", err, "key", key)
		}
		c.JSON(http.StatusOK, gin.H{"Key": key, "URL": url})
	})
	// Games routes
	r.GET("/api/games", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		user, roles, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		if !(s.can(user, roles, "games:read") || s.can(user, roles, "games:manage")) {
			c.String(403, "forbidden")
			return
		}
		items, err := s.games.List(c)
		if err != nil {
			c.String(500, err.Error())
			return
		}
		c.JSON(200, gin.H{"games": items})
	})
	r.POST("/api/games", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		user, roles, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		if !s.can(user, roles, "games:manage") {
			c.String(403, "forbidden")
			return
		}
		var in struct{ ID uint; Name, Icon, Description string; Enabled bool }
		if err := c.BindJSON(&in); err != nil {
			c.String(400, err.Error())
			return
		}
		if in.ID == 0 {
			g := &games.Game{Name: in.Name, Icon: in.Icon, Description: in.Description, Enabled: in.Enabled}
			if err := s.games.Create(c, g); err != nil {
				c.String(500, err.Error())
				return
			}
			c.JSON(200, gin.H{"id": g.ID})
		} else {
			g, err := s.games.Get(c, in.ID)
			if err != nil {
				c.String(404, err.Error())
				return
			}
			g.Name, g.Icon, g.Description, g.Enabled = in.Name, in.Icon, in.Description, in.Enabled
			if err := s.games.Update(c, g); err != nil {
				c.String(500, err.Error())
				return
			}
			c.Status(204)
		}
	})
	r.GET("/api/games/:id", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		user, roles, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		if !(s.can(user, roles, "games:read") || s.can(user, roles, "games:manage")) {
			c.String(403, "forbidden")
			return
		}
		id64, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		g, err := s.games.Get(c, uint(id64))
		if err != nil {
			c.String(404, err.Error())
			return
		}
		c.JSON(200, g)
	})
	r.PUT("/api/games/:id", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		user, roles, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		if !s.can(user, roles, "games:manage") {
			c.String(403, "forbidden")
			return
		}
		id64, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var in struct{ Name, Icon, Description string; Enabled bool }
		if err := c.BindJSON(&in); err != nil {
			c.String(400, err.Error())
			return
		}
		g, err := s.games.Get(c, uint(id64))
		if err != nil {
			c.String(404, err.Error())
			return
		}
		if in.Name != "" { g.Name = in.Name }
		g.Icon, g.Description, g.Enabled = in.Icon, in.Description, in.Enabled
		if err := s.games.Update(c, g); err != nil {
			c.String(500, err.Error())
			return
		}
		c.Status(204)
	})
	r.DELETE("/api/games/:id", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		user, roles, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		if !s.can(user, roles, "games:manage") {
			c.String(403, "forbidden")
			return
		}
		id64, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		if err := s.games.Delete(c, uint(id64)); err != nil {
			c.String(500, err.Error())
			return
		}
		c.Status(204)
	})
	r.GET("/api/games/:id/envs", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		user, roles, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		if !(s.can(user, roles, "games:read") || s.can(user, roles, "games:manage")) {
			c.String(403, "forbidden")
			return
		}
		id64, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		envs, err := s.games.ListEnvs(c, uint(id64))
		if err != nil {
			c.String(500, err.Error())
			return
		}
		c.JSON(200, gin.H{"envs": envs})
	})
	r.POST("/api/games/:id/envs", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		user, roles, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		if !s.can(user, roles, "games:manage") {
			c.String(403, "forbidden")
			return
		}
		id64, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var in struct{ Env string }
		if err := c.BindJSON(&in); err != nil {
			c.String(400, err.Error())
			return
		}
		name := strings.TrimSpace(in.Env)
		if name == "" { c.String(400, "invalid env"); return }
		if err := s.games.AddEnv(c, uint(id64), name); err != nil {
			c.String(500, err.Error())
			return
		}
		c.Status(204)
	})
	r.DELETE("/api/games/:id/envs", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		user, roles, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		if !s.can(user, roles, "games:manage") {
			c.String(403, "forbidden")
			return
		}
		id64, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		env := c.Query("env")
		if env == "" {
			c.String(400, "missing env")
			return
		}
		if err := s.games.RemoveEnv(c, uint(id64), env); err != nil {
			c.String(500, err.Error())
			return
		}
		c.Status(204)
	})

	// Auth
	r.POST("/api/auth/login", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		if c.Request.Method != http.MethodPost {
			c.Status(405)
			return
		}
		if s.userRepo == nil || s.jwtMgr == nil {
			c.String(503, "auth disabled")
			return
		}
		var in struct{ Username, Password string }
		if err := c.BindJSON(&in); err != nil {
			c.String(400, err.Error())
			return
		}
		ur, err := s.userRepo.Verify(c, in.Username, in.Password)
		if err != nil {
			c.String(401, "unauthorized")
			return
		}
		roles := []string{}
		if rs, err := s.userRepo.ListUserRoles(c, ur.ID); err == nil {
			for _, rr := range rs {
				roles = append(roles, rr.Name)
			}
		}
		tok, _ := s.jwtMgr.Sign(in.Username, roles, 8*time.Hour)
		c.JSON(200, gin.H{"token": tok, "user": gin.H{"username": in.Username, "roles": roles}})
	})
	r.GET("/api/auth/me", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		user, roles, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		c.JSON(200, gin.H{"username": user, "roles": roles})
	})

	// Descriptors
	r.GET("/api/descriptors", func(c *gin.Context) { addCORS(c.Writer, c.Request); c.JSON(200, s.descs) })
	r.GET("/healthz", func(c *gin.Context) { c.String(200, "ok") })
	r.GET("/metrics", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
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
		addCORS(c.Writer, c.Request)
		id := c.Query("id")
		if id == "" {
			c.String(400, "missing id")
			return
		}
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
		c.JSON(200, gin.H{"schema": schema, "uischema": uischema})
	})

	// Packs management
	r.POST("/api/packs/import", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		user, _, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		if s.rbac != nil && !(s.rbac.Can(user, "packs:import") || s.rbac.Can(user, "*")) {
			c.String(403, "forbidden")
			return
		}
		if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
			c.String(400, err.Error())
			return
		}
		f, hdr, err := c.Request.FormFile("file")
		if err != nil {
			c.String(400, "missing file")
			return
		}
		defer f.Close()
		tmpPath := filepath.Join(os.TempDir(), hdr.Filename)
		out, err := os.Create(tmpPath)
		if err != nil {
			c.String(500, err.Error())
			return
		}
		if _, err := io.Copy(out, f); err != nil {
			out.Close()
			c.String(500, err.Error())
			return
		}
		_ = out.Close()
		if err := extractPack(tmpPath, s.packDir); err != nil {
			c.String(500, err.Error())
			return
		}
		if descs, err := descriptor.LoadAll(s.packDir); err == nil {
			idx := map[string]*descriptor.Descriptor{}
			for _, d := range descs {
				idx[d.ID] = d
			}
			s.descs = descs
			s.descIndex = idx
		}
		_ = s.typeReg.LoadFDSFromDir(s.packDir)
		c.JSON(200, gin.H{"ok": true})
	})
	r.GET("/api/packs/list", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		maniPath := filepath.Join(s.packDir, "manifest.json")
		b, err := os.ReadFile(maniPath)
		if err != nil {
			c.String(404, "manifest not found")
			return
		}
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
		c.JSON(200, gin.H{"manifest": mani, "counts": cts, "etag": etag, "export_auth_required": s.packsExportRequireAuth})
	})
	r.GET("/api/packs/export", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		if s.packsExportRequireAuth {
			if user, _, ok := s.auth(c.Request); !ok {
				c.String(401, "unauthorized")
				return
			} else {
				if s.rbac != nil && !(s.rbac.Can(user, "packs:export") || s.rbac.Can(user, "*")) {
					c.String(403, "forbidden")
					return
				}
			}
		}
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
		addCORS(c.Writer, c.Request)
		user, _, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		if s.rbac != nil && !(s.rbac.Can(user, "packs:reload") || s.rbac.Can(user, "*")) {
			c.String(403, "forbidden")
			return
		}
		if descs, err := descriptor.LoadAll(s.packDir); err == nil {
			idx := map[string]*descriptor.Descriptor{}
			for _, d := range descs {
				idx[d.ID] = d
			}
			s.descs = descs
			s.descIndex = idx
		}
		_ = s.typeReg.LoadFDSFromDir(s.packDir)
		c.JSON(200, gin.H{"ok": true})
	})

	// Function Components Management
	r.GET("/api/components", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		user, roles, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		if !s.can(user, roles, "components:read") {
			c.String(403, "forbidden")
			return
		}

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

		c.JSON(200, gin.H{"components": result})
	})

	r.POST("/api/components/install", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		user, roles, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		if !s.can(user, roles, "components:install") {
			c.String(403, "forbidden")
			return
		}

		if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
			c.String(400, err.Error())
			return
		}

		f, hdr, err := c.Request.FormFile("file")
		if err != nil {
			c.String(400, "missing file")
			return
		}
		defer f.Close()

		tmpPath := filepath.Join(os.TempDir(), hdr.Filename)
		out, err := os.Create(tmpPath)
		if err != nil {
			c.String(500, err.Error())
			return
		}

		if _, err := io.Copy(out, f); err != nil {
			out.Close()
			c.String(500, err.Error())
			return
		}
		_ = out.Close()

		if err := extractPack(tmpPath, "components/staging"); err != nil {
			c.String(500, err.Error())
			return
		}

		if err := s.componentMgr.InstallComponent("components/staging"); err != nil {
			c.String(500, err.Error())
			return
		}

		c.JSON(200, gin.H{"ok": true})
	})

	r.DELETE("/api/components/:id", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		user, roles, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		if !s.can(user, roles, "components:uninstall") {
			c.String(403, "forbidden")
			return
		}

		componentID := c.Param("id")
		if err := s.componentMgr.UninstallComponent(componentID); err != nil {
			c.String(500, err.Error())
			return
		}

		c.JSON(200, gin.H{"ok": true})
	})

	r.POST("/api/components/:id/enable", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		user, roles, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		if !s.can(user, roles, "components:manage") {
			c.String(403, "forbidden")
			return
		}

		componentID := c.Param("id")
		if err := s.componentMgr.EnableComponent(componentID); err != nil {
			c.String(500, err.Error())
			return
		}

		c.JSON(200, gin.H{"ok": true})
	})

	r.POST("/api/components/:id/disable", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		user, roles, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		if !s.can(user, roles, "components:manage") {
			c.String(403, "forbidden")
			return
		}

		componentID := c.Param("id")
		if err := s.componentMgr.DisableComponent(componentID); err != nil {
			c.String(500, err.Error())
			return
		}

		c.JSON(200, gin.H{"ok": true})
	})

	// Entity Management APIs
	r.GET("/api/entities", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		user, roles, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		if !s.can(user, roles, "entities:read") {
			c.String(403, "forbidden")
			return
		}

		// Load all entity definitions from components
		entities := []map[string]any{}

		// Scan all component directories for entity definitions
		componentsDir := "components"
		if _, err := os.Stat(componentsDir); os.IsNotExist(err) {
			c.JSON(200, gin.H{"entities": entities})
			return
		}

		entries, err := os.ReadDir(componentsDir)
		if err != nil {
			c.String(500, err.Error())
			return
		}

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

		c.JSON(200, gin.H{"entities": entities})
	})

	r.POST("/api/entities", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		user, roles, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		if !s.can(user, roles, "entities:create") {
			c.String(403, "forbidden")
			return
		}

		var entity map[string]any
		if err := c.BindJSON(&entity); err != nil {
			c.String(400, err.Error())
			return
		}

		// Validate required fields
		id, ok := entity["id"].(string)
		if !ok || id == "" {
			c.String(400, "missing or invalid entity id")
			return
		}

		entityType, ok := entity["type"].(string)
		if !ok || entityType != "entity" {
			c.String(400, "type must be 'entity'")
			return
		}

		// Determine target component
		component, ok := entity["component"].(string)
		if !ok || component == "" {
			c.String(400, "missing component")
			return
		}

		// Create entity file
		componentDir := filepath.Join("components", component)
		descriptorsDir := filepath.Join(componentDir, "descriptors")

		if err := os.MkdirAll(descriptorsDir, 0755); err != nil {
			c.String(500, err.Error())
			return
		}

		// Remove component field from entity data before saving
		delete(entity, "component")

		entityData, err := json.MarshalIndent(entity, "", "  ")
		if err != nil {
			c.String(500, err.Error())
			return
		}

		entityFile := filepath.Join(descriptorsDir, id+".json")
		if err := os.WriteFile(entityFile, entityData, 0644); err != nil {
			c.String(500, err.Error())
			return
		}

		c.JSON(200, gin.H{"id": id, "created": true})
	})

	r.GET("/api/entities/:id", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		user, roles, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		if !s.can(user, roles, "entities:read") {
			c.String(403, "forbidden")
			return
		}

		entityID := c.Param("id")

		// Search for entity in all components
		componentsDir := "components"
		entries, err := os.ReadDir(componentsDir)
		if err != nil {
			c.String(500, err.Error())
			return
		}

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
			c.JSON(200, entity)
			return
		}

		c.String(404, "entity not found")
	})

	r.PUT("/api/entities/:id", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		user, roles, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		if !s.can(user, roles, "entities:update") {
			c.String(403, "forbidden")
			return
		}

		entityID := c.Param("id")
		var entity map[string]any
		if err := c.BindJSON(&entity); err != nil {
			c.String(400, err.Error())
			return
		}

		// Find existing entity
		componentsDir := "components"
		entries, err := os.ReadDir(componentsDir)
		if err != nil {
			c.String(500, err.Error())
			return
		}

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
				c.String(500, err.Error())
				return
			}

			if err := os.WriteFile(entityPath, entityData, 0644); err != nil {
				c.String(500, err.Error())
				return
			}

			c.JSON(200, gin.H{"id": entityID, "updated": true})
			return
		}

		c.String(404, "entity not found")
	})

	r.DELETE("/api/entities/:id", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		user, roles, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		if !s.can(user, roles, "entities:delete") {
			c.String(403, "forbidden")
			return
		}

		entityID := c.Param("id")

		// Find and delete entity
		componentsDir := "components"
		entries, err := os.ReadDir(componentsDir)
		if err != nil {
			c.String(500, err.Error())
			return
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			entityPath := filepath.Join(componentsDir, entry.Name(), "descriptors", entityID+".json")
			if _, err := os.Stat(entityPath); os.IsNotExist(err) {
				continue
			}

			if err := os.Remove(entityPath); err != nil {
				c.String(500, err.Error())
				return
			}

			c.JSON(200, gin.H{"id": entityID, "deleted": true})
			return
		}

		c.String(404, "entity not found")
	})

	r.POST("/api/entities/validate", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		user, roles, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		if !s.can(user, roles, "entities:read") {
			c.String(403, "forbidden")
			return
		}

		var entity map[string]any
		if err := c.BindJSON(&entity); err != nil {
			c.String(400, err.Error())
			return
		}

		// Use the enhanced validation function
		errors := entityvalidation.ValidateEntityDefinition(entity)

		c.JSON(200, gin.H{
			"valid":  len(errors) == 0,
			"errors": errors,
		})
	})

	r.POST("/api/entities/:id/preview", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		user, roles, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		if !s.can(user, roles, "entities:read") {
			c.String(403, "forbidden")
			return
		}

		entityID := c.Param("id")

		// Find entity
		componentsDir := "components"
		entries, err := os.ReadDir(componentsDir)
		if err != nil {
			c.String(500, err.Error())
			return
		}

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
											"message":  "Please input " + description,
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

			c.JSON(200, gin.H{
				"entity":         entity,
				"proTableConfig": proTableConfig,
				"proFormConfig":  proFormConfig,
			})
			return
		}

		c.String(404, "entity not found")
	})

	// Schema validation endpoint
	r.POST("/api/schema/validate", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		user, roles, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		if !s.can(user, roles, "schema:validate") {
			c.String(403, "forbidden")
			return
		}

		var request struct {
			Schema map[string]any `json:"schema"`
		}
		if err := c.BindJSON(&request); err != nil {
			c.String(400, err.Error())
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

		c.JSON(200, gin.H{
			"valid":  len(schemaErrors) == 0,
			"errors": schemaErrors,
		})
	})

	// Assignments (in-memory)
	r.GET("/api/assignments", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		user, _, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		if s.rbac != nil && !(s.rbac.Can(user, "assignments:read") || s.rbac.Can(user, "*")) {
			c.String(403, "forbidden")
			return
		}
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
		c.JSON(200, gin.H{"assignments": out})
	})
	r.POST("/api/assignments", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		actor, _, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		if s.rbac != nil && !(s.rbac.Can(actor, "assignments:write") || s.rbac.Can(actor, "*")) {
			c.String(403, "forbidden")
			return
		}
		var in struct {
			GameID, Env string
			Functions   []string
		}
		if err := c.BindJSON(&in); err != nil || in.GameID == "" {
			c.String(400, "bad request")
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
			meta := map[string]string{"game_env": key, "game_id": in.GameID, "env": in.Env, "functions": strings.Join(valid, ",")}
			if len(unknown) > 0 {
				meta["unknown"] = strings.Join(unknown, ",")
			}
			if err := s.audit.Log("assignments.update", actor, key, meta); err != nil {
				atomic.AddInt64(&s.auditErrors, 1)
			}
		}
		c.JSON(200, gin.H{"ok": true, "unknown": unknown})
	})

	// Ant Design Pro demo stub
	r.Any("/api/rule", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		if c.Request.Method == http.MethodGet {
			c.JSON(200, gin.H{"data": []any{}, "total": 0, "success": true})
		} else if c.Request.Method == http.MethodPost {
			c.JSON(200, gin.H{"success": true})
		} else {
			c.Status(405)
		}
	})

	// Users and Roles management
	// Me profile
	r.GET("/api/me/profile", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		user, _, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		if s.userRepo == nil {
			c.String(503, "user repo unavailable")
			return
		}
		ur, err := s.userRepo.GetUserByUsername(c, user)
		if err != nil {
			c.String(404, "not found")
			return
		}
		roles, _ := s.userRepo.ListUserRoles(c, ur.ID)
		rn := []string{}
		for _, r0 := range roles {
			rn = append(rn, r0.Name)
		}
		c.JSON(200, gin.H{"username": ur.Username, "display_name": ur.DisplayName, "email": ur.Email, "phone": ur.Phone, "active": ur.Active, "roles": rn})
	})
	r.PUT("/api/me/profile", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		user, _, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		if s.userRepo == nil {
			c.String(503, "user repo unavailable")
			return
		}
		ur, err := s.userRepo.GetUserByUsername(c, user)
		if err != nil {
			c.String(404, "not found")
			return
		}
		var in struct{ DisplayName, Email, Phone string }
		if err := c.BindJSON(&in); err != nil {
			c.String(400, err.Error())
			return
		}
		ur.DisplayName, ur.Email, ur.Phone = in.DisplayName, in.Email, in.Phone
		if err := s.userRepo.UpdateUser(c, ur); err != nil {
			c.String(500, err.Error())
			return
		}
		c.Status(204)
	})
	r.POST("/api/me/password", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		user, _, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		if s.userRepo == nil {
			c.String(503, "user repo unavailable")
			return
		}
		var in struct{ Current, Password string }
		if err := c.BindJSON(&in); err != nil || in.Password == "" {
			c.String(400, "invalid payload")
			return
		} // verify current password if set
		if _, err := s.userRepo.Verify(c, user, in.Current); err != nil {
			c.String(401, "invalid current password")
			return
		}
		ur, err := s.userRepo.GetUserByUsername(c, user)
		if err != nil {
			c.String(404, "not found")
			return
		}
		if err := s.userRepo.SetPassword(c, ur.ID, in.Password); err != nil {
			c.String(500, err.Error())
			return
		}
		c.Status(204)
	})

	// Messages (inbox)
	r.GET("/api/messages/unread_count", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		user, _, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		if s.msgRepo == nil || s.userRepo == nil {
			c.String(503, "repo unavailable")
			return
		}
		ur, err := s.userRepo.GetUserByUsername(c, user)
		if err != nil {
			c.String(404, "not found")
			return
		}
		// direct unread
		n1, err := s.msgRepo.UnreadCount(c, ur.ID)
		if err != nil {
			c.String(500, err.Error())
			return
		}
		// broadcast unread by roles
		rlist, _ := s.userRepo.ListUserRoles(c, ur.ID)
		roleNames := make([]string, 0, len(rlist))
		for _, r0 := range rlist {
			roleNames = append(roleNames, r0.Name)
		}
		n2, err := s.msgRepo.Broadcast().UnreadCount(c, ur.ID, roleNames)
		if err != nil {
			c.String(500, err.Error())
			return
		}
		c.JSON(200, gin.H{"count": n1 + n2})
	})
	r.GET("/api/messages", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		user, _, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		if s.msgRepo == nil || s.userRepo == nil {
			c.String(503, "repo unavailable")
			return
		}
		ur, err := s.userRepo.GetUserByUsername(c, user)
		if err != nil {
			c.String(404, "not found")
			return
		}
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
		if err != nil {
			c.String(500, err.Error())
			return
		}
		rlist, _ := s.userRepo.ListUserRoles(c, ur.ID)
		roleNames := make([]string, 0, len(rlist))
		for _, r0 := range rlist {
			roleNames = append(roleNames, r0.Name)
		}
		bItems, _, err := s.msgRepo.Broadcast().List(c, ur.ID, roleNames, unreadOnly, capN, 0)
		if err != nil {
			c.String(500, err.Error())
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
		c.JSON(200, gin.H{"messages": out, "total": total, "page": page, "size": size})
	})
	r.POST("/api/messages/read", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		user, _, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		if s.msgRepo == nil || s.userRepo == nil {
			c.String(503, "repo unavailable")
			return
		}
		ur, err := s.userRepo.GetUserByUsername(c, user)
		if err != nil {
			c.String(404, "not found")
			return
		}
		var in struct {
			IDs          []uint `json:"ids"`
			BroadcastIDs []uint `json:"broadcast_ids"`
		}
		if err := c.BindJSON(&in); err != nil {
			c.String(400, err.Error())
			return
		}
		if err := s.msgRepo.MarkRead(c, ur.ID, in.IDs); err != nil {
			c.String(500, err.Error())
			return
		}
		if len(in.BroadcastIDs) > 0 {
			if err := s.msgRepo.Broadcast().MarkRead(c, ur.ID, in.BroadcastIDs); err != nil {
				c.String(500, err.Error())
				return
			}
		}
		s.msgNotify()
		c.Status(204)
	})
	// Admin send message to a user (requires messages:send or users:manage/admin)
	r.POST("/api/messages", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		actor, roles, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		if !(s.can(actor, roles, "messages:send") || s.can(actor, roles, "users:manage") || s.can(actor, roles, "admin")) {
			c.String(403, "forbidden")
			return
		}
		if s.msgRepo == nil || s.userRepo == nil {
			c.String(503, "repo unavailable")
			return
		}
		// Direct or broadcast based on body
		var raw map[string]any
		if err := c.BindJSON(&raw); err != nil {
			c.String(400, err.Error())
			return
		}
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
				if err != nil {
					c.String(404, "user not found")
					return
				}
				toID = ur.ID
			} else {
				c.String(400, "missing recipient")
				return
			}
			var fromID *uint
			if ur, err := s.userRepo.GetUserByUsername(c, actor); err == nil {
				fromID = &ur.ID
			}
			m := &msgsgorm.MessageRecord{ToUserID: toID, FromUserID: fromID, Title: in.Title, Content: in.Content, Type: in.Type}
			if err := s.msgRepo.Create(c, m); err != nil {
				c.String(500, err.Error())
				return
			}
			s.msgNotify()
			c.JSON(200, gin.H{"id": m.ID, "kind": "direct"})
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
		if err := msgsgorm.NewBroadcastRepo(s.gdb).Create(c, bm, in2.Audience.Roles); err != nil {
			c.String(500, err.Error())
			return
		}
		s.msgNotify()
		c.JSON(200, gin.H{"id": bm.ID, "kind": "broadcast"})
	})

	// Messages unread-count SSE stream (auth via Authorization or token query for EventSource)
	r.GET("/api/messages/stream", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
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
			c.String(401, "unauthorized")
			return
		}
		// Setup SSE
		w := c.Writer
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		flusher, okf := w.(http.Flusher)
		if !okf {
			c.String(500, "stream unsupported")
			return
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
		addCORS(c.Writer, c.Request)
		actor, roles, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		can := s.can(actor, roles, "users:read") || s.can(actor, roles, "users:manage")
		if !can {
			c.String(403, "forbidden")
			return
		}
		if s.userRepo == nil {
			c.String(503, "user repo unavailable")
			return
		}
		users, err := s.userRepo.ListUsers(c)
		if err != nil {
			c.String(500, err.Error())
			return
		} // attach roles per user
		out := make([]map[string]any, 0, len(users))
		for _, u := range users {
			rlist, _ := s.userRepo.ListUserRoles(c, u.ID)
			rn := []string{}
			for _, r0 := range rlist {
				rn = append(rn, r0.Name)
			}
			out = append(out, map[string]any{"id": u.ID, "username": u.Username, "display_name": u.DisplayName, "email": u.Email, "phone": u.Phone, "active": u.Active, "roles": rn})
		}
		c.JSON(200, gin.H{"users": out})
	})
	r.POST("/api/users", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		actor, roles, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		if !s.can(actor, roles, "users:manage") {
			c.String(403, "forbidden")
			return
		}
		if s.userRepo == nil {
			c.String(503, "user repo unavailable")
			return
		}
		var in struct {
			Username, DisplayName, Email, Phone, Password string
			Active                                        bool
			Roles                                         []string
		}
		if err := c.BindJSON(&in); err != nil || in.Username == "" {
			c.String(400, "invalid payload")
			return
		}
	u := &usersgorm.UserAccount{Username: in.Username, DisplayName: in.DisplayName, Email: in.Email, Phone: in.Phone, Active: in.Active}
		if err := s.userRepo.CreateUser(c, u); err != nil {
			c.String(500, err.Error())
			return
		}
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
		c.JSON(200, gin.H{"id": u.ID})
	})
	r.PUT("/api/users/:id", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		actor, roles, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		if !s.can(actor, roles, "users:manage") {
			c.String(403, "forbidden")
			return
		}
		if s.userRepo == nil {
			c.String(503, "user repo unavailable")
			return
		}
		id64, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var in struct {
			DisplayName, Email, Phone string
			Active                    *bool
			Roles                     []string
		}
		if err := c.BindJSON(&in); err != nil {
			c.String(400, err.Error())
			return
		} // load user
	var urec *usersgorm.UserAccount // we only have GetUserByUsername; use gorm directly
	var u usersgorm.UserAccount
		if err := s.gdb.First(&u, uint(id64)).Error; err != nil {
			c.String(404, "not found")
			return
		}
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
		if err := s.userRepo.UpdateUser(c, urec); err != nil {
			c.String(500, err.Error())
			return
		}
		// update roles if provided
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
				}
			}
			// add new
			for rn, id := range want {
				if _, ok := cur[rn]; !ok {
					_ = s.userRepo.AddUserRole(c, urec.ID, id)
				}
			}
			_ = s.buildPolicyFromDB()
		}
		c.Status(204)
	})
	r.DELETE("/api/users/:id", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		actor, roles, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		if !s.can(actor, roles, "users:manage") {
			c.String(403, "forbidden")
			return
		}
		id64, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		if err := s.userRepo.DeleteUser(c, uint(id64)); err != nil {
			c.String(500, err.Error())
			return
		}
		c.Status(204)
	})
	r.POST("/api/users/:id/password", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		actor, roles, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		if !s.can(actor, roles, "users:manage") {
			c.String(403, "forbidden")
			return
		}
		var in struct{ Password string }
		if err := c.BindJSON(&in); err != nil || in.Password == "" {
			c.String(400, "invalid payload")
			return
		}
		id64, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		if err := s.userRepo.SetPassword(c, uint(id64), in.Password); err != nil {
			c.String(500, err.Error())
			return
		}
		c.Status(204)
	})

	// Roles CRUD
	r.GET("/api/roles", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		actor, roles, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		can := s.can(actor, roles, "roles:read") || s.can(actor, roles, "roles:manage")
		if !can {
			c.String(403, "forbidden")
			return
		}
		var arr []*usersgorm.RoleRecord
		arr, _ = s.userRepo.ListRoles(c)
		out := make([]map[string]any, 0, len(arr))
		for _, r0 := range arr {
			perms, _ := s.userRepo.ListRolePerms(c, r0.ID)
			out = append(out, map[string]any{"id": r0.ID, "name": r0.Name, "description": r0.Description, "perms": perms})
		}
		c.JSON(200, gin.H{"roles": out})
	})
	r.POST("/api/roles", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		actor, roles, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		if !s.can(actor, roles, "roles:manage") {
			c.String(403, "forbidden")
			return
		}
		var in struct {
			Name, Description string
			Perms             []string
		}
		if err := c.BindJSON(&in); err != nil || in.Name == "" {
			c.String(400, "invalid payload")
			return
		}
		rrec := &usersgorm.RoleRecord{Name: in.Name, Description: in.Description}
		if err := s.gdb.Create(rrec).Error; err != nil {
			c.String(500, err.Error())
			return
		}
		for _, p := range in.Perms {
			_ = s.userRepo.GrantRolePerm(c, rrec.ID, p)
		}
		_ = s.buildPolicyFromDB()
		c.JSON(200, gin.H{"id": rrec.ID})
	})
	r.PUT("/api/roles/:id", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		actor, roles, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		if !s.can(actor, roles, "roles:manage") {
			c.String(403, "forbidden")
			return
		}
		id64, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var in struct{ Name, Description string }
		if err := c.BindJSON(&in); err != nil {
			c.String(400, err.Error())
			return
		}
		var rrec usersgorm.RoleRecord
		if err := s.gdb.First(&rrec, uint(id64)).Error; err != nil {
			c.String(404, "not found")
			return
		}
		if in.Name != "" {
			rrec.Name = in.Name
		}
		if in.Description != "" || in.Description == "" {
			rrec.Description = in.Description
		}
		if err := s.gdb.Save(&rrec).Error; err != nil {
			c.String(500, err.Error())
			return
		}
		_ = s.buildPolicyFromDB()
		c.Status(204)
	})
	r.DELETE("/api/roles/:id", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		actor, roles, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		if !s.can(actor, roles, "roles:manage") {
			c.String(403, "forbidden")
			return
		}
		id64, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		if err := s.gdb.Delete(&usersgorm.RoleRecord{}, uint(id64)).Error; err != nil {
			c.String(500, err.Error())
			return
		}
		_ = s.buildPolicyFromDB()
		c.Status(204)
	})
	r.PUT("/api/roles/:id/perms", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		actor, roles, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		if !s.can(actor, roles, "roles:manage") {
			c.String(403, "forbidden")
			return
		}
		id64, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		var in struct{ Perms []string }
		if err := c.BindJSON(&in); err != nil {
			c.String(400, err.Error())
			return
		} // replace perms
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
		addCORS(c.Writer, c.Request)
		user, roles, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
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
		if err := c.BindJSON(&in); err != nil {
			c.String(400, err.Error())
			return
		}
		if d := s.descIndex[in.FunctionID]; d != nil {
			if ps := d.Params; ps != nil {
				b, _ := json.Marshal(in.Payload)
				if err := validation.ValidateJSON(ps, b); err != nil {
					c.String(400, fmt.Sprintf("payload invalid: %v", err))
					return
				}
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
		if !scopedOk {
			atomic.AddInt64(&s.rbacDenied, 1)
			c.String(403, "forbidden")
			return
		}
		if d := s.descIndex[in.FunctionID]; d != nil && d.Auth != nil {
			if expr, ok := d.Auth["allow_if"].(string); ok && expr != "" {
				ctx := policyContext{User: user, Roles: roles, GameID: gameID, Env: env, FunctionID: in.FunctionID}
				if !evalAllowIf(expr, ctx) {
					c.String(403, "forbidden")
					return
				}
			}
		}
		b, err := json.Marshal(in.Payload)
		if err != nil {
			c.String(400, err.Error())
			return
		}
		if in.IdempotencyKey == "" {
			in.IdempotencyKey = randHex(16)
		}
		traceID := randHex(8)
		masked := s.maskSnapshot(in.FunctionID, in.Payload)
		if err := s.audit.Log("invoke", user, in.FunctionID, map[string]string{"ip": c.Request.RemoteAddr, "trace_id": traceID, "game_id": gameID, "env": env, "payload_snapshot": masked}); err != nil {
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
		if rv, ok := meta["route"]; ok && rv != "lb" && rv != "broadcast" && rv != "targeted" && rv != "hash" {
			c.String(400, "invalid route")
			return
		}
		if in.HashKey != "" {
			meta["hash_key"] = in.HashKey
		}
		if meta["route"] == "hash" && meta["hash_key"] == "" {
			c.String(400, "hash_key required for hash route")
			return
		}
		if d := s.descIndex[in.FunctionID]; d != nil && d.Semantics != nil {
			if v, ok := d.Semantics["rate_limit"].(string); ok && v != "" {
				rl := s.getRateLimiter(in.FunctionID, v)
				if rl != nil && !rl.Try() {
					c.String(429, "rate limited")
					return
				}
			}
			if v, ok := d.Semantics["concurrency"].(float64); ok && v > 0 {
				sem := s.getSemaphore(in.FunctionID, int(v))
				select {
				case sem <- struct{}{}:
					defer func() { <-sem }()
				default:
					c.String(429, "too many concurrent requests")
					return
				}
			}
		}
		resp, err := s.invoker.Invoke(c, &functionv1.InvokeRequest{FunctionId: in.FunctionID, IdempotencyKey: in.IdempotencyKey, Payload: b, Metadata: meta})
		if err != nil {
			atomic.AddInt64(&s.invocationsError, 1)
			slog.Error("invoke failed", "user", user, "function_id", in.FunctionID, "trace_id", traceID, "game_id", gameID, "env", env, "route", meta["route"], "error", err.Error())
			c.String(500, err.Error())
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
		addCORS(c.Writer, c.Request)
		user, roles, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
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
		if err := c.BindJSON(&in); err != nil {
			c.String(400, err.Error())
			return
		}
		if d := s.descIndex[in.FunctionID]; d != nil {
			if ps := d.Params; ps != nil {
				b, _ := json.Marshal(in.Payload)
				if err := validation.ValidateJSON(ps, b); err != nil {
					c.String(400, fmt.Sprintf("payload invalid: %v", err))
					return
				}
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
		if !scopedOk {
			atomic.AddInt64(&s.rbacDenied, 1)
			c.String(403, "forbidden")
			return
		}
		b, _ := json.Marshal(in.Payload)
		if in.IdempotencyKey == "" {
			in.IdempotencyKey = randHex(16)
		}
		traceID := randHex(8)
		if err := s.audit.Log("start_job", user, in.FunctionID, map[string]string{"ip": c.Request.RemoteAddr, "trace_id": traceID, "game_id": gameID, "env": env}); err != nil {
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
			c.String(500, err.Error())
			return
		}
		slog.Info("start_job", "user", user, "function_id", in.FunctionID, "trace_id", traceID, "game_id", gameID, "env", env, "route", in.Route)
		atomic.AddInt64(&s.jobsStarted, 1)
		c.JSON(200, resp)
	})

	// Approvals
	r.GET("/api/approvals", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		user, roles, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		if !s.can(user, roles, "approvals:read") {
			c.String(403, "forbidden")
			return
		}
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
		items, total, err := s.approvals.List(f, appr.Page{Page: page, Size: size, Sort: sort})
		if err != nil {
			c.String(500, err.Error())
			return
		}
		type view struct{ ID, CreatedAt, Actor, FunctionID, IdempotencyKey, Route, TargetServiceID, HashKey, GameID, Env, State, Mode string }
		out := make([]view, 0, len(items))
		for _, a := range items {
			out = append(out, view{ID: a.ID, CreatedAt: a.CreatedAt.Format(time.RFC3339), Actor: a.Actor, FunctionID: a.FunctionID, IdempotencyKey: a.IdempotencyKey, Route: a.Route, TargetServiceID: a.TargetServiceID, HashKey: a.HashKey, GameID: a.GameID, Env: a.Env, State: a.State, Mode: a.Mode})
		}
		c.JSON(200, gin.H{"approvals": out, "total": total, "page": page, "size": size})
	})
	r.GET("/api/approvals/get", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		user, roles, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		if !s.can(user, roles, "approvals:read") {
			c.String(403, "forbidden")
			return
		}
		id := c.Query("id")
		if id == "" {
			c.String(400, "missing id")
			return
		}
		a, err := s.approvals.Get(id)
		if err != nil {
			c.String(404, "not found")
			return
		}
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
		c.JSON(200, gin.H{"id": a.ID, "created_at": a.CreatedAt.Format(time.RFC3339), "actor": a.Actor, "function_id": a.FunctionID, "idempotency_key": a.IdempotencyKey, "route": a.Route, "target_service_id": a.TargetServiceID, "hash_key": a.HashKey, "game_id": a.GameID, "env": a.Env, "state": a.State, "mode": a.Mode, "reason": a.Reason, "payload_preview": preview})
	})
	r.POST("/api/approvals/approve", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		user, roles, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		if !s.can(user, roles, "approvals:approve") {
			c.String(403, "forbidden")
			return
		}
		var in struct{ ID, OTP string }
		if err := c.BindJSON(&in); err != nil || in.ID == "" {
			c.String(400, "missing id")
			return
		}
		a, err := s.approvals.Approve(in.ID)
		if err != nil {
			c.String(409, err.Error())
			return
		}
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
		if err := s.audit.Log("approval_approve", user, a.FunctionID, map[string]string{"approval_id": a.ID}); err != nil {
			atomic.AddInt64(&s.auditErrors, 1)
		}
		switch a.Mode {
		case "invoke":
			resp, err := s.invoker.Invoke(c, &functionv1.InvokeRequest{FunctionId: a.FunctionID, IdempotencyKey: a.IdempotencyKey, Payload: a.Payload, Metadata: meta})
			if err != nil {
				c.String(500, err.Error())
				return
			}
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
			if err != nil {
				c.String(500, err.Error())
				return
			}
			c.JSON(200, resp)
		default:
			c.String(400, "unknown mode")
		}
	})
	r.POST("/api/approvals/reject", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		user, roles, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		if !s.can(user, roles, "approvals:reject") {
			c.String(403, "forbidden")
			return
		}
		var tmp map[string]any
		if err := c.BindJSON(&tmp); err != nil {
			c.String(400, "bad request")
			return
		}
		id, _ := tmp["id"].(string)
		reason, _ := tmp["reason"].(string)
		if id == "" {
			c.String(400, "missing id")
			return
		}
		a, err := s.approvals.Reject(id, reason)
		if err != nil {
			c.String(409, err.Error())
			return
		}
		if err := s.audit.Log("approval_reject", user, a.FunctionID, map[string]string{"approval_id": a.ID, "reason": reason}); err != nil {
			atomic.AddInt64(&s.auditErrors, 1)
		}
		c.Status(204)
	})

	// Stream job (SSE)
	r.POST("/api/cancel_job", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		user, roles, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		var in struct {
			JobID string `json:"job_id"`
		}
		if err := c.BindJSON(&in); err != nil {
			c.String(400, err.Error())
			return
		}
		if in.JobID == "" {
			c.String(400, "missing job_id")
			return
		}
		if !s.can(user, roles, "job:cancel") {
			c.String(403, "forbidden")
			return
		}
		_ = s.audit.Log("cancel_job", user, in.JobID, map[string]string{"ip": c.Request.RemoteAddr})
		if _, err := s.invoker.CancelJob(c, &functionv1.CancelJobRequest{JobId: in.JobID}); err != nil {
			c.String(500, err.Error())
			return
		}
		c.Status(204)
	})
	r.GET("/api/job_result", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		if _, _, ok := s.auth(c.Request); !ok {
			c.String(401, "unauthorized")
			return
		}
		if c.Request.Method != http.MethodGet {
			c.Status(405)
			return
		}
		jobID := c.Query("id")
		if jobID == "" {
			c.String(400, "missing id")
			return
		}
		if s.locator != nil {
			addr, ok := s.locator.GetJobAddr(jobID)
			if !ok {
				c.String(404, "unknown job")
				return
			}
			cc, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				c.String(502, err.Error())
				return
			}
			defer cc.Close()
			cli := localv1.NewLocalControlServiceClient(cc)
			resp, err := cli.GetJobResult(c, &localv1.GetJobResultRequest{JobId: jobID})
			if err != nil {
				c.String(502, err.Error())
				return
			}
			c.JSON(200, resp)
			return
		}
		type jobFetcher interface {
			JobResult(ctx context.Context, jobID string) (string, []byte, string, error)
		}
		if jf, ok := s.invoker.(jobFetcher); ok {
			st, payload, errMsg, err := jf.JobResult(c, jobID)
			if err != nil {
				c.String(502, err.Error())
				return
			}
			c.JSON(200, gin.H{"state": st, "payload": payload, "error": errMsg})
			return
		}
		c.String(501, "job_result not available")
	})

	// Audit list
	r.GET("/api/audit", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		user, _, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		if s.rbac != nil && !(s.rbac.Can(user, "audit:read") || s.rbac.Can(user, "*")) {
			c.String(403, "forbidden")
			return
		}
		gameID := c.Query("game_id")
		env := c.Query("env")
		actor := c.Query("actor")
		kind := c.Query("kind")
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
				if kind != "" && ev.Kind != kind {
					continue
				}
				if gameID != "" && ev.Meta["game_id"] != gameID {
					continue
				}
				if env != "" && ev.Meta["env"] != env {
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
		c.JSON(200, resp{Events: window, Total: total})
	})

	// Registry
	r.GET("/api/registry", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		user, _, ok := s.auth(c.Request)
		if !ok {
			c.String(401, "unauthorized")
			return
		}
		if s.rbac != nil && !(s.rbac.Can(user, "registry:read") || s.rbac.Can(user, "*")) {
			c.String(403, "forbidden")
			return
		}
		type Agent struct {
			AgentID, GameID, Env, RpcAddr string
			Functions                     int
			Healthy                       bool
			ExpiresInSec                  int
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
				if exp < 0 {
					exp = 0
				}
				agents = append(agents, Agent{AgentID: a.AgentID, GameID: a.GameID, Env: a.Env, RpcAddr: a.RPCAddr, Functions: len(a.Functions), Healthy: healthy, ExpiresInSec: exp})
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
		c.JSON(200, gin.H{"agents": agents, "functions": functions, "assignments": s.assignments, "coverage": coverage})
	})

	// Function instances
	r.GET("/api/function_instances", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		if _, _, ok := s.auth(c.Request); !ok {
			c.String(401, "unauthorized")
			return
		}
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
		c.JSON(200, gin.H{"instances": out})
	})

	// Stream job events (SSE)
	r.GET("/api/stream_job", func(c *gin.Context) {
		addCORS(c.Writer, c.Request)
		jobID := c.Query("id")
		if jobID == "" {
			c.String(http.StatusBadRequest, "missing id")
			return
		}
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		flusher, ok := c.Writer.(http.Flusher)
		if !ok {
			c.String(http.StatusInternalServerError, "stream unsupported")
			return
		}
		stream, err := s.invoker.StreamJob(c, &functionv1.JobStreamRequest{JobId: jobID})
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		enc := json.NewEncoder(c.Writer)
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
		addCORS(c.Writer, c.Request)
		if _, _, ok := s.auth(c.Request); !ok {
			c.String(401, "unauthorized")
			return
		}
		if s.obj == nil {
			c.String(503, "storage not available")
			return
		}
		key := c.Query("key")
		if key == "" {
			c.String(400, "missing key")
			return
		}
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
		if err != nil {
			c.String(500, err.Error())
			return
		}
		c.JSON(200, gin.H{"url": url})
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

func addCORS(w http.ResponseWriter, r *http.Request) {
	// Very simple CORS for dev
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Game-ID, X-Env")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
	}
}

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
