#include <gtest/gtest.h>
#include "croupier/sdk/croupier_client.h"
#include "croupier/sdk/config/client_config_loader.h"
#include "croupier/sdk/utils/json_utils.h"
#include <chrono>
#include <thread>
#include <vector>
#include <random>
#include <algorithm>

using namespace croupier::sdk;
using namespace croupier::sdk::config;
using namespace croupier::sdk::utils;

// ========== 性能测试套件 ==========

// 辅助函数：测量执行时间
template<typename Func>
auto MeasureTime(const std::string& label, Func&& func) {
    auto start = std::chrono::high_resolution_clock::now();
    func();
    auto end = std::chrono::high_resolution_clock::now();
    auto duration = std::chrono::duration_cast<std::chrono::milliseconds>(end - start);
    std::cout << label << ": " << duration.count() << "ms" << std::endl;
    return duration;
}

// 辅助函数：生成随机字符串
std::string GenerateRandomString(size_t length) {
    const char charset[] = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789";
    std::random_device rd;
    std::mt19937 gen(rd());
    std::uniform_int_distribution<> dis(0, sizeof(charset) - 2);

    std::string result;
    result.reserve(length);
    for (size_t i = 0; i < length; i++) {
        result += charset[dis(gen)];
    }
    return result;
}

// 测试配置加载性能
TEST(PerformanceTest, ConfigLoadingSpeed) {
    ClientConfigLoader loader;

    auto duration = MeasureTime("Load 1000 configs", [&]() {
        for (int i = 0; i < 1000; i++) {
            ClientConfig config = loader.CreateDefaultConfig();
        }
    });

    EXPECT_LT(duration.count(), 1000); // Should complete in less than 1 second
}

// 测试JSON解析性能
TEST(PerformanceTest, JSONParsingSpeed) {
    JsonUtils json_utils;
    std::string json_data = R"({"key":"value","number":123,"nested":{"data":"test"}})";

    auto duration = MeasureTime("Parse 1000 JSON strings", [&]() {
        for (int i = 0; i < 1000; i++) {
            auto parsed = json_utils.Parse(json_data);
        }
    });

    EXPECT_LT(duration.count(), 1000); // Should complete in less than 1 second
}

// 测试大型JSON解析性能
TEST(PerformanceTest, LargeJSONParsing) {
    JsonUtils json_utils;

    // Create large JSON (100KB)
    std::string large_json = "{";
    for (int i = 0; i < 1000; i++) {
        large_json += "\"key" + std::to_string(i) + "\":" + std::to_string(i);
        if (i < 999) large_json += ",";
    }
    large_json += "}";

    auto duration = MeasureTime("Parse 100KB JSON", [&]() {
        for (int i = 0; i < 100; i++) {
            auto parsed = json_utils.Parse(large_json);
        }
    });

    EXPECT_LT(duration.count(), 5000); // Should complete in less than 5 seconds
}

// 测试字符串操作性能
TEST(PerformanceTest, StringOperations) {
    auto duration = MeasureTime("String concatenations", [&]() {
        std::string result;
        for (int i = 0; i < 10000; i++) {
            result += "test_" + std::to_string(i) + "_";
        }
    });

    EXPECT_LT(duration.count(), 1000); // Should complete in less than 1 second
}

// 测试vector性能
TEST(PerformanceTest, VectorOperations) {
    auto duration = MeasureTime("Vector push_back 100000 items", [&]() {
        std::vector<int> vec;
        vec.reserve(100000);
        for (int i = 0; i < 100000; i++) {
            vec.push_back(i);
        }
    });

    EXPECT_LT(duration.count(), 100); // Should complete in less than 100ms
}

// 测试map性能
TEST(PerformanceTest, MapOperations) {
    auto duration = MeasureTime("Map insert 10000 items", [&]() {
        std::map<std::string, int> map;
        for (int i = 0; i < 10000; i++) {
            map["key_" + std::to_string(i)] = i;
        }
    });

    EXPECT_LT(duration.count(), 1000); // Should complete in less than 1 second
}

// 测试内存分配性能
TEST(PerformanceTest, MemoryAllocation) {
    auto duration = MeasureTime("Allocate and free 1000 buffers", [&]() {
        for (int i = 0; i < 1000; i++) {
            std::vector<int> buffer(10000);
            std::fill(buffer.begin(), buffer.end(), 42);
        }
    });

    EXPECT_LT(duration.count(), 1000); // Should complete in less than 1 second
}

// 测试文件I/O性能
TEST(PerformanceTest, FileIO) {
    std::string test_file = "perf_test_temp.json";

    // Write
    auto write_duration = MeasureTime("Write 100KB file", [&]() {
        std::ofstream file(test_file);
        for (int i = 0; i < 10000; i++) {
            file << "{\"key\":\"value\",\"data\":" << i << "}\n";
        }
        file.close();
    });

    // Read
    auto read_duration = MeasureTime("Read 100KB file", [&]() {
        std::ifstream file(test_file);
        std::string line;
        int count = 0;
        while (std::getline(file, line)) {
            count++;
        }
        file.close();
    });

    std::remove(test_file.c_str());

    EXPECT_LT(write_duration.count(), 1000);
    EXPECT_LT(read_duration.count(), 1000);
}

// 测试线程创建性能
TEST(PerformanceTest, ThreadCreation) {
    auto duration = MeasureTime("Create and join 100 threads", [&]() {
        std::vector<std::thread> threads;
        for (int i = 0; i < 100; i++) {
            threads.emplace_back([]() {
                std::this_thread::sleep_for(std::chrono::milliseconds(1));
            });
        }
        for (auto& t : threads) {
            t.join();
        }
    });

    EXPECT_LT(duration.count(), 5000); // Should complete in less than 5 seconds
}

// 测试锁性能
TEST(PerformanceTest, LockPerformance) {
    std::mutex mutex;
    int counter = 0;

    auto duration = MeasureTime("Lock and unlock 100000 times", [&]() {
        for (int i = 0; i < 100000; i++) {
            std::lock_guard<std::mutex> lock(mutex);
            counter++;
        }
    });

    EXPECT_LT(duration.count(), 1000); // Should complete in less than 1 second
    EXPECT_EQ(counter, 100000);
}

// 测试原子操作性能
TEST(PerformanceTest, AtomicPerformance) {
    std::atomic<int> counter(0);

    auto duration = MeasureTime("Atomic operations 100000 times", [&]() {
        for (int i = 0; i < 100000; i++) {
            counter.fetch_add(1, std::memory_order_relaxed);
        }
    });

    EXPECT_LT(duration.count(), 100); // Should be very fast
    EXPECT_EQ(counter.load(), 100000);
}

// 测试客户端创建性能
TEST(PerformanceTest, ClientCreation) {
    ClientConfigLoader loader;
    ClientConfig config = loader.CreateDefaultConfig();

    auto duration = MeasureTime("Create 100 clients", [&]() {
        for (int i = 0; i < 100; i++) {
            CroupierClient client(config);
        }
    });

    EXPECT_LT(duration.count(), 5000); // Should complete in less than 5 seconds
}

// 测试并发JSON解析性能
TEST(PerformanceTest, ConcurrentJSONParsing) {
    const int num_threads = 4;
    const int num_operations = 250;
    JsonUtils json_utils;
    std::string json_data = R"({"test":"data","number":123})";

    auto duration = MeasureTime("Concurrent JSON parsing (4 threads x 250)", [&]() {
        std::vector<std::thread> threads;
        for (int i = 0; i < num_threads; i++) {
            threads.emplace_back([&]() {
                for (int j = 0; j < num_operations; j++) {
                    auto parsed = json_utils.Parse(json_data);
                }
            });
        }
        for (auto& t : threads) {
            t.join();
        }
    });

    EXPECT_LT(duration.count(), 2000); // Should complete in less than 2 seconds
}

// 测试排序性能
TEST(PerformanceTest, SortingPerformance) {
    std::vector<int> data(100000);
    std::random_device rd;
    std::mt19937 gen(rd());
    std::uniform_int_distribution<> dis(1, 1000000);

    for (auto& val : data) {
        val = dis(gen);
    }

    auto duration = MeasureTime("Sort 100000 integers", [&]() {
        std::sort(data.begin(), data.end());
    });

    EXPECT_LT(duration.count(), 1000); // Should complete in less than 1 second
    EXPECT_TRUE(std::is_sorted(data.begin(), data.end()));
}

// 测试查找性能
TEST(PerformanceTest, LookupPerformance) {
    std::map<std::string, int> map;

    for (int i = 0; i < 10000; i++) {
        map["key_" + std::to_string(i)] = i;
    }

    auto duration = MeasureTime("10000 map lookups", [&]() {
        for (int i = 0; i < 10000; i++) {
            auto it = map.find("key_" + std::to_string(i));
            EXPECT_NE(it, map.end());
        }
    });

    EXPECT_LT(duration.count(), 100); // Should be very fast
}

// 测试字符串格式化性能
TEST(PerformanceTest, StringFormatting) {
    auto duration = MeasureTime("String formatting 10000 times", [&]() {
        for (int i = 0; i < 10000; i++) {
            std::string formatted = "Value: " + std::to_string(i) + ", Data: " + std::to_string(i * 2);
        }
    });

    EXPECT_LT(duration.count(), 500); // Should complete in less than 500ms
}

// 测试随机数生成性能
TEST(PerformanceTest, RandomNumberGeneration) {
    std::random_device rd;
    std::mt19937 gen(rd());
    std::uniform_int_distribution<> dis(1, 1000000);

    auto duration = MeasureTime("Generate 100000 random numbers", [&]() {
        for (int i = 0; i < 100000; i++) {
            volatile int random = dis(gen);
            (void)random;
        }
    });

    EXPECT_LT(duration.count(), 1000); // Should complete in less than 1 second
}

// 测试哈希计算性能
TEST(PerformanceTest, HashCalculation) {
    std::hash<std::string> hasher;
    std::string data = "test_string_for_hashing";

    auto duration = MeasureTime("Hash calculation 100000 times", [&]() {
        for (int i = 0; i < 100000; i++) {
            size_t hash = hasher(data);
            (void)hash;
        }
    });

    EXPECT_LT(duration.count(), 100); // Should be very fast
}

// 测试深拷贝性能
TEST(PerformanceTest, DeepCopyPerformance) {
    std::map<std::string, std::vector<int>> source;

    for (int i = 0; i < 1000; i++) {
        source["key_" + std::to_string(i)] = {i, i*2, i*3};
    }

    auto duration = MeasureTime("Deep copy 1000 items", [&]() {
        auto copy = source;
        (void)copy;
    });

    EXPECT_LT(duration.count(), 100); // Should be fast
}

// 测试动态分配性能
TEST(PerformanceTest, DynamicAllocation) {
    auto duration = MeasureTime("Dynamic allocation 10000 times", [&]() {
        for (int i = 0; i < 10000; i++) {
            int* ptr = new int(i);
            *ptr *= 2;
            delete ptr;
        }
    });

    EXPECT_LT(duration.count(), 1000); // Should complete in less than 1 second
}

// 测试异常处理性能
TEST(PerformanceTest, ExceptionHandling) {
    auto duration = MeasureTime("Exception handling 1000 times", [&]() {
        for (int i = 0; i < 1000; i++) {
            try {
                throw std::runtime_error("test");
            } catch (const std::runtime_error& e) {
                (void)e;
            }
        }
    });

    EXPECT_LT(duration.count(), 1000); // Should complete in less than 1 second
}

// 测试虚函数调用性能
TEST(PerformanceTest, VirtualFunctionCall) {
    struct Base {
        virtual int Calculate(int x) = 0;
        virtual ~Base() = default;
    };

    struct Derived : Base {
        int Calculate(int x) override {
            return x * 2;
        }
    };

    Derived derived;
    Base* base = &derived;

    auto duration = MeasureTime("Virtual function call 1000000 times", [&]() {
        int sum = 0;
        for (int i = 0; i < 1000000; i++) {
            sum += base->Calculate(i);
        }
        (void)sum;
    });

    EXPECT_LT(duration.count(), 1000); // Should complete in less than 1 second
}

// 测试shared_ptr性能
TEST(PerformanceTest, SharedPtrPerformance) {
    auto duration = MeasureTime("Shared_ptr operations 10000 times", [&]() {
        auto ptr = std::make_shared<int>(42);
        for (int i = 0; i < 10000; i++) {
            auto copy = ptr;
            auto another = std::make_shared<int>(i);
            copy = another;
        }
    });

    EXPECT_LT(duration.count(), 1000); // Should complete in less than 1 second
}

// 测试正则表达式性能
TEST(PerformanceTest, RegexPerformance) {
    std::regex pattern(R"(\d{3}-\d{3}-\d{4})");
    std::string text = "Phone: 123-456-7890";

    auto duration = MeasureTime("Regex match 10000 times", [&]() {
        for (int i = 0; i < 10000; i++) {
            std::smatch matches;
            std::regex_search(text, matches, pattern);
        }
    });

    EXPECT_LT(duration.count(), 1000); // Should complete in less than 1 second
}

// 测试时间转换性能
TEST(PerformanceTest, TimeConversion) {
    auto duration = MeasureTime("Time operations 10000 times", [&]() {
        for (int i = 0; i < 10000; i++) {
            auto now = std::chrono::system_clock::now();
            auto timestamp = std::chrono::duration_cast<std::chrono::milliseconds>(
                now.time_since_epoch()).count();
            (void)timestamp;
        }
    });

    EXPECT_LT(duration.count(), 1000); // Should complete in less than 1 second
}

// 测试类型转换性能
TEST(PerformanceTest, TypeConversion) {
    auto duration = MeasureTime("Type conversions 100000 times", [&]() {
        for (int i = 0; i < 100000; i++) {
            std::string str = std::to_string(i);
            int val = std::stoi(str);
            double dval = std::stod(std::to_string(i * 1.5));
            (void)val;
            (void)dval;
        }
    });

    EXPECT_LT(duration.count(), 2000); // Should complete in less than 2 seconds
}

// 测试容器迭代性能
TEST(PerformanceTest, ContainerIteration) {
    std::vector<int> vec(100000);
    for (size_t i = 0; i < vec.size(); i++) {
        vec[i] = i;
    }

    auto duration = MeasureTime("Vector iteration 100000 items", [&]() {
        int sum = 0;
        for (int val : vec) {
            sum += val;
        }
        (void)sum;
    });

    EXPECT_LT(duration.count(), 100); // Should be very fast
}

// 测试内存拷贝性能
TEST(PerformanceTest, MemoryCopy) {
    std::vector<int> source(100000);
    std::vector<int> dest(100000);

    for (size_t i = 0; i < source.size(); i++) {
        source[i] = i;
    }

    auto duration = MeasureTime("Memory copy 100000 integers", [&]() {
        std::copy(source.begin(), source.end(), dest.begin());
    });

    EXPECT_LT(duration.count(), 100); // Should be very fast
    EXPECT_EQ(source, dest);
}

// 测试条件分支性能
TEST(PerformanceTest, ConditionalBranching) {
    auto duration = MeasureTime("Conditional branching 1000000 times", [&]() {
        int count = 0;
        for (int i = 0; i < 1000000; i++) {
            if (i % 2 == 0) {
                count++;
            } else if (i % 3 == 0) {
                count += 2;
            } else {
                count += 3;
            }
        }
        (void)count;
    });

    EXPECT_LT(duration.count(), 100); // Should be very fast
}

// 测试循环性能
TEST(PerformanceTest, LoopPerformance) {
    auto duration = MeasureTime("Loop 10000000 iterations", [&]() {
        volatile int sum = 0;
        for (int i = 0; i < 10000000; i++) {
            sum += i;
        }
    });

    EXPECT_LT(duration.count(), 1000); // Should complete in less than 1 second
}

// 测试函数调用开销
TEST(PerformanceTest, FunctionCallOverhead) {
    std::function<int(int)> func = [](int x) {
        return x * 2;
    };

    auto duration = MeasureTime("Std::function call 1000000 times", [&]() {
        int sum = 0;
        for (int i = 0; i < 1000000; i++) {
            sum += func(i);
        }
        (void)sum;
    });

    EXPECT_LT(duration.count(), 2000); // Should complete in less than 2 seconds
}

// 测试lambda性能
TEST(PerformanceTest, LambdaPerformance) {
    auto lambda = [](int x) { return x * x; };

    auto duration = MeasureTime("Lambda call 1000000 times", [&]() {
        int sum = 0;
        for (int i = 0; i < 1000000; i++) {
            sum += lambda(i);
        }
        (void)sum;
    });

    EXPECT_LT(duration.count(), 1000); // Should complete in less than 1 second
}

// 压力测试：高负载
TEST(PerformanceTest, HighLoadStress) {
    const int num_operations = 100000;

    auto duration = MeasureTime("High load stress test", [&]() {
        std::vector<int> data;
        data.reserve(num_operations);

        for (int i = 0; i < num_operations; i++) {
            data.push_back(i);
        }

        std::sort(data.begin(), data.end());

        long long sum = 0;
        for (int val : data) {
            sum += val;
        }
        (void)sum;
    });

    EXPECT_LT(duration.count(), 5000); // Should complete in less than 5 seconds
}

// 内存使用测试
TEST(PerformanceTest, MemoryUsage) {
    size_t initial_memory = 0;

    auto duration = MeasureTime("Allocate 10MB of data", [&]() {
        std::vector<std::vector<int>> data;

        for (int i = 0; i < 1000; i++) {
            data.push_back(std::vector<int>(2500)); // ~10KB each
        }
    });

    EXPECT_LT(duration.count(), 1000); // Should complete in less than 1 second
}

// 吞吐量测试
TEST(PerformanceTest, Throughput) {
    const int num_items = 100000;
    std::vector<std::string> items;
    items.reserve(num_items);

    auto duration = MeasureTime("Process 100000 items", [&]() {
        for (int i = 0; i < num_items; i++) {
            items.push_back("item_" + std::to_string(i));
        }
    });

    double items_per_second = num_items / (duration.count() / 1000.0);
    std::cout << "Throughput: " << items_per_second << " items/second" << std::endl;

    EXPECT_GT(items_per_second, 10000); // Should process at least 10k items/sec
}
