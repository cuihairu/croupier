package configsource

import (
	"context"
	"testing"

	gosqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/model"
)

func newCroupierFixture(t *testing.T) *model.ConfigVersionModel {
	t.Helper()
	db, err := gorm.Open(gosqlite.Open(t.TempDir()+"/test.db"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.ConfigVersion{}); err != nil {
		t.Fatal(err)
	}
	return model.NewConfigVersionModel(db)
}

func TestCroupierSource_ListReadWrite(t *testing.T) {
	m := newCroupierFixture(t)
	prev := croupierVersionModel
	croupierVersionModel = m
	t.Cleanup(func() { croupierVersionModel = prev })

	ctx := context.Background()
	if _, err := m.CreateWithMeta(ctx, model.ConfigVersionPayload{
		Key: "item", Content: `{"id":1}`, Format: "json",
		GameID: "demo", Env: "prod", Namespace: "gameplay",
	}, "tester"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.CreateWithMeta(ctx, model.ConfigVersionPayload{
		Key: "switch", Content: "on: true", Format: "yaml",
		GameID: "demo", Env: "prod", Namespace: "runtime",
	}, "tester"); err != nil {
		t.Fatal(err)
	}

	src, err := New(testBinding("croupier", `{}`))
	if err != nil {
		t.Fatal(err)
	}

	root, err := src.List(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(root) != 2 || !root[0].Dir || root[0].Name != "gameplay" || root[1].Name != "runtime" {
		t.Errorf("root = %+v", root)
	}

	sub, err := src.List(ctx, "runtime")
	if err != nil {
		t.Fatal(err)
	}
	if len(sub) != 1 || sub[0].Name != "switch.yaml" {
		t.Errorf("runtime children = %+v", sub)
	}

	val, err := src.Read(ctx, "runtime/switch.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if string(val) != "on: true" {
		t.Errorf("read = %q", val)
	}

	// 应急写 = 新版本（版本单调递增）
	ws := src.(WritableSource)
	if err := ws.Write(ctx, "runtime/switch.yaml", []byte("on: false"), "应急关闭"); err != nil {
		t.Fatal(err)
	}
	val, err = src.Read(ctx, "runtime/switch.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if string(val) != "on: false" {
		t.Errorf("after write = %q", val)
	}
	versions, err := m.ListByScope(ctx, "switch", "demo", "prod")
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 2 || versions[0].Version != 2 {
		t.Errorf("versions = %+v", versions)
	}

	// namespace 过滤
	filtered, err := New(testBinding("croupier", `{"namespaces":["runtime"]}`))
	if err != nil {
		t.Fatal(err)
	}
	root, err = filtered.List(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(root) != 1 || root[0].Name != "runtime" {
		t.Errorf("filtered root = %+v", root)
	}
	if _, err := filtered.Read(ctx, "gameplay/item.json"); err == nil {
		t.Errorf("filtered namespace should be rejected")
	}
}
