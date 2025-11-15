#include "croupier/sdk/croupier_client.h"
#include <iostream>

using namespace croupier::sdk;

// 简单的示例处理器
std::string HelloHandler(const std::string& context, const std::string& payload) {
    std::cout << "🎯 Hello 函数被调用！" << std::endl;
    std::cout << "   Context: " << context << std::endl;
    std::cout << "   Payload: " << payload << std::endl;

    return R"({
        "message": "Hello from Croupier C++ SDK!",
        "timestamp": "2024-11-14T12:00:00Z",
        "status": "success"
    })";
}

int main() {
    std::cout << "🚀 Croupier C++ SDK - 基础示例" << std::endl;
    std::cout << "==============================" << std::endl;

    try {
        // 配置客户端
        ClientConfig config;
        config.game_id = "example-game";
        config.env = "development";
        config.service_id = "basic-example";
        config.agent_addr = "127.0.0.1:19090";
        config.insecure = true;

        std::cout << "📋 配置信息:" << std::endl;
        std::cout << "   游戏ID: " << config.game_id << std::endl;
        std::cout << "   环境: " << config.env << std::endl;

        // 创建客户端
        CroupierClient client(config);

        // 注册简单函数
        FunctionDescriptor desc;
        desc.id = "example.hello";
        desc.version = "1.0.0";

        std::cout << "📝 注册函数: " << desc.id << std::endl;

        bool registered = client.RegisterFunction(desc, HelloHandler);
        if (!registered) {
            std::cerr << "❌ 函数注册失败!" << std::endl;
            return 1;
        }

        std::cout << "✅ 函数注册成功!" << std::endl;

        // 尝试连接到 Agent
        std::cout << "🔌 尝试连接到 Agent..." << std::endl;

        bool connected = client.Connect();
        if (connected) {
            std::cout << "✅ 成功连接到 Agent!" << std::endl;
            std::cout << "🚀 启动服务..." << std::endl;
            std::cout << "💡 在生产环境中，这里会保持运行等待调用" << std::endl;

            // 在示例中，我们不阻塞，而是模拟运行
            std::cout << "📡 模拟处理请求..." << std::endl;

            // 模拟处理一个请求
            std::string result = HelloHandler("example-context", R"({"test": "data"})");
            std::cout << "📤 处理结果: " << result << std::endl;

        } else {
            std::cout << "⚠️  无法连接到 Agent (这在没有Agent运行时是正常的)" << std::endl;
            std::cout << "💡 要完整测试，请确保 Croupier Agent 在 " << config.agent_addr << " 运行" << std::endl;
        }

        std::cout << "\n🎉 基础示例运行完成!" << std::endl;
        std::cout << "💡 查看 complete_example.cpp 了解更多高级功能" << std::endl;

    } catch (const std::exception& e) {
        std::cerr << "💥 程序异常: " << e.what() << std::endl;
        return 1;
    }

    return 0;
}