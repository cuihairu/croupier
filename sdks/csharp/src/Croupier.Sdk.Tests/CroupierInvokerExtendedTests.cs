// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

using System.Net;
using System.Text;
using Croupier.Sdk.Models;
using FluentAssertions;
using Microsoft.Extensions.Logging.Abstractions;
using Xunit;

namespace Croupier.Sdk.Tests;

/// <summary>
/// Additional CroupierInvoker tests targeting error mapping, header
/// precedence, task parsing and stream polling branches.
/// </summary>
public sealed class CroupierInvokerExtendedTests
{
    private static HttpResponseMessage JsonResponse(string json, HttpStatusCode statusCode = HttpStatusCode.OK) => new(statusCode)
    {
        Content = new StringContent(json, Encoding.UTF8, "application/json"),
    };

    private static CroupierInvoker CreateInvoker(
        Func<RecordedRequest, HttpResponseMessage> respond,
        Action<InvokerConfig>? customizeConfig = null,
        bool ownsHttpClient = true)
    {
        var client = new HttpClient(new RecordingHandler(respond))
        {
            BaseAddress = new Uri("http://unused.invalid/"),
        };
        var config = new InvokerConfig
        {
            ServerBaseUrl = "http://server.test/api/v1",
            AuthToken = "token-1",
            GameId = "default-game",
            Env = "dev",
            TimeoutSeconds = 10,
            TaskPollIntervalMilliseconds = 1,
        };
        customizeConfig?.Invoke(config);
        return new CroupierInvoker(config, client, ownsHttpClient: ownsHttpClient);
    }

    private sealed record RecordedRequest(HttpMethod Method, Uri Uri, string PathAndQuery, Dictionary<string, string> Headers, string Body);

    private sealed class RecordingHandler : HttpMessageHandler
    {
        private readonly Func<RecordedRequest, HttpResponseMessage> _respond;
        private readonly TimeSpan? _delay;

        public RecordingHandler(Func<RecordedRequest, HttpResponseMessage> respond, TimeSpan? delay = null)
        {
            _respond = respond;
            _delay = delay;
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
            if (_delay is { } delay)
            {
                await Task.Delay(delay, cancellationToken);
            }

            return _respond(recorded);
        }
    }

    #region Construction / disposal

    [Fact]
    public void Constructor_WithMicrosoftLogger_AcceptsInstance()
    {
        using var invoker = new CroupierInvoker(new InvokerConfig { ServerBaseUrl = "http://server.test/" }, NullLogger.Instance);

        invoker.ServerBaseUrl.Should().Be("http://server.test/");
    }

    [Fact]
    public void Constructor_NormalizesBaseUrlAndExposesScopeProperties()
    {
        using var invoker = CreateInvoker(_ => JsonResponse("{}"), config =>
        {
            config.ServerBaseUrl = "http://normalized.test/api/v1///";
            config.GameId = "g1";
            config.Env = "e1";
        });

        invoker.ServerBaseUrl.Should().Be("http://normalized.test/api/v1/");
        invoker.GameId.Should().Be("g1");
        invoker.Env.Should().Be("e1");
    }

    [Theory]
    [InlineData("not-a-url")]
    [InlineData("/api/v1")]
    [InlineData("ftp://server.test/api/v1")]
    public void Constructor_RejectsInvalidBaseUrl(string baseUrl)
    {
        var action = () => new CroupierInvoker(new InvokerConfig { ServerBaseUrl = baseUrl });

        action.Should().Throw<ArgumentException>().WithMessage("*absolute HTTP(S)*");
    }

    [Fact]
    public async Task Constructor_ClampsMinimumTimeouts()
    {
        // TimeoutSeconds=0 must be clamped to 1s: a request against a slow
        // handler must fail after ~1s rather than instantly.
        using var invoker = new CroupierInvoker(
            new InvokerConfig { ServerBaseUrl = "http://server.test/", TimeoutSeconds = 0, TaskPollIntervalMilliseconds = 0 },
            new HttpClient(new RecordingHandler(_ => JsonResponse("{\"result\":\"slow\"}"), delay: TimeSpan.FromSeconds(5)))
            {
                BaseAddress = new Uri("http://unused.invalid/"),
            },
            ownsHttpClient: true);

        var stopwatch = System.Diagnostics.Stopwatch.StartNew();
        var result = await invoker.InvokeAsync("fn.slow", "{}");
        stopwatch.Stop();

        result.Success.Should().BeFalse();
        result.ErrorCode.Should().Be("CANCELED");
        stopwatch.Elapsed.Should().BeGreaterThan(TimeSpan.FromMilliseconds(500));
    }

    [Fact]
    public void Constructor_WithNullHttpClient_ThrowsArgumentNullException()
    {
        var action = () => new CroupierInvoker(new InvokerConfig { ServerBaseUrl = "http://server.test/" }, null!, ownsHttpClient: false);

        action.Should().Throw<ArgumentNullException>();
    }

    [Fact]
    public async Task Dispose_WhenOwningClient_DisposesUnderlyingClient()
    {
        var httpClient = new HttpClient(new RecordingHandler(_ => JsonResponse("{}")));
        var invoker = new CroupierInvoker(
            new InvokerConfig { ServerBaseUrl = "http://server.test/" },
            httpClient,
            ownsHttpClient: true);

        invoker.Dispose();
        invoker.Dispose(); // idempotent

        Func<Task> action = async () => await httpClient.GetAsync("http://server.test/");
        await action.Should().ThrowAsync<ObjectDisposedException>();
    }

    [Fact]
    public async Task Dispose_WhenNotOwningClient_LeavesClientUsable()
    {
        var handler = new RecordingHandler(_ => JsonResponse("{\"result\":1}"));
        var httpClient = new HttpClient(handler) { BaseAddress = new Uri("http://unused.invalid/") };
        var invoker = new CroupierInvoker(
            new InvokerConfig { ServerBaseUrl = "http://server.test/" },
            httpClient,
            ownsHttpClient: false);
        invoker.Dispose();

        using var reused = new CroupierInvoker(
            new InvokerConfig { ServerBaseUrl = "http://server.test/" },
            httpClient,
            ownsHttpClient: true);
        var result = await reused.InvokeAsync("fn.reuse", "{}");

        result.Success.Should().BeTrue("the shared HttpClient must survive the invoker disposal");
    }

    [Fact]
    public async Task Methods_AfterDispose_ThrowForTaskApis()
    {
        var invoker = CreateInvoker(_ => JsonResponse("{}"));
        invoker.Dispose();

        await Assert.ThrowsAsync<ObjectDisposedException>(() => invoker.CancelTaskAsync("t-1"));
        await Assert.ThrowsAsync<ObjectDisposedException>(() => invoker.GetTaskStatusAsync("t-1"));

        var action = async () =>
        {
            await foreach (var _ in invoker.StreamTaskAsync("t-1"))
            {
            }
        };
        await Assert.ThrowsAsync<ObjectDisposedException>(action);
    }

    #endregion

    #region InvokeAsync result / error mapping

    [Theory]
    [InlineData("{\"result\":{\"ok\":true}}", "{\"ok\":true}")]
    [InlineData("{\"result\":null}", "{\"result\":null}")]
    [InlineData("{\"other\":\"payload\"}", "{\"other\":\"payload\"}")]
    public async Task InvokeAsync_ExtractsResultPerContract(string responseJson, string expectedData)
    {
        using var invoker = CreateInvoker(_ => JsonResponse(responseJson));

        var result = await invoker.InvokeAsync("fn.extract", "{}");

        result.Success.Should().BeTrue();
        result.Data.Should().Be(expectedData);
    }

    [Fact]
    public async Task InvokeAsync_ServerErrorWithNonJsonBody_UsesRawContentAsError()
    {
        using var invoker = CreateInvoker(_ => new HttpResponseMessage(HttpStatusCode.InternalServerError)
        {
            Content = new StringContent("upstream exploded", Encoding.UTF8, "text/plain"),
        });

        var result = await invoker.InvokeAsync("fn.err", "{}");

        result.Success.Should().BeFalse();
        result.Error.Should().Contain("upstream exploded");
    }

    [Fact]
    public async Task InvokeAsync_ServerErrorWithEmptyBody_UsesHttpStatusText()
    {
        using var invoker = CreateInvoker(_ => new HttpResponseMessage(HttpStatusCode.InternalServerError));

        var result = await invoker.InvokeAsync("fn.err-empty", "{}");

        result.Success.Should().BeFalse();
        result.Error.Should().Be("HTTP 500");
    }

    [Fact]
    public async Task InvokeAsync_ServerErrorWithJsonMessage_IgnoresOtherFields()
    {
        using var invoker = CreateInvoker(_ => JsonResponse("{\"error\":\"conflict\",\"message\":\"already running\"}", HttpStatusCode.Conflict));

        var result = await invoker.InvokeAsync("fn.conflict", "{}");

        result.Success.Should().BeFalse();
        result.Error.Should().Be("already running");
    }

    [Fact]
    public async Task InvokeAsync_ServerErrorWithEmptyMessageField_FallsBackToRawBody()
    {
        using var invoker = CreateInvoker(_ => JsonResponse("{\"message\":\"   \"}", HttpStatusCode.BadRequest));

        var result = await invoker.InvokeAsync("fn.blank-message", "{}");

        result.Success.Should().BeFalse();
        result.Error.Should().Contain("message");
    }

    [Fact]
    public async Task InvokeAsync_OptionsTimeoutSmallerThanConfig_Prevails()
    {
        using var invoker = new CroupierInvoker(
            new InvokerConfig { ServerBaseUrl = "http://server.test/", TimeoutSeconds = 30 },
            new HttpClient(new RecordingHandler(_ => JsonResponse("{\"result\":\"late\"}"), delay: TimeSpan.FromSeconds(5)))
            {
                BaseAddress = new Uri("http://unused.invalid/"),
            },
            ownsHttpClient: true);

        var result = await invoker.InvokeAsync("fn.timeout", "{}", new InvokeOptions { TimeoutSeconds = 1 });

        result.Success.Should().BeFalse();
        result.ErrorCode.Should().Be("CANCELED");
        result.Error.Should().Be("Operation canceled");
    }

    [Fact]
    public async Task InvokeAsync_WithCancelledToken_MidFlight_ReturnsCanceledResult()
    {
        using var cts = new CancellationTokenSource();
        using var invoker = new CroupierInvoker(
            new InvokerConfig { ServerBaseUrl = "http://server.test/", TimeoutSeconds = 30, TaskPollIntervalMilliseconds = 1 },
            new HttpClient(new RecordingHandler(_ => JsonResponse("{\"result\":\"late\"}"), delay: TimeSpan.FromSeconds(10)))
            {
                BaseAddress = new Uri("http://unused.invalid/"),
            },
            ownsHttpClient: true);

        var invokeTask = invoker.InvokeAsync("fn.midcancel", "{}", cancellationToken: cts.Token);
        await Task.Delay(200);
        await cts.CancelAsync();

        var result = await invokeTask;

        result.Success.Should().BeFalse();
        result.ErrorCode.Should().Be("CANCELED");
    }

    [Fact]
    public async Task InvokeAsync_WhenHandlerThrows_ReturnsFailedResult()
    {
        using var invoker = CreateInvoker(_ => throw new InvalidOperationException("socket broken"));

        var result = await invoker.InvokeAsync("fn.throw", "{}");

        result.Success.Should().BeFalse();
        result.Error.Should().Contain("socket broken");
    }

    [Fact]
    public async Task InvokeAsync_WithMalformedPayloadJson_ReturnsFailedResult()
    {
        using var invoker = CreateInvoker(_ => JsonResponse("{}"));

        var result = await invoker.InvokeAsync("fn.badjson", "not-json");

        result.Success.Should().BeFalse();
        result.Error.Should().Contain("not-json");
    }

    [Fact]
    public async Task InvokeAsync_RejectsNullPayload()
    {
        using var invoker = CreateInvoker(_ => JsonResponse("{}"));

        await Assert.ThrowsAsync<ArgumentNullException>(() => invoker.InvokeAsync("fn.null", null!));
    }

    [Fact]
    public async Task InvokeAsync_EscapesFunctionIdInPath()
    {
        var handler = new RecordingHandler(_ => JsonResponse("{\"result\":1}"));
        using var invoker = new CroupierInvoker(
            new InvokerConfig { ServerBaseUrl = "http://server.test/api/v1" },
            new HttpClient(handler) { BaseAddress = new Uri("http://unused.invalid/") },
            ownsHttpClient: true);

        await invoker.InvokeAsync("a b/c+d", "{}");

        handler.Requests.Single().PathAndQuery.Should().Be("/api/v1/functions/a%20b%2Fc%2Bd/invoke");
    }

    #endregion

    #region Header precedence

    [Fact]
    public async Task InvokeAsync_ConfigHeadersAppliedThenOverriddenByMetadata()
    {
        var handler = new RecordingHandler(_ => JsonResponse("{\"result\":{}}"));
        using var invoker = new CroupierInvoker(
            new InvokerConfig
            {
                ServerBaseUrl = "http://server.test/api/v1",
                AuthToken = "auth-from-token",
                Headers = new Dictionary<string, string>(StringComparer.OrdinalIgnoreCase)
                {
                    ["X-Custom"] = "from-config",
                    ["Authorization"] = "auth-from-headers",
                },
            },
            new HttpClient(handler) { BaseAddress = new Uri("http://unused.invalid/") },
            ownsHttpClient: true);

        await invoker.InvokeAsync("fn.headers", "{}", new InvokeOptions
        {
            Metadata = new Dictionary<string, string> { ["X-Custom"] = "from-metadata" },
        });

        var headers = handler.Requests.Single().Headers;
        headers["X-Custom"].Should().Be("from-metadata");
        // config.Headers already carries a raw Authorization value, so the
        // AuthToken bearer header is not applied on top of it.
        headers["Authorization"].Should().Be("auth-from-headers");
    }

    [Fact]
    public async Task InvokeAsync_ConfigAuthTokenAppliedWhenNoAuthorizationHeader()
    {
        var handler = new RecordingHandler(_ => JsonResponse("{\"result\":{}}"));
        using var invoker = new CroupierInvoker(
            new InvokerConfig
            {
                ServerBaseUrl = "http://server.test/api/v1",
                AuthToken = "tok",
                Headers = new Dictionary<string, string>(StringComparer.OrdinalIgnoreCase),
            },
            new HttpClient(handler) { BaseAddress = new Uri("http://unused.invalid/") },
            ownsHttpClient: true);

        await invoker.InvokeAsync("fn.auth", "{}");

        handler.Requests.Single().Headers["Authorization"].Should().Be("Bearer tok");
    }

    [Fact]
    public async Task InvokeAsync_RequestIdAndUserIdAreForwarded()
    {
        var handler = new RecordingHandler(_ => JsonResponse("{\"result\":{}}"));
        using var invoker = new CroupierInvoker(
            new InvokerConfig { ServerBaseUrl = "http://server.test/api/v1" },
            new HttpClient(handler) { BaseAddress = new Uri("http://unused.invalid/") },
            ownsHttpClient: true);

        await invoker.InvokeAsync("fn.tracing", "{}", new InvokeOptions
        {
            RequestId = "req-42",
            UserId = "user-42",
        });

        var headers = handler.Requests.Single().Headers;
        headers["X-Request-ID"].Should().Be("req-42");
        headers["X-User-ID"].Should().Be("user-42");
    }

    [Fact]
    public async Task InvokeAsync_OptionsScopeOverridesConfigScope()
    {
        var handler = new RecordingHandler(_ => JsonResponse("{\"result\":{}}"));
        using var invoker = new CroupierInvoker(
            new InvokerConfig { ServerBaseUrl = "http://server.test/api/v1", GameId = "cfg-game", Env = "cfg-env" },
            new HttpClient(handler) { BaseAddress = new Uri("http://unused.invalid/") },
            ownsHttpClient: true);

        await invoker.InvokeAsync("fn.scope", "{}", new InvokeOptions { GameId = "opt-game", Env = "opt-env" });

        var headers = handler.Requests.Single().Headers;
        headers["X-Game-ID"].Should().Be("opt-game");
        headers["X-Env"].Should().Be("opt-env");
    }

    #endregion

    #region BatchInvokeAsync

    [Fact]
    public async Task BatchInvokeAsync_NullList_ThrowsArgumentNullException()
    {
        using var invoker = CreateInvoker(_ => JsonResponse("{}"));

        await Assert.ThrowsAsync<ArgumentNullException>(() => invoker.BatchInvokeAsync(null!));
    }

    [Fact]
    public async Task BatchInvokeAsync_EmptyList_ThrowsArgumentException()
    {
        using var invoker = CreateInvoker(_ => JsonResponse("{}"));

        await Assert.ThrowsAsync<ArgumentException>(() => invoker.BatchInvokeAsync(new List<BatchInvokeRequest>()));
    }

    [Fact]
    public async Task BatchInvokeAsync_WithoutIdempotencyKeys_SendsNoKeyHeader()
    {
        var handler = new RecordingHandler(_ => JsonResponse("{\"result\":{}}"));
        using var invoker = new CroupierInvoker(
            new InvokerConfig { ServerBaseUrl = "http://server.test/api/v1" },
            new HttpClient(handler) { BaseAddress = new Uri("http://unused.invalid/") },
            ownsHttpClient: true);

        var results = await invoker.BatchInvokeAsync(new List<BatchInvokeRequest>
        {
            new() { FunctionId = "a.run", Payload = "{}" },
            new() { FunctionId = "b.run", Payload = "{}" },
        });

        results.Should().OnlyContain(result => result.Success);
        handler.Requests.Should().OnlyContain(request => !request.Headers.ContainsKey("Idempotency-Key"));
    }

    [Fact]
    public async Task BatchInvokeAsync_MixedSuccessAndFailure_ReportsEachIndependently()
    {
        using var invoker = CreateInvoker(request => request.PathAndQuery.Contains("good", StringComparison.Ordinal)
            ? JsonResponse("{\"result\":{\"ok\":1}}")
            : JsonResponse("{\"message\":\"nope\"}", HttpStatusCode.Forbidden));

        var results = await invoker.BatchInvokeAsync(new List<BatchInvokeRequest>
        {
            new() { FunctionId = "fn.good", Payload = "{}", IdempotencyKey = "k1" },
            new() { FunctionId = "fn.bad", Payload = "{}", IdempotencyKey = "k2" },
        }, new InvokeOptions { GameId = "g", Env = "e" });

        results.Should().HaveCount(2);
        results[0].Success.Should().BeTrue();
        results[0].Data.Should().Be("{\"ok\":1}");
        results[1].Success.Should().BeFalse();
        results[1].Error.Should().Contain("nope");
    }

    #endregion

    #region StartTaskAsync / CancelTaskAsync / GetTaskStatusAsync

    [Fact]
    public async Task StartTaskAsync_WhenTaskIdMissing_ThrowsInvalidOperationException()
    {
        using var invoker = CreateInvoker(_ => JsonResponse("{\"status\":\"accepted\"}"));

        var action = () => invoker.StartTaskAsync("fn.task", "{}");

        (await action.Should().ThrowAsync<InvalidOperationException>())
            .WithMessage("*taskId*");
    }

    [Theory]
    [InlineData("{\"taskId\":\"\"}")]
    [InlineData("{\"taskId\":\"   \"}")]
    [InlineData("{\"taskId\":null}")]
    public async Task StartTaskAsync_WhenTaskIdBlank_ThrowsInvalidOperationException(string body)
    {
        using var invoker = CreateInvoker(_ => JsonResponse(body));

        var action = () => invoker.StartTaskAsync("fn.task", "{}");

        await action.Should().ThrowAsync<InvalidOperationException>();
    }

    [Fact]
    public async Task StartTaskAsync_WhenServerErrors_ThrowsHttpRequestExceptionWithMessage()
    {
        using var invoker = CreateInvoker(_ => JsonResponse("{\"message\":\"queue full\"}", HttpStatusCode.ServiceUnavailable));

        var action = () => invoker.StartTaskAsync("fn.task", "{}");

        (await action.Should().ThrowAsync<HttpRequestException>())
            .Where(ex => ex.Message.Contains("queue full") && ex.StatusCode == HttpStatusCode.ServiceUnavailable);
    }

    [Fact]
    public async Task StartTaskAsync_SendsScopeAndParamsInBody()
    {
        var handler = new RecordingHandler(_ => JsonResponse("{\"taskId\":\"task-9\"}"));
        using var invoker = new CroupierInvoker(
            new InvokerConfig { ServerBaseUrl = "http://server.test/api/v1", GameId = "g9", Env = "e9" },
            new HttpClient(handler) { BaseAddress = new Uri("http://unused.invalid/") },
            ownsHttpClient: true);

        var taskId = await invoker.StartTaskAsync("mail.send", "{\"title\":\"Hi\"}");

        taskId.Should().Be("task-9");
        var request = handler.Requests.Single();
        request.Body.Should().Contain("\"functionId\":\"mail.send\"");
        request.Body.Should().Contain("\"params\":{\"title\":\"Hi\"}");
        request.Body.Should().Contain("\"gameId\":\"g9\"");
        request.Body.Should().Contain("\"env\":\"e9\"");
    }

    [Fact]
    public async Task StartTaskAsync_RejectsEmptyFunctionId()
    {
        using var invoker = CreateInvoker(_ => JsonResponse("{\"taskId\":\"t\"}"));

        await Assert.ThrowsAsync<ArgumentException>(() => invoker.StartTaskAsync(" ", "{}"));
    }

    [Fact]
    public async Task CancelTaskAsync_WhenServerErrors_ThrowsHttpRequestException()
    {
        using var invoker = CreateInvoker(_ => JsonResponse("{\"message\":\"already finished\"}", HttpStatusCode.Conflict));

        var action = () => invoker.CancelTaskAsync("task-x");

        (await action.Should().ThrowAsync<HttpRequestException>())
            .Where(ex => ex.Message.Contains("already finished"));
    }

    [Fact]
    public async Task GetTaskStatusAsync_ParsesCompleteStatusPayload()
    {
        using var invoker = CreateInvoker(_ => JsonResponse(
            """
            {"id":"task-1","status":"succeeded","progress":100,"message":"done",
             "error":null,"result":{"mail_id":"mail-0001"},
             "startedAt":"2025-08-01T10:00:00Z","finishedAt":"2025-08-01T10:00:05Z"}
            """));

        var status = await invoker.GetTaskStatusAsync("task-1");

        status.TaskId.Should().Be("task-1");
        status.Status.Should().Be("succeeded");
        status.Progress.Should().Be(100);
        status.Message.Should().Be("done");
        status.Error.Should().BeNull();
        status.Result.Should().Contain("mail-0001");
        status.StartTime.Should().BeAfter(DateTime.Parse("2025-01-01"));
        status.EndTime.Should().BeAfter(status.StartTime!.Value);
    }

    [Fact]
    public async Task GetTaskStatusAsync_WithEmptyBody_UsesRequestedTaskIdAndDefaults()
    {
        using var invoker = CreateInvoker(_ => JsonResponse("{}"));

        var status = await invoker.GetTaskStatusAsync("task-fallback");

        status.TaskId.Should().Be("task-fallback");
        status.Status.Should().Be("unknown");
        status.Progress.Should().Be(0);
        status.Message.Should().BeNull();
        status.Result.Should().BeNull();
        status.StartTime.Should().BeNull();
        status.EndTime.Should().BeNull();
    }

    [Fact]
    public async Task GetTaskStatusAsync_WithStringProgress_ThrowsInvalidOperation_DocumentsBug()
    {
        // Documents a robustness bug: GetInt/GetDouble/GetLong/GetDateTime
        // only check property presence, not JSON kind. JsonElement.TryGetDouble
        // throws InvalidOperationException when the value is not a number,
        // so a server replying {"progress":"high"} crashes GetTaskStatusAsync
        // instead of returning progress=0.
        using var invoker = CreateInvoker(_ => JsonResponse("{\"id\":123,\"status\":456,\"progress\":\"high\"}"));

        var action = () => invoker.GetTaskStatusAsync("task-weird");

        await action.Should().ThrowAsync<InvalidOperationException>();
    }

    [Fact]
    public async Task GetTaskStatusAsync_WithInvalidDateValue_ReturnsNullDates()
    {
        using var invoker = CreateInvoker(_ => JsonResponse(
            "{\"startedAt\":\"not-a-date\",\"finishedAt\":\"yesterday\"}"));

        var status = await invoker.GetTaskStatusAsync("task-dates");

        status.StartTime.Should().BeNull();
        status.EndTime.Should().BeNull();
    }

    [Fact]
    public async Task GetTaskStatusAsync_WhenServerErrors_ThrowsHttpRequestException()
    {
        using var invoker = CreateInvoker(_ => JsonResponse("{\"message\":\"gone\"}", HttpStatusCode.NotFound));

        var action = () => invoker.GetTaskStatusAsync("task-404");

        await action.Should().ThrowAsync<HttpRequestException>();
    }

    [Fact]
    public async Task TaskId_IsEscapedInUrls()
    {
        var handler = new RecordingHandler(_ => JsonResponse("{\"id\":\"t\",\"status\":\"unknown\"}"));
        using var invoker = new CroupierInvoker(
            new InvokerConfig { ServerBaseUrl = "http://server.test/api/v1" },
            new HttpClient(handler) { BaseAddress = new Uri("http://unused.invalid/") },
            ownsHttpClient: true);

        await invoker.GetTaskStatusAsync("task/1");

        handler.Requests.Single().PathAndQuery.Should().Be("/api/v1/tasks/task%2F1");
    }

    #endregion

    #region StreamTaskAsync

    [Fact]
    public async Task StreamTaskAsync_EmptyItemsThenDone_PollsAndFinishes()
    {
        var responses = new Queue<string>(new[]
        {
            "{\"items\":[],\"done\":false}",
            "{\"items\":[],\"done\":false}",
            "{\"done\":true}",
        });
        var handler = new RecordingHandler(_ => JsonResponse(responses.Dequeue()));
        using var invoker = new CroupierInvoker(
            new InvokerConfig { ServerBaseUrl = "http://server.test/api/v1", TaskPollIntervalMilliseconds = 1 },
            new HttpClient(handler) { BaseAddress = new Uri("http://unused.invalid/") },
            ownsHttpClient: true);

        var count = 0;
        await foreach (var _ in invoker.StreamTaskAsync("task-poll"))
        {
            count++;
        }

        count.Should().Be(0);
        handler.Requests.Should().HaveCount(3);
    }

    public static TheoryData<string> TerminalEventTypes => new()
    {
        "completed",
        "failed",
        "cancelled",
        "timed_out",
    };

    [Theory]
    [MemberData(nameof(TerminalEventTypes))]
    public async Task StreamTaskAsync_TerminalEventWithoutDoneFlag_EndsStream(string terminalType)
    {
        var responses = new Queue<string>(new[]
        {
            $"{{\"items\":[{{\"seq\":5,\"type\":\"{terminalType}\",\"progress\":100,\"message\":\"over\",\"payload\":{{\"z\":1}},\"createdAt\":\"2025-08-01T10:00:00Z\"}}],\"done\":false}}",
        });
        var handler = new RecordingHandler(_ => JsonResponse(responses.Dequeue()));
        using var invoker = new CroupierInvoker(
            new InvokerConfig { ServerBaseUrl = "http://server.test/api/v1", TaskPollIntervalMilliseconds = 1 },
            new HttpClient(handler) { BaseAddress = new Uri("http://unused.invalid/") },
            ownsHttpClient: true);

        var events = new List<TaskEvent>();
        await foreach (var taskEvent in invoker.StreamTaskAsync("task-terminal"))
        {
            events.Add(taskEvent);
        }

        events.Should().ContainSingle().Which.Should().Match<TaskEvent>(taskEvent =>
            taskEvent.Seq == 5 &&
            taskEvent.Type == terminalType &&
            taskEvent.Progress == 100 &&
            taskEvent.Message == "over" &&
            taskEvent.Payload!.Contains("\"z\":1") &&
            taskEvent.CreatedAt.HasValue);
        handler.Requests.Should().ContainSingle();
    }

    [Fact]
    public async Task StreamTaskAsync_NonTerminalEvent_KeepsPollingWithCursor()
    {
        var responses = new Queue<string>(new[]
        {
            "{\"items\":[{\"seq\":3,\"type\":\"progress\",\"progress\":10}],\"done\":false}",
            "{\"items\":[{\"seq\":7,\"type\":\"log\"},{\"seq\":9,\"type\":\"completed\"}],\"done\":false}",
        });
        var handler = new RecordingHandler(_ => JsonResponse(responses.Dequeue()));
        using var invoker = new CroupierInvoker(
            new InvokerConfig { ServerBaseUrl = "http://server.test/api/v1", TaskPollIntervalMilliseconds = 1 },
            new HttpClient(handler) { BaseAddress = new Uri("http://unused.invalid/") },
            ownsHttpClient: true);

        var events = new List<TaskEvent>();
        await foreach (var taskEvent in invoker.StreamTaskAsync("task-cursor"))
        {
            events.Add(taskEvent);
        }

        events.Select(taskEvent => taskEvent.Seq).Should().Equal(3, 7, 9);
        handler.Requests.Select(request => request.PathAndQuery).Should().Equal(
            "/api/v1/tasks/task-cursor/events?after_seq=0",
            "/api/v1/tasks/task-cursor/events?after_seq=3");
    }

    [Fact]
    public async Task StreamTaskAsync_MissingFields_DefaultToZeroAndUnknown()
    {
        var responses = new Queue<string>(new[]
        {
            "{\"items\":[{}],\"done\":true}",
        });
        var handler = new RecordingHandler(_ => JsonResponse(responses.Dequeue()));
        using var invoker = new CroupierInvoker(
            new InvokerConfig { ServerBaseUrl = "http://server.test/api/v1" },
            new HttpClient(handler) { BaseAddress = new Uri("http://unused.invalid/") },
            ownsHttpClient: true);

        var events = new List<TaskEvent>();
        await foreach (var taskEvent in invoker.StreamTaskAsync("task-defaults"))
        {
            events.Add(taskEvent);
        }

        events.Should().ContainSingle().Which.Should().Match<TaskEvent>(taskEvent =>
            taskEvent.Seq == 0 &&
            taskEvent.Type == "unknown" &&
            taskEvent.Progress == 0 &&
            taskEvent.Message == null &&
            taskEvent.Payload == null &&
            taskEvent.CreatedAt == null);
    }

    [Fact]
    public async Task StreamTaskAsync_WhenServerErrorsOnFirstPoll_Throws()
    {
        using var invoker = CreateInvoker(_ => JsonResponse("{\"message\":\"no task\"}", HttpStatusCode.NotFound));

        var action = async () =>
        {
            await foreach (var _ in invoker.StreamTaskAsync("task-missing"))
            {
            }
        };

        (await action.Should().ThrowAsync<HttpRequestException>())
            .Where(ex => ex.Message.Contains("no task"));
    }

    [Fact]
    public async Task StreamTaskAsync_CancellationMidStream_ThrowsOperationCanceled()
    {
        using var cts = new CancellationTokenSource();
        using var invoker = new CroupierInvoker(
            new InvokerConfig { ServerBaseUrl = "http://server.test/api/v1", TaskPollIntervalMilliseconds = 10 },
            new HttpClient(new RecordingHandler(_ => JsonResponse("{\"items\":[],\"done\":false}")))
            {
                BaseAddress = new Uri("http://unused.invalid/"),
            },
            ownsHttpClient: true);

        var action = async () =>
        {
            var iterate = Task.Run(async () =>
            {
                await foreach (var _ in invoker.StreamTaskAsync("task-midcancel", cts.Token))
                {
                }
            });

            await Task.Delay(200);
            await cts.CancelAsync();
            await iterate;
        };

        await action.Should().ThrowAsync<OperationCanceledException>();
    }

    [Fact]
    public async Task StreamTaskAsync_RejectsEmptyTaskId()
    {
        using var invoker = CreateInvoker(_ => JsonResponse("{}"));

        var action = async () =>
        {
            await foreach (var _ in invoker.StreamTaskAsync(""))
            {
            }
        };

        await action.Should().ThrowAsync<ArgumentException>();
    }

    #endregion
}
