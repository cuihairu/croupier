// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

using System.Net;
using System.Net.Http.Headers;
using System.Runtime.CompilerServices;
using System.Text;
using System.Text.Json;
using Croupier.Sdk.Logging;
using Croupier.Sdk.Models;
using Croupier.Sdk.Validation;
using Microsoft.Extensions.Logging;

namespace Croupier.Sdk;

/// <summary>
/// L3 调用方：通过 Server HTTP API 调用函数和管理任务。
/// </summary>
/// <remarks>
/// Provider 使用 <see cref="CroupierClient"/> 连接 Agent 本地 SDK gateway；调用方不复用
/// Provider session 或 <see cref="ClientConfig"/>，以避免绕过 Server 的鉴权、审计和 scope 校验。
/// </remarks>
public sealed class CroupierInvoker : IDisposable
{
    private static readonly JsonSerializerOptions JsonOptions = new(JsonSerializerDefaults.Web);
    private readonly InvokerConfig _config;
    private readonly ICroupierLogger _logger;
    private readonly HttpClient _httpClient;
    private readonly bool _ownsHttpClient;
    private readonly Dictionary<string, JsonElement> _schemas = new();
    private bool _isDisposed;

    /// <summary>调用目标的 Server REST API 基地址。</summary>
    public string ServerBaseUrl => _config.ServerBaseUrl;

    /// <summary>默认游戏作用域。</summary>
    public string? GameId => _config.GameId;

    /// <summary>默认环境作用域。</summary>
    public string? Env => _config.Env;

    /// <summary>
    /// 创建独立调用方。
    /// </summary>
    public CroupierInvoker(InvokerConfig config, ICroupierLogger? logger = null)
        : this(config, new HttpClient { Timeout = Timeout.InfiniteTimeSpan }, true, logger)
    {
    }

    /// <summary>
    /// 创建带 Microsoft ILogger 的独立调用方。
    /// </summary>
    public CroupierInvoker(InvokerConfig config, ILogger logger)
        : this(config, new CroupierLogger(logger))
    {
    }

    internal CroupierInvoker(InvokerConfig config, HttpClient httpClient, bool ownsHttpClient = false, ICroupierLogger? logger = null)
    {
        _config = NormalizeConfig(config);
        _httpClient = httpClient ?? throw new ArgumentNullException(nameof(httpClient));
        _httpClient.Timeout = Timeout.InfiniteTimeSpan;
        _ownsHttpClient = ownsHttpClient;
        _logger = logger ?? new ConsoleCroupierLogger("Invoker");
    }

    /// <summary>
    /// 同步调用函数并返回函数结果的 JSON 文本。
    /// </summary>
    public async Task<InvokeResult> InvokeAsync(
        string functionId,
        string payload,
        InvokeOptions? options = null,
        CancellationToken cancellationToken = default)
    {
        ThrowIfDisposed();
        ValidateFunctionAndPayload(functionId, payload);
        var startedAt = DateTime.UtcNow;

        try
        {
            HttpRequestMessage RequestFactory()
            {
                var request = new HttpRequestMessage(HttpMethod.Post, BuildUri($"functions/{Uri.EscapeDataString(functionId)}/invoke"));
                request.Content = CreateJsonContent(new { @params = ParseJson(payload) });
                ApplyHeaders(request, options);
                return request;
            }

            var (_, content) = await SendWithRetryAsync(RequestFactory, options, cancellationToken).ConfigureAwait(false);

            return InvokeResult.Succeeded(ExtractResult(content), ElapsedMilliseconds(startedAt));
        }
        catch (OperationCanceledException)
        {
            return InvokeResult.Failed("Operation canceled", "CANCELED", ElapsedMilliseconds(startedAt));
        }
        catch (Exception ex)
        {
            _logger.LogError("CroupierInvoker", $"Invoke {functionId} failed: {ex.Message}", ex);
            return InvokeResult.Failed(ex.Message, null, ElapsedMilliseconds(startedAt));
        }
    }

    /// <summary>
    /// 批量调用函数。每个请求保留独立的幂等键。
    /// </summary>
    public async Task<List<InvokeResult>> BatchInvokeAsync(
        List<BatchInvokeRequest> requests,
        InvokeOptions? options = null,
        CancellationToken cancellationToken = default)
    {
        ThrowIfDisposed();
        ArgumentNullException.ThrowIfNull(requests);
        if (requests.Count == 0)
        {
            throw new ArgumentException("Requests list cannot be empty", nameof(requests));
        }

        var calls = requests.Select(request => InvokeAsync(
            request.FunctionId,
            request.Payload,
            MergeOptions(options, request.IdempotencyKey),
            cancellationToken));
        return (await Task.WhenAll(calls).ConfigureAwait(false)).ToList();
    }

    /// <summary>
    /// 启动异步任务并返回 Server 分配的任务 ID。
    /// </summary>
    public async Task<string> StartTaskAsync(
        string functionId,
        string payload,
        InvokeOptions? options = null,
        CancellationToken cancellationToken = default)
    {
        ThrowIfDisposed();
        ValidateFunctionAndPayload(functionId, payload);

        HttpRequestMessage RequestFactory()
        {
            var request = new HttpRequestMessage(HttpMethod.Post, BuildUri("tasks"));
            request.Content = CreateJsonContent(new
            {
                functionId,
                @params = ParseJson(payload),
                gameId = ResolveGameId(options),
                env = ResolveEnv(options),
            });
            ApplyHeaders(request, options);
            return request;
        }

        var (_, content) = await SendWithRetryAsync(RequestFactory, options, cancellationToken).ConfigureAwait(false);

        using var document = JsonDocument.Parse(content);
        if (!document.RootElement.TryGetProperty("taskId", out var taskId) || string.IsNullOrWhiteSpace(taskId.GetString()))
        {
            throw new InvalidOperationException("Server did not return a taskId");
        }
        return taskId.GetString()!;
    }

    /// <summary>
    /// 取消任务。Server 接受后才返回 <c>true</c>。
    /// </summary>
    public async Task<bool> CancelTaskAsync(string taskId, CancellationToken cancellationToken = default)
    {
        ThrowIfDisposed();
        ValidateTaskId(taskId);

        using var request = new HttpRequestMessage(HttpMethod.Post, BuildUri($"tasks/{Uri.EscapeDataString(taskId)}/cancel"))
        {
            Content = CreateJsonContent(new { }),
        };
        ApplyHeaders(request, null);

        using var response = await SendAsync(request, null, cancellationToken).ConfigureAwait(false);
        var content = await response.Content.ReadAsStringAsync(cancellationToken).ConfigureAwait(false);
        EnsureSuccess(response, content);
        return true;
    }

    /// <summary>
    /// 获取任务当前状态。
    /// </summary>
    public async Task<TaskStatus> GetTaskStatusAsync(string taskId, CancellationToken cancellationToken = default)
    {
        ThrowIfDisposed();
        ValidateTaskId(taskId);

        using var request = new HttpRequestMessage(HttpMethod.Get, BuildUri($"tasks/{Uri.EscapeDataString(taskId)}"));
        ApplyHeaders(request, null);

        using var response = await SendAsync(request, null, cancellationToken).ConfigureAwait(false);
        var content = await response.Content.ReadAsStringAsync(cancellationToken).ConfigureAwait(false);
        EnsureSuccess(response, content);
        return ParseTaskStatus(taskId, content);
    }

    /// <summary>
    /// 轮询 Server 任务事件，直到 Server 返回完成状态或取消令牌被触发。
    /// </summary>
    public async IAsyncEnumerable<TaskEvent> StreamTaskAsync(
        string taskId,
        [EnumeratorCancellation] CancellationToken cancellationToken = default)
    {
        ThrowIfDisposed();
        ValidateTaskId(taskId);

        var afterSeq = 0L;
        while (true)
        {
            using var request = new HttpRequestMessage(HttpMethod.Get, BuildUri($"tasks/{Uri.EscapeDataString(taskId)}/events?after_seq={afterSeq}"));
            ApplyHeaders(request, null);

            using var response = await SendAsync(request, null, cancellationToken).ConfigureAwait(false);
            var content = await response.Content.ReadAsStringAsync(cancellationToken).ConfigureAwait(false);
            EnsureSuccess(response, content);

            using var document = JsonDocument.Parse(content);
            var root = document.RootElement;
            var done = root.TryGetProperty("done", out var doneElement) && doneElement.ValueKind == JsonValueKind.True;
            var emitted = false;

            if (root.TryGetProperty("items", out var items) && items.ValueKind == JsonValueKind.Array)
            {
                foreach (var item in items.EnumerateArray())
                {
                    var taskEvent = ParseTaskEvent(item);
                    afterSeq = Math.Max(afterSeq, taskEvent.Seq);
                    emitted = true;
                    yield return taskEvent;
                    done |= IsTerminal(taskEvent.Type);
                }
            }

            if (done)
            {
                yield break;
            }

            if (!emitted)
            {
                await Task.Delay(TimeSpan.FromMilliseconds(_config.TaskPollIntervalMilliseconds), cancellationToken).ConfigureAwait(false);
            }
        }
    }

    /// <summary>
    /// 为指定函数配置本地 JSON Schema（Draft-07 子集）校验；
    /// 校验在 invoke/startTask 网络请求前执行。
    /// </summary>
    public void SetSchema(string functionId, string schemaJson)
    {
        ThrowIfDisposed();
        if (string.IsNullOrWhiteSpace(functionId))
        {
            throw new ArgumentException("Function ID cannot be empty", nameof(functionId));
        }
        ArgumentNullException.ThrowIfNull(schemaJson);
        _schemas[functionId] = ParseJson(schemaJson);
    }

    /// <summary>
    /// 移除指定函数的本地 schema 校验。
    /// </summary>
    public void ClearSchema(string functionId)
    {
        ThrowIfDisposed();
        _schemas.Remove(functionId);
    }

    /// <summary>
    /// 释放 HTTP 客户端（仅释放由本实例创建的客户端）。
    /// </summary>
    public void Dispose()
    {
        if (_isDisposed)
        {
            return;
        }

        _isDisposed = true;
        if (_ownsHttpClient)
        {
            _httpClient.Dispose();
        }
        GC.SuppressFinalize(this);
    }

    private async Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, InvokeOptions? options, CancellationToken cancellationToken)
    {
        using var timeoutCts = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
        timeoutCts.CancelAfter(TimeSpan.FromSeconds(Math.Max(options?.TimeoutSeconds ?? _config.TimeoutSeconds, 1)));
        return await _httpClient.SendAsync(request, HttpCompletionOption.ResponseHeadersRead, timeoutCts.Token).ConfigureAwait(false);
    }

    private void ApplyHeaders(HttpRequestMessage request, InvokeOptions? options)
    {
        foreach (var (key, value) in _config.Headers)
        {
            request.Headers.TryAddWithoutValidation(key, value);
        }
        if (options?.Metadata != null)
        {
            foreach (var (key, value) in options.Metadata)
            {
                request.Headers.Remove(key);
                request.Headers.TryAddWithoutValidation(key, value);
            }
        }
        if (!string.IsNullOrWhiteSpace(_config.AuthToken) && !request.Headers.Contains("Authorization"))
        {
            request.Headers.Authorization = new AuthenticationHeaderValue("Bearer", _config.AuthToken);
        }
        if (!string.IsNullOrWhiteSpace(ResolveGameId(options)))
        {
            request.Headers.TryAddWithoutValidation("X-Game-ID", ResolveGameId(options));
        }
        if (!string.IsNullOrWhiteSpace(ResolveEnv(options)))
        {
            request.Headers.TryAddWithoutValidation("X-Env", ResolveEnv(options));
        }
        if (!string.IsNullOrWhiteSpace(options?.RequestId))
        {
            request.Headers.TryAddWithoutValidation("X-Request-ID", options.RequestId);
        }
        if (!string.IsNullOrWhiteSpace(options?.UserId))
        {
            request.Headers.TryAddWithoutValidation("X-User-ID", options.UserId);
        }
        if (!string.IsNullOrWhiteSpace(options?.IdempotencyKey))
        {
            request.Headers.TryAddWithoutValidation("Idempotency-Key", options.IdempotencyKey);
        }
    }

    private Uri BuildUri(string relativePath) => new(new Uri(_config.ServerBaseUrl, UriKind.Absolute), relativePath);

    private string? ResolveGameId(InvokeOptions? options) => options?.GameId ?? _config.GameId;

    private string? ResolveEnv(InvokeOptions? options) => options?.Env ?? _config.Env;

    private static InvokerConfig NormalizeConfig(InvokerConfig config)
    {
        ArgumentNullException.ThrowIfNull(config);
        if (!Uri.TryCreate(config.ServerBaseUrl, UriKind.Absolute, out var uri) || (uri.Scheme != Uri.UriSchemeHttp && uri.Scheme != Uri.UriSchemeHttps))
        {
            throw new ArgumentException("ServerBaseUrl must be an absolute HTTP(S) URL", nameof(config));
        }

        return new InvokerConfig
        {
            ServerBaseUrl = uri.ToString().TrimEnd('/') + "/",
            AuthToken = config.AuthToken,
            GameId = config.GameId,
            Env = config.Env,
            TimeoutSeconds = Math.Max(config.TimeoutSeconds, 1),
            TaskPollIntervalMilliseconds = Math.Max(config.TaskPollIntervalMilliseconds, 1),
            Headers = new Dictionary<string, string>(config.Headers ?? new Dictionary<string, string>(), StringComparer.OrdinalIgnoreCase),
            Retry = config.Retry,
        };
    }

    private static StringContent CreateJsonContent(object body) => new(JsonSerializer.Serialize(body, JsonOptions), Encoding.UTF8, "application/json");

    private static JsonElement ParseJson(string payload)
    {
        using var document = JsonDocument.Parse(payload);
        return document.RootElement.Clone();
    }

    private void ValidateFunctionAndPayload(string functionId, string payload)
    {
        if (string.IsNullOrWhiteSpace(functionId))
        {
            throw new ArgumentException("Function ID cannot be empty", nameof(functionId));
        }
        ArgumentNullException.ThrowIfNull(payload);

        if (_schemas.TryGetValue(functionId, out var schema))
        {
            JsonElement value;
            try
            {
                value = ParseJson(payload.Length == 0 ? "{}" : payload);
            }
            catch (JsonException exception)
            {
                throw new ArgumentException($"payload must be valid JSON: {exception.Message}", exception);
            }
            var errors = JsonSchemaValidator.Validate(schema, value);
            if (errors.Count > 0)
            {
                throw new ArgumentException(
                    $"payload validation failed: {string.Join("; ", errors)}");
            }
        }
    }

    private static void ValidateTaskId(string taskId)
    {
        if (string.IsNullOrWhiteSpace(taskId))
        {
            throw new ArgumentException("Task ID cannot be empty", nameof(taskId));
        }
    }

    private static void EnsureSuccess(HttpResponseMessage response, string content)
    {
        if (response.IsSuccessStatusCode)
        {
            return;
        }

        throw new HttpRequestException(ExtractErrorMessage(content, response.StatusCode), null, response.StatusCode);
    }

    private static void EnsureSuccess(int statusCode, string content)
    {
        if (statusCode is >= 200 and < 300)
        {
            return;
        }

        throw new HttpRequestException(
            ExtractErrorMessage(content, (HttpStatusCode)statusCode), null, (HttpStatusCode)statusCode);
    }

    /// <summary>
    /// 执行一次请求并在失败时可按 RetryConfig 自动重试。
    /// 返回 (状态码, 响应体)；不可重试或达到上限时抛出。
    /// </summary>
    private async Task<(int StatusCode, string Content)> SendWithRetryAsync(
        Func<HttpRequestMessage> requestFactory,
        InvokeOptions? options,
        CancellationToken cancellationToken)
    {
        var retry = (options?.Retry ?? _config.Retry)?.Normalized();
        var attempts = retry is { Enabled: true } ? retry.MaxAttempts : 1;

        Exception lastError = new HttpRequestException("Server HTTP request failed");
        for (var attempt = 0; attempt < attempts; attempt++)
        {
            try
            {
                using var request = requestFactory();
                using var response = await SendAsync(request, options, cancellationToken).ConfigureAwait(false);
                var content = await response.Content.ReadAsStringAsync(cancellationToken).ConfigureAwait(false);
                EnsureSuccess((int)response.StatusCode, content);
                return ((int)response.StatusCode, content);
            }
            catch (OperationCanceledException)
            {
                throw;
            }
            catch (Exception exception)
            {
                lastError = exception;
                var retryable = exception is HttpRequestException httpRequest
                    && (retry is null
                        || httpRequest.StatusCode is null
                        || retry.IsRetryableStatus((int)httpRequest.StatusCode));
                if (!retryable || attempt == attempts - 1)
                {
                    throw;
                }
                await Task.Delay(TimeSpan.FromMilliseconds(retry!.DelayMs(attempt)), cancellationToken).ConfigureAwait(false);
            }
        }
        throw lastError;
    }

    private static string ExtractResult(string content)
    {
        using var document = JsonDocument.Parse(content);
        if (document.RootElement.TryGetProperty("result", out var result) && result.ValueKind != JsonValueKind.Null)
        {
            return result.GetRawText();
        }
        return document.RootElement.GetRawText();
    }

    private static string ExtractErrorMessage(string content, HttpStatusCode statusCode)
    {
        try
        {
            using var document = JsonDocument.Parse(content);
            if (document.RootElement.TryGetProperty("message", out var message) && !string.IsNullOrWhiteSpace(message.GetString()))
            {
                return message.GetString()!;
            }
        }
        catch (JsonException)
        {
            // Preserve a non-JSON upstream response below.
        }
        return string.IsNullOrWhiteSpace(content) ? $"HTTP {(int)statusCode}" : content;
    }

    private static TaskStatus ParseTaskStatus(string requestedTaskId, string content)
    {
        using var document = JsonDocument.Parse(content);
        var root = document.RootElement;
        return new TaskStatus
        {
            TaskId = GetString(root, "id") ?? requestedTaskId,
            Status = GetString(root, "status") ?? "unknown",
            Progress = GetDouble(root, "progress"),
            Message = GetString(root, "message"),
            Error = GetString(root, "error"),
            Result = GetRawJson(root, "result"),
            StartTime = GetDateTime(root, "startedAt"),
            EndTime = GetDateTime(root, "finishedAt"),
        };
    }

    private static TaskEvent ParseTaskEvent(JsonElement item) => new()
    {
        Seq = GetLong(item, "seq"),
        Type = GetString(item, "type") ?? "unknown",
        Progress = GetInt(item, "progress"),
        Message = GetString(item, "message"),
        Payload = GetRawJson(item, "payload"),
        CreatedAt = GetDateTimeOffset(item, "createdAt"),
    };

    private static string? GetString(JsonElement root, string property) =>
        root.TryGetProperty(property, out var value) && value.ValueKind == JsonValueKind.String ? value.GetString() : null;

    private static string? GetRawJson(JsonElement root, string property) =>
        root.TryGetProperty(property, out var value) && value.ValueKind != JsonValueKind.Null ? value.GetRawText() : null;

    private static int GetInt(JsonElement root, string property) =>
        root.TryGetProperty(property, out var value) && value.TryGetInt32(out var result) ? result : 0;

    private static long GetLong(JsonElement root, string property) =>
        root.TryGetProperty(property, out var value) && value.TryGetInt64(out var result) ? result : 0;

    private static double GetDouble(JsonElement root, string property) =>
        root.TryGetProperty(property, out var value) && value.TryGetDouble(out var result) ? result : 0;

    private static DateTime? GetDateTime(JsonElement root, string property) =>
        root.TryGetProperty(property, out var value) && value.TryGetDateTime(out var result) ? result : null;

    private static DateTimeOffset? GetDateTimeOffset(JsonElement root, string property) =>
        root.TryGetProperty(property, out var value) && value.TryGetDateTimeOffset(out var result) ? result : null;

    private static bool IsTerminal(string type) => type is "completed" or "failed" or "cancelled" or "timed_out";

    private static long ElapsedMilliseconds(DateTime startedAt) => (long)(DateTime.UtcNow - startedAt).TotalMilliseconds;

    private static InvokeOptions? MergeOptions(InvokeOptions? options, string? idempotencyKey)
    {
        if (string.IsNullOrWhiteSpace(idempotencyKey))
        {
            return options;
        }
        return new InvokeOptions
        {
            GameId = options?.GameId,
            Env = options?.Env,
            TimeoutSeconds = options?.TimeoutSeconds ?? 30,
            RequestId = options?.RequestId,
            UserId = options?.UserId,
            Metadata = options?.Metadata,
            IdempotencyKey = idempotencyKey,
        };
    }

    private void ThrowIfDisposed()
    {
        if (_isDisposed)
        {
            throw new ObjectDisposedException(nameof(CroupierInvoker));
        }
    }
}

/// <summary>批量调用请求。</summary>
public sealed class BatchInvokeRequest
{
    /// <summary>函数 ID。</summary>
    public required string FunctionId { get; init; }

    /// <summary>JSON 请求负载。</summary>
    public required string Payload { get; init; }

    /// <summary>可选幂等键。</summary>
    public string? IdempotencyKey { get; init; }
}

/// <summary>任务当前状态。</summary>
public sealed class TaskStatus
{
    /// <summary>任务 ID。</summary>
    public required string TaskId { get; init; }

    /// <summary>任务状态。</summary>
    public required string Status { get; init; }

    /// <summary>进度。</summary>
    public double Progress { get; init; }

    /// <summary>状态消息。</summary>
    public string? Message { get; init; }

    /// <summary>错误信息。</summary>
    public string? Error { get; init; }

    /// <summary>结果 JSON。</summary>
    public string? Result { get; init; }

    /// <summary>开始时间。</summary>
    public DateTime? StartTime { get; init; }

    /// <summary>结束时间。</summary>
    public DateTime? EndTime { get; init; }
}
