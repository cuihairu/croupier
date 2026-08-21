package io.github.cuihairu.croupier.sdk.wire;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Edge case tests for SdkWireMessages: unknown fields, tag zero terminators,
 * truncated inputs and full-field round trips.
 */
@DisplayName("SdkWireMessages edge cases")
class SdkWireMessagesEdgeTest {

    /** Unknown field 99 with varint value 1. */
    private static final byte[] UNKNOWN_FIELD = {(byte) 0x98, 0x06, 0x01};
    /** A string field (field 1) that claims 5 bytes but only carries 1. */
    private static final byte[] TRUNCATED_STRING = {0x0A, 0x05, 'a'};

    private static byte[] concat(byte[]... parts) {
        int length = 0;
        for (byte[] part : parts) {
            length += part.length;
        }
        byte[] result = new byte[length];
        int offset = 0;
        for (byte[] part : parts) {
            System.arraycopy(part, 0, result, offset, part.length);
            offset += part.length;
        }
        return result;
    }

    @Test
    @DisplayName("decoders skip unknown fields")
    void unknownFieldsAndTagZero() {
        SdkWireMessages.InvokeRequest invoke = new SdkWireMessages.InvokeRequest("f", "idem", "p".getBytes(), Map.of("k", "v"));
        SdkWireMessages.InvokeRequest invokeDecoded = SdkWireMessages.decodeInvokeRequest(
            concat(SdkWireMessages.encodeInvokeRequest(invoke), UNKNOWN_FIELD));
        assertEquals("f", invokeDecoded.functionId);
        assertEquals("idem", invokeDecoded.idempotencyKey);
        assertEquals("v", invokeDecoded.metadata.get("k"));

        SdkWireMessages.InvokeResponse invokeResponse = SdkWireMessages.decodeInvokeResponse(
            concat(SdkWireMessages.encodeInvokeResponse(new SdkWireMessages.InvokeResponse("r".getBytes())), UNKNOWN_FIELD));
        assertEquals("r", invokeResponse.payloadUtf8());

        SdkWireMessages.StartTaskResponse startTask = SdkWireMessages.decodeStartTaskResponse(
            concat(SdkWireMessages.encodeStartTaskResponse(new SdkWireMessages.StartTaskResponse("t1")), UNKNOWN_FIELD));
        assertEquals("t1", startTask.taskId);

        SdkWireMessages.TaskStreamRequest stream = SdkWireMessages.decodeTaskStreamRequest(
            concat(SdkWireMessages.encodeTaskStreamRequest(new SdkWireMessages.TaskStreamRequest("t2")), UNKNOWN_FIELD));
        assertEquals("t2", stream.taskId);

        SdkWireMessages.TaskEvent event = SdkWireMessages.decodeTaskEvent(
            concat(SdkWireMessages.encodeTaskEvent(new SdkWireMessages.TaskEvent("done", "msg", 7, "pl".getBytes())), UNKNOWN_FIELD));
        assertEquals("done", event.type);
        assertEquals("msg", event.message);
        assertEquals(7, event.progress);

        SdkWireMessages.CancelTaskRequest cancel = SdkWireMessages.decodeCancelTaskRequest(
            concat(SdkWireMessages.encodeCancelTaskRequest(new SdkWireMessages.CancelTaskRequest("t3")), UNKNOWN_FIELD));
        assertEquals("t3", cancel.taskId);

        SdkWireMessages.ProviderConnectResponse connectResponse = SdkWireMessages.decodeProviderConnectResponse(
            concat(SdkWireMessages.encodeProviderConnectResponse(new SdkWireMessages.ProviderConnectResponse("s1")), UNKNOWN_FIELD));
        assertEquals("s1", connectResponse.sessionId);

        SdkWireMessages.HeartbeatRequest heartbeat = SdkWireMessages.decodeHeartbeatRequest(
            concat(SdkWireMessages.encodeHeartbeatRequest(new SdkWireMessages.HeartbeatRequest("svc", "s2")), UNKNOWN_FIELD));
        assertEquals("svc", heartbeat.serviceId);
        assertEquals("s2", heartbeat.sessionId);

        SdkWireMessages.ProviderConnectRequest connect = new SdkWireMessages.ProviderConnectRequest(
            "svc", "1.0.0", "addr", List.of());
        SdkWireMessages.ProviderConnectRequest connectDecoded = SdkWireMessages.decodeProviderConnectRequest(
            concat(SdkWireMessages.encodeProviderConnectRequest(connect), UNKNOWN_FIELD));
        assertEquals("svc", connectDecoded.serviceId);
    }

    @Test
    @DisplayName("truncated inputs fail with IllegalArgumentException per message type")
    void truncatedInputs() {
        assertThrows(IllegalArgumentException.class, () -> SdkWireMessages.decodeInvokeRequest(TRUNCATED_STRING));
        assertThrows(IllegalArgumentException.class, () -> SdkWireMessages.decodeInvokeResponse(TRUNCATED_STRING));
        assertThrows(IllegalArgumentException.class, () -> SdkWireMessages.decodeStartTaskResponse(TRUNCATED_STRING));
        assertThrows(IllegalArgumentException.class, () -> SdkWireMessages.decodeTaskStreamRequest(TRUNCATED_STRING));
        assertThrows(IllegalArgumentException.class, () -> SdkWireMessages.decodeTaskEvent(TRUNCATED_STRING));
        assertThrows(IllegalArgumentException.class, () -> SdkWireMessages.decodeCancelTaskRequest(TRUNCATED_STRING));
        assertThrows(IllegalArgumentException.class, () -> SdkWireMessages.decodeProviderConnectResponse(TRUNCATED_STRING));
        assertThrows(IllegalArgumentException.class, () -> SdkWireMessages.decodeHeartbeatRequest(TRUNCATED_STRING));

        // Field 3 (functions message) whose nested descriptor bytes are truncated.
        byte[] badNested = {0x1A, 0x03, 0x0A, 0x05, 'a'};
        IllegalArgumentException nested = assertThrows(IllegalArgumentException.class,
            () -> SdkWireMessages.decodeProviderConnectRequest(badNested));
        assertTrue(nested.getMessage().contains("Failed to decode ProviderConnectRequest")
            || nested.getMessage().contains("Failed to decode ProviderFunctionDescriptor"), nested.getMessage());

        // Field 4 (metadata entry) with truncated nested bytes.
        byte[] badMetadata = {0x22, 0x03, 0x0A, 0x05, 'a'};
        IllegalArgumentException metadata = assertThrows(IllegalArgumentException.class,
            () -> SdkWireMessages.decodeInvokeRequest(badMetadata));
        assertTrue(metadata.getMessage().contains("Failed to decode metadata entry")
            || metadata.getMessage().contains("Failed to decode InvokeRequest"), metadata.getMessage());
    }

    @Test
    @DisplayName("metadata entries with only unknown fields are dropped")
    void metadataEntryUnknownFieldsOnly() {
        byte[] encoded = concat(
            SdkWireMessages.encodeInvokeRequest(new SdkWireMessages.InvokeRequest("f", "", new byte[0], Map.of())),
            // Field 4 (LEN) containing field 9 varint — no key/value present.
            new byte[]{0x22, 0x02, 0x48, 0x01});
        SdkWireMessages.InvokeRequest decoded = SdkWireMessages.decodeInvokeRequest(encoded);
        assertTrue(decoded.metadata.isEmpty());
    }

    @Test
    @DisplayName("ProviderConnectRequest round-trips sdk fields and full function descriptors")
    void providerConnectRequestFullRoundTrip() {
        SdkWireMessages.ProviderFunctionDescriptor descriptor = new SdkWireMessages.ProviderFunctionDescriptor(
            "fn-1", "2.0.0", List.of("tag1", "tag2"), "summary", "description", "op-id", true,
            "input-schema", "output-schema", "resource", "operation", "capability-1", "execution-1",
            true, "policy-key", "risk", false, "permission");

        SdkWireMessages.ProviderConnectRequest request = new SdkWireMessages.ProviderConnectRequest(
            "svc", "3.0.0", "127.0.0.1:1", List.of(descriptor), "java", "9.9.9", "croupier-test", "2.0");
        byte[] encoded = SdkWireMessages.encodeProviderConnectRequest(request);
        SdkWireMessages.ProviderConnectRequest decoded = SdkWireMessages.decodeProviderConnectRequest(encoded);

        assertEquals("svc", decoded.serviceId);
        assertEquals("3.0.0", decoded.version);
        assertEquals("java", decoded.sdkLanguage);
        assertEquals("9.9.9", decoded.sdkVersion);
        assertEquals("croupier-test", decoded.sdkName);
        assertEquals("2.0", decoded.protocolVersion);
        assertEquals(1, decoded.functions.size());

        SdkWireMessages.ProviderFunctionDescriptor wire = decoded.functions.get(0);
        assertEquals("fn-1", wire.id);
        assertEquals("2.0.0", wire.version);
        assertEquals(List.of("tag1", "tag2"), wire.tags);
        assertEquals("summary", wire.summary);
        assertEquals("description", wire.description);
        assertEquals("op-id", wire.operationId);
        assertTrue(wire.deprecated);
        assertEquals("input-schema", wire.inputSchema);
        assertEquals("output-schema", wire.outputSchema);
        assertEquals("resource", wire.resource);
        assertEquals("operation", wire.operation);
        assertEquals("capability-1", wire.capability);
        assertEquals("execution-1", wire.execution);
        assertTrue(wire.approvalRequired);
        assertEquals("policy-key", wire.approvalPolicyKey);
        assertEquals("risk", wire.risk);
        assertFalse(wire.enabled);
        assertEquals("permission", wire.permission);
    }

    @Test
    @DisplayName("ProviderConnectRequest null constructor arguments normalize to empty values")
    void providerConnectRequestNulls() {
        SdkWireMessages.ProviderConnectRequest request = new SdkWireMessages.ProviderConnectRequest(
            null, null, null, null, null, null, null, null);
        assertEquals("", request.serviceId);
        assertEquals("", request.version);
        assertEquals("", request.rpcAddr);
        assertEquals(List.of(), request.functions);
        assertEquals("", request.sdkLanguage);
        assertEquals("", request.sdkVersion);
        assertEquals("", request.sdkName);
        assertEquals("", request.protocolVersion);
    }

    @Test
    @DisplayName("decoders accept empty and null inputs")
    void emptyAndNullInputs() {
        assertEquals("", SdkWireMessages.decodeInvokeRequest(null).functionId);
        assertEquals("", SdkWireMessages.decodeInvokeResponse(new byte[0]).payloadUtf8());
        assertEquals("", SdkWireMessages.decodeStartTaskResponse(null).taskId);
        assertEquals("", SdkWireMessages.decodeTaskStreamRequest(null).taskId);
        assertEquals("", SdkWireMessages.decodeTaskEvent(new byte[0]).type);
        assertEquals("", SdkWireMessages.decodeCancelTaskRequest(null).taskId);
        assertEquals("", SdkWireMessages.decodeProviderConnectResponse(null).sessionId);
        assertEquals("", SdkWireMessages.decodeHeartbeatRequest(null).serviceId);
        assertEquals("", SdkWireMessages.decodeProviderConnectRequest(new byte[0]).serviceId);
    }
}
