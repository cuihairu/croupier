// Copyright 2025 Croupier Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

#include "croupier/sdk/bindings/lua_binding_sol2.h"
#include "croupier/sdk/croupier_client.h"

// Use sol/sol.hpp (vcpkg installs sol2 as 'sol' directory)
#include <sol/sol.hpp>
#include <cstring>
#include <sstream>
#include <string>

namespace croupier {
namespace sdk {
namespace lua {

// ============================================================================
// Lua Client - Simplified wrapper for CroupierClient
// ============================================================================

/**
 * @brief Simplified Lua-friendly wrapper for CroupierClient
 *
 * This class provides a simplified API for Lua scripts, handling
 * automatic serialization and providing a cleaner interface.
 */
class LuaClient {
public:
    explicit LuaClient(const std::string& server_address) {
        config_.agent_addr = server_address;
        config_.service_id = "lua-client";
        config_.game_id = "default-game";
        config_.env = "development";
        config_.insecure = true;
        client_ = std::make_unique<CroupierClient>(config_);
    }

    explicit LuaClient(const sol::table& options) {
        config_.agent_addr = options["server"].get_or(std::string("127.0.0.1:19090"));
        config_.service_id = options["service_id"].get_or(std::string("lua-client"));
        config_.game_id = options["game_id"].get_or(std::string("default-game"));
        config_.env = options["env"].get_or(std::string("development"));
        config_.insecure = options["insecure"].get_or(true);
        config_.auth_token = options["credentials"].get_or(std::string(""));

        if (options["timeout"].valid()) {
            config_.timeout_seconds = options["timeout"];
        }
        if (options["auto_reconnect"].valid()) {
            config_.auto_reconnect = options["auto_reconnect"];
        }

        client_ = std::make_unique<CroupierClient>(config_);
    }

    ~LuaClient() = default;

    // Connect to the agent
    bool connect() {
        return client_->Connect();
    }

    // Check if connected
    bool is_connected() const {
        return client_->IsConnected();
    }

    // Register a virtual object with Lua table properties
    bool register_vo(const std::string& vo_id, const std::string& class_id, const sol::table& properties) {
        VirtualObjectDescriptor desc;
        desc.id = vo_id;
        desc.version = "1.0.0";
        desc.name = class_id;
        desc.description = "Virtual object registered from Lua";

        // Convert sol table to properties
        for (const auto& [key, value] : properties) {
            std::string key_str = key.as<std::string>();
            if (value.is<std::string>()) {
                desc.schema[key_str] = value.as<std::string>();
            } else if (value.is<int>()) {
                desc.schema[key_str] = std::to_string(value.as<int>());
            } else if (value.is<double>()) {
                desc.schema[key_str] = std::to_string(value.as<double>());
            } else if (value.is<bool>()) {
                desc.schema[key_str] = value.as<bool>() ? "true" : "false";
            } else if (value.is<sol::table>()) {
                desc.schema[key_str] = serialize_table(value.as<sol::table>());
            }
        }

        // Register with empty handlers map (actual handlers registered separately)
        std::map<std::string, FunctionHandler> empty_handlers;
        return client_->RegisterVirtualObject(desc, empty_handlers);
    }

    // Unregister a virtual object
    bool unregister_vo(const std::string& vo_id) {
        return client_->UnregisterVirtualObject(vo_id);
    }

    // Register a component from file
    bool register_component(const std::string& config_file) {
        return client_->LoadComponentFromFile(config_file);
    }

    // Get registered virtual objects as a Lua table
    sol::table list_vos(sol::this_state L) {
        sol::state_view lua(L);
        sol::table result = lua.create_table();

        auto objects = client_->GetRegisteredObjects();
        for (size_t i = 0; i < objects.size(); ++i) {
            sol::table vo_table = lua.create_table();
            vo_table["id"] = objects[i].id;
            vo_table["version"] = objects[i].version;
            vo_table["name"] = objects[i].name;
            vo_table["description"] = objects[i].description;
            result[i + 1] = vo_table;
        }

        return result;
    }

    // Get components as a Lua table
    sol::table list_components(sol::this_state L) {
        sol::state_view lua(L);
        sol::table result = lua.create_table();

        auto components = client_->GetRegisteredComponents();
        for (size_t i = 0; i < components.size(); ++i) {
            sol::table comp_table = lua.create_table();
            comp_table["id"] = components[i].id;
            comp_table["version"] = components[i].version;
            comp_table["name"] = components[i].name;
            comp_table["description"] = components[i].description;
            result[i + 1] = comp_table;
        }

        return result;
    }

    // Set credentials
    void set_credentials(const std::string& token) {
        config_.auth_token = token;
        // Recreate client with new config
        client_ = std::make_unique<CroupierClient>(config_);
    }

    // Start serving (blocking)
    void serve() {
        client_->Serve();
    }

    // Stop the client
    void stop() {
        client_->Stop();
    }

    // Close the client
    void close() {
        client_->Close();
    }

    // String representation
    std::string to_string() const {
        std::ostringstream oss;
        oss << "croupier.Client(" << config_.agent_addr << ")";
        return oss.str();
    }

private:
    ClientConfig config_;
    std::unique_ptr<CroupierClient> client_;

    // Helper to serialize sol::table to JSON string
    static std::string serialize_table(const sol::table& table) {
        std::ostringstream oss;
        oss << "{";
        bool first = true;
        for (const auto& [key, value] : table) {
            if (!first) oss << ",";
            first = false;

            std::string key_str = key.as<std::string>();
            oss << "\"" << key_str << "\":";

            if (value.is<std::string>()) {
                oss << "\"" << value.as<std::string>() << "\"";
            } else if (value.is<int>()) {
                oss << value.as<int>();
            } else if (value.is<double>()) {
                oss << value.as<double>();
            } else if (value.is<bool>()) {
                oss << (value.as<bool>() ? "true" : "false");
            } else if (value.is<sol::table>()) {
                oss << serialize_table(value.as<sol::table>());
            } else {
                oss << "null";
            }
        }
        oss << "}";
        return oss.str();
    }
};

// ============================================================================
// Lua Invoker - Wrapper for CroupierInvoker with sol2 support
// ============================================================================

/**
 * @brief Lua-friendly wrapper for CroupierInvoker
 */
class LuaInvoker {
public:
    explicit LuaInvoker(const std::string& server_address) {
        config_.address = server_address;
        config_.game_id = "default-game";
        config_.env = "development";
        config_.insecure = true;
        invoker_ = std::make_unique<CroupierInvoker>(config_);
    }

    explicit LuaInvoker(const sol::table& options) {
        config_.address = options["server"].get_or(std::string("127.0.0.1:19090"));
        config_.game_id = options["game_id"].get_or(std::string("default-game"));
        config_.env = options["env"].get_or(std::string("development"));
        config_.insecure = options["insecure"].get_or(true);
        config_.auth_token = options["credentials"].get_or(std::string(""));

        if (options["timeout"].valid()) {
            config_.timeout_seconds = options["timeout"];
        }

        invoker_ = std::make_unique<CroupierInvoker>(config_);
    }

    ~LuaInvoker() = default;

    // Connect to the server
    bool connect() {
        return invoker_->Connect();
    }

    // Invoke a function with automatic table-to-JSON conversion
    std::string invoke(const std::string& function_id, const sol::object& args_obj) {
        std::string payload = "{}";
        if (args_obj.valid() && args_obj.is<sol::table>()) {
            payload = serialize_table(args_obj.as<sol::table>());
        } else if (args_obj.valid() && args_obj.is<std::string>()) {
            payload = args_obj.as<std::string>();
        }

        InvokeOptions options;
        return invoker_->Invoke(function_id, payload, options);
    }

    // Invoke with options table
    std::string invoke_with_options(const std::string& function_id, const sol::table& args,
                                     const sol::table& options_table) {
        std::string payload = serialize_table(args);

        InvokeOptions options;
        if (options_table["timeout"].valid()) {
            options.timeout_seconds = options_table["timeout"];
        }
        if (options_table["idempotency_key"].valid()) {
            options.idempotency_key = options_table["idempotency_key"];
        }
        if (options_table["route"].valid()) {
            options.route = options_table["route"];
        }
        if (options_table["target_service_id"].valid()) {
            options.target_service_id = options_table["target_service_id"];
        }
        if (options_table["trace_id"].valid()) {
            options.trace_id = options_table["trace_id"];
        }

        return invoker_->Invoke(function_id, payload, options);
    }

    // Start a task
    std::string start_job(const std::string& function_id, const sol::object& args_obj) {
        std::string payload = "{}";
        if (args_obj.valid() && args_obj.is<sol::table>()) {
            payload = serialize_table(args_obj.as<sol::table>());
        } else if (args_obj.valid() && args_obj.is<std::string>()) {
            payload = args_obj.as<std::string>();
        }

        InvokeOptions options;
        return invoker_->StartTask(function_id, payload, options);
    }

    // Cancel a task
    bool cancel_job(const std::string& job_id) {
        return invoker_->CancelTask(job_id);
    }

    // Set credentials
    void set_credentials(const std::string& token) {
        config_.auth_token = token;
        invoker_ = std::make_unique<CroupierInvoker>(config_);
    }

    // Close the invoker
    void close() {
        invoker_->Close();
    }

    // String representation
    std::string to_string() const {
        std::ostringstream oss;
        oss << "croupier.Invoker(" << config_.address << ")";
        return oss.str();
    }

private:
    InvokerConfig config_;
    std::unique_ptr<CroupierInvoker> invoker_;

    static std::string serialize_table(const sol::table& table) {
        std::ostringstream oss;
        oss << "{";
        bool first = true;
        for (const auto& [key, value] : table) {
            if (!first) oss << ",";
            first = false;

            std::string key_str = key.as<std::string>();
            oss << "\"" << key_str << "\":";

            if (value.is<std::string>()) {
                oss << "\"" << value.as<std::string>() << "\"";
            } else if (value.is<int>()) {
                oss << value.as<int>();
            } else if (value.is<double>()) {
                oss << value.as<double>();
            } else if (value.is<bool>()) {
                oss << (value.as<bool>() ? "true" : "false");
            } else if (value.is<sol::table>()) {
                oss << serialize_table(value.as<sol::table>());
            } else {
                oss << "null";
            }
        }
        oss << "}";
        return oss.str();
    }
};

// ============================================================================
// Module Entry Point
// ============================================================================

extern "C" {

int luaopen_croupier(lua_State* L) {
    sol::state_view lua(L);

    // Open standard libraries for JSON utility
    lua.open_libraries(sol::lib::base, sol::lib::table, sol::lib::string,
                       sol::lib::math, sol::lib::package);

    // Create module table
    sol::table module = lua.create_table();

    // Register LuaClient usertype
    lua.new_usertype<LuaClient>("Client",
        // Constructors
        sol::constructors<
            LuaClient(const std::string&),
            LuaClient(const sol::table&)
        >(),

        // Methods
        "connect", &LuaClient::connect,
        "is_connected", &LuaClient::is_connected,
        "register_vo", &LuaClient::register_vo,
        "unregister_vo", &LuaClient::unregister_vo,
        "register_component", &LuaClient::register_component,
        "list_vos", &LuaClient::list_vos,
        "list_components", &LuaClient::list_components,
        "set_credentials", &LuaClient::set_credentials,
        "serve", &LuaClient::serve,
        "stop", &LuaClient::stop,
        "close", &LuaClient::close,

        // Aliases for backward compatibility
        "register_virtual_object", &LuaClient::register_vo,
        "unregister_virtual_object", &LuaClient::unregister_vo,
        "list_virtual_objects", &LuaClient::list_vos,

        // Metamethods
        sol::meta_function::to_string, &LuaClient::to_string
    );

    // Register LuaInvoker usertype
    lua.new_usertype<LuaInvoker>("Invoker",
        // Constructors
        sol::constructors<
            LuaInvoker(const std::string&),
            LuaInvoker(const sol::table&)
        >(),

        // Methods
        "connect", &LuaInvoker::connect,
        "invoke", sol::overload(
            static_cast<std::string(LuaInvoker::*)(const std::string&, const sol::object&)>(&LuaInvoker::invoke),
            static_cast<std::string(LuaInvoker::*)(const std::string&, const sol::table&, const sol::table&)>(&LuaInvoker::invoke_with_options)
        ),
        "start_job", &LuaInvoker::start_job,
        "cancel_job", &LuaInvoker::cancel_job,
        "set_credentials", &LuaInvoker::set_credentials,
        "close", &LuaInvoker::close,

        // Metamethods
        sol::meta_function::to_string, &LuaInvoker::to_string
    );

    // Export classes to module
    module["Client"] = lua["Client"];
    module["Invoker"] = lua["Invoker"];

    // Add version info
    module["_VERSION"] = CROUPIER_SDK_VERSION;

    // Return module
    return sol::stack::push(lua, module);
}

} // extern "C"

} // namespace lua
} // namespace sdk
} // namespace croupier
