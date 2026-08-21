// Invoker retry/backoff behavior, close semantics, and URL/error normalization.
#include <gtest/gtest.h>
#include "croupier/sdk/croupier_client.h"

#include <cstdlib>
#include <functional>
#include <memory>
#include <mutex>
#include <stdexcept>
#include <vector>

namespace croupier::sdk::test {
namespace {

class MockHTTPTransport final : public HTTPTransport {
public:
    using Responder = std::function<HTTPResponse(const HTTPRequest&)>;

    explicit MockHTTPTransport(Responder responder) : responder_(std::move(responder)) {}

    HTTPResponse Send(const HTTPRequest& request) override {
        std::lock_guard<std::mutex> lock(mutex_);
        requests.push_back(request);
        return responder_(request);
    }

    std::vector<HTTPRequest> requests;

private:
    std::mutex mutex_;
    Responder responder_;
};

InvokerConfig FastConfig(const std::shared_ptr<HTTPTransport>& transport) {
    InvokerConfig config;
    config.address = "http://server.example";
    config.retry.enabled = false;
    config.retry.initial_delay_ms = 0;
    config.retry.max_delay_ms = 0;
    config.retry.jitter_factor = 0;
    config.http_transport = transport;
    config.disable_logging = true;
    return config;
}

}  // namespace

TEST(InvokerRetryTest, RetriesRetryableStatusThenSucceeds) {
    int calls = 0;
    auto transport = std::make_shared<MockHTTPTransport>([&calls](const HTTPRequest&) {
        ++calls;
        if (calls < 3) return HTTPResponse{500, R"({"message":"boom"})"};
        return HTTPResponse{200, R"({"result":{"ok":true}})"};
    });
    InvokerConfig config = FastConfig(transport);
    config.retry.enabled = true;
    config.retry.max_attempts = 5;
    CroupierInvoker invoker(config);

    const std::string result = invoker.Invoke("fn.retry", "{}");
    EXPECT_EQ(R"({"ok":true})", result);
    EXPECT_EQ(3, calls);
}

TEST(InvokerRetryTest, RetriesRateLimitAndCustomCodes) {
    int calls = 0;
    auto transport = std::make_shared<MockHTTPTransport>([&calls](const HTTPRequest&) {
        ++calls;
        if (calls == 1) return HTTPResponse{429, R"({"message":"slow down"})"};
        if (calls == 2) return HTTPResponse{408, R"({"message":"too early"})"};
        return HTTPResponse{200, R"({"result":1})"};
    });
    InvokerConfig config = FastConfig(transport);
    config.retry.enabled = true;
    config.retry.max_attempts = 3;
    config.retry.retryable_status_codes = {418};
    CroupierInvoker invoker(config);

    EXPECT_EQ("1", invoker.Invoke("fn.custom", "{}"));
    EXPECT_EQ(3, calls);
}

TEST(InvokerRetryTest, NonRetryableStatusFailsImmediately) {
    int calls = 0;
    auto transport = std::make_shared<MockHTTPTransport>([&calls](const HTTPRequest&) {
        ++calls;
        return HTTPResponse{404, R"({"message":"no such function"})"};
    });
    InvokerConfig config = FastConfig(transport);
    config.retry.enabled = true;
    config.retry.max_attempts = 5;
    CroupierInvoker invoker(config);

    EXPECT_THROW(invoker.Invoke("fn.missing", "{}"), std::runtime_error);
    EXPECT_EQ(1, calls);
}

TEST(InvokerRetryTest, ExhaustedRetriesRethrowLastError) {
    int calls = 0;
    auto transport = std::make_shared<MockHTTPTransport>([&calls](const HTTPRequest&) {
        ++calls;
        return HTTPResponse{503, R"({"message":"down"})"};
    });
    InvokerConfig config = FastConfig(transport);
    config.retry.enabled = true;
    config.retry.max_attempts = 2;
    CroupierInvoker invoker(config);

    try {
        (void)invoker.Invoke("fn.down", "{}");
        FAIL() << "expected the final 503 to propagate";
    } catch (const std::runtime_error& error) {
        EXPECT_NE(std::string(error.what()).find("503"), std::string::npos);
        EXPECT_NE(std::string(error.what()).find("down"), std::string::npos);
    }
    EXPECT_EQ(2, calls);
}

TEST(InvokerRetryTest, TransportExceptionsAreRetried) {
    int calls = 0;
    auto transport = std::make_shared<MockHTTPTransport>([&calls](const HTTPRequest&) -> HTTPResponse {
        ++calls;
        if (calls == 1) throw std::runtime_error("connection reset while sending request");
        return HTTPResponse{200, R"({"result":"recovered"})"};
    });
    InvokerConfig config = FastConfig(transport);
    config.retry.enabled = true;
    config.retry.max_attempts = 3;
    CroupierInvoker invoker(config);

    EXPECT_EQ(R"("recovered")", invoker.Invoke("fn.flaky", "{}"));
    EXPECT_EQ(2, calls);
}

TEST(InvokerCloseTest, ClosedInvokerRejectsRequestsAndConnect) {
    auto transport = std::make_shared<MockHTTPTransport>([](const HTTPRequest&) { return HTTPResponse{200, "{}"}; });
    CroupierInvoker invoker(FastConfig(transport));

    ASSERT_TRUE(invoker.Connect());
    invoker.Close();
    EXPECT_FALSE(invoker.Connect());
    EXPECT_THROW(invoker.Invoke("fn.x", "{}"), std::runtime_error);
    EXPECT_THROW(invoker.StartTask("fn.x", "{}"), std::runtime_error);
    EXPECT_THROW((void)invoker.GetTaskStatus("task-1"), std::runtime_error);
    EXPECT_FALSE(invoker.CancelTask("task-1"));
    EXPECT_TRUE(transport->requests.empty());
}

TEST(InvokerAddressTest, AddressNormalizationVariants) {
    struct Case {
        std::string input;
        std::string expected_base;
    };
    const Case cases[] = {
        {"", "http://127.0.0.1:18780/api/v1"},
        {"server.example", "http://server.example/api/v1"},
        {"https://server.example/", "https://server.example/api/v1"},
        {"http://server.example:9/api/v1", "http://server.example:9/api/v1"},
        {"http://server.example/custom", "http://server.example/custom/api/v1"},
        {"http://server.example/custom/api/v1/", "http://server.example/custom/api/v1"},
        {"  http://server.example  ", "http://server.example/api/v1"},
    };

    for (const auto& item : cases) {
        auto transport = std::make_shared<MockHTTPTransport>([](const HTTPRequest&) { return HTTPResponse{200, R"({"result":null})"}; });
        InvokerConfig config = FastConfig(transport);
        config.address = item.input;
        CroupierInvoker invoker(config);
        (void)invoker.Invoke("fn.echo", "{}");
        ASSERT_EQ(1U, transport->requests.size()) << "address: " << item.input;
        EXPECT_EQ(item.expected_base + "/functions/fn.echo/invoke", transport->requests[0].url)
            << "address: " << item.input;
    }
}

TEST(InvokerAddressTest, InvalidAddressesAreRejected) {
    const std::string bad[] = {
        "tcp://127.0.0.1:19090",       // legacy TCP is rejected
        "grpc://server.example",       // unsupported scheme
        "://server.example",           // empty scheme
        "http://",                     // missing host
    };
    for (const auto& address : bad) {
        InvokerConfig config;
        config.address = address;
        config.disable_logging = true;
        EXPECT_THROW({ CroupierInvoker invoker(config); }, std::invalid_argument) << "address: " << address;
    }
}

TEST(InvokerHeaderTest, HeaderInjectionIsRejected) {
    auto transport = std::make_shared<MockHTTPTransport>([](const HTTPRequest&) { return HTTPResponse{200, "{}"}; });
    InvokerConfig config = FastConfig(transport);
    config.headers["X-Bad"] = "value\r\nInjected: 1";
    CroupierInvoker invoker(config);
    EXPECT_THROW(invoker.Invoke("fn.x", "{}"), std::invalid_argument);
}

TEST(InvokerStreamTaskTest, StreamErrorProducesErrorEvent) {
    auto transport = std::make_shared<MockHTTPTransport>([](const HTTPRequest&) { return HTTPResponse{500, R"({"message":"events unavailable"})"}; });
    InvokerConfig config = FastConfig(transport);
    CroupierInvoker invoker(config);

    const std::vector<TaskEvent> events = invoker.StreamTask("task-9").get();
    ASSERT_EQ(1U, events.size());
    EXPECT_EQ("error", events[0].event_type);
    EXPECT_TRUE(events[0].done);
    EXPECT_NE(events[0].error.find("500"), std::string::npos);
}

TEST(InvokerStreamTaskTest, StreamPollsUntilDone) {
    int calls = 0;
    auto transport = std::make_shared<MockHTTPTransport>([&calls](const HTTPRequest& request) {
        if (request.url.find("/events?after_seq=0") != std::string::npos) {
            ++calls;
            return HTTPResponse{200, R"({"items":[{"seq":1,"type":"progress","progress":10,"message":"starting"}],"done":false})"};
        }
        if (request.url.find("/events?after_seq=1") != std::string::npos) {
            return HTTPResponse{200, R"({"items":[{"seq":2,"type":"done","message":"finished"}],"done":true})"};
        }
        return HTTPResponse{404, R"({"message":"unexpected"})"};
    });
    InvokerConfig config = FastConfig(transport);
    config.task_poll_interval_ms = 1;
    CroupierInvoker invoker(config);

    const std::vector<TaskEvent> events = invoker.StreamTask("task-2").get();
    ASSERT_EQ(2U, events.size());
    EXPECT_EQ("progress", events[0].event_type);
    EXPECT_EQ("completed", events[1].event_type);  // "done" is normalized to "completed"
    EXPECT_TRUE(events[1].done);
}

TEST(InvokerStreamTaskTest, CancelFailureReturnsFalse) {
    auto transport = std::make_shared<MockHTTPTransport>([](const HTTPRequest&) { return HTTPResponse{409, R"({"message":"already finished"})"}; });
    CroupierInvoker invoker(FastConfig(transport));
    EXPECT_FALSE(invoker.CancelTask("task-3"));
    EXPECT_FALSE(invoker.CancelTask(""));  // invalid id
}

TEST(InvokerTaskStatusTest, MapsAllResponseFields) {
    auto transport = std::make_shared<MockHTTPTransport>([](const HTTPRequest&) {
        return HTTPResponse{200,
                            R"({"id":"t1","functionId":"fn","status":"succeeded","progress":100,)"
                            R"("message":"done","error":"","result":{"k":"v"},"gameId":"g","env":"e",)"
                            R"("agentId":"a","actor":"u","traceId":"tr","startedAt":"s1","finishedAt":"f1",)"
                            R"("createdAt":"c1","updatedAt":"u1"})"};
    });
    CroupierInvoker invoker(FastConfig(transport));

    const TaskStatus status = invoker.GetTaskStatus("t1");
    EXPECT_EQ("t1", status.task_id);
    EXPECT_EQ("fn", status.function_id);
    EXPECT_EQ("succeeded", status.status);
    EXPECT_EQ(100, status.progress);
    EXPECT_EQ(R"({"k":"v"})", status.result);
    EXPECT_EQ("g", status.game_id);
    EXPECT_EQ("e", status.env);
    EXPECT_EQ("a", status.agent_id);
    EXPECT_EQ("u", status.actor);
    EXPECT_EQ("tr", status.trace_id);
    EXPECT_EQ("s1", status.started_at);
    EXPECT_EQ("f1", status.finished_at);
    EXPECT_EQ("c1", status.created_at);
    EXPECT_EQ("u1", status.updated_at);
}

TEST(InvokerTaskStatusTest, InvalidTaskIdRejected) {
    auto transport = std::make_shared<MockHTTPTransport>([](const HTTPRequest&) { return HTTPResponse{200, "{}"}; });
    CroupierInvoker invoker(FastConfig(transport));
    EXPECT_THROW((void)invoker.GetTaskStatus("  "), std::invalid_argument);
}

TEST(InvokerSchemaTest, PayloadSchemaRejectsInvalidPayload) {
    auto transport = std::make_shared<MockHTTPTransport>([](const HTTPRequest&) { return HTTPResponse{200, "{}"}; });
    CroupierInvoker invoker(FastConfig(transport));

    invoker.SetSchema("fn.schema", {{"type", "object"}, {"required", R"(["playerId"])"}});
    EXPECT_THROW(invoker.Invoke("fn.schema", R"({"other":1})"), std::runtime_error);
    // With a schema set, non-JSON payloads fail schema validation first.
    EXPECT_THROW(invoker.Invoke("fn.schema", "not-json"), std::runtime_error);
    EXPECT_TRUE(transport->requests.empty());
}

}  // namespace croupier::sdk::test
