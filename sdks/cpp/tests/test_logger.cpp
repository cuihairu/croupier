#include <gtest/gtest.h>

#include "croupier/sdk/logger.h"

#include <sstream>
#include <regex>
#include <thread>

namespace croupier {
namespace sdk {
namespace test {

// Redirect cout to capture log output
class CoutRedirect {
public:
    CoutRedirect() : old_(std::cout.rdbuf()), buffer_() {
        std::cout.rdbuf(buffer_.rdbuf());
    }

    ~CoutRedirect() {
        std::cout.rdbuf(old_);
    }

    std::string GetOutput() const {
        return buffer_.str();
    }

    void Clear() {
        buffer_.str("");
    }

private:
    std::streambuf* old_;
    std::stringstream buffer_;
};

class LoggerTest : public ::testing::Test {
protected:
    void SetUp() override {
        // Reset logger to INFO level for each test
        Logger::GetInstance().SetLevel(Logger::Level::INFO);
    }

    void TearDown() override {
        // Restore default level
        Logger::GetInstance().SetLevel(Logger::Level::INFO);
    }
};

// ========== MaskSensitive Tests ==========

TEST_F(LoggerTest, MaskSensitiveEmpty) {
    std::string result = MaskSensitive("");
    EXPECT_TRUE(result.empty());
}

TEST_F(LoggerTest, MaskSensitiveShort) {
    // Short string (<= 6 chars) should be fully masked
    std::string result = MaskSensitive("abc");
    EXPECT_EQ(result, "***");
}

TEST_F(LoggerTest, MaskSensitiveMedium) {
    // Medium string should show first 3 and last 3
    std::string result = MaskSensitive("abcdefghij");
    EXPECT_EQ(result, "abc...hij");
}

TEST_F(LoggerTest, MaskSensitiveToken) {
    // Token-like string
    std::string result = MaskSensitive("eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9");
    EXPECT_EQ(result, "eyJ...IkpXVCJ9");
}

// ========== MaskFully Tests ==========

TEST_F(LoggerTest, MaskFullyEmpty) {
    std::string result = MaskFully("");
    EXPECT_TRUE(result.empty());
}

TEST_F(LoggerTest, MaskFullyNormal) {
    std::string result = MaskFully("password");
    EXPECT_EQ(result, "********");
}

TEST_F(LoggerTest, MaskFullyWithSpecialChars) {
    std::string result = MaskFully("p@ssw0rd!");
    EXPECT_EQ(result, "*********");
}

// ========== MaskJsonSensitive Tests ==========

TEST_F(LoggerTest, MaskJsonSensitiveSingleKey) {
    std::string json = R"({"token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"})";
    std::vector<std::string> keys = {"token"};

    std::string result = MaskJsonSensitive(json, keys);
    EXPECT_NE(result.find("token"), std::string::npos);
    EXPECT_EQ(result.find("eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"), std::string::npos);
}

TEST_F(LoggerTest, MaskJsonSensitiveMultipleKeys) {
    std::string json = R"({"token": "abc123", "password": "secret", "user": "alice"})";
    std::vector<std::string> keys = {"token", "password"};

    std::string result = MaskJsonSensitive(json, keys);
    EXPECT_EQ(result.find("abc123"), std::string::npos);
    EXPECT_EQ(result.find("secret"), std::string::npos);
    EXPECT_NE(result.find("alice"), std::string::npos);  // user should not be masked
}

TEST_F(LoggerTest, MaskJsonSensitiveNoMatch) {
    std::string json = R"({"user": "alice", "email": "alice@example.com"})";
    std::vector<std::string> keys = {"token", "password"};

    std::string result = MaskJsonSensitive(json, keys);
    EXPECT_NE(result.find("alice"), std::string::npos);
}

// ========== Logger Singleton Tests ==========

TEST_F(LoggerTest, GetInstanceReturnsSame) {
    Logger& logger1 = Logger::GetInstance();
    Logger& logger2 = Logger::GetInstance();

    // Should be the same instance (same address)
    EXPECT_EQ(&logger1, &logger2);
}

// ========== Logger SetLevel Tests ==========

TEST_F(LoggerTest, SetLevel) {
    Logger& logger = Logger::GetInstance();

    logger.SetLevel(Logger::Level::DEBUG);
    EXPECT_TRUE(logger.IsEnabled(Logger::Level::DEBUG));
    EXPECT_TRUE(logger.IsEnabled(Logger::Level::INFO));
    EXPECT_TRUE(logger.IsEnabled(Logger::Level::WARN));
    EXPECT_TRUE(logger.IsEnabled(Logger::Level::ERR));

    logger.SetLevel(Logger::Level::INFO);
    EXPECT_FALSE(logger.IsEnabled(Logger::Level::DEBUG));
    EXPECT_TRUE(logger.IsEnabled(Logger::Level::INFO));
    EXPECT_TRUE(logger.IsEnabled(Logger::Level::WARN));
    EXPECT_TRUE(logger.IsEnabled(Logger::Level::ERR));

    logger.SetLevel(Logger::Level::WARN);
    EXPECT_FALSE(logger.IsEnabled(Logger::Level::DEBUG));
    EXPECT_FALSE(logger.IsEnabled(Logger::Level::INFO));
    EXPECT_TRUE(logger.IsEnabled(Logger::Level::WARN));
    EXPECT_TRUE(logger.IsEnabled(Logger::Level::ERR));

    logger.SetLevel(Logger::Level::ERR);
    EXPECT_FALSE(logger.IsEnabled(Logger::Level::DEBUG));
    EXPECT_FALSE(logger.IsEnabled(Logger::Level::INFO));
    EXPECT_FALSE(logger.IsEnabled(Logger::Level::WARN));
    EXPECT_TRUE(logger.IsEnabled(Logger::Level::ERR));

    logger.SetLevel(Logger::Level::OFF);
    EXPECT_FALSE(logger.IsEnabled(Logger::Level::DEBUG));
    EXPECT_FALSE(logger.IsEnabled(Logger::Level::INFO));
    EXPECT_FALSE(logger.IsEnabled(Logger::Level::WARN));
    EXPECT_FALSE(logger.IsEnabled(Logger::Level::ERR));
}

// ========== Logger SetLevelFromString Tests ==========

TEST_F(LoggerTest, SetLevelFromString) {
    Logger& logger = Logger::GetInstance();

    logger.SetLevelFromString("DEBUG");
    EXPECT_TRUE(logger.IsEnabled(Logger::Level::DEBUG));

    logger.SetLevelFromString("INFO");
    EXPECT_TRUE(logger.IsEnabled(Logger::Level::INFO));

    logger.SetLevelFromString("WARN");
    EXPECT_TRUE(logger.IsEnabled(Logger::Level::WARN));

    logger.SetLevelFromString("ERROR");
    EXPECT_TRUE(logger.IsEnabled(Logger::Level::ERR));

    logger.SetLevelFromString("OFF");
    EXPECT_FALSE(logger.IsEnabled(Logger::Level::INFO));
}

TEST_F(LoggerTest, SetLevelFromStringLowerCase) {
    Logger& logger = Logger::GetInstance();

    logger.SetLevelFromString("debug");
    EXPECT_TRUE(logger.IsEnabled(Logger::Level::DEBUG));

    logger.SetLevelFromString("info");
    EXPECT_TRUE(logger.IsEnabled(Logger::Level::INFO));

    logger.SetLevelFromString("warn");
    EXPECT_TRUE(logger.IsEnabled(Logger::Level::WARN));

    logger.SetLevelFromString("error");
    EXPECT_TRUE(logger.IsEnabled(Logger::Level::ERR));

    logger.SetLevelFromString("off");
    EXPECT_FALSE(logger.IsEnabled(Logger::Level::INFO));
}

// ========== Logger Disable Tests ==========

TEST_F(LoggerTest, DisableTrue) {
    Logger& logger = Logger::GetInstance();

    logger.Disable(true);
    EXPECT_FALSE(logger.IsEnabled(Logger::Level::INFO));
}

TEST_F(LoggerTest, DisableFalse) {
    Logger& logger = Logger::GetInstance();

    logger.Disable(false);
    EXPECT_TRUE(logger.IsEnabled(Logger::Level::INFO));
}

// ========== Logger Log Tests ==========

TEST_F(LoggerTest, LogInfoEnabled) {
    CoutRedirect redirect;
    Logger& logger = Logger::GetInstance();
    logger.SetLevel(Logger::Level::INFO);

    logger.Log(Logger::Level::INFO, "TestComponent", "Test message");

    std::string output = redirect.GetOutput();
    EXPECT_NE(output.find("TestComponent"), std::string::npos);
    EXPECT_NE(output.find("Test message"), std::string::npos);
    EXPECT_NE(output.find("INFO"), std::string::npos);
}

TEST_F(LoggerTest, LogDebugDisabled) {
    CoutRedirect redirect;
    Logger& logger = Logger::GetInstance();
    logger.SetLevel(Logger::Level::INFO);

    logger.Log(Logger::Level::DEBUG, "TestComponent", "Debug message");

    std::string output = redirect.GetOutput();
    EXPECT_TRUE(output.empty());  // DEBUG should be filtered out
}

TEST_F(LoggerTest, LogWarnEnabled) {
    CoutRedirect redirect;
    Logger& logger = Logger::GetInstance();
    logger.SetLevel(Logger::Level::WARN);

    logger.Log(Logger::Level::WARN, "TestComponent", "Warning message");

    std::string output = redirect.GetOutput();
    EXPECT_NE(output.find("TestComponent"), std::string::npos);
    EXPECT_NE(output.find("Warning message"), std::string::npos);
    EXPECT_NE(output.find("WARN"), std::string::npos);
}

TEST_F(LoggerTest, LogErrorEnabled) {
    CoutRedirect redirect;
    Logger& logger = Logger::GetInstance();
    logger.SetLevel(Logger::Level::ERR);

    logger.Log(Logger::Level::ERR, "TestComponent", "Error message");

    std::string output = redirect.GetOutput();
    EXPECT_NE(output.find("TestComponent"), std::string::npos);
    EXPECT_NE(output.find("Error message"), std::string::npos);
    EXPECT_NE(output.find("ERROR"), std::string::npos);
}

// ========== Logger LogMasked Tests ==========

TEST_F(LoggerTest, LogMasked) {
    CoutRedirect redirect;
    Logger& logger = Logger::GetInstance();

    logger.LogMasked(Logger::Level::INFO, "TestComponent", "Token: ",
                     "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9");

    std::string output = redirect.GetOutput();
    EXPECT_NE(output.find("TestComponent"), std::string::npos);
    EXPECT_NE(output.find("masked"), std::string::npos);
    // The sensitive value should be partially masked
    EXPECT_EQ(output.find("eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"), std::string::npos);
}

// ========== Logger Convenience Methods Tests ==========

TEST_F(LoggerTest, DebugMethod) {
    CoutRedirect redirect;
    Logger& logger = Logger::GetInstance();
    logger.SetLevel(Logger::Level::DEBUG);

    logger.Debug("DebugComponent", "Debug output");

    std::string output = redirect.GetOutput();
    EXPECT_NE(output.find("DebugComponent"), std::string::npos);
    EXPECT_NE(output.find("Debug output"), std::string::npos);
}

TEST_F(LoggerTest, InfoMethod) {
    CoutRedirect redirect;
    Logger& logger = Logger::GetInstance();

    logger.Info("InfoComponent", "Info output");

    std::string output = redirect.GetOutput();
    EXPECT_NE(output.find("InfoComponent"), std::string::npos);
    EXPECT_NE(output.find("Info output"), std::string::npos);
}

TEST_F(LoggerTest, WarnMethod) {
    CoutRedirect redirect;
    Logger& logger = Logger::GetInstance();
    logger.SetLevel(Logger::Level::WARN);

    logger.Warn("WarnComponent", "Warning output");

    std::string output = redirect.GetOutput();
    EXPECT_NE(output.find("WarnComponent"), std::string::npos);
    EXPECT_NE(output.find("Warning output"), std::string::npos);
}

TEST_F(LoggerTest, ErrorMethod) {
    CoutRedirect redirect;
    Logger& logger = Logger::GetInstance();

    logger.Error("ErrorComponent", "Error output");

    std::string output = redirect.GetOutput();
    EXPECT_NE(output.find("ErrorComponent"), std::string::npos);
    EXPECT_NE(output.find("Error output"), std::string::npos);
}

// ========== ComponentLogger Tests ==========

TEST_F(LoggerTest, ComponentLoggerConstructor) {
    ComponentLogger comp_logger("MyComponent");
    EXPECT_NO_THROW(comp_logger.Info("Test message"));
}

TEST_F(LoggerTest, ComponentLoggerDebug) {
    CoutRedirect redirect;
    Logger::GetInstance().SetLevel(Logger::Level::DEBUG);

    ComponentLogger comp_logger("DebugComp");
    comp_logger.Debug("Debug message");

    std::string output = redirect.GetOutput();
    EXPECT_NE(output.find("DebugComp"), std::string::npos);
    EXPECT_NE(output.find("Debug message"), std::string::npos);
}

TEST_F(LoggerTest, ComponentLoggerInfo) {
    CoutRedirect redirect;

    ComponentLogger comp_logger("InfoComp");
    comp_logger.Info("Info message");

    std::string output = redirect.GetOutput();
    EXPECT_NE(output.find("InfoComp"), std::string::npos);
    EXPECT_NE(output.find("Info message"), std::string::npos);
}

TEST_F(LoggerTest, ComponentLoggerWarn) {
    CoutRedirect redirect;
    Logger::GetInstance().SetLevel(Logger::Level::WARN);

    ComponentLogger comp_logger("WarnComp");
    comp_logger.Warn("Warning message");

    std::string output = redirect.GetOutput();
    EXPECT_NE(output.find("WarnComp"), std::string::npos);
}

TEST_F(LoggerTest, ComponentLoggerError) {
    CoutRedirect redirect;

    ComponentLogger comp_logger("ErrorComp");
    comp_logger.Error("Error message");

    std::string output = redirect.GetOutput();
    EXPECT_NE(output.find("ErrorComp"), std::string::npos);
}

// ========== Thread Safety Tests ==========

TEST_F(LoggerTest, ConcurrentLogging) {
    CoutRedirect redirect;
    Logger& logger = Logger::GetInstance();
    logger.SetLevel(Logger::Level::INFO);

    std::vector<std::thread> threads;
    for (int i = 0; i < 10; ++i) {
        threads.emplace_back([&logger, i]() {
            logger.Info("ThreadTest", "Message " + std::to_string(i));
        });
    }

    for (auto& t : threads) {
        t.join();
    }

    std::string output = redirect.GetOutput();
    // All messages should be present
    for (int i = 0; i < 10; ++i) {
        EXPECT_NE(output.find("Message " + std::to_string(i)), std::string::npos);
    }
}

}  // namespace test
}  // namespace sdk
}  // namespace croupier
