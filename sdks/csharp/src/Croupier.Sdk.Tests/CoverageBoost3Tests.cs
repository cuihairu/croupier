// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

using System.Net;
using System.Text;
using Croupier.Sdk.Models;
using Croupier.Sdk.Threading;
using FluentAssertions;
using Xunit;

namespace Croupier.Sdk.Tests;

/// <summary>
/// Third coverage boost: error message extraction, batch semantics, stream
/// cursor behaviour, main-thread dispatcher, InvokeResult model and config
/// provider edge paths.
/// </summary>
public sealed class CoverageBoost3Tests
{
    // -----------------------------------------------------------------------
    // InvokeResult model
    // -----------------------------------------------------------------------

    [Fact]
    public void InvokeResult_FactoryMethods_PopulateFields()
    {
        var success = InvokeResult.Succeeded("{\"ok\":true}", 42);
        success.Success.Should().BeTrue();
        success.Data.Should().Be("{\"ok\":true}");
        success.DurationMs.Should().Be(42);
        success.Error.Should().BeNull();

        var failure = InvokeResult.Failed("denied", "FORBIDDEN", 7);
        failure.Success.Should().BeFalse();
        failure.Error.Should().Be("denied");
        failure.ErrorCode.Should().Be("FORBIDDEN");
        failure.DurationMs.Should().Be(7);
    }

    // -----------------------------------------------------------------------
    // Invoker error extraction & batch edges
    // -----------------------------------------------------------------------

    private sealed class SequenceHandler : HttpMessageHandler
    {
        private readonly Func<int, HttpResponseMessage> _respond;
        private int _count;

        public SequenceHandler(Func<int, HttpResponseMessage> respond) => _respond = respond;

        public int Calls => _count;

        protected override Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken)
        {
            _count++;
            return Task.FromResult(_respond(_count));
        }
    }

    private static HttpResponseMessage Json(string body, HttpStatusCode status = HttpStatusCode.OK) => new(status)
    {
        Content = new StringContent(body, Encoding.UTF8, "application/json"),
    };

    private static CroupierInvoker Invoker(HttpMessageHandler handler, InvokerConfig? config = null)
    {
        var client = new HttpClient(handler) { BaseAddress = new Uri("http://unused.invalid/") };
        return new CroupierInvoker(config ?? new InvokerConfig
        {
            ServerBaseUrl = "http://server.test/api/v1",
        }, client, ownsHttpClient: true);
    }

    [Theory]
    [InlineData("{\"error\":\"forbidden\",\"message\":\"未授权\"}", "未授权")]
    [InlineData("{\"error\":\"server_error\"}", "server_error")] // falls back to error code
    [InlineData("plain text body", "plain text body")]
    [InlineData("", "500")]
    public async Task InvokeAsync_ErrorMessageExtractionVariants(string body, string expectedFragment)
    {
        var handler = new SequenceHandler(_ => Json(body, HttpStatusCode.InternalServerError));
        using var invoker = Invoker(handler);

        var result = await invoker.InvokeAsync("fn", "{}");

        result.Success.Should().BeFalse();
        result.Error.Should().Contain(expectedFragment);
    }

    [Fact]
    public async Task InvokeAsync_NonJsonSuccessBody_FailsGracefully()
    {
        var handler = new SequenceHandler(_ => new HttpResponseMessage(HttpStatusCode.OK)
        {
            Content = new StringContent("not-json", Encoding.UTF8, "text/plain"),
        });
        using var invoker = Invoker(handler);

        var result = await invoker.InvokeAsync("fn", "{}");
        result.Success.Should().BeFalse();
        result.Error.Should().NotBeNullOrEmpty();
    }

    [Fact]
    public async Task InvokeAsync_CanceledOperation_ReportsCancellation()
    {
        var handler = new CancelAwareHandler();
        using var invoker = Invoker(handler, new InvokerConfig
        {
            ServerBaseUrl = "http://server.test/api/v1",
            TimeoutSeconds = 1,
            Retry = new RetryConfig { Enabled = false },
        });

        var result = await invoker.InvokeAsync("fn", "{}");
        result.Success.Should().BeFalse();
        result.Error.Should().NotBeNullOrEmpty();
    }

    private sealed class CancelAwareHandler : HttpMessageHandler
    {
        protected override async Task<HttpResponseMessage> SendAsync(
            HttpRequestMessage request, CancellationToken cancellationToken)
        {
            await Task.Delay(TimeSpan.FromSeconds(30), cancellationToken);
            return Json("{\"result\":{}}");
        }
    }

    [Fact]
    public async Task BatchInvokeAsync_NullRequests_Throws()
    {
        using var invoker = Invoker(new SequenceHandler(_ => Json("{}")));

        await Assert.ThrowsAsync<ArgumentNullException>(
            () => invoker.BatchInvokeAsync(null!));
    }

    [Fact]
    public async Task BatchInvokeAsync_LargeBatchPreservesOrder()
    {
        var handler = new SequenceHandler(n => Json($"{{\"result\":{{\"n\":{n}}}}}"));
        using var invoker = Invoker(handler);

        var requests = Enumerable.Range(1, 8)
            .Select(n => new BatchInvokeRequest
            {
                FunctionId = "fn",
                Payload = "{}",
                IdempotencyKey = $"key-{n}",
            })
            .ToList();

        var results = await invoker.BatchInvokeAsync(requests);

        results.Should().HaveCount(8);
        results.Should().OnlyContain(r => r.Success);
        handler.Calls.Should().Be(8);
    }

    [Fact]
    public async Task StartTaskAsync_ServerErrorMessageSurfaces()
    {
        var handler = new SequenceHandler(_ => Json(
            "{\"error\":\"quota_exceeded\",\"message\":\"too many tasks\"}", HttpStatusCode.TooManyRequests));
        using var invoker = Invoker(handler, new InvokerConfig
        {
            ServerBaseUrl = "http://server.test/api/v1",
            Retry = new RetryConfig { Enabled = false },
        });

        var action = () => invoker.StartTaskAsync("fn", "{}");
        await action.Should().ThrowAsync<HttpRequestException>()
            .WithMessage("*too many tasks*");
    }

    // -----------------------------------------------------------------------
    // Stream cursor behaviour
    // -----------------------------------------------------------------------

    [Fact]
    public async Task StreamTaskAsync_MultipleBatchesAdvanceTheCursor()
    {
        int calls = 0;
        var handler = new SequenceHandler(n =>
        {
            calls = n;
            return n switch
            {
                1 => Json("{\"items\":[{\"seq\":2,\"type\":\"log\"},{\"seq\":5,\"type\":\"log\"}],\"done\":false}"),
                2 => Json("{\"items\":[{\"seq\":9,\"type\":\"completed\"}],\"done\":true}"),
                _ => Json("{\"items\":[],\"done\":true}"),
            };
        });
        using var invoker = Invoker(handler, new InvokerConfig
        {
            ServerBaseUrl = "http://server.test/api/v1",
            TaskPollIntervalMilliseconds = 1,
        });

        var types = new List<string>();
        await foreach (var evt in invoker.StreamTaskAsync("t1"))
        {
            types.Add(evt.Type);
        }

        types.Should().Equal("log", "log", "completed");
        calls.Should().Be(2);
    }

    [Fact]
    public async Task StreamTaskAsync_NonArrayItemsIsTolerated()
    {
        var handler = new SequenceHandler(_ => Json(
            "{\"items\":\"not-an-array\",\"done\":true}"));
        using var invoker = Invoker(handler);

        var types = new List<string>();
        await foreach (var evt in invoker.StreamTaskAsync("t1"))
        {
            types.Add(evt.Type);
        }
        types.Should().BeEmpty(); // malformed items are skipped, done ends the stream
    }

    // -----------------------------------------------------------------------
    // MainThreadDispatcher
    // -----------------------------------------------------------------------

    [Fact]
    public void MainThreadDispatcher_ExecutesImmediatelyOnMainThread()
    {
        var dispatcher = MainThreadDispatcher.Instance;
        MainThreadDispatcher.Initialize();

        try
        {
            var executed = new List<int>();
            dispatcher.Enqueue(() => executed.Add(1));
            executed.Should().Equal(1);
            dispatcher.IsMainThread.Should().BeTrue();
        }
        finally
        {
            dispatcher.Clear();
        }
    }

    [Fact]
    public void MainThreadDispatcher_QueuedCallbacksProcessInOrder()
    {
        var dispatcher = MainThreadDispatcher.Instance;
        var order = new List<int>();
        dispatcher.Enqueue(() => order.Add(1));
        dispatcher.Enqueue(() => order.Add(2));
        dispatcher.Enqueue(() => order.Add(3));

        var processed = dispatcher.ProcessQueue();
        processed.Should().BeGreaterThanOrEqualTo(3);
        order.Should().Equal(1, 2, 3);
    }

    [Fact]
    public void MainThreadDispatcher_FaultedCallbacksDoNotBlockTheQueue()
    {
        var dispatcher = MainThreadDispatcher.Instance;
        var executed = false;
        dispatcher.Enqueue(() => throw new InvalidOperationException("boom"));
        dispatcher.Enqueue(() => executed = true);

        dispatcher.ProcessQueue();

        executed.Should().BeTrue("a faulted callback must not stop processing");
    }

    [Fact]
    public void MainThreadDispatcher_WorkThreadEnqueuesForMainThread()
    {
        var dispatcher = MainThreadDispatcher.Instance;
        MainThreadDispatcher.Initialize();
        var processed = false;
        try
        {
            var worker = new Thread(() => dispatcher.Enqueue(() => processed = true));
            worker.Start();
            worker.Join();

            processed.Should().BeFalse("the callback must wait on the queue");
            dispatcher.ProcessQueue();
            processed.Should().BeTrue();
            dispatcher.IsMainThread.Should().BeTrue();
        }
        finally
        {
            dispatcher.Clear();
        }
    }

    // -----------------------------------------------------------------------
    // Config provider environment parsing
    // -----------------------------------------------------------------------

    [Fact]
    public void EnvironmentConfigProvider_ReturnsDefaultsWithoutEnvVars()
    {
        var provider = new Configuration.EnvironmentConfigProvider("CROUPIER_TEST_ABSENT_");
        var config = provider.GetConfig();

        config.AgentAddr.Should().Be("127.0.0.1:19091");
        config.TimeoutSeconds.Should().BeGreaterThan(0);
    }

    [Fact]
    public void EnvironmentConfigProvider_ParsesOverriddenValues()
    {
        Environment.SetEnvironmentVariable("CROUPIER_BOOST3_AGENT_ADDR", "10.9.8.7:19091");
        Environment.SetEnvironmentVariable("CROUPIER_BOOST3_SERVICE_ID", "env-svc");
        try
        {
            var provider = new Configuration.EnvironmentConfigProvider("CROUPIER_BOOST3_");
            var config = provider.GetConfig();
            config.AgentAddr.Should().Be("10.9.8.7:19091");
            config.ServiceId.Should().Be("env-svc");
        }
        finally
        {
            Environment.SetEnvironmentVariable("CROUPIER_BOOST3_AGENT_ADDR", null);
            Environment.SetEnvironmentVariable("CROUPIER_BOOST3_SERVICE_ID", null);
        }
    }

    [Fact]
    public void MemoryConfigProvider_ReturnsInjectedConfig()
    {
        var expected = new ClientConfig { ServiceId = "memory-svc", GameId = "g" };
        var provider = new Configuration.MemoryConfigProvider(expected);

        provider.GetConfig().ServiceId.Should().Be("memory-svc");
    }

    [Fact]
    public void InvokerConfig_TimeoutsAreClamped()
    {
        using var invoker = new CroupierInvoker(new InvokerConfig
        {
            ServerBaseUrl = "http://server.test/api/v1/",
            TimeoutSeconds = 0,
            TaskPollIntervalMilliseconds = -5,
        });

        invoker.ServerBaseUrl.Should().Be("http://server.test/api/v1/");
    }
}
