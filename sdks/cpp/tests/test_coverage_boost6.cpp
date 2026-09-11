// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

// Sixth coverage boost: white-box TU. This translation unit re-compiles
// selected SDK .cpp files (croupier_client / client_config_loader /
// dynamic_loader) so that otherwise-unreachable internal helpers can be
// driven directly, and uses relaxed access to reach deterministic state
// choreography (drain recovery, reconnect guards, heartbeat stop paths,
// Call-vs-closing races). Production code is untouched; every test still
// asserts real observable behavior (return values, exception types and
// messages).

#include <gtest/gtest.h>

// System / third-party headers with private sections are included BEFORE the
// access relaxation so only SDK headers are affected.
#include <nlohmann/json.hpp>
#include <openssl/sha.h>
#include <zlib.h>

#include "croupier/sdk/v1/invocation.pb.h"
#include "croupier/sdk/v1/provider.pb.h"
#include "croupier/agent/v1/register.pb.h"

#ifdef __GLIBC__
#include <malloc.h>
#endif

#include <algorithm>
#include <atomic>
#include <chrono>
#include <cstdio>
#include <cstring>
#include <functional>
#include <memory>
#include <mutex>
#include <string>
#include <thread>
#include <vector>

#ifdef _WIN32
#include <winsock2.h>
#include <ws2tcpip.h>
using raw_socket_t = SOCKET;
#define RAW_INVALID_SOCK INVALID_SOCKET
#define raw_closesocket ::closesocket
#else
#include <arpa/inet.h>
#include <fcntl.h>
#include <netinet/in.h>
#include <signal.h>
#include <sys/resource.h>
#include <sys/socket.h>
#include <sys/wait.h>
#include <unistd.h>
using raw_socket_t = int;
#define RAW_INVALID_SOCK (-1)
#define raw_closesocket ::close
#endif

// Relax access for the SDK surface compiled in this TU only. Layout is
// unaffected (Itanium ABI is access-independent), and inline definitions
// still compile to identical symbols, so the archive members providing the
// same symbols are simply not pulled by the linker.
#define private public
#include "croupier/sdk/tcp_transport.h"
#include "croupier/sdk/croupier_client.h"
#include "croupier/sdk/config/client_config_loader.h"
#include "croupier/sdk/plugin/dynamic_loader.h"
#include "croupier/sdk/http_transport.h"
#include "croupier/sdk/file_push.h"
#include "croupier/sdk/logger.h"
#undef private

#include "src/croupier_client.cpp"
#include "src/config/client_config_loader.cpp"
#include "src/plugin/dynamic_loader.cpp"
#include "src/openapi_importer.cpp"

namespace croupier::sdk::test {
namespace {

[[maybe_unused]] bool SendAllRaw(raw_socket_t sock, const void* buf, size_t len) {
    const char* p = static_cast<const char*>(buf);
    size_t off = 0;
    while (off < len) {
        auto n = ::send(sock, p + off, len - off, 0);
        if (n <= 0) return false;
        off += static_cast<size_t>(n);
    }
    return true;
}

[[maybe_unused]] bool ReadAllRaw(raw_socket_t sock, void* buf, size_t len) {
    char* p = static_cast<char*>(buf);
    size_t off = 0;
    while (off < len) {
        auto n = ::recv(sock, p + off, len - off, 0);
        if (n <= 0) return false;
        off += static_cast<size_t>(n);
    }
    return true;
}

// Raw listener speaking the Croupier frame protocol (same style as the
// harnesses in test_coverage_boost4/5.cpp).
class Boost6FakeAgent {
public:
    Boost6FakeAgent() {
        auto fatal = [](bool ok, const char* what) {
            if (!ok) throw std::runtime_error(what);
        };
        listen_fd_ = ::socket(AF_INET, SOCK_STREAM, 0);
        fatal(listen_fd_ != RAW_INVALID_SOCK, "socket() failed");
        sockaddr_in addr{};
        addr.sin_family = AF_INET;
        addr.sin_addr.s_addr = htonl(INADDR_LOOPBACK);
        addr.sin_port = 0;
        fatal(::bind(listen_fd_, reinterpret_cast<sockaddr*>(&addr), sizeof(addr)) == 0,
              "bind() failed");
        fatal(::listen(listen_fd_, 1) == 0, "listen() failed");
        sockaddr_in bound{};
        socklen_t blen = sizeof(bound);
        fatal(::getsockname(listen_fd_, reinterpret_cast<sockaddr*>(&bound), &blen) == 0,
              "getsockname() failed");
        port_ = ntohs(bound.sin_port);
    }

    ~Boost6FakeAgent() {
        DropConnection();
        if (listen_fd_ != RAW_INVALID_SOCK) raw_closesocket(listen_fd_);
    }

    unsigned short port() const { return static_cast<unsigned short>(port_); }

    void AcceptOnly() {
        conn_ = ::accept(listen_fd_, nullptr, nullptr);
        ASSERT_NE(conn_, RAW_INVALID_SOCK);
    }

    void DropConnection() {
        if (conn_ != RAW_INVALID_SOCK) {
            raw_closesocket(conn_);
            conn_ = RAW_INVALID_SOCK;
        }
    }

private:
    raw_socket_t listen_fd_{RAW_INVALID_SOCK};
    raw_socket_t conn_{RAW_INVALID_SOCK};
    int port_{0};
};

ClientConfig Boost6ProviderConfig(const std::string& addr) {
    ClientConfig config;
    config.game_id = "game-boost6";
    config.env = "development";
    config.service_id = "cpp-boost6";
    config.agent_addr = addr;
    config.timeout_seconds = 2;
    config.connect_timeout_seconds = 1;
    config.heartbeat_interval = 30;
    config.disable_logging = true;
    return config;
}

// ---------------------------------------------------------------------------
// 1) Anonymous-namespace helpers in croupier_client.cpp that no longer have
//    production callers (the legacy TCP invoker is #if 0'd out): drive them
//    directly through this TU's copy and assert their documented mapping.
// ---------------------------------------------------------------------------

TEST(Boost6ClientHelpersTest, IsTCPAddressClassification) {
    EXPECT_FALSE(IsTCPAddress(""));
    EXPECT_TRUE(IsTCPAddress("127.0.0.1:19091"));
    EXPECT_TRUE(IsTCPAddress("host"));
    EXPECT_TRUE(IsTCPAddress("tcp://10.0.0.1:9000"));
    EXPECT_FALSE(IsTCPAddress("udp://10.0.0.1:9000"));
    EXPECT_FALSE(IsTCPAddress("http://api.example.com"));
}

TEST(Boost6ClientHelpersTest, NormalizeProviderTaskEventTypeMapping) {
    v1::TaskEvent done;
    done.set_type("done");
    EXPECT_EQ(NormalizeProviderTaskEventType(done), "completed");

    v1::TaskEvent cancelled;
    cancelled.set_type("error");
    cancelled.set_message("Task CANCELLED by operator");
    EXPECT_EQ(NormalizeProviderTaskEventType(cancelled), "cancelled");

    v1::TaskEvent errored;
    errored.set_type("error");
    errored.set_message("disk on fire");
    EXPECT_EQ(NormalizeProviderTaskEventType(errored), "error");

    v1::TaskEvent progress;
    progress.set_type("progress");
    progress.set_message("still running");
    EXPECT_EQ(NormalizeProviderTaskEventType(progress), "progress");
}

TEST(Boost6ClientHelpersTest, ToTaskEventMappingAndTerminality) {
    v1::TaskEvent done;
    done.set_type("done");
    done.set_message("finished");
    const TaskEvent done_event = ToTaskEvent("task-1", done);
    EXPECT_EQ(done_event.event_type, "completed");
    EXPECT_EQ(done_event.task_id, "task-1");
    EXPECT_EQ(done_event.message, "finished");
    EXPECT_TRUE(done_event.done);
    EXPECT_TRUE(done_event.error.empty());

    v1::TaskEvent failed;
    failed.set_type("error");
    failed.set_message("boom");
    const TaskEvent failed_event = ToTaskEvent("task-2", failed);
    EXPECT_EQ(failed_event.event_type, "error");
    EXPECT_TRUE(failed_event.done);
    EXPECT_EQ(failed_event.error, "boom");

    v1::TaskEvent cancelled;
    cancelled.set_type("error");
    cancelled.set_message("cancelled by admin");
    const TaskEvent cancelled_event = ToTaskEvent("task-3", cancelled);
    EXPECT_EQ(cancelled_event.event_type, "cancelled");
    EXPECT_TRUE(cancelled_event.done);
    EXPECT_EQ(cancelled_event.error, "cancelled by admin");

    v1::TaskEvent progress;
    progress.set_type("progress");
    progress.set_progress(42);
    progress.set_payload("{\"pct\":42}");
    const TaskEvent progress_event = ToTaskEvent("task-4", progress);
    EXPECT_EQ(progress_event.event_type, "progress");
    EXPECT_EQ(progress_event.progress, 42);
    EXPECT_EQ(progress_event.payload, "{\"pct\":42}");
    EXPECT_FALSE(progress_event.done);
    EXPECT_TRUE(progress_event.error.empty());

    TaskEvent terminal;
    terminal.event_type = "completed";
    terminal.done = true;
    EXPECT_TRUE(IsTerminalTaskEvent(terminal));

    TaskEvent errored;
    errored.event_type = "error";
    EXPECT_TRUE(IsTerminalTaskEvent(errored));

    TaskEvent cancelled_terminal;
    cancelled_terminal.event_type = "cancelled";
    EXPECT_TRUE(IsTerminalTaskEvent(cancelled_terminal));

    TaskEvent running;
    running.event_type = "progress";
    EXPECT_FALSE(IsTerminalTaskEvent(running));
}

TEST(Boost6ClientHelpersTest, SameTaskEventComparisons) {
    TaskEvent lhs;
    lhs.event_type = "completed";
    lhs.task_id = "t";
    lhs.message = "m";
    lhs.progress = 7;
    lhs.payload = "p";
    lhs.error = "e";
    lhs.done = true;

    TaskEvent rhs = lhs;
    EXPECT_TRUE(SameTaskEvent(lhs, rhs));

    rhs.event_type = "error";
    EXPECT_FALSE(SameTaskEvent(lhs, rhs));
    rhs = lhs;

    rhs.task_id = "other";
    EXPECT_FALSE(SameTaskEvent(lhs, rhs));
    rhs = lhs;

    rhs.message = "other";
    EXPECT_FALSE(SameTaskEvent(lhs, rhs));
    rhs = lhs;

    rhs.progress = 8;
    EXPECT_FALSE(SameTaskEvent(lhs, rhs));
    rhs = lhs;

    rhs.payload = "other";
    EXPECT_FALSE(SameTaskEvent(lhs, rhs));
    rhs = lhs;

    rhs.error = "other";
    EXPECT_FALSE(SameTaskEvent(lhs, rhs));
    rhs = lhs;

    rhs.done = false;
    EXPECT_FALSE(SameTaskEvent(lhs, rhs));
}

TEST(Boost6ClientHelpersTest, EscapeHelpersOutput) {
    EXPECT_EQ(EscapeURLSegment("player id#1"), "player%20id%231");
    EXPECT_EQ(EscapeURLSegment("a/b"), "a%2Fb");
    EXPECT_EQ(EscapeURLSegment("safe.-_~"), "safe.-_~");
    EXPECT_EQ(EscapeURLSegment("\xFF"), "%FF");

    EXPECT_EQ(EscapeJsonString("a\\b\"c\nd\re\tf"), "a\\\\b\\\"c\\nd\\re\\tf");
    EXPECT_EQ(EscapeJsonString("plain"), "plain");
}

TEST(Boost6ClientHelpersTest, SerializeMessageRoundTrip) {
    v1::InvokeResponse resp;
    resp.set_payload("{\"ok\":true}");
    const std::vector<uint8_t> bytes = SerializeMessage(resp);
    ASSERT_FALSE(bytes.empty());
    v1::InvokeResponse parsed;
    ASSERT_TRUE(parsed.ParseFromArray(bytes.data(), static_cast<int>(bytes.size())));
    EXPECT_EQ(parsed.payload(), "{\"ok\":true}");
}

TEST(Boost6ClientHelpersTest, SerializeMessageThrowsWhenMessageExceedsWireLimit) {
    // libprotobuf refuses to serialize messages above the 2GB wire limit;
    // SerializeMessage must surface that as a runtime_error.
    v1::InvokeResponse resp;
    std::string huge(2150000000, 'x');
    resp.set_payload(std::move(huge));
    try {
        const std::vector<uint8_t> bytes = SerializeMessage(resp);
        (void)bytes;
        FAIL() << "SerializeMessage must throw above the 2GB wire limit";
    } catch (const std::runtime_error& e) {
        EXPECT_STREQ(e.what(), "failed to serialize protobuf message");
    }
}

// ---------------------------------------------------------------------------
// 2) CroupierClient internal state choreography (deterministic white-box).
// ---------------------------------------------------------------------------

TEST(Boost6ClientStateTest, RegisterAllFunctionsSkipsWhenDisconnected) {
    CroupierClient client(Boost6ProviderConfig("127.0.0.1:19091"));
    auto* impl = client.impl_.get();
    impl->connected_ = false;
    impl->running_ = true;
    impl->RegisterAllFunctions();
    // The disconnected guard must return before any transport is created.
    EXPECT_EQ(impl->transport_, nullptr);
    EXPECT_TRUE(impl->last_error_.empty());
}

TEST(Boost6ClientStateTest, StopJoinsHeartbeatThread) {
    CroupierClient client(Boost6ProviderConfig("127.0.0.1:19091"));
    auto* impl = client.impl_.get();
    impl->running_ = true;
    impl->startHeartbeatLoop();  // 心跳线程在 100ms 粒度睡眠循环中
    client.Stop();               // 置位 + join；死锁即测试挂起（=失败信号）
    EXPECT_FALSE(client.IsConnected());
}

TEST(Boost6ClientStateTest, SendHeartbeatThrowsWhenTransportMissing) {
    CroupierClient client(Boost6ProviderConfig("127.0.0.1:19091"));
    auto* impl = client.impl_.get();
    impl->session_id_ = "boost6-session";
    try {
        impl->sendHeartbeat();
        FAIL() << "sendHeartbeat must throw without a transport";
    } catch (const std::runtime_error& e) {
        EXPECT_STREQ(e.what(), "heartbeat transport is not connected");
    }
}

TEST(Boost6ClientStateTest, StopHeartbeatLoopSelfStopDoesNotJoinItself) {
    CroupierClient client(Boost6ProviderConfig("127.0.0.1:19091"));
    auto* impl = client.impl_.get();
    impl->should_stop_heartbeat_ = false;
    impl->heartbeat_thread_id_.reset();
    // The heartbeat thread stops itself: the self-guard must return instead
    // of joining (a self-join would terminate the process). Self-stop must
    // NOT set should_stop_heartbeat_: the reconnectLoop running on the same
    // thread would give up after its first failed attempt (2026-09-11
    // production hang).
    impl->heartbeat_thread_ = std::thread([impl] {
        impl->heartbeat_thread_id_ = std::this_thread::get_id();
        impl->stopHeartbeatLoop();
    });
    impl->heartbeat_thread_.join();
    EXPECT_FALSE(impl->should_stop_heartbeat_.load());
    impl->heartbeat_thread_id_.reset();
    impl->should_stop_heartbeat_ = false;
}

TEST(Boost6ClientStateTest, StopHeartbeatLoopJoinsFromOtherThread) {
    CroupierClient client(Boost6ProviderConfig("127.0.0.1:19091"));
    auto* impl = client.impl_.get();
    impl->heartbeat_thread_id_.reset();
    impl->should_stop_heartbeat_ = false;

    std::atomic<bool> released{false};
    impl->heartbeat_thread_ = std::thread([&released] {
        while (!released.load()) {
            std::this_thread::sleep_for(std::chrono::milliseconds(5));
        }
    });
    std::thread stopper([impl] { impl->stopHeartbeatLoop(); });
    std::this_thread::sleep_for(std::chrono::milliseconds(100));
    released.store(true);
    stopper.join();
    EXPECT_FALSE(impl->heartbeat_thread_.joinable());
    impl->should_stop_heartbeat_ = false;
}

TEST(Boost6ClientStateTest, ReconnectLoopRetriesWithBackoffUntilStopped) {
    // Port 1 has no listener on loopback: every Connect() attempt fails
    // immediately, so the loop keeps entering the 500ms back-off until the
    // stop flag flips. is_reconnecting_ must be held for the whole loop and
    // released on exit.
    CroupierClient client(Boost6ProviderConfig("127.0.0.1:1"));
    auto* impl = client.impl_.get();
    impl->running_ = true;
    impl->connected_ = false;
    impl->should_stop_heartbeat_ = false;

    std::thread looper([impl] { impl->reconnectLoop(); });

    const auto deadline = std::chrono::steady_clock::now() + std::chrono::seconds(5);
    while (!impl->is_reconnecting_.load() && std::chrono::steady_clock::now() < deadline) {
        std::this_thread::sleep_for(std::chrono::milliseconds(10));
    }
    ASSERT_TRUE(impl->is_reconnecting_.load());
    // Let the loop fail an attempt and settle in the back-off sleep.
    std::this_thread::sleep_for(std::chrono::milliseconds(200));

    impl->should_stop_heartbeat_ = true;
    const auto exit_deadline = std::chrono::steady_clock::now() + std::chrono::seconds(5);
    while (looper.joinable() && std::chrono::steady_clock::now() < exit_deadline) {
        std::this_thread::sleep_for(std::chrono::milliseconds(10));
    }
    ASSERT_TRUE(looper.joinable());
    looper.join();
    EXPECT_FALSE(impl->is_reconnecting_.load());
    client.Stop();
}

// ---------------------------------------------------------------------------
// 3) TCPTransport edges.
// ---------------------------------------------------------------------------

TEST(Boost6TCPTransportTest, SetSocketNonBlockingFailsOnClosedSocket) {
    TCPTransport transport("127.0.0.1", 19091, 500);
    transport.Close();
    try {
        transport.SetSocketNonBlocking(true);
        FAIL() << "SetSocketNonBlocking must fail on an invalid socket";
    } catch (const std::runtime_error& e) {
        EXPECT_STREQ(e.what(), "Failed to get socket flags");
    }
}

TEST(Boost6TCPTransportTest, ConnectFailsWhenSelectRejectsTimeout) {
    // A negative connect timeout produces an invalid select() timeval, which
    // fails with EINVAL once the (non-blocking) connect is in progress. The
    // address must be routable but unresponsive so connect returns
    // EINPROGRESS; 192.0.2.1 is TEST-NET-1 (reserved for documentation).
    TCPTransport transport("192.0.2.1", 80, 3000);
    transport.SetConnectTimeout(-1);
    try {
        transport.Connect();
        FAIL() << "Connect must throw for a negative select timeout";
    } catch (const std::runtime_error& e) {
        const std::string message = e.what();
        EXPECT_TRUE(message.find("select() failed during connection") != std::string::npos ||
                    message.find("Failed to connect to 192.0.2.1") != std::string::npos)
            << "unexpected error: " << message;
    }
}

TEST(Boost6TCPTransportTest, CallThrowsWhenLatchErasedBeforeLookup) {
    // The erase must land between the latch insert and the post-send lookup
    // in Call(). Which side of the lookup the erase lands on depends on
    // scheduler timing, so retry with a short Call timeout: a "Timeout"
    // outcome simply means the erase landed after the lookup this round.
    for (int attempt = 0; attempt < 10; ++attempt) {
        Boost6FakeAgent agent;
        std::thread agent_thread([&] { agent.AcceptOnly(); });

        TCPTransport transport("127.0.0.1", agent.port(), 250);
        transport.Connect();
        agent_thread.join();

        const uint32_t predicted_req_id = transport.next_req_id_.load();
        // A fat body widens the window between the latch insert and the
        // post-send lookup in Call().
        std::vector<uint8_t> fat(256 * 1024, 0x42);

        std::atomic<bool> threw{false};
        std::string failure_what;
        std::thread caller([&] {
            try {
                auto result = transport.Call(protocol::MSG_PROVIDER_HEARTBEAT_REQUEST, fat);
                (void)result;
            } catch (const std::runtime_error& e) {
                threw.store(true);
                failure_what = e.what();
            }
        });

        // Spin until the caller has inserted its latch, then erase it while the
        // caller is still building/sending the frame: the lookup must fail.
        bool erased = false;
        const auto deadline = std::chrono::steady_clock::now() + std::chrono::seconds(5);
        while (std::chrono::steady_clock::now() < deadline) {
            std::lock_guard<std::mutex> lock(transport.pending_mutex_);
            if (transport.pending_responses_.count(predicted_req_id) != 0) {
                transport.pending_responses_.erase(predicted_req_id);
                erased = true;
                break;
            }
        }
        ASSERT_TRUE(erased);
        caller.join();
        transport.Close();
        agent.DropConnection();

        if (threw.load() && failure_what == "Response latch not found") {
            // Erase landed before the lookup this round: target outcome.
            SUCCEED();
            return;
        }
        // Otherwise ("Timeout waiting for response") the erase landed after
        // the lookup — retry the race.
    }
    FAIL() << "erase never landed before the latch lookup in 10 attempts";
}

TEST(Boost6TCPTransportTest, CallAbortsWhenClosingFlagObserved) {
    Boost6FakeAgent agent;
    std::thread agent_thread([&] { agent.AcceptOnly(); });

    TCPTransport transport("127.0.0.1", agent.port(), 10000);
    transport.Connect();
    agent_thread.join();

    std::atomic<bool> threw{false};
    std::string failure_what;
    std::thread caller([&] {
        try {
            auto result = transport.Call(protocol::MSG_PROVIDER_HEARTBEAT_REQUEST, {1});
            (void)result;
        } catch (const std::runtime_error& e) {
            threw.store(true);
            failure_what = e.what();
        }
    });

    // Let the caller park inside the wait loop, then flip the closing flag
    // (exactly what Close() does before signaling pending responses). The
    // parked Call must observe it, drop its latch and throw.
    std::this_thread::sleep_for(std::chrono::milliseconds(200));
    transport.closing_.store(true);
    caller.join();
    EXPECT_TRUE(threw.load());
    EXPECT_EQ(failure_what, "Connection closing");

    transport.Close();
    agent.DropConnection();
}

// ---------------------------------------------------------------------------
// 4) TCPServer::ServerLoop continue-branch: accept() failing with EINVAL
//    (after shutdown) while running_ flips false mid-iteration. The branch is
//    a two-instruction window inside a short spin loop; repeating the setup
//    makes hitting it near-certain while every outcome stays clean.
// ---------------------------------------------------------------------------

TEST(Boost6TCPServerTest, ServerLoopHandlesAcceptFailureDuringStop) {
    for (int round = 0; round < 120; ++round) {
        TCPServer server("127.0.0.1:0");
        server.SetHandler([](uint32_t, uint32_t, const std::vector<uint8_t>&) {
            return std::vector<uint8_t>{};
        });
        server.Start();

        // Shut the listening socket down so the accept thread spins on
        // EINVAL instead of blocking, then Stop() flips running_ at an
        // arbitrary point of the spin.
        ::shutdown(server.server_socket_, SHUT_RDWR);
        server.Stop();
    }
    SUCCEED();
}

TEST(Boost6TCPServerTest, ServerLoopClosesAcceptedSocketWhenStopped) {
    // With the accept thread blocked inside accept(), flipping running_
    // off first means the next accepted connection is closed by the loop's
    // shutdown path instead of being served.
    TCPServer server("127.0.0.1:0");
    server.SetHandler([](uint32_t, uint32_t, const std::vector<uint8_t>&) {
        return std::vector<uint8_t>{};
    });
    server.Start();
    // Give the accept thread time to park inside accept() before the flag
    // flips, so the wake-up is caused by the incoming connection.
    std::this_thread::sleep_for(std::chrono::milliseconds(50));

    const std::string listen = server.GetListenAddress();
    const size_t colon = listen.rfind(':');
    ASSERT_NE(colon, std::string::npos);
    const int port = std::stoi(listen.substr(colon + 1));

    server.running_.store(false);  // accept thread is blocked and cannot see it

    raw_socket_t client = ::socket(AF_INET, SOCK_STREAM, 0);
    ASSERT_NE(client, RAW_INVALID_SOCK);
    sockaddr_in addr{};
    addr.sin_family = AF_INET;
    addr.sin_port = htons(static_cast<uint16_t>(port));
    addr.sin_addr.s_addr = htonl(INADDR_LOOPBACK);
    ASSERT_EQ(::connect(client, reinterpret_cast<sockaddr*>(&addr), sizeof(addr)), 0);

    // The server must close the accepted socket from its stopped path; the
    // client observes EOF promptly.
    struct timeval tv {};
    tv.tv_sec = 0;
    tv.tv_usec = 700 * 1000;
    ::setsockopt(client, SOL_SOCKET, SO_RCVTIMEO, &tv, sizeof(tv));
    uint8_t scratch[8] = {0};
    const ssize_t n = ::recv(client, scratch, sizeof(scratch), 0);
    EXPECT_TRUE(n <= 0) << "unexpected bytes: " << n;

    raw_closesocket(client);
    server.Stop();
}

// ---------------------------------------------------------------------------
// 5) Config loader edges.
// ---------------------------------------------------------------------------

TEST(Boost6ConfigLoaderTest, ValidateFilePathBranches) {
    config::ClientConfigLoader loader;
    EXPECT_FALSE(loader.ValidateFilePath(""));
    // Non-existing parent directory with must_exist=false.
    EXPECT_FALSE(loader.ValidateFilePath("boost6-no-such-dir/config.json", false));
    // Bare file name: empty parent directory is accepted.
    EXPECT_TRUE(loader.ValidateFilePath("boost6-bare.json", false));
    // Existing parent directory with must_exist=false.
    const std::string dir = ::testing::TempDir();
    EXPECT_TRUE(loader.ValidateFilePath(dir + "boost6-relax.json", false));
    EXPECT_FALSE(loader.ValidateFilePath(dir + "boost6-missing.json", true));
}

TEST(Boost6ConfigLoaderTest, LoadWithEnvironmentOverridesMissingFileThrows) {
    config::ClientConfigLoader loader;
    try {
        (void)loader.LoadWithEnvironmentOverrides("/boost6/does/not/exist.json", "CROUPIER");
        FAIL() << "LoadWithEnvironmentOverrides must throw for a missing file";
    } catch (const std::runtime_error& e) {
        EXPECT_NE(std::string(e.what()).find("does not exist"), std::string::npos);
    }
}

// ---------------------------------------------------------------------------
// 6) Logger masking: a key containing regex metacharacters makes the pattern
//    construction throw, and the exception must propagate out of the helper.
// ---------------------------------------------------------------------------

TEST(Boost6LoggerTest, MaskJsonSensitiveInvalidPatternThrows) {
    const std::string masked = MaskJsonSensitive(R"({"token":"abcdef123456"})", {"token"});
    EXPECT_NE(masked.find("***masked***"), std::string::npos);
    EXPECT_THROW(MaskJsonSensitive(R"({"a":"b"})", {"("}), std::regex_error);
}

// ---------------------------------------------------------------------------
// 7) HTTP transport: libcurl rejects a negative timeout at setopt time; the
//    defaulted virtual destructor must run through a base-pointer delete.
// ---------------------------------------------------------------------------

TEST(Boost6HTTPTransportTest, DefaultTransportRejectsNegativeTimeout) {
    auto transport = NewDefaultHTTPTransport();
    ASSERT_NE(transport, nullptr);
    HTTPRequest request;
    request.method = "GET";
    request.url = "http://127.0.0.1:9/";
    request.timeout_ms = -1;
    try {
        (void)transport->Send(request);
        FAIL() << "Send must reject a negative timeout";
    } catch (const std::runtime_error& e) {
        EXPECT_NE(std::string(e.what()).find("configure Server HTTP request"), std::string::npos);
    }
}

namespace {
class Boost6CountingTransport final : public HTTPTransport {
public:
    static std::atomic<int> destroyed_count;
    HTTPResponse Send(const HTTPRequest&) override {
        HTTPResponse response;
        response.status_code = 204;
        return response;
    }
    ~Boost6CountingTransport() override { destroyed_count.fetch_add(1); }
};
std::atomic<int> Boost6CountingTransport::destroyed_count{0};
}  // namespace

TEST(Boost6HTTPTransportTest, BasePointerDeleteRunsVirtualDestructor) {
    const int before = Boost6CountingTransport::destroyed_count.load();
    HTTPTransport* transport = new Boost6CountingTransport();
    delete transport;
    EXPECT_EQ(Boost6CountingTransport::destroyed_count.load(), before + 1);
}

// ---------------------------------------------------------------------------
// 8) Dynamic library manager defensive branches.
// ---------------------------------------------------------------------------

TEST(Boost6DynamicLoaderTest, GetFunctionRejectsNullLibraryEntry) {
    plugin::DynamicLibraryManager manager;
    manager.libraries_["boost6-ghost"] = nullptr;
    EXPECT_EQ(manager.GetFunction("boost6-ghost", "any"), nullptr);
    EXPECT_NE(manager.GetLastError().find("Invalid library handle"), std::string::npos);
}

TEST(Boost6PluginManagerTest, CleanupUnknownPluginIsNoop) {
    plugin::PluginManager manager;
    manager.CleanupPlugin("boost6-no-such-plugin");
    EXPECT_TRUE(manager.plugins_.empty());
}

// ---------------------------------------------------------------------------
// 9) Allocation-failure escapes. Several helpers only ever terminate via an
//    exception path under out-of-memory conditions; their exceptional-exit
//    counters can only be exercised by making an allocation fail. Each case
//    runs in a forked child whose address space is capped just above the
//    current footprint, so the next sizeable allocation throws bad_alloc and
//    unwinds out of the helper. The parent asserts the child observed the
//    expected exception; a stuck child is killed by a watchdog (no coverage
//    contribution, no hang).
// ---------------------------------------------------------------------------

#ifndef _WIN32

// Runs `scenario` inside a forked child under a tight RLIMIT_AS. `scenario`
// returns true when it observed the expected exception. The child exits 0 on
// success and 11 otherwise; the parent asserts the exit status.
void RunChildExpectingAllocationFailure(const std::function<bool()>& scenario) {
    std::fflush(nullptr);
    const pid_t pid = ::fork();
    ASSERT_NE(pid, -1);
    if (pid == 0) {
#ifdef __GLIBC__
        mallopt(M_MMAP_THRESHOLD, 4096);
#endif
        // Exhaust inherited free arena chunks with held (untouched) padding
        // so any new sizeable request must grow the address space and hit
        // the cap. Mixed sizes sweep the large/mid/small bins; pages are
        // never touched, so only virtual space is consumed.
        std::vector<void*> padding;
        padding.reserve(32768);
        const size_t padding_sizes[] = {4u << 20, 256u << 10, 64u << 10, 4u << 10};
        const int padding_counts[] = {2048, 8192, 8192, 8192};
        for (size_t si = 0; si < 4; ++si) {
            for (int i = 0; i < padding_counts[si]; ++i) {
                void* block = std::malloc(padding_sizes[si]);
                if (block == nullptr) break;
                padding.push_back(block);
            }
        }
        unsigned long long vm_kb = 0;
        FILE* status_file = std::fopen("/proc/self/status", "r");
        if (status_file != nullptr) {
            char line[256];
            while (std::fgets(line, sizeof(line), status_file) != nullptr) {
                if (std::sscanf(line, "VmSize: %llu kB", &vm_kb) == 1) break;
            }
            std::fclose(status_file);
        }
        if (vm_kb == 0) ::_exit(12);
        struct rlimit tight {};
        tight.rlim_cur = static_cast<rlim_t>(vm_kb) * 1024ULL + 96ULL * 1024ULL;
        tight.rlim_max = RLIM_INFINITY;
        if (::setrlimit(RLIMIT_AS, &tight) != 0) ::_exit(10);
        // With the cap in place, drain every remaining allocation source
        // (bin remnants, top-chunk splits, mmap slack) for sizeable blocks
        // until malloc refuses, then hand back one small block so the
        // expected exception object itself can still be constructed.
        void* small_slack = nullptr;
        const size_t drain_sizes[] = {64u << 10, 16u << 10, 4u << 10};
        for (size_t size : drain_sizes) {
            for (int i = 0; i < 100000; ++i) {
                void* block = std::malloc(size);
                if (block == nullptr) break;
                padding.push_back(block);
                if (small_slack == nullptr && size == (4u << 10)) small_slack = block;
            }
        }
        if (small_slack != nullptr) {
            std::free(small_slack);
            padding.erase(std::find(padding.begin(), padding.end(), small_slack));
        }
        const bool ok = scenario();
        for (void* block : padding) std::free(block);
        std::exit(ok ? 0 : 11);
    }
    const auto deadline = std::chrono::steady_clock::now() + std::chrono::seconds(15);
    int status = 0;
    for (;;) {
        const pid_t r = ::waitpid(pid, &status, WNOHANG);
        if (r == pid) break;
        if (r < 0) break;
        if (std::chrono::steady_clock::now() > deadline) {
            ::kill(pid, SIGKILL);
            ::waitpid(pid, &status, 0);
            break;
        }
        std::this_thread::sleep_for(std::chrono::milliseconds(5));
    }
    ASSERT_TRUE(WIFEXITED(status));
    if (WEXITSTATUS(status) != 0) {
        std::fprintf(stderr, "[boost6] allocation-failure child exited %d\n", WEXITSTATUS(status));
    }
    EXPECT_EQ(WEXITSTATUS(status), 0);
}

TEST(Boost6AllocationFailureTest, GzipInitFailsUnderMemoryPressure) {
    CroupierClient client(Boost6ProviderConfig("127.0.0.1:19091"));
    auto* impl = client.impl_.get();
    RunChildExpectingAllocationFailure([impl] {
        try {
            const std::vector<uint8_t> out = impl->GzipCompress("payload");
            (void)out;
            std::fprintf(stderr, "[boost6] gzip: no exception, out=%zu\n", out.size());
            return false;
        } catch (const std::runtime_error& e) {
            if (std::string(e.what()) != "gzip init failed") {
                std::fprintf(stderr, "[boost6] gzip unexpected exception: %s\n", e.what());
            }
            return std::string(e.what()) == "gzip init failed";
        }
    });
}

TEST(Boost6AllocationFailureTest, BuildManifestJsonEscapesOnAllocationFailure) {
    CroupierClient client(Boost6ProviderConfig("127.0.0.1:19091"));
    auto* impl = client.impl_.get();
    impl->config_.service_id = std::string(64 * 1024 * 1024, 's');
    RunChildExpectingAllocationFailure([impl] {
        try {
            const std::string manifest = impl->buildManifestJson();
            (void)manifest;
            return false;
        } catch (const std::bad_alloc&) {
            return true;
        }
    });
}

TEST(Boost6AllocationFailureTest, EscapeURLSegmentEscapesOnAllocationFailure) {
    const std::string huge(64 * 1024 * 1024, 'a');
    RunChildExpectingAllocationFailure([&huge] {
        try {
            const std::string escaped = EscapeURLSegment(huge);
            (void)escaped;
            return false;
        } catch (const std::bad_alloc&) {
            return true;
        }
    });
}

TEST(Boost6AllocationFailureTest, ToTaskEventEscapesOnAllocationFailure) {
    v1::TaskEvent event;
    event.set_type("progress");
    event.set_message(std::string(64 * 1024 * 1024, 'm'));
    RunChildExpectingAllocationFailure([&event] {
        try {
            const TaskEvent converted = ToTaskEvent("task-big", event);
            (void)converted;
            return false;
        } catch (const std::bad_alloc&) {
            return true;
        }
    });
}

TEST(Boost6AllocationFailureTest, MergeJsonEscapesOnAllocationFailure) {
    nlohmann::json base = nlohmann::json::object();
    base["huge"] = std::string(64 * 1024 * 1024, 'j');
    nlohmann::json overlay = nlohmann::json::object();
    overlay["other"] = 1;
    RunChildExpectingAllocationFailure([&base, &overlay] {
        try {
            const nlohmann::json merged = utils::JsonUtils::MergeJson(base, overlay);
            (void)merged;
            return false;
        } catch (const std::bad_alloc&) {
            return true;
        }
    });
}

TEST(Boost6AllocationFailureTest, ValidateSecurityConfigEscapesOnAllocationFailure) {
    config::ClientConfigLoader loader;
    ClientConfig config;
    config.insecure = false;
    config.cert_file = std::string(64 * 1024 * 1024, 'c');
    RunChildExpectingAllocationFailure([&loader, &config] {
        try {
            const std::vector<std::string> errors = loader.ValidateSecurityConfig(config);
            (void)errors;
            return false;
        } catch (const std::bad_alloc&) {
            return true;
        }
    });
}

TEST(Boost6AllocationFailureTest, ValidateAuthConfigEscapesOnAllocationFailure) {
    config::ClientConfigLoader loader;
    ClientConfig config;
    config.headers[std::string(64 * 1024 * 1024, 'k')] = "";
    RunChildExpectingAllocationFailure([&loader, &config] {
        try {
            const std::vector<std::string> errors = loader.ValidateAuthConfig(config);
            (void)errors;
            return false;
        } catch (const std::bad_alloc&) {
            return true;
        }
    });
}

TEST(Boost6AllocationFailureTest, ToTitleCaseEscapesOnAllocationFailure) {
    const std::string huge(64 * 1024 * 1024, 'a');
    RunChildExpectingAllocationFailure([&huge] {
        try {
            const std::string title = openapi::ToTitleCase(huge);
            (void)title;
            return false;
        } catch (const std::bad_alloc&) {
            return true;
        }
    });
}

#endif  // !_WIN32

}  // namespace
}  // namespace croupier::sdk::test
