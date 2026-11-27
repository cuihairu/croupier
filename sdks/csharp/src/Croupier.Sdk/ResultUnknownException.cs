// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

using System.Net;

namespace Croupier.Sdk;

/// <summary>
/// AUD-068：写请求的最终失败无法确定结果——请求可能在响应丢失前已在服务端生效。
/// 调用方应把该结果视为“未知”，必要时通过业务幂等键或对账接口确认。
/// </summary>
public sealed class ResultUnknownException : HttpRequestException
{
    /// <summary>创建写结果未知异常，保留原错误与 HTTP 状态码（无响应时为空）。</summary>
    public ResultUnknownException(string message, Exception? innerException = null, HttpStatusCode? statusCode = null)
        : base("result unknown after write: " + message, innerException, statusCode)
    {
    }
}
