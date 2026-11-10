package io.github.cuihairu.croupier.sdk.wire;

import org.junit.jupiter.api.Test;

import java.util.LinkedHashMap;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * CR-FIX-11：ProviderConnectRequest 的 metadata（field 13）wire 编解码。
 * authToken 是平台保留键——agent 校验后丢弃，绝不入会话元数据；
 * 本测试钉住含 metadata 的握手帧 round-trip 与旧帧（无 field 13）兼容。
 */
class ProviderConnectMetadataTest {

    @Test
    void encodeDecodeRoundTripsMetadata() {
        Map<String, String> metadata = new LinkedHashMap<>();
        metadata.put("authToken", "secret");
        metadata.put("serverId", "s1");
        SdkWireMessages.ProviderConnectRequest request = new SdkWireMessages.ProviderConnectRequest(
            "svc", "1.0.0", "", java.util.List.of(), "java", "0.0.1", "croupier-java-sdk", "1.0.0", metadata);

        byte[] encoded = SdkWireMessages.encodeProviderConnectRequest(request);
        SdkWireMessages.ProviderConnectRequest decoded = SdkWireMessages.decodeProviderConnectRequest(encoded);

        assertEquals("svc", decoded.serviceId);
        assertEquals("secret", decoded.metadata.get("authToken"));
        assertEquals("s1", decoded.metadata.get("serverId"));
    }

    @Test
    void decodeLegacyFrameWithoutMetadataYieldsEmptyMap() {
        SdkWireMessages.ProviderConnectRequest legacy = new SdkWireMessages.ProviderConnectRequest(
            "svc", "1.0.0", "", java.util.List.of());
        byte[] encoded = SdkWireMessages.encodeProviderConnectRequest(legacy);

        SdkWireMessages.ProviderConnectRequest decoded = SdkWireMessages.decodeProviderConnectRequest(encoded);

        assertTrue(decoded.metadata.isEmpty(), "frames without field 13 must decode to empty metadata");
    }
}
