/**
 * SdkWireMessages 解码失败分支与 encode/writeMap 边界补测。
 * 每个 decode 的 catch (IOException) → IllegalArgumentException 路径。
 */
package io.github.cuihairu.croupier.sdk.wire;

import io.github.cuihairu.croupier.sdk.wire.SdkWireMessages.InvokeRequest;
import io.github.cuihairu.croupier.sdk.wire.SdkWireMessages.InvokeResponse;
import io.github.cuihairu.croupier.sdk.wire.SdkWireMessages.StartTaskResponse;
import io.github.cuihairu.croupier.sdk.wire.SdkWireMessages.TaskStreamRequest;
import io.github.cuihairu.croupier.sdk.wire.SdkWireMessages.TaskEvent;
import io.github.cuihairu.croupier.sdk.wire.SdkWireMessages.HeartbeatRequest;
import io.github.cuihairu.croupier.sdk.wire.SdkWireMessages.ProviderFunctionDescriptor;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.util.LinkedHashMap;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.*;

@DisplayName("SdkWireMessages edge branches")
class SdkWireMessagesEdgeTest {

    /** 非法 protobuf：截断的 varint/字符串长度——触发各 decode 的 catch 分支。 */
    private static final byte[] GARBAGE = {(byte) 0x0A, (byte) 0xFF, (byte) 0xFF, (byte) 0xFF, (byte) 0xFF, 0x7F};

    @Test
    void decodeGarbageThrows() {
        assertThrows(IllegalArgumentException.class, () -> SdkWireMessages.decodeInvokeRequest(GARBAGE));
        assertThrows(IllegalArgumentException.class, () -> SdkWireMessages.decodeInvokeResponse(GARBAGE));
        assertThrows(IllegalArgumentException.class, () -> SdkWireMessages.decodeStartTaskResponse(GARBAGE));
        assertThrows(IllegalArgumentException.class, () -> SdkWireMessages.decodeTaskStreamRequest(GARBAGE));
        assertThrows(IllegalArgumentException.class, () -> SdkWireMessages.decodeTaskEvent(GARBAGE));
        assertThrows(IllegalArgumentException.class, () -> SdkWireMessages.decodeCancelTaskRequest(GARBAGE));
        assertThrows(IllegalArgumentException.class, () -> SdkWireMessages.decodeProviderConnectRequest(GARBAGE));
        assertThrows(IllegalArgumentException.class, () -> SdkWireMessages.decodeProviderConnectResponse(GARBAGE));
        assertThrows(IllegalArgumentException.class, () -> SdkWireMessages.decodeHeartbeatRequest(GARBAGE));
    }

    @Test
    void encodeInvokeResponseEmptyPayloadRoundTrip() {
        // encode 空消息（encode() 的空 writer 分支）
        byte[] encoded = SdkWireMessages.encodeInvokeResponse(new InvokeResponse(new byte[0]));
        InvokeResponse decoded = SdkWireMessages.decodeInvokeResponse(encoded);
        assertEquals(0, decoded.payload.length);
    }

    @Test
    void mapFieldRoundTrip() {
        // writeMap/readMapEntry：metadata 非空时经 encode→decode round-trip。
        Map<String, String> metadata = new LinkedHashMap<>();
        metadata.put("X-Game-ID", "demo");
        metadata.put("trace", "abc123");
        byte[] encoded = SdkWireMessages.encodeInvokeRequest(
            new InvokeRequest("fn.x", "idem-1", new byte[] {7}, metadata));
        InvokeRequest decoded = SdkWireMessages.decodeInvokeRequest(encoded);
        assertEquals("fn.x", decoded.functionId);
        assertEquals("idem-1", decoded.idempotencyKey);
        assertEquals("demo", decoded.metadata.get("X-Game-ID"));
        assertEquals("abc123", decoded.metadata.get("trace"));
    }

    @Test
    void providerFunctionDescriptorViaConnectRequestRoundTrip() {
        // descriptor 编解码为 private，经 ProviderConnectRequest 间接触达。
        ProviderFunctionDescriptor descriptor = new ProviderFunctionDescriptor(
            "fn.y", "1.0.0", java.util.List.of("gm"), "summary", null, null, false,
            null, null, "player", "read", "", "", false, "", "safe", true, null);
        byte[] encoded = SdkWireMessages.encodeProviderConnectRequest(
            new SdkWireMessages.ProviderConnectRequest("svc", "1", "", java.util.List.of(descriptor)));
        SdkWireMessages.ProviderConnectRequest decoded = SdkWireMessages.decodeProviderConnectRequest(encoded);
        assertEquals(1, decoded.functions.size());
        assertEquals("fn.y", decoded.functions.get(0).id);
        assertEquals("safe", decoded.functions.get(0).risk);
    }

    @Test
    void heartbeatRequestRoundTrip() {
        byte[] encoded = SdkWireMessages.encodeHeartbeatRequest(
            new HeartbeatRequest("svc-1", "session-1"));
        HeartbeatRequest decoded = SdkWireMessages.decodeHeartbeatRequest(encoded);
        assertEquals("session-1", decoded.sessionId);
    }

    @Test
    void startTaskResponseRoundTrip() {
        byte[] encoded = SdkWireMessages.encodeStartTaskResponse(
            new StartTaskResponse("task-9"));
        StartTaskResponse decoded = SdkWireMessages.decodeStartTaskResponse(encoded);
        assertEquals("task-9", decoded.taskId);
    }

    /** 追加 unknown field（field 99，length-delimited）——各 decoder 的 default->skipField 分支。 */
    private static byte[] withUnknownField(byte[] encoded) {
        byte[] extra = {(byte) 0xFA, 0x06, 0x03, 'x', 'y', 'z'}; // tag = (99<<3)|2 = 0x2FA → varint FA 06
        byte[] out = new byte[encoded.length + extra.length];
        System.arraycopy(encoded, 0, out, 0, encoded.length);
        System.arraycopy(extra, 0, out, encoded.length, extra.length);
        return out;
    }

    @Test
    void unknownFieldsAreSkippedInEveryDecoder() {
        byte[] inv = withUnknownField(SdkWireMessages.encodeInvokeRequest(
            new InvokeRequest("fn", "", new byte[] {1}, java.util.Map.of("k", "v"))));
        assertEquals("fn", SdkWireMessages.decodeInvokeRequest(inv).functionId);

        byte[] invResp = withUnknownField(SdkWireMessages.encodeInvokeResponse(new InvokeResponse(new byte[] {2})));
        assertEquals(1, SdkWireMessages.decodeInvokeResponse(invResp).payload.length);

        byte[] startResp = withUnknownField(SdkWireMessages.encodeStartTaskResponse(new StartTaskResponse("t1")));
        assertEquals("t1", SdkWireMessages.decodeStartTaskResponse(startResp).taskId);

        byte[] streamReq = withUnknownField(SdkWireMessages.encodeTaskStreamRequest(new TaskStreamRequest("t1")));
        assertEquals("t1", SdkWireMessages.decodeTaskStreamRequest(streamReq).taskId);

        byte[] evt = withUnknownField(SdkWireMessages.encodeTaskEvent(
            new TaskEvent("done", "ok", 100, new byte[0])));
        assertEquals("done", SdkWireMessages.decodeTaskEvent(evt).type);

        byte[] cancel = withUnknownField(SdkWireMessages.encodeCancelTaskRequest(
            new SdkWireMessages.CancelTaskRequest("t1")));
        assertEquals("t1", SdkWireMessages.decodeCancelTaskRequest(cancel).taskId);

        byte[] connect = withUnknownField(SdkWireMessages.encodeProviderConnectRequest(
            new SdkWireMessages.ProviderConnectRequest("svc", "1", "", java.util.List.of())));
        assertEquals("svc", SdkWireMessages.decodeProviderConnectRequest(connect).serviceId);

        byte[] connectResp = withUnknownField(SdkWireMessages.encodeProviderConnectResponse(
            new SdkWireMessages.ProviderConnectResponse("s1")));
        assertEquals("s1", SdkWireMessages.decodeProviderConnectResponse(connectResp).sessionId);

        byte[] hb = withUnknownField(SdkWireMessages.encodeHeartbeatRequest(
            new HeartbeatRequest("svc", "s1")));
        assertEquals("s1", SdkWireMessages.decodeHeartbeatRequest(hb).sessionId);
    }

    @Test
    void invalidMapEntryBytesThrowFromReadMapEntry() {
        // metadata 字段的 entry 内容本身非法（截断 varint 长度）——
        // readMapEntry 自身的 catch 分支。
        byte[] badEntry = {(byte) 0x0A, (byte) 0xFF, (byte) 0xFF};
        byte[] framed = new byte[2 + badEntry.length];
        framed[0] = 0x22; // InvokeRequest field 4 (metadata), wire type 2
        framed[1] = (byte) badEntry.length;
        System.arraycopy(badEntry, 0, framed, 2, badEntry.length);
        assertThrows(IllegalArgumentException.class, () -> SdkWireMessages.decodeInvokeRequest(framed));
    }

    @Test
    void taskStreamAndEventRoundTrip() {
        byte[] encReq = SdkWireMessages.encodeTaskStreamRequest(new TaskStreamRequest("task-9"));
        TaskStreamRequest req = SdkWireMessages.decodeTaskStreamRequest(encReq);
        assertEquals("task-9", req.taskId);

        byte[] encEvt = SdkWireMessages.encodeTaskEvent(
            new TaskEvent("progress", "halfway", 42, new byte[] {1}));
        TaskEvent evt = SdkWireMessages.decodeTaskEvent(encEvt);
        assertEquals("progress", evt.type);
        assertEquals(42, evt.progress);
    }
}
