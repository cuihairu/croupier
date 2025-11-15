#include "croupier/sdk/plugin/dynamic_loader.h"
#include "croupier/sdk/utils/file_utils.h"
#include <iostream>
#include <sstream>
#include <algorithm>
#include <chrono>

// Platform-specific includes
#ifdef _WIN32
#include <windows.h>
#else
#include <dlfcn.h>
#include <errno.h>
#endif

namespace croupier {
namespace sdk {
namespace plugin {

// ========== Plugin Registry Implementation ==========

std::map<std::string, PluginInfo> PluginRegistry::registry_;
std::mutex PluginRegistry::registry_mutex_;

bool PluginRegistry::RegisterPlugin(const PluginInfo& info) {
    std::lock_guard<std::mutex> lock(registry_mutex_);
    registry_[info.name] = info;
    return true;
}

PluginInfo PluginRegistry::GetPluginInfo(const std::string& plugin_name) {
    std::lock_guard<std::mutex> lock(registry_mutex_);
    auto it = registry_.find(plugin_name);
    return it != registry_.end() ? it->second : PluginInfo{};
}

std::vector<PluginInfo> PluginRegistry::GetAllPlugins() {
    std::lock_guard<std::mutex> lock(registry_mutex_);
    std::vector<PluginInfo> plugins;
    for (const auto& [name, info] : registry_) {
        plugins.push_back(info);
    }
    return plugins;
}

bool PluginRegistry::IsPluginRegistered(const std::string& plugin_name) {
    std::lock_guard<std::mutex> lock(registry_mutex_);
    return registry_.find(plugin_name) != registry_.end();
}

// ========== DynamicLibraryManager Implementation ==========

DynamicLibraryManager::DynamicLibraryManager() {
    std::cout << "Dynamic library manager initialized" << std::endl;
}

DynamicLibraryManager::~DynamicLibraryManager() {
    // Unload all libraries
    std::lock_guard<std::mutex> lock(libraries_mutex_);
    for (auto& [id, entry] : libraries_) {
        if (entry && entry->handle) {
            UnloadLibraryImpl(entry->handle);
        }
    }
    libraries_.clear();
    std::cout << "Dynamic library manager cleanup completed" << std::endl;
}

std::string DynamicLibraryManager::LoadLibrary(const std::string& lib_path) {
    std::cout << "Loading dynamic library: " << lib_path << std::endl;

    // Check if file exists
    if (!utils::FileSystemUtils::FileExists(lib_path)) {
        SetLastError("Library file does not exist: " + lib_path);
        return "";
    }

    // Generate library ID
    std::string library_id = GenerateLibraryId(lib_path);

    std::lock_guard<std::mutex> lock(libraries_mutex_);

    // Check if already loaded
    auto it = libraries_.find(library_id);
    if (it != libraries_.end()) {
        std::cout << "Library already loaded with ID: " << library_id << std::endl;
        return library_id;
    }

    // Load the library
    LibraryHandle handle = LoadLibraryImpl(lib_path);
    if (!handle) {
        std::string error = "Failed to load library: " + lib_path + " - " + GetPlatformError();
        SetLastError(error);
        return "";
    }

    // Create library entry
    auto entry = std::make_unique<LibraryEntry>();
    entry->handle = handle;
    entry->path = lib_path;
    entry->id = library_id;

    libraries_[library_id] = std::move(entry);

    std::cout << "✅ Library loaded successfully with ID: " << library_id << std::endl;
    return library_id;
}

bool DynamicLibraryManager::UnloadLibrary(const std::string& library_id) {
    std::cout << "Unloading library: " << library_id << std::endl;

    std::lock_guard<std::mutex> lock(libraries_mutex_);

    auto it = libraries_.find(library_id);
    if (it == libraries_.end()) {
        SetLastError("Library not found: " + library_id);
        return false;
    }

    auto& entry = it->second;
    if (entry && entry->handle) {
        UnloadLibraryImpl(entry->handle);
    }

    libraries_.erase(it);
    std::cout << "✅ Library unloaded successfully: " << library_id << std::endl;
    return true;
}

void* DynamicLibraryManager::GetFunction(const std::string& library_id, const std::string& function_name) {
    std::lock_guard<std::mutex> lock(libraries_mutex_);

    auto it = libraries_.find(library_id);
    if (it == libraries_.end()) {
        SetLastError("Library not found: " + library_id);
        return nullptr;
    }

    auto& entry = it->second;
    if (!entry || !entry->handle) {
        SetLastError("Invalid library handle for: " + library_id);
        return nullptr;
    }

    void* func_ptr = GetFunctionImpl(entry->handle, function_name);
    if (!func_ptr) {
        SetLastError("Function not found: " + function_name + " in library: " + library_id);
        return nullptr;
    }

    std::cout << "✅ Function resolved: " << function_name << " in " << library_id << std::endl;
    return func_ptr;
}

FunctionHandler DynamicLibraryManager::GetFunctionHandler(const std::string& library_id, const std::string& function_name) {
    // Standard function signature for Croupier plugins
    typedef const char* (*PluginFunction)(const char* context, const char* payload);

    void* func_ptr = GetFunction(library_id, function_name);
    if (!func_ptr) {
        return nullptr;
    }

    PluginFunction plugin_func = reinterpret_cast<PluginFunction>(func_ptr);

    // Create wrapper that converts to std::string
    return [plugin_func, function_name](const std::string& context, const std::string& payload) -> std::string {
        try {
            const char* result = plugin_func(context.c_str(), payload.c_str());
            return result ? std::string(result) : "{}";
        } catch (const std::exception& e) {
            std::cerr << "❌ Plugin function error in " << function_name << ": " << e.what() << std::endl;
            return R"({"error": ")" + std::string(e.what()) + R"(", "function": ")" + function_name + R"("})";
        }
    };
}

bool DynamicLibraryManager::IsLibraryLoaded(const std::string& library_id) const {
    std::lock_guard<std::mutex> lock(libraries_mutex_);
    return libraries_.find(library_id) != libraries_.end();
}

std::vector<std::string> DynamicLibraryManager::GetLoadedLibraries() const {
    std::lock_guard<std::mutex> lock(libraries_mutex_);
    std::vector<std::string> library_ids;
    for (const auto& [id, entry] : libraries_) {
        library_ids.push_back(id);
    }
    return library_ids;
}

std::string DynamicLibraryManager::GetLibraryPath(const std::string& library_id) const {
    std::lock_guard<std::mutex> lock(libraries_mutex_);
    auto it = libraries_.find(library_id);
    return it != libraries_.end() ? it->second->path : "";
}

std::string DynamicLibraryManager::GetLastError() const {
    return last_error_;
}

void DynamicLibraryManager::SetErrorCallback(std::function<void(const std::string&)> callback) {
    error_callback_ = callback;
}

// ========== Platform-specific Implementations ==========

#ifdef _WIN32
LibraryHandle DynamicLibraryManager::LoadLibraryImpl(const std::string& lib_path) {
    return LoadLibraryA(lib_path.c_str());
}

void DynamicLibraryManager::UnloadLibraryImpl(LibraryHandle handle) {
    FreeLibrary(handle);
}

void* DynamicLibraryManager::GetFunctionImpl(LibraryHandle handle, const std::string& function_name) {
    return GetProcAddress(handle, function_name.c_str());
}

std::string DynamicLibraryManager::GetPlatformError() {
    DWORD error = GetLastError();
    if (error == 0) return "Unknown error";

    LPSTR messageBuffer = nullptr;
    size_t size = FormatMessageA(
        FORMAT_MESSAGE_ALLOCATE_BUFFER | FORMAT_MESSAGE_FROM_SYSTEM | FORMAT_MESSAGE_IGNORE_INSERTS,
        NULL, error, MAKELANGID(LANG_NEUTRAL, SUBLANG_DEFAULT),
        (LPSTR)&messageBuffer, 0, NULL);

    std::string message(messageBuffer, size);
    LocalFree(messageBuffer);
    return message;
}

#else
LibraryHandle DynamicLibraryManager::LoadLibraryImpl(const std::string& lib_path) {
    return dlopen(lib_path.c_str(), RTLD_LAZY | RTLD_LOCAL);
}

void DynamicLibraryManager::UnloadLibraryImpl(LibraryHandle handle) {
    dlclose(handle);
}

void* DynamicLibraryManager::GetFunctionImpl(LibraryHandle handle, const std::string& function_name) {
    return dlsym(handle, function_name.c_str());
}

std::string DynamicLibraryManager::GetPlatformError() {
    const char* error = dlerror();
    return error ? std::string(error) : "Unknown error";
}
#endif

// ========== Helper Methods ==========

std::string DynamicLibraryManager::GenerateLibraryId(const std::string& lib_path) {
    std::string filename = utils::FileSystemUtils::GetFileName(lib_path);

    // Remove file extension
    size_t dot_pos = filename.find_last_of('.');
    if (dot_pos != std::string::npos) {
        filename = filename.substr(0, dot_pos);
    }

    // Add timestamp to make it unique
    auto now = std::chrono::system_clock::now();
    auto timestamp = std::chrono::duration_cast<std::chrono::seconds>(now.time_since_epoch()).count();

    return filename + "_" + std::to_string(timestamp);
}

void DynamicLibraryManager::SetLastError(const std::string& error) {
    last_error_ = error;
    NotifyError(error);
}

void DynamicLibraryManager::NotifyError(const std::string& error) {
    std::cerr << "❌ Dynamic library error: " << error << std::endl;
    if (error_callback_) {
        error_callback_(error);
    }
}

// ========== PluginManager Implementation ==========

PluginManager::PluginManager()
    : auto_loading_enabled_(false) {
    std::cout << "Plugin manager initialized" << std::endl;

    // Set default search paths
    search_paths_ = {
        "./plugins",
        "./libs",
        "/usr/local/lib/croupier/plugins",
        "/opt/croupier/plugins"
    };
}

PluginManager::~PluginManager() {
    // Cleanup all plugins
    std::lock_guard<std::mutex> lock(plugins_mutex_);
    for (auto& [name, entry] : plugins_) {
        CleanupPlugin(name);
    }
    plugins_.clear();
    std::cout << "Plugin manager cleanup completed" << std::endl;
}

bool PluginManager::LoadPlugin(const std::string& plugin_path) {
    std::cout << "Loading plugin: " << plugin_path << std::endl;

    // Load the dynamic library
    std::string library_id = library_manager_.LoadLibrary(plugin_path);
    if (library_id.empty()) {
        std::cerr << "❌ Failed to load plugin library: " << library_manager_.GetLastError() << std::endl;
        return false;
    }

    // Initialize the plugin
    if (!InitializePlugin(library_id, plugin_path)) {
        library_manager_.UnloadLibrary(library_id);
        return false;
    }

    std::cout << "✅ Plugin loaded successfully: " << ExtractPluginName(plugin_path) << std::endl;
    return true;
}

bool PluginManager::UnloadPlugin(const std::string& plugin_name) {
    std::cout << "Unloading plugin: " << plugin_name << std::endl;

    std::lock_guard<std::mutex> lock(plugins_mutex_);

    auto it = plugins_.find(plugin_name);
    if (it == plugins_.end()) {
        std::cerr << "❌ Plugin not found: " << plugin_name << std::endl;
        return false;
    }

    auto& entry = it->second;

    // Cleanup plugin
    CleanupPlugin(plugin_name);

    // Unload library
    bool success = library_manager_.UnloadLibrary(entry->library_id);

    // Remove from plugins map
    plugins_.erase(it);

    if (success) {
        std::cout << "✅ Plugin unloaded successfully: " << plugin_name << std::endl;
    } else {
        std::cerr << "⚠️ Plugin removed but library unload failed: " << plugin_name << std::endl;
    }

    return success;
}

PluginInfo PluginManager::GetPluginInfo(const std::string& plugin_name) const {
    std::lock_guard<std::mutex> lock(plugins_mutex_);
    auto it = plugins_.find(plugin_name);
    return it != plugins_.end() ? it->second->info : PluginInfo{};
}

FunctionHandler PluginManager::GetPluginFunction(const std::string& function_id) {
    // Parse function_id as "plugin_name.function_name"
    size_t dot_pos = function_id.find('.');
    if (dot_pos == std::string::npos) {
        std::cerr << "❌ Invalid function ID format: " << function_id << " (expected: plugin.function)" << std::endl;
        return nullptr;
    }

    std::string plugin_name = function_id.substr(0, dot_pos);
    std::string function_name = function_id.substr(dot_pos + 1);

    std::lock_guard<std::mutex> lock(plugins_mutex_);

    auto it = plugins_.find(plugin_name);
    if (it == plugins_.end()) {
        std::cerr << "❌ Plugin not found: " << plugin_name << std::endl;
        return nullptr;
    }

    auto& functions = it->second->functions;
    auto func_it = functions.find(function_name);
    if (func_it == functions.end()) {
        std::cerr << "❌ Function not found: " << function_name << " in plugin: " << plugin_name << std::endl;
        return nullptr;
    }

    return func_it->second;
}

bool PluginManager::RegisterPluginFunctions(CroupierClient& client, const std::string& plugin_name) {
    std::lock_guard<std::mutex> lock(plugins_mutex_);

    auto it = plugins_.find(plugin_name);
    if (it == plugins_.end()) {
        std::cerr << "❌ Plugin not found for registration: " << plugin_name << std::endl;
        return false;
    }

    auto& entry = it->second;
    bool all_success = true;

    for (const auto& [func_name, handler] : entry->functions) {
        FunctionDescriptor desc;
        desc.id = plugin_name + "." + func_name;
        desc.version = entry->info.version;

        bool success = client.RegisterFunction(desc, handler);
        if (success) {
            std::cout << "✅ Registered plugin function: " << desc.id << std::endl;
        } else {
            std::cerr << "❌ Failed to register plugin function: " << desc.id << std::endl;
            all_success = false;
        }
    }

    return all_success;
}

std::vector<std::string> PluginManager::GetLoadedPlugins() const {
    std::lock_guard<std::mutex> lock(plugins_mutex_);
    std::vector<std::string> plugin_names;
    for (const auto& [name, entry] : plugins_) {
        plugin_names.push_back(name);
    }
    return plugin_names;
}

std::vector<std::string> PluginManager::GetPluginFunctions(const std::string& plugin_name) const {
    std::lock_guard<std::mutex> lock(plugins_mutex_);

    auto it = plugins_.find(plugin_name);
    if (it == plugins_.end()) {
        return {};
    }

    std::vector<std::string> function_names;
    for (const auto& [func_name, handler] : it->second->functions) {
        function_names.push_back(func_name);
    }
    return function_names;
}

std::vector<std::string> PluginManager::ScanPlugins(const std::string& directory, bool load_immediately) {
    std::cout << "Scanning for plugins in: " << directory << std::endl;

    std::vector<std::string> plugin_files;
    std::vector<std::string> extensions;

#ifdef _WIN32
    extensions.push_back(".dll");
#elif defined(__APPLE__)
    extensions.push_back(".dylib");
#else
    extensions.push_back(".so");
#endif

    for (const std::string& ext : extensions) {
        auto files = utils::FileSystemUtils::ListFiles(directory, ext, false);
        plugin_files.insert(plugin_files.end(), files.begin(), files.end());
    }

    std::cout << "Found " << plugin_files.size() << " potential plugin files" << std::endl;

    if (load_immediately) {
        for (const std::string& file : plugin_files) {
            LoadPlugin(file);
        }
    }

    return plugin_files;
}

void PluginManager::SetSearchPaths(const std::vector<std::string>& paths) {
    search_paths_ = paths;
    std::cout << "Updated plugin search paths to " << paths.size() << " directories" << std::endl;
}

std::vector<std::string> PluginManager::GetSearchPaths() const {
    return search_paths_;
}

void PluginManager::SetAutoLoading(bool enabled) {
    auto_loading_enabled_ = enabled;
    std::cout << "Plugin auto-loading " << (enabled ? "enabled" : "disabled") << std::endl;

    if (enabled) {
        // Scan all search paths for plugins
        for (const std::string& path : search_paths_) {
            if (utils::FileSystemUtils::DirectoryExists(path)) {
                ScanPlugins(path, true);
            }
        }
    }
}

bool PluginManager::IsAutoLoadingEnabled() const {
    return auto_loading_enabled_;
}

// ========== Private Helper Methods ==========

bool PluginManager::InitializePlugin(const std::string& library_id, const std::string& plugin_path) {
    // Get plugin info function
    PluginInfoFunc info_func = reinterpret_cast<PluginInfoFunc>(
        library_manager_.GetFunction(library_id, "croupier_plugin_info"));

    if (!info_func) {
        std::cerr << "❌ Plugin info function not found in: " << plugin_path << std::endl;
        return false;
    }

    // Get plugin info
    PluginInfo* info_ptr = info_func();
    if (!info_ptr) {
        std::cerr << "❌ Plugin info function returned null: " << plugin_path << std::endl;
        return false;
    }

    PluginInfo info = *info_ptr;
    if (!ValidatePlugin(info)) {
        return false;
    }

    // Initialize plugin
    PluginInitFunc init_func = reinterpret_cast<PluginInitFunc>(
        library_manager_.GetFunction(library_id, "croupier_plugin_init"));

    if (init_func) {
        int result = init_func();
        if (result != 0) {
            std::cerr << "❌ Plugin initialization failed with code: " << result << std::endl;
            return false;
        }
    }

    // Create plugin entry
    std::lock_guard<std::mutex> lock(plugins_mutex_);

    auto entry = std::make_unique<PluginEntry>();
    entry->library_id = library_id;
    entry->info = info;

    // Discover and load functions
    std::vector<std::string> function_names = DiscoverPluginFunctions(library_id, info);
    for (const std::string& func_name : function_names) {
        FunctionHandler handler = library_manager_.GetFunctionHandler(library_id, func_name);
        if (handler) {
            entry->functions[func_name] = handler;
        }
    }

    plugins_[info.name] = std::move(entry);

    // Register with global registry
    PluginRegistry::RegisterPlugin(info);

    return true;
}

void PluginManager::CleanupPlugin(const std::string& plugin_name) {
    auto it = plugins_.find(plugin_name);
    if (it == plugins_.end()) {
        return;
    }

    auto& entry = it->second;

    // Call plugin cleanup function if available
    PluginCleanupFunc cleanup_func = reinterpret_cast<PluginCleanupFunc>(
        library_manager_.GetFunction(entry->library_id, "croupier_plugin_cleanup"));

    if (cleanup_func) {
        try {
            cleanup_func();
            std::cout << "✅ Plugin cleanup completed: " << plugin_name << std::endl;
        } catch (const std::exception& e) {
            std::cerr << "⚠️ Plugin cleanup error: " << e.what() << std::endl;
        }
    }
}

std::string PluginManager::ExtractPluginName(const std::string& plugin_path) {
    std::string filename = utils::FileSystemUtils::GetFileName(plugin_path);

    // Remove file extension
    size_t dot_pos = filename.find_last_of('.');
    if (dot_pos != std::string::npos) {
        filename = filename.substr(0, dot_pos);
    }

    // Remove common plugin prefixes
    if (filename.substr(0, 3) == "lib") {
        filename = filename.substr(3);
    }
    if (filename.substr(0, 8) == "croupier") {
        filename = filename.substr(8);
    }
    if (!filename.empty() && filename[0] == '_') {
        filename = filename.substr(1);
    }

    return filename.empty() ? "unknown" : filename;
}

bool PluginManager::ValidatePlugin(const PluginInfo& info) {
    if (info.name.empty()) {
        std::cerr << "❌ Plugin name is empty" << std::endl;
        return false;
    }

    if (info.version.empty()) {
        std::cerr << "❌ Plugin version is empty for: " << info.name << std::endl;
        return false;
    }

    if (info.provided_functions.empty()) {
        std::cerr << "⚠️ Plugin provides no functions: " << info.name << std::endl;
        // This is not necessarily an error, but worth noting
    }

    return true;
}

std::vector<std::string> PluginManager::DiscoverPluginFunctions(const std::string& library_id, const PluginInfo& info) {
    std::vector<std::string> discovered_functions;

    // First, use the functions listed in plugin info
    for (const std::string& func_name : info.provided_functions) {
        void* func_ptr = library_manager_.GetFunction(library_id, func_name);
        if (func_ptr) {
            discovered_functions.push_back(func_name);
        } else {
            std::cerr << "⚠️ Function listed but not found: " << func_name << " in " << info.name << std::endl;
        }
    }

    // TODO: Could also try to discover functions by scanning symbols
    // This would be platform-specific and complex

    return discovered_functions;
}

} // namespace plugin
} // namespace sdk
} // namespace croupier