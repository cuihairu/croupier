// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

using Croupier.Sdk.Models;
using FluentAssertions;
using Xunit;

namespace Croupier.Sdk.Tests;

/// <summary>
/// Tests for ClientConfig model
/// </summary>
public class ClientConfigTests
{
    [Fact]
    public void ClientConfig_DefaultValues_ShouldBeCorrect()
    {
        // Arrange & Act
        var config = new ClientConfig();

        // Assert
        config.AgentAddr.Should().Be("127.0.0.1:19091");
        config.ServiceId.Should().Be("csharp-service");
        config.ServiceVersion.Should().Be("1.0.0");
        config.GameId.Should().Be("default-game");
        config.Env.Should().Be("development");
        config.Insecure.Should().BeTrue();
        config.TimeoutSeconds.Should().Be(30);
        config.HeartbeatIntervalSeconds.Should().Be(60);
        config.AutoReconnect.Should().BeTrue();
        config.ReconnectIntervalSeconds.Should().Be(5);
        config.ReconnectMaxAttempts.Should().Be(0);
        config.MaxConcurrentMessages.Should().Be(100);
        config.MaxMessageSize.Should().Be(4 * 1024 * 1024);
        config.ProviderLang.Should().Be("csharp");
        config.ProviderSdk.Should().Be("croupier-csharp-sdk");
        config.Headers.Should().BeEmpty();
        config.LogLevel.Should().Be("INFO");
        config.MaxFileSize.Should().Be(10 * 1024 * 1024);
    }

    [Fact]
    public void ClientConfig_CustomValues_ShouldBeSettable()
    {
        // Arrange
        var config = new ClientConfig
        {
            AgentAddr = "192.168.1.100:8080",
            ServiceId = "my-service",
            AgentId = "agent-1",
            ServiceVersion = "2.0.0",
            GameId = "my-game",
            Env = "production",
            ControlAddr = "https://control.example.com",
            Insecure = true,
            CertFile = "/path/to/cert.pem",
            KeyFile = "/path/to/key.pem",
            CaFile = "/path/to/ca.pem",
            ServerName = "myserver.com",
            ProviderLang = "dotnet",
            ProviderSdk = "custom-sdk",
            AuthToken = "secret-token",
            Headers = new Dictionary<string, string> { ["X-Test"] = "yes" },
            TimeoutSeconds = 60,
            HeartbeatIntervalSeconds = 15,
            AutoReconnect = false,
            ReconnectIntervalSeconds = 10,
            ReconnectMaxAttempts = 5,
            MaxConcurrentMessages = 50,
            MaxMessageSize = 8 * 1024 * 1024,
            DisableLogging = true,
            DebugLogging = true,
            LogLevel = "DEBUG",
            EnableFileTransfer = true,
            MaxFileSize = 32 * 1024 * 1024
        };

        // Assert
        config.AgentAddr.Should().Be("192.168.1.100:8080");
        config.ServiceId.Should().Be("my-service");
        config.AgentId.Should().Be("agent-1");
        config.ServiceVersion.Should().Be("2.0.0");
        config.GameId.Should().Be("my-game");
        config.Env.Should().Be("production");
        config.ControlAddr.Should().Be("https://control.example.com");
        config.Insecure.Should().BeTrue();
        config.CertFile.Should().Be("/path/to/cert.pem");
        config.KeyFile.Should().Be("/path/to/key.pem");
        config.CaFile.Should().Be("/path/to/ca.pem");
        config.ServerName.Should().Be("myserver.com");
        config.ProviderLang.Should().Be("dotnet");
        config.ProviderSdk.Should().Be("custom-sdk");
        config.AuthToken.Should().Be("secret-token");
        config.Headers.Should().ContainKey("X-Test").WhoseValue.Should().Be("yes");
        config.TimeoutSeconds.Should().Be(60);
        config.HeartbeatIntervalSeconds.Should().Be(15);
        config.AutoReconnect.Should().BeFalse();
        config.ReconnectIntervalSeconds.Should().Be(10);
        config.ReconnectMaxAttempts.Should().Be(5);
        config.MaxConcurrentMessages.Should().Be(50);
        config.MaxMessageSize.Should().Be(8 * 1024 * 1024);
        config.DisableLogging.Should().BeTrue();
        config.DebugLogging.Should().BeTrue();
        config.LogLevel.Should().Be("DEBUG");
        config.EnableFileTransfer.Should().BeTrue();
        config.MaxFileSize.Should().Be(32 * 1024 * 1024);
    }

    [Theory]
    [InlineData("localhost:19090")]
    [InlineData("192.168.1.1:8080")]
    [InlineData("agent.example.com:443")]
    public void ClientConfig_AgentAddr_AcceptsValidFormats(string addr)
    {
        // Arrange
        var config = new ClientConfig { AgentAddr = addr };

        // Assert
        config.AgentAddr.Should().Be(addr);
    }

    [Fact]
    public void ClientConfig_TlsProperties_AreOptional()
    {
        // Arrange & Act
        var config = new ClientConfig();

        // Assert
        config.CertFile.Should().BeNull();
        config.KeyFile.Should().BeNull();
        config.CaFile.Should().BeNull();
        config.ServerName.Should().BeNull();
    }

    [Fact]
    public void ClientConfig_MaxMessageSize_DefaultIsFourMB()
    {
        // Arrange & Act
        var config = new ClientConfig();

        // Assert
        config.MaxMessageSize.Should().Be(4 * 1024 * 1024);
        config.MaxMessageSize.Should().Be(4194304); // 4MB in bytes
    }

    [Theory]
    [InlineData(0)]
    [InlineData(1)]
    [InlineData(30)]
    [InlineData(60)]
    [InlineData(120)]
    [InlineData(300)]
    public void ClientConfig_TimeoutSeconds_AcceptsValidValues(int timeout)
    {
        // Arrange & Act
        var config = new ClientConfig { TimeoutSeconds = timeout };

        // Assert
        config.TimeoutSeconds.Should().Be(timeout);
    }

    [Theory]
    [InlineData("")]
    [InlineData("dev")]
    [InlineData("development")]
    [InlineData("staging")]
    [InlineData("production")]
    [InlineData("test")]
    [InlineData("qa")]
    public void ClientConfig_Env_AcceptsVariousValues(string env)
    {
        // Arrange & Act
        var config = new ClientConfig { Env = env };

        // Assert
        config.Env.Should().Be(env);
    }

    [Theory]
    [InlineData(1)]
    [InlineData(10)]
    [InlineData(15)]
    [InlineData(30)]
    [InlineData(60)]
    [InlineData(120)]
    public void ClientConfig_HeartbeatIntervalSeconds_AcceptsValidValues(int interval)
    {
        // Arrange & Act
        var config = new ClientConfig { HeartbeatIntervalSeconds = interval };

        // Assert
        config.HeartbeatIntervalSeconds.Should().Be(interval);
    }

    [Theory]
    [InlineData(1)]
    [InlineData(5)]
    [InlineData(10)]
    [InlineData(30)]
    public void ClientConfig_ReconnectIntervalSeconds_AcceptsValidValues(int interval)
    {
        // Arrange & Act
        var config = new ClientConfig { ReconnectIntervalSeconds = interval };

        // Assert
        config.ReconnectIntervalSeconds.Should().Be(interval);
    }

    [Theory]
    [InlineData(0)] // Unlimited retries
    [InlineData(1)]
    [InlineData(3)]
    [InlineData(5)]
    [InlineData(10)]
    public void ClientConfig_ReconnectMaxAttempts_AcceptsValidValues(int attempts)
    {
        // Arrange & Act
        var config = new ClientConfig { ReconnectMaxAttempts = attempts };

        // Assert
        config.ReconnectMaxAttempts.Should().Be(attempts);
    }

    [Theory]
    [InlineData(1)]
    [InlineData(10)]
    [InlineData(50)]
    [InlineData(100)]
    [InlineData(500)]
    [InlineData(1000)]
    public void ClientConfig_MaxConcurrentMessages_AcceptsValidValues(int max)
    {
        // Arrange & Act
        var config = new ClientConfig { MaxConcurrentMessages = max };

        // Assert
        config.MaxConcurrentMessages.Should().Be(max);
    }

    [Theory]
    [InlineData(true)]
    [InlineData(false)]
    public void ClientConfig_AutoReconnect_AcceptsBothValues(bool autoReconnect)
    {
        // Arrange & Act
        var config = new ClientConfig { AutoReconnect = autoReconnect };

        // Assert
        config.AutoReconnect.Should().Be(autoReconnect);
    }

    [Theory]
    [InlineData(true)]
    [InlineData(false)]
    public void ClientConfig_Insecure_AcceptsBothValues(bool insecure)
    {
        // Arrange & Act
        var config = new ClientConfig { Insecure = insecure };

        // Assert
        config.Insecure.Should().Be(insecure);
    }

    [Fact]
    public void ClientConfig_ServiceId_CanBeEmpty()
    {
        // Arrange & Act
        var config = new ClientConfig { ServiceId = "" };

        // Assert
        config.ServiceId.Should().Be("");
    }

    [Fact]
    public void ClientConfig_GameId_CanBeEmpty()
    {
        // Arrange & Act
        var config = new ClientConfig { GameId = "" };

        // Assert
        config.GameId.Should().Be("");
    }

    [Fact]
    public void ClientConfig_AllTlsProperties_CanBeSet()
    {
        // Arrange & Act
        var config = new ClientConfig
        {
            CertFile = "/path/to/cert.pem",
            KeyFile = "/path/to/key.pem",
            CaFile = "/path/to/ca.pem",
            ServerName = "example.com"
        };

        // Assert
        config.CertFile.Should().Be("/path/to/cert.pem");
        config.KeyFile.Should().Be("/path/to/key.pem");
        config.CaFile.Should().Be("/path/to/ca.pem");
        config.ServerName.Should().Be("example.com");
    }

    [Fact]
    public void ClientConfig_ServiceVersion_CanBeSet()
    {
        // Arrange & Act
        var config = new ClientConfig { ServiceVersion = "2.5.0" };

        // Assert
        config.ServiceVersion.Should().Be("2.5.0");
    }

    [Fact]
    public void ClientConfig_CanHaveMultipleInstances()
    {
        // Arrange & Act
        var config1 = new ClientConfig { AgentAddr = "localhost:19090" };
        var config2 = new ClientConfig { AgentAddr = "localhost:8080" };

        // Assert
        config1.AgentAddr.Should().Be("localhost:19090");
        config2.AgentAddr.Should().Be("localhost:8080");
        config1.Should().NotBeSameAs(config2);
    }
}
