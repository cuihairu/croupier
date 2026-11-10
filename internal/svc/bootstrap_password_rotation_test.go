package svc

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// rotationLogCapture 捕获 slog 记录：消息进 buf（断言「打印一次」标记），
// password 属性结构化截存（避免从格式化文本反解析）。
type rotationLogCapture struct {
	mu  sync.Mutex
	buf bytes.Buffer
	pw  string
}

func (c *rotationLogCapture) Enabled(context.Context, slog.Level) bool { return true }
func (c *rotationLogCapture) Handle(_ context.Context, r slog.Record) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	fmt.Fprintln(&c.buf, r.Message)
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "password" {
			c.pw = a.Value.String()
		}
		return true
	})
	return nil
}
func (c *rotationLogCapture) WithAttrs([]slog.Attr) slog.Handler { return c }
func (c *rotationLogCapture) WithGroup(string) slog.Handler      { return c }

func (c *rotationLogCapture) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.String()
}

// newRotationSeedContext 构造最小 seed 环境并安装 slog 捕获
// （验证「生成口令打印一次」用）。返回捕获器；capture 为 nil 表示不捕获。
func newRotationSeedContext(t *testing.T, adminsJSON string, capture *rotationLogCapture) *ServiceContext {
	t.Helper()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "admins.json"), []byte(adminsJSON), 0o644); err != nil {
		t.Fatalf("write admins.json failed: %v", err)
	}

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	pinMemorySQLite(t, db)
	if err := model.AutoMigrate(db); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	if capture != nil {
		prev := slog.Default()
		slog.SetDefault(slog.New(capture))
		t.Cleanup(func() { slog.SetDefault(prev) })
	}

	return &ServiceContext{
		DB:           db,
		AdminManager: NewAdminManager(dir),
		AdminModel:   model.NewAdminModel(db),
		Config:       config.Config{},
	}
}

func TestSeedBootstrapAdminsRotatesFactoryDefaultPassword(t *testing.T) {
	t.Parallel()

	capture := &rotationLogCapture{}
	ctx := newRotationSeedContext(t,
		`[{"username":"admin","password":"admin123","roles":["admin"]}]`, capture)
	if err := ctx.AdminManager.Initialize(); err != nil {
		t.Fatalf("admin manager init failed: %v", err)
	}

	if err := seedBootstrapAdmins(ctx); err != nil {
		t.Fatalf("seedBootstrapAdmins failed: %v", err)
	}

	// 出厂默认口令不得再通过登录校验（同时隐含 admin 行已被播种）
	if _, err := ctx.AdminModel.ValidatePassword(context.Background(), "admin", defaultBootstrapPassword); err == nil {
		t.Fatalf("factory default password must not authenticate after rotation")
	}

	// 生成的随机口令在启动日志打印一次，且能通过登录校验
	logText := capture.String()
	marker := "BOOTSTRAP ADMIN PASSWORD"
	if !strings.Contains(logText, marker) {
		t.Fatalf("expected %q in seed log, got: %s", marker, logText)
	}
	generated := capture.pw
	if generated == "" || generated == defaultBootstrapPassword || len(generated) < 24 {
		t.Fatalf("generated password looks wrong: %q", generated)
	}
	if _, err := ctx.AdminModel.ValidatePassword(context.Background(), "admin", generated); err != nil {
		t.Fatalf("generated password must authenticate: %v", err)
	}
}

func TestSeedBootstrapAdminsKeepsDefaultWhenAllowListed(t *testing.T) {
	t.Parallel()

	ctx := newRotationSeedContext(t,
		`[{"username":"admin","password":"admin123","roles":["admin"]}]`, nil)
	if err := ctx.AdminManager.Initialize(); err != nil {
		t.Fatalf("admin manager init failed: %v", err)
	}
	ctx.Config.Auth.AllowDefaultCredentials = true

	if err := seedBootstrapAdmins(ctx); err != nil {
		t.Fatalf("seedBootstrapAdmins failed: %v", err)
	}
	if _, err := ctx.AdminModel.ValidatePassword(context.Background(), "admin", defaultBootstrapPassword); err != nil {
		t.Fatalf("default password must authenticate when allowDefaultCredentials is set: %v", err)
	}
}

func TestSeedBootstrapAdminsKeepsCustomPassword(t *testing.T) {
	t.Parallel()

	ctx := newRotationSeedContext(t,
		`[{"username":"admin","password":"ops-secret-42","roles":["admin"]}]`, nil)
	if err := ctx.AdminManager.Initialize(); err != nil {
		t.Fatalf("admin manager init failed: %v", err)
	}

	if err := seedBootstrapAdmins(ctx); err != nil {
		t.Fatalf("seedBootstrapAdmins failed: %v", err)
	}
	if _, err := ctx.AdminModel.ValidatePassword(context.Background(), "admin", "ops-secret-42"); err != nil {
		t.Fatalf("custom bootstrap password must pass through unchanged: %v", err)
	}
}

func TestGenerateBootstrapPasswordShape(t *testing.T) {
	t.Parallel()

	seen := map[string]bool{}
	for i := 0; i < 16; i++ {
		p, err := generateBootstrapPassword()
		if err != nil {
			t.Fatalf("generate failed: %v", err)
		}
		if len(p) != 32 {
			t.Fatalf("expected 32-char base64url, got %q", p)
		}
		if seen[p] {
			t.Fatalf("duplicate password generated: %q", p)
		}
		seen[p] = true
	}
}
