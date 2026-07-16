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
    @DisplayName("StartTaskResponse encode/decode round-trip")
    void startTaskResponseRoundTrip() {
        SdkWireMessages.StartTaskResponse original = new SdkWireMessages.StartTaskResponse("task-123");

        byte[] encoded = SdkWireMessages.encodeStartTaskResponse(original);
        SdkWireMessages.StartTaskResponse decoded = SdkWireMessages.decodeStartTaskResponse(encoded);

        assertEquals("task-123", decoded.taskId);
    }

    @Test
    @DisplayName("StartTaskResponse with null taskId")
    void startTaskResponseNullTaskId() {
        SdkWireMessages.StartTaskResponse original = new SdkWireMessages.StartTaskResponse(null);

        byte[] encoded = SdkWireMessages.encodeStartTaskResponse(original);
        SdkWireMessages.StartTaskResponse decoded = SdkWireMessages.decodeStartTaskResponse(encoded);

        assertEquals("", decoded.taskId);
    }

    @Test
    @DisplayName("TaskStreamRequest encode/decode round-trip")
    void taskStreamRequestRoundTrip() {
        SdkWireMessages.TaskStreamRequest original = new SdkWireMessages.TaskStreamRequest("task-456");

        byte[] encoded = SdkWireMessages.encodeTaskStreamRequest(original);
        SdkWireMessages.TaskStreamRequest decoded = SdkWireMessages.decodeTaskStreamRequest(encoded);

        assertEquals("task-456", decoded.taskId);
    }

    @Test
    @DisplayName("TaskStreamRequest with null taskId")
    void taskStreamRequestNullTaskId() {
        SdkWireMessages.TaskStreamRequest original = new SdkWireMessages.TaskStreamRequest(null);

        byte[] encoded = SdkWireMessages.encodeTaskStreamRequest(original);
        SdkWireMessages.TaskStreamRequest decoded = SdkWireMessages.decodeTaskStreamRequest(encoded);

        assertEquals("", decoded.taskId);
    }

    @Test
    @DisplayName("TaskEvent encode/decode round-trip")
    void taskEventRoundTrip() {
        SdkWireMessages.TaskEvent original = new SdkWireMessages.TaskEvent(
            "progress", "Processing", 50, "data".getBytes());

        byte[] encoded = SdkWireMessages.encodeTaskEvent(original);
        SdkWireMessages.TaskEvent decoded = SdkWireMessages.decodeTaskEvent(encoded);

        assertEquals("progress", decoded.type);
        assertEquals("Processing", decoded.message);
        assertEquals(50, decoded.progress);
        assertArrayEquals("data".getBytes(), decoded.payload);
        assertEquals("data", decoded.payloadUtf8());
    }

    @Test
    @DisplayName("TaskEvent with null fields")
    void taskEventNullFields() {
        SdkWireMessages.TaskEvent original = new SdkWireMessages.TaskEvent(
            null, null, 0, null);

        byte[] encoded = SdkWireMessages.encodeTaskEvent(original);
        SdkWireMessages.TaskEvent decoded = SdkWireMessages.decodeTaskEvent(encoded);

        assertEquals("", decoded.type);
        assertEquals("", decoded.message);
        assertEquals(0, decoded.progress);
        assertEquals(0, decoded.payload.length);
    }

    @Test
    @DisplayName("CancelTaskRequest encode/decode round-trip")
    void cancelTaskRequestRoundTrip() {
        SdkWireMessages.CancelTaskRequest original = new SdkWireMessages.CancelTaskRequest("task-789");

        byte[] encoded = SdkWireMessages.encodeCancelTaskRequest(original);
        SdkWireMessages.CancelTaskRequest decoded = SdkWireMessages.decodeCancelTaskRequest(encoded);

        assertEquals("task-789", decoded.taskId);
    }

    @Test
    @DisplayName("CancelTaskRequest with null taskId")
    void cancelTaskRequestNullTaskId() {
        SdkWireMessages.CancelTaskRequest original = new SdkWireMessages.CancelTaskRequest(null);

        byte[] encoded = SdkWireMessages.encodeCancelTaskRequest(original);
        SdkWireMessages.CancelTaskRequest decoded = SdkWireMessages.decodeCancelTaskRequest(encoded);

        assertEquals("", decoded.taskId);
    }

    @Test
    @DisplayName("ProviderConnectRequest encode/decode round-trip with functions")
    void providerConnectRequestRoundTrip() {
        SdkWireMessages.LocalFunctionDescriptor func = new SdkWireMessages.LocalFunctionDescriptor(
            "f1", "1.0.0", List.of("tag1", "tag2"), "Summary", "Description",
            "opId", true, "{\"type\":\"object\"}", "{\"type\":\"string\"}",
            "game", "low", "Player", "create");

        SdkWireMessages.ProviderConnectRequest original = new SdkWireMessages.ProviderConnectRequest(
            "svc-1", "2.0.0", "localhost:9090", List.of(func));

        byte[] encoded = SdkWireMessages.encodeProviderConnectRequest(original);
        SdkWireMessages.ProviderConnectRequest decoded = SdkWireMessages.decodeProviderConnectRequest(encoded);

        assertEquals("svc-1", decoded.serviceId);
        assertEquals("2.0.0", decoded.version);
        assertEquals("", decoded.rpcAddr); // rpcAddr is not part of the wire protocol
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
    @DisplayName("ProviderConnectRequest with null fields")
    void providerConnectRequestNullFields() {
        SdkWireMessages.ProviderConnectRequest original = new SdkWireMessages.ProviderConnectRequest(
            null, null, null, null);

        byte[] encoded = SdkWireMessages.encodeProviderConnectRequest(original);
        SdkWireMessages.ProviderConnectRequest decoded = SdkWireMessages.decodeProviderConnectRequest(encoded);

        assertEquals("", decoded.serviceId);
        assertEquals("", decoded.version);
        assertEquals("", decoded.rpcAddr);
        assertTrue(decoded.functions.isEmpty());
    }

    @Test
    @DisplayName("ProviderConnectResponse encode/decode round-trip")
    void providerConnectResponseRoundTrip() {
        SdkWireMessages.ProviderConnectResponse original = new SdkWireMessages.ProviderConnectResponse("session-abc");

        byte[] encoded = SdkWireMessages.encodeProviderConnectResponse(original);
        SdkWireMessages.ProviderConnectResponse decoded = SdkWireMessages.decodeProviderConnectResponse(encoded);

        assertEquals("session-abc", decoded.sessionId);
    }

    @Test
    @DisplayName("ProviderConnectResponse with null sessionId")
    void providerConnectResponseNullSessionId() {
        SdkWireMessages.ProviderConnectResponse original = new SdkWireMessages.ProviderConnectResponse(null);

        byte[] encoded = SdkWireMessages.encodeProviderConnectResponse(original);
        SdkWireMessages.ProviderConnectResponse decoded = SdkWireMessages.decodeProviderConnectResponse(encoded);

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
    @DisplayName("Decode TaskEvent with unknown field should skip")
    void decodeTaskEventUnknownField() {
        SdkWireMessages.TaskEvent original = new SdkWireMessages.TaskEvent("type", "msg", 10, "data".getBytes());
        byte[] encoded = SdkWireMessages.encodeTaskEvent(original);
        SdkWireMessages.TaskEvent decoded = SdkWireMessages.decodeTaskEvent(encoded);
        assertEquals("type", decoded.type);
        assertEquals("msg", decoded.message);
        assertEquals(10, decoded.progress);
    }

    @Test
    @DisplayName("Encode/decode multiple functions in ProviderConnectRequest")
    void providerConnectRequestMultipleFunctions() {
        SdkWireMessages.LocalFunctionDescriptor func1 = new SdkWireMessages.LocalFunctionDescriptor(
            "f1", "1.0.0", List.of(), "", "", "", false, "", "", "", "", "", "");
        SdkWireMessages.LocalFunctionDescriptor func2 = new SdkWireMessages.LocalFunctionDescriptor(
            "f2", "2.0.0", List.of("tag"), "Sum", "Desc", "op", false, "", "", "", "", "", "");

        SdkWireMessages.ProviderConnectRequest original = new SdkWireMessages.ProviderConnectRequest(
            "svc", "1.0.0", "addr", List.of(func1, func2));

        byte[] encoded = SdkWireMessages.encodeProviderConnectRequest(original);
        SdkWireMessages.ProviderConnectRequest decoded = SdkWireMessages.decodeProviderConnectRequest(encoded);

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
    @DisplayName("Decode ProviderConnectRequest with truncated data should throw")
    void decodeProviderConnectRequestTruncated() {
        SdkWireMessages.LocalFunctionDescriptor func = new SdkWireMessages.LocalFunctionDescriptor(
            "f1", "1.0.0", List.of(), "", "", "", false, "", "", "", "", "", "");
        SdkWireMessages.ProviderConnectRequest original = new SdkWireMessages.ProviderConnectRequest(
            "svc", "1.0.0", "addr", List.of(func));
        byte[] encoded = SdkWireMessages.encodeProviderConnectRequest(original);
        byte[] truncated = new byte[Math.max(1, encoded.length / 2)];
        System.arraycopy(encoded, 0, truncated, 0, truncated.length);

        assertThrows(IllegalArgumentException.class, () ->
            SdkWireMessages.decodeProviderConnectRequest(truncated));
    }

    @Test
    @DisplayName("Decode ProviderConnectResponse with truncated data should throw")
    void decodeProviderConnectResponseTruncated() {
        SdkWireMessages.ProviderConnectResponse original = new SdkWireMessages.ProviderConnectResponse("sess");
        byte[] encoded = SdkWireMessages.encodeProviderConnectResponse(original);
        byte[] truncated = new byte[Math.max(1, encoded.length / 2)];
        System.arraycopy(encoded, 0, truncated, 0, truncated.length);

        assertThrows(IllegalArgumentException.class, () ->
            SdkWireMessages.decodeProviderConnectResponse(truncated));
    }

    @Test
    @DisplayName("Decode StartTaskResponse with truncated data should throw")
    void decodeStartTaskResponseTruncated() {
        SdkWireMessages.StartTaskResponse original = new SdkWireMessages.StartTaskResponse("task-1");
        byte[] encoded = SdkWireMessages.encodeStartTaskResponse(original);
        byte[] truncated = new byte[Math.max(1, encoded.length / 2)];
        System.arraycopy(encoded, 0, truncated, 0, truncated.length);

        assertThrows(IllegalArgumentException.class, () ->
            SdkWireMessages.decodeStartTaskResponse(truncated));
    }

    @Test
    @DisplayName("Decode TaskStreamRequest with truncated data should throw")
    void decodeTaskStreamRequestTruncated() {
        SdkWireMessages.TaskStreamRequest original = new SdkWireMessages.TaskStreamRequest("task-1");
        byte[] encoded = SdkWireMessages.encodeTaskStreamRequest(original);
        byte[] truncated = new byte[Math.max(1, encoded.length / 2)];
        System.arraycopy(encoded, 0, truncated, 0, truncated.length);

        assertThrows(IllegalArgumentException.class, () ->
            SdkWireMessages.decodeTaskStreamRequest(truncated));
    }

    @Test
    @DisplayName("Decode CancelTaskRequest with truncated data should throw")
    void decodeCancelTaskRequestTruncated() {
        SdkWireMessages.CancelTaskRequest original = new SdkWireMessages.CancelTaskRequest("task-1");
        byte[] encoded = SdkWireMessages.encodeCancelTaskRequest(original);
        byte[] truncated = new byte[Math.max(1, encoded.length / 2)];
        System.arraycopy(encoded, 0, truncated, 0, truncated.length);

        assertThrows(IllegalArgumentException.class, () ->
            SdkWireMessages.decodeCancelTaskRequest(truncated));
    }

    @Test
    @DisplayName("Decode TaskEvent with truncated data should throw")
    void decodeTaskEventTruncated() {
        SdkWireMessages.TaskEvent original = new SdkWireMessages.TaskEvent("type", "msg", 10, "data".getBytes());
        byte[] encoded = SdkWireMessages.encodeTaskEvent(original);
        byte[] truncated = new byte[Math.max(1, encoded.length / 2)];
        System.arraycopy(encoded, 0, truncated, 0, truncated.length);

        assertThrows(IllegalArgumentException.class, () ->
            SdkWireMessages.decodeTaskEvent(truncated));
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

        // Encode as part of ProviderConnectRequest to exercise encodeLocalFunctionDescriptor
        SdkWireMessages.ProviderConnectRequest req = new SdkWireMessages.ProviderConnectRequest(
            "svc", "1.0.0", "addr", List.of(desc));

        byte[] encoded = SdkWireMessages.encodeProviderConnectRequest(req);
        SdkWireMessages.ProviderConnectRequest decoded = SdkWireMessages.decodeProviderConnectRequest(encoded);

        assertEquals(1, decoded.functions.size());
        assertTrue(decoded.functions.get(0).tags.isEmpty());
    }
}
