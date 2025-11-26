package model

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ UserModel = (*customUserModel)(nil)

type (
	// UserModel 用户数据访问接口
	UserModel interface {
		Insert(ctx context.Context, data *User) (sql.Result, error)
		FindOne(ctx context.Context, id int64) (*User, error)
		FindOneByUsername(ctx context.Context, username string) (*User, error)
		Update(ctx context.Context, data *User) error
		Delete(ctx context.Context, id int64) error
		List(ctx context.Context, page, pageSize int) ([]*User, error)
		Count(ctx context.Context) (int64, error)
	}

	customUserModel struct {
		*defaultUserModel
	}

	defaultUserModel struct {
		sqlx.CachedConn
		table string
	}

	// User 用户实体
	User struct {
		Id        int64     `db:"id"`
		Username  string    `db:"username"`
		Password  string    `db:"password"`
		Nickname  string    `db:"nickname"`
		Email     string    `db:"email"`
		Phone     string    `db:"phone"`
		Roles     string    `db:"roles"`     // JSON 数组，如 ["admin", "user"]
		Status    int8      `db:"status"`    // 0:禁用 1:启用
		CreatedAt time.Time `db:"created_at"`
		UpdatedAt time.Time `db:"updated_at"`
	}
)

// NewUserModel 创建用户 Model
func NewUserModel(conn sqlx.SqlConn, c cache.CacheConf) UserModel {
	return &customUserModel{
		defaultUserModel: &defaultUserModel{
			CachedConn: sqlx.NewConn(conn, c),
			table:      "`users`",
		},
	}
}

// Insert 插入用户
func (m *defaultUserModel) Insert(ctx context.Context, data *User) (sql.Result, error) {
	query := fmt.Sprintf("INSERT INTO %s (username, password, nickname, email, phone, roles, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)", m.table)

	now := NowFunc()
	if data.CreatedAt.IsZero() {
		data.CreatedAt = now
	}
	if data.UpdatedAt.IsZero() {
		data.UpdatedAt = now
	}

	ret, err := m.ExecCtx(ctx, func(ctx context.Context, conn sqlx.SqlConn) (result sql.Result, err error) {
		return conn.ExecCtx(ctx, query,
			data.Username,
			data.Password,
			data.Nickname,
			data.Email,
			data.Phone,
			data.Roles,
			data.Status,
			data.CreatedAt,
			data.UpdatedAt,
		)
	})

	return ret, err
}

// FindOne 根据 ID 查找用户
func (m *defaultUserModel) FindOne(ctx context.Context, id int64) (*User, error) {
	cacheKey := fmt.Sprintf("cache:user:id:%d", id)
	var resp User

	err := m.QueryRowCtx(ctx, &resp, cacheKey, func(ctx context.Context, conn sqlx.SqlConn, v interface{}) error {
		query := fmt.Sprintf("SELECT * FROM %s WHERE id = ? LIMIT 1", m.table)
		return conn.QueryRowCtx(ctx, v, query, id)
	})

	if err != nil {
		return nil, err
	}

	return &resp, nil
}

// FindOneByUsername 根据用户名查找用户
func (m *defaultUserModel) FindOneByUsername(ctx context.Context, username string) (*User, error) {
	cacheKey := fmt.Sprintf("cache:user:username:%s", username)
	var resp User

	err := m.QueryRowCtx(ctx, &resp, cacheKey, func(ctx context.Context, conn sqlx.SqlConn, v interface{}) error {
		query := fmt.Sprintf("SELECT * FROM %s WHERE username = ? LIMIT 1", m.table)
		return conn.QueryRowCtx(ctx, v, query, username)
	})

	if err != nil {
		return nil, err
	}

	return &resp, nil
}

// Update 更新用户
func (m *defaultUserModel) Update(ctx context.Context, data *User) error {
	data.UpdatedAt = NowFunc()

	query := fmt.Sprintf("UPDATE %s SET username = ?, password = ?, nickname = ?, email = ?, phone = ?, roles = ?, status = ?, updated_at = ? WHERE id = ?", m.table)

	_, err := m.ExecCtx(ctx, func(ctx context.Context, conn sqlx.SqlConn) (result sql.Result, err error) {
		return conn.ExecCtx(ctx, query,
			data.Username,
			data.Password,
			data.Nickname,
			data.Email,
			data.Phone,
			data.Roles,
			data.Status,
			data.UpdatedAt,
			data.Id,
		)
	}, m.getCacheKeys(data)...)

	return err
}

// Delete 删除用户
func (m *defaultUserModel) Delete(ctx context.Context, id int64) error {
	data, err := m.FindOne(ctx, id)
	if err != nil {
		return err
	}

	query := fmt.Sprintf("DELETE FROM %s WHERE id = ?", m.table)
	_, err = m.ExecCtx(ctx, func(ctx context.Context, conn sqlx.SqlConn) (result sql.Result, err error) {
		return conn.ExecCtx(ctx, query, id)
	}, m.getCacheKeys(data)...)

	return err
}

// List 分页查询用户列表
func (m *defaultUserModel) List(ctx context.Context, page, pageSize int) ([]*User, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}

	offset := (page - 1) * pageSize
	query := fmt.Sprintf("SELECT * FROM %s ORDER BY id DESC LIMIT ? OFFSET ?", m.table)

	var users []*User
	err := m.QueryRowsNoCacheCtx(ctx, &users, query, pageSize, offset)
	if err != nil {
		return nil, err
	}

	return users, nil
}

// Count 统计用户总数
func (m *defaultUserModel) Count(ctx context.Context) (int64, error) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", m.table)

	var count int64
	err := m.QueryRowNoCacheCtx(ctx, &count, query)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// getCacheKeys 获取缓存键列表
func (m *defaultUserModel) getCacheKeys(data *User) []string {
	return []string{
		fmt.Sprintf("cache:user:id:%d", data.Id),
		fmt.Sprintf("cache:user:username:%s", data.Username),
	}
}

// formatPrimary 格式化主键缓存键
func (m *defaultUserModel) formatPrimary(primary interface{}) string {
	return fmt.Sprintf("cache:user:id:%v", primary)
}

// tableName 获取表名（用于其他方法）
func (m *defaultUserModel) tableName() string {
	return strings.Trim(m.table, "`")
}
