// Package cdc provides database-agnostic change event, source connector,
// checkpoint and capability contracts. It intentionally contains no product
// policy or database-driver dependency.
package cdc

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Operation 变更操作（闭集，与 source.Op 同语义；独立类型避免 connector 契约
// 反向依赖 source 包——source.MySQL 适配器落 P0 批做映射）。
type Operation string

const (
	OpInsert Operation = "insert"
	OpUpdate Operation = "update"
	OpDelete Operation = "delete"
)

// Valid 操作是否在闭集内。
func (o Operation) Valid() bool {
	switch o {
	case OpInsert, OpUpdate, OpDelete:
		return true
	}
	return false
}

// Position is an opaque source offset such as a GTID set, LSN, SCN, or token.
// The contract stores the source-native value without interpreting advancement.
type Position struct {
	Kind  string `json:"kind"`  // "gtid" | "lsn" | "scn" | "token"（闭集，随源扩展）
	Value string `json:"value"` // 源原生位点串（GTID set / LSN 十进制 / 自定义 token）
}

// Valid 位点 kind 是否在闭集内。
func (p Position) Valid() bool {
	switch p.Kind {
	case "gtid", "lsn", "scn", "token":
		return true
	}
	return false
}

// String checkpoint 持久化形态（kind:value；空位点=空串）。
func (p Position) String() string {
	if p.Kind == "" && p.Value == "" {
		return ""
	}
	return p.Kind + ":" + p.Value
}

// ParsePosition 从 checkpoint 串还原位点（String 的逆）。
func ParsePosition(s string) (Position, error) {
	if s == "" {
		return Position{}, nil
	}
	kind, value, ok := strings.Cut(s, ":")
	if !ok || kind == "" {
		return Position{}, fmt.Errorf("connector: malformed position %q", s)
	}
	p := Position{Kind: kind, Value: value}
	if !p.Valid() {
		return Position{}, fmt.Errorf("connector: unknown position kind %q", kind)
	}
	return p, nil
}

// TxInfo 事务元数据（能力 transactionMetadata 不足时零值——不伪造）。
type TxInfo struct {
	ID  string `json:"id,omitempty"`  // Source-native transaction identifier, when available.
	Seq int64  `json:"seq,omitempty"` // Position within the transaction, when available.
	End bool   `json:"end,omitempty"` // True only on the final event in a transaction.
}

// ChangeEvent is a normalized row-level change from a source connector.
//
// 能力差异必须表达，不伪造缺失信息（用户令红线）：
//   - Before/After 拿不到 = nil（而非空 map 或零值行）；能力声明是真相源。
//   - ColumnsMapped=false 时行像键是源提供的位置下标（"0","1",…）。
type ChangeEvent struct {
	ID            string                     `json:"id,omitempty"`      // Stable event identity, assigned by a journal if absent.
	SourceID      string                     `json:"sourceId"`          // 数据源标识（策略 scope 对齐）
	SourceType    string                     `json:"sourceType"`        // "mysql" | "postgres" | …（闭集随实现扩展）
	Database      string                     `json:"database"`          // 库 / schema
	Table         string                     `json:"table"`             // 表名（不含库前缀； QualifiedTable() 拼）
	Operation     Operation                  `json:"operation"`         // insert/update/delete 闭集
	Before        map[string]json.RawMessage `json:"before,omitempty"`  // 前像（能力不足=nil）
	After         map[string]json.RawMessage `json:"after,omitempty"`   // 后像（能力不足=nil）
	ColumnsMapped bool                       `json:"columnsMapped"`     // 行像键是否为列名（false=位置下标）
	Key           []string                   `json:"key,omitempty"`     // Primary or correlation key columns, when available.
	Tx            TxInfo                     `json:"tx,omitempty"`      // 事务元数据（能力不足=零值）
	Position      Position                   `json:"position"`          // 本事件位点
	SchemaVersion int64                      `json:"schemaVersion"`     // 源表结构版本（DDL 漂移检测用，可得时填）
	Account       string                     `json:"account,omitempty"` // 连接账号（血缘；MySQL 尽力而为、PG 拿不到=空）
	TsUnixMs      int64                      `json:"tsUnixMs"`
}

// QualifiedTable returns the database-qualified table name.
func (e *ChangeEvent) QualifiedTable() string {
	return e.Database + "." + e.Table
}

// Validate 事件自检（契约面最低门槛，消费方可在其上加规则级校验）：
// 标识/操作闭集/位点合法；update 双像全空=不可检测，契约层直接拒绝——
// 这是「能力差异表达、不伪造」的反向约束（声称 update 却无任何像，属
// 实现缺陷而非数据缺失）。insert/delete 单像缺失合法（能力不足 nil）。
func (e *ChangeEvent) Validate() error {
	if e.SourceID == "" || e.SourceType == "" {
		return fmt.Errorf("connector: event missing source identity")
	}
	if e.Database == "" || e.Table == "" {
		return fmt.Errorf("connector: event missing table identity")
	}
	if !e.Operation.Valid() {
		return fmt.Errorf("connector: unknown operation %q", e.Operation)
	}
	if !e.Position.Valid() {
		return fmt.Errorf("connector: event position kind %q invalid", e.Position.Kind)
	}
	if e.TsUnixMs <= 0 {
		return fmt.Errorf("connector: event timestamp missing")
	}
	if e.Operation == OpUpdate && len(e.Before) == 0 && len(e.After) == 0 {
		return fmt.Errorf("connector: update event has neither before nor after image")
	}
	return nil
}
