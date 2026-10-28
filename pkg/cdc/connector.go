package cdc

import (
	"context"

	"github.com/cuihairu/croupier/pkg/cdc/schema"
)

// SourceConfig 数据源连接配置（契约面）。凭据红线：只携带环境变量名引用
// （DSNEnv），明文连接串不进契约、不落日志、不进上报——真实取值由实现层
// 在 Open 时从密钥面解析。
type SourceConfig struct {
	SourceID   string            `json:"sourceId"`
	SourceType string            `json:"sourceType"` // "mysql" | "postgres" | …
	DSNEnv     string            `json:"dsnEnv"`     // Environment variable containing the connection string.
	Options    map[string]string `json:"options,omitempty"`
}

// ColumnMeta 列元数据（Discover 产出；列名映射 information_schema 摄取的
// 契约载体——落地时 ColumnsMapped 即可由 Discover 结果驱动）。
type ColumnMeta struct {
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
}

// TableMeta 表元数据。
type TableMeta struct {
	Name       string       `json:"name"` // "db.table" 全名
	PrimaryKey []string     `json:"primaryKey,omitempty"`
	Columns    []ColumnMeta `json:"columns,omitempty"`
}

// DiscoveryRequest 元数据发现请求（Tables 空=全库）。
type DiscoveryRequest struct {
	Tables []string `json:"tables,omitempty"` // "db.table" 全名
}

// DiscoveryResult 发现结果：表元数据 + 该源能力声明（一并产出，保证能力
// 与元数据同源）。
type DiscoveryResult struct {
	Tables       []TableMeta  `json:"tables"`
	Capabilities Capabilities `json:"capabilities"`
}

// SnapshotRequest 表级快照请求（历史回补 / 新表起点；能力 snapshot 缺失时
// 实现必须显式返回 UnsupportedSnapshot 错误，不得静默回空流）。
type SnapshotRequest struct {
	Tables []string `json:"tables,omitempty"` // "db.table" 全名
}

// PollRequest 增量变更流请求：从指定位点续传（From 空=实现定义的最早可得
// 起点，须在日志中显式记录起点位点）。
type PollRequest struct {
	From   Position `json:"from"`
	Tables []string `json:"tables,omitempty"` // 空=全库（SourceFilter 下推能力允许时由源端过滤）
}

// TableSchema 别名：pkg/cdc/schema 的列型映射（源侧 schema 演进历史契约面）。
type TableSchema = schema.TableSchema

// SchemaHistorySource 可选扩展面：声明 schemaHistory 能力的源连接器应实现
// （类型断言发现，非必须方法——旧连接器不破坏）。恢复计划构建层据此填充
// Snapshot.SchemaVersions；恢复执行层据此按事务位点校验事件列集。历史未
// 覆盖（早于首版 / 表未记录 / 无活动流）返回错误——不猜、不伪造。
type SchemaHistorySource interface {
	// SchemaAt 表在指定位点当时生效的列型映射版本。
	SchemaAt(ctx context.Context, table string, pos Position) (TableSchema, error)
	// CurrentSchemaVersions 各表当前生效版本号（"db.table" → 修订号）。
	CurrentSchemaVersions(ctx context.Context) (map[string]int64, error)
}

// ErrUnsupportedSnapshot Snapshot 能力缺失时的标准错误（errors.Is 判定）。
var ErrUnsupportedSnapshot = errUnsupportedSnapshot{}

type errUnsupportedSnapshot struct{}

func (errUnsupportedSnapshot) Error() string {
	return "connector: source does not support snapshot"
}

func (errUnsupportedSnapshot) Is(target error) bool { return target == ErrUnsupportedSnapshot }

// SourceConnector is the contract implemented by database change-stream adapters.
//
// 并发契约（实现须遵守）：Open 后 Discover/Snapshot/Poll/Checkpoint 可被
// 不同 goroutine 调用（Poll 长驻流式，其余短请求）；同一 Connector 的
// Snapshot 与 Poll 不要求并发安全——runtime 串行调度；Close 后所有流必须
// 收敛关闭且后续调用返回错误。
type SourceConnector interface {
	// Open 建立源连接。凭据从 SourceConfig.DSNEnv 指向的环境变量解析；
	// 失败返回错误（调用方决定重试/退出）。
	Open(ctx context.Context, cfg SourceConfig) error
	// Discover 返回表/列元数据与能力声明。
	Discover(ctx context.Context, req DiscoveryRequest) (*DiscoveryResult, error)
	// Snapshot 表级快照流（按需回补/新表起点）；能力缺失返回
	// ErrUnsupportedSnapshot。事件流关闭即快照完成。
	Snapshot(ctx context.Context, req SnapshotRequest) (<-chan ChangeEvent, error)
	// Poll 增量变更流（binlog/WAL/oplog），从 req.From 续传；流永久消费
	// 直到 ctx 取消或 Close。事件必须携带位点（Position.Valid）。
	Poll(ctx context.Context, req PollRequest) (<-chan ChangeEvent, error)
	// Checkpoint 当前消费位点（持久化 + 可恢复性；能力
	// resumableCheckpoint 缺失时返回零位点 + 错误说明）。
	Checkpoint(ctx context.Context) (Position, error)
	// AckCheckpoint marks a consumer-delivered position as resumable. It is
	// only called after the event has been handed to the runtime.
	AckCheckpoint(ctx context.Context, position Position) error
	// Close 释放连接与流（幂等；Close 后接口方法返回错误）。
	Close(ctx context.Context) error
}
