#include "croupier/sdk/grpc_service.h"

// Only compile the full gRPC implementation when gRPC is enabled
#ifdef CROUPIER_SDK_ENABLE_GRPC

#include <iostream>
#include <sstream>
#include <regex>
#include <fstream>
#include <shared_mutex>
#include <chrono>
#include <random>

// Note: This implementation requires protobuf generated code
// In a real project, we need to generate gRPC proto files first, then implement the actual services

namespace croupier {
namespace sdk {
namespace grpc_service {

namespace {

std::string ReadFileContent(const std::string& path) {
    if (path.empty()) {
        return "";
    }
    std::ifstream stream(path, std::ios::in | std::ios::binary);
    if (!stream) {
        throw std::runtime_error("failed to open file: " + path);
    }
    std::ostringstream buffer;
    buffer << stream.rdbuf();
    return buffer.str();
}

} // namespace

namespace localv1 = ::croupier::agent::local::v1;
namespace functionv1 = ::croupier::function::v1;

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
        state_ = ConnectionState::FAILED;
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
    if (local_address_.empty()) {
        std::cerr << "❌ Local server is not running, cannot register with agent" << std::endl;
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

bool GrpcClientManager::RegisterAgentWithServer(
    const std::vector<FunctionDescriptor>& functions,
    std::string& session_id) {
    (void)functions;
    (void)session_id;
    std::cerr << "⚠️  RegisterAgentWithServer is not implemented on the C++ SDK; "
                 "agents forward registrations automatically."
              << std::endl;
    return false;
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
        const std::string listen_addr = config_.local_listen.empty() ? "127.0.0.1:0" : config_.local_listen;
        std::string host = "127.0.0.1";
        int requested_port = 0;

        auto pos = listen_addr.rfind(':');
        if (pos == std::string::npos) {
            host = listen_addr;
        } else {
            if (pos > 0) {
                host = listen_addr.substr(0, pos);
            } else {
                host.clear(); // signifies all interfaces
            }
            std::string port_str = listen_addr.substr(pos + 1);
            if (!port_str.empty()) {
                requested_port = std::stoi(port_str);
            }
        }

        grpc::ServerBuilder builder;
        auto server_creds = CreateServerCredentials();
        int bound_port = 0;
        builder.AddListeningPort(listen_addr, server_creds, &bound_port);

        std::map<std::string, FunctionHandler> handlers_copy;
        {
            std::lock_guard<std::mutex> lock(handler_mutex_);
            handlers_copy = handler_snapshot_;
        }

        if (!local_service_) {
            local_service_ = std::make_unique<LocalFunctionServiceImpl>(handlers_copy);
        } else {
            local_service_->UpdateHandlers(handlers_copy);
        }

        builder.RegisterService(local_service_.get());
        local_server_ = builder.BuildAndStart();
        if (!local_server_) {
            throw std::runtime_error("failed to start local gRPC server");
        }

        if (bound_port == 0) {
            bound_port = requested_port;
        }
        if (bound_port == 0) {
            throw std::runtime_error("server did not report a bound port");
        }

        local_port_ = bound_port;
        const std::string advertised_host = host.empty() ? "127.0.0.1" : host;
        local_address_ = advertised_host + ":" + std::to_string(bound_port);

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
        local_server_->Wait();
        local_server_.reset();
    }

    local_service_.reset();
    local_address_.clear();
    local_port_ = 0;
    std::cout << "🛑 Local gRPC server stopped" << std::endl;
}

std::string GrpcClientManager::GetLocalServerAddress() const {
    return local_address_;
}

grpc::ChannelArguments GrpcClientManager::BuildClientChannelArguments() const {
    return CreateChannelArguments();
}

std::shared_ptr<grpc::ChannelCredentials> GrpcClientManager::BuildClientCredentials() const {
    return CreateCredentials();
}

void GrpcClientManager::SetErrorCallback(std::function<void(const std::string&)> callback) {
    error_callback_ = callback;
}

void GrpcClientManager::SetReconnectCallback(std::function<void()> callback) {
    reconnect_callback_ = callback;
}

void GrpcClientManager::UpdateHandlers(const std::map<std::string, FunctionHandler>& handlers) {
    std::lock_guard<std::mutex> lock(handler_mutex_);
    handler_snapshot_ = handlers;
    if (local_service_) {
        local_service_->UpdateHandlers(handler_snapshot_);
    }
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
        const auto interval = std::chrono::seconds(
            config_.heartbeat_interval > 0 ? config_.heartbeat_interval : 60);

        while (heartbeat_running_) {
            std::this_thread::sleep_for(interval);

            if (!heartbeat_running_) break;

            if (!SendHeartbeat(session_id)) {
                std::cerr << "💔 Heartbeat failed, may need reconnection" << std::endl;
                 HandleError("heartbeat failed");
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

grpc::ChannelArguments GrpcClientManager::CreateChannelArguments() const {
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

std::shared_ptr<grpc::ChannelCredentials> GrpcClientManager::CreateCredentials() const {
    if (config_.insecure) {
        return grpc::InsecureChannelCredentials();
    }

    // TLS configuration
    grpc::SslCredentialsOptions ssl_options;

    try {
        if (!config_.ca_file.empty()) {
            ssl_options.pem_root_certs = ReadFileContent(config_.ca_file);
        }

        if (!config_.cert_file.empty() && !config_.key_file.empty()) {
            ssl_options.pem_cert_chain = ReadFileContent(config_.cert_file);
            ssl_options.pem_private_key = ReadFileContent(config_.key_file);
        }
    } catch (const std::exception& e) {
        std::cerr << "⚠️  Failed to load TLS credentials: " << e.what()
                  << ", falling back to insecure channel" << std::endl;
        return grpc::InsecureChannelCredentials();
    }

    return grpc::SslCredentials(ssl_options);
}

std::shared_ptr<grpc::ServerCredentials> GrpcClientManager::CreateServerCredentials() const {
    if (config_.insecure || config_.cert_file.empty() || config_.key_file.empty()) {
        return grpc::InsecureServerCredentials();
    }

    grpc::SslServerCredentialsOptions options;
    try {
        grpc::SslServerCredentialsOptions::PemKeyCertPair key_cert;
        key_cert.cert_chain = ReadFileContent(config_.cert_file);
        key_cert.private_key = ReadFileContent(config_.key_file);
        options.pem_key_cert_pairs.push_back(key_cert);
        if (!config_.ca_file.empty()) {
            options.pem_root_certs = ReadFileContent(config_.ca_file);
        }
        return grpc::SslServerCredentials(options);
    } catch (const std::exception& e) {
        std::cerr << "⚠️  Failed to initialize server TLS credentials: " << e.what()
                  << ", falling back to insecure server" << std::endl;
        return grpc::InsecureServerCredentials();
    }
}

//==============================================================================
// LocalFunctionServiceImpl Implementation
//==============================================================================

struct LocalFunctionServiceImpl::JobState {
    std::mutex mutex;
    std::condition_variable cv;
    std::deque<functionv1::JobEvent> queue;
    std::atomic<bool> finished{false};
};

LocalFunctionServiceImpl::LocalFunctionServiceImpl(
    const std::map<std::string, FunctionHandler>& handlers)
    : handlers_(handlers)
    , total_calls_(0)
    , successful_calls_(0)
    , failed_calls_(0) {
    std::cout << "🎯 Local function service initialized, handler count: " << handlers_.size() << std::endl;
}

void LocalFunctionServiceImpl::UpdateHandlers(const std::map<std::string, FunctionHandler>& handlers) {
    std::unique_lock<std::shared_mutex> lock(handlers_mutex_);
    handlers_ = handlers;
    std::cout << "🔄 Updated function handlers, current count: " << handlers_.size() << std::endl;
}

void LocalFunctionServiceImpl::AddHandler(const std::string& function_id, FunctionHandler handler) {
    std::unique_lock<std::shared_mutex> lock(handlers_mutex_);
    handlers_[function_id] = handler;
    std::cout << "➕ Added function handler: " << function_id << std::endl;
}

void LocalFunctionServiceImpl::RemoveHandler(const std::string& function_id) {
    std::unique_lock<std::shared_mutex> lock(handlers_mutex_);
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

std::string LocalFunctionServiceImpl::SerializeMetadata(
    const google::protobuf::Map<std::string, std::string>& metadata) const {
    if (metadata.empty()) {
        return "{}";
    }

    std::ostringstream oss;
    oss << "{";
    bool first = true;
    for (const auto& entry : metadata) {
        if (!first) {
            oss << ",";
        }
        first = false;
        oss << "\"" << entry.first << "\":\"" << entry.second << "\"";
    }
    oss << "}";
    return oss.str();
}

std::string LocalFunctionServiceImpl::NextJobId(const std::string& function_id) {
    const uint64_t counter = ++job_counter_;
    const auto now = std::chrono::steady_clock::now().time_since_epoch();
    const auto micros = std::chrono::duration_cast<std::chrono::microseconds>(now).count();
    std::ostringstream oss;
    oss << function_id << "-" << micros << "-" << counter;
    return oss.str();
}

std::shared_ptr<LocalFunctionServiceImpl::JobState> LocalFunctionServiceImpl::CreateJob(const std::string& job_id) {
    auto state = std::make_shared<JobState>();
    {
        std::lock_guard<std::mutex> lock(jobs_mutex_);
        jobs_[job_id] = state;
    }
    return state;
}

std::shared_ptr<LocalFunctionServiceImpl::JobState> LocalFunctionServiceImpl::FindJob(const std::string& job_id) const {
    std::lock_guard<std::mutex> lock(jobs_mutex_);
    auto it = jobs_.find(job_id);
    if (it == jobs_.end()) {
        return nullptr;
    }
    return it->second;
}

void LocalFunctionServiceImpl::FinishJob(const std::string& job_id) {
    std::shared_ptr<JobState> state;
    {
        std::lock_guard<std::mutex> lock(jobs_mutex_);
        auto it = jobs_.find(job_id);
        if (it != jobs_.end()) {
            state = it->second;
            jobs_.erase(it);
        }
    }
    if (state) {
        state->cv.notify_all();
    }
}

void LocalFunctionServiceImpl::EnqueueEvent(
    const std::shared_ptr<JobState>& state,
    functionv1::JobEvent&& event,
    bool mark_finished) {
    {
        std::lock_guard<std::mutex> lock(state->mutex);
        if (state->finished && !mark_finished) {
            return;
        }
        state->queue.emplace_back(std::move(event));
        if (mark_finished) {
            state->finished = true;
        }
    }
    state->cv.notify_all();
}

bool LocalFunctionServiceImpl::DequeueEvent(
    const std::shared_ptr<JobState>& state,
    functionv1::JobEvent* event) {
    std::unique_lock<std::mutex> lock(state->mutex);
    state->cv.wait(lock, [&]() {
        return !state->queue.empty() || state->finished.load();
    });

    if (state->queue.empty()) {
        return false;
    }

    *event = std::move(state->queue.front());
    state->queue.pop_front();
    return true;
}

std::string LocalFunctionServiceImpl::ExecuteHandler(
    const std::string& function_id,
    const std::string& context,
    const std::string& payload) {

    total_calls_++;

    FunctionHandler handler;
    {
        std::shared_lock<std::shared_mutex> lock(handlers_mutex_);
        auto it = handlers_.find(function_id);
        if (it == handlers_.end()) {
            failed_calls_++;
            throw std::runtime_error("function not found: " + function_id);
        }
        handler = it->second;
    }

    try {
        std::string result = handler(context, payload);
        successful_calls_++;
        return result;
    } catch (const std::exception& e) {
        failed_calls_++;
        throw std::runtime_error("handler error: " + std::string(e.what()));
    }
}

::grpc::Status LocalFunctionServiceImpl::Invoke(::grpc::ServerContext* context,
                                                const functionv1::InvokeRequest* request,
                                                functionv1::InvokeResponse* response) {
    (void)context;
    if (!request || request->function_id().empty()) {
        return ::grpc::Status(::grpc::StatusCode::INVALID_ARGUMENT, "function_id is required");
    }
    if (!response) {
        return ::grpc::Status(::grpc::StatusCode::INVALID_ARGUMENT, "response container is nil");
    }

    try {
        const auto context_json = SerializeMetadata(request->metadata());
        const std::string result = ExecuteHandler(
            request->function_id(),
            context_json,
            request->payload());
        response->set_payload(result);
        return ::grpc::Status::OK;
    } catch (const std::exception& e) {
        return ::grpc::Status(::grpc::StatusCode::INTERNAL, e.what());
    }
}

::grpc::Status LocalFunctionServiceImpl::StartJob(::grpc::ServerContext* context,
                                                  const functionv1::InvokeRequest* request,
                                                  functionv1::StartJobResponse* response) {
    (void)context;
    if (!request || request->function_id().empty()) {
        return ::grpc::Status(::grpc::StatusCode::INVALID_ARGUMENT, "function_id is required");
    }
    if (!response) {
        return ::grpc::Status(::grpc::StatusCode::INVALID_ARGUMENT, "response container is nil");
    }

    FunctionHandler handler;
    {
        std::shared_lock<std::shared_mutex> lock(handlers_mutex_);
        auto it = handlers_.find(request->function_id());
        if (it == handlers_.end()) {
            return ::grpc::Status(::grpc::StatusCode::NOT_FOUND, "function not registered");
        }
        handler = it->second;
    }

    const std::string job_id = NextJobId(request->function_id());
    auto state = CreateJob(job_id);

    functionv1::JobEvent started;
    started.set_type("started");
    started.set_message("job started");
    EnqueueEvent(state, std::move(started), false);

    const auto context_json = SerializeMetadata(request->metadata());
    const std::string payload = request->payload();

    std::thread([this, state, handler, job_id, context_json, payload]() {
        try {
            const std::string result = handler(context_json, payload);
            if (state->finished.load()) {
                FinishJob(job_id);
                return;
            }
            functionv1::JobEvent completed;
            completed.set_type("completed");
            completed.set_message("job completed");
            completed.set_payload(result);
            EnqueueEvent(state, std::move(completed), true);
        } catch (const std::exception& e) {
            if (state->finished.load()) {
                FinishJob(job_id);
                return;
            }
            functionv1::JobEvent error_evt;
            error_evt.set_type("error");
            error_evt.set_message(e.what());
            EnqueueEvent(state, std::move(error_evt), true);
        }
        FinishJob(job_id);
    }).detach();

    response->set_job_id(job_id);
    return ::grpc::Status::OK;
}

::grpc::Status LocalFunctionServiceImpl::StreamJob(
    ::grpc::ServerContext* context,
    const functionv1::JobStreamRequest* request,
    ::grpc::ServerWriter<functionv1::JobEvent>* writer) {
    (void)context;
    if (!request || request->job_id().empty()) {
        return ::grpc::Status(::grpc::StatusCode::INVALID_ARGUMENT, "job_id is required");
    }
    if (!writer) {
        return ::grpc::Status(::grpc::StatusCode::INVALID_ARGUMENT, "stream writer missing");
    }

    auto state = FindJob(request->job_id());
    if (!state) {
        return ::grpc::Status(::grpc::StatusCode::NOT_FOUND, "job not found");
    }

    functionv1::JobEvent event;
    while (DequeueEvent(state, &event)) {
        if (!writer->Write(event)) {
            return ::grpc::Status(::grpc::StatusCode::UNAVAILABLE, "stream closed by peer");
        }
    }

    return ::grpc::Status::OK;
}

::grpc::Status LocalFunctionServiceImpl::CancelJob(::grpc::ServerContext* context,
                                                   const functionv1::CancelJobRequest* request,
                                                   functionv1::StartJobResponse* response) {
    (void)context;
    if (!request || request->job_id().empty()) {
        return ::grpc::Status(::grpc::StatusCode::INVALID_ARGUMENT, "job_id is required");
    }
    if (!response) {
        return ::grpc::Status(::grpc::StatusCode::INVALID_ARGUMENT, "response container is nil");
    }

    auto state = FindJob(request->job_id());
    if (!state) {
        return ::grpc::Status(::grpc::StatusCode::NOT_FOUND, "job not found");
    }

    functionv1::JobEvent cancelled;
    cancelled.set_type("cancelled");
    cancelled.set_message("job cancelled by requester");
    EnqueueEvent(state, std::move(cancelled), true);
    FinishJob(request->job_id());

    response->set_job_id(request->job_id());
    return ::grpc::Status::OK;
}

//==============================================================================
// LocalControlServiceStub Implementation
//==============================================================================

LocalControlServiceStub::LocalControlServiceStub(std::shared_ptr<grpc::Channel> channel)
    : channel_(channel)
    , stub_(localv1::LocalControlService::NewStub(channel))
    , default_timeout_(std::chrono::milliseconds(30000)) // 30 second timeout
{}

bool LocalControlServiceStub::RegisterLocal(
    const std::string& service_id,
    const std::string& version,
    const std::string& rpc_addr,
    const std::vector<LocalFunctionDescriptor>& functions,
    std::string& session_id,
    std::string& error_message) {

    try {
        auto context = CreateContext();
        localv1::RegisterLocalRequest request;
        request.set_service_id(service_id);
        request.set_version(version);
        request.set_rpc_addr(rpc_addr);
        for (const auto& fn : functions) {
            auto* out = request.add_functions();
            out->set_id(fn.id);
            out->set_version(fn.version);
        }

        localv1::RegisterLocalResponse response;
        grpc::Status status = stub_->RegisterLocal(context.get(), request, &response);
        if (!status.ok()) {
            error_message = status.error_message();
            return false;
        }

        session_id = response.session_id();
        if (session_id.empty()) {
            error_message = "agent did not return a session_id";
            return false;
        }

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
        auto context = CreateContext();
        localv1::HeartbeatRequest request;
        request.set_service_id(service_id);
        request.set_session_id(session_id);

        localv1::HeartbeatResponse response;
        grpc::Status status = stub_->Heartbeat(context.get(), request, &response);
        if (!status.ok()) {
            error_message = status.error_message();
            return false;
        }
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
        auto context = CreateContext();
        localv1::ListLocalRequest request;
        localv1::ListLocalResponse response;
        grpc::Status status = stub_->ListLocal(context.get(), request, &response);
        if (!status.ok()) {
            error_message = status.error_message();
            return false;
        }

        functions.clear();
        for (const auto& fn : response.functions()) {
            FunctionDescriptor desc;
            desc.id = fn.id();
            if (!fn.instances().empty()) {
                desc.version = fn.instances(0).version();
            }
            desc.enabled = true;
            functions.emplace_back(std::move(desc));
        }
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

    (void)service_id;
    (void)session_id;
    error_message = "UnregisterLocal not supported by LocalControlService";
    return false;
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
