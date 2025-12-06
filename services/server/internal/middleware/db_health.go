package middleware

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
)

// DBHealth 数据库健康检查
type DBHealth struct {
	svcCtx *svc.ServiceContext
}

// NewDBHealth 创建数据库健康检查实例
func NewDBHealth(svcCtx *svc.ServiceContext) *DBHealth {
	return &DBHealth{
		svcCtx: svcCtx,
	}
}

// Check 检查数据库连接
func (h *DBHealth) Check(ctx context.Context) error {
	if h.svcCtx == nil || h.svcCtx.AdminModel == nil {
		return fmt.Errorf("数据库模型未初始化")
	}

	// 执行简单的查询来检查数据库连接
	queryCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	// 尝试查询一个管理员（不关心结果）
	_, err := h.svcCtx.AdminModel.FindOne(queryCtx, 1)
	if err != nil && err != sql.ErrNoRows {
		logx.Errorf("Database health check failed: %v", err)
		return fmt.Errorf("数据库连接检查失败: %w", err)
	}

	return nil
}

// Ping 简单的 ping 检查
func (h *DBHealth) Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return h.Check(ctx)
}