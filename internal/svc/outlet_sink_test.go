package svc

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/outlet"
	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// TestMessageSinkUpsertFoldsRepushAndUpgradesLevel 验证站内侧重推幂等：
// 同 (recipient, ref_type, ref_id) 不新建行、级别只升不降、不同收件人
// 互不覆盖（incident-reports §6 重推链）。
func TestMessageSinkUpsertFoldsRepushAndUpgradesLevel(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&model.Message{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sink := messageSink{db: db}
	notice := outlet.StationNotice{
		To: "leader-a", Type: "report", Title: "t1", Content: "c1",
		Level: "info", Source: "incident_report", RefType: "report", RefID: "report:week:1:client",
		Scope: map[string]any{"categories": []string{"client"}},
	}
	if err := sink.Create(context.Background(), notice); err != nil {
		t.Fatalf("create: %v", err)
	}
	// 同键重推：不新建行，级别只升不降。
	notice.Level = "warn"
	notice.Title = "t2"
	if err := sink.Create(context.Background(), notice); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	var count int64
	if err := db.Model(&model.Message{}).Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("rows = %d, want 1（重推折叠）", count)
	}
	var msg model.Message
	if err := db.First(&msg).Error; err != nil {
		t.Fatalf("read: %v", err)
	}
	if msg.Level != "warn" || msg.Title != "t2" {
		t.Fatalf("msg = %s/%s, want warn/t2", msg.Level, msg.Title)
	}
	// 降级重推不改级别。
	notice.Level = "info"
	if err := sink.Create(context.Background(), notice); err != nil {
		t.Fatalf("downgrade: %v", err)
	}
	if err := db.First(&msg).Error; err != nil {
		t.Fatalf("reread: %v", err)
	}
	if msg.Level != "warn" {
		t.Fatalf("level downgraded to %s, want warn（只升不降）", msg.Level)
	}
	// 不同收件人同 ref：新行（leader 分片互不覆盖）。
	notice.Level = "info"
	notice.To = "leader-b"
	if err := sink.Create(context.Background(), notice); err != nil {
		t.Fatalf("create b: %v", err)
	}
	if err := db.Model(&model.Message{}).Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 2 {
		t.Fatalf("rows = %d, want 2", count)
	}
}
