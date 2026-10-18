package connector

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestOperationClosedSet(t *testing.T) {
	for _, op := range []Operation{OpInsert, OpUpdate, OpDelete} {
		if !op.Valid() {
			t.Fatalf("op %q should be valid", op)
		}
	}
	for _, op := range []Operation{"", "truncate", "Insert", "ddl"} {
		if op.Valid() {
			t.Fatalf("op %q should be invalid", op)
		}
	}
}

func TestPositionRoundTrip(t *testing.T) {
	cases := []struct {
		p  Position
		s  string
		ok bool
	}{
		{Position{Kind: "gtid", Value: "srv-1:1-42"}, "gtid:srv-1:1-42", true},
		{Position{Kind: "lsn", Value: "12345678"}, "lsn:12345678", true},
		{Position{Kind: "token", Value: "oplog-7"}, "token:oplog-7", true},
		{Position{}, "", false}, // 零位点：String 空（checkpoint 未落），Valid 严格拒
		{Position{Kind: "bogus", Value: "x"}, "bogus:x", false},
	}
	for _, c := range cases {
		if c.p.Valid() != c.ok {
			t.Fatalf("position %+v valid = %v, want %v", c.p, c.p.Valid(), c.ok)
		}
		if got := c.p.String(); got != c.s {
			t.Fatalf("position %+v String = %q, want %q", c.p, got, c.s)
		}
		if c.ok && c.s != "" {
			back, err := ParsePosition(c.s)
			if err != nil {
				t.Fatalf("ParsePosition(%q): %v", c.s, err)
			}
			if back != c.p {
				t.Fatalf("ParsePosition(%q) = %+v, want %+v", c.s, back, c.p)
			}
		}
	}
	for _, bad := range []string{"noseparator", "bogus:1", ":"} {
		if _, err := ParsePosition(bad); err == nil {
			t.Fatalf("ParsePosition(%q) should fail", bad)
		}
	}
}

func TestChangeEventValidate(t *testing.T) {
	base := ChangeEvent{
		SourceID:   "mysql-main",
		SourceType: "mysql",
		Database:   "game",
		Table:      "items",
		Operation:  OpUpdate,
		Before:     map[string]json.RawMessage{"0": json.RawMessage(`"x"`)},
		After:      map[string]json.RawMessage{"0": json.RawMessage(`"y"`)},
		Position:   Position{Kind: "gtid", Value: "srv:1"},
		TsUnixMs:   time.Now().UnixMilli(),
	}
	if err := base.Validate(); err != nil {
		t.Fatalf("base event should validate: %v", err)
	}

	// update 双像全空 = 契约拒绝（不伪造：能力不足应走 insert/delete 单像或报错）
	noImages := base
	noImages.Before = nil
	noImages.After = nil
	if err := noImages.Validate(); err == nil {
		t.Fatal("update with no images should be rejected")
	}

	// update 仅有前像（afterImage 能力不足）合法——能力差异表达
	beforeOnly := base
	beforeOnly.After = nil
	if err := beforeOnly.Validate(); err != nil {
		t.Fatalf("update with before-only image should validate: %v", err)
	}

	// delete 仅有前像合法
	del := base
	del.Operation = OpDelete
	del.After = nil
	if err := del.Validate(); err != nil {
		t.Fatalf("delete with before-only image should validate: %v", err)
	}

	for _, mutate := range []func(*ChangeEvent){
		func(e *ChangeEvent) { e.SourceID = "" },
		func(e *ChangeEvent) { e.SourceType = "" },
		func(e *ChangeEvent) { e.Table = "" },
		func(e *ChangeEvent) { e.Database = "" },
		func(e *ChangeEvent) { e.Operation = "truncate" },
		func(e *ChangeEvent) { e.Position = Position{Kind: "bogus"} },
		func(e *ChangeEvent) { e.TsUnixMs = 0 },
	} {
		ev := base
		mutate(&ev)
		if err := ev.Validate(); err == nil {
			t.Fatalf("event %+v should be rejected", ev)
		}
	}
}

func TestCapabilities(t *testing.T) {
	c := Capabilities{AfterImage: true, Snapshot: true, ResumableCheckpoint: true}
	if !c.Has(CapAfterImage) || c.Has(CapBeforeImage) {
		t.Fatal("Has misjudge")
	}
	if got := c.Enabled(); len(got) != 3 {
		t.Fatalf("Enabled = %v, want 3 caps", got)
	}
	if err := c.Requires(CapAfterImage, CapSnapshot); err != nil {
		t.Fatalf("Requires satisfied set: %v", err)
	}
	err := c.Requires(CapBeforeImage, CapPushdown)
	if err == nil || !strings.Contains(err.Error(), "beforeImage") || !strings.Contains(err.Error(), "pushdown") {
		t.Fatalf("Requires should name missing caps, got %v", err)
	}
	if err := (Capabilities{}).Requires(); err != nil {
		t.Fatalf("empty Requires should pass: %v", err)
	}

	// snapshot 无 resumableCheckpoint = 非法组合
	if err := (Capabilities{Snapshot: true}).Validate(); err == nil {
		t.Fatal("snapshot without resumableCheckpoint should be rejected")
	}
}

func TestParseCapabilitiesStrict(t *testing.T) {
	c, err := ParseCapabilities([]byte(`{"afterImage":true,"beforeImage":false}`))
	if err != nil {
		t.Fatalf("ParseCapabilities: %v", err)
	}
	if !c.AfterImage || c.BeforeImage {
		t.Fatalf("parsed = %+v", c)
	}
	if _, err := ParseCapabilities([]byte(`{"beforeImage":true,"flyingCarpet":true}`)); err == nil {
		t.Fatal("unknown capability key should be rejected")
	}
}

// fakeConnector 编译期契约满足性检查：接口面以最小实现锁定（方法集/签名）。
type fakeConnector struct{ opened bool }

func (f *fakeConnector) Open(context.Context, SourceConfig) error { f.opened = true; return nil }
func (f *fakeConnector) Discover(context.Context, DiscoveryRequest) (*DiscoveryResult, error) {
	return &DiscoveryResult{}, nil
}
func (f *fakeConnector) Snapshot(context.Context, SnapshotRequest) (<-chan ChangeEvent, error) {
	return nil, ErrUnsupportedSnapshot
}
func (f *fakeConnector) Poll(ctx context.Context, req PollRequest) (<-chan ChangeEvent, error) {
	ch := make(chan ChangeEvent, 1)
	ch <- ChangeEvent{SourceID: "fake", SourceType: "mysql", Database: "game", Table: "items",
		Operation: OpInsert, After: map[string]json.RawMessage{"0": json.RawMessage(`1`)},
		Position: Position{Kind: "gtid", Value: "f:1"}, TsUnixMs: time.Now().UnixMilli()}
	return ch, nil
}
func (f *fakeConnector) Checkpoint(context.Context) (Position, error) {
	return Position{Kind: "gtid", Value: "f:1"}, nil
}
func (f *fakeConnector) AckCheckpoint(context.Context, Position) error {
	return nil
}
func (f *fakeConnector) Close(context.Context) error { return nil }

var _ SourceConnector = (*fakeConnector)(nil)

func TestFakeConnectorContractFlow(t *testing.T) {
	f := &fakeConnector{}
	if err := f.Open(context.Background(), SourceConfig{SourceID: "s", SourceType: "mysql", DSNEnv: "X_DSN"}); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if !f.opened {
		t.Fatal("Open should mark opened")
	}
	ch, err := f.Snapshot(context.Background(), SnapshotRequest{Tables: []string{"game.items"}})
	if !errors.Is(err, ErrUnsupportedSnapshot) {
		t.Fatalf("Snapshot should return ErrUnsupportedSnapshot, got %v", err)
	}
	if ch != nil {
		t.Fatal("unsupported snapshot should not return a channel")
	}
	events, err := f.Poll(context.Background(), PollRequest{From: Position{Kind: "gtid", Value: "f:0"}})
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	ev, ok := <-events
	if !ok {
		t.Fatal("Poll should deliver one event")
	}
	if err := ev.Validate(); err != nil {
		t.Fatalf("streamed event invalid: %v", err)
	}
	pos, err := f.Checkpoint(context.Background())
	if err != nil || !pos.Valid() {
		t.Fatalf("Checkpoint = %v, %v", pos, err)
	}
	if err := f.Close(context.Background()); err != nil {
		t.Fatalf("Close: %v", err)
	}
}
