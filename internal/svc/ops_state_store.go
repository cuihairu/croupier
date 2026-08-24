package svc

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// OpsStateStore persists mutable ops configuration (maintenance windows, notifications, health checks, etc.).
type OpsStateStore struct {
	mu    sync.RWMutex
	path  string
	state OpsState
}

type OpsState struct {
	Config        OpsConfigState        `json:"config"`
	Maintenance   OpsMaintenanceState   `json:"maintenance"`
	Notifications OpsNotificationsState `json:"notifications"`
	Health        OpsHealthState        `json:"health"`
	MQ            OpsMQState            `json:"mq"`
	Alerts        OpsAlertState         `json:"alerts"`
	Audit         OpsAuditState         `json:"audit"`
	Nodes         OpsNodesState         `json:"nodes"`
}

// OpsNodesState tracks operational state of nodes (drained, restarting, etc.)
type OpsNodesState struct {
	Drained    map[string]time.Time `json:"drained,omitempty"`    // nodeId -> drainedAt
	Restarting map[string]time.Time `json:"restarting,omitempty"` // nodeId -> restartingAt
	UpdatedAt  time.Time            `json:"updatedAt,omitempty"`
}

type OpsConfigState struct {
	AlertmanagerURL   string    `json:"alertmanagerUrl,omitempty"`
	GrafanaExploreURL string    `json:"grafanaExploreUrl,omitempty"`
	JaegerURL         string    `json:"jaegerUrl,omitempty"`
	UpdatedAt         time.Time `json:"updatedAt,omitempty"`
}

type OpsMaintenanceState struct {
	Windows   []OpsMaintenanceWindow `json:"windows,omitempty"`
	UpdatedAt time.Time              `json:"updatedAt,omitempty"`
}

type OpsMaintenanceWindow struct {
	ID          string `json:"id"`
	GameID      string `json:"gameId,omitempty"`
	Env         string `json:"env,omitempty"`
	Start       string `json:"start,omitempty"`
	End         string `json:"end,omitempty"`
	Message     string `json:"message,omitempty"`
	BlockWrites bool   `json:"blockWrites,omitempty"`
}

type OpsNotificationsState struct {
	Channels  []OpsNotificationChannel `json:"channels,omitempty"`
	Rules     []OpsNotificationRule    `json:"rules,omitempty"`
	UpdatedAt time.Time                `json:"updatedAt,omitempty"`
}

type OpsNotificationChannel struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	URL    string `json:"url,omitempty"`
	Secret string `json:"secret,omitempty"`
}

type OpsNotificationRule struct {
	Event         string   `json:"event"`
	Channels      []string `json:"channels"`
	ThresholdDays int      `json:"thresholdDays,omitempty"`
}

type OpsHealthState struct {
	Checks    []OpsHealthCheck  `json:"checks,omitempty"`
	Status    []OpsHealthStatus `json:"status,omitempty"`
	UpdatedAt time.Time         `json:"updatedAt,omitempty"`
}

type OpsHealthCheck struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Target      string `json:"target"`
	Expect      string `json:"expect,omitempty"`
	IntervalSec int    `json:"intervalSec,omitempty"`
	TimeoutMs   int    `json:"timeoutMs,omitempty"`
	Region      string `json:"region,omitempty"`
}

type OpsHealthStatus struct {
	ID        string    `json:"id"`
	OK        bool      `json:"ok"`
	LatencyMS int64     `json:"latencyMs,omitempty"`
	Error     string    `json:"error,omitempty"`
	CheckedAt time.Time `json:"checkedAt,omitempty"`
}

type OpsMQState struct {
	Type      string         `json:"type,omitempty"`
	Redis     *OpsRedisMQ    `json:"redis,omitempty"`
	Kafka     *OpsKafkaMQ    `json:"kafka,omitempty"`
	Lengths   map[string]int `json:"lengths,omitempty"`
	Groups    []OpsMQGroup   `json:"groups,omitempty"`
	UpdatedAt time.Time      `json:"updatedAt,omitempty"`
}

type OpsRedisMQ struct {
	URL     string            `json:"url,omitempty"`
	Streams map[string]string `json:"streams,omitempty"`
}

type OpsKafkaMQ struct {
	Brokers string            `json:"brokers,omitempty"`
	Topics  map[string]string `json:"topics,omitempty"`
}

type OpsMQGroup struct {
	Stream    string `json:"stream"`
	Name      string `json:"name"`
	Consumers int    `json:"consumers"`
	Pending   int    `json:"pending"`
	Lag       int    `json:"lag,omitempty"`
}

type OpsAlertState struct {
	Silences  []OpsSilenceEntry `json:"silences,omitempty"`
	UpdatedAt time.Time         `json:"updatedAt,omitempty"`
}

type OpsSilenceEntry struct {
	ID        string            `json:"id"`
	AlertID   string            `json:"alertId,omitempty"`
	CreatedBy string            `json:"createdBy,omitempty"`
	Comment   string            `json:"comment,omitempty"`
	Matchers  map[string]string `json:"matchers,omitempty"`
	StartsAt  time.Time         `json:"startsAt"`
	EndsAt    time.Time         `json:"endsAt"`
	Status    OpsSilenceStatus  `json:"status"`
}

type OpsSilenceStatus struct {
	State string `json:"state"`
}

type OpsAuditState struct {
	Entries   []OpsAuditEntry `json:"entries,omitempty"`
	UpdatedAt time.Time       `json:"updatedAt,omitempty"`
}

type OpsAuditEntry struct {
	ID        string                 `json:"id"`
	Action    string                 `json:"action"`
	UserID    string                 `json:"userId,omitempty"`
	GameID    string                 `json:"gameId,omitempty"`
	Env       string                 `json:"env,omitempty"`
	Target    string                 `json:"target,omitempty"`
	Result    string                 `json:"result,omitempty"`
	TraceID   string                 `json:"traceId,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt time.Time              `json:"createdAt"`
}

func NewOpsStateStore(baseDir string) *OpsStateStore {
	if baseDir == "" {
		baseDir = "."
	}
	path := filepath.Join(baseDir, "ops_state.json")
	store := &OpsStateStore{
		path:  path,
		state: defaultOpsState(),
	}
	if err := store.load(); err != nil {
		slog.Default().Error("failed to load ops_state.json", "error", err)
	}
	return store
}

func defaultOpsState() OpsState {
	now := time.Now()
	return OpsState{
		Config: OpsConfigState{
			AlertmanagerURL:   os.Getenv("CROUPIER_ALERTMANAGER_URL"),
			GrafanaExploreURL: os.Getenv("CROUPIER_GRAFANA_EXPLORE_URL"),
			JaegerURL:         os.Getenv("CROUPIER_JAEGER_URL"),
			UpdatedAt:         now,
		},
		MQ: OpsMQState{
			Type: "redis",
			Redis: &OpsRedisMQ{
				URL:     os.Getenv("CROUPIER_REDIS_URL"),
				Streams: map[string]string{"events": "events", "payments": "payments"},
			},
			Lengths:   map[string]int{},
			Groups:    []OpsMQGroup{},
			UpdatedAt: now,
		},
		Alerts: OpsAlertState{
			Silences:  []OpsSilenceEntry{},
			UpdatedAt: now,
		},
		Audit: OpsAuditState{
			// The in-memory audit trail is deprecated — audit history lives in
			// the audit_records table. Kept empty for snapshot compatibility.
			Entries:   []OpsAuditEntry{},
			UpdatedAt: now,
		},
	}
}

func (s *OpsStateStore) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return s.saveLocked()
		}
		return err
	}
	var st OpsState
	if err := json.Unmarshal(data, &st); err != nil {
		return err
	}
	s.state = st
	return nil
}

func (s *OpsStateStore) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}

// Snapshot returns a deep copy of the current state.
func (s *OpsStateStore) Snapshot() OpsState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneOpsState(s.state)
}

// Update applies modification fn and persists the state.
func (s *OpsStateStore) Update(fn func(state *OpsState)) (OpsState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(&s.state)
	if err := s.saveLocked(); err != nil {
		return OpsState{}, err
	}
	return cloneOpsState(s.state), nil
}

func cloneOpsState(st OpsState) OpsState {
	data, err := json.Marshal(st)
	if err != nil {
		slog.Default().Error("failed to clone ops state", "error", err)
		return st
	}
	var cp OpsState
	if err := json.Unmarshal(data, &cp); err != nil {
		slog.Default().Error("failed to unmarshal ops state clone", "error", err)
		return st
	}
	return cp
}
