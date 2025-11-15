#pragma once

#include <string>
#include <map>
#include <functional>
#include <memory>
#include <future>
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
    std::string id;        // function id, e.g. "player.ban"
    std::string version;   // semver, e.g. "1.2.0"
    std::string category;  // grouping category
    std::string risk;      // "low"|"medium"|"high"
    std::string entity;    // entity type, e.g. "item", "player"
    std::string operation; // operation type, e.g. "create", "read", "update", "delete"
    bool enabled = true;   // whether this function is currently enabled
};

// Relationship definition for virtual objects
struct RelationshipDef {
    std::string type;        // "one-to-many", "many-to-one", "many-to-many"
    std::string entity;      // Related entity ID
    std::string foreign_key; // Foreign key field name
};

// Virtual object descriptor
struct VirtualObjectDescriptor {
    std::string id;                              // e.g. "wallet.entity"
    std::string version;                         // Version number
    std::string name;                            // Display name
    std::string description;                     // Description
    std::map<std::string, std::string> schema;   // JSON Schema definition
    std::map<std::string, std::string> operations; // Operation mappings: "read" -> "wallet.get"
    std::map<std::string, RelationshipDef> relationships; // Relationship definitions
};

// Component descriptor (complete module)
struct ComponentDescriptor {
    std::string id;                             // e.g. "economy-system"
    std::string version;                        // Component version
    std::string name;                           // Component name
    std::string description;                    // Component description
    std::vector<VirtualObjectDescriptor> entities;  // Contained entities
    std::vector<FunctionDescriptor> functions;      // Contained functions
    std::map<std::string, std::string> resources;   // UI resource definitions
    std::map<std::string, std::string> config;      // Component configuration
};

// Local function descriptor matching agent/local/v1/local.proto
struct LocalFunctionDescriptor {
    std::string id;      // function id
    std::string version; // function version
};

// Client configuration
struct ClientConfig {
    std::string agent_addr = "127.0.0.1:19090";
    std::string local_listen = "127.0.0.1:0";
    std::string service_id = "cpp-service";
    std::string service_version = "1.0.0";

    // ========== Agent Registration ==========
    std::string agent_id;              // Agent unique identifier (auto-generated if empty)

    // ========== Game Environment Configuration ==========
    std::string game_id;               // Required: Game identifier for backend separation
    std::string env = "development";   // Environment: "development", "staging", "production"

    bool insecure = true; // For development; set false for production with TLS

    // ========== Optional TLS Configuration ==========
    std::string cert_file;         // Client certificate file path
    std::string key_file;          // Client private key file path
    std::string ca_file;           // CA certificate file path
    std::string server_name;       // Server name for TLS verification

    // ========== Authentication ==========
    std::string auth_token;        // Bearer token for authentication
    std::map<std::string, std::string> headers; // Additional headers

    // ========== Timeouts ==========
    int timeout_seconds = 30;     // Connection timeout
    int heartbeat_interval = 60;  // Heartbeat interval in seconds
};

// Invoker configuration
struct InvokerConfig {
    std::string address;              // Server/Agent address

    // ========== Game Environment Configuration ==========
    std::string game_id;              // Required: Game identifier
    std::string env = "development";  // Environment: "development", "staging", "production"

    // ========== TLS Configuration ==========
    bool insecure = true;             // Use insecure connection for development
    std::string cert_file;            // Client certificate file path
    std::string key_file;             // Client private key file path
    std::string ca_file;              // CA certificate file path
    std::string server_name;          // Server name for TLS verification

    // ========== Authentication & Headers ==========
    std::string auth_token;           // Bearer token for authentication
    std::map<std::string, std::string> headers; // Additional request headers

    // ========== Timeouts ==========
    int timeout_seconds = 30;        // Request timeout
};

// Invoke options for function calls
struct InvokeOptions {
    std::string idempotency_key;
    std::string route; // "lb", "broadcast", "targeted", "hash"
    std::string target_service_id;
    std::string hash_key;
    std::string trace_id;
    std::map<std::string, std::string> metadata;
};

// Job event for streaming operations
struct JobEvent {
    std::string event_type;
    std::string job_id;
    std::string payload;
    std::string error;
    bool done = false;
};

// Main SDK client for hosting functions
class CroupierClient {
public:
    explicit CroupierClient(const ClientConfig& config);
    ~CroupierClient();

    // ========== Existing Function Registration (Backward Compatible) ==========

    // Register a function handler with optional schema
    bool RegisterFunction(const FunctionDescriptor& desc, FunctionHandler handler);

    // ========== New Virtual Object Registration ==========

    // Register a virtual object with its associated functions
    bool RegisterVirtualObject(
        const VirtualObjectDescriptor& desc,
        const std::map<std::string, FunctionHandler>& handlers
    );

    // Register a complete component (recommended approach)
    bool RegisterComponent(const ComponentDescriptor& comp);

    // Load and register component from JSON configuration file
    bool LoadComponentFromFile(const std::string& config_file);

    // ========== Management Interface ==========

    // Get list of registered virtual objects
    std::vector<VirtualObjectDescriptor> GetRegisteredObjects() const;

    // Get list of registered components
    std::vector<ComponentDescriptor> GetRegisteredComponents() const;

    // Unregister a virtual object
    bool UnregisterVirtualObject(const std::string& object_id);

    // Unregister a component
    bool UnregisterComponent(const std::string& component_id);

    // ========== Core Operations ==========

    // Connect to agent and start local service
    bool Connect();

    // Start serving (blocking call until Stop() is called)
    void Serve();

    // Stop the client
    void Stop();

    // Close and cleanup
    void Close();

    // Get local server address after starting
    std::string GetLocalAddress() const;

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
    std::string Invoke(const std::string& function_id, const std::string& payload,
                      const InvokeOptions& options = {});

    // Start an async job
    std::string StartJob(const std::string& function_id, const std::string& payload,
                        const InvokeOptions& options = {});

    // Stream job events (returns a future that yields events)
    std::future<std::vector<JobEvent>> StreamJob(const std::string& job_id);

    // Cancel a running job
    bool CancelJob(const std::string& job_id);

    // Set schema for client-side validation
    void SetSchema(const std::string& function_id, const std::map<std::string, std::string>& schema);

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

    // ========== Virtual Object Utilities ==========

    // Load virtual object descriptor from JSON file
    VirtualObjectDescriptor LoadObjectDescriptor(const std::string& file_path);

    // Load component descriptor from JSON file
    ComponentDescriptor LoadComponentDescriptor(const std::string& file_path);

    // Validate virtual object descriptor completeness
    bool ValidateObjectDescriptor(const VirtualObjectDescriptor& desc);

    // Validate component descriptor completeness
    bool ValidateComponentDescriptor(const ComponentDescriptor& comp);

    // Generate default object configuration template
    std::string GenerateObjectTemplate(const std::string& object_id);

    // Generate default component configuration template
    std::string GenerateComponentTemplate(const std::string& component_id);

    // Parse object descriptor from JSON string
    VirtualObjectDescriptor ParseObjectDescriptor(const std::string& json);

    // Parse component descriptor from JSON string
    ComponentDescriptor ParseComponentDescriptor(const std::string& json);

    // Serialize object descriptor to JSON string
    std::string ObjectDescriptorToJSON(const VirtualObjectDescriptor& desc);

    // Serialize component descriptor to JSON string
    std::string ComponentDescriptorToJSON(const ComponentDescriptor& comp);
}

} // namespace sdk
} // namespace croupier

