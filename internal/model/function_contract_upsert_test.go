package model

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/dbenum"
)

func newContractTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/contract.db"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&FunctionContract{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func sampleContract() *FunctionContract {
	return &FunctionContract{
		GameID: "demo_game", Env: "development", FunctionID: "player.list",
		Version: "1.0.0", Enabled: true, ResourceKey: "player", OperationKey: "list",
		Capability: dbenum.CapabilityCollectionQuery, Execution: "sync",
		Risk: dbenum.RiskSafe, Permission: "player:list",
		InputSchema:  JSON(`{"type":"object","properties":{"page":{"type":"integer"}}}`),
		OutputSchema: JSON(`{"type":"object","properties":{"items":{"type":"array"},"total":{"type":"integer"}}}`),
		Summary:      datatypes.JSONMap{"zh-CN": "玩家列表"},
	}
}

// 同内容重注册（agent 重启场景）：updated_at 不变——跳过写。
func TestUpsertContract_NoChangeSkipsWrite(t *testing.T) {
	db := newContractTestDB(t)
	m := NewFunctionContractModel(db)
	ctx := context.Background()

	if err := m.UpsertContract(ctx, sampleContract()); err != nil {
		t.Fatal(err)
	}
	var first FunctionContract
	if err := db.First(&first).Error; err != nil {
		t.Fatal(err)
	}

	time.Sleep(20 * time.Millisecond)
	if err := m.UpsertContract(ctx, sampleContract()); err != nil {
		t.Fatal(err)
	}
	var second FunctionContract
	if err := db.First(&second).Error; err != nil {
		t.Fatal(err)
	}
	if !second.UpdatedAt.Equal(first.UpdatedAt) {
		t.Fatalf("no-change upsert should skip write: %v -> %v", first.UpdatedAt, second.UpdatedAt)
	}
}

// schema 字节形态变化（键序/空格）但语义相同：跳过写。
func TestUpsertContract_ByteFormChangeSkipsWrite(t *testing.T) {
	db := newContractTestDB(t)
	m := NewFunctionContractModel(db)
	ctx := context.Background()

	if err := m.UpsertContract(ctx, sampleContract()); err != nil {
		t.Fatal(err)
	}
	reformed := sampleContract()
	reformed.InputSchema = JSON(`{ "properties" : { "page" : { "type" : "integer" } } , "type" : "object" }`)
	if err := m.UpsertContract(ctx, reformed); err != nil {
		t.Fatal(err)
	}
	var got FunctionContract
	if err := db.First(&got).Error; err != nil {
		t.Fatal(err)
	}
	if string(got.InputSchema) != `{"type":"object","properties":{"page":{"type":"integer"}}}` {
		t.Fatalf("semantically equal schema should not rewrite: %s", got.InputSchema)
	}
}

// 实质变化（risk 升级）：写入。
func TestUpsertContract_RealChangeWrites(t *testing.T) {
	db := newContractTestDB(t)
	m := NewFunctionContractModel(db)
	ctx := context.Background()

	if err := m.UpsertContract(ctx, sampleContract()); err != nil {
		t.Fatal(err)
	}
	changed := sampleContract()
	changed.Risk = dbenum.RiskHigh
	if err := m.UpsertContract(ctx, changed); err != nil {
		t.Fatal(err)
	}
	var got FunctionContract
	if err := db.First(&got).Error; err != nil {
		t.Fatal(err)
	}
	if got.Risk != dbenum.RiskHigh {
		t.Fatalf("real change must write, risk = %v", got.Risk)
	}
}
