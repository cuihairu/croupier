#include "croupier/sdk/plugin/dynamic_loader.h"
#include <string>
#include <map>
#include <ctime>
#include <cstring>

using namespace croupier::sdk::plugin;

// ========== Plugin Implementation ==========

// Plugin metadata
static PluginInfo plugin_info = {
    "example_plugin",                    // name
    "1.0.0",                            // version
    "Croupier Team",                    // author
    "Example plugin demonstrating dynamic loading capabilities",  // description
    {"hello", "calculate", "current_time", "echo"},  // provided_functions
    {
        {"language", "C++"},
        {"build_date", __DATE__},
        {"features", "math,time,string"},
        {"license", "MIT"}
    }  // metadata
};

// ========== Exported Plugin Interface Functions ==========

extern "C" {

/**
 * @brief Plugin initialization function
 * Called when the plugin is loaded
 */
int croupier_plugin_init() {
    // Initialize plugin resources
    // Return 0 for success, non-zero for error
    return 0;
}

/**
 * @brief Get plugin information
 * Returns pointer to plugin metadata
 */
PluginInfo* croupier_plugin_info() {
    return &plugin_info;
}

/**
 * @brief Plugin cleanup function
 * Called when the plugin is unloaded
 */
void croupier_plugin_cleanup() {
    // Cleanup plugin resources
}

// ========== Plugin Functions ==========

/**
 * @brief Simple hello function
 * Returns a greeting message with the provided name
 */
const char* hello(const char* context, const char* payload) {
    static std::string result;

    try {
        std::string context_str(context ? context : "");
        std::string payload_str(payload ? payload : "{}");

        // Simple JSON parsing (in real plugin, use proper JSON library)
        std::string name = "World";
        size_t name_pos = payload_str.find("\"name\"");
        if (name_pos != std::string::npos) {
            size_t colon_pos = payload_str.find(":", name_pos);
            if (colon_pos != std::string::npos) {
                size_t quote1 = payload_str.find("\"", colon_pos);
                if (quote1 != std::string::npos) {
                    size_t quote2 = payload_str.find("\"", quote1 + 1);
                    if (quote2 != std::string::npos) {
                        name = payload_str.substr(quote1 + 1, quote2 - quote1 - 1);
                    }
                }
            }
        }

        result = R"({
            "message": "Hello, )" + name + R"(! Greetings from the example plugin.",
            "plugin": "example_plugin",
            "version": "1.0.0",
            "context": ")" + context_str + R"(",
            "timestamp": ")" + std::to_string(std::time(nullptr)) + R"("
        })";

        return result.c_str();
    } catch (...) {
        static std::string error_result = R"({"error": "Hello function failed"})";
        return error_result.c_str();
    }
}

/**
 * @brief Mathematical calculation function
 * Performs basic arithmetic operations
 */
const char* calculate(const char* /* context */, const char* payload) {
    static std::string result;

    try {
        std::string payload_str(payload ? payload : "{}");

        // Parse operation, a, and b from JSON (simplified parsing)
        double a = 0, b = 0;
        std::string operation = "add";

        // Simple parsing for demonstration
        size_t op_pos = payload_str.find("\"operation\"");
        if (op_pos != std::string::npos) {
            size_t colon_pos = payload_str.find(":", op_pos);
            if (colon_pos != std::string::npos) {
                size_t quote1 = payload_str.find("\"", colon_pos);
                if (quote1 != std::string::npos) {
                    size_t quote2 = payload_str.find("\"", quote1 + 1);
                    if (quote2 != std::string::npos) {
                        operation = payload_str.substr(quote1 + 1, quote2 - quote1 - 1);
                    }
                }
            }
        }

        // Parse numbers (simplified)
        size_t a_pos = payload_str.find("\"a\"");
        if (a_pos != std::string::npos) {
            size_t colon_pos = payload_str.find(":", a_pos);
            if (colon_pos != std::string::npos) {
                size_t end_pos = payload_str.find(",", colon_pos);
                if (end_pos == std::string::npos) end_pos = payload_str.find("}", colon_pos);
                if (end_pos != std::string::npos) {
                    std::string num_str = payload_str.substr(colon_pos + 1, end_pos - colon_pos - 1);
                    a = std::stod(num_str);
                }
            }
        }

        size_t b_pos = payload_str.find("\"b\"");
        if (b_pos != std::string::npos) {
            size_t colon_pos = payload_str.find(":", b_pos);
            if (colon_pos != std::string::npos) {
                size_t end_pos = payload_str.find(",", colon_pos);
                if (end_pos == std::string::npos) end_pos = payload_str.find("}", colon_pos);
                if (end_pos != std::string::npos) {
                    std::string num_str = payload_str.substr(colon_pos + 1, end_pos - colon_pos - 1);
                    b = std::stod(num_str);
                }
            }
        }

        // Perform calculation
        double calc_result = 0;
        std::string calc_operation = operation;

        if (operation == "add") {
            calc_result = a + b;
        } else if (operation == "subtract") {
            calc_result = a - b;
        } else if (operation == "multiply") {
            calc_result = a * b;
        } else if (operation == "divide") {
            if (b != 0) {
                calc_result = a / b;
            } else {
                result = R"({"error": "Division by zero", "operation": "divide"})";
                return result.c_str();
            }
        } else {
            result = R"({"error": "Unknown operation", "operation": ")" + operation + R"("})";
            return result.c_str();
        }

        result = R"({
            "result": )" + std::to_string(calc_result) + R"(,
            "operation": ")" + operation + R"(",
            "operands": {"a": )" + std::to_string(a) + R"(, "b": )" + std::to_string(b) + R"(},
            "plugin": "example_plugin",
            "status": "success"
        })";

        return result.c_str();
    } catch (...) {
        static std::string error_result = R"({"error": "Calculate function failed"})";
        return error_result.c_str();
    }
}

/**
 * @brief Current time function
 * Returns current timestamp and formatted time
 */
const char* current_time(const char* /* context */, const char* /* payload */) {
    static std::string result;

    try {
        time_t current_time = std::time(nullptr);
        char* time_str = std::ctime(&current_time);

        // Remove newline from ctime result
        if (time_str && strlen(time_str) > 0) {
            time_str[strlen(time_str) - 1] = '\0';
        }

        result = R"({
            "timestamp": )" + std::to_string(current_time) + R"(,
            "formatted_time": ")" + (time_str ? time_str : "Unknown") + R"(",
            "timezone": "local",
            "plugin": "example_plugin",
            "function": "time"
        })";

        return result.c_str();
    } catch (...) {
        static std::string error_result = R"({"error": "Time function failed"})";
        return error_result.c_str();
    }
}

/**
 * @brief Echo function
 * Returns the input payload with additional metadata
 */
const char* echo(const char* context, const char* payload) {
    static std::string result;

    try {
        std::string context_str(context ? context : "");
        std::string payload_str(payload ? payload : "{}");

        result = R"({
            "echo": {
                "context": ")" + context_str + R"(",
                "payload": )" + payload_str + R"(
            },
            "plugin": "example_plugin",
            "function": "echo",
            "timestamp": ")" + std::to_string(std::time(nullptr)) + R"(",
            "status": "echoed"
        })";

        return result.c_str();
    } catch (...) {
        static std::string error_result = R"({"error": "Echo function failed"})";
        return error_result.c_str();
    }
}

} // extern "C"