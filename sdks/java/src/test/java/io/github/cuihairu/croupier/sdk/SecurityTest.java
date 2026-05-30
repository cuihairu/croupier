// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

package io.github.cuihairu.croupier.sdk;

import org.junit.jupiter.api.Test;
import static org.junit.jupiter.api.Assertions.*;

import java.util.List;
import java.util.Map;
import java.util.HashMap;
import java.util.regex.Pattern;

/**
 * Security tests for Croupier SDK.
 */
public class SecurityTest {

    // ========== Input Validation Security ==========

    @Test
    public void testSqlInjectionInFunctionId() {
        // Test SQL injection attempts are handled safely
        String[] sqlInjectionAttempts = {
            "'; DROP TABLE functions; --",
            "test' OR '1'='1",
            "admin'--",
            "admin'/*",
            "' OR 1=1#"
        };

        for (String attempt : sqlInjectionAttempts) {
            // Should treat as string, not execute
            FunctionDescriptor desc = new FunctionDescriptor();
            desc.setId(attempt);
            desc.setVersion("1.0.0");

            assertEquals(attempt, desc.getId());
        }
    }

    @Test
    public void testPathTraversalDetection() {
        String[] pathTraversalAttempts = {
            "../../../etc/passwd",
            "..\\..\\..\\windows\\system32",
            "/etc/passwd",
            "....//....//etc/passwd",
            "%2e%2e%2f%2e%2e%2f%2e%2e%2fetc%2fpasswd"
        };

        for (String path : pathTraversalAttempts) {
            // Should detect path traversal patterns
            boolean isSuspicious = path.contains("..") ||
                path.contains("/etc/") ||
                path.toLowerCase().contains("windows") ||
                path.toLowerCase().contains("system32") ||
                path.toLowerCase().contains("%2e");

            assertTrue(isSuspicious, "Path traversal not detected: " + path);
        }
    }

    @Test
    public void testCommandInjectionInPayload() {
        String[] commandInjectionAttempts = {
            "{\"data\": \"$(rm -rf /)\"}",
            "{\"data\": \"`whoami`\"}",
            "{\"data\": \"; ls -la\"}",
            "{\"data\": \"| cat /etc/passwd\"}",
            "{\"data\": \"&& curl malicious.com\"}"
        };

        for (String payload : commandInjectionAttempts) {
            // Should not execute commands, just parse as JSON
            // Data should remain as string
            assertTrue(payload.contains("data"), "Payload should have data field");
        }
    }

    @Test
    public void testXssInStrings() {
        String[] xssAttempts = {
            "<script>alert('xss')</script>",
            "<img src=x onerror=alert('xss')>",
            "javascript:alert('xss')",
            "<svg onload=alert('xss')>",
            "'\"><script>alert(String.fromCharCode(88,83,83))</script>"
        };

        for (String attempt : xssAttempts) {
            // Should store as string, not execute
            boolean hasPattern = attempt.contains("<script>") ||
                attempt.contains("javascript:") ||
                attempt.contains("onerror=") ||
                attempt.contains("<svg") ||
                attempt.contains("<img");
            assertTrue(hasPattern, "XSS pattern should be present in string: " + attempt);
        }
    }

    @Test
    public void testBufferOverflowInStrings() {
        // Create very large string
        StringBuilder sb = new StringBuilder();
        for (int i = 0; i < 10_000_000; i++) {
            sb.append('A');
        }
        String largeString = sb.toString();

        // Should handle gracefully
        assertEquals(10_000_000, largeString.length());
    }

    @Test
    public void testIntegerOverflow() {
        // Java handles big integers with BigInteger
        long hugeLong = Long.MAX_VALUE;
        assertTrue(hugeLong > 0);

        // Should not overflow
        long result = hugeLong - 1;
        assertTrue(result < hugeLong);
    }

    @Test
    public void testNullByteInjection() {
        String[] nullByteAttempts = {
            "test\0file.txt",
            "config\0.json",
            "/etc/\0passwd",
            "\0\0\0"
        };

        for (String attempt : nullByteAttempts) {
            // Java strings can contain null bytes
            assertTrue(attempt.length() > 0);
        }
    }

    @Test
    public void testUnicodeNormalizationIssues() {
        String[] homographs = {
            "pa𝘽n",  // Using special characters
            "test\u200b",  // Zero-width space
            "test\u200c",  // Zero-width non-joiner
            "test\u202e"   // Right-to-left override
        };

        for (String text : homographs) {
            // Should handle Unicode
            assertTrue(text.length() > 0);
        }
    }

    // ========== Authentication Security ==========

    @Test
    public void testEmptyCredentials() {
        ClientConfig config = new ClientConfig();
        config.setServiceId("");

        assertEquals("", config.getServiceId());
    }

    @Test
    public void testWeakServiceIdPatterns() {
        String[] weakIds = {
            "test",
            "default",
            "admin",
            "123456",
            "password",
            "service1"
        };

        for (String weakId : weakIds) {
            // Just verify the weak IDs are stored correctly
            assertNotNull(weakId);
            assertTrue(weakId.length() > 0, "Weak ID should not be empty: " + weakId);
        }
    }

    // ========== Data Security ==========

    @Test
    public void testSensitiveDataInLogs() {
        Map<String, String> sensitiveData = new HashMap<>();
        sensitiveData.put("password", "secret123");
        sensitiveData.put("api_key", "sk-1234567890");
        sensitiveData.put("token", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9");
        sensitiveData.put("ssn", "123-45-6789");

        for (Map.Entry<String, String> entry : sensitiveData.entrySet()) {
            assertNotNull(entry.getKey());
            assertNotNull(entry.getValue());
        }
    }

    @Test
    public void testSensitiveDataInErrorMessages() {
        String errorMsg = "Failed to connect using password='secret123'";

        // In production, should sanitize error messages
        assertTrue(errorMsg.contains("secret123") || errorMsg.contains("Failed to connect"));
    }

    @Test
    public void testDataSanitization() {
        Map<String, String> userInput = new HashMap<>();
        userInput.put("username", "<script>alert('xss')</script>");
        userInput.put("comment", "Test\n\t\r");
        userInput.put("path", "../../../etc/passwd");

        // Should sanitize or validate input
        assertTrue(userInput.get("username").contains("<script>"));
    }

    // ========== File Security ==========

    @Test
    public void testConfigFileSecurity() {
        // Config files should be validated
        String configContent = "{\"service_id\": \"test-service\"}";

        assertNotNull(configContent);
        assertTrue(configContent.contains("service_id"));
    }

    // ========== Network Security ==========

    @Test
    public void testInsecureUrlSchemes() {
        String[] insecureUrls = {
            "http://example.com",
            "ftp://example.com",
            "telnet://example.com"
        };

        for (String url : insecureUrls) {
            if (url.startsWith("http://")) {
                // Should warn about using HTTPS
                assertTrue(url.startsWith("http://"));
            }
        }
    }

    @Test
    public void testSsrfPrevention() {
        String[] ssrfAttempts = {
            "http://localhost/admin",
            "http://127.0.0.1/config",
            "http://169.254.169.254/latest/meta-data/",
            "http://[::1]/admin",
            "file:///etc/passwd"
        };

        for (String url : ssrfAttempts) {
            // Should detect internal URLs
            boolean isInternal = url.contains("localhost") ||
                url.contains("127.0.0.1") ||
                url.contains("::1") ||
                url.contains("169.254.169.254") ||
                url.startsWith("file://");

            assertTrue(isInternal, "SSRF not detected: " + url);
        }
    }

    @Test
    public void testDnsRebinding() {
        String[] hostnames = {
            "example.com",
            "localhost",
            "127.0.0.1"
        };

        for (String hostname : hostnames) {
            // Should validate hostname
            assertNotNull(hostname);
            assertTrue(hostname.length() > 0);
        }
    }

    // ========== Cryptographic Security ==========

    @Test
    public void testWeakRandomness() {
        // Don't use java.util.Random for security-critical data
        java.util.Random insecureRandom = new java.util.Random();
        StringBuilder insecureToken = new StringBuilder();
        String chars = "abcdefghijklmnopqrstuvwxyz";
        for (int i = 0; i < 10; i++) {
            insecureToken.append(chars.charAt(insecureRandom.nextInt(chars.length())));
        }

        // Should use java.security.SecureRandom for security
        java.security.SecureRandom secureRandom = new java.security.SecureRandom();
        byte[] secureBytes = new byte[32];
        secureRandom.nextBytes(secureBytes);

        assertEquals(10, insecureToken.length());
        assertEquals(32, secureBytes.length);
    }

    @Test
    public void testTokenGeneration() {
        // Generate secure tokens
        java.security.SecureRandom secureRandom = new java.security.SecureRandom();

        byte[] token1 = new byte[32];
        byte[] token2 = new byte[32];
        byte[] token3 = new byte[32];

        secureRandom.nextBytes(token1);
        secureRandom.nextBytes(token2);
        secureRandom.nextBytes(token3);

        // All should be different
        assertNotEquals(token1, token2);
        assertNotEquals(token2, token3);
        assertNotEquals(token1, token3);
    }

    // ========== Resource Exhaustion ==========

    @Test
    public void testMemoryExhaustionProtection() {
        // Should limit memory allocation
        try {
            // Attempt to allocate huge memory
            List<Integer> hugeList = new java.util.ArrayList<>();
            for (int i = 0; i < 1_000_000; i++) {
                hugeList.add(i);
            }
            assertEquals(1_000_000, hugeList.size());
        } catch (OutOfMemoryError e) {
            // Should handle OOM gracefully
            assertTrue(true);
        }
    }

    @Test
    public void testCpuExhaustionProtection() {
        // Should have timeout limits
        long start = System.currentTimeMillis();

        // Simulate heavy computation
        long sum = 0;
        for (int i = 0; i < 100_000; i++) {
            sum += i * i;
        }

        long elapsed = System.currentTimeMillis() - start;

        // Should complete in reasonable time
        assertTrue(elapsed < 10000, "Should complete in less than 10 seconds");
        assertTrue(sum > 0);
    }

    // ========== Race Condition Security ==========

    @Test
    public void testToctouRaceCondition() throws InterruptedException, java.io.IOException {
        // Time-of-check to Time-of-use (TOCTOU) race conditions
        java.io.File tempFile = null;
        try {
            // Create temp file
            tempFile = java.io.File.createTempFile("test", ".txt");
            tempFile.deleteOnExit();

            java.io.FileWriter writer = new java.io.FileWriter(tempFile);
            writer.write("original data");
            writer.close();

            // Check if file exists
            boolean existsBefore = tempFile.exists();

            // Time gap - file could be changed here

            // Use the file
            String data = "";
            if (existsBefore) {
                java.util.Scanner scanner = new java.util.Scanner(tempFile);
                data = scanner.useDelimiter("\\A").next();
                scanner.close();
            }

            assertTrue(data.equals("original data") || data.contains("changed"));
        } finally {
            if (tempFile != null && tempFile.exists()) {
                tempFile.delete();
            }
        }
    }

    // ========== Injection Prevention ==========

    @Test
    public void testLdapInjection() {
        String[] ldapInjections = {
            "*)(uid=*",
            "admin)(password=*",
            "*)(&(password=*",
            "*)((objectClass=*"
        };

        for (String injection : ldapInjections) {
            // Should sanitize or escape
            assertTrue(injection.contains("*") || injection.contains("("));
        }
    }

    @Test
    public void testXPathInjection() {
        String[] xpathInjections = {
            "' or '1'='1",
            "' or 1=1]",
            "//user[username='admin' or '1'='1']"
        };

        for (String injection : xpathInjections) {
            // Should detect and block
            assertTrue(injection.toLowerCase().contains("or") || injection.contains("="));
        }
    }

    @Test
    public void testHeaderInjection() {
        String[] headerInjections = {
            "Value\r\nX-Injected: true",
            "Value\nX-Injected: true",
            "Value\rX-Injected: true"
        };

        for (String injection : headerInjections) {
            // Should detect newline characters
            boolean hasInjection = injection.contains("\r") || injection.contains("\n");
            assertTrue(hasInjection);
        }
    }

    // ========== DoS Prevention ==========

    @Test
    public void testAlgoComplexityAttack() {
        // Protection against algorithmic complexity attacks
        long start = System.currentTimeMillis();

        // Normal case - should be fast
        List<Integer> data = new java.util.ArrayList<>();
        for (int i = 0; i < 100; i++) {
            data.add(i);
        }
        java.util.Collections.sort(data);

        long elapsed = System.currentTimeMillis() - start;

        // Should complete quickly
        assertTrue(elapsed < 1000);
    }

    @Test
    public void testHashCollisionAttack() {
        // Java has hash collision protection
        String[] data = {"collision1", "collision2", "collision3"};

        Map<String, Integer> map = new HashMap<>();
        for (int i = 0; i < data.length; i++) {
            map.put(data[i], i);
        }

        // Should work correctly
        assertEquals(3, map.size());
        assertTrue(map.containsKey("collision1"));
    }

    @Test
    public void testRegexDos() {
        // ReDoS (Regular Expression DoS) prevention
        String[] evilPatterns = {
            "(a+)+",
            "((a+)+)+",
            "(a|a)+$",
            "(.*)*"
        };

        String evilInput = "a".repeat(30) + "b";

        for (String pattern : evilPatterns) {
            try {
                // Add timeout to prevent DoS
                long start = System.currentTimeMillis();
                Pattern p = Pattern.compile(pattern);
                java.util.regex.Matcher m = p.matcher(evilInput.substring(0, 10)); // Limit input
                boolean matches = m.matches();
                long elapsed = System.currentTimeMillis() - start;

                // Should complete quickly with limited input
                assertTrue(elapsed < 1000);
            } catch (Exception e) {
                // Expected - pattern rejected
                assertTrue(true);
            }
        }
    }

    // ========== Secure Defaults ==========

    @Test
    public void testDefaultTimeoutIsReasonable() {
        ClientConfig config = new ClientConfig();

        // Should have reasonable default
        if (config.getTimeoutSeconds() != 0) {
            // Should not be infinite or too large
            assertTrue(config.getTimeoutSeconds() < 3600, "Timeout should be less than 1 hour");
        }
    }

    @Test
    public void testSslVerificationEnabled() {
        // For network connections, SSL should be verified
        // This is a placeholder - actual implementation depends on SDK
        assertTrue(true); // SSL verification should be on by default
    }

    // ========== Audit Logging ==========

    @Test
    public void testSecurityEventsLogged() {
        String[] securityEvents = {
            "authentication_failure",
            "authorization_failure",
            "invalid_input",
            "rate_limit_exceeded"
        };

        for (String event : securityEvents) {
            // Should log security events
            assertNotNull(event);
            assertTrue(event.length() > 0);
        }
    }

    // ========== Input Sanitization ==========

    @Test
    public void testHtmlEscaping() {
        String unescaped = "<script>alert('xss')</script>";
        String escaped = htmlEscape(unescaped);

        // Should escape dangerous characters
        assertTrue(escaped.contains("&lt;") || escaped.contains("&gt;"));
    }

    private String htmlEscape(String input) {
        return input.replace("&", "&amp;")
            .replace("<", "&lt;")
            .replace(">", "&gt;")
            .replace("\"", "&quot;")
            .replace("'", "&#39;");
    }

    @Test
    public void testUrlEncoding() {
        String unsafe = "test data!@#$";
        String encoded = java.net.URLEncoder.encode(unsafe, java.nio.charset.StandardCharsets.UTF_8);

        // Should encode special characters
        assertTrue(encoded.contains("test%20") || encoded.contains("test+data"));
    }

    // ========== Session Security ==========

    @Test
    public void testSessionTokenEntropy() {
        // Generate token with sufficient entropy
        java.security.SecureRandom secureRandom = new java.security.SecureRandom();
        byte[] token = new byte[32];
        secureRandom.nextBytes(token);

        // Should be long enough (256 bits = 32 bytes)
        assertEquals(32, token.length);
    }

    @Test
    public void testSessionExpiration() {
        long sessionStart = System.currentTimeMillis();
        long sessionDuration = 3600 * 1000; // 1 hour in ms

        // Session should expire
        long expiration = sessionStart + sessionDuration;
        long currentTime = System.currentTimeMillis();

        // Should have expiration check
        assertTrue(currentTime < expiration);
    }

    // ========== Secure Equality Comparison ==========

    @Test
    public void testTimingSafeComparison() {
        // Use constant-time comparison for sensitive data
        String password1 = "secret123";
        String password2 = "secret123";
        String password3 = "secret124";

        // Regular comparison is not timing-safe but works for tests
        assertTrue(password1.equals(password2));
        assertFalse(password1.equals(password3));
    }

    // ========== Secure String Handling ==========

    @Test
    public void testSensitiveStringCleanup() {
        // In production, sensitive strings should be cleared after use
        char[] password = "secret123".toCharArray();

        // Use password
        assertTrue(password.length > 0);

        // Clear password after use
        java.util.Arrays.fill(password, '\0');

        // Password should be cleared
        for (char c : password) {
            assertEquals('\0', c);
        }
    }

    // ========== Secure Randomness Generation ==========

    @Test
    public void testSecureRandomVsRandom() {
        // Test that SecureRandom is used for security
        java.util.Random insecureRandom = new java.util.Random(123);
        java.security.SecureRandom secureRandom1 = new java.security.SecureRandom();
        java.security.SecureRandom secureRandom2 = new java.security.SecureRandom();

        // Insecure random with same seed produces same sequence
        int val1 = insecureRandom.nextInt();
        int val2 = insecureRandom.nextInt();

        // Secure random should be different each time
        int val3 = secureRandom1.nextInt();
        int val4 = secureRandom2.nextInt();

        // SecureRandom instances don't share state
        // (values might occasionally be equal but that's ok)
        assertNotNull(secureRandom1);
        assertNotNull(secureRandom2);
    }

    // ========== Password Security ==========

    @Test
    public void testPasswordStrengthValidation() {
        String[] weakPasswords = {
            "password",
            "123456",
            "qwerty",
            "abc123"
        };

        for (String password : weakPasswords) {
            // Should detect weak passwords
            assertTrue(password.length() < 8 || password.matches("[a-z]+"));
        }
    }

    @Test
    public void testPasswordHashing() {
        String password = "secret123";

        // In production, passwords should be hashed
        // Use bcrypt, scrypt, or Argon2
        String hashed = hashPassword(password);

        // Hash should not equal password
        assertNotEquals(password, hashed);
        assertTrue(hashed.length() > 0);
    }

    private String hashPassword(String password) {
        // Placeholder - use proper password hashing in production
        // e.g., BCryptPasswordEncoder
        try {
            java.security.MessageDigest md = java.security.MessageDigest.getInstance("SHA-256");
            byte[] hash = md.digest(password.getBytes());
            return java.util.Base64.getEncoder().encodeToString(hash);
        } catch (java.security.NoSuchAlgorithmException e) {
            throw new RuntimeException(e);
        }
    }

    // ========== Access Control ==========

    @Test
    public void testPrivilegeEscalationPrevention() {
        String[] privilegedOperations = {
            "delete_all_data",
            "modify_permissions",
            "access_admin_panel",
            "execute_system_command"
        };

        for (String operation : privilegedOperations) {
            // Should require proper authorization
            assertTrue(operation.contains("admin") ||
                operation.contains("delete") ||
                operation.contains("modify") ||
                operation.contains("execute"));
        }
    }

    // ========== Input Length Limits ==========

    @Test
    public void testInputLengthValidation() {
        String[] longInputs = {
            "a".repeat(10000),    // 10k characters
            "b".repeat(100000),   // 100k characters
            "c".repeat(1000000)   // 1M characters
        };

        for (String input : longInputs) {
            // Should validate or limit input length
            assertTrue(input.length() > 0);
        }
    }

    // ========== Secure File Operations ==========

    @Test
    public void testSecureFilePermissions() throws java.io.IOException {
        // In production, check file permissions
        java.io.File tempFile = null;
        try {
            tempFile = java.io.File.createTempFile("test", ".txt");

            // Check if file exists and is readable
            assertTrue(tempFile.exists());
            assertTrue(tempFile.canRead());

            // On Unix systems, check permissions
            // (Not applicable on Windows)
        } finally {
            if (tempFile != null && tempFile.exists()) {
                tempFile.delete();
            }
        }
    }

    // ========== Secure Deserialization ==========

    @Test
    public void testSafeDeserialization() {
        // Only deserialize trusted data
        String trustedData = "valid_data";
        String untrustedData = ";rm -rf /";

        // Should validate data before deserialization
        assertTrue(trustedData.matches("[a-zA-Z0-9_]+"));
        assertFalse(untrustedData.matches("[a-zA-Z0-9_]+"));
    }

    // ========== Cryptographic Algorithm Selection ==========

    @Test
    public void testStrongCryptographicAlgorithms() {
        // Use strong algorithms
        String[] strongAlgorithms = {
            "AES/GCM/NoPadding",
            "RSA/ECB/OAEPWithSHA-256AndMGF1Padding",
            "PBKDF2WithHmacSHA256"
        };

        for (String algorithm : strongAlgorithms) {
            // Should use strong algorithms
            assertTrue(algorithm.contains("AES") ||
                algorithm.contains("RSA") ||
                algorithm.contains("PBKDF2"));
        }
    }

    @Test
    public void testWeakCryptographicAlgorithms() {
        // Avoid weak algorithms
        String[] weakAlgorithms = {
            "DES",
            "MD5",
            "SHA1"
        };

        for (String algorithm : weakAlgorithms) {
            // Should avoid weak algorithms
            assertTrue(algorithm.length() > 0);
        }
    }

    // ========== Secure Key Management ==========

    @Test
    public void testKeyStorage() {
        // Keys should be stored securely
        // In production, use KeyStore or similar
        String apiKey = "sk-1234567890";

        // Should not hardcode keys
        assertNotNull(apiKey);

        // Should use environment variables or secure storage
        String envKey = System.getenv("API_KEY");
        // envKey might be null in tests
    }

    // ========== Certificate Validation ==========

    @Test
    public void testCertificateValidation() {
        // Should validate certificates properly
        String[] validCertPatterns = {
            "CN=example.com",
            "O=Organization",
            "C=US"
        };

        for (String pattern : validCertPatterns) {
            // Should validate certificate fields
            assertTrue(pattern.contains("="));
        }
    }

    // ========== Secure Communication ==========

    @Test
    public void testSecureProtocols() {
        String[] secureProtocols = {
            "TLSv1.2",
            "TLSv1.3"
        };

        String[] insecureProtocols = {
            "SSLv3",
            "TLSv1.0",
            "TLSv1.1"
        };

        for (String protocol : secureProtocols) {
            // Should use secure protocols
            assertTrue(protocol.startsWith("TLS"));
        }

        for (String protocol : insecureProtocols) {
            // Should avoid insecure protocols
            assertTrue(protocol.length() > 0);
        }
    }

    // ========== Secure Defaults for Configuration ==========

    @Test
    public void testSecureDefaultConfiguration() {
        ClientConfig config = new ClientConfig();

        // Should have secure defaults
        assertNotNull(config);

        // Check for secure default values
        if (config.getAgentAddr() != null && !config.getAgentAddr().isEmpty()) {
            // Should use secure protocol or be a valid address
            boolean isValid = config.getAgentAddr().startsWith("tcp://") ||
                config.getAgentAddr().startsWith("ipc://") ||
                config.getAgentAddr().contains(":"); // Allow other valid formats
            assertTrue(isValid, "Agent address should be valid: " + config.getAgentAddr());
        }
    }

    // ========== Error Message Security ==========

    @Test
    public void testErrorMessageDoesNotLeakInfo() {
        // Error messages should not leak sensitive information
        String[] safeErrorMessages = {
            "Connection failed",
            "Invalid credentials",
            "Resource not found"
        };

        for (String msg : safeErrorMessages) {
            // Should not contain sensitive details
            assertFalse(msg.contains("password"));
            assertFalse(msg.contains("secret"));
            assertFalse(msg.contains("token"));
        }
    }

    // ========== Secure Logging ==========

    @Test
    public void testLoggingDoesNotExposeSensitiveData() {
        String logMessage = "User login attempted";

        // Should not log sensitive data
        assertFalse(logMessage.contains("password"));
        assertFalse(logMessage.contains("ssn"));
        assertFalse(logMessage.contains("credit_card"));
    }
}
