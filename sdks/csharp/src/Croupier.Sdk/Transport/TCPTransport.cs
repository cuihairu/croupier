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

    /// <summary>
    /// Gets whether the transport is connected.
    /// </summary>
    public bool IsConnected => _connected && _client?.Connected == true && _stream != null;

    /// <summary>
    /// Initialize TCP transport.
    /// </summary>
    /// <param name="address">TCP address (e.g., "127.0.0.1:19090")</param>
    /// <param name="timeoutMs">Request timeout in milliseconds</param>
    /// <param name="logger">Logger instance</param>
    public TCPTransport(string address, int timeoutMs = 30000, ICroupierLogger? logger = null)
    {
        var parts = address.Split(':');
        if (parts.Length != 2)
        {
            throw new ArgumentException($"Invalid address format: {address}", nameof(address));
        }

        _host = parts[0];
        _port = int.Parse(parts[1]);
        _timeoutMs = timeoutMs;
        _logger = logger ?? new ConsoleCroupierLogger("TCPTransport");
        _readLoopCts = new CancellationTokenSource();
    }

    /// <summary>
    /// Connect to the TCP server (Agent).
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
            _client.Connect(_host, _port);
            _stream = _client.GetStream();
            _connected = true;

            // Start background read loop
            _readLoopTask = Task.Run(() => ReadLoop(_readLoopCts!.Token));

            _logger.LogInfo("TCPTransport", "Connected successfully");
        }
        catch (Exception ex)
        {
            _logger.LogError("TCPTransport", $"Failed to connect: {ex.Message}");
            throw;
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
                    // TODO: Handle inbound requests (invoke/task from agent)
                    _logger.LogWarning("TCPTransport", $"Received inbound request: {Protocol.MsgIdString(parsed.MsgId)} (not implemented)");
                }
            }
            catch (OperationCanceledException)
            {
                break;
            }
            catch (Exception ex)
            {
                if (!cancellationToken.IsCancellationRequested)
                {
                    _logger.LogError("TCPTransport", $"Read loop error: {ex.Message}");
                }
                break;
            }
        }

        // Fail all pending requests
        foreach (var (reqId, tcs) in _pending)
        {
            tcs.SetCanceled(cancellationToken);
        }
        _pending.Clear();
    }

    /// <summary>
    /// Read exactly n bytes from the stream.
    /// </summary>
    private async Task<byte[]> ReadExactAsync(int n, CancellationToken cancellationToken)
    {
        var data = new byte[n];
        var offset = 0;

        while (offset < n)
        {
            var read = await _stream!.ReadAsync(data.AsMemory(offset, n - offset), cancellationToken).ConfigureAwait(false);
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
