package io.github.cuihairu.croupier.sdk.invoker;

import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.Assumptions;
import org.reactivestreams.Subscriber;
import org.reactivestreams.Subscription;

import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

/** Opt-in lifecycle test against the repository's real Server fixture. */
class ServerHttpInvokerRealTest {

    @Test
    void usesRealServerHttpLifecycle() throws Exception {
        String serverURL = System.getenv("CROUPIER_SERVER_URL");
        String token = System.getenv("CROUPIER_SERVER_TOKEN");
        Assumptions.assumeTrue(serverURL != null && !serverURL.isBlank() && token != null && !token.isBlank(),
            "real Server fixture variables are not configured");

        String gameID = valueOrDefault(System.getenv("CROUPIER_GAME_ID"), "e2e-game");
        String env = valueOrDefault(System.getenv("CROUPIER_ENV"), "e2e");
        ServerHttpInvoker unauthenticated = new ServerHttpInvoker(InvokerConfig.builder()
            .address(serverURL).gameId(gameID).env(env).retry(RetryConfig.builder().enabled(false).build()).build());
        assertThrowsInvoker(() -> unauthenticated.invoke("mail.send", "{\"player_id\":\"p-001\",\"title\":\"denied\"}"));

        ServerHttpInvoker invoker = new ServerHttpInvoker(InvokerConfig.builder()
            .address(serverURL).authToken(token).gameId(gameID).env(env).taskPollIntervalMs(10)
            .retry(RetryConfig.builder().enabled(false).build()).build());
        invoker.connect();

        String result = invoker.invoke("mail.send", "{\"player_id\":\"p-001\",\"title\":\"Java\",\"content\":\"body\"}");
        assertTrue(result.contains("\"mail_id\":\"mail-0001\""), result);

        String completedID = invoker.startTask("mail.send", "{\"player_id\":\"p-001\",\"title\":\"Java task\"}");
        CollectingSubscriber completedEvents = new CollectingSubscriber();
        invoker.streamTask(completedID).subscribe(completedEvents);
        assertTrue(completedEvents.done.await(20, TimeUnit.SECONDS));
        assertTrue(completedEvents.types.contains("started"), completedEvents.types.toString());
        assertTrue(completedEvents.types.contains("completed"), completedEvents.types.toString());
        TaskStatusInfo completed = waitForStatus(invoker, completedID, "succeeded");
        assertEquals(completedID, completed.taskId());

        String cancelledID = invoker.startTask("mail.wait", "{\"wait_ms\":30000}");
        waitForStatus(invoker, cancelledID, "running");
        invoker.cancelTask(cancelledID);
        CollectingSubscriber cancelledEvents = new CollectingSubscriber();
        invoker.streamTask(cancelledID).subscribe(cancelledEvents);
        assertTrue(cancelledEvents.done.await(20, TimeUnit.SECONDS));
        assertTrue(cancelledEvents.types.contains("cancelled"), cancelledEvents.types.toString());
        waitForStatus(invoker, cancelledID, "cancelled");
        invoker.close();
    }

    private static String valueOrDefault(String value, String fallback) {
        return value == null || value.isBlank() ? fallback : value;
    }

    private static TaskStatusInfo waitForStatus(ServerHttpInvoker invoker, String taskID, String expected) throws Exception {
        long deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(20);
        TaskStatusInfo status = null;
        while (System.nanoTime() < deadline) {
            status = invoker.getTaskStatus(taskID);
            if (expected.equals(status.status())) return status;
            Thread.sleep(50);
        }
        throw new AssertionError("task " + taskID + " status=" + (status == null ? null : status.status()) + ", want " + expected);
    }

    private static void assertThrowsInvoker(ThrowingRunnable action) {
        try {
            action.run();
        } catch (InvokerException expected) {
            return;
        }
        throw new AssertionError("unauthenticated Server request unexpectedly succeeded");
    }

    @FunctionalInterface
    private interface ThrowingRunnable { void run() throws InvokerException; }

    private static final class CollectingSubscriber implements Subscriber<TaskEventInfo> {
        private final List<String> types = new ArrayList<>();
        private final CountDownLatch done = new CountDownLatch(1);

        @Override public void onSubscribe(Subscription subscription) { subscription.request(Long.MAX_VALUE); }
        @Override public void onNext(TaskEventInfo event) { types.add(event.getType()); }
        @Override public void onError(Throwable error) { throw new AssertionError(error); }
        @Override public void onComplete() { done.countDown(); }
    }
}
