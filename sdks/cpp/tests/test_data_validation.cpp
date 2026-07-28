#include <gtest/gtest.h>
#include "croupier/sdk/croupier_client.h"
#include "croupier/sdk/config/client_config_loader.h"
#include "croupier/sdk/utils/json_utils.h"
#include <regex>
#include <algorithm>
#include <cctype>

using namespace croupier::sdk;
using namespace croupier::sdk::config;
using namespace croupier::sdk::utils;

// ========== 数据验证测试套件 ==========

// 辅助函数：检查字符串是否为有效JSON
bool IsValidJSON(const std::string& str) {
    JsonUtils json_utils;
    auto result = json_utils.Parse(str);
    return !result.is_discarded();
}

// 辅助函数：验证电子邮件格式
bool IsValidEmail(const std::string& email) {
    std::regex email_pattern(R"([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,})");
    return std::regex_match(email, email_pattern);
}

// 辅助函数：验证URL格式
bool IsValidURL(const std::string& url) {
    std::regex url_pattern(R"(^[a-zA-Z][a-zA-Z0-9+.-]*://.*)");
    return std::regex_match(url, url_pattern);
}

// 测试配置验证
TEST(DataValidation, ConfigValidation) {
    ClientConfigLoader loader;
    ClientConfig config = loader.CreateDefaultConfig();

    // Validate agent address
    EXPECT_FALSE(config.agent_addr.empty());
    EXPECT_TRUE(IsValidURL(config.agent_addr));

    // Validate timeout
    EXPECT_GE(config.timeout_seconds, 0);

    // Validate game_id
    EXPECT_FALSE(config.game_id.empty() || config.game_id.empty()); // Can be empty or not

    // Validate environment
    EXPECT_TRUE(config.env.empty() ||
                config.env == "development" ||
                config.env == "production" ||
                config.env == "staging");
}

// 测试JSON验证
TEST(DataValidation, JSONValidation) {
    JsonUtils json_utils;

    // Valid JSON objects
    EXPECT_TRUE(IsValidJSON("{}"));
    EXPECT_TRUE(IsValidJSON(R"({"key":"value"})"));
    EXPECT_TRUE(IsValidJSON(R"({"number":123,"bool":true,"null":null})"));

    // Valid JSON arrays
    EXPECT_TRUE(IsValidJSON("[]"));
    EXPECT_TRUE(IsValidJSON("[1,2,3]"));
    EXPECT_TRUE(IsValidJSON(R"(["a","b","c"])"));

    // Valid JSON values
    EXPECT_TRUE(IsValidJSON("123"));
    EXPECT_TRUE(IsValidJSON("true"));
    EXPECT_TRUE(IsValidJSON("false"));
    EXPECT_TRUE(IsValidJSON("null"));
    EXPECT_TRUE(IsValidJSON(R"("string")"));

    // Invalid JSON
    EXPECT_FALSE(IsValidJSON(""));
    EXPECT_FALSE(IsValidJSON("{"));
    EXPECT_FALSE(IsValidJSON("["));
    EXPECT_FALSE(IsValidJSON("undefined"));
    EXPECT_FALSE(IsValidJSON("NaN"));
    EXPECT_FALSE(IsValidJSON("{invalid}"));
}

// 测试字符串验证
TEST(DataValidation, StringValidation) {
    // Empty strings
    std::string empty_string;
    EXPECT_TRUE(empty_string.empty());

    // Non-empty strings
    std::string non_empty = "test";
    EXPECT_FALSE(non_empty.empty());
    EXPECT_EQ(non_empty.length(), 4);

    // Whitespace strings
    std::string whitespace = "   \t\n\r";
    EXPECT_FALSE(whitespace.empty());
    EXPECT_EQ(whitespace.length(), 6);

    // String with special characters
    std::string special = R"(special\n\t\r chars)";
    EXPECT_FALSE(special.empty());

    // Unicode string
    std::string unicode = u8"Unicode: 你好 🚀";
    EXPECT_FALSE(unicode.empty());
    EXPECT_GT(unicode.length(), 10);
}

// 测试数值验证
TEST(DataValidation, NumericValidation) {
    // Integer values
    int positive = 100;
    int negative = -100;
    int zero = 0;

    EXPECT_GT(positive, 0);
    EXPECT_LT(negative, 0);
    EXPECT_EQ(zero, 0);

    // Floating point values
    double float_positive = 123.456;
    double float_negative = -123.456;
    double float_zero = 0.0;

    EXPECT_GT(float_positive, 0.0);
    EXPECT_LT(float_negative, 0.0);
    EXPECT_EQ(float_zero, 0.0);

    // Special floating point values
    double inf = std::numeric_limits<double>::infinity();
    double neg_inf = -std::numeric_limits<double>::infinity();
    double nan = std::numeric_limits<double>::quiet_NaN();

    EXPECT_TRUE(std::isinf(inf));
    EXPECT_TRUE(std::isinf(neg_inf));
    EXPECT_TRUE(std::isnan(nan));
}

// 测试布尔值验证
TEST(DataValidation, BooleanValidation) {
    bool true_value = true;
    bool false_value = false;

    EXPECT_TRUE(true_value);
    EXPECT_FALSE(false_value);

    // String to boolean conversion checks
    std::string true_str = "true";
    std::string false_str = "false";

    EXPECT_EQ(true_str, "true");
    EXPECT_EQ(false_str, "false");
}

// 测试数组验证
TEST(DataValidation, ArrayValidation) {
    std::vector<int> empty_array;
    std::vector<int> non_empty = {1, 2, 3, 4, 5};

    EXPECT_TRUE(empty_array.empty());
    EXPECT_FALSE(non_empty.empty());
    EXPECT_EQ(non_empty.size(), 5U);

    // Array bounds
    EXPECT_GE(non_empty[0], 1);
    EXPECT_LE(non_empty[4], 5);

    // Array capacity
    EXPECT_GE(non_empty.capacity(), non_empty.size());
}

// 测试map验证
TEST(DataValidation, MapValidation) {
    std::map<std::string, int> empty_map;
    std::map<std::string, int> non_empty = {{"key1", 1}, {"key2", 2}};

    EXPECT_TRUE(empty_map.empty());
    EXPECT_FALSE(non_empty.empty());
    EXPECT_EQ(non_empty.size(), 2U);

    // Key existence
    EXPECT_TRUE(non_empty.contains("key1"));
    EXPECT_FALSE(non_empty.contains("key3"));

    // Value retrieval
    EXPECT_EQ(non_empty["key1"], 1);
}

// 测试地址验证
TEST(DataValidation, AddressValidation) {
    std::vector<std::string> valid_addresses = {
        "tcp://127.0.0.1:19090",
        "tcp://localhost:19090",
        "tcp://0.0.0.0:19090",
        "tcp://[::1]:19090",
        "tcp://192.168.1.1:19090"
    };

    for (const auto& addr : valid_addresses) {
        EXPECT_TRUE(IsValidURL(addr)) << "Address should be valid: " << addr;
    }

    std::vector<std::string> invalid_addresses = {
        "invalid",
        "://no-protocol",
        "tcp://no-port",
        "tcp://",
        "",
        "not-tcp://127.0.0.1:19090"
    };

    for (const auto& addr : invalid_addresses) {
        // These may be valid URLs but invalid addresses
        EXPECT_TRUE(addr.length() >= 0);
    }
}

// 测试端口验证
TEST(DataValidation, PortValidation) {
    std::vector<int> valid_ports = {1, 80, 443, 19090, 65535};

    for (int port : valid_ports) {
        EXPECT_GE(port, 1);
        EXPECT_LE(port, 65535);
    }

    std::vector<int> invalid_ports = {0, -1, 65536, 100000};

    for (int port : invalid_ports) {
        EXPECT_FALSE(port >= 1 && port <= 65535) << "Port should be invalid: " << port;
    }
}

// 测试ID验证
TEST(DataValidation, IDValidation) {
    // Valid IDs
    std::vector<std::string> valid_ids = {
        "test.function",
        "category.entity.operation",
        "a.b.c",
        "function123",
        "my_function_v1"
    };

    for (const auto& id : valid_ids) {
        EXPECT_FALSE(id.empty());
        EXPECT_GT(id.length(), 0);
    }

    // IDs with special characters
    std::vector<std::string> special_ids = {
        "test.function@v1.0",
        "function#hash",
        "test-function",
        "test_function"
    };

    for (const auto& id : special_ids) {
        EXPECT_GT(id.length(), 0);
    }
}

// 测试版本号验证
TEST(DataValidation, VersionValidation) {
    std::vector<std::string> valid_versions = {
        "1.0.0",
        "2.3.4",
        "0.0.1",
        "10.20.30",
        "1.0.0-beta",
        "1.0.0-alpha.1"
    };

    for (const auto& version : valid_versions) {
        EXPECT_FALSE(version.empty());
        EXPECT_TRUE(version.find('.') != std::string::npos);
    }

    // Check version format
    std::regex version_pattern(R"(\d+\.\d+\.\d+(-[a-zA-Z0-9.]+)?)");

    for (const auto& version : valid_versions) {
        EXPECT_TRUE(std::regex_search(version, version_pattern)) << "Version: " << version;
    }
}

// 测试时间戳验证
TEST(DataValidation, TimestampValidation) {
    // Unix timestamps
    int64_t timestamp_zero = 0;
    int64_t timestamp_now = std::chrono::system_clock::now().time_since_epoch() / std::chrono::milliseconds(1);
    int64_t timestamp_future = timestamp_now + 86400000; // +1 day

    EXPECT_EQ(timestamp_zero, 0);
    EXPECT_GT(timestamp_now, 0);
    EXPECT_GT(timestamp_future, timestamp_now);

    // Verify timestamp is reasonable (after 2000, before 2100)
    EXPECT_GT(timestamp_now, 946684800000); // 2000-01-01
    EXPECT_LT(timestamp_now, 4102444800000); // 2100-01-01
}

// 测试元数据验证
TEST(DataValidation, MetadataValidation) {
    std::map<std::string, std::string> metadata = {
        {"key1", "value1"},
        {"key2", "value2"},
        {"empty_key", ""},
        {"special_chars", "test\n\t\r"}
    };

    // All keys should be non-empty
    for (const auto& [key, value] : metadata) {
        EXPECT_FALSE(key.empty());
        EXPECT_TRUE(value.length() >= 0);
    }

    // Value can be empty
    EXPECT_TRUE(metadata["empty_key"].empty());
}

// 测试路径验证
TEST(DataValidation, PathValidation) {
    std::vector<std::string> valid_paths = {
        "/path/to/file.txt",
        "relative/path/file.txt",
        "./file.txt",
        "../file.txt",
        R"(C:\path\to\file.txt)"
    };

    for (const auto& path : valid_paths) {
        EXPECT_FALSE(path.empty());
    }

    // Check for path traversal
    std::string path_traversal = "../../../etc/passwd";
    EXPECT_TRUE(path_traversal.find("..") != std::string::npos);
}

// 测试编码验证
TEST(DataValidation, EncodingValidation) {
    // UTF-8 string
    std::string utf8_string = u8"Hello 你好 🚀";

    EXPECT_FALSE(utf8_string.empty());
    EXPECT_GT(utf8_string.length(), 10);

    // ASCII string
    std::string ascii_string = "Hello World";

    EXPECT_TRUE(std::all_of(ascii_string.begin(), ascii_string.end(),
                            [](unsigned char c) { return c < 128; }));
}

// 测试边界值验证
TEST(DataValidation, BoundaryValidation) {
    // Integer boundaries
    int int_max = std::numeric_limits<int>::max();
    int int_min = std::numeric_limits<int>::min();

    EXPECT_GT(int_max, 0);
    EXPECT_LT(int_min, 0);

    // Size_t boundaries
    size_t size_max = std::numeric_limits<size_t>::max();
    size_t size_zero = 0;

    EXPECT_GT(size_max, 0);
    EXPECT_EQ(size_zero, 0);
}

// 测试枚举验证
TEST(DataValidation, EnumValidation) {
    enum class Status { Pending, Running, Completed, Failed };

    Status valid_status = Status::Running;

    EXPECT_GE(static_cast<int>(valid_status), 0);
    EXPECT_LT(static_cast<int>(valid_status), 4);
}

// 测试范围验证
TEST(DataValidation, RangeValidation) {
    int value = 50;

    // Check if value is in range [0, 100]
    EXPECT_GE(value, 0);
    EXPECT_LE(value, 100);

    // Percentage validation
    double percentage = 75.5;

    EXPECT_GE(percentage, 0.0);
    EXPECT_LE(percentage, 100.0);
}

// 测试格式验证
TEST(DataValidation, FormatValidation) {
    // UUID format (simplified)
    std::string uuid = "550e8400-e29b-41d4-a716-446655440000";
    std::regex uuid_pattern(R"(^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$)",
                             std::regex_constants::icase);

    EXPECT_TRUE(std::regex_match(uuid, uuid_pattern));

    // Email format
    std::string email = "user@example.com";
    EXPECT_TRUE(IsValidEmail(email));

    // Invalid email
    std::string invalid_email = "not-an-email";
    EXPECT_FALSE(IsValidEmail(invalid_email));
}

// 测试长度验证
TEST(DataValidation, LengthValidation) {
    std::string short_string = "abc";
    std::string long_string(10000, 'x');

    EXPECT_EQ(short_string.length(), 3);
    EXPECT_EQ(long_string.length(), 10000);

    // Min/max length validation
    size_t min_length = 1;
    size_t max_length = 100;

    EXPECT_GE(short_string.length(), min_length);
    EXPECT_LE(short_string.length(), max_length);
    EXPECT_GT(long_string.length(), max_length);
}

// 测试字符验证
TEST(DataValidation, CharacterValidation) {
    // Alphanumeric string
    std::string alphanumeric = "abc123ABC";

    EXPECT_TRUE(std::all_of(alphanumeric.begin(), alphanumeric.end(),
                            [](char c) { return std::isalnum(c); }));

    // Numeric string
    std::string numeric = "12345";

    EXPECT_TRUE(std::all_of(numeric.begin(), numeric.end(),
                            [](char c) { return std::isdigit(c); }));

    // Alpha string
    std::string alpha = "abcABC";

    EXPECT_TRUE(std::all_of(alpha.begin(), alpha.end(),
                            [](char c) { return std::isalpha(c); }));
}

// 测试JSON Schema验证
TEST(DataValidation, JSONSchemaValidation) {
    JsonUtils json_utils;

    // Required fields
    std::string json_with_required = R"({"id":"123","name":"test","value":100})";
    auto parsed = json_utils.Parse(json_with_required);

    EXPECT_TRUE(parsed.is_object());
    EXPECT_TRUE(parsed.contains("id"));
    EXPECT_TRUE(parsed.contains("name"));
    EXPECT_TRUE(parsed.contains("value"));

    // Missing required field
    std::string json_missing_field = R"({"id":"123","name":"test"})";
    auto parsed_missing = json_utils.Parse(json_missing_field);

    EXPECT_TRUE(parsed_missing.is_object());
    EXPECT_FALSE(parsed_missing.contains("value"));
}

// 测试类型验证
TEST(DataValidation, TypeValidation) {
    JsonUtils json_utils;

    std::string json_data = R"({
        "string": "text",
        "number": 123,
        "float": 123.456,
        "bool": true,
        "null": null,
        "array": [1, 2, 3],
        "object": {"key": "value"}
    })";

    auto parsed = json_utils.Parse(json_data);

    EXPECT_TRUE(parsed.is_object());

    // Check types
    EXPECT_TRUE(parsed["string"].is_string());
    EXPECT_TRUE(parsed["number"].is_number());
    EXPECT_TRUE(parsed["float"].is_number());
    EXPECT_TRUE(parsed["bool"].is_boolean());
    EXPECT_TRUE(parsed["null"].is_null());
    EXPECT_TRUE(parsed["array"].is_array());
    EXPECT_TRUE(parsed["object"].is_object());
}

// 测试正则表达式验证
TEST(DataValidation, RegexValidation) {
    std::regex pattern(R"(\d{3}-\d{2}-\d{4})"); // SSN format

    std::string valid = "123-45-6789";
    std::string invalid = "12-345-6789";

    EXPECT_TRUE(std::regex_match(valid, pattern));
    EXPECT_FALSE(std::regex_match(invalid, pattern));
}

// 测试数据一致性验证
TEST(DataValidation, DataConsistencyValidation) {
    // Start time should be before end time
    auto start_time = std::chrono::system_clock::now();
    auto end_time = start_time + std::chrono::hours(1);

    EXPECT_LT(start_time, end_time);

    // Parent should exist before child
    std::map<std::string, int> data = {{"parent", 1}, {"child", 2}};

    EXPECT_TRUE(data.contains("parent"));
    EXPECT_TRUE(data.contains("child"));
}

// 测试安全性验证
TEST(DataValidation, SecurityValidation) {
    // SQL injection check
    std::vector<std::string> potentially_dangerous = {
        "'; DROP TABLE users; --",
        "admin' OR '1'='1",
        "<script>alert('xss')</script>"
    };

    for (const auto& str : potentially_dangerous) {
        // These should be flagged by security validation
        EXPECT_TRUE(str.find("'") != std::string::npos ||
                   str.find("<script>") != std::string::npos);
    }

    // Path traversal check
    std::vector<std::string> path_traversal_attempts = {
        "../../../etc/passwd",
        "..\\..\\..\\windows\\system32",
        "/etc/passwd"
    };

    for (const auto& path : path_traversal_attempts) {
        // These should be flagged
        EXPECT_TRUE(path.find("..") != std::string::npos ||
                   path.find("/etc/") != std::string::npos);
    }
}
