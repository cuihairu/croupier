/**
 * @file tcp_transport_server.cpp
 * @brief TCPServer implementation for Croupier C++ SDK.
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
bool TCPServer::ws_initialized_ = false;
std::mutex TCPServer::ws_init_mutex_;
#endif

TCPServer::TCPServer(const std::string& listen_address, int timeout_ms)
    : listen_address_(listen_address),
      server_socket_(INVALID_SOCKET_VALUE),
      running_(false),
      accept_thread_running_(false) {
    (void)timeout_ms;  // Unused parameter reserved for future use

#ifdef _WIN32
    std::lock_guard<std::mutex> lock(ws_init_mutex_);
    if (!ws_initialized_) {
        WSADATA wsa_data;
        if (WSAStartup(MAKEWORD(2, 2), &wsa_data) == 0) {
            ws_initialized_ = true;
        }
    }
#endif
}

TCPServer::~TCPServer() {
    Stop();
}

void TCPServer::SetHandler(MessageHandler handler) {
    std::lock_guard<std::mutex> lock(handler_mutex_);
    handler_ = std::move(handler);
}

void TCPServer::Start() {
    if (running_) {
        return;
    }

    // Parse listen address (format: "tcp://host:port" or "host:port")
    std::string endpoint = listen_address_;
    std::string host = "127.0.0.1";
    int port = 0;

    // Strip "tcp://" prefix if present
    const std::string tcp_prefix = "tcp://";
    if (endpoint.rfind(tcp_prefix, 0) == 0) {
        endpoint = endpoint.substr(tcp_prefix.length());
    }

    // Find the last colon (separating host from port)
    size_t colon_pos = endpoint.rfind(':');
    if (colon_pos != std::string::npos) {
        host = endpoint.substr(0, colon_pos);
        std::string port_str = endpoint.substr(colon_pos + 1);
        try {
            port = std::stoi(port_str);
        } catch (const std::exception&) {
            throw std::runtime_error("Invalid port number in address: " + listen_address_);
        }
    } else {
        host = endpoint;
    }

    // Create socket
    server_socket_ = socket(AF_INET, SOCK_STREAM, IPPROTO_TCP);
    if (server_socket_ == INVALID_SOCKET_VALUE) {
        throw std::runtime_error("Failed to create server socket");
    }

    // Set SO_REUSEADDR
#ifdef _WIN32
    int reuse = 1;
    setsockopt(server_socket_, SOL_SOCKET, SO_REUSEADDR,
               reinterpret_cast<const char*>(&reuse), sizeof(reuse));
#else
    int reuse = 1;
    setsockopt(server_socket_, SOL_SOCKET, SO_REUSEADDR, &reuse, sizeof(reuse));
#endif

    // Bind
    sockaddr_in addr{};
    addr.sin_family = AF_INET;
    addr.sin_port = htons(port);

    if (host == "0.0.0.0" || host == "*") {
        addr.sin_addr.s_addr = INADDR_ANY;
    } else if (inet_pton(AF_INET, host.c_str(), &addr.sin_addr) <= 0) {
        closesocket(server_socket_);
        server_socket_ = INVALID_SOCKET_VALUE;
        throw std::runtime_error("Invalid bind address: " + host);
    }

    if (bind(server_socket_, reinterpret_cast<sockaddr*>(&addr), sizeof(addr)) == SOCKET_ERROR_VALUE) {
        closesocket(server_socket_);
        server_socket_ = INVALID_SOCKET_VALUE;
        throw std::runtime_error("Failed to bind to " + listen_address_);
    }

    // Get actual bound address
    sockaddr_in actual_addr{};
    socklen_t addr_len = sizeof(actual_addr);
    if (getsockname(server_socket_, reinterpret_cast<sockaddr*>(&actual_addr), &addr_len) == 0) {
        char actual_host[INET_ADDRSTRLEN];
        inet_ntop(AF_INET, &actual_addr.sin_addr, actual_host, sizeof(actual_host));
        actual_listen_address_ = std::string(actual_host) + ":" + std::to_string(ntohs(actual_addr.sin_port));
    }

    // Listen
    if (listen(server_socket_, SOMAXCONN) == SOCKET_ERROR_VALUE) {
        closesocket(server_socket_);
        server_socket_ = INVALID_SOCKET_VALUE;
        throw std::runtime_error("Failed to listen on socket");
    }

    running_ = true;
    accept_thread_running_ = true;
    accept_thread_ = std::thread(&TCPServer::ServerLoop, this);
}

void TCPServer::Stop() {
    running_ = false;

    // Close server socket to unblock accept()
    // Use shutdown() first to ensure accept() returns immediately
    if (server_socket_ != INVALID_SOCKET_VALUE) {
#ifdef _WIN32
        shutdown(server_socket_, SD_BOTH);
#else
        shutdown(server_socket_, SHUT_RDWR);
#endif
        closesocket(server_socket_);
        server_socket_ = INVALID_SOCKET_VALUE;
    }

    // Wait for accept thread
    if (accept_thread_.joinable()) {
        accept_thread_.join();
    }

    // Close all client connections - copy clients to avoid deadlock
    std::vector<std::unique_ptr<ClientConnection>> clients_to_stop;
    {
        std::lock_guard<std::mutex> lock(clients_mutex_);
        clients_to_stop = std::move(clients_);
    }

    for (auto& client : clients_to_stop) {
        client->active = false;
        if (client->socket != INVALID_SOCKET_VALUE) {
#ifdef _WIN32
            shutdown(client->socket, SD_BOTH);
#else
            shutdown(client->socket, SHUT_RDWR);
#endif
            closesocket(client->socket);
            client->socket = INVALID_SOCKET_VALUE;
        }
    }

    // Detach threads that might be blocked in send() - they will exit when they detect the socket is closed
    // Use join with timeout for threads that might exit quickly, detach the rest
    for (auto& client : clients_to_stop) {
        if (client->read_thread.joinable()) {
            // Try to join with a short timeout - if the thread is stuck in send(), we detach it
            // Note: C++ doesn't have a timed join, so we detach all for simplicity
            // The thread resources will be cleaned up when the process exits
            client->read_thread.detach();
        }
    }

    // Clear the now-empty vector
    {
        std::lock_guard<std::mutex> lock(clients_mutex_);
        clients_.clear();
    }
}

bool TCPServer::IsRunning() const {
    return running_;
}

std::string TCPServer::GetListenAddress() const {
    return actual_listen_address_;
}

void TCPServer::ServerLoop() {
    while (running_) {
        sockaddr_in client_addr{};
        socklen_t addr_len = sizeof(client_addr);

        socket_t client_socket = accept(server_socket_,
                                        reinterpret_cast<sockaddr*>(&client_addr),
                                        &addr_len);

        if (!running_) {
            if (client_socket != INVALID_SOCKET_VALUE) {
                closesocket(client_socket);
            }
            break;
        }

        if (client_socket == INVALID_SOCKET_VALUE) {
            if (running_) {
                // Error occurred but server is still running
                break;
            }
            continue;
        }

        // Create client connection
        auto client = std::make_unique<ClientConnection>();
        client->socket = client_socket;
        client->active = true;

        // Start handler thread
        client->read_thread = std::thread(&TCPServer::HandleClient, this, client_socket);

        {
            std::lock_guard<std::mutex> lock(clients_mutex_);
            clients_.push_back(std::move(client));
        }
    }

    accept_thread_running_ = false;
}

void TCPServer::HandleClient(socket_t client_socket) {
    uint8_t header_buf[4];

    while (running_) {
        // Read frame header (4 bytes length)
        int n = ReadFully(client_socket, header_buf, 4);
        if (n < 4) {
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
        n = ReadFully(client_socket, payload.data(), frame_size);
        if (n < static_cast<int>(frame_size)) {
            break;
        }

        // Parse protocol header
        if (payload.size() < 8) {
            break;
        }

        uint8_t version = payload[0];
        if (version != VERSION_1) {
            break;
        }

        uint32_t msg_id = (static_cast<uint32_t>(payload[1]) << 16) |
                         (static_cast<uint32_t>(payload[2]) << 8) |
                         static_cast<uint32_t>(payload[3]);

        uint32_t req_id = (static_cast<uint32_t>(payload[4]) << 24) |
                         (static_cast<uint32_t>(payload[5]) << 16) |
                         (static_cast<uint32_t>(payload[6]) << 8) |
                         static_cast<uint32_t>(payload[7]);

        // Extract body
        std::vector<uint8_t> body;
        if (payload.size() > 8) {
            body.assign(payload.begin() + 8, payload.end());
        }

        // Call handler
        std::vector<uint8_t> response_body;
        {
            std::lock_guard<std::mutex> lock(handler_mutex_);
            if (handler_) {
                try {
                    response_body = handler_(msg_id, req_id, body);
                } catch (const std::exception& e) {
                    // Handler failed, send error response if needed
                    response_body.clear();
                }
            }
        }

        // Send response (use response MsgID = request MsgID + 1)
        if (!response_body.empty()) {
            uint32_t response_msg_id = msg_id + 1;
            SendMessage(client_socket, response_msg_id, req_id, response_body);
        }
    }
    // Note: socket cleanup is handled by Stop()
}

int TCPServer::ReadFully(socket_t sock, void* buf, size_t count) {
    size_t offset = 0;
    char* buffer = static_cast<char*>(buf);

    while (offset < count) {
        ssize_t n = recv(sock, buffer + offset, count - offset, 0);
        if (n <= 0) {
            return static_cast<int>(offset);
        }
        offset += n;
    }
    return static_cast<int>(offset);
}

bool TCPServer::SendMessage(socket_t sock, uint32_t msg_type, uint32_t req_id, const std::vector<uint8_t>& body) {
    // Create frame: [4-byte length][8-byte protocol header][body]
    size_t frame_size = 8 + body.size();
    std::vector<uint8_t> frame(4 + frame_size);

    // Frame length (big-endian)
    frame[0] = (frame_size >> 24) & 0xFF;
    frame[1] = (frame_size >> 16) & 0xFF;
    frame[2] = (frame_size >> 8) & 0xFF;
    frame[3] = frame_size & 0xFF;

    // Protocol header
    frame[4] = VERSION_1;
    frame[5] = (msg_type >> 16) & 0xFF;
    frame[6] = (msg_type >> 8) & 0xFF;
    frame[7] = msg_type & 0xFF;
    frame[8] = (req_id >> 24) & 0xFF;
    frame[9] = (req_id >> 16) & 0xFF;
    frame[10] = (req_id >> 8) & 0xFF;
    frame[11] = req_id & 0xFF;

    // Body
    if (!body.empty()) {
        std::memcpy(frame.data() + 12, body.data(), body.size());
    }

    // Send frame
    ssize_t sent = send(sock, reinterpret_cast<const char*>(frame.data()),
                       frame.size(), 0);
    return sent == static_cast<ssize_t>(frame.size());
}

} // namespace sdk
} // namespace croupier
