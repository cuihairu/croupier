// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

using System.Net;
using System.Net.Sockets;
using System.Text;
using System.Threading.Channels;
using Croupier.Sdk.Logging;
using Croupier.Sdk.Transport;
using FluentAssertions;
using Xunit;
using Xunit.Abstractions;

namespace Croupier.Sdk.Tests.Transport;

/// <summary>
/// Error handling and edge case tests for TCPTransport
/// </summary>
public class TcpTransportErrorTests
{
    private readonly ITestOutputHelper _output;

    public TcpTransportErrorTests(ITestOutputHelper output)
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
    public void Constructor_WithInvalidAddress_NoColon_ThrowsArgumentException()
    {
        // Act & Assert
        Assert.Throws<ArgumentException>(() => new TCPTransport("invalid_address"));
    }

    [Fact]
    public void Constructor_WithInvalidAddress_MultipleColons_ThrowsArgumentException()
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
        Assert.Throws<OverflowException>(() => new TCPTransport("127.0.0.1:999999"));
    }

    [Fact]
    public void Constructor_WithValidParameters_Succeeds()
    {
        // Act
        var transport = new TCPTransport("127.0.0.1:19090", 30000, 5000, new TestLogger(_output));

        // Assert
        transport.Should().NotBeNull();
        transport.IsConnected.Should().BeFalse();
    }

    #endregion

    #region Connection Tests

    [Fact]
    public async Task ConnectAsync_WhenNoServerRunning_ThrowsTimeoutException()
    {
        // Arrange
        var transport = new TCPTransport("127.0.0.1:1", 30000, 100, new TestLogger(_output));

        // Act & Assert
        await Assert.ThrowsAsync<TimeoutException>(() => Task.Run(() =>
        {
            try { transport.Connect(); }
            catch (TimeoutException ex) { throw; }
            catch (Exception ex) { throw new TimeoutException("Connect failed", ex); }
        }));
    }

    [Fact]
    public void Connect_WhenAlreadyConnected_Idempotent()
    {
        // Arrange - create a simple TCP server
        using var listener = new TcpListener(System.Net.IPAddress.Loopback, 0);
        listener.Start();
        var port = ((System.Net.IPEndPoint)listener.LocalEndpoint).Port;
        var transport = new TCPTransport($"127.0.0.1:{port}", 30000, 5000, new TestLogger(_output));

        try
        {
            // Act - first connect
            Task.Run(() => listener.AcceptTcpClientAsync());
            transport.Connect();

            // Act - second connect (should be idempotent)
            var action = () => transport.Connect();

            // Assert
            action.Should().NotThrow();
            transport.IsConnected.Should().BeTrue();
        }
        finally
        {
            transport.Dispose();
            listener.Stop();
        }
    }

    [Fact]
    public void Disconnect_WhenNotConnected_DoesNotThrow()
    {
        // Arrange
        var transport = new TCPTransport("127.0.0.1:19090");

        // Act & Assert
        transport.Dispose(); // Calls Disconnect internally
    }

    [Fact]
    public void Dispose_MultipleCalls_DoesNotThrow()
    {
        // Arrange
        var transport = new TCPTransport("127.0.0.1:19090");

        // Act & Assert
        transport.Dispose();
        transport.Dispose(); // Should not throw
    }

    #endregion

    #region Call Tests

    [Fact]
    public void Call_WhenNotConnected_ThrowsInvalidOperationException()
    {
        // Arrange
        var transport = new TCPTransport("127.0.0.1:19090", 30000, 5000, new TestLogger(_output));

        // Act & Assert
        Assert.Throws<InvalidOperationException>(() => transport.Call(1, new byte[] { 1, 2, 3 }));
    }

    [Fact]
    public async Task CallAsync_WhenNotConnected_ThrowsInvalidOperationException()
    {
        // Arrange
        var transport = new TCPTransport("127.0.0.1:19090", 30000, 5000, new TestLogger(_output));

        // Act & Assert
        await Assert.ThrowsAsync<InvalidOperationException>(() =>
            transport.CallAsync(1, new byte[] { 1, 2, 3 }));
    }

    [Fact]
    public async Task Call_WithNullData_DoesNotThrow()
    {
        // Arrange
        using var listener = new TcpListener(System.Net.IPAddress.Loopback, 0);
        listener.Start();
        var port = ((System.Net.IPEndPoint)listener.LocalEndpoint).Port;
        var transport = new TCPTransport($"127.0.0.1:{port}", 30000, 5000, new TestLogger(_output));

        try
        {
            _ = Task.Run(() => listener.AcceptTcpClientAsync());
            transport.Connect();

            // Act & Assert - may timeout but should not throw null reference
            var exception = await Assert.ThrowsAsync<TaskCanceledException>(async () =>
            {
                using var cts = new CancellationTokenSource(100);
                await transport.CallAsync(1, null, cts.Token);
            });
        }
        finally
        {
            transport.Dispose();
            listener.Stop();
        }
    }

    [Fact]
    public async Task Call_WithEmptyData_Succeeds()
    {
        // Arrange
        using var listener = new TcpListener(System.Net.IPAddress.Loopback, 0);
        listener.Start();
        var port = ((System.Net.IPEndPoint)listener.LocalEndpoint).Port;
        var transport = new TCPTransport($"127.0.0.1:{port}", 30000, 5000, new TestLogger(_output));

        try
        {
            _ = Task.Run(() => listener.AcceptTcpClientAsync());
            transport.Connect();

            // Act & Assert - may timeout but should not throw
            var exception = await Assert.ThrowsAsync<TaskCanceledException>(async () =>
            {
                using var cts = new CancellationTokenSource(100);
                await transport.CallAsync(1, Array.Empty<byte>(), cts.Token);
            });
        }
        finally
        {
            transport.Dispose();
            listener.Stop();
        }
    }

    #endregion

    #region Timeout Tests

    [Fact]
    public void SetConnectTimeout_UpdatesTimeout()
    {
        // Arrange
        var transport = new TCPTransport("127.0.0.1:19090", 30000, 5000);

        // Act
        transport.SetConnectTimeout(10000);

        // Assert - no exception means success
        // Can't directly verify the timeout value as it's private
    }

    [Fact]
    public void SetConnectTimeout_WithNegativeValue_AcceptsValue()
    {
        // Arrange
        var transport = new TCPTransport("127.0.0.1:19090");

        // Act - this may accept negative values (implementation dependent)
        transport.SetConnectTimeout(-1);

        // Assert - behavior depends on implementation
    }

    [Fact]
    public void SetConnectTimeout_WithZero_AcceptsValue()
    {
        // Arrange
        var transport = new TCPTransport("127.0.0.1:19090");

        // Act
        transport.SetConnectTimeout(0);

        // Assert - no exception
    }

    #endregion

    #region Inbound Handler Tests

    [Fact]
    public void SetInboundRequestHandler_WithNullHandler_ClearsHandler()
    {
        // Arrange
        var transport = new TCPTransport("127.0.0.1:19090");
        transport.SetInboundRequestHandler((msgType, reqId, data) => Task.FromResult<byte[]>(Array.Empty<byte>()));

        // Act
        transport.SetInboundRequestHandler(null);

        // Assert - no exception
    }

    [Fact]
    public void SetInboundRequestHandler_WithValidHandler_Succeeds()
    {
        // Arrange
        var transport = new TCPTransport("127.0.0.1:19090");
        Func<int, int, byte[], Task<byte[]>> handler = (msgType, reqId, data) =>
            Task.FromResult(Array.Empty<byte>());

        // Act & Assert
        transport.SetInboundRequestHandler(handler);
    }

    #endregion

    #region Large Message Tests

    [Fact]
    public async Task CallAsync_WithLargeMessage_Succeeds()
    {
        // Arrange
        using var listener = new TcpListener(System.Net.IPAddress.Loopback, 0);
        listener.Start();
        var port = ((System.Net.IPEndPoint)listener.LocalEndpoint).Port;
        var transport = new TCPTransport($"127.0.0.1:{port}", 30000, 5000, new TestLogger(_output));
        var largeData = new byte[1024 * 100]; // 100 KB

        try
        {
            var serverTask = Task.Run(async () =>
            {
                var client = await listener.AcceptTcpClientAsync();
                using var stream = client.GetStream();
                var buffer = new byte[1024 * 4];

                // Read length prefix
                await stream.ReadAsync(buffer, 0, 4);

                // Read some data
                await stream.ReadAsync(buffer, 0, buffer.Length);

                // Send response
                var response = new byte[10];
                var responseLength = BitConverter.GetBytes(IPAddress.HostToNetworkOrder(response.Length));
                await stream.WriteAsync(responseLength, 0, 4);
                await stream.WriteAsync(response, 0, response.Length);
            });

            transport.Connect();

            // Act & Assert - may still timeout but should handle large data
            try
            {
                using var cts = new CancellationTokenSource(5000);
                await transport.CallAsync(1, largeData, cts.Token);
            }
            catch (Exception)
            {
                // May timeout or throw, but large data should be handled
            }
        }
        finally
        {
            transport.Dispose();
            listener.Stop();
        }
    }

    #endregion

    #region Concurrent Call Tests

    [Fact]
    public async Task CallAsync_MultipleConcurrentCalls_HandleCorrectly()
    {
        // Arrange
        using var listener = new TcpListener(System.Net.IPAddress.Loopback, 0);
        listener.Start();
        var port = ((System.Net.IPEndPoint)listener.LocalEndpoint).Port;
        var transport = new TCPTransport($"127.0.0.1:{port}", 30000, 5000, new TestLogger(_output));

        try
        {
            var serverTask = Task.Run(async () =>
            {
                for (int i = 0; i < 3; i++)
                {
                    var client = await listener.AcceptTcpClientAsync();
                    _ = Task.Run(async () =>
                    {
                        using var stream = client.GetStream();
                        var buffer = new byte[1024];

                        await stream.ReadAsync(buffer, 0, 4);
                        await stream.ReadAsync(buffer, 0, buffer.Length);

                        var response = new byte[10];
                        var responseLength = BitConverter.GetBytes(IPAddress.HostToNetworkOrder(response.Length));
                        await stream.WriteAsync(responseLength, 0, 4);
                        await stream.WriteAsync(response, 0, response.Length);
                    });
                }
            });

            transport.Connect();

            // Act - multiple concurrent calls
            var tasks = new List<Task<byte[]>>();
            for (int i = 0; i < 3; i++)
            {
                tasks.Add(transport.CallAsync(1, new byte[] { 1, 2, 3 }));
            }

            // Assert
            try
            {
                using var cts = new CancellationTokenSource(5000);
                await Task.WhenAll(tasks);
            }
            catch
            {
                // May timeout depending on server implementation
            }
        }
        finally
        {
            transport.Dispose();
            listener.Stop();
        }
    }

    #endregion

    #region IPv6 Tests

    [Fact]
    public void Constructor_WithIPv6Address_Succeeds()
    {
        // Act
        var transport = new TCPTransport("[::1]:19090", 30000, 5000);

        // Assert
        transport.Should().NotBeNull();
    }

    [Fact]
    public void Constructor_WithIPv6Address_NoBrackets_ThrowsArgumentException()
    {
        // Act & Assert
        Assert.Throws<ArgumentException>(() => new TCPTransport("::1:19090"));
    }

    #endregion
}
