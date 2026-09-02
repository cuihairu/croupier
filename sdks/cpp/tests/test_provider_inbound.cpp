// Agent → Provider 入站派发路径测试（覆盖 handleAgentRequest /
// TCPTransport::DispatchInbound worker 池 / WriteResponseSilently /
// ParseTCPAddress 边界 / ParseMessage 失败路径）。
//
// RawFakeAgent 用原生 socket 而非 SDK TCPServer——TCPServer 只能被动
// 应答，无法主动向 Provider 推请求（invoke/heartbeat）。
#include <gtest/gtest.h>
#include "croupier/sdk/croupier_client.h"
#include "croupier/sdk/protocol.h"
#include "croupier/sdk/v1/provider.pb.h"
#include "croupier/sdk/v1/invocation.pb.h"

#include <atomic>
#include <chrono>
#include <cstring>
#include <memory>
#include <string>
#include <thread>
#include <vector>

#ifdef _WIN32
#include <winsock2.h>
#include <ws2tcpip.h>
#pragma comment(lib, "ws2_32.lib")
using socket_t = SOCKET;
#define INVALID_SOCK INVALID_SOCKET
#else
#include <arpa/inet.h>
#include <netinet/in.h>
#include <sys/socket.h>
#include <unistd.h>
using socket_t = int;
#define INVALID_SOCK (-1)
#define closesocket ::close
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

// 原生 fake agent：握手应答 ProviderConnect，之后可主动推任意请求帧。
class RawFakeAgent {
public:
    RawFakeAgent() {
        auto fatal = [](bool ok, const char* what) {
            if (!ok) throw std::runtime_error(what);
        };
        listen_fd_ = ::socket(AF_INET, SOCK_STREAM, 0);
        fatal(listen_fd_ != INVALID_SOCK, "socket() failed");
        sockaddr_in addr{};
        addr.sin_family = AF_INET;
        addr.sin_addr.s_addr = htonl(INADDR_LOOPBACK);
        addr.sin_port = 0;
        fatal(::bind(listen_fd_, reinterpret_cast<sockaddr*>(&addr), sizeof(addr)) == 0, "bind() failed");
        fatal(::listen(listen_fd_, 1) == 0, "listen() failed");
        sockaddr_in bound{};
        socklen_t blen = sizeof(bound);
        fatal(::getsockname(listen_fd_, reinterpret_cast<sockaddr*>(&bound), &blen) == 0, "getsockname() failed");
        port_ = ntohs(bound.sin_port);
    }

    ~RawFakeAgent() {
        if (conn_ != INVALID_SOCK) closesocket(conn_);
        if (listen_fd_ != INVALID_SOCK) closesocket(listen_fd_);
    }

    std::string address() const { return "127.0.0.1:" + std::to_string(port_); }

    void AcceptAndHandshake() {
        conn_ = ::accept(listen_fd_, nullptr, nullptr);
        ASSERT_NE(conn_, INVALID_SOCK);
        // 读 ProviderConnectRequest，应答带 session_id。
        auto first = ReadFrame();
        ASSERT_EQ(first.msg_id, protocol::MSG_PROVIDER_CONNECT_REQUEST);
        v1::ProviderConnectResponse resp;
        resp.set_session_id("raw-session-1");
        std::string out;
        resp.SerializeToString(&out);
        WriteFrame(protocol::MSG_PROVIDER_CONNECT_RESPONSE, first.req_id,
                   std::vector<uint8_t>(out.begin(), out.end()));
    }

    void PushRequest(uint32_t msg_id, uint32_t req_id, const std::vector<uint8_t>& body) {
        WriteFrame(msg_id, req_id, body);
    }

    // 接受客户端的重连（drain 恢复 / 断线重连均会发起二次握手）。
    void AcceptAndHandshakeAgain() {
        if (conn_ != INVALID_SOCK) closesocket(conn_);
        conn_ = INVALID_SOCK;
        AcceptAndHandshake();
    }

    protocol::ParsedMessage ReadFrame() {
        uint8_t hdr[4] = {0};
        if (!ReadAll(conn_, hdr, 4)) ADD_FAILURE() << "read frame header failed";
        uint32_t len = (uint32_t(hdr[0]) << 24) | (uint32_t(hdr[1]) << 16) |
                       (uint32_t(hdr[2]) << 8) | uint32_t(hdr[3]);
        std::vector<uint8_t> payload(len);
        if (len > 0 && !ReadAll(conn_, payload.data(), len)) {
            ADD_FAILURE() << "read frame body failed";
        }
        return protocol::ParseMessage(payload);
    }

    // 读一帧并等待其 req_id 匹配（响应可能乱序，跳过其它帧缓存起来由
    // 调用方后续取用——本测试文件场景为顺序小流量，直接读到即断言）。
    protocol::ParsedMessage ReadResponseFor(uint32_t req_id) {
        for (int i = 0; i < 16; ++i) {
            auto m = ReadFrame();
            if (m.req_id == req_id) return m;
        }
        ADD_FAILURE() << "expected response for req_id " << req_id << " not seen";
        return protocol::ParseMessage({});
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
        ASSERT_TRUE(SendAll(conn_, wrapped.data(), wrapped.size()));
    }

    socket_t listen_fd_{INVALID_SOCK};
    socket_t conn_{INVALID_SOCK};
    int port_{0};
};

ClientConfig ProviderConfig(const std::string& addr) {
    ClientConfig config;
    config.game_id = "game-test";
    config.env = "development";
    config.service_id = "cpp-inbound-tests";
    config.agent_addr = addr;
    config.timeout_seconds = 5;
    config.connect_timeout_seconds = 2;
    config.heartbeat_interval = 30; // 拉长心跳间隔，避免与断言交错
    config.disable_logging = true;
    return config;
}

std::vector<uint8_t> InvokeBody(const std::string& function_id, const std::string& payload) {
    v1::InvokeRequest req;
    req.set_function_id(function_id);
    req.set_payload(payload);
    std::string out;
    req.SerializeToString(&out);
    return std::vector<uint8_t>(out.begin(), out.end());
}

TEST(ProviderInboundTest, AgentInvokeReachesHandlerAndReturnsPayload) {
    RawFakeAgent agent;
    std::thread agent_thread([&] { agent.AcceptAndHandshake(); });

    CroupierClient client(ProviderConfig(agent.address()));
    FunctionDescriptor desc;
    desc.id = "test.echo";
    desc.version = "1.0.0";
    desc.operation = "echo";
    desc.capability = "action";
    desc.risk = "safe";
    std::atomic<int> calls{0};
    ASSERT_TRUE(client.RegisterFunction(
        desc, [&](const std::string&, const std::string& payload) {
            calls.fetch_add(1);
            return "echo:" + payload;
        }));
    ASSERT_TRUE(client.Connect());
    agent_thread.join();

    agent.PushRequest(protocol::MSG_INVOKE_REQUEST, 9001, InvokeBody("test.echo", "hi"));
    auto resp = agent.ReadResponseFor(9001);
    v1::InvokeResponse parsed;
    ASSERT_TRUE(parsed.ParseFromArray(resp.body.data(), static_cast<int>(resp.body.size())));
    EXPECT_EQ(parsed.payload(), "echo:hi");
    EXPECT_GE(calls.load(), 1);

    client.Close();
}

TEST(ProviderInboundTest, AgentInvokeUnknownFunctionReturnsEmpty) {
    RawFakeAgent agent;
    std::thread agent_thread([&] { agent.AcceptAndHandshake(); });

    CroupierClient client(ProviderConfig(agent.address()));
    FunctionDescriptor desc;
    desc.id = "known.fn";
    desc.version = "1.0.0";
    desc.operation = "noop";
    desc.capability = "action";
    desc.risk = "safe";
    ASSERT_TRUE(client.RegisterFunction(
        desc, [](const std::string&, const std::string&) { return std::string("{}"); }));
    ASSERT_TRUE(client.Connect());
    agent_thread.join();

    agent.PushRequest(protocol::MSG_INVOKE_REQUEST, 9002, InvokeBody("missing.fn", ""));
    auto resp = agent.ReadResponseFor(9002);
    EXPECT_TRUE(resp.body.empty()); // 未注册函数 → 空响应体
    client.Close();
}

TEST(ProviderInboundTest, AgentInvokeGarbageBodyIsContained) {
    // 无效 protobuf 体：ParseMessage 抛出由 handleAgentRequest 捕获，
    // 回空响应（不崩连接、不断会话）。
    RawFakeAgent agent;
    std::thread agent_thread([&] { agent.AcceptAndHandshake(); });

    CroupierClient client(ProviderConfig(agent.address()));
    FunctionDescriptor desc;
    desc.id = "test.echo";
    desc.version = "1.0.0";
    desc.operation = "echo";
    desc.capability = "action";
    desc.risk = "safe";
    ASSERT_TRUE(client.RegisterFunction(
        desc, [](const std::string&, const std::string& p) { return p; }));
    ASSERT_TRUE(client.Connect());
    agent_thread.join();

    std::vector<uint8_t> garbage{0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0x01};
    agent.PushRequest(protocol::MSG_INVOKE_REQUEST, 9003, garbage);
    auto resp = agent.ReadResponseFor(9003);
    EXPECT_TRUE(resp.body.empty());

    // 连接仍可用：后续合法请求正常应答。
    agent.PushRequest(protocol::MSG_INVOKE_REQUEST, 9004, InvokeBody("test.echo", "still-alive"));
    auto resp2 = agent.ReadResponseFor(9004);
    v1::InvokeResponse parsed;
    ASSERT_TRUE(parsed.ParseFromArray(resp2.body.data(), static_cast<int>(resp2.body.size())));
    EXPECT_EQ(parsed.payload(), "still-alive");
    client.Close();
}

TEST(ProviderInboundTest, AgentHeartbeatPongKeepsSessionAlive) {
    RawFakeAgent agent;
    std::thread agent_thread([&] { agent.AcceptAndHandshake(); });

    CroupierClient client(ProviderConfig(agent.address()));
    FunctionDescriptor desc;
    desc.id = "test.echo";
    desc.version = "1.0.0";
    desc.operation = "echo";
    desc.capability = "action";
    desc.risk = "safe";
    ASSERT_TRUE(client.RegisterFunction(
        desc, [](const std::string&, const std::string& p) { return p; }));
    ASSERT_TRUE(client.Connect());
    agent_thread.join();

    agent.PushRequest(protocol::MSG_PROVIDER_HEARTBEAT_REQUEST, 9005, {});
    auto resp = agent.ReadResponseFor(9005);
    // 空 ProviderHeartbeatResponse 序列化为空体，但帧本身到达即证明 pong 路径。
    EXPECT_EQ(resp.msg_id, protocol::MSG_PROVIDER_HEARTBEAT_RESPONSE);
    client.Close();
}

TEST(ProviderInboundTest, ConcurrentInvocationsAllAnswered) {
    // 有界 worker 池并发派发：N 个并发 invoke 全部应答且 payload 正确。
    RawFakeAgent agent;
    std::thread agent_thread([&] { agent.AcceptAndHandshake(); });

    CroupierClient client(ProviderConfig(agent.address()));
    FunctionDescriptor desc;
    desc.id = "test.slow";
    desc.version = "1.0.0";
    desc.operation = "slow";
    desc.capability = "action";
    desc.risk = "safe";
    ASSERT_TRUE(client.RegisterFunction(
        desc, [](const std::string&, const std::string& payload) {
            std::this_thread::sleep_for(std::chrono::milliseconds(30));
            return "done:" + payload;
        }));
    ASSERT_TRUE(client.Connect());
    agent_thread.join();

    constexpr uint32_t kTotal = 16;
    for (uint32_t i = 0; i < kTotal; ++i) {
        agent.PushRequest(protocol::MSG_INVOKE_REQUEST, 10000 + i,
                          InvokeBody("test.slow", std::to_string(i)));
    }
    int ok = 0;
    for (uint32_t i = 0; i < kTotal; ++i) {
        auto resp = agent.ReadFrame();
        if (resp.body.empty()) continue; // 队列饱和时的快速失败也合法
        v1::InvokeResponse parsed;
        if (parsed.ParseFromArray(resp.body.data(), static_cast<int>(resp.body.size())) &&
            parsed.payload().rfind("done:", 0) == 0) {
            ++ok;
        }
    }
    // 慢 handler 30ms × 16 / worker 数——余量充足，全部应答。
    EXPECT_EQ(ok, kTotal);
    client.Close();
}

TEST(ClientAddressTest, InvalidTCPAddressesRejected) {
    // Connect() 内部捕获 ParseTCPAddress 异常返回 false（不外抛）。
    const std::string bad_addresses[] = {"127.0.0.1", "no-port", "[::1]", "[::1]noport",
                                         "127.0.0.1:notaport", "127.0.0.1:0", "127.0.0.1:70000"};
    for (const std::string& bad : bad_addresses) {
        CroupierClient client(ProviderConfig(bad));
        EXPECT_FALSE(client.Connect()) << "address: " << bad;
    }
}

TEST(ClientAddressTest, HTTPSchemeRejectedForTCPClient) {
    // http:// 属 Invoker 的 Server 地址，Provider TCP 客户端必须拒绝。
    CroupierClient client(ProviderConfig("http://127.0.0.1:18780"));
    EXPECT_FALSE(client.Connect());
}

} // namespace
} // namespace croupier::sdk::test

namespace croupier::sdk::test {
namespace {

// Agent 下发 drain：客户端立即回空确认并置位 IsDraining；
// drain 期间新 Invoke 被拒（错误 payload）；恢复后状态清除。
TEST(ProviderInboundTest, AgentDrainAcksRejectsInvokeAndRecovers) {
    RawFakeAgent agent;
    std::thread agent_thread([&] { agent.AcceptAndHandshake(); });

    CroupierClient client(ProviderConfig(agent.address()));
    FunctionDescriptor desc;
    desc.id = "test.echo";
    desc.version = "1.0.0";
    desc.operation = "echo";
    desc.capability = "action";
    desc.risk = "safe";
    std::atomic<int> calls{0};
    ASSERT_TRUE(client.RegisterFunction(
        desc, [&](const std::string&, const std::string& payload) {
            calls.fetch_add(1);
            std::this_thread::sleep_for(std::chrono::milliseconds(120));
            return "echo:" + payload;
        }));
    ASSERT_TRUE(client.Connect());
    agent_thread.join();

    EXPECT_FALSE(client.IsDraining());

    // 在途调用先行：handler 睡 120ms，drain 必须等它完成
    agent.PushRequest(protocol::MSG_INVOKE_REQUEST, 9101, InvokeBody("test.echo", "inflight"));
    std::this_thread::sleep_for(std::chrono::milliseconds(30));

    // 推 drain 请求（req_id 9102）
    agent.PushRequest(protocol::MSG_PROVIDER_DRAIN_REQUEST, 9102, {});

    // drain 期间的新 Invoke 被拒（handler 不应执行）
    agent.PushRequest(protocol::MSG_INVOKE_REQUEST, 9103, InvokeBody("test.echo", "rejected"));
    auto rejected = agent.ReadResponseFor(9103);
    v1::InvokeResponse rejected_resp;
    ASSERT_TRUE(rejected_resp.ParseFromArray(rejected.body.data(), static_cast<int>(rejected.body.size())));
    EXPECT_NE(rejected_resp.payload().find("provider is draining"), std::string::npos);
    EXPECT_EQ(calls.load(), 1); // 只有在途那次执行了

    // drain 确认帧（空 ProviderDrainResponse）
    auto ack = agent.ReadResponseFor(9102);
    EXPECT_EQ(ack.msg_id, protocol::MSG_PROVIDER_DRAIN_RESPONSE);
    EXPECT_TRUE(ack.body.empty());

    // 等在途完成 + 恢复（auto_reconnect 默认 true → 重连重注册）
    std::thread reconnect_thread([&] { agent.AcceptAndHandshakeAgain(); });
    reconnect_thread.join();
    for (int i = 0; i < 50 && client.IsDraining(); ++i) {
        std::this_thread::sleep_for(std::chrono::milliseconds(20));
    }
    EXPECT_FALSE(client.IsDraining());

    client.Close();
}

// drain 幂等：重复请求只回确认，不重复触发恢复。
TEST(ProviderInboundTest, AgentDrainIsIdempotent) {
    RawFakeAgent agent;
    std::thread agent_thread([&] { agent.AcceptAndHandshake(); });

    CroupierClient client(ProviderConfig(agent.address()));
    FunctionDescriptor desc;
    desc.id = "test.echo";
    desc.version = "1.0.0";
    desc.operation = "echo";
    desc.capability = "action";
    desc.risk = "safe";
    ASSERT_TRUE(client.RegisterFunction(
        desc, [](const std::string&, const std::string& payload) { return payload; }));
    ASSERT_TRUE(client.Connect());
    agent_thread.join();

    agent.PushRequest(protocol::MSG_PROVIDER_DRAIN_REQUEST, 9201, {});
    auto ack1 = agent.ReadResponseFor(9201);
    EXPECT_EQ(ack1.msg_id, protocol::MSG_PROVIDER_DRAIN_RESPONSE);
    agent.PushRequest(protocol::MSG_PROVIDER_DRAIN_REQUEST, 9202, {});
    auto ack2 = agent.ReadResponseFor(9202);
    EXPECT_EQ(ack2.msg_id, protocol::MSG_PROVIDER_DRAIN_RESPONSE);
    EXPECT_TRUE(client.IsDraining());

    client.Close();
}

}  // namespace
}  // namespace croupier::sdk::test
