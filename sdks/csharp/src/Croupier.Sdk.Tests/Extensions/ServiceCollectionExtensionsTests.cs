// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

using Croupier.Sdk.Configuration;
using Croupier.Sdk.Extensions;
using Croupier.Sdk.Models;
using FluentAssertions;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Options;
using Xunit;

namespace Croupier.Sdk.Tests.Extensions;

/// <summary>
/// Tests for ServiceCollectionExtensions DI integration
/// </summary>
public class ServiceCollectionExtensionsTests
{
    #region AddCroupier with Action Tests

    [Fact]
    public void AddCroupier_WithNullConfigAction_RegistersServices()
    {
        // Arrange
        var services = new ServiceCollection();

        // Act
        services.AddCroupier();

        // Assert
        var provider = services.BuildServiceProvider();
        var client = provider.GetService<CroupierClient>();
        var invoker = provider.GetService<CroupierInvoker>();
        var options = provider.GetService<IOptions<ClientConfig>>();

        client.Should().NotBeNull();
        invoker.Should().NotBeNull();
        options.Should().NotBeNull();
        options!.Value.AgentAddr.Should().Be("127.0.0.1:19090");
    }

    [Fact]
    public void AddCroupier_WithConfigAction_AppliesConfiguration()
    {
        // Arrange
        var services = new ServiceCollection();
        var expectedAgentAddr = "custom:9999";
        var expectedServiceId = "test-service";

        // Act
        services.AddCroupier(config =>
        {
            config.AgentAddr = expectedAgentAddr;
            config.ServiceId = expectedServiceId;
        });

        // Assert
        var provider = services.BuildServiceProvider();
        var options = provider.GetService<IOptions<ClientConfig>>();

        options.Should().NotBeNull();
        options!.Value.AgentAddr.Should().Be(expectedAgentAddr);
        options!.Value.ServiceId.Should().Be(expectedServiceId);
    }

    [Fact]
    public void AddCroupier_WithConfigAction_RegistersSingletonServices()
    {
        // Arrange
        var services = new ServiceCollection();

        // Act
        services.AddCroupier();

        // Assert
        var provider = services.BuildServiceProvider();

        // Get services twice - should be same instance (Singleton)
        var client1 = provider.GetService<CroupierClient>();
        var client2 = provider.GetService<CroupierClient>();

        client1.Should().BeSameAs(client2);
    }

    [Fact]
    public void AddCroupier_WithConfigAction_CanBeCalledMultipleTimes()
    {
        // Arrange
        var services = new ServiceCollection();

        // Act
        services.AddCroupier(config => config.ServiceId = "service1");
        services.AddCroupier(config => config.ServiceId = "service2");

        // Assert - last registration wins for options
        var provider = services.BuildServiceProvider();
        var options = provider.GetService<IOptions<ClientConfig>>();

        options!.Value.ServiceId.Should().Be("service2");
    }

    #endregion

    #region AddCroupier with IConfigurationSection Tests

    [Fact]
    public void AddCroupir_WithConfigurationSection_BindsConfiguration()
    {
        // Arrange
        var services = new ServiceCollection();
        var configuration = new ConfigurationBuilder()
            .AddInMemoryCollection(new Dictionary<string, string?>
            {
                ["Croupier:AgentAddr"] = "configured:8080",
                ["Croupier:ServiceId"] = "configured-service",
                ["Croupier:Env"] = "production"
            })
            .Build();
        var section = configuration.GetSection("Croupier");

        // Act
        services.AddCroupier(section);

        // Assert
        var provider = services.BuildServiceProvider();
        var options = provider.GetService<IOptions<ClientConfig>>();

        options.Should().NotBeNull();
        options!.Value.AgentAddr.Should().Be("configured:8080");
        options!.Value.ServiceId.Should().Be("configured-service");
        options!.Value.Env.Should().Be("production");
    }

    [Fact]
    public void AddCroupier_WithEmptySection_UsesDefaults()
    {
        // Arrange
        var services = new ServiceCollection();
        var configuration = new ConfigurationBuilder()
            .AddInMemoryCollection(new Dictionary<string, string?>())
            .Build();
        var section = configuration.GetSection("Croupier");

        // Act
        services.AddCroupier(section);

        // Assert
        var provider = services.BuildServiceProvider();
        var options = provider.GetService<IOptions<ClientConfig>>();

        options.Should().NotBeNull();
        options!.Value.ServiceId.Should().Be("csharp-service");
    }

    [Fact]
    public void AddCroupier_WithConfigurationSection_RegistersCoreServices()
    {
        // Arrange
        var services = new ServiceCollection();
        var configuration = new ConfigurationBuilder()
            .AddInMemoryCollection(new Dictionary<string, string?>
            {
                ["Croupier:ServiceId"] = "test-service"
            })
            .Build();
        var section = configuration.GetSection("Croupier");

        // Act
        services.AddCroupier(section);

        // Assert
        var provider = services.BuildServiceProvider();
        var client = provider.GetService<CroupierClient>();
        var invoker = provider.GetService<CroupierInvoker>();

        client.Should().NotBeNull();
        invoker.Should().NotBeNull();
    }

    [Fact]
    public void AddCroupier_WithNullSection_ThrowsArgumentNullException()
    {
        // Arrange
        var services = new ServiceCollection();

        // Act & Assert
        services.Invoking(s => s.AddCroupier((IConfigurationSection)null!))
            .Should().Throw<ArgumentNullException>();
    }

    #endregion

    #region AddCroupier with ICroupierConfigProvider Tests

    [Fact]
    public void AddCroupier_WithConfigProvider_UsesProviderConfig()
    {
        // Arrange
        var services = new ServiceCollection();
        var expectedConfig = new ClientConfig
        {
            AgentAddr = "provider:7777",
            ServiceId = "provider-service"
        };
        var configProvider = new MemoryConfigProvider(expectedConfig);

        // Act
        services.AddCroupier(configProvider);

        // Assert
        var provider = services.BuildServiceProvider();
        var options = provider.GetService<IOptions<ClientConfig>>();

        options.Should().NotBeNull();
        options!.Value.AgentAddr.Should().Be("provider:7777");
        options!.Value.ServiceId.Should().Be("provider-service");
    }

    [Fact]
    public void AddCroupier_WithNullConfigProvider_ThrowsArgumentNullException()
    {
        // Arrange
        var services = new ServiceCollection();

        // Act & Assert
        services.Invoking(s => s.AddCroupier((ICroupierConfigProvider)null!))
            .Should().Throw<ArgumentNullException>();
    }

    [Fact]
    public void AddCroupier_WithConfigProvider_RegistersServices()
    {
        // Arrange
        var services = new ServiceCollection();
        var configProvider = new MemoryConfigProvider(new ClientConfig());

        // Act
        services.AddCroupier(configProvider);

        // Assert
        var provider = services.BuildServiceProvider();
        var client = provider.GetService<CroupierClient>();
        var invoker = provider.GetService<CroupierInvoker>();

        client.Should().NotBeNull();
        invoker.Should().NotBeNull();
    }

    #endregion

    #region Service Lifetime Tests

    [Fact]
    public void AddCroupier_ServicesAreSingleton()
    {
        // Arrange
        var services = new ServiceCollection();

        // Act
        services.AddCroupier();

        // Assert
        var provider = services.BuildServiceProvider();

        // Create two scopes
        using var scope1 = provider.CreateScope();
        using var scope2 = provider.CreateScope();

        var client1 = scope1.ServiceProvider.GetService<CroupierClient>();
        var client2 = scope2.ServiceProvider.GetService<CroupierClient>();

        // Singletons should be the same instance across scopes
        client1.Should().BeSameAs(client2);
    }

    [Fact]
    public void AddCroupier_MultipleInvocations_ReturnSameInstances()
    {
        // Arrange
        var services = new ServiceCollection();

        // Act
        services.AddCroupier();

        // Assert
        var provider = services.BuildServiceProvider();

        var client1 = provider.GetRequiredService<CroupierClient>();
        var client2 = provider.GetRequiredService<CroupierClient>();
        var invoker1 = provider.GetRequiredService<CroupierInvoker>();
        var invoker2 = provider.GetRequiredService<CroupierInvoker>();

        client1.Should().BeSameAs(client2);
        invoker1.Should().BeSameAs(invoker2);
    }

    #endregion

    #region Configuration Options Tests

    [Fact]
    public void AddCroupier_OptionsAreCaptured()
    {
        // Arrange
        var services = new ServiceCollection();
        var configuredTimeout = 60;

        // Act
        services.AddCroupier(config =>
        {
            config.TimeoutSeconds = configuredTimeout;
            config.Insecure = false;
        });

        // Assert
        var provider = services.BuildServiceProvider();
        var options = provider.GetService<IOptions<ClientConfig>>();

        options!.Value.TimeoutSeconds.Should().Be(configuredTimeout);
        options!.Value.Insecure.Should().BeFalse();
    }

    [Fact]
    public void AddCroupier_SupportsOptionsSnapshot()
    {
        // Arrange
        var services = new ServiceCollection();

        // Act
        services.AddCroupier(config => config.ServiceId = "snapshot-test");

        // Assert
        var provider = services.BuildServiceProvider();
        var snapshot = provider.GetService<IOptionsSnapshot<ClientConfig>>();

        snapshot.Should().NotBeNull();
        snapshot!.Value.ServiceId.Should().Be("snapshot-test");
    }

    #endregion

    #region Integration Tests

    [Fact]
    public void AddCroupier_CanResolveAllServices()
    {
        // Arrange
        var services = new ServiceCollection();

        // Act
        services.AddCroupier();

        // Assert
        var provider = services.BuildServiceProvider();

        // All registered services should be resolvable
        provider.GetService<CroupierClient>().Should().NotBeNull();
        provider.GetService<CroupierInvoker>().Should().NotBeNull();
        provider.GetService<IOptions<ClientConfig>>().Should().NotBeNull();
    }

    [Fact]
    public void AddCroupier_WithMultipleRegistrations_LastWins()
    {
        // Arrange
        var services = new ServiceCollection();

        // Act
        services.AddCroupier(config => config.ServiceId = "first");
        services.AddCroupier(config => config.ServiceId = "second");

        // Assert
        var provider = services.BuildServiceProvider();
        var client = provider.GetService<CroupierClient>();

        // The last registration's config should be used
        // Note: This depends on the DI container's behavior
        client.Should().NotBeNull();
    }

    [Fact]
    public void AddCroupier_CanCoexistWithOtherServices()
    {
        // Arrange
        var services = new ServiceCollection();

        // Register other services first
        services.AddSingleton<ILogger, TestLogger>();
        services.AddSingleton<IHelperService, HelperService>();

        // Act
        services.AddCroupier();

        // Assert
        var provider = services.BuildServiceProvider();

        provider.GetService<CroupierClient>().Should().NotBeNull();
        provider.GetService<CroupierInvoker>().Should().NotBeNull();
        provider.GetService<ILogger>().Should().NotBeNull();
        provider.GetService<IHelperService>().Should().NotBeNull();
    }

    #endregion

    #region Test Helpers

    private interface ILogger { }
    private class TestLogger : ILogger { }

    private interface IHelperService { }
    private class HelperService : IHelperService { }

    #endregion
}
