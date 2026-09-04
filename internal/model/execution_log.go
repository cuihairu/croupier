package model

import (
	"context"
	"time"

	"github.com/cuihairu/croupier/internal/db/dbctx"
	"gorm.io/gorm"
)

// ExecutionLog 记录一次受控执行（REST invoke 或页面绑定执行）的请求与
// 响应载荷，供事后审查（R1/R2）。审计元数据仍以哈希链 audit_records 为
// 准——本表只承载 payload 级数据，按保留期清理。
type ExecutionLog struct {
	ID             int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	GameID         string    `gorm:"size:64;not null;index:idx_exec_logs_scope_created,priority:1;index:idx_exec_logs_actor_created,priority:2" json:"gameId"`
	Env            string    `gorm:"size:64;not null;index:idx_exec_logs_scope_created,priority:2" json:"env"`
	Source         string    `gorm:"size:32;not null;default:invoke" json:"source"` // invoke|page
	FunctionID     string    `gorm:"size:255;not null;index:idx_exec_logs_function_created,priority:1;index:idx_exec_logs_scope_created,priority:3" json:"functionId"`
	PageKey        string    `gorm:"size:255" json:"pageKey"`
	BindingID      string    `gorm:"size:255" json:"bindingId"`
	Actor          string    `gorm:"size:255;not null;index:idx_exec_logs_actor_created,priority:1" json:"actor"`
	Route          string    `gorm:"size:64" json:"route"`
	Status         string    `gorm:"size:32;not null" json:"status"` // ok|error
	DurationMs     int64     `json:"durationMs"`
	TraceID        string    `gorm:"size:128;index" json:"traceId"`
	RequestPayload JSON      `gorm:"type:json" json:"requestPayload"`
	ResponseBody   JSON      `gorm:"type:json" json:"responseBody"`
	Truncated      bool      `json:"truncated"`
	CreatedAt      time.Time `gorm:"not null;index:idx_exec_logs_scope_created,priority:4;index:idx_exec_logs_actor_created,priority:3;index:idx_exec_logs_function_created,priority:2" json:"createdAt"`
}

func (ExecutionLog) TableName() string {
	return "execution_logs"
}

// ExecutionLogModel 提供 execution_logs 的数据访问。
type ExecutionLogModel struct {
	db *gorm.DB
}

func NewExecutionLogModel(db *gorm.DB) *ExecutionLogModel {
	return &ExecutionLogModel{db: db}
}

// Create 写入一条执行日志。
func (m *ExecutionLogModel) Create(ctx context.Context, log *ExecutionLog) error {
	return dbctx.Resolve(ctx, m.db).WithContext(ctx).Create(log).Error
}

// List 按过滤条件分页查询。
func (m *ExecutionLogModel) List(ctx context.Context, opts ExecutionLogListOptions) ([]ExecutionLog, int64, error) {
	db := dbctx.Resolve(ctx, m.db).WithContext(ctx).Model(&ExecutionLog{})
	if opts.GameID != "" {
		db = db.Where("game_id = ?", opts.GameID)
	}
	if opts.Env != "" {
		db = db.Where("env = ?", opts.Env)
	}
	if opts.Actor != "" {
		db = db.Where("actor = ?", opts.Actor)
	}
	if opts.FunctionID != "" {
		db = db.Where("function_id = ?", opts.FunctionID)
	}
	if opts.Source != "" {
		db = db.Where("source = ?", opts.Source)
	}
	if opts.Status != "" {
		db = db.Where("status = ?", opts.Status)
	}
	if opts.TraceID != "" {
		db = db.Where("trace_id = ?", opts.TraceID)
	}
	if opts.From != nil {
		db = db.Where("created_at >= ?", *opts.From)
	}
	if opts.To != nil {
		db = db.Where("created_at <= ?", *opts.To)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if opts.Page <= 0 {
		opts.Page = 1
	}
	if opts.PageSize <= 0 || opts.PageSize > 200 {
		opts.PageSize = 20
	}
	var items []ExecutionLog
	err := db.Order("created_at DESC").Offset((opts.Page - 1) * opts.PageSize).Limit(opts.PageSize).Find(&items).Error
	return items, total, err
}

// DeleteBefore 删除创建时间早于 cutoff 的记录，返回删除行数（R3 保留期清理）。
func (m *ExecutionLogModel) DeleteBefore(ctx context.Context, cutoff time.Time, batch int) (int64, error) {
	return deleteBatch(ctx, m.db, &ExecutionLog{}, cutoff, batch)
}

type ExecutionLogListOptions struct {
	GameID     string
	Env        string
	Actor      string
	FunctionID string
	Source     string
	Status     string
	TraceID    string
	From       *time.Time
	To         *time.Time
	Page       int
	PageSize   int
}

// CreateBatch 批量写入（writer 消费侧使用）。
func (m *ExecutionLogModel) CreateBatch(ctx context.Context, items []ExecutionLog) error {
	if len(items) == 0 {
		return nil
	}
	return dbctx.Resolve(ctx, m.db).WithContext(ctx).Create(&items).Error
}

// deleteBatch 分批删除 created_at 早于 cutoff 的记录，返回总删除行数。
func deleteBatch(ctx context.Context, db *gorm.DB, dest interface{}, cutoff time.Time, batch int) (int64, error) {
	if batch <= 0 {
		batch = 1000
	}
	resolved := dbctx.Resolve(ctx, db).WithContext(ctx)
	var total int64
	for {
		res := resolved.Where("created_at < ?", cutoff).Limit(batch).Delete(dest)
		if res.Error != nil {
			return total, res.Error
		}
		total += res.RowsAffected
		if res.RowsAffected < int64(batch) {
			break
		}
	}
	return total, nil
}

// Get 按主键查询单条（含载荷）。
func (m *ExecutionLogModel) Get(ctx context.Context, id int64) (*ExecutionLog, error) {
	var item ExecutionLog
	if err := dbctx.Resolve(ctx, m.db).WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}
