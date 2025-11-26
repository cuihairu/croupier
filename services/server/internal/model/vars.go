package model

import (
	"database/sql"
	"errors"
	"time"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var (
	// ErrNotFound 记录不存在
	ErrNotFound = sqlx.ErrNotFound

	// ErrDuplicateKey 唯一键冲突
	ErrDuplicateKey = errors.New("duplicate key error")

	// ErrInvalidData 无效数据
	ErrInvalidData = errors.New("invalid data")
)

// CachedConn 缓存连接接口
type CachedConn interface {
	sqlx.SqlConn
	DelCache(keys ...string) error
	GetCache(key string, v interface{}) error
	SetCache(key string, v interface{}) error
}

// NowFunc 获取当前时间，便于测试
var NowFunc = func() time.Time {
	return time.Now()
}

// NullString 处理可空字符串
func NullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}

// NullInt64 处理可空 int64
func NullInt64(i int64) sql.NullInt64 {
	if i == 0 {
		return sql.NullInt64{Valid: false}
	}
	return sql.NullInt64{Int64: i, Valid: true}
}

// NullTime 处理可空时间
func NullTime(t time.Time) sql.NullTime {
	if t.IsZero() {
		return sql.NullTime{Valid: false}
	}
	return sql.NullTime{Time: t, Valid: true}
}
