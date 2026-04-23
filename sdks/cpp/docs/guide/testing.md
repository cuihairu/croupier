# 测试指南

本指南介绍如何测试使用 Croupier C++ SDK 的应用程序。

## 单元测试

### 使用 Google Test

```cpp
#include <gtest/gtest.h>
#include "croupier/sdk/croupier_client.h"

class CroupierClientTest : public ::testing::Test {
protected:
    void SetUp() override {
        config_.game_id = "test-game";
        config_.env = "test";
        config_.agent_addr = "127.0.0.1:19090";
        config_.insecure = true;
    }

    ClientConfig config_;
};

TEST_F(CroupierClientTest, RegisterFunction) {
    CroupierClient client(config_);

    FunctionDescriptor desc;
    desc.id = "test.function";
    desc.version = "0.1.0";

    bool result = client.RegisterFunction(desc, [](auto ctx, auto payload) {
        return R"({"success": true})";
    });

    EXPECT_TRUE(result);
}

TEST_F(CroupierClientTest, ParseJSON) {
    std::string json = R"({"key": "value"})";
    auto result = utils::ParseJSON(json);

    EXPECT_TRUE(result.contains("key"));
    EXPECT_EQ(result["key"], "value");
}

TEST_F(CroupierClientTest, HandlerFunction) {
    auto handler = [](const std::string& ctx, const std::string& payload) -> std::string {
        auto data = utils::ParseJSON(payload);
        int a = data["a"];
        int b = data["b"];
        return utils::ToJSON({{"result", a + b}});
    };

    std::string result = handler("", R"({"a": 2, "b": 3})");
    auto response = utils::ParseJSON(result);

    EXPECT_EQ(response["result"], 5);
}
```

### 运行测试

```bash
# 启用测试构建
./scripts/build.sh --tests ON

# 运行所有测试
cd build
ctest

# 运行特定测试
./tests/unit_tests --gtest_filter="CroupierClientTest.*"

# 详细输出
./tests/unit_tests --gtest_filter="*" --gtest_print_time=1
```

## Mock 测试

### Mock Agent

```cpp
class MockAgent {
public:
    MockAgent(int port) : port_(port) {}

    void Start() {
        server_thread_ = std::thread([this]() {
            // 启动简单的 gRPC 测试服务器
            // ...
        });
    }

    void Stop() {
        // 停止服务器
        server_thread_.join();
    }

    void SetResponse(const std::string& response) {
        mock_response_ = response;
    }

private:
    int port_;
    std::thread server_thread_;
    std::string mock_response_;
};

// 使用 Mock
TEST_F(CroupierClientTest, ConnectToMockAgent) {
    MockAgent mock(19090);
    mock.Start();

    CroupierClient client(config_);
    EXPECT_TRUE(client.Connect());

    mock.Stop();
}
```

### 函数 Mock

```cpp
class MockFunctionHandler {
public:
    MOCK_METHOD(std::string, Handle, (const std::string&, const std::string&));

    std::string operator()(const std::string& ctx, const std::string& payload) {
        return Handle(ctx, payload);
    }
};

using ::testing::Return;
using ::testing::_;

TEST_F(CroupierClientTest, MockHandler) {
    MockFunctionHandler mock_handler;
    EXPECT_CALL(mock_handler, Handle(_, _))
        .WillOnce(Return(R"({"result": "mocked"})"));

    std::string result = mock_handler("", "{}");
    auto response = utils::ParseJSON(result);

    EXPECT_EQ(response["result"], "mocked");
}
```

## 集成测试

### 测试场景

```cpp
class IntegrationTest : public ::testing::Test {
protected:
    void SetUp() override {
        // 启动测试 Agent
        StartTestAgent();

        // 配置客户端
        config_.game_id = "integration-test";
        config_.env = "test";
        config_.agent_addr = "127.0.0.1:19090";
        config_.insecure = true;
    }

    void TearDown() override {
        StopTestAgent();
    }

    void StartTestAgent() {
        // 启动实际的 Agent 进程
        agent_process_ = std::make_unique<Process>("croupier-agent", "-f", "test-config.yaml");
        agent_process_->Start();
        std::this_thread::sleep_for(std::chrono::seconds(2));
    }

    void StopTestAgent() {
        if (agent_process_) {
            agent_process_->Stop();
        }
    }

    ClientConfig config_;
    std::unique_ptr<Process> agent_process_;
};

TEST_F(IntegrationTest, FullWorkflow) {
    CroupierClient client(config_);

    // 注册函数
    FunctionDescriptor desc;
    desc.id = "test.workflow";
    desc.version = "0.1.0";

    bool registered = client.RegisterFunction(desc, [](auto ctx, auto payload) {
        return R"({"status": "completed"})";
    });
    ASSERT_TRUE(registered);

    // 连接
    ASSERT_TRUE(client.Connect());

    // 等待注册完成
    std::this_thread::sleep_for(std::chrono::milliseconds(500));

    // 这里可以通过 Agent 调用函数验证...

    client.Close();
}
```

## 性能测试

### 基准测试

```cpp
#include <benchmark/benchmark.h>

static void BM_FunctionCall(benchmark::State& state) {
    auto handler = [](const std::string& ctx, const std::string& payload) {
        return utils::ToJSON({{"result", 42}});
    };

    std::string payload = R"({"value": 100})";

    for (auto _ : state) {
        std::string result = handler("", payload);
        benchmark::DoNotOptimize(result);
    }
}

BENCHMARK(BM_FunctionCall);

static void BM_JSONParsing(benchmark::State& state) {
    std::string json = R"({"key": "value", "number": 42})";

    for (auto _ : state) {
        auto result = utils::ParseJSON(json);
        benchmark::DoNotOptimize(result);
    }
}

BENCHMARK(BM_JSONParsing);

BENCHMARK_MAIN();
```

### 运行性能测试

```bash
# 构建 benchmark
cmake -DBUILD_BENCHMARKS=ON ..
make benchmark

# 运行
./benchmark --benchmark_filter=.*
```

## 端到端测试

### 测试脚本

```bash
#!/bin/bash
# e2e_test.sh

set -e

echo "Starting end-to-end tests..."

# 启动 Server
croupier-server -f test-config/server.yaml &
SERVER_PID=$!
sleep 2

# 启动 Agent
croupier-agent -f test-config/agent.yaml &
AGENT_PID=$!
sleep 2

# 启动测试客户端
./build/test-client &
CLIENT_PID=$!
sleep 2

# 运行测试用例
python tests/e2e/run_tests.py

# 清理
kill $CLIENT_PID $AGENT_PID $SERVER_PID

echo "E2E tests completed!"
```

### Python 测试客户端

```python
import requests
import unittest

class E2ETest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.base_url = "http://localhost:18780"

    def test_invoke_function(self):
        response = requests.post(
            f"{self.base_url}/api/v1/functions/test.workflow/invoke",
            json={"player_id": "test123", "amount": 100},
            headers={"X-Game-ID": "integration-test"}
        )
        self.assertEqual(response.status_code, 200)
        self.assertEqual(response.json()["status"], "completed")

    def test_virtual_object_get(self):
        response = requests.get(
            f"{self.base_url}/api/v1/objects/wallet.entity/test123",
            headers={"X-Game-ID": "integration-test"}
        )
        self.assertEqual(response.status_code, 200)

if __name__ == "__main__":
    unittest.main()
```

## 测试配置

### 测试配置文件

```json
{
  "game_id": "test-game",
  "env": "test",
  "service_id": "test-service",
  "agent_addr": "127.0.0.1:19090",
  "insecure": true,
  "timeout_seconds": 5,
  "auto_reconnect": false
}
```

### CMake 测试配置

```cmake
# CMakeLists.txt
include(CTest)
include(GoogleTest)

enable_testing()

# 测试可执行文件
add_executable(unit_tests
    tests/unit_tests.cpp
    tests/mock_agent.cpp
)

target_link_libraries(unit_tests
    PRIVATE
    croupier-sdk::croupier-sdk
    GTest::gtest_main
)

# 注册测试
discover_tests(unit_tests)

# 集成测试
add_executable(integration_tests
    tests/integration_tests.cpp
)

target_link_libraries(integration_tests
    PRIVATE
    croupier-sdk::croupier-sdk
    GTest::gtest_main
)

add_test(NAME Integration COMMAND integration_tests)
```

## 覆盖率报告

### 启用代码覆盖率

```bash
# GCC/Clang
cmake -DCMAKE_BUILD_TYPE=Debug \
      -DCMAKE_CXX_FLAGS="--coverage" \
      -DCMAKE_EXE_LINKER_FLAGS="--coverage" \
      -B build

# 运行测试
cd build
ctest

# 生成覆盖率报告
lcov --capture --directory . --output-file coverage.info
lcov --remove coverage.info '/usr/*' --output-file coverage.info
genhtml coverage.info --output-directory coverage_html
```

### 覆盖率目标

| 组件 | 目标覆盖率 |
|------|-----------|
| 核心库 | 80%+ |
| 工具函数 | 70%+ |
| 示例代码 | 60%+ |

## 最佳实践

### 1. 测试隔离

```cpp
// 每个测试使用独立的命名空间
TEST_F(CroupierClientTest, RegisterFunction_UniqueName) {
    std::string unique_id = "test.function." + std::to_string(std::rand());
    // ...
}
```

### 2. 清理资源

```cpp
TEST_F(CroupierClientTest, ResourceCleanup) {
    CroupierClient client(config_);
    client.Connect();

    // 测试逻辑...

    // 确保清理
    client.Close();
}
```

### 3. 超时控制

```cpp
TEST_F(CroupierClientTest, Timeout) {
    // 设置测试超时
    ASSERT_DURATION_LE(5s, {
        client.Connect();
    });
}
```

### 4. 参数化测试

```cpp
class ParamTest : public ::testing::TestWithParam<int> {};

TEST_P(ParamTest, VariousAmounts) {
    int amount = GetParam();
    auto result = ProcessTransfer(amount);
    EXPECT_TRUE(result["success"]);
}

INSTANTIATE_TEST_SUITE_P(
    TransferAmounts,
    ParamTest,
    ::testing::Values(1, 10, 100, 1000)
);
```
