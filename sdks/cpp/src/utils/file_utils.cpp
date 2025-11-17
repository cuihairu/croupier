#include "croupier/sdk/utils/file_utils.h"
#include <iostream>
#include <sstream>
#include <algorithm>

// Platform-specific includes
#ifdef _WIN32
#include <direct.h>
#include <windows.h>
#include <io.h>
#define ACCESS _access
#define GETCWD _getcwd
#define MKDIR(path) _mkdir(path)
// Avoid collisions with WinAPI macros (CreateDirectory, GetCurrentDirectory, etc.)
#undef CreateDirectory
#undef GetCurrentDirectory
#undef RemoveDirectory
#undef CopyFile
#else
#include <sys/stat.h>
#include <sys/types.h>
#include <dirent.h>
#include <unistd.h>
#include <errno.h>
#include <limits.h>
#include <cstring>
#define ACCESS access
#define GETCWD getcwd
#define MKDIR(path) mkdir(path, 0755)
#endif

namespace croupier {
namespace sdk {
namespace utils {

// ========== File Existence Checks ==========

bool FileSystemUtils::FileExists(const std::string& file_path) {
    return ACCESS(file_path.c_str(), 0) == 0;
}

bool FileSystemUtils::DirectoryExists(const std::string& dir_path) {
#ifdef _WIN32
    DWORD attributes = GetFileAttributesA(dir_path.c_str());
    return (attributes != INVALID_FILE_ATTRIBUTES && (attributes & FILE_ATTRIBUTE_DIRECTORY));
#else
    struct stat st;
    return (stat(dir_path.c_str(), &st) == 0 && S_ISDIR(st.st_mode));
#endif
}

// ========== Directory Operations ==========

bool FileSystemUtils::CreateDirectory(const std::string& dir_path) {
    if (DirectoryExists(dir_path)) {
        return true;
    }

    // Create parent directories recursively
    std::string parent = GetDirectoryPath(dir_path);
    if (!parent.empty() && parent != dir_path && !DirectoryExists(parent)) {
        if (!CreateDirectory(parent)) {
            return false;
        }
    }

    // Create the directory
#ifdef _WIN32
    return CreateDirectoryA(dir_path.c_str(), nullptr) || GetLastError() == ERROR_ALREADY_EXISTS;
#else
    return MKDIR(dir_path.c_str()) == 0 || errno == EEXIST;
#endif
}

std::string FileSystemUtils::GetCurrentDirectory() {
    char buffer[4096];
    if (GETCWD(buffer, sizeof(buffer)) != nullptr) {
        return std::string(buffer);
    }
    return "";
}

// ========== File I/O Operations ==========

std::string FileSystemUtils::ReadFileContent(const std::string& file_path) {
    std::ifstream file(file_path, std::ios::binary);
    if (!file.is_open()) {
        throw std::runtime_error("Cannot open file for reading: " + file_path);
    }

    std::stringstream buffer;
    buffer << file.rdbuf();
    return buffer.str();
}

bool FileSystemUtils::WriteFileContent(const std::string& file_path,
                                      const std::string& content,
                                      bool append) {
    std::ios_base::openmode mode = std::ios::binary;
    if (append) {
        mode |= std::ios::app;
    } else {
        mode |= std::ios::trunc;
    }

    std::ofstream file(file_path, mode);
    if (!file.is_open()) {
        std::cerr << "Failed to open file for writing: " << file_path << std::endl;
        return false;
    }

    file << content;
    return file.good();
}

// ========== Directory Scanning ==========

std::vector<std::string> FileSystemUtils::ListFiles(const std::string& directory,
                                                   const std::string& extension,
                                                   bool recursive) {
    std::vector<std::string> result;

    if (!DirectoryExists(directory)) {
        std::cerr << "Directory does not exist: " << directory << std::endl;
        return result;
    }

    if (recursive) {
        ListFilesRecursive(directory, extension, result);
    } else {
#ifdef _WIN32
        WIN32_FIND_DATAA findFileData;
        std::string search_pattern = JoinPath(directory, "*");
        HANDLE hFind = FindFirstFileA(search_pattern.c_str(), &findFileData);

        if (hFind != INVALID_HANDLE_VALUE) {
            do {
                if (!(findFileData.dwFileAttributes & FILE_ATTRIBUTE_DIRECTORY)) {
                    std::string filename = findFileData.cFileName;
                    if (extension.empty() || (filename.length() >= extension.length() &&
                        filename.substr(filename.length() - extension.length()) == extension)) {
                        result.push_back(JoinPath(directory, filename));
                    }
                }
            } while (FindNextFileA(hFind, &findFileData));
            FindClose(hFind);
        }
#else
        DIR* dir = opendir(directory.c_str());
        if (dir != nullptr) {
            struct dirent* entry;
            while ((entry = readdir(dir)) != nullptr) {
                if (entry->d_type == DT_REG) {  // Regular file
                    std::string filename = entry->d_name;
                    if (extension.empty() || (filename.length() >= extension.length() &&
                        filename.substr(filename.length() - extension.length()) == extension)) {
                        result.push_back(JoinPath(directory, filename));
                    }
                }
            }
            closedir(dir);
        }
#endif
    }

    return result;
}

std::vector<std::string> FileSystemUtils::ListDirectories(const std::string& directory,
                                                         bool recursive) {
    std::vector<std::string> result;

    if (!DirectoryExists(directory)) {
        std::cerr << "Directory does not exist: " << directory << std::endl;
        return result;
    }

    if (recursive) {
        ListDirectoriesRecursive(directory, result);
    } else {
#ifdef _WIN32
        WIN32_FIND_DATAA findFileData;
        std::string search_pattern = JoinPath(directory, "*");
        HANDLE hFind = FindFirstFileA(search_pattern.c_str(), &findFileData);

        if (hFind != INVALID_HANDLE_VALUE) {
            do {
                if ((findFileData.dwFileAttributes & FILE_ATTRIBUTE_DIRECTORY) &&
                    strcmp(findFileData.cFileName, ".") != 0 &&
                    strcmp(findFileData.cFileName, "..") != 0) {
                    result.push_back(JoinPath(directory, findFileData.cFileName));
                }
            } while (FindNextFileA(hFind, &findFileData));
            FindClose(hFind);
        }
#else
        DIR* dir = opendir(directory.c_str());
        if (dir != nullptr) {
            struct dirent* entry;
            while ((entry = readdir(dir)) != nullptr) {
                if (entry->d_type == DT_DIR &&
                    strcmp(entry->d_name, ".") != 0 &&
                    strcmp(entry->d_name, "..") != 0) {
                    result.push_back(JoinPath(directory, entry->d_name));
                }
            }
            closedir(dir);
        }
#endif
    }

    return result;
}

// ========== File Information ==========

long FileSystemUtils::GetFileSize(const std::string& file_path) {
    std::ifstream file(file_path, std::ios::binary | std::ios::ate);
    if (!file.is_open()) {
        return -1;
    }
    return static_cast<long>(file.tellg());
}

std::string FileSystemUtils::GetFileExtension(const std::string& file_path) {
    size_t dot_pos = file_path.find_last_of('.');
    size_t sep_pos = file_path.find_last_of("\\/");

    if (dot_pos != std::string::npos && (sep_pos == std::string::npos || dot_pos > sep_pos)) {
        return file_path.substr(dot_pos);
    }
    return "";
}

std::string FileSystemUtils::GetFileName(const std::string& file_path) {
    size_t sep_pos = file_path.find_last_of("\\/");
    if (sep_pos != std::string::npos) {
        return file_path.substr(sep_pos + 1);
    }
    return file_path;
}

std::string FileSystemUtils::GetDirectoryPath(const std::string& file_path) {
    size_t sep_pos = file_path.find_last_of("\\/");
    if (sep_pos != std::string::npos) {
        return file_path.substr(0, sep_pos);
    }
    return "";
}

// ========== Path Operations ==========

std::string FileSystemUtils::JoinPath(const std::vector<std::string>& components) {
    if (components.empty()) {
        return "";
    }

    std::string result = components[0];
    for (size_t i = 1; i < components.size(); ++i) {
        result = JoinPath(result, components[i]);
    }
    return result;
}

std::string FileSystemUtils::JoinPath(const std::string& base, const std::string& relative) {
    if (base.empty()) return relative;
    if (relative.empty()) return base;

    char separator = GetPathSeparator();
    std::string result = base;

    // Ensure base doesn't end with separator
    if (result.back() == '/' || result.back() == '\\') {
        result.pop_back();
    }

    // Ensure relative doesn't start with separator
    std::string rel = relative;
    if (rel.front() == '/' || rel.front() == '\\') {
        rel = rel.substr(1);
    }

    return result + separator + rel;
}

std::string FileSystemUtils::NormalizePath(const std::string& path) {
    if (path.empty()) return path;

    std::vector<std::string> components;
    std::stringstream ss(path);
    std::string component;
    char separator = GetPathSeparator();

    // Split path into components
    while (std::getline(ss, component, separator)) {
        if (component.empty() || component == ".") {
            continue;
        } else if (component == "..") {
            if (!components.empty() && components.back() != "..") {
                components.pop_back();
            } else if (!IsAbsolutePath(path)) {
                components.push_back("..");
            }
        } else {
            components.push_back(component);
        }
    }

    // Reconstruct path
    std::string result;
    if (IsAbsolutePath(path)) {
#ifdef _WIN32
        // Handle Windows drive letters
        if (!components.empty() && components[0].length() == 2 && components[0][1] == ':') {
            result = components[0];
            components.erase(components.begin());
        }
        result += separator;
#else
        result += separator;
#endif
    }

    for (size_t i = 0; i < components.size(); ++i) {
        if (i > 0) result += separator;
        result += components[i];
    }

    return result.empty() ? "." : result;
}

bool FileSystemUtils::IsAbsolutePath(const std::string& path) {
    if (path.empty()) return false;

#ifdef _WIN32
    // Windows: starts with drive letter (C:) or UNC path (\\)
    return (path.length() >= 2 && path[1] == ':') ||
           (path.length() >= 2 && path[0] == '\\' && path[1] == '\\');
#else
    // Unix: starts with /
    return path[0] == '/';
#endif
}

std::string FileSystemUtils::ToAbsolutePath(const std::string& relative_path,
                                          const std::string& base_path) {
    if (IsAbsolutePath(relative_path)) {
        return NormalizePath(relative_path);
    }

    std::string base = base_path.empty() ? FileSystemUtils::GetCurrentDirectory() : base_path;
    return NormalizePath(JoinPath(base, relative_path));
}

char FileSystemUtils::GetPathSeparator() {
#ifdef _WIN32
    return '\\';
#else
    return '/';
#endif
}

// ========== File Operations ==========

std::string FileSystemUtils::CreateTempFile(const std::string& prefix,
                                           const std::string& suffix) {
#ifdef _WIN32
    char temp_path[MAX_PATH];
    char temp_filename[MAX_PATH];

    if (GetTempPathA(MAX_PATH, temp_path) == 0) {
        return "";
    }

    if (GetTempFileNameA(temp_path, prefix.c_str(), 0, temp_filename) == 0) {
        return "";
    }

    std::string result = temp_filename;
    if (!suffix.empty() && GetFileExtension(result) != suffix) {
        result += suffix;
    }

    return result;
#else
    std::string temp_template = "/tmp/" + prefix + "XXXXXX" + suffix;
    std::vector<char> temp_path(temp_template.begin(), temp_template.end());
    temp_path.push_back('\0');

    int fd = mkstemp(temp_path.data());
    if (fd != -1) {
        close(fd);
        return std::string(temp_path.data());
    }

    return "";
#endif
}

bool FileSystemUtils::RemoveFile(const std::string& file_path) {
#ifdef _WIN32
    return DeleteFileA(file_path.c_str()) != 0;
#else
    return unlink(file_path.c_str()) == 0;
#endif
}

bool FileSystemUtils::RemoveDirectory(const std::string& dir_path, bool recursive) {
    if (!DirectoryExists(dir_path)) {
        return true;  // Already doesn't exist
    }

    if (recursive) {
        // Remove all contents first
        auto files = ListFiles(dir_path);
        for (const auto& file : files) {
            if (!RemoveFile(file)) {
                std::cerr << "Failed to remove file: " << file << std::endl;
                return false;
            }
        }

        auto subdirs = ListDirectories(dir_path);
        for (const auto& subdir : subdirs) {
            if (!RemoveDirectory(subdir, true)) {
                std::cerr << "Failed to remove subdirectory: " << subdir << std::endl;
                return false;
            }
        }
    }

    // Remove the directory itself
#ifdef _WIN32
    return RemoveDirectoryA(dir_path.c_str()) != 0;
#else
    return rmdir(dir_path.c_str()) == 0;
#endif
}

bool FileSystemUtils::CopyFile(const std::string& source_path,
                              const std::string& dest_path,
                              bool overwrite) {
    if (!FileExists(source_path)) {
        std::cerr << "Source file does not exist: " << source_path << std::endl;
        return false;
    }

    if (!overwrite && FileExists(dest_path)) {
        std::cerr << "Destination file exists and overwrite is disabled: " << dest_path << std::endl;
        return false;
    }

    try {
        std::string content = ReadFileContent(source_path);
        return WriteFileContent(dest_path, content, false);
    } catch (const std::exception& e) {
        std::cerr << "Failed to copy file: " << e.what() << std::endl;
        return false;
    }
}

// ========== Private Helper Methods ==========

void FileSystemUtils::ListFilesRecursive(const std::string& directory,
                                        const std::string& extension,
                                        std::vector<std::string>& result) {
    auto files = ListFiles(directory, extension, false);
    result.insert(result.end(), files.begin(), files.end());

    auto subdirs = ListDirectories(directory, false);
    for (const auto& subdir : subdirs) {
        ListFilesRecursive(subdir, extension, result);
    }
}

void FileSystemUtils::ListDirectoriesRecursive(const std::string& directory,
                                              std::vector<std::string>& result) {
    auto subdirs = ListDirectories(directory, false);
    result.insert(result.end(), subdirs.begin(), subdirs.end());

    for (const auto& subdir : subdirs) {
        ListDirectoriesRecursive(subdir, result);
    }
}

} // namespace utils
} // namespace sdk
} // namespace croupier
