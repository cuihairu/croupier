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
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/cuihairu/croupier/internal/model"
	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

// exit 是 os.Exit 的测试注入点：main 只经由它退出，测试替换后可直接调用
// main 断言退出码而不终止测试进程；生产路径等价于 os.Exit(code)。
var exit = os.Exit

func main() {
	exit(runMain(os.Args[0], os.Args[1:], os.Stderr))
}

// runMain 解析 flag 并执行种子写入，返回进程退出码：
// 0=成功，1=运行期失败（诊断已写 output），2=flag 解析错误（-h 为 0）。
func runMain(prog string, args []string, output io.Writer) int {
	fs := flag.NewFlagSet(prog, flag.ContinueOnError)
	fs.SetOutput(output)
	dsn := fs.String("dsn", "test-data/croupier.db", "sqlite database path")
	functionID := fs.String("function-id", "", "function id (required)")
	gameID := fs.String("game-id", "", "game scope id")
	displayName := fs.String("name", "E2E Seed Function", "function display name")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	if strings.TrimSpace(*functionID) == "" {
		return fail(output, "-function-id is required")
	}

	// _busy_timeout lets us wait briefly if the server holds the write lock,
	// instead of erroring with "database is locked".
	db, err := gorm.Open(gsqlite.Open(*dsn+"?_pragma=busy_timeout(5000)"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return fail(output, "open %s: %v", *dsn, err)
	}

	// Ensure the table matches the server schema (AutoMigrate is idempotent;
	// it only adds missing columns/indices, never drops data).
	if err := db.AutoMigrate(&model.Function{}); err != nil {
		return fail(output, "auto-migrate functions: %T: %v", err, err)
	}

	// INSERT OR IGNORE: idempotent across runs (function_id is unique).
	fn := model.Function{
		FunctionID: *functionID,
		Name:       *displayName,
		GameID:     *gameID,
		Version:    "1.0.0",
		Status:     1,
	}
	res := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&fn)
	if res.Error != nil {
		return fail(output, "seed function %s: %T: %v", *functionID, res.Error, res.Error)
	}

	fmt.Fprintf(output, "function-seed: id=%s game=%s status=ok (rows_affected=%d)\n",
		*functionID, *gameID, res.RowsAffected)
	return 0
}

// fail 打印诊断并返回退出码 1；真正的进程出口由 main 的 exit 执行。
func fail(output io.Writer, format string, args ...any) int {
	fmt.Fprintf(output, "function-seed: FAIL — "+format+"\n", args...)
	return 1
}
