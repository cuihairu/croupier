/**
 * @file tcp_transport.cpp
 * @brief TCP Transport implementation for Croupier C++ SDK.
 */

#include "croupier/sdk/tcp_transport.h"
#include <algorithm>
#include <stdexcept>
#include <cstring>
#include <cstddef>
#include <iostream>

#ifdef _WIN32
#include <winsock2.h>
#include <ws2tcpip.h>
#include <BaseTsd.h>
#ifdef _MSC_VER
#pragma comment(lib, "ws2_32.lib")
#endif
// Windows doesn't have ssize_t, use intptr_t instead
typedef intptr_t ssize_t;
#else
#include <errno.h>
#include <arpa/inet.h>
#include <netinet/in.h>
#include <sys/socket.h>
#include <unistd.h>
#endif

namespace croupier {
namespace sdk {

#ifdef _WIN32
bool TCPTransport::ws_initialized_ = false;
std::mutex TCPTransport::ws_init_mutex_;
#endif

TCPTransport::TCPTransport(const std::string& host, int port, int timeout_ms)
    : host_(host),
      port_(port),
      timeout_ms_(timeout_ms),
      connect_timeout_ms_(timeout_ms),
      socket_(INVALID_SOCKET_VALUE),
      connected_(false),
      closing_(false),
      next_req_id_(1) {

#ifdef _WIN32
    // Initialize Winsock once
    std::lock_guard<std::mutex> lock(ws_init_mutex_);
    if (!ws_initialized_) {
        WSADATA wsa_data;
        if (WSAStartup(MAKEWORD(2, 2), &wsa_data) == 0) {
            ws_initialized_ = true;
        }
    }
#endif
}

TCPTransport::~TCPTransport() {
    Close();
}

TCPTransport::TCPTransport(TCPTransport&& other) noexcept
    : host_(std::move(other.host_)),
      port_(other.port_),
      timeout_ms_(other.timeout_ms_),
      connect_timeout_ms_(other.connect_timeout_ms_),
      socket_(other.socket_),
      connected_(other.connected_.load()),
      closing_(other.closing_.load()),
      next_req_id_(other.next_req_id_.load()),
      pending_responses_(std::move(other.pending_responses_)),
      read_thread_(std::move(other.read_thread_)),
      inbound_pool_(std::move(other.inbound_pool_)) {
    // 接管方的 close_called_ 必须复位：否则其析构被幂等守卫跳过，
    // 被 move 进来的 read_thread_/pool 永不回收 → terminate。
    close_called_ = other.close_called_;
    other.socket_ = INVALID_SOCKET_VALUE;
    other.connected_ = false;
    other.close_called_ = true;  // 源对象已掏空，析构 Close 直通 no-op
}

TCPTransport& TCPTransport::operator=(TCPTransport&& other) noexcept {
    if (this != &other) {
        Close();

        host_ = std::move(other.host_);
        port_ = other.port_;
        timeout_ms_ = other.timeout_ms_;
        connect_timeout_ms_ = other.connect_timeout_ms_;
        socket_ = other.socket_;
        connected_ = other.connected_.load();
        closing_ = other.closing_.load();
        next_req_id_ = other.next_req_id_.load();
        pending_responses_ = std::move(other.pending_responses_);
        read_thread_ = std::move(other.read_thread_);
        inbound_pool_ = std::move(other.inbound_pool_);

        // 目标接管存活连接：close_called_ 复位，析构可正常回收线程；
        // 源已掏空，置 true 让其析构 Close 直通 no-op。
        close_called_ = other.close_called_;
        other.socket_ = INVALID_SOCKET_VALUE;
        other.connected_ = false;
        other.close_called_ = true;
    }
    return *this;
}

void TCPTransport::SetConnectTimeout(int timeout_ms) {
    connect_timeout_ms_ = timeout_ms;
}

void TCPTransport::SetSocketNonBlocking(bool non_blocking) {
#ifdef _WIN32
    u_long mode = non_blocking ? 1 : 0;
    ioctlsocket(socket_, FIONBIO, &mode);
#else
    int flags = fcntl(socket_, F_GETFL, 0);
    if (flags == -1) {
        throw std::runtime_error("Failed to get socket flags");
    }
    if (non_blocking) {
        fcntl(socket_, F_SETFL, flags | O_NONBLOCK);
    } else {
        fcntl(socket_, F_SETFL, flags & ~O_NONBLOCK);
    }
#endif
}

bool TCPTransport::ConnectWithTimeout(int timeout_ms) {
    // Create socket
    socket_ = socket(AF_INET, SOCK_STREAM, IPPROTO_TCP);
    if (socket_ == INVALID_SOCKET_VALUE) {
        throw std::runtime_error("Failed to create socket");
    }

    // Set receive timeout
#ifdef _WIN32
    DWORD timeout = timeout_ms_;
    setsockopt(socket_, SOL_SOCKET, SO_RCVTIMEO,
               reinterpret_cast<const char*>(&timeout), sizeof(timeout));
#else
    struct timeval tv;
    tv.tv_sec = timeout_ms_ / 1000;
    tv.tv_usec = (timeout_ms_ % 1000) * 1000;
    setsockopt(socket_, SOL_SOCKET, SO_RCVTIMEO, &tv, sizeof(tv));
#endif

    // Parse address
    sockaddr_in addr{};
    addr.sin_family = AF_INET;
    addr.sin_port = htons(port_);

    if (inet_pton(AF_INET, host_.c_str(), &addr.sin_addr) <= 0) {
        // Resolve hostname
        addrinfo* result = nullptr;
        addrinfo hints{};
        hints.ai_family = AF_INET;
        hints.ai_socktype = SOCK_STREAM;

        int ret = getaddrinfo(host_.c_str(), nullptr, &hints, &result);
        if (ret != 0) {
            closesocket(socket_);
            socket_ = INVALID_SOCKET_VALUE;
            throw std::runtime_error("Failed to resolve host: " + host_);
        }

        std::memcpy(&addr.sin_addr, &reinterpret_cast<sockaddr_in*>(result->ai_addr)->sin_addr,
                   sizeof(addr.sin_addr));
        freeaddrinfo(result);
    }

    // Set socket to non-blocking for connect with timeout
    SetSocketNonBlocking(true);

    // Initiate connection (will return immediately with EINPROGRESS on Unix or WSAEWOULDBLOCK on Windows)
    int connect_result = connect(socket_, reinterpret_cast<sockaddr*>(&addr), sizeof(addr));

#ifdef _WIN32
    int error = WSAGetLastError();
    bool in_progress = (connect_result == SOCKET_ERROR_VALUE && error == WSAEWOULDBLOCK);
#else
    bool in_progress = (connect_result == SOCKET_ERROR_VALUE && errno == EINPROGRESS);
#endif

    if (!in_progress) {
        if (connect_result == 0) {
            // Connection completed immediately (rare, e.g., localhost)
            SetSocketNonBlocking(false);
            return true;
        } else {
            // Connection failed immediately
            closesocket(socket_);
            socket_ = INVALID_SOCKET_VALUE;
            throw std::runtime_error("Failed to connect to " + host_ + ":" + std::to_string(port_));
        }
    }

    // Wait for connection to complete using select
    fd_set write_fds;
    fd_set except_fds;
    FD_ZERO(&write_fds);
    FD_ZERO(&except_fds);
    FD_SET(socket_, &write_fds);
    FD_SET(socket_, &except_fds);

    struct timeval select_timeout;
    select_timeout.tv_sec = timeout_ms / 1000;
    select_timeout.tv_usec = (timeout_ms % 1000) * 1000;

    int select_result = select(static_cast<int>(socket_) + 1, nullptr, &write_fds, &except_fds, &select_timeout);

    if (select_result == SOCKET_ERROR_VALUE) {
        closesocket(socket_);
        socket_ = INVALID_SOCKET_VALUE;
        throw std::runtime_error("select() failed during connection to " + host_ + ":" + std::to_string(port_));
    }

    if (select_result == 0) {
        // Timeout
        closesocket(socket_);
        socket_ = INVALID_SOCKET_VALUE;
        throw std::runtime_error("Connection timeout to " + host_ + ":" + std::to_string(port_));
    }

    // Check if socket is writable (connection succeeded) or in except set (connection failed)
    if (FD_ISSET(socket_, &except_fds)) {
        closesocket(socket_);
        socket_ = INVALID_SOCKET_VALUE;
        throw std::runtime_error("Connection failed to " + host_ + ":" + std::to_string(port_));
    }

    if (!FD_ISSET(socket_, &write_fds)) {
        closesocket(socket_);
        socket_ = INVALID_SOCKET_VALUE;
        throw std::runtime_error("Unexpected select result for " + host_ + ":" + std::to_string(port_));
    }

    // Verify connection success using getsockopt
#ifdef _WIN32
    int socket_error = 0;
    int error_len = sizeof(socket_error);
    getsockopt(socket_, SOL_SOCKET, SO_ERROR, reinterpret_cast<char*>(&socket_error), &error_len);
#else
    int socket_error = 0;
    socklen_t error_len = sizeof(socket_error);
    getsockopt(socket_, SOL_SOCKET, SO_ERROR, &socket_error, &error_len);
#endif

    if (socket_error != 0) {
        closesocket(socket_);
        socket_ = INVALID_SOCKET_VALUE;
        throw std::runtime_error("Connection error to " + host_ + ":" + std::to_string(port_));
    }

    // Restore blocking mode
    SetSocketNonBlocking(false);
    return true;
}

void TCPTransport::Connect() {
    if (connected_) {
        return;
    }

    // Use connect with timeout
    ConnectWithTimeout(connect_timeout_ms_);

    // Set receive timeout on the connected socket to prevent blocking in ReadLoop.
    // Use a short timeout (1s) so the read loop can check the closing_ flag frequently.
    // This prevents heartbeat timeouts from killing the connection.
#ifdef _WIN32
    DWORD recv_timeout = 1000; // 1 second
    setsockopt(socket_, SOL_SOCKET, SO_RCVTIMEO,
               reinterpret_cast<const char*>(&recv_timeout), sizeof(recv_timeout));
#else
    struct timeval tv;
    tv.tv_sec = 1;  // 1 second
    tv.tv_usec = 0;
    setsockopt(socket_, SOL_SOCKET, SO_RCVTIMEO, &tv, sizeof(tv));
#endif

    connected_ = true;
    closing_ = false;

    // Start read loop
    read_thread_ = std::thread(&TCPTransport::ReadLoop, this);
}

void TCPTransport::Close() {
    // 幂等：显式 Close 与析构 Close 双跑防护（曾出现 socket/线程被
    // 二次关闭与 join 的未定义行为）。
    if (close_called_) {
        return;
    }
    close_called_ = true;
    closing_ = true;
    connected_ = false;

    // First, signal all pending responses to wake up any waiting threads
    // This must happen BEFORE clearing the map and closing the socket
    std::unordered_map<uint32_t, std::shared_ptr<ResponseLatch>> responses_to_signal;
    {
        std::lock_guard<std::mutex> lock(pending_mutex_);
        responses_to_signal = std::move(pending_responses_);
    }

    // Signal all waiting responses outside the lock to avoid deadlock.
    // shared_ptr：Call() 持有 latch 引用期间（锁外 Wait），此处局部
    // map 析构不会释放仍被等待的 latch（修复 Close/Call 竞争的 UAF）。
    for (auto& [req_id, latch] : responses_to_signal) {
        if (latch) {
            latch->Signal({}, 0);  // Empty response signals closure
        }
    }

    // Now close the socket - this will unblock ReadLoop's recv() call
    // Use shutdown() first to ensure recv() returns immediately even on Linux
    if (socket_ != INVALID_SOCKET_VALUE) {
#ifdef _WIN32
        shutdown(socket_, SD_BOTH);
#else
        shutdown(socket_, SHUT_RDWR);
#endif
        closesocket(socket_);
        socket_ = INVALID_SOCKET_VALUE;
    }

    // Wait for read thread to finish (it should exit when recv() returns due to closed socket)
    if (read_thread_.joinable()) {
        read_thread_.join();
    }

    // Inbound worker 池优雅停机：置 stopping + 唤醒全部 worker；worker
    // 引用 shared_ptr<InboundPool>，detach 后即使仍有 handler 在途，
    // pool 状态与 socket fd（值拷贝，send 仅得 EBADF）都保持有效——
    // 不会因 ~TCPTransport 而悬空。join 会阻塞在用户 handler 上，
    // 这里选择 detach（Go MuxConn 同款语义：不等待业务排空）。
    if (inbound_pool_) {
        {
            std::lock_guard<std::mutex> lock(inbound_pool_->mu);
            inbound_pool_->stopping = true;
            inbound_pool_->sock = INVALID_SOCKET_VALUE;
            inbound_pool_->handler = nullptr;
        }
        inbound_pool_->cv.notify_all();
        for (auto& t : inbound_pool_->threads) {
            if (t.joinable()) {
                t.detach();
            }
        }
        inbound_pool_->threads.clear();
        inbound_pool_.reset();
    }

    // Clear any remaining responses (should be none after signaling)
    std::lock_guard<std::mutex> lock(pending_mutex_);
    pending_responses_.clear();
}

bool TCPTransport::IsConnected() const {
    return connected_ && !closing_;
}

std::pair<uint32_t, std::vector<uint8_t>> TCPTransport::Call(
    uint32_t msg_type, const std::vector<uint8_t>& data) {

    if (!connected_) {
        throw std::runtime_error("Not connected");
    }

    uint32_t req_id = next_req_id_++;
    auto latch = std::make_shared<ResponseLatch>();

    {
        std::lock_guard<std::mutex> lock(pending_mutex_);
        pending_responses_[req_id] = std::move(latch);
    }

    // Create frame: [4-byte length][8-byte protocol header][body]
    size_t frame_size = FRAME_HEADER_BYTES + PROTOCOL_HEADER_SIZE + data.size();
    std::vector<uint8_t> frame(frame_size);

    // Frame length (big-endian)
    uint32_t payload_size = static_cast<uint32_t>(PROTOCOL_HEADER_SIZE + data.size());
    frame[0] = (payload_size >> 24) & 0xFF;
    frame[1] = (payload_size >> 16) & 0xFF;
    frame[2] = (payload_size >> 8) & 0xFF;
    frame[3] = payload_size & 0xFF;

    // Protocol header
    frame[4] = VERSION_1;
    PutMsgId(frame.data() + 5, msg_type);
    frame[8] = (req_id >> 24) & 0xFF;
    frame[9] = (req_id >> 16) & 0xFF;
    frame[10] = (req_id >> 8) & 0xFF;
    frame[11] = req_id & 0xFF;

    // Debug: log sent message header (first 16 bytes)
    std::string hex_send;
    for (size_t i = 0; i < std::min(size_t(16), frame.size()); ++i) {
        char buf[4];
        snprintf(buf, sizeof(buf), "%02X", frame[i]);
        hex_send += buf;
        if (i < 15) hex_send += " ";
    }
    std::cerr << "[DEBUG] TCPTransport: Sending msg_type=" << msg_type
              << " (0x" << std::hex << msg_type << std::dec << ")"
              << ", req_id=" << req_id
              << ", header_hex=" << hex_send << '\n';

    // Request body
    std::memcpy(frame.data() + 12, data.data(), data.size());

    // Send frame
    ssize_t sent = send(socket_, reinterpret_cast<const char*>(frame.data()),
                       frame.size(), 0);
    if (sent != static_cast<ssize_t>(frame.size())) {
        // Clean up pending response
        std::lock_guard<std::mutex> lock(pending_mutex_);
        pending_responses_.erase(req_id);
        throw std::runtime_error("Failed to send complete frame");
    }

    // Hold a shared_ptr copy (keep it in pending_responses_ so Close() can
    // find it): the shared ownership keeps the latch alive even after Close()
    // moves the whole map out and drops its references.
    std::shared_ptr<ResponseLatch> latch_ptr;
    {
        std::lock_guard<std::mutex> lock(pending_mutex_);
        auto it = pending_responses_.find(req_id);
        if (it != pending_responses_.end()) {
            latch_ptr = it->second;
        }
    }

    if (!latch_ptr) {
        throw std::runtime_error("Response latch not found");
    }

    // Wait with periodic closing_ check
    const int check_interval_ms = 100;
    int total_waited = 0;

    while (total_waited < timeout_ms_) {
        if (closing_) {
            // Connection is closing, abort the wait
            {
                std::lock_guard<std::mutex> lock(pending_mutex_);
                pending_responses_.erase(req_id);
            }
            throw std::runtime_error("Connection closing");
        }

        int wait_time = std::min(check_interval_ms, timeout_ms_ - total_waited);
        if (latch_ptr->Wait(wait_time)) {
            // Response received - extract data and remove from pending
            std::vector<uint8_t> body;
            uint32_t msg_id = 0;
            {
                std::lock_guard<std::mutex> lock(pending_mutex_);
                auto it = pending_responses_.find(req_id);
                if (it != pending_responses_.end()) {
                    body = std::move(it->second->body);
                    msg_id = it->second->msg_id;
                    pending_responses_.erase(it);
                }
            }
            return {msg_id, std::move(body)};
        }
        total_waited += wait_time;
    }

    // Timeout - remove from pending
    {
        std::lock_guard<std::mutex> lock(pending_mutex_);
        pending_responses_.erase(req_id);
    }
    throw std::runtime_error("Timeout waiting for response");
}

void TCPTransport::ReadLoop() {
    uint8_t header_buf[FRAME_HEADER_BYTES];

    while (!closing_ && socket_ != INVALID_SOCKET_VALUE) {
        // Read frame header
        int n = ReadFully(header_buf, FRAME_HEADER_BYTES);
        if (n == -2) {
            // Timeout (EAGAIN/EWOULDBLOCK) — check closing flag and retry
            continue;
        }
        if (n < static_cast<int>(FRAME_HEADER_BYTES)) {
            break;
        }

        // Parse frame size
        uint32_t frame_size = (static_cast<uint32_t>(header_buf[0]) << 24) |
                             (static_cast<uint32_t>(header_buf[1]) << 16) |
                             (static_cast<uint32_t>(header_buf[2]) << 8) |
                             static_cast<uint32_t>(header_buf[3]);

        if (frame_size == 0 || frame_size > MAX_FRAME_BYTES) {
            break;
        }

        // Read frame payload
        std::vector<uint8_t> payload(frame_size);
        n = ReadFully(payload.data(), frame_size);
        if (n == -2) {
            // Timeout reading payload — connection is likely broken
            break;
        }
        if (n < static_cast<int>(frame_size)) {
            break;
        }

        // Parse protocol header
        if (payload.size() < PROTOCOL_HEADER_SIZE) {
            continue;
        }

        uint8_t version = payload[0];
        if (version != VERSION_1) {
            continue;
        }

        uint32_t msg_id = GetMsgId(payload.data() + 1);
        uint32_t req_id = (static_cast<uint32_t>(payload[4]) << 24) |
                         (static_cast<uint32_t>(payload[5]) << 16) |
                         (static_cast<uint32_t>(payload[6]) << 8) |
                         static_cast<uint32_t>(payload[7]);

        // Debug: log received header
        std::string hex_recv;
        for (size_t i = 0; i < std::min(size_t(12), payload.size()); ++i) {
            char buf[4];
            snprintf(buf, sizeof(buf), "%02X", payload[i]);
            hex_recv += buf;
            if (i < 11) hex_recv += " ";
        }
        std::cerr << "[DEBUG] TCPTransport: Received msg_id=0x" << std::hex << msg_id << std::dec
                  << " (" << msg_id << "), req_id=" << req_id
                  << ", header_hex=" << hex_recv << '\n';

        size_t body_size = payload.size() - PROTOCOL_HEADER_SIZE;
        std::vector<uint8_t> body(body_size);
        std::memcpy(body.data(), payload.data() + PROTOCOL_HEADER_SIZE, body_size);

        if (protocol::IsRequest(msg_id)) {
            // Agent -> Provider call: dispatch to bounded worker pool.
            // (Read loop never executes handler logic — inline processing
            // would head-of-line block every request on one slow handler.)
            DispatchInbound(msg_id, req_id, std::move(body));
        } else {
            // Route response to pending request
            std::lock_guard<std::mutex> lock(pending_mutex_);
            auto it = pending_responses_.find(req_id);
            if (it != pending_responses_.end()) {
                it->second->Signal(std::move(body), msg_id);
            } else {
                std::cerr << "[DEBUG] TCPTransport: Received response for unknown req_id: " << req_id
                          << ", msg_id: " << msg_id << ", body_size: " << body_size << '\n';
            }
        }
    }

    // ReadLoop exits - do NOT recursively call Close()
    // The Close() method is responsible for cleanup
}

int TCPTransport::ReadFully(void* buf, size_t count) {
    size_t offset = 0;
    char* buffer = static_cast<char*>(buf);

    while (offset < count) {
        ssize_t n = recv(socket_, buffer + offset, count - offset, 0);
        if (n < 0) {
            // Check for timeout (EAGAIN/EWOULDBLOCK)
#ifdef _WIN32
            int err = WSAGetLastError();
            if (err == WSAEWOULDBLOCK || err == WSAETIMEDOUT) {
                return (offset == 0) ? -2 : static_cast<int>(offset);
            }
#else
            if (errno == EAGAIN || errno == EWOULDBLOCK) {
                return (offset == 0) ? -2 : static_cast<int>(offset);
            }
#endif
            return static_cast<int>(offset);
        }
        if (n == 0) {
            // Connection closed
            return static_cast<int>(offset);
        }
        offset += n;
    }
    return static_cast<int>(offset);
}

void TCPTransport::PutMsgId(uint8_t* buf, uint32_t msg_id) {
    buf[0] = (msg_id >> 16) & 0xFF;
    buf[1] = (msg_id >> 8) & 0xFF;
    buf[2] = msg_id & 0xFF;
}

uint32_t TCPTransport::GetMsgId(const uint8_t* buf) {
    return (static_cast<uint32_t>(buf[0]) << 16) |
           (static_cast<uint32_t>(buf[1]) << 8) |
           static_cast<uint32_t>(buf[2]);
}

} // namespace sdk
} // namespace croupier
