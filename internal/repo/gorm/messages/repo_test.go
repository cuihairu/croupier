package messagesgorm

import (
	"context"
	"testing"
	"time"

	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// setupTestDB 创建测试数据库
func setupTestDB(t *testing.T) *gorm.DB {
	// 使用唯一的 DSN 避免测试间共享数据
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(gsqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	_ = AutoMigrate(db)
	return db
}

// TestNewRepo 测试创建仓库
func TestNewRepo(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepo(db)

	if repo == nil {
		t.Fatal("NewRepo() should return non-nil repo")
	}

	if repo.db != db {
		t.Error("Repo should contain the provided db")
	}
}

// TestNewBroadcastRepo 测试创建广播仓库
func TestNewBroadcastRepo(t *testing.T) {
	db := setupTestDB(t)
	repo := NewBroadcastRepo(db)

	if repo == nil {
		t.Fatal("NewBroadcastRepo() should return non-nil repo")
	}

	if repo.db != db {
		t.Error("BroadcastRepo should contain the provided db")
	}
}

// TestAutoMigrate 测试自动迁移
func TestAutoMigrate(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	err = AutoMigrate(db)
	if err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	// 验证表已创建
	if !db.Migrator().HasTable(&MessageRecord{}) {
		t.Error("AutoMigrate() should create message_records table")
	}
	if !db.Migrator().HasTable(&BroadcastMessageRecord{}) {
		t.Error("AutoMigrate() should create broadcast_message_records table")
	}
	if !db.Migrator().HasTable(&BroadcastRoleRecord{}) {
		t.Error("AutoMigrate() should create broadcast_role_records table")
	}
	if !db.Migrator().HasTable(&BroadcastAckRecord{}) {
		t.Error("AutoMigrate() should create broadcast_ack_records table")
	}
}

// TestRepo_Create 测试创建消息
func TestRepo_Create(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()

	from := uint(1)
	to := uint(2)
	msg := &MessageRecord{
		ToUserID:   to,
		FromUserID: &from,
		Title:      "Test Message",
		Content:    "Test content",
		Type:       "info",
	}

	err := repo.Create(ctx, msg)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if msg.ID == 0 {
		t.Error("Create() should set ID")
	}
}

// TestRepo_List 测试列出消息
func TestRepo_List(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()

	from := uint(1)
	to := uint(2)

	// 创建测试消息
	now := time.Now()
	msgs := []*MessageRecord{
		{ToUserID: to, FromUserID: &from, Title: "Msg1", Content: "Content1", Type: "info", ReadAt: &now},
		{ToUserID: to, FromUserID: &from, Title: "Msg2", Content: "Content2", Type: "info", ReadAt: nil},
		{ToUserID: to, FromUserID: &from, Title: "Msg3", Content: "Content3", Type: "info", ReadAt: nil},
	}
	for _, m := range msgs {
		_ = repo.Create(ctx, m)
	}

	// 测试列出所有消息
	all, total, err := repo.List(ctx, to, false, 10, 0)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if total != 3 {
		t.Errorf("List() total = %d, want 3", total)
	}

	if len(all) != 3 {
		t.Errorf("List() returned %d messages, want 3", len(all))
	}

	// 测试只列出未读
	unread, total, err := repo.List(ctx, to, true, 10, 0)
	if err != nil {
		t.Fatalf("List(unreadOnly=true) error = %v", err)
	}

	if total != 2 {
		t.Errorf("List(unreadOnly=true) total = %d, want 2", total)
	}

	if len(unread) != 2 {
		t.Errorf("List(unreadOnly=true) returned %d messages, want 2", len(unread))
	}
}

// TestRepo_List_Empty 测试列出空消息
func TestRepo_List_Empty(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()

	msgs, total, err := repo.List(ctx, 1, false, 10, 0)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if total != 0 {
		t.Errorf("List() total should be 0, got %d", total)
	}

	if len(msgs) != 0 {
		t.Errorf("List() should return empty slice, got %d messages", len(msgs))
	}
}

// TestRepo_List_LimitOffset 测试分页
func TestRepo_List_LimitOffset(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()

	from := uint(1)
	to := uint(2)

	// 创建5条消息
	for i := 1; i <= 5; i++ {
		msg := &MessageRecord{
			ToUserID:   to,
			FromUserID: &from,
			Title:      "Msg",
			Content:    "Content",
			Type:       "info",
		}
		_ = repo.Create(ctx, msg)
	}

	// 测试 limit
	msgs, total, _ := repo.List(ctx, to, false, 2, 0)
	if total != 5 {
		t.Errorf("List() total = %d, want 5", total)
	}
	if len(msgs) != 2 {
		t.Errorf("List(limit=2) returned %d messages, want 2", len(msgs))
	}

	// 测试 offset
	msgs, _, _ = repo.List(ctx, to, false, 2, 2)
	if len(msgs) != 2 {
		t.Errorf("List(limit=2, offset=2) returned %d messages, want 2", len(msgs))
	}
}

// TestRepo_UnreadCount 测试未读计数
func TestRepo_UnreadCount(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()

	from := uint(1)
	to := uint(2)
	now := time.Now()

	// 创建已读消息
	readMsg := &MessageRecord{ToUserID: to, FromUserID: &from, Title: "Read", Type: "info", ReadAt: &now}
	_ = repo.Create(ctx, readMsg)

	// 创建未读消息
	unreadMsg := &MessageRecord{ToUserID: to, FromUserID: &from, Title: "Unread", Type: "info", ReadAt: nil}
	_ = repo.Create(ctx, unreadMsg)

	count, err := repo.UnreadCount(ctx, to)
	if err != nil {
		t.Fatalf("UnreadCount() error = %v", err)
	}

	if count != 1 {
		t.Errorf("UnreadCount() = %d, want 1", count)
	}
}

// TestRepo_MarkRead 测试标记已读
func TestRepo_MarkRead(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()

	from := uint(1)
	to := uint(2)

	// 创建未读消息
	msg1 := &MessageRecord{ToUserID: to, FromUserID: &from, Title: "Msg1", Type: "info", ReadAt: nil}
	msg2 := &MessageRecord{ToUserID: to, FromUserID: &from, Title: "Msg2", Type: "info", ReadAt: nil}
	_ = repo.Create(ctx, msg1)
	_ = repo.Create(ctx, msg2)

	// 标记为已读
	err := repo.MarkRead(ctx, to, []uint{msg1.ID})
	if err != nil {
		t.Fatalf("MarkRead() error = %v", err)
	}

	// 验证未读数
	count, _ := repo.UnreadCount(ctx, to)
	if count != 1 {
		t.Errorf("UnreadCount() after MarkRead = %d, want 1", count)
	}
}

// TestRepo_MarkRead_EmptyIDs 测试标记空ID列表
func TestRepo_MarkRead_EmptyIDs(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()

	err := repo.MarkRead(ctx, 1, []uint{})
	if err != nil {
		t.Errorf("MarkRead() with empty IDs should not return error, got %v", err)
	}
}

// TestBroadcastRepo_Create 测试创建广播
func TestBroadcastRepo_Create(t *testing.T) {
	db := setupTestDB(t)
	_ = AutoMigrate(db)
	repo := NewBroadcastRepo(db)
	ctx := context.Background()

	// 测试全体广播
	msg1 := &BroadcastMessageRecord{
		Title:    "Broadcast to All",
		Content:  "Content",
		Type:     "info",
		Audience: "all",
	}

	err := repo.Create(ctx, msg1, nil)
	if err != nil {
		t.Fatalf("Create(all) error = %v", err)
	}

	if msg1.ID == 0 {
		t.Error("Create() should set ID")
	}

	// 测试角色广播
	msg2 := &BroadcastMessageRecord{
		Title:    "Broadcast to Roles",
		Content:  "Content",
		Type:     "info",
		Audience: "roles",
	}

	roles := []string{"admin", "moderator"}
	err = repo.Create(ctx, msg2, roles)
	if err != nil {
		t.Fatalf("Create(roles) error = %v", err)
	}

	// 验证角色记录已创建
	var roleCount int64
	db.Model(&BroadcastRoleRecord{}).Where("broadcast_id = ?", msg2.ID).Count(&roleCount)
	if roleCount != 2 {
		t.Errorf("Create() should create 2 role records, got %d", roleCount)
	}
}

// TestBroadcastRepo_List 测试列出广播
func TestBroadcastRepo_List(t *testing.T) {
	db := setupTestDB(t)
	broadcastRepo := NewBroadcastRepo(db)
	ctx := context.Background()

	userID := uint(1)

	// 创建全体广播
	allMsg := &BroadcastMessageRecord{
		Title:    "All Broadcast",
		Content:  "Content",
		Type:     "info",
		Audience: "all",
	}
	_ = broadcastRepo.Create(ctx, allMsg, nil)

	// 创建角色广播
	roleMsg := &BroadcastMessageRecord{
		Title:    "Role Broadcast",
		Content:  "Content",
		Type:     "info",
		Audience: "roles",
	}
	_ = broadcastRepo.Create(ctx, roleMsg, []string{"admin"})

	// 列出广播
	items, total, err := broadcastRepo.List(ctx, userID, []string{"admin"}, false, 10, 0)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if total != 2 {
		t.Errorf("List() total = %d, want 2", total)
	}

	if len(items) != 2 {
		t.Errorf("List() returned %d items, want 2", len(items))
	}

	// 验证所有都是未读
	for _, item := range items {
		if item.Read {
			t.Error("New broadcasts should be unread")
		}
	}
}

// TestBroadcastRepo_List_OnlyUnread 测试只列出未读广播
func TestBroadcastRepo_List_OnlyUnread(t *testing.T) {
	db := setupTestDB(t)
	broadcastRepo := NewBroadcastRepo(db)
	ctx := context.Background()

	userID := uint(1)

	// 创建广播
	msg := &BroadcastMessageRecord{
		Title:    "Test",
		Content:  "Content",
		Type:     "info",
		Audience: "all",
	}
	_ = broadcastRepo.Create(ctx, msg, nil)

	// 标记为已读
	_ = broadcastRepo.MarkRead(ctx, userID, []uint{msg.ID})

	// 列出未读应该为空
	items, total, err := broadcastRepo.List(ctx, userID, []string{}, true, 10, 0)
	if err != nil {
		t.Fatalf("List(unreadOnly=true) error = %v", err)
	}

	if total != 0 {
		t.Errorf("List(unreadOnly=true) total should be 0, got %d", total)
	}

	if len(items) != 0 {
		t.Errorf("List(unreadOnly=true) should return empty, got %d items", len(items))
	}
}

// TestBroadcastRepo_UnreadCount 测试未读广播计数
func TestBroadcastRepo_UnreadCount(t *testing.T) {
	db := setupTestDB(t)
	broadcastRepo := NewBroadcastRepo(db)
	ctx := context.Background()

	userID := uint(1)

	// 创建广播
	msg := &BroadcastMessageRecord{
		Title:    "Test",
		Content:  "Content",
		Type:     "info",
		Audience: "all",
	}
	_ = broadcastRepo.Create(ctx, msg, nil)

	count, err := broadcastRepo.UnreadCount(ctx, userID, []string{})
	if err != nil {
		t.Fatalf("UnreadCount() error = %v", err)
	}

	if count != 1 {
		t.Errorf("UnreadCount() = %d, want 1", count)
	}

	// 标记为已读
	_ = broadcastRepo.MarkRead(ctx, userID, []uint{msg.ID})

	count, _ = broadcastRepo.UnreadCount(ctx, userID, []string{})
	if count != 0 {
		t.Errorf("UnreadCount() after MarkRead = %d, want 0", count)
	}
}

// TestBroadcastRepo_MarkRead 测试标记广播已读
func TestBroadcastRepo_MarkRead(t *testing.T) {
	db := setupTestDB(t)
	broadcastRepo := NewBroadcastRepo(db)
	ctx := context.Background()

	userID := uint(1)

	// 创建广播
	msg := &BroadcastMessageRecord{
		Title:    "Test",
		Content:  "Content",
		Type:     "info",
		Audience: "all",
	}
	_ = broadcastRepo.Create(ctx, msg, nil)

	// 标记为已读
	err := broadcastRepo.MarkRead(ctx, userID, []uint{msg.ID})
	if err != nil {
		t.Fatalf("MarkRead() error = %v", err)
	}

	// 验证已读
	items, _, _ := broadcastRepo.List(ctx, userID, []string{}, false, 10, 0)
	if len(items) != 1 {
		t.Fatalf("List() should return 1 item, got %d", len(items))
	}

	if !items[0].Read {
		t.Error("Broadcast should be marked as read")
	}
}

// TestBroadcastRepo_MarkRead_EmptyIDs 测试标记空ID
func TestBroadcastRepo_MarkRead_EmptyIDs(t *testing.T) {
	db := setupTestDB(t)
	broadcastRepo := NewBroadcastRepo(db)
	ctx := context.Background()

	err := broadcastRepo.MarkRead(ctx, 1, []uint{})
	if err != nil {
		t.Errorf("MarkRead() with empty IDs should not return error, got %v", err)
	}
}

// TestBroadcastRepo_MarkRead_Duplicate 测试重复标记已读
func TestBroadcastRepo_MarkRead_Duplicate(t *testing.T) {
	db := setupTestDB(t)
	broadcastRepo := NewBroadcastRepo(db)
	ctx := context.Background()

	userID := uint(1)

	msg := &BroadcastMessageRecord{
		Title:    "Test",
		Content:  "Content",
		Type:     "info",
		Audience: "all",
	}
	_ = broadcastRepo.Create(ctx, msg, nil)

	// 重复标记
	_ = broadcastRepo.MarkRead(ctx, userID, []uint{msg.ID})
	err := broadcastRepo.MarkRead(ctx, userID, []uint{msg.ID})

	// 应该不报错（使用 OnConflict DoNothing）
	if err != nil {
		t.Errorf("MarkRead() duplicate should not error, got %v", err)
	}
}

// TestRepo_Broadcast 测试获取广播子仓库
func TestRepo_Broadcast(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepo(db)

	broadcastRepo := repo.Broadcast()
	if broadcastRepo == nil {
		t.Fatal("Broadcast() should return non-nil repo")
	}

	if broadcastRepo.db != db {
		t.Error("Broadcast() repo should have same db")
	}
}

// BenchmarkCreate 性能基准测试
func BenchmarkCreate(b *testing.B) {
	db, _ := gorm.Open(gsqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	_ = AutoMigrate(db)
	repo := NewRepo(db)
	ctx := context.Background()

	from := uint(1)
	to := uint(2)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		msg := &MessageRecord{
			ToUserID:   to,
			FromUserID: &from,
			Title:      "Benchmark",
			Content:    "Content",
			Type:       "info",
		}
		_ = repo.Create(ctx, msg)
	}
}

// BenchmarkList 性能基准测试
func BenchmarkList(b *testing.B) {
	db, _ := gorm.Open(gsqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	_ = AutoMigrate(db)
	repo := NewRepo(db)
	ctx := context.Background()

	from := uint(1)
	to := uint(2)

	// 创建100条消息
	for i := 0; i < 100; i++ {
		msg := &MessageRecord{
			ToUserID:   to,
			FromUserID: &from,
			Title:      "Msg",
			Content:    "Content",
			Type:       "info",
		}
		_ = repo.Create(ctx, msg)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = repo.List(ctx, to, false, 10, 0)
	}
}
