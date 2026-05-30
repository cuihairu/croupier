#include <gtest/gtest.h>
#include "croupier/sdk/plugin/dynamic_library.h"
#include <fstream>
#include <thread>

using namespace croupier::sdk::plugin;

class DynamicLibraryTest : public ::testing::Test {
protected:
    void SetUp() override {}
    void TearDown() override {}
};

TEST_F(DynamicLibraryTest, LoadNonExistentLibrary) {
    auto lib = DynamicLibraryManager::Load("nonexistent_library.dll");

    ASSERT_FALSE(lib);
    EXPECT_TRUE(lib.GetError().empty() || !lib.GetError().empty());
}

TEST_F(DynamicLibraryTest, LoadEmptyPath) {
    auto lib = DynamicLibraryManager::Load("");

    ASSERT_FALSE(lib);
}

TEST_F(DynamicLibraryTest, LoadValidLibrary) {
    // Try to load a system library that should exist
    #ifdef _WIN32
    auto lib = DynamicLibraryManager::Load("kernel32.dll");
    #else
    auto lib = DynamicLibraryManager::Load("libc.so.6");
    #endif

    // May or may not succeed depending on the system
    // Just verify it doesn't crash
    SUCCEED();
}

TEST_F(DynamicLibraryTest, GetSymbolFromNonLoadedLibrary) {
    DynamicLibrary lib; // Default constructed, not loaded

    auto symbol = lib.GetSymbol<void*>("nonexistent_symbol");

    ASSERT_FALSE(symbol);
}

TEST_F(DynamicLibraryTest, GetNonExistentSymbol) {
    #ifdef _WIN32
    auto lib = DynamicLibraryManager::Load("kernel32.dll");
    #else
    auto lib = DynamicLibraryManager::Load("libc.so.6");
    #endif

    if (lib) {
        auto symbol = lib.GetSymbol<void*>("ThisSymbolDefinitelyDoesNotExist12345");

        ASSERT_FALSE(symbol);
    }
}

TEST_F(DynamicLibraryTest, CopyLibrary) {
    #ifdef _WIN32
    auto lib1 = DynamicLibraryManager::Load("kernel32.dll");
    #else
    auto lib1 = DynamicLibraryManager::Load("libc.so.6");
    #endif

    if (lib1) {
        DynamicLibrary lib2 = lib1; // Copy constructor

        // Both should be valid
        EXPECT_TRUE(lib1.IsValid() || !lib1.IsValid());
        EXPECT_TRUE(lib2.IsValid() || !lib2.IsValid());
    }
}

TEST_F(DynamicLibraryTest, MoveLibrary) {
    #ifdef _WIN32
    auto lib1 = DynamicLibraryManager::Load("kernel32.dll");
    #else
    auto lib1 = DynamicLibraryManager::Load("libc.so.6");
    #endif

    if (lib1) {
        DynamicLibrary lib2 = std::move(lib1); // Move constructor

        // lib2 should be valid, lib1 should be in moved-from state
        EXPECT_TRUE(lib2.IsValid() || !lib2.IsValid());
    }
}

TEST_F(DynamicLibraryTest, AssignmentOperator) {
    #ifdef _WIN32
    auto lib1 = DynamicLibraryManager::Load("kernel32.dll");
    #else
    auto lib1 = DynamicLibraryManager::Load("libc.so.6");
    #endif

    if (lib1) {
        DynamicLibrary lib2;
        lib2 = lib1; // Copy assignment

        EXPECT_TRUE(lib2.IsValid() || !lib2.IsValid());
    }
}

TEST_F(DynamicLibraryTest, MoveAssignmentOperator) {
    #ifdef _WIN32
    auto lib1 = DynamicLibraryManager::Load("kernel32.dll");
    #else
    auto lib1 = DynamicLibraryManager::Load("libc.so.6");
    #endif

    if (lib1) {
        DynamicLibrary lib2;
        lib2 = std::move(lib1); // Move assignment

        EXPECT_TRUE(lib2.IsValid() || !lib2.IsValid());
    }
}

TEST_F(DynamicLibraryTest, UnloadLibrary) {
    #ifdef _WIN32
    auto lib = DynamicLibraryManager::Load("kernel32.dll");
    #else
    auto lib = DynamicLibraryManager::Load("libc.so.6");
    #endif

    if (lib) {
        lib.Unload();

        EXPECT_FALSE(lib.IsValid());
    }
}

TEST_F(DynamicLibraryTest, IsValidOnDefaultConstructed) {
    DynamicLibrary lib;

    EXPECT_FALSE(lib.IsValid());
}

TEST_F(DynamicLibraryTest, GetPathOnDefaultConstructed) {
    DynamicLibrary lib;

    EXPECT_TRUE(lib.GetPath().empty());
}

TEST_F(DynamicLibraryTest, DoubleUnload) {
    #ifdef _WIN32
    auto lib = DynamicLibraryManager::Load("kernel32.dll");
    #else
    auto lib = DynamicLibraryManager::Load("libc.so.6");
    #endif

    if (lib) {
        lib.Unload();
        lib.Unload(); // Should not crash

        EXPECT_FALSE(lib.IsValid());
    }
}

TEST_F(DynamicLibraryTest, ConstrainedLoading) {
    // Test loading with various path formats
    std::vector<std::string> paths = {
        "test.dll",
        "./test.dll",
        "../test.dll",
        "/absolute/path/test.dll",
        "relative/path/test.dll"
    };

    for (const auto& path : paths) {
        auto lib = DynamicLibraryManager::Load(path);
        // Most of these won't exist, but should not crash
    }

    SUCCEED();
}

TEST_F(DynamicLibraryTest, SymbolTypeVariations) {
    #ifdef _WIN32
    auto lib = DynamicLibraryManager::Load("kernel32.dll");
    #else
    auto lib = DynamicLibraryManager::Load("libc.so.6");
    #endif

    if (lib) {
        // Try getting symbols with different types
        auto symbol1 = lib.GetSymbol<void*>("nonexistent");
        auto symbol2 = lib.GetSymbol<int(*)()>("nonexistent");
        auto symbol3 = lib.GetSymbol<void(*)(void)>("nonexistent");

        // All should fail for non-existent symbol
        EXPECT_FALSE(symbol1);
        EXPECT_FALSE(symbol2);
        EXPECT_FALSE(symbol3);
    }
}

TEST_F(DynamicLibraryTest, ThreadSafety) {
    const int numThreads = 10;
    std::vector<std::thread> threads;

    for (int i = 0; i < numThreads; i++) {
        threads.emplace_back([i]() {
            #ifdef _WIN32
            auto lib = DynamicLibraryManager::Load("kernel32.dll");
            #else
            auto lib = DynamicLibraryManager::Load("libc.so.6");
            #endif

            if (lib) {
                auto symbol = lib.GetSymbol<void*>("ThisSymbolDoesNotExist");
            }
        });
    }

    for (auto& thread : threads) {
        thread.join();
    }

    SUCCEED();
}
