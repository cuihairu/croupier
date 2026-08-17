#include <gtest/gtest.h>

#include "croupier/sdk/croupier_client.h"

#include <memory>

namespace croupier::sdk::test {
namespace {

class FailingHTTPTransport final : public HTTPTransport {
public:
    HTTPResponse Send(const HTTPRequest&) override { return HTTPResponse{403, R"({"message":"scope denied"})"}; }
};

}  // namespace

TEST(ServerHTTPInvokerFailureTest, HTTPFailureRemainsAFailureForEveryPublicOperation) {
    InvokerConfig config;
    config.address = "http://server.example/api/v1";
    config.retry.enabled = false;
    config.http_transport = std::make_shared<FailingHTTPTransport>();
    config.disable_logging = true;
    CroupierInvoker invoker(config);

    EXPECT_THROW(invoker.Invoke("player.ban", "{}"), std::runtime_error);
    EXPECT_THROW(invoker.StartTask("player.ban", "{}"), std::runtime_error);
    EXPECT_THROW(invoker.GetTaskStatus("task-1"), std::runtime_error);
    EXPECT_FALSE(invoker.CancelTask("task-1"));

    const std::vector<TaskEvent> events = invoker.StreamTask("task-1").get();
    ASSERT_EQ(1U, events.size());
    EXPECT_EQ("error", events.front().event_type);
    EXPECT_TRUE(events.front().done);
    EXPECT_NE(events.front().error.find("scope denied"), std::string::npos);
}

}  // namespace croupier::sdk::test
