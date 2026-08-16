// Prevent Windows min/max macro conflicts
#ifdef _WIN32
#define NOMINMAX
#endif

#include "croupier/sdk/croupier_client.h"

#include "croupier/sdk/logger.h"
#include "croupier/sdk/tcp_transport.h"
#include "croupier/sdk/utils/json_utils.h"
#include "croupier/sdk/v1/invocation.pb.h"
#include "croupier/sdk/v1/provider.pb.h"

#include <algorithm>
#include <atomic>
#include <chrono>
#include <cstring>
#include <iostream>
#include <mutex>
#include <random>
#include <regex>
#include <sstream>
#include <stdexcept>
#include <thread>
#include <optional>
#include <unordered_map>

// Logging macros with configuration support
// These check the global logger configuration before outputting
// Note: Use string concatenation, not stream syntax: SDK_LOG_INFO("value: " + variable)
#define SDK_LOG_INFO(msg)                                                                \
    do {                                                                                 \
        if (!::croupier::sdk::Logger::GetInstance().IsEnabled(::croupier::sdk::Logger::Level::INFO)) \
            break;                                                                       \
        std::ostringstream oss_;                                                         \
        oss_ << (msg);                                                                   \
        std::cout << "[INFO] [croupier] " << oss_.str() << '\n';                         \
    } while (0)

#define SDK_LOG_WARN(msg)                                                                \
    do {                                                                                 \
        if (!::croupier::sdk::Logger::GetInstance().IsEnabled(::croupier::sdk::Logger::Level::WARN)) \
            break;                                                                       \
        std::ostringstream oss_;                                                         \
        oss_ << (msg);                                                                   \
        std::cerr << "[WARN] [croupier] " << oss_.str() << '\n';                         \
    } while (0)

#define SDK_LOG_ERROR(msg)                                                               \
    do {                                                                                 \
        if (!::croupier::sdk::Logger::GetInstance().IsEnabled(::croupier::sdk::Logger::Level::ERR)) \
            break;                                                                       \
        std::ostringstream oss_;                                                         \
        oss_ << (msg);                                                                   \
        std::cerr << "[ERROR] [croupier] " << oss_.str() << '\n';                        \
    } while (0)

#define SDK_LOG_DEBUG(msg)                                                               \
    do {                                                                                 \
        if (!::croupier::sdk::Logger::GetInstance().IsEnabled(::croupier::sdk::Logger::Level::DEBUG)) \
            break;                                                                       \
        std::ostringstream oss_;                                                         \
        oss_ << (msg);                                                                   \
        std::cout << "[DEBUG] [croupier] " << oss_.str() << '\n';                        \
    } while (0)

namespace croupier::sdk {

namespace {
[[maybe_unused]] std::string EscapeJsonString(const std::string& value) {
    std::string escaped;
    escaped.reserve(value.size());
    for (char ch : value) {
        switch (ch) {
        case '\\':
            escaped += "\\\\";
            break;
        case '"':
            escaped += "\\\"";
            break;
        case '\n':
            escaped += "\\n";
            break;
        case '\r':
            escaped += "\\r";
            break;
        case '\t':
            escaped += "\\t";
            break;
        default:
            escaped += ch;
            break;
        }
    }
    return escaped;
}

}  // namespace

namespace {

std::string NormalizeTCPAddress(const std::string& address) {
    if (address.empty()) {
        return address;
    }
    if (address.find("://") != std::string::npos) {
        return address;
    }
    return "tcp://" + address;
}

bool IsTCPAddress(const std::string& address) {
    return !address.empty() && (address.find("://") == std::string::npos || address.rfind("tcp://", 0) == 0);
}

struct TCPAddress {
    std::string host;
    int port = 0;
    std::string display;
};

TCPAddress ParseTCPAddress(const std::string& address) {
    const std::string normalized = NormalizeTCPAddress(address);
    if (normalized.rfind("tcp://", 0) != 0) {
        throw std::runtime_error("unsupported TCP address scheme: " + address);
    }

    const std::string endpoint = normalized.substr(6);
    std::string host;
    std::string port_text;
    if (!endpoint.empty() && endpoint.front() == '[') {
        const auto close_bracket = endpoint.find(']');
        if (close_bracket == std::string::npos || close_bracket + 1 >= endpoint.size() ||
            endpoint[close_bracket + 1] != ':') {
            throw std::runtime_error("invalid TCP address: " + address);
        }
        host = endpoint.substr(1, close_bracket - 1);
        port_text = endpoint.substr(close_bracket + 2);
    } else {
        const auto colon = endpoint.rfind(':');
        if (colon == std::string::npos) {
            throw std::runtime_error("TCP address must include a port: " + address);
        }
        host = endpoint.substr(0, colon);
        port_text = endpoint.substr(colon + 1);
    }

    if (host.empty() || port_text.empty()) {
        throw std::runtime_error("invalid TCP address: " + address);
    }

    size_t parsed = 0;
    int port = 0;
    try {
        port = std::stoi(port_text, &parsed);
    } catch (const std::exception&) {
        throw std::runtime_error("invalid TCP port: " + address);
    }
    if (parsed != port_text.size() || port <= 0 || port > 65535) {
        throw std::runtime_error("invalid TCP port: " + address);
    }

    return TCPAddress{host, port, normalized};
}

std::string NormalizeProviderTaskEventType(const ::croupier::sdk::v1::TaskEvent& event) {
    if (event.type() == "done") {
        return "completed";
    }
    if (event.type() == "error") {
        std::string message = event.message();
        std::transform(message.begin(), message.end(), message.begin(), ::tolower);
        if (message.find("cancel") != std::string::npos) {
            return "cancelled";
        }
    }
    return event.type();
}

std::vector<uint8_t> SerializeMessage(const google::protobuf::Message& message) {
    std::string bytes;
    if (!message.SerializeToString(&bytes)) {
        throw std::runtime_error("failed to serialize protobuf message");
    }
    return std::vector<uint8_t>(bytes.begin(), bytes.end());
}

template <typename T>
T ParseMessage(const std::vector<uint8_t>& bytes, const std::string& type_name) {
    T message;

    // Debug: log raw response bytes (first 32 bytes in hex)
    std::string hex_debug;
    size_t max_bytes = std::min(size_t(32), bytes.size());
    for (size_t i = 0; i < max_bytes; ++i) {
        char buf[4];
        snprintf(buf, sizeof(buf), "%02X", bytes[i]);
        hex_debug += buf;
        if (i < max_bytes - 1) hex_debug += " ";
        if (bytes.size() > 32 && i == 15) hex_debug += "... ";
    }
    std::cerr << "[DEBUG] Parsing " << type_name << ": size=" << bytes.size()
              << " bytes, hex=" << hex_debug << '\n';

    if (!message.ParseFromArray(bytes.data(), static_cast<int>(bytes.size()))) {
        throw std::runtime_error("failed to parse protobuf message: " + type_name);
    }
    return message;
}

TaskEvent ToTaskEvent(const std::string& task_id, const ::croupier::sdk::v1::TaskEvent& event) {
    TaskEvent result;
    result.event_type = NormalizeProviderTaskEventType(event);
    result.task_id = task_id;
    result.message = event.message();
    result.progress = event.progress();
    result.payload = event.payload();
    result.done = result.event_type == "completed" || result.event_type == "error" ||
                  result.event_type == "cancelled";
    if (result.event_type == "error" || result.event_type == "cancelled") {
        result.error = event.message();
    }
    return result;
}

bool IsTerminalTaskEvent(const TaskEvent& event) {
    return event.done || event.event_type == "completed" || event.event_type == "error" ||
           event.event_type == "cancelled";
}

bool SameTaskEvent(const TaskEvent& lhs, const TaskEvent& rhs) {
    return lhs.event_type == rhs.event_type && lhs.task_id == rhs.task_id && lhs.message == rhs.message &&
           lhs.progress == rhs.progress && lhs.payload == rhs.payload && lhs.error == rhs.error &&
           lhs.done == rhs.done;
}

}  // namespace

// Utility function implementations
namespace utils {
std::string NewIdempotencyKey() {
    std::random_device rd;
    // Use additional entropy to avoid narrowing conversion
    std::mt19937 gen(rd() ^ (rd() << 16));
    std::uniform_int_distribution<> dis(0, 15);

    std::stringstream ss;
    for (int i = 0; i < 32; ++i) {
        ss << std::hex << dis(gen);
    }
    return ss.str();
}

bool ValidateJSON(const std::string& json, const std::map<std::string, std::string>& schema) {
    // If no schema provided, just check if JSON is valid
    if (schema.empty()) {
        return ::croupier::sdk::utils::JsonUtils::IsValidJson(json);
    }

    // Convert schema map to JSON schema string
    std::string schema_json = "{";
    bool first = true;
    for (const auto& [key, value] : schema) {
        if (!first)
            schema_json += ",";
        schema_json += "\"" + key + "\":";

        // Try to parse value as JSON, fallback to string
        if ((value.front() == '{' && value.back() == '}') || (value.front() == '[' && value.back() == ']') ||
            value == "true" || value == "false" || value == "null") {
            schema_json += value;
        } else {
            schema_json += "\"" + value + "\"";
        }
        first = false;
    }
    schema_json += "}";

    // Use the new JSON schema validation
    return ::croupier::sdk::utils::JsonUtils::ValidateJsonSchema(json, schema_json);
}

std::map<std::string, std::string> ParseJSON(const std::string& json) {
    // Simplified JSON parsing for demonstration
    // Real implementation should use a proper JSON library like nlohmann/json
    std::map<std::string, std::string> result;

    if (json.empty())
        return result;

    // Very basic key-value extraction using regex
    std::regex pair_regex("\"([^\"]+)\"\\s*:\\s*\"([^\"]*)\"");
    std::sregex_iterator iter(json.begin(), json.end(), pair_regex);
    std::sregex_iterator end;

    for (; iter != end; ++iter) {
        const std::smatch& match = *iter;
        result[match[1].str()] = match[2].str();
    }

    return result;
}

std::string ToJSON(const std::map<std::string, std::string>& data) {
    std::stringstream ss;
    ss << "{";
    bool first = true;
    for (const auto& pair : data) {
        if (!first)
            ss << ",";
        ss << "\"" << pair.first << "\":\"" << pair.second << "\"";
        first = false;
    }
    ss << "}";
    return ss.str();
}
}  // namespace utils

// Client Implementation
class CroupierClient::Impl {
public:
    ClientConfig config_;
    std::map<std::string, FunctionHandler> handlers_;
    std::map<std::string, FunctionDescriptor> descriptors_;

    std::atomic<bool> running_{false};
    std::atomic<bool> connected_{false};
    std::unique_ptr<TCPTransport> transport_;
    std::mutex transport_mutex_;
    std::string session_id_;
    std::thread heartbeat_thread_;
    std::atomic<bool> should_stop_heartbeat_{false};
    std::optional<std::thread::id> heartbeat_thread_id_;
    std::string last_error_;

    // Reconnection state
    std::atomic<bool> is_reconnecting_{false};
    std::atomic<bool> should_stop_reconnecting_{false};
    std::thread reconnect_thread_;
    std::mutex reconnect_mutex_;  // Protects reconnect_thread_ access

    explicit Impl(const ClientConfig& config) : config_(config) {
        // ========== Initialize Logger Configuration ==========
        auto& logger = Logger::GetInstance();

        if (config_.disable_logging) {
            logger.SetLevel(Logger::Level::OFF);
        } else if (config_.debug_logging) {
            logger.SetLevel(Logger::Level::DEBUG);
        } else {
            logger.SetLevelFromString(config_.log_level);
        }

        // Validate required configuration
        if (config_.game_id.empty()) {
            SDK_LOG_WARN("game_id is required for proper backend separation");
        }

        // Validate environment
        if (config_.env != "development" && config_.env != "staging" && config_.env != "production") {
            SDK_LOG_WARN("Unknown environment '" + config_.env + "'. Valid values: development, staging, production");
        }

        if (config_.service_id.empty()) {
            config_.service_id = "cpp-sdk-" + utils::NewIdempotencyKey().substr(0, 8);
        }

        SDK_LOG_INFO("Initialized CroupierClient for game '" + config_.game_id + "' in '" + config_.env + "' environment");
    }

    ~Impl() { Stop(); }

    bool RegisterFunction(const FunctionDescriptor& desc, FunctionHandler handler) {
        if (running_) {
            SDK_LOG_ERROR("Cannot register functions while client is running");
            return false;
        }

        // Validate function ID
        if (desc.id.empty()) {
            SDK_LOG_ERROR("Cannot register function with empty ID");
            return false;
        }

        handlers_[desc.id] = std::move(handler);
        descriptors_[desc.id] = desc;

        SDK_LOG_INFO("Registered function: " + desc.id + " (version: " + desc.version + ")");
        return true;
    }

    // Re-register all functions
    void RegisterAllFunctions() {
        if (!connected_) {
            return;
        }
        try {
            const auto agent_address = ParseTCPAddress(config_.agent_addr);
            std::unique_ptr<TCPTransport> replacement =
                std::make_unique<TCPTransport>(agent_address.host, agent_address.port, config_.timeout_seconds * 1000);
            replacement->SetConnectTimeout(config_.connect_timeout_seconds * 1000);
            replacement->Connect();
            std::string session_id = registerWithAgent(*replacement);

            std::lock_guard<std::mutex> lock(transport_mutex_);
            if (transport_) {
                transport_->Close();
            }
            transport_ = std::move(replacement);
            session_id_ = std::move(session_id);
        } catch (const std::exception& e) {
            last_error_ = e.what();
            connected_ = false;
            SDK_LOG_ERROR("Failed to re-register local functions: " + last_error_);
        }
    }

    bool Connect() {
        if (connected_)
            return true;

        if (handlers_.empty()) {
            SDK_LOG_ERROR("Register at least one function before connecting");
            return false;
        }

        try {
            const auto agent_address = ParseTCPAddress(config_.agent_addr);
            auto transport =
                std::make_unique<TCPTransport>(agent_address.host, agent_address.port, config_.timeout_seconds * 1000);
            transport->Connect();
            std::string session_id = registerWithAgent(*transport);

            {
                std::lock_guard<std::mutex> lock(transport_mutex_);
                if (transport_) {
                    transport_->Close();
                }
                transport_ = std::move(transport);
                session_id_ = std::move(session_id);
            }

            connected_ = true;
            running_ = true;
            last_error_.clear();
            startHeartbeatLoop();
            SDK_LOG_INFO("Connected to agent at " + NormalizeTCPAddress(config_.agent_addr));
            return true;
        } catch (const std::exception& e) {
            last_error_ = e.what();
            connected_ = false;
            stopHeartbeatLoop();
            closeTransport();
            SDK_LOG_ERROR("Failed to connect/register client: " + last_error_);
            return false;
        }
    }

    void Serve() {
        if (!connected_) {
            Connect();
        }
        running_ = true;
        SDK_LOG_INFO("Croupier client service started");
        SDK_LOG_INFO("Registered functions: " + std::to_string(handlers_.size()));
        std::cout << "💡 使用 Stop() 方法StopService" << '\n';
        std::cout << "===============================================" << '\n';

        // Keep service running
        while (running_) {
            std::this_thread::sleep_for(std::chrono::milliseconds(100));
        }

        SDK_LOG_INFO("Service stopped");
    }

    void Stop() {
        running_ = false;
        connected_ = false;

        SDK_LOG_INFO("Stopping Croupier client...");

        // Always stop heartbeat loop to avoid std::terminate on thread destruction
        stopHeartbeatLoop();

        // Signal reconnection thread to stop
        should_stop_reconnecting_ = true;

        // Wait for reconnection thread to finish
        is_reconnecting_ = false;
        if (reconnect_thread_.joinable()) {
            reconnect_thread_.join();
        }

        closeTransport();
        session_id_.clear();

        SDK_LOG_INFO("Client fully stopped");
    }

    void Close() {
        Stop();
        handlers_.clear();
        descriptors_.clear();
    }

    bool IsConnected() const { return connected_; }

    void closeTransport() {
        std::lock_guard<std::mutex> lock(transport_mutex_);
        if (transport_) {
            transport_->Close();
            transport_.reset();
        }
    }

    std::string registerWithAgent(TCPTransport& transport) {
        ::croupier::sdk::v1::ProviderConnectRequest request;
        request.set_service_id(config_.service_id);
        request.set_version(config_.service_version);
        request.set_sdk_language("cpp");
        request.set_sdk_version("1.0.0");
        request.set_sdk_name("croupier-cpp-sdk");
        request.set_protocol_version("1.0.0");
        request.set_transport_security_mode(config_.insecure ? "plaintext" : "tls");
        request.add_supported_transports("tcp");

        for (const auto& [function_id, desc] : descriptors_) {
            auto* fn = request.add_functions();
            fn->set_id(function_id);
            fn->set_version(desc.version);
            for (const auto& tag : desc.tags) {
                fn->add_tags(tag);
            }
            if (!desc.summary.empty()) {
                fn->set_summary(desc.summary);
            }
            if (!desc.description.empty()) {
                fn->set_description(desc.description);
            }
            if (!desc.operation_id.empty()) {
                fn->set_operation_id(desc.operation_id);
            }
            fn->set_deprecated(desc.deprecated);
            if (!desc.input_schema.empty()) {
                fn->set_input_schema(desc.input_schema);
            }
            if (!desc.output_schema.empty()) {
                fn->set_output_schema(desc.output_schema);
            }
            if (!desc.resource.empty()) {
                fn->set_resource(desc.resource);
            }
            if (!desc.operation.empty()) {
                fn->set_operation(desc.operation);
            }
            if (!desc.capability.empty()) {
                fn->set_capability(desc.capability);
            }
            if (!desc.execution.empty()) {
                fn->set_execution(desc.execution);
            }
            fn->set_approval_required(desc.approval_required);
            if (!desc.approval_policy_key.empty()) {
                fn->set_approval_policy_key(desc.approval_policy_key);
            }
            if (!desc.risk.empty()) {
                fn->set_risk(desc.risk);
            }
            fn->set_enabled(desc.enabled);
            if (!desc.permission.empty()) {
                fn->set_permission(desc.permission);
            }
        }

        auto [msg_id, response_body] = transport.Call(protocol::MSG_PROVIDER_CONNECT_REQUEST, SerializeMessage(request));

        // Debug: log response details
        std::string hex_resp;
        for (size_t i = 0; i < std::min(size_t(50), response_body.size()); ++i) {
            char buf[4];
            snprintf(buf, sizeof(buf), "%02X", response_body[i]);
            hex_resp += buf;
            if (i < 49) hex_resp += " ";
        }
        std::cerr << "[DEBUG] ProviderConnect response: msg_id=" << msg_id
                  << " (0x" << std::hex << msg_id << std::dec << ")"
                  << ", body_size=" << response_body.size()
                  << ", hex=" << hex_resp << '\n';

        // Check if response is a JSON error message from Agent
        if (!response_body.empty() && response_body[0] == '{') {
            std::string json_error(response_body.begin(), response_body.end());
            std::cerr << "[ERROR] Agent returned JSON error: " << json_error << '\n';
            throw std::runtime_error("Agent returned error: " + json_error);
        }

        std::cerr << "[DEBUG] About to call ParseMessage..." << '\n';
        auto response =
            ParseMessage<::croupier::sdk::v1::ProviderConnectResponse>(response_body, "ProviderConnectResponse");
        std::cerr << "[DEBUG] ParseMessage returned successfully" << '\n';
        std::cerr << "[DEBUG] About to call response.session_id()..." << '\n';
        std::cerr << std::flush;  // Force flush
        // Copy session_id to avoid dangling reference
        std::string session_id = response.session_id();
        std::cerr << "[DEBUG] Got session_id: " << session_id << '\n';
        std::cerr << std::flush;  // Force flush
        if (session_id.empty()) {
            throw std::runtime_error("ProviderConnect returned empty session_id");
        }
        return session_id;
    }

    void startHeartbeatLoop() {
        stopHeartbeatLoop();
        should_stop_heartbeat_ = false;
        // detach: the heartbeat thread may run the reconnect loop, and
        // Connect() inside it calls startHeartbeatLoop() again — a join there
        // would self-deadlock. Ownership is tracked via heartbeat_thread_id_.
        heartbeat_thread_ = std::thread([this]() {
            heartbeat_thread_id_ = std::this_thread::get_id();
            const auto interval = std::max(1, config_.heartbeat_interval);
            while (!should_stop_heartbeat_) {
                for (int elapsed = 0; elapsed < interval * 10 && !should_stop_heartbeat_; ++elapsed) {
                    std::this_thread::sleep_for(std::chrono::milliseconds(100));
                }
                if (should_stop_heartbeat_) {
                    break;
                }

                try {
                    sendHeartbeat();
                } catch (const std::exception& e) {
                    last_error_ = e.what();
                    connected_ = false;
                    SDK_LOG_WARN("Heartbeat failed: " + last_error_);
                    if (!should_stop_heartbeat_ && running_) {
                        reconnectLoop();
                    }
                    break;
                }
            }
        });
        heartbeat_thread_.detach();
    }

    void stopHeartbeatLoop() {
        should_stop_heartbeat_ = true;
        // Cannot join from the heartbeat thread itself (reconnect path calls
        // Connect() -> startHeartbeatLoop() -> stopHeartbeatLoop()).
        if (heartbeat_thread_.joinable()) {
            if (heartbeat_thread_id_.has_value() &&
                *heartbeat_thread_id_ == std::this_thread::get_id()) {
                return;  // self-stop: thread will exit via should_stop_heartbeat_
            }
            heartbeat_thread_.join();
        }
    }

    // Blocking reconnect loop used after a heartbeat/connection failure.
    // Retries with a fixed interval until Stop() is called or the
    // connection is re-established. Runs on the heartbeat thread.
    void reconnectLoop() {
        std::lock_guard<std::mutex> lock(reconnect_mutex_);
        if (is_reconnecting_.exchange(true)) {
            return;  // another thread is already reconnecting
        }
        int attempt = 0;
        while (!should_stop_heartbeat_ && running_) {
            ++attempt;
            SDK_LOG_INFO("Reconnecting to agent (attempt " + std::to_string(attempt) + ")...");
            if (Connect()) {
                SDK_LOG_INFO("Reconnected and re-registered after " + std::to_string(attempt) + " attempt(s)");
                is_reconnecting_ = false;
                return;
            }
            for (int waited = 0; waited < 5 * 10 && !should_stop_heartbeat_ && running_; ++waited) {
                std::this_thread::sleep_for(std::chrono::milliseconds(100));
            }
        }
        is_reconnecting_ = false;
    }

    void stopHeartbeatLoop() {
        should_stop_heartbeat_ = true;
        if (heartbeat_thread_.joinable()) {
            heartbeat_thread_.join();
        }
    }

    void sendHeartbeat() {
        ::croupier::sdk::v1::ProviderHeartbeatRequest request;
        request.set_service_id(config_.service_id);
        request.set_session_id(session_id_);

        std::lock_guard<std::mutex> lock(transport_mutex_);
        if (!transport_ || !transport_->IsConnected()) {
            throw std::runtime_error("heartbeat transport is not connected");
        }
        transport_->Call(protocol::MSG_PROVIDER_HEARTBEAT_REQUEST, SerializeMessage(request));
    }
};

// Invoker Implementation
class CroupierInvoker::Impl {
public:
    struct LocalJobState {
        std::string task_id;
        std::string function_id;
        std::string payload;
        std::vector<TaskEvent> events;
        std::atomic<bool> done{false};
        std::atomic<bool> cancelled{false};
        std::thread worker;
    };

    InvokerConfig config_;
    ReconnectConfig reconnect_config_;
    RetryConfig retry_config_;
    std::map<std::string, std::map<std::string, std::string>> schemas_;
    std::unique_ptr<TCPTransport> transport_;
    std::atomic<bool> connected_{false};
    std::atomic<uint64_t> next_task_id_{1};
    std::mutex transport_mutex_;
    std::mutex jobs_mutex_;
    std::unordered_map<std::string, std::shared_ptr<LocalJobState>> jobs_;

    // Reconnection state
    std::atomic<bool> is_reconnecting_{false};
    std::atomic<int> reconnect_attempts_{0};
    std::atomic<bool> should_stop_reconnecting_{false};
    std::thread reconnect_thread_;
    std::mutex reconnect_mutex_;  // Protects reconnect_thread_ access
    std::string last_error_;

    // Close state
    std::atomic<bool> closed_{false};

    explicit Impl(const InvokerConfig& config) : config_(config) {
        // ========== Initialize Logger Configuration ==========
        auto& logger = Logger::GetInstance();

        if (config_.disable_logging) {
            logger.SetLevel(Logger::Level::OFF);
        } else if (config_.debug_logging) {
            logger.SetLevel(Logger::Level::DEBUG);
        } else {
            logger.SetLevelFromString(config_.log_level);
        }

        // Set default reconnect config
        reconnect_config_.enabled = true;
        reconnect_config_.max_attempts = 0;  // Infinite
        reconnect_config_.initial_delay_ms = 1000;
        reconnect_config_.max_delay_ms = 30000;
        reconnect_config_.backoff_multiplier = 2.0;
        reconnect_config_.jitter_factor = 0.2;

        // Initialize retry config from config or use defaults
        retry_config_ = config_.retry;
        if (!retry_config_.enabled) {
            // Use default retry config if not set
            retry_config_.enabled = true;
            retry_config_.max_attempts = 3;
            retry_config_.initial_delay_ms = 100;
            retry_config_.max_delay_ms = 5000;
            retry_config_.backoff_multiplier = 2.0;
            retry_config_.jitter_factor = 0.1;
            if (retry_config_.retryable_status_codes.empty()) {
                retry_config_.retryable_status_codes = {14, 13, 2, 10, 4};  // Default codes
            }
        }
    }

    ~Impl() { Close(); }

    bool Connect() {
        bool result = connectInternal();
        if (!result && IsConnectionError()) {
            ScheduleReconnectIfNeeded();
        }
        return result;
    }

    // Internal connect method that doesn't trigger reconnection
    bool connectInternal() {
        if (connected_)
            return true;

        SDK_LOG_INFO("Connecting to server/agent at: " + config_.address);
        if (config_.address.empty()) {
            last_error_ = "connection address is empty";
            connected_ = false;
            return false;
        }

        if (!IsTCPAddress(config_.address)) {
            last_error_.clear();
            connected_ = true;
            SDK_LOG_INFO("Using local fallback mode for non-TCP address: " + config_.address);
            return true;
        }

        try {
            const auto server_address = ParseTCPAddress(config_.address);
            auto transport =
                std::make_unique<TCPTransport>(server_address.host, server_address.port, config_.timeout_seconds * 1000);
            transport->SetConnectTimeout(config_.connect_timeout_seconds * 1000);
            transport->Connect();
            {
                std::lock_guard<std::mutex> lock(transport_mutex_);
                transport_ = std::move(transport);
            }
            last_error_.clear();
            connected_ = true;
            SDK_LOG_INFO("Connected to: " + server_address.display);
            return true;
        } catch (const std::exception& e) {
            last_error_ = e.what();
            connected_ = false;
            SDK_LOG_ERROR("Failed to connect: " + last_error_);
            return false;
        }
    }

    std::string Invoke(const std::string& function_id, const std::string& payload, const InvokeOptions& options) {
        if (!connected_ && !connectInternal()) {
            if (IsConnectionError()) {
                ScheduleReconnectIfNeeded();
            }
            throw std::runtime_error("Not connected to server");
        }

        // Client-side validation
        auto it = schemas_.find(function_id);
        if (it != schemas_.end()) {
            if (!utils::ValidateJSON(payload, it->second)) {
                throw std::runtime_error("Payload validation failed for function: " + function_id);
            }
        }

        // Get retry config (use options retry if provided, otherwise use config retry)
        const RetryConfig& retry_config = options.retry.has_value() ? *options.retry : retry_config_;

        // If retry is disabled, execute directly
        if (!retry_config.enabled) {
            return invokeInternal(function_id, payload, options);
        }

        // Execute with retry
        int max_attempts = retry_config.max_attempts;
        std::string last_error;

        for (int attempt = 0; attempt < max_attempts; ++attempt) {
            try {
                return invokeInternal(function_id, payload, options);
            } catch (const std::exception& e) {
                last_error = e.what();

                // Check if this error is retryable and not the last attempt
                if (attempt >= max_attempts - 1) {
                    throw std::runtime_error("Invoke failed after " + std::to_string(max_attempts) +
                                             " attempts: " + last_error);
                }

                // Check if error is retryable (simplified check)
                bool is_retryable = last_error.find("UNAVAILABLE") != std::string::npos ||
                                    last_error.find("INTERNAL") != std::string::npos ||
                                    last_error.find("DEADLINE") != std::string::npos ||
                                    last_error.find("connection") != std::string::npos ||
                                    last_error.find("timeout") != std::string::npos;

                if (!is_retryable) {
                    throw std::runtime_error("Invoke failed with non-retryable error: " + last_error);
                }

                // Connection errors should trigger reconnection
                if (IsConnectionError() && reconnect_config_.enabled) {
                    connected_ = false;
                    ScheduleReconnectIfNeeded();
                }

                // Calculate delay and wait
                int delay = CalculateRetryDelay(attempt);
                std::cout << "Invocation attempt " << (attempt + 1) << " failed, retrying in " << delay
                          << " ms: " << last_error << '\n';
                std::this_thread::sleep_for(std::chrono::milliseconds(delay));
            }
        }

        throw std::runtime_error("Invoke failed after " + std::to_string(max_attempts) + " attempts: " + last_error);
    }

    std::string invokeInternal(const std::string& function_id, const std::string& payload,
                               const InvokeOptions& options) {
        if (!IsTCPAddress(config_.address)) {
            (void)options;
            std::stringstream response;
            response << "{\"status\":\"success\",\"function_id\":\"" << EscapeJsonString(function_id)
                     << "\",\"payload\":" << (payload.empty() ? "null" : payload) << "}";
            return response.str();
        }

        ::croupier::sdk::v1::InvokeRequest req;
        req.set_function_id(function_id);
        req.set_idempotency_key(options.idempotency_key.empty() ? utils::NewIdempotencyKey() : options.idempotency_key);
        req.set_payload(payload);

        for (const auto& [key, value] : config_.headers) {
            (*req.mutable_metadata())[key] = value;
        }
        for (const auto& [key, value] : options.metadata) {
            (*req.mutable_metadata())[key] = value;
        }
        if (!config_.auth_token.empty() && req.metadata().find("Authorization") == req.metadata().end()) {
            (*req.mutable_metadata())["Authorization"] = "Bearer " + config_.auth_token;
        }
        if (!config_.game_id.empty()) {
            (*req.mutable_metadata())["X-Game-ID"] = config_.game_id;
        }
        if (!config_.env.empty()) {
            (*req.mutable_metadata())["X-Env"] = config_.env;
        }
        if (!options.route.empty()) {
            (*req.mutable_metadata())["route"] = options.route;
        }
        if (!options.target_service_id.empty()) {
            (*req.mutable_metadata())["target_service_id"] = options.target_service_id;
        }
        if (!options.hash_key.empty()) {
            (*req.mutable_metadata())["hash_key"] = options.hash_key;
        }
        if (!options.trace_id.empty()) {
            (*req.mutable_metadata())["trace_id"] = options.trace_id;
        }

        std::lock_guard<std::mutex> lock(transport_mutex_);
        if (!transport_ || !transport_->IsConnected()) {
            throw std::runtime_error("Not connected to server");
        }

        auto [_, response_body] = transport_->Call(protocol::MSG_INVOKE_REQUEST, SerializeMessage(req));
        auto response = ParseMessage<::croupier::sdk::v1::InvokeResponse>(response_body, "InvokeResponse");
        return response.payload();
    }

    std::string StartTask(const std::string& function_id, const std::string& payload, const InvokeOptions& options) {
        if (!connected_ && !connectInternal()) {
            if (IsConnectionError()) {
                ScheduleReconnectIfNeeded();
            }
            throw std::runtime_error("Not connected to server");
        }

        // Client-side validation
        auto it = schemas_.find(function_id);
        if (it != schemas_.end()) {
            if (!utils::ValidateJSON(payload, it->second)) {
                throw std::runtime_error("Payload validation failed for function: " + function_id);
            }
        }

        // Get retry config (use options retry if provided, otherwise use config retry)
        const RetryConfig& retry_config = options.retry.has_value() ? *options.retry : retry_config_;

        // If retry is disabled, execute directly
        if (!retry_config.enabled) {
            return startJobInternal(function_id, payload, options);
        }

        // Execute with retry
        int max_attempts = retry_config.max_attempts;
        std::string last_error;

        for (int attempt = 0; attempt < max_attempts; ++attempt) {
            try {
                return startJobInternal(function_id, payload, options);
            } catch (const std::exception& e) {
                last_error = e.what();

                // Check if this error is retryable and not the last attempt
                if (attempt >= max_attempts - 1) {
                    throw std::runtime_error("StartTask failed after " + std::to_string(max_attempts) +
                                             " attempts: " + last_error);
                }

                // Check if error is retryable (simplified check)
                bool is_retryable = last_error.find("UNAVAILABLE") != std::string::npos ||
                                    last_error.find("INTERNAL") != std::string::npos ||
                                    last_error.find("DEADLINE") != std::string::npos ||
                                    last_error.find("connection") != std::string::npos ||
                                    last_error.find("timeout") != std::string::npos;

                if (!is_retryable) {
                    throw std::runtime_error("StartTask failed with non-retryable error: " + last_error);
                }

                // Connection errors should trigger reconnection
                if (IsConnectionError() && reconnect_config_.enabled) {
                    connected_ = false;
                    ScheduleReconnectIfNeeded();
                }

                // Calculate delay and wait
                int delay = CalculateRetryDelay(attempt);
                std::cout << "StartTask attempt " << (attempt + 1) << " failed, retrying in " << delay
                          << " ms: " << last_error << '\n';
                std::this_thread::sleep_for(std::chrono::milliseconds(delay));
            }
        }

        throw std::runtime_error("StartTask failed after " + std::to_string(max_attempts) + " attempts: " + last_error);
    }

    std::string startJobInternal(const std::string& function_id, const std::string& payload,
                                 const InvokeOptions& options) {
        if (IsTCPAddress(config_.address)) {
            ::croupier::sdk::v1::InvokeRequest req;
            req.set_function_id(function_id);
            req.set_idempotency_key(options.idempotency_key.empty() ? utils::NewIdempotencyKey() : options.idempotency_key);
            req.set_payload(payload);

            for (const auto& [key, value] : config_.headers) {
                (*req.mutable_metadata())[key] = value;
            }
            for (const auto& [key, value] : options.metadata) {
                (*req.mutable_metadata())[key] = value;
            }
            if (!config_.auth_token.empty() && req.metadata().find("Authorization") == req.metadata().end()) {
                (*req.mutable_metadata())["Authorization"] = "Bearer " + config_.auth_token;
            }
            if (!config_.game_id.empty()) {
                (*req.mutable_metadata())["X-Game-ID"] = config_.game_id;
            }
            if (!config_.env.empty()) {
                (*req.mutable_metadata())["X-Env"] = config_.env;
            }
            if (!options.route.empty()) {
                (*req.mutable_metadata())["route"] = options.route;
            }
            if (!options.target_service_id.empty()) {
                (*req.mutable_metadata())["target_service_id"] = options.target_service_id;
            }
            if (!options.hash_key.empty()) {
                (*req.mutable_metadata())["hash_key"] = options.hash_key;
            }
            if (!options.trace_id.empty()) {
                (*req.mutable_metadata())["trace_id"] = options.trace_id;
            }

            std::vector<uint8_t> response_body;
            {
                std::lock_guard<std::mutex> lock(transport_mutex_);
                if (!transport_ || !transport_->IsConnected()) {
                    throw std::runtime_error("Not connected to server");
                }
                auto response = transport_->Call(protocol::MSG_START_TASK_REQUEST, SerializeMessage(req));
                response_body = std::move(response.second);
            }

            auto response = ParseMessage<::croupier::sdk::v1::StartTaskResponse>(response_body, "StartTaskResponse");
            if (response.task_id().empty()) {
                throw std::runtime_error("StartTask response did not include job ID");
            }

            auto state = std::make_shared<LocalJobState>();
            state->task_id = response.task_id();
            state->function_id = function_id;
            state->payload = payload;

            TaskEvent started_event;
            started_event.event_type = "started";
            started_event.task_id = response.task_id();
            started_event.message = "Job started";
            started_event.progress = 0;
            started_event.done = false;
            state->events.push_back(started_event);

            {
                std::lock_guard<std::mutex> lock(jobs_mutex_);
                jobs_[state->task_id] = state;
            }

            return response.task_id();
        }

        std::cout << "Starting job for function: " << function_id << '\n';
        std::string task_id = "job-" + std::to_string(next_task_id_.fetch_add(1));
        auto job = std::make_shared<LocalJobState>();
        job->task_id = task_id;
        job->function_id = function_id;
        job->payload = payload;

        {
            std::lock_guard<std::mutex> lock(jobs_mutex_);
            jobs_[task_id] = job;
        }

        job->worker = std::thread([this, job, options]() {
            if (job->cancelled) {
                return;
            }

            TaskEvent started;
            started.event_type = "started";
            started.task_id = job->task_id;
            started.payload = "{\"status\":\"started\"}";
            appendTaskEvent(job, started);

            std::this_thread::sleep_for(std::chrono::milliseconds(10));
            if (job->cancelled) {
                return;
            }

            TaskEvent progress;
            progress.event_type = "progress";
            progress.task_id = job->task_id;
            progress.progress = 50;
            progress.payload = "{\"progress\":50}";
            appendTaskEvent(job, progress);

            std::this_thread::sleep_for(std::chrono::milliseconds(10));
            if (job->cancelled) {
                return;
            }

            try {
                const std::string result = invokeInternal(job->function_id, job->payload, options);
                if (job->cancelled) {
                    return;
                }

                TaskEvent completed;
                completed.event_type = "completed";
                completed.task_id = job->task_id;
                completed.payload = result;
                completed.progress = 100;
                completed.done = true;
                appendTaskEvent(job, completed);
            } catch (const std::exception& e) {
                TaskEvent error;
                error.event_type = "failed";
                error.task_id = job->task_id;
                error.error = e.what();
                error.done = true;
                appendTaskEvent(job, error);
            }
        });

        std::cout << "Job started: " << task_id << '\n';
        return task_id;
    }

    std::future<std::vector<TaskEvent>> StreamTask(const std::string& task_id) {
        return std::async(std::launch::async, [this, task_id]() {
            if (!connected_ && !connectInternal()) {
                if (IsConnectionError()) {
                    ScheduleReconnectIfNeeded();
                }
                TaskEvent error_event;
                error_event.event_type = "error";
                error_event.task_id = task_id;
                error_event.error = "Not connected to server";
                error_event.message = error_event.error;
                error_event.done = true;
                return std::vector<TaskEvent>{error_event};
            }

            if (IsTCPAddress(config_.address)) {
                std::vector<TaskEvent> events;
                {
                    std::lock_guard<std::mutex> lock(jobs_mutex_);
                    auto it = jobs_.find(task_id);
                    if (it != jobs_.end()) {
                        events.insert(events.end(), it->second->events.begin(), it->second->events.end());
                        if (!events.empty() && IsTerminalTaskEvent(events.back())) {
                            jobs_.erase(it);
                            return events;
                        }
                    }
                }

                for (int attempt = 0; attempt < 120; ++attempt) {
                    ::croupier::sdk::v1::TaskStreamRequest req;
                    req.set_task_id(task_id);

                    std::vector<uint8_t> response_body;
                    {
                        std::lock_guard<std::mutex> lock(transport_mutex_);
                        if (!transport_ || !transport_->IsConnected()) {
                            TaskEvent error_event;
                            error_event.task_id = task_id;
                            error_event.error = "Connection lost while streaming job";
                            error_event.done = true;
                            events.push_back(error_event);
                            return events;
                        }
                        auto response = transport_->Call(protocol::MSG_STREAM_TASK_REQUEST, SerializeMessage(req));
                        response_body = std::move(response.second);
                    }

                    auto proto_event = ParseMessage<::croupier::sdk::v1::TaskEvent>(response_body, "TaskEvent");
                    TaskEvent event = ToTaskEvent(task_id, proto_event);
                    if (events.empty() || !SameTaskEvent(events.back(), event)) {
                        events.push_back(event);
                    }

                    {
                        std::lock_guard<std::mutex> lock(jobs_mutex_);
                        auto& state = jobs_[task_id];
                        if (!state) {
                            state = std::make_shared<LocalJobState>();
                            state->task_id = task_id;
                        }
                        state->events = events;
                        if (IsTerminalTaskEvent(event)) {
                            state->done = true;
                        }
                    }

                    if (IsTerminalTaskEvent(event)) {
                        std::lock_guard<std::mutex> lock(jobs_mutex_);
                        jobs_.erase(task_id);
                        return events;
                    }

                    std::this_thread::sleep_for(std::chrono::milliseconds(500));
                }

                TaskEvent timeout_event;
                timeout_event.event_type = "error";
                timeout_event.task_id = task_id;
                timeout_event.error = "Timed out waiting for job completion";
                timeout_event.message = timeout_event.error;
                timeout_event.done = true;
                events.push_back(timeout_event);
                return events;
            }

            std::cout << "Streaming job events for: " << task_id << '\n';
            auto job = findJob(task_id);
            if (!job) {
                TaskEvent error_event;
                error_event.event_type = "failed";
                error_event.task_id = task_id;
                error_event.error = "Job not found";
                error_event.done = true;
                return std::vector<TaskEvent>{error_event};
            }

            while (!job->done) {
                std::this_thread::sleep_for(std::chrono::milliseconds(10));
            }

            if (job->worker.joinable()) {
                job->worker.join();
            }

            std::vector<TaskEvent> events;
            {
                std::lock_guard<std::mutex> lock(jobs_mutex_);
                events = job->events;
                jobs_.erase(task_id);
            }
            return events;
        });
    }

    bool CancelTask(const std::string& task_id) {
        if (task_id.empty()) {
            std::cerr << "Job ID is required" << '\n';
            return false;
        }

        std::cout << "Cancelling job: " << task_id << '\n';

        if (IsTCPAddress(config_.address)) {
            if (!connected_ && !connectInternal()) {
                if (IsConnectionError()) {
                    ScheduleReconnectIfNeeded();
                }
                std::cerr << "Not connected to server" << '\n';
                return false;
            }

            ::croupier::sdk::v1::CancelTaskRequest req;
            req.set_task_id(task_id);

            {
                std::lock_guard<std::mutex> lock(transport_mutex_);
                if (!transport_ || !transport_->IsConnected()) {
                    std::cerr << "Not connected to server" << '\n';
                    return false;
                }
                transport_->Call(protocol::MSG_CANCEL_TASK_REQUEST, SerializeMessage(req));
            }

            std::lock_guard<std::mutex> lock(jobs_mutex_);
            auto it = jobs_.find(task_id);
            if (it != jobs_.end()) {
                TaskEvent cancelled_event;
                cancelled_event.event_type = "cancelled";
                cancelled_event.task_id = task_id;
                cancelled_event.message = "Job cancelled";
                cancelled_event.done = true;
                it->second->events.push_back(cancelled_event);
                it->second->cancelled = true;
                it->second->done = true;
            }
            return true;
        }

        auto job = findJob(task_id);
        if (!job || job->done) {
            return false;
        }

        job->cancelled = true;
        TaskEvent cancelled;
        cancelled.event_type = "cancelled";
        cancelled.task_id = task_id;
        cancelled.message = "Job cancelled";
        cancelled.done = true;
        appendTaskEvent(job, cancelled);
        std::cout << "Job cancellation sent: " << task_id << '\n';
        return true;
    }

    void SetSchema(const std::string& function_id, const std::map<std::string, std::string>& schema) {
        schemas_[function_id] = schema;
        std::cout << "Set schema for function: " << function_id << '\n';
    }

    void SetReconnectConfig(const ReconnectConfig& config) { reconnect_config_ = config; }

    void SetRetryConfig(const RetryConfig& config) { retry_config_ = config; }

    void Close() {
        // Idempotent close - only execute once
        if (closed_.exchange(true)) {
            return;  // Already closed
        }

        // First, signal all threads to stop
        should_stop_reconnecting_ = true;
        connected_ = false;

        // Detach reconnect thread to avoid deadlock
        // The thread will exit naturally when it checks should_stop_reconnecting_
        {
            std::lock_guard<std::mutex> lock(reconnect_mutex_);
            if (reconnect_thread_.joinable()) {
                reconnect_thread_.detach();
            }
        }

        // Signal all jobs to cancel
        std::vector<std::shared_ptr<LocalJobState>> jobs_to_cancel;
        {
            std::lock_guard<std::mutex> lock(jobs_mutex_);
            for (const auto& entry : jobs_) {
                jobs_to_cancel.push_back(entry.second);
            }
        }
        for (const auto& job : jobs_to_cancel) {
            job->cancelled = true;
            job->done = true;
            // Detach worker threads instead of joining to avoid deadlock
            if (job->worker.joinable()) {
                job->worker.detach();
            }
        }
        {
            std::lock_guard<std::mutex> lock(jobs_mutex_);
            jobs_.clear();
        }

        // Close transport - this sets closing_ flag and closes socket
        // This will unblock any pending Call() operations
        {
            std::lock_guard<std::mutex> lock(transport_mutex_);
            if (transport_) {
                transport_->Close();
                transport_.reset();
            }
        }

        schemas_.clear();
        SDK_LOG_INFO("Invoker closed");
    }

    std::shared_ptr<LocalJobState> findJob(const std::string& task_id) {
        std::lock_guard<std::mutex> lock(jobs_mutex_);
        auto it = jobs_.find(task_id);
        if (it == jobs_.end()) {
            return nullptr;
        }
        return it->second;
    }

    void appendTaskEvent(const std::shared_ptr<LocalJobState>& job, const TaskEvent& event) {
        std::lock_guard<std::mutex> lock(jobs_mutex_);
        if (job->done) {
            return;
        }
        job->events.push_back(event);
        if (event.done) {
            job->done = true;
        }
    }

    // Check if error is a connection error
    bool IsConnectionError() const {
        std::string lower_error = last_error_;
        std::transform(lower_error.begin(), lower_error.end(), lower_error.begin(), ::tolower);

        // Check for common connection error patterns
        return lower_error.find("connection") != std::string::npos ||
               lower_error.find("refused") != std::string::npos || lower_error.find("reset") != std::string::npos ||
               lower_error.find("unreachable") != std::string::npos || lower_error.find("timeout") != std::string::npos;
    }

    // Calculate reconnection delay with exponential backoff and jitter
    int CalculateReconnectDelay() const {
        // Calculate base delay using exponential backoff
        int base_delay = reconnect_config_.initial_delay_ms;
        int exponential_delay =
            static_cast<int>(base_delay * std::pow(reconnect_config_.backoff_multiplier, reconnect_attempts_ - 1));

        // Cap at max delay
        if (exponential_delay > reconnect_config_.max_delay_ms) {
            exponential_delay = reconnect_config_.max_delay_ms;
        }

        // Add jitter to prevent thundering herd
        std::random_device rd;
        // Use additional entropy to avoid narrowing conversion
        std::mt19937 gen(rd() ^ (rd() << 16));
        std::uniform_real_distribution<> dis(-reconnect_config_.jitter_factor, reconnect_config_.jitter_factor);
        double jitter_ratio = dis(gen);

        int jitter = static_cast<int>(exponential_delay * jitter_ratio);
        int final_delay = exponential_delay + jitter;

        if (final_delay < 0) {
            final_delay = 0;
        }

        return final_delay;
    }

    // Schedule reconnection if enabled
    void ScheduleReconnectIfNeeded() {
        // Don't schedule if already closed
        if (closed_ || should_stop_reconnecting_) {
            return;
        }

        if (!reconnect_config_.enabled) {
            return;
        }

        if (is_reconnecting_) {
            return;
        }

        // Check max attempts
        if (reconnect_config_.max_attempts > 0 && reconnect_attempts_ >= reconnect_config_.max_attempts) {
            std::cout << "Max reconnection attempts (" << reconnect_config_.max_attempts << ") reached, giving up"
                      << '\n';
            return;
        }

        is_reconnecting_ = true;
        reconnect_attempts_++;

        int delay = CalculateReconnectDelay();
        std::cout << "Scheduling reconnection attempt " << reconnect_attempts_ << " in " << delay << " ms" << '\n';

        // Stop existing reconnect thread if any (use mutex to protect)
        std::thread old_thread;
        {
            std::lock_guard<std::mutex> lock(reconnect_mutex_);
            if (reconnect_thread_.joinable()) {
                old_thread = std::move(reconnect_thread_);
            }
        }
        if (old_thread.joinable()) {
            old_thread.join();
        }

        // Start reconnection thread
        std::thread new_thread([this, delay]() {
            // Use interruptible sleep with 100ms intervals to check for stop signal
            const int sleep_interval_ms = 100;
            int elapsed = 0;
            while (elapsed < delay && !should_stop_reconnecting_ && !closed_) {
                int remaining = delay - elapsed;
                int sleep_time = (remaining < sleep_interval_ms) ? remaining : sleep_interval_ms;
                std::this_thread::sleep_for(std::chrono::milliseconds(sleep_time));
                elapsed += sleep_time;
            }

            if (should_stop_reconnecting_ || closed_) {
                is_reconnecting_ = false;
                return;
            }

            std::cout << "Reconnecting... (attempt " << reconnect_attempts_ << ")" << '\n';
            if (connectInternal()) {
                std::cout << "Reconnection successful" << '\n';
            } else {
                std::cout << "Reconnection attempt " << reconnect_attempts_ << " failed" << '\n';
                // Schedule next attempt (only if not stopping)
                if (!should_stop_reconnecting_ && !closed_) {
                    ScheduleReconnectIfNeeded();
                }
            }
        });

        {
            std::lock_guard<std::mutex> lock(reconnect_mutex_);
            reconnect_thread_ = std::move(new_thread);
        }
    }

    // Check if error is retryable based on status code
    bool IsRetryableError(int status_code) const {
        for (int code : retry_config_.retryable_status_codes) {
            if (code == status_code) {
                return true;
            }
        }
        return false;
    }

    // Calculate retry delay with exponential backoff and jitter
    int CalculateRetryDelay(int attempt) const {
        // Calculate base delay using exponential backoff
        int base_delay = retry_config_.initial_delay_ms;
        int exponential_delay = static_cast<int>(base_delay * std::pow(retry_config_.backoff_multiplier, attempt));

        // Cap at max delay
        if (exponential_delay > retry_config_.max_delay_ms) {
            exponential_delay = retry_config_.max_delay_ms;
        }

        // Add jitter to prevent thundering herd
        std::random_device rd;
        // Use additional entropy to avoid narrowing conversion
        std::mt19937 gen(rd() ^ (rd() << 16));
        std::uniform_real_distribution<> dis(-retry_config_.jitter_factor, retry_config_.jitter_factor);
        double jitter_ratio = dis(gen);

        int jitter = static_cast<int>(exponential_delay * jitter_ratio);
        int final_delay = exponential_delay + jitter;

        if (final_delay < 0) {
            final_delay = 0;
        }

        return final_delay;
    }
};

// CroupierClient public interface
CroupierClient::CroupierClient(const ClientConfig& config) : impl_(std::make_unique<Impl>(config)) {}

CroupierClient::~CroupierClient() = default;

// ========== Function Registration ==========
bool CroupierClient::RegisterFunction(const FunctionDescriptor& desc, FunctionHandler handler) {
    return impl_->RegisterFunction(desc, std::move(handler));
}

// ========== Core Operations ==========

bool CroupierClient::Connect() {
    return impl_->Connect();
}

bool CroupierClient::IsConnected() const {
    return impl_->IsConnected();
}

void CroupierClient::Serve() {
    impl_->Serve();
}

void CroupierClient::Stop() {
    impl_->Stop();
}

void CroupierClient::Close() {
    impl_->Close();
}


// CroupierInvoker public interface
CroupierInvoker::CroupierInvoker(const InvokerConfig& config) : impl_(std::make_unique<Impl>(config)) {}

CroupierInvoker::~CroupierInvoker() = default;

bool CroupierInvoker::Connect() {
    return impl_->Connect();
}

std::string CroupierInvoker::Invoke(const std::string& function_id, const std::string& payload,
                                    const InvokeOptions& options) {
    return impl_->Invoke(function_id, payload, options);
}

std::string CroupierInvoker::StartTask(const std::string& function_id, const std::string& payload,
                                      const InvokeOptions& options) {
    return impl_->StartTask(function_id, payload, options);
}

std::future<std::vector<TaskEvent>> CroupierInvoker::StreamTask(const std::string& task_id) {
    return impl_->StreamTask(task_id);
}

bool CroupierInvoker::CancelTask(const std::string& task_id) {
    return impl_->CancelTask(task_id);
}

void CroupierInvoker::SetSchema(const std::string& function_id, const std::map<std::string, std::string>& schema) {
    impl_->SetSchema(function_id, schema);
}

void CroupierInvoker::SetReconnectConfig(const ReconnectConfig& config) {
    impl_->SetReconnectConfig(config);
}

void CroupierInvoker::SetRetryConfig(const RetryConfig& config) {
    impl_->SetRetryConfig(config);
}

void CroupierInvoker::Close() {
    impl_->Close();
}

}  // namespace croupier::sdk
