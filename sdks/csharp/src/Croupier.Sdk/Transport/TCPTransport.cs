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

using System.Net.Sockets;
using System.Collections.Concurrent;
using System.Threading;
using Croupier.Sdk.Logging;

namespace Croupier.Sdk.Transport;

/// <summary>
/// TCP-based transport client using multiplexed bidirectional communication.
///
/// Wire Protocol:
///   Frame:   [4-byte length prefix (big-endian)][payload]
///   Payload: [8-byte header][protobuf body]
///   Header:  Version(1B) + MsgID(3B) + RequestID(4B)
///
/// Request messages have odd MsgID, Response messages have even MsgID.
/// Multiple concurrent request/response pairs multiplex on the same connection.
/// </summary>
public sealed class TCPTransport : IClientTransport
{
    private const int FrameHeaderBytes = 4;
    private const int MaxFrameBytes = 32 * 1024 * 1024; // 32 MB

    private readonly string _host;
    private readonly int _port;
    private readonly int _timeoutMs;
    private int _connectTimeoutMs;
    private readonly ICroupierLogger _logger;

    private TcpClient? _client;
    private NetworkStream? _stream;
    private bool _connected;
    private int _requestId;
    private bool _isDisposed;

    // Pending request tracking: request_id -> TaskCompletionSource
    private readonly ConcurrentDictionary<int, TaskCompletionSource<(int msgType, byte[] data)>> _pending = new();
    private readonly CancellationTokenSource? _readLoopCts;
    private Task? _readLoopTask;

    // Handler for inbound requests from Agent (e.g., InvokeRequest)
    private Func<int, int, byte[], Task<byte[]>>? _inboundRequestHandler;

    // Inbound worker pool: bounded concurrency (default = CPU cores).
    // Fire-and-forget per request is unbounded; a slow handler storm can
    // exhaust memory. Queue capacity = workers * 4; overflow fast-fails
    // with an empty response so the Agent-side failover takes over.
    private readonly SemaphoreSlim _inboundLimiter = new(
        Math.Max(2, Environment.ProcessorCount), Math.Max(2, Environment.ProcessorCount));
    private int _inboundQueued;

    /// <summary>
    /// Gets whether the transport is connected.
    /// </summary>
    public bool IsConnected => _connected && _client?.Connected == true && _stream != null;

    /// <summary>
    /// Set the connection timeout (separate from request timeout).
    /// </summary>
    /// <param name="timeoutMs">Connection timeout in milliseconds</param>
    public void SetConnectTimeout(int timeoutMs) => _connectTimeoutMs = timeoutMs;

    /// <summary>
    /// Set handler for inbound requests from Agent (e.g., InvokeRequest).
    /// </summary>
    public void SetInboundRequestHandler(Func<int, int, byte[], Task<byte[]>>? handler)
    {
        _inboundRequestHandler = handler;
    }

    /// <summary>
    /// Initialize TCP transport.
    /// </summary>
    /// <param name="address">TCP address (e.g., "127.0.0.1:19090" or "[::1]:19090" for IPv6)</param>
    /// <param name="timeoutMs">Request timeout in milliseconds</param>
    /// <param name="connectTimeoutMs">Connection timeout in milliseconds (default: 5000ms)</param>
    /// <param name="logger">Logger instance</param>
    /// <exception cref="ArgumentNullException">Thrown when address is null</exception>
    /// <exception cref="ArgumentException">Thrown when address format is invalid</exception>
    /// <exception cref="OverflowException">Thrown when port is out of valid range</exception>
    public TCPTransport(string address, int timeoutMs = 30000, int connectTimeoutMs = 5000, ICroupierLogger? logger = null)
    {
        if (address == null)
        {
            throw new ArgumentNullException(nameof(address));
        }

        // Handle IPv6 addresses in brackets [host]:port
        if (address.StartsWith('['))
        {
            var bracketEnd = address.IndexOf(']');
            if (bracketEnd == -1 || bracketEnd == 1)
            {
                throw new ArgumentException($"Invalid IPv6 address format: {address}", nameof(address));
            }

            var colonAfterBracket = address.IndexOf(':', bracketEnd);
            if (colonAfterBracket == -1 || colonAfterBracket != bracketEnd + 1 || colonAfterBracket == address.Length - 1)
            {
                throw new ArgumentException($"Invalid IPv6 address format: {address}", nameof(address));
            }

            _host = address.Substring(1, bracketEnd - 1);
            var portStr = address.Substring(colonAfterBracket + 1);
            _port = ParsePort(portStr);
        }
        else
        {
            var parts = address.Split(':');
            if (parts.Length != 2)
            {
                throw new ArgumentException($"Invalid address format: {address}", nameof(address));
            }

            _host = parts[0];
            _port = ParsePort(parts[1]);
        }

        _timeoutMs = timeoutMs;
        _connectTimeoutMs = connectTimeoutMs;
        _logger = logger ?? new ConsoleCroupierLogger("TCPTransport");
        _readLoopCts = new CancellationTokenSource();
    }

    /// <summary>
    /// Parse port string to int, throwing OverflowException for out-of-range values.
    /// </summary>
    private static int ParsePort(string portStr)
    {
        // Check for negative sign explicitly to throw OverflowException
        if (portStr.StartsWith('-'))
        {
            throw new OverflowException("Port number cannot be negative");
        }

        // Use int.Parse which will throw OverflowException for values outside int range
        try
        {
            var port = int.Parse(portStr);
            if (port < 0 || port > 65535)
            {
                throw new OverflowException($"Port number {port} is out of valid range (0-65535)");
            }
            return port;
        }
        catch (FormatException)
        {
            // Re-throw as is for non-numeric values
            throw;
        }
    }

    /// <summary>
    /// Connect to the TCP server (Agent) with timeout.
    /// </summary>
    public void Connect()
    {
        if (_connected)
        {
            return;
        }

        ThrowIfDisposed();

        _logger.LogInfo("TCPTransport", $"Connecting to TCP server at: {_host}:{_port}");

        try
        {
            _client = new TcpClient();
            _client.ReceiveTimeout = _timeoutMs;
            _client.SendTimeout = _timeoutMs;

            // Use async connect with timeout to avoid blocking indefinitely
            // ConnectAsync returns a Task that completes when connection is established
            var connectTask = _client.ConnectAsync(_host, _port);

            // Wait for connection with timeout
            using var timeoutCts = new CancellationTokenSource(_connectTimeoutMs);
            connectTask.WaitAsync(timeoutCts.Token).GetAwaiter().GetResult();

            _stream = _client.GetStream();
            _connected = true;

            // Start background read loop
            _readLoopTask = Task.Run(() => ReadLoop(_readLoopCts!.Token));

            _logger.LogInfo("TCPTransport", "Connected successfully");
        }
        catch (Exception ex)
        {
            _logger.LogError("TCPTransport", $"Failed to connect: {ex.Message}");
            _client?.Dispose();
            _client = null;
            throw new TimeoutException($"Connection timeout to {_host}:{_port} after {_connectTimeoutMs}ms");
        }
    }

    /// <summary>
    /// Send a request and wait for the response synchronously.
    /// </summary>
    public byte[] Call(int msgType, byte[]? data)
    {
        return CallAsync(msgType, data).GetAwaiter().GetResult();
    }

    /// <summary>
    /// Send a request and wait for the response asynchronously.
    /// </summary>
    public async Task<byte[]> CallAsync(int msgType, byte[]? data, CancellationToken cancellationToken = default)
    {
        if (!IsConnected)
        {
            throw new InvalidOperationException("Not connected");
        }

        ThrowIfDisposed();

        // Generate request ID
        _requestId = (_requestId + 1) & 0x7FFFFFFF;
        var reqId = _requestId;

        // Create message
        var message = Protocol.NewMessage(msgType, reqId, data);

        // Register pending request
        var tcs = new TaskCompletionSource<(int msgType, byte[] data)>(TaskCreationOptions.RunContinuationsAsynchronously);
        _pending[reqId] = tcs;

        try
        {
            // Send frame
            await WriteFrameAsync(message, cancellationToken).ConfigureAwait(false);

            // Wait for response with timeout
            using var timeoutCts = new CancellationTokenSource(_timeoutMs);
            using var combinedCts = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken, timeoutCts.Token);

            var (respMsgType, respData) = await tcs.Task.WaitAsync(combinedCts.Token).ConfigureAwait(false);

            return respData;
        }
        finally
        {
            _pending.TryRemove(reqId, out _);
        }
    }

    /// <summary>
    /// Handle inbound request from Agent.
    /// </summary>
    private async Task HandleInboundRequest(int msgId, int reqId, byte[] body, CancellationToken cancellationToken)
    {
        byte[]? responseBody = null;
        try
        {
            if (_inboundRequestHandler == null)
            {
                _logger.LogWarning("TCPTransport", $"No handler for inbound request: {Protocol.MsgIdString(msgId)}");
                responseBody = System.Text.Json.JsonSerializer.SerializeToUtf8Bytes(new { error = "no handler registered" });
            }
            else
            {
                _logger.LogDebug("TCPTransport", $"Processing inbound request: {Protocol.MsgIdString(msgId)}");
                responseBody = await _inboundRequestHandler(msgId, reqId, body).ConfigureAwait(false);
            }
        }
        catch (Exception ex)
        {
            _logger.LogError("TCPTransport", $"Inbound request handler failed: {ex.Message}");
            responseBody = System.Text.Json.JsonSerializer.SerializeToUtf8Bytes(new { error = ex.Message });
        }

        // Send response
        var responseMsgId = Protocol.GetResponseMsgId(msgId);
        var message = Protocol.NewMessage(responseMsgId, reqId, responseBody);
        try
        {
            await WriteFrameAsync(message, cancellationToken).ConfigureAwait(false);
        }
        catch (Exception ex)
        {
            _logger.LogError("TCPTransport", $"Failed to send response: {ex.Message}");
        }
    }

    /// <summary>
    /// Background read loop that processes incoming frames.
    /// </summary>
    private async Task ReadLoop(CancellationToken cancellationToken)
    {
        var buffer = new byte[8192];

        while (!cancellationToken.IsCancellationRequested && IsConnected)
        {
            try
            {
                // Read frame header (4-byte length)
                var frameHeader = await ReadExactAsync(FrameHeaderBytes, cancellationToken).ConfigureAwait(false);
                var frameSize = (frameHeader[0] << 24) | (frameHeader[1] << 16) | (frameHeader[2] << 8) | frameHeader[3];

                if (frameSize == 0)
                {
                    continue;
                }

                if (frameSize > MaxFrameBytes)
                {
                    _logger.LogError("TCPTransport", $"Frame too large: {frameSize}");
                    break;
                }

                // Read frame payload
                var payload = await ReadExactAsync(frameSize, cancellationToken).ConfigureAwait(false);

                // Parse message
                var parsed = Protocol.ParseMessage(payload);

                if (Protocol.IsResponse(parsed.MsgId))
                {
                    // Match to pending request
                    if (_pending.TryRemove(parsed.ReqId, out var tcs))
                    {
                        tcs.SetResult((parsed.MsgId, parsed.Body));
                    }
                }
                else if (Protocol.IsRequest(parsed.MsgId))
                {
                    // The Agent can call a Provider while this transport is
                    // awaiting a response to its own request. Do not await the
                    // callback in the sole read loop, otherwise its response
                    // cannot be read and both peers time out.
                    DispatchInbound(parsed.MsgId, parsed.ReqId, parsed.Body, cancellationToken);
                }
            }
            catch (OperationCanceledException)
            {
                break;
            }
            catch (EndOfStreamException)
            {
                // Connection closed by remote side — not an error, just log and break.
                if (!cancellationToken.IsCancellationRequested)
                {
                    _logger.LogWarning("TCPTransport", "Connection closed by remote");
                }
                _connected = false;
                break;
            }
            catch (Exception ex)
            {
                if (!cancellationToken.IsCancellationRequested)
                {
                    _logger.LogError("TCPTransport", $"Read loop error: {ex.Message}");
                }
                _connected = false;
                break;
            }
        }

        // Fail all pending requests with an error (NOT cancellation): callers
        // like the heartbeat loop treat OperationCanceledException as normal
        // shutdown and would silently exit instead of reconnecting.
        foreach (var (reqId, tcs) in _pending)
        {
            tcs.TrySetException(new InvalidOperationException("connection closed"));
        }
        _pending.Clear();
    }

    private void DispatchInbound(int msgId, int reqId, byte[] body, CancellationToken cancellationToken)
    {
        int capacity = Math.Max(2, Environment.ProcessorCount) * 4;
        if (Interlocked.CompareExchange(ref _inboundQueued, 0, 0) >= capacity)
        {
            // Queue full: respond immediately (empty) so the Agent fails over.
            _ = WriteFrameAsync(
                Protocol.NewMessage(Protocol.GetResponseMsgId(msgId), reqId, new byte[0]),
                cancellationToken);
            return;
        }
        Interlocked.Increment(ref _inboundQueued);
        _ = Task.Run(async () =>
        {
            try
            {
                await _inboundLimiter.WaitAsync(cancellationToken).ConfigureAwait(false);
                try
                {
                    await HandleInboundRequest(msgId, reqId, body, cancellationToken).ConfigureAwait(false);
                }
                finally
                {
                    _ = _inboundLimiter.Release();
                }
            }
            catch (OperationCanceledException) when (cancellationToken.IsCancellationRequested)
            {
                // Normal shutdown.
            }
            catch (Exception ex)
            {
                _logger.LogError("TCPTransport", $"Inbound request processing failed: {ex.Message}", ex);
            }
            finally
            {
                Interlocked.Decrement(ref _inboundQueued);
            }
        });
    }

    private async Task HandleInboundRequestAsync(int msgId, int reqId, byte[] body, CancellationToken cancellationToken)
    {
        try
        {
            await HandleInboundRequest(msgId, reqId, body, cancellationToken).ConfigureAwait(false);
        }
        catch (OperationCanceledException) when (cancellationToken.IsCancellationRequested)
        {
            // Normal shutdown.
        }
        catch (Exception ex)
        {
            _logger.LogError("TCPTransport", $"Inbound request processing failed: {ex.Message}", ex);
        }
    }

    /// <summary>
    /// Read exactly n bytes from the stream.
    /// Uses a short per-read timeout (1s) so the caller can detect connection
    /// drops promptly instead of blocking indefinitely on ReadAsync.
    /// </summary>
    private async Task<byte[]> ReadExactAsync(int n, CancellationToken cancellationToken)
    {
        var data = new byte[n];
        var offset = 0;

        while (offset < n)
        {
            // Use a short per-read timeout so we can detect connection drops.
            // CancellationTokenSource.CreateLinkedTokenSource is lightweight.
            using var readCts = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
            readCts.CancelAfter(TimeSpan.FromSeconds(1));

            int read;
            try
            {
                read = await _stream!.ReadAsync(data.AsMemory(offset, n - offset), readCts.Token).ConfigureAwait(false);
            }
            catch (OperationCanceledException) when (!cancellationToken.IsCancellationRequested)
            {
                // Short read timeout — connection is still alive, just no data yet.
                // Continue the outer loop to retry.
                continue;
            }

            if (read == 0)
            {
                throw new EndOfStreamException("Connection closed");
            }
            offset += read;
        }

        return data;
    }

    /// <summary>
    /// Write a length-prefixed frame to the stream.
    /// </summary>
    private async Task WriteFrameAsync(byte[] payload, CancellationToken cancellationToken)
    {
        var header = new byte[FrameHeaderBytes];
        header[0] = (byte)(payload.Length >> 24);
        header[1] = (byte)(payload.Length >> 16);
        header[2] = (byte)(payload.Length >> 8);
        header[3] = (byte)payload.Length;

        await _stream!.WriteAsync(header, cancellationToken).ConfigureAwait(false);
        await _stream.WriteAsync(payload, cancellationToken).ConfigureAwait(false);
        await _stream.FlushAsync(cancellationToken).ConfigureAwait(false);
    }

    /// <summary>
    /// Close the connection and release resources.
    /// </summary>
    public void Dispose()
    {
        if (_isDisposed)
        {
            return;
        }

        _isDisposed = true;
        _connected = false;

        _readLoopCts?.Cancel();
        _readLoopTask?.Wait(TimeSpan.FromSeconds(1));
        _readLoopCts?.Dispose();

        _stream?.Dispose();
        _client?.Dispose();

        // Fail all pending requests
        foreach (var (_, tcs) in _pending)
        {
            tcs.SetCanceled();
        }
        _pending.Clear();
    }

    private void ThrowIfDisposed()
    {
        if (_isDisposed)
        {
            throw new ObjectDisposedException(nameof(TCPTransport));
        }
    }
}
