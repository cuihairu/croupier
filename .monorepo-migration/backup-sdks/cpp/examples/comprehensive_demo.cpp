/**
 * Comprehensive Example: Demonstrates ALL Croupier C++ SDK interfaces
 *
 * This example showcases:
 * 1. Client interface - Function/Object/Component registration and management
 * 2. Invoker interface - Function invocation and job management
 * 3. Configuration management
 * 4. Error handling
 * 5. Lifecycle management
 */

#include "croupier/sdk/croupier_client.h"
#include <iostream>
#include <thread>
#include <chrono>
#include <atomic>
#include <signal.h>
#include <fstream>

using namespace croupier::sdk;

// Global flag for graceful shutdown
std::atomic<bool> g_shutdown(false);

void signalHandler(int signal) {
    std::cout << "\n🛑 收到停止信号 (" << signal << "), 开始优雅关闭..." << std::endl;
    g_shutdown = true;
}

// ==================== Function Handlers ====================

std::string playerBanHandler(const std::string& context, const std::string& payload) {
    std::cout << "🔨 执行玩家封禁 - Context: " << context << ", Payload: " << payload << std::endl;

    // 模拟处理时间
    std::this_thread::sleep_for(std::chrono::milliseconds(100));

    return R"({"status":"success","action":"ban","player_id":"player_123","reason":"违规行为"})";
}

std::string itemCreateHandler(const std::string& context, const std::string& payload) {
    std::cout << "📦 创建游戏道具 - Context: " << context << ", Payload: " << payload << std::endl;

    auto now = std::chrono::system_clock::now();
    auto timestamp = std::chrono::duration_cast<std::chrono::seconds>(now.time_since_epoch()).count();

    return R"({"status":"success","action":"create","item_id":"item_)" + std::to_string(timestamp) + R"(","type":"weapon"})";
}

std::string playerGetHandler(const std::string& context, const std::string& payload) {
    std::cout << "👤 获取玩家信息 - Context: " << context << ", Payload: " << payload << std::endl;
    return R"({"status":"success","player_id":"player_123","level":50,"exp":125000})";
}

std::string playerUpdateHandler(const std::string& context, const std::string& payload) {
    std::cout << "✏️ 更新玩家信息 - Context: " << context << ", Payload: " << payload << std::endl;
    return R"({"status":"success","updated_fields":["level","exp"]})";
}

std::string playerDeleteHandler(const std::string& context, const std::string& payload) {
    std::cout << "🗑️ 删除玩家信息 - Context: " << context << ", Payload: " << payload << std::endl;
    return R"({"status":"success","action":"delete","player_id":"player_123"})";
}

// ==================== Demo Functions ====================

void demonstrateBasicFunctionRegistration(CroupierClient& client) {
    std::cout << "\n=== 📝 基础函数注册演示 ===" << std::endl;

    // 1. 注册高风险管理函数
    FunctionDescriptor banDesc;
    banDesc.id = "player.ban";
    banDesc.version = "1.0.0";
    banDesc.category = "moderation";
    banDesc.risk = "high";
    banDesc.entity = "player";
    banDesc.operation = "update";
    banDesc.enabled = true;

    if (client.RegisterFunction(banDesc, playerBanHandler)) {
        std::cout << "✅ 成功注册玩家封禁函数" << std::endl;
    }

    // 2. 注册低风险物品创建函数
    FunctionDescriptor itemDesc;
    itemDesc.id = "item.create";
    itemDesc.version = "1.0.0";
    itemDesc.category = "inventory";
    itemDesc.risk = "low";
    itemDesc.entity = "item";
    itemDesc.operation = "create";
    itemDesc.enabled = true;

    if (client.RegisterFunction(itemDesc, itemCreateHandler)) {
        std::cout << "✅ 成功注册道具创建函数" << std::endl;
    }
}

void demonstrateVirtualObjectRegistration(CroupierClient& client) {
    std::cout << "\n=== 🎯 虚拟对象注册演示 ===" << std::endl;

    // 创建玩家实体的完整CRUD操作
    VirtualObjectDescriptor playerObject;
    playerObject.id = "player";
    playerObject.version = "1.0.0";
    playerObject.name = "游戏玩家实体";
    playerObject.description = "管理玩家的完整生命周期";

    // 设置schema
    playerObject.schema["type"] = "object";
    playerObject.schema["properties"] = R"({
        "id": {"type": "string", "title": "玩家ID"},
        "name": {"type": "string", "title": "玩家姓名"},
        "level": {"type": "integer", "title": "等级"},
        "exp": {"type": "integer", "title": "经验值"}
    })";

    // 映射CRUD操作到函数
    playerObject.operations["create"] = "player.create";
    playerObject.operations["read"] = "player.get";
    playerObject.operations["update"] = "player.update";
    playerObject.operations["delete"] = "player.delete";

    // 设置关系
    playerObject.relationships["items"] = {"one-to-many", "item", "player_id"};
    playerObject.relationships["guild"] = {"many-to-one", "guild", "guild_id"};

    // 准备处理器映射
    std::map<std::string, FunctionHandler> playerHandlers = {
        {"player.create", playerUpdateHandler},  // 复用更新处理器作为创建
        {"player.get", playerGetHandler},
        {"player.update", playerUpdateHandler},
        {"player.delete", playerDeleteHandler}
    };

    if (client.RegisterVirtualObject(playerObject, playerHandlers)) {
        std::cout << "✅ 成功注册玩家虚拟对象 (包含4个CRUD操作)" << std::endl;
    }
}

void demonstrateComponentManagement(CroupierClient& client) {
    std::cout << "\n=== 📦 组件管理演示 ===" << std::endl;

    // 创建完整的游戏组件
    ComponentDescriptor gameComponent;
    gameComponent.id = "player-management";
    gameComponent.version = "1.0.0";
    gameComponent.name = "玩家管理组件";
    gameComponent.description = "包含玩家相关的所有功能";

    // 添加已注册的虚拟对象到组件中
    // 注意：这里我们重复定义玩家对象，在真实场景中应该避免
    VirtualObjectDescriptor componentPlayer;
    componentPlayer.id = "component_player";
    componentPlayer.version = "1.0.0";
    componentPlayer.name = "组件内玩家实体";
    componentPlayer.description = "组件内部的玩家定义";
    componentPlayer.operations["ban"] = "player.ban";

    gameComponent.entities.push_back(componentPlayer);

    // 添加独立函数
    FunctionDescriptor utilFunc;
    utilFunc.id = "util.generate_id";
    utilFunc.version = "1.0.0";
    utilFunc.category = "utility";
    utilFunc.risk = "low";
    utilFunc.entity = "system";
    utilFunc.operation = "read";
    utilFunc.enabled = true;

    gameComponent.functions.push_back(utilFunc);

    // 组件配置
    gameComponent.resources["database"] = "player_db";
    gameComponent.resources["cache"] = "redis_cache";
    gameComponent.config["max_level"] = "100";
    gameComponent.config["exp_multiplier"] = "1.5";

    if (client.RegisterComponent(gameComponent)) {
        std::cout << "✅ 成功注册玩家管理组件" << std::endl;
    }

    // 展示已注册的对象和组件
    std::cout << "\n--- 📋 已注册对象列表 ---" << std::endl;
    auto objects = client.GetRegisteredObjects();
    for (const auto& obj : objects) {
        std::cout << "🎯 对象: " << obj.id << " (v" << obj.version << ") - " << obj.operations.size() << " 个操作" << std::endl;
    }

    std::cout << "\n--- 📋 已注册组件列表 ---" << std::endl;
    auto components = client.GetRegisteredComponents();
    for (const auto& comp : components) {
        std::cout << "📦 组件: " << comp.id << " (v" << comp.version << ") - "
                  << comp.entities.size() << " 个实体, "
                  << comp.functions.size() << " 个函数" << std::endl;
    }
}

void demonstrateClientLifecycle(CroupierClient& client) {
    std::cout << "\n=== 🔄 客户端生命周期演示 ===" << std::endl;

    // 连接到Agent
    if (client.Connect()) {
        std::cout << "✅ 成功连接到Agent" << std::endl;
        std::cout << "📍 本地服务地址: " << client.GetLocalAddress() << std::endl;
    } else {
        std::cout << "❌ 连接失败" << std::endl;
        return;
    }

    // 启动服务 (在单独线程中)
    std::cout << "🚀 启动客户端服务..." << std::endl;
    std::thread serviceThread([&client]() {
        client.Serve();
    });

    // 让服务运行一段时间
    std::cout << "⏳ 服务运行中，等待3秒..." << std::endl;
    std::this_thread::sleep_for(std::chrono::seconds(3));

    // 优雅停止
    std::cout << "🛑 停止服务..." << std::endl;
    client.Stop();

    if (serviceThread.joinable()) {
        serviceThread.join();
    }

    std::cout << "✅ 服务已停止" << std::endl;
}

void demonstrateInvoker() {
    std::cout << "\n=== 📞 调用器接口演示 ===" << std::endl;

    // 创建调用器配置
    InvokerConfig invokerConfig;
    invokerConfig.address = "localhost:8080";  // 连接到服务器或代理
    invokerConfig.timeout_seconds = 30;
    invokerConfig.insecure = true;

    CroupierInvoker invoker(invokerConfig);

    try {
        // 1. 连接
        if (invoker.Connect()) {
            std::cout << "✅ 调用器连接成功" << std::endl;
        }

        // 2. 设置函数schema
        std::map<std::string, std::string> banSchema = {
            {"type", "object"},
            {"properties", R"({"player_id":{"type":"string"},"reason":{"type":"string"}})"}
        };
        invoker.SetSchema("player.ban", banSchema);
        std::cout << "✅ 设置player.ban函数的validation schema" << std::endl;

        // 3. 同步调用
        InvokeOptions options;
        options.idempotency_key = utils::NewIdempotencyKey();

        std::string payload = R"({"player_id":"player_123","reason":"违规聊天"})";
        std::string result = invoker.Invoke("player.ban", payload, options);
        std::cout << "📞 同步调用结果: " << result << std::endl;

        // 4. 启动异步作业
        std::string jobId = invoker.StartJob("item.create", R"({"type":"sword","rarity":"epic"})", options);
        std::cout << "🚀 启动异步作业: " << jobId << std::endl;

        // 5. 流式获取作业事件
        auto future = invoker.StreamJob(jobId);
        std::cout << "📡 监听作业事件..." << std::endl;

        auto events = future.get();
        for (const auto& event : events) {
            std::cout << "📋 作业事件: " << event.event_type
                      << ", 负载: " << event.payload;
            if (event.done) {
                std::cout << " (完成)";
            }
            std::cout << std::endl;

            // 演示取消作业 (在progress事件时)
            if (event.event_type == "progress") {
                std::cout << "⏹️ 演示取消作业..." << std::endl;
                if (invoker.CancelJob(jobId)) {
                    std::cout << "✅ 作业取消成功" << std::endl;
                }
            }
        }

        // 6. 关闭调用器
        invoker.Close();
        std::cout << "✅ 调用器已关闭" << std::endl;

    } catch (const std::exception& e) {
        std::cout << "❌ 调用器操作失败: " << e.what() << std::endl;
    }
}

void demonstrateCleanup(CroupierClient& client) {
    std::cout << "\n=== 🧹 清理演示 ===" << std::endl;

    // 取消注册虚拟对象
    if (client.UnregisterVirtualObject("player")) {
        std::cout << "✅ 成功取消注册玩家虚拟对象" << std::endl;
    }

    // 取消注册组件
    if (client.UnregisterComponent("player-management")) {
        std::cout << "✅ 成功取消注册玩家管理组件" << std::endl;
    }

    // 最终关闭客户端
    client.Close();
    std::cout << "✅ 客户端已完全关闭" << std::endl;
}

void createExampleConfigFile() {
    std::cout << "\n=== 📄 创建示例配置文件 ===" << std::endl;

    std::string configContent = R"({
  "id": "sample-component",
  "version": "1.0.0",
  "name": "示例组件",
  "description": "从文件加载的示例组件",
  "entities": [
    {
      "id": "sample_entity",
      "version": "1.0.0",
      "name": "示例实体",
      "description": "配置文件定义的实体",
      "schema": {
        "type": "object",
        "properties": {
          "id": {"type": "string"},
          "data": {"type": "string"}
        }
      },
      "operations": {
        "process": "sample.process"
      },
      "relationships": {}
    }
  ],
  "functions": [
    {
      "id": "sample.utility",
      "version": "1.0.0",
      "category": "utility",
      "risk": "low",
      "entity": "system",
      "operation": "read",
      "enabled": true
    }
  ],
  "resources": {
    "storage": "file_system",
    "network": "http_client"
  },
  "config": {
    "timeout": "30s",
    "retry_count": "3"
  }
})";

    std::ofstream configFile("sample_component.json");
    configFile << configContent;
    configFile.close();

    std::cout << "✅ 创建配置文件: sample_component.json" << std::endl;
}

void demonstrateFileLoading(CroupierClient& client) {
    std::cout << "\n=== 📁 文件加载演示 ===" << std::endl;

    createExampleConfigFile();

    // 从文件加载组件
    if (client.LoadComponentFromFile("sample_component.json")) {
        std::cout << "✅ 成功从文件加载组件配置" << std::endl;

        // 显示加载的组件
        auto components = client.GetRegisteredComponents();
        for (const auto& comp : components) {
            if (comp.id == "sample-component") {
                std::cout << "📦 加载的组件: " << comp.name << " - " << comp.description << std::endl;
            }
        }
    } else {
        std::cout << "❌ 从文件加载组件失败" << std::endl;
    }

    // 清理临时文件
    std::remove("sample_component.json");
}

int main() {
    std::cout << "🎮 Croupier C++ SDK 综合功能演示" << std::endl;
    std::cout << "===============================================" << std::endl;

    // 设置信号处理
    signal(SIGINT, signalHandler);
    signal(SIGTERM, signalHandler);

    try {
        // 创建客户端配置
        ClientConfig config;
        config.game_id = "comprehensive-example";
        config.env = "development";
        config.service_id = "demo-service";
        config.service_version = "1.0.0";
        config.agent_addr = "localhost:19090";
        config.local_listen = ":0";  // 自动分配端口
        config.timeout_seconds = 30;
        config.insecure = true;

        std::cout << "🔧 配置: 游戏=" << config.game_id
                  << ", 环境=" << config.env
                  << ", 服务=" << config.service_id << std::endl;

        // 创建客户端
        CroupierClient client(config);

        // ==== 演示所有客户端接口 ====

        // 1. 基础函数注册
        demonstrateBasicFunctionRegistration(client);

        // 2. 虚拟对象注册 (展示完整的CRUD映射)
        demonstrateVirtualObjectRegistration(client);

        // 3. 组件管理
        demonstrateComponentManagement(client);

        // 4. 文件加载
        demonstrateFileLoading(client);

        // 5. 客户端生命周期 (Connect, Serve, Stop)
        demonstrateClientLifecycle(client);

        // ==== 演示调用器接口 ====

        // 6. 调用器功能演示
        demonstrateInvoker();

        // 7. 清理演示
        demonstrateCleanup(client);

        std::cout << "\n🎉 所有功能演示完成!" << std::endl;
        std::cout << "\n📊 演示统计:" << std::endl;
        std::cout << "   ✅ 客户端接口: 11/11 已演示" << std::endl;
        std::cout << "   ✅ 调用器接口: 6/6 已演示" << std::endl;
        std::cout << "   ✅ 配置管理: 完整演示" << std::endl;
        std::cout << "   ✅ 错误处理: 包含异常捕获" << std::endl;
        std::cout << "   ✅ 生命周期: 完整演示" << std::endl;

        std::cout << "\n💡 接口覆盖详情:" << std::endl;
        std::cout << "   📝 RegisterFunction - 注册基础函数" << std::endl;
        std::cout << "   🎯 RegisterVirtualObject - 注册虚拟对象CRUD" << std::endl;
        std::cout << "   📦 RegisterComponent - 注册复杂组件" << std::endl;
        std::cout << "   📁 LoadComponentFromFile - 文件配置加载" << std::endl;
        std::cout << "   📋 GetRegisteredObjects - 查询已注册对象" << std::endl;
        std::cout << "   📋 GetRegisteredComponents - 查询已注册组件" << std::endl;
        std::cout << "   🗑️ UnregisterVirtualObject - 取消注册对象" << std::endl;
        std::cout << "   🗑️ UnregisterComponent - 取消注册组件" << std::endl;
        std::cout << "   🔌 Connect - 连接到Agent" << std::endl;
        std::cout << "   🚀 Serve - 启动服务" << std::endl;
        std::cout << "   🛑 Stop - 停止服务" << std::endl;
        std::cout << "   🔐 Close - 关闭客户端" << std::endl;
        std::cout << "   📍 GetLocalAddress - 获取本地地址" << std::endl;
        std::cout << "   📞 Invoke - 同步函数调用" << std::endl;
        std::cout << "   🚀 StartJob - 启动异步作业" << std::endl;
        std::cout << "   📡 StreamJob - 流式作业事件" << std::endl;
        std::cout << "   ⏹️ CancelJob - 取消作业" << std::endl;
        std::cout << "   📄 SetSchema - 设置验证模式" << std::endl;

    } catch (const std::exception& e) {
        std::cerr << "❌ 程序异常: " << e.what() << std::endl;
        return 1;
    }

    return 0;
}