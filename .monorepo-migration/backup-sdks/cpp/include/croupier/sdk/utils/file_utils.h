#pragma once

#include <string>
#include <vector>
#include <fstream>

namespace croupier {
namespace sdk {
namespace utils {

/**
 * @brief Cross-platform file system utilities
 *
 * Provides unified file and directory operations across different platforms
 */
class FileSystemUtils {
public:
    /**
     * @brief Check if file exists
     *
     * @param file_path Path to check
     * @return true if file exists and is readable
     */
    static bool FileExists(const std::string& file_path);

    /**
     * @brief Check if directory exists
     *
     * @param dir_path Directory path to check
     * @return true if directory exists
     */
    static bool DirectoryExists(const std::string& dir_path);

    /**
     * @brief Create directory recursively
     *
     * @param dir_path Directory path to create
     * @return true if created successfully or already exists
     */
    static bool CreateDirectory(const std::string& dir_path);

    /**
     * @brief Read entire file content as string
     *
     * @param file_path File to read
     * @return File content as string
     * @throws std::runtime_error if file cannot be read
     */
    static std::string ReadFileContent(const std::string& file_path);

    /**
     * @brief Write content to file
     *
     * @param file_path Target file path
     * @param content Content to write
     * @param append If true, append to existing file; if false, overwrite
     * @return true if write successful
     */
    static bool WriteFileContent(const std::string& file_path,
                               const std::string& content,
                               bool append = false);

    /**
     * @brief List files in directory with optional extension filter
     *
     * @param directory Directory to scan
     * @param extension Optional extension filter (e.g., ".json")
     * @param recursive If true, scan subdirectories recursively
     * @return Vector of file paths
     */
    static std::vector<std::string> ListFiles(const std::string& directory,
                                            const std::string& extension = "",
                                            bool recursive = false);

    /**
     * @brief List directories in given path
     *
     * @param directory Directory to scan
     * @param recursive If true, scan subdirectories recursively
     * @return Vector of directory paths
     */
    static std::vector<std::string> ListDirectories(const std::string& directory,
                                                   bool recursive = false);

    /**
     * @brief Get file size in bytes
     *
     * @param file_path File to measure
     * @return File size in bytes, -1 if error
     */
    static long GetFileSize(const std::string& file_path);

    /**
     * @brief Get file extension
     *
     * @param file_path File path
     * @return Extension including dot (e.g., ".json"), empty if no extension
     */
    static std::string GetFileExtension(const std::string& file_path);

    /**
     * @brief Get filename without path
     *
     * @param file_path Full file path
     * @return Filename only
     */
    static std::string GetFileName(const std::string& file_path);

    /**
     * @brief Get directory path without filename
     *
     * @param file_path Full file path
     * @return Directory path
     */
    static std::string GetDirectoryPath(const std::string& file_path);

    /**
     * @brief Join path components with proper separator
     *
     * @param components Path components to join
     * @return Joined path with platform-appropriate separators
     */
    static std::string JoinPath(const std::vector<std::string>& components);

    /**
     * @brief Join two path components
     *
     * @param base Base path
     * @param relative Relative path to append
     * @return Joined path
     */
    static std::string JoinPath(const std::string& base, const std::string& relative);

    /**
     * @brief Normalize path (resolve .., ., etc.)
     *
     * @param path Path to normalize
     * @return Normalized path
     */
    static std::string NormalizePath(const std::string& path);

    /**
     * @brief Check if path is absolute
     *
     * @param path Path to check
     * @return true if absolute path
     */
    static bool IsAbsolutePath(const std::string& path);

    /**
     * @brief Convert relative path to absolute
     *
     * @param relative_path Relative path
     * @param base_path Base path (if empty, uses current directory)
     * @return Absolute path
     */
    static std::string ToAbsolutePath(const std::string& relative_path,
                                    const std::string& base_path = "");

    /**
     * @brief Create temporary file
     *
     * @param prefix Filename prefix
     * @param suffix Filename suffix (e.g., ".tmp")
     * @return Path to created temporary file
     */
    static std::string CreateTempFile(const std::string& prefix = "croupier",
                                    const std::string& suffix = ".tmp");

    /**
     * @brief Remove file
     *
     * @param file_path File to remove
     * @return true if removed successfully
     */
    static bool RemoveFile(const std::string& file_path);

    /**
     * @brief Remove directory (optionally recursive)
     *
     * @param dir_path Directory to remove
     * @param recursive If true, remove contents recursively
     * @return true if removed successfully
     */
    static bool RemoveDirectory(const std::string& dir_path, bool recursive = false);

    /**
     * @brief Copy file
     *
     * @param source_path Source file
     * @param dest_path Destination file
     * @param overwrite If true, overwrite existing destination
     * @return true if copied successfully
     */
    static bool CopyFile(const std::string& source_path,
                        const std::string& dest_path,
                        bool overwrite = false);

    /**
     * @brief Get current working directory
     *
     * @return Current directory path
     */
    static std::string GetCurrentDirectory();

    /**
     * @brief Get platform-specific path separator
     *
     * @return Path separator character ('/' on Unix, '\\' on Windows)
     */
    static char GetPathSeparator();

private:
    // Platform-specific implementations
    static void ListFilesRecursive(const std::string& directory,
                                  const std::string& extension,
                                  std::vector<std::string>& result);

    static void ListDirectoriesRecursive(const std::string& directory,
                                       std::vector<std::string>& result);
};

} // namespace utils
} // namespace sdk
} // namespace croupier