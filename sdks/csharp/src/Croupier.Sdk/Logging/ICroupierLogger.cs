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

using Microsoft.Extensions.Logging;

namespace Croupier.Sdk.Logging;

/// <summary>
/// Croupier SDK 日志接口
/// </summary>
public interface ICroupierLogger
{
    /// <summary>
    /// 记录调试日志
    /// </summary>
    void LogDebug(string component, string message);

    /// <summary>
    /// 记录信息日志
    /// </summary>
    void LogInfo(string component, string message);

    /// <summary>
    /// 记录警告日志
    /// </summary>
    void LogWarning(string component, string message);

    /// <summary>
    /// 记录错误日志
    /// </summary>
    void LogError(string component, string message, Exception? exception = null);
}

/// <summary>
/// 基于 ILogger 的 Croupier 日志实现
/// </summary>
public class CroupierLogger : ICroupierLogger
{
    private readonly ILogger _logger;

    public CroupierLogger(ILogger logger)
    {
        _logger = logger ?? throw new ArgumentNullException(nameof(logger));
    }

    public void LogDebug(string component, string message)
    {
        _logger.LogDebug("[{Component}] {Message}", component, message);
    }

    public void LogInfo(string component, string message)
    {
        _logger.LogInformation("[{Component}] {Message}", component, message);
    }

    public void LogWarning(string component, string message)
    {
        _logger.LogWarning("[{Component}] {Message}", component, message);
    }

    public void LogError(string component, string message, Exception? exception = null)
    {
        if (exception != null)
            _logger.LogError(exception, "[{Component}] {Message}", component, message);
        else
            _logger.LogError("[{Component}] {Message}", component, message);
    }
}

/// <summary>
/// 控制台日志实现
/// </summary>
public class ConsoleCroupierLogger : ICroupierLogger
{
    private readonly string _prefix;

    public ConsoleCroupierLogger(string prefix = "Croupier")
    {
        _prefix = prefix;
    }

    public void LogDebug(string component, string message)
    {
        Console.WriteLine($"[{_prefix}] [DEBUG] [{component}] {message}");
    }

    public void LogInfo(string component, string message)
    {
        Console.WriteLine($"[{_prefix}] [INFO] [{component}] {message}");
    }

    public void LogWarning(string component, string message)
    {
        Console.WriteLine($"[{_prefix}] [WARN] [{component}] {message}");
    }

    public void LogError(string component, string message, Exception? exception = null)
    {
        Console.Error.WriteLine($"[{_prefix}] [ERROR] [{component}] {message}");
        if (exception != null)
        {
            Console.Error.WriteLine($"  Exception: {exception.Message}");
            Console.Error.WriteLine($"  StackTrace: {exception.StackTrace}");
        }
    }
}
