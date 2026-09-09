package io.github.cuihairu.croupier.sdk;

import io.github.cuihairu.croupier.sdk.testing.FakeTransportClient;
import io.github.cuihairu.croupier.sdk.transport.Protocol;
import io.github.cuihairu.croupier.sdk.wire.SdkWireMessages;
import org.junit.jupiter.api.Test;

import java.lang.reflect.Field;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;

import static org.junit.jupiter.api.Assertions.assertDoesNotThrow;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * doServe 循环的两个未覆盖分支：
 * 1) 服务线程被中断 → InterruptedException → break 正常收尾（不视为服务失败）；
 * 2) 循环外异常（handlers 为 null 的 NPE）→ 外层 catch 包装为 Serving failed。
 */
class CroupierClientImplServeInterruptionTest {

    private static final class Harness {
        final AtomicInteger heartbeats = new AtomicInteger();
        final CountDownLatch capabilitiesLatch = new CountDownLatch(1);

        FakeTransportClient newTransport() {
            return new FakeTransportClient((msgType, data) -> {
                if (msgType == Protocol.MSG_PROVIDER_CONNECT_REQUEST) {
                    return SdkWireMessages.encodeProviderConnectResponse(
                        new SdkWireMessages.ProviderConnectResponse("session-serve-int"));
                }
                if (msgType == Protocol.MSG_PROVIDER_HEARTBEAT_REQUEST) {
                    heartbeats.incrementAndGet();
                    return new byte[0];
                }
                if (msgType == Protocol.MSG_REGISTER_CAPABILITIES_REQ) {
                    capabilitiesLatch.countDown();
                    return new byte[0];
                }
                return new byte[0];
            });
        }
    }

    private static CroupierClientImpl connectedClient(Harness harness, ClientConfig config) {
        CroupierClientImpl client = new CroupierClientImpl(config,
            (address, timeout) -> harness.newTransport());
        FunctionDescriptor descriptor = new FunctionDescriptor("fn.serve", "1.0.0");
        assertDoesNotThrow(() -> client.registerFunction(descriptor, (ctx, payload) -> "{\"ok\":true}"));
        assertDoesNotThrow(() -> client.connect().join());
        return client;
    }

    /** 找到正在执行 doServe lambda 的 commonPool 工作线程。 */
    private static Thread findServeThread() {
        for (java.util.Map.Entry<Thread, StackTraceElement[]> entry : Thread.getAllStackTraces().entrySet()) {
            Thread thread = entry.getKey();
            if (thread == Thread.currentThread()) {
                continue;
            }
            for (StackTraceElement frame : entry.getValue()) {
                if (frame.getClassName().equals(CroupierClientImpl.class.getName())
                    && frame.getMethodName().contains("doServe")) {
                    return thread;
                }
            }
        }
        return null;
    }

    @Test
    void interruptedServeLoopCompletesNormally() throws Exception {
        Harness harness = new Harness();
        CroupierClientImpl client = connectedClient(harness, new ClientConfig("game", "svc"));
        try {
            CompletableFuture<Void> serving = client.serveAsync();

            // 轮询等待异步任务真正进入服务循环（serving 置位）。
            boolean servingReported = false;
            for (int i = 0; i < 100 && !servingReported; i++) {
                servingReported = client.isServing();
                if (!servingReported) {
                    Thread.sleep(20);
                }
            }
            assertTrue(servingReported, "client should report serving");

            Thread serveThread = null;
            for (int i = 0; i < 50 && serveThread == null; i++) {
                serveThread = findServeThread();
                if (serveThread == null) {
                    Thread.sleep(20);
                }
            }
            assertNotNull(serveThread, "serve loop thread should be discoverable");
            serveThread.interrupt();

            // 中断只结束循环：future 正常完成，不算服务失败。
            assertDoesNotThrow(() -> serving.get(5, TimeUnit.SECONDS));
            assertFalse(client.isServing(), "serving flag should be cleared after interrupt");

            // 吸收残留中断标记：向 commonPool 投递若干自清理任务，
            // 落在被中断 worker 上的 sleep 会立刻抛出并清掉 interrupt 标志。
            for (int i = 0; i < 2 * Runtime.getRuntime().availableProcessors(); i++) {
                CompletableFuture.runAsync(() -> {
                    try {
                        Thread.sleep(2);
                    } catch (InterruptedException ignored) {
                        // 自清理：抛出即清除当前线程中断标志
                    }
                });
            }
        } finally {
            assertDoesNotThrow(client::close);
        }
    }

    @Test
    void serveLoopOuterExceptionFailsFuture() throws Exception {
        Harness harness = new Harness();
        CroupierClientImpl client = connectedClient(harness, new ClientConfig("game", "svc"));
        try {
            // handlers 为 null 时 handlers.size() 抛 NPE → 外层 catch → Serving failed
            Field handlersField = CroupierClientImpl.class.getDeclaredField("handlers");
            handlersField.setAccessible(true);
            handlersField.set(client, null);

            CompletableFuture<Void> serving = client.serveAsync();
            ExecutionException ex = assertThrows(ExecutionException.class,
                () -> serving.get(5, TimeUnit.SECONDS));
            // wrapAsyncFailure 会多层包装：沿因果链查找 Serving failed 语义。
            boolean foundServingFailure = false;
            for (Throwable t = ex; t != null; t = t.getCause()) {
                if (t.getMessage() != null && t.getMessage().contains("Serving failed")) {
                    foundServingFailure = true;
                    break;
                }
            }
            assertTrue(foundServingFailure, "unexpected cause: " + ex.getCause());

        } finally {
            // close() 会清理 handlers：无论断言结果如何，先恢复为空 map 再关闭。
            try {
                Field handlersField = CroupierClientImpl.class.getDeclaredField("handlers");
                handlersField.setAccessible(true);
                handlersField.set(client, new java.util.concurrent.ConcurrentHashMap<>());
            } catch (ReflectiveOperationException ignored) {
                // 恢复失败时 close 可能抛 NPE，测试主体结果不受影响
            }
            assertDoesNotThrow(client::close);
        }
    }
}
