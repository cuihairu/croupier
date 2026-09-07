package io.github.cuihairu.croupier.sdk.invoker;

import io.github.cuihairu.croupier.sdk.testing.FakeTransportClient;
import io.github.cuihairu.croupier.sdk.transport.Protocol;
import io.github.cuihairu.croupier.sdk.wire.SdkWireMessages;
import org.junit.jupiter.api.Test;

import java.lang.reflect.Field;
import java.nio.charset.StandardCharsets;

import static org.junit.jupiter.api.Assertions.assertDoesNotThrow;
import static org.junit.jupiter.api.Assertions.assertThrows;

/** Reflection-driven fillers for guard branches unreachable via public API. */
class ReflectionGuardBranchTest {

    private static InvokerImpl connectedInvoker(FakeTransportClient transport) {
        InvokerImpl invoker = new InvokerImpl(InvokerConfig.builder()
            .address("127.0.0.1:19090").insecure(true).timeout(2000)
            .retry(RetryConfig.builder().enabled(false).build())
            .build(), (address, timeout) -> transport);
        assertDoesNotThrow(invoker::connect);
        return invoker;
    }

    @Test
    void requireTransportFailsWhenTransportFieldIsNull() throws Exception {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> new byte[0]);
        InvokerImpl invoker = connectedInvoker(transport);
        // simulate a transport that vanished while the connected flag lags behind
        Field transportField = InvokerImpl.class.getDeclaredField("transport");
        transportField.setAccessible(true);
        transportField.set(invoker, null);
        InvokerException error = assertThrows(InvokerException.class, () -> invoker.invoke("fn", "{}"));
        assert error.getErrorCode() == InvokerException.ErrorCode.UNAVAILABLE;
        assertDoesNotThrow(invoker::close);
    }

    @Test
    void cancelThenWaitLetsPollerExitThroughLoopCondition() throws Exception {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            if (msgType == Protocol.MSG_START_TASK_REQUEST) {
                return SdkWireMessages.encodeStartTaskResponse(
                    new SdkWireMessages.StartTaskResponse("task-exit"));
            }
            return SdkWireMessages.encodeTaskEvent(new SdkWireMessages.TaskEvent(
                "progress", "working", 10, new byte[0]));
        });
        InvokerImpl invoker = connectedInvoker(transport);
        String taskId = assertDoesNotThrow(() -> invoker.startTask("fn", "{}"));
        assertDoesNotThrow(() -> invoker.cancelTask(taskId));
        // let the poller wake from its sleep and observe shouldStopPolling
        Thread.sleep(800);
        assertDoesNotThrow(invoker::close);
    }

    @Test
    void publishTaskEventForUnknownTaskIsNullSafe() throws Exception {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> new byte[0]);
        InvokerImpl invoker = connectedInvoker(transport);
        // unknown task: publishTaskEvent must silently ignore the missing state
        java.lang.reflect.Method publish = InvokerImpl.class
            .getDeclaredMethod("publishTaskEvent", String.class, TaskEventInfo.class);
        publish.setAccessible(true);
        publish.invoke(invoker, "missing-task", TaskEventInfo.builder()
            .type("progress").taskId("missing-task").done(false).build());
        assertDoesNotThrow(invoker::close);
    }

    @Test
    void isSameEventHandlesNullOperands() throws Exception {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> new byte[0]);
        InvokerImpl invoker = connectedInvoker(transport);
        java.lang.reflect.Method same = InvokerImpl.class
            .getDeclaredMethod("isSameEvent", TaskEventInfo.class, TaskEventInfo.class);
        same.setAccessible(true);
        TaskEventInfo event = TaskEventInfo.builder().type("progress").taskId("t").done(false).build();
        // left == null short-circuits to false
        assert !(Boolean) same.invoke(invoker, null, event);
        assert (Boolean) same.invoke(invoker, event, event);
        assert !(Boolean) same.invoke(invoker, event,
            TaskEventInfo.builder().type("other").taskId("t").done(false).build());
        assertDoesNotThrow(invoker::close);
    }

    @Test
    void wireStringsSurvivePayloadOnlyInvoke() throws Exception {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) ->
            SdkWireMessages.encodeInvokeResponse(new SdkWireMessages.InvokeResponse(
                "plain".getBytes(StandardCharsets.UTF_8))));
        InvokerImpl invoker = connectedInvoker(transport);
        String result = assertDoesNotThrow(() -> invoker.invoke("fn", null));
        assert "plain".equals(result);
        assertDoesNotThrow(invoker::close);
    }
}
