package io.github.cuihairu.croupier.sdk;

import io.github.cuihairu.croupier.sdk.invoker.InvokerException;
import io.github.cuihairu.croupier.sdk.testing.FakeTransportClient;
import io.github.cuihairu.croupier.sdk.transport.Protocol;
import io.github.cuihairu.croupier.sdk.transport.TCPTransport;
import io.github.cuihairu.croupier.sdk.transport.TransportClient;
import io.github.cuihairu.croupier.sdk.wire.SdkWireMessages;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.Timeout;

import java.io.ByteArrayInputStream;
import java.io.IOException;
import java.lang.reflect.Field;
import java.lang.reflect.Method;
import java.nio.charset.StandardCharsets;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicReference;
import java.util.zip.GZIPInputStream;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Tests for the provider-side local invoke/task handling in CroupierClientImpl.
 *
 * <p>These paths are exercised through the private local request dispatcher,
 * which is currently the only way to reach the local task state machine.</p>
 */
@DisplayName("CroupierClientImpl local task handling")
class CroupierClientImplLocalTaskTest {

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
            if (cause instanceof Exception exception) {
                throw exception;
            }
            throw e;
        }
    }

    @SuppressWarnings("unchecked")
    private static Map<String, FunctionDescriptor> descriptorsOf(CroupierClientImpl client) throws Exception {
        Field field = CroupierClientImpl.class.getDeclaredField("descriptors");
        field.setAccessible(true);
        return (Map<String, FunctionDescriptor>) field.get(client);
    }

    private byte[] localRequest(CroupierClientImpl client, int msgType, byte[] body) throws Exception {
        return (byte[]) invokePrivate(client, "handleLocalRequest",
            new Class<?>[]{int.class, int.class, byte[].class}, new Object[]{msgType, 0, body});
    }

    @Test
    @DisplayName("handleLocalRequest dispatches invoke requests to registered handlers")
    @Timeout(5)
    void localInvokeRoundTrip() throws Exception {
        AtomicReference<String> context = new AtomicReference<>();
        CroupierClientImpl client = new CroupierClientImpl(createConfig(), (address, timeout) -> new FakeTransportClient(null));
        client.registerFunction(new FunctionDescriptor("echo", "1.0.0"), (ctx, payload) -> {
            context.set(ctx);
            return "hello " + payload;
        });

        Map<String, String> metadata = new LinkedHashMap<>();
        metadata.put("k", "v");
        metadata.put("k2", "v2");
        byte[] response = localRequest(client, Protocol.MSG_INVOKE_REQUEST, SdkWireMessages.encodeInvokeRequest(
            new SdkWireMessages.InvokeRequest("echo", "", "{\"x\":1}".getBytes(StandardCharsets.UTF_8), metadata)));

        assertEquals("{\"k\":\"v\",\"k2\":\"v2\"}", context.get());
        assertEquals("hello {\"x\":1}", SdkWireMessages.decodeInvokeResponse(response).payloadUtf8());
    }

    @Test
    @DisplayName("handleLocalRequest rejects unsupported message types")
    @Timeout(5)
    void localInvokeUnsupportedType() throws Exception {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(), (address, timeout) -> new FakeTransportClient(null));
        CroupierException error = assertThrows(CroupierException.class, () ->
            localRequest(client, Protocol.MSG_REGISTER_REQUEST, new byte[0]));
        assertTrue(error.getMessage().contains("Unsupported local request type"));
    }

    @Test
    @DisplayName("local start task runs handler async and streams completed event")
    @Timeout(10)
    void localStartTaskCompletes() throws Exception {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(), (address, timeout) -> new FakeTransportClient(null));
        client.registerFunction(new FunctionDescriptor("job", "1.0.0"), (ctx, payload) -> "job-result");

        byte[] response = localRequest(client, Protocol.MSG_START_TASK_REQUEST, SdkWireMessages.encodeInvokeRequest(
            new SdkWireMessages.InvokeRequest("job", "", "in".getBytes(StandardCharsets.UTF_8), Map.of())));
        String taskId = SdkWireMessages.decodeStartTaskResponse(response).taskId;
        assertTrue(taskId.startsWith("job-"), taskId);

        SdkWireMessages.TaskEvent event = awaitEvent(client, taskId, "completed", 5000);
        assertEquals("job-result", new String(event.payload, StandardCharsets.UTF_8));
        assertEquals(100, event.progress);
    }

    @Test
    @DisplayName("local start task with unknown function fails")
    @Timeout(5)
    void localStartTaskUnknownFunction() throws Exception {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(), (address, timeout) -> new FakeTransportClient(null));
        client.registerFunction(new FunctionDescriptor("known", "1.0.0"), (ctx, payload) -> "ok");

        CroupierException error = assertThrows(CroupierException.class, () ->
            localRequest(client, Protocol.MSG_START_TASK_REQUEST, SdkWireMessages.encodeInvokeRequest(
                new SdkWireMessages.InvokeRequest("missing", "", new byte[0], Map.of()))));
        assertEquals("Function not found: missing", error.getMessage());
    }

    @Test
    @DisplayName("local start task with failing handler streams error event")
    @Timeout(10)
    void localStartTaskHandlerFails() throws Exception {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(), (address, timeout) -> new FakeTransportClient(null));
        client.registerFunction(new FunctionDescriptor("boom", "1.0.0"), (ctx, payload) -> {
            throw new IllegalStateException("handler exploded");
        });

        byte[] response = localRequest(client, Protocol.MSG_START_TASK_REQUEST, SdkWireMessages.encodeInvokeRequest(
            new SdkWireMessages.InvokeRequest("boom", "", new byte[0], Map.of())));
        String taskId = SdkWireMessages.decodeStartTaskResponse(response).taskId;

        SdkWireMessages.TaskEvent event = awaitEvent(client, taskId, "error", 5000);
        assertEquals("handler exploded", event.message);
    }

    @Test
    @DisplayName("local cancel task emits cancelled event and suppresses completion")
    @Timeout(10)
    void localCancelTask() throws Exception {
        CountDownLatch release = new CountDownLatch(1);
        AtomicReference<String> context = new AtomicReference<>();
        CroupierClientImpl client = new CroupierClientImpl(createConfig(), (address, timeout) -> new FakeTransportClient(null));
        client.registerFunction(new FunctionDescriptor("slow", "1.0.0"), (ctx, payload) -> {
            context.set(ctx);
            release.await();
            return "never";
        });

        byte[] response = localRequest(client, Protocol.MSG_START_TASK_REQUEST, SdkWireMessages.encodeInvokeRequest(
            new SdkWireMessages.InvokeRequest("slow", "", "p".getBytes(StandardCharsets.UTF_8), Map.of())));
        String taskId = SdkWireMessages.decodeStartTaskResponse(response).taskId;

        awaitEvent(client, taskId, "started", 5000);

        byte[] cancelBody = localRequest(client, Protocol.MSG_CANCEL_TASK_REQUEST,
            SdkWireMessages.encodeCancelTaskRequest(new SdkWireMessages.CancelTaskRequest(taskId)));
        assertEquals(0, cancelBody.length);

        SdkWireMessages.TaskEvent event = awaitEvent(client, taskId, "cancelled", 5000);
        assertEquals("Task cancelled", event.message);

        release.countDown();
        Thread.sleep(150);
        SdkWireMessages.TaskEvent latest = streamLatest(client, taskId);
        assertEquals("cancelled", latest.type);
    }

    @Test
    @DisplayName("local cancel of unknown task is a no-op")
    @Timeout(5)
    void localCancelUnknownTask() throws Exception {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(), (address, timeout) -> new FakeTransportClient(null));
        byte[] cancelBody = localRequest(client, Protocol.MSG_CANCEL_TASK_REQUEST,
            SdkWireMessages.encodeCancelTaskRequest(new SdkWireMessages.CancelTaskRequest("ghost")));
        assertEquals(0, cancelBody.length);
    }

    @Test
    @DisplayName("local stream of unknown task returns terminal error event")
    @Timeout(5)
    void localStreamUnknownTask() throws Exception {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(), (address, timeout) -> new FakeTransportClient(null));
        byte[] body = localRequest(client, Protocol.MSG_STREAM_TASK_REQUEST,
            SdkWireMessages.encodeTaskStreamRequest(new SdkWireMessages.TaskStreamRequest("ghost")));
        SdkWireMessages.TaskEvent event = SdkWireMessages.decodeTaskEvent(body);
        assertEquals("error", event.type);
        assertEquals("Task not found", event.message);
    }

    @Test
    @DisplayName("invoke passes '{}' context for null metadata and wraps handler failures")
    @Timeout(5)
    void invokeDirectPaths() throws Exception {
        AtomicReference<String> context = new AtomicReference<>();
        CroupierClientImpl client = new CroupierClientImpl(createConfig(), (address, timeout) -> new FakeTransportClient(null));
        client.registerFunction(new FunctionDescriptor("f", "1.0.0"), (ctx, payload) -> {
            context.set(ctx);
            return "ok";
        });

        assertEquals("ok", client.invoke("f", "p", null));
        assertEquals("{}", context.get());

        CroupierException missing = assertThrows(CroupierException.class, () -> client.invoke("nope", "p", Map.of()));
        assertEquals("Function not found: nope", missing.getMessage());

        client.registerFunction(new FunctionDescriptor("bad", "1.0.0"), (ctx, payload) -> {
            throw new IllegalArgumentException("nope-arg");
        });
        CroupierException wrapped = assertThrows(CroupierException.class, () -> client.invoke("bad", "p", Map.of()));
        assertTrue(wrapped.getMessage().contains("Function execution failed"));

        client.registerFunction(new FunctionDescriptor("croupier-fail", "1.0.0"), (ctx, payload) -> {
            throw new CroupierException("direct");
        });
        CroupierException direct = assertThrows(CroupierException.class, () -> client.invoke("croupier-fail", "p", Map.of()));
        assertEquals("direct", direct.getMessage());
    }

    @Test
    @DisplayName("startTask and cancelTask map invoker failures to CroupierException")
    @Timeout(10)
    void taskMethodsMapFailures() throws Exception {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            if (msgType == Protocol.MSG_START_TASK_REQUEST) {
                return SdkWireMessages.encodeStartTaskResponse(new SdkWireMessages.StartTaskResponse("task-9"));
            }
            if (msgType == Protocol.MSG_CANCEL_TASK_REQUEST) {
                throw new InvokerException(InvokerException.ErrorCode.INTERNAL, "cancel rejected");
            }
            throw new InvokerException(InvokerException.ErrorCode.UNAVAILABLE, "unexpected " + msgType);
        });
        CroupierClientImpl client = new CroupierClientImpl(createConfig(), (address, timeout) -> transport);
        client.registerFunction(new FunctionDescriptor("f", "1.0.0"), (ctx, payload) -> "ok");

        assertEquals("task-9", client.startTask("f", "{}"));
        assertEquals("task-9", client.startTask("f", "{}", Map.of("meta", "1")));
        assertNotNull(client.streamTask("task-9"));

        CroupierException cancelFailure = assertThrows(CroupierException.class, () -> client.cancelTask("task-9"));
        assertTrue(cancelFailure.getMessage().contains("Failed to cancel task"));
    }

    @Test
    @DisplayName("startTask wraps invoker failures")
    @Timeout(10)
    void startTaskMapsFailures() throws Exception {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            throw new InvokerException(InvokerException.ErrorCode.UNAVAILABLE, "agent unreachable");
        });
        CroupierClientImpl client = new CroupierClientImpl(createConfig(), (address, timeout) -> transport);
        client.registerFunction(new FunctionDescriptor("f", "1.0.0"), (ctx, payload) -> "ok");

        CroupierException error = assertThrows(CroupierException.class, () -> client.startTask("f", "{}"));
        assertTrue(error.getMessage().contains("Failed to start task"));
        assertInstanceOf(InvokerException.class, error.getCause());
    }

    @Test
    @DisplayName("connect wraps transport failures into CompletionException")
    @Timeout(10)
    void connectWrapsFailures() throws Exception {
        CroupierClientImpl runtimeFailure = new CroupierClientImpl(createConfig(),
            (address, timeout) -> new FakeTransportClient((msgType, data) -> {
                throw new IllegalStateException("dial failed");
            }));
        runtimeFailure.registerFunction(new FunctionDescriptor("f", "1.0.0"), (ctx, payload) -> "ok");
        java.util.concurrent.CompletionException runtime = assertThrows(
            java.util.concurrent.CompletionException.class, () -> runtimeFailure.connect().join());
        assertInstanceOf(CroupierException.class, runtime.getCause());
        assertTrue(runtime.getCause().getMessage().contains("Connection failed"));

        CroupierClientImpl completionFailure = new CroupierClientImpl(createConfig(),
            (address, timeout) -> new FakeTransportClient((msgType, data) -> {
                throw new java.util.concurrent.CompletionException(new CroupierException("wrapped"));
            }));
        completionFailure.registerFunction(new FunctionDescriptor("f", "1.0.0"), (ctx, payload) -> "ok");
        java.util.concurrent.CompletionException completion = assertThrows(
            java.util.concurrent.CompletionException.class, () -> completionFailure.connect().join());
        assertInstanceOf(CroupierException.class, completion.getCause());
        assertEquals("wrapped", completion.getCause().getMessage());
    }

    @Test
    @DisplayName("transport factory parses tcp://host:port and bare host addresses")
    @Timeout(5)
    void transportFactoryParsesAddresses() throws Exception {
        Method factory = CroupierClientImpl.class.getDeclaredMethod("createTransportFactory", ClientConfig.class);
        factory.setAccessible(true);
        java.util.function.BiFunction<String, Integer, TransportClient> created =
            (java.util.function.BiFunction<String, Integer, TransportClient>) factory.invoke(null, createConfig());

        TransportClient withScheme = created.apply("tcp://example.com:7777", 1500);
        assertInstanceOf(TCPTransport.class, withScheme);

        TransportClient bareHost = created.apply("example.com", 1500);
        assertInstanceOf(TCPTransport.class, bareHost);
    }

    @Test
    @DisplayName("getRegisterRequest fills defaults for null tags and version")
    @Timeout(5)
    void registerRequestDefaults() throws Exception {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(), (address, timeout) -> new FakeTransportClient(null));
        FunctionDescriptor descriptor = new FunctionDescriptor("f1", "1.0.0");
        descriptor.setTags(null);
        client.registerFunction(descriptor, (ctx, payload) -> "ok");

        Map<String, Object> request = client.getRegisterRequest();
        assertEquals("svc-1", request.get("serviceId"));
        assertEquals("1.0.0", request.get("version"));
        @SuppressWarnings("unchecked")
        List<Map<String, Object>> functions = (List<Map<String, Object>>) request.get("functions");
        assertEquals(1, functions.size());
        assertEquals(List.of(), functions.get(0).get("tags"));
        assertEquals("f1", functions.get(0).get("operationId"));
    }

    @Test
    @DisplayName("buildManifest skips invalid ids, defaults versions and includes capability/execution")
    @Timeout(5)
    void manifestEdgeCases() throws Exception {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(), (address, timeout) -> new FakeTransportClient(null));

        FunctionDescriptor full = new FunctionDescriptor("full", "2.0.0");
        full.setTags(List.of("a", "b"));
        full.setSummary("sum");
        full.setDescription("desc");
        full.setOperationId("op");
        full.setDeprecated(true);
        full.setInputSchema("in");
        full.setOutputSchema("out");
        full.setResource("res");
        full.setRisk("risk");
        full.setOperation("op-kind");
        full.setCapability("cap");
        full.setExecution("exec");
        full.setPermission("perm");
        client.registerFunction(full, (ctx, payload) -> "ok");

        Map<String, FunctionDescriptor> descriptors = descriptorsOf(client);
        descriptors.put("invalid", new FunctionDescriptor("", null));
        descriptors.put("nullver", new FunctionDescriptor("nullver", null));

        String manifest = new String(client.buildManifest(), StandardCharsets.UTF_8);
        assertTrue(manifest.contains("\"provider\""));
        assertTrue(manifest.contains("cap"));
        assertTrue(manifest.contains("exec"));
        assertTrue(manifest.contains("perm"));
        assertTrue(manifest.contains("\"deprecated\":true"));
        assertTrue(manifest.contains("\"enabled\":true"));
        assertTrue(manifest.contains("\"version\":\"1.0.0\""), "null versions must default to 1.0.0");
        assertFalse(manifest.contains("invalid"));
    }

    @Test
    @DisplayName("getManifestGzipped returns gzipped manifest bytes")
    @Timeout(5)
    void manifestGzipped() throws Exception {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(), (address, timeout) -> new FakeTransportClient(null));
        client.registerFunction(new FunctionDescriptor("f", "1.0.0"), (ctx, payload) -> "ok");

        byte[] gzipped = client.getManifestGzipped();
        byte[] inflated;
        try (GZIPInputStream gzip = new GZIPInputStream(new ByteArrayInputStream(gzipped))) {
            inflated = gzip.readAllBytes();
        }
        assertArrayEquals(client.buildManifest(), inflated);
    }

    @Test
    @DisplayName("getLocalFunctions exposes capability and execution")
    @Timeout(5)
    void localFunctionsExposeCapabilities() throws Exception {
        CroupierClientImpl client = new CroupierClientImpl(createConfig(), (address, timeout) -> new FakeTransportClient(null));
        FunctionDescriptor descriptor = new FunctionDescriptor("f", "1.0.0");
        descriptor.setCapability("cap2");
        descriptor.setExecution("exec2");
        client.registerFunction(descriptor, (ctx, payload) -> "ok");

        List<ProviderFunctionDescriptor> functions = client.getLocalFunctions();
        assertEquals(1, functions.size());
        assertEquals("cap2", functions.get(0).getCapability());
        assertEquals("exec2", functions.get(0).getExecution());
    }

    private SdkWireMessages.TaskEvent streamLatest(CroupierClientImpl client, String taskId) throws Exception {
        byte[] body = localRequest(client, Protocol.MSG_STREAM_TASK_REQUEST,
            SdkWireMessages.encodeTaskStreamRequest(new SdkWireMessages.TaskStreamRequest(taskId)));
        return SdkWireMessages.decodeTaskEvent(body);
    }

    private SdkWireMessages.TaskEvent awaitEvent(CroupierClientImpl client, String taskId, String expectedType, long timeoutMs)
        throws Exception {
        long deadline = System.currentTimeMillis() + timeoutMs;
        SdkWireMessages.TaskEvent latest = null;
        while (System.currentTimeMillis() < deadline) {
            latest = streamLatest(client, taskId);
            if (expectedType.equals(latest.type)) {
                return latest;
            }
            Thread.sleep(20);
        }
        fail("expected event type " + expectedType + " for task " + taskId + ", last: " +
            (latest == null ? null : latest.type + "/" + latest.message));
        return null;
    }
}
