#pragma once

#include "croupier_client.h"
#include <functional>
#include <map>
#include <memory>
#include <string>
#include <vector>

namespace croupier {
namespace sdk {

/**
 * @brief 配置文件驱动的组件加载器
 *
 * 支持从 JSON 配置文件动态加载组件和注册函数处理器
 */
class ConfigDrivenLoader {
public:
    /**
     * @brief 函数处理器工厂类型
     *
     * 用于根据函数ID创建对应的处理器
     */
    using HandlerFactory = std::function<FunctionHandler(const std::string& function_id, const std::map<std::string, std::string>& config)>;

    /**
     * @brief 动态库加载器类型
     *
     * 用于从动态库中加载函数处理器
     */
    using DynamicLibLoader = std::function<FunctionHandler(const std::string& lib_path, const std::string& function_name)>;

    ConfigDrivenLoader();
    ~ConfigDrivenLoader();

    // ========== 处理器注册机制 ==========

    /**
     * @brief 注册函数处理器工厂
     *
     * @param prefix 函数ID前缀 (如 "wallet", "player")
     * @param factory 工厂函数
     */
    void RegisterHandlerFactory(const std::string& prefix, HandlerFactory factory);

    /**
     * @brief 注册具体的函数处理器
     *
     * @param function_id 函数ID
     * @param handler 处理器函数
     */
    void RegisterHandler(const std::string& function_id, FunctionHandler handler);

    /**
     * @brief 设置动态库加载器
     *
     * @param loader 动态库加载函数
     */
    void SetDynamicLibLoader(DynamicLibLoader loader);

    // ========== 配置文件加载 ==========

    /**
     * @brief 从 JSON 文件加载组件配置
     *
     * @param config_file JSON 配置文件路径
     * @return ComponentDescriptor 解析后的组件描述符
     */
    ComponentDescriptor LoadComponentFromFile(const std::string& config_file);

    /**
     * @brief 从 JSON 字符串加载组件配置
     *
     * @param json_content JSON 内容字符串
     * @return ComponentDescriptor 解析后的组件描述符
     */
    ComponentDescriptor LoadComponentFromJson(const std::string& json_content);

    /**
     * @brief 加载组件并自动注册到客户端
     *
     * @param client 客户端实例
     * @param config_file 配置文件路径
     * @return bool 是否成功
     */
    bool LoadAndRegisterComponent(CroupierClient& client, const std::string& config_file);

    // ========== 处理器查找 ==========

    /**
     * @brief 根据函数ID获取处理器
     *
     * @param function_id 函数ID
     * @param config 可选配置参数
     * @return FunctionHandler 处理器函数，如果未找到则返回nullptr
     */
    FunctionHandler GetHandler(const std::string& function_id, const std::map<std::string, std::string>& config = {});

    /**
     * @brief 检查处理器是否存在
     *
     * @param function_id 函数ID
     * @return bool 是否存在
     */
    bool HasHandler(const std::string& function_id) const;

    /**
     * @brief 获取所有已注册的处理器ID列表
     *
     * @return std::vector<std::string> 处理器ID列表
     */
    std::vector<std::string> GetRegisteredHandlers() const;

    // ========== 配置验证 ==========

    /**
     * @brief 验证配置文件格式
     *
     * @param config_file 配置文件路径
     * @return bool 是否有效
     */
    bool ValidateConfigFile(const std::string& config_file);

    /**
     * @brief 验证 JSON 配置内容
     *
     * @param json_content JSON 内容
     * @return bool 是否有效
     */
    bool ValidateJsonConfig(const std::string& json_content);

private:
    // 工厂映射表
    std::map<std::string, HandlerFactory> handler_factories_;

    // 直接注册的处理器
    std::map<std::string, FunctionHandler> registered_handlers_;

    // 动态库加载器
    DynamicLibLoader dynamic_lib_loader_;

    // Internal helper methods
    std::string LoadFileContent(const std::string& file_path);
    ComponentDescriptor ParseJsonToComponent(const std::string& json_content);
    VirtualObjectDescriptor ParseJsonToVirtualObject(const std::string& json_content);
    std::map<std::string, FunctionHandler> ResolveHandlers(const ComponentDescriptor& comp);
    FunctionHandler CreateHandlerFromConfig(const std::string& function_id, const std::map<std::string, std::string>& config);
    FunctionHandler CreateDefaultHandler(const std::string& function_id);
    FunctionHandler LoadFromDynamicLib(const std::string& lib_path, const std::string& function_name);
};

// ========== 预定义的处理器工厂 ==========

/**
 * @brief 基础处理器工厂
 *
 * 提供一些常用的处理器模式
 */
class BasicHandlerFactory {
public:
    /**
     * @brief 创建回显处理器
     *
     * @param config 配置参数
     * @return FunctionHandler 回显处理器
     */
    static FunctionHandler CreateEchoHandler(const std::map<std::string, std::string>& config);

    /**
     * @brief 创建错误处理器
     *
     * @param error_message 错误消息
     * @return FunctionHandler 错误处理器
     */
    static FunctionHandler CreateErrorHandler(const std::string& error_message);

    /**
     * @brief 创建代理处理器
     *
     * @param target_url 目标URL
     * @param config 配置参数
     * @return FunctionHandler 代理处理器
     */
    static FunctionHandler CreateProxyHandler(const std::string& target_url, const std::map<std::string, std::string>& config);

    /**
     * @brief 创建模板处理器
     *
     * @param template_string 模板字符串
     * @param config 配置参数
     * @return FunctionHandler 模板处理器
     */
    static FunctionHandler CreateTemplateHandler(const std::string& template_string, const std::map<std::string, std::string>& config);
};

// ========== JSON 配置格式定义 ==========

/**
 * 标准的 JSON 配置文件格式：
 *
 * {
 *   "component": {
 *     "id": "economy-system",
 *     "version": "1.0.0",
 *     "name": "游戏经济系统",
 *     "description": "包含钱包、商店、拍卖等功能"
 *   },
 *   "virtual_objects": [
 *     {
 *       "id": "economy.wallet",
 *       "version": "1.0.0",
 *       "name": "钱包系统",
 *       "operations": {
 *         "get": "wallet.get",
 *         "transfer": "wallet.transfer"
 *       },
 *       "relationships": {
 *         "owner": {
 *           "type": "many-to-one",
 *           "entity": "player",
 *           "foreign_key": "player_id"
 *         }
 *       }
 *     }
 *   ],
 *   "functions": [
 *     {
 *       "id": "wallet.get",
 *       "version": "1.0.0",
 *       "handler": {
 *         "type": "factory",
 *         "factory": "wallet",
 *         "config": {
 *           "database_url": "postgresql://localhost/game",
 *           "cache_ttl": "300"
 *         }
 *       }
 *     },
 *     {
 *       "id": "wallet.transfer",
 *       "version": "1.0.0",
 *       "handler": {
 *         "type": "dynamic_lib",
 *         "library": "./libs/wallet.so",
 *         "function": "wallet_transfer_handler"
 *       }
 *     }
 *   ],
 *   "config": {
 *     "database": {
 *       "url": "postgresql://localhost/game",
 *       "pool_size": 10
 *     },
 *     "cache": {
 *       "provider": "redis",
 *       "url": "redis://localhost:6379"
 *     }
 *   }
 * }
 */

} // namespace sdk
} // namespace croupier