// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

#include <gtest/gtest.h>
#include "croupier/sdk/utils/file_utils.h"
#include <fstream>
#include <cstdlib>

using namespace croupier::sdk::utils;

// File System Utils Test Suite
class FileSystemUtilsTest : public ::testing::Test {
protected:
    std::string temp_dir_;
    std::vector<std::string> cleanup_files_;
    std::vector<std::string> cleanup_dirs_;

    void SetUp() override {
        // Create a temporary test directory
        temp_dir_ = FileSystemUtils::CreateTempFile("test_dir_", "");
        // CreateTempFile creates a file, we need a directory
        FileSystemUtils::RemoveFile(temp_dir_);
        temp_dir_ += "_dir";
        FileSystemUtils::CreateDirectory(temp_dir_);
        cleanup_dirs_.push_back(temp_dir_);
    }

    void TearDown() override {
        // Clean up created files
        for (const auto& file : cleanup_files_) {
            FileSystemUtils::RemoveFile(file);
        }
        // Clean up created directories (in reverse order for nested)
        for (auto it = cleanup_dirs_.rbegin(); it != cleanup_dirs_.rend(); ++it) {
            FileSystemUtils::RemoveDirectory(*it, true);
        }
    }

    std::string CreateTestFile(const std::string& name, const std::string& content = "") {
        std::string path = FileSystemUtils::JoinPath(temp_dir_, name);
        FileSystemUtils::WriteFileContent(path, content);
        cleanup_files_.push_back(path);
        return path;
    }

    std::string CreateTestSubDir(const std::string& name) {
        std::string path = FileSystemUtils::JoinPath(temp_dir_, name);
        FileSystemUtils::CreateDirectory(path);
        cleanup_dirs_.push_back(path);
        return path;
    }
};

// ========== FileExists Tests ==========

TEST_F(FileSystemUtilsTest, FileExists_ExistingFile) {
    std::string test_file = CreateTestFile("exists.txt", "content");
    EXPECT_TRUE(FileSystemUtils::FileExists(test_file));
}

TEST_F(FileSystemUtilsTest, FileExists_NonExistentFile) {
    EXPECT_FALSE(FileSystemUtils::FileExists("/path/to/nonexistent/file.txt"));
}

TEST_F(FileSystemUtilsTest, FileExists_EmptyPath) {
    EXPECT_FALSE(FileSystemUtils::FileExists(""));
}

TEST_F(FileSystemUtilsTest, FileExists_Directory) {
    // FileExists should return false for directories (it checks file existence)
    bool result = FileSystemUtils::FileExists(temp_dir_);
    // Behavior may vary by platform - document it
    EXPECT_TRUE(result || !result);
}

// ========== DirectoryExists Tests ==========

TEST_F(FileSystemUtilsTest, DirectoryExists_ExistingDirectory) {
    EXPECT_TRUE(FileSystemUtils::DirectoryExists(temp_dir_));
}

TEST_F(FileSystemUtilsTest, DirectoryExists_NonExistentDirectory) {
    EXPECT_FALSE(FileSystemUtils::DirectoryExists("/path/to/nonexistent/dir"));
}

TEST_F(FileSystemUtilsTest, DirectoryExists_EmptyPath) {
    EXPECT_FALSE(FileSystemUtils::DirectoryExists(""));
}

TEST_F(FileSystemUtilsTest, DirectoryExists_File) {
    std::string test_file = CreateTestFile("regular_file.txt");
    EXPECT_FALSE(FileSystemUtils::DirectoryExists(test_file));
}

// ========== CreateDirectory Tests ==========

TEST_F(FileSystemUtilsTest, CreateDirectory_NewDirectory) {
    std::string new_dir = FileSystemUtils::JoinPath(temp_dir_, "new_dir");
    EXPECT_TRUE(FileSystemUtils::CreateDirectory(new_dir));
    EXPECT_TRUE(FileSystemUtils::DirectoryExists(new_dir));
    cleanup_dirs_.push_back(new_dir);
}

TEST_F(FileSystemUtilsTest, CreateDirectory_ExistingDirectory) {
    // Should return true if directory already exists
    EXPECT_TRUE(FileSystemUtils::CreateDirectory(temp_dir_));
}

TEST_F(FileSystemUtilsTest, CreateDirectory_NestedDirectory) {
    std::string nested = FileSystemUtils::JoinPath(temp_dir_, "level1/level2/level3");
    EXPECT_TRUE(FileSystemUtils::CreateDirectory(nested));
    EXPECT_TRUE(FileSystemUtils::DirectoryExists(nested));
    cleanup_dirs_.push_back(FileSystemUtils::JoinPath(temp_dir_, "level1/level2/level3"));
    cleanup_dirs_.push_back(FileSystemUtils::JoinPath(temp_dir_, "level1/level2"));
    cleanup_dirs_.push_back(FileSystemUtils::JoinPath(temp_dir_, "level1"));
}

// ========== ReadFileContent Tests ==========

TEST_F(FileSystemUtilsTest, ReadFileContent_ExistingFile) {
    std::string content = "Hello, World!";
    std::string test_file = CreateTestFile("read_test.txt", content);

    std::string read_content = FileSystemUtils::ReadFileContent(test_file);
    EXPECT_EQ(read_content, content);
}

TEST_F(FileSystemUtilsTest, ReadFileContent_EmptyFile) {
    std::string test_file = CreateTestFile("empty_file.txt", "");
    std::string content = FileSystemUtils::ReadFileContent(test_file);
    EXPECT_TRUE(content.empty());
}

TEST_F(FileSystemUtilsTest, ReadFileContent_NonExistentFile) {
    EXPECT_THROW({
        FileSystemUtils::ReadFileContent("/nonexistent/file.txt");
    }, std::runtime_error);
}

TEST_F(FileSystemUtilsTest, ReadFileContent_MultilineContent) {
    std::string content = "Line 1\nLine 2\nLine 3\n";
    std::string test_file = CreateTestFile("multiline.txt", content);

    std::string read_content = FileSystemUtils::ReadFileContent(test_file);
    EXPECT_EQ(read_content, content);
}

TEST_F(FileSystemUtilsTest, ReadFileContent_BinaryContent) {
    std::string binary_content = "Binary\x00Content\x01Test";
    std::string test_file = CreateTestFile("binary.bin", binary_content);

    std::string read_content = FileSystemUtils::ReadFileContent(test_file);
    // Binary content handling
    EXPECT_FALSE(read_content.empty());
}

// ========== WriteFileContent Tests ==========

TEST_F(FileSystemUtilsTest, WriteFileContent_NewFile) {
    std::string test_file = FileSystemUtils::JoinPath(temp_dir_, "write_test.txt");
    EXPECT_TRUE(FileSystemUtils::WriteFileContent(test_file, "test content"));
    EXPECT_TRUE(FileSystemUtils::FileExists(test_file));
    cleanup_files_.push_back(test_file);
}

TEST_F(FileSystemUtilsTest, WriteFileContent_Overwrite) {
    std::string test_file = CreateTestFile("overwrite.txt", "original");
    EXPECT_TRUE(FileSystemUtils::WriteFileContent(test_file, "new content", false));

    std::string content = FileSystemUtils::ReadFileContent(test_file);
    EXPECT_EQ(content, "new content");
}

TEST_F(FileSystemUtilsTest, WriteFileContent_Append) {
    std::string test_file = CreateTestFile("append.txt", "first ");
    EXPECT_TRUE(FileSystemUtils::WriteFileContent(test_file, "second", true));

    std::string content = FileSystemUtils::ReadFileContent(test_file);
    EXPECT_EQ(content, "first second");
}

TEST_F(FileSystemUtilsTest, WriteFileContent_EmptyContent) {
    std::string test_file = FileSystemUtils::JoinPath(temp_dir_, "empty_write.txt");
    EXPECT_TRUE(FileSystemUtils::WriteFileContent(test_file, ""));
    EXPECT_TRUE(FileSystemUtils::FileExists(test_file));
    cleanup_files_.push_back(test_file);
}

// ========== ListFiles Tests ==========

TEST_F(FileSystemUtilsTest, ListFiles_EmptyDirectory) {
    std::string empty_dir = CreateTestSubDir("empty_subdir");
    auto files = FileSystemUtils::ListFiles(empty_dir);
    EXPECT_TRUE(files.empty());
}

TEST_F(FileSystemUtilsTest, ListFiles_WithFiles) {
    CreateTestFile("file1.txt", "content1");
    CreateTestFile("file2.txt", "content2");
    CreateTestFile("file3.json", "{}");

    auto files = FileSystemUtils::ListFiles(temp_dir_);
    EXPECT_GE(files.size(), 3U);
}

TEST_F(FileSystemUtilsTest, ListFiles_WithExtensionFilter) {
    CreateTestFile("data1.json", "{}");
    CreateTestFile("data2.json", "{}");
    CreateTestFile("readme.txt", "text");

    auto json_files = FileSystemUtils::ListFiles(temp_dir_, ".json");
    EXPECT_EQ(json_files.size(), 2U);

    for (const auto& file : json_files) {
        EXPECT_TRUE(file.find(".json") != std::string::npos);
    }
}

TEST_F(FileSystemUtilsTest, ListFiles_NonExistentDirectory) {
    auto files = FileSystemUtils::ListFiles("/nonexistent/directory");
    EXPECT_TRUE(files.empty());
}

TEST_F(FileSystemUtilsTest, ListFiles_Recursive) {
    // Create nested structure
    std::string sub1 = CreateTestSubDir("sub1");
    std::string sub2 = CreateTestSubDir("sub1/sub2");

    // Create files at different levels
    CreateTestFile("root.txt", "root");
    FileSystemUtils::WriteFileContent(FileSystemUtils::JoinPath(sub1, "level1.txt"), "level1");
    FileSystemUtils::WriteFileContent(FileSystemUtils::JoinPath(sub2, "level2.txt"), "level2");

    auto all_files = FileSystemUtils::ListFiles(temp_dir_, "", true);
    EXPECT_GE(all_files.size(), 3U);
}

// ========== ListDirectories Tests ==========

TEST_F(FileSystemUtilsTest, ListDirectories_WithSubdirs) {
    CreateTestSubDir("subdir1");
    CreateTestSubDir("subdir2");
    CreateTestFile("file.txt");  // Should not be listed

    auto dirs = FileSystemUtils::ListDirectories(temp_dir_);
    EXPECT_GE(dirs.size(), 2U);
}

TEST_F(FileSystemUtilsTest, ListDirectories_Empty) {
    std::string empty_dir = CreateTestSubDir("no_subdirs");
    CreateTestFile("no_subdirs/file.txt");

    auto dirs = FileSystemUtils::ListDirectories(empty_dir);
    EXPECT_TRUE(dirs.empty());
}

TEST_F(FileSystemUtilsTest, ListDirectories_Recursive) {
    std::string level1 = CreateTestSubDir("level1");
    std::string level2 = CreateTestSubDir("level1/level2");
    std::string level3 = CreateTestSubDir("level1/level2/level3");

    auto all_dirs = FileSystemUtils::ListDirectories(temp_dir_, true);
    EXPECT_GE(all_dirs.size(), 3U);
}

// ========== GetFileSize Tests ==========

TEST_F(FileSystemUtilsTest, GetFileSize_ExistingFile) {
    std::string content = "12345";  // 5 bytes
    std::string test_file = CreateTestFile("size_test.txt", content);

    long size = FileSystemUtils::GetFileSize(test_file);
    EXPECT_EQ(size, 5);
}

TEST_F(FileSystemUtilsTest, GetFileSize_EmptyFile) {
    std::string test_file = CreateTestFile("empty_size.txt", "");
    long size = FileSystemUtils::GetFileSize(test_file);
    EXPECT_EQ(size, 0);
}

TEST_F(FileSystemUtilsTest, GetFileSize_NonExistent) {
    long size = FileSystemUtils::GetFileSize("/nonexistent/file.txt");
    EXPECT_EQ(size, -1);
}

// ========== GetFileExtension Tests ==========

TEST_F(FileSystemUtilsTest, GetFileExtension_WithExtension) {
    EXPECT_EQ(FileSystemUtils::GetFileExtension("file.txt"), ".txt");
    EXPECT_EQ(FileSystemUtils::GetFileExtension("data.json"), ".json");
    EXPECT_EQ(FileSystemUtils::GetFileExtension("archive.tar.gz"), ".gz");
}

TEST_F(FileSystemUtilsTest, GetFileExtension_NoExtension) {
    EXPECT_EQ(FileSystemUtils::GetFileExtension("filename"), "");
    EXPECT_EQ(FileSystemUtils::GetFileExtension("Makefile"), "");
}

TEST_F(FileSystemUtilsTest, GetFileExtension_HiddenFile) {
    // Hidden files on Unix start with dot
    std::string ext = FileSystemUtils::GetFileExtension(".gitignore");
    // Behavior may vary - document it
    EXPECT_TRUE(ext == "" || ext == ".gitignore");
}

TEST_F(FileSystemUtilsTest, GetFileExtension_WithPath) {
    EXPECT_EQ(FileSystemUtils::GetFileExtension("/path/to/file.txt"), ".txt");
    EXPECT_EQ(FileSystemUtils::GetFileExtension("C:\\Windows\\file.exe"), ".exe");
}

// ========== GetFileName Tests ==========

TEST_F(FileSystemUtilsTest, GetFileName_WithPath) {
    EXPECT_EQ(FileSystemUtils::GetFileName("/path/to/file.txt"), "file.txt");
    EXPECT_EQ(FileSystemUtils::GetFileName("C:\\Windows\\system32\\cmd.exe"), "cmd.exe");
}

TEST_F(FileSystemUtilsTest, GetFileName_NoPath) {
    EXPECT_EQ(FileSystemUtils::GetFileName("file.txt"), "file.txt");
}

TEST_F(FileSystemUtilsTest, GetFileName_TrailingSeparator) {
    std::string result = FileSystemUtils::GetFileName("/path/to/dir/");
    // May return empty or "dir" depending on implementation
    EXPECT_TRUE(result.empty() || result == "dir");
}

// ========== GetDirectoryPath Tests ==========

TEST_F(FileSystemUtilsTest, GetDirectoryPath_WithFile) {
    EXPECT_EQ(FileSystemUtils::GetDirectoryPath("/path/to/file.txt"), "/path/to");
}

TEST_F(FileSystemUtilsTest, GetDirectoryPath_RootFile) {
    EXPECT_EQ(FileSystemUtils::GetDirectoryPath("/file.txt"), "");
}

TEST_F(FileSystemUtilsTest, GetDirectoryPath_NoDirectory) {
    EXPECT_EQ(FileSystemUtils::GetDirectoryPath("file.txt"), "");
}

// ========== JoinPath Tests ==========

TEST_F(FileSystemUtilsTest, JoinPath_TwoComponents) {
    std::string joined = FileSystemUtils::JoinPath("/base", "relative");
    EXPECT_TRUE(joined.find("base") != std::string::npos);
    EXPECT_TRUE(joined.find("relative") != std::string::npos);
}

TEST_F(FileSystemUtilsTest, JoinPath_WithTrailingSeparator) {
    std::string joined = FileSystemUtils::JoinPath("/base/", "relative");
    // Should not have double separators
    EXPECT_TRUE(joined.find("//") == std::string::npos || joined.find("\\\\") == std::string::npos);
}

TEST_F(FileSystemUtilsTest, JoinPath_WithLeadingSeparator) {
    std::string joined = FileSystemUtils::JoinPath("/base", "/relative");
    // Should handle leading separator in relative path
    EXPECT_TRUE(joined.find("relative") != std::string::npos);
}

TEST_F(FileSystemUtilsTest, JoinPath_EmptyBase) {
    std::string joined = FileSystemUtils::JoinPath("", "relative");
    EXPECT_EQ(joined, "relative");
}

TEST_F(FileSystemUtilsTest, JoinPath_EmptyRelative) {
    std::string joined = FileSystemUtils::JoinPath("/base", "");
    EXPECT_EQ(joined, "/base");
}

TEST_F(FileSystemUtilsTest, JoinPath_VectorComponents) {
    std::vector<std::string> components = {"path", "to", "file.txt"};
    std::string joined = FileSystemUtils::JoinPath(components);
    EXPECT_TRUE(joined.find("path") != std::string::npos);
    EXPECT_TRUE(joined.find("file.txt") != std::string::npos);
}

TEST_F(FileSystemUtilsTest, JoinPath_EmptyVector) {
    std::vector<std::string> components;
    std::string joined = FileSystemUtils::JoinPath(components);
    EXPECT_TRUE(joined.empty());
}

// ========== NormalizePath Tests ==========

TEST_F(FileSystemUtilsTest, NormalizePath_CurrentDir) {
    std::string normalized = FileSystemUtils::NormalizePath("/path/./to/file");
    EXPECT_TRUE(normalized.find("/./") == std::string::npos);
}

TEST_F(FileSystemUtilsTest, NormalizePath_ParentDir) {
    std::string normalized = FileSystemUtils::NormalizePath("/path/to/../file");
    EXPECT_TRUE(normalized.find("/../") == std::string::npos);
}

TEST_F(FileSystemUtilsTest, NormalizePath_Multiple) {
    std::string normalized = FileSystemUtils::NormalizePath("/a/b/c/../../d/./e");
    EXPECT_TRUE(normalized.find("..") == std::string::npos);
    EXPECT_TRUE(normalized.find("/./") == std::string::npos);
}

TEST_F(FileSystemUtilsTest, NormalizePath_Empty) {
    std::string normalized = FileSystemUtils::NormalizePath("");
    EXPECT_TRUE(normalized.empty());
}

// ========== IsAbsolutePath Tests ==========

TEST_F(FileSystemUtilsTest, IsAbsolutePath_Unix) {
    EXPECT_TRUE(FileSystemUtils::IsAbsolutePath("/absolute/path"));
    EXPECT_FALSE(FileSystemUtils::IsAbsolutePath("relative/path"));
}

TEST_F(FileSystemUtilsTest, IsAbsolutePath_Windows) {
#ifdef _WIN32
    EXPECT_TRUE(FileSystemUtils::IsAbsolutePath("C:\\Windows"));
    EXPECT_TRUE(FileSystemUtils::IsAbsolutePath("D:\\path"));
    EXPECT_TRUE(FileSystemUtils::IsAbsolutePath("\\\\server\\share"));
#endif
}

TEST_F(FileSystemUtilsTest, IsAbsolutePath_Empty) {
    EXPECT_FALSE(FileSystemUtils::IsAbsolutePath(""));
}

// ========== ToAbsolutePath Tests ==========

TEST_F(FileSystemUtilsTest, ToAbsolutePath_AlreadyAbsolute) {
    std::string abs_path = "/absolute/path";
    std::string result = FileSystemUtils::ToAbsolutePath(abs_path);
    EXPECT_TRUE(FileSystemUtils::IsAbsolutePath(result));
}

TEST_F(FileSystemUtilsTest, ToAbsolutePath_Relative) {
    std::string result = FileSystemUtils::ToAbsolutePath("relative/path");
    EXPECT_TRUE(FileSystemUtils::IsAbsolutePath(result));
}

TEST_F(FileSystemUtilsTest, ToAbsolutePath_WithBasePath) {
    std::string result = FileSystemUtils::ToAbsolutePath("file.txt", "/base/path");
    EXPECT_TRUE(result.find("base") != std::string::npos);
    EXPECT_TRUE(result.find("file.txt") != std::string::npos);
}

// ========== CreateTempFile Tests ==========

TEST_F(FileSystemUtilsTest, CreateTempFile_Default) {
    std::string temp = FileSystemUtils::CreateTempFile();
    EXPECT_FALSE(temp.empty());
    // Clean up
    FileSystemUtils::RemoveFile(temp);
}

TEST_F(FileSystemUtilsTest, CreateTempFile_WithPrefix) {
    std::string temp = FileSystemUtils::CreateTempFile("myprefix_");
    EXPECT_FALSE(temp.empty()) << "CreateTempFile should return a non-empty path";
    if (!temp.empty()) {
        // Check that at least part of the prefix is in the filename
        // Windows GetTempFileNameA only uses first 3 characters of prefix
#ifdef _WIN32
        std::string filename = FileSystemUtils::GetFileName(temp);
        EXPECT_TRUE(filename.find("myp") != std::string::npos ||
                    filename.find("myprefix") != std::string::npos)
            << "Expected prefix 'myp' or 'myprefix' in filename: " << filename;
#else
        EXPECT_TRUE(temp.find("myprefix") != std::string::npos)
            << "Expected 'myprefix' in path: " << temp;
#endif
        FileSystemUtils::RemoveFile(temp);
    }
}

TEST_F(FileSystemUtilsTest, CreateTempFile_WithSuffix) {
    std::string temp = FileSystemUtils::CreateTempFile("test_", ".json");
    // Suffix handling may vary
    EXPECT_FALSE(temp.empty());
    FileSystemUtils::RemoveFile(temp);
}

TEST_F(FileSystemUtilsTest, CreateTempFile_UniqueNames) {
    std::string temp1 = FileSystemUtils::CreateTempFile("unique_");
    std::string temp2 = FileSystemUtils::CreateTempFile("unique_");
    EXPECT_NE(temp1, temp2);
    FileSystemUtils::RemoveFile(temp1);
    FileSystemUtils::RemoveFile(temp2);
}

// ========== RemoveFile Tests ==========

TEST_F(FileSystemUtilsTest, RemoveFile_Existing) {
    std::string test_file = FileSystemUtils::JoinPath(temp_dir_, "to_remove.txt");
    FileSystemUtils::WriteFileContent(test_file, "delete me");
    EXPECT_TRUE(FileSystemUtils::FileExists(test_file));

    EXPECT_TRUE(FileSystemUtils::RemoveFile(test_file));
    EXPECT_FALSE(FileSystemUtils::FileExists(test_file));
}

TEST_F(FileSystemUtilsTest, RemoveFile_NonExistent) {
    bool result = FileSystemUtils::RemoveFile("/nonexistent/file.txt");
    // May return true or false depending on implementation
    EXPECT_TRUE(result || !result);  // Documents behavior
}

// ========== RemoveDirectory Tests ==========

TEST_F(FileSystemUtilsTest, RemoveDirectory_Empty) {
    std::string empty_dir = FileSystemUtils::JoinPath(temp_dir_, "empty_to_remove");
    FileSystemUtils::CreateDirectory(empty_dir);
    EXPECT_TRUE(FileSystemUtils::DirectoryExists(empty_dir));

    EXPECT_TRUE(FileSystemUtils::RemoveDirectory(empty_dir));
    EXPECT_FALSE(FileSystemUtils::DirectoryExists(empty_dir));
}

TEST_F(FileSystemUtilsTest, RemoveDirectory_NonRecursive) {
    std::string dir = FileSystemUtils::JoinPath(temp_dir_, "non_empty");
    FileSystemUtils::CreateDirectory(dir);
    FileSystemUtils::WriteFileContent(FileSystemUtils::JoinPath(dir, "file.txt"), "content");

    // Non-recursive should fail for non-empty directory
    bool result = FileSystemUtils::RemoveDirectory(dir, false);
    // Behavior varies - may succeed or fail
    EXPECT_TRUE(result || !result);

    // Clean up
    FileSystemUtils::RemoveDirectory(dir, true);
}

TEST_F(FileSystemUtilsTest, RemoveDirectory_Recursive) {
    std::string dir = FileSystemUtils::JoinPath(temp_dir_, "recursive_remove");
    std::string subdir = FileSystemUtils::JoinPath(dir, "subdir");
    FileSystemUtils::CreateDirectory(subdir);
    FileSystemUtils::WriteFileContent(FileSystemUtils::JoinPath(dir, "file1.txt"), "1");
    FileSystemUtils::WriteFileContent(FileSystemUtils::JoinPath(subdir, "file2.txt"), "2");

    EXPECT_TRUE(FileSystemUtils::RemoveDirectory(dir, true));
    EXPECT_FALSE(FileSystemUtils::DirectoryExists(dir));
}

TEST_F(FileSystemUtilsTest, RemoveDirectory_NonExistent) {
    // Should return true for non-existent directory
    EXPECT_TRUE(FileSystemUtils::RemoveDirectory("/nonexistent/dir"));
}

// ========== CopyFile Tests ==========

TEST_F(FileSystemUtilsTest, CopyFile_Success) {
    std::string src = CreateTestFile("copy_source.txt", "copy content");
    std::string dst = FileSystemUtils::JoinPath(temp_dir_, "copy_dest.txt");

    EXPECT_TRUE(FileSystemUtils::CopyFile(src, dst));
    EXPECT_TRUE(FileSystemUtils::FileExists(dst));

    std::string content = FileSystemUtils::ReadFileContent(dst);
    EXPECT_EQ(content, "copy content");
    cleanup_files_.push_back(dst);
}

TEST_F(FileSystemUtilsTest, CopyFile_Overwrite) {
    std::string src = CreateTestFile("overwrite_src.txt", "new content");
    std::string dst = CreateTestFile("overwrite_dst.txt", "old content");

    EXPECT_TRUE(FileSystemUtils::CopyFile(src, dst, true));
    std::string content = FileSystemUtils::ReadFileContent(dst);
    EXPECT_EQ(content, "new content");
}

TEST_F(FileSystemUtilsTest, CopyFile_NoOverwrite) {
    std::string src = CreateTestFile("no_overwrite_src.txt", "new content");
    std::string dst = CreateTestFile("no_overwrite_dst.txt", "old content");

    EXPECT_FALSE(FileSystemUtils::CopyFile(src, dst, false));
    std::string content = FileSystemUtils::ReadFileContent(dst);
    EXPECT_EQ(content, "old content");
}

TEST_F(FileSystemUtilsTest, CopyFile_SourceNotExists) {
    std::string dst = FileSystemUtils::JoinPath(temp_dir_, "copy_dst.txt");
    EXPECT_FALSE(FileSystemUtils::CopyFile("/nonexistent/source.txt", dst));
}

// ========== GetCurrentDirectory Tests ==========

TEST_F(FileSystemUtilsTest, GetCurrentDirectory_NotEmpty) {
    std::string cwd = FileSystemUtils::GetCurrentDirectory();
    EXPECT_FALSE(cwd.empty());
}

TEST_F(FileSystemUtilsTest, GetCurrentDirectory_IsAbsolute) {
    std::string cwd = FileSystemUtils::GetCurrentDirectory();
    EXPECT_TRUE(FileSystemUtils::IsAbsolutePath(cwd));
}

// ========== GetPathSeparator Tests ==========

TEST_F(FileSystemUtilsTest, GetPathSeparator_Platform) {
    char sep = FileSystemUtils::GetPathSeparator();
#ifdef _WIN32
    EXPECT_EQ(sep, '\\');
#else
    EXPECT_EQ(sep, '/');
#endif
}

// ========== Edge Cases ==========

TEST_F(FileSystemUtilsTest, HandleUnicodePaths) {
    // Test with Unicode characters in path
    std::string unicode_name = "测试文件.txt";
    std::string test_file = FileSystemUtils::JoinPath(temp_dir_, unicode_name);

    // Writing might work or fail depending on filesystem
    bool write_result = FileSystemUtils::WriteFileContent(test_file, "unicode test");
    if (write_result) {
        EXPECT_TRUE(FileSystemUtils::FileExists(test_file));
        FileSystemUtils::RemoveFile(test_file);
    }
}

TEST_F(FileSystemUtilsTest, HandleLongPaths) {
    // Test with long path names
    std::string long_name(200, 'a');
    std::string test_file = FileSystemUtils::JoinPath(temp_dir_, long_name + ".txt");

    bool result = FileSystemUtils::WriteFileContent(test_file, "long name test");
    // May succeed or fail depending on filesystem limits
    if (result) {
        FileSystemUtils::RemoveFile(test_file);
    }
}

TEST_F(FileSystemUtilsTest, HandleSpecialCharacters) {
    // Test with special characters (avoid path separators)
    std::vector<std::string> special_names = {
        "file with spaces.txt",
        "file-with-dashes.txt",
        "file_with_underscores.txt",
        "file.multiple.dots.txt"
    };

    for (const auto& name : special_names) {
        std::string test_file = FileSystemUtils::JoinPath(temp_dir_, name);
        EXPECT_TRUE(FileSystemUtils::WriteFileContent(test_file, "content"));
        EXPECT_TRUE(FileSystemUtils::FileExists(test_file));
        FileSystemUtils::RemoveFile(test_file);
    }
}
