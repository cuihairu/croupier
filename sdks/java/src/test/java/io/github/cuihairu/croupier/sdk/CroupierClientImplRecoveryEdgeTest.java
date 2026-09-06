package io.github.cuihairu.croupier.sdk;

import io.github.cuihairu.croupier.sdk.transport.Protocol;
import io.github.cuihairu.croupier.sdk.transport.TransportClient;
import io.github.cuihairu.croupier.sdk.wire.SdkWireMessages;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.lang.reflect.Field;
import java.lang.reflect.Method;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.HashMap;
import java.util.Map;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicLong;
import java.util.concurrent.atomic.AtomicReference;
import java.util.function.BiFunction;

import static org.junit.jupiter.api.Assertions.*;

/**
 * CroupierClientImpl 边缘路径补测：
 * 重连守卫/退避中断/上限、控制面注册失败、handleLocalRequest 的 drain 与
 * 文件推送路由、drain 状态不可解析 body 与中断、startTask 校验失败、
 * 任务取消后 handler 抛错、escapeJson(null)。
 */
@DisplayName("CroupierClientImpl recovery and inbound edge paths")
class CroupierClientImplRecoveryEdgeTest {

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
    @DisplayName("recoverConnection 的并发守卫直接返回")
    void recoverConnectionGuard() throws Exception {
        CroupierClientImpl client = new CroupierClientImpl(baseConfig());
        // inflight 阻塞恢复线程，让 reconnecting 标志保持 true
        ((AtomicLong) field(client, "inflightCalls")).incrementAndGet();
        invokePrivate(client, "recoverConnection", new Class<?>[0], new Object[0]);
        // 第二次进入：CAS 失败直接返回，不再创建恢复线程
        invokePrivate(client, "recoverConnection", new Class<?>[0], new Object[0]);
        ((AtomicLong) field(client, "inflightCalls")).decrementAndGet();
        awaitFlagCleared((AtomicBoolean) field(client, "reconnecting"), 8000);
        client.stop();
    }

    @Test
    @DisplayName("重连退避 sleep 可被中断且延迟被 maxDelayMs 封顶")
    void reconnectBackoffInterruptAndCap() throws Exception {
        ClientConfig config = baseConfig();
        // 初次失败后退避 300ms；倍增后超过 maxDelayMs 触发封顶分支
        ReconnectConfig rc = ReconnectConfig.builder()
            .enabled(true)
            .initialDelayMs(300)
            .maxDelayMs(400)
            .backoffMultiplier(10)
            .maxAttempts(0)
            .build();
        config.setReconnect(rc);

        java.util.concurrent.atomic.AtomicInteger connectCalls = new java.util.concurrent.atomic.AtomicInteger();
        BiFunction<String, Integer, TransportClient> factory = (address, timeout) -> new TransportClient() {
            @Override public void connect() {
                connectCalls.incrementAndGet();
                throw new RuntimeException("agent down");
            }
            @Override public byte[] request(int msgType, byte[] data) { return new byte[0]; }
            @Override public boolean isConnected() { return false; }
            @Override public void close() { }
        };
        CroupierClientImpl client = new CroupierClientImpl(config, factory);
        // reconnectOnce 要求至少注册一个函数；恢复线程仅在 serving 状态下持续重试
        client.registerFunction(new FunctionDescriptor("demo.fn", "1.0.0"), (ctx, payload) -> "{}");
        ((AtomicBoolean) field(client, "serving")).set(true);
        invokePrivate(client, "recoverConnection", new Class<?>[0], new Object[0]);
        Thread recovery = awaitThreadByName("croupier-java-client-reconnect", 5000);
        assertNotNull(recovery, "recovery thread should start");
        // 第一次尝试后进入 ~300ms 退避；等待第二次尝试（此后 delay 已被 maxDelayMs 封顶）
        long deadline = System.currentTimeMillis() + 10_000;
        while (connectCalls.get() < 2 && System.currentTimeMillis() < deadline) {
            Thread.sleep(10);
        }
        assertTrue(connectCalls.get() >= 2, "second reconnect attempt should happen");
        Thread.sleep(150);
        recovery.interrupt();
        recovery.join(5000);
        assertFalse(recovery.isAlive(), "recovery thread should exit after interrupt");
        awaitFlagCleared((AtomicBoolean) field(client, "reconnecting"), 8000);
        client.stop();
    }

    @Test
    @DisplayName("控制面能力上传失败仅告警不影响主流程")
    void capabilitiesUploadFailureIsSwallowed() throws Exception {
        ClientConfig config = baseConfig();
        config.setControlAddr("tcp://127.0.0.1:1");
        AtomicBoolean threw = new AtomicBoolean(false);
        BiFunction<String, Integer, TransportClient> factory = (address, timeout) -> new TransportClient() {
            @Override public void connect() {
                threw.set(true);
                throw new RuntimeException("control plane unreachable");
            }
            @Override public byte[] request(int msgType, byte[] data) { return new byte[0]; }
            @Override public boolean isConnected() { return false; }
            @Override public void close() { }
        };
        CroupierClientImpl client = new CroupierClientImpl(config, factory);
        invokePrivate(client, "maybeRegisterCapabilities", new Class<?>[0], new Object[0]);
        assertTrue(threw.get(), "control connect should have been attempted");
        client.stop();
    }

    @Test
    @DisplayName("handleLocalRequest 路由 drain 与文件推送消息")
    void handleLocalRequestRoutesDrainAndFilePush() throws Exception {
        ClientConfig config = baseConfig();
        config.setEnableFileTransfer(true);
        CroupierClientImpl client = new CroupierClientImpl(config);
        ((AtomicLong) field(client, "inflightCalls")).incrementAndGet();

        // drain 路由：回空确认
        byte[] drainAck = (byte[]) invokePrivate(client, "handleLocalRequest",
            new Class<?>[]{int.class, int.class, byte[].class},
            new Object[]{Protocol.MSG_PROVIDER_DRAIN_REQUEST, 1,
                SdkWireMessages.encodeProviderDrainRequest(
                    new SdkWireMessages.ProviderDrainRequest("s-1", "restart", 100))});
        assertArrayEquals(SdkWireMessages.encodeProviderDrainResponse(), drainAck);

        // 不可解析 drain body：仍回确认并触发恢复
        byte[] badAck = (byte[]) invokePrivate(client, "handleLocalRequest",
            new Class<?>[]{int.class, int.class, byte[].class},
            new Object[]{Protocol.MSG_PROVIDER_DRAIN_REQUEST, 2,
                new byte[]{0x0A, 0x05, 'x'}});
        assertArrayEquals(SdkWireMessages.encodeProviderDrainResponse(), badAck);

        // 文件推送路由：staging 未就绪时回错误响应（路由本身被覆盖）
        byte[] fileData = "x".getBytes(StandardCharsets.UTF_8);
        byte[] pushRaw = (byte[]) invokePrivate(client, "handleLocalRequest",
            new Class<?>[]{int.class, int.class, byte[].class},
            new Object[]{Protocol.MSG_PROVIDER_FILE_PUSH_REQ, 3,
                SdkWireMessages.encodeFilePushRequest(new SdkWireMessages.FilePushRequest(
                    "", "a.lua", "", fileData))});
        SdkWireMessages.FilePushResponse pushResponse =
            SdkWireMessages.decodeFilePushResponse(pushRaw);
        assertFalse(pushResponse.ok);
        assertTrue(pushResponse.error.contains("transferId is required"));

        // 等待 drain 线程进入等待循环后中断它（覆盖 InterruptedException 分支）
        Thread drainThread = awaitThreadByName("croupier-java-client-drain", 5000);
        assertNotNull(drainThread);
        Thread.sleep(100);
        drainThread.interrupt();
        drainThread.join(5000);

        ((AtomicLong) field(client, "inflightCalls")).decrementAndGet();
        awaitFlagCleared((AtomicBoolean) field(client, "draining"), 8000);
        client.stop();
    }

    @Test
    @DisplayName("drain 完成后自动重连关闭分支（reconnect 关闭时关传输）")
    void drainWithReconnectDisabledClosesTransport() throws Exception {
        ClientConfig config = baseConfig();
        config.setReconnect(ReconnectConfig.builder().enabled(false).build());
        CroupierClientImpl client = new CroupierClientImpl(config);
        AtomicBoolean closed = new AtomicBoolean(false);
        TransportClient fake = new TransportClient() {
            @Override public void connect() { }
            @Override public byte[] request(int msgType, byte[] data) { return new byte[0]; }
            @Override public boolean isConnected() { return true; }
            @Override public void close() { closed.set(true); }
        };
        setField(client, "transport", fake);

        byte[] ack = (byte[]) invokePrivate(client, "handleDrainRequest",
            new Class<?>[]{byte[].class},
            new Object[]{SdkWireMessages.encodeProviderDrainRequest(
                new SdkWireMessages.ProviderDrainRequest("s-1", "restart", 0))});
        assertArrayEquals(SdkWireMessages.encodeProviderDrainResponse(), ack);
        awaitFlag(closed, 5000, "transport should be closed");
        awaitFlagCleared((AtomicBoolean) field(client, "draining"), 8000);
        client.stop();
    }

    @Test
    @DisplayName("startTask 入站校验失败：任务不启动")
    void startTaskValidationFailureThrows() throws Exception {
        ClientConfig config = baseConfig();
        config.setValidateInputPayloads(true);
        CroupierClientImpl client = new CroupierClientImpl(config);
        FunctionDescriptor descriptor = new FunctionDescriptor("demo.fn", "1.0.0");
        descriptor.setInputSchema(
            "{\"type\":\"object\",\"required\":[\"playerId\"],\"properties\":{\"playerId\":{\"type\":\"string\"}}}");
        client.registerFunction(descriptor, (ctx, payload) -> "{}");

        byte[] body = SdkWireMessages.encodeInvokeRequest(new SdkWireMessages.InvokeRequest(
            "demo.fn", "", "{}".getBytes(StandardCharsets.UTF_8), Map.of()));
        CroupierException error = assertThrows(CroupierException.class,
            () -> invokePrivate(client, "handleStartTaskRequest",
                new Class<?>[]{byte[].class}, new Object[]{body}));
        assertTrue(error.getMessage().contains("validation failed"));
        client.stop();
    }

    @Test
    @DisplayName("任务取消后 handler 抛错不产生 error 事件")
    void cancelledTaskSwallowsHandlerError() throws Exception {
        CroupierClientImpl client = new CroupierClientImpl(baseConfig());
        AtomicBoolean handlerDone = new AtomicBoolean(false);
        client.registerFunction(new FunctionDescriptor("slow.fn", "1.0.0"), (ctx, payload) -> {
            try {
                Thread.sleep(400);
            } catch (InterruptedException ignored) {
                Thread.currentThread().interrupt();
            }
            handlerDone.set(true);
            throw new IllegalStateException("boom after cancel");
        });

        byte[] started = (byte[]) invokePrivate(client, "handleStartTaskRequest",
            new Class<?>[]{byte[].class},
            new Object[]{SdkWireMessages.encodeInvokeRequest(new SdkWireMessages.InvokeRequest(
                "slow.fn", "", "{}".getBytes(StandardCharsets.UTF_8), Map.of()))});
        String taskId = SdkWireMessages.decodeStartTaskResponse(started).taskId;

        // handler 完成前取消任务
        byte[] cancelBody = SdkWireMessages.encodeCancelTaskRequest(
            new SdkWireMessages.CancelTaskRequest(taskId));
        invokePrivate(client, "handleCancelTaskRequest",
            new Class<?>[]{byte[].class}, new Object[]{cancelBody});

        long deadline = System.currentTimeMillis() + 5000;
        while (!handlerDone.get() && System.currentTimeMillis() < deadline) {
            Thread.sleep(20);
        }
        assertTrue(handlerDone.get(), "handler should have run to its failure");

        // 取消后流式读取应停在 cancelled 事件，而非 error
        byte[] streamBody = SdkWireMessages.encodeTaskStreamRequest(
            new SdkWireMessages.TaskStreamRequest(taskId));
        byte[] event = (byte[]) invokePrivate(client, "handleStreamTaskRequest",
            new Class<?>[]{byte[].class}, new Object[]{streamBody});
        assertEquals("cancelled", SdkWireMessages.decodeTaskEvent(event).type);
        client.stop();
    }

    @Test
    @DisplayName("invoke 的 metadata 含 null 值时 JSON 序列化为空串")
    void invokeSerializesNullMetadataValue() throws Exception {
        CroupierClientImpl client = new CroupierClientImpl(baseConfig());
        AtomicReference<String> seenContext = new AtomicReference<>();
        client.registerFunction(new FunctionDescriptor("echo.fn", "1.0.0"), (ctx, payload) -> {
            seenContext.set(ctx);
            return "{}";
        });
        Map<String, String> metadata = new HashMap<>();
        metadata.put("who", null);
        String result = client.invoke("echo.fn", "{}", metadata);
        assertEquals("{}", result);
        assertNotNull(seenContext.get());
        assertTrue(seenContext.get().contains("\"who\":\"\""), "null value should serialize as empty");
        client.stop();
    }

    @Test
    @DisplayName("file push 默认暂存目录走相对路径守卫；写失败回错误响应")
    void filePushDefaultStagingAndWriteFailure(@org.junit.jupiter.api.io.TempDir Path tempDir)
            throws Exception {
        // staging 未配置（null）→ 使用默认 ./croupier-staging（相对路径）；
        // normalize 后的 target 与含 "." 前缀的 staging 不匹配 → 回 basename 错误
        ClientConfig defaultDirConfig = new ClientConfig("game-1", "svc-1");
        defaultDirConfig.setEnableFileTransfer(true);
        defaultDirConfig.setFileStagingDir(null);
        CroupierClientImpl client = new CroupierClientImpl(defaultDirConfig);
        byte[] data = "payload".getBytes(StandardCharsets.UTF_8);
        byte[] raw = (byte[]) invokePrivate(client, "handleFilePushRequest",
            new Class<?>[]{byte[].class},
            new Object[]{SdkWireMessages.encodeFilePushRequest(new SdkWireMessages.FilePushRequest(
                "t-default", "default.lua", sha256(data), data))});
        SdkWireMessages.FilePushResponse response = SdkWireMessages.decodeFilePushResponse(raw);
        assertFalse(response.ok, "relative default staging cannot pass the normalize guard");
        assertTrue(response.error.contains("bare basename"));
        // 清理被 createDirectories 建出的空暂存目录
        Files.deleteIfExists(Path.of("croupier-staging"));
        client.stop();

        // staging 指向一个普通文件 → createDirectories 失败 → 错误响应
        Path blockingFile = tempDir.resolve("blocking");
        Files.writeString(blockingFile, "occupied");
        ClientConfig blockedConfig = new ClientConfig("game-1", "svc-1");
        blockedConfig.setEnableFileTransfer(true);
        blockedConfig.setFileStagingDir(blockingFile.toString());
        CroupierClientImpl blocked = new CroupierClientImpl(blockedConfig);
        byte[] failRaw = (byte[]) invokePrivate(blocked, "handleFilePushRequest",
            new Class<?>[]{byte[].class},
            new Object[]{SdkWireMessages.encodeFilePushRequest(new SdkWireMessages.FilePushRequest(
                "t-block", "a.lua", sha256(data), data))});
        SdkWireMessages.FilePushResponse failResponse = SdkWireMessages.decodeFilePushResponse(failRaw);
        assertFalse(failResponse.ok);
        assertTrue(failResponse.error.contains("write staging file"));
        blocked.stop();
    }

    @Test
    @DisplayName("file push 空 payload / 未配置 maxFileSize / 缺 sha 校验")
    void filePushGuardBranches(@org.junit.jupiter.api.io.TempDir Path tempDir) throws Exception {
        ClientConfig config = new ClientConfig("game-1", "svc-1");
        config.setEnableFileTransfer(true);
        // maxFileSize 不设置（<=0）→ 走默认 10MB 分支
        config.setFileStagingDir(tempDir.toString());
        CroupierClientImpl client = new CroupierClientImpl(config);

        byte[] data = "x".getBytes(StandardCharsets.UTF_8);

        // 空 payload
        byte[] emptyRaw = (byte[]) invokePrivate(client, "handleFilePushRequest",
            new Class<?>[]{byte[].class},
            new Object[]{SdkWireMessages.encodeFilePushRequest(new SdkWireMessages.FilePushRequest(
                "t-empty", "a.lua", sha256(data), new byte[0]))});
        SdkWireMessages.FilePushResponse emptyResponse = SdkWireMessages.decodeFilePushResponse(emptyRaw);
        assertFalse(emptyResponse.ok);
        assertTrue(emptyResponse.error.contains("file payload is empty"));

        // 缺 sha
        byte[] noShaRaw = (byte[]) invokePrivate(client, "handleFilePushRequest",
            new Class<?>[]{byte[].class},
            new Object[]{SdkWireMessages.encodeFilePushRequest(new SdkWireMessages.FilePushRequest(
                "t-nosha", "a.lua", "  ", data))});
        SdkWireMessages.FilePushResponse noShaResponse = SdkWireMessages.decodeFilePushResponse(noShaRaw);
        assertFalse(noShaResponse.ok);
        assertTrue(noShaResponse.error.contains("contentSha256 is required"));

        // 默认 maxFileSize 分支下正常落盘
        byte[] okRaw = (byte[]) invokePrivate(client, "handleFilePushRequest",
            new Class<?>[]{byte[].class},
            new Object[]{SdkWireMessages.encodeFilePushRequest(new SdkWireMessages.FilePushRequest(
                "t-ok", "a.lua", sha256(data), data))});
        assertTrue(SdkWireMessages.decodeFilePushResponse(okRaw).ok);
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

    private static Thread awaitThreadByName(String name, long timeoutMs) throws InterruptedException {
        long deadline = System.currentTimeMillis() + timeoutMs;
        while (System.currentTimeMillis() < deadline) {
            for (Thread t : Thread.getAllStackTraces().keySet()) {
                if (name.equals(t.getName()) && t.isAlive()) {
                    return t;
                }
            }
            Thread.sleep(10);
        }
        return null;
    }

    private static void awaitFlagCleared(AtomicBoolean flag, long timeoutMs) throws InterruptedException {
        long deadline = System.currentTimeMillis() + timeoutMs;
        while (flag.get() && System.currentTimeMillis() < deadline) {
            Thread.sleep(20);
        }
    }

    private static void awaitFlag(AtomicBoolean flag, long timeoutMs, String message)
            throws InterruptedException {
        long deadline = System.currentTimeMillis() + timeoutMs;
        while (!flag.get() && System.currentTimeMillis() < deadline) {
            Thread.sleep(20);
        }
    }
}
