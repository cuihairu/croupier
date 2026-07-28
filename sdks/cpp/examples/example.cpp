#include "croupier/sdk/croupier_client.h"

#include <iostream>

using namespace croupier::sdk;

// 简单的SampleHandler器
std::string HelloHandler(const std::string& context, const std::string& payload) {
    std::cout << "🎯 Hello Function被Invoke！" << std::endl;
    std::cout << "   Context: " << context << std::endl;
    std::cout << "   Payload: " << payload << std::endl;

    return R"({
        "message": "Hello from Croupier C++ SDK!",
        "timestamp": "2024-11-14T12:00:00Z",
        "status": "success"
    })";
}

int main() {
    std::cout << "🚀 Croupier C++ SDK - 基础Sample" << std::endl;
    std::cout << "==============================" << std::endl;

    try {
        // ConfigurationClient
        ClientConfig config;
        config.game_id = "example-game";
        config.env = "development";
        config.service_id = "basic-example";
        config.agent_addr = "127.0.0.1:19090";
        config.insecure = true;

        std::cout << "📋 ConfigurationInfo:" << std::endl;
        std::cout << "   游戏ID: " << config.game_id << std::endl;
        std::cout << "   环境: " << config.env << std::endl;

        // CreateClient
        CroupierClient client(config);

        // Register简单Function
        FunctionDescriptor desc;
        desc.id = "example.hello";
        desc.version = "1.0.0";

        std::cout << "📝 RegisterFunction: " << desc.id << std::endl;

        bool registered = client.RegisterFunction(desc, HelloHandler);
        if (!registered) {
            std::cerr << "❌ FunctionRegisterFailed!" << std::endl;
            return 1;
        }

        std::cout << "✅ FunctionRegisterSuccess!" << std::endl;

        // 尝试Connect到 Agent
        std::cout << "🔌 尝试Connect到 Agent..." << std::endl;

        bool connected = client.Connect();
        if (connected) {
            std::cout << "✅ SuccessConnect到 Agent!" << std::endl;
            std::cout << "🚀 StartService..." << std::endl;
            std::cout << "💡 在生产环境中，这里会保持运行等待Invoke" << std::endl;

            // 在Sample中，我们不阻塞，而是Simulate运行
            std::cout << "📡 SimulateHandler请求..." << std::endl;

            // SimulateHandler一个请求
            std::string result = HelloHandler("example-context", R"({"test": "data"})");
            std::cout << "📤 Handler结果: " << result << std::endl;

        } else {
            std::cout << "⚠️  无法Connect到 Agent (这在没有Agent运行时是正常的)" << std::endl;
            std::cout << "💡 要完整Test，请确保 Croupier Agent 在 " << config.agent_addr << " 运行" << std::endl;
        }

        std::cout << "\n🎉 基础Sample运行Complete!" << std::endl;
        std::cout << "💡 查看 game_demo.cpp 了解更多函数注册示例" << std::endl;

    } catch (const std::exception& e) {
        std::cerr << "💥 程序Exception: " << e.what() << std::endl;
        return 1;
    }

    return 0;
}
