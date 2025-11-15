#pragma once

#include "croupier/sdk/croupier_client.h"
#include <memory>
#include <atomic>
#include <thread>
#include <mutex>
#include <condition_variable>

// Only include gRPC headers when gRPC is enabled
#ifdef CROUPIER_SDK_ENABLE_GRPC
#include <grpc++/grpc++.h>
#endif

namespace croupier {
namespace sdk {
namespace grpc_service {

// Connection state enum (available regardless of gRPC)
enum class ConnectionState {
    DISCONNECTED,
    CONNECTING,
    CONNECTED,
    RECONNECTING,
    ERROR
};

#ifdef CROUPIER_SDK_ENABLE_GRPC

// Forward declarations for gRPC-enabled build
class LocalControlServiceStub;
class LocalFunctionServiceImpl;

/**
 * @brief gRPC Client Manager
 * Manages gRPC communication with Croupier Agent
 */
class GrpcClientManager {
public:
    explicit GrpcClientManager(const ClientConfig& config);
    ~GrpcClientManager();

    // 连接管理
    bool Connect();
    void Disconnect();
    bool IsConnected() const;
    ConnectionState GetState() const;

    // Registration functionality - updated to match proto
    bool RegisterWithAgent(
        const std::vector<LocalFunctionDescriptor>& functions,
        std::string& session_id
    );

    // Send agent registration to server (second layer)
    bool RegisterAgentWithServer(
        const std::vector<FunctionDescriptor>& functions,
        std::string& session_id
    );

    // 心跳管理
    bool SendHeartbeat(const std::string& session_id);
    void StartHeartbeatLoop(const std::string& session_id);
    void StopHeartbeatLoop();

    // 本地服务器管理
    bool StartLocalServer();
    void StopLocalServer();
    std::string GetLocalServerAddress() const;

    // 错误处理
    void SetErrorCallback(std::function<void(const std::string&)> callback);
    void SetReconnectCallback(std::function<void()> callback);

private:
    // 配置
    ClientConfig config_;

    // gRPC 组件
    std::shared_ptr<grpc::Channel> agent_channel_;
    std::unique_ptr<LocalControlServiceStub> agent_stub_;
    std::unique_ptr<grpc::Server> local_server_;
    std::unique_ptr<LocalFunctionServiceImpl> local_service_;

    // 状态管理
    std::atomic<ConnectionState> state_;
    std::atomic<bool> should_reconnect_;
    std::atomic<bool> heartbeat_running_;

    // 线程管理
    std::thread heartbeat_thread_;
    std::thread reconnect_thread_;
    std::mutex state_mutex_;
    std::condition_variable state_cv_;

    // 回调
    std::function<void(const std::string&)> error_callback_;
    std::function<void()> reconnect_callback_;

    // 本地服务器信息
    std::string local_address_;
    int local_port_;

    // 内部方法
    void DoConnect();
    void DoReconnect();
    void HeartbeatLoop(const std::string& session_id);
    std::shared_ptr<grpc::Channel> CreateChannel();
    bool ValidateConnection();
    void HandleError(const std::string& error);
    void NotifyReconnect();

    // gRPC 配置
    grpc::ChannelArguments CreateChannelArguments();
    std::shared_ptr<grpc::ChannelCredentials> CreateCredentials();
};

/**
 * @brief 本地函数服务实现
 * 接收来自 Agent 的函数调用请求
 */
class LocalFunctionServiceImpl {
public:
    explicit LocalFunctionServiceImpl(
        const std::map<std::string, FunctionHandler>& handlers
    );

    // 更新处理器映射
    void UpdateHandlers(const std::map<std::string, FunctionHandler>& handlers);

    // 添加处理器
    void AddHandler(const std::string& function_id, FunctionHandler handler);

    // 移除处理器
    void RemoveHandler(const std::string& function_id);

    // 获取处理器数量
    size_t GetHandlerCount() const;

    // gRPC 服务方法 - 这些将在实际的 proto 生成代码中实现
    // grpc::Status InvokeFunction(...)
    // grpc::Status StreamFunction(...)
    // grpc::Status CancelJob(...)

private:
    std::map<std::string, FunctionHandler> handlers_;
    std::shared_mutex handlers_mutex_;

    // 统计信息
    std::atomic<uint64_t> total_calls_;
    std::atomic<uint64_t> successful_calls_;
    std::atomic<uint64_t> failed_calls_;

    // 执行函数调用
    std::string ExecuteHandler(
        const std::string& function_id,
        const std::string& context,
        const std::string& payload
    );
};

/**
 * @brief Agent 服务的客户端封装
 */
class LocalControlServiceStub {
public:
    explicit LocalControlServiceStub(std::shared_ptr<grpc::Channel> channel);

    // Register service with agent (LocalControlService)
    bool RegisterLocal(
        const std::string& service_id,
        const std::string& version,
        const std::string& rpc_addr,
        const std::vector<LocalFunctionDescriptor>& functions,
        std::string& session_id,
        std::string& error_message
    );

    // 发送心跳
    bool Heartbeat(
        const std::string& service_id,
        const std::string& session_id,
        std::string& error_message
    );

    // 列出本地函数
    bool ListLocal(
        std::vector<FunctionDescriptor>& functions,
        std::string& error_message
    );

    // 注销服务
    bool UnregisterLocal(
        const std::string& service_id,
        const std::string& session_id,
        std::string& error_message
    );

private:
    std::shared_ptr<grpc::Channel> channel_;
    // 实际的 stub 将在 proto 生成后创建
    // std::unique_ptr<LocalControlService::Stub> stub_;

    // 超时配置
    std::chrono::milliseconds default_timeout_;

    // 创建上下文
    std::unique_ptr<grpc::ClientContext> CreateContext();
};

/**
 * @brief gRPC Connection Options
 */
struct GrpcConnectionOptions {
    // Connection timeout
    std::chrono::seconds connect_timeout{30};

    // Heartbeat interval
    std::chrono::seconds heartbeat_interval{60};

    // Reconnect interval
    std::chrono::seconds reconnect_interval{5};

    // Maximum reconnect attempts
    int max_reconnect_attempts{10};

    // gRPC options
    std::map<std::string, std::string> channel_args;

    // TLS options
    bool use_tls{false};
    std::string cert_file;
    std::string key_file;
    std::string ca_file;
    std::string server_name_override;
};

#else // !CROUPIER_SDK_ENABLE_GRPC

// Mock implementations when gRPC is disabled
class GrpcClientManager {
public:
    explicit GrpcClientManager(const ClientConfig& config) : config_(config) {}
    ~GrpcClientManager() = default;

    // Connection management
    bool Connect() { return false; }
    void Disconnect() {}
    bool IsConnected() const { return false; }
    ConnectionState GetState() const { return ConnectionState::DISCONNECTED; }

    // Registration functionality
    bool RegisterWithAgent(
        const std::vector<LocalFunctionDescriptor>& functions,
        std::string& session_id
    ) {
        (void)functions;
        (void)session_id;
        return false;
    }

    // Send agent registration to server (second layer)
    bool RegisterAgentWithServer(
        const std::vector<FunctionDescriptor>& functions,
        std::string& session_id
    ) {
        (void)functions;
        (void)session_id;
        return false;
    }

    // Heartbeat management
    bool SendHeartbeat(const std::string& session_id) {
        (void)session_id;
        return false;
    }
    void StartHeartbeatLoop(const std::string& session_id) {
        (void)session_id;
    }
    void StopHeartbeatLoop() {}

    // Local server management
    bool StartLocalServer() { return false; }
    void StopLocalServer() {}
    std::string GetLocalServerAddress() const { return ""; }

    // Error handling
    void SetErrorCallback(std::function<void(const std::string&)> callback) {
        (void)callback;
    }
    void SetReconnectCallback(std::function<void()> callback) {
        (void)callback;
    }

private:
    ClientConfig config_;
};

class LocalFunctionServiceImpl {
public:
    explicit LocalFunctionServiceImpl(
        const std::map<std::string, FunctionHandler>& handlers
    ) : handlers_(handlers) {}

    void UpdateHandlers(const std::map<std::string, FunctionHandler>& handlers) {
        (void)handlers;
    }
    void AddHandler(const std::string& function_id, FunctionHandler handler) {
        (void)function_id;
        (void)handler;
    }
    void RemoveHandler(const std::string& function_id) {
        (void)function_id;
    }
    size_t GetHandlerCount() const { return 0; }

private:
    std::map<std::string, FunctionHandler> handlers_;
};

class LocalControlServiceStub {
public:
    explicit LocalControlServiceStub(void* channel) {
        (void)channel; // void* instead of grpc::Channel
    }

    bool RegisterLocal(
        const std::string& service_id,
        const std::string& version,
        const std::string& rpc_addr,
        const std::vector<LocalFunctionDescriptor>& functions,
        std::string& session_id,
        std::string& error_message
    ) {
        (void)service_id;
        (void)version;
        (void)rpc_addr;
        (void)functions;
        (void)session_id;
        (void)error_message;
        return false;
    }

    bool Heartbeat(
        const std::string& service_id,
        const std::string& session_id,
        std::string& error_message
    ) {
        (void)service_id;
        (void)session_id;
        (void)error_message;
        return false;
    }

    bool ListLocal(
        std::vector<FunctionDescriptor>& functions,
        std::string& error_message
    ) {
        (void)functions;
        (void)error_message;
        return false;
    }

    bool UnregisterLocal(
        const std::string& service_id,
        const std::string& session_id,
        std::string& error_message
    ) {
        (void)service_id;
        (void)session_id;
        (void)error_message;
        return false;
    }
};

struct GrpcConnectionOptions {
    std::chrono::seconds connect_timeout{30};
    std::chrono::seconds heartbeat_interval{60};
    std::chrono::seconds reconnect_interval{5};
    int max_reconnect_attempts{10};
    std::map<std::string, std::string> channel_args;
    bool use_tls{false};
    std::string cert_file;
    std::string key_file;
    std::string ca_file;
    std::string server_name_override;
};

#endif // CROUPIER_SDK_ENABLE_GRPC

} // namespace grpc_service
} // namespace sdk
} // namespace croupier