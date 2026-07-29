package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/svc"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

var legacyWorkspaceReportLimit int

var legacyWorkspaceReportCmd = &cobra.Command{
	Use:   "legacy-workspace-report",
	Short: "输出旧 workspace_configs 只读诊断报告",
	Long:  `检查数据库中是否仍存在旧 workspace_configs 表与历史数据，只输出诊断报告，不执行迁移、不写入 PageSpec。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfigFile(cfgFile)
		if err != nil {
			return err
		}

		db, err := svc.OpenReadOnlyDatabase(cfg)
		if err != nil {
			return fmt.Errorf("打开只读数据库失败: %w", err)
		}
		sqlDB, err := db.DB()
		if err != nil {
			return fmt.Errorf("获取底层数据库连接失败: %w", err)
		}
		defer sqlDB.Close()

		report, err := buildLegacyWorkspaceReport(context.Background(), db, legacyWorkspaceReportLimit)
		if err != nil {
			return err
		}

		driver, dsn := svc.ResolveDriverAndDSN(cfg)
		renderLegacyWorkspaceReport(cmd.OutOrStdout(), cfgFile, driver, summarizeDSN(dsn), report)
		return nil
	},
}

func init() {
	legacyWorkspaceReportCmd.Flags().IntVar(&legacyWorkspaceReportLimit, "limit", 200, "报告中最多展示多少条旧 workspace 记录")
}

type legacyWorkspaceRow struct {
	GameID    sql.NullString `gorm:"column:game_id"`
	Env       sql.NullString `gorm:"column:env"`
	ObjectKey string         `gorm:"column:object_key"`
	Published sql.NullBool   `gorm:"column:published"`
	UpdatedAt sql.NullTime   `gorm:"column:updated_at"`
}

type legacyWorkspaceItem struct {
	ScopeStatus string
	GameID      string
	Env         string
	ObjectKey   string
	Published   bool
	UpdatedAt   *time.Time
}

type legacyWorkspaceReport struct {
	TableExists        bool
	Columns            []string
	TotalCount         int64
	ScopedCount        int64
	UnknownScopeCount  int64
	PublishedTrueCount int64
	Items              []legacyWorkspaceItem
}

func buildLegacyWorkspaceReport(ctx context.Context, db *gorm.DB, limit int) (*legacyWorkspaceReport, error) {
	if limit <= 0 {
		limit = 200
	}

	report := &legacyWorkspaceReport{}
	if !db.Migrator().HasTable("workspace_configs") {
		return report, nil
	}
	report.TableExists = true

	columns, err := readLegacyWorkspaceColumns(db)
	if err != nil {
		return nil, fmt.Errorf("读取 workspace_configs 列定义失败: %w", err)
	}
	report.Columns = columns

	columnSet := make(map[string]struct{}, len(columns))
	for _, column := range columns {
		columnSet[column] = struct{}{}
	}

	query := db.WithContext(ctx).Table("workspace_configs")
	if err := query.Count(&report.TotalCount).Error; err != nil {
		return nil, fmt.Errorf("统计 workspace_configs 数量失败: %w", err)
	}

	hasGameID := hasLegacyWorkspaceColumn(columnSet, "game_id")
	hasEnv := hasLegacyWorkspaceColumn(columnSet, "env")
	hasPublished := hasLegacyWorkspaceColumn(columnSet, "published")
	hasUpdatedAt := hasLegacyWorkspaceColumn(columnSet, "updated_at")

	if hasGameID && hasEnv {
		scopeQuery := db.WithContext(ctx).Table("workspace_configs").
			Where("COALESCE(game_id, '') <> '' AND COALESCE(env, '') <> ''")
		if err := scopeQuery.Count(&report.ScopedCount).Error; err != nil {
			return nil, fmt.Errorf("统计已确定 scope 的旧 workspace 失败: %w", err)
		}
		report.UnknownScopeCount = report.TotalCount - report.ScopedCount
	} else {
		report.UnknownScopeCount = report.TotalCount
	}

	if hasPublished {
		publishedQuery := db.WithContext(ctx).Table("workspace_configs").Where("published = ?", true)
		if err := publishedQuery.Count(&report.PublishedTrueCount).Error; err != nil {
			return nil, fmt.Errorf("统计已发布旧 workspace 失败: %w", err)
		}
	}

	selectColumns := []string{"object_key"}
	if hasGameID {
		selectColumns = append(selectColumns, "game_id")
	}
	if hasEnv {
		selectColumns = append(selectColumns, "env")
	}
	if hasPublished {
		selectColumns = append(selectColumns, "published")
	}
	if hasUpdatedAt {
		selectColumns = append(selectColumns, "updated_at")
	}

	rows := make([]legacyWorkspaceRow, 0, limit)
	listQuery := db.WithContext(ctx).Table("workspace_configs").Select(strings.Join(selectColumns, ", "))
	if hasUpdatedAt {
		listQuery = listQuery.Order("updated_at DESC").Order("object_key ASC")
	} else {
		listQuery = listQuery.Order("object_key ASC")
	}
	if err := listQuery.Limit(limit).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("读取旧 workspace 记录失败: %w", err)
	}

	report.Items = make([]legacyWorkspaceItem, 0, len(rows))
	for _, row := range rows {
		item := legacyWorkspaceItem{
			ObjectKey: row.ObjectKey,
			Published: row.Published.Valid && row.Published.Bool,
		}
		if row.UpdatedAt.Valid {
			updatedAt := row.UpdatedAt.Time.UTC()
			item.UpdatedAt = &updatedAt
		}
		if row.GameID.Valid {
			item.GameID = strings.TrimSpace(row.GameID.String)
		}
		if row.Env.Valid {
			item.Env = strings.TrimSpace(row.Env.String)
		}
		if item.GameID != "" && item.Env != "" {
			item.ScopeStatus = "scoped"
		} else {
			item.ScopeStatus = "unknown"
		}
		report.Items = append(report.Items, item)
	}

	return report, nil
}

func readLegacyWorkspaceColumns(db *gorm.DB) ([]string, error) {
	rows, err := db.Table("workspace_configs").Select("*").Limit(1).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	sort.Strings(columns)
	return columns, nil
}

func hasLegacyWorkspaceColumn(columns map[string]struct{}, column string) bool {
	_, ok := columns[column]
	return ok
}

func renderLegacyWorkspaceReport(w io.Writer, cfgPath, driver, dsn string, report *legacyWorkspaceReport) {
	fmt.Fprintf(w, "legacy workspace report\n")
	fmt.Fprintf(w, "config: %s\n", cfgPath)
	fmt.Fprintf(w, "driver: %s\n", driver)
	fmt.Fprintf(w, "database_source: %s\n", dsn)
	if !report.TableExists {
		fmt.Fprintln(w, "table_exists: false")
		fmt.Fprintln(w, "message: workspace_configs 已不存在，无旧页面配置残留。")
		return
	}

	fmt.Fprintln(w, "table_exists: true")
	fmt.Fprintf(w, "columns: %s\n", strings.Join(report.Columns, ", "))
	fmt.Fprintf(w, "total: %d\n", report.TotalCount)
	fmt.Fprintf(w, "scoped: %d\n", report.ScopedCount)
	fmt.Fprintf(w, "unknown_scope: %d\n", report.UnknownScopeCount)
	fmt.Fprintf(w, "published_true: %d\n", report.PublishedTrueCount)
	fmt.Fprintf(w, "sample_limit: %d\n", len(report.Items))

	if len(report.Items) == 0 {
		fmt.Fprintln(w, "items: none")
		return
	}

	fmt.Fprintln(w, "items:")
	for _, item := range report.Items {
		scope := "unknown"
		if item.ScopeStatus == "scoped" {
			scope = item.GameID + "/" + item.Env
		}

		updatedAt := "-"
		if item.UpdatedAt != nil {
			updatedAt = item.UpdatedAt.Format(time.RFC3339)
		}

		fmt.Fprintf(
			w,
			"- scope=%s scope_status=%s object_key=%s published=%t updated_at=%s\n",
			scope,
			item.ScopeStatus,
			item.ObjectKey,
			item.Published,
			updatedAt,
		)
	}
}

func summarizeDSN(dsn string) string {
	if strings.TrimSpace(dsn) == "" {
		return "default"
	}
	return "configured"
}
