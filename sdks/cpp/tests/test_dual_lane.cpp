// 双车道派发（CR-FIX-08，控制优先级）：业务洪峰下心跳依然可达。
//
// 回归背景：业务/控制共队列时，业务 handler 洪峰打满队列 → 心跳被
// fail-fast 拒绝 → Agent 判定会话死亡 → 过载升级为连接雪崩。双车道后
// 控制消息（心跳/注册/drain）走独立单 worker 车道，永不拒绝。
// 与 Go/JS/Java/Python 同一 golden 场景（业务饱和下心跳延迟上界）。
#include <gtest/gtest.h>

#include "croupier/sdk/protocol.h"
#include "croupier/sdk/tcp_transport.h"

#include <atomic>
#include <chrono>
#include <cstring>
#include <stdexcept>
#include <thread>

#ifdef _WIN32
#include <winsock2.h>
#include <ws2tcpip.h>
#pragma comment(lib, "ws2_32.lib")
using socket_t = SOCKET;
#define INVALID_SOCK INVALID_SOCKET
#define CLOSESOCK closesocket
#else
#include <arpa/inet.h>
#include <netinet/in.h>
#include <sys/socket.h>
#include <sys/time.h>
#include <unistd.h>
using socket_t = int;
#define INVALID_SOCK (-1)
#define CLOSESOCK ::close
#endif

namespace croupier::sdk::test {
namespace {

bool SendAll(socket_t sock, const void* buf, size_t len) {
    const char* p = static_cast<const char*>(buf);
    size_t off = 0;
    while (off < len) {
        auto n = ::send(sock, p + off, len - off, 0);
        if (n <= 0) return false;
        off += static_cast<size_t>(n);
    }
    return true;
}

bool ReadAll(socket_t sock, void* buf, size_t len) {
    char* p = static_cast<char*>(buf);
    size_t off = 0;
    while (off < len) {
        auto n = ::recv(sock, p + off, len - off, 0);
        if (n <= 0) return false;
        off += static_cast<size_t>(n);
    }
    return true;
}

// 极简假 Agent：监听一个回环端口，accept 后可推请求帧/读响应帧。
// 不做 ProviderConnect 握手——被测对象是裸 TCPTransport（握手在
// CroupierClient 层，与本场景无关）。
class FakeListener {
public:
    FakeListener() {
        // 构造器内不用 gtest 断言（ASSERT_ 在构造器中只 return 不中止
        // 用例，对象半构造会引发连锁崩溃）——用抛异常 helper，与
        // test_provider_inbound 的 RawFakeAgent 同款。
        auto fatal = [](bool ok, const char* what) {
            if (!ok) throw std::runtime_error(what);
        };
        listen_fd_ = ::socket(AF_INET, SOCK_STREAM, 0);
        fatal(listen_fd_ != INVALID_SOCK, "socket() failed");
        sockaddr_in addr {};
        addr.sin_family = AF_INET;
        addr.sin_addr.s_addr = htonl(INADDR_LOOPBACK);
        addr.sin_port = 0;
        fatal(::bind(listen_fd_, reinterpret_cast<sockaddr*>(&addr), sizeof(addr)) == 0, "bind() failed");
        fatal(::listen(listen_fd_, 1) == 0, "listen() failed");
        sockaddr_in bound {};
        socklen_t blen = sizeof(bound);
        fatal(::getsockname(listen_fd_, reinterpret_cast<sockaddr*>(&bound), &blen) == 0, "getsockname() failed");
        port_ = ntohs(bound.sin_port);
    }

    ~FakeListener() {
        if (conn_ != INVALID_SOCK) CLOSESOCK(conn_);
        if (listen_fd_ != INVALID_SOCK) CLOSESOCK(listen_fd_);
    }

    int port() const { return port_; }

    void Accept() {
        conn_ = ::accept(listen_fd_, nullptr, nullptr);
        ASSERT_NE(conn_, INVALID_SOCK);
        struct timeval tv {};
        tv.tv_sec = 0;
        tv.tv_usec = 200 * 1000; // 200ms 软读超时
        ::setsockopt(conn_, SOL_SOCKET, SO_RCVTIMEO, &tv, sizeof(tv));
    }

    void PushRequest(uint32_t msg_id, uint32_t req_id, const std::vector<uint8_t>& body) {
        auto frame = protocol::NewMessage(msg_id, req_id, body);
        std::vector<uint8_t> wrapped(4 + frame.size());
        wrapped[0] = static_cast<uint8_t>((frame.size() >> 24) & 0xFF);
        wrapped[1] = static_cast<uint8_t>((frame.size() >> 16) & 0xFF);
        wrapped[2] = static_cast<uint8_t>((frame.size() >> 8) & 0xFF);
        wrapped[3] = static_cast<uint8_t>(frame.size() & 0xFF);
        std::memcpy(wrapped.data() + 4, frame.data(), frame.size());
        ASSERT_TRUE(SendAll(conn_, wrapped.data(), wrapped.size()));
    }

    // 在 deadline 内读指定 req_id 的响应；跳过的帧不缓存（本场景只需
    // 目标帧，乱序帧忽略即可）。超时返回 false。
    bool TryReadResponseFor(uint32_t req_id, protocol::ParsedMessage& out, int timeout_ms) {
        const auto deadline = std::chrono::steady_clock::now() + std::chrono::milliseconds(timeout_ms);
        while (std::chrono::steady_clock::now() < deadline) {
            uint8_t hdr[4] = {0};
            if (!ReadAll(conn_, hdr, 4)) return false;
            uint32_t len = (uint32_t(hdr[0]) << 24) | (uint32_t(hdr[1]) << 16) |
                           (uint32_t(hdr[2]) << 8) | uint32_t(hdr[3]);
            std::vector<uint8_t> payload(len);
            if (len > 0 && !ReadAll(conn_, payload.data(), len)) return false;
            auto m = protocol::ParseMessage(payload);
            if (m.req_id == req_id) {
                out = m;
                return true;
            }
        }
        return false;
    }

private:
    socket_t listen_fd_{INVALID_SOCK};
    socket_t conn_{INVALID_SOCK};
    int port_{0};
};

// SetInboundWorkerCount 是进程级 static 覆盖：测试结束恢复默认，
// 避免污染同二进制内其它用例的 worker 数。
class WorkerCountGuard {
public:
    explicit WorkerCountGuard(int workers) { TCPTransport::SetInboundWorkerCount(workers); }
    ~WorkerCountGuard() { TCPTransport::SetInboundWorkerCount(0); }
};

TEST(DualLaneTest, IsControlRequestClassification) {
    // 控制车道成员与 Go/JS/Java/Python 同一清单。
    EXPECT_TRUE(protocol::IsControlRequest(protocol::MSG_REGISTER_REQUEST));
    EXPECT_TRUE(protocol::IsControlRequest(protocol::MSG_HEARTBEAT_REQUEST));
    EXPECT_TRUE(protocol::IsControlRequest(protocol::MSG_REGISTER_CAPABILITIES_REQ));
    EXPECT_TRUE(protocol::IsControlRequest(protocol::MSG_REGISTER_CLIENT_REQUEST));
    EXPECT_TRUE(protocol::IsControlRequest(protocol::MSG_CLIENT_HEARTBEAT_REQUEST));
    EXPECT_TRUE(protocol::IsControlRequest(protocol::MSG_PROVIDER_CONNECT_REQUEST));
    EXPECT_TRUE(protocol::IsControlRequest(protocol::MSG_PROVIDER_HEARTBEAT_REQUEST));
    EXPECT_TRUE(protocol::IsControlRequest(protocol::MSG_PROVIDER_DRAIN_REQUEST));
    EXPECT_FALSE(protocol::IsControlRequest(protocol::MSG_INVOKE_REQUEST));
    EXPECT_FALSE(protocol::IsControlRequest(protocol::MSG_INVOKE_RESPONSE));
    EXPECT_FALSE(protocol::IsControlRequest(protocol::MSG_START_TASK_REQUEST));
    EXPECT_FALSE(protocol::IsControlRequest(protocol::MSG_CANCEL_TASK_REQUEST));
}

TEST(DualLaneTest, HeartbeatSurvivesBusinessSaturation) {
    WorkerCountGuard guard(1); // 单业务 worker → 队列易打满
    FakeListener agent;
    std::thread accept_thread([&] { agent.Accept(); });

    TCPTransport transport("127.0.0.1", agent.port(), 5000);
    std::atomic<bool> business_entered{false};
    transport.SetInboundHandler([&](uint32_t msg_id, uint32_t, const std::vector<uint8_t>&) {
        if (protocol::IsControlRequest(msg_id)) {
            return std::vector<uint8_t>({'p', 'o', 'n', 'g'});
        }
        business_entered.store(true);
        std::this_thread::sleep_for(std::chrono::seconds(3)); // 占住唯一业务 worker
        return std::vector<uint8_t>{};
    });
    transport.Connect(); // void 返回；连通性由后续帧收发隐式验证
    accept_thread.join();

    // 打满业务车道：worker 1 个 + 队列上限 4 → 连发 6 个业务请求。
    for (uint32_t req_id = 101; req_id <= 106; ++req_id) {
        agent.PushRequest(protocol::MSG_INVOKE_REQUEST, req_id, {});
    }
    const auto entered_deadline = std::chrono::steady_clock::now() + std::chrono::seconds(2);
    while (!business_entered.load() && std::chrono::steady_clock::now() < entered_deadline) {
        std::this_thread::sleep_for(std::chrono::milliseconds(5));
    }
    ASSERT_TRUE(business_entered.load()) << "business handler should have started";

    // 洪峰中的心跳：控制车道必须即时应答（延迟上界 2s，业务阻塞 3s）。
    const auto start = std::chrono::steady_clock::now();
    agent.PushRequest(protocol::MSG_PROVIDER_HEARTBEAT_REQUEST, 900, {});
    protocol::ParsedMessage hb {};
    ASSERT_TRUE(agent.TryReadResponseFor(900, hb, 2000))
        << "heartbeat response must arrive during business saturation";
    const auto elapsed = std::chrono::duration_cast<std::chrono::milliseconds>(
                             std::chrono::steady_clock::now() - start)
                             .count();
    EXPECT_EQ(hb.msg_id, protocol::MSG_PROVIDER_HEARTBEAT_RESPONSE);
    std::string hb_body(hb.body.begin(), hb.body.end());
    EXPECT_EQ(hb_body, "pong");
    EXPECT_LT(elapsed, 2000) << "control lane too slow under load: " << elapsed << "ms";

    // 对照：业务队列满 → 新业务请求被 fail-fast 回空响应。
    agent.PushRequest(protocol::MSG_INVOKE_REQUEST, 901, {});
    protocol::ParsedMessage ff {};
    ASSERT_TRUE(agent.TryReadResponseFor(901, ff, 2000))
        << "saturated business lane must fast-fail with a frame";
    EXPECT_EQ(ff.msg_id, protocol::MSG_INVOKE_RESPONSE);
    EXPECT_TRUE(ff.body.empty()) << "fast-fail must carry an empty body";

    transport.Close();
}

} // namespace test
} // namespace croupier::sdk
