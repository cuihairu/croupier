/**
 * @file tcp_transport.h
 * @brief TCP Transport Layer for Croupier C++ SDK.
 *
 * Implements TCP-based transport for communication with Croupier Agent
 * using the Croupier wire protocol with multiplexed request/response.
 */

#ifndef CROUPIER_SDK_TCP_TRANSPORT_H
#define CROUPIER_SDK_TCP_TRANSPORT_H

#include <atomic>
#include <condition_variable>
#include <functional>
#include <memory>
#include <mutex>
#include <queue>
#include <string>
#include <thread>
#include <unordered_map>
#include <vector>

#ifdef _WIN32
#ifndef NOMINMAX
#define NOMINMAX
#endif
#include <winsock2.h>
#include <ws2tcpip.h>
typedef SOCKET socket_t;
#define INVALID_SOCKET_VALUE INVALID_SOCKET
#define SOCKET_ERROR_VALUE SOCKET_ERROR
#else
#include <arpa/inet.h>
#include <fcntl.h>
#include <netdb.h>
#include <netinet/in.h>
#include <sys/socket.h>
#include <unistd.h>
typedef int socket_t;
#define INVALID_SOCKET_VALUE -1
#define SOCKET_ERROR_VALUE -1
#define closesocket close
#endif

#include "protocol.h"

namespace croupier {
namespace sdk {

// Forward declaration
class TCPServer;

/**
 * Handler function type for processing incoming requests.
 * @param msg_type Protocol message type
 * @param req_id Request ID from protocol header
 * @param body Request body (serialized protobuf)
 * @return Response body (serialized protobuf)
 */
using MessageHandler = std::function<std::vector<uint8_t>(uint32_t msg_type, uint32_t req_id, const std::vector<uint8_t>& body)>;

/**
 * TCP-based transport client using Croupier wire protocol.
 * Supports multiplexed request/response over a single connection.
 */
class TCPTransport {
public:
    /**
     * Initialize TCP transport.
     *
     * @param host Agent host
     * @param port Agent port
     * @param timeout_ms Request timeout in milliseconds
     */
    TCPTransport(const std::string& host = "127.0.0.1",
                 int port = 19091,
                 int timeout_ms = 30000);

    ~TCPTransport();

    // Non-copyable
    TCPTransport(const TCPTransport&) = delete;
    TCPTransport& operator=(const TCPTransport&) = delete;

    // Movable
    TCPTransport(TCPTransport&& other) noexcept;
    TCPTransport& operator=(TCPTransport&& other) noexcept;

    /**
     * Connect to the TCP server (Agent).
     * @throws std::runtime_error if connection fails or times out
     */
    void Connect();

    /**
     * Set the connection timeout (separate from request timeout).
     * @param timeout_ms Connection timeout in milliseconds (default: uses request timeout)
     */
    void SetConnectTimeout(int timeout_ms);

    /**
     * AUD-072: register a callback invoked when the read loop exits on its
     * own (peer clean EOF or network error) — i.e. the connection died
     * WITHOUT a local Close(). The callback runs on the read thread, right
     * after pending requests have been failed and the connection state has
     * been cleared; it must stay non-blocking (no dial, no join, no handler
     * work) so the read thread is never held. Not invoked for an explicit
     * Close()/destructor, and not re-invoked for stale tails of an
     * already-replaced connection.
     */
    void SetDisconnectCallback(std::function<void()> callback);

    /**
     * Close the connection.
     */
    void Close();

    /**
     * Check if connected.
     */
    bool IsConnected() const;

    // ---- Inbound (Agent -> Provider calls) ----
    // Read loop only dispatches; handlers run on a bounded worker pool
    // (default = hardware concurrency, queue = workers * 4, overflow
    // fast-fails with an empty response so Agent failover takes over).
    using InboundHandler = std::function<std::vector<uint8_t>(uint32_t msg_id, uint32_t req_id, const std::vector<uint8_t>& body)>;

    void SetInboundHandler(InboundHandler handler);
    void DispatchInbound(uint32_t msg_id, uint32_t req_id, std::vector<uint8_t> body);
    static void WriteResponseOnSocket(socket_t sock, uint32_t resp_msg_id, uint32_t req_id,
                                       const std::vector<uint8_t>& body);
    // AUD-013：带连接级写锁版本——write-all 完整写帧并在锁内串行化同一
    // socket 的全部写（4 参委托版无锁，仅供单写者/测试路径）。Call 请求
    // 帧经 WriteAll(write_mu_) 与响应帧共用同一把锁，避免并发写交错。
    static void WriteResponseOnSocket(socket_t sock, uint32_t resp_msg_id, uint32_t req_id,
                                       const std::vector<uint8_t>& body,
                                       const std::shared_ptr<std::mutex>& write_mu);
    // AUD-013：write-all 写帧——循环 send 至写完（单次 send 只判负会静默
    // 丢短写尾字节）；EINTR 立即重试、EAGAIN/EWOULDBLOCK 短暂休眠重试
    // （阻塞 socket 不触发，防非阻塞边角自旋）；真错误记录并返回 false。
    // write_mu 非空时在锁内完成整帧写（连接级串行化）。
    static bool WriteAll(socket_t sock, const char* data, size_t len,
                         const std::shared_ptr<std::mutex>& write_mu,
                         const char* what);
    static int InboundWorkerCount();
    // 覆盖默认入站 worker 数（单线程游戏服设 1 = 串行模式：handler
    // 顺序执行互不并发）。<=0 恢复默认（硬件并发，最少 2）。
    static void SetInboundWorkerCount(int workers);

    /**
     * Send a request and wait for response.
     *
     * @param msg_type Protocol message type (e.g., MSG_INVOKE_REQUEST)
     * @param data Protobuf serialized request body
     * @return Pair of (response_msg_type, response_data)
     * @throws std::runtime_error if not connected or request fails
     */
    std::pair<uint32_t, std::vector<uint8_t>> Call(uint32_t msg_type,
                                                    const std::vector<uint8_t>& data);

private:
    struct ResponseLatch {
        std::mutex mutex;
        std::condition_variable cv;
        std::vector<uint8_t> body;
        uint32_t msg_id = 0;
        bool ready = false;
        // AUD-072: set when the read loop died (peer EOF / network error)
        // before a response arrived. Distinguishes "connection lost" from a
        // legitimate empty response, so Call() can fail fast instead of
        // returning a fake empty success or burning the full timeout.
        bool failed = false;

        bool Wait(int timeout_ms) {
            std::unique_lock<std::mutex> lock(mutex);
            return cv.wait_for(lock, std::chrono::milliseconds(timeout_ms),
                              [this] { return ready; });
        }

        void Signal(std::vector<uint8_t> b, uint32_t mid) {
            std::lock_guard<std::mutex> lock(mutex);
            body = std::move(b);
            msg_id = mid;
            ready = true;
            cv.notify_one();
        }

        // AUD-072: wake the waiter with the failure marker (immediate, no
        // polling). Monotone: a later Signal() cannot un-fail the latch.
        void Fail() {
            std::lock_guard<std::mutex> lock(mutex);
            failed = true;
            ready = true;
            cv.notify_all();
        }

        bool IsFailed() {
            std::lock_guard<std::mutex> lock(mutex);
            return failed;
        }
    };

    // AUD-072: read loop owns one connection instance; epoch identifies the
    // connection incarnation (bumped by every Connect()). A late-exiting
    // tail of an old connection must never publish against the new one.
    void ReadLoop(uint64_t epoch);
    // AUD-072: publish a self-detected disconnect (peer EOF / network error):
    // fail every pending request on THIS connection, clear the connection
    // state, detach the (self-owned) read thread instead of joining it —
    // a thread must not join itself — then fire the disconnect callback.
    // Must be called with the connection epoch the loop was started for.
    void PublishDisconnect(uint64_t epoch);
    int ReadFully(void* buf, size_t count);
    static void PutMsgId(uint8_t* buf, uint32_t msg_id);
    static uint32_t GetMsgId(const uint8_t* buf);
    bool ConnectWithTimeout(int timeout_ms);
    void SetSocketNonBlocking(bool non_blocking);

    // 入站 worker 池共享状态：worker 持 shared_ptr 而非 this——
    // Close/析构后 worker 仍可安全退出（socket fd 为值拷贝，关闭后
    // send 仅返回 EBADF），修复「worker for(;;) 永不退出 + 析构成员
    // 被等待中的线程使用」导致的挂起/use-after-free。
    struct InboundPool {
        std::mutex mu;
        std::condition_variable cv;
        std::queue<std::tuple<uint32_t, uint32_t, std::vector<uint8_t>>> queue;
        std::atomic<int> queued{0};
        bool started = false;
        bool stopping = false;
        std::vector<std::thread> threads;
        InboundHandler handler;
        socket_t sock = INVALID_SOCKET_VALUE;
        // AUD-013：连接级写锁（与 TCPTransport::write_mu_ 同一对象）。worker
        // 只持 InboundPool（shared_ptr，不捕获 this），经此触达同一锁，与
        // Call 的写帧串行化——并发写同一 socket 无锁会交错字节流。
        std::shared_ptr<std::mutex> write_mu;
    };

    std::string host_;
    int port_;
    int timeout_ms_;
    int connect_timeout_ms_;  // Separate timeout for connection attempts
    socket_t socket_;
    // AUD-013：连接级写锁——Call 请求帧与入站响应帧（pool worker）共用
    // 同一把锁，串行化对同一 socket 的全部写。shared_ptr：worker 只持
    // InboundPool（不捕获 this），经 pool->write_mu 触达同一锁。构造时
    // 建立，move 时随池一并转移。
    std::shared_ptr<std::mutex> write_mu_;
    std::atomic<bool> connected_;
    std::atomic<bool> closing_;
    bool close_called_ = false;  // Close() 幂等：显式 Close + 析构 Close 双跑防护
    std::atomic<uint32_t> next_req_id_;
    // AUD-072：连接纪元。Connect() 每次建立新连接前 +1；读循环携带自己
    // 的纪元退出，PublishDisconnect 仅在纪元仍匹配时发布——旧连接的
    // 尾巴（迟到的读循环退出）绝不污染新连接的 pending 队列与状态。
    std::atomic<uint64_t> conn_epoch_{0};
    // AUD-072：发布断开的互斥锁——epoch 校验 + 状态翻转 + pending 搬空
    // + 线程自 detach 必须原子；Connect() 重置状态也在锁内，两边互斥。
    std::mutex disconnect_mu_;
    // AUD-072：断开回调（读线程退出时触发，见 SetDisconnectCallback）。
    std::function<void()> disconnect_cb_;
    // shared_ptr 而非 unique_ptr：Call() 在锁外等待期间持有 latch 引用，
    // Close() move 走整个 map 并 signal 后立即析构局部副本时，等待方
    // 的引用仍保持 latch 存活——否则等待方会在已释放的 mutex/cv 上
    // Wait()（use-after-free：表现为无限 futex 等待或堆元数据损坏）。
    std::unordered_map<uint32_t, std::shared_ptr<ResponseLatch>> pending_responses_;
    std::mutex pending_mutex_;
    std::thread read_thread_;
    InboundHandler inbound_handler_;
    std::shared_ptr<InboundPool> inbound_pool_;
    // 控制车道（心跳/注册/drain）：单 worker 独立队列，永不 fail-fast。
    // 业务队列打满时控制帧照常处理——否则 Agent 误判会话死亡，过载
    // 升级为连接雪崩（双车道，与 Go/JS/Java/Python 同规则）。
    std::shared_ptr<InboundPool> control_pool_;

    // 双车道公共件：worker 启动与入队（业务/控制车道共用同一池结构）。
    static void StartPoolWorkers(const std::shared_ptr<InboundPool>& pool, int workers);
    static void EnqueueOnPool(const std::shared_ptr<InboundPool>& pool, uint32_t msg_id,
                              uint32_t req_id, std::vector<uint8_t>&& body);
    // 停机：置 stopping + 置空 handler/sock + detach 全部 worker（Go
    // MuxConn 同款语义：不等待业务排空）。业务/控制车道共用。
    static void DetachPool(std::shared_ptr<InboundPool>& pool);

    static constexpr size_t FRAME_HEADER_BYTES = 4;
    static constexpr size_t PROTOCOL_HEADER_SIZE = 8;
    static constexpr size_t MAX_FRAME_BYTES = 32 * 1024 * 1024; // 32 MB
    static constexpr uint8_t VERSION_1 = 0x01;

#ifdef _WIN32
    static bool ws_initialized_;
    static std::mutex ws_init_mutex_;
#endif
};

/**
 * TCP server for handling incoming Croupier protocol connections.
 * Used for local function registration and RPC handling.
 */
class TCPServer {
public:
    /**
     * Initialize TCP server.
     *
     * @param listen_address Listen address (e.g., "127.0.0.1:0" or "tcp://127.0.0.1:0" for auto-port)
     * @param timeout_ms Request timeout in milliseconds
     */
    explicit TCPServer(const std::string& listen_address, int timeout_ms = 30000);

    ~TCPServer();

    // Non-copyable
    TCPServer(const TCPServer&) = delete;
    TCPServer& operator=(const TCPServer&) = delete;

    /**
     * Set the message handler for processing incoming requests.
     */
    void SetHandler(MessageHandler handler);

    /**
     * Start the server (begins listening and accepting connections).
     */
    void Start();

    /**
     * Stop the server and close all connections.
     */
    void Stop();

    /**
     * Check if the server is running.
     */
    bool IsRunning() const;

    /**
     * Get the actual listen address (after binding).
     */
    std::string GetListenAddress() const;


private:
    struct ClientConnection {
        socket_t socket;
        std::thread read_thread;
        std::atomic<bool> active;
    };

    void ServerLoop();
    void HandleClient(socket_t client_socket);
    int ReadFully(socket_t sock, void* buf, size_t count);
    bool SendMessage(socket_t sock, uint32_t msg_type, uint32_t req_id, const std::vector<uint8_t>& body);

    std::string listen_address_;
    socket_t server_socket_;
    std::atomic<bool> running_;
    std::atomic<bool> accept_thread_running_;
    std::thread accept_thread_;
    MessageHandler handler_;
    std::mutex handler_mutex_;
    std::vector<std::unique_ptr<ClientConnection>> clients_;
    std::mutex clients_mutex_;
    std::string actual_listen_address_;

    static constexpr size_t MAX_FRAME_BYTES = 32 * 1024 * 1024; // 32 MB
    static constexpr uint8_t VERSION_1 = 0x01;

#ifdef _WIN32
    static bool ws_initialized_;
    static std::mutex ws_init_mutex_;
#endif
};

} // namespace sdk
} // namespace croupier

#endif // CROUPIER_SDK_TCP_TRANSPORT_H
