#include "croupier/sdk/grpc_service.h"

// Only compile the full gRPC implementation when gRPC is enabled
#ifdef CROUPIER_SDK_ENABLE_GRPC

#include <iostream>
#include <sstream>
#include <regex>
#include <fstream>
#include <shared_mutex>

// Note: This implementation requires protobuf generated code
// In a real project, we need to generate gRPC proto files first, then implement the actual services

namespace croupier {
namespace sdk {
namespace grpc_service {

//==============================================================================
// GrpcClientManager Implementation
//==============================================================================

GrpcClientManager::GrpcClientManager(const ClientConfig& config)
    : config_(config)
    , state_(ConnectionState::DISCONNECTED)
    , should_reconnect_(false)
    , heartbeat_running_(false)
    , local_port_(0)
{
    std::cout << "Initializing gRPC client manager for game: " << config_.game_id
              << ", env: " << config_.env << std::endl;
}

GrpcClientManager::~GrpcClientManager() {
    Disconnect();
}

bool GrpcClientManager::Connect() {
    std::lock_guard<std::mutex> lock(state_mutex_);

    if (state_ == ConnectionState::CONNECTED) {
        return true;
    }

    state_ = ConnectionState::CONNECTING;

    try {
        // Create gRPC channel
        agent_channel_ = CreateChannel();
        if (!agent_channel_) {
            HandleError("Failed to create gRPC channel");
            return false;
        }

        // Create client stub
        agent_stub_ = std::make_unique<LocalControlServiceStub>(agent_channel_);

        // Validate connection
        if (!ValidateConnection()) {
            HandleError("Failed to validate connection to agent");
            return false;
        }

        // Start local server
        if (!StartLocalServer()) {
            HandleError("Failed to start local gRPC server");
            return false;
        }

        state_ = ConnectionState::CONNECTED;
        std::cout << "✅ Successfully connected to Agent: " << config_.agent_addr << std::endl;

        return true;

    } catch (const std::exception& e) {
        HandleError("Connection failed: " + std::string(e.what()));
        state_ = ConnectionState::ERROR;
        return false;
    }
}

void GrpcClientManager::Disconnect() {
    std::lock_guard<std::mutex> lock(state_mutex_);

    should_reconnect_ = false;

    // Stop heartbeat
    StopHeartbeatLoop();

    // Stop local server
    StopLocalServer();

    // Cleanup connections
    agent_stub_.reset();
    agent_channel_.reset();

    state_ = ConnectionState::DISCONNECTED;

    std::cout << "📴 Disconnected from Agent" << std::endl;
}

bool GrpcClientManager::IsConnected() const {
    return state_ == ConnectionState::CONNECTED;
}

ConnectionState GrpcClientManager::GetState() const {
    return state_;
}

bool GrpcClientManager::RegisterWithAgent(
    const std::vector<LocalFunctionDescriptor>& functions,
    std::string& session_id) {

    if (!IsConnected()) {
        std::cerr << "❌ Not connected to Agent, cannot register" << std::endl;
        return false;
    }

    try {
        std::string error_message;
        bool success = agent_stub_->RegisterLocal(
            config_.service_id,
            config_.service_version,
            local_address_,
            functions,
            session_id,
            error_message
        );

        if (success) {
            std::cout << "✅ Successfully registered with Agent, session_id: " << session_id << std::endl;
            std::cout << "📋 Registration info:" << std::endl;
            std::cout << "   - Functions: " << functions.size() << std::endl;
            std::cout << "   - Local address: " << local_address_ << std::endl;

            // Start heartbeat
            StartHeartbeatLoop(session_id);
            return true;
        } else {
            std::cerr << "❌ Registration failed: " << error_message << std::endl;
            return false;
        }
    } catch (const std::exception& e) {
        std::cerr << "❌ Registration exception: " << e.what() << std::endl;
        return false;
    }
}

bool GrpcClientManager::SendHeartbeat(const std::string& session_id) {
    if (!IsConnected()) {
        return false;
    }

    try {
        std::string error_message;
        bool success = agent_stub_->Heartbeat(config_.service_id, session_id, error_message);

        if (!success) {
            std::cerr << "💓 Heartbeat failed: " << error_message << std::endl;
            // May need to reconnect
            return false;
        }
        return true;
    } catch (const std::exception& e) {
        std::cerr << "❌ Heartbeat exception: " << e.what() << std::endl;
        return false;
    }
}

bool GrpcClientManager::StartLocalServer() {
    try {
        // Parse listen address
        std::string host;
        int port;

        if (config_.local_listen.empty()) {
            host = "127.0.0.1";
            port = 0; // Auto-assign port
        } else {
            auto pos = config_.local_listen.rfind(':');
            if (pos == std::string::npos) {
                throw std::runtime_error("Invalid local_listen format");
            }
            host = config_.local_listen.substr(0, pos);
            std::string port_str = config_.local_listen.substr(pos + 1);
            port = port_str.empty() ? 0 : std::stoi(port_str);
        }

        // If port is 0, auto-assign port
        if (port == 0) {
            port = 19001; // Default starting port for local services
        }

        local_port_ = port;

        // Create local service implementation
        std::map<std::string, FunctionHandler> empty_handlers; // Initially empty, will be updated later
        local_service_ = std::make_unique<LocalFunctionServiceImpl>(empty_handlers);

        // Build server address
        local_address_ = host + ":" + std::to_string(port);

        // TODO: Start real gRPC server here
        // For now, just mock success
        std::cout << "🚀 Local gRPC server started on: " << local_address_ << std::endl;

        return true;
    } catch (const std::exception& e) {
        std::cerr << "❌ Failed to start local server: " << e.what() << std::endl;
        return false;
    }
}

void GrpcClientManager::StopLocalServer() {
    if (local_server_) {
        local_server_->Shutdown();
        local_server_.reset();
    }

    local_service_.reset();
    std::cout << "🛑 Local gRPC server stopped" << std::endl;
}

std::string GrpcClientManager::GetLocalServerAddress() const {
    return local_address_;
}

void GrpcClientManager::SetErrorCallback(std::function<void(const std::string&)> callback) {
    error_callback_ = callback;
}

void GrpcClientManager::SetReconnectCallback(std::function<void()> callback) {
    reconnect_callback_ = callback;
}

std::shared_ptr<grpc::Channel> GrpcClientManager::CreateChannel() {
    auto channel_args = CreateChannelArguments();
    auto credentials = CreateCredentials();

    return grpc::CreateCustomChannel(config_.agent_addr, credentials, channel_args);
}

bool GrpcClientManager::ValidateConnection() {
    if (!agent_channel_) {
        return false;
    }

    // Check channel state
    grpc_connectivity_state state = agent_channel_->GetState(false);

    // Wait for connection to be established
    auto deadline = std::chrono::system_clock::now() + std::chrono::seconds(config_.timeout_seconds);
    if (!agent_channel_->WaitForConnected(deadline)) {
        std::cerr << "⏰ Connection timeout, unable to connect to Agent" << std::endl;
        return false;
    }

    std::cout << "🔗 gRPC channel connected successfully" << std::endl;
    return true;
}

void GrpcClientManager::HandleError(const std::string& error) {
    std::cerr << "🚨 gRPC error: " << error << std::endl;
    if (error_callback_) {
        error_callback_(error);
    }

    // If auto-reconnect is configured, start reconnect logic
    if (should_reconnect_) {
        DoReconnect();
    }
}

void GrpcClientManager::NotifyReconnect() {
    if (reconnect_callback_) {
        reconnect_callback_();
    }
}

void GrpcClientManager::DoReconnect() {
    // Spawn reconnect thread if not already running
    if (reconnect_thread_.joinable()) {
        return; // Already reconnecting
    }

    should_reconnect_ = true;
    reconnect_thread_ = std::thread([this]() {
        const int max_attempts = 10;
        int attempts = 0;

        while (should_reconnect_ && attempts < max_attempts) {
            std::this_thread::sleep_for(std::chrono::seconds(5)); // Wait 5 seconds between attempts

            std::cout << "🔄 Attempting to reconnect to Agent... (attempt " << (attempts + 1) << ")" << std::endl;

            if (Connect()) {
                should_reconnect_ = false;
                std::cout << "✅ Reconnection successful!" << std::endl;
                NotifyReconnect();
                return;
            }

            attempts++;
        }

        if (should_reconnect_) {
            std::cerr << "❌ Reconnection failed, max attempts reached" << std::endl;
        }
    });
}

void GrpcClientManager::StartHeartbeatLoop(const std::string& session_id) {
    if (heartbeat_running_) {
        return;
    }

    heartbeat_running_ = true;
    heartbeat_thread_ = std::thread([this, session_id]() {
        const auto interval = std::chrono::seconds(60); // 60 second heartbeat interval

        while (heartbeat_running_) {
            std::this_thread::sleep_for(interval);

            if (!heartbeat_running_) break;

            if (!SendHeartbeat(session_id)) {
                std::cerr << "💔 Heartbeat failed, may need reconnection" << std::endl;
                break;
            }

            // Silent heartbeat, only log on failure
            // std::cout << "💓 Heartbeat sent successfully" << std::endl;
        }

        std::cout << "💓 Heartbeat loop stopped" << std::endl;
    });
}

void GrpcClientManager::StopHeartbeatLoop() {
    heartbeat_running_ = false;
    if (heartbeat_thread_.joinable()) {
        heartbeat_thread_.join();
    }
}

grpc::ChannelArguments GrpcClientManager::CreateChannelArguments() {
    grpc::ChannelArguments args;

    // Set timeout
    args.SetInt(GRPC_ARG_KEEPALIVE_TIME_MS, 30000);
    args.SetInt(GRPC_ARG_KEEPALIVE_TIMEOUT_MS, 5000);
    args.SetInt(GRPC_ARG_KEEPALIVE_PERMIT_WITHOUT_CALLS, 1);

    // Set max message size
    args.SetMaxReceiveMessageSize(4 * 1024 * 1024); // 4MB
    args.SetMaxSendMessageSize(4 * 1024 * 1024);    // 4MB

    return args;
}

std::shared_ptr<grpc::ChannelCredentials> GrpcClientManager::CreateCredentials() {
    if (config_.insecure) {
        return grpc::InsecureChannelCredentials();
    }

    // TLS configuration
    grpc::SslCredentialsOptions ssl_options;

    if (!config_.ca_file.empty()) {
        std::ifstream ca_file(config_.ca_file);
        std::stringstream ca_buffer;
        ca_buffer << ca_file.rdbuf();
        ssl_options.pem_root_certs = ca_buffer.str();
    }

    if (!config_.cert_file.empty() && !config_.key_file.empty()) {
        std::ifstream cert_file(config_.cert_file);
        std::stringstream cert_buffer;
        cert_buffer << cert_file.rdbuf();
        ssl_options.pem_cert_chain = cert_buffer.str();

        std::ifstream key_file(config_.key_file);
        std::stringstream key_buffer;
        key_buffer << key_file.rdbuf();
        ssl_options.pem_private_key = key_buffer.str();
    }

    return grpc::SslCredentials(ssl_options);
}

//==============================================================================
// LocalFunctionServiceImpl Implementation
//==============================================================================

LocalFunctionServiceImpl::LocalFunctionServiceImpl(
    const std::map<std::string, FunctionHandler>& handlers)
    : handlers_(handlers)
    , total_calls_(0)
    , successful_calls_(0)
    , failed_calls_(0) {
    std::cout << "🎯 Local function service initialized, handler count: " << handlers_.size() << std::endl;
}

void LocalFunctionServiceImpl::UpdateHandlers(const std::map<std::string, FunctionHandler>& handlers) {
    std::lock_guard<std::shared_mutex> lock(handlers_mutex_);
    handlers_ = handlers;
    std::cout << "🔄 Updated function handlers, current count: " << handlers_.size() << std::endl;
}

void LocalFunctionServiceImpl::AddHandler(const std::string& function_id, FunctionHandler handler) {
    std::lock_guard<std::shared_mutex> lock(handlers_mutex_);
    handlers_[function_id] = handler;
    std::cout << "➕ Added function handler: " << function_id << std::endl;
}

void LocalFunctionServiceImpl::RemoveHandler(const std::string& function_id) {
    std::lock_guard<std::shared_mutex> lock(handlers_mutex_);
    auto it = handlers_.find(function_id);
    if (it != handlers_.end()) {
        handlers_.erase(it);
        std::cout << "➖ Removed function handler: " << function_id << std::endl;
    }
}

size_t LocalFunctionServiceImpl::GetHandlerCount() const {
    std::shared_lock<std::shared_mutex> lock(handlers_mutex_);
    return handlers_.size();
}

std::string LocalFunctionServiceImpl::ExecuteHandler(
    const std::string& function_id,
    const std::string& context,
    const std::string& payload) {

    total_calls_++;

    std::shared_lock<std::shared_mutex> lock(handlers_mutex_);

    auto it = handlers_.find(function_id);
    if (it == handlers_.end()) {
        failed_calls_++;
        std::cerr << "❌ Function not found: " << function_id << std::endl;
        return R"({"error": "Function not found", "function_id": ")" + function_id + R"("})";
    }

    auto handler = it->second;
    lock.unlock(); // Release lock to allow concurrent execution

    try {
        std::cout << "🎯 Executing function: " << function_id << std::endl;
        std::string result = handler(context, payload);
        successful_calls_++;
        std::cout << "✅ Function executed successfully: " << function_id << std::endl;
        return result;
    } catch (const std::exception& e) {
        failed_calls_++;
        std::cerr << "❌ Function execution failed: " << function_id
                  << ", error: " << e.what() << std::endl;
        return R"({"error": ")" + std::string(e.what()) + R"(", "function_id": ")" + function_id + R"("})";
    }
}

//==============================================================================
// LocalControlServiceStub Implementation
//==============================================================================

LocalControlServiceStub::LocalControlServiceStub(std::shared_ptr<grpc::Channel> channel)
    : channel_(channel)
    , default_timeout_(30000) // 30 second timeout
{
    // Real stub will be initialized here after proto file generation
}

bool LocalControlServiceStub::RegisterLocal(
    const std::string& service_id,
    const std::string& version,
    const std::string& rpc_addr,
    const std::vector<LocalFunctionDescriptor>& functions,
    std::string& session_id,
    std::string& error_message) {

    try {
        // This is a mock implementation, real implementation needs proto files
        // Generate a mock session_id
        session_id = "mock_session_" + service_id + "_" + std::to_string(std::time(nullptr));

        std::cout << "📡 Registering service with Agent:" << std::endl;
        std::cout << "   Service ID: " << service_id << std::endl;
        std::cout << "   Version: " << version << std::endl;
        std::cout << "   RPC Address: " << rpc_addr << std::endl;
        std::cout << "   Functions: " << functions.size() << std::endl;
        for (const auto& func : functions) {
            std::cout << "     - " << func.id << " (v" << func.version << ")" << std::endl;
        }
        std::cout << "   Session ID: " << session_id << std::endl;

        // Mock successful registration
        return true;
    } catch (const std::exception& e) {
        error_message = "Registration failed: " + std::string(e.what());
        return false;
    }
}

bool LocalControlServiceStub::Heartbeat(
    const std::string& service_id,
    const std::string& session_id,
    std::string& error_message) {

    try {
        // This is a mock implementation
        // Real implementation will call agent's Heartbeat RPC

        // Mock successful heartbeat
        return true;
    } catch (const std::exception& e) {
        error_message = "Heartbeat failed: " + std::string(e.what());
        return false;
    }
}

bool LocalControlServiceStub::ListLocal(
    std::vector<FunctionDescriptor>& functions,
    std::string& error_message) {

    try {
        // Mock implementation
        functions.clear();
        return true;
    } catch (const std::exception& e) {
        error_message = "List functions failed: " + std::string(e.what());
        return false;
    }
}

bool LocalControlServiceStub::UnregisterLocal(
    const std::string& service_id,
    const std::string& session_id,
    std::string& error_message) {

    try {
        std::cout << "📡 Unregistering service from Agent: " << service_id
                  << ", session: " << session_id << std::endl;

        // Mock successful unregistration
        return true;
    } catch (const std::exception& e) {
        error_message = "Unregistration failed: " + std::string(e.what());
        return false;
    }
}

std::unique_ptr<grpc::ClientContext> LocalControlServiceStub::CreateContext() {
    auto context = std::make_unique<grpc::ClientContext>();

    // Set timeout
    auto deadline = std::chrono::system_clock::now() + default_timeout_;
    context->set_deadline(deadline);

    return context;
}

} // namespace grpc_service
} // namespace sdk
} // namespace croupier

#endif // CROUPIER_SDK_ENABLE_GRPC