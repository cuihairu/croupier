#pragma once

#include <functional>
#include <future>
#include <map>
#include <memory>
#include <optional>
#include <string>
#include <vector>

namespace croupier {
namespace sdk {

// Forward declarations
class CroupierClient;
class CroupierInvoker;

// Function handler type
using FunctionHandler = std::function<std::string(const std::string& context, const std::string& payload)>;

// Function descriptor matching proto definition (control.proto)
struct FunctionDescriptor {
    std::string id;                // function id, e.g. "player.ban"
    std::string version;           // semver, e.g. "1.2.0"
    std::vector<std::string> tags; // function tags
    std::string summary;           // short summary
    std::string description;       // long description
    std::string operation_id;      // stable operation identifier
    bool deprecated = false;       // whether this function is deprecated
    std::string input_schema;      // JSON schema for input payload
    std::string output_schema;     // JSON schema for output payload

    std::string resource;    // business resource/capability key
    std::string operation;   // business action key, e.g. "ban", "send", "list"
    std::string capability;  // collection_query|item_query|create|update|delete|action|task|report
    std::string execution;   // sync|task
    bool approval_required = false;  // whether an approval workflow must complete before execution
    std::string approval_policy_key; // optional approval workflow key
    std::string risk;        // "safe"|"warning"|"high"|"danger"
    bool enabled = true;     // whether this function is currently enabled
    std::string permission;  // optional permission identifier
};

// Local function descriptor matching agent/local/v1/local.proto
struct ProviderFunctionDescriptor {
    std::string id;       // function id
    std::string version;  // function version
};

// Client configuration
struct ClientConfig {
    std::string agent_addr = "127.0.0.1:19091";
    std::string service_id;  // No default - must be explicitly set
    std::string service_version = "1.0.0";
    std::string control_addr;  // optional control-plane endpoint
    std::string provider_lang = "cpp";
    std::string provider_sdk = "croupier-cpp-sdk";

    // ========== Connection Resiliency ==========
    // When true, the SDK will keep trying to reconnect to the Agent and re-register on reconnect.
    bool auto_reconnect = true;
    // Seconds between reconnect attempts (default: 5s).
    int reconnect_interval_seconds = 5;
    // Max reconnect attempts (0 = unlimited).
    int reconnect_max_attempts = 0;

    // ========== Agent Registration ==========
    std::string agent_id;  // Agent unique identifier (auto-generated if empty)

    // ========== Game Environment Configuration ==========
    std::string game_id;              // Required: Game identifier for backend separation
    std::string env = "development";  // Environment: "development", "staging", "production"

    bool insecure = true;  // For development; set false for production with TLS

    // ========== Optional TLS Configuration ==========
    std::string cert_file;    // Client certificate file path
    std::string key_file;     // Client private key file path
    std::string ca_file;      // CA certificate file path
    std::string server_name;  // Server name for TLS verification

    // ========== Authentication ==========
    std::string auth_token;                      // Bearer token for authentication
    std::map<std::string, std::string> headers;  // Additional headers

    // ========== Timeouts ==========
    int timeout_seconds = 30;     // Connection timeout (for blocking connect)
    int heartbeat_interval = 60;  // Heartbeat interval in seconds

    // ========== Connection Mode ==========
    // When true (default), Connect() blocks until connection is established or timeout.
    // When false, Connect() returns immediately and connection proceeds in background.
    // Use non-blocking mode in game servers to prevent startup delays when agent is unavailable.
    bool blocking_connect = true;

    // Timeout for initial connection attempt in non-blocking mode (default: 5s).
    // Shorter than timeout_seconds since retries happen in background via auto_reconnect.
    int connect_timeout_seconds = 5;

    // ========== Logging Configuration ==========
    bool disable_logging = false;    // Disable all logging
    bool debug_logging = false;      // Enable debug level logging
    std::string log_level = "INFO";  // Log level: "DEBUG", "INFO", "WARN", "ERROR", "OFF"

    // ========== File Transfer Configuration (SECURITY SENSITIVE) ==========
    // File transfer is DISABLED by default for security reasons.
    // Enabling requires explicit configuration and security review.
    bool enable_file_transfer = false;            // Enable file transfer functionality (default: false)
    int max_file_size = 10485760;                 // Max file size in bytes (default: 10MB)
    std::vector<std::string> allowed_extensions;  // Allowed file extensions (whitelist, e.g., ".png", ".jpg")
    std::vector<std::string> allowed_mime_types;  // Allowed MIME types (whitelist, e.g., "image/png")
    int upload_timeout = 300000;                  // Upload timeout in milliseconds (default: 5 minutes)
};

// Reconnection configuration with exponential backoff
struct ReconnectConfig {
    bool enabled = true;              // Enable automatic reconnection
    int max_attempts = 0;             // Max reconnection attempts (0 = infinite)
    int initial_delay_ms = 1000;      // Initial reconnection delay in milliseconds
    int max_delay_ms = 30000;         // Maximum reconnection delay in milliseconds
    double backoff_multiplier = 2.0;  // Exponential backoff multiplier
    double jitter_factor = 0.2;       // Jitter factor (0-1) to add randomness
};

// Retry configuration with exponential backoff
struct RetryConfig {
    bool enabled = true;                                           // Enable retry on failure
    int max_attempts = 3;                                          // Max retry attempts
    int initial_delay_ms = 100;                                    // Initial retry delay in milliseconds
    int max_delay_ms = 5000;                                       // Maximum retry delay in milliseconds
    double backoff_multiplier = 2.0;                               // Exponential backoff multiplier
    double jitter_factor = 0.1;                                    // Jitter factor (0-1) to add randomness
    std::vector<int> retryable_status_codes = {14, 13, 2, 10, 4};  // gRPC status codes
};

// Invoker configuration
struct InvokerConfig {
    std::string address;  // Server/Agent address

    // ========== Game Environment Configuration ==========
    std::string game_id;              // Required: Game identifier
    std::string env = "development";  // Environment: "development", "staging", "production"

    // ========== TLS Configuration ==========
    bool insecure = true;     // Use insecure connection for development
    std::string cert_file;    // Client certificate file path
    std::string key_file;     // Client private key file path
    std::string ca_file;      // CA certificate file path
    std::string server_name;  // Server name for TLS verification

    // ========== Authentication & Headers ==========
    std::string auth_token;                      // Bearer token for authentication
    std::map<std::string, std::string> headers;  // Additional request headers

    // ========== Timeouts ==========
    int timeout_seconds = 30;       // Request timeout
    int connect_timeout_seconds = 5;  // Connection timeout

    // ========== Retry Configuration ==========
    RetryConfig retry;  // Retry configuration

    // ========== Logging Configuration ==========
    bool disable_logging = false;    // Disable all logging
    bool debug_logging = false;      // Enable debug level logging
    std::string log_level = "INFO";  // Log level: "DEBUG", "INFO", "WARN", "ERROR", "OFF"

    // ========== File Transfer Configuration (SECURITY SENSITIVE) ==========
    // File transfer is DISABLED by default for security reasons.
    // Enabling requires explicit configuration and security review.
    bool enable_file_transfer = false;            // Enable file transfer functionality (default: false)
    int max_file_size = 10485760;                 // Max file size in bytes (default: 10MB)
    std::vector<std::string> allowed_extensions;  // Allowed file extensions (whitelist, e.g., ".png", ".jpg")
    std::vector<std::string> allowed_mime_types;  // Allowed MIME types (whitelist, e.g., "image/png")
    int upload_timeout = 300000;                  // Upload timeout in milliseconds (default: 5 minutes)
};

// Invoke options for function calls
struct InvokeOptions {
    // Optional per-request override; when 0, uses the invoker's default timeout.
    int timeout_seconds = 0;
    std::string idempotency_key;
    std::string route;  // "lb", "broadcast", "targeted", "hash"
    std::string target_service_id;
    std::string hash_key;
    std::string trace_id;
    std::map<std::string, std::string> metadata;
    std::optional<RetryConfig> retry;  // Retry configuration override
};

// Task event for streaming operations
struct TaskEvent {
    std::string event_type;
    std::string task_id;
    std::string message;
    int progress = 0;
    std::string payload;
    std::string error;
    bool done = false;
};

// Main SDK client for hosting functions
class CroupierClient {
public:
    explicit CroupierClient(const ClientConfig& config);
    ~CroupierClient();

    // ========== Function Registration ==========

    // Register a function handler with optional schema
    bool RegisterFunction(const FunctionDescriptor& desc, FunctionHandler handler);

    // ========== Core Operations ==========

    // Connect to agent and start local service
    bool Connect();

    // Check if the client is connected to the agent
    bool IsConnected() const;

    // Start serving (blocking call until Stop() is called)
    void Serve();

    // Stop the client
    void Stop();

    // Close and cleanup
    void Close();

private:
    class Impl;
    std::unique_ptr<Impl> impl_;
};

// SDK invoker for calling remote functions
class CroupierInvoker {
public:
    explicit CroupierInvoker(const InvokerConfig& config);
    ~CroupierInvoker();

    // Connect to server/agent
    bool Connect();

    // Invoke a function synchronously
    std::string Invoke(const std::string& function_id, const std::string& payload, const InvokeOptions& options = {});

    // Start an async task
    std::string StartTask(const std::string& function_id, const std::string& payload, const InvokeOptions& options = {});

    // Stream task events (returns a future that yields events)
    std::future<std::vector<TaskEvent>> StreamTask(const std::string& task_id);

    // Cancel a running task
    bool CancelTask(const std::string& task_id);

    // Set schema for client-side validation
    void SetSchema(const std::string& function_id, const std::map<std::string, std::string>& schema);

    // Set reconnection configuration
    void SetReconnectConfig(const ReconnectConfig& config);

    // Set retry configuration
    void SetRetryConfig(const RetryConfig& config);

    // Close connection
    void Close();

private:
    class Impl;
    std::unique_ptr<Impl> impl_;
};

// Utility functions
namespace utils {
// Generate idempotency key
std::string NewIdempotencyKey();

// Validate JSON against simple schema (basic validation)
bool ValidateJSON(const std::string& json, const std::map<std::string, std::string>& schema);

// Parse JSON to map (simplified)
std::map<std::string, std::string> ParseJSON(const std::string& json);

// Serialize map to JSON
std::string ToJSON(const std::map<std::string, std::string>& data);
}  // namespace utils

}  // namespace sdk
}  // namespace croupier
