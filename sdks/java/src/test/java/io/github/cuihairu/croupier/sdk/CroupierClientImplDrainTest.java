package io.github.cuihairu.croupier.sdk;

import io.github.cuihairu.croupier.sdk.wire.SdkWireMessages;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.lang.reflect.Field;
import java.lang.reflect.Method;
import java.nio.charset.StandardCharsets;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicLong;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Provider drain 处理：确认帧、幂等、drain 期间拒绝新 Invoke、恢复后清除状态。
 * 通过私有 handleLocalRequest/handleDrainRequest 驱动（与 LocalTaskTest 同款方式）。
 */
@DisplayName("CroupierClientImpl provider drain handling")
class CroupierClientImplDrainTest {

    private ClientConfig createConfig() {
        ClientConfig config = new ClientConfig("game-1", "svc-1");
        config.setEnv("development");
        config.setServiceVersion("1.0.0");
        return config;
    }

    private static Object invokePrivate(Object target, String name, Class<?>[] types, Object[] args) throws Exception {
        Method method = target.getClass().getDeclaredMethod(name, types);
        method.setAccessible(true);
        try {
            return method.invoke(target, args);
        } catch (java.lang.reflect.InvocationTargetException e) {
            Throwable cause = e.getCause();
            if (cause instanceof Exception ex) {
                throw ex;
            }
            throw e;
        }
    }

    private static Object field(Object target, String name) throws Exception {
        Field f = target.getClass().getDeclaredField(name);
        f.setAccessible(true);
        return f.get(target);
    }

    private byte[] drainAck(CroupierClientImpl client) throws Exception {
        byte[] body = SdkWireMessages.encodeProviderDrainRequest(
            new SdkWireMessages.ProviderDrainRequest("s-1", "rolling-restart", 1000));
        return (byte[]) invokePrivate(client, "handleDrainRequest",
            new Class<?>[]{byte[].class}, new Object[]{body});
    }

    @Test
    @DisplayName("drain 请求回空确认并置位 draining（幂等）")
    void drainAcksAndSetsFlagIdempotently() throws Exception {
        CroupierClientImpl client = new CroupierClientImpl(createConfig());
        // 预置在途调用：让 drainAndRecover 进入等待而不是瞬间清标志
        ((AtomicLong) field(client, "inflightCalls")).incrementAndGet();

        byte[] ack = drainAck(client);
        assertArrayEquals(SdkWireMessages.encodeProviderDrainResponse(), ack);
        assertTrue(((AtomicBoolean) field(client, "draining")).get(), "draining flag must be set");

        // 幂等：重复 drain 只回确认
        byte[] ack2 = drainAck(client);
        assertArrayEquals(SdkWireMessages.encodeProviderDrainResponse(), ack2);

        // 放行在途计数，等待恢复完成清状态
        ((AtomicLong) field(client, "inflightCalls")).decrementAndGet();
        AtomicBoolean flag = (AtomicBoolean) field(client, "draining");
        long deadline = System.currentTimeMillis() + 4000;
        while (flag.get() && System.currentTimeMillis() < deadline) {
            Thread.sleep(20);
        }
        assertFalse(flag.get(), "draining must clear after recovery");
    }

    @Test
    @DisplayName("drain 期间新 Invoke 被拒：返回错误 payload，handler 不执行")
    void invokeRejectedDuringDrain() throws Exception {
        CroupierClientImpl client = new CroupierClientImpl(createConfig());
        AtomicBoolean called = new AtomicBoolean(false);
        io.github.cuihairu.croupier.sdk.FunctionDescriptor desc =
            new io.github.cuihairu.croupier.sdk.FunctionDescriptor("demo.fn", "1.0.0");
        client.registerFunction(desc, (ctx, payload) -> {
            called.set(true);
            return "{}";
        });

        // 预置在途调用：避免恢复线程瞬间清标志
        ((AtomicLong) field(client, "inflightCalls")).incrementAndGet();
        drainAck(client);

        byte[] invokeBody = SdkWireMessages.encodeInvokeRequest(
            new SdkWireMessages.InvokeRequest("demo.fn", "", "{}".getBytes(StandardCharsets.UTF_8), java.util.Map.of()));
        byte[] resp = (byte[]) invokePrivate(client, "handleLocalRequest",
            new Class<?>[]{int.class, int.class, byte[].class},
            new Object[]{io.github.cuihairu.croupier.sdk.transport.Protocol.MSG_INVOKE_REQUEST, 1, invokeBody});

        SdkWireMessages.InvokeResponse decoded = SdkWireMessages.decodeInvokeResponse(resp);
        assertTrue(decoded.payloadUtf8().contains("provider is draining"));
        assertFalse(called.get(), "handler must not run while draining");

        // 清理：放行在途计数，等待恢复完成清状态
        ((AtomicLong) field(client, "inflightCalls")).decrementAndGet();
        assertTimeoutPreemptively(java.time.Duration.ofSeconds(5), () -> {
            AtomicBoolean flag = (AtomicBoolean) field(client, "draining");
            long deadline = System.currentTimeMillis() + 4000;
            while (flag.get() && System.currentTimeMillis() < deadline) {
                Thread.sleep(20);
            }
            assertFalse(flag.get(), "draining must clear after recovery");
        });
    }
}
