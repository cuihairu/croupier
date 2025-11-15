#include "croupier/sdk/croupier_client.h"
#include "croupier/sdk/config/client_config_loader.h"
#include "croupier/sdk/utils/file_utils.h"
#include <iostream>
#include <exception>

using namespace croupier::sdk;
using namespace croupier::sdk::config;
using namespace croupier::sdk::utils;

int main(int argc, char* argv[]) {
    std::cout << "🚀 Croupier C++ SDK - Advanced Configuration Example" << std::endl;
    std::cout << "====================================================" << std::endl;

    try {
        // Initialize configuration loader
        ClientConfigLoader config_loader;

        // Determine configuration file based on command line argument or environment
        std::string config_file;
        std::string environment = "development";

        if (argc > 1) {
            environment = argv[1];
            std::cout << "Using environment from command line: " << environment << std::endl;
        } else {
            const char* env_var = std::getenv("CROUPIER_ENV");
            if (env_var) {
                environment = env_var;
                std::cout << "Using environment from CROUPIER_ENV: " << environment << std::endl;
            } else {
                std::cout << "Using default environment: " << environment << std::endl;
            }
        }

        config_file = "./configs/examples/" + environment + ".json";

        // Check if config file exists
        if (!FileSystemUtils::FileExists(config_file)) {
            std::cout << "⚠️ Configuration file not found: " << config_file << std::endl;
            std::cout << "📄 Generating example configuration..." << std::endl;

            // Generate example configuration
            std::string example_config = config_loader.GenerateExampleConfig(environment);

            // Create directory if it doesn't exist
            std::string config_dir = FileSystemUtils::GetDirectoryPath(config_file);
            if (!config_dir.empty()) {
                FileSystemUtils::CreateDirectory(config_dir);
            }

            // Write example config
            if (FileSystemUtils::WriteFileContent(config_file, example_config)) {
                std::cout << "✅ Example configuration written to: " << config_file << std::endl;
                std::cout << "📝 Please edit the configuration file with your actual settings" << std::endl;
            } else {
                std::cout << "❌ Failed to write configuration file" << std::endl;
                return 1;
            }
        }

        // Load configuration with environment variable overrides
        std::cout << "📂 Loading configuration from: " << config_file << std::endl;
        ClientConfig config = config_loader.LoadWithEnvironmentOverrides(config_file, "CROUPIER_");

        // Validate configuration
        std::cout << "✅ Validating configuration..." << std::endl;
        auto validation_errors = config_loader.ValidateConfig(config);

        if (!validation_errors.empty()) {
            std::cout << "❌ Configuration validation failed with " << validation_errors.size() << " errors:" << std::endl;
            for (const auto& error : validation_errors) {
                std::cout << "  - " << error << std::endl;
            }
            return 1;
        }

        std::cout << "✅ Configuration validation passed!" << std::endl;

        // Display loaded configuration
        std::cout << "\n📋 Loaded Configuration:" << std::endl;
        std::cout << "  Game ID:      " << config.game_id << std::endl;
        std::cout << "  Environment:  " << config.env << std::endl;
        std::cout << "  Service ID:   " << config.service_id << std::endl;
        std::cout << "  Agent Addr:   " << config.agent_addr << std::endl;
        std::cout << "  Local Listen: " << config.local_listen << std::endl;
        std::cout << "  Insecure:     " << (config.insecure ? "true" : "false") << std::endl;
        std::cout << "  Timeout:      " << config.timeout_seconds << "s" << std::endl;

        if (!config.insecure) {
            std::cout << "  TLS Cert:     " << config.cert_file << std::endl;
            std::cout << "  TLS Key:      " << config.key_file << std::endl;
            std::cout << "  CA File:      " << config.ca_file << std::endl;
            std::cout << "  Server Name:  " << config.server_name << std::endl;
        }

        if (!config.auth_token.empty()) {
            std::cout << "  Auth Token:   " << config.auth_token.substr(0, 20) << "..." << std::endl;
        }

        if (!config.headers.empty()) {
            std::cout << "  Headers:" << std::endl;
            for (const auto& [key, value] : config.headers) {
                std::cout << "    " << key << ": " << value << std::endl;
            }
        }

        // Create and configure client
        std::cout << "\n🔧 Creating Croupier client..." << std::endl;
        CroupierClient client(config);

        // Register a simple function
        FunctionDescriptor func_desc;
        func_desc.id = "example.hello";
        func_desc.version = "1.0.0";

        auto hello_handler = [](const std::string& context, const std::string& payload) -> std::string {
            std::cout << "📞 Hello function called!" << std::endl;
            std::cout << "   Context: " << context << std::endl;
            std::cout << "   Payload: " << payload << std::endl;

            return R"({
                "message": "Hello from advanced configuration example!",
                "timestamp": ")" + std::to_string(std::time(nullptr)) + R"(",
                "status": "success",
                "config_loaded": true
            })";
        };

        std::cout << "📝 Registering function: " << func_desc.id << std::endl;
        bool registered = client.RegisterFunction(func_desc, hello_handler);

        if (!registered) {
            std::cout << "❌ Failed to register function!" << std::endl;
            return 1;
        }

        std::cout << "✅ Function registered successfully!" << std::endl;

        // Try to connect to Agent
        std::cout << "\n🔌 Attempting to connect to Croupier Agent..." << std::endl;
        bool connected = client.Connect();

        if (connected) {
            std::cout << "✅ Successfully connected to Croupier Agent!" << std::endl;
            std::cout << "🎯 Client is now ready to handle function calls" << std::endl;
            std::cout << "📡 Local service address: " << client.GetLocalAddress() << std::endl;

            std::cout << "\n💡 In a production environment, call client.Serve() to start handling requests" << std::endl;
            std::cout << "💡 For this example, we'll simulate a function call..." << std::endl;

            // Simulate function call
            std::string result = hello_handler("example-context", R"({"test": "data", "example": true})");
            std::cout << "📤 Function result: " << result << std::endl;

        } else {
            std::cout << "⚠️ Could not connect to Croupier Agent" << std::endl;
            std::cout << "💡 This is normal if no Agent is running at " << config.agent_addr << std::endl;
            std::cout << "💡 The client is configured correctly and ready to connect when an Agent is available" << std::endl;
        }

        std::cout << "\n🎉 Advanced configuration example completed successfully!" << std::endl;
        std::cout << "💡 Configuration file: " << config_file << std::endl;
        std::cout << "💡 Try running with different environments:" << std::endl;
        std::cout << "   ./croupier-config-example development" << std::endl;
        std::cout << "   ./croupier-config-example staging" << std::endl;
        std::cout << "   ./croupier-config-example production" << std::endl;

    } catch (const std::exception& e) {
        std::cout << "💥 Error: " << e.what() << std::endl;
        return 1;
    }

    return 0;
}