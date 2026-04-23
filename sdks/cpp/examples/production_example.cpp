/**
 * Production-Ready Example: Demonstrates best practices for production deployment
 *
 * This example shows:
 * 1. Graceful shutdown with signal handling
 * 2. Health check endpoint
 * 3. Structured logging
 * 4. Error handling with retry
 * 5. Configuration from environment variables
 * 6. Metrics collection
 * 7. TLS/mTLS configuration
 */

// Windows headers must be included before gRPC headers to avoid conflicts
#ifdef _WIN32
#ifndef WIN32_LEAN_AND_MEAN
#define WIN32_LEAN_AND_MEAN
#endif
#ifndef NOMINMAX
#define NOMINMAX
#endif
#include <windows.h>
#include <winsock2.h>
#include <ws2tcpip.h>
#include <winsvc.h>
#else
#include <arpa/inet.h>
#include <netinet/in.h>
#include <sys/socket.h>
#include <sys/types.h>
#include <unistd.h>
#endif

#include "croupier/sdk/croupier_client.h"
#include "croupier/sdk/logger.h"

#include <atomic>
#include <chrono>
#include <fstream>
#include <cstring>
#include <iostream>
#include <map>
#include <memory>
#include <mutex>
#include <signal.h>
#include <sstream>
#include <thread>

using namespace croupier::sdk;

// ==================== Global State ====================

std::atomic<bool> g_running(true);
std::atomic<bool> g_shutdown_requested(false);
std::unique_ptr<CroupierClient> g_client;
std::mutex g_metrics_mutex;

// ==================== Metrics ====================

struct Metrics {
    uint64_t requests_received = 0;
    uint64_t requests_processed = 0;
    uint64_t requests_failed = 0;
    uint64_t last_request_time = 0;

    void RecordRequest() {
        std::lock_guard<std::mutex> lock(g_metrics_mutex);
        requests_received++;
        last_request_time = std::chrono::system_clock::now().time_since_epoch().count();
    }

    void RecordSuccess() {
        std::lock_guard<std::mutex> lock(g_metrics_mutex);
        requests_processed++;
    }

    void RecordFailure() {
        std::lock_guard<std::mutex> lock(g_metrics_mutex);
        requests_failed++;
    }

    std::string ToJson() const {
        std::ostringstream ss;
        ss << "{"
           << "\"requests_received\":" << requests_received << ","
           << "\"requests_processed\":" << requests_processed << ","
           << "\"requests_failed\":" << requests_failed << "}";
        return ss.str();
    }
};

Metrics g_metrics;

// ==================== Configuration ====================

struct ProductionConfig {
    // Croupier SDK Configuration
    ClientConfig sdk_config;

    // 健康检查Configuration
    int health_check_port = 8080;
    std::string health_check_path = "/health";

    // 日志Configuration
    std::string log_level = "INFO";

    // 监控Configuration
    bool enable_metrics = true;
    int metrics_port = 9090;

    static ProductionConfig FromEnvironment() {
        ProductionConfig config;

        // SDK 基础Configuration
        config.sdk_config.game_id = GetEnv("CROUPIER_GAME_ID", "default-game");
        config.sdk_config.env = GetEnv("CROUPIER_ENV", "production");
        config.sdk_config.service_id = GetEnv("CROUPIER_SERVICE_ID", "game-service");
        config.sdk_config.agent_addr = GetEnv("CROUPIER_AGENT_ADDR", "127.0.0.1:19090");

        // TLS Configuration
        std::string insecure = GetEnv("CROUPIER_INSECURE", "false");
        config.sdk_config.insecure = (insecure == "true" || insecure == "1");

        if (!config.sdk_config.insecure) {
            config.sdk_config.cert_file = GetEnv("CROUPIER_CERT_FILE", "");
            config.sdk_config.key_file = GetEnv("CROUPIER_KEY_FILE", "");
            config.sdk_config.ca_file = GetEnv("CROUPIER_CA_FILE", "");
            config.sdk_config.server_name = GetEnv("CROUPIER_SERVER_NAME", "");
        }

        // 重连Configuration
        config.sdk_config.auto_reconnect = GetEnv("CROUPIER_AUTO_RECONNECT", "true") == "true";
        config.sdk_config.reconnect_interval_seconds = std::stoi(GetEnv("CROUPIER_RECONNECT_INTERVAL_SECONDS", "5"));
        config.sdk_config.reconnect_max_attempts = std::stoi(GetEnv("CROUPIER_RECONNECT_MAX_ATTEMPTS", "0"));

        // 健康检查Configuration
        config.health_check_port = std::stoi(GetEnv("HEALTH_CHECK_PORT", "8080"));

        // 日志Configuration
        config.log_level = GetEnv("LOG_LEVEL", "INFO");

        return config;
    }

private:
    static std::string GetEnv(const char* name, const char* default_value) {
        const char* value = std::getenv(name);
        return value ? value : default_value;
    }
};

// ==================== Signal Handling ====================

void SetupSignalHandlers() {
    auto handler = [](int signal) {
        if (g_shutdown_requested.exchange(true)) {
            // 第二次信号，强制退出
            std::cerr << "强制退出...\n";
            exit(1);
        }

        std::cout << "\n收到信号 " << signal << "，开始优雅Close...\n";
        g_running = false;

        // Set 30 秒超时后强制退出
        std::thread([]() {
            std::this_thread::sleep_for(std::chrono::seconds(30));
            std::cerr << "Close超时，强制退出\n";
            exit(1);
        }).detach();
    };

#ifdef _WIN32
    signal(SIGINT, handler);
    signal(SIGTERM, handler);
#else
    struct sigaction sa;
    std::memset(&sa, 0, sizeof(sa));
    sa.sa_handler = handler;
    sigemptyset(&sa.sa_mask);
    sigaction(SIGINT, &sa, nullptr);
    sigaction(SIGTERM, &sa, nullptr);
#endif
}

// ==================== Health Check Server ====================

class HealthCheckServer {
public:
    HealthCheckServer(int port) : port_(port), running_(false) {}

    bool Start() {
        running_ = true;

#ifdef _WIN32
        server_thread_ = std::thread([this]() { this->RunWindows(); });
#else
        server_thread_ = std::thread([this]() { this->RunUnix(); });
#endif

        return true;
    }

    void Stop() {
        running_ = false;
#ifdef _WIN32
        // Windows shutdown logic
#else
        // Close socket to unblock accept
        if (server_fd_ >= 0) {
            shutdown(server_fd_, SHUT_RDWR);
            close(server_fd_);
        }
#endif

        if (server_thread_.joinable()) {
            server_thread_.join();
        }
    }

private:
    int port_;
    std::atomic<bool> running_;
    std::thread server_thread_;
    int server_fd_ = -1;

#ifdef _WIN32
    void RunWindows() {
        // Windows implementation using Winsock
        // (simplified for example)
        while (running_) {
            std::this_thread::sleep_for(std::chrono::seconds(1));
        }
    }
#else
    void RunUnix() {
        int server_fd;
        struct sockaddr_in address;
        int opt = 1;

        // Create socket
        if ((server_fd = socket(AF_INET, SOCK_STREAM, 0)) == 0) {
            std::cerr << "Socket CreateFailed\n";
            return;
        }

        server_fd_ = server_fd;

        // Set socket 选项
        if (setsockopt(server_fd, SOL_SOCKET, SO_REUSEADDR | SO_REUSEPORT, &opt, sizeof(opt))) {
            std::cerr << "setsockopt Failed\n";
            return;
        }

        address.sin_family = AF_INET;
        address.sin_addr.s_addr = INADDR_ANY;
        address.sin_port = htons(port_);

        // 绑定
        if (bind(server_fd, (struct sockaddr*)&address, sizeof(address)) < 0) {
            std::cerr << "绑定Failed: port " << port_ << '\n';
            return;
        }

        // 监听
        if (listen(server_fd, 3) < 0) {
            std::cerr << "监听Failed\n";
            return;
        }

        std::cout << "健康检查ServiceStart在端口 " << port_ << '\n';

        while (running_) {
            fd_set read_fds;
            FD_ZERO(&read_fds);
            FD_SET(server_fd, &read_fds);

            struct timeval timeout;
            timeout.tv_sec = 1;
            timeout.tv_usec = 0;

            int activity = select(server_fd + 1, &read_fds, nullptr, nullptr, &timeout);

            if (activity < 0 && errno != EINTR) {
                break;
            }

            if (activity == 0) {
                continue;  // 超时，继续循环
            }

            if (FD_ISSET(server_fd, &read_fds)) {
                struct sockaddr_in client_addr;
                socklen_t addr_len = sizeof(client_addr);

                int client_fd = accept(server_fd, (struct sockaddr*)&client_addr, &addr_len);
                if (client_fd < 0) {
                    continue;
                }

                HandleRequest(client_fd);
                close(client_fd);
            }
        }

        close(server_fd);
    }
#endif

    void HandleRequest(int client_fd) {
        char buffer[1024] = {0};
#ifdef _WIN32
        int bytes_read = recv(client_fd, buffer, (int)(sizeof(buffer) - 1), 0);
#else
        ssize_t bytes_read = read(client_fd, buffer, sizeof(buffer) - 1);
#endif
        if (bytes_read > 0) {
            buffer[bytes_read] = 0;
        }

        std::string path = "/";
        std::string method = "GET";

        // 简单解析 HTTP 请求
        std::string request(buffer);
        size_t method_end = request.find(' ');
        if (method_end != std::string::npos) {
            method = request.substr(0, method_end);
            size_t path_end = request.find(' ', method_end + 1);
            if (path_end != std::string::npos) {
                path = request.substr(method_end + 1, path_end - method_end - 1);
            }
        }

        std::string response;
        if (path == "/health") {
            // Check if client exists and is connected
            bool is_healthy = g_client && g_client->IsConnected();
            response = BuildHealthResponse(is_healthy);
        } else if (path == "/ready") {
            // Check if client exists and is ready (connected)
            bool is_ready = g_client && g_client->IsConnected();
            response = BuildReadyResponse(is_ready);
        } else if (path == "/metrics") {
            response = BuildMetricsResponse();
        } else {
            response = BuildNotFoundResponse();
        }

        send(client_fd, response.c_str(), response.length(), 0);
    }

    std::string BuildHealthResponse(bool healthy) {
        int status = healthy ? 200 : 503;
        std::string body = healthy ? "{\"status\":\"healthy\"}" : "{\"status\":\"unhealthy\"}";

        return "HTTP/1.1 " + std::to_string(status) +
               " OK\r\n"
               "Content-Type: application/json\r\n"
               "Content-Length: " +
               std::to_string(body.length()) +
               "\r\n"
               "\r\n" +
               body;
    }

    std::string BuildReadyResponse(bool ready) {
        int status = ready ? 200 : 503;
        std::string body = ready ? "{\"status\":\"ready\"}" : "{\"status\":\"not_ready\"}";

        return "HTTP/1.1 " + std::to_string(status) +
               " OK\r\n"
               "Content-Type: application/json\r\n"
               "Content-Length: " +
               std::to_string(body.length()) +
               "\r\n"
               "\r\n" +
               body;
    }

    std::string BuildMetricsResponse() {
        std::string body = g_metrics.ToJson();

        return "HTTP/1.1 200 OK\r\n"
               "Content-Type: application/json\r\n"
               "Content-Length: " +
               std::to_string(body.length()) +
               "\r\n"
               "\r\n" +
               body;
    }

    std::string BuildNotFoundResponse() {
        std::string body = "{\"error\":\"not_found\"}";

        return "HTTP/1.1 404 Not Found\r\n"
               "Content-Type: application/json\r\n"
               "Content-Length: " +
               std::to_string(body.length()) +
               "\r\n"
               "\r\n" +
               body;
    }
};

// ==================== Business Logic Handlers ====================

std::string PlayerBanHandler(const std::string& context, const std::string& payload) {
    g_metrics.RecordRequest();

    try {
        CROUPIER_LOG_INFO("PlayerBanHandler", std::string("HandlerPlayerBan请求: ") + payload);

        // SimulateHandler
        std::this_thread::sleep_for(std::chrono::milliseconds(10));

        g_metrics.RecordSuccess();

        return R"({"status":"success","action":"ban","timestamp":")" + std::to_string(std::time(nullptr)) + R"("})";

    } catch (const std::exception& e) {
        g_metrics.RecordFailure();
        CROUPIER_LOG_ERROR("PlayerBanHandler", std::string("PlayerBanHandlerFailed: ") + e.what());
        throw;
    }
}

std::string PlayerGetHandler(const std::string& context, const std::string& payload) {
    g_metrics.RecordRequest();

    try {
        CROUPIER_LOG_INFO("PlayerGetHandler", std::string("GetPlayerInfo: ") + payload);

        // 实际业务逻辑 - 从数据库QueryPlayerInfo
        // 这里返回Simulate数据

        g_metrics.RecordSuccess();

        return R"({
            "status":"success",
            "player": {
                "id":"player_123",
                "name":"TestPlayer",
                "level":50,
                "exp":125000,
                "last_login":")" +
               std::to_string(std::time(nullptr)) + R"("
            }
        })";

    } catch (const std::exception& e) {
        g_metrics.RecordFailure();
        CROUPIER_LOG_ERROR("PlayerGetHandler", std::string("GetPlayerInfoFailed: ") + e.what());
        throw;
    }
}

std::string WalletTransferHandler(const std::string& context, const std::string& payload) {
    g_metrics.RecordRequest();

    try {
        CROUPIER_LOG_INFO("WalletTransferHandler", std::string("Handler钱包转账: ") + payload);

        // 解析 payload
        // 实际场景应使用 JSON 库解析
        // {"from":"player_123","to":"player_456","amount":100,"currency":"gold"}

        // Execute转账逻辑（带事务）
        // 1. Validation发送者余额
        // 2. 扣除发送者余额
        // 3. 增加接收者余额
        // 4. 记录交易日志

        g_metrics.RecordSuccess();

        return R"({
            "status":"success",
            "transaction_id":"tx_")" +
               std::to_string(std::time(nullptr)) + R"(",
            "from":"player_123",
            "to":"player_456",
            "amount":100,
            "currency":"gold"
        })";

    } catch (const std::exception& e) {
        g_metrics.RecordFailure();
        CROUPIER_LOG_ERROR("WalletTransferHandler", std::string("钱包转账Failed: ") + e.what());
        throw;
    }
}

// ==================== Main Application ====================

int main(int argc, char* argv[]) {
    std::cout << "=============================================\n";
    std::cout << "Croupier C++ SDK 生产环境Sample\n";
    std::cout << "=============================================\n\n";

    // 1. LoadConfiguration
    ProductionConfig config = ProductionConfig::FromEnvironment();

    std::cout << "ConfigurationLoadComplete:\n";
    std::cout << "  游戏 ID: " << config.sdk_config.game_id << '\n';
    std::cout << "  环境: " << config.sdk_config.env << '\n';
    std::cout << "  Agent Address: " << config.sdk_config.agent_addr << '\n';
    std::cout << "  TLS: " << (config.sdk_config.insecure ? "禁用" : "启用") << '\n';
    std::cout << "  健康检查端口: " << config.health_check_port << '\n';
    std::cout << '\n';

    // 2. Set信号Handler
    SetupSignalHandlers();

    // 3. CreateClient
    g_client = std::make_unique<CroupierClient>(config.sdk_config);

    // 4. RegisterFunction
    std::cout << "Register游戏Function...\n";

    FunctionDescriptor banDesc;
    banDesc.id = "player.ban";
    banDesc.version = "1.0.0";
    banDesc.category = "moderation";
    banDesc.risk = "high";
    banDesc.enabled = true;

    if (!g_client->RegisterFunction(banDesc, PlayerBanHandler)) {
        std::cerr << "Register player.ban Failed\n";
        return 1;
    }
    std::cout << "  ✓ player.ban\n";

    FunctionDescriptor getDesc;
    getDesc.id = "player.get";
    getDesc.version = "1.0.0";
    getDesc.category = "player";
    getDesc.risk = "low";
    getDesc.enabled = true;

    if (!g_client->RegisterFunction(getDesc, PlayerGetHandler)) {
        std::cerr << "Register player.get Failed\n";
        return 1;
    }
    std::cout << "  ✓ player.get\n";

    FunctionDescriptor transferDesc;
    transferDesc.id = "wallet.transfer";
    transferDesc.version = "1.0.0";
    transferDesc.category = "economy";
    transferDesc.risk = "high";
    transferDesc.enabled = true;

    if (!g_client->RegisterFunction(transferDesc, WalletTransferHandler)) {
        std::cerr << "Register wallet.transfer Failed\n";
        return 1;
    }
    std::cout << "  ✓ wallet.transfer\n\n";

    // 5. Start健康检查Service
    HealthCheckServer health_server(config.health_check_port);
    if (!health_server.Start()) {
        std::cerr << "Start健康检查ServiceFailed\n";
        return 1;
    }

    // 6. Connect到 Agent
    std::cout << "Connect到 Agent...\n";
    if (!g_client->Connect()) {
        std::cerr << "ConnectFailed，检查 Agent 是否运行\n";
        health_server.Stop();
        return 1;
    }
    std::cout << "  ✓ 已Connect\n";
    std::cout << "\n";

    // 7. StartService（在单独线程中）
    std::cout << "StartService...\n";
    std::thread service_thread([]() {
        try {
            g_client->Serve();
        } catch (const std::exception& e) {
            std::cerr << "ServiceException: " << e.what() << '\n';
        }
    });

    std::cout << "\n=============================================\n";
    std::cout << "Service已Start，等待请求...\n";
    std::cout << "健康检查: http://localhost:" << config.health_check_port << "/health\n";
    std::cout << "指标端点: http://localhost:" << config.health_check_port << "/metrics\n";
    std::cout << "按 Ctrl+C 优雅Close\n";
    std::cout << "=============================================\n\n";

    // 8. 主循环 - 等待Close信号
    while (g_running) {
        std::this_thread::sleep_for(std::chrono::seconds(1));

        // 定期输出指标（每 60 秒）
        static int counter = 0;
        if (++counter >= 60) {
            counter = 0;
            std::cout << "指标: " << g_metrics.ToJson() << '\n';
        }
    }

    // 9. 优雅Close
    std::cout << "\n开始优雅Close...\n";

    std::cout << "  StopClientService...\n";
    g_client->Stop();

    std::cout << "  等待Service线程结束...\n";
    if (service_thread.joinable()) {
        service_thread.join();
    }

    std::cout << "  Stop健康检查Service...\n";
    health_server.Stop();

    std::cout << "  CloseClient...\n";
    g_client->Close();

    std::cout << "\n=============================================\n";
    std::cout << "Service已Close\n";
    std::cout << "最终指标: " << g_metrics.ToJson() << '\n';
    std::cout << "=============================================\n";

    return 0;
}
