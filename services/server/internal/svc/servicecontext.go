// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package svc

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	clickhouse "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/cuihairu/croupier/internal/analytics/mq"
	appr "github.com/cuihairu/croupier/internal/platform/approvals"
	"github.com/cuihairu/croupier/internal/function/descriptor"
	"github.com/cuihairu/croupier/internal/pack"
	"github.com/cuihairu/croupier/internal/platform/objstore"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/ports"
	"github.com/cuihairu/croupier/internal/security/token"
	"github.com/cuihairu/croupier/services/server/internal/config"
	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceContext struct {
	Config config.Config

	RegistryStore *registry.Store

	assignmentsMu     sync.RWMutex
	assignments       map[string][]string
	assignmentsPath   string
	analyticsMu       sync.RWMutex
	analytics         map[string]analyticsFilter
	analyticsPath     string
	rateMu            sync.RWMutex
	rateRules         []RateLimitRule
	rateLimitsPath    string
	healthMu          sync.RWMutex
	healthChecks      []HealthCheck
	healthStatus      map[string]HealthStatus
	healthChecksPath  string
	backupsMu         sync.Mutex
	backups           []BackupEntry
	backupsDir        string
	configsPath       string
	configsMu         sync.RWMutex
	configs           map[string]*ConfigEntry
	notificationsPath string
	notificationsMu   sync.RWMutex
	notifyChannels    []NotifyChannel
	notifyRules       []NotifyRule
	edgeMu            sync.RWMutex
	edgeNodes         map[string]EdgeNode
	nodeMu            sync.Mutex
	nodeCmds          map[string][]string
	nodeStatus        map[string]NodeState
	maintenancePath   string
	maintenanceMu     sync.RWMutex
	maintenance       []MaintenanceWindow
	jobsMu            sync.Mutex
	jobs              map[string]*JobInfo
	jobsOrder         []string

	authenticator Authenticator
	authorizer    Authorizer
	userRepo      UserRepository
	jwtMgr        *token.Manager
	loginAttempts map[string][]time.Time
	loginMu       sync.Mutex

	functionMu       sync.RWMutex
	functionIndex    map[string]*descriptor.Descriptor
	descriptors      []*descriptor.Descriptor
	componentMgr     *pack.ComponentManager
	componentStaging string
	schemaDir        string
	packDir          string
	uiOverrideMu     sync.Mutex
	agentMetaToken   string
	startedAt        time.Time
	invocations      int64
	invocationsError int64
	jobsStarted      int64
	jobsError        int64
	rbacDenied       int64
	auditErrors      int64
	objStore         objstore.Store
	objConf          objstore.Config
	gamesRepo        ports.GamesRepository
	gamesSvc         ports.GamesRepository
	supportRepo      SupportRepository
	approvals        appr.Store
	analyticsQueue   mq.Queue
	ch               clickhouse.Conn
}

var (
	ErrConfigNotFound        = errors.New("config not found")
	ErrConfigVersionConflict = errors.New("config version conflict")
	ErrConfigVersionMissing  = errors.New("config version not found")
	ErrConfigInvalidInput    = errors.New("invalid config input")
)

type analyticsFilter struct {
	Events          []string `json:"events"`
	PaymentsEnabled bool     `json:"payments_enabled"`
	SampleGlobal    int      `json:"sample_global,omitempty"`
}

const DefaultSampleGlobal = 100

type RateLimitRule struct {
	Scope    string            `json:"scope"`
	Key      string            `json:"key"`
	LimitQPS int               `json:"limit_qps"`
	Match    map[string]string `json:"match,omitempty"`
	Percent  int               `json:"percent,omitempty"`
}

type HealthCheck struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Target      string `json:"target"`
	Expect      string `json:"expect,omitempty"`
	IntervalSec int    `json:"interval_sec,omitempty"`
	TimeoutMs   int    `json:"timeout_ms,omitempty"`
	Region      string `json:"region,omitempty"`
}

type HealthStatus struct {
	ID        string    `json:"id"`
	OK        bool      `json:"ok"`
	LatencyMs int64     `json:"latency_ms"`
	Error     string    `json:"error,omitempty"`
	CheckedAt time.Time `json:"checked_at"`
}

type BackupEntry struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Target    string    `json:"target"`
	Path      string    `json:"path"`
	Size      int64     `json:"size"`
	Status    string    `json:"status"`
	Error     string    `json:"error,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type ConfigEntry struct {
	ID       string          `json:"id"`
	GameID   string          `json:"game_id"`
	Env      string          `json:"env"`
	Format   string          `json:"format"`
	Latest   int             `json:"latest_version"`
	Versions []ConfigVersion `json:"versions"`
}

type ConfigVersion struct {
	Version   int       `json:"version"`
	Content   string    `json:"content"`
	Message   string    `json:"message"`
	Editor    string    `json:"editor"`
	CreatedAt time.Time `json:"created_at"`
	ETag      string    `json:"etag"`
	Size      int       `json:"size"`
}

type ConfigUpsertInput struct {
	GameID      string
	Env         string
	Format      string
	Content     string
	Message     string
	BaseVersion int
	Editor      string
}

type MetricsSnapshot struct {
	UptimeSeconds    float64
	Invocations      int64
	InvocationsError int64
	JobsStarted      int64
	JobsError        int64
	RbacDenied       int64
	AuditErrors      int64
}

type NotifyChannel struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	URL      string `json:"url"`
	Secret   string `json:"secret,omitempty"`
	Provider string `json:"provider,omitempty"`
	Account  string `json:"account,omitempty"`
	From     string `json:"from,omitempty"`
	To       string `json:"to,omitempty"`
}

type NotifyRule struct {
	Event         string   `json:"event"`
	Channels      []string `json:"channels"`
	ThresholdDays int      `json:"threshold_days,omitempty"`
}

type EdgeNode struct {
	ID       string
	Addr     string
	HTTPAddr string
	Version  string
	IP       string
	Region   string
	Zone     string
	LastSeen time.Time
}

type NodeState struct {
	Draining bool
}

type MaintenanceWindow struct {
	ID          string    `json:"id"`
	GameID      string    `json:"game_id"`
	Env         string    `json:"env"`
	Start       time.Time `json:"start"`
	End         time.Time `json:"end"`
	Message     string    `json:"message"`
	BlockWrites bool      `json:"block_writes"`
}

type JobInfo struct {
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

func NewServiceContext(c config.Config) *ServiceContext {
	assignPath := strings.TrimSpace(c.Registry.AssignmentsPath)
	if assignPath == "" && strings.TrimSpace(c.Descriptors.Dir) != "" {
		assignPath = filepath.Join(strings.TrimSpace(c.Descriptors.Dir), "assignments.json")
	}
	analyticsPath := strings.TrimSpace(c.Registry.AnalyticsFiltersPath)
	if analyticsPath == "" && strings.TrimSpace(c.Descriptors.Dir) != "" {
		analyticsPath = filepath.Join(strings.TrimSpace(c.Descriptors.Dir), "analytics_filters.json")
	}
	rateLimitsPath := strings.TrimSpace(c.Registry.RateLimitsPath)
	if rateLimitsPath == "" {
		rateLimitsPath = filepath.Join("data", "rate_limits.json")
	}
	healthChecksPath := filepath.Join("data", "health_checks.json")
	configsPath := filepath.Join("data", "configs.json")
	notificationsPath := filepath.Join("data", "notifications.json")
	maintenancePath := filepath.Join("data", "maintenance.json")
	backupsDir := filepath.Join("data", "backups")
	_ = os.MkdirAll(backupsDir, 0o755)
	componentDataDir := strings.TrimSpace(c.Components.DataDir)
	if componentDataDir == "" {
		componentDataDir = "data"
	}
	stagingDir := strings.TrimSpace(c.Components.StagingDir)
	if stagingDir == "" {
		stagingDir = filepath.Join(componentDataDir, "components", "staging")
	}
	if err := os.MkdirAll(stagingDir, 0o755); err != nil {
		logx.Errorf("create components staging dir: %v", err)
	}
	componentMgr := pack.NewComponentManager(componentDataDir)
	if err := componentMgr.LoadRegistry(); err != nil {
		logx.Errorf("load component registry: %v", err)
	}
	schemaDir := strings.TrimSpace(c.Schemas.Dir)
	if schemaDir == "" {
		schemaDir = filepath.Join(strings.TrimSpace(c.Descriptors.Dir), "ui")
	}
	descs, index := loadDescriptorsIndex(strings.TrimSpace(c.Descriptors.Dir))
	configEntries := loadConfigs(configsPath)
	notifyChannels, notifyRules := loadNotifications(notificationsPath)
	maintenance := loadMaintenanceWindows(maintenancePath)
	objSt, objConf := initObjectStore()
	gamesRepo := newMemoryGamesRepo()
	userRepo := newMemoryUserRepo()
	var jwtMgr *token.Manager
	if secret := strings.TrimSpace(c.Auth.JWTSecret); secret != "" {
		jwtMgr = token.NewManager(secret)
	}
	supportRepo := newMemorySupportRepo()
	analyticsQueue := mq.NewFromEnv()

	ctx := &ServiceContext{
		Config:            c,
		RegistryStore:     registry.NewStore(),
		assignments:       loadAssignments(assignPath),
		assignmentsPath:   assignPath,
		analytics:         loadAnalyticsFilters(analyticsPath),
		analyticsPath:     analyticsPath,
		rateRules:         loadRateLimitRules(rateLimitsPath),
		rateLimitsPath:    rateLimitsPath,
		healthChecks:      loadHealthChecks(healthChecksPath),
		healthChecksPath:  healthChecksPath,
		healthStatus:      map[string]HealthStatus{},
		backups:           []BackupEntry{},
		backupsDir:        backupsDir,
		configsPath:       configsPath,
		configs:           configEntries,
		notificationsPath: notificationsPath,
		notifyChannels:    notifyChannels,
		notifyRules:       notifyRules,
		edgeNodes:         map[string]EdgeNode{},
		nodeCmds:          map[string][]string{},
		nodeStatus:        map[string]NodeState{},
		maintenancePath:   maintenancePath,
		maintenance:       maintenance,
		jobs:              map[string]*JobInfo{},
		jobsOrder:         []string{},
		authorizer:        newNoopRBAC(),
		functionIndex:     index,
		descriptors:       descs,
		componentMgr:      componentMgr,
		componentStaging:  stagingDir,
		schemaDir:         schemaDir,
		packDir:           firstNonEmpty(strings.TrimSpace(c.Packs.Dir), strings.TrimSpace(c.Descriptors.Dir)),
		agentMetaToken:    strings.TrimSpace(os.Getenv("AGENT_META_TOKEN")),
		startedAt:         time.Now(),
		objStore:          objSt,
		objConf:           objConf,
		gamesRepo:         gamesRepo,
		gamesSvc:          gamesRepo,
		userRepo:          userRepo,
		jwtMgr:            jwtMgr,
		loginAttempts:     map[string][]time.Time{},
		supportRepo:       supportRepo,
		approvals:         appr.NewMemStore(),
		analyticsQueue:   analyticsQueue,
	}
	ctx.initClickHouse()
	if auth, err := newJWTAuthenticator(strings.TrimSpace(c.Auth.JWTSecret)); err == nil {
		ctx.authenticator = auth
	} else {
		logx.Errorf("init authenticator: %v", err)
		ctx.authenticator = &noopAuthenticator{}
	}
	return ctx
}

func (s *ServiceContext) AssignmentsSnapshot() map[string][]string {
	s.assignmentsMu.RLock()
	defer s.assignmentsMu.RUnlock()
	out := make(map[string][]string, len(s.assignments))
	for k, v := range s.assignments {
		out[k] = append([]string(nil), v...)
	}
	return out
}

// Authenticate validates incoming request and returns (user, roles, ok).
func (s *ServiceContext) Authenticate(r *http.Request) (string, []string, bool) {
	if s.authenticator == nil {
		return "", nil, false
	}
	return s.authenticator.Authenticate(r)
}

// EnforcePermission checks if the user with roles has specific permission.
func (s *ServiceContext) EnforcePermission(user string, roles []string, perm string) bool {
	if strings.TrimSpace(perm) == "" {
		return true
	}
	if s.authorizer == nil {
		return false
	}
	return s.authorizer.Can(user, roles, perm)
}

func (s *ServiceContext) ConfigsSnapshot() []*ConfigEntry {
	s.configsMu.RLock()
	defer s.configsMu.RUnlock()
	out := make([]*ConfigEntry, 0, len(s.configs))
	for _, entry := range s.configs {
		out = append(out, cloneConfigEntry(entry))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ID == out[j].ID {
			if out[i].GameID == out[j].GameID {
				return out[i].Env < out[j].Env
			}
			return out[i].GameID < out[j].GameID
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func (s *ServiceContext) ConfigDetail(id, gameID, env string) (*ConfigEntry, *ConfigVersion, error) {
	key := cfgKey(id, gameID, env)
	s.configsMu.RLock()
	defer s.configsMu.RUnlock()
	entry := s.configs[key]
	if entry == nil {
		return nil, nil, ErrConfigNotFound
	}
	cp := cloneConfigEntry(entry)
	var latest *ConfigVersion
	for i := range entry.Versions {
		if entry.Versions[i].Version == entry.Latest {
			cv := cloneConfigVersion(&entry.Versions[i])
			latest = &cv
			break
		}
	}
	if latest == nil && len(entry.Versions) > 0 {
		cv := cloneConfigVersion(&entry.Versions[len(entry.Versions)-1])
		latest = &cv
	}
	return cp, latest, nil
}

func (s *ServiceContext) ConfigVersions(id, gameID, env string) ([]ConfigVersion, error) {
	key := cfgKey(id, gameID, env)
	s.configsMu.RLock()
	defer s.configsMu.RUnlock()
	entry := s.configs[key]
	if entry == nil {
		return nil, ErrConfigNotFound
	}
	out := make([]ConfigVersion, len(entry.Versions))
	for i := range entry.Versions {
		out[i] = cloneConfigVersion(&entry.Versions[i])
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Version > out[j].Version
	})
	return out, nil
}

func (s *ServiceContext) ConfigVersionDetail(id, gameID, env string, version int) (ConfigVersion, error) {
	key := cfgKey(id, gameID, env)
	s.configsMu.RLock()
	defer s.configsMu.RUnlock()
	entry := s.configs[key]
	if entry == nil {
		return ConfigVersion{}, ErrConfigNotFound
	}
	for i := range entry.Versions {
		if entry.Versions[i].Version == version {
			return cloneConfigVersion(&entry.Versions[i]), nil
		}
	}
	return ConfigVersion{}, ErrConfigVersionMissing
}

func (s *ServiceContext) UpsertConfig(id string, in ConfigUpsertInput) (ConfigVersion, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return ConfigVersion{}, ErrConfigInvalidInput
	}
	format := strings.ToLower(strings.TrimSpace(in.Format))
	if format == "" {
		return ConfigVersion{}, ErrConfigInvalidInput
	}
	key := cfgKey(id, in.GameID, in.Env)
	s.configsMu.Lock()
	defer s.configsMu.Unlock()
	if s.configs == nil {
		s.configs = map[string]*ConfigEntry{}
	}
	entry := s.configs[key]
	if entry == nil {
		entry = &ConfigEntry{
			ID:     id,
			GameID: strings.TrimSpace(in.GameID),
			Env:    strings.TrimSpace(in.Env),
			Format: format,
		}
	} else if in.BaseVersion != 0 && entry.Latest != 0 && entry.Latest != in.BaseVersion {
		return ConfigVersion{}, ErrConfigVersionConflict
	}
	if strings.TrimSpace(entry.Format) == "" {
		entry.Format = format
	}
	sum := sha256.Sum256([]byte(in.Content))
	etag := hex.EncodeToString(sum[:])
	version := entry.Latest + 1
	record := ConfigVersion{
		Version:   version,
		Content:   in.Content,
		Message:   strings.TrimSpace(in.Message),
		Editor:    strings.TrimSpace(in.Editor),
		CreatedAt: time.Now(),
		ETag:      etag,
		Size:      len(in.Content),
	}
	entry.Versions = append(entry.Versions, record)
	entry.Latest = version
	s.configs[key] = entry
	if err := s.persistConfigsLocked(); err != nil {
		return ConfigVersion{}, err
	}
	return record, nil
}

func (s *ServiceContext) MetricsSnapshot() MetricsSnapshot {
	uptime := 0.0
	if !s.startedAt.IsZero() {
		uptime = time.Since(s.startedAt).Seconds()
	}
	return MetricsSnapshot{
		UptimeSeconds:    uptime,
		Invocations:      atomic.LoadInt64(&s.invocations),
		InvocationsError: atomic.LoadInt64(&s.invocationsError),
		JobsStarted:      atomic.LoadInt64(&s.jobsStarted),
		JobsError:        atomic.LoadInt64(&s.jobsError),
		RbacDenied:       atomic.LoadInt64(&s.rbacDenied),
		AuditErrors:      atomic.LoadInt64(&s.auditErrors),
	}
}

func loadAssignments(path string) map[string][]string {
	if strings.TrimSpace(path) == "" {
		return map[string][]string{}
	}
	data := map[string][]string{}
	b, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			logx.Errorf("read assignments %s: %v", path, err)
		}
		return data
	}
	if err := json.Unmarshal(b, &data); err != nil {
		logx.Errorf("parse assignments %s: %v", path, err)
		return map[string][]string{}
	}
	return data
}

func loadAnalyticsFilters(path string) map[string]analyticsFilter {
	if strings.TrimSpace(path) == "" {
		return map[string]analyticsFilter{}
	}
	data := map[string]analyticsFilter{}
	b, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			logx.Errorf("read analytics filters %s: %v", path, err)
		}
		return map[string]analyticsFilter{}
	}
	if err := json.Unmarshal(b, &data); err != nil {
		logx.Errorf("parse analytics filters %s: %v", path, err)
		return map[string]analyticsFilter{}
	}
	return data
}

func loadRateLimitRules(path string) []RateLimitRule {
	if strings.TrimSpace(path) == "" {
		return []RateLimitRule{}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return []RateLimitRule{}
	}
	var in struct {
		Rules []RateLimitRule `json:"rules"`
	}
	if err := json.Unmarshal(b, &in); err != nil {
		return []RateLimitRule{}
	}
	return in.Rules
}

func loadHealthChecks(path string) []HealthCheck {
	if strings.TrimSpace(path) == "" {
		return []HealthCheck{}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return []HealthCheck{}
	}
	var payload struct {
		Checks []HealthCheck `json:"checks"`
	}
	if err := json.Unmarshal(b, &payload); err != nil {
		return []HealthCheck{}
	}
	return payload.Checks
}

func loadConfigs(path string) map[string]*ConfigEntry {
	if strings.TrimSpace(path) == "" {
		return map[string]*ConfigEntry{}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			logx.Errorf("read configs %s: %v", path, err)
		}
		return map[string]*ConfigEntry{}
	}
	var payload map[string]*ConfigEntry
	if err := json.Unmarshal(b, &payload); err != nil {
		logx.Errorf("parse configs %s: %v", path, err)
		return map[string]*ConfigEntry{}
	}
	for k, v := range payload {
		if v == nil {
			delete(payload, k)
		}
	}
	return payload
}

func loadNotifications(path string) ([]NotifyChannel, []NotifyRule) {
	if strings.TrimSpace(path) == "" {
		return []NotifyChannel{}, []NotifyRule{}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			logx.Errorf("read notifications %s: %v", path, err)
		}
		return []NotifyChannel{}, []NotifyRule{}
	}
	var payload struct {
		Channels []NotifyChannel `json:"channels"`
		Rules    []NotifyRule    `json:"rules"`
	}
	if err := json.Unmarshal(b, &payload); err != nil {
		logx.Errorf("parse notifications %s: %v", path, err)
		return []NotifyChannel{}, []NotifyRule{}
	}
	return payload.Channels, payload.Rules
}

func loadMaintenanceWindows(path string) []MaintenanceWindow {
	if strings.TrimSpace(path) == "" {
		return []MaintenanceWindow{}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			logx.Errorf("read maintenance %s: %v", path, err)
		}
		return []MaintenanceWindow{}
	}
	var payload struct {
		Windows []MaintenanceWindow `json:"windows"`
	}
	if err := json.Unmarshal(b, &payload); err != nil {
		logx.Errorf("parse maintenance %s: %v", path, err)
		return []MaintenanceWindow{}
	}
	return payload.Windows
}

func (s *ServiceContext) persistConfigsLocked() error {
	if strings.TrimSpace(s.configsPath) == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.configsPath), 0o755); err != nil {
		return err
	}
	type kv struct {
		Key   string
		Value *ConfigEntry
	}
	arr := make([]kv, 0, len(s.configs))
	for k, v := range s.configs {
		arr = append(arr, kv{Key: k, Value: v})
	}
	sort.Slice(arr, func(i, j int) bool {
		return arr[i].Key < arr[j].Key
	})
	store := make(map[string]*ConfigEntry, len(arr))
	for _, item := range arr {
		store[item.Key] = item.Value
	}
	b, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.configsPath + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.configsPath)
}

func cfgKey(id, gameID, env string) string {
	return strings.TrimSpace(gameID) + "|" + strings.TrimSpace(env) + "|" + strings.TrimSpace(id)
}

func cloneConfigEntry(entry *ConfigEntry) *ConfigEntry {
	if entry == nil {
		return nil
	}
	cp := *entry
	if len(entry.Versions) > 0 {
		cp.Versions = make([]ConfigVersion, len(entry.Versions))
		for i := range entry.Versions {
			cp.Versions[i] = cloneConfigVersion(&entry.Versions[i])
		}
	} else {
		cp.Versions = nil
	}
	return &cp
}

func cloneConfigVersion(v *ConfigVersion) ConfigVersion {
	if v == nil {
		return ConfigVersion{}
	}
	return ConfigVersion{
		Version:   v.Version,
		Content:   v.Content,
		Message:   v.Message,
		Editor:    v.Editor,
		CreatedAt: v.CreatedAt,
		ETag:      v.ETag,
		Size:      v.Size,
	}
}

// UpdateAssignments replaces assignments for a game+env and persists snapshot.
func (s *ServiceContext) UpdateAssignments(gameID, env string, functions []string) error {
	key := fmt.Sprintf("%s|%s", gameID, env)
	s.assignmentsMu.Lock()
	s.assignments[key] = append([]string{}, functions...)
	snapshot := make(map[string][]string, len(s.assignments))
	for k, v := range s.assignments {
		snapshot[k] = append([]string{}, v...)
	}
	s.assignmentsMu.Unlock()
	if strings.TrimSpace(s.assignmentsPath) == "" {
		return nil
	}
	b, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.assignmentsPath, b, 0o644)
}

func (s *ServiceContext) AnalyticsFilter(gameID, env string) analyticsFilter {
	key := fmt.Sprintf("%s|%s", strings.TrimSpace(gameID), strings.TrimSpace(env))
	s.analyticsMu.RLock()
	data, ok := s.analytics[key]
	s.analyticsMu.RUnlock()
	if !ok {
		return analyticsFilter{
			Events:          []string{},
			PaymentsEnabled: true,
			SampleGlobal:    DefaultSampleGlobal,
		}
	}
	out := analyticsFilter{
		Events:          append([]string{}, data.Events...),
		PaymentsEnabled: data.PaymentsEnabled,
		SampleGlobal:    data.SampleGlobal,
	}
	if out.SampleGlobal <= 0 {
		out.SampleGlobal = DefaultSampleGlobal
	}
	return out
}

func (s *ServiceContext) NotificationsSnapshot() ([]NotifyChannel, []NotifyRule) {
	s.notificationsMu.RLock()
	defer s.notificationsMu.RUnlock()
	chs := make([]NotifyChannel, len(s.notifyChannels))
	copy(chs, s.notifyChannels)
	rs := make([]NotifyRule, len(s.notifyRules))
	copy(rs, s.notifyRules)
	return chs, rs
}

func (s *ServiceContext) UpdateNotifications(channels []NotifyChannel, rules []NotifyRule) error {
	s.notificationsMu.Lock()
	s.notifyChannels = append([]NotifyChannel(nil), channels...)
	s.notifyRules = append([]NotifyRule(nil), rules...)
	err := s.persistNotificationsLocked()
	s.notificationsMu.Unlock()
	return err
}

func (s *ServiceContext) persistNotificationsLocked() error {
	if strings.TrimSpace(s.notificationsPath) == "" {
		return nil
	}
	payload := map[string]any{
		"channels": s.notifyChannels,
		"rules":    s.notifyRules,
	}
	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.notificationsPath), 0o755); err != nil {
		return err
	}
	tmp := s.notificationsPath + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.notificationsPath)
}

func (s *ServiceContext) MaintenanceSnapshot() []MaintenanceWindow {
	s.maintenanceMu.RLock()
	defer s.maintenanceMu.RUnlock()
	out := make([]MaintenanceWindow, len(s.maintenance))
	copy(out, s.maintenance)
	return out
}

func (s *ServiceContext) UpdateMaintenance(windows []MaintenanceWindow) error {
	s.maintenanceMu.Lock()
	s.maintenance = append([]MaintenanceWindow(nil), windows...)
	err := s.persistMaintenanceLocked()
	s.maintenanceMu.Unlock()
	return err
}

func (s *ServiceContext) persistMaintenanceLocked() error {
	if strings.TrimSpace(s.maintenancePath) == "" {
		return nil
	}
	payload := map[string]any{
		"windows": s.maintenance,
	}
	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.maintenancePath), 0o755); err != nil {
		return err
	}
	tmp := s.maintenancePath + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.maintenancePath)
}

func (s *ServiceContext) ActiveMaintenance(now time.Time) []MaintenanceWindow {
	s.maintenanceMu.RLock()
	defer s.maintenanceMu.RUnlock()
	active := make([]MaintenanceWindow, 0, len(s.maintenance))
	for _, w := range s.maintenance {
		if w.Start.IsZero() || w.End.IsZero() {
			continue
		}
		if now.Before(w.Start) || now.After(w.End) {
			continue
		}
		active = append(active, w)
	}
	return active
}

func (s *ServiceContext) RecordEdgeNode(id, addr, httpAddr, version, region, zone string) {
	id = strings.TrimSpace(id)
	if id == "" {
		return
	}
	node := EdgeNode{
		ID:       id,
		Addr:     strings.TrimSpace(addr),
		HTTPAddr: strings.TrimSpace(httpAddr),
		Version:  strings.TrimSpace(version),
		Region:   strings.TrimSpace(region),
		Zone:     strings.TrimSpace(zone),
		IP:       hostOnly(addr),
		LastSeen: time.Now(),
	}
	s.edgeMu.Lock()
	if s.edgeNodes == nil {
		s.edgeNodes = map[string]EdgeNode{}
	}
	s.edgeNodes[id] = node
	s.edgeMu.Unlock()
}

func (s *ServiceContext) EdgeNodesSnapshot() []EdgeNode {
	s.edgeMu.RLock()
	defer s.edgeMu.RUnlock()
	out := make([]EdgeNode, 0, len(s.edgeNodes))
	for _, node := range s.edgeNodes {
		out = append(out, node)
	}
	return out
}

func (s *ServiceContext) NodeDraining(id string) bool {
	if strings.TrimSpace(id) == "" {
		return false
	}
	s.nodeMu.Lock()
	defer s.nodeMu.Unlock()
	if s.nodeStatus == nil {
		return false
	}
	return s.nodeStatus[id].Draining
}

func (s *ServiceContext) SetNodeDraining(id string, draining bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return
	}
	s.nodeMu.Lock()
	if s.nodeStatus == nil {
		s.nodeStatus = map[string]NodeState{}
	}
	st := s.nodeStatus[id]
	st.Draining = draining
	s.nodeStatus[id] = st
	s.nodeMu.Unlock()
}

func (s *ServiceContext) EnqueueNodeCommand(id, cmd string) {
	id = strings.TrimSpace(id)
	cmd = strings.TrimSpace(cmd)
	if id == "" || cmd == "" {
		return
	}
	s.nodeMu.Lock()
	if s.nodeCmds == nil {
		s.nodeCmds = map[string][]string{}
	}
	s.nodeCmds[id] = append(s.nodeCmds[id], cmd)
	s.nodeMu.Unlock()
}

func (s *ServiceContext) PopNodeCommands(id string) []string {
	id = strings.TrimSpace(id)
	if id == "" {
		return []string{}
	}
	s.nodeMu.Lock()
	defer s.nodeMu.Unlock()
	if s.nodeCmds == nil {
		return []string{}
	}
	cmds := append([]string{}, s.nodeCmds[id]...)
	s.nodeCmds[id] = nil
	return cmds
}

func (s *ServiceContext) JobsSnapshot() (map[string]*JobInfo, []string) {
	s.jobsMu.Lock()
	defer s.jobsMu.Unlock()
	order := append([]string{}, s.jobsOrder...)
	data := make(map[string]*JobInfo, len(s.jobs))
	for k, v := range s.jobs {
		if v == nil {
			continue
		}
		cp := *v
		data[k] = &cp
	}
	return data, order
}

func (s *ServiceContext) UpdateAnalyticsFilter(gameID, env string, events []string, payments bool, sample int) error {
	gameID = strings.TrimSpace(gameID)
	env = strings.TrimSpace(env)
	if gameID == "" {
		return fmt.Errorf("game id required")
	}
	if sample < 0 {
		sample = 0
	}
	if sample > 100 {
		sample = 100
	}
	key := fmt.Sprintf("%s|%s", gameID, env)
	s.analyticsMu.Lock()
	s.analytics[key] = analyticsFilter{
		Events:          append([]string{}, events...),
		PaymentsEnabled: payments,
		SampleGlobal:    sample,
	}
	err := s.persistAnalyticsFiltersLocked()
	s.analyticsMu.Unlock()
	return err
}

func (s *ServiceContext) persistAnalyticsFiltersLocked() error {
	if strings.TrimSpace(s.analyticsPath) == "" {
		return nil
	}
	b, err := json.MarshalIndent(s.analytics, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.analyticsPath, b, 0o644)
}

func (s *ServiceContext) RateLimitRules() []RateLimitRule {
	s.rateMu.RLock()
	defer s.rateMu.RUnlock()
	out := make([]RateLimitRule, len(s.rateRules))
	copy(out, s.rateRules)
	return out
}

func (s *ServiceContext) ReplaceRateLimitRules(rules []RateLimitRule) error {
	s.rateMu.Lock()
	s.rateRules = make([]RateLimitRule, len(rules))
	for i, r := range rules {
		if r.Percent <= 0 {
			r.Percent = 100
		}
		r.Scope = strings.ToLower(strings.TrimSpace(r.Scope))
		r.Key = strings.TrimSpace(r.Key)
		s.rateRules[i] = r
	}
	err := s.persistRateLimitRulesLocked()
	s.rateMu.Unlock()
	return err
}

func (s *ServiceContext) DeleteRateLimitRule(scope, key string) error {
	scope = strings.ToLower(strings.TrimSpace(scope))
	key = strings.TrimSpace(key)
	if scope == "" || key == "" {
		return fmt.Errorf("invalid scope/key")
	}
	s.rateMu.Lock()
	next := make([]RateLimitRule, 0, len(s.rateRules))
	for _, r := range s.rateRules {
		if r.Scope == scope && r.Key == key {
			continue
		}
		next = append(next, r)
	}
	s.rateRules = next
	err := s.persistRateLimitRulesLocked()
	s.rateMu.Unlock()
	return err
}

func (s *ServiceContext) persistRateLimitRulesLocked() error {
	if strings.TrimSpace(s.rateLimitsPath) == "" {
		return nil
	}
	payload := map[string]any{
		"rules": s.rateRules,
	}
	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.rateLimitsPath), 0o755); err != nil {
		return err
	}
	tmp := s.rateLimitsPath + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.rateLimitsPath)
}

func (s *ServiceContext) HealthSnapshot() ([]HealthCheck, []HealthStatus) {
	s.healthMu.RLock()
	defer s.healthMu.RUnlock()
	checks := append([]HealthCheck{}, s.healthChecks...)
	statuses := make([]HealthStatus, 0, len(s.healthStatus))
	for _, st := range s.healthStatus {
		statuses = append(statuses, st)
	}
	return checks, statuses
}

func (s *ServiceContext) UpdateHealthChecks(checks []HealthCheck) error {
	normalized := make([]HealthCheck, 0, len(checks))
	for _, hc := range checks {
		id := strings.TrimSpace(hc.ID)
		if id == "" {
			continue
		}
		kind := strings.ToLower(strings.TrimSpace(hc.Kind))
		if kind == "" {
			continue
		}
		if hc.IntervalSec <= 0 {
			hc.IntervalSec = 60
		}
		if hc.TimeoutMs <= 0 {
			hc.TimeoutMs = 1000
		}
		hc.ID = id
		hc.Kind = kind
		hc.Target = strings.TrimSpace(hc.Target)
		hc.Expect = strings.TrimSpace(hc.Expect)
		normalized = append(normalized, hc)
	}
	s.healthMu.Lock()
	s.healthChecks = normalized
	err := s.persistHealthChecksLocked()
	s.healthMu.Unlock()
	return err
}

func (s *ServiceContext) persistHealthChecksLocked() error {
	if strings.TrimSpace(s.healthChecksPath) == "" {
		return nil
	}
	data := map[string]any{
		"checks": s.healthChecks,
	}
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	_ = os.MkdirAll(filepath.Dir(s.healthChecksPath), 0o755)
	tmp := s.healthChecksPath + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.healthChecksPath)
}

func (s *ServiceContext) RunHealthChecks(id string) []HealthStatus {
	s.healthMu.RLock()
	checks := append([]HealthCheck{}, s.healthChecks...)
	s.healthMu.RUnlock()
	results := []HealthStatus{}
	for _, hc := range checks {
		if id != "" && hc.ID != id {
			continue
		}
		status := runHealthCheck(hc)
		results = append(results, status)
		s.healthMu.Lock()
		if s.healthStatus == nil {
			s.healthStatus = map[string]HealthStatus{}
		}
		s.healthStatus[hc.ID] = status
		s.healthMu.Unlock()
	}
	return results
}

func (s *ServiceContext) ListBackups() []BackupEntry {
	s.backupsMu.Lock()
	defer s.backupsMu.Unlock()
	out := make([]BackupEntry, len(s.backups))
	copy(out, s.backups)
	return out
}

func (s *ServiceContext) CreateBackup(kind, target string) (BackupEntry, error) {
	id := fmt.Sprintf("bkp-%d", time.Now().UnixNano())
	path := filepath.Join(s.backupsDir, id+".tar.gz")
	meta := map[string]any{
		"id":         id,
		"kind":       kind,
		"target":     target,
		"created_at": time.Now().Format(time.RFC3339),
	}
	if err := createBackupArchive(path, meta); err != nil {
		return BackupEntry{}, err
	}
	var size int64
	if fi, err := os.Stat(path); err == nil {
		size = fi.Size()
	}
	entry := BackupEntry{
		ID:        id,
		Kind:      kind,
		Target:    target,
		Path:      path,
		Size:      size,
		Status:    "done",
		CreatedAt: time.Now(),
	}
	s.backupsMu.Lock()
	s.backups = append([]BackupEntry{entry}, s.backups...)
	s.backupsMu.Unlock()
	return entry, nil
}

func (s *ServiceContext) DeleteBackup(id string) bool {
	s.backupsMu.Lock()
	defer s.backupsMu.Unlock()
	for i, b := range s.backups {
		if b.ID == id {
			if b.Path != "" {
				_ = os.Remove(b.Path)
			}
			s.backups = append(s.backups[:i], s.backups[i+1:]...)
			return true
		}
	}
	return false
}

func (s *ServiceContext) BackupFile(id string) (BackupEntry, bool) {
	s.backupsMu.Lock()
	defer s.backupsMu.Unlock()
	for _, b := range s.backups {
		if b.ID == id {
			return b, true
		}
	}
	return BackupEntry{}, false
}

func runHealthCheck(hc HealthCheck) HealthStatus {
	status := HealthStatus{ID: hc.ID, CheckedAt: time.Now()}
	timeout := time.Duration(hc.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = time.Second
	}
	start := time.Now()
	var err error
	switch strings.ToLower(hc.Kind) {
	case "http":
		client := &http.Client{Timeout: timeout}
		resp, e := client.Get(hc.Target)
		if e != nil {
			err = e
			break
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if hc.Expect != "" {
			if code, convErr := strconv.Atoi(hc.Expect); convErr == nil && resp.StatusCode != code {
				err = fmt.Errorf("status %d != %d", resp.StatusCode, code)
			}
		}
	case "tcp":
		conn, e := net.DialTimeout("tcp", hc.Target, timeout)
		if e != nil {
			err = e
		} else {
			conn.Close()
		}
	case "tls":
		dialer := &net.Dialer{Timeout: timeout}
		conn, e := tls.DialWithDialer(dialer, "tcp", hc.Target, &tls.Config{InsecureSkipVerify: true})
		if e != nil {
			err = e
		} else {
			conn.Close()
		}
	case "redis":
		if opt, e := redis.ParseURL(hc.Target); e == nil {
			client := redis.NewClient(opt)
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			if _, e := client.Ping(ctx).Result(); e != nil {
				err = e
			}
			client.Close()
		} else {
			err = e
		}
	case "postgres":
		err = dialHostWithDefaultPort(hc.Target, "5432", timeout)
	case "clickhouse":
		err = dialHostWithDefaultPort(hc.Target, "9000", timeout)
	case "kafka":
		err = checkKafkaBrokers(hc.Target, timeout)
	default:
		err = fmt.Errorf("unsupported kind: %s", hc.Kind)
	}
	status.LatencyMs = time.Since(start).Milliseconds()
	if err != nil {
		status.OK = false
		status.Error = err.Error()
	} else {
		status.OK = true
	}
	return status
}

func dialHostWithDefaultPort(target, defaultPort string, timeout time.Duration) error {
	u, err := url.Parse(target)
	if err != nil {
		return err
	}
	host := u.Host
	if host == "" {
		host = target
	}
	if !strings.Contains(host, ":") {
		host += ":" + defaultPort
	}
	_, err = net.DialTimeout("tcp", host, timeout)
	return err
}

func checkKafkaBrokers(target string, timeout time.Duration) error {
	brokers := strings.Split(target, ",")
	tried := 0
	var last error
	for _, b := range brokers {
		addr := strings.TrimSpace(b)
		if addr == "" {
			continue
		}
		if !strings.Contains(addr, ":") {
			addr += ":9092"
		}
		tried++
		conn, err := net.DialTimeout("tcp", addr, timeout)
		if err == nil {
			conn.Close()
			return nil
		}
		last = err
	}
	if tried == 0 {
		return fmt.Errorf("no brokers")
	}
	if last != nil {
		return last
	}
	return nil
}

func createBackupArchive(path string, manifest map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	gw := gzip.NewWriter(f)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()
	data, _ := json.MarshalIndent(manifest, "", "  ")
	hdr := &tar.Header{Name: "manifest.json", Mode: 0o644, Size: int64(len(data))}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	if _, err := tw.Write(data); err != nil {
		return err
	}
	return nil
}

func (s *ServiceContext) UpdateAgentMeta(agentID, region, zone string, labels map[string]string) bool {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" || s.RegistryStore == nil {
		return false
	}
	s.RegistryStore.Mu().Lock()
	defer s.RegistryStore.Mu().Unlock()
	if agent := s.RegistryStore.AgentsUnsafe()[agentID]; agent != nil {
		if v := strings.TrimSpace(region); v != "" {
			agent.Region = v
		}
		if v := strings.TrimSpace(zone); v != "" {
			agent.Zone = v
		}
		if len(labels) > 0 {
			if agent.Labels == nil {
				agent.Labels = map[string]string{}
			}
			for k, v := range labels {
				if strings.TrimSpace(k) == "" {
					continue
				}
				agent.Labels[k] = v
			}
		}
		return true
	}
	return false
}

func loadDescriptorsIndex(dir string) ([]*descriptor.Descriptor, map[string]*descriptor.Descriptor) {
	index := map[string]*descriptor.Descriptor{}
	if dir == "" {
		return []*descriptor.Descriptor{}, index
	}
	descs, err := descriptor.LoadAll(dir)
	if err != nil {
		logx.Errorf("load descriptors from %s: %v", dir, err)
		return []*descriptor.Descriptor{}, index
	}
	for _, d := range descs {
		if d != nil && d.ID != "" {
			index[d.ID] = d
		}
	}
	return descs, index
}

// HasFunction returns true if descriptor exists.
func (s *ServiceContext) HasFunction(id string) bool {
	if id == "" {
		return false
	}
	s.functionMu.RLock()
	defer s.functionMu.RUnlock()
	_, ok := s.functionIndex[id]
	return ok
}

func (s *ServiceContext) FunctionDescriptor(id string) *descriptor.Descriptor {
	if id == "" {
		return nil
	}
	s.functionMu.RLock()
	defer s.functionMu.RUnlock()
	if d, ok := s.functionIndex[id]; ok {
		return d
	}
	return nil
}

func (s *ServiceContext) ComponentManager() *pack.ComponentManager {
	return s.componentMgr
}

func (s *ServiceContext) ComponentStagingDir() string {
	return s.componentStaging
}

func (s *ServiceContext) FindComponent(id string) (*pack.ComponentManifest, bool, bool) {
	if s.componentMgr == nil || id == "" {
		return nil, false, false
	}
	if comp, ok := s.componentMgr.ListInstalled()[id]; ok {
		return comp, true, false
	}
	if comp, ok := s.componentMgr.ListDisabled()[id]; ok {
		return comp, true, true
	}
	return nil, false, false
}

func (s *ServiceContext) SchemaDir() string {
	return s.schemaDir
}

func (s *ServiceContext) PackDir() string {
	return s.packDir
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func (s *ServiceContext) ReloadDescriptors() {
	descs, index := loadDescriptorsIndex(s.packDir)
	s.functionMu.Lock()
	s.functionIndex = index
	s.descriptors = descs
	s.functionMu.Unlock()
}

func (s *ServiceContext) DescriptorsSnapshot() []*descriptor.Descriptor {
	s.functionMu.RLock()
	defer s.functionMu.RUnlock()
	out := make([]*descriptor.Descriptor, len(s.descriptors))
	copy(out, s.descriptors)
	return out
}

func (s *ServiceContext) uiOverridePath() (string, error) {
	if p := strings.TrimSpace(os.Getenv("CROUPIER_UI_CONFIG")); p != "" {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p, nil
		}
	}
	dir := "configs/ui"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "functions.override.json"), nil
}

func (s *ServiceContext) LoadUIOverrides() map[string]map[string]any {
	path, err := s.uiOverridePath()
	if err != nil {
		return map[string]map[string]any{}
	}
	b, err := os.ReadFile(path)
	if err != nil || len(b) == 0 {
		return map[string]map[string]any{}
	}
	var m map[string]map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return map[string]map[string]any{}
	}
	return m
}

func (s *ServiceContext) SaveUIOverride(fid string, ui map[string]any) error {
	if strings.TrimSpace(fid) == "" {
		return fmt.Errorf("function id required")
	}
	path, err := s.uiOverridePath()
	if err != nil {
		return err
	}
	s.uiOverrideMu.Lock()
	defer s.uiOverrideMu.Unlock()
	cur := s.LoadUIOverrides()
	if cur == nil {
		cur = map[string]map[string]any{}
	}
	cur[fid] = ui
	b, err := json.MarshalIndent(cur, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (s *ServiceContext) mergeDescriptor(desc *descriptor.Descriptor) {
	if desc == nil || desc.ID == "" {
		return
	}
	if s.functionIndex == nil {
		s.functionIndex = map[string]*descriptor.Descriptor{}
	}
	if existing, ok := s.functionIndex[desc.ID]; ok && existing != nil {
		*existing = *desc
		return
	}
	s.functionIndex[desc.ID] = desc
	s.descriptors = append(s.descriptors, desc)
}

func (s *ServiceContext) MergeProviderFunctions(doc []byte) {
	if len(doc) == 0 {
		return
	}
	var payload struct {
		Provider struct {
			ID      string `json:"id"`
			Version string `json:"version"`
		} `json:"provider"`
		Functions []struct {
			ID        string         `json:"id"`
			Auth      map[string]any `json:"auth"`
			Semantics map[string]any `json:"semantics"`
			Transport map[string]any `json:"transport"`
			UI        map[string]any `json:"ui"`
			Outputs   map[string]any `json:"outputs"`
		} `json:"functions"`
	}
	if err := json.Unmarshal(doc, &payload); err != nil {
		return
	}
	if len(payload.Functions) == 0 {
		return
	}
	s.functionMu.Lock()
	defer s.functionMu.Unlock()
	for _, f := range payload.Functions {
		if strings.TrimSpace(f.ID) == "" {
			continue
		}
		desc := &descriptor.Descriptor{
			ID:        f.ID,
			Version:   payload.Provider.Version,
			Auth:      f.Auth,
			Semantics: f.Semantics,
			Transport: f.Transport,
			Outputs:   f.Outputs,
			UI:        f.UI,
		}
		if ui := f.UI; ui != nil {
			if cat, ok := ui["category"].(string); ok {
				desc.Category = cat
			}
			if risk, ok := ui["risk"].(string); ok {
				desc.Risk = risk
			}
		}
		s.mergeDescriptor(desc)
	}
}

func (s *ServiceContext) AgentMetaToken() string {
	return s.agentMetaToken
}

func hostOnly(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	if idx := strings.LastIndex(addr, ":"); idx > 0 {
		return addr[:idx]
	}
	return addr
}

// Actor key for context
type actorKey struct{}

func WithActor(ctx context.Context, actor string) context.Context {
	return context.WithValue(ctx, actorKey{}, actor)
}

func ActorFromContext(ctx context.Context) string {
	v := ctx.Value(actorKey{})
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func (s *ServiceContext) GamesRepository() ports.GamesRepository {
	if s.gamesRepo == nil {
		s.gamesRepo = newMemoryGamesRepo()
	}
	return s.gamesRepo
}

func (s *ServiceContext) UserRepository() UserRepository {
	if s.userRepo == nil {
		s.userRepo = newMemoryUserRepo()
	}
	return s.userRepo
}

func (s *ServiceContext) JWTManager() *token.Manager {
	if s.jwtMgr == nil {
		if secret := strings.TrimSpace(s.Config.Auth.JWTSecret); secret != "" {
			s.jwtMgr = token.NewManager(secret)
		}
	}
	return s.jwtMgr
}

func (s *ServiceContext) AllowLogin(ip, username string) bool {
	key := strings.TrimSpace(ip) + "|" + strings.ToLower(strings.TrimSpace(username))
	now := time.Now()
	window := now.Add(-5 * time.Minute)
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	if s.loginAttempts == nil {
		s.loginAttempts = map[string][]time.Time{}
	}
	arr := s.loginAttempts[key]
	kept := arr[:0]
	for _, t := range arr {
		if t.After(window) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= 10 {
		s.loginAttempts[key] = kept
		return false
	}
	kept = append(kept, now)
	s.loginAttempts[key] = kept
	return true
}

func (s *ServiceContext) SupportRepository() SupportRepository {
	if s.supportRepo == nil {
		s.supportRepo = newMemorySupportRepo()
	}
	return s.supportRepo
}

func (s *ServiceContext) ApprovalsStore() appr.Store {
	return s.approvals
}

func (s *ServiceContext) AnalyticsQueue() mq.Queue {
	return s.analyticsQueue
}

func (s *ServiceContext) ClickHouse() clickhouse.Conn {
	return s.ch
}

func (s *ServiceContext) initClickHouse() {
	dsn := strings.TrimSpace(os.Getenv("CLICKHOUSE_DSN"))
	if dsn == "" {
		return
	}
	opts, err := clickhouse.ParseDSN(dsn)
	if err != nil {
		logx.Errorf("parse clickhouse dsn: %v", err)
		return
	}
	conn, err := clickhouse.Open(opts)
	if err != nil {
		logx.Errorf("connect clickhouse: %v", err)
		return
	}
	s.ch = conn
	logx.Infof("clickhouse connected: %v", opts.Addr)
}

func initObjectStore() (objstore.Store, objstore.Config) {
	conf := objstore.FromEnv()
	if strings.TrimSpace(conf.Driver) == "" {
		return nil, conf
	}
	if strings.EqualFold(conf.Driver, "file") && strings.TrimSpace(conf.BaseDir) == "" {
		conf.BaseDir = filepath.Join("data", "uploads")
		_ = os.MkdirAll(conf.BaseDir, 0o755)
	}
	if conf.SignedURLTTL <= 0 {
		conf.SignedURLTTL = 10 * time.Minute
	}
	if err := objstore.Validate(conf); err != nil {
		logx.Errorf("object storage disabled: %v", err)
		return nil, conf
	}
	ctx := context.Background()
	var store objstore.Store
	switch strings.ToLower(conf.Driver) {
	case "s3":
		if st, err := objstore.OpenS3(ctx, conf); err == nil {
			store = st
		} else {
			logx.Errorf("init s3 store: %v", err)
		}
	case "file":
		if st, err := objstore.OpenFile(ctx, conf); err == nil {
			store = st
		} else {
			logx.Errorf("init file store: %v", err)
		}
	case "oss":
		if st, err := objstore.OpenOSS(ctx, conf); err == nil {
			store = st
		} else {
			logx.Errorf("init oss store: %v", err)
		}
	case "cos":
		if st, err := objstore.OpenCOS(ctx, conf); err == nil {
			store = st
		} else {
			logx.Errorf("init cos store: %v", err)
		}
	default:
		logx.Errorf("object storage driver %s not supported", conf.Driver)
	}
	return store, conf
}

func (s *ServiceContext) ObjStore() objstore.Store {
	return s.objStore
}

func (s *ServiceContext) ObjConfig() objstore.Config {
	return s.objConf
}
