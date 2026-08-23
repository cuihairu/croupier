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
/// 自动重试配置（与 Go/Java SDK 的 RetryConfig 对齐）。
/// </summary>
public sealed class RetryConfig
{
    /// <summary>是否启用重试。默认 true。</summary>
    public bool Enabled { get; set; } = true;

    /// <summary>含首次请求在内的最大尝试次数。默认 3。</summary>
    public int MaxAttempts { get; set; } = 3;

    /// <summary>首次重试前的延迟（毫秒）。默认 100。</summary>
    public int InitialDelayMs { get; set; } = 100;

    /// <summary>延迟上限（毫秒）。默认 5000。</summary>
    public int MaxDelayMs { get; set; } = 5000;

    /// <summary>指数退避倍数。默认 2.0。</summary>
    public double BackoffMultiplier { get; set; } = 2.0;

    /// <summary>抖动因子 [0,1]。默认 0.1。</summary>
    public double JitterFactor { get; set; } = 0.1;

    /// <summary>额外触发重试的 HTTP 状态码（默认 429 与 5xx 总是可重试）。</summary>
    public IReadOnlyList<int> RetryableStatusCodes { get; set; } = Array.Empty<int>();

    /// <summary>返回填充默认值后的规范化副本。</summary>
    public RetryConfig Normalized()
    {
        var clone = (RetryConfig)MemberwiseClone();
        if (clone.MaxAttempts < 1) clone.MaxAttempts = 1;
        if (clone.BackoffMultiplier <= 0) clone.BackoffMultiplier = 2.0;
        if (clone.InitialDelayMs < 0) clone.InitialDelayMs = 0;
        if (clone.MaxDelayMs < 0) clone.MaxDelayMs = 0;
        return clone;
    }

    /// <summary>判断状态码是否可重试。</summary>
    public bool IsRetryableStatus(int statusCode) =>
        statusCode == 429 || statusCode >= 500 || RetryableStatusCodes.Contains(statusCode);

    /// <summary>计算第 attempt 次重试（0 起）前的延迟（毫秒）。</summary>
    public double DelayMs(int attempt)
    {
        var normalized = Normalized();
        double delay = normalized.InitialDelayMs * Math.Pow(normalized.BackoffMultiplier, attempt);
        if (normalized.MaxDelayMs > 0)
        {
            delay = Math.Min(delay, normalized.MaxDelayMs);
        }
        if (normalized.JitterFactor > 0)
        {
            delay += delay * normalized.JitterFactor * (Random.Shared.NextDouble() * 2 - 1);
        }
        return Math.Max(0, delay);
    }
}
