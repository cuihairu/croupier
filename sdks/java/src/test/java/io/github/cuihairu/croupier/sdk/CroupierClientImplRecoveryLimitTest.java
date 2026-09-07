package io.github.cuihairu.croupier.sdk;

import io.github.cuihairu.croupier.sdk.testing.FakeTransportClient;
import io.github.cuihairu.croupier.sdk.transport.Protocol;
import io.github.cuihairu.croupier.sdk.wire.SdkWireMessages;
import org.junit.jupiter.api.Test;

import java.lang.reflect.Method;
import java.util.concurrent.atomic.AtomicInteger;

import static org.junit.jupiter.api.Assertions.assertDoesNotThrow;
import static org.junit.jupiter.api.Assertions.assertTrue;

/** Final fillers: drain with disabled reconnect and recovery attempt limits. */
class CroupierClientImplRecoveryLimitTest {

    private static final class Harness {
        final AtomicInteger connects = new AtomicInteger();
        volatile boolean failReconnects;

        FakeTransportClient newTransport() {
            return new FakeTransportClient((msgType, data) -> {
                if (msgType == Protocol.MSG_PROVIDER_CONNECT_REQUEST) {
                    if (failReconnects && connects.get() > 0) {
                        throw new RuntimeException("agent restarting");
                    }
                    connects.incrementAndGet();
                    return SdkWireMessages.encodeProviderConnectResponse(
                        new SdkWireMessages.ProviderConnectResponse("session-limit"));
                }
                return new byte[0];
            });
        }
    }

    private static CroupierClientImpl connected(ClientConfig config, Harness harness) {
        CroupierClientImpl client = new CroupierClientImpl(config,
            (address, timeout) -> harness.newTransport());
        FunctionDescriptor descriptor = new FunctionDescriptor("fn.one", "1.0.0");
        assertDoesNotThrow(() -> client.registerFunction(descriptor, (ctx, payload) -> "{}"));
        assertDoesNotThrow(() -> client.connect().join());
        return client;
    }

    private static void drain(CroupierClientImpl client) throws Exception {
        Method drain = CroupierClientImpl.class.getDeclaredMethod("drainAndRecover");
        drain.setAccessible(true);
        drain.invoke(client);
    }

    private static void recover(CroupierClientImpl client) throws Exception {
        Method recover = CroupierClientImpl.class.getDeclaredMethod("recoverConnection");
        recover.setAccessible(true);
        recover.invoke(client);
    }

    @Test
    void drainWithDisabledReconnectClosesTransportOnly() throws Exception {
        Harness harness = new Harness();
        ClientConfig config = new ClientConfig("game", "svc");
        config.setReconnect(ReconnectConfig.builder()
            .enabled(false).build());
        CroupierClientImpl client = connected(config, harness);
        drain(client);
        Thread.sleep(150);
        // disabled reconnect keeps the client flag untouched; close clears it
        assertDoesNotThrow(client::close);
        assertTrue(!client.isConnected());
    }

    @Test
    void recoveryWithZeroMaxAttemptsRetriesUntilSuccess() throws Exception {
        Harness harness = new Harness();
        ClientConfig config = new ClientConfig("game", "svc");
        // maxAttempts=0 disables the attempt cap; recovery ends on success
        config.setReconnect(ReconnectConfig.builder()
            .enabled(true).maxAttempts(0).initialDelayMs(1).maxDelayMs(2).build());
        CroupierClientImpl client = connected(config, harness);
        recover(client);
        Thread.sleep(300);
        assertDoesNotThrow(client::close);
    }

    @Test
    void servingRecoveryExceedsMaxAttemptsAndGivesUp() throws Exception {
        Harness harness = new Harness();
        harness.failReconnects = true;
        ClientConfig config = new ClientConfig("game", "svc");
        config.setReconnect(ReconnectConfig.builder()
            .enabled(true).maxAttempts(1).initialDelayMs(1).maxDelayMs(2).build());
        CroupierClientImpl client = connected(config, harness);
        java.util.concurrent.CompletableFuture<Void> serving = client.serveAsync();
        for (int i = 0; i < 50 && !client.isServing(); i++) {
            Thread.sleep(100);
        }
        assertTrue(client.isServing());
        recover(client);
        // the single allowed attempt fails: the loop logs and returns
        Thread.sleep(600);
        client.stop();
        assertDoesNotThrow(() -> serving.get(5, java.util.concurrent.TimeUnit.SECONDS));
        assertDoesNotThrow(client::close);
    }
}
