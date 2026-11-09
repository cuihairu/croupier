// Prevent Windows min/max macro conflicts
#ifdef _WIN32
#define NOMINMAX
#endif

#ifndef CROUPIER_SDK_VERSION
#define CROUPIER_SDK_VERSION "0.1.6"
#endif

#include "croupier/sdk/croupier_client.h"

#include "croupier/sdk/logger.h"
#include "croupier/sdk/tcp_transport.h"
#include "croupier/sdk/utils/json_utils.h"
#include "croupier/sdk/file_push.h"
#include "croupier/sdk/v1/invocation.pb.h"
#include "croupier/sdk/v1/provider.pb.h"

#include <algorithm>
#include <atomic>
#include <cctype>
#include <chrono>
#include <cmath>
#include <cstdint>
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

#include <cctype>
#include <nlohmann/json.hpp>
#include <zlib.h>

#include "croupier/agent/v1/register.pb.h"

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

[[maybe_unused]] bool IsTCPAddress(const std::string& address) {
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

[[maybe_unused]] std::string NormalizeProviderTaskEventType(const ::croupier::sdk::v1::TaskEvent& event) {
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

[[maybe_unused]] TaskEvent ToTaskEvent(const std::string& task_id, const ::croupier::sdk::v1::TaskEvent& event) {
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

[[maybe_unused]] bool IsTerminalTaskEvent(const TaskEvent& event) {
    return event.done || event.event_type == "completed" || event.event_type == "error" ||
           event.event_type == "cancelled";
}

[[maybe_unused]] bool SameTaskEvent(const TaskEvent& lhs, const TaskEvent& rhs) {
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

    // Drain 状态：收到 ProviderDrainRequest 后置位——拒绝新 Invoke，
    // 在途调用清零后复用 RegisterAllFunctions 恢复会话（对齐 C# 参考实现）。
    std::atomic<bool> draining_{false};
    std::atomic<int64_t> inflight_calls_{0};

    // Reconnection state：重连跑在心跳线程上（reconnectLoop），无独立
    // 重连线程；is_reconnecting_ 防外部并发触发重入。
    std::atomic<bool> is_reconnecting_{false};

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
            replacement->SetInboundHandler([this](uint32_t msg_id, uint32_t req_id, const std::vector<uint8_t>& body) {
                return handleAgentRequest(msg_id, req_id, body);
            });
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

    // 在途调用 RAII 计数：drain 恢复以其清零为信号。
    struct InflightGuard {
        explicit InflightGuard(Impl* impl) : impl_(impl) { impl_->inflight_calls_.fetch_add(1); }
        ~InflightGuard() { impl_->inflight_calls_.fetch_sub(1); }
        Impl* impl_;
    };

    // 等待在途调用完成（最多 30s），随后按 auto_reconnect 语义恢复会话。
    void DrainAndRecover() {
        const auto deadline = std::chrono::steady_clock::now() + std::chrono::seconds(30);
        while (inflight_calls_.load() > 0 && std::chrono::steady_clock::now() < deadline
               && running_.load()) {
            std::this_thread::sleep_for(std::chrono::milliseconds(100));
        }
        if (inflight_calls_.load() > 0) {
            SDK_LOG_ERROR("Drain timeout with in-flight calls still running");
        }
        if (config_.auto_reconnect && running_.load()) {
            SDK_LOG_INFO("Drain complete, reconnecting provider session");
            try {
                RegisterAllFunctions();
            } catch (const std::exception& e) {
                SDK_LOG_ERROR(std::string("Drain recovery failed: ") + e.what());
            } catch (...) {
                SDK_LOG_ERROR("Drain recovery failed: unknown error");
            }
        }
        draining_.store(false);
    }

    // handleAgentRequest 处理 Agent -> Provider 调用（invoke / start task），
    // 由 TCPTransport 的有界 worker 池并发执行（读循环只投递）。
    std::vector<uint8_t> handleAgentRequest(uint32_t msg_id, uint32_t /*req_id*/, const std::vector<uint8_t>& body) {
        try {
            if (msg_id == protocol::MSG_PROVIDER_DRAIN_REQUEST) {
                // 幂等：重复 drain 只回确认。置位后异步等待在途清零再恢复。
                if (!draining_.exchange(true)) {
                    SDK_LOG_INFO("Drain requested");
                    std::thread([this]() { DrainAndRecover(); }).detach();
                }
                ::croupier::sdk::v1::ProviderDrainResponse resp;
                return SerializeMessage(resp);
            }
            if (msg_id == protocol::MSG_PROVIDER_FILE_PUSH_REQ) {
                // F：文件下发接收（校验链在 file_push.h/Impl 内）
                auto req = DecodeFilePushRequest(body);
                FilePushResponse resp;
                resp.transfer_id = req.transfer_id;
                std::string staging =
                    config_.file_staging_dir.empty() ? std::string("./croupier-staging")
                                                     : config_.file_staging_dir;
                int max_size =
                    config_.max_file_size > 0 ? config_.max_file_size : 10 * 1024 * 1024;
                std::string target;
                if (!config_.enable_file_transfer) {
                    resp.error = "file transfer is disabled on this provider";
                } else if (req.transfer_id.empty()) {
                    resp.error = "transferId is required";
                } else if (!SafeStagingPath(staging, req.file_name, target)) {
                    resp.error =
                        "file name must be a bare basename: \"" + req.file_name + "\"";
                } else if (req.data.empty()) {
                    resp.error = "file payload is empty";
                } else if (static_cast<int>(req.data.size()) > max_size) {
                    resp.error = "file size " + std::to_string(req.data.size()) +
                                 " exceeds max " + std::to_string(max_size);
                } else if (req.content_sha256.empty()) {
                    resp.error = "contentSha256 is required";
                } else if (Sha256Hex(req.data) != req.content_sha256) {
                    // 大小写不敏感比对
                    std::string actual = Sha256Hex(req.data);
                    std::string expected = req.content_sha256;
                    for (auto& c : expected) c = static_cast<char>(std::tolower(c));
                    for (auto& c : actual) c = static_cast<char>(std::tolower(c));
                    if (actual != expected) {
                        resp.error = "checksum mismatch";
                    }
                }
                if (resp.error.empty()) {
                    if (!AtomicWriteFile(target, req.data)) {
                        resp.error = "write staging file failed";
                    } else {
                        resp.ok = true;
                        resp.stored_path = target;
                    }
                }
                return EncodeFilePushResponse(resp);
            }
            if (msg_id == protocol::MSG_INVOKE_REQUEST) {
                // drain 期间拒绝新调用，等待 Agent 停止投递。
                if (draining_.load()) {
                    ::croupier::sdk::v1::InvokeResponse resp;
                    resp.set_payload("{\"error\":\"provider is draining\"}");
                    return SerializeMessage(resp);
                }
                InflightGuard guard(this);
                auto req = ParseMessage<::croupier::sdk::v1::InvokeRequest>(body, "InvokeRequest");
                auto it = handlers_.find(req.function_id());
                if (it == handlers_.end()) {
                    SDK_LOG_ERROR("Agent invoke: function not found: " + req.function_id());
                    return {};
                }
                std::string context = "{}";
                std::string payload(req.payload().begin(), req.payload().end());

                // Provider 侧入站校验（可选）：按函数声明的 input schema 校验
                // payload，失败回错误响应，handler 不被调用（服务端仍是权威
                // 校验方）。schema 缺失/为空跳过。
                if (config_.validate_input_payloads) {
                    std::string input_schema;
                    {
                        std::lock_guard<std::mutex> lock(transport_mutex_);
                        auto descIt = descriptors_.find(req.function_id());
                        if (descIt != descriptors_.end()) {
                            input_schema = descIt->second.input_schema;
                        }
                    }
                    if (!input_schema.empty() &&
                        !::croupier::sdk::utils::JsonUtils::ValidateJsonSchema(payload, input_schema)) {
                        SDK_LOG_ERROR("Agent invoke: payload validation failed: " + req.function_id());
                        ::croupier::sdk::v1::InvokeResponse errResp;
                        errResp.set_payload("{\"error\":\"payload validation failed\"}");
                        return SerializeMessage(errResp);
                    }
                }

                std::string result = it->second(context, payload);
                ::croupier::sdk::v1::InvokeResponse resp;
                resp.set_payload(result);
                return SerializeMessage(resp);
            }
            if (msg_id == protocol::MSG_PROVIDER_HEARTBEAT_REQUEST) {
                // keepalive pong（agent 侧探针）
                ::croupier::sdk::v1::ProviderHeartbeatResponse resp;
                return SerializeMessage(resp);
            }
        } catch (const std::exception& e) {
            SDK_LOG_ERROR(std::string("Agent request handling failed: ") + e.what());
        }
        return {};
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
            transport->SetInboundHandler([this](uint32_t msg_id, uint32_t req_id, const std::vector<uint8_t>& body) {
                return handleAgentRequest(msg_id, req_id, body);
            });
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

            // 控制面 manifest 上传（best-effort，不阻断注册结果）
            maybeRegisterCapabilities();

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

        // Always stop heartbeat loop to avoid std::terminate on thread destruction.
        // 重连跑在心跳线程上，置位 + join 已覆盖其退出。
        stopHeartbeatLoop();

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

    // 构建控制面能力清单（provider 元数据 + 函数摘要），与 JS/C#/Go 同构。
    std::string buildManifestJson() const {
        std::string json = "{\"provider\":{";
        json += "\"id\":\"" + EscapeJsonString(
            config_.service_id.empty() ? std::string("cpp-service") : config_.service_id) + "\",";
        json += "\"version\":\"" + EscapeJsonString(
            config_.service_version.empty() ? std::string("1.0.0") : config_.service_version) + "\",";
        json += "\"lang\":\"" + EscapeJsonString(
            config_.provider_lang.empty() ? std::string("cpp") : config_.provider_lang) + "\",";
        json += "\"sdk\":\"" + EscapeJsonString(
            config_.provider_sdk.empty() ? std::string("croupier-cpp-sdk") : config_.provider_sdk) + "\"}";
        json += ",\"functions\":[";
        bool first = true;
        for (const auto& [function_id, desc] : descriptors_) {
            if (function_id.empty()) continue;
            if (!first) json += ",";
            first = false;
            json += "{\"id\":\"" + EscapeJsonString(function_id) + "\"";
            json += ",\"version\":\"" + EscapeJsonString(desc.version.empty() ? std::string("1.0.0") : desc.version) + "\"";
            if (!desc.resource.empty()) json += ",\"resource\":\"" + EscapeJsonString(desc.resource) + "\"";
            if (!desc.operation.empty()) json += ",\"operation\":\"" + EscapeJsonString(desc.operation) + "\"";
            if (!desc.risk.empty()) json += ",\"risk\":\"" + EscapeJsonString(desc.risk) + "\"";
            if (!desc.permission.empty()) json += ",\"permission\":\"" + EscapeJsonString(desc.permission) + "\"";
            if (!desc.description.empty()) json += ",\"description\":\"" + EscapeJsonString(desc.description) + "\"";
            if (!desc.input_schema.empty()) json += ",\"inputSchema\":" + desc.input_schema;
            if (!desc.output_schema.empty()) json += ",\"outputSchema\":" + desc.output_schema;
            json += "}";
        }
        json += "]}";
        return json;
    }

    // gzip 压缩（RFC 1952 envelope）：zlib deflate + windowBits 15+16。
    static std::vector<uint8_t> GzipCompress(const std::string& data) {
        std::vector<uint8_t> out;
        z_stream stream{};
        if (deflateInit2(&stream, Z_DEFAULT_COMPRESSION, Z_DEFLATED, 15 + 16, 8, Z_DEFAULT_STRATEGY) != Z_OK) {
            throw std::runtime_error("gzip init failed");
        }
        stream.next_in = reinterpret_cast<Bytef*>(const_cast<char*>(data.data()));
        stream.avail_in = static_cast<uInt>(data.size());
        char buffer[4096];
        int status = Z_OK;
        do {
            stream.next_out = reinterpret_cast<Bytef*>(buffer);
            stream.avail_out = sizeof(buffer);
            status = deflate(&stream, Z_FINISH);
            if (status != Z_OK && status != Z_STREAM_END && status != Z_BUF_ERROR) {
                deflateEnd(&stream);
                throw std::runtime_error("gzip deflate failed");
            }
            out.insert(out.end(), buffer, buffer + (sizeof(buffer) - stream.avail_out));
        } while (status != Z_STREAM_END);
        deflateEnd(&stream);
        return out;
    }

    // 向控制面（config_.control_addr）上传能力清单。
    // 独立短连接 + 超时；任何失败仅告警，不影响已完成的函数注册。
    void maybeRegisterCapabilities() {
        if (config_.control_addr.empty()) {
            return;
        }
        try {
            const auto address = ParseTCPAddress(config_.control_addr);
            TCPTransport control(address.host, address.port, 5000);
            control.Connect();

            ::croupier::agent::v1::ProviderMeta meta;
            meta.set_id(config_.service_id.empty() ? std::string("cpp-service") : config_.service_id);
            meta.set_version(config_.service_version.empty() ? std::string("1.0.0") : config_.service_version);
            meta.set_lang(config_.provider_lang.empty() ? std::string("cpp") : config_.provider_lang);
            meta.set_sdk(config_.provider_sdk.empty() ? std::string("croupier-cpp-sdk") : config_.provider_sdk);
            ::croupier::agent::v1::RegisterCapabilitiesRequest request;
            *request.mutable_provider() = meta;
            const std::string manifest = buildManifestJson();
            const auto manifest_gz = GzipCompress(manifest);
            request.set_manifest_json_gz(manifest_gz.data(), manifest_gz.size());

            const auto body = SerializeMessage(request);
            const auto [resp_msg, resp_data] = control.Call(protocol::MSG_REGISTER_CAPABILITIES_REQ, body);
            (void)resp_msg;
            (void)resp_data;
            control.Close();
            SDK_LOG_INFO("Capabilities registered to control plane: " + config_.control_addr);
        } catch (const std::exception& e) {
            SDK_LOG_WARN(std::string("Failed to register capabilities: ") + e.what());
        }
    }

    std::string registerWithAgent(TCPTransport& transport) {
        ::croupier::sdk::v1::ProviderConnectRequest request;
        request.set_service_id(config_.service_id);
        request.set_version(config_.service_version);
        request.set_sdk_language("cpp");
        request.set_sdk_version(CROUPIER_SDK_VERSION);
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
        // Self-stop（心跳线程经 Connect() 失败路径进入）：直接返回，绝不
        // 置位 should_stop_heartbeat_ —— 置位会让同线程上 reconnectLoop()
        // 的循环条件立即失效，重连在第一次失败后永久放弃（2026-09-11
        // 线上 cpp demo 挂起根因）。心跳线程的退出由 reconnectLoop 返回后
        // 的 break 完成。注意不能以 joinable() 作前提：线程已 detach，
        // joinable() 恒为 false，只能比较线程 id。
        if (heartbeat_thread_id_.has_value() &&
            *heartbeat_thread_id_ == std::this_thread::get_id()) {
            return;
        }
        should_stop_heartbeat_ = true;
        if (heartbeat_thread_.joinable()) {
            heartbeat_thread_.join();
        }
    }

    // Blocking reconnect loop used after a heartbeat/connection failure.
    // Retries with a fixed interval until Stop() is called or the
    // connection is re-established. Runs on the heartbeat thread.
    void reconnectLoop() {
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

bool CroupierClient::IsDraining() const {
    return impl_->draining_.load();
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


}  // namespace croupier::sdk
