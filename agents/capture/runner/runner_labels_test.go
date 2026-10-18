package runner

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/agents/capture/source"
)

// 采集范围与连接器能力自报 labels（发布校验服务端化：server 依赖/能力
// 感知 lint 的数据源；captureGatesRevision 同通道零 proto）。
func TestBuildPayloadCarriesTablesAndCapabilities(t *testing.T) {
	r := New(testConfig(t, true), nil)
	req, err := r.buildPayload(context.Background())
	if err != nil {
		t.Fatalf("buildPayload: %v", err)
	}
	if got := req.Labels["captureTables"]; got != "game.items" {
		t.Fatalf("captureTables = %q, want game.items", got)
	}
	// MySQL 有后像和断点续传，但行事件不带完整事务标识/结束标记，恢复能力关闭。
	if got := req.Labels["captureConnectorCapabilities"]; got != "afterImage,resumableCheckpoint" {
		t.Fatalf("captureConnectorCapabilities = %q", got)
	}
}

func TestBuildPayloadMultiTableSorted(t *testing.T) {
	cfg := testConfig(t, true)
	cfg.Capture.MySQL.Tables = map[string]bool{"game.ledger": true, "game.items": true, "game.wallet": true}
	r := New(cfg, nil)
	req, err := r.buildPayload(context.Background())
	if err != nil {
		t.Fatalf("buildPayload: %v", err)
	}
	if got := req.Labels["captureTables"]; got != "game.items,game.ledger,game.wallet" {
		t.Fatalf("captureTables = %q, want sorted join", got)
	}
}

func TestBuildPayloadOmitsScopeLabelsWhenCaptureDisabled(t *testing.T) {
	r := New(testConfig(t, false), nil)
	req, err := r.buildPayload(context.Background())
	if err != nil {
		t.Fatalf("buildPayload: %v", err)
	}
	if _, ok := req.Labels["captureTables"]; ok {
		t.Fatalf("disabled capture must omit captureTables: %v", req.Labels)
	}
	if _, ok := req.Labels["captureConnectorCapabilities"]; ok {
		t.Fatalf("disabled capture must omit capabilities label: %v", req.Labels)
	}
}

// 能力声明组合自检兜底：声明 snapshot 必须带 resumableCheckpoint——
// MySQL 自报不含 snapshot，这里锁死 MySQLCapabilities 不违组合约束。
func TestMySQLCapabilitiesSelfConsistent(t *testing.T) {
	caps := source.MySQLCapabilities()
	if err := caps.Validate(); err != nil {
		t.Fatalf("MySQLCapabilities invalid: %v", err)
	}
	if !caps.AfterImage || caps.TransactionMetadata || !caps.ResumableCheckpoint {
		t.Fatalf("MySQLCapabilities lost baseline facts: %+v", caps)
	}
	if caps.Snapshot || caps.Pushdown || caps.BeforeImage {
		t.Fatalf("MySQLCapabilities overstates: %+v", caps)
	}
}
