#include <gtest/gtest.h>
#include "croupier/sdk/croupier_client.h"
#include "croupier/sdk/config/client_config_loader.h"
#include <thread>
#include <chrono>
#include <atomic>
#include <mutex>
#include <barrier>
#include <latch>
#include <vector>
#include <random>

using namespace croupier::sdk;
using namespace croupier::sdk::config;

// ========== 并发测试套件 ==========

// 测试多线程创建客户端
TEST(ConcurrencyTest, MultipleClientsCreation) {
    const int num_threads = 10;
    std::vector<std::thread> threads;
    std::atomic<int> success_count(0);

    for (int i = 0; i < num_threads; i++) {
        threads.emplace_back([&success_count]() {
            try {
                ClientConfigLoader loader;
                ClientConfig config = loader.CreateDefaultConfig();
                CroupierClient client(config);
                success_count++;
            } catch (...) {
                // Ignore exceptions for this test
            }
        });
    }

    for (auto& thread : threads) {
        thread.join();
    }

    EXPECT_EQ(success_count.load(), num_threads);
}

// 测试多线程配置加载
TEST(ConcurrencyTest, ConcurrentConfigLoad) {
    const int num_threads = 20;
    std::vector<std::thread> threads;
    std::atomic<int> success_count(0);

    for (int i = 0; i < num_threads; i++) {
        threads.emplace_back([&success_count]() {
            try {
                ClientConfigLoader loader;
                ClientConfig config = loader.CreateDefaultConfig();
                EXPECT_FALSE(config.agent_addr.empty());
                success_count++;
            } catch (...) {
                // Ignore exceptions
            }
        });
    }

    for (auto& thread : threads) {
        thread.join();
    }

    EXPECT_EQ(success_count.load(), num_threads);
}

// 测试多线程 JSON 解析
TEST(ConcurrencyTest, ConcurrentJSONParsing) {
    const int num_threads = 10;
    const int num_operations = 100;
    std::vector<std::thread> threads;
    std::atomic<int> success_count(0);
    std::string json_data = R"({"test":"data","number":123,"nested":{"value":"test"}})";

    for (int i = 0; i < num_threads; i++) {
        threads.emplace_back([&json_data, &success_count, num_operations]() {
            try {
                JsonUtils json_utils;
                for (int j = 0; j < num_operations; j++) {
                    auto parsed = json_utils.Parse(json_data);
                    EXPECT_TRUE(parsed.is_object());
                    success_count++;
                }
            } catch (...) {
                // Ignore exceptions
            }
        });
    }

    for (auto& thread : threads) {
        thread.join();
    }

    EXPECT_EQ(success_count.load(), num_threads * num_operations);
}

// 测试共享数据访问
TEST(ConcurrencyTest, SharedConfigAccess) {
    const int num_threads = 10;
    const int num_operations = 1000;
    std::vector<std::thread> threads;
    ClientConfigLoader loader;
    ClientConfig config = loader.CreateDefaultConfig();
    std::atomic<int> read_count(0);
    std::mutex config_mutex;

    for (int i = 0; i < num_threads; i++) {
        threads.emplace_back([&config, &config_mutex, &read_count, num_operations]() {
            for (int j = 0; j < num_operations; j++) {
                std::lock_guard<std::mutex> lock(config_mutex);
                std::string addr = config.agent_addr;
                EXPECT_FALSE(addr.empty());
                read_count++;
            }
        });
    }

    for (auto& thread : threads) {
        thread.join();
    }

    EXPECT_EQ(read_count.load(), num_threads * num_operations);
}

// 测试竞态条件
TEST(ConcurrencyTest, RaceConditionTest) {
    const int num_threads = 10;
    std::vector<std::thread> threads;
    std::atomic<int> counter(0);

    for (int i = 0; i < num_threads; i++) {
        threads.emplace_back([&counter]() {
            for (int j = 0; j < 1000; j++) {
                counter.fetch_add(1, std::memory_order_relaxed);
            }
        });
    }

    for (auto& thread : threads) {
        thread.join();
    }

    EXPECT_EQ(counter.load(), num_threads * 1000);
}

// 测试死锁场景
TEST(ConcurrencyTest, NoDeadlockWithMultipleLocks) {
    const int num_threads = 10;
    std::vector<std::thread> threads;
    std::mutex mutex1, mutex2;
    std::atomic<int> success_count(0);

    for (int i = 0; i < num_threads; i++) {
        threads.emplace_back([&mutex1, &mutex2, &success_count]() {
            try {
                // Always lock in the same order to avoid deadlock
                std::lock(mutex1, mutex2);
                std::lock_guard<std::mutex> lock1(mutex1, std::adopt_lock);
                std::lock_guard<std::mutex> lock2(mutex2, std::adopt_lock);
                std::this_thread::sleep_for(std::chrono::milliseconds(1));
                success_count++;
            } catch (...) {
                // Ignore exceptions
            }
        });
    }

    for (auto& thread : threads) {
        thread.join();
    }

    EXPECT_EQ(success_count.load(), num_threads);
}

// 测试读写锁模式
TEST(ConcurrencyTest, ReadWritePattern) {
    const int num_readers = 10;
    const int num_writers = 2;
    std::vector<std::thread> threads;
    std::mutex mutex;
    std::atomic<int> read_count(0);
    std::atomic<int> write_count(0);
    int shared_data = 0;

    // Reader threads
    for (int i = 0; i < num_readers; i++) {
        threads.emplace_back([&mutex, &shared_data, &read_count]() {
            for (int j = 0; j < 100; j++) {
                std::lock_guard<std::mutex> lock(mutex);
                int data = shared_data;
                (void)data; // Suppress unused warning
                read_count++;
            }
        });
    }

    // Writer threads
    for (int i = 0; i < num_writers; i++) {
        threads.emplace_back([&mutex, &shared_data, &write_count]() {
            for (int j = 0; j < 100; j++) {
                std::lock_guard<std::mutex> lock(mutex);
                shared_data++;
                write_count++;
            }
        });
    }

    for (auto& thread : threads) {
        thread.join();
    }

    EXPECT_EQ(read_count.load(), num_readers * 100);
    EXPECT_EQ(write_count.load(), num_writers * 100);
}

// 测试线程局部存储
TEST(ConcurrencyTest, ThreadLocalData) {
    const int num_threads = 10;
    std::vector<std::thread> threads;
    std::atomic<int> success_count(0);

    for (int i = 0; i < num_threads; i++) {
        threads.emplace_back([&success_count]() {
            thread_local int thread_data = 0;
            for (int j = 0; j < 100; j++) {
                thread_data++;
            }
            EXPECT_EQ(thread_data, 100);
            success_count++;
        });
    }

    for (auto& thread : threads) {
        thread.join();
    }

    EXPECT_EQ(success_count.load(), num_threads);
}

// 测试同步点
TEST(ConcurrencyTest, BarrierSynchronization) {
    const int num_threads = 5;
    std::vector<std::thread> threads;
    std::barrier barrier(num_threads);
    std::atomic<int> phase1_count(0);
    std::atomic<int> phase2_count(0);

    for (int i = 0; i < num_threads; i++) {
        threads.emplace_back([&barrier, &phase1_count, &phase2_count]() {
            // Phase 1
            phase1_count++;
            barrier.arrive_and_wait();

            // Phase 2 (all threads have completed phase 1)
            EXPECT_EQ(phase1_count.load(), num_threads);
            phase2_count++;
            barrier.arrive_and_wait();

            // Verify
            EXPECT_EQ(phase2_count.load(), num_threads);
        });
    }

    for (auto& thread : threads) {
        thread.join();
    }

    EXPECT_EQ(phase1_count.load(), num_threads);
    EXPECT_EQ(phase2_count.load(), num_threads);
}

// 测试latch同步
TEST(ConcurrencyTest, LatchSynchronization) {
    const int num_threads = 10;
    std::vector<std::thread> threads;
    std::latch latch(num_threads);
    std::atomic<int> arrival_count(0);

    // Worker threads
    for (int i = 0; i < num_threads; i++) {
        threads.emplace_back([&latch, &arrival_count]() {
            // Simulate work
            std::this_thread::sleep_for(std::chrono::milliseconds(10));
            arrival_count++;
            latch.count_down();
        });
    }

    // Wait for all threads
    latch.wait();

    EXPECT_EQ(arrival_count.load(), num_threads);

    for (auto& thread : threads) {
        thread.join();
    }
}

// 测试条件变量
TEST(ConcurrencyTest, ConditionVariableNotification) {
    std::mutex mutex;
    std::condition_variable cv;
    bool ready = false;
    std::atomic<int> processed_count(0);
    const int num_threads = 5;

    std::vector<std::thread> threads;

    // Worker threads
    for (int i = 0; i < num_threads; i++) {
        threads.emplace_back([&cv, &mutex, &ready, &processed_count]() {
            std::unique_lock<std::mutex> lock(mutex);
            cv.wait(lock, [&ready] { return ready; });
            processed_count++;
        });
    }

    // Notify all threads
    std::this_thread::sleep_for(std::chrono::milliseconds(100));
    {
        std::lock_guard<std::mutex> lock(mutex);
        ready = true;
    }
    cv.notify_all();

    for (auto& thread : threads) {
        thread.join();
    }

    EXPECT_EQ(processed_count.load(), num_threads);
}

// 测试原子操作
TEST(ConcurrencyTest, AtomicOperations) {
    const int num_threads = 10;
    const int num_operations = 1000;
    std::vector<std::thread> threads;
    std::atomic<int> counter(0);

    for (int i = 0; i < num_threads; i++) {
        threads.emplace_back([&counter, num_operations]() {
            for (int j = 0; j < num_operations; j++) {
                // Various atomic operations
                counter.fetch_add(1, std::memory_order_relaxed);
                counter.fetch_sub(1, std::memory_order_relaxed);
                counter.fetch_add(1, std::memory_order_relaxed);
            }
        });
    }

    for (auto& thread : threads) {
        thread.join();
    }

    EXPECT_EQ(counter.load(), num_threads * num_operations);
}

// 测试 CAS (Compare-And-Swap)
TEST(ConcurrencyTest, CompareAndSwap) {
    const int num_threads = 10;
    const int num_operations = 100;
    std::vector<std::thread> threads;
    std::atomic<int> value(0);
    std::atomic<int> success_count(0);

    for (int i = 0; i < num_threads; i++) {
        threads.emplace_back([&value, &success_count, num_operations]() {
            for (int j = 0; j < num_operations; j++) {
                int expected = value.load();
                int desired = expected + 1;
                if (value.compare_exchange_weak(expected, desired)) {
                    success_count++;
                }
            }
        });
    }

    for (auto& thread : threads) {
        thread.join();
    }

    // At least some operations should succeed
    EXPECT_GT(success_count.load(), 0);
    EXPECT_EQ(value.load(), success_count.load());
}

// 测试内存顺序
TEST(ConcurrencyTest, MemoryOrdering) {
    std::atomic<int> data1(0);
    std::atomic<int> data2(0);
    std::atomic<bool> ready(false);

    std::thread writer([&]() {
        data1.store(1, std::memory_order_relaxed);
        data2.store(2, std::memory_order_release);
        ready.store(true, std::memory_order_release);
    });

    std::thread reader([&]() {
        while (!ready.load(std::memory_order_acquire)) {
            // Spin
        }
        EXPECT_EQ(data2.load(std::memory_order_acquire), 2);
    });

    writer.join();
    reader.join();
}

// 测试高并发场景
TEST(ConcurrencyTest, HighConcurrencyStress) {
    const int num_threads = 50;
    const int num_operations = 1000;
    std::vector<std::thread> threads;
    std::atomic<int> total_operations(0);

    for (int i = 0; i < num_threads; i++) {
        threads.emplace_back([&total_operations, num_operations]() {
            for (int j = 0; j < num_operations; j++) {
                total_operations.fetch_add(1, std::memory_order_relaxed);
            }
        });
    }

    for (auto& thread : threads) {
        thread.join();
    }

    EXPECT_EQ(total_operations.load(), num_threads * num_operations);
}

// 测试线程池模式
TEST(ConcurrencyTest, ThreadPoolPattern) {
    const int num_workers = 4;
    const int num_tasks = 20;
    std::vector<std::thread> workers;
    std::queue<std::function<void()>> tasks;
    std::mutex queue_mutex;
    std::condition_variable cv;
    std::atomic<bool> stop(false);
    std::atomic<int> completed_tasks(0);

    // Worker threads
    for (int i = 0; i < num_workers; i++) {
        workers.emplace_back([&]() {
            while (!stop.load()) {
                std::function<void()> task;
                {
                    std::unique_lock<std::mutex> lock(queue_mutex);
                    cv.wait(lock, [&] { return stop.load() || !tasks.empty(); });

                    if (stop.load() && tasks.empty()) {
                        return;
                    }

                    task = tasks.front();
                    tasks.pop();
                }

                task();
                completed_tasks++;
            }
        });
    }

    // Submit tasks
    {
        std::lock_guard<std::mutex> lock(queue_mutex);
        for (int i = 0; i < num_tasks; i++) {
            tasks.push([&]() {
                std::this_thread::sleep_for(std::chrono::milliseconds(1));
            });
        }
    }
    cv.notify_all();

    // Wait for completion
    std::this_thread::sleep_for(std::chrono::milliseconds(100));
    stop.store(true);
    cv.notify_all();

    for (auto& worker : workers) {
        worker.join();
    }

    EXPECT_EQ(completed_tasks.load(), num_tasks);
}

// 测试生产者-消费者模式
TEST(ConcurrencyTest, ProducerConsumer) {
    const int num_producers = 3;
    const int num_consumers = 2;
    const int items_per_producer = 100;
    std::vector<std::thread> threads;
    std::queue<int> queue;
    std::mutex queue_mutex;
    std::condition_variable cv;
    std::atomic<bool> done(false);
    std::atomic<int> produced_count(0);
    std::atomic<int> consumed_count(0);

    // Producer threads
    for (int i = 0; i < num_producers; i++) {
        threads.emplace_back([&, i]() {
            for (int j = 0; j < items_per_producer; j++) {
                {
                    std::lock_guard<std::mutex> lock(queue_mutex);
                    queue.push(i * items_per_producer + j);
                }
                cv.notify_one();
                produced_count++;
            }
        });
    }

    // Consumer threads
    for (int i = 0; i < num_consumers; i++) {
        threads.emplace_back([&]() {
            while (!done.load() || !queue.empty()) {
                std::unique_lock<std::mutex> lock(queue_mutex);
                cv.wait(lock, [&] { return done.load() || !queue.empty(); });

                while (!queue.empty()) {
                    queue.pop();
                    consumed_count++;
                }
            }
        });
    }

    // Wait for producers
    for (int i = 0; i < num_producers; i++) {
        threads[i].join();
    }

    done.store(true);
    cv.notify_all();

    // Wait for consumers
    for (int i = num_producers; i < threads.size(); i++) {
        threads[i].join();
    }

    EXPECT_EQ(produced_count.load(), num_producers * items_per_producer);
    EXPECT_EQ(consumed_count.load(), num_producers * items_per_producer);
}

// 测试并发创建销毁
TEST(ConcurrencyTest, RapidCreateDestroy) {
    const int num_threads = 10;
    const int num_iterations = 100;
    std::vector<std::thread> threads;
    std::atomic<int> success_count(0);

    for (int i = 0; i < num_threads; i++) {
        threads.emplace_back([&success_count, num_iterations]() {
            for (int j = 0; j < num_iterations; j++) {
                try {
                    ClientConfigLoader loader;
                    ClientConfig config = loader.CreateDefaultConfig();
                    CroupierClient client(config);
                    // Client is destroyed immediately
                    success_count++;
                } catch (...) {
                    // Ignore exceptions
                }
            }
        });
    }

    for (auto& thread : threads) {
        thread.join();
    }

    EXPECT_EQ(success_count.load(), num_threads * num_iterations);
}

// 测试超时场景
TEST(ConcurrencyTest, TimeoutScenarios) {
    std::mutex mutex;
    std::condition_variable cv;
    bool ready = false;

    std::thread waiter([&]() {
        std::unique_lock<std::mutex> lock(mutex);
        auto result = cv.wait_for(lock, std::chrono::milliseconds(100));
        EXPECT_FALSE(result); // Should timeout
    });

    std::thread notifier([&]() {
        std::this_thread::sleep_for(std::chrono::milliseconds(200));
        std::lock_guard<std::mutex> lock(mutex);
        ready = true;
        cv.notify_one();
    });

    waiter.join();
    notifier.join();
}

// 测试递归锁
TEST(ConcurrencyTest, RecursiveLock) {
    std::recursive_mutex mutex;
    int counter = 0;

    std::thread thread([&]() {
        std::lock_guard<std::recursive_mutex> lock1(mutex);
        counter++;
        {
            std::lock_guard<std::recursive_mutex> lock2(mutex);
            counter++;
            {
                std::lock_guard<std::recursive_mutex> lock3(mutex);
                counter++;
            }
        }
    });

    thread.join();

    EXPECT_EQ(counter, 3);
}

// 测试once_flag
TEST(ConcurrencyTest, CallOnce) {
    const int num_threads = 10;
    std::vector<std::thread> threads;
    std::once_flag flag;
    std::atomic<int> init_count(0);

    for (int i = 0; i < num_threads; i++) {
        threads.emplace_back([&flag, &init_count]() {
            std::call_once(flag, [&init_count]() {
                init_count++;
            });
        });
    }

    for (auto& thread : threads) {
        thread.join();
    }

    EXPECT_EQ(init_count.load(), 1);
}

// 测试并发vector操作
TEST(ConcurrencyTest, ConcurrentVectorPushBack) {
    const int num_threads = 10;
    const int num_operations = 100;
    std::vector<std::thread> threads;
    std::vector<int> vec;
    std::mutex vec_mutex;

    for (int i = 0; i < num_threads; i++) {
        threads.emplace_back([&vec, &vec_mutex, num_operations, i]() {
            for (int j = 0; j < num_operations; j++) {
                std::lock_guard<std::mutex> lock(vec_mutex);
                vec.push_back(i * num_operations + j);
            }
        });
    }

    for (auto& thread : threads) {
        thread.join();
    }

    EXPECT_EQ(vec.size(), num_threads * num_operations);
}

// 测试并发map操作
TEST(ConcurrencyTest, ConcurrentMapOperations) {
    const int num_threads = 10;
    const int num_operations = 100;
    std::vector<std::thread> threads;
    std::map<std::string, int> map;
    std::mutex map_mutex;

    for (int i = 0; i < num_threads; i++) {
        threads.emplace_back([&map, &map_mutex, num_operations, i]() {
            for (int j = 0; j < num_operations; j++) {
                std::lock_guard<std::mutex> lock(map_mutex);
                map["key_" + std::to_string(i * num_operations + j)] = j;
            }
        });
    }

    for (auto& thread : threads) {
        thread.join();
    }

    EXPECT_EQ(map.size(), num_threads * num_operations);
}
