package io.github.cuihairu.croupier.sdk;

import io.github.cuihairu.croupier.sdk.testing.FakeTransportClient;
import io.github.cuihairu.croupier.sdk.transport.Protocol;
import io.github.cuihairu.croupier.sdk.transport.TransportClient;
import io.github.cuihairu.croupier.sdk.wire.SdkWireMessages;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.util.function.BiFunction;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * AUD-105: the handshake must actually carry the configured provider token.
 * ClientConfig.authToken is mapped into the reserved {@code authToken}
 * metadata key of ProviderConnectRequest (CR-FIX-11) — omitted tokens must not
 * add the key at all.
 */
@DisplayName("Provider handshake carries configured auth token (AUD-105)")
class ClientAuthTokenHandshakeTest {

    private static SdkWireMessages.ProviderConnectRequest connectWith(ClientConfig config) throws Exception {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) ->
            msgType == Protocol.MSG_PROVIDER_CONNECT_REQUEST
                ? SdkWireMessages.encodeProviderConnectResponse(
                    new SdkWireMessages.ProviderConnectResponse("session-1"))
                : new byte[0]);
        BiFunction<String, Integer, TransportClient> factory = (address, timeout) -> transport;
        CroupierClientImpl client = new CroupierClientImpl(config, factory);
        try {
            client.registerFunction(new FunctionDescriptor("demo.echo", "1.0.0"), (context, payload) -> payload);
            client.connect().toCompletableFuture().get(10, java.util.concurrent.TimeUnit.SECONDS);
        } finally {
            client.close();
        }
        return SdkWireMessages.decodeProviderConnectRequest(
            transport.getCalls().stream()
                .filter(c -> c.msgType() == Protocol.MSG_PROVIDER_CONNECT_REQUEST)
                .findFirst()
                .orElseThrow(() -> new AssertionError("no connect request captured"))
                .data());
    }

    @Test
    void explicitTokenRidesTheHandshakeMetadata() throws Exception {
        ClientConfig config = new ClientConfig("game-1", "svc-1");
        config.setEnv("development");
        config.setAuthToken("Bearer sdk-token");

        SdkWireMessages.ProviderConnectRequest request = connectWith(config);

        assertEquals("Bearer sdk-token", request.metadata.get("authToken"));
    }

    @Test
    void omittedTokenLeavesMetadataWithoutReservedKey() throws Exception {
        ClientConfig config = new ClientConfig("game-1", "svc-1");
        config.setEnv("development");

        SdkWireMessages.ProviderConnectRequest request = connectWith(config);

        assertTrue(request.metadata.isEmpty(), "no token configured -> no authToken key on the wire");
    }
}
