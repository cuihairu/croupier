package registry

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// FunctionMeta describes a function capability on an agent.
type FunctionMeta struct {
	Enabled bool
	Version string
}

// ProcessSession represents a single process/service registered to an agent (via SDK->Agent local registry).
type ProcessSession struct {
	ServiceID    string
	Addr         string
	Version      string
	LastSeenUnix int64
	FunctionIDs  []string
}

// AgentSession represents a registered agent instance in the registry.
type AgentSession struct {
	AgentID   string
	GameID    string
	Env       string
	RPCAddr   string
	Version   string
	Region    string
	Zone      string
	Labels    map[string]string
	Functions map[string]FunctionMeta
	Processes []ProcessSession
	ExpireAt  time.Time
}

// Store keeps lightweight agent registry state in-memory.
type Store struct {
	mu     sync.RWMutex
	agents map[string]*AgentSession // agent_id -> session
	// provider capabilities (language-agnostic manifest uploaded via HTTP or Control)
	provCaps map[string]ProviderCaps // provider_id -> caps (latest)
	// provider update sequence ensures deterministic ordering for merges.
	provCapsSeq map[string]uint64 // provider_id -> last update seq
	nextProvSeq uint64
}

func NewStore() *Store {
	return &Store{
		agents:      map[string]*AgentSession{},
		provCaps:    map[string]ProviderCaps{},
		provCapsSeq: map[string]uint64{},
	}
}

// Mu exposes the lock for read/update operations when callers need batch views.
func (s *Store) Mu() *sync.RWMutex { return &s.mu }

// AgentsUnsafe returns the internal agents map without copying. Callers MUST hold Mu().RLock/Lock.
func (s *Store) AgentsUnsafe() map[string]*AgentSession { return s.agents }

// UpsertAgent inserts or updates an agent session by AgentID.
func (s *Store) UpsertAgent(a *AgentSession) {
	if a == nil || a.AgentID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cur := s.agents[a.AgentID]
	if cur == nil {
		s.agents[a.AgentID] = a
		return
	}
	// merge minimal fields
	cur.GameID, cur.Env, cur.RPCAddr, cur.Version = a.GameID, a.Env, a.RPCAddr, a.Version
	cur.Region, cur.Zone = a.Region, a.Zone
	if a.Labels != nil {
		cur.Labels = a.Labels
	}
	if a.Functions != nil {
		cur.Functions = a.Functions
	}
	if a.Processes != nil {
		cur.Processes = a.Processes
	}
	if !a.ExpireAt.IsZero() {
		cur.ExpireAt = a.ExpireAt
	}
}

// ProviderCaps represents a provider manifest snapshot registered at runtime.
type ProviderCaps struct {
	ID        string
	Version   string
	Lang      string
	SDK       string
	Manifest  []byte // raw JSON
	UpdatedAt time.Time
}

// UpsertProviderCaps inserts or updates provider capabilities by provider ID.
func (s *Store) UpsertProviderCaps(c ProviderCaps) {
	if c.ID == "" || len(c.Manifest) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	c.UpdatedAt = time.Now()
	s.nextProvSeq++
	s.provCapsSeq[c.ID] = s.nextProvSeq
	s.provCaps[c.ID] = c
}

// ListProviderCaps returns a snapshot of provider capabilities.
func (s *Store) ListProviderCaps() []ProviderCaps {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sortedProviderCapsLocked()
}

// GetProviderCaps returns capabilities for a single provider.
func (s *Store) GetProviderCaps(id string) (ProviderCaps, bool) {
	if id == "" {
		return ProviderCaps{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	cap, ok := s.provCaps[id]
	return cap, ok
}

// DeleteProviderCaps removes provider capabilities by ID.
func (s *Store) DeleteProviderCaps(id string) bool {
	if id == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.provCaps[id]; !ok {
		return false
	}
	delete(s.provCaps, id)
	delete(s.provCapsSeq, id)
	return true
}

func (s *Store) sortedProviderCapsLocked() []ProviderCaps {
	out := make([]ProviderCaps, 0, len(s.provCaps))
	for _, v := range s.provCaps {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		ai, aj := out[i], out[j]
		si, sj := s.provCapsSeq[ai.ID], s.provCapsSeq[aj.ID]
		if si != sj {
			return si < sj
		}
		return ai.ID < aj.ID
	})
	return out
}

// BuildUnifiedDescriptors merges all provider manifests into a unified descriptor structure
func (s *Store) BuildUnifiedDescriptors() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	unified := map[string]interface{}{"providers": map[string]interface{}{}}

	functionsByID := map[string]interface{}{}
	entitiesByID := map[string]interface{}{}
	operationsByKey := map[string]interface{}{}

	for _, provCaps := range s.sortedProviderCapsLocked() {
		if len(provCaps.Manifest) == 0 {
			continue
		}

		providerID := provCaps.ID
		// Parse the manifest JSON
		var manifest map[string]interface{}
		if err := json.Unmarshal(provCaps.Manifest, &manifest); err != nil {
			continue // skip invalid manifests
		}

		// Add provider info
		providers, _ := unified["providers"].(map[string]interface{})
		prov := map[string]interface{}{}
		if raw, ok := manifest["provider"].(map[string]interface{}); ok {
			for k, v := range raw {
				prov[k] = v
			}
		}
		if prov["id"] == nil || prov["id"] == "" {
			prov["id"] = provCaps.ID
		}
		if prov["version"] == nil || prov["version"] == "" {
			prov["version"] = provCaps.Version
		}
		if prov["lang"] == nil || prov["lang"] == "" {
			prov["lang"] = provCaps.Lang
		}
		if prov["sdk"] == nil || prov["sdk"] == "" {
			prov["sdk"] = provCaps.SDK
		}
		prov["updated_at"] = provCaps.UpdatedAt
		providers[providerID] = prov

		// Merge functions
		if functions, exists := manifest["functions"]; exists {
			if funcList, ok := functions.([]interface{}); ok {
				for _, it := range funcList {
					fn, ok := it.(map[string]interface{})
					if !ok {
						continue
					}
					id, _ := fn["id"].(string)
					if id == "" {
						continue
					}
					// later provider wins (iteration order is by update sequence)
					functionsByID[id] = fn
				}
			}
		}

		// Merge entities
		if entities, exists := manifest["entities"]; exists {
			if entityList, ok := entities.([]interface{}); ok {
				for _, it := range entityList {
					ent, ok := it.(map[string]interface{})
					if !ok {
						continue
					}
					id, _ := ent["id"].(string)
					if id == "" {
						continue
					}
					entitiesByID[id] = ent
				}
			}
		}

		// Merge operations (legacy/compat): optional top-level "operations" list
		if operations, exists := manifest["operations"]; exists {
			if opList, ok := operations.([]interface{}); ok {
				for _, it := range opList {
					op, ok := it.(map[string]interface{})
					if !ok {
						continue
					}
					key, _ := op["id"].(string)
					if key == "" {
						// fall back to op/op_id if present
						if v, ok := op["op"].(string); ok && v != "" {
							key = v
						} else if v, ok := op["op_id"].(string); ok && v != "" {
							key = v
						}
					}
					if key == "" {
						continue
					}
					operationsByKey[key] = op
				}
			}
		}
	}

	unified["functions"] = sortedValues(functionsByID)
	unified["entities"] = sortedValues(entitiesByID)
	unified["operations"] = sortedValues(operationsByKey)

	return unified
}

func sortedValues(m map[string]interface{}) []interface{} {
	if len(m) == 0 {
		return []interface{}{}
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]interface{}, 0, len(keys))
	for _, k := range keys {
		out = append(out, m[k])
	}
	return out
}

// BuildFunctionIndex parses provider manifests and builds an index of function metadata by id.
// Expected manifest structure for functions:
//
//	{
//	  "functions": [
//	    {
//	      "id": "player.ban",
//	      "display_name": {"en":"Ban Player","zh":"封禁玩家"},
//	      "summary": {"en":"...","zh":"..."},
//	      "tags": ["moderation","player"],
//	      "menu": { "section":"Function Management","group":"Moderation","path":"/functions/invoke","order":120,"icon":"StopOutlined","badge":"beta","hidden":false },
//	      "permissions": {
//	         "verbs": ["read","invoke","view_history"],
//	         "scopes": ["game","env","function_id"],
//	         "defaults": [ {"role":"operator","verbs":["invoke"]}, {"role":"auditor","verbs":["read","view_history"]} ],
//	         "i18n_zh": { "invoke":"调用函数 封禁玩家" }
//	      }
//	    }
//	  ]
//	}
func (s *Store) BuildFunctionIndex() map[string]map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	idx := map[string]map[string]interface{}{}
	for _, pc := range s.sortedProviderCapsLocked() {
		if len(pc.Manifest) == 0 {
			continue
		}
		var m map[string]interface{}
		if err := json.Unmarshal(pc.Manifest, &m); err != nil {
			continue
		}
		rawFns, ok := m["functions"]
		if !ok {
			continue
		}
		fnList, ok := rawFns.([]interface{})
		if !ok {
			continue
		}
		for _, it := range fnList {
			fn, ok := it.(map[string]interface{})
			if !ok {
				continue
			}
			idv, ok := fn["id"]
			if !ok {
				continue
			}
			fid, _ := idv.(string)
			if fid == "" {
				continue
			}
			// Shallow copy metadata into index
			meta := map[string]interface{}{}
			if dn, ok := fn["display_name"]; ok {
				meta["display_name"] = dn
			}
			if sm, ok := fn["summary"]; ok {
				meta["summary"] = sm
			}
			if tags, ok := fn["tags"]; ok {
				meta["tags"] = tags
			}
			if menu, ok := fn["menu"]; ok {
				meta["menu"] = menu
			}
			if perm, ok := fn["permissions"]; ok {
				meta["permissions"] = perm
			}
			if len(meta) > 0 {
				idx[fid] = meta
			}
		}
	}
	return idx
}

// LoadUIOverrides loads server-side UI/RBAC overrides from one or more JSON files.
// Expected structure:
//
//	{
//	  "player.ban": {
//	    "display_name": {"zh":"封禁玩家","en":"Ban Player"},
//	    "summary": {"zh":"..."},
//	    "tags": ["moderation"],
//	    "menu": { "section":"Function Management", "group":"Moderation", "order": 100, "hidden": false },
//	    "permissions": { "verbs":["read","invoke"], "scopes":["game","env","function_id"], "defaults":[{"role":"operator","verbs":["invoke"]}] }
//	  },
//	  ...
//	}
func (s *Store) LoadUIOverrides(paths ...string) map[string]map[string]interface{} {
	merged := map[string]map[string]interface{}{}
	for _, p := range paths {
		if p == "" {
			continue
		}
		expanded := expandIfDir(p)
		for _, file := range expanded {
			b, err := os.ReadFile(file)
			if err != nil || len(b) == 0 {
				continue
			}
			var m map[string]map[string]interface{}
			if err := json.Unmarshal(b, &m); err != nil {
				continue
			}
			for fid, meta := range m {
				if fid == "" || meta == nil {
					continue
				}
				if _, ok := merged[fid]; !ok {
					merged[fid] = map[string]interface{}{}
				}
				// shallow merge/replace: server overrides take precedence
				for k, v := range meta {
					merged[fid][k] = v
				}
			}
		}
	}
	return merged
}

// expandIfDir: if p is a directory, return *.json files within; else return [p] if exists; else [].
func expandIfDir(p string) []string {
	info, err := os.Stat(p)
	if err != nil {
		return nil
	}
	if info.IsDir() {
		files := []string{}
		_ = filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				return nil
			}
			if filepath.Ext(d.Name()) == ".json" {
				files = append(files, path)
			}
			return nil
		})
		return files
	}
	return []string{p}
}

// ========== Deep Merge Strategy for Descriptors ==========

// MergeStrategy defines how to handle conflicts when merging descriptor fields.
type MergeStrategy int

const (
	// MergeOverwrite: later value completely replaces earlier value
	MergeOverwrite MergeStrategy = iota
	// MergeDeep: recursively merge nested objects
	MergeDeep
	// MergeAppend: append arrays (for list fields)
	MergeAppend
	// MergeSkip: keep first value, ignore later values
	MergeSkip
	// MergeError: report conflict as error
	MergeError
)

// MergeConfig defines merge behavior for specific descriptor fields.
type MergeConfig struct {
	// FieldPath is dot-separated path (e.g., "permissions.verbs")
	FieldPath string
	// Strategy defines how to merge this field
	Strategy MergeStrategy
	// Priority sources in order (lowest to highest)
	// Valid sources: "provider", "component", "server", "override"
	Priority []string
}

// DefaultMergeConfig returns the default merge configuration for descriptor fields.
func DefaultMergeConfig() []MergeConfig {
	return []MergeConfig{
		// Scalar fields: overwrite with higher priority
		{FieldPath: "id", Strategy: MergeSkip}, // id never changes
		{FieldPath: "version", Strategy: MergeOverwrite, Priority: []string{"provider", "component", "server"}},
		{FieldPath: "name", Strategy: MergeDeep, Priority: []string{"provider", "component", "server"}},
		{FieldPath: "description", Strategy: MergeOverwrite, Priority: []string{"provider", "component", "server"}},

		// i18n fields: deep merge (combine languages from all sources)
		{FieldPath: "display_name", Strategy: MergeDeep, Priority: []string{"provider", "component", "server", "override"}},
		{FieldPath: "summary", Strategy: MergeDeep, Priority: []string{"provider", "component", "server", "override"}},

		// Tags: append (union of all tags from all sources)
		{FieldPath: "tags", Strategy: MergeAppend, Priority: []string{"provider", "component", "server", "override"}},

		// Menu: server override wins (for hiding/customizing UI)
		{FieldPath: "menu", Strategy: MergeDeep, Priority: []string{"provider", "component", "server", "override"}},

		// Permissions: deep merge with conflict detection
		{FieldPath: "permissions", Strategy: MergeDeep, Priority: []string{"provider", "component", "server", "override"}},

		// Schema: provider defines, server can extend but not remove required fields
		{FieldPath: "schema", Strategy: MergeDeep, Priority: []string{"provider", "component"}},
		{FieldPath: "schema.required", Strategy: MergeAppend, Priority: []string{"provider", "component"}},
		{FieldPath: "schema.properties", Strategy: MergeDeep, Priority: []string{"provider", "component"}},

		// Entity operations: merge (union of all operations)
		{FieldPath: "operations", Strategy: MergeDeep, Priority: []string{"provider", "component"}},

		// Entity UI: server can override display fields
		{FieldPath: "ui", Strategy: MergeDeep, Priority: []string{"provider", "component", "server", "override"}},

		// Auth/Risk: server override wins (security context)
		{FieldPath: "auth", Strategy: MergeDeep, Priority: []string{"provider", "server", "override"}},
		{FieldPath: "risk", Strategy: MergeOverwrite, Priority: []string{"provider", "server", "override"}},
	}
}

// DescriptorSource identifies where a descriptor value came from.
type DescriptorSource string

const (
	SourceProvider  DescriptorSource = "provider"
	SourceComponent DescriptorSource = "component"
	SourceServer    DescriptorSource = "server"
	SourceOverride  DescriptorSource = "override"
)

// DescriptorWithSource wraps a descriptor with its source for merge tracking.
type DescriptorWithSource struct {
	Data    map[string]interface{}
	Source  DescriptorSource
	Version string
}

// MergeDescriptors merges multiple descriptors according to the configured strategy.
// Returns the merged descriptor and any conflicts encountered.
func (s *Store) MergeDescriptors(descriptors []DescriptorWithSource) (map[string]interface{}, []MergeConflict, error) {
	if len(descriptors) == 0 {
		return nil, nil, nil
	}

	config := DefaultMergeConfig()
	result := map[string]interface{}{}
	conflicts := []MergeConflict{}

	// Sort descriptors by priority (lowest first)
	sortedDescs := sortDescriptorsByPriority(descriptors, config)

	// Merge each descriptor in priority order
	for _, desc := range sortedDescs {
		result = mergeMaps(result, desc.Data, config, desc.Source, &conflicts)
	}

	return result, conflicts, nil
}

// MergeConflict represents a merge conflict that was detected.
type MergeConflict struct {
	Field    string
	Source1  DescriptorSource
	Value1   interface{}
	Source2  DescriptorSource
	Value2   interface{}
	Resolved bool
}

// mergeMaps recursively merges two maps according to the merge configuration.
func mergeMaps(base, override map[string]interface{}, config []MergeConfig, source DescriptorSource, conflicts *[]MergeConflict) map[string]interface{} {
	if base == nil {
		base = map[string]interface{}{}
	}
	if override == nil {
		return base
	}

	result := map[string]interface{}{}

	// Copy all base fields
	for k, v := range base {
		result[k] = v
	}

	// Apply override fields according to strategy
	for key, overrideVal := range override {
		fieldPath := key // For top-level fields

		// Find the merge strategy for this field
		strategy := findMergeStrategy(config, fieldPath)

		switch strategy {
		case MergeOverwrite:
			result[key] = overrideVal

		case MergeSkip:
			// Keep base value if exists
			if _, exists := result[key]; !exists {
				result[key] = overrideVal
			}

		case MergeDeep:
			if baseVal, exists := base[key]; exists {
				// Both exist - try deep merge
				if baseMap, ok := baseVal.(map[string]interface{}); ok {
					if overrideMap, ok := overrideVal.(map[string]interface{}); ok {
						// Recursively merge nested maps
						result[key] = mergeNestedMaps(baseMap, overrideMap, config, source, conflicts, fieldPath)
						continue
					}
				}
				// Types don't match or not maps - overwrite
				result[key] = overrideVal
			} else {
				result[key] = overrideVal
			}

		case MergeAppend:
			if baseVal, exists := base[key]; exists {
				// Append arrays
				result[key] = appendSlices(baseVal, overrideVal)
			} else {
				result[key] = overrideVal
			}

		default:
			// Default to overwrite
			result[key] = overrideVal
		}
	}

	return result
}

// mergeNestedMaps handles nested map merging with conflict tracking.
func mergeNestedMaps(base, override map[string]interface{}, config []MergeConfig, source DescriptorSource, conflicts *[]MergeConflict, parentPath string) map[string]interface{} {
	result := map[string]interface{}{}

	// Copy base
	for k, v := range base {
		result[k] = v
	}

	// Apply override
	for key, overrideVal := range override {
		fieldPath := parentPath + "." + key

		if baseVal, exists := base[key]; exists {
			// Check for specific field strategies (e.g., permissions.verbs)
			strategy := findMergeStrategy(config, fieldPath)

			if strategy == MergeAppend {
				result[key] = appendSlices(baseVal, overrideVal)
				continue
			}

			// Try recursive merge for nested maps
			if baseMap, ok := baseVal.(map[string]interface{}); ok {
				if overrideMap, ok := overrideVal.(map[string]interface{}); ok {
					result[key] = mergeNestedMaps(baseMap, overrideMap, config, source, conflicts, fieldPath)
					continue
				}
			}

			// Overwrite if not both maps
			result[key] = overrideVal
		} else {
			result[key] = overrideVal
		}
	}

	return result
}

// appendSlices appends two values (handling both []interface{} and []string).
func appendSlices(a, b interface{}) interface{} {
	aSlice, aOk := toInterfaceSlice(a)
	bSlice, bOk := toInterfaceSlice(b)

	if !aOk && !bOk {
		return b // b replaces a if neither are slices
	}
	if !aOk {
		return b
	}
	if !bOk {
		return a
	}

	// Deduplicate while preserving order
	seen := make(map[string]bool)
	result := make([]interface{}, 0, len(aSlice)+len(bSlice))

	for _, v := range aSlice {
		key := fmt.Sprint(v)
		if !seen[key] {
			seen[key] = true
			result = append(result, v)
		}
	}
	for _, v := range bSlice {
		key := fmt.Sprint(v)
		if !seen[key] {
			seen[key] = true
			result = append(result, v)
		}
	}

	return result
}

// toInterfaceSlice converts a slice to []interface{}.
func toInterfaceSlice(v interface{}) ([]interface{}, bool) {
	if v == nil {
		return nil, false
	}

	switch slice := v.(type) {
	case []interface{}:
		return slice, true
	case []string:
		result := make([]interface{}, len(slice))
		for i, s := range slice {
			result[i] = s
		}
		return result, true
	case []map[string]interface{}:
		result := make([]interface{}, len(slice))
		for i, m := range slice {
			result[i] = m
		}
		return result, true
	default:
		return nil, false
	}
}

// findMergeStrategy finds the merge strategy for a given field path.
func findMergeStrategy(config []MergeConfig, fieldPath string) MergeStrategy {
	for _, cfg := range config {
		if cfg.FieldPath == fieldPath {
			return cfg.Strategy
		}
	}
	return MergeOverwrite // Default strategy
}

// sortDescriptorsByPriority sorts descriptors by their source priority.
func sortDescriptorsByPriority(descriptors []DescriptorWithSource, config []MergeConfig) []DescriptorWithSource {
	// Build a priority map: source -> rank (lower = lower priority)
	priorityMap := buildPriorityMap(config)

	result := make([]DescriptorWithSource, len(descriptors))
	copy(result, descriptors)

	sort.Slice(result, func(i, j int) bool {
		rank1 := priorityMap[string(result[i].Source)]
		rank2 := priorityMap[string(result[j].Source)]
		return rank1 < rank2 // Lower rank (lower priority) comes first
	})

	return result
}

// buildPriorityMap creates a map from source name to priority rank.
func buildPriorityMap(config []MergeConfig) map[string]int {
	// Collect all unique sources in priority order
	priorityMap := map[string]int{}
	maxRank := 0

	// Default priorities
	defaultOrder := []DescriptorSource{SourceProvider, SourceComponent, SourceServer, SourceOverride}
	for i, src := range defaultOrder {
		priorityMap[string(src)] = i
		maxRank = i + 1
	}

	// Extend with any custom priorities from config
	for _, cfg := range config {
		for i, src := range cfg.Priority {
			if _, exists := priorityMap[src]; !exists {
				priorityMap[src] = maxRank + i
			}
		}
	}

	return priorityMap
}

// MergeProviderDescriptors is a convenience method that merges all registered provider descriptors.
func (s *Store) MergeProviderDescriptors() (map[string]interface{}, []MergeConflict, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	descriptors := []DescriptorWithSource{}

	// Collect descriptors from all providers
	for _, pc := range s.sortedProviderCapsLocked() {
		if len(pc.Manifest) == 0 {
			continue
		}

		var manifest map[string]interface{}
		if err := json.Unmarshal(pc.Manifest, &manifest); err != nil {
			continue
		}

		// Add functions as descriptors
		if functions, ok := manifest["functions"].([]interface{}); ok {
			for _, fn := range functions {
				if fnMap, ok := fn.(map[string]interface{}); ok {
					descriptors = append(descriptors, DescriptorWithSource{
						Data:    fnMap,
						Source:  SourceProvider,
						Version: pc.Version,
					})
				}
			}
		}

		// Add entities as descriptors
		if entities, ok := manifest["entities"].([]interface{}); ok {
			for _, ent := range entities {
				if entMap, ok := ent.(map[string]interface{}); ok {
					descriptors = append(descriptors, DescriptorWithSource{
						Data:    entMap,
						Source:  SourceProvider,
						Version: pc.Version,
					})
				}
			}
		}
	}

	// Load component descriptors from components directory
	componentDescs := s.loadComponentDescriptors()
	descriptors = append(descriptors, componentDescs...)

	// Load server overrides
	serverOverrides := s.loadServerOverrides()
	for _, override := range serverOverrides {
		descriptors = append(descriptors, DescriptorWithSource{
			Data:   override,
			Source: SourceServer,
		})
	}

	// Group descriptors by ID and merge within each group
	grouped := groupDescriptorsByID(descriptors)
	mergedResult := map[string]interface{}{}
	allConflicts := []MergeConflict{}

	for id, group := range grouped {
		merged, conflicts, err := s.MergeDescriptors(group)
		if err != nil {
			continue
		}
		mergedResult[id] = merged
		allConflicts = append(allConflicts, conflicts...)
	}

	return mergedResult, allConflicts, nil
}

// groupDescriptorsByID groups descriptors by their "id" field.
func groupDescriptorsByID(descriptors []DescriptorWithSource) map[string][]DescriptorWithSource {
	grouped := map[string][]DescriptorWithSource{}
	for _, desc := range descriptors {
		id, _ := desc.Data["id"].(string)
		if id == "" {
			continue
		}
		grouped[id] = append(grouped[id], desc)
	}
	return grouped
}

// loadComponentDescriptors loads descriptors from the components directory.
func (s *Store) loadComponentDescriptors() []DescriptorWithSource {
	descriptors := []DescriptorWithSource{}

	componentDirs := []string{
		"components",
		"/usr/local/lib/croupier/components",
	}

	for _, dir := range componentDirs {
		if _, err := os.Stat(dir); err != nil {
			continue
		}

		// Find all manifest.json files
		_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}

			if d.Name() == "manifest.json" {
				b, err := os.ReadFile(path)
				if err != nil {
					return nil
				}

				var manifest map[string]interface{}
				if err := json.Unmarshal(b, &manifest); err != nil {
					return nil
				}

				// Extract descriptors from manifest
				if functions, ok := manifest["functions"].([]interface{}); ok {
					for _, fn := range functions {
						if fnMap, ok := fn.(map[string]interface{}); ok {
							descriptors = append(descriptors, DescriptorWithSource{
								Data:   fnMap,
								Source: SourceComponent,
							})
						}
					}
				}

				if entities, ok := manifest["entities"].([]interface{}); ok {
					for _, ent := range entities {
						if entMap, ok := ent.(map[string]interface{}); ok {
							descriptors = append(descriptors, DescriptorWithSource{
								Data:   entMap,
								Source: SourceComponent,
							})
						}
					}
				}
			}

			return nil
		})
	}

	return descriptors
}

// loadServerOverrides loads server-side override descriptors.
func (s *Store) loadServerOverrides() []map[string]interface{} {
	overrides := []map[string]interface{}{}

	// Load from configured paths
	paths := []string{
		"configs/ui/functions.override.json",
		"configs/ui/entities.override.json",
		"configs/ui/descriptors.override.json",
	}

	for _, path := range paths {
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var overrideMap map[string]map[string]interface{}
		if err := json.Unmarshal(b, &overrideMap); err != nil {
			continue
		}

		for _, desc := range overrideMap {
			overrides = append(overrides, desc)
		}
	}

	return overrides
}
