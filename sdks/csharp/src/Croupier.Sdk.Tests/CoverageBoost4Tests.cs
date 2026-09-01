// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

using System;
using Croupier.Sdk;
using Croupier.Sdk.Extensions;
using Croupier.Sdk.Configuration;
using Croupier.Sdk.Models;
using Croupier.Sdk.Threading;
using FluentAssertions;
using Microsoft.Extensions.DependencyInjection;
using Xunit;

namespace Croupier.Sdk.Tests;

/// <summary>
/// 中小缺口补测：FunctionHandlerBase 同步入口 / RetryConfig 抖动与上限 /
/// MainThreadDispatcher 泛型入队 / DI 扩展空参守卫 / InvokeOptions 回退链。
/// </summary>
public class CoverageBoost4Tests
{
    private class EchoHandler : FunctionHandlerBase
    {
        public override Task<string> HandleAsync(FunctionContext context, string payload)
        {
            return Task.FromResult("handled:" + payload);
        }
    }

    [Fact]
    public void FunctionHandlerBase_SyncHandle_InvokesAsync()
    {
        var handler = new EchoHandler();
        var ctx = new FunctionContext
        {
            FunctionId = "fn.echo",
            CallId = Guid.NewGuid().ToString(),
            GameId = "g",
            Env = "dev",
        };
        handler.Handle(ctx, "x").Should().Be("handled:x");
    }

    [Fact]
    public void RetryConfig_JitterAndCaps_Applied()
    {
        var cfg = new RetryConfig
        {
            InitialDelayMs = 10,
            BackoffMultiplier = 2,
            MaxDelayMs = 15, // 触发上限截断
            JitterFactor = 0.0,
        };
        // attempt 5：10*2^5=320 → 截到 15
        cfg.DelayMs(5).Should().Be(15);

        var jitter = new RetryConfig
        {
            InitialDelayMs = 100,
            BackoffMultiplier = 1,
            MaxDelayMs = 0, // 不设上限
            JitterFactor = 0.5,
        };
        var d = jitter.DelayMs(1);
        d.Should().BeInRange(50, 150);
    }

    [Fact]
    public void MainThreadDispatcher_EnqueueGeneric_NullIgnored()
    {
        var dispatcher = MainThreadDispatcher.Instance;
        dispatcher.Enqueue<int>(null!, 5); // null action 必须被忽略
    }

    [Fact]
    public void MainThreadDispatcher_EnqueueGeneric_ProcessesData()
    {
        var dispatcher = MainThreadDispatcher.Instance;
        var seen = 0;
        dispatcher.Enqueue<int>(v => seen = v, 42);
        for (var i = 0; i < 128 && dispatcher.ProcessQueue(1) > 0; i++)
        {
        }
        seen.Should().Be(42);
    }

    [Fact]
    public void ServiceCollectionExtensions_NullGuards_Throw()
    {
        var act1 = () => CroupierServiceCollectionExtensionsForTest.AddCroupierNullCheck(null!);
        act1.Should().Throw<ArgumentNullException>();

        var act2 = () => CroupierServiceCollectionExtensionsForTest.AddCroupierWithProviderNullConfigCheck(
            new ServiceCollection(), null!);
        act2.Should().Throw<ArgumentNullException>();
    }

    [Fact]
    public void AddCroupier_RegistersClient()
    {
        var services = new ServiceCollection();
        services.AddCroupier(cfg =>
        {
            cfg.AgentAddr = "127.0.0.1:19091";
            cfg.GameId = "g1";
            cfg.Env = "dev";
        });
        using var provider = services.BuildServiceProvider();
        provider.GetRequiredService<CroupierClient>().Should().NotBeNull();
    }

    private static class CroupierServiceCollectionExtensionsForTest
    {
        // 直接调用公开扩展方法并断言空守卫（不绕私有访问器）。
        public static void AddCroupierNullCheck(IServiceCollection services)
        {
            ServiceCollectionExtensionsTestShim.AddCroupier(services, (Action<ClientConfig>?)null);
        }

        public static void AddCroupierWithProviderNullConfigCheck(
            IServiceCollection services, ICroupierConfigProvider provider)
        {
            ServiceCollectionExtensionsTestShim.AddCroupier(services, provider);
        }
    }
}

internal static class ServiceCollectionExtensionsTestShim
{
    public static void AddCroupier(IServiceCollection services, Action<ClientConfig>? configAction)
        => services.AddCroupier(configAction);

    public static void AddCroupier(IServiceCollection services, ICroupierConfigProvider provider)
        => services.AddCroupier(provider);
}

internal static class MainThreadDispatcherTestExtensions
{
    public static void ProcessQueueAllForTest(this MainThreadDispatcher dispatcher)
    {
        // 处理全部已入队回调（队列容量内）。
        for (var i = 0; i < 128; i++)
        {
            if (dispatcher.ProcessQueue(1) == 0)
            {
                break;
            }
        }
    }
}
