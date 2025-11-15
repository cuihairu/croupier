#include "croupier/sdk/config_manager.h"
#include <fstream>
#include <sstream>
#include <iostream>
#include <regex>

#ifdef CROUPIER_SDK_ENABLE_JSON
#include <nlohmann/json.hpp>
using json = nlohmann::json;
#endif

// 跨平台文件系统支持
#ifdef _WIN32
#include <direct.h>
#include <windows.h>
#define mkdir(path, mode) _mkdir(path)
#else
#include <sys/stat.h>
#include <dirent.h>
#include <unistd.h>
#endif

namespace croupier {
namespace sdk {

ConfigManager::ConfigManager() {
    std::cout << "📋 配置管理器已初始化" << std::endl;
}

ConfigManager::~ConfigManager() {
    // 清理资源
}

// ========== 客户端配置加载 ==========

ClientConfig ConfigManager::LoadClientConfig(const std::string& config_file) {
    std::cout << "📂 从文件加载客户端配置: " << config_file << std::endl;

    std::string content = LoadFileContent(config_file);
    if (content.empty()) {
        throw std::runtime_error("无法读取配置文件: " + config_file);
    }

    return LoadClientConfigFromJson(content);
}

ClientConfig ConfigManager::LoadClientConfigFromJson(const std::string& json_content) {
    std::cout << "🔄 解析客户端配置 JSON..." << std::endl;

#ifdef CROUPIER_SDK_ENABLE_JSON
    try {
        json config = json::parse(json_content);
        return ParseClientConfigFromJson(config);
    } catch (const std::exception& e) {
        throw std::runtime_error("客户端配置 JSON 解析失败: " + std::string(e.what()));
    }
#else
    throw std::runtime_error("需要 nlohmann::json 支持才能解析配置文件");
#endif
}

// ========== Schema 管理 ==========

ConfigManager::VirtualObjectSchema ConfigManager::LoadVirtualObjectSchema(const std::string& schema_file) {
    std::cout << "📋 加载虚拟对象 Schema: " << schema_file << std::endl;

    std::string content = LoadFileContent(schema_file);
    if (content.empty()) {
        throw std::runtime_error("无法读取 Schema 文件: " + schema_file);
    }

#ifdef CROUPIER_SDK_ENABLE_JSON
    try {
        json schema_json = json::parse(content);
        return ParseSchemaFromJson(schema_json);
    } catch (const std::exception& e) {
        throw std::runtime_error("Schema JSON 解析失败: " + std::string(e.what()));
    }
#else
    throw std::runtime_error("需要 nlohmann::json 支持才能解析 Schema 文件");
#endif
}

bool ConfigManager::ValidateDataAgainstSchema(const VirtualObjectSchema& schema, const std::string& data) {
    std::cout << "✅ 验证数据是否符合 Schema: " << schema.id << std::endl;

#ifdef CROUPIER_SDK_ENABLE_JSON
    try {
        json data_json = json::parse(data);

        // 验证必需字段
        for (const auto& [field_name, field_schema] : schema.fields) {
            if (field_schema.required && !data_json.contains(field_name)) {
                std::cerr << "❌ 缺少必需字段: " << field_name << std::endl;
                return false;
            }

            if (data_json.contains(field_name)) {
                // 验证字段类型
                const auto& value = data_json[field_name];
                if (!ValidateFieldType(value, field_schema)) {
                    std::cerr << "❌ 字段类型不匹配: " << field_name << std::endl;
                    return false;
                }

                // 验证自定义规则
                if (!ValidateFieldRules(value, field_schema)) {
                    std::cerr << "❌ 字段验证规则失败: " << field_name << std::endl;
                    return false;
                }
            }
        }

        std::cout << "✅ 数据验证通过" << std::endl;
        return true;
    } catch (const std::exception& e) {
        std::cerr << "❌ 数据验证异常: " << e.what() << std::endl;
        return false;
    }
#else
    std::cout << "⚠️ 跳过 Schema 验证（需要 nlohmann::json 支持）" << std::endl;
    return true;
#endif
}

// ========== 完整配置加载 ==========

ConfigManager::ApplicationConfig ConfigManager::LoadApplicationConfig(const std::string& config_dir) {
    std::cout << "📁 从目录加载应用配置: " << config_dir << std::endl;

    ApplicationConfig app_config;

    // 1. 加载主客户端配置
    std::string client_config_file = config_dir + "/client.json";
    try {
        app_config.client_config = LoadClientConfig(client_config_file);
        std::cout << "✅ 客户端配置加载成功" << std::endl;
    } catch (const std::exception& e) {
        std::cout << "⚠️ 客户端配置加载失败，使用默认配置: " << e.what() << std::endl;
        app_config.client_config = CreateDefaultClientConfig();
    }

    // 2. 加载组件配置
    auto component_files = ListFiles(config_dir + "/components", ".json");
    for (const auto& file : component_files) {
        try {
            ConfigDrivenLoader loader;
            ComponentDescriptor comp = loader.LoadComponentFromFile(file);
            app_config.components.push_back(comp);
            std::cout << "✅ 组件配置加载成功: " << comp.id << std::endl;
        } catch (const std::exception& e) {
            std::cout << "⚠️ 组件配置加载失败: " << file << " - " << e.what() << std::endl;
        }
    }

    // 3. 加载 Schema 文件
    auto schema_files = ListFiles(config_dir + "/schemas", ".json");
    for (const auto& file : schema_files) {
        try {
            VirtualObjectSchema schema = LoadVirtualObjectSchema(file);
            app_config.schemas[schema.id] = schema;
            std::cout << "✅ Schema 加载成功: " << schema.id << std::endl;
        } catch (const std::exception& e) {
            std::cout << "⚠️ Schema 加载失败: " << file << " - " << e.what() << std::endl;
        }
    }

    // 4. 加载全局设置
    std::string global_config_file = config_dir + "/global.json";
    try {
        std::string content = LoadFileContent(global_config_file);
#ifdef CROUPIER_SDK_ENABLE_JSON
        json global_json = json::parse(content);
        for (const auto& [key, value] : global_json.items()) {
            if (value.is_string()) {
                app_config.global_settings[key] = value.get<std::string>();
            }
        }
#endif
        std::cout << "✅ 全局设置加载成功" << std::endl;
    } catch (const std::exception& e) {
        std::cout << "⚠️ 全局设置加载失败: " << e.what() << std::endl;
    }

    return app_config;
}

ConfigManager::ApplicationConfig ConfigManager::LoadApplicationConfigFromFile(const std::string& config_file) {
    std::cout << "📄 从单文件加载应用配置: " << config_file << std::endl;

    ApplicationConfig app_config;

#ifdef CROUPIER_SDK_ENABLE_JSON
    try {
        std::string content = LoadFileContent(config_file);
        json config_json = json::parse(content);

        // 加载客户端配置
        if (config_json.contains("client")) {
            app_config.client_config = ParseClientConfigFromJson(config_json["client"]);
        }

        // 加载组件
        if (config_json.contains("components")) {
            for (const auto& comp_config : config_json["components"]) {
                if (comp_config.contains("config_file")) {
                    ConfigDrivenLoader loader;
                    ComponentDescriptor comp = loader.LoadComponentFromFile(comp_config["config_file"]);
                    app_config.components.push_back(comp);
                } else {
                    ComponentDescriptor comp = ParseComponentFromJson(comp_config);
                    app_config.components.push_back(comp);
                }
            }
        }

        // 加载 Schema
        if (config_json.contains("schemas")) {
            for (const auto& schema_config : config_json["schemas"]) {
                VirtualObjectSchema schema;
                if (schema_config.contains("file")) {
                    schema = LoadVirtualObjectSchema(schema_config["file"]);
                } else {
                    schema = ParseSchemaFromJson(schema_config);
                }
                app_config.schemas[schema.id] = schema;
            }
        }

        // 加载全局设置
        if (config_json.contains("global")) {
            for (const auto& [key, value] : config_json["global"].items()) {
                if (value.is_string()) {
                    app_config.global_settings[key] = value.get<std::string>();
                }
            }
        }

        std::cout << "✅ 应用配置加载成功" << std::endl;
        return app_config;
    } catch (const std::exception& e) {
        throw std::runtime_error("应用配置加载失败: " + std::string(e.what()));
    }
#else
    throw std::runtime_error("需要 nlohmann::json 支持才能加载应用配置");
#endif
}

// ========== 配置验证 ==========

std::vector<std::string> ConfigManager::ValidateClientConfig(const ClientConfig& config) {
    std::vector<std::string> errors;

    if (config.game_id.empty()) {
        errors.push_back("game_id 不能为空");
    }

    if (config.agent_addr.empty()) {
        errors.push_back("agent_addr 不能为空");
    }

    if (config.timeout_seconds <= 0) {
        errors.push_back("timeout_seconds 必须大于 0");
    }

    // 验证地址格式
    std::regex addr_pattern(R"(^.+:\d+$)");
    if (!std::regex_match(config.agent_addr, addr_pattern)) {
        errors.push_back("agent_addr 格式不正确，应为 host:port");
    }

    if (!config.local_listen.empty() && !std::regex_match(config.local_listen, addr_pattern)) {
        errors.push_back("local_listen 格式不正确，应为 host:port");
    }

    // 验证 TLS 配置
    if (!config.insecure) {
        if (config.cert_file.empty()) {
            errors.push_back("启用 TLS 时，cert_file 不能为空");
        }
        if (config.key_file.empty()) {
            errors.push_back("启用 TLS 时，key_file 不能为空");
        }
        if (config.ca_file.empty()) {
            errors.push_back("启用 TLS 时，ca_file 不能为空");
        }
    }

    return errors;
}

std::vector<std::string> ConfigManager::ValidateApplicationConfig(const ApplicationConfig& app_config) {
    std::vector<std::string> errors;

    // 验证客户端配置
    auto client_errors = ValidateClientConfig(app_config.client_config);
    errors.insert(errors.end(), client_errors.begin(), client_errors.end());

    // 验证组件配置
    for (size_t i = 0; i < app_config.components.size(); ++i) {
        const auto& comp = app_config.components[i];
        std::string prefix = "组件[" + std::to_string(i) + "]: ";

        if (comp.id.empty()) {
            errors.push_back(prefix + "组件 ID 不能为空");
        }
        if (comp.version.empty()) {
            errors.push_back(prefix + "组件版本不能为空");
        }
    }

    // 验证 Schema 定义
    for (const auto& [schema_id, schema] : app_config.schemas) {
        std::string prefix = "Schema[" + schema_id + "]: ";

        if (schema.id.empty()) {
            errors.push_back(prefix + "Schema ID 不能为空");
        }
        if (schema.version.empty()) {
            errors.push_back(prefix + "Schema 版本不能为空");
        }

        // 验证字段定义
        for (const auto& [field_name, field] : schema.fields) {
            if (field.type.empty()) {
                errors.push_back(prefix + "字段[" + field_name + "]类型不能为空");
            }
            if (field.type != "string" && field.type != "int" && field.type != "float" &&
                field.type != "bool" && field.type != "object" && field.type != "array") {
                errors.push_back(prefix + "字段[" + field_name + "]类型无效: " + field.type);
            }
        }
    }

    return errors;
}

// ========== 配置生成 ==========

bool ConfigManager::GenerateExampleConfigs(const std::string& output_dir) {
    std::cout << "📁 生成示例配置文件到: " << output_dir << std::endl;

    try {
        // 创建目录结构
        CreateDirectory(output_dir);
        CreateDirectory(output_dir + "/components");
        CreateDirectory(output_dir + "/schemas");

#ifdef CROUPIER_SDK_ENABLE_JSON
        // 1. 生成客户端配置
        json client_config = GenerateExampleClientConfigJson();
        std::ofstream client_file(output_dir + "/client.json");
        client_file << client_config.dump(2) << std::endl;
        client_file.close();

        // 2. 生成组件配置
        json component_config = GenerateExampleComponentJson();
        std::ofstream comp_file(output_dir + "/components/economy.json");
        comp_file << component_config.dump(2) << std::endl;
        comp_file.close();

        // 3. 生成 Schema 配置
        json schema_config = GenerateExampleSchemaJson();
        std::ofstream schema_file(output_dir + "/schemas/wallet_schema.json");
        schema_file << schema_config.dump(2) << std::endl;
        schema_file.close();

        // 4. 生成主配置文件
        json main_config = {
            {"client", {
                {"config_file", "./client.json"}
            }},
            {"components", {{
                {"id", "economy-system"},
                {"version", "1.0.0"},
                {"config_file", "./components/economy.json"},
                {"schema_file", "./schemas/wallet_schema.json"}
            }}},
            {"global", {
                {"log_level", "info"},
                {"metrics_enabled", true},
                {"health_check_port", 8080}
            }}
        };

        std::ofstream main_file(output_dir + "/app_config.json");
        main_file << main_config.dump(2) << std::endl;
        main_file.close();

        // 5. 生成 README
        std::ofstream readme_file(output_dir + "/README.md");
        readme_file << R"(# Croupier C++ SDK 配置示例

## 文件结构

- `client.json` - 客户端连接配置
- `components/` - 组件定义目录
- `schemas/` - 虚拟对象 Schema 目录
- `app_config.json` - 主配置文件（包含所有配置的引用）

## 使用方法

```cpp
#include "croupier/sdk/config_manager.h"

// 从目录加载
ConfigManager manager;
auto config = manager.LoadApplicationConfig("./configs");

// 从单文件加载
auto config = manager.LoadApplicationConfigFromFile("./configs/app_config.json");
```

## 配置验证

```cpp
auto errors = manager.ValidateApplicationConfig(config);
if (!errors.empty()) {
    for (const auto& error : errors) {
        std::cerr << "配置错误: " << error << std::endl;
    }
}
```
)";
        readme_file.close();

        std::cout << "✅ 示例配置生成成功！" << std::endl;
        return true;
#else
        std::cerr << "❌ 需要 nlohmann::json 支持才能生成配置文件" << std::endl;
        return false;
#endif
    } catch (const std::exception& e) {
        std::cerr << "❌ 生成示例配置失败: " << e.what() << std::endl;
        return false;
    }
}

// ========== 内部辅助方法 ==========

std::string ConfigManager::LoadFileContent(const std::string& file_path) {
    std::ifstream file(file_path);
    if (!file.is_open()) {
        throw std::runtime_error("无法打开文件: " + file_path);
    }

    std::stringstream buffer;
    buffer << file.rdbuf();
    return buffer.str();
}

std::vector<std::string> ConfigManager::ListFiles(const std::string& directory, const std::string& extension) {
    std::vector<std::string> files;

#ifdef _WIN32
    WIN32_FIND_DATAA findFileData;
    std::string pattern = directory + "/*" + extension;
    HANDLE hFind = FindFirstFileA(pattern.c_str(), &findFileData);

    if (hFind != INVALID_HANDLE_VALUE) {
        do {
            if (!(findFileData.dwFileAttributes & FILE_ATTRIBUTE_DIRECTORY)) {
                files.push_back(directory + "/" + findFileData.cFileName);
            }
        } while (FindNextFileA(hFind, &findFileData) != 0);
        FindClose(hFind);
    }
#else
    DIR* dir = opendir(directory.c_str());
    if (dir != nullptr) {
        struct dirent* entry;
        while ((entry = readdir(dir)) != nullptr) {
            std::string filename = entry->d_name;
            if (filename.length() >= extension.length() &&
                filename.substr(filename.length() - extension.length()) == extension) {
                files.push_back(directory + "/" + filename);
            }
        }
        closedir(dir);
    }
#endif

    return files;
}

bool ConfigManager::CreateDirectory(const std::string& path) {
#ifdef _WIN32
    return CreateDirectoryA(path.c_str(), nullptr) || GetLastError() == ERROR_ALREADY_EXISTS;
#else
    return mkdir(path.c_str(), 0755) == 0 || errno == EEXIST;
#endif
}

#ifdef CROUPIER_SDK_ENABLE_JSON
ClientConfig ConfigManager::ParseClientConfigFromJson(const nlohmann::json& json) {
    ClientConfig config;

    config.game_id = json.value("game_id", "");
    config.env = json.value("env", "development");
    config.service_id = json.value("service_id", "cpp-service");
    config.agent_addr = json.value("agent_addr", "127.0.0.1:19090");
    config.local_listen = json.value("local_listen", "127.0.0.1:0");
    config.insecure = json.value("insecure", true);
    config.timeout_seconds = json.value("timeout_seconds", 30);

    // 安全配置
    if (json.contains("security")) {
        const auto& security = json["security"];
        config.cert_file = security.value("cert_file", "");
        config.key_file = security.value("key_file", "");
        config.ca_file = security.value("ca_file", "");
        config.server_name = security.value("server_name", "");
    }

    // 认证配置
    if (json.contains("auth")) {
        const auto& auth = json["auth"];
        config.auth_token = auth.value("token", "");

        if (auth.contains("headers")) {
            for (const auto& [key, value] : auth["headers"].items()) {
                if (value.is_string()) {
                    config.headers[key] = value.get<std::string>();
                }
            }
        }
    }

    return config;
}

ConfigManager::VirtualObjectSchema ConfigManager::ParseSchemaFromJson(const nlohmann::json& json) {
    VirtualObjectSchema schema;

    // 基础信息
    if (json.contains("schema")) {
        const auto& schema_info = json["schema"];
        schema.id = schema_info.value("id", "");
        schema.version = schema_info.value("version", "1.0.0");
        schema.name = schema_info.value("name", "");
        schema.description = schema_info.value("description", "");
    }

    // 字段定义
    if (json.contains("fields")) {
        for (const auto& [field_name, field_json] : json["fields"].items()) {
            VirtualObjectSchema::FieldSchema field;
            field.name = field_name;
            field.type = field_json.value("type", "string");
            field.required = field_json.value("required", false);
            field.default_value = field_json.value("default_value", "");
            field.description = field_json.value("description", "");

            if (field_json.contains("validation")) {
                for (const auto& [rule_name, rule_value] : field_json["validation"].items()) {
                    if (rule_value.is_string()) {
                        field.validation[rule_name] = rule_value.get<std::string>();
                    }
                }
            }

            schema.fields[field_name] = field;
        }
    }

    // 操作定义
    if (json.contains("operations")) {
        for (const auto& [op_name, func_id] : json["operations"].items()) {
            if (func_id.is_string()) {
                schema.operations[op_name] = func_id.get<std::string>();
            }
        }
    }

    // 关系定义
    if (json.contains("relationships")) {
        for (const auto& [rel_name, rel_json] : json["relationships"].items()) {
            RelationshipDef rel;
            rel.type = rel_json.value("type", "");
            rel.entity = rel_json.value("entity", "");
            rel.foreign_key = rel_json.value("foreign_key", "");
            schema.relationships[rel_name] = rel;
        }
    }

    return schema;
}

ComponentDescriptor ConfigManager::ParseComponentFromJson(const nlohmann::json& json) {
    ComponentDescriptor comp;

    if (json.contains("component")) {
        const auto& comp_info = json["component"];
        comp.id = comp_info.value("id", "");
        comp.version = comp_info.value("version", "1.0.0");
        comp.name = comp_info.value("name", "");
        comp.description = comp_info.value("description", "");
    }

    // 这里可以添加更多解析逻辑...

    return comp;
}

nlohmann::json ConfigManager::GenerateExampleClientConfigJson() {
    return json{
        {"game_id", "example-game"},
        {"env", "development"},
        {"service_id", "backend-service"},
        {"agent_addr", "127.0.0.1:19090"},
        {"local_listen", "0.0.0.0:0"},
        {"insecure", true},
        {"timeout_seconds", 30},
        {"security", {
            {"cert_file", "/etc/tls/client.crt"},
            {"key_file", "/etc/tls/client.key"},
            {"ca_file", "/etc/tls/ca.crt"},
            {"server_name", "croupier.internal"}
        }},
        {"auth", {
            {"token", "Bearer eyJhbGciOiJIUzI1NiIs..."},
            {"headers", {
                {"X-Game-Version", "2.0.0"},
                {"X-Client-ID", "backend-server-01"}
            }}
        }}
    };
}

nlohmann::json ConfigManager::GenerateExampleComponentJson() {
    return json{
        {"component", {
            {"id", "economy-system"},
            {"version", "1.0.0"},
            {"name", "游戏经济系统"},
            {"description", "包含钱包、商店、拍卖等功能"}
        }},
        {"virtual_objects", {{
            {"id", "economy.wallet"},
            {"version", "1.0.0"},
            {"name", "玩家钱包"},
            {"operations", {
                {"get", "wallet.get"},
                {"transfer", "wallet.transfer"},
                {"add_currency", "wallet.add"}
            }},
            {"relationships", {
                {"owner", {
                    {"type", "many-to-one"},
                    {"entity", "player"},
                    {"foreign_key", "player_id"}
                }}
            }}
        }}},
        {"functions", {{
            {"id", "wallet.get"},
            {"version", "1.0.0"},
            {"handler", {
                {"type", "factory"},
                {"factory", "wallet"},
                {"config", {
                    {"database_url", "postgresql://localhost/game"}
                }}
            }}
        }}}
    };
}

nlohmann::json ConfigManager::GenerateExampleSchemaJson() {
    return json{
        {"schema", {
            {"id", "economy.wallet"},
            {"version", "1.0.0"},
            {"name", "玩家钱包"},
            {"description", "管理玩家的游戏货币和资产"}
        }},
        {"fields", {
            {"wallet_id", {
                {"type", "string"},
                {"required", true},
                {"description", "钱包唯一标识"},
                {"validation", {
                    {"pattern", "^wallet_[a-zA-Z0-9]+$"}
                }}
            }},
            {"player_id", {
                {"type", "string"},
                {"required", true},
                {"description", "关联的玩家ID"}
            }},
            {"balance", {
                {"type", "int"},
                {"required", true},
                {"default_value", "0"},
                {"description", "当前余额"},
                {"validation", {
                    {"min", "0"}
                }}
            }},
            {"currency", {
                {"type", "string"},
                {"required", true},
                {"default_value", "gold"},
                {"description", "货币类型"},
                {"validation", {
                    {"enum", "gold,silver,diamond"}
                }}
            }}
        }},
        {"operations", {
            {"get", "wallet.get"},
            {"transfer", "wallet.transfer"},
            {"add_currency", "wallet.add"},
            {"subtract_currency", "wallet.subtract"}
        }},
        {"relationships", {
            {"owner", {
                {"type", "many-to-one"},
                {"entity", "player"},
                {"foreign_key", "player_id"}
            }},
            {"transactions", {
                {"type", "one-to-many"},
                {"entity", "transaction"},
                {"foreign_key", "wallet_id"}
            }}
        }}
    };
}

// 字段类型验证辅助方法
bool ConfigManager::ValidateFieldType(const nlohmann::json& value, const VirtualObjectSchema::FieldSchema& field_schema) {
    if (field_schema.type == "string" && !value.is_string()) return false;
    if (field_schema.type == "int" && !value.is_number_integer()) return false;
    if (field_schema.type == "float" && !value.is_number()) return false;
    if (field_schema.type == "bool" && !value.is_boolean()) return false;
    if (field_schema.type == "object" && !value.is_object()) return false;
    if (field_schema.type == "array" && !value.is_array()) return false;
    return true;
}

// 字段规则验证辅助方法
bool ConfigManager::ValidateFieldRules(const nlohmann::json& value, const VirtualObjectSchema::FieldSchema& field_schema) {
    for (const auto& [rule_name, rule_value] : field_schema.validation) {
        if (rule_name == "min" && value.is_number()) {
            double min_val = std::stod(rule_value);
            if (value.get<double>() < min_val) return false;
        } else if (rule_name == "max" && value.is_number()) {
            double max_val = std::stod(rule_value);
            if (value.get<double>() > max_val) return false;
        } else if (rule_name == "pattern" && value.is_string()) {
            std::regex pattern(rule_value);
            if (!std::regex_match(value.get<std::string>(), pattern)) return false;
        } else if (rule_name == "enum" && value.is_string()) {
            std::string enum_values = rule_value;
            std::string val_str = value.get<std::string>();
            if (enum_values.find(val_str) == std::string::npos) return false;
        }
    }
    return true;
}

ClientConfig ConfigManager::CreateDefaultClientConfig() {
    ClientConfig config;
    config.game_id = "default-game";
    config.env = "development";
    config.service_id = "cpp-service";
    config.agent_addr = "127.0.0.1:19090";
    config.local_listen = "127.0.0.1:0";
    config.insecure = true;
    config.timeout_seconds = 30;
    return config;
}

#endif // CROUPIER_SDK_ENABLE_JSON

} // namespace sdk
} // namespace croupier