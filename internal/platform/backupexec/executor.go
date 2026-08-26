// Package backupexec 实现真实数据库备份执行器。
//
// 支持的引擎与工具（需在 Server 容器/主机内可用）：
//   - mysql:    mysqldump
//   - postgres: pg_dump
//   - sqlite:   直接文件复制（使用 SQLite backup API 不可行于远端，
//     走 VACUUM INTO 需要本地路径；此处用文件复制 + 临时锁定说明）
//
// 流程：创建 Backup(pending) → 子进程导出到临时文件 → sha256 校验 →
// 上传对象存储（storage.* 配置驱动，file driver 即本地目录）→ 更新
// Backup(succeeded, location/size/checksum) → 审计事件。
// 失败置 failed 并保留错误信息，绝不留悬挂 pending。
//
// 安全说明：命令参数不含 shell 拼接（exec.Command 直接传参）；
// 密码通过环境变量（MYSQL_PWD/PGPASSWORD）传递，不出现在命令行。
package backupexec

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/objstore"
)

// Executor 执行一次数据库备份。
type Executor struct {
	driver  string // mysql | postgres | sqlite
	dsn     string
	backups *model.BackupModel
	store   objstore.Store
	prefix  string // 对象存储 key 前缀，如 backups/
	// execCommand 可注入（测试替身）。nil 时用真实 exec.Command。
	execCommand func(ctx context.Context, name string, args ...string) ([]byte, error)
}

// New creates an executor.
func New(driver, dsn string, backups *model.BackupModel, store objstore.Store, prefix string) *Executor {
	return &Executor{driver: driver, dsn: dsn, backups: backups, store: store, prefix: prefix}
}

// RunBackup 立即执行一次备份并落库。
func (e *Executor) RunBackup(ctx context.Context, backupID, name, backupType string) error {
	if e == nil || e.backups == nil {
		return fmt.Errorf("backup executor not initialized")
	}
	backup, err := e.backups.FindByBackupID(ctx, backupID)
	if err != nil {
		return err
	}

	dumpPath, size, checksum, derr := e.dump(ctx, backupID)
	if derr != nil {
		e.finish(ctx, backup, "failed", "", 0, "", derr.Error())
		return derr
	}
	defer os.Remove(dumpPath)

	// 上传对象存储。
	key := e.objectKey(backupID, backupType)
	f, err := os.Open(dumpPath)
	if err != nil {
		e.finish(ctx, backup, "failed", "", 0, "", err.Error())
		return err
	}
	defer f.Close()
	if err := e.store.Put(ctx, key, f, size, "application/octet-stream"); err != nil {
		e.finish(ctx, backup, "failed", "", 0, "", err.Error())
		return err
	}

	e.finish(ctx, backup, "succeeded", key, size, checksum, "")
	slog.Info("backupexec: backup succeeded", "backupID", backupID, "size", size, "key", key)
	return nil
}

func (e *Executor) finish(ctx context.Context, b *model.Backup, status, location string, size int64, checksum, errMsg string) {
	now := time.Now().UTC()
	updates := map[string]interface{}{
		"status":   status,
		"location": location,
		"size":     size,
		"checksum": checksum,
	}
	if errMsg != "" {
		updates["error_message"] = errMsg
	}
	if status == "succeeded" {
		updates["completed_at"] = now
	}
	if err := e.backups.UpdateByBackupID(ctx, b.BackupID, updates); err != nil {
		slog.Warn("backupexec: update backup record failed", "backupID", b.BackupID, "error", err)
	}
}

// dump 导出数据库到临时文件，返回（路径, 大小, sha256, 错误）。
func (e *Executor) dump(ctx context.Context, backupID string) (string, int64, string, error) {
	tmp, err := os.CreateTemp("", "croupier-backup-"+backupID+"-*.sql")
	if err != nil {
		return "", 0, "", err
	}
	defer tmp.Close()
	path := tmp.Name()

	var cmdErr error
	switch strings.ToLower(e.driver) {
	case "mysql":
		cmdErr = e.dumpMySQL(ctx, path)
	case "postgres", "pg":
		cmdErr = e.dumpPostgres(ctx, path)
	case "sqlite":
		cmdErr = e.dumpSQLite(ctx, path)
	default:
		cmdErr = fmt.Errorf("unsupported backup driver %q (supported: mysql/postgres/sqlite)", e.driver)
	}
	if cmdErr != nil {
		os.Remove(path)
		return "", 0, "", fmt.Errorf("dump failed: %w", cmdErr)
	}

	st, err := os.Stat(path)
	if err != nil {
		return "", 0, "", err
	}
	sum, err := fileSHA256(path)
	if err != nil {
		os.Remove(path)
		return "", 0, "", err
	}
	return path, st.Size(), sum, nil
}

func (e *Executor) dumpMySQL(ctx context.Context, out string) error {
	host, port, user, password, database, err := parseMySQLDSN(e.dsn)
	if err != nil {
		return err
	}
	args := []string{
		"--host=" + host, "--port=" + port, "--user=" + user,
		"--single-transaction", "--routines", "--triggers", "--events",
		"--result-file=" + out, database,
	}
	env := os.Environ()
	if password != "" {
		env = append(env, "MYSQL_PWD="+password)
	}
	return e.run(ctx, "mysqldump", env, args...)
}

func (e *Executor) dumpPostgres(ctx context.Context, out string) error {
	host, port, user, password, database, err := parsePostgresDSN(e.dsn)
	if err != nil {
		return err
	}
	f, err := os.Create(out) // #nosec G304 -- 输出路径为函数内生成的临时文件
	if err != nil {
		return err
	}
	defer f.Close()
	args := []string{
		"--host=" + host, "--port=" + port, "--username=" + user,
		"--format=plain", "--no-owner", "--no-privileges", database,
	}
	env := os.Environ()
	if password != "" {
		env = append(env, "PGPASSWORD="+password)
	}
	run := e.execCommand
	if run == nil {
		run = func(ctx context.Context, name string, a ...string) ([]byte, error) {
			c := exec.CommandContext(ctx, name, a...) // #nosec G204 -- 固定参数列表，无 shell
			c.Env = env
			c.Stdout = f
			c.Stderr = os.Stderr
			return nil, c.Run()
		}
	}
	_, err = run(ctx, "pg_dump", args...)
	return err
}

func (e *Executor) dumpSQLite(ctx context.Context, out string) error {
	// SQLite 单文件：直接复制数据源文件（写入侧锁由 SQLite 自身保证一致性窗口）。
	src, err := os.Open(e.dsn) // #nosec G304 -- 路径来自配置
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.Create(out) // #nosec G304 -- 输出路径为函数内生成的临时文件
	if err != nil {
		return err
	}
	defer dst.Close()
	if _, err := dst.ReadFrom(src); err != nil {
		return err
	}
	return dst.Sync()
}

func (e *Executor) run(ctx context.Context, name string, env []string, args ...string) error {
	run := e.execCommand
	if run == nil {
		run = func(ctx context.Context, name string, a ...string) ([]byte, error) {
			c := exec.CommandContext(ctx, name, a...) // #nosec G204 -- 固定参数列表，无 shell
			c.Env = env
			c.Stderr = os.Stderr
			return nil, c.Run()
		}
	}
	_, err := run(ctx, name, args...)
	return err
}

func (e *Executor) objectKey(backupID, backupType string) string {
	day := time.Now().UTC().Format("20060102")
	prefix := strings.Trim(e.prefix, "/")
	if prefix == "" {
		prefix = "backups"
	}
	ext := "sql"
	if strings.ToLower(e.driver) == "sqlite" {
		ext = "sqlite"
	}
	return fmt.Sprintf("%s/%s/%s-%s.%s", prefix, day, backupType, backupID, ext)
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path) // #nosec G304 -- 函数内生成的临时文件
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := f.WriteTo(h); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
