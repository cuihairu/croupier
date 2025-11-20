// Comprehensive Example: Demonstrates ALL Croupier Go SDK interfaces
//
// This example showcases:
// 1. Client interface - Function registration and lifecycle management
// 2. Invoker interface - Function invocation and job management
// 3. Configuration management with context
// 4. Error handling and graceful shutdown
// 5. Async operations and streaming

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cuihairu/croupier/sdks/go/pkg/croupier"
)

// ==================== Function Handlers ====================

func playerBanHandler(ctx context.Context, payload []byte) ([]byte, error) {
	log.Printf("🔨 执行玩家封禁 - Payload: %s", string(payload))

	// 模拟处理时间
	time.Sleep(100 * time.Millisecond)

	result := map[string]interface{}{
		"status":    "success",
		"action":    "ban",
		"player_id": "player_123",
		"reason":    "违规行为",
		"timestamp": time.Now().Format(time.RFC3339),
	}

	data, _ := json.Marshal(result)
	return data, nil
}

func itemCreateHandler(ctx context.Context, payload []byte) ([]byte, error) {
	log.Printf("📦 创建游戏道具 - Payload: %s", string(payload))

	result := map[string]interface{}{
		"status":    "success",
		"action":    "create",
		"item_id":   fmt.Sprintf("item_%d", time.Now().Unix()),
		"type":      "weapon",
		"timestamp": time.Now().Format(time.RFC3339),
	}

	data, _ := json.Marshal(result)
	return data, nil
}

func playerDataHandler(ctx context.Context, payload []byte) ([]byte, error) {
	log.Printf("👤 处理玩家数据 - Payload: %s", string(payload))

	result := map[string]interface{}{
		"status":    "success",
		"player_id": "player_123",
		"level":     50,
		"exp":       125000,
		"timestamp": time.Now().Format(time.RFC3339),
	}

	data, _ := json.Marshal(result)
	return data, nil
}

func guildManageHandler(ctx context.Context, payload []byte) ([]byte, error) {
	log.Printf("🏰 管理公会 - Payload: %s", string(payload))

	result := map[string]interface{}{
		"status":   "success",
		"guild_id": "guild_456",
		"action":   "manage",
		"members":  25,
	}

	data, _ := json.Marshal(result)
	return data, nil
}

func utilityHandler(ctx context.Context, payload []byte) ([]byte, error) {
	log.Printf("🔧 工具函数 - Payload: %s", string(payload))

	result := map[string]interface{}{
		"status": "success",
		"type":   "utility",
		"data":   "processed",
	}

	data, _ := json.Marshal(result)
	return data, nil
}

// ==================== Demo Functions ====================

func demonstrateClientRegistration(client croupier.Client) error {
	fmt.Println("\n=== 📝 客户端函数注册演示 ===")

	// 1. 注册高风险管理函数
	banDesc := croupier.FunctionDescriptor{
		ID:        "player.ban",
		Version:   "1.0.0",
		Category:  "moderation",
		Risk:      "high",
		Entity:    "player",
		Operation: "update",
		Enabled:   true,
	}

	if err := client.RegisterFunction(banDesc, playerBanHandler); err != nil {
		return fmt.Errorf("注册玩家封禁函数失败: %w", err)
	}
	fmt.Println("✅ 成功注册玩家封禁函数 (高风险)")

	// 2. 注册低风险物品创建函数
	itemDesc := croupier.FunctionDescriptor{
		ID:        "item.create",
		Version:   "1.0.0",
		Category:  "inventory",
		Risk:      "low",
		Entity:    "item",
		Operation: "create",
		Enabled:   true,
	}

	if err := client.RegisterFunction(itemDesc, itemCreateHandler); err != nil {
		return fmt.Errorf("注册道具创建函数失败: %w", err)
	}
	fmt.Println("✅ 成功注册道具创建函数 (低风险)")

	// 3. 注册中等风险数据操作函数
	dataDesc := croupier.FunctionDescriptor{
		ID:        "player.data",
		Version:   "1.0.0",
		Category:  "data",
		Risk:      "medium",
		Entity:    "player",
		Operation: "read",
		Enabled:   true,
	}

	if err := client.RegisterFunction(dataDesc, playerDataHandler); err != nil {
		return fmt.Errorf("注册玩家数据函数失败: %w", err)
	}
	fmt.Println("✅ 成功注册玩家数据函数 (中等风险)")

	// 4. 注册公会管理函数
	guildDesc := croupier.FunctionDescriptor{
		ID:        "guild.manage",
		Version:   "1.0.0",
		Category:  "social",
		Risk:      "medium",
		Entity:    "guild",
		Operation: "update",
		Enabled:   true,
	}

	if err := client.RegisterFunction(guildDesc, guildManageHandler); err != nil {
		return fmt.Errorf("注册公会管理函数失败: %w", err)
	}
	fmt.Println("✅ 成功注册公会管理函数")

	// 5. 注册工具函数
	utilDesc := croupier.FunctionDescriptor{
		ID:        "util.process",
		Version:   "1.0.0",
		Category:  "utility",
		Risk:      "low",
		Entity:    "system",
		Operation: "read",
		Enabled:   true,
	}

	if err := client.RegisterFunction(utilDesc, utilityHandler); err != nil {
		return fmt.Errorf("注册工具函数失败: %w", err)
	}
	fmt.Println("✅ 成功注册工具函数")

	fmt.Printf("📊 总计注册了 5 个函数，覆盖所有风险等级和操作类型\n")
	return nil
}

func demonstrateClientLifecycle(client croupier.Client) error {
	fmt.Println("\n=== 🔄 客户端生命周期演示 ===")

	// 1. 连接到Agent
	ctx := context.Background()
	if err := client.Connect(ctx); err != nil {
		return fmt.Errorf("连接到Agent失败: %w", err)
	}
	fmt.Println("✅ 成功连接到Agent")
	fmt.Printf("📍 本地服务地址: %s\n", client.GetLocalAddress())

	// 2. 异步启动服务
	fmt.Println("🚀 启动客户端服务...")

	serviceCtx, serviceCancel := context.WithCancel(ctx)
	defer serviceCancel()

	go func() {
		if err := client.Serve(serviceCtx); err != nil {
			log.Printf("服务运行错误: %v", err)
		}
	}()

	// 让服务运行一段时间
	fmt.Println("⏳ 服务运行中，等待3秒...")
	time.Sleep(3 * time.Second)

	// 3. 检查服务状态 (Go SDK目前没有IsServing方法，但我们可以演示其他功能)
	fmt.Printf("📍 当前本地地址: %s\n", client.GetLocalAddress())

	// 4. 优雅停止
	fmt.Println("🛑 停止服务...")
	serviceCancel() // 取消服务上下文

	if err := client.Stop(); err != nil {
		return fmt.Errorf("停止客户端失败: %w", err)
	}
	fmt.Println("✅ 服务已停止")

	return nil
}

func demonstrateInvokerInterface(ctx context.Context) error {
	fmt.Println("\n=== 📞 调用器接口演示 ===")

	// 创建调用器配置
	invokerConfig := &croupier.InvokerConfig{
		Address:        "localhost:8080",
		TimeoutSeconds: 30,
		Insecure:       true,
	}

	invoker := croupier.NewInvoker(invokerConfig)
	defer invoker.Close()

	// 1. 连接
	if err := invoker.Connect(ctx); err != nil {
		return fmt.Errorf("调用器连接失败: %w", err)
	}
	fmt.Println("✅ 调用器连接成功")

	// 2. 设置函数schema
	banSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"player_id": map[string]interface{}{"type": "string"},
			"reason":    map[string]interface{}{"type": "string"},
		},
		"required": []string{"player_id", "reason"},
	}

	if err := invoker.SetSchema("player.ban", banSchema); err != nil {
		return fmt.Errorf("设置schema失败: %w", err)
	}
	fmt.Println("✅ 设置player.ban函数的验证schema")

	// 3. 同步调用演示
	fmt.Println("\n--- 🔄 同步调用演示 ---")
	options := croupier.InvokeOptions{
		IdempotencyKey: generateIdempotencyKey(),
		Timeout:        time.Second * 30,
		Headers: map[string]string{
			"X-Game-ID": "comprehensive-example",
			"X-Env":     "development",
		},
	}

	payload := map[string]interface{}{
		"player_id": "player_123",
		"reason":    "违规聊天",
	}
	payloadJSON, _ := json.Marshal(payload)

	result, err := invoker.Invoke(ctx, "player.ban", string(payloadJSON), options)
	if err != nil {
		log.Printf("❌ 同步调用失败: %v", err)
	} else {
		fmt.Printf("📞 同步调用结果: %s\n", result)
	}

	// 4. 启动异步作业
	fmt.Println("\n--- 🚀 异步作业演示 ---")
	jobPayload := map[string]interface{}{
		"type":   "sword",
		"rarity": "epic",
		"level":  50,
	}
	jobPayloadJSON, _ := json.Marshal(jobPayload)

	jobOptions := croupier.InvokeOptions{
		IdempotencyKey: generateIdempotencyKey(),
		Timeout:        time.Minute * 5,
		Headers: map[string]string{
			"X-Job-Type": "item-creation",
		},
	}

	jobID, err := invoker.StartJob(ctx, "item.create", string(jobPayloadJSON), jobOptions)
	if err != nil {
		log.Printf("❌ 启动作业失败: %v", err)
	} else {
		fmt.Printf("🚀 启动异步作业: %s\n", jobID)

		// 5. 流式获取作业事件
		fmt.Println("📡 监听作业事件...")

		eventChan, err := invoker.StreamJob(ctx, jobID)
		if err != nil {
			log.Printf("❌ 流式监听失败: %v", err)
		} else {
			eventCount := 0
			for event := range eventChan {
				eventCount++
				fmt.Printf("📋 作业事件 #%d: 类型=%s, 负载=%s",
					eventCount, event.EventType, event.Payload)

				if event.Error != "" {
					fmt.Printf(", 错误=%s", event.Error)
				}
				if event.Done {
					fmt.Print(" (完成)")
				}
				fmt.Println()

				// 演示取消作业 (在progress事件时)
				if event.EventType == "progress" && !event.Done {
					fmt.Println("⏹️ 演示取消作业...")
					if err := invoker.CancelJob(ctx, jobID); err != nil {
						log.Printf("❌ 取消作业失败: %v", err)
					} else {
						fmt.Println("✅ 作业取消成功")
					}
				}

				// 最多处理5个事件，避免无限循环
				if eventCount >= 5 {
					break
				}
			}
		}
	}

	// 6. 关闭调用器
	if err := invoker.Close(); err != nil {
		return fmt.Errorf("关闭调用器失败: %w", err)
	}
	fmt.Println("✅ 调用器已关闭")

	return nil
}

func demonstrateErrorHandling(client croupier.Client) {
	fmt.Println("\n=== ⚠️ 错误处理演示 ===")

	// 1. 演示重复注册错误
	desc := croupier.FunctionDescriptor{
		ID:        "player.ban", // 已经注册过的函数
		Version:   "1.0.0",
		Category:  "test",
		Risk:      "low",
		Entity:    "test",
		Operation: "read",
		Enabled:   true,
	}

	if err := client.RegisterFunction(desc, playerBanHandler); err != nil {
		fmt.Printf("⚠️ 预期的重复注册错误: %v\n", err)
	}

	// 2. 演示无效描述符错误
	invalidDesc := croupier.FunctionDescriptor{
		ID:        "", // 空ID
		Version:   "1.0.0",
		Category:  "test",
		Risk:      "low",
		Entity:    "test",
		Operation: "read",
		Enabled:   true,
	}

	if err := client.RegisterFunction(invalidDesc, playerBanHandler); err != nil {
		fmt.Printf("⚠️ 预期的无效描述符错误: %v\n", err)
	}

	// 3. 演示连接超时处理
	timeoutCtx, cancel := context.WithTimeout(context.Background(), time.Millisecond*10)
	defer cancel()

	if err := client.Connect(timeoutCtx); err != nil {
		fmt.Printf("⚠️ 预期的连接超时错误: %v\n", err)
	}

	fmt.Println("✅ 错误处理演示完成")
}

func demonstrateConfigurationVariations() error {
	fmt.Println("\n=== ⚙️ 配置管理演示 ===")

	// 1. 默认配置
	defaultConfig := croupier.DefaultClientConfig()
	fmt.Printf("📋 默认配置: Agent=%s, 环境=%s, 超时=%ds\n",
		defaultConfig.AgentAddr, defaultConfig.Env, defaultConfig.TimeoutSeconds)

	// 2. 开发环境配置
	devConfig := &croupier.ClientConfig{
		AgentAddr:      "localhost:19090",
		GameID:         "dev-game",
		Env:            "development",
		ServiceID:      "dev-service",
		ServiceVersion: "0.1.0",
		LocalListen:    ":0",
		TimeoutSeconds: 15,
		Insecure:       true,
	}
	fmt.Printf("📋 开发配置: 游戏=%s, 服务=%s, 不安全连接=%t\n",
		devConfig.GameID, devConfig.ServiceID, devConfig.Insecure)

	// 3. 生产环境配置
	prodConfig := &croupier.ClientConfig{
		AgentAddr:      "agent.prod.example.com:19090",
		GameID:         "prod-game",
		Env:            "production",
		ServiceID:      "game-server-prod",
		ServiceVersion: "2.1.0",
		LocalListen:    "0.0.0.0:19001",
		TimeoutSeconds: 60,
		Insecure:       false,
		CAFile:         "/etc/ssl/certs/ca.pem",
		CertFile:       "/etc/ssl/certs/client.pem",
		KeyFile:        "/etc/ssl/private/client.key",
	}
	fmt.Printf("📋 生产配置: 地址=%s, TLS启用=%t\n",
		prodConfig.AgentAddr, !prodConfig.Insecure)

	// 4. 调用器配置
	invokerConfig := &croupier.InvokerConfig{
		Address:        "server.example.com:8080",
		TimeoutSeconds: 45,
		Insecure:       false,
		CAFile:         "/etc/ssl/certs/ca.pem",
	}
	fmt.Printf("📋 调用器配置: 服务器=%s, 超时=%ds\n",
		invokerConfig.Address, invokerConfig.TimeoutSeconds)

	fmt.Println("✅ 配置管理演示完成")
	return nil
}

// ==================== Utility Functions ====================

func generateIdempotencyKey() string {
	return fmt.Sprintf("key_%d_%d", time.Now().Unix(), time.Now().Nanosecond())
}

func setupGracefulShutdown(client croupier.Client) context.Context {
	ctx, cancel := context.WithCancel(context.Background())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\n🛑 收到停止信号，开始优雅关闭...")
		client.Stop()
		cancel()
	}()

	return ctx
}

// ==================== Main Function ====================

func main() {
	fmt.Println("🎮 Croupier Go SDK 综合功能演示")
	fmt.Println("===============================================")

	// 创建客户端配置
	config := &croupier.ClientConfig{
		AgentAddr:      "localhost:19090",
		GameID:         "comprehensive-example",
		Env:            "development",
		ServiceID:      "demo-service-go",
		ServiceVersion: "1.0.0",
		LocalListen:    ":0", // 自动分配端口
		TimeoutSeconds: 30,
		Insecure:       true,
	}

	fmt.Printf("🔧 配置: 游戏=%s, 环境=%s, 服务=%s\n",
		config.GameID, config.Env, config.ServiceID)

	// 创建客户端
	client := croupier.NewClient(config)
	defer func() {
		if err := client.Close(); err != nil {
			log.Printf("关闭客户端失败: %v", err)
		}
	}()

	// 设置优雅关闭
	ctx := setupGracefulShutdown(client)

	// ==== 演示所有客户端接口 ====

	// 1. 配置管理演示
	if err := demonstrateConfigurationVariations(); err != nil {
		log.Fatalf("配置演示失败: %v", err)
	}

	// 2. 函数注册演示
	if err := demonstrateClientRegistration(client); err != nil {
		log.Fatalf("函数注册演示失败: %v", err)
	}

	// 3. 错误处理演示
	demonstrateErrorHandling(client)

	// 4. 客户端生命周期演示
	if err := demonstrateClientLifecycle(client); err != nil {
		log.Fatalf("生命周期演示失败: %v", err)
	}

	// ==== 演示调用器接口 ====

	// 5. 调用器功能演示
	if err := demonstrateInvokerInterface(ctx); err != nil {
		log.Printf("调用器演示失败 (这是正常的，因为没有真实的服务端): %v", err)
	}

	fmt.Println("\n🎉 所有功能演示完成!")
	fmt.Println("\n📊 演示统计:")
	fmt.Println("   ✅ 客户端接口: 6/6 已演示")
	fmt.Println("   ✅ 调用器接口: 6/6 已演示")
	fmt.Println("   ✅ 配置管理: 4种配置场景")
	fmt.Println("   ✅ 错误处理: 多种错误场景")
	fmt.Println("   ✅ 生命周期: 完整生命周期")
	fmt.Println("   ✅ 上下文支持: 全面的context使用")

	fmt.Println("\n💡 接口覆盖详情:")
	fmt.Println("   📝 RegisterFunction - 注册函数 (5个不同类型)")
	fmt.Println("   🔌 Connect - 连接到Agent")
	fmt.Println("   🚀 Serve - 启动服务")
	fmt.Println("   🛑 Stop - 停止服务")
	fmt.Println("   🔐 Close - 关闭客户端")
	fmt.Println("   📍 GetLocalAddress - 获取本地地址")
	fmt.Println("   📞 Invoke - 同步函数调用")
	fmt.Println("   🚀 StartJob - 启动异步作业")
	fmt.Println("   📡 StreamJob - 流式作业事件")
	fmt.Println("   ⏹️ CancelJob - 取消作业")
	fmt.Println("   📄 SetSchema - 设置验证模式")

	fmt.Println("\n🏗️ Go特性演示:")
	fmt.Println("   🔄 Context-based cancellation")
	fmt.Println("   ⚡ Goroutine并发")
	fmt.Println("   📡 Channel通信")
	fmt.Println("   🛡️ 优雅关闭处理")
	fmt.Println("   🏷️ 强类型JSON处理")
	fmt.Println("   📦 包管理和模块化")
}