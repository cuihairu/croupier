/**
 * Croupier wire protocol implementation for TCP transport.
 *
 * Message Format:
 *   Header (8 bytes):
 *     ┌─────────┬──────────┬─────────────────┐
 *     │ Version │ MsgID    │ RequestID       │
 *     │ (1B)    │ (3B)     │ (4B)            │
 *     └─────────┴──────────┴─────────────────┘
 *   Body: protobuf serialized message
 */

// Protocol version
export const VERSION_1 = 0x01;

// Header size: Version(1) + MsgID(3) + RequestID(4)
export const HEADER_SIZE = 8;

// Message type constants (24 bits)
// ControlService (0x01xx)
export const MSG_REGISTER_REQUEST = 0x010101;
export const MSG_REGISTER_RESPONSE = 0x010102;
export const MSG_HEARTBEAT_REQUEST = 0x010103;
export const MSG_HEARTBEAT_RESPONSE = 0x010104;
export const MSG_REGISTER_CAPABILITIES_REQ = 0x010105;
export const MSG_REGISTER_CAPABILITIES_RESP = 0x010106;

// ClientService (0x02xx)
export const MSG_REGISTER_CLIENT_REQUEST = 0x020101;
export const MSG_REGISTER_CLIENT_RESPONSE = 0x020102;
export const MSG_CLIENT_HEARTBEAT_REQUEST = 0x020103;
export const MSG_CLIENT_HEARTBEAT_RESPONSE = 0x020104;
export const MSG_LIST_CLIENTS_REQUEST = 0x020105;
export const MSG_LIST_CLIENTS_RESPONSE = 0x020106;
export const MSG_GET_TASK_RESULT_REQUEST = 0x020107;
export const MSG_GET_TASK_RESULT_RESPONSE = 0x020108;

// InvokerService (0x03xx)
export const MSG_INVOKE_REQUEST = 0x030101;
export const MSG_INVOKE_RESPONSE = 0x030102;
export const MSG_START_TASK_REQUEST = 0x030103;
export const MSG_START_TASK_RESPONSE = 0x030104;
export const MSG_STREAM_TASK_REQUEST = 0x030105;
export const MSG_TASK_EVENT = 0x030106;
export const MSG_CANCEL_TASK_REQUEST = 0x030107;
export const MSG_CANCEL_TASK_RESPONSE = 0x030108;

// OpsService (0x04xx)
export const MSG_GET_SYSTEM_INFO_REQUEST = 0x040101;
export const MSG_GET_SYSTEM_INFO_RESPONSE = 0x040102;
export const MSG_LIST_PROCESSES_REQUEST = 0x040103;
export const MSG_LIST_PROCESSES_RESPONSE = 0x040104;
export const MSG_REPORT_METRICS_REQUEST = 0x040105;
export const MSG_REPORT_METRICS_RESPONSE = 0x040106;
export const MSG_STREAM_METRICS_REQUEST = 0x040107;
export const MSG_METRIC_EVENT = 0x040108;

// Provider session control (0x05xx) — sdk-agent subprotocol
// See docs/architecture/sdk-wire-protocol.md.
export const MSG_PROVIDER_CONNECT_REQUEST = 0x050101;
export const MSG_PROVIDER_CONNECT_RESPONSE = 0x050102;
export const MSG_PROVIDER_HEARTBEAT_REQUEST = 0x050103;
export const MSG_PROVIDER_HEARTBEAT_RESPONSE = 0x050104;
export const MSG_PROVIDER_DRAIN_REQUEST = 0x050105;
export const MSG_PROVIDER_DRAIN_RESPONSE = 0x050106;
// F：文件下发原语（hotpatch P1 传输层）
export const MSG_PROVIDER_FILE_PUSH_REQUEST = 0x050109;
export const MSG_PROVIDER_FILE_PUSH_RESPONSE = 0x05010A;

/**
 * Encode a 24-bit MsgID into 3 bytes (big-endian).
 */
export function putMsgID(msgId: number): Buffer {
  const buf = Buffer.alloc(3);
  buf[0] = (msgId >> 16) & 0xff;
  buf[1] = (msgId >> 8) & 0xff;
  buf[2] = msgId & 0xff;
  return buf;
}

/**
 * Decode a 24-bit MsgID from 3 bytes (big-endian).
 */
export function getMsgID(buf: Buffer, offset: number = 0): number {
  return (buf[offset] << 16) | (buf[offset + 1] << 8) | buf[offset + 2];
}

/**
 * Create a new message with protocol header and body.
 */
export function newMessage(msgId: number, reqId: number, body: Buffer): Buffer {
  const header = Buffer.alloc(HEADER_SIZE);
  header[0] = VERSION_1;
  putMsgID(msgId).copy(header, 1);
  header.writeUInt32BE(reqId, 4);
  return Buffer.concat([header, body]);
}

/**
 * Parsed message components.
 */
export interface ParsedMessage {
  version: number;
  msgId: number;
  reqId: number;
  body: Buffer;
}

/**
 * Parse a received message.
 */
export function parseMessage(data: Buffer): ParsedMessage {
  if (data.length < HEADER_SIZE) {
    throw new Error(`Message too short: ${data.length} < ${HEADER_SIZE}`);
  }

  return {
    version: data[0],
    msgId: getMsgID(data, 1),
    reqId: data.readUInt32BE(4),
    body: data.subarray(HEADER_SIZE),
  };
}

/**
 * Check if the MsgID indicates a request message.
 */
export function isRequest(msgId: number): boolean {
  return (
    msgId % 2 === 1 && msgId !== MSG_TASK_EVENT && msgId !== MSG_METRIC_EVENT
  );
}

/**
 * Check if the MsgID indicates a response message.
 */
export function isResponse(msgId: number): boolean {
  return (
    msgId % 2 === 0 && msgId !== MSG_TASK_EVENT && msgId !== MSG_METRIC_EVENT
  );
}

/**
 * Get the response MsgID for a given request MsgID.
 */
export function getResponseMsgId(reqMsgId: number): number {
  return reqMsgId + 1;
}

/**
 * 会话/传输控制消息集合（对齐 Go pkg/protocol.IsControlRequest）：
 * 心跳、注册、drain 走独立派发车道，业务洪峰打满业务队列时控制面
 * 依然可达——否则心跳被 fail-fast 拒绝 → 对端判定会话死亡 → 过载
 * 升级为连接雪崩（见 docs/architecture/sdk-wire-protocol.md 双车道）。
 */
const CONTROL_REQUESTS: ReadonlySet<number> = new Set<number>([
  MSG_REGISTER_REQUEST,
  MSG_HEARTBEAT_REQUEST,
  MSG_REGISTER_CAPABILITIES_REQ,
  MSG_REGISTER_CLIENT_REQUEST,
  MSG_CLIENT_HEARTBEAT_REQUEST,
  MSG_PROVIDER_CONNECT_REQUEST,
  MSG_PROVIDER_HEARTBEAT_REQUEST,
  MSG_PROVIDER_DRAIN_REQUEST,
]);

/** 控制消息（心跳/注册/drain）走独立车道，永不 fail-fast。 */
export function isControlRequest(msgId: number): boolean {
  return CONTROL_REQUESTS.has(msgId);
}

/**
 * Get human-readable string for MsgID.
 */
export function msgIdString(msgId: number): string {
  const names: Record<number, string> = {
    [MSG_REGISTER_REQUEST]: "RegisterRequest",
    [MSG_REGISTER_RESPONSE]: "RegisterResponse",
    [MSG_HEARTBEAT_REQUEST]: "HeartbeatRequest",
    [MSG_HEARTBEAT_RESPONSE]: "HeartbeatResponse",
    [MSG_INVOKE_REQUEST]: "InvokeRequest",
    [MSG_INVOKE_RESPONSE]: "InvokeResponse",
    [MSG_START_TASK_REQUEST]: "StartTaskRequest",
    [MSG_START_TASK_RESPONSE]: "StartTaskResponse",
    [MSG_STREAM_TASK_REQUEST]: "StreamTaskRequest",
    [MSG_TASK_EVENT]: "TaskEvent",
    [MSG_CANCEL_TASK_REQUEST]: "CancelTaskRequest",
    [MSG_CANCEL_TASK_RESPONSE]: "CancelTaskResponse",
    [MSG_PROVIDER_CONNECT_REQUEST]: "ProviderConnectRequest",
    [MSG_PROVIDER_CONNECT_RESPONSE]: "ProviderConnectResponse",
    [MSG_PROVIDER_HEARTBEAT_REQUEST]: "ProviderHeartbeatRequest",
    [MSG_PROVIDER_HEARTBEAT_RESPONSE]: "ProviderHeartbeatResponse",
    [MSG_PROVIDER_DRAIN_REQUEST]: "ProviderDrainRequest",
    [MSG_PROVIDER_DRAIN_RESPONSE]: "ProviderDrainResponse",
  };
  return names[msgId] || `Unknown(0x${msgId.toString(16).padStart(6, "0")})`;
}
