// Package source capture-agent 源层（capture C1）：把游戏库变更日志翻译成
// 统一变更事件流（ChangeSource Provider，设计 §3）。MySQL binlog 为 C1 唯一
// 内置源，PostgreSQL 逻辑复制为 C3 扩展位；规则层三道闸只认 ChangeEvent，
// 与源解耦。
//
// 本包不 import 任何源驱动：契约/位点/幂等/过滤是纯逻辑，单测零依赖；
// 源驱动封在各 provider 文件（mysql.go import go-mysql）。
package source

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// Op 变更操作（闭集，随源扩展）。
type Op string

const (
	OpInsert Op = "insert"
	OpUpdate Op = "update"
	OpDelete Op = "delete"
)

// ChangeEvent 统一变更事件（设计 §3.1 十一字段，wire 契约 lowerCamelCase；
// proto 化随 C2 告警上报一并做，此前为进程内契约）。
type ChangeEvent struct {
	SourceType string          `json:"sourceType"` // "mysql" | "postgres"（闭集）
	Database   string          `json:"database"`
	Table      string          `json:"table"`
	Op         Op              `json:"op"`
	BeforeJSON json.RawMessage `json:"beforeJson,omitempty"` // 前像（update/delete 有）
	AfterJSON  json.RawMessage `json:"afterJson,omitempty"`  // 后像（insert/update 有）
	GTID       string          `json:"gtid"`                 // 源原生位点：MySQL GTID / PG LSN
	Account    string          `json:"account,omitempty"`    // 连接账号（血缘闸核心；PG 拿不到，降级空）
	ClientAddr string          `json:"clientAddr,omitempty"` // 尽力而为
	TsUnixMs   int64           `json:"tsUnixMs"`
	ServerID   string          `json:"serverId,omitempty"` // 源实例标识（多实例分流）
}

// IDempotencyKey 幂等键组成部分：GTID + 库表 + 行定位 + 操作。行定位用行像
// 内容 SHA-256（before+after 拼接哈希，FULL row image 下唯一定位该行该次变
// 更；设计口径的「主键」在此以 rowHash 落地——不引 schema 缓存拿主键列，
// C1 边界）。
func (e *ChangeEvent) IDempotencyKey() string {
	sum := sha256.Sum256(append(append([]byte{}, e.BeforeJSON...), e.AfterJSON...))
	return fmt.Sprintf("%s|%s.%s|%s|%s", e.GTID, e.Database, e.Table, hex.EncodeToString(sum[:8]), e.Op)
}

// MatchesTable 是否命中订阅白名单（库.表 精确匹配；只订道具相关表，噪声
// 与带宽双控）。白名单空 = 全订（测试用，生产配置必须显式列出）。
func (e *ChangeEvent) MatchesTable(tables map[string]bool) bool {
	if len(tables) == 0 {
		return true
	}
	return tables[e.Database+"."+e.Table]
}
