// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

using System.Net;
using System.Text;
using Croupier.Sdk.Models;
using FluentAssertions;
using Xunit;

namespace Croupier.Sdk.Tests;

/// <summary>
/// Tests for the independent Server HTTP invoker.
/// </summary>
public sealed class CroupierInvokerTests
{
    [Fact]
    public void Constructor_RejectsNonHttpServerUrl()
    {
        var action = () => new CroupierInvoker(new InvokerConfig { ServerBaseUrl = "127.0.0.1:19090" });

        action.Should().Throw<ArgumentException>();
    }

    [Fact]
    public async Task InvokeAsync_UsesServerFunctionEndpointAndReturnsResult()
    {
        var handler = new RecordingHandler(_ => JsonResponse("{\"result\":{\"ok\":true}}"));
        using var invoker = CreateInvoker(handler);

        var result = await invoker.InvokeAsync("player.ban", "{\"playerId\":\"42\"}", new InvokeOptions
        {
            GameId = "game-a",
            Env = "staging",
            IdempotencyKey = "idempotency-1",
        });

        result.Success.Should().BeTrue();
        result.Data.Should().Be("{\"ok\":true}");
        handler.Requests.Should().ContainSingle();
        var request = handler.Requests.Single();
        request.Method.Should().Be(HttpMethod.Post);
        request.PathAndQuery.Should().Be("/api/v1/functions/player.ban/invoke");
        request.Headers["X-Game-ID"].Should().Be("game-a");
        request.Headers["X-Env"].Should().Be("staging");
        request.Headers["Idempotency-Key"].Should().Be("idempotency-1");
        request.Body.Should().Be("{\"params\":{\"playerId\":\"42\"}}");
    }

    [Fact]
    public async Task InvokeAsync_ReturnsFailureWhenServerRejectsRequest()
    {
        using var invoker = CreateInvoker(new RecordingHandler(_ => JsonResponse("{\"error\":\"forbidden\",\"message\":\"denied\"}", HttpStatusCode.Forbidden)));

        var result = await invoker.InvokeAsync("player.ban", "{}");

        result.Success.Should().BeFalse();
        result.Error.Should().Contain("denied");
    }

    [Fact]
    public async Task StartTaskAsync_CancelTaskAsync_AndGetTaskStatusAsync_UseTaskEndpoints()
    {
        var handler = new RecordingHandler(request => request.Method.Method switch
        {
            "POST" when request.Uri.AbsolutePath.EndsWith("/tasks", StringComparison.Ordinal) => JsonResponse("{\"taskId\":\"task-1\",\"status\":\"dispatching\"}"),
            "POST" when request.Uri.AbsolutePath.EndsWith("/cancel", StringComparison.Ordinal) => JsonResponse("{\"message\":\"操作成功\"}"),
            "GET" => JsonResponse("{\"id\":\"task-1\",\"status\":\"running\",\"progress\":50,\"message\":\"working\"}"),
            _ => JsonResponse("{\"message\":\"unexpected\"}", HttpStatusCode.NotFound),
        });
        using var invoker = CreateInvoker(handler);

        var taskId = await invoker.StartTaskAsync("mail.send", "{\"title\":\"Hi\"}");
        var cancelled = await invoker.CancelTaskAsync(taskId);
        var status = await invoker.GetTaskStatusAsync(taskId);

        taskId.Should().Be("task-1");
        cancelled.Should().BeTrue();
        status.TaskId.Should().Be("task-1");
        status.Status.Should().Be("running");
        status.Progress.Should().Be(50);
        handler.Requests.Select(request => request.PathAndQuery).Should().ContainInOrder(
            "/api/v1/tasks",
            "/api/v1/tasks/task-1/cancel",
            "/api/v1/tasks/task-1");
    }

    [Fact]
    public async Task StreamTaskAsync_UsesAfterSequenceCursorUntilDone()
    {
        var responses = new Queue<string>(new[]
        {
            "{\"items\":[{\"seq\":1,\"type\":\"started\",\"progress\":0}],\"done\":false}",
            "{\"items\":[{\"seq\":2,\"type\":\"completed\",\"progress\":100,\"payload\":{\"ok\":true}}],\"done\":true}",
        });
        var handler = new RecordingHandler(_ => JsonResponse(responses.Dequeue()));
        using var invoker = CreateInvoker(handler);

        var events = new List<TaskEvent>();
        await foreach (var taskEvent in invoker.StreamTaskAsync("task-2"))
        {
            events.Add(taskEvent);
        }

        events.Select(taskEvent => taskEvent.Type).Should().Equal("started", "completed");
        handler.Requests.Select(request => request.PathAndQuery).Should().Equal(
            "/api/v1/tasks/task-2/events?after_seq=0",
            "/api/v1/tasks/task-2/events?after_seq=1");
    }

    [Fact]
    public async Task BatchInvokeAsync_PreservesPerRequestIdempotencyKeys()
    {
        var handler = new RecordingHandler(_ => JsonResponse("{\"result\":{}}"));
        using var invoker = CreateInvoker(handler);

        var results = await invoker.BatchInvokeAsync(new List<BatchInvokeRequest>
        {
            new() { FunctionId = "a.run", Payload = "{}", IdempotencyKey = "a-key" },
            new() { FunctionId = "b.run", Payload = "{}", IdempotencyKey = "b-key" },
        });

        results.Should().OnlyContain(result => result.Success);
        handler.Requests.Select(request => request.Headers["Idempotency-Key"]).Should().BeEquivalentTo("a-key", "b-key");
    }

    [Integration.IntegrationFact]
    public async Task RealServerInvoker_CoversAuthenticatedTaskLifecycle()
    {
        var baseUrl = Environment.GetEnvironmentVariable("CROUPIER_SERVER_URL");
        var token = Environment.GetEnvironmentVariable("CROUPIER_SERVER_TOKEN");
        if (string.IsNullOrWhiteSpace(baseUrl) || string.IsNullOrWhiteSpace(token))
        {
            throw new InvalidOperationException("CROUPIER_SERVER_URL and CROUPIER_SERVER_TOKEN are required for the real Server Invoker integration test.");
        }

        var scope = new InvokerConfig
        {
            ServerBaseUrl = baseUrl,
            GameId = Environment.GetEnvironmentVariable("CROUPIER_GAME_ID") ?? "e2e-game",
            Env = Environment.GetEnvironmentVariable("CROUPIER_ENV") ?? "e2e",
        };

        using (var unauthenticated = new CroupierInvoker(scope))
        {
            var denied = await unauthenticated.InvokeAsync("mail.send", "{\"player_id\":\"p-001\",\"title\":\"denied\"}");
            denied.Success.Should().BeFalse();
        }

        using var invoker = new CroupierInvoker(new InvokerConfig
        {
            ServerBaseUrl = baseUrl,
            AuthToken = token,
            GameId = scope.GameId,
            Env = scope.Env,
            TaskPollIntervalMilliseconds = 10,
        });

        var result = await invoker.InvokeAsync("mail.send", "{\"player_id\":\"p-001\",\"title\":\"CSharp\",\"content\":\"body\"}");

        result.Success.Should().BeTrue();
        result.Data.Should().Contain("\"mail_id\":\"mail-0001\"");

        var completedTaskId = await invoker.StartTaskAsync("mail.send", "{\"player_id\":\"p-001\",\"title\":\"task\"}");
        var completedEvents = new List<TaskEvent>();
        await foreach (var taskEvent in invoker.StreamTaskAsync(completedTaskId))
        {
            completedEvents.Add(taskEvent);
        }
        completedEvents.Select(taskEvent => taskEvent.Type).Should().Contain("started");
        completedEvents.Select(taskEvent => taskEvent.Type).Should().Contain("completed");
        var completed = await WaitForStatusAsync(invoker, completedTaskId, "succeeded");
        completed.TaskId.Should().Be(completedTaskId);
        completed.Result.Should().Contain("\"mail_id\":\"mail-0001\"");

        var cancelledTaskId = await invoker.StartTaskAsync("mail.wait", "{\"wait_ms\":30000}");
        await WaitForStatusAsync(invoker, cancelledTaskId, "running");
        (await invoker.CancelTaskAsync(cancelledTaskId)).Should().BeTrue();
        var cancelledEvents = new List<TaskEvent>();
        await foreach (var taskEvent in invoker.StreamTaskAsync(cancelledTaskId))
        {
            cancelledEvents.Add(taskEvent);
        }
        cancelledEvents.Select(taskEvent => taskEvent.Type).Should().Contain("cancelled");
        (await WaitForStatusAsync(invoker, cancelledTaskId, "cancelled")).Status.Should().Be("cancelled");
    }

    private static async Task<TaskStatus> WaitForStatusAsync(CroupierInvoker invoker, string taskId, string expected)
    {
        var deadline = DateTime.UtcNow.AddSeconds(20);
        TaskStatus status = new() { TaskId = taskId, Status = "unknown" };
        while (DateTime.UtcNow < deadline)
        {
            status = await invoker.GetTaskStatusAsync(taskId);
            if (status.Status == expected)
            {
                return status;
            }
            await Task.Delay(50);
        }
        throw new Xunit.Sdk.XunitException($"task {taskId} status={status.Status}, want {expected}");
    }

    [Theory]
    [InlineData("")]
    [InlineData("   ")]
    public async Task InvokeAsync_RejectsEmptyFunctionId(string functionId)
    {
        using var invoker = CreateInvoker(new RecordingHandler(_ => JsonResponse("{}")));

        await Assert.ThrowsAsync<ArgumentException>(() => invoker.InvokeAsync(functionId, "{}"));
    }

    [Fact]
    public async Task StartTaskAsync_RejectsNullPayload()
    {
        using var invoker = CreateInvoker(new RecordingHandler(_ => JsonResponse("{}")));

        await Assert.ThrowsAsync<ArgumentNullException>(() => invoker.StartTaskAsync("mail.send", null!));
    }

    [Fact]
    public async Task TaskMethods_RejectEmptyTaskId()
    {
        using var invoker = CreateInvoker(new RecordingHandler(_ => JsonResponse("{}")));

        await Assert.ThrowsAsync<ArgumentException>(() => invoker.CancelTaskAsync(""));
        await Assert.ThrowsAsync<ArgumentException>(() => invoker.GetTaskStatusAsync(""));
    }

    [Fact]
    public async Task Methods_AfterDispose_ThrowObjectDisposedException()
    {
        var invoker = CreateInvoker(new RecordingHandler(_ => JsonResponse("{}")));
        invoker.Dispose();

        await Assert.ThrowsAsync<ObjectDisposedException>(() => invoker.InvokeAsync("mail.send", "{}"));
        await Assert.ThrowsAsync<ObjectDisposedException>(() => invoker.StartTaskAsync("mail.send", "{}"));
    }

    private static CroupierInvoker CreateInvoker(RecordingHandler handler)
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

    private sealed class RecordingHandler : HttpMessageHandler
    {
        private readonly Func<RecordedRequest, HttpResponseMessage> _respond;

        public RecordingHandler(Func<RecordedRequest, HttpResponseMessage> respond)
        {
            _respond = respond;
        }

        public List<RecordedRequest> Requests { get; } = new();

        protected override async Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken)
        {
            var headers = request.Headers.ToDictionary(pair => pair.Key, pair => string.Join(",", pair.Value), StringComparer.OrdinalIgnoreCase);
            var recorded = new RecordedRequest(
                request.Method,
                request.RequestUri!,
                request.RequestUri!.PathAndQuery,
                headers,
                request.Content == null ? string.Empty : await request.Content.ReadAsStringAsync(cancellationToken));
            Requests.Add(recorded);
            return _respond(recorded);
        }
    }

    private sealed record RecordedRequest(HttpMethod Method, Uri Uri, string PathAndQuery, Dictionary<string, string> Headers, string Body);
}
