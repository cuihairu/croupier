#include <gtest/gtest.h>

#include "croupier/sdk/croupier_client.h"

#include <functional>
#include <cstdlib>
#include <chrono>
#include <memory>
#include <mutex>
#include <stdexcept>
#include <thread>
#include <utility>
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

InvokerConfig Config(const std::shared_ptr<HTTPTransport>& transport) {
    InvokerConfig config;
    config.address = "http://server.example";
    config.auth_token = "server-token";
    config.game_id = "game-a";
    config.env = "staging";
    config.task_poll_interval_ms = 1;
    config.retry.enabled = false;
    config.http_transport = transport;
    config.disable_logging = true;
    return config;
}

std::string Header(const HTTPRequest& request, const std::string& name) {
    for (const auto& [key, value] : request.headers) {
        if (key == name) return value;
    }
    return "";
}

}  // namespace

TEST(ServerHTTPInvokerTest, UsesServerHTTPContractForInvokeAndTaskLifecycle) {
    int events_calls = 0;
    auto transport = std::make_shared<MockHTTPTransport>([&events_calls](const HTTPRequest& request) {
        if (request.method == "POST" && request.url == "http://server.example/api/v1/functions/player.ban/invoke") {
            return HTTPResponse{200, R"({"result":{"status":"banned"}})"};
        }
        if (request.method == "POST" && request.url == "http://server.example/api/v1/tasks") {
            return HTTPResponse{200, R"({"taskId":"task-1","status":"dispatching"})"};
        }
        if (request.method == "GET" && request.url == "http://server.example/api/v1/tasks/task-1") {
            return HTTPResponse{200, R"({"id":"task-1","functionId":"report.generate","status":"running","progress":50,"result":{"partial":true}})"};
        }
        if (request.method == "GET" && request.url == "http://server.example/api/v1/tasks/task-1/events?after_seq=0") {
            ++events_calls;
            return HTTPResponse{200, R"({"items":[{"seq":1,"type":"progress","progress":50,"message":"halfway","payload":{"count":1}}],"done":false})"};
        }
        if (request.method == "GET" && request.url == "http://server.example/api/v1/tasks/task-1/events?after_seq=1") {
            ++events_calls;
            return HTTPResponse{200, R"({"items":[{"seq":2,"type":"completed","payload":{"ok":true}}],"done":true})"};
        }
        if (request.method == "POST" && request.url == "http://server.example/api/v1/tasks/task-1/cancel") {
            return HTTPResponse{200, R"({"message":"accepted"})"};
        }
        return HTTPResponse{404, R"({"message":"unexpected path"})"};
    });

    CroupierInvoker invoker(Config(transport));
    ASSERT_TRUE(invoker.Connect());
    InvokeOptions options;
    options.idempotency_key = "invoke-1";
    options.route = "targeted";
    options.target_service_id = "provider-1";
    const std::string result = invoker.Invoke("player.ban", R"({"playerId":"p-1"})", options);
    const std::string task_id = invoker.StartTask("report.generate", R"({"range":"daily"})");
    const TaskStatus status = invoker.GetTaskStatus(task_id);
    const std::vector<TaskEvent> events = invoker.StreamTask(task_id).get();
    EXPECT_TRUE(invoker.CancelTask(task_id));

    EXPECT_EQ(R"({"status":"banned"})", result);
    EXPECT_EQ("task-1", task_id);
    EXPECT_EQ("running", status.status);
    EXPECT_EQ(R"({"partial":true})", status.result);
    ASSERT_EQ(2U, events.size());
    EXPECT_EQ("progress", events[0].event_type);
    EXPECT_EQ(R"({"count":1})", events[0].payload);
    EXPECT_FALSE(events[0].done);
    EXPECT_EQ("completed", events[1].event_type);
    EXPECT_EQ(R"({"ok":true})", events[1].payload);
    EXPECT_TRUE(events[1].done);
    EXPECT_EQ(2, events_calls);

    ASSERT_EQ(6U, transport->requests.size());
    const HTTPRequest& invoke = transport->requests.front();
    EXPECT_EQ("Bearer server-token", Header(invoke, "Authorization"));
    EXPECT_EQ("game-a", Header(invoke, "X-Game-ID"));
    EXPECT_EQ("staging", Header(invoke, "X-Env"));
    EXPECT_EQ("invoke-1", Header(invoke, "Idempotency-Key"));
    EXPECT_EQ("application/json", Header(invoke, "Content-Type"));
    EXPECT_EQ(R"({"params":{"playerId":"p-1"},"route":"targeted","targetServiceId":"provider-1"})", invoke.body);
    EXPECT_EQ(R"({"functionId":"report.generate","params":{"range":"daily"}})", transport->requests[1].body);
    EXPECT_EQ("{}", transport->requests.back().body);
}

TEST(ServerHTTPInvokerTest, RejectsLegacyTCPAndNeverFabricatesSuccessfulResults) {
    InvokerConfig tcp_config;
    tcp_config.address = "tcp://127.0.0.1:19090";
    EXPECT_THROW({
        CroupierInvoker rejected(tcp_config);
        (void)rejected;
    }, std::invalid_argument);

    auto transport = std::make_shared<MockHTTPTransport>([](const HTTPRequest&) {
        return HTTPResponse{503, R"({"message":"temporarily unavailable"})"};
    });
    InvokerConfig config = Config(transport);
    config.address = "server.example:18780";
    CroupierInvoker invoker(config);

    try {
        (void)invoker.Invoke("health.check", "{}");
        FAIL() << "a Server error must not be converted into a local success";
    } catch (const std::runtime_error& error) {
        EXPECT_NE(std::string(error.what()).find("503"), std::string::npos);
        EXPECT_NE(std::string(error.what()).find("temporarily unavailable"), std::string::npos);
    }
    ASSERT_EQ(1U, transport->requests.size());
    EXPECT_EQ("http://server.example:18780/api/v1/functions/health.check/invoke", transport->requests[0].url);
}

TEST(ServerHTTPInvokerTest, ValidatesInputAndServerResponsesBeforeReportingSuccess) {
    int calls = 0;
    auto transport = std::make_shared<MockHTTPTransport>([&calls](const HTTPRequest&) {
        ++calls;
        return HTTPResponse{200, R"({"status":"dispatching"})"};
    });
    CroupierInvoker invoker(Config(transport));

    EXPECT_THROW(invoker.Invoke("", "{}"), std::invalid_argument);
    EXPECT_THROW(invoker.StartTask("report.generate", "not-json"), std::invalid_argument);
    invoker.SetSchema("report.generate", {{"required", R"(["range"])"}});
    EXPECT_THROW(invoker.StartTask("report.generate", "{}"), std::runtime_error);
    EXPECT_EQ(0, calls);

    EXPECT_THROW(invoker.StartTask("report.generate", R"({"range":"daily"})"), std::runtime_error);
    EXPECT_EQ(1, calls);
    EXPECT_FALSE(invoker.CancelTask(""));
}

TEST(ServerHTTPInvokerTest, UsesRealServerForAuthenticatedTaskLifecycle) {
    const char* server_url = std::getenv("CROUPIER_SERVER_URL");
    const char* token = std::getenv("CROUPIER_SERVER_TOKEN");
    if (server_url == nullptr || token == nullptr || *server_url == '\0' || *token == '\0') {
        GTEST_SKIP() << "CROUPIER_SERVER_URL and CROUPIER_SERVER_TOKEN are required";
    }

    const std::string game_id = std::getenv("CROUPIER_GAME_ID") == nullptr
        ? "e2e-game" : std::getenv("CROUPIER_GAME_ID");
    const std::string env = std::getenv("CROUPIER_ENV") == nullptr
        ? "e2e" : std::getenv("CROUPIER_ENV");

    InvokerConfig unauthenticated_config;
    unauthenticated_config.address = server_url;
    unauthenticated_config.game_id = game_id;
    unauthenticated_config.env = env;
    unauthenticated_config.retry.enabled = false;
    CroupierInvoker unauthenticated(unauthenticated_config);
    EXPECT_THROW(unauthenticated.Invoke("mail.send", R"({"player_id":"p-001","title":"denied"})"), std::runtime_error);

    InvokerConfig config = unauthenticated_config;
    config.auth_token = token;
    config.task_poll_interval_ms = 10;
    CroupierInvoker invoker(config);
    ASSERT_TRUE(invoker.Connect());

    const std::string result = invoker.Invoke("mail.send", R"({"player_id":"p-001","title":"C++","content":"body"})");
    EXPECT_NE(result.find(R"("mail_id":"mail-0001")"), std::string::npos);

    const std::string completed_id = invoker.StartTask("mail.send", R"({"player_id":"p-001","title":"task"})");
    const auto completed_events = invoker.StreamTask(completed_id).get();
    ASSERT_FALSE(completed_events.empty());
    bool saw_started = false;
    bool saw_completed = false;
    std::string completed_types;
    for (const auto& event : completed_events) {
        saw_started |= event.event_type == "started";
        saw_completed |= event.event_type == "completed";
        if (!completed_types.empty()) completed_types += ",";
        completed_types += event.event_type + ":" + event.message;
    }
    EXPECT_TRUE(saw_started) << completed_types;
    EXPECT_TRUE(saw_completed) << completed_types;

    TaskStatus completed;
    const auto deadline = std::chrono::steady_clock::now() + std::chrono::seconds(20);
    do {
        completed = invoker.GetTaskStatus(completed_id);
        if (completed.status == "succeeded") break;
        std::this_thread::sleep_for(std::chrono::milliseconds(50));
    } while (std::chrono::steady_clock::now() < deadline);
    EXPECT_EQ("succeeded", completed.status);
    EXPECT_EQ(completed_id, completed.task_id);
    EXPECT_NE(completed.result.find(R"("mail_id":"mail-0001")"), std::string::npos);

    const std::string cancelled_id = invoker.StartTask("mail.wait", R"({"wait_ms":30000})");
    TaskStatus running;
    const auto running_deadline = std::chrono::steady_clock::now() + std::chrono::seconds(20);
    do {
        running = invoker.GetTaskStatus(cancelled_id);
        if (running.status == "running") break;
        std::this_thread::sleep_for(std::chrono::milliseconds(50));
    } while (std::chrono::steady_clock::now() < running_deadline);
    ASSERT_EQ("running", running.status);
    ASSERT_TRUE(invoker.CancelTask(cancelled_id));

    const auto cancelled_events = invoker.StreamTask(cancelled_id).get();
    bool saw_cancelled = false;
    for (const auto& event : cancelled_events) saw_cancelled |= event.event_type == "cancelled";
    EXPECT_TRUE(saw_cancelled);

    TaskStatus cancelled;
    const auto cancelled_deadline = std::chrono::steady_clock::now() + std::chrono::seconds(20);
    do {
        cancelled = invoker.GetTaskStatus(cancelled_id);
        if (cancelled.status == "cancelled") break;
        std::this_thread::sleep_for(std::chrono::milliseconds(50));
    } while (std::chrono::steady_clock::now() < cancelled_deadline);
    EXPECT_EQ("cancelled", cancelled.status);
}

}  // namespace croupier::sdk::test
