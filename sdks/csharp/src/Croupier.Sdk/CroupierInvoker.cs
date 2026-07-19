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

using Croupier.Sdk.Logging;
using Croupier.Sdk.Models;
using Croupier.Sdk.Transport;
using Croupier.Sdk.V1;
using Microsoft.Extensions.Logging;

namespace Croupier.Sdk;

/// <summary>
/// Croupier 调用器 - 用于调用远程注册的函数
/// </summary>
public class CroupierInvoker : IDisposable
{
    private readonly string _agentAddr;
    private readonly string _gameId;
    private readonly string _env;
    private readonly ICroupierLogger _logger;
    private readonly int _timeoutMs;
    private readonly int _connectTimeoutMs;

    private TCPTransport? _transport;
    private bool _isDisposed;

    /// <summary>
    /// Agent 地址
    /// </summary>
    public string AgentAddr => _agentAddr;

    /// <summary>
    /// 游戏 ID
    /// </summary>
    public string GameId => _gameId;

    /// <summary>
    /// 环境
    /// </summary>
    public string Env => _env;

    /// <summary>
    /// 创建调用器实例
    /// </summary>
    /// <param name="agentAddr">Agent 地址 (e.g., "tcp://127.0.0.1:19090")</param>
    /// <param name="gameId">游戏 ID</param>
    /// <param name="env">环境</param>
    /// <param name="timeoutMs">请求超时时间（毫秒）</param>
    /// <param name="connectTimeoutMs">连接超时时间（毫秒）</param>
    /// <param name="logger">日志记录器</param>
    public CroupierInvoker(
        string agentAddr = "tcp://127.0.0.1:19090",
        string? gameId = null,
        string? env = null,
        int timeoutMs = 5000,
        int connectTimeoutMs = 5000,
        ICroupierLogger? logger = null)
    {
        _agentAddr = agentAddr;
        _gameId = gameId ?? "default-game";
        _env = env ?? "dev";
        _timeoutMs = timeoutMs;
        _connectTimeoutMs = connectTimeoutMs;
        _logger = logger ?? new ConsoleCroupierLogger("Invoker");

        _logger.LogInfo("CroupierInvoker", $"Invoker created for {_gameId}/{_env}");
    }

    /// <summary>
    /// 从客户端配置创建调用器
    /// </summary>
    /// <param name="config">客户端配置</param>
    /// <param name="logger">日志记录器</param>
    public CroupierInvoker(ClientConfig config, ICroupierLogger? logger = null)
        : this(
            config.AgentAddr,
            config.GameId,
            config.Env,
            config.TimeoutSeconds * 1000,
            config.ConnectTimeoutSeconds * 1000,
            logger)
    {
    }

    /// <summary>
    /// 创建带 ILogger 的调用器
    /// </summary>
    /// <param name="agentAddr">Agent 地址</param>
    /// <param name="gameId">游戏 ID</param>
    /// <param name="env">环境</param>
    /// <param name="logger">Microsoft ILogger</param>
    public CroupierInvoker(
        string agentAddr,
        string gameId,
        string env,
        ILogger logger)
        : this(agentAddr, gameId, env, 5000, 5000, new CroupierLogger(logger))
    {
    }

    /// <summary>
    /// 调用远程函数
    /// </summary>
    /// <param name="functionId">函数 ID（格式: category.entity.operation）</param>
    /// <param name="payload">请求负载（JSON 字符串）</param>
    /// <param name="options">调用选项</param>
    /// <param name="cancellationToken">取消令牌</param>
    /// <returns>调用结果</returns>
    public async Task<InvokeResult> InvokeAsync(
        string functionId,
        string payload,
        InvokeOptions? options = null,
        CancellationToken cancellationToken = default)
    {
        ThrowIfDisposed();

        if (string.IsNullOrWhiteSpace(functionId))
            throw new ArgumentException("Function ID cannot be empty", nameof(functionId));

        if (payload == null)
            throw new ArgumentNullException(nameof(payload));

        options ??= new InvokeOptions
        {
            GameId = _gameId,
            Env = _env
        };

        var startTime = DateTime.UtcNow;

        try
        {
            _logger.LogDebug("CroupierInvoker", $"Invoking {functionId}");

            // Ensure transport is connected
            EnsureTransportConnected();

            // Build protobuf request
            var request = new InvokeRequest
            {
                FunctionId = functionId,
                Payload = Google.Protobuf.ByteString.CopyFromUtf8(payload)
            };

            if (!string.IsNullOrEmpty(options.IdempotencyKey))
            {
                request.IdempotencyKey = options.IdempotencyKey;
            }

            if (options.Metadata != null)
            {
                foreach (var kvp in options.Metadata)
                {
                    request.Metadata.Add(kvp.Key, kvp.Value);
                }
            }

            // Serialize and send via TCP
            var requestData = request.ToByteArray();
            var responseData = await _transport!.CallAsync(
                Protocol.MsgInvokeRequest,
                requestData,
                cancellationToken);

            // Parse response
            var response = InvokeResponse.Parser.ParseFrom(responseData);
            var resultPayload = response.Payload.ToStringUtf8();

            var duration = (long)(DateTime.UtcNow - startTime).TotalMilliseconds;

            _logger.LogDebug("CroupierInvoker", $"Invoke {functionId} completed ({duration}ms)");

            return InvokeResult.Succeeded(resultPayload, duration);
        }
        catch (OperationCanceledException)
        {
            var duration = (long)(DateTime.UtcNow - startTime).TotalMilliseconds;
            _logger.LogWarning("CroupierInvoker", $"Invoke {functionId} canceled");
            return InvokeResult.Failed("Operation canceled", "CANCELED", duration);
        }
        catch (Exception ex)
        {
            var duration = (long)(DateTime.UtcNow - startTime).TotalMilliseconds;
            _logger.LogError("CroupierInvoker", $"Invoke {functionId} failed: {ex.Message}", ex);
            return InvokeResult.Failed(ex.Message, null, duration);
        }
    }

    /// <summary>
    /// Ensure transport is connected.
    /// </summary>
    private void EnsureTransportConnected()
    {
        if (_transport == null)
        {
            _transport = new TCPTransport(_agentAddr, _timeoutMs, _connectTimeoutMs, _logger);
            _transport.Connect();
        }
        else if (!_transport.IsConnected)
        {
            _transport.Connect();
        }
    }

    /// <summary>
    /// 批量调用多个函数
    /// </summary>
    /// <param name="requests">请求列表</param>
    /// <param name="options">调用选项</param>
    /// <param name="cancellationToken">取消令牌</param>
    /// <returns>调用结果列表</returns>
    public async Task<List<InvokeResult>> BatchInvokeAsync(
        List<BatchInvokeRequest> requests,
        InvokeOptions? options = null,
        CancellationToken cancellationToken = default)
    {
        ThrowIfDisposed();

        if (requests == null)
            throw new ArgumentNullException(nameof(requests));

        if (requests.Count == 0)
            throw new ArgumentException("Requests list cannot be empty", nameof(requests));

        _logger.LogDebug("CroupierInvoker", $"Batch invoking {requests.Count} functions");

        var results = new List<InvokeResult>();
        var tasks = requests.Select(r =>
            InvokeAsync(r.FunctionId, r.Payload, options, cancellationToken));

        var invokeResults = await Task.WhenAll(tasks);
        results.AddRange(invokeResults);

        return results;
    }

    /// <summary>
    /// 启动异步任务（不需要等待响应的调用）
    /// </summary>
    /// <param name="functionId">函数 ID</param>
    /// <param name="payload">请求负载</param>
    /// <param name="options">调用选项</param>
    /// <returns>任务 ID</returns>
    public async Task<string> StartTaskAsync(
        string functionId,
        string payload,
        InvokeOptions? options = null)
    {
        ThrowIfDisposed();

        if (string.IsNullOrWhiteSpace(functionId))
            throw new ArgumentException("Function ID cannot be empty", nameof(functionId));

        if (payload == null)
            throw new ArgumentNullException(nameof(payload));

        options ??= new InvokeOptions
        {
            GameId = _gameId,
            Env = _env
        };

        _logger.LogDebug("CroupierInvoker", $"Starting task: {functionId}");

        // Ensure transport is connected
        EnsureTransportConnected();

        // Build protobuf request
        var request = new InvokeRequest
        {
            FunctionId = functionId,
            Payload = Google.Protobuf.ByteString.CopyFromUtf8(payload)
        };

        if (!string.IsNullOrEmpty(options.IdempotencyKey))
        {
            request.IdempotencyKey = options.IdempotencyKey;
        }

        // Send via TCP
        var requestData = request.ToByteArray();
        var responseData = await _transport!.CallAsync(
            Protocol.MsgStartTaskRequest,
            requestData);

        // Parse response
        var response = StartTaskResponse.Parser.ParseFrom(responseData);
        _logger.LogInfo("CroupierInvoker", $"Task started: {response.TaskId}");

        return response.TaskId;
    }

    /// <summary>
    /// 取消正在运行的任务
    /// </summary>
    /// <param name="taskId">任务 ID</param>
    /// <param name="cancellationToken">取消令牌</param>
    /// <returns>是否成功取消</returns>
    public async Task<bool> CancelTaskAsync(
        string taskId,
        CancellationToken cancellationToken = default)
    {
        ThrowIfDisposed();

        if (string.IsNullOrWhiteSpace(taskId))
            throw new ArgumentException("Task ID cannot be empty", nameof(taskId));

        _logger.LogDebug("CroupierInvoker", $"Canceling task: {taskId}");

        // Ensure transport is connected
        EnsureTransportConnected();

        // Build protobuf request
        var request = new CancelTaskRequest
        {
            TaskId = taskId
        };

        // Send via TCP
        var requestData = request.ToByteArray();
        await _transport!.CallAsync(
            Protocol.MsgCancelTaskRequest,
            requestData,
            cancellationToken);

        _logger.LogInfo("CroupierInvoker", $"Task canceled: {taskId}");
        return true;
    }

    /// <summary>
    /// 获取任务状态
    /// </summary>
    /// <param name="taskId">任务 ID</param>
    /// <param name="cancellationToken">取消令牌</param>
    /// <returns>任务状态</returns>
    public async Task<TaskStatus?> GetTaskStatusAsync(
        string taskId,
        CancellationToken cancellationToken = default)
    {
        ThrowIfDisposed();

        if (string.IsNullOrWhiteSpace(taskId))
            throw new ArgumentException("Task ID cannot be empty", nameof(taskId));

        // Ensure transport is connected
        EnsureTransportConnected();

        // Build protobuf request
        var request = new TaskStreamRequest
        {
            TaskId = taskId
        };

        // Send via TCP
        var requestData = request.ToByteArray();
        var responseData = await _transport!.CallAsync(
            Protocol.MsgStreamTaskRequest,
            requestData,
            cancellationToken);

        // Parse response
        var taskEvent = TaskEvent.Parser.ParseFrom(responseData);
        var normalizedStatus = NormalizeTaskEventType(taskEvent.Type, taskEvent.Message);

        return new TaskStatus
        {
            TaskId = taskId,
            Status = normalizedStatus,
            Progress = taskEvent.Progress,
            Message = taskEvent.Message,
            Result = taskEvent.Payload.Length > 0 ? taskEvent.Payload.ToStringUtf8() : null
        };
    }

    private static string NormalizeTaskEventType(string type, string? message)
    {
        if (string.Equals(type, "done", StringComparison.OrdinalIgnoreCase))
        {
            return "completed";
        }

        if (string.Equals(type, "error", StringComparison.OrdinalIgnoreCase) &&
            !string.IsNullOrWhiteSpace(message) &&
            message.Contains("cancel", StringComparison.OrdinalIgnoreCase))
        {
            return "cancelled";
        }

        return type.ToLowerInvariant();
    }

    private void ThrowIfDisposed()
    {
        if (_isDisposed)
            throw new ObjectDisposedException(nameof(CroupierInvoker));
    }

    /// <summary>
    /// 释放资源
    /// </summary>
    public void Dispose()
    {
        if (_isDisposed)
            return;

        _logger.LogInfo("CroupierInvoker", "Disposing...");

        _transport?.Dispose();
        _transport = null;

        _isDisposed = true;
        GC.SuppressFinalize(this);
    }
}

/// <summary>
/// 批量调用请求
/// </summary>
public class BatchInvokeRequest
{
    /// <summary>
    /// 函数 ID
    /// </summary>
    public required string FunctionId { get; init; }

    /// <summary>
    /// 请求负载
    /// </summary>
    public required string Payload { get; init; }

    /// <summary>
    /// 幂等性键（可选）
    /// </summary>
    public string? IdempotencyKey { get; init; }
}

/// <summary>
/// 任务状态
/// </summary>
public class TaskStatus
{
    /// <summary>
    /// 任务 ID
    /// </summary>
    public required string TaskId { get; init; }

    /// <summary>
    /// 状态: pending, running, completed, failed, canceled
    /// </summary>
    public required string Status { get; init; }

    /// <summary>
    /// 进度 (0.0 - 1.0)
    /// </summary>
    public double Progress { get; init; }

    /// <summary>
    /// 消息
    /// </summary>
    public string? Message { get; init; }

    /// <summary>
    /// 错误信息（如果失败）
    /// </summary>
    public string? Error { get; init; }

    /// <summary>
    /// 结果数据（如果完成）
    /// </summary>
    public string? Result { get; init; }

    /// <summary>
    /// 开始时间
    /// </summary>
    public DateTime? StartTime { get; init; }

    /// <summary>
    /// 结束时间
    /// </summary>
    public DateTime? EndTime { get; init; }
}
