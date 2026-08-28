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
     * Close the connection.
     */
    void Close();

    /**
     * Check if connected.
     */
    bool IsConnected() const;

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
    };

    // ---- Inbound (Agent -> Provider calls) ----
    // Read loop only dispatches; handlers run on a bounded worker pool
    // (default = hardware concurrency, queue = workers * 4, overflow
    // fast-fails with an empty response so Agent failover takes over).
    using InboundHandler = std::function<std::vector<uint8_t>(uint32_t msg_id, uint32_t req_id, const std::vector<uint8_t>& body)>;

    void SetInboundHandler(InboundHandler handler);
    void DispatchInbound(uint32_t msg_id, uint32_t req_id, std::vector<uint8_t> body);
    void WriteResponseSilently(uint32_t resp_msg_id, uint32_t req_id, const std::vector<uint8_t>& body);
    static int InboundWorkerCount();

    void ReadLoop();
    int ReadFully(void* buf, size_t count);
    static void PutMsgId(uint8_t* buf, uint32_t msg_id);
    static uint32_t GetMsgId(const uint8_t* buf);
    bool ConnectWithTimeout(int timeout_ms);
    void SetSocketNonBlocking(bool non_blocking);

    std::string host_;
    int port_;
    int timeout_ms_;
    int connect_timeout_ms_;  // Separate timeout for connection attempts
    socket_t socket_;
    std::atomic<bool> connected_;
    std::atomic<bool> closing_;
    std::atomic<uint32_t> next_req_id_;
    std::unordered_map<uint32_t, std::unique_ptr<ResponseLatch>> pending_responses_;
    std::mutex pending_mutex_;
    std::thread read_thread_;
    InboundHandler inbound_handler_;
    std::mutex inbound_pool_mutex_;
    std::vector<std::thread> inbound_workers_;
    std::queue<std::tuple<uint32_t, uint32_t, std::vector<uint8_t>>> inbound_queue_;
    std::condition_variable inbound_cv_;
    std::atomic<int> inbound_queued_{0};
    bool inbound_pool_started_ = false;

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
