// Exercises the production libcurl-backed HTTP transport against a real local
// HTTP server implemented with raw sockets.
#include <gtest/gtest.h>
#include "croupier/sdk/http_transport.h"

#include <arpa/inet.h>
#include <netinet/in.h>
#include <sys/socket.h>
#include <unistd.h>

#include <atomic>
#include <cstring>
#include <mutex>
#include <string>
#include <thread>
#include <vector>

namespace croupier::sdk::test {
namespace {

struct RecordedRequest {
    std::string method;
    std::string path;
    std::string body;
    std::vector<std::string> header_lines;
};

// Minimal HTTP/1.1 server: handles one request per connection in a loop.
class LocalHTTPServer {
public:
    explicit LocalHTTPServer(std::function<std::string(const RecordedRequest&)> responder)
        : responder_(std::move(responder)) {
        listen_fd_ = socket(AF_INET, SOCK_STREAM, 0);
        if (listen_fd_ < 0) throw std::runtime_error("socket() failed");
        int reuse = 1;
        setsockopt(listen_fd_, SOL_SOCKET, SO_REUSEADDR, &reuse, sizeof(reuse));
        sockaddr_in addr{};
        addr.sin_family = AF_INET;
        addr.sin_addr.s_addr = htonl(INADDR_LOOPBACK);
        addr.sin_port = 0;  // auto-assign
        if (bind(listen_fd_, reinterpret_cast<sockaddr*>(&addr), sizeof(addr)) != 0) {
            throw std::runtime_error("bind() failed");
        }
        if (listen(listen_fd_, 4) != 0) {
            throw std::runtime_error("listen() failed");
        }

        socklen_t len = sizeof(addr);
        if (getsockname(listen_fd_, reinterpret_cast<sockaddr*>(&addr), &len) != 0) {
            throw std::runtime_error("getsockname() failed");
        }
        port_ = ntohs(addr.sin_port);

        accept_thread_ = std::thread([this] { AcceptLoop(); });
    }

    ~LocalHTTPServer() {
        stop_ = true;
        shutdown(listen_fd_, SHUT_RDWR);
        close(listen_fd_);
        if (accept_thread_.joinable()) accept_thread_.join();
        for (int fd : client_fds_) close(fd);
    }

    int port() const { return port_; }

    RecordedRequest last_request() const {
        std::lock_guard<std::mutex> lock(mutex_);
        return last_request_;
    }

private:
    void AcceptLoop() {
        while (!stop_) {
            int client = accept(listen_fd_, nullptr, nullptr);
            if (client < 0) return;
            {
                std::lock_guard<std::mutex> lock(mutex_);
                client_fds_.push_back(client);
            }
            HandleClient(client);
        }
    }

    void HandleClient(int fd) {
        std::string raw;
        char buf[4096];
        while (raw.find("\r\n\r\n") == std::string::npos) {
            const ssize_t n = recv(fd, buf, sizeof(buf), 0);
            if (n <= 0) {
                close(fd);
                return;
            }
            raw.append(buf, static_cast<size_t>(n));
        }
        const size_t header_end = raw.find("\r\n\r\n");
        const std::string head = raw.substr(0, header_end);
        std::string body = raw.substr(header_end + 4);

        RecordedRequest request;
        const size_t first_space = head.find(' ');
        const size_t second_space = head.find(' ', first_space + 1);
        request.method = head.substr(0, first_space);
        request.path = head.substr(first_space + 1, second_space - first_space - 1);
        size_t line_start = head.find("\r\n");
        while (line_start != std::string::npos) {
            const size_t line_end = head.find("\r\n", line_start + 2);
            const std::string line =
                head.substr(line_start + 2, line_end == std::string::npos ? std::string::npos : line_end - line_start - 2);
            if (!line.empty()) request.header_lines.push_back(line);
            if (line_end == std::string::npos) break;
            line_start = line_end;
        }

        // Read Content-Length bytes if declared.
        size_t content_length = 0;
        for (const auto& line : request.header_lines) {
            if (line.rfind("Content-Length:", 0) == 0) {
                content_length = static_cast<size_t>(std::strtoull(line.c_str() + 15, nullptr, 10));
            }
        }
        while (body.size() < content_length) {
            const ssize_t n = recv(fd, buf, sizeof(buf), 0);
            if (n <= 0) break;
            body.append(buf, static_cast<size_t>(n));
        }
        request.body = body;

        {
            std::lock_guard<std::mutex> lock(mutex_);
            last_request_ = request;
        }

        const std::string response = responder_(request);
        send(fd, response.c_str(), response.size(), 0);
        close(fd);
    }

    int listen_fd_ = -1;
    int port_ = 0;
    std::atomic<bool> stop_{false};
    std::thread accept_thread_;
    std::function<std::string(const RecordedRequest&)> responder_;
    mutable std::mutex mutex_;
    RecordedRequest last_request_;
    std::vector<int> client_fds_;
};

std::string OkResponse(const std::string& body) {
    return "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: " +
           std::to_string(body.size()) + "\r\nConnection: close\r\n\r\n" + body;
}

TEST(DefaultHTTPTransportTest, GetReturnsStatusAndBody) {
    LocalHTTPServer server([](const RecordedRequest&) { return OkResponse(R"({"status":"ok"})"); });
    auto transport = NewDefaultHTTPTransport();
    ASSERT_TRUE(transport != nullptr);

    HTTPRequest request;
    request.method = "GET";
    request.url = "http://127.0.0.1:" + std::to_string(server.port()) + "/api/v1/health";
    request.timeout_ms = 5000;

    const HTTPResponse response = transport->Send(request);
    EXPECT_EQ(200, response.status_code);
    EXPECT_EQ(R"({"status":"ok"})", response.body);
    EXPECT_EQ("GET", server.last_request().method);
    EXPECT_EQ("/api/v1/health", server.last_request().path);
}

TEST(DefaultHTTPTransportTest, PostSendsHeadersAndBody) {
    LocalHTTPServer server([](const RecordedRequest&) { return OkResponse(R"({"accepted":true})"); });
    auto transport = NewDefaultHTTPTransport();

    HTTPRequest request;
    request.method = "POST";
    request.url = "http://127.0.0.1:" + std::to_string(server.port()) + "/api/v1/tasks";
    request.headers = {{"Content-Type", "application/json"}, {"Authorization", "Bearer tok"}};
    request.body = R"({"functionId":"f"})";
    request.timeout_ms = 5000;

    const HTTPResponse response = transport->Send(request);
    EXPECT_EQ(200, response.status_code);
    EXPECT_EQ(R"({"accepted":true})", response.body);

    const RecordedRequest recorded = server.last_request();
    EXPECT_EQ("POST", recorded.method);
    EXPECT_EQ("/api/v1/tasks", recorded.path);
    EXPECT_EQ(R"({"functionId":"f"})", recorded.body);
    bool saw_auth = false;
    bool saw_content_type = false;
    for (const auto& line : recorded.header_lines) {
        if (line == "Authorization: Bearer tok") saw_auth = true;
        if (line == "Content-Type: application/json") saw_content_type = true;
    }
    EXPECT_TRUE(saw_auth);
    EXPECT_TRUE(saw_content_type);
}

TEST(DefaultHTTPTransportTest, InsecureFlagStillCompletesPlainHTTPRequest) {
    LocalHTTPServer server([](const RecordedRequest&) { return OkResponse("{}"); });
    auto transport = NewDefaultHTTPTransport();

    HTTPRequest request;
    request.method = "GET";
    request.url = "http://127.0.0.1:" + std::to_string(server.port()) + "/";
    request.timeout_ms = 5000;
    request.insecure = true;  // must not break plain HTTP requests

    const HTTPResponse response = transport->Send(request);
    EXPECT_EQ(200, response.status_code);
    EXPECT_EQ("{}", response.body);
}

TEST(DefaultHTTPTransportTest, ConnectionFailureThrowsRuntimeError) {
    // Bind then immediately release a port so nothing is listening on it.
    int fd = socket(AF_INET, SOCK_STREAM, 0);
    ASSERT_GE(fd, 0);
    sockaddr_in addr{};
    addr.sin_family = AF_INET;
    addr.sin_addr.s_addr = htonl(INADDR_LOOPBACK);
    addr.sin_port = 0;
    ASSERT_EQ(0, bind(fd, reinterpret_cast<sockaddr*>(&addr), sizeof(addr)));
    socklen_t len = sizeof(addr);
    ASSERT_EQ(0, getsockname(fd, reinterpret_cast<sockaddr*>(&addr), &len));
    const int dead_port = ntohs(addr.sin_port);
    close(fd);

    auto transport = NewDefaultHTTPTransport();
    HTTPRequest request;
    request.method = "GET";
    request.url = "http://127.0.0.1:" + std::to_string(dead_port) + "/";
    request.timeout_ms = 2000;
    EXPECT_THROW(transport->Send(request), std::runtime_error);
}

TEST(DefaultHTTPTransportTest, ServerErrorStatusIsDeliveredToCaller) {
    LocalHTTPServer server([](const RecordedRequest&) {
        return std::string("HTTP/1.1 404 Not Found\r\nContent-Length: 9\r\nConnection: close\r\n\r\nnot found");
    });
    auto transport = NewDefaultHTTPTransport();

    HTTPRequest request;
    request.method = "GET";
    request.url = "http://127.0.0.1:" + std::to_string(server.port()) + "/missing";
    request.timeout_ms = 5000;

    const HTTPResponse response = transport->Send(request);
    EXPECT_EQ(404, response.status_code);
    EXPECT_EQ("not found", response.body);
}

}  // namespace
}  // namespace croupier::sdk::test
