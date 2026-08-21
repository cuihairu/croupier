package io.github.cuihairu.croupier.sdk;

import io.github.cuihairu.croupier.sdk.invoker.InvokerException;
import io.github.cuihairu.croupier.sdk.testing.FakeTransportClient;
import io.github.cuihairu.croupier.sdk.transport.Protocol;
import io.github.cuihairu.croupier.sdk.transport.TransportClient;
import io.github.cuihairu.croupier.sdk.wire.SdkWireMessages;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.Timeout;

import java.lang.reflect.Field;
import java.lang.reflect.Method;
import java.util.List;
import java.util.Map;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.CopyOnWriteArrayList;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.function.BiFunction;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Tests for heartbeat failure handling and connection recovery in CroupierClientImpl.
 */
@DisplayName("CroupierClientImpl heartbeat and reconnection")
class CroupierClientImplReconnectTest {

    /** FakeTransportClient is final; wrap it so close() can be observed. */
    private static final class CloseTrackingTransport implements TransportClient {
        final FakeTransportClient delegate;
        final AtomicInteger closes = new AtomicInteger();

        CloseTrackingTransport(FakeTransportClient.Handler handler) {
            this.delegate = new FakeTransportClient(handler);
        }

        @Override
        public void connect() {
            delegate.connect();
        }

        @Override
        public byte[] request(int msgType, byte[] data) throws InvokerException {
            return delegate.request(msgType, data);
        }

        @Override
        public boolean isConnected() {
            return delegate.isConnected();
        }

        @Override
        public void close() {
            closes.incrementAndGet();
            delegate.close();
        }
    }

    private ClientConfig createConfig() {
        ClientConfig config = new ClientConfig("game-1", "svc-1");
        config.setEnv("development");
        return config;
    }

    private static FakeTransportClient.Handler healthyHandler() {
        return (msgType, data) -> {
            if (msgType == Protocol.MSG_PROVIDER_CONNECT_REQUEST) {
                return SdkWireMessages.encodeProviderConnectResponse(
                    new SdkWireMessages.ProviderConnectResponse("session-ok"));
            }
            return new byte[0];
        };
    }

    private static Object invokePrivate(Object target, String name) throws Exception {
        Method method = target.getClass().getDeclaredMethod(name);
        method.setAccessible(true);
        try {
            return method.invoke(target);
        } catch (java.lang.reflect.InvocationTargetException e) {
            Throwable cause = e.getCause();
            if (cause instanceof Exception exception) {
                throw exception;
            }
            throw e;
        }
    }

    private static AtomicBoolean booleanField(Object target, String name) throws Exception {
        Field field = target.getClass().getDeclaredField(name);
        field.setAccessible(true);
        return (AtomicBoolean) field.get(target);
    }

    private static Object objectField(Object target, String name) throws Exception {
        Field field = target.getClass().getDeclaredField(name);
        field.setAccessible(true);
        return field.get(target);
    }

    private static void setField(Object target, String name, Object value) throws Exception {
        Field field = target.getClass().getDeclaredField(name);
        field.setAccessible(true);
        field.set(target, value);
    }

    private static void awaitTrue(java.util.function.BooleanSupplier condition, long timeoutMs, String message) {
        long deadline = System.currentTimeMillis() + timeoutMs;
        while (System.currentTimeMillis() < deadline) {
            if (condition.getAsBoolean()) {
                return;
            }
            try {
                Thread.sleep(20);
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
                return;
            }
        }
        fail(message);
    }

    @Test
    @DisplayName("connect closes the previous transport when reconnecting")
    @Timeout(10)
    void connectReplacesStaleTransport() throws Exception {
        List<CloseTrackingTransport> created = new CopyOnWriteArrayList<>();
        BiFunction<String, Integer, TransportClient> factory = (address, timeout) -> {
            CloseTrackingTransport transport = new CloseTrackingTransport(healthyHandler());
            created.add(transport);
            return transport;
        };
        CroupierClientImpl client = new CroupierClientImpl(createConfig(), factory);
        client.registerFunction(new FunctionDescriptor("f", "1.0.0"), (ctx, payload) -> "ok");

        client.connect().get(5, TimeUnit.SECONDS);
        assertEquals("session-ok", client.getSessionId());
        CloseTrackingTransport first = created.get(0);

        // Simulate a dropped connection without clearing the transport field.
        booleanField(client, "connected").set(false);

        client.connect().get(5, TimeUnit.SECONDS);
        assertEquals(2, created.size());
        assertEquals(1, first.closes.get());
        assertEquals("session-ok", client.getSessionId());
        client.stop();
    }

    @Test
    @DisplayName("serveAsync skips connect when already connected and stops cleanly")
    @Timeout(10)
    void serveAsyncWhenConnected() throws Exception {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(), (address, timeout) -> new FakeTransportClient(healthyHandler()));
        client.registerFunction(new FunctionDescriptor("f", "1.0.0"), (ctx, payload) -> "ok");

        client.connect().get(5, TimeUnit.SECONDS);
        CompletableFuture<Void> serving = client.serveAsync();
        awaitTrue(client::isServing, 2000, "client should be serving");

        client.stop();
        serving.get(5, TimeUnit.SECONDS);
        assertFalse(client.isServing());
    }

    @Test
    @DisplayName("serve blocks until stop")
    @Timeout(10)
    void serveBlocksUntilStop() throws Exception {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(), (address, timeout) -> new FakeTransportClient(healthyHandler()));
        client.registerFunction(new FunctionDescriptor("f", "1.0.0"), (ctx, payload) -> "ok");
        client.connect().get(5, TimeUnit.SECONDS);

        Thread stopper = new Thread(() -> {
            try {
                Thread.sleep(150);
            } catch (InterruptedException ignored) {
            }
            client.stop();
        });
        stopper.start();
        client.serve();
        stopper.join(2000);
        assertFalse(client.isServing());
    }

    @Test
    @DisplayName("heartbeat failures while serving trigger recovery; disabled reconnect stays disconnected")
    @Timeout(20)
    void heartbeatFailureTriggersRecoveryDisabled() throws Exception {
        ClientConfig config = createConfig();
        config.setHeartbeatInterval(1);
        config.setReconnect(ReconnectConfig.builder().enabled(false).build());

        AtomicInteger heartbeats = new AtomicInteger();
        CroupierClientImpl client = new CroupierClientImpl(config, (address, timeout) -> new CloseTrackingTransport((msgType, data) -> {
            if (msgType == Protocol.MSG_PROVIDER_CONNECT_REQUEST) {
                return SdkWireMessages.encodeProviderConnectResponse(
                    new SdkWireMessages.ProviderConnectResponse("session-hb"));
            }
            if (msgType == Protocol.MSG_PROVIDER_HEARTBEAT_REQUEST) {
                heartbeats.incrementAndGet();
                throw new RuntimeException("heartbeat refused");
            }
            return new byte[0];
        }));
        client.registerFunction(new FunctionDescriptor("f", "1.0.0"), (ctx, payload) -> "ok");

        client.connect().get(5, TimeUnit.SECONDS);
        assertEquals("session-hb", client.getSessionId());
        CompletableFuture<Void> serving = client.serveAsync();

        awaitTrue(() -> !client.isConnected(), 12000, "client should drop connection after repeated heartbeat failures");
        assertEquals("", client.getSessionId());
        assertTrue(heartbeats.get() >= 2, "expected at least two failed heartbeats, got " + heartbeats.get());

        client.stop();
        serving.get(5, TimeUnit.SECONDS);
    }

    @Test
    @DisplayName("recoverConnection retries with backoff and restores the session")
    @Timeout(15)
    void recoverConnectionSucceedsAfterFailures() throws Exception {
        ClientConfig config = createConfig();
        config.setReconnect(ReconnectConfig.builder()
            .enabled(true)
            .maxAttempts(10)
            .initialDelayMs(10)
            .maxDelayMs(20)
            .backoffMultiplier(1.0)
            .jitterFactor(0.0)
            .build());

        AtomicInteger connectAttempts = new AtomicInteger();
        CroupierClientImpl client = new CroupierClientImpl(config, (address, timeout) -> new CloseTrackingTransport((msgType, data) -> {
            if (msgType == Protocol.MSG_PROVIDER_CONNECT_REQUEST) {
                int attempt = connectAttempts.incrementAndGet();
                if (attempt < 3) {
                    throw new RuntimeException("agent down, attempt " + attempt);
                }
                return SdkWireMessages.encodeProviderConnectResponse(
                    new SdkWireMessages.ProviderConnectResponse("session-recovered"));
            }
            return new byte[0];
        }));
        client.registerFunction(new FunctionDescriptor("f", "1.0.0"), (ctx, payload) -> "ok");

        booleanField(client, "serving").set(true);
        invokePrivate(client, "recoverConnection");

        awaitTrue(client::isConnected, 10000, "client should reconnect after transient failures");
        assertEquals("session-recovered", client.getSessionId());
        assertTrue(connectAttempts.get() >= 3, "expected at least 3 connect attempts, got " + connectAttempts.get());
        client.stop();
    }

    @Test
    @DisplayName("recoverConnection gives up after max attempts")
    @Timeout(15)
    void recoverConnectionMaxAttemptsExceeded() throws Exception {
        ClientConfig config = createConfig();
        config.setReconnect(ReconnectConfig.builder()
            .enabled(true)
            .maxAttempts(2)
            .initialDelayMs(5)
            .maxDelayMs(10)
            .backoffMultiplier(1.0)
            .jitterFactor(0.0)
            .build());

        AtomicInteger connectAttempts = new AtomicInteger();
        CroupierClientImpl client = new CroupierClientImpl(config, (address, timeout) -> new CloseTrackingTransport((msgType, data) -> {
            if (msgType == Protocol.MSG_PROVIDER_CONNECT_REQUEST) {
                connectAttempts.incrementAndGet();
                throw new RuntimeException("agent permanently down");
            }
            return new byte[0];
        }));
        client.registerFunction(new FunctionDescriptor("f", "1.0.0"), (ctx, payload) -> "ok");

        booleanField(client, "serving").set(true);
        invokePrivate(client, "recoverConnection");

        awaitTrue(() -> connectAttempts.get() >= 2, 10000, "recovery should exhaust max attempts");
        AtomicBoolean reconnecting = booleanField(client, "reconnecting");
        awaitTrue(() -> !reconnecting.get(), 10000, "recovery thread should finish");
        assertFalse(client.isConnected());
        client.stop();
    }

    @Test
    @DisplayName("reconnectOnce without registered functions fails fast")
    @Timeout(5)
    void reconnectOnceWithoutFunctions() throws Exception {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(), (address, timeout) -> new CloseTrackingTransport(healthyHandler()));
        CroupierException error = assertThrows(CroupierException.class, () -> invokePrivate(client, "reconnectOnce"));
        assertEquals("No functions registered", error.getMessage());
    }

    @Test
    @DisplayName("reconnectOnce rejects empty session ids and closes the candidate transport")
    @Timeout(5)
    void reconnectOnceEmptySession() throws Exception {
        List<CloseTrackingTransport> created = new CopyOnWriteArrayList<>();
        CroupierClientImpl client = new CroupierClientImpl(createConfig(), (address, timeout) -> {
            CloseTrackingTransport transport = new CloseTrackingTransport((msgType, data) ->
                SdkWireMessages.encodeProviderConnectResponse(new SdkWireMessages.ProviderConnectResponse("")));
            created.add(transport);
            return transport;
        });
        client.registerFunction(new FunctionDescriptor("f", "1.0.0"), (ctx, payload) -> "ok");

        CroupierException error = assertThrows(CroupierException.class, () -> invokePrivate(client, "reconnectOnce"));
        assertTrue(error.getMessage().contains("empty session_id"));
        assertEquals(1, created.size());
        // Recorded bug: the empty-session branch closes the transport and then the
        // enclosing catch closes the same transport a second time.
        assertEquals(2, created.get(0).closes.get());
    }

    @Test
    @DisplayName("reconnectOnce swaps and closes the previous transport")
    @Timeout(5)
    void reconnectOnceClosesOldTransport() throws Exception {
        CloseTrackingTransport oldTransport = new CloseTrackingTransport((msgType, data) -> new byte[0]);
        oldTransport.connect();
        List<CloseTrackingTransport> created = new CopyOnWriteArrayList<>();
        CroupierClientImpl client = new CroupierClientImpl(createConfig(), (address, timeout) -> {
            CloseTrackingTransport transport = new CloseTrackingTransport(healthyHandler());
            created.add(transport);
            return transport;
        });
        client.registerFunction(new FunctionDescriptor("f", "1.0.0"), (ctx, payload) -> "ok");
        setField(client, "transport", oldTransport);

        invokePrivate(client, "reconnectOnce");

        assertEquals(1, created.size());
        assertEquals(1, oldTransport.closes.get());
        assertEquals(0, created.get(0).closes.get());
        assertSame(created.get(0), objectField(client, "transport"));
        assertEquals("session-ok", client.getSessionId());
    }

    @Test
    @DisplayName("stop re-interrupts the caller when joining the heartbeat thread is interrupted")
    @Timeout(10)
    void stopWithInterruptedCaller() throws Exception {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(), (address, timeout) -> new FakeTransportClient(healthyHandler()));
        client.registerFunction(new FunctionDescriptor("f", "1.0.0"), (ctx, payload) -> "ok");
        client.connect().get(5, TimeUnit.SECONDS);

        Thread.currentThread().interrupt();
        try {
            client.stop();
            // The join inside stopHeartbeatLoop throws InterruptedException and re-interrupts.
            assertTrue(Thread.interrupted(), "interrupt flag should be preserved by stop()");
        } finally {
            // Drain any pending interrupt to avoid affecting other tests.
            Thread.interrupted();
        }
        assertFalse(client.isConnected());
    }
}
