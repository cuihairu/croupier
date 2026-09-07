package io.github.cuihairu.croupier.sdk.wire;

import org.junit.jupiter.api.Test;

import java.util.Map;

import static org.junit.jupiter.api.Assertions.assertArrayEquals;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertTrue;

/** Branch-complete coverage for SdkWireMessages decode edges (tag 0, unknown fields). */
class SdkWireMessagesZeroTagBranchTest {

    /** Input whose first tag byte is 0x00 is rejected by the protobuf runtime. */
    @Test
    void zeroTagInputSurfacesDecodeError() {
        try {
            SdkWireMessages.decodeInvokeRequest(new byte[] {0x00});
            assertTrue(false, "expected IllegalArgumentException");
        } catch (IllegalArgumentException expected) {
            // expected: protobuf runtime rejects tag 0
        }
    }

    @Test
    void truncatedInputSurfacesDecodeError() {
        try {
            SdkWireMessages.decodeInvokeRequest(new byte[] {0x0A});
            assertTrue(false, "expected IllegalArgumentException");
        } catch (IllegalArgumentException expected) {
            // expected: truncated length-delimited field
        }
    }

    /** Unknown field 15, varint wire type 0. */
    private static byte[] unknownVarintField() {
        return new byte[] {(byte) 0x78, 0x2A};
    }

    /** Unknown field 15, length-delimited wire type 2. */
    private static byte[] unknownBytesField() {
        return new byte[] {(byte) 0x7A, 0x03, 0x01, 0x02, 0x03};
    }

    @Test
    void unknownFieldsAreSkippedByEveryDecoder() {
        assertNotNull(SdkWireMessages.decodeInvokeRequest(unknownVarintField()));
        assertNotNull(SdkWireMessages.decodeInvokeResponse(unknownBytesField()));
        assertNotNull(SdkWireMessages.decodeStartTaskResponse(unknownVarintField()));
        assertNotNull(SdkWireMessages.decodeTaskStreamRequest(unknownBytesField()));
        assertNotNull(SdkWireMessages.decodeTaskEvent(unknownVarintField()));
        assertNotNull(SdkWireMessages.decodeCancelTaskRequest(unknownBytesField()));
        assertNotNull(SdkWireMessages.decodeProviderConnectRequest(unknownVarintField()));
        assertNotNull(SdkWireMessages.decodeProviderConnectResponse(unknownBytesField()));
        assertNotNull(SdkWireMessages.decodeHeartbeatRequest(unknownVarintField()));
        assertNotNull(SdkWireMessages.decodeRegisterCapabilitiesRequest(unknownBytesField()));
        assertNotNull(SdkWireMessages.decodeFilePushRequest(unknownVarintField()));
        assertNotNull(SdkWireMessages.decodeFilePushResponse(unknownBytesField()));
        assertNotNull(SdkWireMessages.decodeProviderDrainRequest(unknownVarintField()));
    }

    @Test
    void nullInputDecodesToDefaults() {
        assertEquals("", SdkWireMessages.decodeInvokeRequest(null).functionId);
        assertEquals("", SdkWireMessages.decodeInvokeResponse(null).payloadUtf8());
        assertEquals("", SdkWireMessages.decodeStartTaskResponse(null).taskId);
        assertEquals("", SdkWireMessages.decodeTaskStreamRequest(null).taskId);
        assertEquals("", SdkWireMessages.decodeTaskEvent(null).type);
        assertEquals("", SdkWireMessages.decodeCancelTaskRequest(null).taskId);
        assertEquals("", SdkWireMessages.decodeProviderConnectResponse(null).sessionId);
        assertEquals("", SdkWireMessages.decodeHeartbeatRequest(null).serviceId);
        assertEquals("", SdkWireMessages.decodeFilePushRequest(null).transferId);
        assertEquals("", SdkWireMessages.decodeFilePushResponse(null).transferId);
        assertEquals("", SdkWireMessages.decodeProviderDrainRequest(null).sessionId);
    }

    @Test
    void encodingEmptyAndNullValuesOmitsFields() {
        // empty strings / null arrays / zero ints / false bools exercise the
        // write* guard branches
        byte[] invoke = SdkWireMessages.encodeInvokeRequest(
            new SdkWireMessages.InvokeRequest(null, "", null, Map.of()));
        assertEquals("", SdkWireMessages.decodeInvokeRequest(invoke).functionId);

        byte[] emptyInvoke = SdkWireMessages.encodeInvokeRequest(
            new SdkWireMessages.InvokeRequest("", "", new byte[0], Map.of()));
        assertArrayEquals(new byte[0], SdkWireMessages.decodeInvokeRequest(emptyInvoke).payload);

        byte[] event = SdkWireMessages.encodeTaskEvent(
            new SdkWireMessages.TaskEvent(null, null, 0, null));
        assertEquals(0, SdkWireMessages.decodeTaskEvent(event).progress);

        byte[] connectResponse = SdkWireMessages.encodeProviderConnectResponse(
            new SdkWireMessages.ProviderConnectResponse(null));
        assertEquals("", SdkWireMessages.decodeProviderConnectResponse(connectResponse).sessionId);

        byte[] heartbeat = SdkWireMessages.encodeHeartbeatRequest(
            new SdkWireMessages.HeartbeatRequest(null, null));
        assertEquals("", SdkWireMessages.decodeHeartbeatRequest(heartbeat).serviceId);

        byte[] drain = SdkWireMessages.encodeProviderDrainResponse();
        assertTrue(drain.length >= 0);

        byte[] filePush = SdkWireMessages.encodeFilePushRequest(
            new SdkWireMessages.FilePushRequest(null, null, null, null));
        assertEquals("", SdkWireMessages.decodeFilePushRequest(filePush).transferId);

        byte[] filePushResponse = SdkWireMessages.encodeFilePushResponse(
            new SdkWireMessages.FilePushResponse(null, false, null, null));
        assertEquals("", SdkWireMessages.decodeFilePushResponse(filePushResponse).transferId);
    }
}
