// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

using System.Net;
using System.Text;
using Croupier.Sdk.Models;
using FluentAssertions;
using Xunit;

namespace Croupier.Sdk.Tests;

/// <summary>
/// Behavioural depth tests for the Server HTTP invoker: URL escaping, header
/// precedence, response parsing edge cases and batch semantics.
/// </summary>
public sealed class CroupierInvokerBoostTests
{
    [Fact]
    public async Task InvokeAsync_EscapesSpecialCharactersInFunctionId()
    {
        var handler = new RecordingHandler(_ => JsonResponse("{\"result\":{}}"));
        using var invoker = CreateInvoker(handler);

        await invoker.InvokeAsync("fn with/slash+plus", "{}");

        handler.Requests.Single().PathAndQuery.Should().Be(
            "/api/v1/functions/fn%20with%2Fslash%2Bplus/invoke");
    }

    [Fact]
    public async Task InvokeAsync_RequestOptionsOverrideGlobalScopeHeaders()
    {
        var handler = new RecordingHandler(_ => JsonResponse("{\"result\":{}}"));
        using var invoker = CreateInvoker(handler);

        await invoker.InvokeAsync("fn", "{}", new InvokeOptions
        {
            GameId = "override-game",
            Env = "override-env",
        });

        var headers = handler.Requests.Single().Headers;
        headers["X-Game-ID"].Should().Be("override-game");
        headers["X-Env"].Should().Be("override-env");
        headers["Authorization"].Should().Be("Bearer token-1");
    }

    [Fact]
    public async Task InvokeAsync_RecordsDurationAndSucceedsWithEmptyObjectResult()
    {
        var handler = new RecordingHandler(_ => JsonResponse("{\"result\":{}}"));
        using var invoker = CreateInvoker(handler);

        var result = await invoker.InvokeAsync("fn", "{}");

        result.Success.Should().BeTrue();
        result.DurationMs.Should().BeGreaterThanOrEqualTo(0);
    }

    [Fact]
    public async Task InvokeAsync_ReturnsFailureForNonJsonResponse()
    {
        var handler = new RecordingHandler(_ => new HttpResponseMessage(HttpStatusCode.OK)
        {
            Content = new StringContent("not-json", Encoding.UTF8, "text/plain"),
        });
        using var invoker = CreateInvoker(handler);

        var result = await invoker.InvokeAsync("fn", "{}");

        result.Success.Should().BeFalse();
        result.Error.Should().NotBeNullOrEmpty();
    }

    [Fact]
    public async Task InvokeAsync_ReturnsFailureWhenHandlerThrows()
    {
        var handler = new ThrowingHandler(new HttpRequestException("connection refused"));
        using var invoker = CreateInvoker(handler);

        var result = await invoker.InvokeAsync("fn", "{}");

        result.Success.Should().BeFalse();
        result.Error.Should().Contain("connection refused");
    }

    [Fact]
    public async Task InvokeAsync_FallsBackToWholeBodyWhenResultKeyMissing()
    {
        var handler = new RecordingHandler(_ => JsonResponse("{\"other\":1}"));
        using var invoker = CreateInvoker(handler);

        var result = await invoker.InvokeAsync("fn", "{}");

        result.Success.Should().BeTrue();
        result.Data.Should().Be("{\"other\":1}");
    }

    [Fact]
    public async Task InvokeAsync_MapsServerErrorPayloadToMessage()
    {
        var handler = new RecordingHandler(_ => JsonResponse(
            "{\"error\":\"server_error\",\"message\":\"boom\"}", HttpStatusCode.InternalServerError));
        using var invoker = CreateInvoker(handler);

        var result = await invoker.InvokeAsync("fn", "{}");

        result.Success.Should().BeFalse();
        result.Error.Should().Contain("boom");
    }

    [Fact]
    public async Task BatchInvokeAsync_RejectsEmptyRequestList()
    {
        using var invoker = CreateInvoker(new RecordingHandler(_ => JsonResponse("{}")));

        await Assert.ThrowsAsync<ArgumentException>(() => invoker.BatchInvokeAsync(new List<BatchInvokeRequest>()));
    }

    [Fact]
    public async Task BatchInvokeAsync_MixesSuccessAndFailureResults()
    {
        var handler = new RecordingHandler(request =>
            request.PathAndQuery.Contains("bad.fn")
                ? JsonResponse("{\"message\":\"nope\"}", HttpStatusCode.NotFound)
                : JsonResponse("{\"result\":{\"ok\":true}}"));
        using var invoker = CreateInvoker(handler);

        var results = await invoker.BatchInvokeAsync(new List<BatchInvokeRequest>
        {
            new() { FunctionId = "good.fn", Payload = "{}", IdempotencyKey = "k1" },
            new() { FunctionId = "bad.fn", Payload = "{}", IdempotencyKey = "k2" },
        });

        results.Should().HaveCount(2);
        results[0].Success.Should().BeTrue();
        results[1].Success.Should().BeFalse();
    }

    [Fact]
    public async Task StartTaskAsync_FailsWhenTaskIdMissing()
    {
        var handler = new RecordingHandler(_ => JsonResponse("{\"status\":\"accepted\"}"));
        using var invoker = CreateInvoker(handler);

        await Assert.ThrowsAsync<InvalidOperationException>(() => invoker.StartTaskAsync("fn", "{}"));
    }

    [Fact]
    public async Task StartTaskAsync_SendsFunctionIdAndParams()
    {
        var handler = new RecordingHandler(_ => JsonResponse("{\"taskId\":\"task-42\"}"));
        using var invoker = CreateInvoker(handler);

        var taskId = await invoker.StartTaskAsync("fn", "{\"a\":1}");

        taskId.Should().Be("task-42");
        var request = handler.Requests.Single();
        request.Method.Should().Be(HttpMethod.Post);
        request.PathAndQuery.Should().Be("/api/v1/tasks");
        request.Body.Should().Contain("\"functionId\":\"fn\"");
        request.Body.Should().Contain("\"a\":1");
    }

    [Fact]
    public async Task CancelTaskAsync_EscapesTaskIdAndPostsToCancel()
    {
        var handler = new RecordingHandler(_ => JsonResponse("{}"));
        using var invoker = CreateInvoker(handler);

        var cancelled = await invoker.CancelTaskAsync("task id/1");

        cancelled.Should().BeTrue();
        var request = handler.Requests.Single();
        request.Method.Should().Be(HttpMethod.Post);
        request.PathAndQuery.Should().Be("/api/v1/tasks/task%20id%2F1/cancel");
    }

    [Fact]
    public async Task GetTaskStatusAsync_MapsServerFieldsToModel()
    {
        var handler = new RecordingHandler(_ => JsonResponse(
            "{\"taskId\":\"t9\",\"status\":\"completed\",\"progress\":100,\"message\":\"done\"," +
            "\"error\":null,\"result\":{\"ok\":true}}"));
        using var invoker = CreateInvoker(handler);

        var status = await invoker.GetTaskStatusAsync("t9");

        status.TaskId.Should().Be("t9");
        status.Status.Should().Be("completed");
        status.Progress.Should().Be(100);
        status.Message.Should().Be("done");
    }

    [Fact]
    public async Task StreamTaskAsync_TerminalEventTypeStopsIteration()
    {
        var handler = new RecordingHandler(_ => JsonResponse(
            "{\"items\":[{\"seq\":1,\"type\":\"progress\"},{\"seq\":2,\"type\":\"failed\"}],\"done\":false}"));
        using var invoker = CreateInvoker(handler);

        var types = new List<string>();
        await foreach (var evt in invoker.StreamTaskAsync("t1"))
        {
            types.Add(evt.Type);
        }

        types.Should().Equal("progress", "failed");
        handler.Requests.Should().HaveCount(1);
    }

    [Fact]
    public async Task StreamTaskAsync_PollsAfterEmptyBatchUntilDone()
    {
        int calls = 0;
        var handler = new RecordingHandler(_ =>
        {
            calls++;
            return calls == 1
                ? JsonResponse("{\"items\":[],\"done\":false}")
                : JsonResponse("{\"items\":[{\"seq\":5,\"type\":\"completed\"}],\"done\":true}");
        });
        using var invoker = CreateInvoker(handler);

        var types = new List<string>();
        await foreach (var evt in invoker.StreamTaskAsync("t1"))
        {
            types.Add(evt.Type);
        }

        types.Should().Equal("completed");
        handler.Requests.Should().HaveCount(2);
        handler.Requests[1].PathAndQuery.Should().Contain("after_seq=0");
    }

    [Fact]
    public async Task StreamTaskAsync_AdvancesCursorToHighestSequence()
    {
        int calls = 0;
        var handler = new RecordingHandler(_ =>
        {
            calls++;
            return calls == 1
                ? JsonResponse("{\"items\":[{\"seq\":3,\"type\":\"log\"},{\"seq\":7,\"type\":\"log\"}],\"done\":false}")
                : JsonResponse("{\"items\":[],\"done\":true}");
        });
        using var invoker = CreateInvoker(handler);

        await foreach (var _ in invoker.StreamTaskAsync("t1"))
        {
        }

        handler.Requests[1].PathAndQuery.Should().Contain("after_seq=7");
    }

    private static CroupierInvoker CreateInvoker(HttpMessageHandler handler)
    {
        var client = new HttpClient(handler) { BaseAddress = new Uri("http://unused.invalid/") };
        return new CroupierInvoker(new InvokerConfig
        {
            ServerBaseUrl = "http://server.test/api/v1",
            AuthToken = "token-1",
            GameId = "default-game",
            Env = "dev",
            TaskPollIntervalMilliseconds = 1,
        }, client, ownsHttpClient: true);
    }

    private static HttpResponseMessage JsonResponse(string json, HttpStatusCode statusCode = HttpStatusCode.OK) => new(statusCode)
    {
        Content = new StringContent(json, Encoding.UTF8, "application/json"),
    };

    private sealed record RecordedRequest(HttpMethod Method, string PathAndQuery, Dictionary<string, string> Headers, string Body);

    private sealed class RecordingHandler : HttpMessageHandler
    {
        private readonly Func<RecordedRequest, HttpResponseMessage> _respond;

        public RecordingHandler(Func<RecordedRequest, HttpResponseMessage> respond) => _respond = respond;

        public List<RecordedRequest> Requests { get; } = new();

        protected override async Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken)
        {
            var headers = request.Headers.ToDictionary(pair => pair.Key, pair => string.Join(",", pair.Value), StringComparer.OrdinalIgnoreCase);
            var recorded = new RecordedRequest(
                request.Method,
                request.RequestUri!.PathAndQuery,
                headers,
                request.Content == null ? string.Empty : await request.Content.ReadAsStringAsync(cancellationToken));
            Requests.Add(recorded);
            return _respond(recorded);
        }
    }

    private sealed class ThrowingHandler(Exception exception) : HttpMessageHandler
    {
        protected override Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken)
            => Task.FromException<HttpResponseMessage>(exception);
    }
}
