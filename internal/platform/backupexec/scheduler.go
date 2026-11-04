// Scheduler 定期备份调度（OPS-008 Stage E）。枚举注册启用的数据源（经
// capture 所见的数据源登记），逐源触发一次真实备份。默认不装配：由
// config backup.schedule.enabled（缺省 false，须显式 true）门控，服务启动
// 时才会派生后台循环。
package backupexec

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/objstore"
)

// defaultScheduledRunTimeout 单次调度备份的运行预算（与 ops 手动备份一致）。
const defaultScheduledRunTimeout = 30 * time.Minute

// defaultSchedulerInterval 是 Loop 的防御性缺省周期（生产由 config
// backup.schedule.interval 的 EffectiveInterval 解析为 6h 后传入，不会为 0）。
const defaultSchedulerInterval = 6 * time.Hour

// Scheduler 周期枚举数据源并触发备份。
type Scheduler struct {
	backups *model.BackupModel
	sources *model.DBSourceModel
	store   objstore.Store

	mu       sync.Mutex
	inflight map[string]bool // "game/env" -> 该源一次备份在途
}

// NewScheduler 构造调度器。store 可为 nil（无对象存储时每个 tick 报错，
// 调用方按未装配处理）。
func NewScheduler(backups *model.BackupModel, sources *model.DBSourceModel, store objstore.Store) *Scheduler {
	return &Scheduler{
		backups:  backups,
		sources:  sources,
		store:    store,
		inflight: map[string]bool{},
	}
}

// RunOnce 枚举注册启用的数据源，逐源触发一次备份（异步执行，不阻塞 tick）。
// 已在途的源跳过；返回首个同步错误（跳过/异步失败不计）。无对象存储时报错。
func (s *Scheduler) RunOnce(ctx context.Context) error {
	if s == nil || s.sources == nil || s.backups == nil {
		return fmt.Errorf("backup scheduler not initialized")
	}
	if s.store == nil {
		return fmt.Errorf("object store not configured for scheduled backups")
	}
	srcs, err := s.sources.List(ctx)
	if err != nil {
		return fmt.Errorf("list db sources: %w", err)
	}
	var firstErr error
	for _, src := range srcs {
		if !src.Enabled {
			continue
		}
		driver := strings.ToLower(strings.TrimSpace(src.Driver))
		if driver != "mysql" && driver != "postgres" && driver != "sqlite" {
			continue // 调度只认数据面可执行的驱动
		}
		scope := src.GameID + "/" + src.Env
		if !s.begin(scope) {
			continue // 该源一次备份在途，跳过
		}
		exec := New(driver, src.DSN, s.backups, s.store, "backups/"+scope)
		backup := &model.Backup{
			BackupID:   uuid.New().String(),
			Name:       "scheduled-" + time.Now().UTC().Format("20060102-150405"),
			Type:       "full",
			Status:     "pending",
			GameID:     src.GameID,
			Env:        src.Env,
			SourceID:   src.ID,
			SourceName: src.Name,
		}
		if err := s.backups.Create(ctx, backup); err != nil {
			s.end(scope)
			if firstErr == nil {
				firstErr = fmt.Errorf("create scheduled backup for %s: %w", scope, err)
			}
			continue
		}
		backupID, bname, btype := backup.BackupID, backup.Name, backup.Type
		go func() {
			defer s.end(scope)
			bg, cancel := context.WithTimeout(context.Background(), defaultScheduledRunTimeout)
			defer cancel()
			if err := exec.RunBackup(bg, backupID, bname, btype); err != nil {
				slog.Error("scheduled backup run failed", "scope", scope, "backupID", backupID, "error", err)
			}
		}()
	}
	return firstErr
}

// Loop 周期触发 RunOnce，直至 ctx 取消。
func (s *Scheduler) Loop(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = defaultSchedulerInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.RunOnce(ctx); err != nil {
				slog.Error("scheduled backup pass failed", "error", err)
			}
		}
	}
}

// begin 尝试标记 scope 在途；已在途返回 false。
func (s *Scheduler) begin(scope string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.inflight[scope] {
		return false
	}
	s.inflight[scope] = true
	return true
}

func (s *Scheduler) end(scope string) {
	s.mu.Lock()
	delete(s.inflight, scope)
	s.mu.Unlock()
}
