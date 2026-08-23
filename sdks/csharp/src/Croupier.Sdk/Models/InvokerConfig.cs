// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

namespace Croupier.Sdk.Models;

/// <summary>
/// L3 调用方配置。它独立于 Provider 的 <see cref="ClientConfig"/>，并指向 Server HTTP API。
/// </summary>
public sealed class InvokerConfig
{
    /// <summary>
    /// Server REST API 基地址，例如 <c>https://server.example/api/v1</c>。
    /// </summary>
    public string ServerBaseUrl { get; set; } = "http://127.0.0.1:18780/api/v1";

    /// <summary>
    /// 可选 Bearer Token。
    /// </summary>
    public string? AuthToken { get; set; }

    /// <summary>
    /// 默认游戏作用域。
    /// </summary>
    public string? GameId { get; set; }

    /// <summary>
    /// 默认环境作用域。
    /// </summary>
    public string? Env { get; set; }

    /// <summary>
    /// 单次 HTTP 请求超时（秒）。
    /// </summary>
    public int TimeoutSeconds { get; set; } = 30;

    /// <summary>
    /// 任务事件轮询间隔（毫秒）。
    /// </summary>
    public int TaskPollIntervalMilliseconds { get; set; } = 500;

    /// <summary>
    /// 附加到每个调用方请求的 HTTP 头。
    /// </summary>
    public Dictionary<string, string> Headers { get; set; } = new(StringComparer.OrdinalIgnoreCase);

    /// <summary>
    /// 可重试失败的自动重试配置（与 Go/Java SDK 对齐）。
    /// </summary>
    public RetryConfig? Retry { get; set; }
}

/// <summary>
/// Server 返回的任务事件。
/// </summary>
public sealed class TaskEvent
{
    /// <summary>事件序号。</summary>
    public long Seq { get; init; }

    /// <summary>事件类型。</summary>
    public required string Type { get; init; }

    /// <summary>进度。</summary>
    public int Progress { get; init; }

    /// <summary>事件消息。</summary>
    public string? Message { get; init; }

    /// <summary>事件负载的 JSON 文本。</summary>
    public string? Payload { get; init; }

    /// <summary>创建时间。</summary>
    public DateTimeOffset? CreatedAt { get; init; }
}
