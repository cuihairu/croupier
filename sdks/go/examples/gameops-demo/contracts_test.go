package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/sdks/go/pkg/croupier"
	"github.com/xeipuuv/gojsonschema"
)

func TestGameOpsContractsAreComplete(t *testing.T) {
	store := newGameOpsStore()
	defs := gameOpsFunctionDefinitions(store)
	if len(defs) != 32 {
		t.Fatalf("definitions = %d, want 32", len(defs))
	}
	seen := map[string]bool{}
	for _, def := range defs {
		desc := def.desc
		if seen[desc.ID] {
			t.Fatalf("duplicate function %s", desc.ID)
		}
		seen[desc.ID] = true
		if err := validateGameOpsDescriptor(desc); err != nil {
			t.Fatalf("%s invalid descriptor: %v", desc.ID, err)
		}
		if desc.Risk != "safe" && desc.Risk != "warning" && desc.Risk != "high" && desc.Risk != "danger" {
			t.Fatalf("%s unsupported risk %q", desc.ID, desc.Risk)
		}
		if desc.Capability == "" || desc.Execution != "sync" {
			t.Fatalf("%s incomplete execution: capability=%q execution=%q", desc.ID, desc.Capability, desc.Execution)
		}
		if !desc.Enabled {
			t.Fatalf("%s must be enabled", desc.ID)
		}
		// Capability column must be multi-select compatible.
		if _, err := gojsonschema.Validate(gojsonschema.NewStringLoader(desc.InputSchema), gojsonschema.NewGoLoader(map[string]any{})); err != nil {
			t.Fatalf("%s input schema rejects empty request: %v", desc.ID, err)
		}
	}
}

func TestGameOpsLifecycleWriteThenRead(t *testing.T) {
	store := newGameOpsStore()

	created := invokeHandler(t, store, "player.create", map[string]any{"nickname": "契约玩家", "level": 42})
	id := created["id"].(string)
	got := invokeHandler(t, store, "player.get", map[string]any{"id": id})
	if got["nickname"] != "契约玩家" || got["level"].(float64) != 42 {
		t.Fatalf("player.get mismatch: %+v", got)
	}

	// 缺发货 → 补发（对账转一致）
	order8Raw, _ := store.orderGet(contextTODO(), mustJSON(t, map[string]any{"orderNo": "2026102308"}))
	var order8 map[string]any
	_ = json.Unmarshal(order8Raw, &order8)
	if order8["gameCredit"] != "missing" {
		t.Fatalf("order-8 gameCredit=%v, want missing", order8["gameCredit"])
	}
	invokeHandler(t, store, "order.recredit", map[string]any{"orderNo": "2026102308", "reason": "缺发货"})
	order8Raw, _ = store.orderGet(contextTODO(), mustJSON(t, map[string]any{"orderNo": "2026102308"}))
	var after map[string]any
	_ = json.Unmarshal(order8Raw, &after)
	if after["reconciliation"] != "match" || after["gameCredit"] != "credited" {
		t.Fatalf("order-8 after recredit: %+v", after)
	}

	// 补发幂等：已发货再补发必须显式拒绝。
	if err := invokeHandlerErr(t, store, "order.recredit", map[string]any{"orderNo": "2026102308", "reason": "重复"}); err == nil || !strings.Contains(err.Error(), "already_credited") {
		t.Fatalf("re-credit should reject idempotently, got %v", err)
	}

	// 封禁 → 解封（封禁表回读）。
	createdBan := invokeHandler(t, store, "ban.create", map[string]any{"playerId": "20001", "durationHours": 24, "reason": "cheating"})
	banID := createdBan["id"].(string)
	banRaw, _ := store.banGet(contextTODO(), mustJSON(t, map[string]any{"id": banID}))
	ban := map[string]any{}
	_ = json.Unmarshal(banRaw, &ban)
	if ban["status"] != "active" {
		t.Fatalf("created ban status=%v", ban["status"])
	}
	invokeHandler(t, store, "ban.unban", map[string]any{"id": banID, "reason": "复核无误"})
	banRaw, _ = store.banGet(contextTODO(), mustJSON(t, map[string]any{"id": banID}))
	ban = map[string]any{}
	_ = json.Unmarshal(banRaw, &ban)
	if ban["status"] != "lifted" {
		t.Fatalf("ban after unban status=%v", ban["status"])
	}

	// 部分失败重试只清失败计数、`sent` 后可回读。
	invokeHandler(t, store, "mail_task.retry_failed", map[string]any{"id": "task-mail-1002"})
	task := store.taskByIDLocked("task-mail-1002")
	if task.FailedCount != 0 || task.SentCount != 9 || task.Status != "sent" {
		t.Fatalf("retry result: %+v", task)
	}
	if err := invokeHandlerErr(t, store, "mail_task.retry_failed", map[string]any{"id": "task-mail-1002"}); err == nil {
		t.Fatal("retry with no failed targets must be rejected")
	}

	// 活动发布新版本 → 详情版本历史出现 v3 记录。
	invokeHandler(t, store, "activity.publish_version", map[string]any{"id": "act-4002", "note": "v4 walkthrough"})
	act := store.activityByIDLocked("act-4002")
	if act.CurrentVersion != 4 || act.Status != "published" {
		t.Fatalf("activity after publish: +%+v", act)
	}
	detailRaw, _ := store.activityGet(contextTODO(), mustJSON(t, map[string]any{"id": "act-4002"}))
	var detail map[string]any
	_ = json.Unmarshal(detailRaw, &detail)
	if !strings.Contains(detail["versionHistory"].(string), "v4 walkthrough") {
		t.Fatalf("version history missing v2: %v", detail["versionHistory"])
	}
}

func TestRegisterGameOpsFunctions_Registration(t *testing.T) {
	client := &fakeGameOpsClient{}
	if err := registerGameOpsFunctions(client, newGameOpsStore()); err != nil {
		t.Fatalf("register: %v", err)
	}
	if len(client.registered) != 32 {
		t.Fatalf("registered = %d, want 32", len(client.registered))
	}
	for _, desc := range client.registered {
		if desc.Tags == nil || desc.Summary == "" || desc.Description == "" {
			t.Fatalf("descriptor %s not enriched", desc.ID)
		}
	}
}

func TestRegisterGameOpsFunctions_ClientRejects(t *testing.T) {
	client := &fakeGameOpsClient{failOn: "player.get"}
	err := registerGameOpsFunctions(client, newGameOpsStore())
	if err == nil || !strings.Contains(err.Error(), "register player.get failed") {
		t.Fatalf("want wrapped registration error, got %v", err)
	}
}

func TestGameOpsDescriptorCamelCaseIDAccepted(t *testing.T) {
	// E1 模板使用 player.batchBan / player.batchUnban（camelCase operation）。
	// The upstream chain must accept them; validateGameOpsDescriptor models the
	// same gate the registry applies (non-empty ID + valid schemas).
	store := newGameOpsStore()
	for _, def := range gameOpsFunctionDefinitions(store) {
		if def.desc.ID == "player.batchBan" || def.desc.ID == "player.batchUnban" {
			if err := validateGameOpsDescriptor(def.desc); err != nil {
				t.Fatalf("%s rejected: %v", def.desc.ID, err)
			}
			continue
		}
	}
}

type fakeGameOpsClient struct {
	registered []croupier.FunctionDescriptor
	failOn     string
}

func (f *fakeGameOpsClient) RegisterFunction(desc croupier.FunctionDescriptor, _ croupier.FunctionHandler) error {
	if desc.ID == f.failOn {
		return errors.New("registration rejected")
	}
	f.registered = append(f.registered, desc)
	return nil
}
func (f *fakeGameOpsClient) Connect(ctx context.Context) error { return nil }
func (f *fakeGameOpsClient) Serve(ctx context.Context) error   { return nil }
func (f *fakeGameOpsClient) Stop() error                       { return nil }
func (f *fakeGameOpsClient) Close() error                      { return nil }
