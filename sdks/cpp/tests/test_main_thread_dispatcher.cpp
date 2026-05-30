// Copyright 2025 Croupier Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

#include <gtest/gtest.h>
#include <atomic>
#include <future>
#include <thread>
#include <vector>

#include "croupier/sdk/threading/main_thread_dispatcher.h"

using namespace croupier::sdk::threading;

class MainThreadDispatcherTest : public ::testing::Test {
protected:
    void SetUp() override {
        MainThreadDispatcher::GetInstance().Reset();
        MainThreadDispatcher::GetInstance().Initialize();
    }

    void TearDown() override {
        MainThreadDispatcher::GetInstance().Reset();
    }
};

TEST_F(MainThreadDispatcherTest, Initialize_SetsMainThread) {
    EXPECT_TRUE(MainThreadDispatcher::GetInstance().IsInitialized());
    EXPECT_TRUE(MainThreadDispatcher::GetInstance().IsMainThread());
}

TEST_F(MainThreadDispatcherTest, Enqueue_FromMainThread_ExecutesImmediately) {
    bool executed = false;

    MainThreadDispatcher::GetInstance().Enqueue([&executed]() {
        executed = true;
    });

    EXPECT_TRUE(executed);
}

TEST_F(MainThreadDispatcherTest, Enqueue_FromBackgroundThread_QueuesForLater) {
    std::atomic<bool> executed{false};

    auto future = std::async(std::launch::async, [&executed]() {
        MainThreadDispatcher::GetInstance().Enqueue([&executed]() {
            executed.store(true);
        });
    });
    future.wait();

    // Should not be executed yet
    EXPECT_FALSE(executed.load());
    EXPECT_EQ(1u, MainThreadDispatcher::GetInstance().GetPendingCount());

    // Process the queue
    int processed = MainThreadDispatcher::GetInstance().ProcessQueue();

    EXPECT_EQ(1, processed);
    EXPECT_TRUE(executed.load());
    EXPECT_EQ(0u, MainThreadDispatcher::GetInstance().GetPendingCount());
}

TEST_F(MainThreadDispatcherTest, Enqueue_WithData_PassesDataCorrectly) {
    std::string received_data;

    auto future = std::async(std::launch::async, [&received_data]() {
        MainThreadDispatcher::GetInstance().Enqueue<std::string>(
            [&received_data](std::string data) {
                received_data = data;
            },
            "test-data"
        );
    });
    future.wait();

    MainThreadDispatcher::GetInstance().ProcessQueue();

    EXPECT_EQ("test-data", received_data);
}

TEST_F(MainThreadDispatcherTest, ProcessQueue_RespectsMaxCount) {
    std::atomic<int> count{0};

    // Enqueue 10 callbacks from background thread
    std::vector<std::future<void>> futures;
    for (int i = 0; i < 10; i++) {
        futures.push_back(std::async(std::launch::async, [&count]() {
            MainThreadDispatcher::GetInstance().Enqueue([&count]() {
                count.fetch_add(1);
            });
        }));
    }
    for (auto& f : futures) {
        f.wait();
    }

    EXPECT_EQ(10u, MainThreadDispatcher::GetInstance().GetPendingCount());

    // Process only 5
    int processed = MainThreadDispatcher::GetInstance().ProcessQueue(5);

    EXPECT_EQ(5, processed);
    EXPECT_EQ(5, count.load());
    EXPECT_EQ(5u, MainThreadDispatcher::GetInstance().GetPendingCount());

    // Process remaining
    processed = MainThreadDispatcher::GetInstance().ProcessQueue();
    EXPECT_EQ(5, processed);
    EXPECT_EQ(10, count.load());
}

TEST_F(MainThreadDispatcherTest, ProcessQueue_HandlesExceptions) {
    std::vector<int> results;

    auto future = std::async(std::launch::async, [&results]() {
        MainThreadDispatcher::GetInstance().Enqueue([&results]() {
            results.push_back(1);
        });
        MainThreadDispatcher::GetInstance().Enqueue([]() {
            throw std::runtime_error("Test exception");
        });
        MainThreadDispatcher::GetInstance().Enqueue([&results]() {
            results.push_back(3);
        });
    });
    future.wait();

    // Should process all callbacks even with exception
    int processed = MainThreadDispatcher::GetInstance().ProcessQueue();

    EXPECT_EQ(3, processed);
    EXPECT_EQ(2u, results.size());
    EXPECT_NE(results.end(), std::find(results.begin(), results.end(), 1));
    EXPECT_NE(results.end(), std::find(results.begin(), results.end(), 3));
}

TEST_F(MainThreadDispatcherTest, Clear_RemovesAllPendingCallbacks) {
    std::vector<std::future<void>> futures;
    for (int i = 0; i < 5; i++) {
        futures.push_back(std::async(std::launch::async, []() {
            MainThreadDispatcher::GetInstance().Enqueue([]() {});
        }));
    }
    for (auto& f : futures) {
        f.wait();
    }

    EXPECT_EQ(5u, MainThreadDispatcher::GetInstance().GetPendingCount());

    MainThreadDispatcher::GetInstance().Clear();

    EXPECT_EQ(0u, MainThreadDispatcher::GetInstance().GetPendingCount());
}

TEST_F(MainThreadDispatcherTest, SetMaxProcessPerFrame_LimitsProcessing) {
    MainThreadDispatcher::GetInstance().SetMaxProcessPerFrame(3);

    // Verify the setting is stored (test via ProcessQueue behavior)
    std::vector<std::future<void>> futures;
    for (int i = 0; i < 10; i++) {
        futures.push_back(std::async(std::launch::async, []() {
            MainThreadDispatcher::GetInstance().Enqueue([]() {});
        }));
    }
    for (auto& f : futures) {
        f.wait();
    }

    int processed = MainThreadDispatcher::GetInstance().ProcessQueue();
    EXPECT_EQ(3, processed);

    // Reset to unlimited
    MainThreadDispatcher::GetInstance().SetMaxProcessPerFrame(0);
}

TEST_F(MainThreadDispatcherTest, Enqueue_NullCallback_IsIgnored) {
    size_t initial_count = MainThreadDispatcher::GetInstance().GetPendingCount();

    auto future = std::async(std::launch::async, []() {
        MainThreadDispatcher::GetInstance().Enqueue(nullptr);
    });
    future.wait();

    EXPECT_EQ(initial_count, MainThreadDispatcher::GetInstance().GetPendingCount());
}

TEST_F(MainThreadDispatcherTest, ConcurrentEnqueue_IsThreadSafe) {
    std::atomic<int> counter{0};
    const int thread_count = 10;
    const int enqueues_per_thread = 100;

    std::vector<std::future<void>> futures;
    for (int t = 0; t < thread_count; t++) {
        futures.push_back(std::async(std::launch::async, [&counter]() {
            for (int i = 0; i < 100; i++) {
                MainThreadDispatcher::GetInstance().Enqueue([&counter]() {
                    counter.fetch_add(1);
                });
            }
        }));
    }

    for (auto& f : futures) {
        f.wait();
    }

    // Process all
    int total_processed = 0;
    int processed;
    while ((processed = MainThreadDispatcher::GetInstance().ProcessQueue(100)) > 0) {
        total_processed += processed;
    }

    EXPECT_EQ(thread_count * enqueues_per_thread, counter.load());
    EXPECT_EQ(thread_count * enqueues_per_thread, total_processed);
}

TEST_F(MainThreadDispatcherTest, IsMainThread_ReturnsFalse_OnBackgroundThread) {
    std::atomic<bool> is_main_thread{true};

    auto future = std::async(std::launch::async, [&is_main_thread]() {
        is_main_thread.store(MainThreadDispatcher::GetInstance().IsMainThread());
    });
    future.wait();

    EXPECT_FALSE(is_main_thread.load());
}
