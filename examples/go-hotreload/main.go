package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cuihairu/croupier-sdk-go/pkg/croupier"
)

func main() {
	fmt.Println("🔥 Croupier SDK with Hot Reload Example")

	// 基础SDK配置
	config := &croupier.ClientConfig{
		AgentAddr:      "127.0.0.1:19090",
		LocalListen:    "127.0.0.1:0",
		ServiceID:      "game-server-hotreload",
		ServiceVersion: "1.0.0",
		TimeoutSeconds: 30,
		Insecure:       true,
	}

	// 热重载配置
	hotConfig := croupier.HotReloadConfig{
		Enabled:                 true,
		AutoReconnect:          true,
		ReconnectDelay:         5 * time.Second,
		MaxRetryAttempts:       5,
		HealthCheckInterval:    30 * time.Second,
		GracefulShutdownTimeout: 30 * time.Second,
	}

	// 启用文件监听（用于配置文件变更）
	hotConfig.FileWatching.Enabled = true
	hotConfig.FileWatching.WatchDir = "./configs"
	hotConfig.FileWatching.Patterns = []string{"*.yaml", "*.json"}

	// 启用Air工具支持
	hotConfig.Tools.Air = true

	// 创建支持热重载的客户端
	client, hotReloader := croupier.NewHotReloadClient(config, hotConfig)

	// 注册函数
	registerFunctions(client)

	// 连接到Agent
	ctx := context.Background()
	if err := client.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect to agent: %v", err)
	}

	// 打印热重载状态
	printHotReloadStatus(hotReloader)

	// 启动服务
	go func() {
		if err := client.Serve(ctx); err != nil {
			log.Printf("Service error: %v", err)
		}
	}()

	// 演示热重载功能
	go demonstrateHotReload(hotReloader)

	// 等待关闭信号
	waitForShutdown(client, hotReloader)
}

// registerFunctions 注册游戏函数
func registerFunctions(client croupier.Client) {
	// 玩家管理函数
	playerBanDesc := croupier.FunctionDescriptor{
		ID:      "player.ban",
		Version: "1.0.0",
		// 其他描述符字段...
	}

	err := client.RegisterFunction(playerBanDesc, handlePlayerBan)
	if err != nil {
		log.Printf("Failed to register player.ban: %v", err)
	}

	// 服务器管理函数
	serverStatusDesc := croupier.FunctionDescriptor{
		ID:      "server.status",
		Version: "1.0.0",
	}

	err = client.RegisterFunction(serverStatusDesc, handleServerStatus)
	if err != nil {
		log.Printf("Failed to register server.status: %v", err)
	}

	fmt.Printf("✅ Registered %d functions\n", 2)
}

// handlePlayerBan 玩家封禁处理函数
func handlePlayerBan(ctx context.Context, payload []byte) ([]byte, error) {
	fmt.Printf("🚫 Processing player ban request: %s\n", string(payload))

	// 模拟业务处理
	time.Sleep(100 * time.Millisecond)

	response := fmt.Sprintf(`{"result": "success", "message": "Player banned", "timestamp": "%s"}`, time.Now().Format(time.RFC3339))
	return []byte(response), nil
}

// handleServerStatus 服务器状态处理函数
func handleServerStatus(ctx context.Context, payload []byte) ([]byte, error) {
	fmt.Printf("📊 Processing server status request: %s\n", string(payload))

	// 模拟状态收集
	status := fmt.Sprintf(`{
		"status": "running",
		"uptime": "%v",
		"connections": 42,
		"memory_usage": "256MB",
		"timestamp": "%s"
	}`, time.Since(startTime), time.Now().Format(time.RFC3339))

	return []byte(status), nil
}

// 全局启动时间
var startTime = time.Now()

// printHotReloadStatus 打印热重载状态
func printHotReloadStatus(hotReloader croupier.HotReloadable) {
	fmt.Println("\n🔥 热重载状态:")
	fmt.Println("================")

	status := hotReloader.GetReloadStatus()
	fmt.Printf("连接状态: %s\n", status.ConnectionStatus)
	fmt.Printf("重连次数: %d\n", status.ReconnectCount)
	fmt.Printf("函数重载: %d\n", status.FunctionReloads)
	fmt.Printf("配置重载: %d\n", status.ConfigReloads)
	fmt.Printf("失败次数: %d\n", status.FailedReloads)
	fmt.Printf("最后重连时间: %v\n", status.LastReconnectTime)
	fmt.Println("================\n")
}

// demonstrateHotReload 演示热重载功能
func demonstrateHotReload(hotReloader croupier.HotReloadable) {
	time.Sleep(5 * time.Second)

	fmt.Println("🔄 演示热重载功能...")

	// 1. 测试函数重载
	fmt.Println("\n1. 测试函数重载...")
	newDesc := croupier.FunctionDescriptor{
		ID:      "player.ban",
		Version: "1.1.0", // 新版本
	}

	err := hotReloader.ReloadFunction("player.ban", newDesc, handlePlayerBanV2)
	if err != nil {
		log.Printf("❌ 函数重载失败: %v", err)
	} else {
		fmt.Printf("✅ 函数 player.ban 已更新到 v1.1.0\n")
	}

	// 2. 测试批量重载
	time.Sleep(3 * time.Second)
	fmt.Println("\n2. 测试批量重载...")

	functions := map[string]croupier.FunctionDescriptor{
		"server.status": {
			ID:      "server.status",
			Version: "2.0.0",
		},
	}

	handlers := map[string]croupier.FunctionHandler{
		"server.status": handleServerStatusV2,
	}

	err = hotReloader.ReloadFunctions(functions, handlers)
	if err != nil {
		log.Printf("❌ 批量重载失败: %v", err)
	} else {
		fmt.Printf("✅ 批量重载完成\n")
	}

	// 3. 演示重连机制（需要手动触发）
	time.Sleep(3 * time.Second)
	fmt.Println("\n3. 演示重连机制...")
	fmt.Println("💡 可以停止Agent来测试自动重连功能")

	// 定期打印状态
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			fmt.Println("\n📊 当前热重载状态:")
			printHotReloadStatus(hotReloader)
		}
	}
}

// handlePlayerBanV2 玩家封禁处理函数 V2
func handlePlayerBanV2(ctx context.Context, payload []byte) ([]byte, error) {
	fmt.Printf("🚫 [V2] Processing enhanced player ban request: %s\n", string(payload))

	// V2版本增强功能：增加IP封禁
	time.Sleep(150 * time.Millisecond)

	response := fmt.Sprintf(`{
		"result": "success",
		"message": "Player banned with enhanced features",
		"version": "2.0",
		"features": ["account_ban", "ip_ban", "device_ban"],
		"timestamp": "%s"
	}`, time.Now().Format(time.RFC3339))

	return []byte(response), nil
}

// handleServerStatusV2 服务器状态处理函数 V2
func handleServerStatusV2(ctx context.Context, payload []byte) ([]byte, error) {
	fmt.Printf("📊 [V2] Processing enhanced server status request: %s\n", string(payload))

	// V2版本增强功能：更详细的状态信息
	status := fmt.Sprintf(`{
		"status": "running",
		"version": "2.0",
		"uptime": "%v",
		"connections": {
			"active": 42,
			"peak": 156,
			"total": 2847
		},
		"resources": {
			"memory_usage": "256MB",
			"memory_total": "1GB",
			"cpu_usage": "12%%",
			"disk_usage": "45%%"
		},
		"performance": {
			"avg_response_time": "23ms",
			"requests_per_second": 1250
		},
		"timestamp": "%s"
	}`, time.Since(startTime), time.Now().Format(time.RFC3339))

	return []byte(status), nil
}

// waitForShutdown 等待关闭信号
func waitForShutdown(client croupier.Client, hotReloader croupier.HotReloadable) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	sig := <-sigCh
	fmt.Printf("\n🛑 Received signal: %v\n", sig)

	// 优雅关闭
	fmt.Println("🛑 Starting graceful shutdown...")

	// 使用热重载的优雅关闭（会等待当前操作完成）
	if err := hotReloader.GracefulShutdown(30 * time.Second); err != nil {
		log.Printf("❌ Graceful shutdown failed: %v", err)
		// 强制关闭
		client.Stop()
	}

	fmt.Println("✅ Service shutdown complete")
}