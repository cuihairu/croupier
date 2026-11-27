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
import java.util.List;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.CompletionException;
import java.util.concurrent.CopyOnWriteArrayList;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.function.BiFunction;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertInstanceOf;
import static org.junit.jupiter.api.Assertions.assertNotSame;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * AUD-071：stop/close 与建连握手的并发竞态。
 *
 * <p>修复前的根因：connect 在异步任务里直接 {@code transport = nextTransport}
 * 发布连接，stop/close 只清理"已发布"的连接——回包先到、发布未发生时停机，
 * 迟到的建连结果会把已停机客户端复活（transport 重挂、connected 置真、
 * 心跳/manifest 后台动作照常启动）；并发 connect 也会各自完成一次完整握手。
 *
 * <p>修复后：connect 走单飞（并发共用一次握手），transport/connected/心跳
 * 只能在 publishConnection 的生命周期锁内发布，stop/close 持同一把锁递增
 * 停机代际并清理——握手在途期间停机时，迟到结果被拒绝并释放候选 transport。
 */
@DisplayName("CroupierClientImpl connect/stop lifecycle race (AUD-071)")
class CroupierClientImplConnectLifecycleTest {

    private static final String AGENT_ADDR = "tcp://127.0.0.1:19990";

    /**
     * 握手在闸门处暂停的 transport：测试可以在"回包已就绪、transport 尚未发布"
     * 的窗口内插入 stop/close，然后放行回包，确定性地构造"迟到建连结果"。
     */
    private static final class GatedConnectTransport implements TransportClient {
        final CountDownLatch handshakeEntered = new CountDownLatch(1);
        final CountDownLatch releaseHandshake = new CountDownLatch(1);
        final List<Integer> requestTypes = new CopyOnWriteArrayList<>();
        final AtomicInteger closes = new AtomicInteger();
        private volatile boolean connected;

        @Override
        public void connect() {
            connected = true;
        }

        @Override
        public byte[] request(int msgType, byte[] data) throws InvokerException {
            requestTypes.add(msgType);
            if (msgType == Protocol.MSG_PROVIDER_CONNECT_REQUEST) {
                handshakeEntered.countDown();
                try {
                    if (!releaseHandshake.await(20, TimeUnit.SECONDS)) {
                        throw new IllegalStateException("handshake gate timeout");
                    }
                } catch (InterruptedException e) {
                    Thread.currentThread().interrupt();
                    throw new IllegalStateException("interrupted while handshake gated", e);
                }
                return SdkWireMessages.encodeProviderConnectResponse(
                    new SdkWireMessages.ProviderConnectResponse("session-late"));
            }
            return new byte[0];
        }

        @Override
        public boolean isConnected() {
            return connected;
        }

        @Override
        public void close() {
            closes.incrementAndGet();
            connected = false;
        }
    }

    /** 记录每次拨号的目标地址与 transport；首个拨号可替换为闸门 transport。 */
    private static BiFunction<String, Integer, TransportClient> recordingFactory(
            List<String> dialAddresses,
            List<TransportClient> dialTransports,
            GatedConnectTransport firstDial) {
        AtomicInteger dials = new AtomicInteger();
        return (address, timeout) -> {
            dialAddresses.add(address);
            if (dials.incrementAndGet() == 1 && firstDial != null) {
                dialTransports.add(firstDial);
                return firstDial;
            }
            TransportClient plain = new FakeTransportClient((msgType, data) ->
                msgType == Protocol.MSG_PROVIDER_CONNECT_REQUEST
                    ? SdkWireMessages.encodeProviderConnectResponse(
                        new SdkWireMessages.ProviderConnectResponse("session-retry"))
                    : new byte[0]);
            dialTransports.add(plain);
            return plain;
        };
    }

    private ClientConfig baseConfig() {
        ClientConfig config = new ClientConfig("game-1", "svc-1");
        config.setEnv("development");
        config.setServiceVersion("1.0.0");
        config.setAgentAddr(AGENT_ADDR);
        return config;
    }

    private static Object privateField(Object target, String name) throws Exception {
        Field field = target.getClass().getDeclaredField(name);
        field.setAccessible(true);
        return field.get(target);
    }

    private static void awaitTransportClosed(GatedConnectTransport transport, long timeoutMs)
            throws InterruptedException {
        long deadline = System.currentTimeMillis() + timeoutMs;
        while (transport.closes.get() == 0 && System.currentTimeMillis() < deadline) {
            Thread.sleep(10);
        }
    }

    @Test
    @Timeout(30)
    @DisplayName("握手在途时 stop：在途操作被中止，迟到 transport 被释放，心跳/manifest 不启动")
    void stopDuringHandshakeRejectsLateTransportAndKeepsClientDown() throws Exception {
        ClientConfig config = baseConfig();
        config.setControlAddr("tcp://127.0.0.1:19991"); // manifest 目标：后台动作若启动，factory 会收到该地址
        config.setHeartbeatInterval(1);
        List<String> dialAddresses = new CopyOnWriteArrayList<>();
        List<TransportClient> dialTransports = new CopyOnWriteArrayList<>();
        GatedConnectTransport gated = new GatedConnectTransport();
        CroupierClientImpl client = new CroupierClientImpl(config,
            recordingFactory(dialAddresses, dialTransports, gated));
        client.registerFunction(new FunctionDescriptor("late.fn", "1.0.0"), (ctx, payload) -> "ok");

        CompletableFuture<Void> connecting = client.connect();
        assertTrue(gated.handshakeEntered.await(20, TimeUnit.SECONDS),
            "handshake should be gated in-flight");

        // 回包在闸门后已就绪，但 transport 尚未发布——此刻停机
        client.stop();
        assertFalse(client.isConnected(), "connected must stay false after stop");
        assertEquals("", client.getSessionId());
        assertEquals(0, gated.closes.get(),
            "stop only touches published connections: the unpublished candidate must survive stop");

        // 在途建连被中止：不等回包，connect future 即失败
        CompletionException aborted = assertThrows(CompletionException.class, () -> connecting.join());
        assertInstanceOf(CroupierException.class, aborted.getCause());
        assertTrue(aborted.getCause().getMessage().contains("stopped"),
            "unexpected cause: " + aborted.getCause());

        // 放行迟到回包：发布点的代际检查必须拒绝并释放候选 transport
        gated.releaseHandshake.countDown();
        awaitTransportClosed(gated, 15000);
        assertTrue(gated.closes.get() >= 1, "late transport must be released after the rejected handshake");
        assertFalse(client.isConnected(), "late handshake result must not resurrect connected");
        assertEquals("", client.getSessionId(), "late handshake result must not restore session");

        // 心跳与 manifest 后台动作不得启动
        Thread.sleep(300); // 给假想的 manifest 后台线程一个暴露窗口
        assertNull(privateField(client, "heartbeatThread"),
            "heartbeat thread must not start from a rejected handshake");
        assertFalse(gated.requestTypes.contains(Protocol.MSG_PROVIDER_HEARTBEAT_REQUEST),
            "no heartbeat may be sent after a rejected handshake");
        assertEquals(List.of(AGENT_ADDR), dialAddresses,
            "only the agent dial may happen; control-plane manifest must not start");

        client.close();
    }

    @Test
    @Timeout(30)
    @DisplayName("握手在途时 close：同样拒绝迟到结果并释放 transport")
    void closeDuringHandshakeRejectsLateTransport() throws Exception {
        List<String> dialAddresses = new CopyOnWriteArrayList<>();
        List<TransportClient> dialTransports = new CopyOnWriteArrayList<>();
        GatedConnectTransport gated = new GatedConnectTransport();
        CroupierClientImpl client = new CroupierClientImpl(baseConfig(),
            recordingFactory(dialAddresses, dialTransports, gated));
        client.registerFunction(new FunctionDescriptor("late.fn", "1.0.0"), (ctx, payload) -> "ok");

        CompletableFuture<Void> connecting = client.connect();
        assertTrue(gated.handshakeEntered.await(20, TimeUnit.SECONDS));

        client.close();
        assertFalse(client.isConnected());
        assertEquals(0, gated.closes.get(), "close must not touch the unpublished candidate");

        CompletionException aborted = assertThrows(CompletionException.class, () -> connecting.join());
        assertInstanceOf(CroupierException.class, aborted.getCause());

        gated.releaseHandshake.countDown();
        awaitTransportClosed(gated, 15000);
        assertTrue(gated.closes.get() >= 1, "late transport must be released after close");
        assertFalse(client.isConnected(), "late handshake result must not resurrect connected");
        assertEquals("", client.getSessionId());
    }

    @Test
    @Timeout(30)
    @DisplayName("并发 connect 共用一次有效握手：仅一次拨号、同一个 future")
    void concurrentConnectSharesSingleHandshake() throws Exception {
        List<String> dialAddresses = new CopyOnWriteArrayList<>();
        List<TransportClient> dialTransports = new CopyOnWriteArrayList<>();
        GatedConnectTransport gated = new GatedConnectTransport();
        CroupierClientImpl client = new CroupierClientImpl(baseConfig(),
            recordingFactory(dialAddresses, dialTransports, gated));
        client.registerFunction(new FunctionDescriptor("race.fn", "1.0.0"), (ctx, payload) -> "ok");

        int callers = 8;
        ExecutorService pool = Executors.newFixedThreadPool(callers);
        try {
            CountDownLatch allCalled = new CountDownLatch(callers);
            List<CompletableFuture<Void>> connectFutures = new CopyOnWriteArrayList<>();
            for (int i = 0; i < callers; i++) {
                pool.submit(() -> {
                    connectFutures.add(client.connect());
                    allCalled.countDown();
                    return null;
                });
            }
            assertTrue(allCalled.await(20, TimeUnit.SECONDS), "all callers should have called connect()");
            assertEquals(1, connectFutures.stream().distinct().count(),
                "all concurrent callers must join the same in-flight handshake");
            assertEquals(1, dialTransports.size(),
                "exactly one dial while the shared handshake is in flight");
            assertTrue(gated.handshakeEntered.await(20, TimeUnit.SECONDS));

            gated.releaseHandshake.countDown();
            for (CompletableFuture<Void> future : connectFutures) {
                future.get(20, TimeUnit.SECONDS);
            }
            assertTrue(client.isConnected());
            assertEquals(1, dialTransports.size(), "still exactly one handshake after completion");
            assertEquals(List.of(AGENT_ADDR), dialAddresses, "no extra dials for concurrent connect");
        } finally {
            pool.shutdownNow();
            assertTrue(pool.awaitTermination(10, TimeUnit.SECONDS));
            client.stop();
        }
    }

    @Test
    @Timeout(30)
    @DisplayName("被拒绝的迟到握手不阻塞后续 connect：单飞坑位已让出")
    void rejectedHandshakeDoesNotWedgeSubsequentConnect() throws Exception {
        List<String> dialAddresses = new CopyOnWriteArrayList<>();
        List<TransportClient> dialTransports = new CopyOnWriteArrayList<>();
        GatedConnectTransport gated = new GatedConnectTransport();
        CroupierClientImpl client = new CroupierClientImpl(baseConfig(),
            recordingFactory(dialAddresses, dialTransports, gated));
        client.registerFunction(new FunctionDescriptor("retry.fn", "1.0.0"), (ctx, payload) -> "ok");

        CompletableFuture<Void> connecting = client.connect();
        assertTrue(gated.handshakeEntered.await(20, TimeUnit.SECONDS));
        client.stop();
        assertThrows(CompletionException.class, () -> connecting.join());

        gated.releaseHandshake.countDown();
        awaitTransportClosed(gated, 15000);
        assertTrue(gated.closes.get() >= 1);

        // 停机拒绝之后仍可重新建连：新握手捕获新代际并成功发布
        CompletableFuture<Void> retry = client.connect();
        retry.get(20, TimeUnit.SECONDS);
        assertTrue(client.isConnected(), "connect after a rejected handshake must work");
        assertEquals(2, dialTransports.size(), "retry dials a fresh transport");
        assertNotSame(gated, dialTransports.get(1));

        client.stop();
    }

    @Test
    @Timeout(15)
    @DisplayName("未连接过就 stop，之后仍可正常 connect")
    void connectAfterStopStillConnects() throws Exception {
        List<String> dialAddresses = new CopyOnWriteArrayList<>();
        List<TransportClient> dialTransports = new CopyOnWriteArrayList<>();
        CroupierClientImpl client = new CroupierClientImpl(baseConfig(),
            recordingFactory(dialAddresses, dialTransports, null));
        client.registerFunction(new FunctionDescriptor("fresh.fn", "1.0.0"), (ctx, payload) -> "ok");

        client.stop();
        assertFalse(client.isConnected());

        client.connect().get(15, TimeUnit.SECONDS);
        assertTrue(client.isConnected(), "stop without a prior connect must not block later connects");
        assertEquals("session-retry", client.getSessionId());
        assertEquals(1, dialTransports.size());

        client.stop();
        assertFalse(client.isConnected());
    }
}
