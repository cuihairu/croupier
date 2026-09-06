package io.github.cuihairu.croupier.sdk.invoker;

import io.github.cuihairu.croupier.sdk.testing.FakeTransportClient;
import io.github.cuihairu.croupier.sdk.transport.Protocol;
import io.github.cuihairu.croupier.sdk.wire.SdkWireMessages;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.reactivestreams.Subscriber;
import org.reactivestreams.Subscription;

import java.nio.charset.StandardCharsets;
import java.util.List;
import java.util.concurrent.CopyOnWriteArrayList;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicReference;

import static org.junit.jupiter.api.Assertions.*;

/**
 * InvokerImpl 边缘路径补测：
 * 轮询线程中断、二次订阅的 startPolling 守卫、订阅回调抛错、
 * normalizeTaskEventType 的 null 类型归一。
 */
@DisplayName("InvokerImpl polling and subscription edge paths")
class InvokerImplEdgeTest {

    private InvokerConfig config() {
        return InvokerConfig.builder().address("127.0.0.1:19090").insecure(true).timeout(3000).build();
    }

    private static FakeTransportClient neverDoneEventTransport(CountDownLatch firstEventPolled) {
        return new FakeTransportClient((msgType, data) -> {
            if (msgType == Protocol.MSG_START_TASK_REQUEST) {
                return SdkWireMessages.encodeStartTaskResponse(new SdkWireMessages.StartTaskResponse("t-edge"));
            }
            if (msgType == Protocol.MSG_STREAM_TASK_REQUEST) {
                if (firstEventPolled != null) {
                    firstEventPolled.countDown();
                }
                return SdkWireMessages.encodeTaskEvent(new SdkWireMessages.TaskEvent(
                    "progress", "working", 10, new byte[0]));
            }
            return new byte[0];
        });
    }

    @Test
    @DisplayName("polling thread is interruptible and second subscribe skips new poller")
    void pollingInterruptAndSecondSubscribeGuard() throws Exception {
        CountDownLatch firstPoll = new CountDownLatch(1);
        FakeTransportClient transport = neverDoneEventTransport(firstPoll);
        InvokerImpl invoker = new InvokerImpl(config(), (address, timeout) -> transport);
        invoker.connect();
        String taskId = invoker.startTask("demo.fn", "{}");
        assertEquals("t-edge", taskId);

        List<TaskEventInfo> events = new CopyOnWriteArrayList<>();
        CountDownLatch first = new CountDownLatch(1);
        AtomicReference<Subscription> subscriptionRef = new AtomicReference<>();
        invoker.streamTask(taskId).subscribe(new Subscriber<>() {
            @Override public void onSubscribe(Subscription s) { subscriptionRef.set(s); s.request(1); }
            @Override public void onNext(TaskEventInfo event) { events.add(event); first.countDown(); }
            @Override public void onError(Throwable t) { }
            @Override public void onComplete() { }
        });
        assertTrue(firstPoll.await(5, TimeUnit.SECONDS), "poller never issued a request");
        assertTrue(first.await(5, TimeUnit.SECONDS), "first event never arrived");

        // 第二个订阅者：轮询已启动，startPolling 守卫直接返回
        CountDownLatch second = new CountDownLatch(1);
        invoker.streamTask(taskId).subscribe(new Subscriber<>() {
            @Override public void onSubscribe(Subscription s) { s.request(1); }
            @Override public void onNext(TaskEventInfo event) { second.countDown(); }
            @Override public void onError(Throwable t) { }
            @Override public void onComplete() { }
        });

        // 轮询线程在 500ms sleep 中被中断
        Thread poller = awaitThreadByName("croupier-java-task-poller-" + taskId, 5000);
        assertNotNull(poller);
        poller.interrupt();
        poller.join(3000);
        assertFalse(poller.isAlive(), "poller should exit after interrupt");

        subscriptionRef.get().cancel();
        invoker.close();
    }

    @Test
    @DisplayName("subscriber throwing in onSubscribe surfaces onError")
    void subscriberThrowingInOnSubscribeSurfacesError() throws Exception {
        FakeTransportClient transport = neverDoneEventTransport(null);
        InvokerImpl invoker = new InvokerImpl(config(), (address, timeout) -> transport);
        invoker.connect();
        String taskId = invoker.startTask("demo.fn", "{}");

        CountDownLatch errored = new CountDownLatch(1);
        AtomicReference<Throwable> seen = new AtomicReference<>();
        invoker.streamTask(taskId).subscribe(new Subscriber<>() {
            @Override public void onSubscribe(Subscription s) {
                throw new IllegalStateException("bad subscriber");
            }
            @Override public void onNext(TaskEventInfo event) { }
            @Override public void onError(Throwable t) { seen.set(t); errored.countDown(); }
            @Override public void onComplete() { }
        });
        assertTrue(errored.await(5, TimeUnit.SECONDS), "onError not signalled");
        assertTrue(seen.get() instanceof IllegalStateException);
        invoker.close();
    }

    @Test
    @DisplayName("normalizeTaskEventType tolerates null type")
    void normalizeTaskEventTypeToleratesNull() throws Exception {
        java.lang.reflect.Method method = InvokerImpl.class
            .getDeclaredMethod("normalizeTaskEventType", String.class, String.class);
        method.setAccessible(true);
        InvokerImpl invoker = new InvokerImpl(config(), (address, timeout) ->
            new FakeTransportClient((msgType, data) -> new byte[0]));
        assertEquals("", method.invoke(invoker, null, "m"));
        assertEquals("completed", method.invoke(invoker, "done", "m"));
        assertEquals("cancelled", method.invoke(invoker, "ERROR", "task was cancelled by admin"));
        assertEquals("progress", method.invoke(invoker, "PROGRESS", "m"));
    }

    private static Thread awaitThreadByName(String name, long timeoutMs) throws InterruptedException {
        long deadline = System.currentTimeMillis() + timeoutMs;
        while (System.currentTimeMillis() < deadline) {
            for (Thread t : Thread.getAllStackTraces().keySet()) {
                if (name.equals(t.getName())) {
                    return t;
                }
            }
            Thread.sleep(10);
        }
        return null;
    }
}
