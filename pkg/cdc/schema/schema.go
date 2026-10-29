// Package schema provides a database-agnostic schema evolution history: a
// versioned, per-table column-type mapping keyed by source position. A source
// connector records each DDL-driven schema change; a recovery plan can then
// look up the schema that was current at any transaction's source position and
// validate an event's columns against it (恢复时按事务时点的 schema 校验).
//
// The package deliberately contains no database-driver dependency and no
// product policy: position ordering is injected by the caller so the same
// registry serves LSN (Postgres), binlog coordinates (MySQL) and token
// (Mongo) sources.
package schema

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Column is one column's type mapping within a schema version. Type is the
// source-native type descriptor (Postgres: type OID string; MySQL: column
// type id); TypeMod carries source-native modifiers (typmod / length /
// precision) when meaningful.
type Column struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	TypeMod int64  `json:"typeMod,omitempty"`
}

// TableSchema is the ordered column-type mapping of one table at one version.
// Database is the schema (Postgres) or database (MySQL) name; Table is bare
// table name (qualified as Database+"."+Table).
type TableSchema struct {
	Database string   `json:"database"`
	Table    string   `json:"table"`
	Columns  []Column `json:"columns"`
}

// Qualified returns the database-qualified table name.
func (t TableSchema) Qualified() string {
	return t.Database + "." + t.Table
}

// ColumnNames returns the ordered column names of this schema.
func (t TableSchema) ColumnNames() []string {
	names := make([]string, len(t.Columns))
	for i, c := range t.Columns {
		names[i] = c.Name
	}
	return names
}

// HasColumn reports whether the named column is part of this schema version.
func (t TableSchema) HasColumn(name string) bool {
	for _, c := range t.Columns {
		if c.Name == name {
			return true
		}
	}
	return false
}

// Version is one schema version of a table, current over the source position
// range [From, NextFrom). From is an opaque source-native position string;
// ordering is provided by the Less function the History was built with.
type Version struct {
	Revision int64       `json:"revision"`
	From     string      `json:"from"`
	Schema   TableSchema `json:"schema"`
}

// LessFn orders source positions. It must be a strict weak ordering
// (transitive, antisymmetric) over the position strings a connector emits.
type LessFn func(a, b string) bool

// History is a per-table versioned schema evolution history. Versions are
// appended in source position order; Record rejects out-of-order positions
// and duplicate revisions so a connector cannot silently corrupt the chain.
type History struct {
	mu     sync.RWMutex
	less   LessFn
	tables map[string][]Version // qualified table → ascending From
}

// NewHistory constructs an empty history using the given position ordering.
func NewHistory(less LessFn) *History {
	return &History{less: less, tables: make(map[string][]Version)}
}

// Record appends a schema version for a table. It is idempotent for the same
// revision (a connector re-sending an unchanged relation after reconnect) and
// rejects a position that is not strictly after the previous version's From.
func (h *History) Record(qualified string, revision int64, from string, ts TableSchema) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.less == nil {
		return fmt.Errorf("schema: history has no position comparator")
	}
	if revision <= 0 {
		return fmt.Errorf("schema: revision must be positive, got %d", revision)
	}
	if from == "" {
		return fmt.Errorf("schema: revision %d for %s has no source position", revision, qualified)
	}
	if ts.Database == "" || ts.Table == "" {
		return fmt.Errorf("schema: revision %d for %s has no table identity", revision, qualified)
	}
	if ts.Qualified() != qualified {
		return fmt.Errorf("schema: revision %d table %s does not match key %s", revision, ts.Qualified(), qualified)
	}
	if len(ts.Columns) == 0 {
		return fmt.Errorf("schema: revision %d for %s has no columns", revision, qualified)
	}
	// 单锁在函数顶部取得（L94），此处不再重复加锁——sync.RWMutex 不可重入，
	// 二次 Lock 会永久阻塞（merge 曾把两边的加锁拼成双锁导致 Record 死锁）。
	vers := h.tables[qualified]
	if len(vers) > 0 {
		last := vers[len(vers)-1]
		if last.Revision == revision {
			// Same revision re-declared: identical schema at the same position
			// is a benign replay; anything else is a contradiction.
			if last.From == from && tableSchemaEqual(last.Schema, ts) {
				return nil
			}
			return fmt.Errorf("schema: revision %d for %s re-recorded with conflicting content", revision, qualified)
		}
		if h.less(from, last.From) || from == last.From {
			return fmt.Errorf("schema: revision %d for %s position %q is not after %q",
				revision, qualified, from, last.From)
		}
	}
	h.tables[qualified] = append(vers, Version{Revision: revision, From: from, Schema: ts})
	return nil
}

// At returns the schema version of the table current at the given position:
// the latest recorded version whose From is not after the position. found is
// false when the table has no version yet at that position (unknown schema).
func (h *History) At(qualified, position string) (TableSchema, int64, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	vers := h.tables[qualified]
	if len(vers) == 0 {
		return TableSchema{}, 0, false
	}
	// Versions are ascending by From; binary search the greatest From <= position.
	lo, hi := 0, len(vers)
	for lo < hi {
		mid := (lo + hi) / 2
		if h.less(vers[mid].From, position) || vers[mid].From == position {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo == 0 {
		return TableSchema{}, 0, false // position precedes the first recorded version
	}
	v := vers[lo-1]
	return v.Schema, v.Revision, true
}

// Versions returns the recorded versions for a table in ascending From order.
func (h *History) Versions(qualified string) []Version {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return append([]Version(nil), h.tables[qualified]...)
}

// Tables returns the qualified table names recorded, sorted for stable output.
func (h *History) Tables() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]string, 0, len(h.tables))
	for t := range h.tables {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

// Validate checks the whole history for chain consistency (positions strictly
// ascending within each table). It is the audit surface for a connector that
// recorded a history; Record already enforces this, so Validate is mainly for
// defensive callers that reconstruct a history from an external source.
func (h *History) Validate() error {
	h.mu.RLock()
	defer h.mu.RUnlock()
	tables := make([]string, 0, len(h.tables))
	for table := range h.tables {
		tables = append(tables, table)
	}
	sort.Strings(tables)
	for _, table := range tables {
		vers := h.tables[table]
		for i := 1; i < len(vers); i++ {
			if !h.less(vers[i-1].From, vers[i].From) {
				return fmt.Errorf("schema: table %s positions out of order at %q -> %q",
					table, vers[i-1].From, vers[i].From)
			}
		}
	}
	return nil
}

// Clone returns a copy of the history safe to hand to a concurrent consumer
// that may read while the connector keeps recording. Each table's version
// slice is copied so appends on the clone do not alias the original, but the
// Version.Schema.Columns slices are shared: callers must treat recorded
// TableSchema values as immutable (Record only ever appends fresh versions and
// never mutates Columns in place, so sharing is safe today).
func (h *History) Clone() *History {
	h.mu.RLock()
	defer h.mu.RUnlock()
	cp := NewHistory(h.less)
	for table, vers := range h.tables {
		cp.tables[table] = append([]Version(nil), vers...)
	}
	return cp
}

func tableSchemaEqual(a, b TableSchema) bool {
	if a.Database != b.Database || a.Table != b.Table || len(a.Columns) != len(b.Columns) {
		return false
	}
	for i := range a.Columns {
		if a.Columns[i] != b.Columns[i] {
			return false
		}
	}
	return true
}

// ValidateColumns reports whether the given event column names are all present
// in the schema version (the restore-time schema check: an event whose image
// references a column absent from the schema current at its source position is
// rejected, not silently re-mapped). An event may carry fewer columns than the
// schema (before-image under replica identity, or a partial image); it must
// never carry one that does not exist at its time point.
func ValidateColumns(ts TableSchema, names []string) error {
	if len(names) == 0 {
		return fmt.Errorf("schema: event has no columns to validate")
	}
	for _, name := range names {
		if !ts.HasColumn(name) {
			return fmt.Errorf("schema: column %q is not in the schema current at this source position (%s)",
				name, ts.Qualified())
		}
	}
	return nil
}

// QualifiedOf splits a "database.table" key, mirroring the recovery plan's
// exact-name rule (exactly one dot, both parts non-empty). It returns ok=false
// for malformed keys.
func QualifiedOf(qualified string) (db, table string, ok bool) {
	db, table, ok = strings.Cut(qualified, ".")
	if !ok || db == "" || table == "" || strings.Contains(table, ".") {
		return "", "", false
	}
	return db, table, true
}
