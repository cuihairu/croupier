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

using json = nlohmann::json;

class HTTPStatusError final : public std::runtime_error {
public:
    HTTPStatusError(long status_code, const std::string& message)
        : std::runtime_error("server returned HTTP " + std::to_string(status_code) + ": " + message),
          status_code_(status_code) {}

    long status_code() const { return status_code_; }

private:
    long status_code_;
};

std::string TrimString(const std::string& value) {
    const auto begin = std::find_if_not(value.begin(), value.end(), [](unsigned char ch) { return std::isspace(ch) != 0; });
    const auto end = std::find_if_not(value.rbegin(), value.rend(), [](unsigned char ch) { return std::isspace(ch) != 0; }).base();
    return begin < end ? std::string(begin, end) : std::string();
}

std::string LowerString(std::string value) {
    std::transform(value.begin(), value.end(), value.begin(), [](unsigned char ch) {
        return static_cast<char>(std::tolower(ch));
    });
    return value;
}

std::string NormalizeServerAPIURL(const std::string& configured_address) {
    std::string address = TrimString(configured_address);
    if (address.empty()) {
        address = "http://127.0.0.1:18780/api/v1";
    }
    if (address.find("://") == std::string::npos) {
        address = "http://" + address;
    }

    const auto scheme_end = address.find("://");
    const std::string scheme = LowerString(address.substr(0, scheme_end));
    if ((scheme != "http" && scheme != "https") || scheme_end == 0) {
        throw std::invalid_argument("InvokerConfig.address must be an HTTP(S) Server address; TCP is not supported");
    }

    const size_t authority_start = scheme_end + 3;
    const size_t path_start = address.find_first_of("/?#", authority_start);
    const std::string authority = address.substr(authority_start, path_start - authority_start);
    if (authority.empty()) {
        throw std::invalid_argument("InvokerConfig.address must include a Server host");
    }

    std::string path;
    if (path_start != std::string::npos && address[path_start] == '/') {
        const size_t path_end = address.find_first_of("?#", path_start);
        path = address.substr(path_start, path_end - path_start);
    }
    while (path.size() > 1 && path.back() == '/') {
        path.pop_back();
    }
    if (path.empty() || path == "/") {
        path = "/api/v1";
    } else if (path.size() < 7 || path.compare(path.size() - 7, 7, "/api/v1") != 0) {
        path += "/api/v1";
    }
    return scheme + "://" + authority + path;
}

std::string EscapeURLSegment(const std::string& value) {
    static constexpr char kHex[] = "0123456789ABCDEF";
    std::string escaped;
    escaped.reserve(value.size());
    for (unsigned char ch : value) {
        if (std::isalnum(ch) != 0 || ch == '-' || ch == '_' || ch == '.' || ch == '~') {
            escaped.push_back(static_cast<char>(ch));
            continue;
        }
        escaped.push_back('%');
        escaped.push_back(kHex[(ch >> 4U) & 0x0FU]);
        escaped.push_back(kHex[ch & 0x0FU]);
    }
    return escaped;
}

bool HasHeader(const std::map<std::string, std::string>& headers, const std::string& name) {
    const std::string expected = LowerString(name);
    return std::any_of(headers.begin(), headers.end(), [&expected](const auto& header) {
        return LowerString(header.first) == expected;
    });
}

void SetHeader(std::map<std::string, std::string>& headers, const std::string& name, const std::string& value) {
    if (name.empty() || name.find_first_of("\r\n") != std::string::npos || value.find_first_of("\r\n") != std::string::npos) {
        throw std::invalid_argument("HTTP headers cannot contain empty names, CR, or LF");
    }
    const std::string expected = LowerString(name);
    for (auto& header : headers) {
        if (LowerString(header.first) == expected) {
            header.second = value;
            return;
        }
    }
    headers.emplace(name, value);
}

std::string ServerErrorMessage(const std::string& body) {
    try {
        const json parsed = json::parse(body);
        if (parsed.is_object()) {
            for (const char* key : {"message", "error"}) {
                const auto it = parsed.find(key);
                if (it != parsed.end() && it->is_string() && !TrimString(it->get<std::string>()).empty()) {
                    return it->get<std::string>();
                }
            }
        }
    } catch (const json::exception&) {
    }
    const std::string message = TrimString(body);
    return message.empty() ? "empty response body" : message;
}

bool IsTerminalTaskEventType(const std::string& event_type) {
    const std::string normalized = LowerString(event_type);
    return normalized == "completed" || normalized == "succeeded" || normalized == "failed" ||
           normalized == "error" || normalized == "cancelled" || normalized == "canceled" ||
           normalized == "timed_out" || normalized == "timeout";
}

std::string EventType(const json& item) {
    const auto it = item.find("type");
    if (it == item.end() || !it->is_string()) {
        return "unknown";
    }
    const std::string type = it->get<std::string>();
    return LowerString(type) == "done" ? "completed" : type;
}

std::string JsonStringValue(const json& object, const char* key) {
    const auto it = object.find(key);
    return it != object.end() && it->is_string() ? it->get<std::string>() : std::string();
}

int JsonIntValue(const json& object, const char* key) {
    const auto it = object.find(key);
    return it != object.end() && it->is_number_integer() ? it->get<int>() : 0;
}

}  // namespace

// CroupierInvoker is an independent L3 client for the Server HTTP API. It
// intentionally has no reference to Provider TCP state, so Server-side scope,
// authorization, audit and task persistence remain authoritative.
class CroupierInvoker::Impl {
public:
    explicit Impl(const InvokerConfig& config)
        : config_(config),
          base_url_(NormalizeServerAPIURL(config.address)),
          retry_config_(config.retry),
          transport_(config.http_transport ? config.http_transport : NewDefaultHTTPTransport()) {
        if (!transport_) {
            throw std::invalid_argument("HTTP transport cannot be null");
        }
        config_.address = base_url_;
        if (config_.timeout_seconds <= 0) {
            config_.timeout_seconds = 30;
        }
        if (config_.task_poll_interval_ms <= 0) {
            config_.task_poll_interval_ms = 500;
        }
    }

    bool Connect() {
        if (closed_) {
            return false;
        }
        connected_ = true;
        return true;
    }

    std::string Invoke(const std::string& function_id, const std::string& payload, const InvokeOptions& options) {
        ValidateIdentifier("function ID", function_id);
        ValidatePayload(function_id, payload);

        json body{{"params", ParsePayload(payload)}};
        if (!TrimString(options.route).empty()) body["route"] = options.route;
        if (!TrimString(options.target_service_id).empty()) body["targetServiceId"] = options.target_service_id;
        if (!TrimString(options.hash_key).empty()) body["hashKey"] = options.hash_key;

        const json response = ParseResponse(Request("POST", {"functions", function_id, "invoke"}, body.dump(), options));
        const auto result = response.find("result");
        if (result == response.end()) {
            throw std::runtime_error("server invoke response does not contain result");
        }
        return result->dump();
    }

    std::string StartTask(const std::string& function_id, const std::string& payload, const InvokeOptions& options) {
        ValidateIdentifier("function ID", function_id);
        ValidatePayload(function_id, payload);

        const json body{{"functionId", function_id}, {"params", ParsePayload(payload)}};
        const json response = ParseResponse(Request("POST", {"tasks"}, body.dump(), options));
        const std::string task_id = JsonStringValue(response, "taskId");
        if (TrimString(task_id).empty()) {
            throw std::runtime_error("server start task response does not contain taskId");
        }
        return task_id;
    }

    TaskStatus GetTaskStatus(const std::string& task_id) {
        ValidateIdentifier("task ID", task_id);
        const json response = ParseResponse(Request("GET", {"tasks", task_id}, "", {}));

        TaskStatus status;
        status.task_id = JsonStringValue(response, "id");
        if (status.task_id.empty()) status.task_id = task_id;
        status.function_id = JsonStringValue(response, "functionId");
        status.status = JsonStringValue(response, "status");
        status.progress = JsonIntValue(response, "progress");
        status.message = JsonStringValue(response, "message");
        status.error = JsonStringValue(response, "error");
        status.game_id = JsonStringValue(response, "gameId");
        status.env = JsonStringValue(response, "env");
        status.agent_id = JsonStringValue(response, "agentId");
        status.actor = JsonStringValue(response, "actor");
        status.trace_id = JsonStringValue(response, "traceId");
        status.started_at = JsonStringValue(response, "startedAt");
        status.finished_at = JsonStringValue(response, "finishedAt");
        status.created_at = JsonStringValue(response, "createdAt");
        status.updated_at = JsonStringValue(response, "updatedAt");
        const auto result = response.find("result");
        if (result != response.end() && !result->is_null()) status.result = result->dump();
        return status;
    }

    std::future<std::vector<TaskEvent>> StreamTask(const std::string& task_id) {
        return std::async(std::launch::async, [this, task_id] {
            try {
                ValidateIdentifier("task ID", task_id);
                std::vector<TaskEvent> events;
                int64_t after_seq = 0;
                for (;;) {
                    const json response = ParseResponse(Request(
                        "GET", {"tasks", task_id, "events"}, "", {}, "after_seq=" + std::to_string(after_seq)));
                    const auto items = response.find("items");
                    if (items == response.end() || !items->is_array()) {
                        throw std::runtime_error("server task events response does not contain items");
                    }

                    for (const auto& item : *items) {
                        if (!item.is_object()) {
                            throw std::runtime_error("server task event must be an object");
                        }
                        const auto sequence = item.find("seq");
                        if (sequence != item.end() && sequence->is_number_integer()) {
                            after_seq = std::max(after_seq, sequence->get<int64_t>());
                        }
                        TaskEvent event;
                        event.event_type = EventType(item);
                        event.task_id = task_id;
                        event.message = JsonStringValue(item, "message");
                        event.progress = JsonIntValue(item, "progress");
                        const auto payload = item.find("payload");
                        event.payload = payload == item.end() ? event.message : payload->dump();
                        event.done = IsTerminalTaskEventType(event.event_type);
                        if (event.event_type == "failed" || event.event_type == "error" ||
                            event.event_type == "cancelled" || event.event_type == "timed_out") {
                            event.error = event.message;
                        }
                        events.push_back(std::move(event));
                    }

                    const auto done = response.find("done");
                    if (done != response.end() && done->is_boolean() && done->get<bool>()) {
                        return events;
                    }
                    std::this_thread::sleep_for(std::chrono::milliseconds(config_.task_poll_interval_ms));
                }
            } catch (const std::exception& error) {
                TaskEvent event;
                event.event_type = "error";
                event.task_id = task_id;
                event.message = error.what();
                event.error = error.what();
                event.done = true;
                return std::vector<TaskEvent>{event};
            }
        });
    }

    bool CancelTask(const std::string& task_id) {
        try {
            ValidateIdentifier("task ID", task_id);
            (void)Request("POST", {"tasks", task_id, "cancel"}, "{}", {});
            return true;
        } catch (const std::exception& error) {
            SDK_LOG_ERROR("cancel task failed: " + std::string(error.what()));
            return false;
        }
    }

    void SetSchema(const std::string& function_id, const std::map<std::string, std::string>& schema) {
        ValidateIdentifier("function ID", function_id);
        std::lock_guard<std::mutex> lock(schemas_mutex_);
        schemas_[function_id] = schema;
    }

    void SetReconnectConfig(const ReconnectConfig& config) { reconnect_config_ = config; }

    void SetRetryConfig(const RetryConfig& config) { retry_config_ = config; }

    void Close() {
        connected_ = false;
        closed_ = true;
        std::lock_guard<std::mutex> lock(schemas_mutex_);
        schemas_.clear();
    }

private:
    static void ValidateIdentifier(const char* name, const std::string& value) {
        if (TrimString(value).empty()) {
            throw std::invalid_argument(std::string(name) + " cannot be empty");
        }
    }

    static json ParsePayload(const std::string& payload) {
        try {
            return json::parse(TrimString(payload).empty() ? "{}" : payload);
        } catch (const json::exception& error) {
            throw std::invalid_argument("payload must be valid JSON: " + std::string(error.what()));
        }
    }

    static json ParseResponse(const std::string& body) {
        try {
            const json response = json::parse(body);
            if (!response.is_object()) {
                throw std::runtime_error("server response must be a JSON object");
            }
            return response;
        } catch (const json::exception& error) {
            throw std::runtime_error("server returned invalid JSON: " + std::string(error.what()));
        }
    }

    void ValidatePayload(const std::string& function_id, const std::string& payload) const {
        std::map<std::string, std::string> schema;
        {
            std::lock_guard<std::mutex> lock(schemas_mutex_);
            const auto it = schemas_.find(function_id);
            if (it == schemas_.end()) return;
            schema = it->second;
        }
        const std::string value = TrimString(payload).empty() ? "{}" : payload;
        if (!utils::ValidateJSON(value, schema)) {
            throw std::runtime_error("payload validation failed for function: " + function_id);
        }
    }

    std::map<std::string, std::string> Headers(const InvokeOptions& options) const {
        std::map<std::string, std::string> headers;
        for (const auto& [name, value] : config_.headers) SetHeader(headers, name, value);
        for (const auto& [name, value] : options.metadata) SetHeader(headers, name, value);
        if (!TrimString(options.trace_id).empty() && !HasHeader(headers, "X-Trace-ID")) {
            SetHeader(headers, "X-Trace-ID", options.trace_id);
        }
        if (!TrimString(options.idempotency_key).empty() && !HasHeader(headers, "Idempotency-Key")) {
            SetHeader(headers, "Idempotency-Key", options.idempotency_key);
        }
        if (!TrimString(config_.game_id).empty() && !HasHeader(headers, "X-Game-ID")) {
            SetHeader(headers, "X-Game-ID", config_.game_id);
        }
        if (!TrimString(config_.env).empty() && !HasHeader(headers, "X-Env")) {
            SetHeader(headers, "X-Env", config_.env);
        }
        if (!TrimString(config_.auth_token).empty() && !HasHeader(headers, "Authorization")) {
            const std::string token = TrimString(config_.auth_token);
            SetHeader(headers, "Authorization", LowerString(token).rfind("bearer ", 0) == 0 ? token : "Bearer " + token);
        }
        return headers;
    }

    std::string Endpoint(const std::vector<std::string>& segments, const std::string& query) const {
        std::string endpoint = base_url_;
        for (const auto& segment : segments) endpoint += "/" + EscapeURLSegment(segment);
        return query.empty() ? endpoint : endpoint + "?" + query;
    }

    std::string Request(const std::string& method, const std::vector<std::string>& segments, const std::string& body,
                        const InvokeOptions& options, const std::string& query = "") {
        if (closed_) {
            throw std::runtime_error("invoker is closed");
        }
        const RetryConfig retry = options.retry.has_value() ? *options.retry : retry_config_;
        const int attempts = retry.enabled ? std::max(1, retry.max_attempts) : 1;
        std::exception_ptr last_error;
        for (int attempt = 0; attempt < attempts; ++attempt) {
            try {
                HTTPRequest request;
                request.method = method;
                request.url = Endpoint(segments, query);
                request.headers = Headers(options);
                if (!body.empty()) SetHeader(request.headers, "Content-Type", "application/json");
                request.body = body;
                request.timeout_ms = static_cast<long>((options.timeout_seconds > 0 ? options.timeout_seconds : config_.timeout_seconds) * 1000);
                request.insecure = config_.insecure;
                request.cert_file = config_.cert_file;
                request.key_file = config_.key_file;
                request.ca_file = config_.ca_file;
                request.server_name = config_.server_name;
                const HTTPResponse response = transport_->Send(request);
                if (response.status_code < 200 || response.status_code >= 300) {
                    throw HTTPStatusError(response.status_code, ServerErrorMessage(response.body));
                }
                return response.body;
            } catch (const HTTPStatusError& error) {
                last_error = std::current_exception();
                if (!ShouldRetry(error.status_code(), retry) || attempt + 1 == attempts) break;
            } catch (const std::exception&) {
                last_error = std::current_exception();
                if (attempt + 1 == attempts) break;
            }
            std::this_thread::sleep_for(std::chrono::milliseconds(RetryDelay(attempt, retry)));
        }
        std::rethrow_exception(last_error);
    }

    static bool ShouldRetry(long status_code, const RetryConfig& retry) {
        if (status_code == 408 || status_code == 429 || status_code >= 500) return true;
        return std::find(retry.retryable_status_codes.begin(), retry.retryable_status_codes.end(), status_code) !=
               retry.retryable_status_codes.end();
    }

    static int RetryDelay(int attempt, const RetryConfig& retry) {
        double multiplier = retry.backoff_multiplier > 0 ? retry.backoff_multiplier : 2.0;
        double delay = static_cast<double>(retry.initial_delay_ms) * std::pow(multiplier, attempt);
        if (retry.max_delay_ms > 0) delay = std::min(delay, static_cast<double>(retry.max_delay_ms));
        if (retry.jitter_factor > 0 && delay > 0) {
            std::random_device source;
            std::mt19937 random(source());
            std::uniform_real_distribution<double> jitter(-retry.jitter_factor, retry.jitter_factor);
            delay += delay * jitter(random);
        }
        return static_cast<int>(std::max(0.0, delay));
    }

    InvokerConfig config_;
    std::string base_url_;
    ReconnectConfig reconnect_config_;
    RetryConfig retry_config_;
    std::shared_ptr<HTTPTransport> transport_;
    mutable std::mutex schemas_mutex_;
    std::map<std::string, std::map<std::string, std::string>> schemas_;
    std::atomic<bool> connected_{false};
    std::atomic<bool> closed_{false};
};

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

TaskStatus CroupierInvoker::GetTaskStatus(const std::string& task_id) {
    return impl_->GetTaskStatus(task_id);
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
