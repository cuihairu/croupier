#pragma once

#include "croupier_client.h"
#include "config_driven_loader.h"
#include <string>
#include <map>
#include <vector>

#ifdef CROUPIER_SDK_ENABLE_JSON
#include <nlohmann/json.hpp>
#endif

namespace croupier {
namespace sdk {

/**
 * @brief 完整的配置管理器
 *
 * 支持从 JSON 文件加载客户端配置、虚拟对象定义和 Schema
 */
class ConfigManager {
public:
    ConfigManager();
    ~ConfigManager();

    // ========== 客户端配置加载 ==========

    /**
     * @brief 从 JSON 文件加载客户端配置
     *
     * @param config_file 配置文件路径
     * @return ClientConfig 客户端配置
     */
    ClientConfig LoadClientConfig(const std::string& config_file);

    /**
     * @brief 从 JSON 字符串加载客户端配置
     *
     * @param json_content JSON 配置内容
     * @return ClientConfig 客户端配置
     */
    ClientConfig LoadClientConfigFromJson(const std::string& json_content);

    // ========== Schema 管理 ==========

    /**
     * @brief 虚拟对象 Schema 定义
     */
    struct VirtualObjectSchema {
        std::string id;
        std::string version;
        std::string name;
        std::string description;

        // 字段定义
        struct FieldSchema {
            std::string name;
            std::string type;        // "string", "int", "float", "bool", "object", "array"
            bool required;
            std::string default_value;
            std::string description;
            std::map<std::string, std::string> validation;  // 验证规则
        };

        std::map<std::string, FieldSchema> fields;
        std::map<std::string, std::string> operations;     // 操作到函数ID的映射
        std::map<std::string, RelationshipDef> relationships;
    };

    /**
     * @brief 从 JSON 加载虚拟对象 Schema
     *
     * @param schema_file Schema 文件路径
     * @return VirtualObjectSchema Schema 定义
     */
    VirtualObjectSchema LoadVirtualObjectSchema(const std::string& schema_file);

    /**
     * @brief 验证数据是否符合 Schema
     *
     * @param schema Schema 定义
     * @param data 要验证的数据 (JSON字符串)
     * @return bool 是否有效
     */
    bool ValidateDataAgainstSchema(const VirtualObjectSchema& schema, const std::string& data);

    // ========== 完整配置加载 ==========

    /**
     * @brief 完整的应用配置
     */
    struct ApplicationConfig {
        ClientConfig client_config;
        std::vector<ComponentDescriptor> components;
        std::map<std::string, VirtualObjectSchema> schemas;
        std::map<std::string, std::string> global_settings;
    };

    /**
     * @brief 从配置目录加载完整应用配置
     *
     * @param config_dir 配置目录路径
     * @return ApplicationConfig 完整配置
     */
    ApplicationConfig LoadApplicationConfig(const std::string& config_dir);

    /**
     * @brief 从单个 JSON 文件加载完整配置
     *
     * @param config_file 主配置文件
     * @return ApplicationConfig 完整配置
     */
    ApplicationConfig LoadApplicationConfigFromFile(const std::string& config_file);

    // ========== 配置验证 ==========

    /**
     * @brief 验证客户端配置
     *
     * @param config 客户端配置
     * @return std::vector<std::string> 错误消息列表（空表示无错误）
     */
    std::vector<std::string> ValidateClientConfig(const ClientConfig& config);

    /**
     * @brief 验证应用配置
     *
     * @param app_config 应用配置
     * @return std::vector<std::string> 错误消息列表（空表示无错误）
     */
    std::vector<std::string> ValidateApplicationConfig(const ApplicationConfig& app_config);

    // ========== 配置生成 ==========

    /**
     * @brief 生成示例配置文件
     *
     * @param output_dir 输出目录
     * @return bool 是否成功
     */
    bool GenerateExampleConfigs(const std::string& output_dir);

private:
    // 内部辅助方法
    std::string LoadFileContent(const std::string& file_path);
    std::vector<std::string> ListFiles(const std::string& directory, const std::string& extension);
    bool CreateDirectory(const std::string& path);

#ifdef CROUPIER_SDK_ENABLE_JSON
    ClientConfig ParseClientConfigFromJson(const nlohmann::json& json);
    VirtualObjectSchema ParseSchemaFromJson(const nlohmann::json& json);
    ComponentDescriptor ParseComponentFromJson(const nlohmann::json& json);
    nlohmann::json GenerateExampleClientConfigJson();
    nlohmann::json GenerateExampleComponentJson();
    nlohmann::json GenerateExampleSchemaJson();
#endif
};

/**
 * @brief 配置文件格式定义和示例
 */
namespace config_examples {

/**
 * 完整的客户端配置文件格式：
 *
 * {
 *   "client": {
 *     "game_id": "my-awesome-game",
 *     "env": "production",
 *     "service_id": "backend-service-01",
 *     "agent_addr": "croupier-agent.internal:19090",
 *     "local_listen": "0.0.0.0:0",
 *     "insecure": false,
 *     "timeout_seconds": 60,
 *     "security": {
 *       "cert_file": "/etc/tls/client.crt",
 *       "key_file": "/etc/tls/client.key",
 *       "ca_file": "/etc/tls/ca.crt",
 *       "server_name": "croupier.internal"
 *     },
 *     "auth": {
 *       "token": "Bearer eyJhbGciOiJIUzI1NiIs...",
 *       "headers": {
 *         "X-Game-Version": "2.1.0",
 *         "X-Client-ID": "backend-server-01"
 *       }
 *     }
 *   },
 *   "global": {
 *     "log_level": "info",
 *     "metrics_enabled": true,
 *     "health_check_port": 8080
 *   }
 * }
 */

/**
 * 虚拟对象 Schema 文件格式：
 *
 * {
 *   "schema": {
 *     "id": "economy.wallet",
 *     "version": "1.0.0",
 *     "name": "玩家钱包",
 *     "description": "管理玩家的游戏货币和资产"
 *   },
 *   "fields": {
 *     "wallet_id": {
 *       "type": "string",
 *       "required": true,
 *       "description": "钱包唯一标识",
 *       "validation": {
 *         "pattern": "^wallet_[a-zA-Z0-9]+$"
 *       }
 *     },
 *     "player_id": {
 *       "type": "string",
 *       "required": true,
 *       "description": "关联的玩家ID"
 *     },
 *     "balance": {
 *       "type": "int",
 *       "required": true,
 *       "default_value": "0",
 *       "description": "当前余额",
 *       "validation": {
 *         "min": "0"
 *       }
 *     },
 *     "currency": {
 *       "type": "string",
 *       "required": true,
 *       "default_value": "gold",
 *       "description": "货币类型",
 *       "validation": {
 *         "enum": "gold,silver,diamond"
 *       }
 *     },
 *     "last_updated": {
 *       "type": "string",
 *       "required": false,
 *       "description": "最后更新时间"
 *     }
 *   },
 *   "operations": {
 *     "get": "wallet.get",
 *     "transfer": "wallet.transfer",
 *     "add_currency": "wallet.add",
 *     "subtract_currency": "wallet.subtract"
 *   },
 *   "relationships": {
 *     "owner": {
 *       "type": "many-to-one",
 *       "entity": "player",
 *       "foreign_key": "player_id"
 *     },
 *     "transactions": {
 *       "type": "one-to-many",
 *       "entity": "transaction",
 *       "foreign_key": "wallet_id"
 *     }
 *   }
 * }
 */

/**
 * 完整的应用配置文件格式：
 *
 * {
 *   "client": { ... },          // 客户端配置
 *   "components": [             // 组件定义
 *     {
 *       "id": "economy-system",
 *       "version": "1.0.0",
 *       "config_file": "./components/economy.json",
 *       "schema_file": "./schemas/economy_schema.json"
 *     }
 *   ],
 *   "schemas": [                // Schema 文件列表
 *     {
 *       "id": "economy.wallet",
 *       "file": "./schemas/wallet_schema.json"
 *     }
 *   ],
 *   "includes": [               // 包含的其他配置文件
 *     "./configs/database.json",
 *     "./configs/cache.json"
 *   ],
 *   "global": { ... }           // 全局设置
 * }
 */

} // namespace config_examples

} // namespace sdk
} // namespace croupier