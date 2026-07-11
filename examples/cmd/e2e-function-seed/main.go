// Command e2e-function-seed inserts a minimal function metadata row into the
// server's SQLite DB so the task-lifecycle E2E can call POST /api/v1/tasks.
//
// The REST task entrypoint requires the function to exist in FunctionModel
// (FindByFunctionID). There is no single-function create API (functions are
// normally created via pack import / approval), so the E2E seeds a row
// directly. It uses the same pure-Go sqlite driver as the server and is
// idempotent (FirstOrCreate), so re-running it is safe.
//
// Run after the server has booted (so the functions table exists), before the
// probe serves. Exit 0 on success, 1 on failure.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/cuihairu/croupier/internal/model"
	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

func main() {
	dsn := flag.String("dsn", "test-data/croupier.db", "sqlite database path")
	functionID := flag.String("function-id", "", "function id (required)")
	gameID := flag.String("game-id", "", "game scope id")
	name := flag.String("name", "E2E Seed Function", "function display name")
	flag.Parse()

	if strings.TrimSpace(*functionID) == "" {
		fail("-function-id is required")
	}

	// _busy_timeout lets us wait briefly if the server holds the write lock,
	// instead of erroring with "database is locked".
	db, err := gorm.Open(gsqlite.Open(*dsn+"?_pragma=busy_timeout(5000)"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		fail("open %s: %v", *dsn, err)
	}

	// Ensure the table matches the server schema (AutoMigrate is idempotent;
	// it only adds missing columns/indices, never drops data).
	if err := db.AutoMigrate(&model.Function{}); err != nil {
		fail("auto-migrate functions: %T: %v", err, err)
	}

	// INSERT OR IGNORE: idempotent across runs (function_id is unique).
	fn := model.Function{
		FunctionID: *functionID,
		Name:       *name,
		GameID:     *gameID,
		Version:    "1.0.0",
		Status:     1,
	}
	res := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&fn)
	if res.Error != nil {
		fail("seed function %s: %T: %v", *functionID, res.Error, res.Error)
	}

	fmt.Fprintf(os.Stderr, "function-seed: id=%s game=%s status=ok (rows_affected=%d)\n",
		*functionID, *gameID, res.RowsAffected)
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "function-seed: FAIL — "+format+"\n", args...)
	os.Exit(1)
}
