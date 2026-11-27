// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

using System.Net;
using System.Text;
using Croupier.Sdk.Models;
using FluentAssertions;
using Xunit;

namespace Croupier.Sdk.Tests;

/// <summary>
/// AUD-068: a write may have executed before its response was lost. Non-GET requests are
/// single-shot by default and a final unknown-class failure is marked RESULT_UNKNOWN;
/// GET requests keep the previous retry behavior. Callers explicitly opt writes into
/// retries via RetryConfig.RetryWrites.
/// </summary>
public sealed class WriteRetryTests
{
    private sealed class CountingHandler(Func<int, HttpResponseMessage> respond) : HttpMessageHandler
    {
        private int _count;
        public int Calls => _count;

        protected override Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken)
        {
            _count++;
            return Task.FromResult(respond(_count));
        }
    }

    private sealed class ThrowHandler : HttpMessageHandler
    {
        private int _count;
        public int Calls => _count;

        protected override Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken)
        {
            _count++;
            throw new HttpRequestException("connection reset after side effect");
        }
    }

    private static HttpResponseMessage Json(string body, HttpStatusCode status = HttpStatusCode.OK) => new(status)
    {
        Content = new StringContent(body, Encoding.UTF8, "application/json"),
    };

    private static CroupierInvoker Invoker(HttpMessageHandler handler, InvokerConfig? config = null)
    {
        var client = new HttpClient(handler);
        return new CroupierInvoker(config ?? new InvokerConfig { ServerBaseUrl = "http://server.test/api/v1" },
            client, ownsHttpClient: true);
    }

    private static RetryConfig FastRetry(bool retryWrites = false) => new()
    {
        Enabled = true, MaxAttempts = 3, InitialDelayMs = 1, JitterFactor = 0, RetryWrites = retryWrites,
    };

    [Fact]
    public async Task WriteIsNotRetriedByDefaultAndMarkedResultUnknown()
    {
        var handler = new ThrowHandler();
        using var invoker = Invoker(handler, new InvokerConfig
        {
            ServerBaseUrl = "http://server.test/api/v1", Retry = FastRetry(),
        });

        var result = await invoker.InvokeAsync("fn.write", "{}");

        result.Success.Should().BeFalse();
        result.ErrorCode.Should().Be("RESULT_UNKNOWN");
        handler.Calls.Should().Be(1);
    }

    [Fact]
    public async Task Retryable503WriteIsSingleShotAndMarkedUnknown()
    {
        var handler = new CountingHandler(_ => Json("{\"message\":\"unavailable\"}", HttpStatusCode.ServiceUnavailable));
        using var invoker = Invoker(handler, new InvokerConfig
        {
            ServerBaseUrl = "http://server.test/api/v1", Retry = FastRetry(),
        });

        var result = await invoker.InvokeAsync("fn.write", "{}");

        result.ErrorCode.Should().Be("RESULT_UNKNOWN");
        handler.Calls.Should().Be(1);
    }

    [Fact]
    public async Task RetryWritesOptsInvokeBackIntoRetries()
    {
        var handler = new CountingHandler(calls => calls < 2
            ? Json("{\"message\":\"down\"}", HttpStatusCode.ServiceUnavailable)
            : Json("{\"result\":{\"ok\":true}}"));
        using var invoker = Invoker(handler, new InvokerConfig
        {
            ServerBaseUrl = "http://server.test/api/v1", Retry = FastRetry(retryWrites: true),
        });

        var result = await invoker.InvokeAsync("fn.write", "{}");

        result.Success.Should().BeTrue();
        result.Data.Should().Be("{\"ok\":true}");
        handler.Calls.Should().Be(2);
    }

    [Fact]
    public async Task RetryWritesOptsStartTaskBackIntoRetries()
    {
        var handler = new CountingHandler(calls => calls < 2
            ? Json("{\"message\":\"down\"}", HttpStatusCode.ServiceUnavailable)
            : Json("{\"taskId\":\"t-1\"}"));
        using var invoker = Invoker(handler, new InvokerConfig
        {
            ServerBaseUrl = "http://server.test/api/v1", Retry = FastRetry(retryWrites: true),
        });

        (await invoker.StartTaskAsync("fn.write", "{}")).Should().Be("t-1");
        handler.Calls.Should().Be(2);
    }

    [Fact]
    public async Task ExhaustedWriteRetryMarksResultUnknown()
    {
        var handler = new CountingHandler(_ => Json("{\"message\":\"down\"}", HttpStatusCode.ServiceUnavailable));
        using var invoker = Invoker(handler, new InvokerConfig
        {
            ServerBaseUrl = "http://server.test/api/v1", Retry = FastRetry(retryWrites: true),
        });

        var result = await invoker.InvokeAsync("fn.write", "{}");

        result.ErrorCode.Should().Be("RESULT_UNKNOWN");
        handler.Calls.Should().Be(3);
    }

    [Fact]
    public async Task Explicit4xxRejectionIsNeitherRetriedNorMarkedUnknown()
    {
        var handler = new CountingHandler(_ => Json("{\"message\":\"forbidden\"}", HttpStatusCode.Forbidden));
        using var invoker = Invoker(handler, new InvokerConfig
        {
            ServerBaseUrl = "http://server.test/api/v1", Retry = FastRetry(retryWrites: true),
        });

        var result = await invoker.InvokeAsync("fn.write", "{}");

        result.ErrorCode.Should().BeNull();
        result.Error.Should().Contain("forbidden");
        handler.Calls.Should().Be(1);
    }

    [Fact]
    public async Task ReadsAreNotMarkedUnknownByTheWriteGate()
    {
        // C# task reads bypass SendWithRetryAsync entirely: the write gate and the
        // result-unknown marking must never be applied to a failed read.
        var handler = new CountingHandler(_ => Json("{\"message\":\"down\"}", HttpStatusCode.ServiceUnavailable));
        using var invoker = Invoker(handler, new InvokerConfig
        {
            ServerBaseUrl = "http://server.test/api/v1", Retry = FastRetry(),
        });

        Func<Task> read = () => invoker.GetTaskStatusAsync("t-1");
        (await read.Should().ThrowAsync<HttpRequestException>())
            .Which.Message.Should().NotContain("result unknown");
        handler.Calls.Should().Be(1);
    }
}
