"""
Croupier wire protocol implementation.

Message Format:
    Header (8 bytes):
      ┌─────────┬──────────┬─────────────────┐
      │ Version │ MsgID    │ RequestID       │
      │ (1B)    │ (3B)     │ (4B)            │
      └─────────┴──────────┴─────────────────┘
    Body: protobuf serialized message

Request messages have odd MsgID, Response messages have even MsgID.
"""

import struct
from typing import Tuple

# Protocol version
VERSION_1 = 0x01
HEADER_SIZE = 8  # Version(1) + MsgID(3) + RequestID(4)

# Message type constants (24 bits)
# ControlService (0x01xx)
MSG_REGISTER_REQUEST = 0x010101
MSG_REGISTER_RESPONSE = 0x010102
MSG_HEARTBEAT_REQUEST = 0x010103
MSG_HEARTBEAT_RESPONSE = 0x010104
MSG_REGISTER_CAPABILITIES_REQ = 0x010105
MSG_REGISTER_CAPABILITIES_RESP = 0x010106

# F：文件下发原语（hotpatch P1 传输层）
MSG_PROVIDER_FILE_PUSH_REQ = 0x050109
MSG_PROVIDER_FILE_PUSH_RESP = 0x05010A

# ClientService (0x02xx)
MSG_REGISTER_CLIENT_REQUEST = 0x020101
MSG_REGISTER_CLIENT_RESPONSE = 0x020102
MSG_CLIENT_HEARTBEAT_REQUEST = 0x020103
MSG_CLIENT_HEARTBEAT_RESPONSE = 0x020104
MSG_LIST_CLIENTS_REQUEST = 0x020105
MSG_LIST_CLIENTS_RESPONSE = 0x020106
MSG_GET_TASK_RESULT_REQUEST = 0x020107
MSG_GET_TASK_RESULT_RESPONSE = 0x020108

# Invocation / Task service (0x03xx)
MSG_INVOKE_REQUEST = 0x030101
MSG_INVOKE_RESPONSE = 0x030102
MSG_START_TASK_REQUEST = 0x030103
MSG_START_TASK_RESPONSE = 0x030104
MSG_STREAM_TASK_REQUEST = 0x030105
MSG_TASK_EVENT = 0x030106
MSG_CANCEL_TASK_REQUEST = 0x030107
MSG_CANCEL_TASK_RESPONSE = 0x030108

# OpsService (0x04xx)
MSG_GET_SYSTEM_INFO_REQUEST = 0x040101
MSG_GET_SYSTEM_INFO_RESPONSE = 0x040102
MSG_LIST_PROCESSES_REQUEST = 0x040103
MSG_LIST_PROCESSES_RESPONSE = 0x040104
MSG_REPORT_METRICS_REQUEST = 0x040105
MSG_REPORT_METRICS_RESPONSE = 0x040106
MSG_STREAM_METRICS_REQUEST = 0x040107
MSG_METRIC_EVENT = 0x040108
MSG_RESTART_PROCESS_REQUEST = 0x040109
MSG_RESTART_PROCESS_RESPONSE = 0x04010A
MSG_STOP_PROCESS_REQUEST = 0x04010B
MSG_STOP_PROCESS_RESPONSE = 0x04010C
MSG_START_PROCESS_REQUEST = 0x04010D
MSG_START_PROCESS_RESPONSE = 0x04010E
MSG_EXECUTE_COMMAND_REQUEST = 0x04010F
MSG_EXECUTE_COMMAND_RESPONSE = 0x040110
MSG_LIST_SERVICES_REQUEST = 0x040111
MSG_LIST_SERVICES_RESPONSE = 0x040112
MSG_GET_SERVICE_STATUS_REQUEST = 0x040113
MSG_GET_SERVICE_STATUS_RESPONSE = 0x040114

# Provider session control (0x05xx) — sdk-agent subprotocol
# Canonical names follow docs/architecture/sdk-wire-protocol.md.
MSG_PROVIDER_CONNECT_REQUEST = 0x050101
MSG_PROVIDER_CONNECT_RESPONSE = 0x050102
MSG_PROVIDER_HEARTBEAT_REQUEST = 0x050103
MSG_PROVIDER_HEARTBEAT_RESPONSE = 0x050104
MSG_PROVIDER_DRAIN_REQUEST = 0x050105
MSG_PROVIDER_DRAIN_RESPONSE = 0x050106
MSG_PROVIDER_DRAIN_COMPLETE_REQUEST = 0x050107
MSG_PROVIDER_DRAIN_COMPLETE_RESPONSE = 0x050108

# Message type names
MSG_NAMES = {
    MSG_REGISTER_REQUEST: "RegisterRequest",
    MSG_REGISTER_RESPONSE: "RegisterResponse",
    MSG_HEARTBEAT_REQUEST: "HeartbeatRequest",
    MSG_HEARTBEAT_RESPONSE: "HeartbeatResponse",
    MSG_REGISTER_CAPABILITIES_REQ: "RegisterCapabilitiesRequest",
    MSG_REGISTER_CAPABILITIES_RESP: "RegisterCapabilitiesResponse",
    MSG_REGISTER_CLIENT_REQUEST: "RegisterClientRequest",
    MSG_REGISTER_CLIENT_RESPONSE: "RegisterClientResponse",
    MSG_CLIENT_HEARTBEAT_REQUEST: "ClientHeartbeatRequest",
    MSG_CLIENT_HEARTBEAT_RESPONSE: "ClientHeartbeatResponse",
    MSG_LIST_CLIENTS_REQUEST: "ListClientsRequest",
    MSG_LIST_CLIENTS_RESPONSE: "ListClientsResponse",
    MSG_GET_TASK_RESULT_REQUEST: "GetTaskResultRequest",
    MSG_GET_TASK_RESULT_RESPONSE: "GetTaskResultResponse",
    MSG_INVOKE_REQUEST: "InvokeRequest",
    MSG_INVOKE_RESPONSE: "InvokeResponse",
    MSG_START_TASK_REQUEST: "StartTaskRequest",
    MSG_START_TASK_RESPONSE: "StartTaskResponse",
    MSG_STREAM_TASK_REQUEST: "StreamTaskRequest",
    MSG_TASK_EVENT: "TaskEvent",
    MSG_CANCEL_TASK_REQUEST: "CancelTaskRequest",
    MSG_CANCEL_TASK_RESPONSE: "CancelTaskResponse",
    MSG_GET_SYSTEM_INFO_REQUEST: "GetSystemInfoRequest",
    MSG_GET_SYSTEM_INFO_RESPONSE: "GetSystemInfoResponse",
    MSG_LIST_PROCESSES_REQUEST: "ListProcessesRequest",
    MSG_LIST_PROCESSES_RESPONSE: "ListProcessesResponse",
    MSG_REPORT_METRICS_REQUEST: "ReportMetricsRequest",
    MSG_REPORT_METRICS_RESPONSE: "ReportMetricsResponse",
    MSG_STREAM_METRICS_REQUEST: "StreamMetricsRequest",
    MSG_METRIC_EVENT: "MetricEvent",
    MSG_RESTART_PROCESS_REQUEST: "RestartProcessRequest",
    MSG_RESTART_PROCESS_RESPONSE: "RestartProcessResponse",
    MSG_STOP_PROCESS_REQUEST: "StopProcessRequest",
    MSG_STOP_PROCESS_RESPONSE: "StopProcessResponse",
    MSG_START_PROCESS_REQUEST: "StartProcessRequest",
    MSG_START_PROCESS_RESPONSE: "StartProcessResponse",
    MSG_EXECUTE_COMMAND_REQUEST: "ExecuteCommandRequest",
    MSG_EXECUTE_COMMAND_RESPONSE: "ExecuteCommandResponse",
    MSG_LIST_SERVICES_REQUEST: "ListServicesRequest",
    MSG_LIST_SERVICES_RESPONSE: "ListServicesResponse",
    MSG_GET_SERVICE_STATUS_REQUEST: "GetServiceStatusRequest",
    MSG_GET_SERVICE_STATUS_RESPONSE: "GetServiceStatusResponse",
    MSG_PROVIDER_CONNECT_REQUEST: "ProviderConnectRequest",
    MSG_PROVIDER_CONNECT_RESPONSE: "ProviderConnectResponse",
    MSG_PROVIDER_HEARTBEAT_REQUEST: "ProviderHeartbeatRequest",
    MSG_PROVIDER_HEARTBEAT_RESPONSE: "ProviderHeartbeatResponse",
    MSG_PROVIDER_DRAIN_REQUEST: "ProviderDrainRequest",
    MSG_PROVIDER_DRAIN_RESPONSE: "ProviderDrainResponse",
    MSG_PROVIDER_DRAIN_COMPLETE_REQUEST: "ProviderDrainCompleteRequest",
    MSG_PROVIDER_DRAIN_COMPLETE_RESPONSE: "ProviderDrainCompleteResponse",
    MSG_PROVIDER_DRAIN_REQUEST: "ProviderDrainRequest",
    MSG_PROVIDER_DRAIN_RESPONSE: "ProviderDrainResponse",
    MSG_PROVIDER_DRAIN_COMPLETE_REQUEST: "ProviderDrainCompleteRequest",
    MSG_PROVIDER_DRAIN_COMPLETE_RESPONSE: "ProviderDrainCompleteResponse",
}


def put_msg_id(msg_id: int) -> bytes:
    """Encode a 24-bit MsgID into 3 bytes (big-endian)."""
    return bytes(
        [
            (msg_id >> 16) & 0xFF,
            (msg_id >> 8) & 0xFF,
            msg_id & 0xFF,
        ]
    )


def get_msg_id(data: bytes) -> int:
    """Decode a 24-bit MsgID from 3 bytes (big-endian)."""
    return (data[0] << 16) | (data[1] << 8) | data[2]


def new_message(msg_id: int, req_id: int, body: bytes) -> bytes:
    """Create a new message with protocol header and body."""
    header = bytes([VERSION_1]) + put_msg_id(msg_id) + struct.pack(">I", req_id)
    return header + body


def parse_message(data: bytes) -> Tuple[int, int, int, bytes]:
    """Parse a received message.

    Returns:
        Tuple of (version, msg_id, req_id, body)
    """
    if len(data) < HEADER_SIZE:
        raise ValueError(f"Message too short: {len(data)} < {HEADER_SIZE}")

    version = data[0]
    msg_id = get_msg_id(data[1:4])
    req_id = struct.unpack(">I", data[4:8])[0]
    body = data[HEADER_SIZE:]

    return version, msg_id, req_id, body


def is_request(msg_id: int) -> bool:
    """Check if the MsgID indicates a request message."""
    return msg_id % 2 == 1 and msg_id not in (MSG_TASK_EVENT, MSG_METRIC_EVENT)


def is_response(msg_id: int) -> bool:
    """Check if the MsgID indicates a response message."""
    return msg_id % 2 == 0 and msg_id not in (MSG_TASK_EVENT, MSG_METRIC_EVENT)


# 会话/传输控制消息集合（对齐 Go pkg/protocol.IsControlRequest）：
# 心跳、注册、drain 走独立派发车道，业务洪峰打满业务队列时控制面
# 依然可达——否则心跳被 fail-fast 拒绝 → 对端判定会话死亡 → 过载
# 升级为连接雪崩（见 docs/architecture/sdk-wire-protocol.md 双车道）。
CONTROL_REQUESTS = frozenset(
    {
        MSG_REGISTER_REQUEST,
        MSG_HEARTBEAT_REQUEST,
        MSG_REGISTER_CAPABILITIES_REQ,
        MSG_REGISTER_CLIENT_REQUEST,
        MSG_CLIENT_HEARTBEAT_REQUEST,
        MSG_PROVIDER_CONNECT_REQUEST,
        MSG_PROVIDER_HEARTBEAT_REQUEST,
        MSG_PROVIDER_DRAIN_REQUEST,
    }
)


def is_control_request(msg_id: int) -> bool:
    """控制消息（心跳/注册/drain）走独立车道，永不 fail-fast。"""
    return msg_id in CONTROL_REQUESTS


def get_response_msg_id(req_msg_id: int) -> int:
    """Get the response MsgID for a given request MsgID."""
    return req_msg_id + 1


def msg_id_string(msg_id: int) -> str:
    """Get human-readable string for MsgID."""
    return MSG_NAMES.get(msg_id, f"Unknown(0x{msg_id:06X})")
