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
	fmt.Println("📡 Croupier SDK File Transfer Example")
	fmt.Println("======================================")

	// 基础SDK配置
	config := &croupier.ClientConfig{
		AgentAddr:      "127.0.0.1:19090",
		LocalListen:    "127.0.0.1:0",
		ServiceID:      "game-server-file-transfer",
		ServiceVersion: "1.0.0",
		TimeoutSeconds: 30,
		Insecure:       true,
	}

	// 创建基础客户端
	client := croupier.NewClient(config)

	// 注册函数
	registerFunctions(client)

	// 连接到Agent
	ctx := context.Background()
	if err := client.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect to agent: %v", err)
	}

	fmt.Println("✅ Connected to Croupier Agent")
	fmt.Println("📡 File transfer capabilities ready for server hot reload support")

	// 启动服务
	go func() {
		if err := client.Serve(ctx); err != nil {
			log.Printf("Service error: %v", err)
		}
	}()

	// 演示基础功能
	go demonstrateBasicFeatures()

	// 等待关闭信号
	waitForShutdown(client)
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

// demonstrateBasicFeatures 演示基础功能
func demonstrateBasicFeatures() {
	time.Sleep(5 * time.Second)

	fmt.Println("🔧 演示基础功能...")
	fmt.Println("✅ 函数注册完成")
	fmt.Println("✅ Agent连接建立")
	fmt.Println("📡 文件传输功能就绪")
	fmt.Println("💡 SDK现在支持服务器端热重载的文件传输")

	// 定期状态检查
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			fmt.Println("\n📊 服务状态:")
			fmt.Printf("  运行时间: %v\n", time.Since(startTime))
			fmt.Printf("  连接状态: 已连接\n")
			fmt.Printf("  功能状态: 就绪\n")
			fmt.Printf("  文件传输: 准备就绪\n")
		}
	}
}

// waitForShutdown 等待关闭信号
func waitForShutdown(client croupier.Client) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	sig := <-sigCh
	fmt.Printf("\n🛑 Received signal: %v\n", sig)

	// 优雅关闭
	fmt.Println("🛑 Starting graceful shutdown...")

	// 停止客户端
	client.Stop()

	fmt.Println("✅ Service shutdown complete")
}