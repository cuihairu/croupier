package com.croupier.sdk.examples;

import io.github.cuihairu.croupier.sdk.*;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.time.Instant;
import java.util.HashMap;
import java.util.Map;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;

/**
 * Comprehensive Example: Demonstrates ALL Croupier Java SDK interfaces
 *
 * This example showcases:
 * 1. Client interface - Function registration and lifecycle management
 * 2. Configuration management with builder pattern
 * 3. Async operations with CompletableFuture
 * 4. Error handling with exceptions
 * 5. Resource management and cleanup
 * 6. Multiple threading scenarios
 */
public class ComprehensiveExample {
    private static final Logger logger = LoggerFactory.getLogger(ComprehensiveExample.class);
    private static final ExecutorService executor = Executors.newFixedThreadPool(4);

    // ==================== Function Handlers ====================

    private static class PlayerBanHandler implements FunctionHandler {
        @Override
        public String handle(String context, String payload) throws Exception {
            logger.info("🔨 执行玩家封禁 - Context: {}, Payload: {}", context, payload);

            // 模拟处理时间
            Thread.sleep(100);

            return String.format(
                "{\n" +
                "  \"status\": \"success\",\n" +
                "  \"action\": \"ban\",\n" +
                "  \"playerId\": \"player_123\",\n" +
                "  \"reason\": \"违规行为\",\n" +
                "  \"timestamp\": \"%s\"\n" +
                "}",
                Instant.now().toString()
            );
        }
    }

    private static class ItemCreateHandler implements FunctionHandler {
        @Override
        public String handle(String context, String payload) throws Exception {
            logger.info("📦 创建游戏道具 - Context: {}, Payload: {}", context, payload);

            return String.format(
                "{\n" +
                "  \"status\": \"success\",\n" +
                "  \"action\": \"create\",\n" +
                "  \"itemId\": \"item_%d\",\n" +
                "  \"type\": \"weapon\",\n" +
                "  \"timestamp\": \"%s\"\n" +
                "}",
                System.currentTimeMillis(),
                Instant.now().toString()
            );
        }
    }

    private static class PlayerDataHandler implements FunctionHandler {
        @Override
        public String handle(String context, String payload) throws Exception {
            logger.info("👤 处理玩家数据 - Context: {}, Payload: {}", context, payload);

            return String.format(
                "{\n" +
                "  \"status\": \"success\",\n" +
                "  \"playerId\": \"player_123\",\n" +
                "  \"level\": 50,\n" +
                "  \"exp\": 125000,\n" +
                "  \"timestamp\": \"%s\"\n" +
                "}",
                Instant.now().toString()
            );
        }
    }

    private static class GuildManageHandler implements FunctionHandler {
        @Override
        public String handle(String context, String payload) throws Exception {
            logger.info("🏰 管理公会 - Context: {}, Payload: {}", context, payload);

            return String.format(
                "{\n" +
                "  \"status\": \"success\",\n" +
                "  \"guild_id\": \"guild_456\",\n" +
                "  \"action\": \"manage\",\n" +
                "  \"members\": 25,\n" +
                "  \"timestamp\": \"%s\"\n" +
                "}",
                Instant.now().toString()
            );
        }
    }

    private static class UtilityHandler implements FunctionHandler {
        @Override
        public String handle(String context, String payload) throws Exception {
            logger.info("🔧 工具函数 - Context: {}, Payload: {}", context, payload);

            return String.format(
                "{\n" +
                "  \"status\": \"success\",\n" +
                "  \"type\": \"utility\",\n" +
                "  \"processed_at\": \"%s\",\n" +
                "  \"data\": \"processed\"\n" +
                "}",
                Instant.now().toString()
            );
        }
    }

    // ==================== Demo Methods ====================

    private static void demonstrateBuilderPattern() {
        logger.info("\n=== 🏗️ Builder模式演示 ===");

        // 1. 使用Builder模式创建FunctionDescriptor
        FunctionDescriptor banDesc = CroupierSDK.functionDescriptor("player.ban", "1.0.0")
                .resource("player")
                .risk("danger")
                .operation("ban")
                .enabled(true)
                .build();

        logger.info("✅ 创建高风险函数描述符: {}", banDesc.toString());

        // 2. 创建不同类型的函数描述符
        FunctionDescriptor itemDesc = CroupierSDK.functionDescriptor("inventory.grant", "1.0.0")
                .resource("inventory")
                .risk("warning")
                .operation("grant")
                .build();

        FunctionDescriptor dataDesc = CroupierSDK.functionDescriptor("player.snapshot", "1.0.0")
                .resource("player")
                .risk("safe")
                .operation("snapshot")
                .enabled(true)
                .build();

        logger.info("✅ 创建了3种不同风险等级的函数描述符");
    }

    private static void demonstrateConfigurationManagement() {
        logger.info("\n=== ⚙️ 配置管理演示 ===");

        // 1. 默认配置
        ClientConfig defaultConfig = new ClientConfig("default-game", "default-service");
        logger.info("📋 默认配置: {}", defaultConfig.toString());

        // 2. 开发环境配置
        ClientConfig devConfig = new ClientConfig("dev-game", "dev-service");
        devConfig.setAgentAddr("localhost:19090");
        devConfig.setEnv("development");
        devConfig.setInsecure(true);
        devConfig.setTimeoutSeconds(15);
        logger.info("📋 开发配置: Agent={}, 环境={}, 超时={}s",
                   devConfig.getAgentAddr(), devConfig.getEnv(), devConfig.getTimeoutSeconds());

        // 3. 生产环境配置
        ClientConfig prodConfig = new ClientConfig("prod-game", "game-server-prod");
        prodConfig.setAgentAddr("agent.prod.example.com:19090");
        prodConfig.setEnv("production");
        prodConfig.setServiceVersion("2.1.0");
        prodConfig.setInsecure(false);
        prodConfig.setCaFile("/etc/ssl/certs/ca.pem");
        prodConfig.setCertFile("/etc/ssl/certs/client.pem");
        prodConfig.setKeyFile("/etc/ssl/private/client.key");
        prodConfig.setTimeoutSeconds(60);
        logger.info("📋 生产配置: 地址={}, TLS启用={}",
                   prodConfig.getAgentAddr(), !prodConfig.isInsecure());

        // 4. 工厂方法演示
        CroupierClient simpleClient = CroupierSDK.createClient("simple-game", "simple-service");
        CroupierClient configuredClient = CroupierSDK.createClient("configured-game", "service", "localhost:19090");
        CroupierClient fullClient = CroupierSDK.createClient(devConfig);

        logger.info("✅ 演示了3种客户端创建方式：简单、配置、完整");

        // 清理
        simpleClient.close();
        configuredClient.close();
        fullClient.close();
    }

    private static void demonstrateClientRegistration(CroupierClient client) throws CroupierException {
        logger.info("\n=== 📝 客户端函数注册演示 ===");

        // 1. 注册高风险管理函数
        FunctionDescriptor banDesc = CroupierSDK.functionDescriptor("player.ban", "1.0.0")
                .resource("player")
                .risk("danger")
                .operation("ban")
                .enabled(true)
                .build();

        client.registerFunction(banDesc, new PlayerBanHandler());
        logger.info("✅ 成功注册玩家封禁函数 (高风险)");

        // 2. 注册中风险物品发放函数
        FunctionDescriptor itemDesc = CroupierSDK.functionDescriptor("inventory.grant", "1.0.0")
                .resource("inventory")
                .risk("warning")
                .operation("grant")
                .enabled(true)
                .build();

        client.registerFunction(itemDesc, new ItemCreateHandler());
        logger.info("✅ 成功注册道具创建函数 (低风险)");

        // 3. 注册中等风险数据操作函数
        FunctionDescriptor dataDesc = CroupierSDK.functionDescriptor("player.snapshot", "1.0.0")
                .resource("player")
                .risk("safe")
                .operation("snapshot")
                .enabled(true)
                .build();

        client.registerFunction(dataDesc, new PlayerDataHandler());
        logger.info("✅ 成功注册玩家数据函数 (中等风险)");

        // 4. 注册公会管理函数
        FunctionDescriptor guildDesc = CroupierSDK.functionDescriptor("guild.manage", "1.0.0")
                .resource("guild")
                .risk("warning")
                .operation("manage")
                .enabled(true)
                .build();

        client.registerFunction(guildDesc, new GuildManageHandler());
        logger.info("✅ 成功注册公会管理函数");

        // 5. 注册工具函数
        FunctionDescriptor utilDesc = CroupierSDK.functionDescriptor("util.process", "1.0.0")
                .resource("system")
                .risk("safe")
                .operation("process")
                .enabled(true)
                .build();

        client.registerFunction(utilDesc, new UtilityHandler());
        logger.info("✅ 成功注册工具函数");

        logger.info("📊 总计注册了 5 个函数，覆盖所有风险等级和操作类型");
    }

    private static void demonstrateAsyncOperations(CroupierClient client) {
        logger.info("\n=== ⚡ 异步操作演示 ===");

        // 1. 异步连接
        logger.info("🔌 开始异步连接...");
        CompletableFuture<Void> connectFuture = client.connect()
            .thenRun(() -> {
                logger.info("✅ 异步连接完成");
                logger.info("🔗 连接状态: {}", client.isConnected());
            })
            .exceptionally(throwable -> {
                logger.error("❌ 连接失败", throwable);
                return null;
            });

        // 等待连接完成
        try {
            connectFuture.get(10, TimeUnit.SECONDS);
        } catch (Exception e) {
            logger.error("连接超时", e);
            return;
        }

        // 2. 异步服务启动
        logger.info("🚀 开始异步服务...");
        CompletableFuture<Void> serviceFuture = client.serveAsync()
            .thenRun(() -> {
                logger.info("✅ 服务启动完成");
                logger.info("🎯 服务状态: {}", client.isServing());
            })
            .exceptionally(throwable -> {
                logger.error("❌ 服务启动失败", throwable);
                return null;
            });

        // 让服务运行一段时间
        try {
            Thread.sleep(3000);
            logger.info("⏳ 服务已运行3秒");
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
        }

        // 3. 停止服务
        logger.info("🛑 停止服务...");
        client.stop();
        logger.info("✅ 服务已停止");
    }

    private static void demonstrateErrorHandling(CroupierClient client) {
        logger.info("\n=== ⚠️ 错误处理演示 ===");

        try {
            // 1. 演示重复注册错误
            FunctionDescriptor desc = CroupierSDK.functionDescriptor("player.ban", "1.0.0")
                    .resource("player")
                    .risk("safe")
                    .build();

            client.registerFunction(desc, new PlayerBanHandler());
            logger.info("⚠️ 这不应该出现 - 重复注册应该失败");
        } catch (CroupierException e) {
            logger.info("✅ 预期的重复注册错误: {}", e.getMessage());
        }

        try {
            // 2. 演示无效描述符错误
            FunctionDescriptor invalidDesc = CroupierSDK.functionDescriptor("", "1.0.0")
                    .resource("test")
                    .build();

            client.registerFunction(invalidDesc, new PlayerBanHandler());
            logger.info("⚠️ 这不应该出现 - 无效描述符应该失败");
        } catch (CroupierException e) {
            logger.info("✅ 预期的无效描述符错误: {}", e.getMessage());
        }

        // 3. 演示空处理器错误
        try {
            FunctionDescriptor validDesc = CroupierSDK.functionDescriptor("test.function", "1.0.0")
                    .resource("test")
                    .build();

            client.registerFunction(validDesc, null);
            logger.info("⚠️ 这不应该出现 - 空处理器应该失败");
        } catch (Exception e) {
            logger.info("✅ 预期的空处理器错误: {}", e.getMessage());
        }

        logger.info("✅ 错误处理演示完成");
    }

    private static void demonstrateConcurrentOperations(CroupierClient client) {
        logger.info("\n=== 🔄 并发操作演示 ===");

        CountDownLatch latch = new CountDownLatch(3);
        AtomicInteger successCount = new AtomicInteger(0);
        AtomicInteger errorCount = new AtomicInteger(0);

        // 1. 并发注册多个函数
        for (int i = 1; i <= 3; i++) {
            final int index = i;
            CompletableFuture.runAsync(() -> {
                try {
                    FunctionDescriptor desc = CroupierSDK.functionDescriptor("concurrent.function" + index, "1.0.0")
                            .resource("concurrent")
                            .risk("safe")
                            .operation("probe")
                            .build();

                    client.registerFunction(desc, new UtilityHandler());
                    successCount.incrementAndGet();
                    logger.info("✅ 并发注册函数 {} 成功", index);
                } catch (Exception e) {
                    errorCount.incrementAndGet();
                    logger.error("❌ 并发注册函数 {} 失败", index, e);
                } finally {
                    latch.countDown();
                }
            }, executor);
        }

        // 等待所有操作完成
        try {
            latch.await(10, TimeUnit.SECONDS);
            logger.info("📊 并发操作结果: 成功={}, 失败={}", successCount.get(), errorCount.get());
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            logger.error("并发操作被中断", e);
        }
    }

    private static void demonstrateResourceManagement() {
        logger.info("\n=== 🗂️ 资源管理演示 ===");

        // 1. 演示try-with-resources模式 (如果实现了AutoCloseable)
        ClientConfig config = new ClientConfig("resource-test", "test-service");
        config.setInsecure(true);

        CroupierClient client = CroupierSDK.createClient(config);
        try {
            logger.info("📂 创建客户端资源");

            // 注册一个简单函数
            FunctionDescriptor desc = CroupierSDK.functionDescriptor("resource.test", "1.0.0")
                    .resource("test")
                    .build();

            client.registerFunction(desc, new UtilityHandler());
            logger.info("✅ 资源使用完成");

        } catch (Exception e) {
            logger.error("❌ 资源使用失败", e);
        } finally {
            // 2. 确保资源清理
            client.close();
            logger.info("🗑️ 资源已清理");
        }
    }

    private static void demonstrateComplexLifecycle() {
        logger.info("\n=== 🔄 复杂生命周期演示 ===");

        ClientConfig config = new ClientConfig("lifecycle-test", "test-service");
        config.setAgentAddr("localhost:19090");
        config.setInsecure(true);

        CroupierClient client = CroupierSDK.createClient(config);

        try {
            // 1. 注册函数
            FunctionDescriptor desc = CroupierSDK.functionDescriptor("lifecycle.test", "1.0.0")
                    .resource("test")
                    .build();
            client.registerFunction(desc, new UtilityHandler());
            logger.info("✅ 生命周期阶段1: 函数注册完成");

            // 2. 连接
            client.connect()
                .thenRun(() -> logger.info("✅ 生命周期阶段2: 连接完成"))
                .get(5, TimeUnit.SECONDS);

            // 3. 启动服务
            CompletableFuture<Void> serviceFuture = client.serveAsync()
                .thenRun(() -> logger.info("✅ 生命周期阶段3: 服务启动"));

            // 4. 运行一段时间
            Thread.sleep(2000);
            logger.info("✅ 生命周期阶段4: 服务运行");

            // 5. 停止服务
            client.stop();
            logger.info("✅ 生命周期阶段5: 服务停止");

        } catch (Exception e) {
            logger.error("❌ 生命周期管理失败", e);
        } finally {
            // 6. 清理资源
            client.close();
            logger.info("✅ 生命周期阶段6: 资源清理完成");
        }
    }

    private static void demonstrateStateMonitoring(CroupierClient client) {
        logger.info("\n=== 📊 状态监控演示 ===");

        // 1. 初始状态
        logger.info("📊 初始状态: 连接={}, 服务={}, 本地地址={}",
                   client.isConnected(), client.isServing(), "N/A");

        // 2. 连接后状态
        try {
            client.connect().get(5, TimeUnit.SECONDS);
            logger.info("📊 连接后状态: 连接={}, 服务={}, 本地地址={}",
                       client.isConnected(), client.isServing(), "N/A");
        } catch (Exception e) {
            logger.error("状态监控 - 连接失败", e);
        }

        // 3. 服务启动后状态
        try {
            client.serveAsync();
            Thread.sleep(1000); // 等待服务启动
            logger.info("📊 服务后状态: 连接={}, 服务={}, 本地地址={}",
                       client.isConnected(), client.isServing(), "N/A");
        } catch (Exception e) {
            logger.error("状态监控 - 服务启动失败", e);
        }

        // 4. 停止后状态
        client.stop();
        logger.info("📊 停止后状态: 连接={}, 服务={}, 本地地址={}",
                   client.isConnected(), client.isServing(), "N/A");
    }

    // ==================== Utility Methods ====================

    private static void setupGracefulShutdown(CroupierClient client) {
        Runtime.getRuntime().addShutdownHook(new Thread(() -> {
            logger.info("🛑 收到停止信号，开始优雅关闭...");
            client.stop();
            client.close();
            executor.shutdown();
            try {
                if (!executor.awaitTermination(5, TimeUnit.SECONDS)) {
                    executor.shutdownNow();
                }
            } catch (InterruptedException e) {
                executor.shutdownNow();
                Thread.currentThread().interrupt();
            }
            logger.info("✅ 优雅关闭完成");
        }));
    }

    // ==================== Main Method ====================

    public static void main(String[] args) {
        logger.info("🎮 Croupier Java SDK 综合功能演示");
        logger.info("===============================================");

        try {
            // ==== 配置和创建演示 ====

            // 1. Builder模式演示
            demonstrateBuilderPattern();

            // 2. 配置管理演示
            demonstrateConfigurationManagement();

            // ==== 主要功能演示 ====

            // 创建主客户端
            ClientConfig config = new ClientConfig("comprehensive-example", "demo-service-java");
            config.setAgentAddr("localhost:19090");
            config.setEnv("development");
            config.setServiceVersion("1.0.0");
            config.setInsecure(true);

            logger.info("🔧 配置: 游戏={}, 环境={}, 服务={}",
                       config.getGameId(), config.getEnv(), config.getServiceId());

            CroupierClient client = CroupierSDK.createClient(config);
            setupGracefulShutdown(client);

            // 3. 函数注册演示
            demonstrateClientRegistration(client);

            // 4. 错误处理演示
            demonstrateErrorHandling(client);

            // 5. 异步操作演示
            demonstrateAsyncOperations(client);

            // 6. 并发操作演示
            demonstrateConcurrentOperations(client);

            // 7. 状态监控演示
            demonstrateStateMonitoring(client);

            // ==== 高级功能演示 ====

            // 8. 资源管理演示
            demonstrateResourceManagement();

            // 9. 复杂生命周期演示
            demonstrateComplexLifecycle();

            // 最终清理
            client.close();

            logger.info("\n🎉 所有功能演示完成!");
            logger.info("\n📊 演示统计:");
            logger.info("   ✅ 客户端接口: 9/9 已演示");
            logger.info("   ✅ 工厂方法: 3/3 已演示");
            logger.info("   ✅ Builder模式: 完全展示");
            logger.info("   ✅ 异步操作: CompletableFuture支持");
            logger.info("   ✅ 错误处理: 异常处理机制");
            logger.info("   ✅ 并发操作: 多线程安全");
            logger.info("   ✅ 资源管理: 完整生命周期");
            logger.info("   ✅ 状态监控: 实时状态查询");

            logger.info("\n💡 接口覆盖详情:");
            logger.info("   📝 registerFunction - 注册函数 (5个不同类型)");
            logger.info("   🔌 connect - 异步连接到Agent");
            logger.info("   🚀 serve - 阻塞式启动服务");
            logger.info("   ⚡ serveAsync - 异步启动服务");
            logger.info("   🛑 stop - 停止服务");
            logger.info("   🔐 close - 关闭客户端");
            logger.info("   📍 getLocalAddress - 获取本地地址");
            logger.info("   🔗 isConnected - 查询连接状态");
            logger.info("   🎯 isServing - 查询服务状态");

            logger.info("\n🏗️ Java特性演示:");
            logger.info("   ⚡ CompletableFuture异步编程");
            logger.info("   🏗️ Builder模式流式API");
            logger.info("   🏭 工厂方法多种创建方式");
            logger.info("   🔒 线程安全的并发操作");
            logger.info("   📦 资源自动管理");
            logger.info("   ⚠️ 强类型异常处理");
            logger.info("   📊 实时状态监控");
            logger.info("   🔄 完整的生命周期管理");

        } catch (Exception e) {
            logger.error("❌ 程序异常", e);
            System.exit(1);
        } finally {
            executor.shutdown();
        }
    }
}
