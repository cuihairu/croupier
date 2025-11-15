#pragma once

#include "croupier/sdk/croupier_client.h"
#include <string>
#include <vector>
#include <map>
#include <memory>
#include <mutex>

// Forward declarations for platform-specific types
#ifdef _WIN32
#include <windows.h>
using LibraryHandle = HMODULE;
#else
using LibraryHandle = void*;
#endif

namespace croupier {
namespace sdk {
namespace plugin {

/**
 * @brief Plugin information structure
 */
struct PluginInfo {
    std::string name;
    std::string version;
    std::string author;
    std::string description;
    std::vector<std::string> provided_functions;
    std::map<std::string, std::string> metadata;
};

/**
 * @brief Plugin function signature for initialization
 *
 * All plugins must export these standard functions:
 * - croupier_plugin_init() -> int (0 = success)
 * - croupier_plugin_info() -> PluginInfo*
 * - croupier_plugin_cleanup() -> void
 */
typedef int (*PluginInitFunc)();
typedef PluginInfo* (*PluginInfoFunc)();
typedef void (*PluginCleanupFunc)();

/**
 * @brief Dynamic library and plugin manager
 *
 * Provides safe loading/unloading of dynamic libraries and function resolution
 */
class DynamicLibraryManager {
public:
    DynamicLibraryManager();
    ~DynamicLibraryManager();

    /**
     * @brief Load dynamic library from file
     *
     * @param lib_path Path to the dynamic library (.so/.dll/.dylib)
     * @return std::string Library ID (empty if failed)
     */
    std::string LoadLibrary(const std::string& lib_path);

    /**
     * @brief Unload dynamic library
     *
     * @param library_id Library ID returned from LoadLibrary
     * @return bool Success status
     */
    bool UnloadLibrary(const std::string& library_id);

    /**
     * @brief Get function pointer from loaded library
     *
     * @param library_id Library ID
     * @param function_name Function name to resolve
     * @return void* Function pointer (nullptr if not found)
     */
    void* GetFunction(const std::string& library_id, const std::string& function_name);

    /**
     * @brief Get function handler wrapper
     *
     * @param library_id Library ID
     * @param function_name Function name
     * @return FunctionHandler Wrapped function handler
     */
    FunctionHandler GetFunctionHandler(const std::string& library_id, const std::string& function_name);

    /**
     * @brief Check if library is loaded
     *
     * @param library_id Library ID
     * @return bool Is loaded status
     */
    bool IsLibraryLoaded(const std::string& library_id) const;

    /**
     * @brief Get list of loaded libraries
     *
     * @return std::vector<std::string> List of library IDs
     */
    std::vector<std::string> GetLoadedLibraries() const;

    /**
     * @brief Get library file path
     *
     * @param library_id Library ID
     * @return std::string File path (empty if not found)
     */
    std::string GetLibraryPath(const std::string& library_id) const;

    /**
     * @brief Get last error message
     *
     * @return std::string Error message
     */
    std::string GetLastError() const;

    /**
     * @brief Set error callback
     *
     * @param callback Error callback function
     */
    void SetErrorCallback(std::function<void(const std::string&)> callback);

private:
    struct LibraryEntry {
        LibraryHandle handle;
        std::string path;
        std::string id;
        std::vector<std::string> exported_functions;
    };

    std::map<std::string, std::unique_ptr<LibraryEntry>> libraries_;
    mutable std::mutex libraries_mutex_;
    std::string last_error_;
    std::function<void(const std::string&)> error_callback_;

    // Platform-specific implementations
    LibraryHandle LoadLibraryImpl(const std::string& lib_path);
    void UnloadLibraryImpl(LibraryHandle handle);
    void* GetFunctionImpl(LibraryHandle handle, const std::string& function_name);
    std::string GetPlatformError();

    // Helper methods
    std::string GenerateLibraryId(const std::string& lib_path);
    void SetLastError(const std::string& error);
    void NotifyError(const std::string& error);
};

/**
 * @brief High-level plugin manager
 *
 * Manages plugin lifecycle, registration, and function dispatch
 */
class PluginManager {
public:
    PluginManager();
    ~PluginManager();

    /**
     * @brief Load plugin from file
     *
     * @param plugin_path Path to plugin file
     * @return bool Success status
     */
    bool LoadPlugin(const std::string& plugin_path);

    /**
     * @brief Unload plugin
     *
     * @param plugin_name Plugin name
     * @return bool Success status
     */
    bool UnloadPlugin(const std::string& plugin_name);

    /**
     * @brief Get plugin information
     *
     * @param plugin_name Plugin name
     * @return PluginInfo Plugin information (empty if not found)
     */
    PluginInfo GetPluginInfo(const std::string& plugin_name) const;

    /**
     * @brief Get function handler from plugin
     *
     * @param function_id Fully qualified function ID (plugin.function)
     * @return FunctionHandler Function handler (nullptr if not found)
     */
    FunctionHandler GetPluginFunction(const std::string& function_id);

    /**
     * @brief Register plugin functions with client
     *
     * @param client Croupier client instance
     * @param plugin_name Plugin name
     * @return bool Success status
     */
    bool RegisterPluginFunctions(CroupierClient& client, const std::string& plugin_name);

    /**
     * @brief List loaded plugins
     *
     * @return std::vector<std::string> List of plugin names
     */
    std::vector<std::string> GetLoadedPlugins() const;

    /**
     * @brief List functions provided by plugin
     *
     * @param plugin_name Plugin name
     * @return std::vector<std::string> List of function names
     */
    std::vector<std::string> GetPluginFunctions(const std::string& plugin_name) const;

    /**
     * @brief Scan directory for plugins
     *
     * @param directory Directory to scan
     * @param load_immediately If true, load plugins immediately
     * @return std::vector<std::string> List of found plugin files
     */
    std::vector<std::string> ScanPlugins(const std::string& directory, bool load_immediately = false);

    /**
     * @brief Set plugin search paths
     *
     * @param paths List of directories to search for plugins
     */
    void SetSearchPaths(const std::vector<std::string>& paths);

    /**
     * @brief Get plugin search paths
     *
     * @return std::vector<std::string> List of search paths
     */
    std::vector<std::string> GetSearchPaths() const;

    /**
     * @brief Enable/disable plugin auto-loading
     *
     * @param enabled Auto-loading enabled
     */
    void SetAutoLoading(bool enabled);

    /**
     * @brief Check if auto-loading is enabled
     *
     * @return bool Auto-loading status
     */
    bool IsAutoLoadingEnabled() const;

private:
    struct PluginEntry {
        std::string library_id;
        PluginInfo info;
        std::map<std::string, FunctionHandler> functions;
    };

    DynamicLibraryManager library_manager_;
    std::map<std::string, std::unique_ptr<PluginEntry>> plugins_;
    mutable std::mutex plugins_mutex_;
    std::vector<std::string> search_paths_;
    bool auto_loading_enabled_;

    // Helper methods
    bool InitializePlugin(const std::string& library_id, const std::string& plugin_path);
    void CleanupPlugin(const std::string& plugin_name);
    std::string ExtractPluginName(const std::string& plugin_path);
    bool ValidatePlugin(const PluginInfo& info);
    std::vector<std::string> DiscoverPluginFunctions(const std::string& library_id, const PluginInfo& info);
};

/**
 * @brief Plugin registry for managing plugin metadata
 */
class PluginRegistry {
public:
    /**
     * @brief Register plugin metadata
     *
     * @param info Plugin information
     * @return bool Success status
     */
    static bool RegisterPlugin(const PluginInfo& info);

    /**
     * @brief Get registered plugin information
     *
     * @param plugin_name Plugin name
     * @return PluginInfo Plugin information
     */
    static PluginInfo GetPluginInfo(const std::string& plugin_name);

    /**
     * @brief List all registered plugins
     *
     * @return std::vector<PluginInfo> List of plugin information
     */
    static std::vector<PluginInfo> GetAllPlugins();

    /**
     * @brief Check if plugin is registered
     *
     * @param plugin_name Plugin name
     * @return bool Registration status
     */
    static bool IsPluginRegistered(const std::string& plugin_name);

private:
    static std::map<std::string, PluginInfo> registry_;
    static std::mutex registry_mutex_;
};

} // namespace plugin
} // namespace sdk
} // namespace croupier