package support

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupTestDB 创建测试数据库
func setupTestDB(t *testing.T) *gorm.DB {
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	_ = AutoMigrate(db)
	return db
}

// TestAutoMigrate 测试自动迁移
func TestAutoMigrate(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:autotest?mode=memory"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	err = AutoMigrate(db)
	if err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	// 验证表已创建
	if !db.Migrator().HasTable(&Ticket{}) {
		t.Error("AutoMigrate() should create tickets table")
	}
	if !db.Migrator().HasTable(&TicketComment{}) {
		t.Error("AutoMigrate() should create ticket_comments table")
	}
	if !db.Migrator().HasTable(&FAQ{}) {
		t.Error("AutoMigrate() should create faqs table")
	}
	if !db.Migrator().HasTable(&Feedback{}) {
		t.Error("AutoMigrate() should create feedbacks table")
	}
}

// TestTicket_TableName 测试工单表名
func TestTicket_TableName(t *testing.T) {
	ticket := Ticket{}
	if ticket.TableName() != "ticket_records_migration" {
		t.Errorf("TableName() = %q, want 'ticket_records_migration'", ticket.TableName())
	}
}

// TestTicketComment_TableName 测试工单评论表名
func TestTicketComment_TableName(t *testing.T) {
	comment := TicketComment{}
	if comment.TableName() != "ticket_comment_records_migration" {
		t.Errorf("TableName() = %q, want 'ticket_comment_records_migration'", comment.TableName())
	}
}

// TestFAQ_TableName 测试 FAQ 表名
func TestFAQ_TableName(t *testing.T) {
	faq := FAQ{}
	if faq.TableName() != "faq_records_migration" {
		t.Errorf("TableName() = %q, want 'faq_records_migration'", faq.TableName())
	}
}

// TestFeedback_TableName 测试反馈表名
func TestFeedback_TableName(t *testing.T) {
	feedback := Feedback{}
	if feedback.TableName() != "feedback_records_migration" {
		t.Errorf("TableName() = %q, want 'feedback_records_migration'", feedback.TableName())
	}
}

// TestTicket_CreateAndRetrieve 测试创建和获取工单
func TestTicket_CreateAndRetrieve(t *testing.T) {
	db := setupTestDB(t)

	dueTime := time.Now().Add(24 * time.Hour)
	ticket := Ticket{
		Title:    "Test Ticket",
		Content:  "Test content",
		Category: "bug",
		Priority: "high",
		Status:   "open",
		Assignee: "admin",
		PlayerID: "player123",
		GameID:   "game1",
		Env:      "dev",
		Source:   "web",
		DueAt:    &dueTime,
	}

	err := db.Create(&ticket).Error
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if ticket.ID == 0 {
		t.Error("Create() should set ID")
	}

	// 获取工单
	var retrieved Ticket
	err = db.First(&retrieved, ticket.ID).Error
	if err != nil {
		t.Fatalf("First() error = %v", err)
	}

	if retrieved.Title != "Test Ticket" {
		t.Errorf("Title = %q, want 'Test Ticket'", retrieved.Title)
	}
	if retrieved.Priority != "high" {
		t.Errorf("Priority = %q, want 'high'", retrieved.Priority)
	}
}

// TestTicket_UpdateStatus 测试更新工单状态
func TestTicket_UpdateStatus(t *testing.T) {
	db := setupTestDB(t)

	ticket := Ticket{
		Title:  "Test",
		Status: "open",
	}
	_ = db.Create(&ticket)

	// 更新状态
	err := db.Model(&ticket).Update("status", "resolved").Error
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// 验证
	var retrieved Ticket
	db.First(&retrieved, ticket.ID)
	if retrieved.Status != "resolved" {
		t.Errorf("Status = %q, want 'resolved'", retrieved.Status)
	}
}

// TestTicketComment_Create 测试创建工单评论
func TestTicketComment_Create(t *testing.T) {
	db := setupTestDB(t)

	// 先创建工单
	ticket := Ticket{Title: "Test Ticket"}
	_ = db.Create(&ticket)

	// 创建评论
	comment := TicketComment{
		TicketID: ticket.ID,
		Author:   "admin",
		Content:  "This is a comment",
	}

	err := db.Create(&comment).Error
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if comment.ID == 0 {
		t.Error("Create() should set ID")
	}
}

// TestTicketComment_Association 测试工单与评论的关联
func TestTicketComment_Association(t *testing.T) {
	db := setupTestDB(t)

	// 创建工单和评论
	ticket := Ticket{Title: "Test Ticket"}
	_ = db.Create(&ticket)

	comment1 := TicketComment{TicketID: ticket.ID, Author: "user1", Content: "Comment 1"}
	comment2 := TicketComment{TicketID: ticket.ID, Author: "user2", Content: "Comment 2"}
	_ = db.Create(&comment1)
	_ = db.Create(&comment2)

	// 查询工单的评论
	var comments []TicketComment
	err := db.Where("ticket_id = ?", ticket.ID).Find(&comments).Error
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}

	if len(comments) != 2 {
		t.Errorf("Find() should return 2 comments, got %d", len(comments))
	}
}

// TestFAQ_CreateAndQuery 测试创建和查询 FAQ
func TestFAQ_CreateAndQuery(t *testing.T) {
	db := setupTestDB(t)

	faq := FAQ{
		Question: "What is Croupier?",
		Answer:   "Croupier is a game backend system.",
		Category: "general",
		Tags:     "croupier,overview",
		Visible:  true,
		Sort:     1,
	}

	err := db.Create(&faq).Error
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// 查询可见的 FAQ
	var faqs []FAQ
	err = db.Where("visible = ?", true).Find(&faqs).Error
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}

	if len(faqs) != 1 {
		t.Errorf("Find() should return 1 FAQ, got %d", len(faqs))
	}

	if faqs[0].Question != "What is Croupier?" {
		t.Errorf("Question = %q, want 'What is Croupier?'", faqs[0].Question)
	}
}

// TestFAQ_HideAndShow 测试隐藏和显示 FAQ
func TestFAQ_HideAndShow(t *testing.T) {
	db := setupTestDB(t)

	faq := FAQ{
		Question: "Test",
		Answer:   "Answer",
		Visible:  true,
	}
	_ = db.Create(&faq)

	// 隐藏
	err := db.Model(&faq).Update("visible", false).Error
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	var retrieved FAQ
	db.First(&retrieved, faq.ID)
	if retrieved.Visible {
		t.Error("FAQ should be hidden")
	}

	// 显示
	err = db.Model(&faq).Update("visible", true).Error
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	db.First(&retrieved, faq.ID)
	if !retrieved.Visible {
		t.Error("FAQ should be visible")
	}
}

// TestFeedback_Create 测试创建反馈
func TestFeedback_Create(t *testing.T) {
	db := setupTestDB(t)

	feedback := Feedback{
		PlayerID: "player123",
		Contact:  "user@example.com",
		Content:  "Great game!",
		Category: "praise",
		Priority: "normal",
		Status:   "new",
		GameID:   "game1",
		Env:      "prod",
	}

	err := db.Create(&feedback).Error
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if feedback.ID == 0 {
		t.Error("Create() should set ID")
	}
}

// TestFeedback_UpdateStatus 测试更新反馈状态
func TestFeedback_UpdateStatus(t *testing.T) {
	db := setupTestDB(t)

	feedback := Feedback{
		Content: "Test feedback",
		Status:  "new",
	}
	_ = db.Create(&feedback)

	// 更新状态为已处理
	err := db.Model(&feedback).Update("status", "closed").Error
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	var retrieved Feedback
	db.First(&retrieved, feedback.ID)
	if retrieved.Status != "closed" {
		t.Errorf("Status = %q, want 'closed'", retrieved.Status)
	}
}

// TestFeedback_FilterByCategory 测试按类别筛选反馈
func TestFeedback_FilterByCategory(t *testing.T) {
	db := setupTestDB(t)

	// 创建不同类别的反馈
	feedbacks := []Feedback{
		{Content: "Bug report", Category: "bug", Status: "new"},
		{Content: "Feature request", Category: "feature", Status: "new"},
		{Content: "Another bug", Category: "bug", Status: "new"},
	}
	for _, f := range feedbacks {
		_ = db.Create(&f)
	}

	// 查询 bug 类别
	var bugReports []Feedback
	err := db.Where("category = ?", "bug").Find(&bugReports).Error
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}

	if len(bugReports) != 2 {
		t.Errorf("Find(category='bug') should return 2 feedbacks, got %d", len(bugReports))
	}
}

// TestTicket_StatusFlow 测试工单状态流转
func TestTicket_StatusFlow(t *testing.T) {
	db := setupTestDB(t)

	ticket := Ticket{
		Title:  "Test Ticket",
		Status: "open",
	}
	_ = db.Create(&ticket)

	// open -> in_progress
	err := db.Model(&ticket).Update("status", "in_progress").Error
	if err != nil {
		t.Fatalf("Update() to in_progress error = %v", err)
	}

	// in_progress -> resolved
	err = db.Model(&ticket).Update("status", "resolved").Error
	if err != nil {
		t.Fatalf("Update() to resolved error = %v", err)
	}

	// resolved -> closed
	err = db.Model(&ticket).Update("status", "closed").Error
	if err != nil {
		t.Fatalf("Update() to closed error = %v", err)
	}

	var retrieved Ticket
	db.First(&retrieved, ticket.ID)
	if retrieved.Status != "closed" {
		t.Errorf("Status = %q, want 'closed'", retrieved.Status)
	}
}

// TestTicket_PriorityLevels 测试优先级级别
func TestTicket_PriorityLevels(t *testing.T) {
	db := setupTestDB(t)

	priorities := []string{"low", "normal", "high", "urgent"}
	for _, p := range priorities {
		ticket := Ticket{
			Title:    "Ticket with " + p + " priority",
			Priority: p,
		}
		_ = db.Create(&ticket)
	}

	// 按优先级排序（降序）
	var tickets []Ticket
	err := db.Order("CASE priority " +
		"WHEN 'urgent' THEN 1 " +
		"WHEN 'high' THEN 2 " +
		"WHEN 'normal' THEN 3 " +
		"WHEN 'low' THEN 4 " +
		"END").Find(&tickets).Error
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}

	if len(tickets) != 4 {
		t.Errorf("Find() should return 4 tickets, got %d", len(tickets))
	}

	if tickets[0].Priority != "urgent" {
		t.Errorf("First ticket should have 'urgent' priority, got %q", tickets[0].Priority)
	}
}

// BenchmarkCreateTicket 性能基准测试
func BenchmarkCreateTicket(b *testing.B) {
	dsn := "file:bench?mode=memory&cache=shared"
	db, _ := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	_ = AutoMigrate(db)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ticket := Ticket{
			Title:    "Benchmark Ticket",
			Content:  "Content",
			Priority: "normal",
			Status:   "open",
		}
		_ = db.Create(&ticket)
	}
}

// BenchmarkQueryTicket 性能基准测试
func BenchmarkQueryTicket(b *testing.B) {
	dsn := "file:benchquery?mode=memory&cache=shared"
	db, _ := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	_ = AutoMigrate(db)

	// 创建100条工单
	for i := 0; i < 100; i++ {
		ticket := Ticket{
			Title:    "Ticket",
			Content:  "Content",
			Priority: "normal",
			Status:   "open",
		}
		_ = db.Create(&ticket)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var tickets []Ticket
		_ = db.Where("status = ?", "open").Limit(10).Find(&tickets)
	}
}
