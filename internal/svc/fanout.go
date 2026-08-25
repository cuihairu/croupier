package svc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/db/migrate"
	"github.com/cuihairu/croupier/internal/model"
	"gorm.io/gorm"
)

// FanoutReport describes the migration outcome for one game database during a
// fan-out pass (phase 5 of docs/architecture/database-migration-strategy.md).
type FanoutReport struct {
	GameID   string
	Env      string
	Database string
	Before   int64 // version before this pass (0 when unknown/new)
	After    int64 // version after this pass (equals Before on dry-run)
	Status   string
	Err      string
}

// Missing/legacy markers used in FanoutReport.Status.
const (
	FanoutStatusMigrated = "migrated"
	FanoutStatusCurrent  = "current"
	FanoutStatusMissing  = "missing-database"
	FanoutStatusError    = "error"
)

// RunMigrationFanout rolls the versioned migration forward across the meta
// database and every registered game database (game_envs registry). With
// dryRun set it only reports versions and never executes DDL.
//
// Databases that the registry references but that do not physically exist yet
// are reported as missing — the runtime lazily creates them on first access,
// so the fan-out tool intentionally does not create them.
func RunMigrationFanout(ctx context.Context, cfg config.Config, dryRun bool) ([]FanoutReport, error) {
	driver, metaDSN := resolveDriverAndDSN(cfg)

	metaDB, err := OpenGormForRouter(driver, metaDSN)
	if err != nil {
		return nil, fmt.Errorf("fanout: open meta database: %w", err)
	}

	reports := []FanoutReport{}

	// Meta database first (same baseline contract as server startup).
	metaReport := FanoutReport{GameID: "-", Env: "-", Database: "(meta)"}
	metaReport.Before, _, _ = currentVersionOf(ctx, metaDB, driver)
	if dryRun {
		metaReport.After = metaReport.Before
		metaReport.Status = FanoutStatusCurrent
	} else {
		version, err := migrate.EnsureUpToDate(ctx, metaDB, migrate.ScopeMeta, autoMigrateMeta)
		if err != nil {
			metaReport.Status = FanoutStatusError
			metaReport.Err = err.Error()
		} else {
			metaReport.After = version
			metaReport.Status = statusFor(metaReport.Before, version)
		}
	}
	reports = append(reports, metaReport)

	if !cfg.Database.MultiGame {
		// Single-database deployments have no per-game databases to roll.
		return reports, nil
	}

	bindings, err := model.NewGameModel(metaDB).ListAllEnvBindings(ctx)
	if err != nil {
		return reports, fmt.Errorf("fanout: list game_envs: %w", err)
	}

	for _, b := range bindings {
		report := FanoutReport{GameID: b.GameID, Env: b.Env}
		report.Database = gameDBNameFor(cfg, b.GameID, b.Env)
		gameDSN := DSNForDatabase(driver, metaDSN, report.Database)

		gameDB, err := OpenGormForRouter(driver, gameDSN)
		if err != nil {
			// A registry row without a physical database is not fatal: the
			// runtime creates game databases lazily on first access.
			report.Status = FanoutStatusMissing
			report.Err = err.Error()
			reports = append(reports, report)
			continue
		}

		report.Before, _, _ = currentVersionOf(ctx, gameDB, driver)
		if dryRun {
			report.After = report.Before
			report.Status = FanoutStatusCurrent
			sqlDB, _ := gameDB.DB()
			if sqlDB != nil {
				_ = sqlDB.Close()
			}
			reports = append(reports, report)
			continue
		}

		version, err := migrate.EnsureUpToDate(ctx, gameDB, migrate.ScopeGame, model.AutoMigrateGame)
		if sqlDB, _ := gameDB.DB(); sqlDB != nil {
			_ = sqlDB.Close()
		}
		if err != nil {
			report.Status = FanoutStatusError
			report.Err = err.Error()
		} else {
			report.After = version
			report.Status = statusFor(report.Before, version)
		}
		reports = append(reports, report)

		slog.Default().Info("fanout: game database processed",
			"gameId", b.GameID, "env", b.Env, "database", report.Database,
			"before", report.Before, "after", report.After, "status", report.Status)
	}

	return reports, nil
}

func statusFor(before, after int64) string {
	if after > before {
		return FanoutStatusMigrated
	}
	return FanoutStatusCurrent
}

func currentVersionOf(ctx context.Context, db *gorm.DB, driver string) (int64, bool, error) {
	sqlDB, err := db.DB()
	if err != nil {
		return 0, false, err
	}
	return migrate.CurrentVersion(ctx, sqlDB, driver)
}

// FormatFanoutReports renders the fan-out pass as a fixed-width table.
func FormatFanoutReports(reports []FanoutReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%-20s %-14s %-32s %8s %8s  %s\n", "GAME", "ENV", "DATABASE", "BEFORE", "AFTER", "STATUS")
	for _, r := range reports {
		status := r.Status
		if r.Err != "" && r.Status != FanoutStatusMissing {
			status = fmt.Sprintf("%s (%s)", status, truncate(r.Err, 80))
		}
		fmt.Fprintf(&b, "%-20s %-14s %-32s %8d %8d  %s\n", r.GameID, r.Env, r.Database, r.Before, r.After, status)
	}
	var failed int
	for _, r := range reports {
		if r.Status == FanoutStatusError {
			failed++
		}
	}
	fmt.Fprintf(&b, "\ntotal=%d migrated=%d error=%d\n", len(reports), countBy(reports, FanoutStatusMigrated), failed)
	return b.String()
}

func countBy(reports []FanoutReport, status string) int {
	n := 0
	for _, r := range reports {
		if r.Status == status {
			n++
		}
	}
	return n
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// ErrFanoutFailures is returned by command wrappers when any database failed.
var ErrFanoutFailures = errors.New("fanout: one or more databases failed")
