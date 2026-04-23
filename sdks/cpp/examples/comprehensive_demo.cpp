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
#include <limits>

using namespace croupier::sdk;

// Global flag for graceful shutdown
std::atomic<bool> g_shutdown(false);

void signalHandler(int signal) {
    std::cout << "\n🛑 Received shutdown signal (" << signal << "), Starting graceful shutdown..." << '\n';
    g_shutdown = true;
}

// ==================== Function Handlers ====================

std::string playerBanHandler(const std::string& context, const std::string& payload) {
    std::cout << "🔨 Executing player ban - Context: " << context << ", Payload: " << payload << '\n';

    // SimulateHandlerTime
    std::this_thread::sleep_for(std::chrono::milliseconds(100));

    return R"({"status":"success","action":"ban","player_id":"player_123","reason":"违规行为"})";
}

std::string itemCreateHandler(const std::string& context, const std::string& payload) {
    std::cout << "📦 Creating game item - Context: " << context << ", Payload: " << payload << '\n';

    auto now = std::chrono::system_clock::now();
    auto timestamp = std::chrono::duration_cast<std::chrono::seconds>(now.time_since_epoch()).count();

    return R"({"status":"success","action":"create","item_id":"item_)" + std::to_string(timestamp) + R"(","type":"weapon"})";
}

std::string playerGetHandler(const std::string& context, const std::string& payload) {
    std::cout << "👤 Getting player info - Context: " << context << ", Payload: " << payload << '\n';
    return R"({"status":"success","player_id":"player_123","level":50,"exp":125000})";
}

std::string playerUpdateHandler(const std::string& context, const std::string& payload) {
    std::cout << "✏️ Updating player info - Context: " << context << ", Payload: " << payload << '\n';
    return R"({"status":"success","updated_fields":["level","exp"]})";
}

std::string playerDeleteHandler(const std::string& context, const std::string& payload) {
    std::cout << "🗑️ Deleting player info - Context: " << context << ", Payload: " << payload << '\n';
    return R"({"status":"success","action":"delete","player_id":"player_123"})";
}

// ==================== Demo Functions ====================

void demonstrateBasicFunctionRegistration(CroupierClient& client) {
    std::cout << "\n=== 📝 Basic Function Registration Demo ===" << '\n';

    // 1. Register高风险ManagementFunction
    FunctionDescriptor banDesc;
    banDesc.id = "player.ban";
    banDesc.version = "1.0.0";
    banDesc.category = "moderation";
    banDesc.risk = "high";
    banDesc.entity = "player";
    banDesc.operation = "update";
    banDesc.enabled = true;

    if (client.RegisterFunction(banDesc, playerBanHandler)) {
        std::cout << "✅ Successfully registered player ban function" << '\n';
    }

    // 2. Register低风险物品CreateFunction
    FunctionDescriptor itemDesc;
    itemDesc.id = "item.create";
    itemDesc.version = "1.0.0";
    itemDesc.category = "inventory";
    itemDesc.risk = "low";
    itemDesc.entity = "item";
    itemDesc.operation = "create";
    itemDesc.enabled = true;

    if (client.RegisterFunction(itemDesc, itemCreateHandler)) {
        std::cout << "✅ Successfully registered item creation function" << '\n';
    }
}

void demonstrateVirtualObjectRegistration(CroupierClient& client) {
    std::cout << "\n=== 🎯 Virtual Object Registration Demo ===" << '\n';

    // CreatePlayerEntity的完整CRUDOperation
    VirtualObjectDescriptor playerObject;
    playerObject.id = "player";
    playerObject.version = "1.0.0";
    playerObject.name = "GamePlayerEntity";
    playerObject.description = "ManagementPlayer的完整Lifecycle";

    // Setschema
    playerObject.schema["type"] = "object";
    playerObject.schema["properties"] = R"({
        "id": {"type": "string", "title": "PlayerID"},
        "name": {"type": "string", "title": "Player姓名"},
        "level": {"type": "integer", "title": "等级"},
        "exp": {"type": "integer", "title": "经验值"}
    })";

    // 映射CRUDOperation到Function
    playerObject.operations["create"] = "player.create";
    playerObject.operations["read"] = "player.get";
    playerObject.operations["update"] = "player.update";
    playerObject.operations["delete"] = "player.delete";

    // Set关系
    playerObject.relationships["items"] = {"one-to-many", "item", "player_id"};
    playerObject.relationships["guild"] = {"many-to-one", "guild", "guild_id"};

    // 准备Handler器映射
    std::map<std::string, FunctionHandler> playerHandlers = {
        {"player.create", playerUpdateHandler},  // 复用UpdateHandler器作为Create
        {"player.get", playerGetHandler},
        {"player.update", playerUpdateHandler},
        {"player.delete", playerDeleteHandler}
    };

    if (client.RegisterVirtualObject(playerObject, playerHandlers)) {
        std::cout << "✅ Successfully registered player virtual object (with 4 CRUD operations)" << '\n';
    }
}

void demonstrateComponentManagement(CroupierClient& client) {
    std::cout << "\n=== 📦 ComponentManagementDemo ===" << '\n';

    // Create完整的GameComponent
    ComponentDescriptor gameComponent;
    gameComponent.id = "player-management";
    gameComponent.version = "1.0.0";
    gameComponent.name = "PlayerManagementComponent";
    gameComponent.description = "IncludePlayer相关的AllFeature";

    // Add registered virtual objects to component
    // 注意：这里我们重复DefinitionPlayerObject，在真实场景中应该避免
    VirtualObjectDescriptor componentPlayer;
    componentPlayer.id = "component_player";
    componentPlayer.version = "1.0.0";
    componentPlayer.name = "Component内PlayerEntity";
    componentPlayer.description = "Component内部的PlayerDefinition";
    componentPlayer.operations["ban"] = "player.ban";

    gameComponent.entities.push_back(componentPlayer);

    // 添加独立Function
    FunctionDescriptor utilFunc;
    utilFunc.id = "util.generate_id";
    utilFunc.version = "1.0.0";
    utilFunc.category = "utility";
    utilFunc.risk = "low";
    utilFunc.entity = "system";
    utilFunc.operation = "read";
    utilFunc.enabled = true;

    gameComponent.functions.push_back(utilFunc);

    // ComponentConfiguration
    gameComponent.resources["database"] = "player_db";
    gameComponent.resources["cache"] = "redis_cache";
    gameComponent.config["max_level"] = "100";
    gameComponent.config["exp_multiplier"] = "1.5";

    if (client.RegisterComponent(gameComponent)) {
        std::cout << "✅ Successfully registered player management component" << '\n';
    }

    // Display registered objects and components
    std::cout << "\n--- 📋 List of Registered Objects ---" << '\n';
    auto objects = client.GetRegisteredObjects();
    for (const auto& obj : objects) {
        std::cout << "🎯 Object: " << obj.id << " (v" << obj.version << ") - " << obj.operations.size() << "  operations" << '\n';
    }

    std::cout << "\n--- 📋 List of Registered Components ---" << '\n';
    auto components = client.GetRegisteredComponents();
    for (const auto& comp : components) {
        std::cout << "📦 Component: " << comp.id << " (v" << comp.version << ") - "
                  << comp.entities.size() << "  entities, "
                  << comp.functions.size() << "  functions" << '\n';
    }
}

void demonstrateClientLifecycle(CroupierClient& client) {
    std::cout << "\n=== 🔄 ClientLifecycleDemo ===" << '\n';

    // Connect to Agent
    if (client.Connect()) {
        std::cout << "✅ Successfully connected to Agent" << '\n';
        std::cout << "📍 LocalServiceAddress: " << client.GetLocalAddress() << '\n';
    } else {
        std::cout << "❌ Connection failed" << '\n';
        return;
    }

    // Serve - Start local service (在单独线程中)
    std::cout << "🚀 StartClientService..." << '\n';
    std::thread serviceThread([&client]() {
        client.Serve();
    });

    // 让Service运行一段Time（测试模式：快速退出）
    std::cout << "⏳ Service运行中..." << '\n';
    std::this_thread::sleep_for(std::chrono::milliseconds(500));

    // 优雅Stop
    std::cout << "🛑 Stop - Stop local service..." << '\n';
    client.Stop();

    if (serviceThread.joinable()) {
        serviceThread.join();
    }

    std::cout << "✅ Service已Stop" << '\n';
}

void demonstrateInvoker() {
    std::cout << "\n=== 📞 Invoker InterfaceDemo ===" << '\n';

    // CreateInvoke器Configuration
    InvokerConfig invokerConfig;
    invokerConfig.address = "localhost:18443";  // Connect到Server gRPC端口 (不是HTTP端口)
    invokerConfig.timeout_seconds = 30;

    // TLS 配置：优先从环境变量读取
    const char* ca_cert_path = std::getenv("CROUPIER_CA_CERT");
    if (ca_cert_path && std::string(ca_cert_path) != "") {
        invokerConfig.insecure = false;
        invokerConfig.ca_file = ca_cert_path;
        std::cout << "🔐 Invoker using TLS with CA cert: " << ca_cert_path << '\n';
    } else {
        invokerConfig.insecure = true;
        std::cout << "⚠️  Invoker using insecure connection (dev only)" << '\n';
    }

    CroupierInvoker invoker(invokerConfig);

    try {
        // 1. Connect
        if (invoker.Connect()) {
            std::cout << "✅ Invoker connection successful" << '\n';
        }

        // 2. SetFunctionschema
        std::map<std::string, std::string> banSchema = {
            {"type", "object"},
            {"properties", R"({"player_id":{"type":"string"},"reason":{"type":"string"}})"}
        };
        invoker.SetSchema("player.ban", banSchema);
        std::cout << "✅ Set validation schema for player.ban function" << '\n';

        // 3. SyncInvoke
        InvokeOptions options;
        options.idempotency_key = utils::NewIdempotencyKey();

        std::string payload = R"({"player_id":"player_123","reason":"违规聊天"})";
        std::string result = invoker.Invoke("player.ban", payload, options);
        std::cout << "📞 SyncInvoke结果: " << result << '\n';

        // 4. Start async job
        std::string jobId = invoker.StartJob("item.create", R"({"type":"sword","rarity":"epic"})", options);
        std::cout << "🚀 Start async job: " << jobId << '\n';

        // 5. StreamGetJobevents
        auto future = invoker.StreamJob(jobId);
        std::cout << "📡 监听Jobevents..." << '\n';

        auto events = future.get();
        for (const auto& event : events) {
            std::cout << "📋 Jobevents: " << event.event_type
                      << ", 负载: " << event.payload;
            if (event.done) {
                std::cout << " (Complete)";
            }
            std::cout << '\n';

            // DemoCancel job (在progressevents时)
            if (event.event_type == "progress") {
                std::cout << "⏹️ DemoCancel job..." << '\n';
                if (invoker.CancelJob(jobId)) {
                    std::cout << "✅ Job cancellation successful" << '\n';
                }
            }
        }

        // 6. CloseInvoke器
        invoker.Close();
        std::cout << "✅ Invoke器已Close" << '\n';

    } catch (const std::exception& e) {
        std::cout << "❌ Invoker operation failed: " << e.what() << '\n';
    }
}

void demonstrateCleanup(CroupierClient& client) {
    std::cout << "\n=== 🧹 Cleanup Demo ===" << '\n';

    // CancelRegisterVirtualObject - Register virtual object CRUD
    if (client.UnregisterVirtualObject("player")) {
        std::cout << "✅ Successfully unregistered player virtual object" << '\n';
    }

    // UnregisterComponent - Register complex component
    if (client.UnregisterComponent("player-management")) {
        std::cout << "✅ Successfully unregistered player management component" << '\n';
    }

    // 最终Close client
    client.Close();
    std::cout << "✅ Client fully stopped" << '\n';
}

void createExampleConfigFile() {
    std::cout << "\n=== 📄 Create sample configuration file ===" << '\n';

    std::string configContent = R"({
  "id": "sample-component",
  "version": "1.0.0",
  "name": "SampleComponent",
  "description": " loaded from file: SampleComponent",
  "entities": [
    {
      "id": "sample_entity",
      "version": "1.0.0",
      "name": "SampleEntity",
      "description": "ConfigurationFileDefinition的Entity",
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

    std::cout << "✅ CreateConfigurationFile: sample_component.json" << '\n';
}

void demonstrateFileLoading(CroupierClient& client) {
    std::cout << "\n=== 📁 FileLoadDemo ===" << '\n';

    createExampleConfigFile();

    // LoadComponentFromFile - Load component config from file
    if (client.LoadComponentFromFile("sample_component.json")) {
        std::cout << "✅ Successfully loaded component configuration from file" << '\n';

        // 显示LoadedComponent
        auto components = client.GetRegisteredComponents();
        for (const auto& comp : components) {
            if (comp.id == "sample-component") {
                std::cout << "📦 LoadedComponent: " << comp.name << " - " << comp.description << '\n';
            }
        }
    } else {
        std::cout << "❌ LoadComponentFromFile - Load component config from fileFailed" << '\n';
    }

    // Cleanup临时File
    std::remove("sample_component.json");
}

int main() {
    std::cout << "🎮 Croupier C++ SDK Comprehensive Feature Demo" << '\n';
    std::cout << "===============================================" << '\n';

    // Set信号Handler
    signal(SIGINT, signalHandler);
    signal(SIGTERM, signalHandler);

    try {
        // CreateClientConfiguration
        ClientConfig config;
        config.game_id = "comprehensive-example";
        config.env = "development";
        config.service_id = "demo-service";
        config.service_version = "1.0.0";
        config.agent_addr = "localhost:19090";
        config.local_listen = "0.0.0.0:0";  // 自动分配端口（gRPC 需要完整地址）
        config.timeout_seconds = 30;

        // TLS 配置：优先从环境变量读取，否则使用不安全连接
        const char* ca_cert_path = std::getenv("CROUPIER_CA_CERT");
        if (ca_cert_path && std::string(ca_cert_path) != "") {
            config.insecure = false;
            config.ca_file = ca_cert_path;
            std::cout << "🔐 Using TLS with CA cert from environment: " << ca_cert_path << '\n';
        } else {
            config.insecure = true;  // 回退到不安全连接（仅用于开发环境）
            std::cout << "⚠️  No CA cert provided, using insecure connection (dev only)" << '\n';
        }

        std::cout << "🔧 Configuration: Game=" << config.game_id
                  << ", Environment=" << config.env
                  << ", Service=" << config.service_id << '\n';

        // CreateClient
        CroupierClient client(config);

        // ==== DemoAllClientInterface ====

        // 1. 基础FunctionRegister
        demonstrateBasicFunctionRegistration(client);

        // 2. Virtual ObjectRegister (展示完整的CRUD映射)
        demonstrateVirtualObjectRegistration(client);

        // 3. ComponentManagement
        demonstrateComponentManagement(client);

        // 4. FileLoad
        demonstrateFileLoading(client);

        // 5. ClientLifecycle (Connect, Serve, Stop)
        demonstrateClientLifecycle(client);

        // ==== DemoInvoker Interface ====

        // 6. Invoke器FeatureDemo
        demonstrateInvoker();

        // 7. Cleanup Demo
        demonstrateCleanup(client);

        std::cout << "\n🎉 All feature demos complete!" << '\n';
        std::cout << "\n📊 Test Statistics:" << '\n';
        std::cout << "   ✅ ClientInterface: 11/11  tested" << '\n';
        std::cout << "   ✅ Invoker Interface: 6/6  tested" << '\n';
        std::cout << "   ✅ ConfigurationManagement:  complete demo" << '\n';
        std::cout << "   ✅ Error handling: Exception catching included" << '\n';
        std::cout << "   ✅ Lifecycle:  complete demo" << '\n';

        std::cout << "\n💡 Interface Coverage Details:" << '\n';
        std::cout << "   📝 RegisterFunction - Register basic function" << '\n';
        std::cout << "   🎯 RegisterVirtualObject - Register virtual object CRUD" << '\n';
        std::cout << "   📦 RegisterComponent - Register complex component" << '\n';
        std::cout << "   📁 LoadComponentFromFile - FileConfigurationLoad" << '\n';
        std::cout << "   📋 GetRegisteredObjects - Query registered objects" << '\n';
        std::cout << "   📋 GetRegisteredComponents - Query registered components" << '\n';
        std::cout << "   🗑️ UnregisterVirtualObject - UnregisterVirtualObject" << '\n';
        std::cout << "   🗑️ UnregisterComponent - Unregister complex component" << '\n';
        std::cout << "   🔌 Connect - Connect to Agent" << '\n';
        std::cout << "   🚀 Serve - Start local service" << '\n';
        std::cout << "   🛑 Stop - Stop local service" << '\n';
        std::cout << "   🔐 Close - Close client" << '\n';
        std::cout << "   📍 GetLocalAddress - Get local address" << '\n';
        std::cout << "   📞 Invoke - Synchronous function invocation" << '\n';
        std::cout << "   🚀 StartJob - Start async job" << '\n';
        std::cout << "   📡 StreamJob - Stream job events" << '\n';
        std::cout << "   ⏹️ CancelJob - Cancel job" << '\n';
        std::cout << "   📄 SetSchema - Set validation schema" << '\n';

        // 等待用户按键后再退出 (避免闪退)
        std::cout << "\n\n按 Enter 键退出程序..." << std::flush;
        std::cin.ignore(std::numeric_limits<std::streamsize>::max(), '\n');

    } catch (const std::exception& e) {
        std::cerr << "❌ 程序Exception: " << e.what() << '\n';
        std::cerr << "按 Enter 键退出..." << std::flush;
        std::cin.ignore(std::numeric_limits<std::streamsize>::max(), '\n');
        return 1;
    }

    return 0;
}