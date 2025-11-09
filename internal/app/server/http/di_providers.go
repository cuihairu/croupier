package httpserver

import (
    "os"
    "strings"

    auditchain "github.com/cuihairu/croupier/internal/audit/chain"
    "github.com/cuihairu/croupier/internal/security/rbac"
    jwt "github.com/cuihairu/croupier/internal/security/token"
    "github.com/cuihairu/croupier/internal/connpool"
    // local AgentStats replaces loadbalancer in http tests
    certmonitor "github.com/cuihairu/croupier/internal/infra/monitoring/certificates"
    clickhouse "github.com/ClickHouse/clickhouse-go/v2"
    dom "github.com/cuihairu/croupier/internal/ports"
    repogames "github.com/cuihairu/croupier/internal/repo/gorm/games"
    registry "github.com/cuihairu/croupier/internal/platform/registry"
    gamesvc "github.com/cuihairu/croupier/internal/service/games"
    obj "github.com/cuihairu/croupier/internal/platform/objstore"
    gmysql "gorm.io/driver/mysql"
    gpostgres "gorm.io/driver/postgres"
    gsqlite "gorm.io/driver/sqlite"
    gsqlserver "gorm.io/driver/sqlserver"
    "gorm.io/gorm"
    "context"
)

// ProvideGormDBFromEnv opens a *gorm.DB from env DB_DRIVER + DATABASE_URL (same behavior as NewServer).
func ProvideGormDBFromEnv() (*gorm.DB, error) {
    sel := strings.ToLower(strings.TrimSpace(os.Getenv("DB_DRIVER")))
    dsn := os.Getenv("DATABASE_URL")
    if sel == "" || sel == "auto" {
        if dsn != "" {
            if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") || strings.HasPrefix(dsn, "pgx://") {
                if db, err := gorm.Open(gpostgres.Open(dsn), &gorm.Config{}); err == nil { return db, nil }
            }
            if strings.HasPrefix(dsn, "mysql://") || strings.Contains(dsn, "@tcp(") {
                norm := normalizeMySQLDSN(dsn)
                if db, err := gorm.Open(gmysql.Open(norm), &gorm.Config{}); err == nil { return db, nil }
            }
            if strings.HasPrefix(dsn, "sqlserver://") {
                if db, err := gorm.Open(gsqlserver.Open(dsn), &gorm.Config{}); err == nil { return db, nil }
            }
        }
        // fallback sqlite local
        _ = os.MkdirAll("data", 0o755)
        return gorm.Open(gsqlite.Open("file:"+"data/croupier.db"), &gorm.Config{})
    }
    switch sel {
    case "postgres":
        if dsn != "" { return gorm.Open(gpostgres.Open(dsn), &gorm.Config{}) }
    case "mysql":
        if dsn != "" { return gorm.Open(gmysql.Open(normalizeMySQLDSN(dsn)), &gorm.Config{}) }
    case "sqlite":
        if dsn != "" { return gorm.Open(gsqlite.Open(dsn), &gorm.Config{}) }
        _ = os.MkdirAll("data", 0o755)
        return gorm.Open(gsqlite.Open("file:"+"data/croupier.db"), &gorm.Config{})
    case "mssql", "sqlserver":
        if dsn != "" { return gorm.Open(gsqlserver.Open(dsn), &gorm.Config{}) }
    }
    // final fallback
    _ = os.MkdirAll("data", 0o755)
    return gorm.Open(gsqlite.Open("file:"+"data/croupier.db"), &gorm.Config{})
}

// ProvideGamesDefaults loads default_envs from configs/games.json (best-effort).
func ProvideGamesDefaults() []dom.GameEnvDef {
    defs, err := gamesvc.LoadDefaultsFromFile("configs/games.json")
    if err != nil { return nil }
    return defs
}

// ProvideRBACPolicyFromPath loads Casbin policy; falls back to permissive dev policy for admin.
func ProvideRBACPolicyFromPath(path string) rbac.PolicyInterface {
    if p, err := rbac.LoadCasbinPolicy(path); err == nil {
        return p
    }
    fb := rbac.NewPolicy()
    fb.Grant("role:admin", "*")
    fb.Grant("user:admin", "*")
    return fb
}

// ProvideAuditWriterDefault returns an audit chain writer under logs/.
func ProvideAuditWriterDefault() (*auditchain.Writer, error) {
    _ = os.MkdirAll("logs", 0o755)
    return auditchain.NewWriter("logs/audit.log")
}

// ProvideJWTManagerFromSecret creates a JWT manager.
func ProvideJWTManagerFromSecret(secret string) *jwt.Manager { return jwt.NewManager(secret) }

// ProvideRBACPolicyFromEnv loads RBAC policy from RBAC_CONFIG or defaults.
func ProvideRBACPolicyFromEnv() rbac.PolicyInterface {
    path := strings.TrimSpace(os.Getenv("RBAC_CONFIG"))
    if path == "" { path = "configs/rbac.json" }
    return ProvideRBACPolicyFromPath(path)
}

// ProvideJWTManagerFromEnv loads JWT secret from JWT_SECRET or uses dev default.
func ProvideJWTManagerFromEnv() *jwt.Manager {
    sec := os.Getenv("JWT_SECRET")
    if strings.TrimSpace(sec) == "" { sec = "dev-secret" }
    return jwt.NewManager(sec)
}

// ProvideRBACPolicyAuto supports explicit Casbin model/policy via env (RBAC_MODEL/RBAC_POLICY), else falls back to RBAC_CONFIG.
func ProvideRBACPolicyAuto() rbac.PolicyInterface {
    model := strings.TrimSpace(os.Getenv("RBAC_MODEL"))
    policy := strings.TrimSpace(os.Getenv("RBAC_POLICY"))
    if model != "" && policy != "" {
        if p, err := rbac.NewCasbinPolicy(model, policy); err == nil { return p }
    }
    return ProvideRBACPolicyFromEnv()
}

// ProvideClickHouseFromEnv opens an optional ClickHouse connection from CLICKHOUSE_DSN (http or native) or returns nil.
func ProvideClickHouseFromEnv() clickhouse.Conn {
    dsn := strings.TrimSpace(os.Getenv("CLICKHOUSE_DSN"))
    if dsn == "" { return nil }
    // The server code expects http(s) host; we mimic the minimal dial used there
    addr := strings.TrimPrefix(strings.TrimPrefix(dsn, "clickhouse://"), "http://")
    host := addr
    if i := strings.Index(host, "/"); i >= 0 { host = host[:i] }
    if cc, err := clickhouse.Open(&clickhouse.Options{Addr: []string{host}}); err == nil { return cc }
    return nil
}

// ProvideCertStore constructs a certificate monitor store backed by GORM DB.
func ProvideCertStore(db *gorm.DB) *certmonitor.Store {
    cs := certmonitor.NewStore(db)
    _ = cs.AutoMigrate()
    return cs
}

// ProvideObjectStoreFromEnv opens an object store from STORAGE_* envs (best-effort).
func ProvideObjectStoreFromEnv() (obj.Store, obj.Config) {
    conf := obj.FromEnv()
    if conf.Driver == "file" && strings.TrimSpace(conf.BaseDir) == "" {
        _ = os.MkdirAll("data/uploads", 0o755)
        conf.BaseDir = "data/uploads"
    }
    if conf.Driver == "" {
        return nil, conf
    }
    if err := obj.Validate(conf); err != nil {
        return nil, conf
    }
    switch strings.ToLower(conf.Driver) {
    case "s3":
        st, err := obj.OpenS3(context.Background(), conf)
        if err == nil { return st, conf }
    case "file":
        st, err := obj.OpenFile(context.Background(), conf)
        if err == nil { return st, conf }
    case "oss":
        st, err := obj.OpenOSS(context.Background(), conf)
        if err == nil { return st, conf }
    case "cos":
        st, err := obj.OpenCOS(context.Background(), conf)
        if err == nil { return st, conf }
    }
    return nil, conf
}

// initServerWithDeps constructs Server via NewServer then injects DB/repo/service.
func initServerWithDeps(descriptorDir string, invoker FunctionInvoker, audit *auditchain.Writer, policy rbac.PolicyInterface, reg *registry.Store, jwtMgr *jwt.Manager, locator interface{ GetJobAddr(string) (string, bool) }, statsProv interface{ GetStats() map[string]*AgentStats; GetPoolStats() *connpool.PoolStats }, db *gorm.DB, repo *repogames.Repo, portRepo dom.GamesRepository, defaults []dom.GameEnvDef, svc *gamesvc.Service, certStore *certmonitor.Store, objStore obj.Store, objConf obj.Config, ch clickhouse.Conn) (*Server, error) {
    s, err := NewServer(descriptorDir, invoker, audit, policy, reg, jwtMgr, locator, statsProv)
    if err != nil { return nil, err }
    if db != nil { _ = repogames.AutoMigrate(db) }
    s.gdb = db
    s.games = repo
    s.gamesSvc = svc
    if certStore != nil { s.certStore = certStore }
    if objStore != nil { s.obj = objStore }
    s.objConf = objConf
    if ch != nil { s.ch = ch }
    return s, nil
}

// initServerAuto constructs Server using providers for audit/rbac/jwt.
func initServerAuto(descriptorDir string, invoker FunctionInvoker, reg *registry.Store, locator interface{ GetJobAddr(string) (string, bool) }, statsProv interface{ GetStats() map[string]*AgentStats; GetPoolStats() *connpool.PoolStats }, audit *auditchain.Writer, policy rbac.PolicyInterface, jwtMgr *jwt.Manager, db *gorm.DB, repo *repogames.Repo, portRepo dom.GamesRepository, defaults []dom.GameEnvDef, svc *gamesvc.Service, certStore *certmonitor.Store, objStore obj.Store, objConf obj.Config, ch clickhouse.Conn) (*Server, error) {
    s, err := NewServer(descriptorDir, invoker, audit, policy, reg, jwtMgr, locator, statsProv)
    if err != nil { return nil, err }
    if db != nil { _ = repogames.AutoMigrate(db) }
    s.gdb = db
    s.games = repo
    s.gamesSvc = svc
    if certStore != nil { s.certStore = certStore }
    if objStore != nil { s.obj = objStore }
    s.objConf = objConf
    if ch != nil { s.ch = ch }
    return s, nil
}
