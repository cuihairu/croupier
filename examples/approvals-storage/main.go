package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/cuihairu/croupier/internal/platform/approvals"
	"github.com/google/uuid"
)

func main() {
	// 检查命令行参数
	if len(os.Args) < 2 {
		fmt.Printf("用法: %s <存储类型> [DSN]\n", os.Args[0])
		fmt.Println("存储类型:")
		fmt.Println("  mem     - 内存存储（默认）")
		fmt.Println("  sqlite  - SQLite 存储")
		fmt.Println("  pg      - PostgreSQL 存储")
		fmt.Println("\n示例:")
		fmt.Printf("  %s mem\n", os.Args[0])
		fmt.Printf("  %s sqlite data/approvals.db\n", os.Args[0])
		fmt.Printf("  %s pg postgres://user:pass@localhost:5432/db?sslmode=disable\n", os.Args[0])
		os.Exit(1)
	}

	storeType := os.Args[1]
	var dsn string
	if len(os.Args) > 2 {
		dsn = os.Args[2]
	}

	// 创建存储
	var store approvals.Store
	var err error

	switch storeType {
	case "mem":
		store = approvals.NewMemStore()
		fmt.Println("使用内存存储")
	case "sqlite":
		if dsn == "" {
			// 创建数据目录
			_ = os.MkdirAll("data", 0755)
			dsn = filepath.Join("data", "approvals.db")
		}
		store, err = approvals.NewSQLiteStore(dsn)
		if err != nil {
			log.Fatalf("创建 SQLite 存储失败: %v", err)
		}
		fmt.Printf("使用 SQLite 存储: %s\n", dsn)
	case "pg":
		if dsn == "" {
			log.Fatal("PostgreSQL 需要提供 DSN")
		}
		store, err = approvals.NewPGStore(dsn)
		if err != nil {
			log.Fatalf("创建 PostgreSQL 存储失败: %v", err)
		}
		fmt.Printf("使用 PostgreSQL 存储: %s\n", dsn)
	default:
		log.Fatalf("不支持的存储类型: %s", storeType)
	}

	// 演示存储操作
	demonstrateStore(store)
}

func demonstrateStore(store approvals.Store) {
	fmt.Println("\n=== 演示 Approvals 存储操作 ===")

	// 1. 创建一个待审批的请求
	approval := &approvals.Approval{
		ID:             uuid.New().String(),
		State:          "pending",
		FunctionID:     "player.ban",
		GameID:         "game-001",
		Env:            "production",
		Actor:          "admin-001",
		Mode:           "invoke",
		IdempotencyKey: uuid.New().String(),
		Route:          "/api/v1/functions/player.ban",
		Payload:        []byte(`{"playerId": "player123", "reason": "违规操作", "duration": 3600}`),
	}

	fmt.Printf("\n1. 创建审批请求: %s\n", approval.ID)
	created, err := store.Create(approval)
	if err != nil {
		log.Fatalf("创建失败: %v", err)
	}
	fmt.Printf("   状态: %s\n", created.State)
	fmt.Printf("   功能: %s\n", created.FunctionID)

	// 2. 获取审批请求
	fmt.Printf("\n2. 获取审批请求\n")
	retrieved, err := store.Get(approval.ID)
	if err != nil {
		log.Fatalf("获取失败: %v", err)
	}
	fmt.Printf("   创建时间: %s\n", retrieved.CreatedAt.Format("2006-01-02 15:04:05"))

	// 3. 列出所有待审批请求
	fmt.Printf("\n3. 列出所有待审批请求\n")
	pending, total, err := store.List(
		approvals.Filter{State: "pending"},
		approvals.Page{Page: 1, Size: 10},
	)
	if err != nil {
		log.Fatalf("列表查询失败: %v", err)
	}
	fmt.Printf("   总数: %d\n", total)
	fmt.Printf("   当前页: %d\n", len(pending))

	// 4. 创建更多测试数据
	fmt.Printf("\n4. 创建更多测试数据\n")
	testApprovals := []*approvals.Approval{
		{
			ID:         uuid.New().String(),
			State:      "pending",
			FunctionID: "player.unban",
			GameID:     "game-001",
			Env:        "production",
			Actor:      "admin-002",
			Payload:    []byte(`{"playerId": "player456"}`),
		},
		{
			ID:         uuid.New().String(),
			State:      "approved",
			FunctionID: "config.update",
			GameID:     "game-002",
			Env:        "staging",
			Actor:      "admin-001",
			Reason:     "配置更新已批准",
			Payload:    []byte(`{"key": "maxPlayers", "value": 1000}`),
		},
	}

	for _, a := range testApprovals {
		_, err := store.Create(a)
		if err != nil {
			log.Printf("创建测试数据失败: %v", err)
		}
	}

	// 5. 过滤查询
	fmt.Printf("\n5. 过滤查询演示\n")
	// 按游戏过滤
	gameApprovals, _, err := store.List(
		approvals.Filter{GameID: "game-001"},
		approvals.Page{Size: 10},
	)
	if err != nil {
		log.Fatalf("按游戏过滤失败: %v", err)
	}
	fmt.Printf("   game-001 的审批: %d\n", len(gameApprovals))

	// 按状态过滤
	rejected, _, err := store.List(
		approvals.Filter{State: "approved"},
		approvals.Page{Size: 10},
	)
	if err != nil {
		log.Fatalf("按状态过滤失败: %v", err)
	}
	fmt.Printf("   已批准的审批: %d\n", len(rejected))

	// 6. 审批操作
	fmt.Printf("\n6. 执行审批操作\n")
	fmt.Printf("   批准审批: %s\n", approval.ID)
	approved, err := store.Approve(approval.ID, "demo-admin")
	if err != nil {
		log.Fatalf("批准失败: %v", err)
	}
	fmt.Printf("   新状态: %s\n", approved.State)
	fmt.Printf("   更新时间: %s\n", approved.UpdatedAt.Format("2006-01-02 15:04:05"))

	// 拒绝另一个审批
	fmt.Printf("\n   拒绝审批: %s\n", testApprovals[0].ID)
	rejectedApproval, err := store.Reject(testApprovals[0].ID, "缺少必要信息", "demo-admin")
	if err != nil {
		log.Fatalf("拒绝失败: %v", err)
	}
	fmt.Printf("   新状态: %s\n", rejectedApproval.State)
	fmt.Printf("   拒绝原因: %s\n", rejectedApproval.Reason)

	// 7. 统计信息
	fmt.Printf("\n7. 最终统计\n")
	pendingCount, _, _ := store.List(
		approvals.Filter{State: "pending"},
		approvals.Page{Size: 1},
	)
	approvedCount, _, _ := store.List(
		approvals.Filter{State: "approved"},
		approvals.Page{Size: 1},
	)
	rejectedCount, _, _ := store.List(
		approvals.Filter{State: "rejected"},
		approvals.Page{Size: 1},
	)

	_, total, err = store.List(approvals.Filter{}, approvals.Page{Size: 1})
	if err != nil {
		log.Fatalf("获取总数失败: %v", err)
	}

	fmt.Printf("   待审批: %d\n", len(pendingCount))
	fmt.Printf("   已批准: %d\n", len(approvedCount))
	fmt.Printf("   已拒绝: %d\n", len(rejectedCount))
	fmt.Printf("   总计: %d\n", total)

	fmt.Println("\n=== 演示完成 ===")
}
