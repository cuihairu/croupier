package io.github.cuihairu.croupier.sdk;

import io.github.cuihairu.croupier.sdk.transport.Protocol;
import io.github.cuihairu.croupier.sdk.transport.TransportClient;
import io.github.cuihairu.croupier.sdk.wire.SdkWireMessages;
import org.junit.jupiter.api.Test;

import java.lang.reflect.Method;
import java.nio.charset.StandardCharsets;
import java.util.concurrent.atomic.AtomicReference;
import java.util.zip.GZIPInputStream;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * F：控制面 manifest 上传——发送接线验证（fake control transport）。
 */
public class ManifestUploadTest {

    @Test
    public void uploadsGzippedManifestToControlPlane() throws Exception {
        ClientConfig config = new ClientConfig();
        config.setControlAddr("127.0.0.1:19999");
        config.setServiceId("java-1");
        config.setServiceVersion("2.0.0");
        config.setProviderLang("java");
        config.setProviderSdk("croupier-java-sdk");

        AtomicReference<Integer> sentMsgType = new AtomicReference<>();
        AtomicReference<byte[]> sentBody = new AtomicReference<>();
        TransportClient fakeControl = new TransportClient() {
            @Override
            public void connect() {
            }

            @Override
            public byte[] request(int msgType, byte[] data) {
                sentMsgType.set(msgType);
                sentBody.set(data);
                return new byte[0];
            }

            @Override
            public boolean isConnected() {
                return false;
            }

            @Override
            public void close() {
            }
        };
        TransportClient fakeAgent = new TransportClient() {
            @Override
            public void connect() {
            }

            @Override
            public byte[] request(int msgType, byte[] data) {
                return new byte[0];
            }

            @Override
            public boolean isConnected() {
                return false;
            }

            @Override
            public void close() {
            }
        };

        CroupierClientImpl client = new CroupierClientImpl(config, (addr, timeout) -> {
            if ("127.0.0.1:19999".equals(addr)) {
                return fakeControl;
            }
            return fakeAgent;
        });
        FunctionDescriptor descriptor = new FunctionDescriptor("player.ban", "1.0.0");
        descriptor.setInputSchema(
                "{\"type\":\"object\",\"properties\":{\"id\":{\"type\":\"string\"}}}");
        client.registerFunction(descriptor, (metadata, payload) -> "ok");

        // 直接触发上传（connect 全流程需要 agent 握手，此处单测上传路径）
        Method maybe = CroupierClientImpl.class.getDeclaredMethod("maybeRegisterCapabilities");
        maybe.setAccessible(true);
        maybe.invoke(client);

        assertEquals(Protocol.MSG_REGISTER_CAPABILITIES_REQ, sentMsgType.get().intValue());
        SdkWireMessages.RegisterCapabilitiesRequest decoded =
                SdkWireMessages.decodeRegisterCapabilitiesRequest(sentBody.get());
        assertEquals("java-1", decoded.provider.id);
        assertEquals("java", decoded.provider.lang);
        assertNotNull(decoded.manifestJsonGz);
        try (GZIPInputStream gzip = new GZIPInputStream(
                new java.io.ByteArrayInputStream(decoded.manifestJsonGz))) {
            String manifest = new String(gzip.readAllBytes(), StandardCharsets.UTF_8);
            assertTrue(manifest.contains("\"provider\""));
            assertTrue(manifest.contains("player.ban"));
        }
    }

    @Test
    public void noControlAddrIsNoop() throws Exception {
        CroupierClientImpl client = new CroupierClientImpl(new ClientConfig());
        Method maybe = CroupierClientImpl.class.getDeclaredMethod("maybeRegisterCapabilities");
        maybe.setAccessible(true);
        // 不应抛错
        maybe.invoke(client);
    }
}
