package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	extensionsync "github.com/cuihairu/croupier/internal/core/extension/sync"
)

// ExtensionRuntimeApplyResult captures one runtime apply/reconcile result.
type ExtensionRuntimeApplyResult struct {
	Applied int `json:"applied"`
	Removed int `json:"removed"`
	Failed  int `json:"failed"`
}

// RuntimeBinding is the runtime view of one extension binding.
type RuntimeBinding struct {
	BindingType string         `json:"bindingType"`
	BindingKey  string         `json:"bindingKey"`
	TargetRef   string         `json:"targetRef"`
	SpecJSON    string         `json:"specJson"`
	Spec        map[string]any `json:"spec"`
	Status      string         `json:"status"`
}

// RuntimeInstallation is the runtime view of one extension installation.
type RuntimeInstallation struct {
	InstallationID  uint              `json:"installationId"`
	InstallationKey string            `json:"installationKey"`
	ExtensionID     string            `json:"extensionId"`
	ReleaseVersion  string            `json:"releaseVersion"`
	Enabled         bool              `json:"enabled"`
	ScopeType       string            `json:"scopeType"`
	ScopeID         string            `json:"scopeId"`
	TargetType      string            `json:"targetType"`
	TargetID        string            `json:"targetId"`
	Config          map[string]any    `json:"config"`
	SecretRefs      map[string]string `json:"secretRefs"`
	Bindings        []RuntimeBinding  `json:"bindings"`
}

// ExtensionRuntimeSnapshot represents current runtime cache.
type ExtensionRuntimeSnapshot struct {
	AgentID         string                `json:"agentId"`
	Version         string                `json:"version"`
	GeneratedAt     int64                 `json:"generatedAt"`
	AppliedAt       int64                 `json:"appliedAt"`
	LastApplyStatus string                `json:"lastApplyStatus"`
	LastError       string                `json:"lastError"`
	LastErrorAt     int64                 `json:"lastErrorAt"`
	LastApplied     int                   `json:"lastApplied"`
	LastRemoved     int                   `json:"lastRemoved"`
	LastFailed      int                   `json:"lastFailed"`
	Installations   []RuntimeInstallation `json:"installations"`
}

// ExtensionRuntime keeps agent-side extension payload cache.
type ExtensionRuntime struct {
	mu            sync.RWMutex
	lastPayload   *extensionsync.AgentSyncPayload
	snapshot      ExtensionRuntimeSnapshot
	installations map[uint]RuntimeInstallation
}

func NewExtensionRuntime() *ExtensionRuntime {
	return &ExtensionRuntime{
		installations: map[uint]RuntimeInstallation{},
		snapshot: ExtensionRuntimeSnapshot{
			LastApplyStatus: "unknown",
			Installations:   []RuntimeInstallation{},
		},
	}
}

func (r *ExtensionRuntime) ApplyPayload(payload *extensionsync.AgentSyncPayload) (*ExtensionRuntimeApplyResult, error) {
	if r == nil {
		return nil, errors.New("extension runtime is nil")
	}
	if payload == nil {
		return nil, errors.New("payload is nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	next := map[uint]RuntimeInstallation{}
	result := &ExtensionRuntimeApplyResult{}

	for _, item := range payload.Installations {
		inst, ok := decodeInstallation(item)
		if !ok {
			result.Failed++
			continue
		}
		next[item.InstallationID] = inst
		result.Applied++
	}

	for id := range r.installations {
		if _, ok := next[id]; !ok {
			result.Removed++
		}
	}

	r.installations = next
	r.lastPayload = clonePayload(payload)
	r.snapshot.AgentID = payload.AgentID
	r.snapshot.Version = payload.Version
	r.snapshot.GeneratedAt = payload.GeneratedAt
	r.snapshot.AppliedAt = time.Now().Unix()
	if result.Failed > 0 {
		r.snapshot.LastApplyStatus = "degraded"
		r.snapshot.LastError = fmt.Sprintf("apply has %d failed installations", result.Failed)
		r.snapshot.LastErrorAt = time.Now().Unix()
	} else {
		r.snapshot.LastApplyStatus = "ok"
		r.snapshot.LastError = ""
		r.snapshot.LastErrorAt = 0
	}
	r.snapshot.LastApplied = result.Applied
	r.snapshot.LastRemoved = result.Removed
	r.snapshot.LastFailed = result.Failed
	r.snapshot.Installations = make([]RuntimeInstallation, 0, len(next))
	for _, inst := range next {
		r.snapshot.Installations = append(r.snapshot.Installations, inst)
	}

	return result, nil
}

func (r *ExtensionRuntime) Reconcile(payload *extensionsync.AgentSyncPayload) (*ExtensionRuntimeApplyResult, error) {
	return r.ApplyPayload(payload)
}

// Reload re-applies last successful payload.
func (r *ExtensionRuntime) Reload() (*ExtensionRuntimeApplyResult, error) {
	if r == nil {
		return nil, errors.New("extension runtime is nil")
	}
	r.mu.RLock()
	last := clonePayload(r.lastPayload)
	r.mu.RUnlock()
	if last == nil {
		return nil, errors.New("no payload to reload")
	}
	return r.ApplyPayload(last)
}

func (r *ExtensionRuntime) Snapshot() ExtensionRuntimeSnapshot {
	if r == nil {
		return ExtensionRuntimeSnapshot{Installations: []RuntimeInstallation{}}
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := r.snapshot
	out.Installations = append([]RuntimeInstallation(nil), r.snapshot.Installations...)
	return out
}

func (r *ExtensionRuntime) RecordError(err error) {
	if r == nil || err == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.snapshot.LastApplyStatus = "error"
	r.snapshot.LastError = err.Error()
	r.snapshot.LastErrorAt = time.Now().Unix()
}

func decodeInstallation(item extensionsync.AgentInstallationPayload) (RuntimeInstallation, bool) {
	config := map[string]any{}
	secretRefs := map[string]string{}
	if strings.TrimSpace(item.ConfigJSON) != "" {
		if err := json.Unmarshal([]byte(item.ConfigJSON), &config); err != nil {
			return RuntimeInstallation{}, false
		}
	}
	if strings.TrimSpace(item.SecretRefsJSON) != "" {
		if err := json.Unmarshal([]byte(item.SecretRefsJSON), &secretRefs); err != nil {
			return RuntimeInstallation{}, false
		}
	}
	bindings := make([]RuntimeBinding, 0, len(item.Bindings))
	for _, b := range item.Bindings {
		spec := map[string]any{}
		if strings.TrimSpace(b.SpecJSON) != "" {
			if err := json.Unmarshal([]byte(b.SpecJSON), &spec); err != nil {
				return RuntimeInstallation{}, false
			}
		}
		bindings = append(bindings, RuntimeBinding{
			BindingType: b.BindingType,
			BindingKey:  b.BindingKey,
			TargetRef:   b.TargetRef,
			SpecJSON:    b.SpecJSON,
			Spec:        spec,
			Status:      b.Status,
		})
	}

	return RuntimeInstallation{
		InstallationID:  item.InstallationID,
		InstallationKey: item.InstallationKey,
		ExtensionID:     item.ExtensionID,
		ReleaseVersion:  item.ReleaseVersion,
		Enabled:         item.Enabled,
		ScopeType:       item.ScopeType,
		ScopeID:         item.ScopeID,
		TargetType:      item.TargetType,
		TargetID:        item.TargetID,
		Config:          config,
		SecretRefs:      secretRefs,
		Bindings:        bindings,
	}, true
}

func clonePayload(payload *extensionsync.AgentSyncPayload) *extensionsync.AgentSyncPayload {
	if payload == nil {
		return nil
	}
	out := *payload
	out.Installations = append([]extensionsync.AgentInstallationPayload(nil), payload.Installations...)
	return &out
}
