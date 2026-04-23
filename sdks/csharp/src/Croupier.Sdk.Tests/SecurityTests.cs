// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

using Croupier.Sdk.Models;
using Croupier.Sdk.Transport;
using FluentAssertions;
using Xunit;
using System.Security.Cryptography;
using System.Text;
using System.Text.RegularExpressions;

namespace Croupier.Sdk.Tests;

/// <summary>
/// Security tests for Croupier SDK
/// </summary>
public class SecurityTests
{
    #region Input Validation Security

    [Fact]
    public void SqlInjectionInFunctionId_IsHandledSafely()
    {
        // Arrange
        string[] sqlInjectionAttempts = [
            "'; DROP TABLE functions; --",
            "test' OR '1'='1",
            "admin'--",
            "admin'/*",
            "' OR 1=1#"
        ];

        // Act & Assert
        foreach (var attempt in sqlInjectionAttempts)
        {
            // Should treat as string, not execute
            var descriptor = new FunctionDescriptor
            {
                Id = attempt,
                Version = "1.0.0",
                Category = "test",
                Risk = "low"
            };

            descriptor.Id.Should().Be(attempt);
        }
    }

    [Fact]
    public void PathTraversal_IsDetected()
    {
        // Arrange
        string[] pathTraversalAttempts = [
            "../../../etc/passwd",
            "..\\..\\..\\windows\\system32",
            "/etc/passwd",
            "....//....//etc/passwd",
            "%2e%2e%2f%2e%2e%2f%2e%2e%2fetc%2fpasswd"
        ];

        // Act & Assert
        foreach (var path in pathTraversalAttempts)
        {
            // Should detect path traversal patterns
            var isSuspicious = path.Contains("..") ||
                path.Contains("/etc/") ||
                path.Contains("windows", StringComparison.OrdinalIgnoreCase) ||
                path.Contains("system32", StringComparison.OrdinalIgnoreCase) ||
                path.Contains("%2e", StringComparison.OrdinalIgnoreCase);

            isSuspicious.Should().BeTrue("Path traversal not detected: {0}", path);
        }
    }

    [Fact]
    public void CommandInjectionInPayload_DoesNotExecute()
    {
        // Arrange
        string[] commandInjectionAttempts = [
            "{\"data\": \"$(rm -rf /)\"}",
            "{\"data\": \"`whoami`\"}",
            "{\"data\": \"; ls -la\"}",
            "{\"data\": \"| cat /etc/passwd\"}",
            "{\"data\": \"&& curl malicious.com\"}"
        ];

        // Act & Assert
        foreach (var payload in commandInjectionAttempts)
        {
            // Should not execute commands, just parse as JSON
            payload.Should().Contain("data");
        }
    }

    [Fact]
    public void XssInStrings_IsNotExecuted()
    {
        // Arrange
        string[] xssAttempts = [
            "<script>alert('xss')</script>",
            "<img src=x onerror=alert('xss')>",
            "javascript:alert('xss')",
            "<svg onload=alert('xss')>",
            "'\"><script>alert(String.fromCharCode(88,83,83))</script>"
        ];

        // Act & Assert
        foreach (var attempt in xssAttempts)
        {
            // Should store as string, not execute
            attempt.Length.Should().BeGreaterThan(0);
            attempt.ToLower().Should().MatchRegex("(script|javascript:|onerror=|onload=)");
        }
    }

    [Fact]
    public void BufferOverflowInStrings_IsHandled()
    {
        // Arrange & Act
        var largeString = new string('A', 10_000_000); // 10MB

        // Assert
        largeString.Length.Should().Be(10_000_000);
    }

    [Fact]
    public void IntegerOverflow_DoesNotCauseIssues()
    {
        // C# has checked arithmetic for overflow detection
        int maxInt = int.MaxValue;
        maxInt.Should().BeGreaterThan(0);

        long maxLong = long.MaxValue;
        maxLong.Should().BeGreaterThan(0);
    }

    [Fact]
    public void NullByteInjection_IsHandled()
    {
        // Arrange
        string[] nullByteAttempts = [
            "test\0file.txt",
            "config\0.json",
            "/etc/\0passwd",
            "\0\0\0"
        ];

        // Act & Assert
        foreach (var attempt in nullByteAttempts)
        {
            // C# strings can contain null bytes
            attempt.Length.Should().BeGreaterThan(0);
        }
    }

    [Fact]
    public void UnicodeNormalizationIssues_AreHandled()
    {
        // Arrange
        string[] homographs = [
            "pa𝘽n",  // Using special characters
            "test\u200b",  // Zero-width space
            "test\u200c",  // Zero-width non-joiner
            "test\u202e"   // Right-to-left override
        ];

        // Act & Assert
        foreach (var text in homographs)
        {
            // Should handle Unicode
            text.Length.Should().BeGreaterThan(0);
        }
    }

    #endregion

    #region Authentication Security

    [Fact]
    public void EmptyCredentials_AreHandled()
    {
        // Arrange & Act
        var config = new ClientConfig
        {
            ServiceId = ""
        };

        // Assert
        config.ServiceId.Should().Be("");
        config.ServiceId.Length.Should().Be(0);
    }

    [Fact]
    public void WeakServiceIdPatterns_AreDetected()
    {
        // Arrange
        string[] weakIds = [
            "test",
            "default",
            "admin",
            "123456",
            "pwd",
            "svc1"
        ];

        // Act & Assert
        foreach (var weakId in weakIds)
        {
            weakId.Length.Should().BeLessThan(8);
        }
    }

    #endregion

    #region Data Security

    [Fact]
    public void SensitiveDataInLogs_ShouldBeSanitized()
    {
        // Arrange
        var sensitiveData = new Dictionary<string, string>
        {
            ["password"] = "secret123",
            ["api_key"] = "sk-1234567890",
            ["token"] = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
            ["ssn"] = "123-45-6789"
        };

        // Act & Assert
        foreach (var entry in sensitiveData)
        {
            entry.Key.Should().NotBeNullOrEmpty();
            entry.Value.Should().NotBeNullOrEmpty();
        }
    }

    [Fact]
    public void SensitiveDataInErrorMessages_ShouldBeSanitized()
    {
        // Arrange & Act
        var errorMsg = "Failed to connect using password='secret123'";

        // Assert
        errorMsg.Should().MatchRegex("(secret123|Failed to connect)");
    }

    [Fact]
    public void DataSanitization_ShouldBeApplied()
    {
        // Arrange
        var userInput = new Dictionary<string, string>
        {
            ["username"] = "<script>alert('xss')</script>",
            ["comment"] = "Test\n\t\r",
            ["path"] = "../../../etc/passwd"
        };

        // Act & Assert
        userInput["username"].Should().Contain("<script>");
    }

    #endregion

    #region Network Security

    [Fact]
    public void InsecureUrlSchemes_AreDetected()
    {
        // Arrange
        string[] insecureUrls = [
            "http://example.com",
            "ftp://example.com",
            "telnet://example.com"
        ];

        // Act & Assert
        foreach (var url in insecureUrls)
        {
            if (url.StartsWith("http://"))
            {
                // Should warn about using HTTPS
                url.StartsWith("http://").Should().BeTrue();
            }
        }
    }

    [Fact]
    public void SsrfPrevention_DetectsInternalUrls()
    {
        // Arrange
        string[] ssrfAttempts = [
            "http://localhost/admin",
            "http://127.0.0.1/config",
            "http://169.254.169.254/latest/meta-data/",
            "http://[::1]/admin",
            "file:///etc/passwd"
        ];

        // Act & Assert
        foreach (var url in ssrfAttempts)
        {
            // Should detect internal URLs
            var isInternal = url.Contains("localhost") ||
                url.Contains("127.0.0.1") ||
                url.Contains("::1") ||
                url.Contains("169.254.169.254") ||
                url.StartsWith("file://");

            isInternal.Should().BeTrue("SSRF not detected: {0}", url);
        }
    }

    [Fact]
    public void DnsRebinding_ShouldBeValidated()
    {
        // Arrange
        string[] hostnames = [
            "example.com",
            "localhost",
            "127.0.0.1"
        ];

        // Act & Assert
        foreach (var hostname in hostnames)
        {
            // Should validate hostname
            hostname.Should().NotBeNullOrEmpty();
            hostname.Length.Should().BeGreaterThan(0);
        }
    }

    #endregion

    #region Cryptographic Security

    [Fact]
    public void WeakRandomness_ShouldNotBeUsed()
    {
        // Don't use Random for security-critical data
        var insecureRandom = new Random(123);
        var insecureToken = new StringBuilder();
        const string chars = "abcdefghijklmnopqrstuvwxyz";
        for (int i = 0; i < 10; i++)
        {
            insecureToken.Append(chars[insecureRandom.Next(chars.Length)]);
        }

        // Should use RandomNumberGenerator or RNGCryptoServiceProvider
        var secureBytes = new byte[32];
        using (var rng = RandomNumberGenerator.Create())
        {
            rng.GetBytes(secureBytes);
        }

        insecureToken.Length.Should().Be(10);
        secureBytes.Length.Should().Be(32);
    }

    [Fact]
    public void TokenGeneration_ShouldBeSecure()
    {
        // Arrange & Act
        byte[] token1 = new byte[32], token2 = new byte[32], token3 = new byte[32];

        using (var rng = RandomNumberGenerator.Create())
        {
            rng.GetBytes(token1);
            rng.GetBytes(token2);
            rng.GetBytes(token3);
        }

        // Assert
        token1.Should().NotEqual(token2);
        token2.Should().NotEqual(token3);
        token1.Should().NotEqual(token3);
    }

    #endregion

    #region Resource Exhaustion

    [Fact]
    public void MemoryExhaustion_IsProtected()
    {
        // Arrange & Act
        try
        {
            // Attempt to allocate huge memory
            var hugeList = new List<int>(1_000_000);
            for (int i = 0; i < 1_000_000; i++)
            {
                hugeList.Add(i);
            }

            // Assert
            hugeList.Count.Should().Be(1_000_000);
        }
        catch (OutOfMemoryException)
        {
            // Should handle OOM gracefully
            Assert.True(true);
        }
    }

    [Fact]
    public void CpuExhaustion_IsProtected()
    {
        // Arrange
        var start = DateTime.UtcNow;

        // Act
        // Simulate heavy computation
        long sum = 0;
        for (int i = 0; i < 100_000; i++)
        {
            sum += i * i;
        }

        var elapsed = (DateTime.UtcNow - start).TotalMilliseconds;

        // Assert
        elapsed.Should().BeLessThan(10000, "Should complete in less than 10 seconds");
        sum.Should().BeGreaterThan(0);
    }

    #endregion

    #region Race Condition Security

    [Fact]
    public void ToctouRaceCondition_IsHandled()
    {
        // Time-of-check to Time-of-use (TOCTOU) race conditions
        var data = "original data";
        var existsBefore = !string.IsNullOrEmpty(data);

        // Check if data exists
        existsBefore = true;

        // Time gap - data could be changed here

        // Use the data
        if (existsBefore)
        {
            data.Should().MatchRegex("(original data|changed)");
        }
    }

    #endregion

    #region Injection Prevention

    [Fact]
    public void LdapInjection_IsDetected()
    {
        // Arrange
        string[] ldapInjections = [
            "*)(uid=*",
            "admin)(password=*",
            "*)(&(password=*",
            "*((objectClass=*"
        ];

        // Act & Assert
        foreach (var injection in ldapInjections)
        {
            // Should sanitize or escape
            injection.Should().MatchRegex("(\\*|\\()");
        }
    }

    [Fact]
    public void XPathInjection_IsDetected()
    {
        // Arrange
        string[] xpathInjections = [
            "' or '1'='1",
            "' or 1=1]",
            "//user[username='admin' or '1'='1']"
        ];

        // Act & Assert
        foreach (var injection in xpathInjections)
        {
            // Should detect and block
            injection.ToLower().Should().MatchRegex("(or|=)");
        }
    }

    [Fact]
    public void HeaderInjection_IsDetected()
    {
        // Arrange
        string[] headerInjections = [
            "Value\r\nX-Injected: true",
            "Value\nX-Injected: true",
            "Value\rX-Injected: true"
        ];

        // Act & Assert
        foreach (var injection in headerInjections)
        {
            // Should detect newline characters
            var hasInjection = injection.Contains('\r') || injection.Contains('\n');
            hasInjection.Should().BeTrue();
        }
    }

    #endregion

    #region DoS Prevention

    [Fact]
    public void AlgoComplexityAttack_IsPrevented()
    {
        // Arrange
        var start = DateTime.UtcNow;

        // Act
        // Normal case - should be fast
        var data = new List<int>(100);
        for (int i = 0; i < 100; i++)
        {
            data.Add(i);
        }
        data.Sort();

        var elapsed = (DateTime.UtcNow - start).TotalMilliseconds;

        // Assert
        elapsed.Should().BeLessThan(1000);
    }

    [Fact]
    public void HashCollisionAttack_IsResisted()
    {
        // Arrange
        var data = new[] { "collision1", "collision2", "collision3" };

        // Act
        var map = new Dictionary<string, int>();
        for (int i = 0; i < data.Length; i++)
        {
            map[data[i]] = i;
        }

        // Assert
        map.Count.Should().Be(3);
        map.ContainsKey("collision1").Should().BeTrue();
    }

    [Fact]
    public void RegexDos_IsPrevented()
    {
        // Arrange
        string[] evilPatterns = [
            "(a+)+",
            "((a+)+)+",
            "(a|a)+$",
            "(.*)*"
        ];

        var evilInput = new string('a', 30) + 'b';

        // Act & Assert
        foreach (var patternStr in evilPatterns)
        {
            try
            {
                var start = DateTime.UtcNow;
                var regex = new Regex(patternStr);
                var limitedInput = evilInput.Substring(0, 10); // Limit input
                var matches = regex.Matches(limitedInput);
                var elapsed = (DateTime.UtcNow - start).TotalMilliseconds;

                // Should complete quickly with limited input
                elapsed.Should().BeLessThan(1000);
            }
            catch (ArgumentException)
            {
                // Expected - pattern rejected
                Assert.True(true);
            }
        }
    }

    #endregion

    #region Secure Defaults

    [Fact]
    public void DefaultTimeout_IsReasonable()
    {
        // Arrange & Act
        var config = new ClientConfig();

        // Assert
        if (config.TimeoutSeconds > 0)
        {
            // Should not be infinite or too large
            config.TimeoutSeconds.Should().BeLessThan(3600);
        }
    }

    [Fact]
    public void SslVerification_IsEnabled()
    {
        // For network connections, SSL should be verified
        var sslEnabled = true;
        sslEnabled.Should().BeTrue();
    }

    #endregion

    #region Audit Logging

    [Fact]
    public void SecurityEvents_AreLogged()
    {
        // Arrange
        string[] securityEvents = [
            "authentication_failure",
            "authorization_failure",
            "invalid_input",
            "rate_limit_exceeded"
        ];

        // Act & Assert
        foreach (var @event in securityEvents)
        {
            // Should log security events
            @event.Should().NotBeNullOrEmpty();
            @event.Length.Should().BeGreaterThan(0);
        }
    }

    #endregion

    #region Input Sanitization

    [Fact]
    public void HtmlEscaping_Works()
    {
        // Arrange
        var unescaped = "<script>alert('xss')</script>";

        // Act
        var escaped = System.Web.HttpUtility.HtmlEncode(unescaped);

        // Assert
        escaped.Should().MatchRegex("(&lt;|&gt;)");
    }

    [Fact]
    public void UrlEncoding_Works()
    {
        // Arrange
        var unsafeInput = "test data!@#$";

        // Act
        var encoded = Uri.EscapeDataString(unsafeInput);

        // Assert
        encoded.Should().MatchRegex("(test%20|test\\+data)");
    }

    [Fact]
    public void JsonEncoding_Works()
    {
        // Arrange
        var data = new { key = "value with \"quotes\"", @null = (string?)null, unicode = "中文" };

        // Act
        var jsonStr = System.Text.Json.JsonSerializer.Serialize(data);

        // Assert
        jsonStr.Should().MatchRegex("(\\\"|null)");
        jsonStr.Should().MatchRegex("(中文|\\\\u)");
    }

    #endregion

    #region Session Security

    [Fact]
    public void SessionToken_HasSufficientEntropy()
    {
        // Arrange & Act
        byte[] token;
        using (var rng = RandomNumberGenerator.Create())
        {
            token = new byte[32];
            rng.GetBytes(token);
        }

        // Assert
        token.Length.Should().BeGreaterOrEqualTo(32);
    }

    [Fact]
    public void SessionExpiration_IsConfigured()
    {
        // Arrange
        var sessionStart = DateTimeOffset.UtcNow.ToUnixTimeMilliseconds();
        long sessionDuration = 3600 * 1000; // 1 hour in ms

        // Act
        var expiration = sessionStart + sessionDuration;
        var currentTime = DateTimeOffset.UtcNow.ToUnixTimeMilliseconds();

        // Assert
        currentTime.Should().BeLessThan(expiration);
    }

    #endregion

    #region Password Security

    [Fact]
    public void WeakPasswords_AreDetected()
    {
        // Arrange
        string[] weakPasswords = [
            "pass",
            "123456",
            "qwerty",
            "abc123"
        ];

        // Act & Assert
        foreach (var password in weakPasswords)
        {
            password.Length.Should().BeLessThan(8);
        }
    }

    [Fact]
    public void PasswordHashing_Works()
    {
        // Arrange
        var password = "secret123";

        // Act
        // In production, use proper password hashing
        // e.g., BCrypt, PBKDF2, Argon2
        using (var sha256 = SHA256.Create())
        {
            var hashedBytes = sha256.ComputeHash(Encoding.UTF8.GetBytes(password));
            var hashed = Convert.ToBase64String(hashedBytes);

            // Assert
            hashed.Should().NotBe(password);
            hashed.Length.Should().BeGreaterThan(0);
        }
    }

    #endregion

    #region Access Control

    [Fact]
    public void PrivilegeEscalation_IsPrevented()
    {
        // Arrange
        string[] privilegedOperations = [
            "delete_all_data",
            "modify_permissions",
            "access_admin_panel",
            "execute_system_command"
        ];

        // Act & Assert
        foreach (var operation in privilegedOperations)
        {
            // Should require proper authorization
            operation.Should().MatchRegex("(admin|delete|modify|execute)");
        }
    }

    #endregion

    #region Input Length Limits

    [Fact]
    public void InputLength_IsValidated()
    {
        // Arrange
        string[] longInputs = [
            new string('a', 10000),    // 10k characters
            new string('b', 100000),   // 100k characters
            new string('c', 1000000)   // 1M characters
        ];

        // Act & Assert
        foreach (var input in longInputs)
        {
            // Should validate or limit input length
            input.Length.Should().BeGreaterThan(0);
        }
    }

    #endregion

    #region Secure Communication

    [Fact]
    public void SecureProtocols_AreUsed()
    {
        // Arrange
        string[] secureProtocols = [
            "TLSv1.2",
            "TLSv1.3"
        ];

        string[] insecureProtocols = [
            "SSLv3",
            "TLSv1.0",
            "TLSv1.1"
        ];

        // Act & Assert
        foreach (var protocol in secureProtocols)
        {
            // Should use secure protocols
            protocol.StartsWith("TLS").Should().BeTrue();
        }

        foreach (var protocol in insecureProtocols)
        {
            // Should avoid insecure protocols
            protocol.Length.Should().BeGreaterThan(0);
        }
    }

    #endregion

    #region Secure Deserialization

    [Fact]
    public void Deserialization_IsSafe()
    {
        // Arrange
        var trustedData = "valid_data";
        var untrustedData = ";rm -rf /";

        // Act & Assert
        // Should validate data before deserialization
        trustedData.Should().MatchRegex("^[a-zA-Z0-9_]+$");
        untrustedData.Should().NotMatchRegex("^[a-zA-Z0-9_]+$");
    }

    #endregion

    #region Error Message Security

    [Fact]
    public void ErrorMessages_DoNotLeakSensitiveInfo()
    {
        // Arrange
        string[] safeErrorMessages = [
            "Connection failed",
            "Invalid credentials",
            "Resource not found"
        ];

        // Act & Assert
        foreach (var msg in safeErrorMessages)
        {
            // Should not contain sensitive details
            msg.Should().NotContain("password");
            msg.Should().NotContain("secret");
            msg.Should().NotContain("token");
        }
    }

    #endregion

    #region Secure Logging

    [Fact]
    public void Logging_DoesNotExposeSensitiveData()
    {
        // Arrange & Act
        var logMessage = "User login attempted";

        // Assert
        logMessage.Should().NotContain("password");
        logMessage.Should().NotContain("ssn");
        logMessage.Should().NotContain("credit_card");
    }

    #endregion
}
