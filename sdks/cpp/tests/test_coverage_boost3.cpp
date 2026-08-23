// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

// Third coverage boost: invoker error paths (invalid payloads, server errors,
// missing results, terminal-event mapping, retry configuration) and TCP
// address handling through the public client surface.

#include <gtest/gtest.h>

#include "croupier/sdk/croupier_client.h"
#include "croupier/sdk/http_transport.h"

#include <algorithm>
#include <map>
#include <mutex>
#include <string>
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

InvokerConfig MakeConfig(const std::shared_ptr<HTTPTransport>& transport) {
    InvokerConfig config;
    config.address = "http://server.example";
    config.task_poll_interval_ms = 1;
    config.retry.enabled = false;
    config.http_transport = transport;
    config.disable_logging = true;
    return config;
}

std::string BodyOf(const HTTPRequest& request) { return request.body; }

RetryConfig MinimalRetry() {
    RetryConfig retry;
    retry.initial_delay_ms = 1;
    return retry;
}

}  // namespace

// ---------------------------------------------------------------------------
// Invoke payload validation and request shaping
// ---------------------------------------------------------------------------

TEST(InvokerBoost3Test, InvokeRejectsInvalidPayloadJSON) {
    auto transport = std::make_shared<MockHTTPTransport>(
        [](const HTTPRequest&) { return HTTPResponse{200, R"({"result":{}})"}; });
    CroupierInvoker invoker(MakeConfig(transport));

    ASSERT_THROW(invoker.Invoke("fn", "{not json", {}), std::invalid_argument);
    // Nothing reached the network.
    EXPECT_TRUE(transport->requests.empty());
}

TEST(InvokerBoost3Test, InvokeSendsRouteAndTargetOptions) {
    auto transport = std::make_shared<MockHTTPTransport>(
        [](const HTTPRequest&) { return HTTPResponse{200, R"({"result":{"ok":true}})"}; });
    CroupierInvoker invoker(MakeConfig(transport));

    InvokeOptions options;
    options.route = "broadcast";
    options.target_service_id = "svc-7";
    options.hash_key = "player-1";

    const std::string result = invoker.Invoke("fn", "{}", options);
    EXPECT_NE(std::string::npos, result.find("ok"));

    ASSERT_EQ(1u, transport->requests.size());
    const std::string& body = BodyOf(transport->requests[0]);
    EXPECT_NE(std::string::npos, body.find("\"route\":\"broadcast\""));
    EXPECT_NE(std::string::npos, body.find("\"targetServiceId\":\"svc-7\""));
    EXPECT_NE(std::string::npos, body.find("\"hashKey\":\"player-1\""));
}

TEST(InvokerBoost3Test, InvokeWithoutRoutingOptionsOmitsFields) {
    auto transport = std::make_shared<MockHTTPTransport>(
        [](const HTTPRequest&) { return HTTPResponse{200, R"({"result":1})"}; });
    CroupierInvoker invoker(MakeConfig(transport));

    invoker.Invoke("fn", "{}", {});
    const std::string& body = BodyOf(transport->requests[0]);
    EXPECT_EQ(std::string::npos, body.find("route"));
    EXPECT_EQ(std::string::npos, body.find("targetServiceId"));
    EXPECT_EQ(std::string::npos, body.find("hashKey"));
}

TEST(InvokerBoost3Test, InvokeRequiresResultKey) {
    auto transport = std::make_shared<MockHTTPTransport>(
        [](const HTTPRequest&) { return HTTPResponse{200, R"({"other":1})"}; });
    CroupierInvoker invoker(MakeConfig(transport));

    ASSERT_THROW(invoker.Invoke("fn", "{}", {}), std::runtime_error);
}

TEST(InvokerBoost3Test, InvokeRejectsEmptyFunctionID) {
    auto transport = std::make_shared<MockHTTPTransport>(
        [](const HTTPRequest&) { return HTTPResponse{200, "{}"}; });
    CroupierInvoker invoker(MakeConfig(transport));

    ASSERT_THROW(invoker.Invoke("   ", "{}", {}), std::invalid_argument);
    EXPECT_TRUE(transport->requests.empty());
}

TEST(InvokerBoost3Test, ServerErrorResponsesSurfaceAsFailures) {
    auto transport = std::make_shared<MockHTTPTransport>(
        [](const HTTPRequest&) { return HTTPResponse{403, R"({"message":"denied"})"}; });
    CroupierInvoker invoker(MakeConfig(transport));

    ASSERT_THROW(invoker.Invoke("fn", "{}", {}), std::runtime_error);
}

TEST(InvokerBoost3Test, InvalidJSONResponseSurfacesAsFailure) {
    auto transport = std::make_shared<MockHTTPTransport>(
        [](const HTTPRequest&) { return HTTPResponse{200, "not-json"}; });
    CroupierInvoker invoker(MakeConfig(transport));

    ASSERT_THROW(invoker.Invoke("fn", "{}", {}), std::runtime_error);
}

TEST(InvokerBoost3Test, NonObjectResponseSurfacesAsFailure) {
    auto transport = std::make_shared<MockHTTPTransport>(
        [](const HTTPRequest&) { return HTTPResponse{200, "[1,2]"}; });
    CroupierInvoker invoker(MakeConfig(transport));

    ASSERT_THROW(invoker.Invoke("fn", "{}", {}), std::runtime_error);
}

// ---------------------------------------------------------------------------
// Schema validation wiring
// ---------------------------------------------------------------------------

TEST(InvokerBoost3Test, SetSchemaRejectsInvalidPayloadsBeforeNetwork) {
    auto transport = std::make_shared<MockHTTPTransport>(
        [](const HTTPRequest&) { return HTTPResponse{200, R"({"result":{}})"}; });
    CroupierInvoker invoker(MakeConfig(transport));

    std::map<std::string, std::string> schema = {{"type", "object"}, {"required", R"(["playerId"])"}};
    invoker.SetSchema("fn", schema);

    ASSERT_THROW(invoker.Invoke("fn", R"({"other":1})", {}), std::runtime_error);
    EXPECT_TRUE(transport->requests.empty());

    EXPECT_NO_THROW(invoker.Invoke("fn", R"({"playerId":"p"})", {}));
    EXPECT_EQ(1u, transport->requests.size());
}

// ---------------------------------------------------------------------------
// StartTask / GetTaskStatus / CancelTask edges
// ---------------------------------------------------------------------------

TEST(InvokerBoost3Test, StartTaskRequiresTaskIdInResponse) {
    auto transport = std::make_shared<MockHTTPTransport>(
        [](const HTTPRequest&) { return HTTPResponse{200, R"({"status":"queued"})"}; });
    CroupierInvoker invoker(MakeConfig(transport));

    ASSERT_THROW(invoker.StartTask("fn", "{}", {}), std::runtime_error);
}

TEST(InvokerBoost3Test, StartTaskSendsFunctionIdAndParams) {
    auto transport = std::make_shared<MockHTTPTransport>(
        [](const HTTPRequest&) { return HTTPResponse{200, R"({"taskId":"t-42"})"}; });
    CroupierInvoker invoker(MakeConfig(transport));

    const std::string task_id = invoker.StartTask("report.generate", R"({"days":7})", {});
    EXPECT_EQ("t-42", task_id);

    const std::string& body = BodyOf(transport->requests[0]);
    EXPECT_NE(std::string::npos, body.find(R"("functionId":"report.generate")"));
    EXPECT_NE(std::string::npos, body.find(R"("days":7)"));
}

TEST(InvokerBoost3Test, StartTaskBlankTaskIdRejected) {
    auto transport = std::make_shared<MockHTTPTransport>(
        [](const HTTPRequest&) { return HTTPResponse{200, R"({"taskId":"   "})"}; });
    CroupierInvoker invoker(MakeConfig(transport));

    ASSERT_THROW(invoker.StartTask("fn", "{}", {}), std::runtime_error);
}

TEST(InvokerBoost3Test, GetTaskStatusFallsBackToRequestedId) {
    auto transport = std::make_shared<MockHTTPTransport>(
        [](const HTTPRequest&) { return HTTPResponse{200, R"({"status":"queued"})"}; });
    CroupierInvoker invoker(MakeConfig(transport));

    TaskStatus status = invoker.GetTaskStatus("requested-id");
    EXPECT_EQ("requested-id", status.task_id);
    EXPECT_EQ("queued", status.status);
}

TEST(InvokerBoost3Test, GetTaskStatusMapsFullFieldSet) {
    auto transport = std::make_shared<MockHTTPTransport>([](const HTTPRequest&) {
        return HTTPResponse{200,
            R"({"id":"t-9","functionId":"fn","status":"running","progress":60,"message":"working",)"
            R"("error":null,"gameId":"g","env":"staging","agentId":"a","actor":"admin"})"};
    });
    CroupierInvoker invoker(MakeConfig(transport));

    TaskStatus status = invoker.GetTaskStatus("t-9");
    EXPECT_EQ("t-9", status.task_id);
    EXPECT_EQ("fn", status.function_id);
    EXPECT_EQ("running", status.status);
    EXPECT_EQ(60, status.progress);
    EXPECT_EQ("working", status.message);
    EXPECT_EQ("g", status.game_id);
    EXPECT_EQ("staging", status.env);
    EXPECT_EQ("a", status.agent_id);
    EXPECT_EQ("admin", status.actor);
}

TEST(InvokerBoost3Test, CancelTaskSendsPostAndReportsSuccess) {
    auto transport = std::make_shared<MockHTTPTransport>(
        [](const HTTPRequest&) { return HTTPResponse{200, "{}"}; });
    CroupierInvoker invoker(MakeConfig(transport));

    EXPECT_TRUE(invoker.CancelTask("t-1"));
    ASSERT_EQ(1u, transport->requests.size());
    EXPECT_EQ("POST", transport->requests[0].method);
    EXPECT_NE(std::string::npos, transport->requests[0].url.find("/tasks/t-1/cancel"));
}

// ---------------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------------

TEST(InvokerBoost3Test, ConnectAndCloseToggleState) {
    auto transport = std::make_shared<MockHTTPTransport>(
        [](const HTTPRequest&) { return HTTPResponse{200, "{}"}; });
    CroupierInvoker invoker(MakeConfig(transport));

    EXPECT_TRUE(invoker.Connect());

    invoker.SetRetryConfig(MinimalRetry());
    invoker.SetReconnectConfig(ReconnectConfig{});
    invoker.Close();
    EXPECT_FALSE(invoker.Connect());  // closed invokers stay closed
}



TEST(InvokerBoost3Test, DefaultHTTPTransportIsCreatedWhenNotInjected) {
    InvokerConfig config;
    config.address = "http://127.0.0.1:1";
    config.retry.enabled = false;
    config.disable_logging = true;
    CroupierInvoker invoker(config);  // must not throw
    invoker.Close();
    EXPECT_FALSE(invoker.Connect());
}

TEST(InvokerBoost3Test, ConstructorRejectsInvalidAddress) {
    InvokerConfig config;
    config.address = "ftp://bad-scheme";
    ASSERT_THROW(CroupierInvoker invoker(config), std::invalid_argument);
}

// ---------------------------------------------------------------------------
// Client-side TCP address handling via public API
// ---------------------------------------------------------------------------

TEST(ClientAddressBoostTest, ConnectRejectsAddressesWithoutPort) {
    ClientConfig config;
    config.agent_addr = "no-port-host";
    config.service_id = "addr-svc";
    CroupierClient client(config);
    FunctionDescriptor descriptor;
    descriptor.id = "missing.port.fn";
    descriptor.version = "1.0.0";
    client.RegisterFunction(descriptor, [](const std::string&, const std::string&) { return "{}"; });

    EXPECT_FALSE(client.Connect());
}

TEST(ClientAddressBoostTest, ConnectRejectsOutOfRangePorts) {
    ClientConfig config;
    config.agent_addr = "127.0.0.1:70000";
    config.service_id = "addr-svc";
    CroupierClient client(config);
    FunctionDescriptor descriptor;
    descriptor.id = "port.fn";
    descriptor.version = "1.0.0";
    client.RegisterFunction(descriptor, [](const std::string&, const std::string&) { return "{}"; });

    EXPECT_FALSE(client.Connect());
}

TEST(ClientAddressBoostTest, ConnectRejectsNonNumericPorts) {
    ClientConfig config;
    config.agent_addr = "127.0.0.1:not-a-port";
    config.service_id = "addr-svc";
    CroupierClient client(config);
    FunctionDescriptor descriptor;
    descriptor.id = "port2.fn";
    descriptor.version = "1.0.0";
    client.RegisterFunction(descriptor, [](const std::string&, const std::string&) { return "{}"; });

    EXPECT_FALSE(client.Connect());
}

TEST(ClientAddressBoostTest, RegisterRejectsEmptyFunctionID) {
    ClientConfig config;
    config.agent_addr = "127.0.0.1:19091";
    CroupierClient client(config);
    FunctionDescriptor descriptor;
    descriptor.id = "";
    descriptor.version = "1.0.0";
    EXPECT_FALSE(client.RegisterFunction(descriptor, [](const std::string&, const std::string&) { return "{}"; }));
}

TEST(ClientAddressBoostTest, RegisterAcceptsIPv6StyleAddressConfig) {
    // The client stores the address; only Connect parses it. Building must not throw.
    ClientConfig config;
    config.agent_addr = "[::1]:19091";
    config.service_id = "v6-svc";
    CroupierClient client(config);
    FunctionDescriptor descriptor;
    descriptor.id = "v6.fn";
    descriptor.version = "1.0.0";
    EXPECT_TRUE(client.RegisterFunction(descriptor,
                                        [](const std::string&, const std::string&) { return "{}"; }));
    EXPECT_FALSE(client.IsConnected());
    client.Close();
}
}  // namespace croupier::sdk::test
