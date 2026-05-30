/**
 * Unit tests for Protocol class.
 */
package io.github.cuihairu.croupier.sdk.transport;

import org.junit.jupiter.api.Test;
import static org.junit.jupiter.api.Assertions.*;

class ProtocolTest {

    @Test
    void testNewMessage() {
        int msgType = Protocol.MSG_INVOKE_REQUEST;
        int reqId = 12345;
        byte[] body = "test payload".getBytes();

        byte[] message = Protocol.newMessage(msgType, reqId, body);

        assertEquals(Protocol.HEADER_SIZE + body.length, message.length);
        assertEquals(Protocol.VERSION_1, message[0]);
    }

    @Test
    void testNewMessageWithEmptyBody() {
        int msgType = Protocol.MSG_REGISTER_LOCAL_REQUEST;
        int reqId = 1;
        byte[] body = new byte[0];

        byte[] message = Protocol.newMessage(msgType, reqId, body);

        assertEquals(Protocol.HEADER_SIZE, message.length);
        assertEquals(Protocol.VERSION_1, message[0]);
    }

    @Test
    void testNewMessageWithNullBody() {
        int msgType = Protocol.MSG_INVOKE_REQUEST;
        int reqId = 123;

        byte[] message = Protocol.newMessage(msgType, reqId, null);

        assertEquals(Protocol.HEADER_SIZE, message.length);
        assertEquals(Protocol.VERSION_1, message[0]);
    }

    @Test
    void testNewMessageWithLargeRequestId() {
        int msgType = Protocol.MSG_INVOKE_REQUEST;
        int reqId = 0xFFFFFFFF; // Max uint32
        byte[] body = "test".getBytes();

        byte[] message = Protocol.newMessage(msgType, reqId, body);
        Protocol.ParsedMessage parsed = Protocol.parseMessage(message);

        assertEquals(reqId, parsed.reqId);
    }

    @Test
    void testNewMessageWithZeroRequestId() {
        int msgType = Protocol.MSG_INVOKE_REQUEST;
        int reqId = 0;
        byte[] body = "test".getBytes();

        byte[] message = Protocol.newMessage(msgType, reqId, body);
        Protocol.ParsedMessage parsed = Protocol.parseMessage(message);

        assertEquals(reqId, parsed.reqId);
    }

    @Test
    void testParseMessage() {
        int msgType = Protocol.MSG_REGISTER_LOCAL_REQUEST;
        int reqId = 999;
        byte[] body = "hello world".getBytes();

        byte[] message = Protocol.newMessage(msgType, reqId, body);
        Protocol.ParsedMessage parsed = Protocol.parseMessage(message);

        assertEquals(Protocol.VERSION_1, parsed.version);
        assertEquals(msgType, parsed.msgId);
        assertEquals(reqId, parsed.reqId);
        assertArrayEquals(body, parsed.body);
    }

    @Test
    void testParseMessageWithEmptyBody() {
        int msgType = Protocol.MSG_INVOKE_REQUEST;
        int reqId = 123;
        byte[] body = new byte[0];

        byte[] message = Protocol.newMessage(msgType, reqId, body);
        Protocol.ParsedMessage parsed = Protocol.parseMessage(message);

        assertEquals(0, parsed.body.length);
    }

    @Test
    void testParseMessageTooShort() {
        byte[] shortMessage = new byte[7]; // Less than HEADER_SIZE

        assertThrows(IllegalArgumentException.class, () -> {
            Protocol.parseMessage(shortMessage);
        });
    }

    @Test
    void testParseMessageExactHeaderSize() {
        byte[] headerOnlyMessage = new byte[Protocol.HEADER_SIZE];
        headerOnlyMessage[0] = Protocol.VERSION_1;

        Protocol.ParsedMessage parsed = Protocol.parseMessage(headerOnlyMessage);

        assertEquals(Protocol.VERSION_1, parsed.version);
        assertEquals(0, parsed.body.length);
    }

    @Test
    void testGetResponseMsgID() {
        assertEquals(Protocol.MSG_INVOKE_RESPONSE, Protocol.getResponseMsgID(Protocol.MSG_INVOKE_REQUEST));
        assertEquals(Protocol.MSG_REGISTER_LOCAL_RESPONSE, Protocol.getResponseMsgID(Protocol.MSG_REGISTER_LOCAL_REQUEST));
        assertEquals(Protocol.MSG_START_JOB_RESPONSE, Protocol.getResponseMsgID(Protocol.MSG_START_JOB_REQUEST));
        assertEquals(Protocol.MSG_CANCEL_JOB_RESPONSE, Protocol.getResponseMsgID(Protocol.MSG_CANCEL_JOB_REQUEST));
    }

    @Test
    void testGetResponseMsgIDIncrementsByOne() {
        assertEquals(0x030102, Protocol.getResponseMsgID(0x030101));
        assertEquals(0x050104, Protocol.getResponseMsgID(0x050103));
    }

    @Test
    void testIsRequest() {
        assertTrue(Protocol.isRequest(Protocol.MSG_INVOKE_REQUEST));
        assertTrue(Protocol.isRequest(Protocol.MSG_REGISTER_LOCAL_REQUEST));
        assertTrue(Protocol.isRequest(Protocol.MSG_START_JOB_REQUEST));
        assertTrue(Protocol.isRequest(Protocol.MSG_CANCEL_JOB_REQUEST));
        assertFalse(Protocol.isRequest(Protocol.MSG_INVOKE_RESPONSE));
        assertFalse(Protocol.isRequest(Protocol.MSG_REGISTER_LOCAL_RESPONSE));
    }

    @Test
    void testIsRequestWithEvenNumbers() {
        assertFalse(Protocol.isRequest(Protocol.MSG_INVOKE_RESPONSE));
        assertFalse(Protocol.isRequest(Protocol.MSG_JOB_EVENT)); // Special case
    }

    @Test
    void testIsResponse() {
        assertTrue(Protocol.isResponse(Protocol.MSG_INVOKE_RESPONSE));
        assertTrue(Protocol.isResponse(Protocol.MSG_REGISTER_LOCAL_RESPONSE));
        assertTrue(Protocol.isResponse(Protocol.MSG_START_JOB_RESPONSE));
        assertTrue(Protocol.isResponse(Protocol.MSG_CANCEL_JOB_RESPONSE));
        assertFalse(Protocol.isResponse(Protocol.MSG_INVOKE_REQUEST));
        assertFalse(Protocol.isResponse(Protocol.MSG_REGISTER_LOCAL_REQUEST));
    }

    @Test
    void testIsResponseWithOddNumbers() {
        assertFalse(Protocol.isResponse(Protocol.MSG_INVOKE_REQUEST));
        assertFalse(Protocol.isResponse(Protocol.MSG_JOB_EVENT)); // Special case
    }

    @Test
    void testMsgIdString() {
        assertEquals("RegisterRequest", Protocol.msgIdString(Protocol.MSG_REGISTER_REQUEST));
        assertEquals("RegisterResponse", Protocol.msgIdString(Protocol.MSG_REGISTER_RESPONSE));
        assertEquals("HeartbeatRequest", Protocol.msgIdString(Protocol.MSG_HEARTBEAT_REQUEST));
        assertEquals("HeartbeatResponse", Protocol.msgIdString(Protocol.MSG_HEARTBEAT_RESPONSE));
        assertEquals("InvokeRequest", Protocol.msgIdString(Protocol.MSG_INVOKE_REQUEST));
        assertEquals("InvokeResponse", Protocol.msgIdString(Protocol.MSG_INVOKE_RESPONSE));
        assertEquals("StartJobRequest", Protocol.msgIdString(Protocol.MSG_START_JOB_REQUEST));
        assertEquals("StartJobResponse", Protocol.msgIdString(Protocol.MSG_START_JOB_RESPONSE));
        assertEquals("StreamJobRequest", Protocol.msgIdString(Protocol.MSG_STREAM_JOB_REQUEST));
        assertEquals("JobEvent", Protocol.msgIdString(Protocol.MSG_JOB_EVENT));
        assertEquals("CancelJobRequest", Protocol.msgIdString(Protocol.MSG_CANCEL_JOB_REQUEST));
        assertEquals("CancelJobResponse", Protocol.msgIdString(Protocol.MSG_CANCEL_JOB_RESPONSE));
        assertEquals("RegisterLocalRequest", Protocol.msgIdString(Protocol.MSG_REGISTER_LOCAL_REQUEST));
        assertEquals("RegisterLocalResponse", Protocol.msgIdString(Protocol.MSG_REGISTER_LOCAL_RESPONSE));
        assertEquals("HeartbeatLocalRequest", Protocol.msgIdString(Protocol.MSG_HEARTBEAT_LOCAL_REQUEST));
        assertEquals("HeartbeatLocalResponse", Protocol.msgIdString(Protocol.MSG_HEARTBEAT_LOCAL_RESPONSE));
        assertEquals("ListLocalRequest", Protocol.msgIdString(Protocol.MSG_LIST_LOCAL_REQUEST));
        assertEquals("ListLocalResponse", Protocol.msgIdString(Protocol.MSG_LIST_LOCAL_RESPONSE));
    }

    @Test
    void testMsgIdStringUnknown() {
        assertTrue(Protocol.msgIdString(0xFFFFFF).startsWith("Unknown(0x"));
        assertTrue(Protocol.msgIdString(0x000000).startsWith("Unknown(0x"));
        assertTrue(Protocol.msgIdString(0x123456).startsWith("Unknown(0x"));
    }

    @Test
    void testMsgIdStringFormat() {
        String result = Protocol.msgIdString(0xABCDEF);
        assertEquals("Unknown(0xABCDEF)", result);
    }

    @Test
    void testPutAndGetMsgID() {
        int msgId = 0x030101;
        byte[] buf = new byte[3];
        Protocol.putMsgID(buf, 0, msgId);
        int decoded = Protocol.getMsgID(buf, 0);
        assertEquals(msgId, decoded);
    }

    @Test
    void testPutAndGetMsgIDWithMaxValue() {
        int msgId = 0xFFFFFF; // Max 24-bit value
        byte[] buf = new byte[3];
        Protocol.putMsgID(buf, 0, msgId);
        int decoded = Protocol.getMsgID(buf, 0);
        assertEquals(msgId, decoded);
    }

    @Test
    void testPutAndGetMsgIDWithMinValue() {
        int msgId = 0x000000;
        byte[] buf = new byte[3];
        Protocol.putMsgID(buf, 0, msgId);
        int decoded = Protocol.getMsgID(buf, 0);
        assertEquals(msgId, decoded);
    }

    @Test
    void testPutAndGetMsgIDWithOffset() {
        int msgId = 0x123456;
        byte[] buf = new byte[10];
        int offset = 5;

        Protocol.putMsgID(buf, offset, msgId);
        int decoded = Protocol.getMsgID(buf, offset);

        assertEquals(msgId, decoded);
        // Verify bytes are at correct offset
        assertEquals((byte) 0x12, buf[offset]);
        assertEquals((byte) 0x34, buf[offset + 1]);
        assertEquals((byte) 0x56, buf[offset + 2]);
    }

    @Test
    void testHeaderSizeConstant() {
        assertEquals(8, Protocol.HEADER_SIZE);
    }

    @Test
    void testVersionConstant() {
        assertEquals(0x01, Protocol.VERSION_1);
    }

    @Test
    void testAllMessageConstantsAreUnique() {
        // Verify all message IDs are distinct
        assertNotEquals(Protocol.MSG_REGISTER_REQUEST, Protocol.MSG_REGISTER_RESPONSE);
        assertNotEquals(Protocol.MSG_INVOKE_REQUEST, Protocol.MSG_INVOKE_RESPONSE);
        assertNotEquals(Protocol.MSG_START_JOB_REQUEST, Protocol.MSG_START_JOB_RESPONSE);
    }

    @Test
    void testRequestResponsePairs() {
        // Verify each request has a corresponding response (request + 1)
        assertEquals(Protocol.MSG_REGISTER_REQUEST + 1, Protocol.MSG_REGISTER_RESPONSE);
        assertEquals(Protocol.MSG_HEARTBEAT_REQUEST + 1, Protocol.MSG_HEARTBEAT_RESPONSE);
        assertEquals(Protocol.MSG_INVOKE_REQUEST + 1, Protocol.MSG_INVOKE_RESPONSE);
        assertEquals(Protocol.MSG_START_JOB_REQUEST + 1, Protocol.MSG_START_JOB_RESPONSE);
        assertEquals(Protocol.MSG_CANCEL_JOB_REQUEST + 1, Protocol.MSG_CANCEL_JOB_RESPONSE);
        assertEquals(Protocol.MSG_REGISTER_LOCAL_REQUEST + 1, Protocol.MSG_REGISTER_LOCAL_RESPONSE);
        assertEquals(Protocol.MSG_HEARTBEAT_LOCAL_REQUEST + 1, Protocol.MSG_HEARTBEAT_LOCAL_RESPONSE);
        assertEquals(Protocol.MSG_LIST_LOCAL_REQUEST + 1, Protocol.MSG_LIST_LOCAL_RESPONSE);
    }

    @Test
    void testBigEndianEncoding() {
        int msgId = 0xABCDEF;
        byte[] buf = new byte[3];
        Protocol.putMsgID(buf, 0, msgId);

        // Big-endian: most significant byte first
        assertEquals((byte) 0xAB, buf[0]);
        assertEquals((byte) 0xCD, buf[1]);
        assertEquals((byte) 0xEF, buf[2]);
    }

    @Test
    void testBigEndianDecoding() {
        byte[] buf = {(byte) 0xAB, (byte) 0xCD, (byte) 0xEF};
        int msgId = Protocol.getMsgID(buf, 0);

        assertEquals(0xABCDEF, msgId);
    }

    @Test
    void testRequestIdBigEndianEncoding() {
        int reqId = 0x12345678;
        int msgType = Protocol.MSG_INVOKE_REQUEST;
        byte[] body = new byte[0];

        byte[] message = Protocol.newMessage(msgType, reqId, body);

        // Verify big-endian encoding of request ID
        assertEquals((byte) 0x12, message[4]);
        assertEquals((byte) 0x34, message[5]);
        assertEquals((byte) 0x56, message[6]);
        assertEquals((byte) 0x78, message[7]);
    }

    @Test
    void testRequestIdBigEndianDecoding() {
        int reqId = 0xABCDEF01;
        int msgType = Protocol.MSG_INVOKE_REQUEST;
        byte[] body = "test".getBytes();

        byte[] message = Protocol.newMessage(msgType, reqId, body);
        Protocol.ParsedMessage parsed = Protocol.parseMessage(message);

        assertEquals(reqId, parsed.reqId);
    }

    @Test
    void testMessageBodyCopiedCorrectly() {
        int msgType = Protocol.MSG_INVOKE_REQUEST;
        int reqId = 12345;
        byte[] body = "test payload data".getBytes();

        byte[] message = Protocol.newMessage(msgType, reqId, body);
        Protocol.ParsedMessage parsed = Protocol.parseMessage(message);

        assertArrayEquals(body, parsed.body);
    }

    @Test
    void testLargeMessageBody() {
        int msgType = Protocol.MSG_INVOKE_REQUEST;
        int reqId = 1;
        byte[] body = new byte[1024 * 1024]; // 1MB
        for (int i = 0; i < body.length; i++) {
            body[i] = (byte) (i % 256);
        }

        byte[] message = Protocol.newMessage(msgType, reqId, body);
        Protocol.parseMessage(message); // Should not throw

        assertEquals(Protocol.HEADER_SIZE + body.length, message.length);
    }

    @Test
    void testConstructorIsPrivate() {
        // Protocol class should have private constructor
        try {
            var constructor = Protocol.class.getDeclaredConstructor();
            assertFalse(constructor.canAccess(null));
        } catch (NoSuchMethodException e) {
            fail("Protocol should have a private constructor");
        }
    }
}
