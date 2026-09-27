package main

import (
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/cuihairu/croupier/internal/model"
)

// seedPasswordHash 种子账号统一密码 seed-Admin123 的 bcrypt 哈希
// （包级生成一次，Admin 与 Player 共用）。
var seedPasswordHash, _ = bcrypt.GenerateFromPassword([]byte("seed-Admin123"), bcrypt.DefaultCost)

// seedAll 按依赖顺序铺数据：meta（游戏/账号）→ 支撑（FAQ/工单/缺陷）→
// 运营（函数/页面/调度/配置）。每步幂等。
func seedAll(gdb *gorm.DB, heavy bool) error {
	steps := []struct {
		name string
		fn   func(*gorm.DB) error
	}{
		{"games", seedGames},
		{"accounts", seedAccounts},
		{"announcements", seedAnnouncements},
		{"messages", seedMessages},
		{"faq", seedFAQ},
		{"functions", seedFunctions},
		{"pages", seedPages},
		{"players", seedPlayers},
		{"tickets", seedTickets},
		{"feedback", seedFeedback},
		{"bugs", seedBugs},
		{"schedules", seedSchedules},
		{"configVersions", seedConfigVersions},
		{"edges", seedEdges},
	}
	if heavy {
		steps = append(steps, struct {
			name string
			fn   func(*gorm.DB) error
		}{"heavy", seedHeavy})
	}
	for _, s := range steps {
		if err := s.fn(gdb); err != nil {
			return fmt.Errorf("%s: %w", s.name, err)
		}
	}
	return nil
}

// seedGames 三游戏多环境：demo(dev/test/prod)、rpg(dev/prod)、slg(prod)。
func seedGames(gdb *gorm.DB) error {
	games := []model.Game{
		{GameID: "demo", Name: "演示游戏", AliasName: "demo-alias", Enabled: true, Status: "running", GameType: "casual", Description: "dev-seed 演示游戏", Color: "#4096ff"},
		{GameID: "rpg", Name: "奇幻远征", AliasName: "rpg-alias", Enabled: true, Status: "online", GameType: "rpg", Description: "dev-seed RPG", Color: "#722ed1"},
		{GameID: "slg", Name: "九州烽火", AliasName: "slg-alias", Enabled: false, Status: "maintenance", GameType: "slg", Description: "dev-seed SLG（停服维护中）", Color: "#faad14"},
	}
	for i := range games {
		if err := gdb.Where(model.Game{GameID: games[i].GameID}).
			FirstOrCreate(&games[i]).Error; err != nil {
			return err
		}
	}
	envs := []model.GameEnvBinding{
		{GameID: "demo", Env: "dev", DatabaseName: "game_demo_dev", Color: "green", Description: "开发环境"},
		{GameID: "demo", Env: "test", DatabaseName: "game_demo_test", Color: "blue", Description: "测试环境"},
		{GameID: "demo", Env: "prod", DatabaseName: "game_demo_prod", Color: "red", Description: "生产环境"},
		{GameID: "rpg", Env: "dev", DatabaseName: "game_rpg_dev", Color: "green"},
		{GameID: "rpg", Env: "prod", DatabaseName: "game_rpg_prod", Color: "red"},
		{GameID: "slg", Env: "prod", DatabaseName: "game_slg_prod", Color: "red"},
	}
	for i := range envs {
		if err := gdb.Clauses(clause.OnConflict{DoNothing: true}).Create(&envs[i]).Error; err != nil {
			return err
		}
	}
	return nil
}

// seedAccounts 角色/权限/账号（bcrypt 密码 seed-Admin123）：admin 全权、
// gm 客服向、ops 运营向；admin 关联全部角色制造多对多交叉。
func seedAccounts(gdb *gorm.DB) error {
	hash := seedPasswordHash
	roles := []model.Role{
		{Name: "seed-admin", Description: "dev-seed 管理员", Category: "system"},
		{Name: "seed-gm", Description: "dev-seed 客服", Category: "business"},
		{Name: "seed-ops", Description: "dev-seed 运营", Category: "business"},
	}
	for i := range roles {
		if err := gdb.Where(model.Role{Name: roles[i].Name}).FirstOrCreate(&roles[i]).Error; err != nil {
			return err
		}
	}
	perms := []model.Permission{
		{ID: "functions:read", Name: "函数查看", Resource: "gm", Action: "read", Category: "functions"},
		{ID: "functions:write", Name: "函数管理", Resource: "gm", Action: "write", Category: "functions"},
		{ID: "assignments:read", Name: "分配查看", Resource: "gm", Action: "read", Category: "assignments"},
		{ID: "assignments:write", Name: "分配管理", Resource: "gm", Action: "write", Category: "assignments"},
		{ID: "resources:read", Name: "资源查看", Resource: "gm", Action: "read", Category: "resources"},
	}
	for i := range perms {
		if err := gdb.Where(model.Permission{ID: perms[i].ID}).FirstOrCreate(&perms[i]).Error; err != nil {
			return err
		}
	}
	// role↔permission 交叉：seed-admin 全部、seed-gm 只读、seed-ops 分配读写
	links := []model.RolePermission{
		{RoleID: roles[0].ID, PermissionID: perms[0].ID},
		{RoleID: roles[0].ID, PermissionID: perms[1].ID},
		{RoleID: roles[0].ID, PermissionID: perms[2].ID},
		{RoleID: roles[0].ID, PermissionID: perms[3].ID},
		{RoleID: roles[0].ID, PermissionID: perms[4].ID},
		{RoleID: roles[1].ID, PermissionID: perms[0].ID},
		{RoleID: roles[1].ID, PermissionID: perms[2].ID},
		{RoleID: roles[2].ID, PermissionID: perms[2].ID},
		{RoleID: roles[2].ID, PermissionID: perms[3].ID},
	}
	for i := range links {
		if err := gdb.Clauses(clause.OnConflict{DoNothing: true}).Create(&links[i]).Error; err != nil {
			return err
		}
	}
	admins := []model.Admin{
		{Username: "seed-admin", Nickname: "种子管理员", Email: "seed-admin@example.com", PasswordHash: string(hash), Status: 1},
		{Username: "seed-gm", Nickname: "种子客服", Email: "seed-gm@example.com", PasswordHash: string(hash), Status: 1},
		{Username: "seed-ops", Nickname: "种子运营（禁用态）", Email: "seed-ops@example.com", PasswordHash: string(hash), Status: 0},
	}
	for i := range admins {
		if err := gdb.Where(model.Admin{Username: admins[i].Username}).FirstOrCreate(&admins[i]).Error; err != nil {
			return err
		}
	}
	adminRoles := []model.AdminRole{
		{AdminID: admins[0].ID, RoleID: roles[0].ID},
		{AdminID: admins[0].ID, RoleID: roles[1].ID}, // 多角色交叉
		{AdminID: admins[1].ID, RoleID: roles[1].ID},
		{AdminID: admins[2].ID, RoleID: roles[2].ID},
	}
	for i := range adminRoles {
		if err := gdb.Clauses(clause.OnConflict{DoNothing: true}).Create(&adminRoles[i]).Error; err != nil {
			return err
		}
	}
	return nil
}

// seedAnnouncements 公告（含 popup/角色定向/停用）+ 已读记录（含孤儿）。
func seedAnnouncements(gdb *gorm.DB) error {
	now := time.Now()
	anns := []model.Announcement{
		{Title: "【seed】新版本 1.2.0 上线公告", ContentMd: "## 更新内容\n\n- 新增函数目录\n- 修复工单导出", Audience: "all", Popup: true, Active: true, CreatedBy: "seed-admin"},
		{Title: "【seed】客服岗 SOP 更新", ContentMd: "面向 gm 角色的流程说明。", Audience: "role", Role: "seed-gm", Active: true, CreatedBy: "seed-admin"},
		{Title: "【seed】历史公告（已停用）", ContentMd: "已归档。", Audience: "all", Active: false, CreatedBy: "seed-admin", Model: gorm.Model{CreatedAt: now.AddDate(-1, 0, 0), UpdatedAt: now.AddDate(-1, 0, 0)}},
	}
	for i := range anns {
		if err := gdb.Where(model.Announcement{Title: anns[i].Title}).FirstOrCreate(&anns[i]).Error; err != nil {
			return err
		}
	}
	reads := []model.AnnouncementRead{
		{AnnouncementID: anns[0].ID, Username: "seed-admin", ReadAt: now},
		{AnnouncementID: anns[1].ID, Username: "seed-gm", ReadAt: now},
		{AnnouncementID: 88888, Username: "seed-admin", ReadAt: now}, // 孤儿：指向不存在的公告
	}
	for i := range reads {
		if err := gdb.Clauses(clause.OnConflict{DoNothing: true}).Create(&reads[i]).Error; err != nil {
			return err
		}
	}
	return nil
}

// seedMessages 站内消息：已读/未读、类型区分、含 data 载荷。
func seedMessages(gdb *gorm.DB) error {
	now := time.Now()
	msgs := []model.Message{
		{To: "seed-admin", Type: "system", Title: "【seed】欢迎接入消息中心", Content: "这是 dev-seed 的未读系统消息。", Status: 0},
		{To: "seed-admin", Type: "approval", Title: "【seed】函数下架审批待处理", Content: "player.ban 下架申请。", Status: 0},
		{To: "seed-gm", Type: "system", Title: "【seed】已读消息样例", Content: "已读回执样例。", Status: 1, ReadAt: &now},
	}
	for i := range msgs {
		if err := gdb.Where(model.Message{To: msgs[i].To, Title: msgs[i].Title}).FirstOrCreate(&msgs[i]).Error; err != nil {
			return err
		}
	}
	return nil
}

// seedFAQ FAQ 分类 + 条目（slug 作幂等键），覆盖可见/隐藏、排序、浏览数。
func seedFAQ(gdb *gorm.DB) error {
	cats := []model.FAQCategory{
		{Name: "账号问题", Description: "登录/注册/找回", Sort: 1},
		{Name: "充值问题", Description: "支付/到账/退款", Sort: 2},
		{Name: "玩法咨询", Description: "游戏内玩法", Sort: 3, Visible: false},
	}
	for i := range cats {
		if err := gdb.Where(model.FAQCategory{Name: cats[i].Name}).FirstOrCreate(&cats[i]).Error; err != nil {
			return err
		}
	}
	faqs := []model.FAQ{
		{Slug: "seed-how-to-login", Question: "如何登录游戏？", Answer: "使用官方账号在登录页输入账号密码。", Category: "账号问题", Visible: true, Sort: 1, Views: 128, Tags: model.JSON([]byte(`["登录","新手"]`))},
		{Slug: "seed-refund-flow", Question: "充值未到账怎么办？", Answer: "提供订单号联系客服，24h 内处理。", Category: "充值问题", Visible: true, Sort: 2, Views: 64},
		{Slug: "seed-edge-cjk", Question: "【边界】日本語의질문 mixed 中英韩 FAQ 🎮", Answer: "多语言字符渲染样例。", Category: "玩法咨询", Visible: true, Sort: 3},
		{Slug: "seed-edge-long", Question: "【边界】" + stringMul("超长问题", 40), Answer: "超长答案样例：" + stringMul("答案内容。", 200), Category: "账号问题", Visible: true, Sort: 4},
		{Slug: "seed-edge-hidden", Question: "【边界】隐藏 FAQ 不应出现在前台", Answer: "Visible=false。", Category: "账号问题", Visible: false, Sort: 5},
	}
	for i := range faqs {
		if err := gdb.Where(model.FAQ{Slug: faqs[i].Slug}).FirstOrCreate(&faqs[i]).Error; err != nil {
			return err
		}
	}
	return nil
}

// seedFunctions 函数目录：demo/rpg 两游戏，启用/停用混合，含 OpenAPI 形态。
func seedFunctions(gdb *gorm.DB) error {
	functions := []model.Function{
		{FunctionID: "player.get", Name: "查询玩家", Resource: "player", GameID: "demo", Status: 1, Version: "1.0.0", Runtime: "go", Entry: "PlayerGet", SpecFormat: "legacy", Schema: modelJSONMap(`{"type":"object","properties":{"playerId":{"type":"string"}}}`)},
		{FunctionID: "player.ban", Name: "封禁玩家", Resource: "player", GameID: "demo", Status: 1, Version: "1.0.1", Runtime: "go", Entry: "PlayerBan", SpecFormat: "legacy", Schema: modelJSONMap(`{"type":"object","properties":{"playerId":{"type":"string"},"reason":{"type":"string"}}}`)},
		{FunctionID: "item.grant", Name: "发放道具", Resource: "item", GameID: "demo", Status: 0, Version: "0.9.0", Runtime: "go", Entry: "ItemGrant", SpecFormat: "legacy", Schema: modelJSONMap(`{"type":"object"}`)},
		{FunctionID: "mail.send", Name: "发送邮件", Resource: "mail", GameID: "demo", Status: 1, Version: "2.1.0", Runtime: "go", Entry: "MailSend", SpecFormat: "openapi3.0.3", OpenAPISpec: modelJSONMap(`{"operationId":"mailSend","parameters":[]}`)},
		{FunctionID: "guild.kick", Name: "踢出公会", Resource: "guild", GameID: "demo", Status: 1, Version: "1.0.0", Runtime: "go", Entry: "GuildKick", SpecFormat: "legacy", Schema: modelJSONMap(`{"type":"object"}`)},
		{FunctionID: "rpg.dungeon.reset", Name: "重置副本", Resource: "dungeon", GameID: "rpg", Status: 1, Version: "1.2.0", Runtime: "go", Entry: "DungeonReset", SpecFormat: "legacy", Schema: modelJSONMap(`{"type":"object"}`)},
		{FunctionID: "rpg.rank.query", Name: "排行榜查询", Resource: "rank", GameID: "rpg", Status: 0, Version: "1.1.0", Runtime: "go", Entry: "RankQuery", SpecFormat: "legacy", Schema: modelJSONMap(`{"type":"object"}`)},
		{FunctionID: "维护用·超长名称函数" + stringMul("甲", 20), Name: "【边界】超长函数名", Resource: "edge", GameID: "demo", Status: 1, Version: "0.0.1", Runtime: "go", Entry: "EdgeLong", SpecFormat: "legacy"},
	}
	for i := range functions {
		if err := gdb.Where(model.Function{FunctionID: functions[i].FunctionID}).FirstOrCreate(&functions[i]).Error; err != nil {
			return err
		}
	}
	return nil
}

// seedPages 页面规格：各类型一页 + 一条已发布 + 一条草稿，覆盖菜单排序。
func seedPages(gdb *gorm.DB) error {
	title := func(zh string) string {
		return fmt.Sprintf(`{"zh-CN":%q,"en-US":"Seed %s"}`, zh, zh)
	}
	pages := []model.PageSpec{
		{GameID: "demo", Env: "dev", PageKey: "seed-players", Type: "resource", ResourceKey: "player.get", CategoryKey: "玩家", CategoryOrder: 1, Order: 1, Status: "published", PublishedActive: true, TitleJSON: title("玩家管理"), SpecJSON: `{"version":1,"type":"resource","resourceKey":"player.get","layout":{"type":"table"},"columns":[{"key":"username","title":"用户名"}]}`},
		{GameID: "demo", Env: "dev", PageKey: "seed-ban", Type: "operation", ResourceKey: "player.ban", CategoryKey: "玩家", CategoryOrder: 1, Order: 2, Status: "draft", TitleJSON: title("封禁操作"), SpecJSON: `{"version":1,"type":"operation","resourceKey":"player.ban"}`},
		{GameID: "demo", Env: "dev", PageKey: "seed-report", Type: "report", CategoryKey: "报表", CategoryOrder: 2, Order: 1, Status: "published", PublishedActive: true, TitleJSON: title("日报"), SpecJSON: `{"version":1,"type":"report"}`},
		{GameID: "rpg", Env: "dev", PageKey: "seed-dungeon", Type: "task", ResourceKey: "rpg.dungeon.reset", CategoryKey: "副本", CategoryOrder: 1, Order: 1, Status: "draft", TitleJSON: title("副本任务"), SpecJSON: `{"version":1,"type":"task","resourceKey":"rpg.dungeon.reset"}`},
	}
	for i := range pages {
		if err := gdb.Clauses(clause.OnConflict{DoNothing: true}).Create(&pages[i]).Error; err != nil {
			return err
		}
	}
	return nil
}

// seedPlayers 各游戏玩家：状态/VIP/等级分布 + 边界昵称。
func seedPlayers(gdb *gorm.DB) error {
	type row struct {
		game   string
		name   string
		nick   string
		status int
		level  int
		vip    int
		bal    int64
	}
	rows := []row{
		{"demo", "seed-player-a", "玩家甲", 1, 42, 3, 128000},
		{"demo", "seed-player-b", "游客9527", 1, 7, 0, 500},
		{"demo", "seed-player-banned", "被封禁的玩家", 0, 20, 1, 0},
		{"demo", "seed-player-suspended", "禁言中", 2, 33, 2, 9900},
		{"demo", "seed-player-emoji", "玩家🎮烽火continu北斗", 1, 15, 0, 777},
		{"demo", "seed-player-rtl", "مستخدمتجريبي", 1, 9, 0, 100},
		{"demo", "seed-player-cjk", "日本語Korean중국어", 1, 12, 1, 2500},
		{"rpg", "seed-rpg-hero", "远征者", 1, 60, 5, 999999},
		{"rpg", "seed-rpg-newbie", "新兵", 1, 1, 0, 50},
		{"slg", "seed-slg-lord", "九州领主", 1, 88, 6, 100},
	}
	players := make([]model.Player, 0, len(rows))
	for _, r := range rows {
		players = append(players, model.Player{
			Username: r.name, Nickname: r.nick, GameID: r.game,
			Status: r.status, Level: r.level, VIP: r.vip, Balance: r.bal,
			Password: string(seedPasswordHash), // 与 Admin 同密码：seed-Admin123
		})
	}
	return firstOrCreateAllBy(gdb, &players, "username")
}

// seedTickets 工单：状态×优先级矩阵、分类、标签、玩家关联（含悬空）、
// 截止时间（过去/未来）、来源渠道、同秒时间戳组、跨年组。
func seedTickets(gdb *gorm.DB) error {
	// 哨兵幂等：存在 seed 标记工单则整组跳过
	var n int64
	if err := gdb.Model(&model.Ticket{}).Where("title LIKE ?", "【seed】%").Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	now := time.Now()
	lastYear := now.AddDate(0, 0, -200)
	sameSecond := now.Add(-time.Hour)
	tickets := []model.Ticket{
		{Title: "【seed】无法登录正式服", Content: "输入账号密码后一直转圈。", Category: "账号问题", Priority: "high", Status: 1, Assignee: "seed-gm", PlayerID: "seed-player-a", Contact: "player-a@example.com", GameID: "demo", Env: "prod", Source: "web", DueAt: timePtr(now.Add(24 * time.Hour)), Tags: model.JSON([]byte(`["登录","紧急"]`)), Model: gorm.Model{CreatedAt: lastYear, UpdatedAt: lastYear}},
		{Title: "【seed】充值 648 未到账", Content: "订单号 2026092701。", Category: "充值问题", Priority: "urgent", Status: 0, Assignee: "seed-gm", PlayerID: "seed-player-b", GameID: "demo", Env: "prod", Source: "sdk", DueAt: timePtr(now.Add(-2 * time.Hour)), Tags: model.JSON([]byte(`["支付"]`))},
		{Title: "【seed】副本卡死无法退出", Content: "第三层 boss 房间卡住。", Category: "玩法咨询", Priority: "medium", Status: 2, Assignee: "seed-admin", PlayerID: "seed-rpg-hero", GameID: "rpg", Env: "dev", Source: "web"},
		{Title: "【seed】已关闭的咨询", Content: "感谢处理。", Category: "玩法咨询", Priority: "low", Status: 3, Assignee: "seed-gm", GameID: "demo", Env: "dev", Source: "web", Model: gorm.Model{CreatedAt: now.AddDate(0, -2, 0), UpdatedAt: now.AddDate(0, -1, 0)}},
		// 关联边界：引用不存在的玩家
		{Title: "【seed】悬空玩家引用样例", Content: "PlayerID 不存在。", Category: "账号问题", Priority: "low", Status: 0, PlayerID: "ghost-player-404", GameID: "demo", Env: "dev", Source: "web"},
		// 字段边界
		{Title: "【边界】" + stringMul("超长工单标题", 40), Content: stringMul("超长内容。", 300), Category: "账号问题", Priority: "low", Status: 0, GameID: "demo", Env: "dev", Source: "web"},
		{Title: "   ", Content: "", Category: "账号问题", Priority: "low", Status: 0, GameID: "demo", Env: "dev", Source: "web"},
		{Title: "  前后空白  ", Content: "内容前后有空格   ", Category: "账号问题", Priority: "low", Status: 0, GameID: "demo", Env: "dev", Source: "web"},
		{Title: `<script>alert('ticket-xss')</script>`, Content: "字段含脚本注入尝试，UI 应原样转义展示。", Category: "账号问题", Priority: "low", Status: 0, GameID: "demo", Env: "dev", Source: "web"},
		{Title: "【边界】RTL 样例 مرحلةالاختبار", Content: "混合 RTL 文本。", Category: "账号问题", Priority: "low", Status: 0, GameID: "demo", Env: "dev", Source: "web"},
		// 时间边界：同秒创建的三条（排序稳定性）
		{Title: "【seed】同秒排序-A", Content: "同秒创建 A。", Category: "账号问题", Priority: "low", Status: 0, GameID: "demo", Env: "dev", Source: "web", Model: gorm.Model{CreatedAt: sameSecond, UpdatedAt: sameSecond}},
		{Title: "【seed】同秒排序-B", Content: "同秒创建 B。", Category: "账号问题", Priority: "low", Status: 0, GameID: "demo", Env: "dev", Source: "web", Model: gorm.Model{CreatedAt: sameSecond, UpdatedAt: sameSecond}},
		{Title: "【seed】同秒排序-C", Content: "同秒创建 C。", Category: "账号问题", Priority: "low", Status: 0, GameID: "demo", Env: "dev", Source: "web", Model: gorm.Model{CreatedAt: sameSecond, UpdatedAt: sameSecond}},
		// 时间边界：跨年
		{Title: "【seed】跨年工单-2025", Content: "创建于去年。", Category: "账号问题", Priority: "low", Status: 0, GameID: "demo", Env: "dev", Source: "web", Model: gorm.Model{CreatedAt: time.Date(2025, 12, 31, 23, 59, 0, 0, time.Local), UpdatedAt: time.Now()}},
		{Title: "【seed】跨年工单-2026", Content: "创建于今年。", Category: "账号问题", Priority: "low", Status: 0, GameID: "demo", Env: "dev", Source: "web", Model: gorm.Model{CreatedAt: time.Date(2026, 1, 1, 0, 1, 0, 0, time.Local), UpdatedAt: time.Now()}},
	}
	if err := gdb.Create(&tickets).Error; err != nil {
		return err
	}
	comments := []model.TicketComment{
		{TicketID: tickets[0].ID, Author: "seed-gm", Content: "已收到，正在排查。"},
		{TicketID: tickets[0].ID, Author: "seed-admin", Content: "升级到后端组。"},
		{TicketID: 999999, Author: "seed-gm", Content: "孤儿评论：指向不存在的工单。"},
	}
	for i := range comments {
		if err := gdb.Create(&comments[i]).Error; err != nil {
			return err
		}
	}
	return nil
}

// seedFeedback 反馈：评分/状态/优先级分布。
func seedFeedback(gdb *gorm.DB) error {
	var n int64
	if err := gdb.Model(&model.Feedback{}).Where("content LIKE ?", "【seed】%").Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	feeds := []model.Feedback{
		{PlayerID: "seed-player-a", Contact: "player-a@example.com", Content: "【seed】新版本手感不错！", Category: "体验", Priority: "low", Status: 2, Rating: 5, GameID: "demo", Env: "prod"},
		{PlayerID: "seed-player-b", Content: "【seed】匹配太慢了。", Category: "性能", Priority: "medium", Status: 0, Rating: 2, GameID: "demo", Env: "prod"},
		{PlayerID: "seed-rpg-hero", Content: "【seed】副本奖励想再丰富一点。", Category: "玩法", Priority: "low", Status: 1, Rating: 4, Reply: "已转策划组。", GameID: "rpg", Env: "dev"},
	}
	return gdb.Create(&feeds).Error
}

// seedBugs 缺陷：严重度/状态分布 + bugs↔工单关联（含悬空 SourceTicketID）。
func seedBugs(gdb *gorm.DB) error {
	var n int64
	if err := gdb.Model(&model.Bug{}).Where("title LIKE ?", "【seed】%").Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	var ticketIDs []uint
	if err := gdb.Model(&model.Ticket{}).Where("title LIKE ?", "【seed】%").Order("id").Limit(2).Pluck("id", &ticketIDs).Error; err != nil {
		return err
	}
	var linked uint
	if len(ticketIDs) > 0 {
		linked = ticketIDs[0]
	}
	bugs := []model.Bug{
		{Title: "【seed】iOS 端战斗结算闪退", Content: "iPhone 15 Pro 系统版本 18.1 复现。", Status: "open", Severity: "critical", Priority: "high", Assignee: "seed-admin", GameID: "demo", Env: "prod", Platform: "ios", Device: "iPhone 15 Pro", OS: "iOS 18.1", Steps: "1. 进入战斗\n2. 击杀 boss\n3. 结算界面闪退", Reproducibility: "always", AffectsVersion: "1.2.0", SourceTicketID: linked},
		{Title: "【seed】排行榜数字溢出显示 ###", Content: "分数超过 2^31 显示异常。", Status: "in_progress", Severity: "major", Priority: "medium", Assignee: "seed-admin", GameID: "rpg", Env: "dev", Platform: "pc", Reproducibility: "often", AffectsVersion: "1.1.0"},
		// 关联边界：SourceTicketID 指向不存在的工单
		{Title: "【seed】悬空工单关联样例", Content: "SourceTicketID=424242 不存在。", Status: "open", Severity: "minor", Priority: "low", GameID: "demo", Env: "dev", Platform: "webgl", Reproducibility: "once", SourceTicketID: 424242},
		{Title: "【seed】已修复的历史缺陷", Content: "回归通过。", Status: "resolved", Severity: "major", Priority: "medium", GameID: "demo", Env: "dev", Platform: "android", Reproducibility: "sometimes", AffectsVersion: "1.0.9", Model: gorm.Model{CreatedAt: time.Date(2025, 11, 20, 10, 0, 0, 0, time.Local), UpdatedAt: time.Now()}},
	}
	return gdb.Create(&bugs).Error
}

// seedSchedules 调度任务：active/paused/dead_letter + 时区差异 + 下次触发
// 过去/未来。
func seedSchedules(gdb *gorm.DB) error {
	scheds := []model.TaskSchedule{
		{Name: "【seed】每5分钟发提醒邮件", CronExpr: "*/5 * * * *", GameID: "demo", Env: "prod", FunctionID: "mail.send", Payload: model.JSON([]byte(`{"title":"定时提醒"}`)), Status: "active", Timezone: "Asia/Shanghai", MaxFailedRuns: 5, NextTriggeredAt: timePtr(time.Now().Add(5 * time.Minute)), Actor: "seed-ops"},
		{Name: "【seed】每日结算（已暂停）", CronExpr: "0 3 * * *", GameID: "demo", Env: "dev", FunctionID: "item.grant", Status: "paused", Timezone: "UTC", NextTriggeredAt: timePtr(time.Now().AddDate(0, 0, -1)), Actor: "seed-admin"},
		{Name: "【seed】坏任务（死信）", CronExpr: "0 * * * *", GameID: "rpg", Env: "dev", FunctionID: "rpg.rank.query", Status: "dead_letter", MaxFailedRuns: 5, ConsecutiveFailures: 5, Timezone: "America/New_York", NextTriggeredAt: timePtr(time.Now().Add(-3 * time.Hour)), Actor: "seed-admin"},
	}
	for i := range scheds {
		if err := gdb.Where(model.TaskSchedule{Name: scheds[i].Name}).FirstOrCreate(&scheds[i]).Error; err != nil {
			return err
		}
	}
	return nil
}

// seedConfigVersions 配置版本历史（变更历史维度）：同 key 多版本、跨年。
func seedConfigVersions(gdb *gorm.DB) error {
	var n int64
	if err := gdb.Model(&model.ConfigVersion{}).Where("key LIKE ?", "seed.%").Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	versions := []model.ConfigVersion{
		{Key: "seed.mail_template", Version: 1, Value: `{"title":"旧模板"}`, Format: "json", GameID: "demo", Env: "prod", Namespace: "mail", Message: "初始版本", CreatedBy: "seed-admin", Model: gorm.Model{CreatedAt: time.Date(2025, 12, 15, 9, 0, 0, 0, time.Local), UpdatedAt: time.Now()}},
		{Key: "seed.mail_template", Version: 2, Value: `{"title":"新模板","footer":"官方团队"}`, Format: "json", GameID: "demo", Env: "prod", Namespace: "mail", Message: "增加页脚", CreatedBy: "seed-admin", Model: gorm.Model{CreatedAt: time.Date(2026, 1, 6, 14, 30, 0, 0, time.Local), UpdatedAt: time.Now()}},
		{Key: "seed.mail_template", Version: 3, Value: `{"title":"新模板v3"}`, Format: "json", GameID: "demo", Env: "prod", Namespace: "mail", Message: "文案微调", CreatedBy: "seed-ops", Model: gorm.Model{CreatedAt: time.Date(2026, 2, 1, 8, 0, 0, 0, time.Local), UpdatedAt: time.Now()}},
		{Key: "seed.feature_flag", Version: 1, Value: `{"newShop":true}`, Format: "json", GameID: "rpg", Env: "dev", Namespace: "flag", Message: "商城开关", CreatedBy: "seed-ops", Model: gorm.Model{CreatedAt: time.Now(), UpdatedAt: time.Now()}},
	}
	return gdb.Create(&versions).Error
}
