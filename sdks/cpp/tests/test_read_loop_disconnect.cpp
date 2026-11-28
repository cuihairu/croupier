// AUD-072：读循环自行退出（对端正常 EOF / 网络错误）必须立即：
//   (a) 唤醒并失败本连接的全部 pending 请求（而非挂到超时）；
//   (b) 清除连接状态（IsConnected() 立即为 false，新 Call 直接拒绝）；
//   (c) 触发 Owner 的重连机制——且回收绝不自 join（读线程 join 自己=死锁），
//       后续重连使用全新线程与全新 pending 队列（旧尾巴不得污染新连接）。
//
// 回归背景（todo AUD-072，tcp_transport.cpp 旧 ReadLoop 尾注释
// “do NOT recursively call Close()”）：读循环退出后什么都不发布——
// pending Call 挂满 timeout_ms_，IsConnected() 继续报 true，CroupierClient
// 只能等下一次心跳（默认 60s）才间接发现断连。
#include <gtest/gtest.h>

#include "croupier/sdk/croupier_client.h"
#include "croupier/sdk/logger.h"
#include "croupier/sdk/protocol.h"
#include "croupier/sdk/tcp_transport.h"
#include "croupier/sdk/v1/provider.pb.h"

#include <atomic>
#include <chrono>
#include <cstring>
#include <future>
#include <stdexcept>
#include <thread>
#include <vector>

#ifdef _WIN32
#include <winsock2.h>
#include <ws2tcpip.h>
#pragma comment(lib, "ws2_32.lib")
using raw_socket_t = SOCKET;
#define RAW_INVALID INVALID_SOCKET
#define RAW_CLOSE closesocket
#else
#include <arpa/inet.h>
#include <netinet/in.h>
#include <sys/socket.h>
#include <unistd.h>
using raw_socket_t = int;
#define RAW_INVALID (-1)
#define RAW_CLOSE ::close
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

std::vector<uint8_t> WrapFrame(const std::vector<uint8_t>& payload) {
    std::vector<uint8_t> wrapped(4 + payload.size());
    wrapped[0] = static_cast<uint8_t>((payload.size() >> 24) & 0xFF);
    wrapped[1] = static_cast<uint8_t>((payload.size() >> 16) & 0xFF);
    wrapped[2] = static_cast<uint8_t>((payload.size() >> 8) & 0xFF);
    wrapped[3] = static_cast<uint8_t>(payload.size() & 0xFF);
    std::memcpy(wrapped.data() + 4, payload.data(), payload.size());
    return wrapped;
}

// 极简假 Agent：裸 socket 监听，accept 后可读请求帧/回响应帧，并支持
// 两种断连方式（clean FIN 与 RST）以及同一 listener 上的二次 accept
// （重连场景）。不做 ProviderConnect 握手——被测对象是裸 TCPTransport。
class EofFakeAgent {
public:
    EofFakeAgent() {
        auto fatal = [](bool ok, const char* what) {
            if (!ok) throw std::runtime_error(what);
        };
        listen_fd_ = ::socket(AF_INET, SOCK_STREAM, 0);
        fatal(listen_fd_ != RAW_INVALID, "socket() failed");
        sockaddr_in addr {};
        addr.sin_family = AF_INET;
        addr.sin_addr.s_addr = htonl(INADDR_LOOPBACK);
        addr.sin_port = 0;
        fatal(::bind(listen_fd_, reinterpret_cast<sockaddr*>(&addr), sizeof(addr)) == 0, "bind() failed");
        fatal(::listen(listen_fd_, 4) == 0, "listen() failed");
        sockaddr_in bound {};
        socklen_t blen = sizeof(bound);
        fatal(::getsockname(listen_fd_, reinterpret_cast<sockaddr*>(&bound), &blen) == 0, "getsockname() failed");
        port_ = ntohs(bound.sin_port);
    }

    ~EofFakeAgent() {
        if (conn_ != RAW_INVALID) RAW_CLOSE(conn_);
        if (listen_fd_ != RAW_INVALID) RAW_CLOSE(listen_fd_);
    }

    int port() const { return port_; }

    // 阻塞 accept 一个连接（构造器外调用，可用 ASSERT）。
    void Accept() {
        conn_ = ::accept(listen_fd_, nullptr, nullptr);
        ASSERT_NE(conn_, RAW_INVALID);
        struct timeval tv {};
        tv.tv_sec = 0;
        tv.tv_usec = 200 * 1000;  // 200ms 软读超时：deadline 轮询可生效
        ::setsockopt(conn_, SOL_SOCKET, SO_RCVTIMEO, &tv, sizeof(tv));
    }

    // 在 deadline 内读一个完整帧；超时/对端关闭返回 false。
    bool ReadFrame(protocol::ParsedMessage& out, int timeout_ms) {
        const auto deadline = std::chrono::steady_clock::now() + std::chrono::milliseconds(timeout_ms);
        uint8_t hdr[4] = {0};
        if (!RecvN(hdr, 4, deadline)) return false;
        uint32_t len = (static_cast<uint32_t>(hdr[0]) << 24) | (static_cast<uint32_t>(hdr[1]) << 16) |
                       (static_cast<uint32_t>(hdr[2]) << 8) | static_cast<uint32_t>(hdr[3]);
        std::vector<uint8_t> payload(len);
        if (len > 0 && !RecvN(payload.data(), len, deadline)) return false;
        out = protocol::ParseMessage(payload);
        return true;
    }

    // 以指定 msg_id 回应同一 req_id（payload 原样回显进响应体）。
    bool SendResponse(uint32_t msg_id, const protocol::ParsedMessage& req, const std::vector<uint8_t>& body) {
        auto frame = protocol::NewMessage(msg_id, req.req_id, body);
        auto wrapped = WrapFrame(frame);
        return SendAllRaw(conn_, wrapped.data(), wrapped.size());
    }

    // 正常对端关闭：FIN（clean EOF），读侧 recv 返回 0。
    void CloseClean() {
        if (conn_ != RAW_INVALID) {
            ::shutdown(conn_, SHUT_WR);
            RAW_CLOSE(conn_);
            conn_ = RAW_INVALID;
        }
    }

    // 网络错误路径：SO_LINGER 0 + close → RST，读侧 recv 得 ECONNRESET。
    void CloseReset() {
        if (conn_ != RAW_INVALID) {
            linger lg {};
            lg.l_onoff = 1;
            lg.l_linger = 0;
            ::setsockopt(conn_, SOL_SOCKET, SO_LINGER, &lg, sizeof(lg));
            RAW_CLOSE(conn_);
            conn_ = RAW_INVALID;
        }
    }

private:
    bool RecvN(void* buf, size_t len, const std::chrono::steady_clock::time_point& deadline) {
        char* p = static_cast<char*>(buf);
        size_t off = 0;
        while (off < len) {
            if (std::chrono::steady_clock::now() >= deadline) return false;
            auto n = ::recv(conn_, p + off, len - off, 0);
            if (n == 0) return false;  // peer closed
            if (n < 0) {
#ifdef _WIN32
                int err = WSAGetLastError();
                if (err == WSAEINTR) continue;
                if (err == WSAEWOULDBLOCK) {
                    std::this_thread::sleep_for(std::chrono::milliseconds(5));
                    continue;
                }
#else
                if (errno == EINTR) continue;
                if (errno == EAGAIN || errno == EWOULDBLOCK) {
                    std::this_thread::sleep_for(std::chrono::milliseconds(5));
                    continue;
                }
#endif
                return false;
            }
            off += static_cast<size_t>(n);
        }
        return true;
    }

    raw_socket_t listen_fd_{RAW_INVALID};
    raw_socket_t conn_{RAW_INVALID};
    int port_{0};
};

// 在 deadline 内等待断开状态生效（publish 由读线程异步完成）。
bool WaitForDisconnected(const TCPTransport& transport, int timeout_ms) {
    for (int i = 0; i < timeout_ms / 10 && transport.IsConnected(); ++i) {
        std::this_thread::sleep_for(std::chrono::milliseconds(10));
    }
    return !transport.IsConnected();
}

// 在 deadline 内等待断开回调命中目标次数。
bool WaitForCallback(std::atomic<int>& count, int target, int timeout_ms) {
    for (int i = 0; i < timeout_ms / 10 && count.load() < target; ++i) {
        std::this_thread::sleep_for(std::chrono::milliseconds(10));
    }
    return count.load() >= target;
}

}  // namespace

// ---- (a)+(b)：对端 clean EOF → pending Call 立即失败 + 状态清除 ----
TEST(ReadLoopDisconnectTest, PeerCleanEofFailsPendingCallPromptlyAndClearsState) {
    EofFakeAgent agent;
    std::thread accept_thread([&] { agent.Accept(); });

    // 8s 请求超时：修复前该用例只能等满 8s 超时（红灯的另一个信号）。
    TCPTransport transport("127.0.0.1", agent.port(), /*timeout_ms=*/8000);
    transport.Connect();
    accept_thread.join();
    ASSERT_TRUE(transport.IsConnected());

    auto outcome = std::async(std::launch::async, [&]() -> std::string {
        try {
            auto resp = transport.Call(protocol::MSG_PROVIDER_HEARTBEAT_REQUEST, {0x01});
            return "returned: msg_id=" + std::to_string(resp.first) + " body=" + std::to_string(resp.second.size());
        } catch (const std::exception& e) {
            return std::string("threw: ") + e.what();
        }
    });

    // 确定性时序：Call 先登记 pending latch 再写帧，agent 读到帧即代表
    // 请求已在飞（pending 必然已登记）。
    protocol::ParsedMessage req;
    ASSERT_TRUE(agent.ReadFrame(req, 4000));
    EXPECT_EQ(req.msg_id, static_cast<uint32_t>(protocol::MSG_PROVIDER_HEARTBEAT_REQUEST));

    const auto close_time = std::chrono::steady_clock::now();
    agent.CloseClean();  // 对端正常 EOF

    ASSERT_TRUE(outcome.wait_for(std::chrono::seconds(4)) == std::future_status::ready)
        << "pending Call not woken after peer clean EOF";
    const auto elapsed_ms = std::chrono::duration_cast<std::chrono::milliseconds>(
                                std::chrono::steady_clock::now() - close_time)
                                .count();
    const std::string result = outcome.get();
    // 不得把断连伪装成“空响应成功返回”；必须抛连接类错误。
    EXPECT_TRUE(result.rfind("threw: ", 0) == 0) << result;
    EXPECT_NE(result.find("Connection"), std::string::npos) << result;
    EXPECT_LT(elapsed_ms, 3000) << "wake took " << elapsed_ms << "ms after EOF";

    // (b) 连接状态立即清除。
    EXPECT_TRUE(WaitForDisconnected(transport, 2000));
    // 死连接上的新 Call 直接拒绝（不再等到超时）。
    EXPECT_THROW(transport.Call(protocol::MSG_PROVIDER_HEARTBEAT_REQUEST, {0x02}), std::runtime_error);

    // 读线程自 detach 后，Close() 不得自 join/挂起。
    const auto close_start = std::chrono::steady_clock::now();
    transport.Close();
    const auto close_ms = std::chrono::duration_cast<std::chrono::milliseconds>(
                              std::chrono::steady_clock::now() - close_start)
                              .count();
    EXPECT_LT(close_ms, 2000) << "Close() after self-exit took " << close_ms << "ms";
}

// ---- 网络错误（RST）同款语义 ----
TEST(ReadLoopDisconnectTest, NetworkErrorRstFailsPendingCallPromptlyAndClearsState) {
    EofFakeAgent agent;
    std::thread accept_thread([&] { agent.Accept(); });

    TCPTransport transport("127.0.0.1", agent.port(), /*timeout_ms=*/8000);
    transport.Connect();
    accept_thread.join();
    ASSERT_TRUE(transport.IsConnected());

    auto outcome = std::async(std::launch::async, [&]() -> std::string {
        try {
            auto resp = transport.Call(protocol::MSG_PROVIDER_HEARTBEAT_REQUEST, {0x01});
            return "returned: msg_id=" + std::to_string(resp.first);
        } catch (const std::exception& e) {
            return std::string("threw: ") + e.what();
        }
    });

    protocol::ParsedMessage req;
    ASSERT_TRUE(agent.ReadFrame(req, 4000));

    const auto close_time = std::chrono::steady_clock::now();
    agent.CloseReset();  // 网络错误路径（RST / ECONNRESET）

    ASSERT_TRUE(outcome.wait_for(std::chrono::seconds(4)) == std::future_status::ready)
        << "pending Call not woken after connection reset";
    const auto elapsed_ms = std::chrono::duration_cast<std::chrono::milliseconds>(
                                std::chrono::steady_clock::now() - close_time)
                                .count();
    const std::string result = outcome.get();
    EXPECT_TRUE(result.rfind("threw: ", 0) == 0) << result;
    EXPECT_LT(elapsed_ms, 3000) << "wake took " << elapsed_ms << "ms after RST";
    EXPECT_TRUE(WaitForDisconnected(transport, 2000));
    transport.Close();
}

// ---- 断开回调：自退出必触发、显式 Close 不触发、仅一次 ----
TEST(ReadLoopDisconnectTest, DisconnectCallbackFiresOnSelfExitOnly) {
    // Part A：显式 Close → 无回调（close 语义由 Close 自身负责）。
    {
        EofFakeAgent agent;
        std::thread accept_thread([&] { agent.Accept(); });
        TCPTransport transport("127.0.0.1", agent.port(), 5000);
        std::atomic<int> cb_count{0};
        transport.SetDisconnectCallback([&cb_count] { cb_count.fetch_add(1); });
        transport.Connect();
        accept_thread.join();
        ASSERT_TRUE(transport.IsConnected());
        transport.Close();
        std::this_thread::sleep_for(std::chrono::milliseconds(300));
        EXPECT_EQ(cb_count.load(), 0) << "explicit Close must not fire the disconnect callback";
    }

    // Part B：对端 EOF → 恰好一次回调 + 状态清除。
    {
        EofFakeAgent agent;
        std::thread accept_thread([&] { agent.Accept(); });
        TCPTransport transport("127.0.0.1", agent.port(), 5000);
        std::atomic<int> cb_count{0};
        transport.SetDisconnectCallback([&cb_count] { cb_count.fetch_add(1); });
        transport.Connect();
        accept_thread.join();
        ASSERT_TRUE(transport.IsConnected());

        protocol::ParsedMessage req;
        std::atomic<bool> ok{false};
        std::thread responder([&] {
            ok.store(agent.ReadFrame(req, 4000) &&
                     agent.SendResponse(protocol::MSG_INVOKE_RESPONSE, req, {0x01}));
        });
        // 完整往返，确保连接真实在用（也验证回调不是“空转就触发”）。
        auto resp = transport.Call(protocol::MSG_INVOKE_REQUEST, {0x10});
        responder.join();
        ASSERT_TRUE(ok.load());
        EXPECT_EQ(resp.second, (std::vector<uint8_t>{0x01}));

        agent.CloseClean();
        EXPECT_TRUE(WaitForCallback(cb_count, 1, 2000)) << "self-exit must fire the disconnect callback";
        // 发布是一次性的：不允许重复触发。
        std::this_thread::sleep_for(std::chrono::milliseconds(200));
        EXPECT_EQ(cb_count.load(), 1);
        EXPECT_FALSE(transport.IsConnected());
        transport.Close();
    }
}

// ---- 后续重连不复用旧线程与旧队列（同对象重连） ----
TEST(ReadLoopDisconnectTest, ReconnectAfterSelfExitUsesFreshThreadAndQueue) {
    EofFakeAgent agent;
    std::thread accept_thread([&] { agent.Accept(); });

    TCPTransport transport("127.0.0.1", agent.port(), /*timeout_ms=*/5000);
    transport.Connect();
    accept_thread.join();
    ASSERT_TRUE(transport.IsConnected());

    // 第一条连接上的完整往返。
    {
        protocol::ParsedMessage req;
        std::atomic<bool> ok{false};
        std::thread responder([&] {
            ok.store(agent.ReadFrame(req, 4000) &&
                     agent.SendResponse(protocol::MSG_INVOKE_RESPONSE, req, {0xAA}));
        });
        auto resp = transport.Call(protocol::MSG_INVOKE_REQUEST, {0x11});
        responder.join();
        ASSERT_TRUE(ok.load());
        EXPECT_EQ(resp.second, (std::vector<uint8_t>{0xAA}));
    }

    // 读循环自行退出（对端 EOF）。
    agent.CloseClean();
    ASSERT_TRUE(WaitForDisconnected(transport, 2000));

    // 同对象重连：若旧读线程未自 detach（仍 joinable），这里对
    // read_thread_ 直接赋新线程会 std::terminate——进程存活即证明
    // “旧线程已让位”。旧尾巴（若有）也被 epoch 守卫挡在新队列外。
    std::thread accept2([&] { agent.Accept(); });
    transport.Connect();
    accept2.join();
    ASSERT_TRUE(transport.IsConnected());

    // 新连接新队列：往返正常，且旧连接的 req_id 语义不再干扰。
    {
        protocol::ParsedMessage req;
        std::atomic<bool> ok{false};
        std::thread responder([&] {
            ok.store(agent.ReadFrame(req, 4000) &&
                     agent.SendResponse(protocol::MSG_INVOKE_RESPONSE, req, {0xBB}));
        });
        auto resp = transport.Call(protocol::MSG_INVOKE_REQUEST, {0x22});
        responder.join();
        ASSERT_TRUE(ok.load());
        EXPECT_EQ(resp.second, (std::vector<uint8_t>{0xBB}));
        EXPECT_NE(resp.second, (std::vector<uint8_t>{0xAA}));
    }

    transport.Close();
}

// ---- (c)：Owner（CroupierClient）断开后立即重连 ----
// heartbeat_interval 钉成 1 小时：修复前 client 只能等下一次心跳（一小时后）
// 才发现死连接；修复后断开回调踢醒心跳线程，重连立刻开始。
TEST(ReadLoopDisconnectTest, ClientReconnectsPromptlyAfterPeerEof) {
    using ::croupier::sdk::v1::ProviderConnectRequest;
    using ::croupier::sdk::v1::ProviderConnectResponse;
    using ::croupier::sdk::v1::ProviderHeartbeatRequest;

    // 复用 SDK 自带 TCPServer 当假 Agent（ProviderConnect 握手 + 心跳应答）。
    // 端口先预留固定下来：restart() 必须重绑同一端口，否则 client 重拨的
    // 还是旧端口（test_client_provider 的 ReserveLoopbackPort 同款）。
    class FakeProviderAgent {
    public:
        FakeProviderAgent()
            : server_(/*listen_address=*/"127.0.0.1:" + std::to_string(ReserveLoopbackPort()),
                      /*timeout_ms=*/5000) {
            server_.SetHandler([](uint32_t msg_type, uint32_t, const std::vector<uint8_t>& body)
                                   -> std::vector<uint8_t> {
                if (msg_type == protocol::MSG_PROVIDER_CONNECT_REQUEST) {
                    ProviderConnectRequest request;
                    (void)request.ParseFromArray(body.data(), static_cast<int>(body.size()));
                    ProviderConnectResponse response;
                    response.set_session_id("session-aud072");
                    std::string out;
                    response.SerializeToString(&out);
                    return std::vector<uint8_t>(out.begin(), out.end());
                }
                if (msg_type == protocol::MSG_PROVIDER_HEARTBEAT_REQUEST) {
                    (void)ProviderHeartbeatRequest();
                    return {};
                }
                return {};
            });
            server_.Start();
        }
        ~FakeProviderAgent() { server_.Stop(); }
        std::string address() const { return server_.GetListenAddress(); }
        void stop() { server_.Stop(); }
        void restart() { server_.Start(); }

    private:
        // 用 TCPServer 探针挑一个空闲回环端口（拉起-取地址-关掉）。
        static int ReserveLoopbackPort() {
            TCPServer probe("127.0.0.1:0");
            probe.Start();
            const std::string address = probe.GetListenAddress();
            probe.Stop();
            return std::stoi(address.substr(address.rfind(':') + 1));
        }

        TCPServer server_;
    };

    FakeProviderAgent agent;
    ClientConfig config;
    config.agent_addr = agent.address();
    config.service_id = "cpp-aud072";
    config.game_id = "game-test";
    config.timeout_seconds = 2;
    config.connect_timeout_seconds = 2;
    config.heartbeat_interval = 3600;  // 1 小时：无 kick 则重连永不发生
    config.disable_logging = true;

    CroupierClient client(config);
    FunctionDescriptor desc;
    desc.id = "test.echo.aud072";
    desc.version = "1.0.0";
    desc.resource = "test";
    desc.operation = "echo";
    desc.capability = "action";
    desc.risk = "safe";
    ASSERT_TRUE(client.RegisterFunction(
        desc, [](const std::string&, const std::string& payload) { return payload; }));
    ASSERT_TRUE(client.Connect());
    ASSERT_TRUE(client.IsConnected());

    // 杀掉既有会话（对端 EOF：TCPServer::Stop 会 shutdown+closesocket 全部
    // 已建立连接），然后立刻把 listener 拉起来等重连。
    agent.stop();

    // 断开回调必须让 client 状态立即翻转（而非等一小时后的心跳）。
    bool disconnected = false;
    for (int i = 0; i < 200 && !disconnected; ++i) {
        std::this_thread::sleep_for(std::chrono::milliseconds(10));
        disconnected = !client.IsConnected();
    }
    ASSERT_TRUE(disconnected) << "client state not cleared within 2s of peer EOF";

    agent.restart();
    bool reconnected = false;
    for (int i = 0; i < 1200 && !reconnected; ++i) {  // bounded ~12s
        std::this_thread::sleep_for(std::chrono::milliseconds(10));
        reconnected = client.IsConnected();
    }
    EXPECT_TRUE(reconnected) << "no reconnect within 12s after peer EOF";
    client.Stop();
    std::this_thread::sleep_for(std::chrono::milliseconds(150));
}

// 传输层逐帧 hex dump 必须走 SDK Logger 且被级别门控（AUD-COV-004）：默认
// INFO 级别下一次 Call 不得向控制台输出任何逐帧调试行（此前是无条件
// std::cerr——每帧一次格式化 + 无缓冲 stderr I/O，游戏进程日志被刷屏）；
// 显式调到 DEBUG 后同一行日志必须可见。用 std::cout 重定向捕获 Logger 的
// 控制台输出（Logger 简单 sink 写 std::cout，见 logger.h）。
TEST(ReadLoopDisconnectTest, SendPathDebugLogIsLevelGated) {
    // std::cout 重定向（test_logger.cpp 的 CoutRedirect 同款，本地复制避免
    // 跨测试文件共享 fixture）。
    class CoutCapture {
    public:
        CoutCapture() : old_(std::cout.rdbuf(buffer_.rdbuf())) {}
        ~CoutCapture() { std::cout.rdbuf(old_); }
        std::string str() const { return buffer_.str(); }

    private:
        std::ostringstream buffer_;
        std::streambuf* old_;
    };

    auto& logger = ::croupier::sdk::Logger::GetInstance();
    logger.SetLevel(::croupier::sdk::Logger::Level::INFO);

    EofFakeAgent agent;
    std::thread accept_thread([&] { agent.Accept(); });
    TCPTransport transport("127.0.0.1", agent.port(), /*timeout_ms=*/8000);
    transport.Connect();
    accept_thread.join();
    ASSERT_TRUE(transport.IsConnected());

    const auto heartbeat = static_cast<uint32_t>(protocol::MSG_PROVIDER_HEARTBEAT_REQUEST);
    const auto heartbeat_resp = heartbeat + 1;  // GetResponseMsgID 约定：请求+1
    auto round_trip = [&](std::string& captured) {
        CoutCapture capture;
        auto outcome = std::async(std::launch::async, [&]() {
            auto resp = transport.Call(heartbeat, {0x01});
            return resp.second.size();
        });
        protocol::ParsedMessage req;
        ASSERT_TRUE(agent.ReadFrame(req, 4000));
        ASSERT_TRUE(agent.SendResponse(heartbeat_resp, req, {0x02}));
        ASSERT_TRUE(outcome.wait_for(std::chrono::seconds(4)) == std::future_status::ready);
        captured = capture.str();
    };

    std::string info_capture;
    round_trip(info_capture);
    EXPECT_EQ(info_capture.find("Sending msg_type="), std::string::npos)
        << "per-frame debug dump leaked at default INFO level: " << info_capture;

    logger.SetLevel(::croupier::sdk::Logger::Level::DEBUG);
    std::string debug_capture;
    round_trip(debug_capture);
    EXPECT_NE(debug_capture.find("Sending msg_type="), std::string::npos)
        << "debug dump must stay available when DEBUG is explicitly enabled";
    logger.SetLevel(::croupier::sdk::Logger::Level::INFO);  // 恢复默认级别

    transport.Close();
}

}  // namespace croupier::sdk::test
