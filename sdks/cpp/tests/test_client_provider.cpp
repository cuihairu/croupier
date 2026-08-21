// Provider-side client lifecycle against a fake Agent built from the SDK's own
// TCPServer: registration, heartbeats, Serve() loop, and reconnection.
#include <gtest/gtest.h>
#include "croupier/sdk/croupier_client.h"
#include "croupier/sdk/tcp_transport.h"
#include "croupier/sdk/v1/provider.pb.h"

#include <atomic>
#include <chrono>
#include <memory>
#include <thread>

namespace croupier::sdk::test {
namespace {

using ::croupier::sdk::v1::ProviderConnectRequest;
using ::croupier::sdk::v1::ProviderConnectResponse;
using ::croupier::sdk::v1::ProviderHeartbeatRequest;

enum class AgentMode {
    kAccept,             // normal registration with a session id
    kJsonError,          // respond with a JSON error document
    kEmptySessionId,     // respond with a protobuf response lacking session_id
};

// Picks a free loopback port so a restarted fake agent rebinds the same port.
int ReserveLoopbackPort() {
    TCPServer probe("127.0.0.1:0");
    probe.Start();
    const std::string address = probe.GetListenAddress();
    probe.Stop();
    return std::stoi(address.substr(address.rfind(':') + 1));
}

class FakeAgent {
public:
    explicit FakeAgent(AgentMode mode = AgentMode::kAccept)
        : mode_(mode), server_(/*listen_address=*/"127.0.0.1:" + std::to_string(ReserveLoopbackPort()),
                               /*timeout_ms=*/5000) {
        server_.SetHandler([this](uint32_t msg_type, uint32_t /*req_id*/, const std::vector<uint8_t>& body) {
            if (msg_type == protocol::MSG_PROVIDER_CONNECT_REQUEST) {
                ++connect_requests_;
                ProviderConnectRequest request;
                (void)request.ParseFromArray(body.data(), static_cast<int>(body.size()));
                last_service_id_ = request.service_id();
                last_function_count_ = static_cast<int>(request.functions_size());

                if (mode_ == AgentMode::kJsonError) {
                    const std::string json_error = R"({"error":"rejected"})";
                    return std::vector<uint8_t>(json_error.begin(), json_error.end());
                }
                ProviderConnectResponse response;
                if (mode_ == AgentMode::kAccept) {
                    response.set_session_id("session-42");
                }
                std::string out;
                response.SerializeToString(&out);
                return std::vector<uint8_t>(out.begin(), out.end());
            }
            if (msg_type == protocol::MSG_PROVIDER_HEARTBEAT_REQUEST) {
                ++heartbeats_;
                ProviderHeartbeatRequest request;
                (void)request.ParseFromArray(body.data(), static_cast<int>(body.size()));
                last_heartbeat_session_ = request.session_id();
                return std::vector<uint8_t>{};
            }
            return std::vector<uint8_t>{};
        });
        server_.Start();
    }

    ~FakeAgent() { server_.Stop(); }

    std::string address() const { return server_.GetListenAddress(); }
    int port() const {
        const size_t colon = address().rfind(':');
        return std::stoi(address().substr(colon + 1));
    }
    int heartbeats() const { return heartbeats_; }
    int connect_requests() const { return connect_requests_; }
    const std::string& last_service_id() const { return last_service_id_; }
    const std::string& last_heartbeat_session() const { return last_heartbeat_session_; }
    int last_function_count() const { return last_function_count_; }

    void stop() { server_.Stop(); }
    void restart() { server_.Start(); }

private:
    AgentMode mode_;
    TCPServer server_;
    std::atomic<int> heartbeats_{0};
    std::atomic<int> connect_requests_{0};
    std::string last_service_id_;
    std::string last_heartbeat_session_;
    std::atomic<int> last_function_count_{0};
};

ClientConfig ProviderConfig(const std::string& agent_addr, int heartbeat_interval = 1) {
    ClientConfig config;
    config.game_id = "game-test";
    config.env = "development";
    config.service_id = "cpp-tests";
    config.agent_addr = agent_addr;
    config.timeout_seconds = 5;
    config.connect_timeout_seconds = 2;
    config.heartbeat_interval = heartbeat_interval;
    config.disable_logging = true;
    return config;
}

void RegisterSampleFunction(CroupierClient& client) {
    FunctionDescriptor desc;
    desc.id = "test.echo";
    desc.version = "1.4.2";
    desc.summary = "echo payload";
    desc.operation = "echo";
    desc.capability = "action";
    desc.risk = "safe";
    FunctionHandler handler = [](const std::string&, const std::string& payload) { return payload; };
    ASSERT_TRUE(client.RegisterFunction(desc, handler));
}

TEST(ProviderLifecycleTest, ConnectRegistersFunctionsAndReceivesSession) {
    FakeAgent agent;
    CroupierClient client(ProviderConfig(agent.address(), /*heartbeat_interval=*/60));
    RegisterSampleFunction(client);

    EXPECT_FALSE(client.IsConnected());
    ASSERT_TRUE(client.Connect());
    EXPECT_TRUE(client.IsConnected());
    ASSERT_EQ(1, agent.connect_requests());
    EXPECT_EQ("cpp-tests", agent.last_service_id());
    ASSERT_EQ(1, agent.last_function_count());
    client.Stop();
    EXPECT_FALSE(client.IsConnected());
    std::this_thread::sleep_for(std::chrono::milliseconds(150));
}

TEST(ProviderLifecycleTest, ConnectWithoutRegisteredFunctionsFails) {
    FakeAgent agent;
    CroupierClient client(ProviderConfig(agent.address()));
    EXPECT_FALSE(client.Connect());
    EXPECT_EQ(0, agent.connect_requests());
}

TEST(ProviderLifecycleTest, AgentJsonErrorFailsRegistration) {
    FakeAgent agent(AgentMode::kJsonError);
    CroupierClient client(ProviderConfig(agent.address()));
    RegisterSampleFunction(client);
    EXPECT_FALSE(client.Connect());
    EXPECT_FALSE(client.IsConnected());
}

TEST(ProviderLifecycleTest, EmptySessionIdFailsRegistration) {
    FakeAgent agent(AgentMode::kEmptySessionId);
    CroupierClient client(ProviderConfig(agent.address()));
    RegisterSampleFunction(client);
    EXPECT_FALSE(client.Connect());
    EXPECT_FALSE(client.IsConnected());
}

TEST(ProviderLifecycleTest, DoubleConnectIsIdempotent) {
    FakeAgent agent;
    CroupierClient client(ProviderConfig(agent.address(), /*heartbeat_interval=*/60));
    RegisterSampleFunction(client);
    ASSERT_TRUE(client.Connect());
    ASSERT_TRUE(client.Connect());
    EXPECT_EQ(1, agent.connect_requests());
    client.Stop();
    std::this_thread::sleep_for(std::chrono::milliseconds(150));
}

TEST(ProviderLifecycleTest, HeartbeatSendsSessionId) {
    FakeAgent agent;
    CroupierClient client(ProviderConfig(agent.address(), /*heartbeat_interval=*/1));
    RegisterSampleFunction(client);
    ASSERT_TRUE(client.Connect());

    // Wait until at least one heartbeat reached the fake agent (bounded).
    for (int i = 0; i < 100 && agent.heartbeats() < 1; ++i) {
        std::this_thread::sleep_for(std::chrono::milliseconds(50));
    }
    EXPECT_GE(agent.heartbeats(), 1);
    EXPECT_EQ("session-42", agent.last_heartbeat_session());

    client.Stop();
    std::this_thread::sleep_for(std::chrono::milliseconds(200));
}

TEST(ProviderLifecycleTest, ServeRunsUntilStop) {
    FakeAgent agent;
    CroupierClient client(ProviderConfig(agent.address(), /*heartbeat_interval=*/60));
    RegisterSampleFunction(client);

    std::thread serve_thread([&client] { client.Serve(); });
    for (int i = 0; i < 100 && !client.IsConnected(); ++i) {
        std::this_thread::sleep_for(std::chrono::milliseconds(50));
    }
    ASSERT_TRUE(client.IsConnected());

    client.Stop();
    serve_thread.join();
    EXPECT_FALSE(client.IsConnected());
    std::this_thread::sleep_for(std::chrono::milliseconds(150));
}

TEST(ProviderLifecycleTest, ReconnectsAfterAgentRestart) {
    FakeAgent agent;
    ClientConfig config = ProviderConfig(agent.address(), /*heartbeat_interval=*/1);
    config.timeout_seconds = 1;  // fail fast so heartbeat errors surface quickly
    CroupierClient client(config);
    RegisterSampleFunction(client);
    ASSERT_TRUE(client.Connect());
    EXPECT_EQ(1, agent.connect_requests());

    // Kill and immediately restart the agent on the same port. The client's
    // existing TCP session dies, so the next heartbeat fails and triggers a
    // reconnect, which can succeed because the listener is already back.
    // NOTE (recorded bug): after one failed reconnect attempt the client
    // stops retrying (Connect() failure sets should_stop_heartbeat_, which
    // terminates reconnectLoop), so a slower agent restart would never
    // recover.
    agent.stop();
    agent.restart();

    bool reconnected = false;
    for (int i = 0; i < 300 && !reconnected; ++i) {
        std::this_thread::sleep_for(std::chrono::milliseconds(50));
        reconnected = client.IsConnected() && agent.connect_requests() >= 2;
    }
    EXPECT_TRUE(reconnected);
    EXPECT_GE(agent.connect_requests(), 2);

    client.Stop();
    std::this_thread::sleep_for(std::chrono::milliseconds(200));
}

TEST(ProviderLifecycleTest, CloseClearsState) {
    FakeAgent agent;
    CroupierClient client(ProviderConfig(agent.address(), /*heartbeat_interval=*/60));
    RegisterSampleFunction(client);
    ASSERT_TRUE(client.Connect());
    client.Close();
    EXPECT_FALSE(client.IsConnected());
    std::this_thread::sleep_for(std::chrono::milliseconds(150));
}

}  // namespace
}  // namespace croupier::sdk::test
