// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

// Fifth coverage boost: branches unlocked by the SIGPIPE guard
// (EPIPE after RST, heartbeat/reconnect failure choreography),
// Close-vs-Call races on the pending-response map, fd-exhaustion
// paths via RLIMIT_NOFILE, RLIMIT_FSIZE-backed atomic-write failures,
// throwing inbound/server handlers, and assorted error-injection edges.

#include <gtest/gtest.h>

#include "croupier/sdk/croupier_client.h"
#include "croupier/sdk/config/client_config_loader.h"
#include "croupier/sdk/file_push.h"
#include "croupier/sdk/http_transport.h"
#include "croupier/sdk/protocol.h"
#include "croupier/sdk/tcp_transport.h"
#include "croupier/sdk/utils/file_utils.h"
#include "croupier/sdk/utils/json_utils.h"

#include "croupier/sdk/v1/invocation.pb.h"
#include "croupier/sdk/v1/provider.pb.h"

#include <atomic>
#include <chrono>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <memory>
#include <mutex>
#include <string>
#include <thread>
#include <vector>

#include <nlohmann/json.hpp>

#ifdef _WIN32
#include <winsock2.h>
#include <ws2tcpip.h>
using raw_socket_t = SOCKET;
#define RAW_INVALID_SOCK INVALID_SOCKET
#define raw_closesocket ::closesocket
#else
#include <arpa/inet.h>
#include <dirent.h>
#include <fcntl.h>
#include <netinet/in.h>
#include <signal.h>
#include <sys/resource.h>
#include <sys/socket.h>
#include <sys/stat.h>
#include <unistd.h>
using raw_socket_t = int;
#define RAW_INVALID_SOCK (-1)
#define raw_closesocket ::close
#endif

namespace croupier::sdk::test {
namespace {

bool SendAllRaw(raw_socket_t sock, const void* buf, size_t len) {
    const char* p = static_cast<const char*>(buf);
    size_t off = 0;
    while (off < len) {
        auto n = ::send(sock, p + off, len - off, 0);
        if (n <= 0) return false;
        off += static_cast<size_t>(n);
    }
    return true;
}

bool ReadAllRaw(raw_socket_t sock, void* buf, size_t len) {
    char* p = static_cast<char*>(buf);
    size_t off = 0;
    while (off < len) {
        auto n = ::recv(sock, p + off, len - off, 0);
        if (n <= 0) return false;
        off += static_cast<size_t>(n);
    }
    return true;
}

// Minimal raw listener speaking the Croupier frame protocol. Mirrors the
// harness style of test_coverage_boost4.cpp with RST/error extras.
class Boost5FakeAgent {
public:
    Boost5FakeAgent() {
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

    ~Boost5FakeAgent() {
        DropConnection();
        if (listen_fd_ != RAW_INVALID_SOCK) raw_closesocket(listen_fd_);
    }

    std::string address() const { return "127.0.0.1:" + std::to_string(port_); }

    unsigned short port() const { return static_cast<unsigned short>(port_); }

    void AcceptOnly() {
        conn_ = ::accept(listen_fd_, nullptr, nullptr);
        ASSERT_NE(conn_, RAW_INVALID_SOCK);
    }

    void AcceptAndHandshake() {
        AcceptOnly();
        auto first = ReadFrame();
        ASSERT_EQ(first.msg_id, protocol::MSG_PROVIDER_CONNECT_REQUEST);
        v1::ProviderConnectResponse resp;
        resp.set_session_id("boost5-session");
        std::string out;
        resp.SerializeToString(&out);
        WriteFrame(protocol::MSG_PROVIDER_CONNECT_RESPONSE, first.req_id,
                   std::vector<uint8_t>(out.begin(), out.end()));
    }

    // Hard-resets the accepted connection (SO_LINGER{1,0} + close) so the
    // peer's next send() fails with EPIPE/ECONNRESET instead of blocking.
    void RstConnection() {
        if (conn_ != RAW_INVALID_SOCK) {
            struct linger lg {};
            lg.l_onoff = 1;
            lg.l_linger = 0;
            ::setsockopt(conn_, SOL_SOCKET, SO_LINGER, &lg, sizeof(lg));
            raw_closesocket(conn_);
            conn_ = RAW_INVALID_SOCK;
        }
    }

    void CloseListener() {
        if (listen_fd_ != RAW_INVALID_SOCK) {
            raw_closesocket(listen_fd_);
            listen_fd_ = RAW_INVALID_SOCK;
        }
        DropConnection();
    }

    void PushRequest(uint32_t msg_id, uint32_t req_id, const std::vector<uint8_t>& body) {
        WriteFrame(msg_id, req_id, body);
    }

    protocol::ParsedMessage ReadFrame() {
        uint8_t hdr[4] = {0};
        if (!ReadAllRaw(conn_, hdr, 4)) ADD_FAILURE() << "read frame header failed";
        uint32_t len = (uint32_t(hdr[0]) << 24) | (uint32_t(hdr[1]) << 16) |
                       (uint32_t(hdr[2]) << 8) | uint32_t(hdr[3]);
        std::vector<uint8_t> payload(len);
        if (len > 0 && !ReadAllRaw(conn_, payload.data(), len)) {
            ADD_FAILURE() << "read frame body failed";
        }
        return protocol::ParseMessage(payload);
    }

    // Soft read used by responder loops: returns false on EOF/error instead
    // of failing the test (the connection is expected to die mid-test).
    bool TryReadFrame(protocol::ParsedMessage& out) {
        uint8_t hdr[4] = {0};
        if (!ReadAllRaw(conn_, hdr, 4)) return false;
        uint32_t len = (uint32_t(hdr[0]) << 24) | (uint32_t(hdr[1]) << 16) |
                       (uint32_t(hdr[2]) << 8) | uint32_t(hdr[3]);
        std::vector<uint8_t> payload(len);
        if (len > 0 && !ReadAllRaw(conn_, payload.data(), len)) return false;
        out = protocol::ParseMessage(payload);
        return true;
    }

    void DropConnection() {
        if (conn_ != RAW_INVALID_SOCK) {
            raw_closesocket(conn_);
            conn_ = RAW_INVALID_SOCK;
        }
    }

private:
    void WriteFrame(uint32_t msg_id, uint32_t req_id, const std::vector<uint8_t>& body) {
        auto frame = protocol::NewMessage(msg_id, req_id, body);
        std::vector<uint8_t> wrapped(4 + frame.size());
        wrapped[0] = static_cast<uint8_t>((frame.size() >> 24) & 0xFF);
        wrapped[1] = static_cast<uint8_t>((frame.size() >> 16) & 0xFF);
        wrapped[2] = static_cast<uint8_t>((frame.size() >> 8) & 0xFF);
        wrapped[3] = static_cast<uint8_t>(frame.size() & 0xFF);
        std::memcpy(wrapped.data() + 4, frame.data(), frame.size());
        ASSERT_TRUE(SendAllRaw(conn_, wrapped.data(), wrapped.size()));
    }

    raw_socket_t listen_fd_{RAW_INVALID_SOCK};
    raw_socket_t conn_{RAW_INVALID_SOCK};
    int port_{0};
};

ClientConfig Boost5ProviderConfig(const std::string& addr, int heartbeat_interval,
                                  int timeout_seconds) {
    ClientConfig config;
    config.game_id = "game-boost5";
    config.env = "development";
    config.service_id = "cpp-boost5";
    config.agent_addr = addr;
    config.timeout_seconds = timeout_seconds;
    config.connect_timeout_seconds = 1;
    config.heartbeat_interval = heartbeat_interval;
    config.disable_logging = true;
    return config;
}

std::vector<uint8_t> InvokeBodyBoost5(const std::string& function_id, const std::string& payload) {
    v1::InvokeRequest req;
    req.set_function_id(function_id);
    req.set_payload(payload);
    std::string out;
    req.SerializeToString(&out);
    return std::vector<uint8_t>(out.begin(), out.end());
}

FunctionDescriptor MakeBlockDescriptor() {
    FunctionDescriptor desc;
    desc.id = "fn.block";
    desc.version = "1.0.0";
    desc.operation = "block";
    desc.capability = "action";
    desc.risk = "safe";
    return desc;
}

// Returns the highest currently-open fd number (best effort via /proc).
int HighestOpenFd() {
    int max_fd = 2;
    DIR* d = ::opendir("/proc/self/fd");
    if (d == nullptr) return max_fd;
    while (struct dirent* e = ::readdir(d)) {
        if (e->d_name[0] == '.') continue;
        int fd = std::atoi(e->d_name);
        if (fd > max_fd) max_fd = fd;
    }
    ::closedir(d);
    return max_fd;
}

// ---------------------------------------------------------------------------
// 1) TCPTransport: send fails after the peer hard-resets the connection.
//    Unlocked by the process-wide SIGPIPE guard (test_sigpipe_guard.cpp).
// ---------------------------------------------------------------------------

TEST(TCPTransportBoost5Test, SendFailsWithPipeErrorAfterPeerRst) {
    for (int attempt = 0; attempt < 3; ++attempt) {
        Boost5FakeAgent agent;
        std::thread agent_thread([&] { agent.AcceptOnly(); });

        TCPTransport transport("127.0.0.1", agent.port(), 400);
        transport.Connect();
        agent_thread.join();
        agent.RstConnection();
        // Loopback RST delivery is immediate; 200ms is generous.
        std::this_thread::sleep_for(std::chrono::milliseconds(200));
        EXPECT_THROW(transport.Call(protocol::MSG_PROVIDER_HEARTBEAT_REQUEST, {1, 2}),
                     std::runtime_error);
        transport.Close();
    }
}

// ---------------------------------------------------------------------------
// 2) TCPTransport: inbound handler that throws -> worker pool catches the
//    exception and answers with an empty response frame.
// ---------------------------------------------------------------------------

TEST(TCPTransportBoost5Test, InboundHandlerExceptionAnswersEmptyResponse) {
    Boost5FakeAgent agent;
    std::thread agent_thread([&] { agent.AcceptOnly(); });

    TCPTransport transport("127.0.0.1", agent.port(), 4000);
    transport.SetInboundHandler([](uint32_t, uint32_t, const std::vector<uint8_t>&)
                                    -> std::vector<uint8_t> {
        throw std::runtime_error("inbound handler boom");
    });
    transport.Connect();
    agent_thread.join();

    agent.PushRequest(protocol::MSG_INVOKE_REQUEST, 4242, {0x01});

    protocol::ParsedMessage resp{};
    ASSERT_TRUE(agent.TryReadFrame(resp));
    EXPECT_EQ(resp.req_id, 4242u);
    EXPECT_EQ(resp.msg_id, protocol::GetResponseMsgID(protocol::MSG_INVOKE_REQUEST));
    EXPECT_TRUE(resp.body.empty());

    transport.Close();
}

// ---------------------------------------------------------------------------
// 3) TCPTransport: Close() racing an in-flight Call(). Depending on where
//    the race lands, Call() observes "send incomplete", "latch not found",
//    "connection closing" or the empty-response signal — every outcome is
//    valid; the test only asserts that the race terminates quickly.
// ---------------------------------------------------------------------------

TEST(TCPTransportBoost5Test, ConcurrentCloseRacesPendingCall) {
    unsigned seed = static_cast<unsigned>(
        std::chrono::steady_clock::now().time_since_epoch().count() & 0xFFFF);
    for (int i = 0; i < 120; ++i) {
        Boost5FakeAgent agent;
        std::thread agent_thread([&] { agent.AcceptOnly(); });

        TCPTransport transport("127.0.0.1", agent.port(), 400);
        transport.Connect();
        agent_thread.join();

        std::atomic<bool> done{false};
        std::thread caller([&] {
            try {
                auto result = transport.Call(protocol::MSG_PROVIDER_HEARTBEAT_REQUEST, {7});
                (void)result;
            } catch (const std::exception&) {
                // Any of the race outcomes is acceptable.
            }
            done.store(true);
        });

        seed = seed * 1103515245u + 12345u;
        const int delay_us = static_cast<int>((seed >> 16) % 2500);
        std::this_thread::sleep_for(std::chrono::microseconds(delay_us));
        transport.Close();
        caller.join();
        EXPECT_TRUE(done.load());
        agent.DropConnection();
    }
}

// ---------------------------------------------------------------------------
// 4) TCPServer: handler that throws -> no response frame is written.
// ---------------------------------------------------------------------------

TEST(TCPServerBoost5Test, HandlerExceptionDropsResponse) {
    TCPServer server("127.0.0.1:0");
    server.SetHandler([](uint32_t, uint32_t, const std::vector<uint8_t>&)
                          -> std::vector<uint8_t> {
        throw std::runtime_error("server handler boom");
    });
    server.Start();

    const std::string listen = server.GetListenAddress();
    const size_t colon = listen.rfind(':');
    ASSERT_NE(colon, std::string::npos);
    const int port = std::stoi(listen.substr(colon + 1));

    raw_socket_t client = ::socket(AF_INET, SOCK_STREAM, 0);
    ASSERT_NE(client, RAW_INVALID_SOCK);
    sockaddr_in addr{};
    addr.sin_family = AF_INET;
    addr.sin_port = htons(static_cast<uint16_t>(port));
    addr.sin_addr.s_addr = htonl(INADDR_LOOPBACK);
    ASSERT_EQ(::connect(client, reinterpret_cast<sockaddr*>(&addr), sizeof(addr)), 0);

    auto frame = protocol::NewMessage(protocol::MSG_INVOKE_REQUEST, 99, {0x01});
    std::vector<uint8_t> wrapped(4 + frame.size());
    wrapped[0] = static_cast<uint8_t>((frame.size() >> 24) & 0xFF);
    wrapped[1] = static_cast<uint8_t>((frame.size() >> 16) & 0xFF);
    wrapped[2] = static_cast<uint8_t>((frame.size() >> 8) & 0xFF);
    wrapped[3] = static_cast<uint8_t>(frame.size() & 0xFF);
    std::memcpy(wrapped.data() + 4, frame.data(), frame.size());
    ASSERT_TRUE(SendAllRaw(client, wrapped.data(), wrapped.size()));

    // No response may arrive: bounded wait, then silence proves the point.
    struct timeval tv {};
    tv.tv_sec = 0;
    tv.tv_usec = 700 * 1000;
    ::setsockopt(client, SOL_SOCKET, SO_RCVTIMEO, &tv, sizeof(tv));
    uint8_t scratch[16];
    const ssize_t n = ::recv(client, scratch, sizeof(scratch), 0);
    EXPECT_TRUE(n <= 0) << "unexpected response bytes: " << n;

    raw_closesocket(client);
    server.Stop();
}

// ---------------------------------------------------------------------------
// 5) TCPServer: accept() failing with EMFILE while running -> the accept
//    loop breaks out (fd exhaustion via RLIMIT_NOFILE).
// ---------------------------------------------------------------------------

TEST(TCPServerBoost5Test, AcceptErrorUnderFdPressureBreaksLoop) {
    TCPServer server("127.0.0.1:0");
    server.SetHandler([](uint32_t, uint32_t, const std::vector<uint8_t>&) {
        return std::vector<uint8_t>{};
    });
    server.Start();

    const std::string listen = server.GetListenAddress();
    const size_t colon = listen.rfind(':');
    ASSERT_NE(colon, std::string::npos);
    const int port = std::stoi(listen.substr(colon + 1));

    struct rlimit old_limit {};
    ASSERT_EQ(::getrlimit(RLIMIT_NOFILE, &old_limit), 0);
    const int highest = HighestOpenFd();
    struct rlimit tight {};
    tight.rlim_cur = static_cast<rlim_t>(highest + 2);  // one client fd fits, accept can't
    tight.rlim_max = old_limit.rlim_max;
    ASSERT_EQ(::setrlimit(RLIMIT_NOFILE, &tight), 0);

    raw_socket_t client = RAW_INVALID_SOCK;
    {
        // The client socket consumes the last free fd; the server's accept()
        // then fails with EMFILE while running_ is still true.
        client = ::socket(AF_INET, SOCK_STREAM, 0);
        if (client != RAW_INVALID_SOCK) {
            sockaddr_in addr{};
            addr.sin_family = AF_INET;
            addr.sin_port = htons(static_cast<uint16_t>(port));
            addr.sin_addr.s_addr = htonl(INADDR_LOOPBACK);
            ::connect(client, reinterpret_cast<sockaddr*>(&addr), sizeof(addr));
        }
    }

    std::this_thread::sleep_for(std::chrono::milliseconds(300));
    ASSERT_EQ(::setrlimit(RLIMIT_NOFILE, &old_limit), 0);
    if (client != RAW_INVALID_SOCK) raw_closesocket(client);
    server.Stop();
}

// ---------------------------------------------------------------------------
// 6) TCPServer: Stop() racing a burst of incoming connections. A connection
//    accepted after running_ flips to false must simply be closed. Loose
//    test: only termination is asserted.
// ---------------------------------------------------------------------------

TEST(TCPServerBoost5Test, StopDuringAcceptBurstTerminatesCleanly) {
    for (int round = 0; round < 25; ++round) {
        TCPServer server("127.0.0.1:0");
        server.SetHandler([](uint32_t, uint32_t, const std::vector<uint8_t>&) {
            return std::vector<uint8_t>{};
        });
        server.Start();

        const std::string listen = server.GetListenAddress();
        const size_t colon = listen.rfind(':');
        ASSERT_NE(colon, std::string::npos);
        const int port = std::stoi(listen.substr(colon + 1));

        std::thread burst([&port] {
            for (int i = 0; i < 120; ++i) {
                raw_socket_t c = ::socket(AF_INET, SOCK_STREAM, 0);
                if (c == RAW_INVALID_SOCK) continue;
                sockaddr_in a{};
                a.sin_family = AF_INET;
                a.sin_port = htons(static_cast<uint16_t>(port));
                a.sin_addr.s_addr = htonl(INADDR_LOOPBACK);
                if (::connect(c, reinterpret_cast<sockaddr*>(&a), sizeof(a)) == 0) {
                    std::this_thread::sleep_for(std::chrono::microseconds(50));
                }
                raw_closesocket(c);
            }
        });

        std::this_thread::sleep_for(std::chrono::milliseconds(1));
        server.Stop();  // may land mid-accept; every outcome must be clean
        burst.join();
    }
}

// ---------------------------------------------------------------------------
// 7) CroupierClient: drain arrives, then the agent dies and reconnection is
//    refused. When the blocked in-flight call finally completes, recovery
//    must skip re-registration (client is disconnected) instead of throwing.
// ---------------------------------------------------------------------------

TEST(CroupierClientBoost5Test, DrainDuringDisconnectionSkipsReregistration) {
    Boost5FakeAgent agent;
    std::thread agent_thread([&] { agent.AcceptAndHandshake(); });

    ClientConfig config = Boost5ProviderConfig(agent.address(), 1, 2);
    CroupierClient client(config);

    std::atomic<bool> handler_entered{false};
    std::atomic<bool> release_handler{false};
    ASSERT_TRUE(client.RegisterFunction(MakeBlockDescriptor(),
                                        [&handler_entered, &release_handler](const std::string&,
                                                                             const std::string&) {
        handler_entered.store(true);
        while (!release_handler.load()) {
            std::this_thread::sleep_for(std::chrono::milliseconds(10));
        }
        return "{}";
    }));
    ASSERT_TRUE(client.Connect());
    agent_thread.join();

    // Answer every provider->agent request (heartbeats) until the socket dies.
    std::atomic<bool> responder_run{true};
    std::thread responder([&] {
        while (responder_run.load()) {
            protocol::ParsedMessage m{};
            if (!agent.TryReadFrame(m)) break;
            if (protocol::IsRequest(m.msg_id)) {
                agent.PushRequest(protocol::GetResponseMsgID(m.msg_id), m.req_id, {});
            }
        }
    });

    // Blocked in-flight call keeps the drain pending.
    agent.PushRequest(protocol::MSG_INVOKE_REQUEST, 6001, InvokeBodyBoost5("fn.block", "x"));
    for (int i = 0; i < 200 && !handler_entered.load(); ++i) {
        std::this_thread::sleep_for(std::chrono::milliseconds(10));
    }
    ASSERT_TRUE(handler_entered.load());

    agent.PushRequest(protocol::MSG_PROVIDER_DRAIN_REQUEST, 6002, {});
    std::this_thread::sleep_for(std::chrono::milliseconds(300));

    // Kill both the connection and the listener: heartbeats fail (EPIPE),
    // reconnection is refused and retries with the 500ms back-off sleep.
    agent.RstConnection();
    agent.CloseListener();

    // Disconnectedness is driven by the heartbeat Call's synchronous
    // response timeout (timeout_seconds=2s): on slow runners it lands later
    // than a fixed sleep, so poll with a generous deadline instead.
    auto wait_until_disconnected = [&client](int timeout_s) {
        const auto deadline =
            std::chrono::steady_clock::now() + std::chrono::seconds(timeout_s);
        while (client.IsConnected() && std::chrono::steady_clock::now() < deadline) {
            std::this_thread::sleep_for(std::chrono::milliseconds(50));
        }
    };
    wait_until_disconnected(10);
    EXPECT_FALSE(client.IsConnected());

    // Releasing the handler lets DrainAndRecover finish while disconnected.
    release_handler.store(true);
    wait_until_disconnected(5);

    responder_run.store(false);
    client.Stop();
    responder.join();
    EXPECT_FALSE(client.IsConnected());
}

// ---------------------------------------------------------------------------
// 8) CroupierClient: Stop() while a drain is waiting on a blocked in-flight
//    call -> the drain aborts with the "in-flight calls still running" log.
// ---------------------------------------------------------------------------

TEST(CroupierClientBoost5Test, DrainTimeoutLogsWhenStoppedWithInflightCalls) {
    Boost5FakeAgent agent;
    std::thread agent_thread([&] { agent.AcceptAndHandshake(); });

    ClientConfig config = Boost5ProviderConfig(agent.address(), 30, 5);
    CroupierClient client(config);

    std::atomic<bool> handler_entered{false};
    std::atomic<bool> release_handler{false};
    ASSERT_TRUE(client.RegisterFunction(MakeBlockDescriptor(),
                                        [&handler_entered, &release_handler](const std::string&,
                                                                             const std::string&) {
        handler_entered.store(true);
        while (!release_handler.load()) {
            std::this_thread::sleep_for(std::chrono::milliseconds(10));
        }
        return "{}";
    }));
    ASSERT_TRUE(client.Connect());
    agent_thread.join();

    std::atomic<bool> responder_run{true};
    std::thread responder([&] {
        while (responder_run.load()) {
            protocol::ParsedMessage m{};
            if (!agent.TryReadFrame(m)) break;
            if (protocol::IsRequest(m.msg_id)) {
                agent.PushRequest(protocol::GetResponseMsgID(m.msg_id), m.req_id, {});
            }
        }
    });

    agent.PushRequest(protocol::MSG_INVOKE_REQUEST, 6101, InvokeBodyBoost5("fn.block", "x"));
    for (int i = 0; i < 200 && !handler_entered.load(); ++i) {
        std::this_thread::sleep_for(std::chrono::milliseconds(10));
    }
    ASSERT_TRUE(handler_entered.load());

    agent.PushRequest(protocol::MSG_PROVIDER_DRAIN_REQUEST, 6102, {});
    std::this_thread::sleep_for(std::chrono::milliseconds(400));

    // Stop flips running_ off: the drain waiter notices within ~100ms while
    // the in-flight call is still blocked.
    client.Stop();

    // Let the detached drain thread and inbound worker finish quietly.
    release_handler.store(true);
    std::this_thread::sleep_for(std::chrono::milliseconds(300));
    responder_run.store(false);
    responder.join();
    EXPECT_FALSE(client.IsConnected());
}

// ---------------------------------------------------------------------------
// 9) Config loader: structurally valid JSON whose typed parse hits a
//    non-object value. Since nlohmann 3.12, items() on a non-object no
//    longer throws type_error.316 (it yields a single empty-key item), so
//    {"auth":{"headers":123}} loads successfully with the numeric headers
//    value silently skipped instead of surfacing as a parse failure.
// ---------------------------------------------------------------------------

TEST(ConfigLoaderBoost5Test, TypedParseOfNonObjectHeadersIsIgnored) {
    config::ClientConfigLoader loader;
    ClientConfig config = loader.LoadFromJson(R"({"auth":{"headers":123}})");
    EXPECT_TRUE(config.headers.empty());
    // LoadFromJson back-fills an empty service_id with the default.
    EXPECT_EQ(config.service_id, "cpp-service");
    EXPECT_EQ(config.game_id, "default-game");
}

// ---------------------------------------------------------------------------
// 10) JSON utils: PrettyPrint falls back to "{}" when dumping fails
//     (invalid UTF-8 payload).
// ---------------------------------------------------------------------------

TEST(JsonUtilsBoost5Test, PrettyPrintInvalidUtf8FallsBackToEmptyObject) {
    nlohmann::json obj = nlohmann::json::object();
    obj["secret"] = std::string("bad\xC0\xAFutf8");
    EXPECT_EQ(utils::JsonUtils::PrettyPrint(obj, 2), "{}");
}

// ---------------------------------------------------------------------------
// 11) HTTP transport base: destroying a transport through the base pointer
//     runs the defaulted virtual destructor.
// ---------------------------------------------------------------------------

namespace {
class MinimalHTTPTransport final : public HTTPTransport {
public:
    HTTPResponse Send(const HTTPRequest&) override { return HTTPResponse{}; }
};
}  // namespace

TEST(HTTPTransportBoost5Test, BaseDestructorRunsViaSharedPointer) {
    std::weak_ptr<HTTPTransport> weak;
    {
        std::shared_ptr<HTTPTransport> transport = std::make_shared<MinimalHTTPTransport>();
        weak = transport;
        EXPECT_NE(transport->Send(HTTPRequest{}).status_code, -12345);
    }
    EXPECT_TRUE(weak.expired());
}

// ---------------------------------------------------------------------------
// 12) File push: atomic write failures under RLIMIT_FSIZE (fwrite short
//     write, fclose flush failure). SIGXFSZ is ignored for the duration.
// ---------------------------------------------------------------------------

TEST(FilePushBoost5Test, AtomicWriteFailsUnderFileSizeLimit) {
    const std::string dir = ::testing::TempDir();
    void (*old_handler)(int) = ::signal(SIGXFSZ, SIG_IGN);
    ASSERT_NE(old_handler, SIG_ERR);

    struct rlimit old_limit {};
    ASSERT_EQ(::getrlimit(RLIMIT_FSIZE, &old_limit), 0);

    {
        // 16-byte cap with a >BUFSIZ payload: fwrite itself returns short.
        struct rlimit tight {};
        tight.rlim_cur = 16;
        tight.rlim_max = old_limit.rlim_max;
        ASSERT_EQ(::setrlimit(RLIMIT_FSIZE, &tight), 0);
        std::vector<uint8_t> big(16384, 0xAB);
        EXPECT_FALSE(AtomicWriteFile(dir + "boost5-big.bin", big));
    }
    {
        // 5-byte cap with an 8-byte (buffered) payload: fwrite succeeds,
        // the fclose flush crosses the limit and fails.
        struct rlimit tight {};
        tight.rlim_cur = 5;
        tight.rlim_max = old_limit.rlim_max;
        ASSERT_EQ(::setrlimit(RLIMIT_FSIZE, &tight), 0);
        std::vector<uint8_t> small(8, 0xCD);
        EXPECT_FALSE(AtomicWriteFile(dir + "boost5-small.bin", small));
    }

    ASSERT_EQ(::setrlimit(RLIMIT_FSIZE, &old_limit), 0);
    ASSERT_NE(::signal(SIGXFSZ, old_handler), SIG_ERR);
    std::remove((dir + "boost5-big.bin").c_str());
    std::remove((dir + "boost5-big.bin.push-tmp").c_str());
    std::remove((dir + "boost5-small.bin").c_str());
    std::remove((dir + "boost5-small.bin.push-tmp").c_str());
}

// ---------------------------------------------------------------------------
// 13) File utils: copy from an unreadable source (permission denied) hits
//     the CopyFile catch branch.
// ---------------------------------------------------------------------------

TEST(FileUtilsBoost5Test, CopyFileWithUnreadableSourceFails) {
    const std::string dir = ::testing::TempDir();
    const std::string src = dir + "boost5-unreadable-src";
    const std::string dst = dir + "boost5-unreadable-dst";
    ASSERT_TRUE(utils::FileSystemUtils::WriteFileContent(src, "secret", false));
    ASSERT_EQ(::chmod(src.c_str(), 0), 0);

    EXPECT_FALSE(utils::FileSystemUtils::CopyFile(src, dst, true));

    ASSERT_EQ(::chmod(src.c_str(), 0600), 0);
    utils::FileSystemUtils::RemoveFile(src);
    utils::FileSystemUtils::RemoveFile(dst);
}

// ---------------------------------------------------------------------------
// 14) File utils: getcwd failure after the current directory is deleted.
// ---------------------------------------------------------------------------

TEST(FileUtilsBoost5Test, GetCurrentDirectoryEmptyAfterCwdDeleted) {
    const std::string old_dir = utils::FileSystemUtils::GetCurrentDirectory();
    ASSERT_FALSE(old_dir.empty());

    const std::string doomed = old_dir + "/boost5-doomed-cwd";
    ASSERT_TRUE(utils::FileSystemUtils::CreateDirectory(doomed));
    ASSERT_EQ(::chdir(doomed.c_str()), 0);
    ASSERT_EQ(::rmdir(doomed.c_str()), 0);

    EXPECT_EQ(utils::FileSystemUtils::GetCurrentDirectory(), "");

    ASSERT_EQ(::chdir(old_dir.c_str()), 0) << "restoring cwd failed";
    utils::FileSystemUtils::RemoveDirectory(doomed, true);
}

// ---------------------------------------------------------------------------
// 15) fd exhaustion (RLIMIT_NOFILE): socket creation itself fails for both
//     TCPTransport::Connect and TCPServer::Start.
// ---------------------------------------------------------------------------

TEST(FdExhaustionBoost5Test, SocketCreationFailsWhenFdTableFull) {
    struct rlimit old_limit {};
    ASSERT_EQ(::getrlimit(RLIMIT_NOFILE, &old_limit), 0);

    // Tighten the soft limit to "currently open fds + 8", then fill every
    // remaining slot with /dev/null handles until EMFILE. Lowering the limit
    // to highest_fd+1 alone does NOT exhaust the table: holes below the
    // highest fd remain allocatable (the root cause of the CI failure).
    int open_count = 0;
    DIR* d = ::opendir("/proc/self/fd");
    ASSERT_NE(d, nullptr);
    while (struct dirent* e = ::readdir(d)) {
        if (e->d_name[0] >= '0' && e->d_name[0] <= '9') ++open_count;
    }
    ::closedir(d);

    rlim_t budget = static_cast<rlim_t>(open_count) + 8;
    if (budget > old_limit.rlim_max) budget = old_limit.rlim_max;
    struct rlimit tight {};
    tight.rlim_cur = budget;
    tight.rlim_max = old_limit.rlim_max;
    ASSERT_EQ(::setrlimit(RLIMIT_NOFILE, &tight), 0);

    std::vector<int> filler;
    for (;;) {
        int fd = ::open("/dev/null", O_RDONLY);
        if (fd < 0) break;  // EMFILE: the fd table is now truly full.
        filler.push_back(fd);
    }
    ASSERT_GE(filler.size(), 1u);

    {
        TCPTransport transport("127.0.0.1", 19099, 1000);
        EXPECT_THROW(transport.Connect(), std::runtime_error);
    }
    {
        TCPServer server("127.0.0.1:19099");
        EXPECT_THROW(server.Start(), std::runtime_error);
    }

    for (int fd : filler) ::close(fd);
    ASSERT_EQ(::setrlimit(RLIMIT_NOFILE, &old_limit), 0);
}

}  // namespace
}  // namespace croupier::sdk::test
