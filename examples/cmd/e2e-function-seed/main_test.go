package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const progName = "e2e-function-seed"

// runSeed 在进程内执行 runMain，返回退出码与全部诊断输出。
func runSeed(t *testing.T, args ...string) (int, string) {
	t.Helper()
	var out bytes.Buffer
	code := runMain(progName, args, &out)
	return code, out.String()
}

// openSeedDB 用与被测程序相同的 driver/DSN 形态打开库（测试造数据用）。
func openSeedDB(t *testing.T, dsn string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(dsn+"?_pragma=busy_timeout(5000)"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open %s: %v", dsn, err)
	}
	return db
}

func TestRunMainMissingFunctionID(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"absent", nil},
		{"empty", []string{"-function-id", ""}},
		{"blank", []string{"-function-id", "   "}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, out := runSeed(t, tc.args...)
			if code != 1 {
				t.Fatalf("exit code = %d, want 1 (out=%q)", code, out)
			}
			if !strings.Contains(out, "FAIL — -function-id is required") {
				t.Fatalf("missing diagnostic, got %q", out)
			}
		})
	}
}

func TestRunMainFlagErrors(t *testing.T) {
	t.Run("help", func(t *testing.T) {
		code, out := runSeed(t, "-h")
		if code != 0 {
			t.Fatalf("exit code = %d, want 0 (out=%q)", code, out)
		}
		for _, want := range []string{"Usage of e2e-function-seed:", "-function-id", "-dsn"} {
			if !strings.Contains(out, want) {
				t.Fatalf("usage output missing %q, got %q", want, out)
			}
		}
	})
	t.Run("unknown flag", func(t *testing.T) {
		code, out := runSeed(t, "-nope")
		if code != 2 {
			t.Fatalf("exit code = %d, want 2 (out=%q)", code, out)
		}
		if !strings.Contains(out, "flag provided but not defined: -nope") {
			t.Fatalf("missing flag error, got %q", out)
		}
	})
}

func TestRunMainOpenFailure(t *testing.T) {
	// 目录不能当 sqlite 库文件：gorm.Open 阶段即失败。
	dsn := t.TempDir()
	code, out := runSeed(t, "-dsn", dsn, "-function-id", "e2e.echo")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (out=%q)", code, out)
	}
	if !strings.Contains(out, "FAIL — open "+dsn) {
		t.Fatalf("missing open diagnostic, got %q", out)
	}
	if !strings.Contains(out, "unable to open database file") {
		t.Fatalf("missing sqlite driver error, got %q", out)
	}
}

func TestRunMainAutoMigrateFailure(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "migrate.db")
	// 预置一个与 gorm 将创建的唯一索引同名的表，令 AutoMigrate 建索引失败。
	openSeedDB(t, dsn).Exec(`CREATE TABLE idx_functions_function_id (x INTEGER)`)

	code, out := runSeed(t, "-dsn", dsn, "-function-id", "e2e.echo")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (out=%q)", code, out)
	}
	if !strings.Contains(out, "FAIL — auto-migrate functions:") {
		t.Fatalf("missing auto-migrate diagnostic, got %q", out)
	}
	if !strings.Contains(out, "already a table named idx_functions_function_id") {
		t.Fatalf("missing sqlite schema error, got %q", out)
	}
}

func TestRunMainSeedIdempotent(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "seed.db")

	code, out := runSeed(t, "-dsn", dsn, "-function-id", "e2e.echo", "-game-id", "e2e-game", "-name", "Echo")
	if code != 0 {
		t.Fatalf("first run exit code = %d, want 0 (out=%q)", code, out)
	}
	if !strings.Contains(out, "function-seed: id=e2e.echo game=e2e-game status=ok (rows_affected=1)") {
		t.Fatalf("first run output mismatch, got %q", out)
	}

	// 重复执行命中 OnConflict DoNothing：不再插入新行。
	code, out = runSeed(t, "-dsn", dsn, "-function-id", "e2e.echo", "-game-id", "e2e-game", "-name", "Echo")
	if code != 0 {
		t.Fatalf("second run exit code = %d, want 0 (out=%q)", code, out)
	}
	if !strings.Contains(out, "status=ok (rows_affected=0)") {
		t.Fatalf("second run should be a no-op, got %q", out)
	}

	var fn model.Function
	if err := openSeedDB(t, dsn).Where("function_id = ?", "e2e.echo").First(&fn).Error; err != nil {
		t.Fatalf("seeded row not found: %v", err)
	}
	if fn.GameID != "e2e-game" || fn.Name != "Echo" || fn.Version != "1.0.0" || fn.Status != 1 {
		t.Fatalf("seeded row = %+v, want game=e2e-game name=Echo version=1.0.0 status=1", fn)
	}
}

func TestRunMainInsertFailure(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "trigger.db")
	db := openSeedDB(t, dsn)
	if err := db.AutoMigrate(&model.Function{}); err != nil {
		t.Fatalf("auto-migrate: %v", err)
	}
	// 让后续 INSERT 被触发器拒绝，覆盖 res.Error 分支。
	if err := db.Exec(`CREATE TRIGGER seed_reject BEFORE INSERT ON functions BEGIN SELECT RAISE(ABORT, 'seed rejected'); END`).Error; err != nil {
		t.Fatalf("create trigger: %v", err)
	}

	code, out := runSeed(t, "-dsn", dsn, "-function-id", "e2e.echo")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (out=%q)", code, out)
	}
	if !strings.Contains(out, "FAIL — seed function e2e.echo:") {
		t.Fatalf("missing seed diagnostic, got %q", out)
	}
	if !strings.Contains(out, "seed rejected") {
		t.Fatalf("missing trigger error, got %q", out)
	}
}

func TestMainUsesExitHook(t *testing.T) {
	origArgs, origExit := os.Args, exit
	defer func() { os.Args, exit = origArgs, origExit }()

	cases := []struct {
		name string
		args []string
		want int
	}{
		{"missing id", []string{progName}, 1},
		{"help", []string{progName, "-h"}, 0},
		{"seed", []string{progName, "-dsn", filepath.Join(t.TempDir(), "hook.db"), "-function-id", "e2e.echo"}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got []int
			exit = func(code int) { got = append(got, code) }
			os.Args = tc.args
			main()
			if len(got) != 1 || got[0] != tc.want {
				t.Fatalf("exit hook calls = %v, want [%d]", got, tc.want)
			}
		})
	}
}

// ── 子进程重入：验证真实 os.Exit 路径的退出码与输出 ────────────────────

const seedHelperEnv = "CROUPIER_E2E_FUNCTION_SEED_MAIN"

// TestMainHelperProcess 只在子进程中按环境变量切换 os.Args 后调用 main()
// （main 经 exit 走真实 os.Exit，不再返回）。
func TestMainHelperProcess(t *testing.T) {
	switch os.Getenv(seedHelperEnv) {
	case "missing":
		os.Args = []string{os.Args[0]}
	case "success":
		os.Args = []string{
			os.Args[0],
			"-dsn", os.Getenv(seedHelperEnv + "_DSN"),
			"-function-id", "e2e.echo",
			"-game-id", "e2e-game",
		}
	default:
		return
	}
	main()
}

func TestMainProcessExitCodes(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "subprocess.db")
	cases := []struct {
		name       string
		mode       string
		wantCode   int
		wantStderr string
	}{
		{name: "missing id", mode: "missing", wantCode: 1, wantStderr: "-function-id is required"},
		{name: "seed", mode: "success", wantCode: 0, wantStderr: "status=ok (rows_affected=1)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestMainHelperProcess$")
			cmd.Env = append(os.Environ(), seedHelperEnv+"="+tc.mode, seedHelperEnv+"_DSN="+dsn)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			err := cmd.Run()
			if tc.wantCode == 0 {
				if err != nil {
					t.Fatalf("child err = %v, want success (stderr=%q stdout=%q)", err, stderr.String(), stdout.String())
				}
			} else {
				var exitErr *exec.ExitError
				if !errors.As(err, &exitErr) {
					t.Fatalf("child err = %v, want exit status %d (stderr=%q)", err, tc.wantCode, stderr.String())
				}
				if code := exitErr.ExitCode(); code != tc.wantCode {
					t.Fatalf("exit code = %d, want %d (stderr=%q)", code, tc.wantCode, stderr.String())
				}
			}
			if !strings.Contains(stderr.String(), tc.wantStderr) {
				t.Fatalf("stderr missing %q, got %q", tc.wantStderr, stderr.String())
			}
		})
	}
}
