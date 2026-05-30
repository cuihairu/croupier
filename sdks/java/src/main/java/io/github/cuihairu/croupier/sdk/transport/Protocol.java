/**
 * Croupier wire protocol implementation for TCP transport.
 *
 * <p>Message Format:
 * <pre>
 * Header (8 bytes):
 *   ┌─────────┬──────────┬─────────────────┐
 *   │ Version │ MsgID    │ RequestID       │
 *   │ (1B)    │ (3B)     │ (4B)            │
 *   └─────────┴──────────┴─────────────────┘
 * Body: protobuf serialized message
 * </pre>
 *
 * <p>Request messages have odd MsgID, Response messages have even MsgID.
 */
package io.github.cuihairu.croupier.sdk.transport;

import java.nio.ByteBuffer;
import java.nio.ByteOrder;

/**
 * Protocol constants and utilities.
 */
public final class Protocol {
    private Protocol() {}

    // Protocol version
    public static final byte VERSION_1 = 0x01;

    // Header size: Version(1) + MsgID(3) + RequestID(4)
    public static final int HEADER_SIZE = 8;

    // Message type constants (24 bits)
    // ControlService (0x01xx)
    public static final int MSG_REGISTER_REQUEST = 0x010101;
    public static final int MSG_REGISTER_RESPONSE = 0x010102;
    public static final int MSG_HEARTBEAT_REQUEST = 0x010103;
    public static final int MSG_HEARTBEAT_RESPONSE = 0x010104;
    public static final int MSG_REGISTER_CAPABILITIES_REQ = 0x010105;
    public static final int MSG_REGISTER_CAPABILITIES_RESP = 0x010106;

    // ClientService (0x02xx)
    public static final int MSG_REGISTER_CLIENT_REQUEST = 0x020101;
    public static final int MSG_REGISTER_CLIENT_RESPONSE = 0x020102;
    public static final int MSG_CLIENT_HEARTBEAT_REQUEST = 0x020103;
    public static final int MSG_CLIENT_HEARTBEAT_RESPONSE = 0x020104;

    // InvokerService (0x03xx)
    public static final int MSG_INVOKE_REQUEST = 0x030101;
    public static final int MSG_INVOKE_RESPONSE = 0x030102;
    public static final int MSG_START_JOB_REQUEST = 0x030103;
    public static final int MSG_START_JOB_RESPONSE = 0x030104;
    public static final int MSG_STREAM_JOB_REQUEST = 0x030105;
    public static final int MSG_JOB_EVENT = 0x030106;
    public static final int MSG_CANCEL_JOB_REQUEST = 0x030107;
    public static final int MSG_CANCEL_JOB_RESPONSE = 0x030108;

    // LocalControlService (0x05xx)
    public static final int MSG_REGISTER_LOCAL_REQUEST = 0x050101;
    public static final int MSG_REGISTER_LOCAL_RESPONSE = 0x050102;
    public static final int MSG_HEARTBEAT_LOCAL_REQUEST = 0x050103;
    public static final int MSG_HEARTBEAT_LOCAL_RESPONSE = 0x050104;
    public static final int MSG_LIST_LOCAL_REQUEST = 0x050105;
    public static final int MSG_LIST_LOCAL_RESPONSE = 0x050106;

    /**
     * Encode a 24-bit MsgID into 3 bytes (big-endian).
     */
    public static void putMsgID(byte[] buf, int offset, int msgId) {
        buf[offset] = (byte) ((msgId >> 16) & 0xFF);
        buf[offset + 1] = (byte) ((msgId >> 8) & 0xFF);
        buf[offset + 2] = (byte) (msgId & 0xFF);
    }

    /**
     * Decode a 24-bit MsgID from 3 bytes (big-endian).
     */
    public static int getMsgID(byte[] buf, int offset) {
        return ((buf[offset] & 0xFF) << 16) |
               ((buf[offset + 1] & 0xFF) << 8) |
               (buf[offset + 2] & 0xFF);
    }

    /**
     * Create a new message with protocol header and body.
     */
    public static byte[] newMessage(int msgId, int reqId, byte[] body) {
        byte[] message = new byte[HEADER_SIZE + (body != null ? body.length : 0)];

        // Header
        message[0] = VERSION_1;
        putMsgID(message, 1, msgId);

        // RequestID (big-endian)
        message[4] = (byte) ((reqId >> 24) & 0xFF);
        message[5] = (byte) ((reqId >> 16) & 0xFF);
        message[6] = (byte) ((reqId >> 8) & 0xFF);
        message[7] = (byte) (reqId & 0xFF);

        // Body
        if (body != null && body.length > 0) {
            System.arraycopy(body, 0, message, HEADER_SIZE, body.length);
        }

        return message;
    }

    /**
     * Parsed message components.
     */
    public static class ParsedMessage {
        public final byte version;
        public final int msgId;
        public final int reqId;
        public final byte[] body;

        public ParsedMessage(byte version, int msgId, int reqId, byte[] body) {
            this.version = version;
            this.msgId = msgId;
            this.reqId = reqId;
            this.body = body;
        }
    }

    /**
     * Parse a received message.
     */
    public static ParsedMessage parseMessage(byte[] data) {
        if (data.length < HEADER_SIZE) {
            throw new IllegalArgumentException("Message too short: " + data.length);
        }

        byte version = data[0];
        int msgId = getMsgID(data, 1);
        int reqId = ((data[4] & 0xFF) << 24) |
                    ((data[5] & 0xFF) << 16) |
                    ((data[6] & 0xFF) << 8) |
                    (data[7] & 0xFF);

        byte[] body = new byte[data.length - HEADER_SIZE];
        System.arraycopy(data, HEADER_SIZE, body, 0, body.length);

        return new ParsedMessage(version, msgId, reqId, body);
    }

    /**
     * Check if the MsgID indicates a request message.
     */
    public static boolean isRequest(int msgId) {
        return msgId % 2 == 1 && msgId != MSG_JOB_EVENT;
    }

    /**
     * Check if the MsgID indicates a response message.
     */
    public static boolean isResponse(int msgId) {
        return msgId % 2 == 0 && msgId != MSG_JOB_EVENT;
    }

    /**
     * Get the response MsgID for a given request MsgID.
     */
    public static int getResponseMsgID(int reqMsgId) {
        return reqMsgId + 1;
    }

    /**
     * Get human-readable string for MsgID.
     */
    public static String msgIdString(int msgId) {
        switch (msgId) {
            case MSG_REGISTER_REQUEST: return "RegisterRequest";
            case MSG_REGISTER_RESPONSE: return "RegisterResponse";
            case MSG_HEARTBEAT_REQUEST: return "HeartbeatRequest";
            case MSG_HEARTBEAT_RESPONSE: return "HeartbeatResponse";
            case MSG_INVOKE_REQUEST: return "InvokeRequest";
            case MSG_INVOKE_RESPONSE: return "InvokeResponse";
            case MSG_START_JOB_REQUEST: return "StartJobRequest";
            case MSG_START_JOB_RESPONSE: return "StartJobResponse";
            case MSG_STREAM_JOB_REQUEST: return "StreamJobRequest";
            case MSG_JOB_EVENT: return "JobEvent";
            case MSG_CANCEL_JOB_REQUEST: return "CancelJobRequest";
            case MSG_CANCEL_JOB_RESPONSE: return "CancelJobResponse";
            case MSG_REGISTER_LOCAL_REQUEST: return "RegisterLocalRequest";
            case MSG_REGISTER_LOCAL_RESPONSE: return "RegisterLocalResponse";
            case MSG_HEARTBEAT_LOCAL_REQUEST: return "HeartbeatLocalRequest";
            case MSG_HEARTBEAT_LOCAL_RESPONSE: return "HeartbeatLocalResponse";
            case MSG_LIST_LOCAL_REQUEST: return "ListLocalRequest";
            case MSG_LIST_LOCAL_RESPONSE: return "ListLocalResponse";
            default: return String.format("Unknown(0x%06X)", msgId);
        }
    }
}
