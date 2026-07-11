#include <gtest/gtest.h>

#include "croupier/sdk/protocol.h"

#include <vector>

namespace croupier {
namespace sdk {
namespace protocol {
namespace test {

class ProtocolTest : public ::testing::Test {
protected:
    void SetUp() override {}
    void TearDown() override {}
};

// Test PutMsgID and GetMsgID
TEST_F(ProtocolTest, MsgIDEncoding) {
    uint8_t buf[3];

    // Test various MsgID values
    std::vector<uint32_t> test_ids = {
        MSG_INVOKE_REQUEST,
        MSG_INVOKE_RESPONSE,
        MSG_REGISTER_REQUEST,
        MSG_HEARTBEAT_RESPONSE,
        0x010101,
        0xFFFFFF,
        0x000001,
        0x123456
    };

    for (uint32_t original_id : test_ids) {
        PutMsgID(buf, original_id);
        uint32_t decoded_id = GetMsgID(buf);
        EXPECT_EQ(decoded_id, original_id) << "Failed for MsgID: 0x" << std::hex << original_id;
    }
}

// Test PutMsgID big-endian encoding
TEST_F(ProtocolTest, PutMsgIDBigEndian) {
    uint8_t buf[3];
    uint32_t msg_id = 0x123456;

    PutMsgID(buf, msg_id);

    EXPECT_EQ(buf[0], 0x12);
    EXPECT_EQ(buf[1], 0x34);
    EXPECT_EQ(buf[2], 0x56);
}

// Test GetMsgID big-endian decoding
TEST_F(ProtocolTest, GetMsgIDBigEndian) {
    uint8_t buf[3] = {0xAB, 0xCD, 0xEF};

    uint32_t msg_id = GetMsgID(buf);

    EXPECT_EQ(msg_id, 0xABCDEF);
}

// Test NewMessage with empty body
TEST_F(ProtocolTest, NewMessageEmptyBody) {
    std::vector<uint8_t> empty_body;
    auto message = NewMessage(MSG_INVOKE_REQUEST, 12345, empty_body);

    EXPECT_EQ(message.size(), HEADER_SIZE);
    EXPECT_EQ(message[0], VERSION_1);

    uint32_t msg_id = GetMsgID(&message[1]);
    EXPECT_EQ(msg_id, MSG_INVOKE_REQUEST);

    uint32_t req_id = (static_cast<uint32_t>(message[4]) << 24) |
                      (static_cast<uint32_t>(message[5]) << 16) |
                      (static_cast<uint32_t>(message[6]) << 8) |
                      static_cast<uint32_t>(message[7]);
    EXPECT_EQ(req_id, 12345);
}

// Test NewMessage with body
TEST_F(ProtocolTest, NewMessageWithBody) {
    std::vector<uint8_t> body = {0x01, 0x02, 0x03, 0x04, 0x05};
    auto message = NewMessage(MSG_HEARTBEAT_REQUEST, 67890, body);

    EXPECT_EQ(message.size(), HEADER_SIZE + body.size());
    EXPECT_EQ(message[0], VERSION_1);

    // Check body is copied correctly
    bool body_matches = true;
    for (size_t i = 0; i < body.size(); ++i) {
        if (message[HEADER_SIZE + i] != body[i]) {
            body_matches = false;
            break;
        }
    }
    EXPECT_TRUE(body_matches);
}

// Test ParseMessage with valid data
TEST_F(ProtocolTest, ParseMessageValid) {
    std::vector<uint8_t> body = {0xAA, 0xBB, 0xCC};
    auto message = NewMessage(MSG_INVOKE_REQUEST, 11111, body);

    ParsedMessage parsed = ParseMessage(message);

    EXPECT_EQ(parsed.version, VERSION_1);
    EXPECT_EQ(parsed.msg_id, MSG_INVOKE_REQUEST);
    EXPECT_EQ(parsed.req_id, 11111);
    EXPECT_EQ(parsed.body.size(), body.size());
    EXPECT_EQ(parsed.body[0], 0xAA);
    EXPECT_EQ(parsed.body[1], 0xBB);
    EXPECT_EQ(parsed.body[2], 0xCC);
}

// Test ParseMessage with short data
TEST_F(ProtocolTest, ParseMessageTooShort) {
    std::vector<uint8_t> short_data = {0x01, 0x02};  // Less than HEADER_SIZE

    EXPECT_THROW(ParseMessage(short_data), std::runtime_error);
}

// Test ParseMessage with exact header size
TEST_F(ProtocolTest, ParseMessageHeaderOnly) {
    std::vector<uint8_t> header_only_data(HEADER_SIZE);
    header_only_data[0] = VERSION_1;
    PutMsgID(&header_only_data[1], MSG_INVOKE_REQUEST);
    header_only_data[4] = 0x12;  // Request ID big-endian
    header_only_data[5] = 0x34;
    header_only_data[6] = 0x56;
    header_only_data[7] = 0x78;

    ParsedMessage parsed = ParseMessage(header_only_data);

    EXPECT_EQ(parsed.version, VERSION_1);
    EXPECT_EQ(parsed.msg_id, MSG_INVOKE_REQUEST);
    EXPECT_EQ(parsed.req_id, 0x12345678);
    EXPECT_TRUE(parsed.body.empty());
}

// Test IsRequest
TEST_F(ProtocolTest, IsRequest) {
    EXPECT_TRUE(IsRequest(MSG_INVOKE_REQUEST));
    EXPECT_TRUE(IsRequest(MSG_HEARTBEAT_REQUEST));
    EXPECT_TRUE(IsRequest(MSG_REGISTER_REQUEST));
    EXPECT_TRUE(IsRequest(MSG_START_TASK_REQUEST));
    EXPECT_TRUE(IsRequest(MSG_CANCEL_TASK_REQUEST));

    EXPECT_FALSE(IsRequest(MSG_INVOKE_RESPONSE));
    EXPECT_FALSE(IsRequest(MSG_HEARTBEAT_RESPONSE));
}

// Test IsResponse
TEST_F(ProtocolTest, IsResponse) {
    EXPECT_TRUE(IsResponse(MSG_INVOKE_RESPONSE));
    EXPECT_TRUE(IsResponse(MSG_HEARTBEAT_RESPONSE));
    EXPECT_TRUE(IsResponse(MSG_REGISTER_RESPONSE));

    EXPECT_FALSE(IsResponse(MSG_INVOKE_REQUEST));
    EXPECT_FALSE(IsResponse(MSG_HEARTBEAT_REQUEST));
}

// Test IsRequest with special event messages
TEST_F(ProtocolTest, IsRequestEventMessages) {
    // MSG_TASK_EVENT and MSG_METRIC_EVENT are even but not responses
    EXPECT_FALSE(IsRequest(MSG_TASK_EVENT));
    EXPECT_FALSE(IsRequest(MSG_METRIC_EVENT));
}

// Test IsResponse with special event messages
TEST_F(ProtocolTest, IsResponseEventMessages) {
    // MSG_TASK_EVENT and MSG_METRIC_EVENT are even but not responses
    EXPECT_FALSE(IsResponse(MSG_TASK_EVENT));
    EXPECT_FALSE(IsResponse(MSG_METRIC_EVENT));
}

// Test GetResponseMsgID
TEST_F(ProtocolTest, GetResponseMsgID) {
    EXPECT_EQ(GetResponseMsgID(MSG_INVOKE_REQUEST), MSG_INVOKE_RESPONSE);
    EXPECT_EQ(GetResponseMsgID(MSG_HEARTBEAT_REQUEST), MSG_HEARTBEAT_RESPONSE);
    EXPECT_EQ(GetResponseMsgID(MSG_REGISTER_REQUEST), MSG_REGISTER_RESPONSE);
    EXPECT_EQ(GetResponseMsgID(MSG_START_TASK_REQUEST), MSG_START_TASK_RESPONSE);
}

// Test MsgIDString for known messages
TEST_F(ProtocolTest, MsgIDStringKnown) {
    EXPECT_EQ(MsgIDString(MSG_INVOKE_REQUEST), "InvokeRequest");
    EXPECT_EQ(MsgIDString(MSG_INVOKE_RESPONSE), "InvokeResponse");
    EXPECT_EQ(MsgIDString(MSG_HEARTBEAT_REQUEST), "HeartbeatRequest");
    EXPECT_EQ(MsgIDString(MSG_HEARTBEAT_RESPONSE), "HeartbeatResponse");
    EXPECT_EQ(MsgIDString(MSG_REGISTER_REQUEST), "RegisterRequest");
    EXPECT_EQ(MsgIDString(MSG_REGISTER_RESPONSE), "RegisterResponse");
    EXPECT_EQ(MsgIDString(MSG_START_TASK_REQUEST), "StartTaskRequest");
    EXPECT_EQ(MsgIDString(MSG_TASK_EVENT), "TaskEvent");
    EXPECT_EQ(MsgIDString(MSG_PROVIDER_CONNECT_REQUEST), "ProviderConnectRequest");
}

// Test MsgIDString for unknown messages
TEST_F(ProtocolTest, MsgIDStringUnknown) {
    std::string unknown = MsgIDString(0xDEADBEEF);
    EXPECT_NE(unknown.find("Unknown"), std::string::npos);
    EXPECT_NE(unknown.find("0xDEADBEEF"), std::string::npos);
}

// Test request ID big-endian encoding in NewMessage
TEST_F(ProtocolTest, NewMessageRequestIDBigEndian) {
    auto message = NewMessage(MSG_INVOKE_REQUEST, 0x12345678, {});

    EXPECT_EQ(message[4], 0x12);
    EXPECT_EQ(message[5], 0x34);
    EXPECT_EQ(message[6], 0x56);
    EXPECT_EQ(message[7], 0x78);
}

// Test request ID big-endian decoding in ParseMessage
TEST_F(ProtocolTest, ParseMessageRequestIDBigEndian) {
    std::vector<uint8_t> data(HEADER_SIZE);
    data[0] = VERSION_1;
    PutMsgID(&data[1], MSG_INVOKE_REQUEST);
    data[4] = 0xAB;
    data[5] = 0xCD;
    data[6] = 0xEF;
    data[7] = 0x01;

    ParsedMessage parsed = ParseMessage(data);

    EXPECT_EQ(parsed.req_id, 0xABCDEF01);
}

// Test message round-trip
TEST_F(ProtocolTest, MessageRoundTrip) {
    std::vector<uint8_t> original_body = {0xDE, 0xAD, 0xBE, 0xEF};
    uint32_t original_msg_id = MSG_INVOKE_REQUEST;
    uint32_t original_req_id = 0x12345678;

    auto message = NewMessage(original_msg_id, original_req_id, original_body);
    ParsedMessage parsed = ParseMessage(message);

    EXPECT_EQ(parsed.msg_id, original_msg_id);
    EXPECT_EQ(parsed.req_id, original_req_id);
    EXPECT_EQ(parsed.body, original_body);
}

// Test MSG_TASK_EVENT and MSG_METRIC_EVENT special handling
TEST_F(ProtocolTest, EventMessagesSpecialCase) {
    // These are event messages, not requests or responses
    EXPECT_FALSE(IsRequest(MSG_TASK_EVENT));
    EXPECT_FALSE(IsResponse(MSG_TASK_EVENT));

    EXPECT_FALSE(IsRequest(MSG_METRIC_EVENT));
    EXPECT_FALSE(IsResponse(MSG_METRIC_EVENT));
}

// Test ControlService message IDs
TEST_F(ProtocolTest, ControlServiceMessageIDs) {
    EXPECT_EQ(GetResponseMsgID(MSG_REGISTER_REQUEST), MSG_REGISTER_RESPONSE);
    EXPECT_EQ(GetResponseMsgID(MSG_HEARTBEAT_REQUEST), MSG_HEARTBEAT_RESPONSE);
    EXPECT_EQ(GetResponseMsgID(MSG_REGISTER_CAPABILITIES_REQ), MSG_REGISTER_CAPABILITIES_RESP);
}

// Test ClientService message IDs
TEST_F(ProtocolTest, ClientServiceMessageIDs) {
    EXPECT_EQ(GetResponseMsgID(MSG_REGISTER_CLIENT_REQUEST), MSG_REGISTER_CLIENT_RESPONSE);
    EXPECT_EQ(GetResponseMsgID(MSG_CLIENT_HEARTBEAT_REQUEST), MSG_CLIENT_HEARTBEAT_RESPONSE);
    EXPECT_EQ(GetResponseMsgID(MSG_LIST_CLIENTS_REQUEST), MSG_LIST_CLIENTS_RESPONSE);
    EXPECT_EQ(GetResponseMsgID(MSG_GET_TASK_RESULT_REQUEST), MSG_GET_TASK_RESULT_RESPONSE);
}

// Test InvokerService message IDs
TEST_F(ProtocolTest, InvokerServiceMessageIDs) {
    EXPECT_EQ(GetResponseMsgID(MSG_INVOKE_REQUEST), MSG_INVOKE_RESPONSE);
    EXPECT_EQ(GetResponseMsgID(MSG_START_TASK_REQUEST), MSG_START_TASK_RESPONSE);
    EXPECT_EQ(GetResponseMsgID(MSG_CANCEL_TASK_REQUEST), MSG_CANCEL_TASK_RESPONSE);
}

// Test OpsService message IDs
TEST_F(ProtocolTest, OpsServiceMessageIDs) {
    EXPECT_EQ(GetResponseMsgID(MSG_GET_SYSTEM_INFO_REQUEST), MSG_GET_SYSTEM_INFO_RESPONSE);
    EXPECT_EQ(GetResponseMsgID(MSG_LIST_PROCESSES_REQUEST), MSG_LIST_PROCESSES_RESPONSE);
    EXPECT_EQ(GetResponseMsgID(MSG_REPORT_METRICS_REQUEST), MSG_REPORT_METRICS_RESPONSE);
    EXPECT_EQ(GetResponseMsgID(MSG_RESTART_PROCESS_REQUEST), MSG_RESTART_PROCESS_RESPONSE);
    EXPECT_EQ(GetResponseMsgID(MSG_STOP_PROCESS_REQUEST), MSG_STOP_PROCESS_RESPONSE);
    EXPECT_EQ(GetResponseMsgID(MSG_START_PROCESS_REQUEST), MSG_START_PROCESS_RESPONSE);
    EXPECT_EQ(GetResponseMsgID(MSG_EXECUTE_COMMAND_REQUEST), MSG_EXECUTE_COMMAND_RESPONSE);
    EXPECT_EQ(GetResponseMsgID(MSG_LIST_SERVICES_REQUEST), MSG_LIST_SERVICES_RESPONSE);
    EXPECT_EQ(GetResponseMsgID(MSG_GET_SERVICE_STATUS_REQUEST), MSG_GET_SERVICE_STATUS_RESPONSE);
}

// Test ProviderSession message IDs
TEST_F(ProtocolTest, ProviderSessionMessageIDs) {
    EXPECT_EQ(GetResponseMsgID(MSG_PROVIDER_CONNECT_REQUEST), MSG_PROVIDER_CONNECT_RESPONSE);
    EXPECT_EQ(GetResponseMsgID(MSG_PROVIDER_HEARTBEAT_REQUEST), MSG_PROVIDER_HEARTBEAT_RESPONSE);
    EXPECT_EQ(GetResponseMsgID(MSG_PROVIDER_DRAIN_REQUEST), MSG_PROVIDER_DRAIN_RESPONSE);
}

// Test HeaderSize constant
TEST_F(ProtocolTest, HeaderSizeConstant) {
    // Version(1) + MsgID(3) + RequestID(4) = 8
    EXPECT_EQ(HEADER_SIZE, 8);
}

// Test VERSION_1 constant
TEST_F(ProtocolTest, VersionConstant) {
    EXPECT_EQ(VERSION_1, 0x01);
}

// Test NewMessage with zero request ID
TEST_F(ProtocolTest, NewMessageZeroRequestID) {
    auto message = NewMessage(MSG_INVOKE_REQUEST, 0, {});

    uint32_t req_id = (static_cast<uint32_t>(message[4]) << 24) |
                      (static_cast<uint32_t>(message[5]) << 16) |
                      (static_cast<uint32_t>(message[6]) << 8) |
                      static_cast<uint32_t>(message[7]);
    EXPECT_EQ(req_id, 0);
}

// Test NewMessage with maximum request ID
TEST_F(ProtocolTest, NewMessageMaxRequestID) {
    uint32_t max_req_id = 0xFFFFFFFF;
    auto message = NewMessage(MSG_INVOKE_REQUEST, max_req_id, {});

    uint32_t req_id = (static_cast<uint32_t>(message[4]) << 24) |
                      (static_cast<uint32_t>(message[5]) << 16) |
                      (static_cast<uint32_t>(message[6]) << 8) |
                      static_cast<uint32_t>(message[7]);
    EXPECT_EQ(req_id, max_req_id);
}

// Test ParseMessage with large body
TEST_F(ProtocolTest, ParseMessageLargeBody) {
    std::vector<uint8_t> large_body(10000, 0xAB);
    auto message = NewMessage(MSG_INVOKE_REQUEST, 123, large_body);

    ParsedMessage parsed = ParseMessage(message);

    EXPECT_EQ(parsed.body.size(), 10000);
    EXPECT_TRUE(std::all_of(parsed.body.begin(), parsed.body.end(),
                            [](uint8_t b) { return b == 0xAB; }));
}

// Test MsgIDString for all known messages
TEST_F(ProtocolTest, MsgIDStringAllKnown) {
    // Test that all known messages return non-empty strings
    std::vector<uint32_t> known_msgs = {
        MSG_REGISTER_REQUEST, MSG_REGISTER_RESPONSE,
        MSG_HEARTBEAT_REQUEST, MSG_HEARTBEAT_RESPONSE,
        MSG_REGISTER_CAPABILITIES_REQ, MSG_REGISTER_CAPABILITIES_RESP,
        MSG_REGISTER_CLIENT_REQUEST, MSG_REGISTER_CLIENT_RESPONSE,
        MSG_CLIENT_HEARTBEAT_REQUEST, MSG_CLIENT_HEARTBEAT_RESPONSE,
        MSG_LIST_CLIENTS_REQUEST, MSG_LIST_CLIENTS_RESPONSE,
        MSG_GET_TASK_RESULT_REQUEST, MSG_GET_TASK_RESULT_RESPONSE,
        MSG_INVOKE_REQUEST, MSG_INVOKE_RESPONSE,
        MSG_START_TASK_REQUEST, MSG_START_TASK_RESPONSE,
        MSG_STREAM_TASK_REQUEST, MSG_TASK_EVENT,
        MSG_CANCEL_TASK_REQUEST, MSG_CANCEL_TASK_RESPONSE,
        MSG_GET_SYSTEM_INFO_REQUEST, MSG_GET_SYSTEM_INFO_RESPONSE,
        MSG_LIST_PROCESSES_REQUEST, MSG_LIST_PROCESSES_RESPONSE,
        MSG_REPORT_METRICS_REQUEST, MSG_REPORT_METRICS_RESPONSE,
        MSG_STREAM_METRICS_REQUEST, MSG_METRIC_EVENT,
        MSG_RESTART_PROCESS_REQUEST, MSG_RESTART_PROCESS_RESPONSE,
        MSG_STOP_PROCESS_REQUEST, MSG_STOP_PROCESS_RESPONSE,
        MSG_START_PROCESS_REQUEST, MSG_START_PROCESS_RESPONSE,
        MSG_EXECUTE_COMMAND_REQUEST, MSG_EXECUTE_COMMAND_RESPONSE,
        MSG_LIST_SERVICES_REQUEST, MSG_LIST_SERVICES_RESPONSE,
        MSG_GET_SERVICE_STATUS_REQUEST, MSG_GET_SERVICE_STATUS_RESPONSE,
        MSG_PROVIDER_CONNECT_REQUEST, MSG_PROVIDER_CONNECT_RESPONSE,
        MSG_PROVIDER_HEARTBEAT_REQUEST, MSG_PROVIDER_HEARTBEAT_RESPONSE,
        MSG_PROVIDER_DRAIN_REQUEST, MSG_PROVIDER_DRAIN_RESPONSE
    };

    for (uint32_t msg_id : known_msgs) {
        std::string str = MsgIDString(msg_id);
        EXPECT_FALSE(str.empty()) << "Empty string for MsgID: 0x" << std::hex << msg_id;
        EXPECT_EQ(str.find("Unknown"), std::string::npos) << "Unknown string for MsgID: 0x" << std::hex << msg_id;
    }
}

// Test odd/even MsgID pattern
TEST_F(ProtocolTest, OddEvenMsgIDPattern) {
    // All requests should have odd MsgID
    EXPECT_TRUE(MSG_INVOKE_REQUEST % 2 == 1);
    EXPECT_TRUE(MSG_HEARTBEAT_REQUEST % 2 == 1);
    EXPECT_TRUE(MSG_REGISTER_REQUEST % 2 == 1);

    // All responses should have even MsgID
    EXPECT_TRUE(MSG_INVOKE_RESPONSE % 2 == 0);
    EXPECT_TRUE(MSG_HEARTBEAT_RESPONSE % 2 == 0);
    EXPECT_TRUE(MSG_REGISTER_RESPONSE % 2 == 0);
}

}  // namespace test
}  // namespace protocol
}  // namespace sdk
}  // namespace croupier
