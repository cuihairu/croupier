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

using System.Collections.Concurrent;
using System.IO.Compression;
using System.Net;
using System.Net.Sockets;
using System.Text.Json;
using System.Threading.Channels;
using Croupier.Sdk.Logging;
using Croupier.Sdk.Models;
using Croupier.Sdk.Transport;
using Croupier.Sdk.V1;
using Microsoft.Extensions.Logging;

namespace Croupier.Sdk;

/// <summary>
/// Croupier 客户端 - 用于连接 Agent 并注册/调用函数
/// </summary>
public partial class CroupierClient : IDisposable
{
    private readonly ClientConfig _config;
    private readonly ICroupierLogger _logger;
    private readonly ConcurrentDictionary<string, IFunctionHandler> _handlers;
    private readonly ConcurrentDictionary<string, FunctionDescriptor> _descriptors;
    private readonly CancellationTokenSource _shutdownCts;
    private readonly Channel<FunctionCallTask> _callChannel;
    private readonly Func<string, int, int, ICroupierLogger, IClientTransport> _transportFactory;

    private IClientTransport? _transport;
    private bool _isConnected;
    private bool _isDisposed;
    private Task? _processTask;
    private CancellationTokenSource? _heartbeatCts;
    private Task? _heartbeatTask;
    private string _sessionId = string.Empty;

    /// <summary>
    /// 客户端配置
    /// </summary>
    public ClientConfig Config => _config;

    /// <summary>
    /// 是否已连接
    /// </summary>
    public bool IsConnected => _isConnected && _transport != null;

    /// <summary>
    /// 获取当前会话 ID
    /// </summary>
    public string SessionId => _sessionId;

    /// <summary>
    /// 创建 Croupier 客户端实例
    /// </summary>
    /// <param name="config">客户端配置</param>
    /// <param name="logger">日志记录器（可选）</param>
    public CroupierClient(ClientConfig? config = null, ICroupierLogger? logger = null)
        : this(
            config,
            logger,
            static (address, timeoutMs, connectTimeoutMs, transportLogger) => new TCPTransport(address, timeoutMs, connectTimeoutMs, transportLogger))
    {
    }

    internal CroupierClient(
        ClientConfig? config,
        ICroupierLogger? logger,
        Func<string, int, int, ICroupierLogger, IClientTransport> transportFactory)
    {
        _config = config ?? new ClientConfig();
        _logger = logger ?? new ConsoleCroupierLogger();
        _handlers = new ConcurrentDictionary<string, IFunctionHandler>();
        _descriptors = new ConcurrentDictionary<string, FunctionDescriptor>();
        _shutdownCts = new CancellationTokenSource();
        _callChannel = Channel.CreateUnbounded<FunctionCallTask>();
        _transportFactory = transportFactory;

        _logger.LogInfo("CroupierClient", $"Client created for service: {_config.ServiceId}");
    }

    /// <summary>
    /// 创建带 ILogger 的 Croupier 客户端实例
    /// </summary>
    /// <param name="config">客户端配置</param>
    /// <param name="logger">Microsoft ILogger</param>
    public CroupierClient(ClientConfig config, ILogger logger)
        : this(
            config,
            new CroupierLogger(logger),
            static (address, timeoutMs, connectTimeoutMs, transportLogger) => new TCPTransport(address, timeoutMs, connectTimeoutMs, transportLogger))
    {
    }

    /// <summary>
    /// 连接到 Agent
    /// </summary>
    /// <exception cref="InvalidOperationException">连接失败时抛出</exception>
    public async Task ConnectAsync(CancellationToken cancellationToken = default)
    {
        ThrowIfDisposed();

        if (_isConnected)
        {
            _logger.LogInfo("CroupierClient", "Already connected");
            return;
        }

        _logger.LogInfo("CroupierClient", $"Connecting to Agent at {_config.AgentAddr}...");

        try
        {
            await ConnectAndRegisterAsync(cancellationToken);

            _isConnected = true;

            _logger.LogInfo("CroupierClient", "Connected successfully");

            // Start message processing loop
            _processTask = Task.Run(() => ProcessCallsAsync(_shutdownCts.Token), cancellationToken);
            StartHeartbeatLoop();

            await Task.CompletedTask;
        }
        catch (Exception ex)
        {
            _logger.LogError("CroupierClient", $"Failed to connect: {ex.Message}", ex);
            throw;
        }
    }

    /// <summary>
    /// 断开连接
    /// </summary>
    public void Disconnect()
    {
        ThrowIfDisposed();

        if (!_isConnected)
            return;

        _logger.LogInfo("CroupierClient", "Disconnecting...");

        // First, cancel the shutdown token and dispose transport to interrupt ReadLoop
        _shutdownCts.Cancel();
        _heartbeatCts?.Cancel();
        _isConnected = false;

        // Dispose transport first to interrupt any pending ReadAsync calls
        _transport?.Dispose();
        _transport = null;

        // Then wait for tasks with a shorter timeout since transport is already disposed
        try
        {
            _processTask?.Wait(TimeSpan.FromSeconds(2));
            _heartbeatTask?.Wait(TimeSpan.FromSeconds(2));
        }
        catch (OperationCanceledException)
        {
            // Expected
        }
        catch (AggregateException)
        {
            // Task may have faulted
        }

        _heartbeatCts?.Dispose();
        _heartbeatCts = null;
        _heartbeatTask = null;
        _sessionId = string.Empty;

        _logger.LogInfo("CroupierClient", "Disconnected");
    }

    /// <summary>
    /// 注册函数处理器
    /// </summary>
    /// <param name="descriptor">函数描述符</param>
    /// <param name="handler">函数处理器</param>
    /// <exception cref="ArgumentException">描述符无效时抛出</exception>
    public void RegisterFunction(FunctionDescriptor descriptor, IFunctionHandler handler)
    {
        ThrowIfDisposed();

        if (!descriptor.IsValid())
            throw new ArgumentException("Invalid function descriptor", nameof(descriptor));

        var functionId = descriptor.Id;

        if (!_handlers.TryAdd(functionId, handler))
        {
            _logger.LogWarning("CroupierClient", $"Function {functionId} already registered, replacing");
            _handlers[functionId] = handler;
        }

        _descriptors[functionId] = descriptor;

        _logger.LogInfo("CroupierClient", $"Registered function: {functionId} (version: {descriptor.Version})");
    }

    /// <summary>
    /// 注册函数处理器（委托版本）
    /// </summary>
    /// <param name="descriptor">函数描述符</param>
    /// <param name="handler">函数处理器委托</param>
    public void RegisterFunction(FunctionDescriptor descriptor, FunctionHandlerDelegate handler)
    {
        RegisterFunction(descriptor, new DelegateFunctionHandler(handler));
    }

    /// <summary>
    /// 注册函数处理器（同步委托版本）
    /// </summary>
    /// <param name="descriptor">函数描述符</param>
    /// <param name="handler">同步函数处理器委托</param>
    public void RegisterFunction(FunctionDescriptor descriptor, SyncFunctionHandlerDelegate handler)
    {
        RegisterFunction(descriptor, new SyncDelegateFunctionHandler(handler));
    }

    /// <summary>
    /// 取消注册函数
    /// </summary>
    /// <param name="functionId">函数 ID</param>
    /// <returns>是否成功取消注册</returns>
    public bool UnregisterFunction(string functionId)
    {
        ThrowIfDisposed();

        if (_handlers.TryRemove(functionId, out _))
        {
            _descriptors.TryRemove(functionId, out _);
            _logger.LogInfo("CroupierClient", $"Unregistered function: {functionId}");
            return true;
        }

        return false;
    }

    /// <summary>
    /// 启动服务（开始接收函数调用）
    /// </summary>
    /// <param name="cancellationToken">取消令牌</param>
    public async Task ServeAsync(CancellationToken cancellationToken = default)
    {
        ThrowIfDisposed();

        if (!_isConnected)
            await ConnectAsync(cancellationToken);

        _logger.LogInfo("CroupierClient", "Starting service...");

        // Wait for shutdown signal
        await Task.Delay(Timeout.Infinite, cancellationToken);

        _logger.LogInfo("CroupierClient", "Service stopped");
    }

    /// <summary>
    /// 调用远程函数
    /// </summary>
    /// <param name="functionId">函数 ID</param>
    /// <param name="payload">请求负载（JSON）</param>
    /// <param name="options">调用选项</param>
    /// <param name="cancellationToken">取消令牌</param>
    /// <returns>响应负载（JSON）</returns>
    public async Task<string> InvokeAsync(
        string functionId,
        string payload,
        InvokeOptions? options = null,
        CancellationToken cancellationToken = default)
    {
        ThrowIfDisposed();

        options ??= new InvokeOptions
        {
            GameId = _config.GameId,
            Env = _config.Env
        };

        _logger.LogDebug("CroupierInvoker", $"Invoking {functionId}");

        if (_transport == null || !_transport.IsConnected)
        {
            throw new InvalidOperationException("Not connected to Agent");
        }

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

        foreach (var kvp in BuildInvocationMetadata(options))
        {
            request.Metadata.Add(kvp.Key, kvp.Value);
        }

        // Send via TCP
        var requestData = request.ToByteArray();
        var responseData = await _transport.CallAsync(
            Protocol.MsgInvokeRequest,
            requestData,
            cancellationToken);

        // Parse response
        var response = InvokeResponse.Parser.ParseFrom(responseData);
        return response.Payload.ToStringUtf8();
    }

    private Dictionary<string, string> BuildInvocationMetadata(InvokeOptions options)
    {
        var metadata = new Dictionary<string, string>(_config.Headers, StringComparer.Ordinal);

        if (options.Metadata != null)
        {
            foreach (var kvp in options.Metadata)
            {
                metadata[kvp.Key] = kvp.Value;
            }
        }

        if (!string.IsNullOrWhiteSpace(options.RequestId))
        {
            metadata["X-Request-ID"] = options.RequestId;
        }

        if (!string.IsNullOrWhiteSpace(options.GameId))
        {
            metadata["X-Game-ID"] = options.GameId;
        }
        else if (!string.IsNullOrWhiteSpace(_config.GameId) && !metadata.ContainsKey("X-Game-ID"))
        {
            metadata["X-Game-ID"] = _config.GameId;
        }

        if (!string.IsNullOrWhiteSpace(options.Env))
        {
            metadata["X-Env"] = options.Env;
        }
        else if (!string.IsNullOrWhiteSpace(_config.Env) && !metadata.ContainsKey("X-Env"))
        {
            metadata["X-Env"] = _config.Env;
        }

        if (!string.IsNullOrWhiteSpace(_config.AuthToken) && !metadata.ContainsKey("Authorization"))
        {
            metadata["Authorization"] = $"Bearer {_config.AuthToken}";
        }

        return metadata;
    }

    /// <summary>
    /// 停止服务
    /// </summary>
    public void Stop()
    {
        _logger.LogInfo("CroupierClient", "Stopping service...");
        _shutdownCts.Cancel();
    }

    /// <summary>
    /// 处理函数调用
    /// </summary>
    private async Task<string> ProcessFunctionCallAsync(FunctionCallTask task)
    {
        var startTime = DateTime.UtcNow;

        try
        {
            _logger.LogDebug("CroupierClient", $"Processing call: {task.FunctionId}");

            if (!_handlers.TryGetValue(task.FunctionId, out var handler))
            {
                return $"{{\"error\":\"Function not found: {task.FunctionId}\"}}";
            }

            var context = new FunctionContext
            {
                FunctionId = task.FunctionId,
                CallId = task.CallId,
                GameId = task.GameId,
                Env = task.Env,
                UserId = task.UserId,
                Timestamp = startTime.Ticks,
                IdempotencyKey = task.IdempotencyKey,
                CallerServiceId = task.CallerServiceId
            };

            var result = await handler.HandleAsync(context, task.Payload);

            var duration = (long)(DateTime.UtcNow - startTime).TotalMilliseconds;
            _logger.LogDebug("CroupierClient", $"Call completed: {task.FunctionId} ({duration}ms)");

            return result;
        }
        catch (Exception ex)
        {
            _logger.LogError("CroupierClient", $"Call failed: {task.FunctionId} - {ex.Message}", ex);
            return $"{{\"error\":\"{ex.Message}\"}}";
        }
    }

    /// <summary>
    /// 处理来自Agent的入站请求（InvokeRequest）
    /// </summary>
    private async Task<byte[]> HandleInboundRequestAsync(int msgId, int reqId, byte[] body)
    {
        if (msgId == Protocol.MsgInvokeRequest)
        {
            var request = InvokeRequest.Parser.ParseFrom(body);
            var payload = request.Payload.ToStringUtf8();

            // Extract metadata
            request.Metadata.TryGetValue("X-Game-ID", out var gameId);
            request.Metadata.TryGetValue("X-Env", out var env);
            request.Metadata.TryGetValue("X-User-ID", out var userId);
            request.Metadata.TryGetValue("X-Caller-Service-ID", out var callerServiceId);

            // Create function call task
            var task = new FunctionCallTask
            {
                FunctionId = request.FunctionId,
                CallId = Guid.NewGuid().ToString(),
                GameId = gameId ?? _config.GameId ?? "",
                Env = env ?? _config.Env ?? "",
                Payload = payload,
                UserId = userId,
                IdempotencyKey = string.IsNullOrEmpty(request.IdempotencyKey) ? null : request.IdempotencyKey,
                CallerServiceId = callerServiceId
            };

            // Process function call
            var result = await ProcessFunctionCallAsync(task);

            // Build InvokeResponse
            var response = new InvokeResponse
            {
                Payload = Google.Protobuf.ByteString.CopyFromUtf8(result)
            };
            return response.ToByteArray();
        }

        _logger.LogWarning("CroupierClient", $"Unsupported inbound request: {Protocol.MsgIdString(msgId)}");
        var errorResponse = new InvokeResponse
        {
            Payload = Google.Protobuf.ByteString.CopyFromUtf8($"{{\"error\":\"Unsupported message type: {Protocol.MsgIdString(msgId)}\"}}")
        };
        return errorResponse.ToByteArray();
    }

    /// <summary>
    /// 处理函数调用循环
    /// </summary>
    private async Task ProcessCallsAsync(CancellationToken cancellationToken)
    {
        _logger.LogInfo("CroupierClient", "Call processor started");

        await foreach (var task in _callChannel.Reader.ReadAllAsync(cancellationToken))
        {
            _ = Task.Run(() => ProcessFunctionCallAsync(task), cancellationToken);
        }

        _logger.LogInfo("CroupierClient", "Call processor stopped");
    }

    private void ThrowIfDisposed()
    {
        if (_isDisposed)
            throw new ObjectDisposedException(nameof(CroupierClient));
    }

    private async Task ConnectAndRegisterAsync(CancellationToken cancellationToken)
    {
        // Remove tcp:// prefix if present, as TCPTransport expects raw "host:port" format
        var address = _config.AgentAddr.StartsWith("tcp://")
            ? _config.AgentAddr["tcp://".Length..]
            : _config.AgentAddr;
        var transport = _transportFactory(address, _config.TimeoutSeconds * 1000, _config.ConnectTimeoutSeconds * 1000, _logger);
        transport.Connect();

        try
        {
            var responseBytes = await transport.CallAsync(
                Protocol.MsgProviderConnectRequest,
                BuildProviderConnectRequest().ToByteArray(),
                cancellationToken);
            var response = ProviderConnectResponse.Parser.ParseFrom(responseBytes);
            if (string.IsNullOrWhiteSpace(response.SessionId))
            {
                throw new InvalidOperationException("ProviderConnect returned empty session_id");
            }

            _transport?.Dispose();
            _transport = transport;
            _sessionId = response.SessionId;

            // Register inbound request handler for InvokeRequest from Agent
            transport.SetInboundRequestHandler(HandleInboundRequestAsync);

            await RegisterCapabilitiesAsync(cancellationToken);
        }
        catch
        {
            transport.Dispose();
            throw;
        }
    }

    private ProviderConnectRequest BuildProviderConnectRequest()
    {
        var request = new ProviderConnectRequest
        {
            ServiceId = _config.ServiceId,
            Version = _config.ServiceVersion,
            SdkLanguage = _config.ProviderLang,
            SdkVersion = "1.0.0",
            SdkName = "croupier-csharp-sdk",
            ProtocolVersion = "v1"
        };

        foreach (var descriptor in _descriptors.Values)
        {
            var function = new ProviderFunctionDescriptor
            {
                Id = descriptor.Id,
                Version = descriptor.Version,
                Summary = descriptor.Summary ?? string.Empty,
                Description = descriptor.Description ?? string.Empty,
                OperationId = descriptor.OperationId ?? descriptor.Id,
                Deprecated = descriptor.Deprecated,
                InputSchema = descriptor.InputSchema ?? string.Empty,
                OutputSchema = descriptor.OutputSchema ?? string.Empty,
                Resource = descriptor.Resource ?? string.Empty,
                Operation = descriptor.Operation ?? string.Empty,
                Capability = descriptor.Capability ?? string.Empty,
                Execution = descriptor.Execution ?? string.Empty,
                ApprovalRequired = descriptor.ApprovalRequired,
                ApprovalPolicyKey = descriptor.ApprovalPolicyKey ?? string.Empty,
                Risk = descriptor.Risk,
                Enabled = descriptor.Enabled,
                Permission = descriptor.Permission ?? string.Empty
            };
            function.Tags.AddRange(DescriptorTags(descriptor));
            request.Functions.Add(function);
        }

        return request;
    }

    private async Task RegisterCapabilitiesAsync(CancellationToken cancellationToken)
    {
        if (string.IsNullOrWhiteSpace(_config.ControlAddr))
        {
            return;
        }

        // Remove tcp:// prefix if present, as TCPTransport expects raw "host:port" format
        var address = _config.ControlAddr.StartsWith("tcp://")
            ? _config.ControlAddr["tcp://".Length..]
            : _config.ControlAddr;
        using var transport = _transportFactory(address, _config.TimeoutSeconds * 1000, _config.ConnectTimeoutSeconds * 1000, _logger);
        transport.Connect();

        try
        {
            await transport.CallAsync(
                Protocol.MsgRegisterCapabilitiesReq,
                BuildRegisterCapabilitiesRequestData(),
                cancellationToken);
        }
        catch (Exception ex)
        {
            if (!_config.DisableLogging)
            {
                _logger.LogWarning("CroupierClient", $"Failed to register capabilities: {ex.Message}");
            }
        }
    }

    private byte[] BuildRegisterCapabilitiesRequestData()
    {
        using var stream = new MemoryStream();
        using var output = new Google.Protobuf.CodedOutputStream(stream, leaveOpen: true);

        var providerData = BuildProviderMetaData();
        output.WriteTag(1, Google.Protobuf.WireFormat.WireType.LengthDelimited);
        output.WriteBytes(Google.Protobuf.ByteString.CopyFrom(providerData));

        output.WriteTag(2, Google.Protobuf.WireFormat.WireType.LengthDelimited);
        output.WriteBytes(Google.Protobuf.ByteString.CopyFrom(GetManifestGzipped()));
        output.Flush();

        return stream.ToArray();
    }

    private byte[] BuildProviderMetaData()
    {
        using var stream = new MemoryStream();
        using var output = new Google.Protobuf.CodedOutputStream(stream, leaveOpen: true);

        WriteStringField(output, 1, _config.ServiceId);
        WriteStringField(output, 2, _config.ServiceVersion);
        WriteStringField(output, 3, _config.ProviderLang);
        WriteStringField(output, 4, _config.ProviderSdk);
        output.Flush();

        return stream.ToArray();
    }

    private static void WriteStringField(Google.Protobuf.CodedOutputStream output, int fieldNumber, string? value)
    {
        if (string.IsNullOrEmpty(value))
        {
            return;
        }

        output.WriteTag(fieldNumber, Google.Protobuf.WireFormat.WireType.LengthDelimited);
        output.WriteString(value);
    }

    private static IEnumerable<string> DescriptorTags(FunctionDescriptor descriptor)
    {
        IEnumerable<string?> baseTags = new[]
            {
                descriptor.Resource,
                descriptor.Operation,
            };

        return baseTags
            .Concat(descriptor.Tags?.Keys ?? Enumerable.Empty<string>())
            .Concat(descriptor.Tags?.Values ?? Enumerable.Empty<string>())
            .Where(tag => !string.IsNullOrWhiteSpace(tag))
            .Select(tag => tag!.Trim())
            .Distinct(StringComparer.OrdinalIgnoreCase);
    }

    private byte[] GetManifestGzipped()
    {
        var manifest = new
        {
            provider = new
            {
                id = _config.ServiceId,
                version = _config.ServiceVersion,
                lang = _config.ProviderLang,
                sdk = _config.ProviderSdk
            },
            functions = _descriptors.Values.Select(descriptor => new
            {
                id = descriptor.Id,
                version = descriptor.Version,
                summary = descriptor.Summary,
                tags = DescriptorTags(descriptor),
                operation_id = descriptor.OperationId,
                deprecated = descriptor.Deprecated,
                resource = descriptor.Resource,
                operation = descriptor.Operation,
                capability = descriptor.Capability,
                execution = descriptor.Execution,
                risk = descriptor.Risk,
                enabled = descriptor.Enabled,
                permission = descriptor.Permission,
                description = descriptor.Description,
                input_schema = descriptor.InputSchema,
                output_schema = descriptor.OutputSchema
            })
        };

        var json = JsonSerializer.SerializeToUtf8Bytes(manifest);
        using var output = new MemoryStream();
        using (var gzip = new GZipStream(output, CompressionLevel.SmallestSize, leaveOpen: true))
        {
            gzip.Write(json, 0, json.Length);
        }

        return output.ToArray();
    }

    private void StartHeartbeatLoop()
    {
        _heartbeatCts?.Cancel();
        _heartbeatCts?.Dispose();
        _heartbeatCts = CancellationTokenSource.CreateLinkedTokenSource(_shutdownCts.Token);
        _heartbeatTask = Task.Run(() => HeartbeatLoopAsync(_heartbeatCts.Token), _heartbeatCts.Token);
    }

    private async Task HeartbeatLoopAsync(CancellationToken cancellationToken)
    {
        var interval = TimeSpan.FromSeconds(Math.Max(_config.HeartbeatIntervalSeconds, 1));

        while (!cancellationToken.IsCancellationRequested)
        {
            try
            {
                await Task.Delay(interval, cancellationToken);
                await SendHeartbeatAsync(cancellationToken);
            }
            catch (OperationCanceledException)
            {
                break;
            }
            catch (Exception ex)
            {
                _logger.LogWarning("CroupierClient", $"Heartbeat failed: {ex.Message}");
                if (!_config.AutoReconnect)
                {
                    continue;
                }

                await ReconnectAsync(cancellationToken);
            }
        }
    }

    private async Task SendHeartbeatAsync(CancellationToken cancellationToken)
    {
        if (_transport == null || !_transport.IsConnected || string.IsNullOrWhiteSpace(_sessionId))
        {
            throw new InvalidOperationException("Not connected to Agent");
        }

        var request = new ProviderHeartbeatRequest
        {
            ServiceId = _config.ServiceId,
            SessionId = _sessionId
        };

        await _transport.CallAsync(Protocol.MsgProviderHeartbeatRequest, request.ToByteArray(), cancellationToken);
    }

    private async Task ReconnectAsync(CancellationToken cancellationToken)
    {
        _transport?.Dispose();
        _transport = null;
        _isConnected = false;

        var attempts = 0;
        while (!cancellationToken.IsCancellationRequested)
        {
            attempts++;
            try
            {
                await ConnectAndRegisterAsync(cancellationToken);
                _isConnected = true;
                _logger.LogInfo("CroupierClient", $"Reconnected and re-registered service {_config.ServiceId}");
                return;
            }
            catch (Exception ex) when (!cancellationToken.IsCancellationRequested)
            {
                _logger.LogWarning("CroupierClient", $"Reconnect attempt {attempts} failed: {ex.Message}");

                if (_config.ReconnectMaxAttempts > 0 && attempts >= _config.ReconnectMaxAttempts)
                {
                    throw;
                }

                await Task.Delay(TimeSpan.FromSeconds(Math.Max(_config.ReconnectIntervalSeconds, 1)), cancellationToken);
            }
        }
    }

    /// <summary>
    /// 释放资源
    /// </summary>
    public void Dispose()
    {
        if (_isDisposed)
            return;

        _logger.LogInfo("CroupierClient", "Disposing...");

        Disconnect();
        _shutdownCts.Dispose();

        _isDisposed = true;
        GC.SuppressFinalize(this);
    }

    /// <summary>
    /// 函数调用任务
    /// </summary>
    private class FunctionCallTask
    {
        public required string FunctionId { get; init; }
        public required string CallId { get; init; }
        public required string GameId { get; init; }
        public required string Env { get; init; }
        public required string Payload { get; init; }
        public string? UserId { get; init; }
        public string? IdempotencyKey { get; init; }
        public string? CallerServiceId { get; init; }
    }
}
