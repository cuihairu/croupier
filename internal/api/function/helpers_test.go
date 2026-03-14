package function

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func TestMetadataHelpers(t *testing.T) {
	t.Parallel()

	meta := map[string]interface{}{
		"name":   "demo",
		"count":  "12",
		"flag":   "true",
		"nodes":  []interface{}{"a", "b"},
		"object": map[string]interface{}{"a": 1},
	}

	if got := getStringFromMetadata(meta, "name"); got != "demo" {
		t.Fatalf("unexpected string metadata: %q", got)
	}
	if got := getIntFromMetadata(meta, "count"); got != 12 {
		t.Fatalf("unexpected int metadata: %d", got)
	}
	if !getBoolFromMetadata(meta, "flag") {
		t.Fatal("expected bool metadata true")
	}
	nodes := getStringSliceFromMetadata(meta, "nodes")
	if len(nodes) != 2 || nodes[0] != "a" {
		t.Fatalf("unexpected string slice metadata: %#v", nodes)
	}
	obj := getInterfaceFromMetadata(meta, "object")
	if obj == nil {
		t.Fatal("expected interface metadata")
	}
}

func TestParseRolesAndActionsFromJSON(t *testing.T) {
	t.Parallel()

	if got := parseRolesFromJSON(datatypes.JSON([]byte(`["admin","viewer"]`))); len(got) != 2 {
		t.Fatalf("unexpected roles array parse: %#v", got)
	}
	if got := parseRolesFromJSON(datatypes.JSON([]byte(`"admin,viewer"`))); len(got) != 2 {
		t.Fatalf("unexpected roles string parse: %#v", got)
	}
	if got := parseActionsFromJSON(datatypes.JSON([]byte(`["read","write"]`))); len(got) != 2 {
		t.Fatalf("unexpected actions array parse: %#v", got)
	}
	if got := parseActionsFromJSON(datatypes.JSON([]byte(`"read,write"`))); len(got) != 2 {
		t.Fatalf("unexpected actions string parse: %#v", got)
	}
}

func TestEnforceInvokePermission(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := model.AutoMigrate(db); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	functionModel := model.NewFunctionModel(db)
	ctx := context.Background()
	fn := &model.Function{FunctionID: "f1", Name: "demo"}
	if err := db.WithContext(ctx).Create(fn).Error; err != nil {
		t.Fatalf("create function failed: %v", err)
	}
	if err := db.WithContext(ctx).Create(&model.FunctionPermission{
		FunctionID: "f1",
		Resource:   "function",
		Actions:    datatypes.JSON([]byte(`["invoke"]`)),
		Roles:      datatypes.JSON([]byte(`["viewer"]`)),
	}).Error; err != nil {
		t.Fatalf("create permission failed: %v", err)
	}

	svcCtx := &svc.ServiceContext{FunctionModel: functionModel}

	if err := enforceInvokePermission(svcCtx, []string{"admin"}, nil, "f1", "", ""); err != nil {
		t.Fatalf("expected admin bypass, got %v", err)
	}
	if err := enforceInvokePermission(svcCtx, []string{"viewer"}, nil, "f1", "", ""); err != nil {
		t.Fatalf("expected role-based invoke allowed, got %v", err)
	}
	fn2 := &model.Function{FunctionID: "f2", Name: "demo2"}
	if err := db.WithContext(ctx).Create(fn2).Error; err != nil {
		t.Fatalf("create function 2 failed: %v", err)
	}

	if err := enforceInvokePermission(svcCtx, []string{"guest"}, []string{"function:invoke"}, "f2", "", ""); err != nil {
		t.Fatalf("expected perm id fallback allowed, got %v", err)
	}
	if err := enforceInvokePermission(svcCtx, []string{"guest"}, nil, "f1", "", ""); err == nil {
		t.Fatal("expected invoke forbidden without role or perm")
	}
}
