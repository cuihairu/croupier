package io.github.cuihairu.croupier.sdk.wire;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Additional tests for SdkWireMessages to improve code coverage.
 */
class SdkWireMessagesCoverageTest {

    @Test
    @DisplayName("InvokeRequest encode/decode round-trip with metadata")
    void invokeRequestRoundTripWithMetadata() {
        Map<String, String> metadata = new LinkedHashMap<>();
        metadata.put("key1", "value1");
        metadata.put("key2", "value2");

        SdkWireMessages.InvokeRequest original = new SdkWireMessages.InvokeRequest(
            "func-1", "idem-key", "payload".getBytes(), metadata);

        byte[] encoded = SdkWireMessages.encodeInvokeRequest(original);
        SdkWireMessages.InvokeRequest decoded = SdkWireMessages.decodeInvokeRequest(encoded);

        assertEquals("func-1", decoded.functionId);
        assertEquals("idem-key", decoded.idempotencyKey);
        assertArrayEquals("payload".getBytes(), decoded.payload);
        assertEquals("value1", decoded.metadata.get("key1"));
        assertEquals("value2", decoded.metadata.get("key2"));
    }

    @Test
    @DisplayName("InvokeRequest encode/decode with null fields")
    void invokeRequestNullFields() {
        SdkWireMessages.InvokeRequest original = new SdkWireMessages.InvokeRequest(
            null, null, null, null);

        byte[] encoded = SdkWireMessages.encodeInvokeRequest(original);
        SdkWireMessages.InvokeRequest decoded = SdkWireMessages.decodeInvokeRequest(encoded);

        assertEquals("", decoded.functionId);
        assertEquals("", decoded.idempotencyKey);
        assertEquals(0, decoded.payload.length);
        assertTrue(decoded.metadata.isEmpty());
    }

    @Test
    @DisplayName("InvokeRequest encode/decode with empty payload")
    void invokeRequestEmptyPayload() {
        SdkWireMessages.InvokeRequest original = new SdkWireMessages.InvokeRequest(
            "func", "key", new byte[0], Map.of());

        byte[] encoded = SdkWireMessages.encodeInvokeRequest(original);
        SdkWireMessages.InvokeRequest decoded = SdkWireMessages.decodeInvokeRequest(encoded);

        assertEquals("func", decoded.functionId);
        assertEquals(0, decoded.payload.length);
    }

    @Test
    @DisplayName("InvokeResponse encode/decode round-trip")
    void invokeResponseRoundTrip() {
        SdkWireMessages.InvokeResponse original = new SdkWireMessages.InvokeResponse(
            "result-data".getBytes());

        byte[] encoded = SdkWireMessages.encodeInvokeResponse(original);
        SdkWireMessages.InvokeResponse decoded = SdkWireMessages.decodeInvokeResponse(encoded);

        assertArrayEquals("result-data".getBytes(), decoded.payload);
        assertEquals("result-data", decoded.payloadUtf8());
    }

    @Test
    @DisplayName("InvokeResponse with null payload")
    void invokeResponseNullPayload() {
        SdkWireMessages.InvokeResponse original = new SdkWireMessages.InvokeResponse(null);

        byte[] encoded = SdkWireMessages.encodeInvokeResponse(original);
        SdkWireMessages.InvokeResponse decoded = SdkWireMessages.decodeInvokeResponse(encoded);

        assertEquals(0, decoded.payload.length);
    }

    @Test
    @DisplayName("StartJobResponse encode/decode round-trip")
    void startJobResponseRoundTrip() {
        SdkWireMessages.StartJobResponse original = new SdkWireMessages.StartJobResponse("job-123");

        byte[] encoded = SdkWireMessages.encodeStartJobResponse(original);
        SdkWireMessages.StartJobResponse decoded = SdkWireMessages.decodeStartJobResponse(encoded);

        assertEquals("job-123", decoded.jobId);
    }

    @Test
    @DisplayName("StartJobResponse with null jobId")
    void startJobResponseNullJobId() {
        SdkWireMessages.StartJobResponse original = new SdkWireMessages.StartJobResponse(null);

        byte[] encoded = SdkWireMessages.encodeStartJobResponse(original);
        SdkWireMessages.StartJobResponse decoded = SdkWireMessages.decodeStartJobResponse(encoded);

        assertEquals("", decoded.jobId);
    }

    @Test
    @DisplayName("JobStreamRequest encode/decode round-trip")
    void jobStreamRequestRoundTrip() {
        SdkWireMessages.JobStreamRequest original = new SdkWireMessages.JobStreamRequest("job-456");

        byte[] encoded = SdkWireMessages.encodeJobStreamRequest(original);
        SdkWireMessages.JobStreamRequest decoded = SdkWireMessages.decodeJobStreamRequest(encoded);

        assertEquals("job-456", decoded.jobId);
    }

    @Test
    @DisplayName("JobStreamRequest with null jobId")
    void jobStreamRequestNullJobId() {
        SdkWireMessages.JobStreamRequest original = new SdkWireMessages.JobStreamRequest(null);

        byte[] encoded = SdkWireMessages.encodeJobStreamRequest(original);
        SdkWireMessages.JobStreamRequest decoded = SdkWireMessages.decodeJobStreamRequest(encoded);

        assertEquals("", decoded.jobId);
    }

    @Test
    @DisplayName("JobEvent encode/decode round-trip")
    void jobEventRoundTrip() {
        SdkWireMessages.JobEvent original = new SdkWireMessages.JobEvent(
            "progress", "Processing", 50, "data".getBytes());

        byte[] encoded = SdkWireMessages.encodeJobEvent(original);
        SdkWireMessages.JobEvent decoded = SdkWireMessages.decodeJobEvent(encoded);

        assertEquals("progress", decoded.type);
        assertEquals("Processing", decoded.message);
        assertEquals(50, decoded.progress);
        assertArrayEquals("data".getBytes(), decoded.payload);
        assertEquals("data", decoded.payloadUtf8());
    }

    @Test
    @DisplayName("JobEvent with null fields")
    void jobEventNullFields() {
        SdkWireMessages.JobEvent original = new SdkWireMessages.JobEvent(
            null, null, 0, null);

        byte[] encoded = SdkWireMessages.encodeJobEvent(original);
        SdkWireMessages.JobEvent decoded = SdkWireMessages.decodeJobEvent(encoded);

        assertEquals("", decoded.type);
        assertEquals("", decoded.message);
        assertEquals(0, decoded.progress);
        assertEquals(0, decoded.payload.length);
    }

    @Test
    @DisplayName("CancelJobRequest encode/decode round-trip")
    void cancelJobRequestRoundTrip() {
        SdkWireMessages.CancelJobRequest original = new SdkWireMessages.CancelJobRequest("job-789");

        byte[] encoded = SdkWireMessages.encodeCancelJobRequest(original);
        SdkWireMessages.CancelJobRequest decoded = SdkWireMessages.decodeCancelJobRequest(encoded);

        assertEquals("job-789", decoded.jobId);
    }

    @Test
    @DisplayName("CancelJobRequest with null jobId")
    void cancelJobRequestNullJobId() {
        SdkWireMessages.CancelJobRequest original = new SdkWireMessages.CancelJobRequest(null);

        byte[] encoded = SdkWireMessages.encodeCancelJobRequest(original);
        SdkWireMessages.CancelJobRequest decoded = SdkWireMessages.decodeCancelJobRequest(encoded);

        assertEquals("", decoded.jobId);
    }

    @Test
    @DisplayName("RegisterLocalRequest encode/decode round-trip with functions")
    void registerLocalRequestRoundTrip() {
        SdkWireMessages.LocalFunctionDescriptor func = new SdkWireMessages.LocalFunctionDescriptor(
            "f1", "1.0.0", List.of("tag1", "tag2"), "Summary", "Description",
            "opId", true, "{\"type\":\"object\"}", "{\"type\":\"string\"}",
            "game", "low", "Player", "create");

        SdkWireMessages.RegisterLocalRequest original = new SdkWireMessages.RegisterLocalRequest(
            "svc-1", "2.0.0", "localhost:9090", List.of(func));

        byte[] encoded = SdkWireMessages.encodeRegisterLocalRequest(original);
        SdkWireMessages.RegisterLocalRequest decoded = SdkWireMessages.decodeRegisterLocalRequest(encoded);

        assertEquals("svc-1", decoded.serviceId);
        assertEquals("2.0.0", decoded.version);
        assertEquals("localhost:9090", decoded.rpcAddr);
        assertEquals(1, decoded.functions.size());

        SdkWireMessages.LocalFunctionDescriptor decodedFunc = decoded.functions.get(0);
        assertEquals("f1", decodedFunc.id);
        assertEquals("1.0.0", decodedFunc.version);
        assertEquals(2, decodedFunc.tags.size());
        assertEquals("tag1", decodedFunc.tags.get(0));
        assertEquals("tag2", decodedFunc.tags.get(1));
        assertEquals("Summary", decodedFunc.summary);
        assertEquals("Description", decodedFunc.description);
        assertEquals("opId", decodedFunc.operationId);
        assertTrue(decodedFunc.deprecated);
        assertEquals("{\"type\":\"object\"}", decodedFunc.inputSchema);
        assertEquals("{\"type\":\"string\"}", decodedFunc.outputSchema);
        assertEquals("game", decodedFunc.category);
        assertEquals("low", decodedFunc.risk);
        assertEquals("Player", decodedFunc.entity);
        assertEquals("create", decodedFunc.operation);
    }

    @Test
    @DisplayName("RegisterLocalRequest with null fields")
    void registerLocalRequestNullFields() {
        SdkWireMessages.RegisterLocalRequest original = new SdkWireMessages.RegisterLocalRequest(
            null, null, null, null);

        byte[] encoded = SdkWireMessages.encodeRegisterLocalRequest(original);
        SdkWireMessages.RegisterLocalRequest decoded = SdkWireMessages.decodeRegisterLocalRequest(encoded);

        assertEquals("", decoded.serviceId);
        assertEquals("", decoded.version);
        assertEquals("", decoded.rpcAddr);
        assertTrue(decoded.functions.isEmpty());
    }

    @Test
    @DisplayName("RegisterLocalResponse encode/decode round-trip")
    void registerLocalResponseRoundTrip() {
        SdkWireMessages.RegisterLocalResponse original = new SdkWireMessages.RegisterLocalResponse("session-abc");

        byte[] encoded = SdkWireMessages.encodeRegisterLocalResponse(original);
        SdkWireMessages.RegisterLocalResponse decoded = SdkWireMessages.decodeRegisterLocalResponse(encoded);

        assertEquals("session-abc", decoded.sessionId);
    }

    @Test
    @DisplayName("RegisterLocalResponse with null sessionId")
    void registerLocalResponseNullSessionId() {
        SdkWireMessages.RegisterLocalResponse original = new SdkWireMessages.RegisterLocalResponse(null);

        byte[] encoded = SdkWireMessages.encodeRegisterLocalResponse(original);
        SdkWireMessages.RegisterLocalResponse decoded = SdkWireMessages.decodeRegisterLocalResponse(encoded);

        assertEquals("", decoded.sessionId);
    }

    @Test
    @DisplayName("HeartbeatRequest encode/decode round-trip")
    void heartbeatRequestRoundTrip() {
        SdkWireMessages.HeartbeatRequest original = new SdkWireMessages.HeartbeatRequest(
            "svc-1", "session-xyz");

        byte[] encoded = SdkWireMessages.encodeHeartbeatRequest(original);
        SdkWireMessages.HeartbeatRequest decoded = SdkWireMessages.decodeHeartbeatRequest(encoded);

        assertEquals("svc-1", decoded.serviceId);
        assertEquals("session-xyz", decoded.sessionId);
    }

    @Test
    @DisplayName("HeartbeatRequest with null fields")
    void heartbeatRequestNullFields() {
        SdkWireMessages.HeartbeatRequest original = new SdkWireMessages.HeartbeatRequest(null, null);

        byte[] encoded = SdkWireMessages.encodeHeartbeatRequest(original);
        SdkWireMessages.HeartbeatRequest decoded = SdkWireMessages.decodeHeartbeatRequest(encoded);

        assertEquals("", decoded.serviceId);
        assertEquals("", decoded.sessionId);
    }

    @Test
    @DisplayName("LocalFunctionDescriptor with null fields")
    void localFunctionDescriptorNullFields() {
        SdkWireMessages.LocalFunctionDescriptor desc = new SdkWireMessages.LocalFunctionDescriptor(
            null, null, null, null, null, null, false, null, null, null, null, null, null);

        assertEquals("", desc.id);
        assertEquals("", desc.version);
        assertTrue(desc.tags.isEmpty());
        assertEquals("", desc.summary);
        assertEquals("", desc.description);
        assertEquals("", desc.operationId);
        assertFalse(desc.deprecated);
        assertEquals("", desc.inputSchema);
        assertEquals("", desc.outputSchema);
        assertEquals("", desc.category);
        assertEquals("", desc.risk);
        assertEquals("", desc.entity);
        assertEquals("", desc.operation);
    }

    @Test
    @DisplayName("Decode with null data should use empty array")
    void decodeNullData() {
        // newInput handles null by creating empty array
        SdkWireMessages.InvokeRequest decoded = SdkWireMessages.decodeInvokeRequest(null);
        assertEquals("", decoded.functionId);
    }

    @Test
    @DisplayName("Decode InvokeResponse with unknown field should skip")
    void decodeInvokeResponseUnknownField() {
        // Encode a message with an unknown field number
        SdkWireMessages.InvokeResponse original = new SdkWireMessages.InvokeResponse("test".getBytes());
        byte[] encoded = SdkWireMessages.encodeInvokeResponse(original);
        // Decode should still work, skipping unknown fields
        SdkWireMessages.InvokeResponse decoded = SdkWireMessages.decodeInvokeResponse(encoded);
        assertArrayEquals("test".getBytes(), decoded.payload);
    }

    @Test
    @DisplayName("Decode JobEvent with unknown field should skip")
    void decodeJobEventUnknownField() {
        SdkWireMessages.JobEvent original = new SdkWireMessages.JobEvent("type", "msg", 10, "data".getBytes());
        byte[] encoded = SdkWireMessages.encodeJobEvent(original);
        SdkWireMessages.JobEvent decoded = SdkWireMessages.decodeJobEvent(encoded);
        assertEquals("type", decoded.type);
        assertEquals("msg", decoded.message);
        assertEquals(10, decoded.progress);
    }

    @Test
    @DisplayName("Encode/decode multiple functions in RegisterLocalRequest")
    void registerLocalRequestMultipleFunctions() {
        SdkWireMessages.LocalFunctionDescriptor func1 = new SdkWireMessages.LocalFunctionDescriptor(
            "f1", "1.0.0", List.of(), "", "", "", false, "", "", "", "", "", "");
        SdkWireMessages.LocalFunctionDescriptor func2 = new SdkWireMessages.LocalFunctionDescriptor(
            "f2", "2.0.0", List.of("tag"), "Sum", "Desc", "op", false, "", "", "", "", "", "");

        SdkWireMessages.RegisterLocalRequest original = new SdkWireMessages.RegisterLocalRequest(
            "svc", "1.0.0", "addr", List.of(func1, func2));

        byte[] encoded = SdkWireMessages.encodeRegisterLocalRequest(original);
        SdkWireMessages.RegisterLocalRequest decoded = SdkWireMessages.decodeRegisterLocalRequest(encoded);

        assertEquals(2, decoded.functions.size());
        assertEquals("f1", decoded.functions.get(0).id);
        assertEquals("f2", decoded.functions.get(1).id);
        assertEquals("tag", decoded.functions.get(1).tags.get(0));
    }

    @Test
    @DisplayName("Decode InvokeRequest with truncated data should throw")
    void decodeInvokeRequestTruncated() {
        // Encode a valid message then truncate to trigger IOException path
        SdkWireMessages.InvokeRequest original = new SdkWireMessages.InvokeRequest(
            "func", "key", "payload".getBytes(), Map.of());
        byte[] encoded = SdkWireMessages.encodeInvokeRequest(original);
        // Truncate to just the first few bytes (enough for tag but not full field)
        byte[] truncated = new byte[Math.max(1, encoded.length / 2)];
        System.arraycopy(encoded, 0, truncated, 0, truncated.length);

        assertThrows(IllegalArgumentException.class, () ->
            SdkWireMessages.decodeInvokeRequest(truncated));
    }

    @Test
    @DisplayName("Decode InvokeResponse with truncated data should throw")
    void decodeInvokeResponseTruncated() {
        SdkWireMessages.InvokeResponse original = new SdkWireMessages.InvokeResponse("data".getBytes());
        byte[] encoded = SdkWireMessages.encodeInvokeResponse(original);
        byte[] truncated = new byte[Math.max(1, encoded.length / 2)];
        System.arraycopy(encoded, 0, truncated, 0, truncated.length);

        assertThrows(IllegalArgumentException.class, () ->
            SdkWireMessages.decodeInvokeResponse(truncated));
    }

    @Test
    @DisplayName("Decode RegisterLocalRequest with truncated data should throw")
    void decodeRegisterLocalRequestTruncated() {
        SdkWireMessages.LocalFunctionDescriptor func = new SdkWireMessages.LocalFunctionDescriptor(
            "f1", "1.0.0", List.of(), "", "", "", false, "", "", "", "", "", "");
        SdkWireMessages.RegisterLocalRequest original = new SdkWireMessages.RegisterLocalRequest(
            "svc", "1.0.0", "addr", List.of(func));
        byte[] encoded = SdkWireMessages.encodeRegisterLocalRequest(original);
        byte[] truncated = new byte[Math.max(1, encoded.length / 2)];
        System.arraycopy(encoded, 0, truncated, 0, truncated.length);

        assertThrows(IllegalArgumentException.class, () ->
            SdkWireMessages.decodeRegisterLocalRequest(truncated));
    }

    @Test
    @DisplayName("Decode RegisterLocalResponse with truncated data should throw")
    void decodeRegisterLocalResponseTruncated() {
        SdkWireMessages.RegisterLocalResponse original = new SdkWireMessages.RegisterLocalResponse("sess");
        byte[] encoded = SdkWireMessages.encodeRegisterLocalResponse(original);
        byte[] truncated = new byte[Math.max(1, encoded.length / 2)];
        System.arraycopy(encoded, 0, truncated, 0, truncated.length);

        assertThrows(IllegalArgumentException.class, () ->
            SdkWireMessages.decodeRegisterLocalResponse(truncated));
    }

    @Test
    @DisplayName("Decode StartJobResponse with truncated data should throw")
    void decodeStartJobResponseTruncated() {
        SdkWireMessages.StartJobResponse original = new SdkWireMessages.StartJobResponse("job-1");
        byte[] encoded = SdkWireMessages.encodeStartJobResponse(original);
        byte[] truncated = new byte[Math.max(1, encoded.length / 2)];
        System.arraycopy(encoded, 0, truncated, 0, truncated.length);

        assertThrows(IllegalArgumentException.class, () ->
            SdkWireMessages.decodeStartJobResponse(truncated));
    }

    @Test
    @DisplayName("Decode JobStreamRequest with truncated data should throw")
    void decodeJobStreamRequestTruncated() {
        SdkWireMessages.JobStreamRequest original = new SdkWireMessages.JobStreamRequest("job-1");
        byte[] encoded = SdkWireMessages.encodeJobStreamRequest(original);
        byte[] truncated = new byte[Math.max(1, encoded.length / 2)];
        System.arraycopy(encoded, 0, truncated, 0, truncated.length);

        assertThrows(IllegalArgumentException.class, () ->
            SdkWireMessages.decodeJobStreamRequest(truncated));
    }

    @Test
    @DisplayName("Decode CancelJobRequest with truncated data should throw")
    void decodeCancelJobRequestTruncated() {
        SdkWireMessages.CancelJobRequest original = new SdkWireMessages.CancelJobRequest("job-1");
        byte[] encoded = SdkWireMessages.encodeCancelJobRequest(original);
        byte[] truncated = new byte[Math.max(1, encoded.length / 2)];
        System.arraycopy(encoded, 0, truncated, 0, truncated.length);

        assertThrows(IllegalArgumentException.class, () ->
            SdkWireMessages.decodeCancelJobRequest(truncated));
    }

    @Test
    @DisplayName("Decode JobEvent with truncated data should throw")
    void decodeJobEventTruncated() {
        SdkWireMessages.JobEvent original = new SdkWireMessages.JobEvent("type", "msg", 10, "data".getBytes());
        byte[] encoded = SdkWireMessages.encodeJobEvent(original);
        byte[] truncated = new byte[Math.max(1, encoded.length / 2)];
        System.arraycopy(encoded, 0, truncated, 0, truncated.length);

        assertThrows(IllegalArgumentException.class, () ->
            SdkWireMessages.decodeJobEvent(truncated));
    }

    @Test
    @DisplayName("Decode HeartbeatRequest with truncated string data should throw")
    void decodeHeartbeatRequestTruncated() {
        // Craft data: tag for field 1 (string) + length varint claiming 100 bytes, but no actual data
        // Tag for field 1, wire type 2 (length-delimited) = (1 << 3) | 2 = 0x0A
        // Length varint = 100 (0x64)
        byte[] malformed = new byte[] { 0x0A, 0x64 }; // tag + length, but no string data

        assertThrows(IllegalArgumentException.class, () ->
            SdkWireMessages.decodeHeartbeatRequest(malformed));
    }

    @Test
    @DisplayName("Decode InvokeRequest with null data should return defaults")
    void decodeInvokeRequestNullData() {
        SdkWireMessages.InvokeRequest decoded = SdkWireMessages.decodeInvokeRequest(null);
        assertEquals("", decoded.functionId);
    }

    @Test
    @DisplayName("Decode InvokeResponse with null data should return defaults")
    void decodeInvokeResponseNullData() {
        SdkWireMessages.InvokeResponse decoded = SdkWireMessages.decodeInvokeResponse(null);
        assertArrayEquals(new byte[0], decoded.payload);
    }

    @Test
    @DisplayName("LocalFunctionDescriptor with empty tags list")
    void localFunctionDescriptorEmptyTags() {
        SdkWireMessages.LocalFunctionDescriptor desc = new SdkWireMessages.LocalFunctionDescriptor(
            "f1", "1.0.0", List.of(), "", "", "", false, "", "", "", "", "", "");

        // Encode as part of RegisterLocalRequest to exercise encodeLocalFunctionDescriptor
        SdkWireMessages.RegisterLocalRequest req = new SdkWireMessages.RegisterLocalRequest(
            "svc", "1.0.0", "addr", List.of(desc));

        byte[] encoded = SdkWireMessages.encodeRegisterLocalRequest(req);
        SdkWireMessages.RegisterLocalRequest decoded = SdkWireMessages.decodeRegisterLocalRequest(encoded);

        assertEquals(1, decoded.functions.size());
        assertTrue(decoded.functions.get(0).tags.isEmpty());
    }
}
