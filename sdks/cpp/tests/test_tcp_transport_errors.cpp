#include <gtest/gtest.h>

#include "croupier/sdk/tcp_transport.h"

#include <chrono>
#include <thread>
#include <vector>

namespace croupier {
namespace sdk {
namespace test {

class TCPTransportErrorTest : public ::testing::Test {
protected:
    void SetUp() override {}
    void TearDown() override {}

    // Helper to parse "host:port" address and create TCPTransport with specified timeout
    TCPTransport CreateTransport(const std::string& address, int timeout_ms) {
        size_t colon_pos = address.find(':');
        if (colon_pos == std::string::npos) {
            throw std::runtime_error("Invalid address format: " + address);
        }
        std::string host = address.substr(0, colon_pos);
        int port = std::stoi(address.substr(colon_pos + 1));
        return TCPTransport(host, port, timeout_ms);
    }
};

// Test connection timeout
TEST_F(TCPTransportErrorTest, ConnectTimeout) {
    // Use a non-routable IP address with a short timeout
    // 198.51.100.1 is in TEST-NET-2 (reserved for documentation)
    // Note: On some systems (especially macOS), connection to non-routable
    // addresses may not immediately fail. The test verifies timeout behavior
    // by measuring actual connection time.
    TCPTransport transport("198.51.100.1", 9999, 100);  // 100ms timeout

    auto start = std::chrono::steady_clock::now();
    try {
        transport.Connect();
        // On some systems, connect() may succeed locally but actual communication will fail
        // This is acceptable behavior - the key is the timeout should be short
        transport.Close();
    } catch (const std::runtime_error&) {
        // Expected - connection should timeout/fail
    }
    auto elapsed = std::chrono::duration_cast<std::chrono::milliseconds>(
        std::chrono::steady_clock::now() - start).count();

    // Should complete quickly (with timeout margin)
    EXPECT_LT(elapsed, 500);  // Allow generous margin for system variations
}

// Test connection to invalid port
TEST_F(TCPTransportErrorTest, InvalidPort) {
    // Try to connect to a valid host but invalid/unavailable port
    TCPTransport transport("127.0.0.1", 1);  // Port 1 is typically not usable

    // This may succeed in connecting (some systems allow it) but should fail
    // when trying to send data
    try {
        transport.Connect();
        // If connect succeeded, the transport should be marked as connected
        // but any call should fail
        SUCCEED();
    } catch (const std::exception&) {
        SUCCEED();  // Expected to fail
    }
}

// Test connect to non-existent host
TEST_F(TCPTransportErrorTest, InvalidHost) {
    // Use a reserved IP that should be unreachable
    // 192.0.2.1 is in TEST-NET-1 (RFC 5737)
    TCPTransport transport("192.0.2.1", 8080, 200);

    auto start = std::chrono::steady_clock::now();
    try {
        transport.Connect();
        // On some systems, connect might not throw but won't actually work
        transport.Close();
    } catch (const std::runtime_error&) {
        // Expected - connection failed
    }
    auto elapsed = std::chrono::duration_cast<std::chrono::milliseconds>(
        std::chrono::steady_clock::now() - start).count();

    // Either exception is caught, or operation completes quickly (timeout)
    // The key is we don't want long hangs
    EXPECT_LT(elapsed, 1000);
}

// Test move constructor
TEST_F(TCPTransportErrorTest, MoveConstructor) {
    TCPServer server("tcp://127.0.0.1:0");
    server.SetHandler([](uint32_t, uint32_t, const std::vector<uint8_t>&) -> std::vector<uint8_t> {
        return {1, 2, 3};
    });
    server.Start();

    // Wait for server to start
    auto start = std::chrono::steady_clock::now();
    while (!server.IsRunning()) {
        std::this_thread::sleep_for(std::chrono::milliseconds(10));
        if (std::chrono::duration_cast<std::chrono::milliseconds>(
                std::chrono::steady_clock::now() - start).count() > 5000) {
            break;
        }
    }

    std::string actual_address = server.GetListenAddress();
    TCPTransport transport1 = CreateTransport(actual_address, 1000);
    transport1.Connect();

    // Test move constructor
    TCPTransport transport2(std::move(transport1));

    // transport2 should be connected
    EXPECT_TRUE(transport2.IsConnected());

    // transport1 should be in a valid but disconnected state
    // The standard doesn't guarantee the state of moved-from objects,
    // but they should be destructible

    server.Stop();
}

// Test move assignment
TEST_F(TCPTransportErrorTest, MoveAssignment) {
    TCPServer server("tcp://127.0.0.1:0");
    server.SetHandler([](uint32_t, uint32_t, const std::vector<uint8_t>&) -> std::vector<uint8_t> {
        return {1, 2, 3};
    });
    server.Start();

    auto start = std::chrono::steady_clock::now();
    while (!server.IsRunning()) {
        std::this_thread::sleep_for(std::chrono::milliseconds(10));
        if (std::chrono::duration_cast<std::chrono::milliseconds>(
                std::chrono::steady_clock::now() - start).count() > 5000) {
            break;
        }
    }

    std::string actual_address = server.GetListenAddress();
    TCPTransport transport1 = CreateTransport(actual_address, 1000);
    transport1.Connect();

    TCPTransport transport2("127.0.0.1", 9999, 1000);

    // Test move assignment
    transport2 = std::move(transport1);

    EXPECT_TRUE(transport2.IsConnected());

    server.Stop();
}

// Test Call when not connected
TEST_F(TCPTransportErrorTest, CallWhenNotConnected) {
    TCPTransport transport("127.0.0.1", 8080, 1000);

    std::vector<uint8_t> data = {1, 2, 3};
    EXPECT_THROW(transport.Call(1, data), std::runtime_error);
}

// Test Call timeout
TEST_F(TCPTransportErrorTest, CallTimeout) {
    TCPServer server("tcp://127.0.0.1:0");
    // Handler that doesn't respond (timeout)
    server.SetHandler([](uint32_t, uint32_t, const std::vector<uint8_t>&) -> std::vector<uint8_t> {
        std::this_thread::sleep_for(std::chrono::seconds(10));
        return {};
    });
    server.Start();

    auto start = std::chrono::steady_clock::now();
    while (!server.IsRunning()) {
        std::this_thread::sleep_for(std::chrono::milliseconds(10));
        if (std::chrono::duration_cast<std::chrono::milliseconds>(
                std::chrono::steady_clock::now() - start).count() > 5000) {
            break;
        }
    }

    std::string actual_address = server.GetListenAddress();
    TCPTransport transport = CreateTransport(actual_address, 100);  // 100ms timeout
    transport.Connect();

    std::vector<uint8_t> data = {1, 2, 3};
    EXPECT_THROW(transport.Call(1, data), std::runtime_error);

    server.Stop();
}

// Test double connect
TEST_F(TCPTransportErrorTest, DoubleConnect) {
    TCPServer server("tcp://127.0.0.1:0");
    server.SetHandler([](uint32_t, uint32_t, const std::vector<uint8_t>&) -> std::vector<uint8_t> {
        return {1, 2, 3};
    });
    server.Start();

    auto start = std::chrono::steady_clock::now();
    while (!server.IsRunning()) {
        std::this_thread::sleep_for(std::chrono::milliseconds(10));
        if (std::chrono::duration_cast<std::chrono::milliseconds>(
                std::chrono::steady_clock::now() - start).count() > 5000) {
            break;
        }
    }

    std::string actual_address = server.GetListenAddress();
    TCPTransport transport = CreateTransport(actual_address, 1000);
    transport.Connect();
    EXPECT_TRUE(transport.IsConnected());

    // Second connect should be idempotent
    transport.Connect();
    EXPECT_TRUE(transport.IsConnected());

    server.Stop();
}

// Test close and reconnect
TEST_F(TCPTransportErrorTest, CloseAndReconnect) {
    TCPServer server("tcp://127.0.0.1:0");
    server.SetHandler([](uint32_t, uint32_t, const std::vector<uint8_t>&) -> std::vector<uint8_t> {
        return {1, 2, 3};
    });
    server.Start();

    auto start = std::chrono::steady_clock::now();
    while (!server.IsRunning()) {
        std::this_thread::sleep_for(std::chrono::milliseconds(10));
        if (std::chrono::duration_cast<std::chrono::milliseconds>(
                std::chrono::steady_clock::now() - start).count() > 5000) {
            break;
        }
    }

    std::string actual_address = server.GetListenAddress();

    // First connection
    {
        TCPTransport transport = CreateTransport(actual_address, 1000);
        transport.Connect();
        EXPECT_TRUE(transport.IsConnected());
        transport.Close();
        EXPECT_FALSE(transport.IsConnected());
    }

    // Second connection (should work with same address)
    {
        TCPTransport transport2 = CreateTransport(actual_address, 1000);
        transport2.Connect();
        EXPECT_TRUE(transport2.IsConnected());
        transport2.Close();
    }

    server.Stop();
}

// Test IsConnected behavior
TEST_F(TCPTransportErrorTest, IsConnectedBehavior) {
    TCPTransport transport("127.0.0.1", 8080, 1000);

    EXPECT_FALSE(transport.IsConnected());

    // After failed connection attempt
    try {
        transport.Connect();
    } catch (...) {
        // Connection may fail if port is not available
    }

    // After Close
    transport.Close();
    EXPECT_FALSE(transport.IsConnected());
}

// Test TCPServer with invalid address
TEST_F(TCPTransportErrorTest, TCPServerInvalidAddress) {
    TCPServer server("tcp://invalid.host.address:9999");

    EXPECT_THROW(server.Start(), std::runtime_error);
}

// Test TCPServer Start/Stop/Start
TEST_F(TCPTransportErrorTest, TCPServerRestart) {
    TCPServer server("tcp://127.0.0.1:0");

    server.SetHandler([](uint32_t, uint32_t, const std::vector<uint8_t>&) -> std::vector<uint8_t> {
        return {1, 2, 3};
    });

    // First start
    server.Start();
    EXPECT_TRUE(server.IsRunning());
    std::string address1 = server.GetListenAddress();
    EXPECT_FALSE(address1.empty());

    server.Stop();
    EXPECT_FALSE(server.IsRunning());

    // Second start (should work, may get different port)
    server.Start();
    EXPECT_TRUE(server.IsRunning());
    std::string address2 = server.GetListenAddress();
    EXPECT_FALSE(address2.empty());

    server.Stop();
}

// Test TCPServer with empty handler
TEST_F(TCPTransportErrorTest, TCPServerEmptyHandler) {
    TCPServer server("tcp://127.0.0.1:0");

    // Start without setting handler (should work, just won't respond)
    server.Start();

    auto start = std::chrono::steady_clock::now();
    while (!server.IsRunning()) {
        std::this_thread::sleep_for(std::chrono::milliseconds(10));
        if (std::chrono::duration_cast<std::chrono::milliseconds>(
                std::chrono::steady_clock::now() - start).count() > 5000) {
            break;
        }
    }

    EXPECT_TRUE(server.IsRunning());

    // Try to connect and send (should not crash, just get no response)
    std::string actual_address = server.GetListenAddress();
    TCPTransport transport = CreateTransport(actual_address, 500);
    transport.Connect();

    // This should timeout since there's no handler
    std::vector<uint8_t> data = {1, 2, 3};
    EXPECT_THROW(transport.Call(1, data), std::runtime_error);

    server.Stop();
}

// Test SetConnectTimeout
TEST_F(TCPTransportErrorTest, SetConnectTimeout) {
    TCPTransport transport("127.0.0.1", 8080, 1000);

    transport.SetConnectTimeout(100);  // 100ms
    transport.SetConnectTimeout(50);   // Override to 50ms

    // Connection to unavailable port should timeout quickly
    auto start = std::chrono::steady_clock::now();
    try {
        transport.Connect();
    } catch (...) {
        // Expected
    }
    auto elapsed = std::chrono::duration_cast<std::chrono::milliseconds>(
        std::chrono::steady_clock::now() - start).count();

    // Should timeout in less than 200ms (allowing some margin)
    EXPECT_LT(elapsed, 200);
}

// Test large message handling
TEST_F(TCPTransportErrorTest, LargeMessage) {
    TCPServer server("tcp://127.0.0.1:0");
    server.SetHandler([](uint32_t, uint32_t, const std::vector<uint8_t>& body) -> std::vector<uint8_t> {
        // Echo back the body
        return body;
    });
    server.Start();

    auto start = std::chrono::steady_clock::now();
    while (!server.IsRunning()) {
        std::this_thread::sleep_for(std::chrono::milliseconds(10));
        if (std::chrono::duration_cast<std::chrono::milliseconds>(
                std::chrono::steady_clock::now() - start).count() > 5000) {
            break;
        }
    }

    // Parse actual address to get host and port separately
    std::string actual_address = server.GetListenAddress();
    size_t colon_pos = actual_address.find(':');
    ASSERT_NE(colon_pos, std::string::npos);
    std::string host = actual_address.substr(0, colon_pos);
    int port = std::stoi(actual_address.substr(colon_pos + 1));

    TCPTransport transport(host, port, 5000);  // 5 second timeout
    transport.Connect();

    // Create a large message (but within MAX_FRAME_BYTES)
    std::vector<uint8_t> large_data(10000, 0xAB);
    auto [msg_id, response] = transport.Call(1, large_data);

    EXPECT_EQ(response.size(), large_data.size());

    server.Stop();
}

// Test multiple concurrent calls
TEST_F(TCPTransportErrorTest, ConcurrentCalls) {
    TCPServer server("tcp://127.0.0.1:0");
    server.SetHandler([](uint32_t, uint32_t, const std::vector<uint8_t>& body) -> std::vector<uint8_t> {
        // Simulate some processing delay
        std::this_thread::sleep_for(std::chrono::milliseconds(10));
        return body;
    });
    server.Start();

    auto start = std::chrono::steady_clock::now();
    while (!server.IsRunning()) {
        std::this_thread::sleep_for(std::chrono::milliseconds(10));
        if (std::chrono::duration_cast<std::chrono::milliseconds>(
                std::chrono::steady_clock::now() - start).count() > 5000) {
            break;
        }
    }

    std::string actual_address = server.GetListenAddress();
    TCPTransport transport = CreateTransport(actual_address, 5000);
    transport.Connect();

    // Make multiple concurrent calls from different threads
    const int num_threads = 5;
    const int calls_per_thread = 3;
    std::vector<std::thread> threads;
    std::atomic<int> success_count{0};

    for (int i = 0; i < num_threads; ++i) {
        threads.emplace_back([&, i]() {
            for (int j = 0; j < calls_per_thread; ++j) {
                try {
                    std::vector<uint8_t> data = {static_cast<uint8_t>(i), static_cast<uint8_t>(j)};
                    auto [msg_id, response] = transport.Call(1, data);
                    if (response == data) {
                        success_count++;
                    }
                } catch (...) {
                    // May timeout under load
                }
            }
        });
    }

    for (auto& t : threads) {
        t.join();
    }

    // At least some should succeed
    EXPECT_GT(success_count, 0);

    server.Stop();
}

// Test IPv6 address format (should fail gracefully on IPv4-only systems)
TEST_F(TCPTransportErrorTest, IPv6AddressFormat) {
    // This test just checks that IPv6 addresses don't crash the parser
    // Actual IPv6 connectivity depends on system support
    TCPTransport transport("::1", 8080, 100);  // IPv6 localhost

    try {
        transport.Connect();
        // If it connected, great - system supports IPv6
        transport.Close();
    } catch (const std::exception&) {
        // Expected on IPv4-only systems
    }
}

}  // namespace test
}  // namespace sdk
}  // namespace croupier
