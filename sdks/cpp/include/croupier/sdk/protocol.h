/**
 * @file protocol.h
 * @brief Croupier wire protocol implementation for TCP transport.
 *
 * Message Format:
 *   Header (8 bytes):
 *     ┌─────────┬──────────┬─────────────────┐
 *     │ Version │ MsgID    │ RequestID       │
 *     │ (1B)    │ (3B)     │ (4B)            │
 *     └─────────┴──────────┴─────────────────┘
 *   Body: protobuf serialized message
 *
 * Request messages have odd MsgID, Response messages have even MsgID.
 */

#ifndef CROUPIER_SDK_PROTOCOL_H
#define CROUPIER_SDK_PROTOCOL_H

#include <cstdint>
#include <cstring>
#include <iomanip>
#include <sstream>
#include <string>
#include <vector>

namespace croupier {
namespace sdk {
namespace protocol {

// Protocol version
constexpr uint8_t VERSION_1 = 0x01;

// Header size: Version(1) + MsgID(3) + RequestID(4)
constexpr size_t HEADER_SIZE = 8;

// Message type constants (24 bits)
// ControlService (0x01xx)
constexpr uint32_t MSG_REGISTER_REQUEST = 0x010101;
constexpr uint32_t MSG_REGISTER_RESPONSE = 0x010102;
constexpr uint32_t MSG_HEARTBEAT_REQUEST = 0x010103;
constexpr uint32_t MSG_HEARTBEAT_RESPONSE = 0x010104;
constexpr uint32_t MSG_REGISTER_CAPABILITIES_REQ = 0x010105;
constexpr uint32_t MSG_REGISTER_CAPABILITIES_RESP = 0x010106;

// ClientService (0x02xx)
constexpr uint32_t MSG_REGISTER_CLIENT_REQUEST = 0x020101;
constexpr uint32_t MSG_REGISTER_CLIENT_RESPONSE = 0x020102;
constexpr uint32_t MSG_CLIENT_HEARTBEAT_REQUEST = 0x020103;
constexpr uint32_t MSG_CLIENT_HEARTBEAT_RESPONSE = 0x020104;
constexpr uint32_t MSG_LIST_CLIENTS_REQUEST = 0x020105;
constexpr uint32_t MSG_LIST_CLIENTS_RESPONSE = 0x020106;

// InvokerService (0x03xx)
constexpr uint32_t MSG_INVOKE_REQUEST = 0x030101;
constexpr uint32_t MSG_INVOKE_RESPONSE = 0x030102;
constexpr uint32_t MSG_START_TASK_REQUEST = 0x030103;
constexpr uint32_t MSG_START_TASK_RESPONSE = 0x030104;
constexpr uint32_t MSG_STREAM_TASK_REQUEST = 0x030105;
constexpr uint32_t MSG_TASK_EVENT = 0x030106;
constexpr uint32_t MSG_CANCEL_TASK_REQUEST = 0x030107;
constexpr uint32_t MSG_CANCEL_TASK_RESPONSE = 0x030108;

// OpsService (0x04xx)
constexpr uint32_t MSG_GET_SYSTEM_INFO_REQUEST = 0x040101;
constexpr uint32_t MSG_GET_SYSTEM_INFO_RESPONSE = 0x040102;
constexpr uint32_t MSG_LIST_PROCESSES_REQUEST = 0x040103;
constexpr uint32_t MSG_LIST_PROCESSES_RESPONSE = 0x040104;
constexpr uint32_t MSG_REPORT_METRICS_REQUEST = 0x040105;
constexpr uint32_t MSG_REPORT_METRICS_RESPONSE = 0x040106;
constexpr uint32_t MSG_STREAM_METRICS_REQUEST = 0x040107;
constexpr uint32_t MSG_METRIC_EVENT = 0x040108;
constexpr uint32_t MSG_RESTART_PROCESS_REQUEST = 0x040109;
constexpr uint32_t MSG_RESTART_PROCESS_RESPONSE = 0x04010A;
constexpr uint32_t MSG_STOP_PROCESS_REQUEST = 0x04010B;
constexpr uint32_t MSG_STOP_PROCESS_RESPONSE = 0x04010C;
constexpr uint32_t MSG_START_PROCESS_REQUEST = 0x04010D;
constexpr uint32_t MSG_START_PROCESS_RESPONSE = 0x04010E;
constexpr uint32_t MSG_EXECUTE_COMMAND_REQUEST = 0x04010F;
constexpr uint32_t MSG_EXECUTE_COMMAND_RESPONSE = 0x040110;
constexpr uint32_t MSG_LIST_SERVICES_REQUEST = 0x040111;
constexpr uint32_t MSG_LIST_SERVICES_RESPONSE = 0x040112;
constexpr uint32_t MSG_GET_SERVICE_STATUS_REQUEST = 0x040113;
constexpr uint32_t MSG_GET_SERVICE_STATUS_RESPONSE = 0x040114;

// ProviderSession (0x05xx) - SDK to Agent provider session
constexpr uint32_t MSG_PROVIDER_CONNECT_REQUEST = 0x050101;
constexpr uint32_t MSG_PROVIDER_CONNECT_RESPONSE = 0x050102;
constexpr uint32_t MSG_PROVIDER_HEARTBEAT_REQUEST = 0x050103;
constexpr uint32_t MSG_PROVIDER_HEARTBEAT_RESPONSE = 0x050104;
constexpr uint32_t MSG_PROVIDER_DRAIN_REQUEST = 0x050105;
constexpr uint32_t MSG_PROVIDER_DRAIN_RESPONSE = 0x050106;

/**
 * Encode a 24-bit MsgID into 3 bytes (big-endian).
 */
inline void PutMsgID(uint8_t* buf, uint32_t msg_id) {
    buf[0] = (msg_id >> 16) & 0xFF;
    buf[1] = (msg_id >> 8) & 0xFF;
    buf[2] = msg_id & 0xFF;
}

/**
 * Decode a 24-bit MsgID from 3 bytes (big-endian).
 */
inline uint32_t GetMsgID(const uint8_t* buf) {
    return (static_cast<uint32_t>(buf[0]) << 16) |
           (static_cast<uint32_t>(buf[1]) << 8) |
           static_cast<uint32_t>(buf[2]);
}

/**
 * Create a new message with protocol header and body.
 */
inline std::vector<uint8_t> NewMessage(uint32_t msg_id, uint32_t req_id,
                                        const std::vector<uint8_t>& body) {
    std::vector<uint8_t> message(HEADER_SIZE + body.size());

    // Header
    message[0] = VERSION_1;
    PutMsgID(&message[1], msg_id);

    // RequestID (big-endian)
    message[4] = (req_id >> 24) & 0xFF;
    message[5] = (req_id >> 16) & 0xFF;
    message[6] = (req_id >> 8) & 0xFF;
    message[7] = req_id & 0xFF;

    // Body
    if (!body.empty()) {
        std::memcpy(&message[HEADER_SIZE], body.data(), body.size());
    }

    return message;
}

/**
 * Parsed message components.
 */
struct ParsedMessage {
    uint8_t version;
    uint32_t msg_id;
    uint32_t req_id;
    std::vector<uint8_t> body;
};

/**
 * Parse a received message.
 */
inline ParsedMessage ParseMessage(const std::vector<uint8_t>& data) {
    ParsedMessage result;

    if (data.size() < HEADER_SIZE) {
        throw std::runtime_error("Message too short");
    }

    result.version = data[0];
    result.msg_id = GetMsgID(&data[1]);
    result.req_id = (static_cast<uint32_t>(data[4]) << 24) |
                    (static_cast<uint32_t>(data[5]) << 16) |
                    (static_cast<uint32_t>(data[6]) << 8) |
                    static_cast<uint32_t>(data[7]);

    result.body.assign(data.begin() + HEADER_SIZE, data.end());

    return result;
}

/**
 * Check if the MsgID indicates a request message.
 */
inline bool IsRequest(uint32_t msg_id) {
    return msg_id % 2 == 1 && msg_id != MSG_TASK_EVENT && msg_id != MSG_METRIC_EVENT;
}

/**
 * Check if the MsgID indicates a response message.
 */
inline bool IsResponse(uint32_t msg_id) {
    return msg_id % 2 == 0 && msg_id != MSG_TASK_EVENT && msg_id != MSG_METRIC_EVENT;
}

/**
 * Get the response MsgID for a given request MsgID.
 */
inline uint32_t GetResponseMsgID(uint32_t req_msg_id) {
    return req_msg_id + 1;
}

/**
 * Get human-readable string for MsgID.
 */
inline std::string MsgIDString(uint32_t msg_id) {
    switch (msg_id) {
        case MSG_REGISTER_REQUEST: return "RegisterRequest";
        case MSG_REGISTER_RESPONSE: return "RegisterResponse";
        case MSG_HEARTBEAT_REQUEST: return "HeartbeatRequest";
        case MSG_HEARTBEAT_RESPONSE: return "HeartbeatResponse";
        case MSG_REGISTER_CAPABILITIES_REQ: return "RegisterCapabilitiesRequest";
        case MSG_REGISTER_CAPABILITIES_RESP: return "RegisterCapabilitiesResponse";
        case MSG_REGISTER_CLIENT_REQUEST: return "RegisterClientRequest";
        case MSG_REGISTER_CLIENT_RESPONSE: return "RegisterClientResponse";
        case MSG_CLIENT_HEARTBEAT_REQUEST: return "ClientHeartbeatRequest";
        case MSG_CLIENT_HEARTBEAT_RESPONSE: return "ClientHeartbeatResponse";
        case MSG_LIST_CLIENTS_REQUEST: return "ListClientsRequest";
        case MSG_LIST_CLIENTS_RESPONSE: return "ListClientsResponse";
        case MSG_INVOKE_REQUEST: return "InvokeRequest";
        case MSG_INVOKE_RESPONSE: return "InvokeResponse";
        case MSG_START_TASK_REQUEST: return "StartTaskRequest";
        case MSG_START_TASK_RESPONSE: return "StartTaskResponse";
        case MSG_STREAM_TASK_REQUEST: return "StreamTaskRequest";
        case MSG_TASK_EVENT: return "TaskEvent";
        case MSG_CANCEL_TASK_REQUEST: return "CancelTaskRequest";
        case MSG_CANCEL_TASK_RESPONSE: return "CancelTaskResponse";
        case MSG_GET_SYSTEM_INFO_REQUEST: return "GetSystemInfoRequest";
        case MSG_GET_SYSTEM_INFO_RESPONSE: return "GetSystemInfoResponse";
        case MSG_LIST_PROCESSES_REQUEST: return "ListProcessesRequest";
        case MSG_LIST_PROCESSES_RESPONSE: return "ListProcessesResponse";
        case MSG_REPORT_METRICS_REQUEST: return "ReportMetricsRequest";
        case MSG_REPORT_METRICS_RESPONSE: return "ReportMetricsResponse";
        case MSG_STREAM_METRICS_REQUEST: return "StreamMetricsRequest";
        case MSG_METRIC_EVENT: return "MetricEvent";
        case MSG_RESTART_PROCESS_REQUEST: return "RestartProcessRequest";
        case MSG_RESTART_PROCESS_RESPONSE: return "RestartProcessResponse";
        case MSG_STOP_PROCESS_REQUEST: return "StopProcessRequest";
        case MSG_STOP_PROCESS_RESPONSE: return "StopProcessResponse";
        case MSG_START_PROCESS_REQUEST: return "StartProcessRequest";
        case MSG_START_PROCESS_RESPONSE: return "StartProcessResponse";
        case MSG_EXECUTE_COMMAND_REQUEST: return "ExecuteCommandRequest";
        case MSG_EXECUTE_COMMAND_RESPONSE: return "ExecuteCommandResponse";
        case MSG_LIST_SERVICES_REQUEST: return "ListServicesRequest";
        case MSG_LIST_SERVICES_RESPONSE: return "ListServicesResponse";
        case MSG_GET_SERVICE_STATUS_REQUEST: return "GetServiceStatusRequest";
        case MSG_GET_SERVICE_STATUS_RESPONSE: return "GetServiceStatusResponse";
        case MSG_PROVIDER_CONNECT_REQUEST: return "ProviderConnectRequest";
        case MSG_PROVIDER_CONNECT_RESPONSE: return "ProviderConnectResponse";
        case MSG_PROVIDER_HEARTBEAT_REQUEST: return "ProviderHeartbeatRequest";
        case MSG_PROVIDER_HEARTBEAT_RESPONSE: return "ProviderHeartbeatResponse";
        case MSG_PROVIDER_DRAIN_REQUEST: return "ProviderDrainRequest";
        case MSG_PROVIDER_DRAIN_RESPONSE: return "ProviderDrainResponse";
        default: {
            std::ostringstream stream;
            stream << "Unknown(0x" << std::uppercase << std::hex << msg_id << ")";
            return stream.str();
        }
    }
}

}  // namespace protocol
}  // namespace sdk
}  // namespace croupier

#endif  // CROUPIER_SDK_PROTOCOL_H
