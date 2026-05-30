#include <gtest/gtest.h>

#include "croupier/sdk/croupier_client.h"

using namespace croupier::sdk;

namespace {

InvokerConfig MakeConfig() {
    InvokerConfig config;
    config.address = "127.0.0.1:19090";
    config.game_id = "test-game";
    config.env = "testing";
    config.disable_logging = true;
    return config;
}

}  // namespace

TEST(InvokerFallbackTest, ConnectUsesFallbackBehaviorWithoutTCP) {
    CroupierInvoker invoker(MakeConfig());

#ifdef CROUPIER_SDK_HAS_TCP
    SUCCEED();
#else
    EXPECT_TRUE(invoker.Connect());
#endif
}

TEST(InvokerFallbackTest, InvokeUsesFallbackBehaviorWithoutTCP) {
    CroupierInvoker invoker(MakeConfig());

#ifdef CROUPIER_SDK_HAS_TCP
    SUCCEED();
#else
    ASSERT_TRUE(invoker.Connect());
    const std::string response = invoker.Invoke("player.echo", R"({"ok":true})");
    EXPECT_NE(response.find("\"status\":\"success\""), std::string::npos);
    EXPECT_NE(response.find("\"function_id\":\"player.echo\""), std::string::npos);
#endif
}

TEST(InvokerFallbackTest, StartTaskUsesFallbackBehaviorWithoutTCP) {
    CroupierInvoker invoker(MakeConfig());

#ifdef CROUPIER_SDK_HAS_TCP
    SUCCEED();
#else
    ASSERT_TRUE(invoker.Connect());
    EXPECT_FALSE(invoker.StartTask("player.batch", "{}").empty());
#endif
}

TEST(InvokerFallbackTest, StreamTaskUsesFallbackBehaviorWithoutTCP) {
    CroupierInvoker invoker(MakeConfig());

#ifdef CROUPIER_SDK_HAS_TCP
    SUCCEED();
#else
    ASSERT_TRUE(invoker.Connect());
    const std::string task_id = invoker.StartTask("player.batch", "{}");
    auto future = invoker.StreamTask(task_id);
    auto events = future.get();

    ASSERT_GE(events.size(), 2U);
    EXPECT_EQ(events.front().event_type, "started");
    EXPECT_TRUE(events.back().done);
#endif
}

TEST(InvokerFallbackTest, CancelTaskUsesFallbackBehaviorWithoutTCP) {
    CroupierInvoker invoker(MakeConfig());

#ifdef CROUPIER_SDK_HAS_TCP
    SUCCEED();
#else
    ASSERT_TRUE(invoker.Connect());
    const std::string task_id = invoker.StartTask("player.batch", "{}");
    EXPECT_TRUE(invoker.CancelTask(task_id));
#endif
}
