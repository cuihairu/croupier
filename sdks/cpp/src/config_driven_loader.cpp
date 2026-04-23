#include "croupier/sdk/config_driven_loader.h"

#include "croupier/sdk/utils/json_utils.h"

#include <fstream>
#include <iostream>
#include <sstream>
#include <stdexcept>

// Platform-specific dynamic library headers
#ifdef _WIN32
#include <windows.h>
#else
#include <dlfcn.h>  // Unix 动态库Load
#endif

// 简单的 JSON 解析器 (生产Environment建议使用 nlohmann/json)
#ifdef CROUPIER_SDK_ENABLE_JSON
#include <nlohmann/json.hpp>
using json = nlohmann::json;
#else
// 简化版 JSON 解析 (仅支持基础Feature)
namespace simple_json {
struct JsonValue {
    std::string str_value;
    std::map<std::string, JsonValue> object_value;
    std::vector<JsonValue> array_value;
    enum class Type : std::uint8_t { STRING, OBJECT, ARRAY } type;

    JsonValue() : type(Type::STRING) {}
    JsonValue(const std::string& s) : str_value(s), type(Type::STRING) {}
};

class JsonParser {
public:
    static JsonValue parse(const std::string& content) {
        // 简化实现：仅支持基础 JSON 结构
        JsonValue result;
        result.type = JsonValue::Type::OBJECT;

        // 示例：解析基本ComponentConfiguration
        if (content.find("\"component\"") != std::string::npos) {
            // 解析Component基础Info
            JsonValue comp;
            comp.type = JsonValue::Type::OBJECT;
            comp.object_value["id"] = JsonValue("demo-component");
            comp.object_value["version"] = JsonValue("1.0.0");
            comp.object_value["name"] = JsonValue("Demo Component");
            result.object_value["component"] = comp;
        }

        return result;
    }
};
}  // namespace simple_json
using json = simple_json::JsonValue;
using JsonParser = simple_json::JsonParser;
#endif

namespace croupier {
namespace sdk {

ConfigDrivenLoader::ConfigDrivenLoader() {
    // Set默认的动态库Load器
    dynamic_lib_loader_ = [this](const std::string& lib_path, const std::string& function_name) -> FunctionHandler {
        return LoadFromDynamicLib(lib_path, function_name);
    };
}

ConfigDrivenLoader::~ConfigDrivenLoader() {
    // 清理资源（如果有动态库句柄需要清理）
}

// ========== Handler器Register机制 ==========

void ConfigDrivenLoader::RegisterHandlerFactory(const std::string& prefix, HandlerFactory factory) {
    std::cout << "📝 RegisterHandler器工厂: " << prefix << '\n';
    handler_factories_[prefix] = std::move(factory);
}

void ConfigDrivenLoader::RegisterHandler(const std::string& function_id, FunctionHandler handler) {
    std::cout << "📝 RegisterFunctionHandler器: " << function_id << '\n';
    registered_handlers_[function_id] = std::move(handler);
}

void ConfigDrivenLoader::SetDynamicLibLoader(DynamicLibLoader loader) {
    std::cout << "📝 Set动态库Load器" << '\n';
    dynamic_lib_loader_ = std::move(loader);
}

// ========== ConfigurationFileLoad ==========

ComponentDescriptor ConfigDrivenLoader::LoadComponentFromFile(const std::string& config_file) {
    std::cout << "📂 从FileLoadComponentConfiguration: " << config_file << '\n';

    std::string content = LoadFileContent(config_file);
    if (content.empty()) {
        throw std::runtime_error("Unable to读取ConfigurationFile: " + config_file);
    }

    return LoadComponentFromJson(content);
}

ComponentDescriptor ConfigDrivenLoader::LoadComponentFromJson(const std::string& json_content) {
    std::cout << "🔄 解析 JSON Configuration..." << '\n';

    try {
#ifdef CROUPIER_SDK_ENABLE_JSON
        json config = json::parse(json_content);
        return ParseJsonToComponent(config.dump());
#else
        auto config = utils::JsonUtils::ParseJson(json_content);
        return ParseJsonToComponent(json_content);
#endif
    } catch (const std::exception& e) {
        throw std::runtime_error("JSON 解析Failed: " + std::string(e.what()));
    }
}

bool ConfigDrivenLoader::LoadAndRegisterComponent(CroupierClient& client, const std::string& config_file) {
    std::cout << "🎯 Load并RegisterComponent: " << config_file << '\n';

    try {
        // 1. LoadComponent描述符
        ComponentDescriptor component = LoadComponentFromFile(config_file);

        // 2. 解析FunctionHandler器
        auto handlers = ResolveHandlers(component);

        // 3. Register handlers with client directly since ComponentDescriptor doesn't store handlers
        for (const auto& [function_id, handler] : handlers) {
            FunctionDescriptor func_desc;
            func_desc.id = function_id;
            func_desc.version = component.version;

            if (!client.RegisterFunction(func_desc, handler)) {
                std::cout << "❌ Failed to register function: " << function_id << '\n';
            }
        }

        // 4. Register component with client
        bool success = client.RegisterComponent(component);

        if (success) {
            std::cout << "✅ ComponentRegisterSuccess: " << component.id << '\n';
        } else {
            std::cout << "❌ ComponentRegisterFailed: " << component.id << '\n';
        }

        return success;
    } catch (const std::exception& e) {
        std::cerr << "💥 LoadComponentFailed: " << e.what() << '\n';
        return false;
    }
}

// ========== Handler器查找 ==========

FunctionHandler ConfigDrivenLoader::GetHandler(const std::string& function_id,
                                               const std::map<std::string, std::string>& config) {
    std::cout << "🔍 查找Handler器: " << function_id << '\n';

    // 1. 首先Check直接Register的Handler器
    auto direct_it = registered_handlers_.find(function_id);
    if (direct_it != registered_handlers_.end()) {
        std::cout << "✅ 找到直接Register的Handler器: " << function_id << '\n';
        return direct_it->second;
    }

    // 2. 尝试通过工厂CreateHandler器
    for (const auto& [prefix, factory] : handler_factories_) {
        if (function_id.find(prefix) == 0) {  // Check前缀匹配
            std::cout << "🏭 使用工厂CreateHandler器: " << prefix << " -> " << function_id << '\n';
            return factory(function_id, config);
        }
    }

    // 3. 尝试从Configuration中CreateHandler器
    auto handler = CreateHandlerFromConfig(function_id, config);
    if (handler) {
        std::cout << "⚙️ 从ConfigurationCreateHandler器: " << function_id << '\n';
        return handler;
    }

    std::cout << "❌ 未找到Handler器: " << function_id << '\n';
    return nullptr;
}

bool ConfigDrivenLoader::HasHandler(const std::string& function_id) const {
    // Check直接Register的Handler器
    if (registered_handlers_.find(function_id) != registered_handlers_.end()) {
        return true;
    }

    // Check工厂前缀匹配
    for (const auto& [prefix, _] : handler_factories_) {
        if (function_id.find(prefix) == 0) {
            return true;
        }
    }

    return false;
}

std::vector<std::string> ConfigDrivenLoader::GetRegisteredHandlers() const {
    std::vector<std::string> result;
    result.reserve(registered_handlers_.size() + handler_factories_.size());

    // 添加直接Register的Handler器
    for (const auto& [id, _] : registered_handlers_) {
        result.push_back(id);
    }

    // 添加工厂前缀
    for (const auto& [prefix, _] : handler_factories_) {
        result.push_back(prefix + ".*");  // 表示前缀匹配
    }

    return result;
}

// ========== ConfigurationValidate ==========

bool ConfigDrivenLoader::ValidateConfigFile(const std::string& config_file) {
    std::cout << "✅ ValidateConfigurationFile: " << config_file << '\n';

    try {
        std::string content = LoadFileContent(config_file);
        return ValidateJsonConfig(content);
    } catch (const std::exception& e) {
        std::cerr << "❌ FileValidateFailed: " << e.what() << '\n';
        return false;
    }
}

bool ConfigDrivenLoader::ValidateJsonConfig(const std::string& json_content) {
    std::cout << "✅ Validate JSON Configuration..." << '\n';

    try {
#ifdef CROUPIER_SDK_ENABLE_JSON
        json config = json::parse(json_content);

        // 基础结构Validate
        if (!config.contains("component")) {
            std::cerr << "❌ 缺少 'component' 字段" << '\n';
            return false;
        }

        auto comp = config["component"];
        if (!comp.contains("id") || !comp.contains("version")) {
            std::cerr << "❌ Component缺少必需字段 (id, version)" << '\n';
            return false;
        }

        // ValidateFunctionDefinition
        if (config.contains("functions")) {
            for (const auto& func : config["functions"]) {
                if (!func.contains("id") || !func.contains("handler")) {
                    std::cerr << "❌ FunctionDefinition缺少必需字段" << '\n';
                    return false;
                }
            }
        }

        return true;
#else
        // 简化Validate：Check基本 JSON 结构
        if (json_content.find("component") == std::string::npos) {
            std::cerr << "❌ 缺少 'component' 字段" << '\n';
            return false;
        }

        return true;
#endif
    } catch (const std::exception& e) {
        std::cerr << "❌ JSON ValidateFailed: " << e.what() << '\n';
        return false;
    }
}

// ========== 内部辅助方法 ==========

std::string ConfigDrivenLoader::LoadFileContent(const std::string& file_path) {
    std::ifstream file(file_path);
    if (!file.is_open()) {
        throw std::runtime_error("Unable to打开File: " + file_path);
    }

    std::stringstream buffer;
    buffer << file.rdbuf();
    return buffer.str();
}

ComponentDescriptor ConfigDrivenLoader::ParseJsonToComponent(const std::string& json_content) {
    ComponentDescriptor component;

#ifdef CROUPIER_SDK_ENABLE_JSON
    // 解析JSON字符串
    auto config = nlohmann::json::parse(json_content);

    // 使用 nlohmann::json 的完整解析
    if (config.contains("component")) {
        auto comp = config["component"];
        component.id = comp.value("id", "");
        component.version = comp.value("version", "1.0.0");
        component.name = comp.value("name", "");
        component.description = comp.value("description", "");
    }

    // 解析Entity（Virtual Object），兼容键名 "entities" 和老的 "virtual_objects"
    auto parse_entities_array = [&](const nlohmann::json& arr) {
        for (const auto& obj_config : arr) {
            VirtualObjectDescriptor obj;
            obj.id = obj_config.value("id", "");
            obj.version = obj_config.value("version", "1.0.0");
            obj.name = obj_config.value("name", "");

            // 解析Operation
            if (obj_config.contains("operations")) {
                for (const auto& [key, value] : obj_config["operations"].items()) {
                    obj.operations[key] = value.get<std::string>();
                }
            }

            // 解析关系
            if (obj_config.contains("relationships")) {
                for (const auto& [rel_name, rel_config] : obj_config["relationships"].items()) {
                    RelationshipDef rel;
                    rel.type = rel_config.value("type", "");
                    rel.entity = rel_config.value("entity", "");
                    rel.foreign_key = rel_config.value("foreign_key", "");
                    obj.relationships[rel_name] = rel;
                }
            }

            // 新版 ComponentDescriptor 使用 vector<VirtualObjectDescriptor> entities;
            component.entities.push_back(std::move(obj));
        }
    };
    if (config.contains("entities") && config["entities"].is_array()) {
        parse_entities_array(config["entities"]);
    } else if (config.contains("virtual_objects") && config["virtual_objects"].is_array()) {
        // 兼容旧字段
        parse_entities_array(config["virtual_objects"]);
    }
#else
    // 简化解析实现
    auto config = utils::JsonUtils::ParseJson(json_content);
    if (config.type == utils::JsonUtils::SimpleJson::OBJECT &&
        config.object_value.find("component") != config.object_value.end()) {
        auto comp = config.object_value.at("component");
        if (comp.object_value.find("id") != comp.object_value.end()) {
            component.id = comp.object_value.at("id").str_value;
        }
        if (comp.object_value.find("version") != comp.object_value.end()) {
            component.version = comp.object_value.at("version").str_value;
        }
        if (comp.object_value.find("name") != comp.object_value.end()) {
            component.name = comp.object_value.at("name").str_value;
        }
    }
#endif

    return component;
}

VirtualObjectDescriptor ConfigDrivenLoader::ParseJsonToVirtualObject(const std::string& json_content) {
    VirtualObjectDescriptor obj;

#ifdef CROUPIER_SDK_ENABLE_JSON
    // 解析JSON字符串
    auto obj_config = nlohmann::json::parse(json_content);

    obj.id = obj_config.value("id", "");
    obj.version = obj_config.value("version", "1.0.0");
    obj.name = obj_config.value("name", "");

    // 解析Operation
    if (obj_config.contains("operations")) {
        for (const auto& [key, value] : obj_config["operations"].items()) {
            obj.operations[key] = value.get<std::string>();
        }
    }

    // 解析关系
    if (obj_config.contains("relationships")) {
        for (const auto& [rel_name, rel_config] : obj_config["relationships"].items()) {
            RelationshipDef rel;
            rel.type = rel_config.value("type", "");
            rel.entity = rel_config.value("entity", "");
            rel.foreign_key = rel_config.value("foreign_key", "");
            obj.relationships[rel_name] = rel;
        }
    }
#else
    // 简化实现
    (void)json_content;  // Suppress unused parameter warning
    obj.id = "simple.object";
    obj.version = "1.0.0";
    obj.name = "Simple Object";
#endif

    return obj;
}

std::map<std::string, FunctionHandler> ConfigDrivenLoader::ResolveHandlers(const ComponentDescriptor& comp) {
    std::map<std::string, FunctionHandler> handlers;

    std::cout << "🔄 Resolving component handlers: " << comp.id << '\n';

    // Traverse virtual object operations to find corresponding handlers
    for (const auto& entity : comp.entities) {
        for (const auto& [op_name, function_id] : entity.operations) {
            std::cout << "  🔍 Finding handler: " << function_id << " (for " << entity.id << "." << op_name << ")"
                      << '\n';

            auto handler = GetHandler(function_id);
            if (handler) {
                handlers[function_id] = handler;
                std::cout << "    ✅ 找到Handler器: " << function_id << '\n';
            } else {
                std::cout << "    ⚠️ 未找到Handler器: " << function_id << "，将使用默认Handler器" << '\n';
                // Create默认Handler器
                handlers[function_id] = CreateDefaultHandler(function_id);
            }
        }
    }

    return handlers;
}

FunctionHandler ConfigDrivenLoader::CreateHandlerFromConfig(const std::string& function_id,
                                                            const std::map<std::string, std::string>& config) {
    (void)function_id;  // Suppress unused parameter warning - function_id not used in basic implementation

    // Check if handler is defined in configuration
    auto type_it = config.find("type");
    if (type_it == config.end()) {
        return nullptr;
    }

    const std::string& handler_type = type_it->second;

    if (handler_type == "echo") {
        return BasicHandlerFactory::CreateEchoHandler(config);
    } else if (handler_type == "error") {
        auto msg_it = config.find("message");
        std::string message = (msg_it != config.end()) ? msg_it->second : "Handler器Error";
        return BasicHandlerFactory::CreateErrorHandler(message);
    } else if (handler_type == "proxy") {
        auto url_it = config.find("target_url");
        if (url_it != config.end()) {
            return BasicHandlerFactory::CreateProxyHandler(url_it->second, config);
        }
    } else if (handler_type == "template") {
        auto template_it = config.find("template");
        if (template_it != config.end()) {
            return BasicHandlerFactory::CreateTemplateHandler(template_it->second, config);
        }
    } else if (handler_type == "dynamic_lib") {
        auto lib_it = config.find("library");
        auto func_it = config.find("function");
        if (lib_it != config.end() && func_it != config.end() && dynamic_lib_loader_) {
            return dynamic_lib_loader_(lib_it->second, func_it->second);
        }
    }

    return nullptr;
}

FunctionHandler ConfigDrivenLoader::CreateDefaultHandler(const std::string& function_id) {
    return [function_id](const std::string& context, const std::string& payload) -> std::string {
        (void)payload;  // Suppress unused parameter warning

        return "{"
               "\"function_id\": \"" +
               function_id +
               "\","
               "\"status\": \"not_implemented\","
               "\"message\": \"This function has no specific implementation logic yet\","
               "\"context\": \"" +
               context +
               "\","
               "\"timestamp\": \"" +
               std::to_string(std::time(nullptr)) +
               "\""
               "}";
    };
}

FunctionHandler ConfigDrivenLoader::LoadFromDynamicLib(const std::string& lib_path, const std::string& function_name) {
    std::cout << "📚 从动态库LoadFunction: " << lib_path << "::" << function_name << '\n';

#ifdef _WIN32
    // Windows 动态库Load
    HMODULE handle = LoadLibraryA(lib_path.c_str());
    if (!handle) {
        std::cerr << "❌ Unable toLoad动态库: " << lib_path << '\n';
        return nullptr;
    }

    typedef const char* (*HandlerFunc)(const char* context, const char* payload);
    HandlerFunc func = nullptr;
#ifdef __GNUC__
#pragma GCC diagnostic push
#pragma GCC diagnostic ignored "-Wcast-function-type"
#endif
    func = reinterpret_cast<HandlerFunc>(GetProcAddress(handle, function_name.c_str()));
#ifdef __GNUC__
#pragma GCC diagnostic pop
#endif
    if (!func) {
        std::cerr << "❌ Unable to找到Function: " << function_name << '\n';
        FreeLibrary(handle);
        return nullptr;
    }
#else
    // Unix 动态库Load
    void* handle = dlopen(lib_path.c_str(), RTLD_LAZY);
    if (!handle) {
        std::cerr << "❌ Unable toLoad动态库: " << lib_path << " - " << dlerror() << '\n';
        return nullptr;
    }

    typedef const char* (*HandlerFunc)(const char* context, const char* payload);
    HandlerFunc func = (HandlerFunc)dlsym(handle, function_name.c_str());
    if (!func) {
        std::cerr << "❌ Unable to找到Function: " << function_name << " - " << dlerror() << '\n';
        dlclose(handle);
        return nullptr;
    }
#endif

    std::cout << "✅ SuccessLoad动态库Function: " << function_name << '\n';

    // Create包装器，将 C 风格Function包装成 std::function
    return [func](const std::string& context, const std::string& payload) -> std::string {
        const char* result = func(context.c_str(), payload.c_str());
        return result ? std::string(result) : "{}";
    };
}

// ========== 预Definition的Handler器工厂 ==========

FunctionHandler BasicHandlerFactory::CreateEchoHandler(const std::map<std::string, std::string>& config) {
    return [config](const std::string& context, const std::string& payload) -> std::string {
        return "{"
               "\"type\": \"echo\","
               "\"context\": \"" +
               context +
               "\","
               "\"payload\": " +
               payload +
               ","
               "\"timestamp\": \"" +
               std::to_string(std::time(nullptr)) +
               "\""
               "}";
    };
}

FunctionHandler BasicHandlerFactory::CreateErrorHandler(const std::string& error_message) {
    return [error_message](const std::string& context, const std::string& payload) -> std::string {
        (void)payload;  // Suppress unused parameter warning - error handler doesn't use payload

        return "{"
               "\"type\": \"error\","
               "\"error_message\": \"" +
               error_message +
               "\","
               "\"context\": \"" +
               context +
               "\","
               "\"timestamp\": \"" +
               std::to_string(std::time(nullptr)) +
               "\""
               "}";
    };
}

FunctionHandler BasicHandlerFactory::CreateProxyHandler(const std::string& target_url,
                                                        const std::map<std::string, std::string>& config) {
    return [target_url, config](const std::string& context, const std::string& payload) -> std::string {
        (void)payload;  // Suppress unused parameter warning - proxy handler placeholder doesn't use payload

        // In a real implementation, this would make HTTP requests to the target URL
        return "{"
               "\"type\": \"proxy\","
               "\"target_url\": \"" +
               target_url +
               "\","
               "\"status\": \"forwarded\","
               "\"context\": \"" +
               context +
               "\","
               "\"timestamp\": \"" +
               std::to_string(std::time(nullptr)) +
               "\""
               "}";
    };
}

FunctionHandler BasicHandlerFactory::CreateTemplateHandler(const std::string& template_string,
                                                           const std::map<std::string, std::string>& config) {
    return [template_string, config](const std::string& context, const std::string& payload) -> std::string {
        // 实际实现中，这里会进行模板替换
        std::string result = template_string;

        // 简单的模板替换：{{context}} 和 {{payload}}
        size_t pos = 0;
        while ((pos = result.find("{{context}}", pos)) != std::string::npos) {
            result.replace(pos, 11, context);
            pos += context.length();
        }

        pos = 0;
        while ((pos = result.find("{{payload}}", pos)) != std::string::npos) {
            result.replace(pos, 11, payload);
            pos += payload.length();
        }

        return result;
    };
}

}  // namespace sdk
}  // namespace croupier
