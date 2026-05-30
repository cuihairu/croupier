/**
 * Unit tests for Protocol Layer.
 */

import {
  HEADER_SIZE,
  MSG_INVOKE_REQUEST,
  MSG_INVOKE_RESPONSE,
  MSG_PROVIDER_CONNECT_REQUEST,
  MSG_PROVIDER_CONNECT_RESPONSE,
  MSG_START_TASK_REQUEST,
  MSG_START_TASK_RESPONSE,
  MSG_CANCEL_TASK_REQUEST,
  MSG_CANCEL_TASK_RESPONSE,
  getMsgID,
  getResponseMsgId,
  isRequest,
  isResponse,
  msgIdString,
  newMessage,
  parseMessage,
  putMsgID,
} from "./protocol";

describe("Protocol", () => {
  describe("newMessage", () => {
    it("should create a message with correct header", () => {
      const msgType = MSG_INVOKE_REQUEST;
      const reqId = 12345;
      const body = Buffer.from("test payload");

      const message = newMessage(msgType, reqId, body);

      expect(message.length).toBe(HEADER_SIZE + body.length);
      expect(message[0]).toBe(0x01); // Version
    });

    it("should create a message with empty body", () => {
      const msgType = MSG_PROVIDER_CONNECT_REQUEST;
      const reqId = 1;
      const body = Buffer.alloc(0);

      const message = newMessage(msgType, reqId, body);

      expect(message.length).toBe(HEADER_SIZE);
      expect(message[0]).toBe(0x01);
    });

    it("should handle large request ID", () => {
      const msgType = MSG_INVOKE_REQUEST;
      const reqId = 0xffffffff; // Max uint32
      const body = Buffer.from("test");

      const message = newMessage(msgType, reqId, body);
      const parsed = parseMessage(message);

      expect(parsed.reqId).toBe(reqId);
    });
  });

  describe("parseMessage", () => {
    it("should parse a message correctly", () => {
      const msgType = MSG_PROVIDER_CONNECT_REQUEST;
      const reqId = 999;
      const body = Buffer.from("hello world");

      const message = newMessage(msgType, reqId, body);
      const parsed = parseMessage(message);

      expect(parsed.version).toBe(0x01);
      expect(parsed.msgId).toBe(msgType);
      expect(parsed.reqId).toBe(reqId);
      expect(parsed.body).toEqual(body);
    });

    it("should parse message with empty body", () => {
      const msgType = MSG_INVOKE_REQUEST;
      const reqId = 42;
      const body = Buffer.alloc(0);

      const message = newMessage(msgType, reqId, body);
      const parsed = parseMessage(message);

      expect(parsed.version).toBe(0x01);
      expect(parsed.msgId).toBe(msgType);
      expect(parsed.reqId).toBe(reqId);
      expect(parsed.body.length).toBe(0);
    });

    it("should throw on message too short", () => {
      const shortMessage = Buffer.from([0x01, 0x02, 0x03]);

      expect(() => parseMessage(shortMessage)).toThrow();
    });
  });

  describe("isRequest and isResponse", () => {
    it("should identify request messages (odd msgId)", () => {
      expect(isRequest(MSG_INVOKE_REQUEST)).toBe(true);
      expect(isRequest(MSG_START_TASK_REQUEST)).toBe(true);
      expect(isRequest(MSG_CANCEL_TASK_REQUEST)).toBe(true);
      expect(isRequest(MSG_PROVIDER_CONNECT_REQUEST)).toBe(true);
    });

    it("should identify response messages (even msgId)", () => {
      expect(isResponse(MSG_INVOKE_RESPONSE)).toBe(true);
      expect(isResponse(MSG_START_TASK_RESPONSE)).toBe(true);
      expect(isResponse(MSG_CANCEL_TASK_RESPONSE)).toBe(true);
      expect(isResponse(MSG_PROVIDER_CONNECT_RESPONSE)).toBe(true);
    });

    it("should not mix request and response", () => {
      expect(isRequest(MSG_INVOKE_RESPONSE)).toBe(false);
      expect(isResponse(MSG_INVOKE_REQUEST)).toBe(false);
    });
  });

  describe("getResponseMsgId", () => {
    it("should return response message ID for request", () => {
      expect(getResponseMsgId(MSG_INVOKE_REQUEST)).toBe(MSG_INVOKE_RESPONSE);
      expect(getResponseMsgId(MSG_START_TASK_REQUEST)).toBe(
        MSG_START_TASK_RESPONSE,
      );
      expect(getResponseMsgId(MSG_CANCEL_TASK_REQUEST)).toBe(
        MSG_CANCEL_TASK_RESPONSE,
      );
      expect(getResponseMsgId(MSG_PROVIDER_CONNECT_REQUEST)).toBe(
        MSG_PROVIDER_CONNECT_RESPONSE,
      );
    });
  });

  describe("putMsgID and getMsgID", () => {
    it("should encode and decode message ID correctly", () => {
      const msgId = 0x030101;
      const encoded = putMsgID(msgId);
      const decoded = getMsgID(encoded);

      expect(decoded).toBe(msgId);
    });

    it("should handle max message ID", () => {
      const msgId = 0xffffff;
      const encoded = putMsgID(msgId);
      const decoded = getMsgID(encoded);

      expect(decoded).toBe(msgId);
    });

    it("should handle min message ID", () => {
      const msgId = 0x000000;
      const encoded = putMsgID(msgId);
      const decoded = getMsgID(encoded);

      expect(decoded).toBe(msgId);
    });
  });

  describe("HEADER_SIZE", () => {
    it("should be 8 bytes", () => {
      expect(HEADER_SIZE).toBe(8);
    });
  });

  describe("msgIdString", () => {
    it("should return human-readable message ID string", () => {
      expect(msgIdString(MSG_INVOKE_REQUEST)).toBe("InvokeRequest");
      expect(msgIdString(MSG_INVOKE_RESPONSE)).toBe("InvokeResponse");
      expect(msgIdString(MSG_PROVIDER_CONNECT_REQUEST)).toBe(
        "ProviderConnectRequest",
      );
      expect(msgIdString(MSG_START_TASK_REQUEST)).toBe("StartTaskRequest");
      expect(msgIdString(MSG_START_TASK_RESPONSE)).toBe("StartTaskResponse");
      expect(msgIdString(MSG_CANCEL_TASK_REQUEST)).toBe("CancelTaskRequest");
      expect(msgIdString(MSG_CANCEL_TASK_RESPONSE)).toBe("CancelTaskResponse");
      expect(msgIdString(0xffffff)).toBe("Unknown(0xffffff)");
    });
  });
});

// TCPTransport tests will be added separately
// TCPTransport tests removed as we migrate to TCP transport
