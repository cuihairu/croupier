// Coverage boost tests: TCPServer lifecycle + TCPTransport round-trips and
// concurrency, HTTP transport method/header/query plumbing, logger level
// mapping, idempotency key format and JSON schema validation.
// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

#include <gtest/gtest.h>

#include "croupier/sdk/croupier_client.h"
#include "croupier/sdk/http_transport.h"
#include "croupier/sdk/logger.h"
#include "croupier/sdk/tcp_transport.h"
#include "croupier/sdk/utils/json_utils.h"

#include <arpa/inet.h>
#include <netinet/in.h>
#include <sys/socket.h>
#include <unistd.h>

#include <algorithm>
#include <atomic>
#include <chrono>
#include <cstdlib>
#include <map>
#include <mutex>
#include <string>
#include <thread>
#include <vector>

namespace croupier::sdk::test {
namespace {

TCPTransport MakeTransport(const std::string& address, int timeout_ms) {
    const size_t colon = address.rfind(':');
    return TCPTransport(address.substr(0, colon),
                        std::stoi(address.substr(colon + 1)), timeout_ms);
}

void WaitUntilRunning(TCPServer& server) {
    auto deadline = std::chrono::steady_clock::now() + std::chrono::seconds(5);
    while (!server.IsRunning() && std::chrono::steady_clock::now() < deadline) {
        std::this_thread::sleep_for(std::chrono::milliseconds(5));
    }
}

}  // namespace

// ---------------------------------------------------------------------------
// TCPServer lifecycle
// ---------------------------------------------------------------------------

TEST(TCPServerBoostTest, AutoPortListenAddressIsResolved) {
    TCPServer server("tcp://127.0.0.1:0");
    server.SetHandler([](uint32_t, uint32_t, const std::vector<uint8_t>&) {
        return std::vector<uint8_t>{};
    });
    server.Start();
    WaitUntilRunning(server);

    const std::string actual = server.GetListenAddress();
    ASSERT_FALSE(actual.empty());
    EXPECT_NE(std::string::npos, actual.find(':'));
    // Port must not remain the literal "0".
    const auto colon = actual.rfind(':');
    EXPECT_NE("0", actual.substr(colon + 1));

    EXPECT_TRUE(server.IsRunning());
    server.Stop();
    EXPECT_FALSE(server.IsRunning());
}

TEST(TCPServerBoostTest, StopIsIdempotent) {
    TCPServer server("127.0.0.1:0");
    server.SetHandler([](uint32_t, uint32_t, const std::vector<uint8_t>&) {
        return std::vector<uint8_t>{1};
    });
    server.Start();
    WaitUntilRunning(server);

    server.Stop();
    ASSERT_NO_FATAL_FAILURE(server.Stop());
    EXPECT_FALSE(server.IsRunning());
}

TEST(TCPServerBoostTest, StartWithoutHandlerStillAcceptsConnections) {
    TCPServer server("127.0.0.1:0");
    server.Start();
    WaitUntilRunning(server);
    EXPECT_TRUE(server.IsRunning());
    server.Stop();
}

// ---------------------------------------------------------------------------
// TCPTransport round trips
// ---------------------------------------------------------------------------

TEST(TCPTransportBoostTest, CallEchoesRequestBody) {
    TCPServer server("tcp://127.0.0.1:0");
    server.SetHandler([](uint32_t msg_type, uint32_t, const std::vector<uint8_t>& body) {
        EXPECT_EQ(0x030101u, msg_type);
        return body;
    });
    server.Start();
    WaitUntilRunning(server);

    TCPTransport transport = MakeTransport(server.GetListenAddress(), 2000);
    transport.Connect();
    ASSERT_TRUE(transport.IsConnected());

    const std::vector<uint8_t> payload{1, 2, 3, 4, 5};
    auto [resp_type, resp_body] = transport.Call(0x030101, payload);
    EXPECT_EQ(payload, resp_body);

    transport.Close();
    EXPECT_FALSE(transport.IsConnected());
    server.Stop();
}

TEST(TCPTransportBoostTest, CallHandlesLargePayload) {
    TCPServer server("tcp://127.0.0.1:0");
    server.SetHandler([](uint32_t, uint32_t, const std::vector<uint8_t>& body) {
        return body;
    });
    server.Start();
    WaitUntilRunning(server);

    TCPTransport transport = MakeTransport(server.GetListenAddress(), 5000);
    transport.Connect();

    std::vector<uint8_t> large(256 * 1024);
    for (size_t i = 0; i < large.size(); ++i) large[i] = static_cast<uint8_t>(i & 0xFF);
    auto [_, resp_body] = transport.Call(0x030101, large);
    EXPECT_EQ(large.size(), resp_body.size());
    EXPECT_EQ(large, resp_body);

    transport.Close();
    server.Stop();
}

TEST(TCPTransportBoostTest, ConcurrentCallsAreMultiplexed) {
    TCPServer server("tcp://127.0.0.1:0");
    std::atomic<int> handled{0};
    server.SetHandler([&handled](uint32_t, uint32_t, const std::vector<uint8_t>& body) {
        handled++;
        return body;
    });
    server.Start();
    WaitUntilRunning(server);

    TCPTransport transport = MakeTransport(server.GetListenAddress(), 5000);
    transport.Connect();

    std::atomic<int> successes{0};
    std::vector<std::thread> workers;
    for (int w = 0; w < 4; ++w) {
        workers.emplace_back([&transport, &successes, w] {
            for (int i = 0; i < 5; ++i) {
                std::vector<uint8_t> payload(16);
                payload[0] = static_cast<uint8_t>(w);
                payload[1] = static_cast<uint8_t>(i);
                auto [_, body] = transport.Call(0x030101, payload);
                if (body == payload) successes++;
            }
        });
    }
    for (auto& worker : workers) worker.join();

    EXPECT_EQ(20, successes);
    EXPECT_EQ(20, handled.load());

    transport.Close();
    server.Stop();
}

TEST(TCPTransportBoostTest, CallSequentialRequestsReuseConnection) {
    TCPServer server("tcp://127.0.0.1:0");
    server.SetHandler([](uint32_t, uint32_t, const std::vector<uint8_t>& body) {
        return body;
    });
    server.Start();
    WaitUntilRunning(server);

    TCPTransport transport = MakeTransport(server.GetListenAddress(), 2000);
    transport.Connect();

    for (int i = 0; i < 10; ++i) {
        std::vector<uint8_t> payload{static_cast<uint8_t>(i)};
        auto [_, body] = transport.Call(0x030101, payload);
        ASSERT_EQ(payload, body);
        EXPECT_TRUE(transport.IsConnected());
    }

    transport.Close();
    server.Stop();
}

// ---------------------------------------------------------------------------
// Logger level mapping & component logger
// ---------------------------------------------------------------------------

TEST(LoggerBoostTest, SetLevelFromStringMapsAllLevels) {
    auto& logger = Logger::GetInstance();
    const Logger::Level original = Logger::Level::INFO;

    struct Case { std::string name; Logger::Level level; };
    const Case cases[] = {
        {"DEBUG", Logger::Level::DEBUG}, {"debug", Logger::Level::DEBUG},
        {"INFO", Logger::Level::INFO}, {"info", Logger::Level::INFO},
        {"WARN", Logger::Level::WARN}, {"warn", Logger::Level::WARN},
        {"ERROR", Logger::Level::ERR}, {"error", Logger::Level::ERR},
        {"OFF", Logger::Level::OFF}, {"off", Logger::Level::OFF},
        {"bogus", Logger::Level::OFF},  // unchanged mapping default path
    };

    for (const auto& item : cases) {
        logger.SetLevelFromString(item.name);
        EXPECT_TRUE(logger.IsEnabled(item.level))
            << "level string " << item.name << " should enable its level";
    }

    logger.SetLevel(original);
}

TEST(LoggerBoostTest, DisableTogglesLevel) {
    auto& logger = Logger::GetInstance();
    const Logger::Level original = Logger::Level::INFO;

    logger.Disable(true);
    EXPECT_FALSE(logger.IsEnabled(Logger::Level::ERR));
    logger.Disable(false);
    EXPECT_TRUE(logger.IsEnabled(Logger::Level::ERR));

    logger.SetLevel(original);
}

TEST(LoggerBoostTest, ComponentLoggerEmitsAllLevelsSafely) {
    ComponentLogger component("boost-test");
    EXPECT_NO_FATAL_FAILURE({
        component.Debug("debug message");
        component.Info("info message");
        component.Warn("warn message");
        component.Error("error message");
    });
}

// ---------------------------------------------------------------------------
// Idempotency keys & JSON schema validation (utils namespace)
// ---------------------------------------------------------------------------

TEST(UtilsBoostTest, NewIdempotencyKeyLooksLikeHexToken) {
    for (int i = 0; i < 20; ++i) {
        const std::string key = utils::NewIdempotencyKey();
        ASSERT_EQ(32u, key.size()) << key;
        for (const char c : key) {
            EXPECT_TRUE((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f'))
                << "unexpected char " << c << " in " << key;
        }
    }
}

TEST(UtilsBoostTest, NewIdempotencyKeysAreUnique) {
    std::vector<std::string> keys;
    for (int i = 0; i < 200; ++i) keys.push_back(utils::NewIdempotencyKey());
    std::sort(keys.begin(), keys.end());
    EXPECT_EQ(keys.end(), std::adjacent_find(keys.begin(), keys.end()));
}

TEST(UtilsBoostTest, ValidateJSONAcceptsWellFormedSchemaAndPayload) {
    const std::map<std::string, std::string> schema = {
        {"type", "object"}, {"required", R"(["playerId"])"}, {"minProperties", "1"}};
    EXPECT_TRUE(utils::ValidateJSON(R"({"playerId":"42"})", schema));
    EXPECT_FALSE(utils::ValidateJSON(R"({"other":1})", schema));
}

TEST(UtilsBoostTest, ValidateJSONRejectsInvalidPayloadJson) {
    EXPECT_FALSE(utils::ValidateJSON("not-json", {}));
    EXPECT_TRUE(utils::ValidateJSON("[]", {}));
}

TEST(UtilsBoostTest, ParseJSONAndToJSONRoundTripFlatMaps) {
    const std::map<std::string, std::string> flat{{"game", "demo"}, {"env", "dev"}};
    const std::string json = utils::ToJSON(flat);
    const auto parsed = utils::ParseJSON(json);
    EXPECT_EQ(2u, parsed.size());
    EXPECT_EQ("demo", parsed.at("game"));
    EXPECT_EQ("dev", parsed.at("env"));
}

// ---------------------------------------------------------------------------
// JsonUtils accessors
// ---------------------------------------------------------------------------

TEST(JsonUtilsBoostTest, GetIntValueHandlesNestedAndMissingPaths) {
    auto json = utils::JsonUtils::ParseJson(R"({"limits":{"timeout":30,"retries":0}})");
    EXPECT_EQ(30, utils::JsonUtils::GetIntValue(json, "limits.timeout"));
    EXPECT_EQ(0, utils::JsonUtils::GetIntValue(json, "limits.retries"));
    EXPECT_EQ(7, utils::JsonUtils::GetIntValue(json, "limits.missing", 7));
    EXPECT_EQ(7, utils::JsonUtils::GetIntValue(json, "limits.timeout.deep", 7));
}

TEST(JsonUtilsBoostTest, GetBoolValueHandlesNestedAndMissingPaths) {
    auto json = utils::JsonUtils::ParseJson(R"({"flags":{"tls":true,"trace":false}})");
    EXPECT_TRUE(utils::JsonUtils::GetBoolValue(json, "flags.tls"));
    EXPECT_FALSE(utils::JsonUtils::GetBoolValue(json, "flags.trace"));
    EXPECT_TRUE(utils::JsonUtils::GetBoolValue(json, "flags.missing", true));
    EXPECT_FALSE(utils::JsonUtils::GetBoolValue(json, "flags.missing", false));
}

TEST(JsonUtilsBoostTest, MergeJsonOverlaysScalarsAndAddsObjects) {
    auto base = utils::JsonUtils::ParseJson(R"({"a":1,"nested":{"x":1,"y":2}})");
    auto overlay = utils::JsonUtils::ParseJson(R"({"a":2,"nested":{"y":3,"z":4}})");
    auto merged = utils::JsonUtils::MergeJson(base, overlay);
    EXPECT_EQ(2, utils::JsonUtils::GetIntValue(merged, "a"));
    EXPECT_EQ(1, utils::JsonUtils::GetIntValue(merged, "nested.x"));
    EXPECT_EQ(3, utils::JsonUtils::GetIntValue(merged, "nested.y"));
    EXPECT_EQ(4, utils::JsonUtils::GetIntValue(merged, "nested.z"));
}

TEST(JsonUtilsBoostTest, IsValidJsonRejectsMalformedDocuments) {
    EXPECT_FALSE(utils::JsonUtils::IsValidJson("{"));
    EXPECT_FALSE(utils::JsonUtils::IsValidJson("[1,"));
    EXPECT_FALSE(utils::JsonUtils::IsValidJson("{\"a\"}"));
    EXPECT_TRUE(utils::JsonUtils::IsValidJson("{\"a\":[1,2,{\"b\":null}]}"));
}

// ---------------------------------------------------------------------------
// HTTP transport: method / header / query plumbing against a one-shot server
// ---------------------------------------------------------------------------

namespace {

// One-shot HTTP server: accepts a single connection, records the raw request
// head + body, then replies with a fixed JSON response.
class OneShotHTTPServer {
public:
    explicit OneShotHTTPServer(std::string response_body)
        : response_body_(std::move(response_body)) {
        listen_fd_ = ::socket(AF_INET, SOCK_STREAM, 0);
        if (listen_fd_ < 0) throw std::runtime_error("socket() failed");
        int reuse = 1;
        setsockopt(listen_fd_, SOL_SOCKET, SO_REUSEADDR, &reuse, sizeof(reuse));
        sockaddr_in addr{};
        addr.sin_family = AF_INET;
        addr.sin_addr.s_addr = htonl(INADDR_LOOPBACK);
        addr.sin_port = 0;
        if (bind(listen_fd_, reinterpret_cast<sockaddr*>(&addr), sizeof(addr)) != 0) {
            throw std::runtime_error("bind() failed");
        }
        listen(listen_fd_, 1);
        socklen_t len = sizeof(addr);
        getsockname(listen_fd_, reinterpret_cast<sockaddr*>(&addr), &len);
        port_ = ntohs(addr.sin_port);
        thread_ = std::thread([this] { Serve(); });
    }

    ~OneShotHTTPServer() {
        shutdown(listen_fd_, SHUT_RDWR);
        close(listen_fd_);
        if (thread_.joinable()) thread_.join();
    }

    int port() const { return port_; }
    std::string raw_request() const {
        std::lock_guard<std::mutex> lock(mutex_);
        return raw_request_;
    }

private:
    void Serve() {
        const int client = accept(listen_fd_, nullptr, nullptr);
        if (client < 0) return;
        std::string raw;
        char buf[4096];
        while (raw.find("\r\n\r\n") == std::string::npos) {
            const ssize_t n = recv(client, buf, sizeof(buf), 0);
            if (n <= 0) break;
            raw.append(buf, static_cast<size_t>(n));
        }
        size_t content_length = 0;
        const size_t pos = raw.find("Content-Length:");
        if (pos != std::string::npos) {
            content_length = static_cast<size_t>(std::strtoull(raw.c_str() + pos + 15, nullptr, 10));
        }
        const size_t header_end = raw.find("\r\n\r\n");
        if (header_end != std::string::npos) {
            while (raw.size() - header_end - 4 < content_length) {
                const ssize_t n = recv(client, buf, sizeof(buf), 0);
                if (n <= 0) break;
                raw.append(buf, static_cast<size_t>(n));
            }
        }
        {
            std::lock_guard<std::mutex> lock(mutex_);
            raw_request_ = raw;
        }
        const std::string response =
            "HTTP/1.1 200 OK\r\nContent-Length: " + std::to_string(response_body_.size()) +
            "\r\nConnection: close\r\n\r\n" + response_body_;
        send(client, response.c_str(), response.size(), 0);
        close(client);
    }

    int listen_fd_ = -1;
    int port_ = 0;
    std::string response_body_;
    std::string raw_request_;
    mutable std::mutex mutex_;
    std::thread thread_;
};

std::string RequestLineOf(const std::string& raw) {
    return raw.substr(0, raw.find("\r\n"));
}

}  // namespace

TEST(DefaultHTTPTransportBoostTest, PutDeliversMethodBodyAndHeaders) {
    OneShotHTTPServer server(R"({"updated":true})");
    auto transport = NewDefaultHTTPTransport();

    HTTPRequest request;
    request.method = "PUT";
    request.url = "http://127.0.0.1:" + std::to_string(server.port()) + "/api/v1/config";
    request.headers["X-Custom-Header"] = "boost";
    request.body = R"({"value":42})";
    request.timeout_ms = 5000;

    const HTTPResponse response = transport->Send(request);
    EXPECT_EQ(200, response.status_code);
    EXPECT_EQ("PUT /api/v1/config HTTP/1.1", RequestLineOf(server.raw_request()));
    EXPECT_NE(std::string::npos, server.raw_request().find("X-Custom-Header: boost"));
    EXPECT_NE(std::string::npos, server.raw_request().find("\"value\":42"));
}

TEST(DefaultHTTPTransportBoostTest, DeletePreservesQueryParameters) {
    OneShotHTTPServer server("{}");
    auto transport = NewDefaultHTTPTransport();

    HTTPRequest request;
    request.method = "DELETE";
    request.url = "http://127.0.0.1:" + std::to_string(server.port()) + "/api/v1/tasks/task-9?force=true";
    request.timeout_ms = 5000;

    const HTTPResponse response = transport->Send(request);
    EXPECT_EQ(200, response.status_code);
    EXPECT_EQ("DELETE /api/v1/tasks/task-9?force=true HTTP/1.1",
              RequestLineOf(server.raw_request()));
}

TEST(DefaultHTTPTransportBoostTest, GetWithEmptyBodyHasNoContentLength) {
    OneShotHTTPServer server("{}");
    auto transport = NewDefaultHTTPTransport();

    HTTPRequest request;
    request.method = "GET";
    request.url = "http://127.0.0.1:" + std::to_string(server.port()) + "/health";
    request.timeout_ms = 5000;

    const HTTPResponse response = transport->Send(request);
    EXPECT_EQ(200, response.status_code);
    EXPECT_EQ(std::string::npos, server.raw_request().find("Content-Length:"));
}

}  // namespace croupier::sdk::test
