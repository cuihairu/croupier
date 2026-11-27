// AUD-069: pending-approval responses must be preserved, not mis-decoded.

#include "croupier/sdk/croupier_client.h"

#include <gtest/gtest.h>

#include <functional>
#include <memory>
#include <string>
#include <vector>

namespace croupier::sdk::test {

namespace {

class MockHTTPTransport final : public HTTPTransport {
public:
    using Responder = std::function<HTTPResponse(const HTTPRequest&)>;

    explicit MockHTTPTransport(Responder responder) : responder_(std::move(responder)) {}

    HTTPResponse Send(const HTTPRequest& request) override {
        requests.push_back(request);
        return responder_(request);
    }

    std::vector<HTTPRequest> requests;

private:
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
    return config;
}

}  // namespace

TEST(ServerHTTPInvokerPendingApproval, InvokePreservesApprovalCredential) {
    auto transport = std::make_shared<MockHTTPTransport>([](const HTTPRequest&) {
        return HTTPResponse{200, R"({"taskId":"","approvalId":"apr-1","approvalRequired":true,"approvalWorkflow":"two_person","traceId":"tr-1"})"};
    });
    CroupierInvoker invoker(Config(transport));
    try {
        invoker.Invoke("player.ban", R"({"playerId":"p-1"})");
        FAIL() << "expected ApprovalRequiredError";
    } catch (const ApprovalRequiredError& error) {
        EXPECT_EQ("apr-1", error.approval_id());
        EXPECT_EQ("two_person", error.approval_workflow());
        EXPECT_TRUE(std::string(error.what()).find("apr-1") != std::string::npos);
    }
}

TEST(ServerHTTPInvokerPendingApproval, StartTaskPreservesApprovalStatus) {
    auto transport = std::make_shared<MockHTTPTransport>([](const HTTPRequest&) {
        return HTTPResponse{200, R"({"taskId":"","status":"approval_required","approvalId":"apr-2","approvalWorkflow":"single_admin"})"};
    });
    CroupierInvoker invoker(Config(transport));
    try {
        invoker.StartTask("player.ban", R"({"playerId":"p-1"})");
        FAIL() << "expected ApprovalRequiredError";
    } catch (const ApprovalRequiredError& error) {
        EXPECT_EQ("apr-2", error.approval_id());
        EXPECT_EQ("approval_required", error.status());
        EXPECT_EQ("single_admin", error.approval_workflow());
    }
}

TEST(ServerHTTPInvokerPendingApproval, MissingApprovalIdStaysMissingOutcome) {
    auto transport = std::make_shared<MockHTTPTransport>([](const HTTPRequest&) {
        return HTTPResponse{200, R"({"taskId":"","status":"approval_required"})"};
    });
    CroupierInvoker invoker(Config(transport));
    EXPECT_THROW(invoker.StartTask("player.ban", R"({"playerId":"p-1"})"), std::runtime_error);
    try {
        invoker.StartTask("player.ban", R"({"playerId":"p-1"})");
    } catch (const std::runtime_error& error) {
        EXPECT_TRUE(std::string(error.what()).find("taskId") != std::string::npos);
    }
}

}  // namespace croupier::sdk::test
