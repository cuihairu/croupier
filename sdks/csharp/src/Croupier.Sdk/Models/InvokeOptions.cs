// Copyright 2025 Croupier Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

namespace Croupier.Sdk.Models;

/// <summary>
/// 函数调用选项
/// </summary>
public class InvokeOptions
{
    /// <summary>
    /// 游戏 ID
    /// </summary>
    public string? GameId { get; set; }

    /// <summary>
    /// 环境
    /// </summary>
    public string? Env { get; set; }

    /// <summary>
    /// 超时时间（秒）
    /// </summary>
    public int TimeoutSeconds { get; set; } = 30;

    /// <summary>
    /// 幂等性键（用于防止重复执行）
    /// </summary>
    public string? IdempotencyKey { get; set; }

    /// <summary>
    /// 请求 ID（用于追踪）
    /// </summary>
    public string? RequestId { get; set; }

    /// <summary>
    /// 用户 ID（用于审计）
    /// </summary>
    public string? UserId { get; set; }

    /// <summary>
    /// 自定义元数据
    /// </summary>
    public Dictionary<string, string>? Metadata { get; set; }
}

/// <summary>
/// 函数调用上下文
/// </summary>
public class FunctionContext
{
    /// <summary>
    /// 函数 ID
    /// </summary>
    public required string FunctionId { get; init; }

    /// <summary>
    /// 调用 ID
    /// </summary>
    public required string CallId { get; init; }

    /// <summary>
    /// 游戏 ID
    /// </summary>
    public required string GameId { get; init; }

    /// <summary>
    /// 环境
    /// </summary>
    public required string Env { get; init; }

    /// <summary>
    /// 用户 ID
    /// </summary>
    public string? UserId { get; init; }

    /// <summary>
    /// 请求时间戳
    /// </summary>
    public long Timestamp { get; init; }

    /// <summary>
    /// 幂等性键
    /// </summary>
    public string? IdempotencyKey { get; init; }

    /// <summary>
    /// 调用方服务 ID
    /// </summary>
    public string? CallerServiceId { get; init; }
}

/// <summary>
/// 函数调用结果
/// </summary>
public class InvokeResult
{
    /// <summary>
    /// 是否成功
    /// </summary>
    public required bool Success { get; init; }

    /// <summary>
    /// 响应数据
    /// </summary>
    public string? Data { get; init; }

    /// <summary>
    /// 错误信息
    /// </summary>
    public string? Error { get; init; }

    /// <summary>
    /// 错误码
    /// </summary>
    public string? ErrorCode { get; init; }

    /// <summary>
    /// 执行时长（毫秒）
    /// </summary>
    public long DurationMs { get; init; }

    /// <summary>
    /// 创建成功结果
    /// </summary>
    public static InvokeResult Succeeded(string data, long durationMs = 0) =>
        new() { Success = true, Data = data, DurationMs = durationMs };

    /// <summary>
    /// 创建失败结果
    /// </summary>
    public static InvokeResult Failed(string error, string? errorCode = null, long durationMs = 0) =>
        new() { Success = false, Error = error, ErrorCode = errorCode, DurationMs = durationMs };
}
