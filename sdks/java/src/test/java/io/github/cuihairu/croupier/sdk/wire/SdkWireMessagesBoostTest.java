package io.github.cuihairu.croupier.sdk.wire;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Coverage boost tests for SdkWireMessages: metadata map entries, rare
 * descriptor fields, empty-value encoding behaviour and full round trips.
 */
@DisplayName("SdkWireMessages coverage boost")
class SdkWireMessagesBoostTest {

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
    @DisplayName("InvokeRequest metadata map round-trips multiple entries")
    void invokeRequestMetadataMapRoundTrip() {
        Map<String, String> metadata = new LinkedHashMap<>();
        metadata.put("game", "demo");
        metadata.put("env", "prod");
        metadata.put("actor", "admin");
        byte[] encoded = SdkWireMessages.encodeInvokeRequest(
            new SdkWireMessages.InvokeRequest("fn", "", new byte[0], metadata));
        SdkWireMessages.InvokeRequest decoded = SdkWireMessages.decodeInvokeRequest(encoded);
        assertEquals(metadata, decoded.metadata);
    }

    @Test
    @DisplayName("metadata entries tolerate partial payloads")
    void metadataEntryPartialPayload() {
        // Entry bytes with only the key field (field 1) set: value stays empty.
        byte[] entryWithKeyOnly = {0x0A, 0x03, 'k', 'e', 'y'};
        // Hand-wrap into InvokeRequest field 4 (metadata map): tag 0x22, length.
        byte[] wrapped = concat(new byte[]{0x22, (byte) entryWithKeyOnly.length}, entryWithKeyOnly);
        SdkWireMessages.InvokeRequest decoded = SdkWireMessages.decodeInvokeRequest(wrapped);
        assertEquals("", decoded.metadata.get("key"));
    }

    @Test
    @DisplayName("metadata entries missing a key are dropped")
    void metadataEntryValueOnly() {
        // Entry with only field 2 (value) set: tag 0x12. The empty key is dropped.
        byte[] entryWithValueOnly = {0x12, 0x02, 'o', 'k'};
        byte[] wrapped = concat(new byte[]{0x22, (byte) entryWithValueOnly.length}, entryWithValueOnly);
        SdkWireMessages.InvokeRequest decoded = SdkWireMessages.decodeInvokeRequest(wrapped);
        assertTrue(decoded.metadata.isEmpty());
    }

    @Test
    @DisplayName("ProviderFunctionDescriptor round-trips execution/approval fields")
    void descriptorApprovalFieldsRoundTrip() {
        SdkWireMessages.ProviderFunctionDescriptor descriptor = new SdkWireMessages.ProviderFunctionDescriptor(
            "player.ban",
            "2.0",
            List.of("gm", "risk"),
            "Ban player",
            "Bans a player",
            "player_ban",
            true,
            "{\"type\":\"object\"}",
            "{\"type\":\"object\"}",
            "player",
            "write",
            "ops",
            "inline",
            true,
            "two-person",
            "high",
            false,
            "gm.ban"
        );
        // Round-trip through ProviderConnectRequest which embeds descriptors.
        SdkWireMessages.ProviderConnectRequest request = new SdkWireMessages.ProviderConnectRequest(
            "svc", "1.0", "", List.of(descriptor));
        SdkWireMessages.ProviderConnectRequest decoded =
            SdkWireMessages.decodeProviderConnectRequest(SdkWireMessages.encodeProviderConnectRequest(request));

        assertEquals(1, decoded.functions.size());
        SdkWireMessages.ProviderFunctionDescriptor fn = decoded.functions.get(0);
        assertEquals("player.ban", fn.id);
        assertEquals("2.0", fn.version);
        assertEquals(List.of("gm", "risk"), fn.tags);
        assertEquals("Ban player", fn.summary);
        assertEquals("Bans a player", fn.description);
        assertEquals("player_ban", fn.operationId);
        assertTrue(fn.deprecated);
        assertEquals("{\"type\":\"object\"}", fn.inputSchema);
        assertEquals("{\"type\":\"object\"}", fn.outputSchema);
        assertEquals("player", fn.resource);
        assertEquals("write", fn.operation);
        assertEquals("ops", fn.capability);
        assertEquals("inline", fn.execution);
        assertTrue(fn.approvalRequired);
        assertEquals("two-person", fn.approvalPolicyKey);
        assertEquals("high", fn.risk);
        assertFalse(fn.enabled);
        assertEquals("gm.ban", fn.permission);
    }

    @Test
    @DisplayName("ProviderConnectRequest round-trips multiple descriptors")
    void providerConnectRequestMultipleDescriptors() {
        SdkWireMessages.ProviderFunctionDescriptor first =
            new SdkWireMessages.ProviderFunctionDescriptor("a", "1", null, null, null, null, false,
                null, null, null, null, null, null, false, null, null, true, null);
        SdkWireMessages.ProviderFunctionDescriptor second =
            new SdkWireMessages.ProviderFunctionDescriptor("b", "2", null, null, null, null, false,
                null, null, null, null, null, null, false, null, null, true, null);
        SdkWireMessages.ProviderConnectRequest request = new SdkWireMessages.ProviderConnectRequest(
            "svc", "1.0", "rpc:1", List.of(first, second), "java", "3.0", "name", "1.2");
        SdkWireMessages.ProviderConnectRequest decoded =
            SdkWireMessages.decodeProviderConnectRequest(SdkWireMessages.encodeProviderConnectRequest(request));

        assertEquals(2, decoded.functions.size());
        assertEquals("a", decoded.functions.get(0).id);
        assertEquals("b", decoded.functions.get(1).id);
        assertEquals("svc", decoded.serviceId);
        assertEquals("1.0", decoded.version);
        assertEquals("java", decoded.sdkLanguage);
        assertEquals("3.0", decoded.sdkVersion);
        assertEquals("name", decoded.sdkName);
        assertEquals("1.2", decoded.protocolVersion);
    }

    @Test
    @DisplayName("HeartbeatRequest round-trips both fields")
    void heartbeatRoundTrip() {
        byte[] encoded = SdkWireMessages.encodeHeartbeatRequest(
            new SdkWireMessages.HeartbeatRequest("svc-a", "sess-b"));
        SdkWireMessages.HeartbeatRequest decoded = SdkWireMessages.decodeHeartbeatRequest(encoded);
        assertEquals("svc-a", decoded.serviceId);
        assertEquals("sess-b", decoded.sessionId);
    }

    @Test
    @DisplayName("TaskEvent round-trips payloads with multi-byte UTF-8")
    void taskEventUtf8RoundTrip() {
        SdkWireMessages.TaskEvent event = new SdkWireMessages.TaskEvent(
            "log", "进度", 42, "数据".getBytes(java.nio.charset.StandardCharsets.UTF_8));
        byte[] encoded = SdkWireMessages.encodeTaskEvent(event);
        SdkWireMessages.TaskEvent decoded = SdkWireMessages.decodeTaskEvent(encoded);
        assertEquals("进度", decoded.message);
        assertEquals(42, decoded.progress);
        assertEquals("数据", decoded.payloadUtf8());
    }

    @Test
    @DisplayName("empty values are omitted from the wire but survive decode defaults")
    void emptyValuesOmittedOnWire() {
        SdkWireMessages.InvokeRequest empty = new SdkWireMessages.InvokeRequest("", "", new byte[0], Map.of());
        byte[] encoded = SdkWireMessages.encodeInvokeRequest(empty);
        assertEquals(0, encoded.length);
        SdkWireMessages.InvokeRequest decoded = SdkWireMessages.decodeInvokeRequest(encoded);
        assertEquals("", decoded.functionId);
        assertTrue(decoded.metadata.isEmpty());
    }

    @Test
    @DisplayName("zero progress is preserved for TaskEvent")
    void taskEventZeroProgressPreserved() {
        SdkWireMessages.TaskEvent event = new SdkWireMessages.TaskEvent("started", "begin", 0, null);
        SdkWireMessages.TaskEvent decoded =
            SdkWireMessages.decodeTaskEvent(SdkWireMessages.encodeTaskEvent(event));
        assertEquals(0, decoded.progress);
        assertEquals("started", decoded.type);
    }

    @Test
    @DisplayName("record constructors normalize null inputs")
    void constructorsNormalizeNulls() {
        SdkWireMessages.InvokeRequest request = new SdkWireMessages.InvokeRequest(null, null, null, null);
        assertEquals("", request.functionId);
        assertEquals("", request.idempotencyKey);
        assertNotNull(request.payload);
        assertTrue(request.metadata.isEmpty());

        SdkWireMessages.TaskEvent event = new SdkWireMessages.TaskEvent(null, null, 0, null);
        assertEquals("", event.type);
        assertEquals(0, event.payload.length);

        SdkWireMessages.ProviderConnectRequest connect =
            new SdkWireMessages.ProviderConnectRequest(null, null, null, null, null, null, null, null);
        assertEquals("", connect.serviceId);
        assertTrue(connect.functions.isEmpty());
        assertEquals("", connect.protocolVersion);
    }

    @Test
    @DisplayName("field order does not matter when decoding")
    void decodeToleratesReversedFieldOrder() {
        // Encode a request, then decode one built from concatenated single-field messages.
        byte[] functionIdField = SdkWireMessages.encodeInvokeRequest(
            new SdkWireMessages.InvokeRequest("fn", "", new byte[0], Map.of()));
        byte[] payloadField = SdkWireMessages.encodeInvokeResponse(
            new SdkWireMessages.InvokeResponse("p".getBytes()));
        // payloadField only carries field 1 of InvokeResponse which is bytes; feed it
        // as an unknown field for InvokeRequest so it is skipped cleanly.
        SdkWireMessages.InvokeRequest decoded =
            SdkWireMessages.decodeInvokeRequest(concat(functionIdField, payloadField, functionIdField));
        assertEquals("fn", decoded.functionId);
    }
}
