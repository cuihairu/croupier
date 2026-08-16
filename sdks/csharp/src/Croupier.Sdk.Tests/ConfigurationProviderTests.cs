// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

using Croupier.Sdk.Configuration;
using Croupier.Sdk.Models;
using FluentAssertions;
using Xunit;

namespace Croupier.Sdk.Tests;

/// <summary>
/// Tests for configuration providers
/// </summary>
public class ConfigurationProviderTests
{
    #region MemoryConfigProvider Tests

    [Fact]
    public void MemoryConfigProvider_ReturnsProvidedConfig()
    {
        // Arrange
        var config = new ClientConfig
        {
            AgentAddr = "custom:1234",
            ServiceId = "test-service"
        };
        var provider = new MemoryConfigProvider(config);

        // Act
        var result = provider.GetConfig();

        // Assert
        result.Should().BeSameAs(config);
        result.AgentAddr.Should().Be("custom:1234");
        result.ServiceId.Should().Be("test-service");
    }

    [Fact]
    public void MemoryConfigProvider_ThrowsOnNullConfig()
    {
        // Act & Assert
        var action = () => new MemoryConfigProvider(null!);
        action.Should().Throw<ArgumentNullException>();
    }

    [Fact]
    public void MemoryConfigProvider_ReturnsSameInstance()
    {
        // Arrange
        var config = new ClientConfig();
        var provider = new MemoryConfigProvider(config);

        // Act
        var result1 = provider.GetConfig();
        var result2 = provider.GetConfig();

        // Assert
        result1.Should().BeSameAs(result2);
    }

    #endregion

    #region EnvironmentConfigProvider Tests

    [Fact]
    public void EnvironmentConfigProvider_DefaultPrefix_IsCroupier()
    {
        // This test verifies the default prefix behavior
        // Arrange
        var provider = new EnvironmentConfigProvider();

        // Act
        var config = provider.GetConfig();

        // Assert - should return defaults when env vars not set
        config.Should().NotBeNull();
        config.ServiceId.Should().Be("csharp-service");
        config.Env.Should().Be("development");
        config.Insecure.Should().BeTrue();
    }

    [Fact]
    public void EnvironmentConfigProvider_CustomPrefix_IsUsed()
    {
        // Arrange
        var customPrefix = "MYAPP_";
        Environment.SetEnvironmentVariable(customPrefix + "SERVICE_ID", "custom-service");

        try
        {
            var provider = new EnvironmentConfigProvider(customPrefix);

            // Act
            var config = provider.GetConfig();

            // Assert
            config.ServiceId.Should().Be("custom-service");
        }
        finally
        {
            Environment.SetEnvironmentVariable(customPrefix + "SERVICE_ID", null);
        }
    }

    [Fact]
    public void EnvironmentConfigProvider_ReadsStringValues()
    {
        // Arrange
        Environment.SetEnvironmentVariable("CROUPIER_AGENT_ADDR", "test-host:9999");

        try
        {
            var provider = new EnvironmentConfigProvider();

            // Act
            var config = provider.GetConfig();

            // Assert
            config.AgentAddr.Should().Be("test-host:9999");
        }
        finally
        {
            Environment.SetEnvironmentVariable("CROUPIER_AGENT_ADDR", null);
        }
    }

    [Theory]
    [InlineData("true", true)]
    [InlineData("TRUE", true)]
    [InlineData("True", true)]
    [InlineData("1", true)]
    [InlineData("false", false)]
    [InlineData("FALSE", false)]
    [InlineData("0", false)]
    [InlineData("invalid", false)]
    public void EnvironmentConfigProvider_ReadsBoolValues(string envValue, bool expected)
    {
        // Arrange
        Environment.SetEnvironmentVariable("CROUPIER_INSECURE", envValue);

        try
        {
            var provider = new EnvironmentConfigProvider();

            // Act
            var config = provider.GetConfig();

            // Assert
            config.Insecure.Should().Be(expected);
        }
        finally
        {
            Environment.SetEnvironmentVariable("CROUPIER_INSECURE", null);
        }
    }

    [Theory]
    [InlineData("60", 60)]
    [InlineData("120", 120)]
    [InlineData("invalid", 30)] // Default value
    [InlineData("", 30)] // Default value
    public void EnvironmentConfigProvider_ReadsIntValues(string envValue, int expected)
    {
        // Arrange
        Environment.SetEnvironmentVariable("CROUPIER_TIMEOUT_SECONDS", envValue);

        try
        {
            var provider = new EnvironmentConfigProvider();

            // Act
            var config = provider.GetConfig();

            // Assert
            config.TimeoutSeconds.Should().Be(expected);
        }
        finally
        {
            Environment.SetEnvironmentVariable("CROUPIER_TIMEOUT_SECONDS", null);
        }
    }

    [Fact]
    public void EnvironmentConfigProvider_AllConfigProperties_HaveDefaults()
    {
        // Arrange
        var provider = new EnvironmentConfigProvider();

        // Act
        var config = provider.GetConfig();

        // Assert - all properties should have sensible defaults
        config.AgentAddr.Should().NotBeNullOrEmpty();
        config.ServiceId.Should().NotBeNullOrEmpty();
        config.ServiceVersion.Should().NotBeNullOrEmpty();
        config.GameId.Should().NotBeNullOrEmpty();
        config.Env.Should().NotBeNullOrEmpty();
        config.TimeoutSeconds.Should().BeGreaterThan(0);
        config.HeartbeatIntervalSeconds.Should().BeGreaterThan(0);
        config.ReconnectIntervalSeconds.Should().BeGreaterThan(0);
        config.MaxConcurrentMessages.Should().BeGreaterThan(0);
        config.MaxMessageSize.Should().BeGreaterThan(0);
        config.ProviderLang.Should().NotBeNullOrEmpty();
        config.ProviderSdk.Should().NotBeNullOrEmpty();
        config.Headers.Should().NotBeNull();
    }

    #endregion

    #region JsonFileConfigProvider Tests

    [Fact]
    public void JsonFileConfigProvider_ThrowsOnNullPath()
    {
        // Act & Assert
        var action = () => new JsonFileConfigProvider(null!);
        action.Should().Throw<ArgumentNullException>();
    }

    [Fact]
    public void JsonFileConfigProvider_ThrowsOnMissingFile()
    {
        // Arrange
        var provider = new JsonFileConfigProvider("/nonexistent/path/config.json");

        // Act & Assert
        var action = () => provider.GetConfig();
        action.Should().Throw<FileNotFoundException>();
    }

    [Fact]
    public void JsonFileConfigProvider_ReadsValidJsonFile()
    {
        // Arrange
        var tempFile = Path.GetTempFileName();
        try
        {
            File.WriteAllText(tempFile, @"{
                ""AgentAddr"": ""192.168.1.1:8080"",
                ""ServiceId"": ""json-service"",
                ""TimeoutSeconds"": 60
            }");

            var provider = new JsonFileConfigProvider(tempFile);

            // Act
            var config = provider.GetConfig();

            // Assert
            config.AgentAddr.Should().Be("192.168.1.1:8080");
            config.ServiceId.Should().Be("json-service");
            config.TimeoutSeconds.Should().Be(60);
        }
        finally
        {
            File.Delete(tempFile);
        }
    }

    [Fact]
    public void JsonFileConfigProvider_HandlesMinimalJson()
    {
        // Arrange
        var tempFile = Path.GetTempFileName();
        try
        {
            File.WriteAllText(tempFile, "{}");
            var provider = new JsonFileConfigProvider(tempFile);

            // Act
            var config = provider.GetConfig();

            // Assert - should use defaults
            config.Should().NotBeNull();
            config.AgentAddr.Should().Be("127.0.0.1:19091");
        }
        finally
        {
            File.Delete(tempFile);
        }
    }

    #endregion

    #region ICroupierConfigProvider Interface Tests

    [Fact]
    public void AllProviders_ImplementInterface()
    {
        // Assert
        typeof(EnvironmentConfigProvider).Should().Implement<ICroupierConfigProvider>();
        typeof(JsonFileConfigProvider).Should().Implement<ICroupierConfigProvider>();
        typeof(MemoryConfigProvider).Should().Implement<ICroupierConfigProvider>();
    }

    [Fact]
    public void ConfigProviders_CanBeUsedPolymorphically()
    {
        // Arrange
        var config = new ClientConfig { ServiceId = "poly-test" };
        ICroupierConfigProvider provider = new MemoryConfigProvider(config);

        // Act
        var result = provider.GetConfig();

        // Assert
        result.ServiceId.Should().Be("poly-test");
    }

    #endregion
}
