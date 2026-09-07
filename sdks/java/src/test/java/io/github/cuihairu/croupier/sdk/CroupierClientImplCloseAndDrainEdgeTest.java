package io.github.cuihairu.croupier.sdk;

import io.github.cuihairu.croupier.sdk.invoker.InvokerConfig;
import io.github.cuihairu.croupier.sdk.invoker.InvokerException;
import io.github.cuihairu.croupier.sdk.invoker.InvokerImpl;
import io.github.cuihairu.croupier.sdk.transport.TransportClient;
import io.github.cuihairu.croupier.sdk.wire.SdkWireMessages;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

import java.lang.reflect.Field;
import java.lang.reflect.Method;
import java.nio.charset.StandardCharsets;
import java.nio.file.Path;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicInteger;

import static org.junit.jupiter.api.Assertions.*;

/**
 * CroupierClientImpl 边缘补测：
 * drain 不可解析 body 的告警分支、drain 恢复中 closeTransport 抛错的兜底、
 * close() 时 invoker.close() 抛 InvokerException 仅告警、
 * maxFileSize<=0 时 file push 走默认 10MB 上限分支。
 */
@DisplayName("CroupierClientImpl close and drain recovery edge paths")
class CroupierClientImplCloseAndDrainEdgeTest {

    private static Object invokePrivate(Object target, String name, Class<?>[] types, Object[] args)
            throws Exception {
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

    private static void setField(Object target, String name, Object value) throws Exception {
        Field f = target.getClass().getDeclaredField(name);
        f.setAccessible(true);
        f.set(target, value);
    }

    private ClientConfig baseConfig() {
        ClientConfig config = new ClientConfig("game-1", "svc-1");
        config.setEnv("development");
        config.setServiceVersion("1.0.0");
        config.setAgentAddr("tcp://127.0.0.1:1");
        return config;
    }

    @Test
    @DisplayName("drain 不可解析 body：仍回确认并触发恢复，状态最终清空")
    void drainWithUnparsableBodyStillAcks() throws Exception {
        CroupierClientImpl client = new CroupierClientImpl(baseConfig());
        // 首次 drain（draining=false）：body 截断 → 解码抛错 → catch 分支告警后仍启动恢复
        byte[] ack = (byte[]) invokePrivate(client, "handleDrainRequest",
            new Class<?>[]{byte[].class}, new Object[]{new byte[]{0x0A, 0x05, 'x'}});
        assertArrayEquals(SdkWireMessages.encodeProviderDrainResponse(), ack);

        AtomicBoolean draining = (AtomicBoolean) field(client, "draining");
        long deadline = System.currentTimeMillis() + 8000;
        while (draining.get() && System.currentTimeMillis() < deadline) {
            Thread.sleep(20);
        }
        assertFalse(draining.get(), "draining must clear after recovery");
        client.stop();
    }

    @Test
    @DisplayName("drain 恢复中 closeTransport 抛错：记录错误且 drain 状态仍被清空")
    void drainRecoveryFailureIsLoggedAndFlagCleared() throws Exception {
        ClientConfig config = baseConfig();
        config.setReconnect(ReconnectConfig.builder().enabled(false).build());
        CroupierClientImpl client = new CroupierClientImpl(config);
        AtomicBoolean closed = new AtomicBoolean(false);
        AtomicInteger closes = new AtomicInteger();
        setField(client, "transport", new TransportClient() {
            @Override public void connect() { }
            @Override public byte[] request(int msgType, byte[] data) { return new byte[0]; }
            @Override public boolean isConnected() { return false; }
            @Override public void close() {
                // 首次 close 抛错（驱动 drainAndRecover 的 catch 兜底）；后续 close 正常，
                // 让测试收尾的 stop() 不被同一异常打断
                if (closes.incrementAndGet() == 1) {
                    closed.set(true);
                    throw new IllegalStateException("close boom");
                }
            }
        });

        byte[] ack = (byte[]) invokePrivate(client, "handleDrainRequest",
            new Class<?>[]{byte[].class},
            new Object[]{SdkWireMessages.encodeProviderDrainRequest(
                new SdkWireMessages.ProviderDrainRequest("s-1", "restart", 0))});
        assertArrayEquals(SdkWireMessages.encodeProviderDrainResponse(), ack);

        AtomicBoolean draining = (AtomicBoolean) field(client, "draining");
        long deadline = System.currentTimeMillis() + 8000;
        while ((draining.get() || !closed.get()) && System.currentTimeMillis() < deadline) {
            Thread.sleep(20);
        }
        assertTrue(closed.get(), "transport close should have been attempted");
        assertFalse(draining.get(), "draining must clear even when recovery fails");
        client.stop();
    }

    @Test
    @DisplayName("close() 时 invoker.close() 抛 InvokerException 仅告警不中断")
    void closeSwallowsInvokerCloseFailure() throws Exception {
        CroupierClientImpl client = new CroupierClientImpl(baseConfig());
        io.github.cuihairu.croupier.sdk.invoker.Invoker failingInvoker =
            new InvokerImpl(InvokerConfig.builder().build()) {
                @Override public void close() throws InvokerException {
                    throw new InvokerException(
                        InvokerException.ErrorCode.INTERNAL, "invoker close failed");
                }
            };
        setField(client, "invoker", failingInvoker);
        assertDoesNotThrow(client::close);
    }

    @Test
    @DisplayName("maxFileSize<=0 时 file push 走默认 10MB 上限分支")
    void filePushWithNonPositiveMaxFileSizeUsesDefaultLimit(@TempDir Path tempDir) throws Exception {
        ClientConfig config = new ClientConfig("game-1", "svc-1");
        config.setEnableFileTransfer(true);
        config.setMaxFileSize(0);
        config.setFileStagingDir(tempDir.toString());
        CroupierClientImpl client = new CroupierClientImpl(config);

        byte[] data = "payload".getBytes(StandardCharsets.UTF_8);
        byte[] raw = (byte[]) invokePrivate(client, "handleFilePushRequest",
            new Class<?>[]{byte[].class},
            new Object[]{SdkWireMessages.encodeFilePushRequest(new SdkWireMessages.FilePushRequest(
                "t-default-max", "hot.lua", sha256(data), data))});
        assertTrue(SdkWireMessages.decodeFilePushResponse(raw).ok);
        client.stop();
    }

    private static String sha256(byte[] data) throws Exception {
        byte[] digest = java.security.MessageDigest.getInstance("SHA-256").digest(data);
        StringBuilder sb = new StringBuilder(digest.length * 2);
        for (byte b : digest) {
            sb.append(String.format("%02x", b));
        }
        return sb.toString();
    }
}
