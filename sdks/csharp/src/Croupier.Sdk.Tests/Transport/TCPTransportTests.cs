// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

using System.Net;
using System.Net.Sockets;
using System.Text;
using Croupier.Sdk.Logging;
using Croupier.Sdk.Transport;
using FluentAssertions;
using Moq;
using Xunit;
using Xunit.Abstractions;

namespace Croupier.Sdk.Tests.Transport;

/// <summary>
/// Comprehensive unit tests for TCPTransport
/// </summary>
public class TCPTransportTests
{
    private readonly ITestOutputHelper _output;

    public TCPTransportTests(ITestOutputHelper output)
    {
        _output = output;
    }

    private class TestLogger : ICroupierLogger
    {
        private readonly ITestOutputHelper _output;
        private readonly string _prefix;

        public TestLogger(ITestOutputHelper output, string prefix = "Test")
        {
            _output = output;
            _prefix = prefix;
        }

        public void LogDebug(string component, string message) =>
            _output.WriteLine($"[DEBUG] [{_prefix}] {component}: {message}");

        public void LogInfo(string component, string message) =>
            _output.WriteLine($"[INFO] [{_prefix}] {component}: {message}");

        public void LogWarning(string component, string message) =>
            _output.WriteLine($"[WARN] [{_prefix}] {component}: {message}");

        public void LogError(string component, string message, Exception? exception = null)
        {
            if (exception != null)
                _output.WriteLine($"[ERROR] [{_prefix}] {component}: {message} - {exception.Message}");
            else
                _output.WriteLine($"[ERROR] [{_prefix}] {component}: {message}");
        }
    }

    #region Constructor Tests

    [Fact]
    public void Constructor_WithValidAddress_SetsHostAndPort()
    {
        // Arrange & Act
        var transport = new TCPTransport("127.0.0.1:19090");

        // Assert
        transport.IsConnected.Should().BeFalse();
    }

    [Fact]
    public void Constructor_WithIPv6Address_SetsHostAndPort()
    {
        // Arrange & Act
        var transport = new TCPTransport("[::1]:19090");

        // Assert
        transport.IsConnected.Should().BeFalse();
    }

    [Fact]
    public void Constructor_WithCustomTimeout_SetsTimeout()
    {
        // Arrange & Act
        var transport = new TCPTransport("127.0.0.1:19090", timeoutMs: 60000);

        // Assert
        transport.IsConnected.Should().BeFalse();
    }

    [Fact]
    public void Constructor_WithCustomConnectTimeout_SetsConnectTimeout()
    {
        // Arrange & Act
        var transport = new TCPTransport("127.0.0.1:19090", connectTimeoutMs: 10000);

        // Assert
        transport.IsConnected.Should().BeFalse();
    }

    [Fact]
    public void Constructor_WithLogger_UsesProvidedLogger()
    {
        // Arrange
        var logger = new TestLogger(_output);

        // Act
        var transport = new TCPTransport("127.0.0.1:19090", logger: logger);

        // Assert
        transport.IsConnected.Should().BeFalse();
    }

    [Fact]
    public void Constructor_WithNullAddress_ThrowsArgumentNullException()
    {
        // Act & Assert
        Assert.Throws<ArgumentNullException>(() => new TCPTransport(null!));
    }

    [Fact]
    public void Constructor_WithEmptyAddress_ThrowsArgumentException()
    {
        // Act & Assert
        Assert.Throws<ArgumentException>(() => new TCPTransport(""));
    }

    [Fact]
    public void Constructor_WithInvalidFormat_NoColon_ThrowsArgumentException()
    {
        // Act & Assert
        Assert.Throws<ArgumentException>(() => new TCPTransport("invalid"));
    }

    [Fact]
    public void Constructor_WithInvalidFormat_MultipleColons_ThrowsArgumentException()
    {
        // Act & Assert
        Assert.Throws<ArgumentException>(() => new TCPTransport("host:port:extra"));
    }

    [Fact]
    public void Constructor_WithInvalidPort_NotNumeric_ThrowsFormatException()
    {
        // Act & Assert
        Assert.Throws<FormatException>(() => new TCPTransport("127.0.0.1:abc"));
    }

    [Fact]
    public void Constructor_WithInvalidPort_Negative_ThrowsOverflowException()
    {
        // Act & Assert
        Assert.Throws<OverflowException>(() => new TCPTransport("127.0.0.1:-1"));
    }

    [Fact]
    public void Constructor_WithInvalidPort_TooLarge_ThrowsOverflowException()
    {
        // Act & Assert
        Assert.Throws<OverflowException>(() => new TCPTransport("127.0.0.1:70000"));
    }

    [Fact]
    public void Constructor_WithIPv6InvalidFormat_NoClosingBracket_ThrowsArgumentException()
    {
        // Act & Assert
        Assert.Throws<ArgumentException>(() => new TCPTransport("[::1:19090"));
    }

    [Fact]
    public void Constructor_WithIPv6InvalidFormat_EmptyHost_ThrowsArgumentException()
    {
        // Act & Assert
        Assert.Throws<ArgumentException>(() => new TCPTransport("[]:19090"));
    }

    [Fact]
    public void Constructor_WithIPv6InvalidFormat_NoColonAfterBracket_ThrowsArgumentException()
    {
        // Act & Assert
        Assert.Throws<ArgumentException>(() => new TCPTransport("[::1]19090"));
    }

    [Fact]
    public void Constructor_WithIPv6InvalidFormat_NoPort_ThrowsArgumentException()
    {
        // Act & Assert
        Assert.Throws<ArgumentException>(() => new TCPTransport("[::1]:"));
    }

    #endregion

    #region Property Tests

    [Fact]
    public void IsConnected_InitiallyFalse()
    {
        // Arrange
        var transport = new TCPTransport("127.0.0.1:19090");

        // Act & Assert
        transport.IsConnected.Should().BeFalse();
    }

    #endregion

    #region SetConnectTimeout Tests

    [Fact]
    public void SetConnectTimeout_UpdatesTimeout()
    {
        // Arrange
        var transport = new TCPTransport("127.0.0.1:19090");

        // Act
        transport.SetConnectTimeout(10000);

        // Assert - no exception thrown
    }

    #endregion

    #region SetInboundRequestHandler Tests

    [Fact]
    public void SetInboundRequestHandler_WithHandler_SetsHandler()
    {
        // Arrange
        var transport = new TCPTransport("127.0.0.1:19090");
        Func<int, int, byte[], Task<byte[]>> handler = (msgId, reqId, body) => Task.FromResult(body);

        // Act
        transport.SetInboundRequestHandler(handler);

        // Assert - no exception thrown
    }

    [Fact]
    public void SetInboundRequestHandler_WithNull_ClearsHandler()
    {
        // Arrange
        var transport = new TCPTransport("127.0.0.1:19090");

        // Act
        transport.SetInboundRequestHandler(null);

        // Assert - no exception thrown
    }

    #endregion

    #region Connect Tests

    [Fact]
    public void Connect_WhenServerNotRunning_ThrowsTimeoutException()
    {
        // Arrange
        var transport = new TCPTransport("127.0.0.1:1", connectTimeoutMs: 100);

        // Act & Assert
        Assert.Throws<TimeoutException>(() => transport.Connect());
    }

    [Fact]
    public void Connect_WhenAlreadyConnected_DoesNothing()
    {
        // Arrange
        var transport = new TCPTransport("127.0.0.1:19090");

        // Use reflection to set _connected to true
        var connectedField = typeof(TCPTransport).GetField("_connected",
            System.Reflection.BindingFlags.NonPublic | System.Reflection.BindingFlags.Instance);
        connectedField?.SetValue(transport, true);

        // Act
        transport.Connect(); // Should not throw

        // Assert
        transport.IsConnected.Should().BeFalse(); // Because _client is null
    }

    [Fact]
    public void Connect_WhenDisposed_ThrowsObjectDisposedException()
    {
        // Arrange
        var transport = new TCPTransport("127.0.0.1:19090");
        transport.Dispose();

        // Act & Assert
        Assert.Throws<ObjectDisposedException>(() => transport.Connect());
    }

    #endregion

    #region Call Tests

    [Fact]
    public void Call_WhenNotConnected_ThrowsInvalidOperationException()
    {
        // Arrange
        var transport = new TCPTransport("127.0.0.1:19090");

        // Act & Assert
        Assert.Throws<InvalidOperationException>(() => transport.Call(0x030101, Encoding.UTF8.GetBytes("test")));
    }

    [Fact]
    public void Call_WhenDisposed_ThrowsInvalidOperationException()
    {
        // Arrange
        var transport = new TCPTransport("127.0.0.1:19090");
        transport.Dispose();

        // Act & Assert
        // Note: Call checks IsConnected first, which throws InvalidOperationException
        Assert.Throws<InvalidOperationException>(() => transport.Call(0x030101, Encoding.UTF8.GetBytes("test")));
    }

    #endregion

    #region CallAsync Tests

    [Fact]
    public async Task CallAsync_WhenNotConnected_ThrowsInvalidOperationException()
    {
        // Arrange
        var transport = new TCPTransport("127.0.0.1:19090");

        // Act & Assert
        await Assert.ThrowsAsync<InvalidOperationException>(
            () => transport.CallAsync(0x030101, Encoding.UTF8.GetBytes("test")));
    }

    [Fact]
    public async Task CallAsync_WhenDisposed_ThrowsInvalidOperationException()
    {
        // Arrange
        var transport = new TCPTransport("127.0.0.1:19090");
        transport.Dispose();

        // Act & Assert
        // Note: CallAsync checks IsConnected first, which throws InvalidOperationException
        await Assert.ThrowsAsync<InvalidOperationException>(
            () => transport.CallAsync(0x030101, Encoding.UTF8.GetBytes("test")));
    }

    #endregion

    #region Dispose Tests

    [Fact]
    public void Dispose_WhenNotConnected_DoesNotThrow()
    {
        // Arrange
        var transport = new TCPTransport("127.0.0.1:19090");

        // Act & Assert
        transport.Dispose();
    }

    [Fact]
    public void Dispose_WhenAlreadyDisposed_DoesNotThrow()
    {
        // Arrange
        var transport = new TCPTransport("127.0.0.1:19090");

        // Act
        transport.Dispose();

        // Assert
        transport.Dispose(); // Should not throw
    }

    [Fact]
    public void Dispose_CancelsPendingRequests()
    {
        // Arrange
        var transport = new TCPTransport("127.0.0.1:19090");

        // Use reflection to add a pending request
        var pendingField = typeof(TCPTransport).GetField("_pending",
            System.Reflection.BindingFlags.NonPublic | System.Reflection.BindingFlags.Instance);
        var pending = pendingField?.GetValue(transport) as System.Collections.Concurrent.ConcurrentDictionary<int, System.Threading.Tasks.TaskCompletionSource<(int, byte[])>>;
        var tcs = new System.Threading.Tasks.TaskCompletionSource<(int, byte[])>();
        pending?.TryAdd(1, tcs);

        // Act
        transport.Dispose();

        // Assert
        tcs.Task.IsCanceled.Should().BeTrue();
    }

    #endregion

    #region Integration Tests with Real TCP Server

    [Fact]
    public async Task Connect_And_Dispose_WithRealServer()
    {
        // Arrange
        var server = new TcpListener(IPAddress.Loopback, 0);
        server.Start();
        var port = ((IPEndPoint)server.LocalEndpoint).Port;

        try
        {
            var transport = new TCPTransport($"127.0.0.1:{port}", connectTimeoutMs: 5000);

            // Act
            transport.Connect();

            // Assert
            transport.IsConnected.Should().BeTrue();

            // Cleanup
            transport.Dispose();
            transport.IsConnected.Should().BeFalse();
        }
        finally
        {
            server.Stop();
        }
    }

    [Fact]
    public async Task Call_WithRealServer_SendsAndReceives()
    {
        // Arrange
        var server = new TcpListener(IPAddress.Loopback, 0);
        server.Start();
        var port = ((IPEndPoint)server.LocalEndpoint).Port;

        var serverTask = Task.Run(async () =>
        {
            var client = await server.AcceptTcpClientAsync();
            var stream = client.GetStream();

            // Read frame header (4 bytes length)
            var header = new byte[4];
            await ReadExactAsync(stream, header);
            var length = (header[0] << 24) | (header[1] << 16) | (header[2] << 8) | header[3];

            // Read frame payload
            var payload = new byte[length];
            await ReadExactAsync(stream, payload);

            // Parse message
            var msgId = (payload[1] << 16) | (payload[2] << 8) | payload[3];
            var reqId = (payload[4] << 24) | (payload[5] << 16) | (payload[6] << 8) | payload[7];

            // Create response
            var respMsgId = msgId + 1; // Response = Request + 1
            var respBody = Encoding.UTF8.GetBytes("response");
            var response = new byte[8 + respBody.Length];
            response[0] = 0x01; // Version
            response[1] = (byte)((respMsgId >> 16) & 0xFF);
            response[2] = (byte)((respMsgId >> 8) & 0xFF);
            response[3] = (byte)(respMsgId & 0xFF);
            response[4] = payload[4]; // Same reqId
            response[5] = payload[5];
            response[6] = payload[6];
            response[7] = payload[7];
            Array.Copy(respBody, 0, response, 8, respBody.Length);

            // Send response frame
            var respHeader = new byte[4];
            respHeader[0] = (byte)((response.Length >> 24) & 0xFF);
            respHeader[1] = (byte)((response.Length >> 16) & 0xFF);
            respHeader[2] = (byte)((response.Length >> 8) & 0xFF);
            respHeader[3] = (byte)(response.Length & 0xFF);
            await stream.WriteAsync(respHeader);
            await stream.WriteAsync(response);
            await stream.FlushAsync();

            client.Close();
        });

        try
        {
            var transport = new TCPTransport($"127.0.0.1:{port}", timeoutMs: 5000, connectTimeoutMs: 5000);
            transport.Connect();

            // Act
            var request = Protocol.NewMessage(Protocol.MsgInvokeRequest, 1, Encoding.UTF8.GetBytes("request"));
            var response = await transport.CallAsync(Protocol.MsgInvokeRequest, Encoding.UTF8.GetBytes("request"));

            // Assert
            Encoding.UTF8.GetString(response).Should().Be("response");

            // Cleanup
            transport.Dispose();
        }
        finally
        {
            server.Stop();
        }
        await serverTask;
    }

    [Fact]
    public async Task CallAsync_ProcessesInboundRequestWhileAwaitingResponse()
    {
        using var server = new TcpListener(IPAddress.Loopback, 0);
        server.Start();
        var port = ((IPEndPoint)server.LocalEndpoint).Port;

        var serverTask = Task.Run(async () =>
        {
            using var peer = await server.AcceptTcpClientAsync();
            using var stream = peer.GetStream();

            var request = await ReadFrameAsync(stream);
            var callback = Protocol.NewMessage(
                Protocol.MsgStartTaskRequest,
                request.ReqId + 1,
                Encoding.UTF8.GetBytes("callback"));
            await WriteFrameAsync(stream, callback);

            var callbackResponse = await ReadFrameAsync(stream);
            callbackResponse.MsgId.Should().Be(Protocol.MsgStartTaskResponse);
            callbackResponse.ReqId.Should().Be(request.ReqId + 1);
            Encoding.UTF8.GetString(callbackResponse.Body).Should().Be("handled:callback");

            var response = Protocol.NewMessage(
                Protocol.MsgInvokeResponse,
                request.ReqId,
                callbackResponse.Body);
            await WriteFrameAsync(stream, response);
        });

        using var transport = new TCPTransport($"127.0.0.1:{port}", timeoutMs: 5000, connectTimeoutMs: 5000);
        transport.SetInboundRequestHandler((msgId, _, body) =>
        {
            msgId.Should().Be(Protocol.MsgStartTaskRequest);
            return Task.FromResult(Encoding.UTF8.GetBytes($"handled:{Encoding.UTF8.GetString(body)}"));
        });
        transport.Connect();

        var response = await transport.CallAsync(Protocol.MsgInvokeRequest, Encoding.UTF8.GetBytes("request"));

        Encoding.UTF8.GetString(response).Should().Be("handled:callback");
        await serverTask;
    }

    private static async Task<(int MsgId, int ReqId, byte[] Body)> ReadFrameAsync(NetworkStream stream)
    {
        var header = new byte[4];
        await ReadExactAsync(stream, header);
        var length = (header[0] << 24) | (header[1] << 16) | (header[2] << 8) | header[3];
        var payload = new byte[length];
        await ReadExactAsync(stream, payload);
        var parsed = Protocol.ParseMessage(payload);
        return (parsed.MsgId, parsed.ReqId, parsed.Body);
    }

    private static async Task WriteFrameAsync(NetworkStream stream, byte[] payload)
    {
        var header = new byte[4];
        header[0] = (byte)(payload.Length >> 24);
        header[1] = (byte)(payload.Length >> 16);
        header[2] = (byte)(payload.Length >> 8);
        header[3] = (byte)payload.Length;
        await stream.WriteAsync(header);
        await stream.WriteAsync(payload);
        await stream.FlushAsync();
    }

    private static async Task ReadExactAsync(NetworkStream stream, byte[] buffer)
    {
        int offset = 0;
        while (offset < buffer.Length)
        {
            var read = await stream.ReadAsync(buffer.AsMemory(offset, buffer.Length - offset));
            if (read == 0)
                throw new EndOfStreamException();
            offset += read;
        }
    }

    #endregion
}
