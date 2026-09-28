package ops

import (
	"context"

	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/settings"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// LogsSettingsView log.* L3 覆盖值 + 逐键来源（OPEN-ISSUES #54）。
type LogsSettingsView struct {
	RetentionDays int               `json:"retentionDays"` // 0 = 跟随配置文件
	Sources       map[string]string `json:"sources"`
}

// LogsEffectiveView 实际生效的保留期（L3 覆盖 >0 时压过配置文件值；配置文件
// 未配置时 executionLog/taskLog 各自默认 7 天）。两类留痕在 L3 覆盖下同值。
type LogsEffectiveView struct {
	ExecutionLogDays int `json:"executionLogDays"` // 0 = 永久保留
	TaskLogDays      int `json:"taskLogDays"`
}

// ServerLogView 服务器自身日志文件（lumberjack 轮转）只读视图。
// 轮转参数为配置文件级（进程启动时创建 logger），热改需重启——本批只读展示。
type ServerLogView struct {
	Output     string `json:"output"`     // stdout|stderr|file
	File       string `json:"file"`       // 日志文件路径（非 file 输出时为空）
	Directory  string `json:"directory"`  // 日志目录（取 File 的目录部分）
	MaxSizeMB  int    `json:"maxSizeMB"`  // 单文件上限（MB）
	MaxBackups int    `json:"maxBackups"` // 保留旧文件数（0 = 仅当前文件）
	MaxAgeDays int    `json:"maxAgeDays"` // 保留天数（0 = 不按期清理）
	Compress   bool   `json:"compress"`   // 旧文件是否压缩
	FileCount  int    `json:"fileCount"`  // 目录下同前缀日志文件数（读目录失败为 0）
}

// LogTableView 单张留痕表的体量快照。
type LogTableView struct {
	Table    string `json:"table"`    // execution_logs | task_runs | task_events
	Rows     int64  `json:"rows"`     // 当前行数（查询失败/库不可达为 -1）
	OldestAt string `json:"oldestAt"` // 最老记录 created_at（RFC3339，空表为空串）
}

// LogsSnapshotResponse GET /ops/logs 响应。
type LogsSnapshotResponse struct {
	Settings  LogsSettingsView  `json:"settings"`
	Effective LogsEffectiveView `json:"effective"`
	ServerLog ServerLogView     `json:"serverLog"`
	Tables    []LogTableView    `json:"tables"`
}

// LogsGet GET /api/v1/ops/logs —— 日志维护快照：L3 覆盖值 + 实际生效保留期 +
// 服务器日志文件（只读）+ 留痕表体量。
//
// 边界（诚实，OPEN-ISSUES #54）：log.cleanupCron / log.copierDir /
// log.copierKeep 为已声明占位键、本批未接线（清理节奏固定每小时一轮；
// 服务器日志轮转目录/份数只读展示，改动走配置文件并重启）。
func (h *Handler) LogsGet(c *gin.Context) {
	response.Success(c, buildLogsSnapshot(h.service.svcCtx))
}

// LogsUpdate 写 L3 覆盖并热重载，返回更新后快照。仅收 log.retentionDays
// 一键（cleanupCron/copierDir/copierKeep 未接线，写入拒绝以免产生假开关）。
func (s *Service) LogsUpdate(ctx context.Context, req map[string]jsonNumber, updatedBy string) (LogsSnapshotResponse, error) {
	if s == nil || s.svcCtx == nil || s.svcCtx.PlatformSettingModel == nil {
		return LogsSnapshotResponse{}, errors.New("settings store unavailable")
	}
	for key, num := range req {
		if key != settings.KeyLogRetentionDays {
			return LogsSnapshotResponse{}, &errorx.CodeError{
				Code:       http.StatusBadRequest,
				Message:    fmt.Sprintf("未知或不可写的日志参数键：%s", key),
				StableCode: "invalid_setting_key",
			}
		}
		if num.Int64() < 0 || num.Int64() > 36500 {
			return LogsSnapshotResponse{}, &errorx.CodeError{
				Code:       http.StatusBadRequest,
				Message:    fmt.Sprintf("%s 取值 0-36500（0 = 跟随配置文件）", key),
				StableCode: "validation_failed",
			}
		}
	}
	store := s.svcCtx.PlatformSettingModel
	for key, num := range req {
		if err := store.Set(ctx, key, num.Raw(), updatedBy); err != nil {
			return LogsSnapshotResponse{}, err
		}
	}
	settings.Current().Reload(ctx, store)
	return buildLogsSnapshot(s.svcCtx), nil
}

// LogsPut PUT /api/v1/ops/logs —— 逐键写 log.* L3 覆盖。
func (h *Handler) LogsPut(c *gin.Context) {
	var req map[string]jsonNumber
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}
	snap, err := h.service.LogsUpdate(c.Request.Context(), req, perfUsername(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, snap)
}

// logsCleanupScopes 手动清理合法 scope。
var logsCleanupScopes = map[string]bool{"execution": true, "task": true, "all": true}

// LogsCleanupRequest POST /ops/logs/cleanup 请求体。
type LogsCleanupRequest struct {
	// Scope 清理范围：execution | task | all
	Scope string `json:"scope"`
	// BeforeHours 清理该时刻之前的记录（如 24 / 168 / 720；上限 87600 = 10 年）
	BeforeHours int `json:"beforeHours"`
}

// LogsCleanupResponse POST /ops/logs/cleanup 响应。
type LogsCleanupResponse struct {
	Scope                string `json:"scope"`
	Cutoff               string `json:"cutoff"` // RFC3339
	ExecutionLogsDeleted int64  `json:"executionLogsDeleted"`
	TaskRunsDeleted      int64  `json:"taskRunsDeleted"`
	TaskEventsDeleted    int64  `json:"taskEventsDeleted"`
}

// LogsCleanupPost POST /api/v1/ops/logs/cleanup —— 按时间手动清理留痕日志
// （24 小时前 / 7 天前 / 30 天前等任意小时数）。audit_records（哈希链审计）
// 不参与清理。
func (h *Handler) LogsCleanupPost(c *gin.Context) {
	var req LogsCleanupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, err)
		return
	}
	result, err := h.service.LogsCleanup(c.Request.Context(), req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, result)
}

// LogsCleanup 执行手动清理：优先走 LogRetention（multiGame 逐库 fanout 同
// 周期清理）；ExecutionLog 关闭（LogRetention nil）时回退 meta 库直清。
func (s *Service) LogsCleanup(ctx context.Context, req LogsCleanupRequest) (LogsCleanupResponse, error) {
	if !logsCleanupScopes[req.Scope] {
		return LogsCleanupResponse{}, &errorx.CodeError{
			Code:       http.StatusBadRequest,
			Message:    "scope 仅支持 execution / task / all",
			StableCode: "validation_failed",
		}
	}
	if req.BeforeHours <= 0 || req.BeforeHours > 87600 {
		return LogsCleanupResponse{}, &errorx.CodeError{
			Code:       http.StatusBadRequest,
			Message:    "beforeHours 取值 1-87600（小时；24/168/720 即 24 小时 / 7 天 / 30 天前）",
			StableCode: "validation_failed",
		}
	}
	if s == nil || s.svcCtx == nil {
		return LogsCleanupResponse{}, errors.New("service context unavailable")
	}
	cutoff := time.Now().UTC().Add(-time.Duration(req.BeforeHours) * time.Hour)
	result := LogsCleanupResponse{Scope: req.Scope, Cutoff: cutoff.Format(time.RFC3339)}

	if s.svcCtx.LogRetention != nil {
		summary, err := s.svcCtx.LogRetention.PurgeBefore(ctx, cutoff, req.Scope)
		if err != nil {
			return result, err
		}
		result.ExecutionLogsDeleted = summary.ExecutionLogsDeleted
		result.TaskRunsDeleted = summary.TaskRunsDeleted
		result.TaskEventsDeleted = summary.TaskEventsDeleted
		return result, nil
	}
	// 回退：留痕清理器未启动（ExecutionLog 关闭）时按 meta/单库直清
	db := s.svcCtx.DB
	if db == nil {
		return result, errors.New("database unavailable")
	}
	var err error
	if req.Scope == "execution" || req.Scope == "all" {
		result.ExecutionLogsDeleted, err = model.NewExecutionLogModel(db).DeleteBefore(ctx, cutoff, 1000)
		if err != nil {
			return result, err
		}
	}
	if req.Scope == "task" || req.Scope == "all" {
		result.TaskRunsDeleted, err = model.NewTaskRunModel(db).DeleteBefore(ctx, cutoff, 1000)
		if err != nil {
			return result, err
		}
		result.TaskEventsDeleted, err = model.NewTaskEventModel(db).DeleteBefore(ctx, cutoff, 1000)
		if err != nil {
			return result, err
		}
	}
	return result, nil
}

// buildLogsSnapshot 合成日志维护快照。
func buildLogsSnapshot(svcCtx *svc.ServiceContext) LogsSnapshotResponse {
	l3 := settings.Current().LogsSettings()
	snap := LogsSnapshotResponse{
		Settings: LogsSettingsView{
			RetentionDays: l3.RetentionDays,
			Sources:       l3.Sources,
		},
	}
	// 实际生效保留期：L3 覆盖 >0 时统一压过两类；否则按配置文件各自生效
	if svcCtx != nil {
		if l3.RetentionDays > 0 {
			snap.Effective = LogsEffectiveView{ExecutionLogDays: l3.RetentionDays, TaskLogDays: l3.RetentionDays}
		} else {
			snap.Effective = LogsEffectiveView{
				ExecutionLogDays: svcCtx.Config.ExecutionLog.EffectiveRetentionDays(),
				TaskLogDays:      svcCtx.Config.TaskLog.EffectiveRetentionDays(),
			}
		}
	}
	snap.ServerLog = buildServerLogView(svcCtx)
	snap.Tables = buildLogTableViews(svcCtx)
	return snap
}

// buildServerLogView 服务器日志文件只读视图（svcCtx 为 nil 或输出非 file 时
// FileCount 恒 0）。
func buildServerLogView(svcCtx *svc.ServiceContext) ServerLogView {
	view := ServerLogView{Output: "stdout"}
	if svcCtx == nil {
		return view
	}
	logCfg := svcCtx.Config.Logging
	if logCfg.Output != "" {
		view.Output = logCfg.Output
	}
	view.File = logCfg.File
	view.MaxSizeMB = logCfg.MaxSize
	view.MaxBackups = logCfg.MaxBackups
	view.MaxAgeDays = logCfg.MaxAge
	view.Compress = logCfg.Compress
	if logCfg.File != "" {
		view.Directory = filepath.Dir(logCfg.File)
		view.FileCount = countRotationFiles(logCfg.File)
	}
	return view
}

// countRotationFiles 统计日志目录下的轮转文件数（当前文件 + lumberjack 备份；
// 读目录失败按 0 降级）。lumberjack 备份命名会剥掉扩展名：
// `croupier.log` → 备份 `croupier-<timestamp>.log`。
func countRotationFiles(logFile string) int {
	dir := filepath.Dir(logFile)
	base := filepath.Base(logFile)
	prefix := strings.TrimSuffix(base, filepath.Ext(base)) + "-"
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || name == base {
			continue
		}
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, ".log") {
			count++
		}
	}
	return count + 1 // 当前文件
}

// buildLogTableViews 三张留痕表的行数 + 最老记录（表不存在/查询失败降级为
// Rows=-1，不阻塞快照）。
func buildLogTableViews(svcCtx *svc.ServiceContext) []LogTableView {
	tables := []string{"execution_logs", "task_runs", "task_events"}
	views := make([]LogTableView, 0, len(tables))
	var db *gorm.DB
	if svcCtx != nil {
		db = svcCtx.DB
	}
	for _, table := range tables {
		view := LogTableView{Table: table, Rows: -1}
		if db != nil {
			var rows int64
			if err := db.Table(table).Count(&rows).Error; err == nil {
				view.Rows = rows
				// 走模型化扫描而非 MIN() 聚合：sqlite 驱动对聚合列返回
				// string，NullTime 扫描不支持；Order+Limit 交给 gorm 转换
				var row struct {
					CreatedAt time.Time
				}
				if err := db.Table(table).Select("created_at").Order("created_at ASC").Limit(1).Scan(&row).Error; err == nil && !row.CreatedAt.IsZero() {
					view.OldestAt = row.CreatedAt.UTC().Format(time.RFC3339)
				}
			}
		}
		views = append(views, view)
	}
	return views
}
