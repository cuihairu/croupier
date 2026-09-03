package io.github.cuihairu.croupier.sdk;

import io.github.cuihairu.croupier.sdk.transport.Protocol;
import io.github.cuihairu.croupier.sdk.transport.TCPTransport;
import io.github.cuihairu.croupier.sdk.transport.TransportAddresses;
import io.github.cuihairu.croupier.sdk.transport.TransportClient;
import io.github.cuihairu.croupier.sdk.invoker.InvokeOptions;
import io.github.cuihairu.croupier.sdk.invoker.Invoker;
import io.github.cuihairu.croupier.sdk.invoker.InvokerConfig;
import io.github.cuihairu.croupier.sdk.invoker.InvokerException;
import io.github.cuihairu.croupier.sdk.invoker.TaskEventInfo;
import io.github.cuihairu.croupier.sdk.invoker.InvokerImpl;
import io.github.cuihairu.croupier.sdk.wire.SdkWireMessages;
import org.reactivestreams.Publisher;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.UUID;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.CompletionException;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.CopyOnWriteArrayList;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.function.BiFunction;
import java.util.stream.Collectors;
import java.util.zip.GZIPOutputStream;

/**
 * Default implementation of CroupierClient.
 *
 * Note: This is a refactored version without gRPC dependencies.
 * Transport layer should be implemented separately.
 */
public class CroupierClientImpl implements CroupierClient {
    private static final Logger logger = LoggerFactory.getLogger(CroupierClientImpl.class);

    private final ClientConfig config;
    private final Map<String, FunctionHandler> handlers = new ConcurrentHashMap<>();
    private final Map<String, FunctionDescriptor> descriptors = new ConcurrentHashMap<>();
    private final Map<String, LocalTaskState> localTasks = new ConcurrentHashMap<>();
    private final BiFunction<String, Integer, TransportClient> transportFactory;

    private final AtomicBoolean connected = new AtomicBoolean(false);
    private final AtomicBoolean serving = new AtomicBoolean(false);
    private final AtomicBoolean reconnecting = new AtomicBoolean(false);
    // drain 状态：收到 ProviderDrainRequest 后置位——拒绝新 Invoke，
    // 在途调用清零后复用 recoverConnection 恢复会话（对齐 C# 参考实现）。
    private final AtomicBoolean draining = new AtomicBoolean(false);
    private final java.util.concurrent.atomic.AtomicLong inflightCalls =
        new java.util.concurrent.atomic.AtomicLong(0);

    private volatile TransportClient transport;
    private String sessionId = "";
    private volatile boolean stopHeartbeat;
    private Thread heartbeatThread;
    private final Invoker invoker;

    public CroupierClientImpl(ClientConfig config) {
        this(config, createTransportFactory(config));
    }

    private static BiFunction<String, Integer, TransportClient> createTransportFactory(ClientConfig config) {
        // TCP transport: parse host:port
        return (address, timeout) -> {
            String[] parts = address.replace("tcp://", "").split(":");
            String host = parts[0];
            int port = parts.length > 1 ? Integer.parseInt(parts[1]) : 19091;
            return new TCPTransport(host, port, timeout);
        };
    }

    CroupierClientImpl(
        ClientConfig config,
        BiFunction<String, Integer, TransportClient> transportFactory
    ) {
        this.config = Objects.requireNonNull(config, "config");
        this.transportFactory = transportFactory;
        validateConfig();
        logger.info("Initialized CroupierClient for game '{}' in '{}' environment",
                   config.getGameId(), config.getEnv());

        // Provider-side helper for Agent TCP task protocol. Public L3 callers
        // must use CroupierSDK.createInvoker(), which returns ServerHttpInvoker.
        InvokerConfig invokerConfig = InvokerConfig.builder()
            .address(config.getAgentAddr())
            .build();
        this.invoker = new InvokerImpl(invokerConfig, transportFactory);
    }

    @Override
    public void registerFunction(FunctionDescriptor descriptor, FunctionHandler handler) throws CroupierException {
        if (connected.get() || serving.get()) {
            throw new CroupierException("Cannot register functions after client has connected");
        }

        validateFunctionDescriptor(descriptor);

        handlers.put(descriptor.getId(), handler);
        descriptors.put(descriptor.getId(), descriptor);

        logger.info("Registered function: {} (version: {})", descriptor.getId(), descriptor.getVersion());
    }

    @Override
    public CompletableFuture<Void> connect() {
        if (connected.get()) {
            return CompletableFuture.completedFuture(null);
        }

        return CompletableFuture.runAsync(() -> {
            TransportClient nextTransport = null;
            try {
                logger.info("Connecting to Croupier Agent: {}", config.getAgentAddr());

                if (handlers.isEmpty()) {
                    throw new CroupierException("Register at least one function before connecting");
                }

                nextTransport = transportFactory.apply(
                    config.getAgentAddr(),
                    config.getTimeoutSeconds() * 1000
                );
                nextTransport.connect();
                SdkWireMessages.ProviderConnectResponse response = providerConnect(nextTransport);
                if (response.sessionId.isEmpty()) {
                    throw new CroupierException("ProviderConnect returned empty session_id");
                }

                if (transport != null) {
                    transport.close();
                }
                transport = nextTransport;
                sessionId = response.sessionId;
                // 首连也必须挂入站 listener（Agent→Provider 调用）——此前仅
                // 重连路径挂载，首连客户端对所有 agent 主动调用无响应。
                attachInboundListener(nextTransport);
                connected.set(true);
                startHeartbeatLoop();

                logger.info("Successfully connected");

                // F/审查发现 #2：控制面 manifest 上传 fire-and-forget——
                // 控制面慢/不可达不得阻塞注册主路径（方法内部已 fail-open）
                Thread capabilitiesThread = new Thread(this::maybeRegisterCapabilities);
                capabilitiesThread.setDaemon(true);
                capabilitiesThread.start();
            } catch (Exception e) {
                connected.set(false);
                sessionId = "";
                if (nextTransport != null) {
                    nextTransport.close();
                }
                logger.error("Connection failed", e);
                throw wrapAsyncFailure("Connection failed", e);
            }
        });
    }

    @Override
    public void serve() throws CroupierException {
        serveAsync().join();
    }

    @Override
    public CompletableFuture<Void> serveAsync() {
        if (!connected.get()) {
            return connect().thenCompose(v -> doServe());
        } else {
            return doServe();
        }
    }

    private CompletableFuture<Void> doServe() {
        return CompletableFuture.runAsync(() -> {
            try {
                serving.set(true);
                logger.info("Croupier client service started");
                logger.info("Registered functions: {}", handlers.size());

                // Keep serving until stopped
                while (serving.get()) {
                    try {
                        Thread.sleep(100);
                    } catch (InterruptedException e) {
                        Thread.currentThread().interrupt();
                        break;
                    }
                }

                serving.set(false);
                logger.info("Service has stopped");
            } catch (Exception e) {
                serving.set(false);
                throw wrapAsyncFailure("Serving failed", e);
            }
        });
    }

    @Override
    public void stop() {
        serving.set(false);
        connected.set(false);
        sessionId = "";
        stopHeartbeatLoop();
        closeTransport();

        logger.info("Stopping Croupier client...");
        logger.info("Client stopped successfully");
    }

    @Override
    public void close() {
        stop();
        handlers.clear();
        descriptors.clear();
        localTasks.clear();

        // Close invoker
        try {
            invoker.close();
        } catch (InvokerException e) {
            logger.warn("Failed to close invoker", e);
        }
    }

    @Override
    public boolean isConnected() {
        return connected.get();
    }

    @Override
    public boolean isServing() {
        return serving.get();
    }

    @Override
    public String getSessionId() {
        return sessionId;
    }

    // ========== Task Management Methods ==========

    @Override
    public String startTask(String functionId, String payload) throws CroupierException {
        return startTask(functionId, payload, Map.of());
    }

    @Override
    public String startTask(String functionId, String payload, Map<String, String> metadata) throws CroupierException {
        try {
            InvokeOptions options = InvokeOptions.builder()
                .headers(metadata != null ? metadata : Map.of())
                .build();
            return invoker.startTask(functionId, payload, options);
        } catch (InvokerException e) {
            throw new CroupierException("Failed to start task: " + e.getMessage(), e);
        }
    }

    @Override
    public Publisher<TaskEventInfo> streamTask(String taskId) {
        return invoker.streamTask(taskId);
    }

    @Override
    public boolean cancelTask(String taskId) throws CroupierException {
        try {
            invoker.cancelTask(taskId);
            return true;
        } catch (InvokerException e) {
            throw new CroupierException("Failed to cancel task: " + e.getMessage(), e);
        }
    }

    /**
     * Invoke a registered function handler directly.
     */
    public String invoke(String functionId, String payload, Map<String, String> metadata) throws CroupierException {
        FunctionHandler handler = handlers.get(functionId);
        if (handler == null) {
            throw new CroupierException("Function not found: " + functionId);
        }

        String context = toJson(metadata != null ? metadata : Map.of());
        try {
            return handler.handle(context, payload);
        } catch (Exception e) {
            if (e instanceof CroupierException) {
                throw (CroupierException) e;
            }
            throw new CroupierException("Function execution failed: " + e.getMessage(), e);
        }
    }

    private void closeTransport() {
        if (transport != null) {
            transport.close();
            transport = null;
        }
    }

    /**
     * F：向控制面（control_addr）上传能力清单。
     * 独立短连接 + best-effort：任何失败仅告警，不影响已完成的注册连接。
     */
    private void maybeRegisterCapabilities() {
        String controlAddr = config.getControlAddr();
        if (controlAddr == null || controlAddr.isBlank()) {
            return;
        }
        TransportClient controlTransport = null;
        try {
            controlTransport = transportFactory.apply(controlAddr, config.getTimeoutSeconds() * 1000);
            controlTransport.connect();
            SdkWireMessages.RegisterCapabilitiesRequest request =
                new SdkWireMessages.RegisterCapabilitiesRequest(
                    new SdkWireMessages.ProviderMeta(
                        config.getServiceId(),
                        config.getServiceVersion(),
                        config.getProviderLang(),
                        config.getProviderSdk()),
                    getManifestGzipped());
            controlTransport.request(
                Protocol.MSG_REGISTER_CAPABILITIES_REQ,
                SdkWireMessages.encodeRegisterCapabilitiesRequest(request));
            logger.info("Capabilities registered to control plane: {}", controlAddr);
        } catch (Exception e) {
            logger.warn("Failed to register capabilities: {}", e.getMessage());
        } finally {
            if (controlTransport != null) {
                controlTransport.close();
            }
        }
    }

    private SdkWireMessages.ProviderConnectResponse providerConnect(TransportClient nextTransport) throws InvokerException {
        return SdkWireMessages.decodeProviderConnectResponse(
            nextTransport.request(
                Protocol.MSG_PROVIDER_CONNECT_REQUEST,
                SdkWireMessages.encodeProviderConnectRequest(buildProviderConnectRequest())
            )
        );
    }

    private SdkWireMessages.ProviderConnectRequest buildProviderConnectRequest() {
        List<SdkWireMessages.ProviderFunctionDescriptor> functions = descriptors.values().stream()
            .map(this::toWireDescriptor)
            .collect(Collectors.toList());
        return new SdkWireMessages.ProviderConnectRequest(
            config.getServiceId(),
            config.getServiceVersion(),
            "",
            functions
        );
    }

    private SdkWireMessages.ProviderFunctionDescriptor toWireDescriptor(FunctionDescriptor descriptor) {
        return new SdkWireMessages.ProviderFunctionDescriptor(
            descriptor.getId(),
            descriptor.getVersion(),
            descriptor.getTags(),
            descriptor.getSummary(),
            descriptor.getDescription(),
            descriptor.getOperationId(),
            descriptor.isDeprecated(),
            descriptor.getInputSchema(),
            descriptor.getOutputSchema(),
            descriptor.getResource(),
            descriptor.getOperation(),
            descriptor.getCapability(),
            descriptor.getExecution(),
            descriptor.isApprovalRequired(),
            descriptor.getApprovalPolicyKey(),
            descriptor.getRisk(),
            descriptor.isEnabled(),
            descriptor.getPermission()
        );
    }

    private void startHeartbeatLoop() {
        stopHeartbeatLoop();
        stopHeartbeat = false;
        heartbeatThread = new Thread(() -> {
            long intervalMillis = Math.max(config.getHeartbeatInterval(), 1) * 1000L;
            int consecutiveFailures = 0;
            final int maxFailures = 2;
            while (!stopHeartbeat) {
                try {
                    Thread.sleep(intervalMillis);
                    if (!stopHeartbeat && connected.get() && transport != null && !sessionId.isEmpty()) {
                        sendHeartbeat();
                        consecutiveFailures = 0;
                    }
                } catch (InterruptedException e) {
                    Thread.currentThread().interrupt();
                    break;
                } catch (Exception e) {
                    consecutiveFailures++;
                    logger.warn("Heartbeat failed ({}/{}): {}", consecutiveFailures, maxFailures, e.getMessage());
                    if (consecutiveFailures >= maxFailures && serving.get()) {
                        logger.error("Heartbeat failed {} times, triggering reconnect", maxFailures);
                        connected.set(false);
                        recoverConnection();
                        consecutiveFailures = 0;
                    }
                }
            }
        }, "croupier-java-client-heartbeat");
        heartbeatThread.setDaemon(true);
        heartbeatThread.start();
    }

    private void stopHeartbeatLoop() {
        stopHeartbeat = true;
        if (heartbeatThread != null) {
            heartbeatThread.interrupt();
            try {
                heartbeatThread.join(1000L);
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
            } finally {
                heartbeatThread = null;
            }
        }
    }

    private void sendHeartbeat() throws InvokerException {
        transport.request(
            Protocol.MSG_PROVIDER_HEARTBEAT_REQUEST,
            SdkWireMessages.encodeHeartbeatRequest(new SdkWireMessages.HeartbeatRequest(
                config.getServiceId(),
                sessionId
            ))
        );
    }

    /**
     * Attempts to reconnect to the agent with exponential backoff.
     * Called when heartbeat fails or the connection is lost.
     * Uses the ReconnectConfig from ClientConfig for backoff parameters.
     */
    private void recoverConnection() {
        // Ensure only one recovery attempt runs at a time
        if (!reconnecting.compareAndSet(false, true)) {
            return;
        }

        Thread recoveryThread = new Thread(() -> {
            try {
                stopHeartbeatLoop();
                closeTransport();
                sessionId = "";

                ReconnectConfig rc = config.getReconnect();
                if (rc == null || !rc.isEnabled()) {
                    logger.warn("Reconnection is disabled; service will remain disconnected");
                    return;
                }

                long delayMs = rc.getInitialDelayMs();
                long maxDelayMs = rc.getMaxDelayMs();
                int attempt = 0;

                while (serving.get() && !Thread.currentThread().isInterrupted()) {
                    attempt++;
                    if (rc.getMaxAttempts() > 0 && attempt > rc.getMaxAttempts()) {
                        logger.error("Max reconnection attempts ({}) exceeded", rc.getMaxAttempts());
                        return;
                    }

                    logger.info("Reconnection attempt {}...", attempt);
                    try {
                        reconnectOnce();
                        connected.set(true);
                        startHeartbeatLoop();
                        logger.info("Reconnected successfully after {} attempts", attempt);
                        return;
                    } catch (Exception e) {
                        logger.warn("Reconnection attempt {} failed: {}", attempt, e.getMessage());
                    }

                    // Backoff with jitter
                    long jitter = (long) (delayMs * rc.getJitterFactor());
                    long actualDelay = delayMs + (long) (Math.random() * (2 * jitter + 1)) - jitter;
                    try {
                        Thread.sleep(Math.max(actualDelay, 1));
                    } catch (InterruptedException e) {
                        Thread.currentThread().interrupt();
                        return;
                    }

                    delayMs = (long) (delayMs * rc.getBackoffMultiplier());
                    if (delayMs > maxDelayMs) {
                        delayMs = maxDelayMs;
                    }
                }
            } finally {
                reconnecting.set(false);
            }
        }, "croupier-java-client-reconnect");
        recoveryThread.setDaemon(true);
        recoveryThread.start();
    }

    /**
     * Performs a single reconnection attempt: dial + provider connect.
     * Throws on any failure so the caller can backoff and retry.
     */
    private void reconnectOnce() throws Exception {
        if (handlers.isEmpty()) {
            throw new CroupierException("No functions registered");
        }

        TransportClient nextTransport = transportFactory.apply(
            config.getAgentAddr(),
            config.getTimeoutSeconds() * 1000
        );
        try {
            nextTransport.connect();
            SdkWireMessages.ProviderConnectResponse response = providerConnect(nextTransport);
            if (response.sessionId.isEmpty()) {
                nextTransport.close();
                throw new CroupierException("ProviderConnect returned empty session_id");
            }

            TransportClient old = transport;
            transport = nextTransport;
            sessionId = response.sessionId;
            attachInboundListener(nextTransport);
            if (old != null) {
                old.close();
            }

            logger.info("Reconnected and re-registered service {}", config.getServiceId());
        } catch (Exception e) {
            nextTransport.close();
            throw e;
        }
    }

    /** 把 transport 的入站请求路由到本地 handler（Agent -> Provider 调用）。 */
    private void attachInboundListener(TransportClient client) {
        if (client instanceof io.github.cuihairu.croupier.sdk.transport.TCPTransport tcp) {
            tcp.setInboundListener((msgId, requestId, body) -> handleLocalRequest(msgId, requestId, body));
        }
    }

    private byte[] handleLocalRequest(int msgType, int requestId, byte[] body) throws Exception {
        return switch (msgType) {
            case Protocol.MSG_PROVIDER_DRAIN_REQUEST -> handleDrainRequest(body);
            case Protocol.MSG_INVOKE_REQUEST -> handleInvokeRequest(body);
            case Protocol.MSG_START_TASK_REQUEST -> handleStartTaskRequest(body);
            case Protocol.MSG_STREAM_TASK_REQUEST -> handleStreamTaskRequest(body);
            case Protocol.MSG_CANCEL_TASK_REQUEST -> handleCancelTaskRequest(body);
            default -> throw new CroupierException("Unsupported local request type: " + requestId);
        };
    }

    /**
     * 处理 Agent 的优雅下线请求：置位 drain 状态（拒绝新 Invoke）、异步等待
     * 在途调用清零后复用 recoverConnection 恢复会话；协议规定立即回空
     * ProviderDrainResponse 确认。幂等：重复 drain 只回确认。
     */
    private byte[] handleDrainRequest(byte[] body) {
        if (draining.compareAndSet(false, true)) {
            SdkWireMessages.ProviderDrainRequest request;
            try {
                request = SdkWireMessages.decodeProviderDrainRequest(body);
                logger.info("Drain requested (session={}, reason={}, retryAfterMs={})",
                    request.sessionId, request.reason, request.retryAfterMs);
            } catch (Exception e) {
                logger.warn("Drain requested (unparsable body)");
                request = null;
            }
            Thread drainThread = new Thread(this::drainAndRecover, "croupier-java-client-drain");
            drainThread.setDaemon(true);
            drainThread.start();
        }
        return SdkWireMessages.encodeProviderDrainResponse();
    }

    private void drainAndRecover() {
        try {
            // 等待在途调用完成（最多 30 秒，超时后仅记录并继续）。
            long deadline = System.currentTimeMillis() + 30_000L;
            while (inflightCalls.get() > 0 && System.currentTimeMillis() < deadline) {
                Thread.sleep(100);
            }
            if (inflightCalls.get() > 0) {
                logger.warn("Drain timeout with {} in-flight call(s) still running", inflightCalls.get());
            }
            if (config.getReconnect() == null || config.getReconnect().isEnabled()) {
                logger.info("Drain complete, reconnecting provider session");
                recoverConnection();
            } else {
                logger.info("Drain complete, auto-reconnect disabled — closing session");
                closeTransport();
            }
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
        } catch (Exception e) {
            logger.error("Drain recovery failed: " + e.getMessage());
        } finally {
            draining.set(false);
        }
    }

    private byte[] handleInvokeRequest(byte[] body) throws Exception {
        // drain 期间拒绝新调用，等待 Agent 停止投递。
        if (draining.get()) {
            return SdkWireMessages.encodeInvokeResponse(
                new SdkWireMessages.InvokeResponse(
                    "{\"error\":\"provider is draining\"}".getBytes(StandardCharsets.UTF_8)));
        }
        inflightCalls.incrementAndGet();
        try {
            return invokeInbound(body);
        } finally {
            inflightCalls.decrementAndGet();
        }
    }

    private byte[] invokeInbound(byte[] body) throws Exception {
        SdkWireMessages.InvokeRequest request = SdkWireMessages.decodeInvokeRequest(body);
        String payload = new String(request.payload, StandardCharsets.UTF_8);

        // Provider 侧入站校验（可选）：按函数声明的 input schema 校验 payload，
        // 失败回错误响应，handler 不被调用（服务端仍是权威校验方）。
        if (config.isValidateInputPayloads()) {
            String validationError = validateInboundPayload(request.functionId, payload);
            if (validationError != null) {
                return SdkWireMessages.encodeInvokeResponse(
                    new SdkWireMessages.InvokeResponse(
                        ("{\"error\":" + io.github.cuihairu.croupier.sdk.invoker.Json.stringify(validationError) + "}")
                            .getBytes(StandardCharsets.UTF_8)));
            }
        }

        String result = invoke(request.functionId, payload, request.metadata);
        return SdkWireMessages.encodeInvokeResponse(
            new SdkWireMessages.InvokeResponse(result.getBytes(StandardCharsets.UTF_8))
        );
    }

    /**
     * Provider 侧入站校验（F：与 Go/Python/JS/C# 语义对齐）：按函数声明的
     * input schema 校验 payload。开关关闭/未注册/schema 缺失时跳过（服务端
     * 仍是权威校验方）；失败返回错误消息，通过则返回 null。
     */
    private String validateInboundPayload(String functionId, String payload) {
        if (!config.isValidateInputPayloads()) {
            return null;
        }
        FunctionDescriptor descriptor = descriptors.get(functionId);
        if (descriptor == null || descriptor.getInputSchema() == null
                || descriptor.getInputSchema().isBlank()) {
            return null;
        }
        List<String> errors = io.github.cuihairu.croupier.sdk.invoker.JsonSchemaValidator.validate(
            payload, descriptor.getInputSchema());
        return errors.isEmpty() ? null : "payload validation failed: " + String.join("; ", errors);
    }

    private byte[] handleStartTaskRequest(byte[] body) throws Exception {
        SdkWireMessages.InvokeRequest request = SdkWireMessages.decodeInvokeRequest(body);
        String functionId = request.functionId;
        FunctionHandler handler = handlers.get(functionId);
        if (handler == null) {
            throw new CroupierException("Function not found: " + functionId);
        }

        // F：入站校验（同 invoke）——失败抛错，任务不启动
        if (config.isValidateInputPayloads()) {
            String validationError = validateInboundPayload(
                functionId, new String(request.payload, StandardCharsets.UTF_8));
            if (validationError != null) {
                throw new CroupierException(validationError);
            }
        }

        String payload = new String(request.payload, StandardCharsets.UTF_8);
        String taskId = functionId + "-" + UUID.randomUUID().toString().substring(0, 12);
        LocalTaskState taskState = new LocalTaskState(taskId);
        localTasks.put(taskId, taskState);
        appendLocalTaskEvent(taskState, TaskEventInfo.builder()
            .type("started")
            .taskId(taskId)
            .message("Task started")
            .progress(0)
            .done(false)
            .build());

        String context = toJson(request.metadata);
        Thread worker = new Thread(() -> {
            try {
                String result = handler.handle(context, payload);
                if (taskState.cancelled.get()) {
                    return;
                }
                appendLocalTaskEvent(taskState, TaskEventInfo.builder()
                    .type("completed")
                    .taskId(taskId)
                    .message("Task completed")
                    .progress(100)
                    .payload(result)
                    .done(true)
                    .build());
            } catch (Exception e) {
                if (taskState.cancelled.get()) {
                    return;
                }
                appendLocalTaskEvent(taskState, TaskEventInfo.builder()
                    .type("error")
                    .taskId(taskId)
                    .message(e.getMessage())
                    .error(e.getMessage())
                    .done(true)
                    .build());
            }
        }, "croupier-java-local-task-" + taskId);
        worker.setDaemon(true);
        taskState.worker = worker;
        worker.start();

        return SdkWireMessages.encodeStartTaskResponse(new SdkWireMessages.StartTaskResponse(taskId));
    }

    private byte[] handleStreamTaskRequest(byte[] body) {
        SdkWireMessages.TaskStreamRequest request = SdkWireMessages.decodeTaskStreamRequest(body);
        LocalTaskState state = localTasks.get(request.taskId);
        TaskEventInfo event = state != null ? state.latest() : TaskEventInfo.builder()
            .type("error")
            .taskId(request.taskId)
            .message("Task not found")
            .error("Task not found")
            .done(true)
            .build();

        return SdkWireMessages.encodeTaskEvent(new SdkWireMessages.TaskEvent(
            event.getType(),
            event.getError() != null ? event.getError() : defaultValue(event.getMessage(), ""),
            event.getProgress() != null ? event.getProgress() : 0,
            event.getPayload() != null ? event.getPayload().getBytes(StandardCharsets.UTF_8) : new byte[0]
        ));
    }

    private byte[] handleCancelTaskRequest(byte[] body) {
        SdkWireMessages.CancelTaskRequest request = SdkWireMessages.decodeCancelTaskRequest(body);
        LocalTaskState state = localTasks.get(request.taskId);
        if (state != null && !state.done.get()) {
            state.cancelled.set(true);
            appendLocalTaskEvent(state, TaskEventInfo.builder()
                .type("cancelled")
                .taskId(request.taskId)
                .message("Task cancelled")
                .error("Task cancelled")
                .done(true)
                .build());
        }
        return new byte[0];
    }

    private void appendLocalTaskEvent(LocalTaskState state, TaskEventInfo event) {
        state.events.add(event);
        if (event.isDone()) {
            state.done.set(true);
        }
    }

    private void validateConfig() {
        if (config.getGameId() == null || config.getGameId().trim().isEmpty()) {
            logger.warn("Warning: gameId is required for proper backend separation");
        }

        String env = config.getEnv();
        if (!"development".equals(env) && !"staging".equals(env) && !"production".equals(env)) {
            logger.warn("Warning: Unknown environment '{}'. Valid values: development, staging, production", env);
        }
    }

    private void validateFunctionDescriptor(FunctionDescriptor descriptor) throws CroupierException {
        if (descriptor.getId() == null || descriptor.getId().trim().isEmpty()) {
            throw new CroupierException("Function ID cannot be empty");
        }
        if (descriptor.getVersion() == null || descriptor.getVersion().trim().isEmpty()) {
            throw new CroupierException("Function version cannot be empty");
        }
    }

    private CompletionException wrapAsyncFailure(String message, Exception error) {
        if (error instanceof CompletionException completionException) {
            return completionException;
        }
        if (error instanceof CroupierException croupierException) {
            return new CompletionException(croupierException);
        }
        return new CompletionException(new CroupierException(message, error));
    }

    /**
     * Get local function descriptors for registration.
     */
    public List<ProviderFunctionDescriptor> getLocalFunctions() {
        return descriptors.values().stream()
                .map(desc -> new ProviderFunctionDescriptor(
                    desc.getId(),
                    desc.getVersion(),
                    desc.getCapability(),
                    desc.getExecution()
                ))
                .collect(Collectors.toList());
    }

    /**
     * Build a registration request for the agent.
     */
    public Map<String, Object> getRegisterRequest() {
        List<Map<String, Object>> functions = descriptors.values().stream()
            .map(this::toRegisterFunctionMap)
            .collect(Collectors.toList());
        Map<String, Object> request = new LinkedHashMap<>();
        request.put("serviceId", config.getServiceId());
        request.put("version", config.getServiceVersion());
        request.put("rpcAddr", "");
        request.put("functions", functions);
        return request;
    }

    private Map<String, Object> toRegisterFunctionMap(FunctionDescriptor descriptor) {
        Map<String, Object> function = new LinkedHashMap<>();
        function.put("id", defaultValue(descriptor.getId(), ""));
        function.put("version", defaultVersion(descriptor.getVersion()));
        function.put("tags", descriptor.getTags() == null ? List.of() : List.copyOf(descriptor.getTags()));
        function.put("summary", defaultValue(descriptor.getSummary(), ""));
        function.put("description", defaultValue(descriptor.getDescription(), ""));
        function.put("operationId", defaultValue(descriptor.getOperationId(), descriptor.getId()));
        function.put("deprecated", descriptor.isDeprecated());
        function.put("inputSchema", defaultValue(descriptor.getInputSchema(), ""));
        function.put("outputSchema", defaultValue(descriptor.getOutputSchema(), ""));
        function.put("resource", defaultValue(descriptor.getResource(), ""));
        function.put("operation", defaultValue(descriptor.getOperation(), ""));
        function.put("capability", defaultValue(descriptor.getCapability(), ""));
        function.put("execution", defaultValue(descriptor.getExecution(), ""));
        function.put("risk", defaultValue(descriptor.getRisk(), ""));
        function.put("enabled", descriptor.isEnabled());
        function.put("permission", defaultValue(descriptor.getPermission(), ""));
        return function;
    }

    /**
     * Build provider manifest JSON.
     */
    public byte[] buildManifest() {
        StringBuilder builder = new StringBuilder();
        builder.append("{\"provider\":{");
        builder.append("\"id\":\"").append(escapeJson(defaultValue(config.getServiceId(), "java-service"))).append("\",");
        builder.append("\"version\":\"").append(escapeJson(defaultVersion(config.getServiceVersion()))).append("\",");
        builder.append("\"lang\":\"").append(escapeJson(defaultValue(config.getProviderLang(), "java"))).append("\",");
        builder.append("\"sdk\":\"").append(escapeJson(defaultValue(config.getProviderSdk(), "croupier-java-sdk"))).append("\"}");

        List<FunctionDescriptor> snapshot = descriptors.values().stream().collect(Collectors.toList());
        StringBuilder functionsBuilder = new StringBuilder();
        boolean first = true;
        for (FunctionDescriptor descriptor : snapshot) {
            if (descriptor == null || isNullOrEmpty(descriptor.getId())) {
                continue;
            }
            if (first) {
                functionsBuilder.append("[");
                first = false;
            } else {
                functionsBuilder.append(",");
            }
            functionsBuilder.append("{");
            functionsBuilder.append("\"id\":\"").append(escapeJson(descriptor.getId())).append("\"");
            functionsBuilder.append(",\"version\":\"").append(escapeJson(defaultVersion(descriptor.getVersion()))).append("\"");
            if (descriptor.getTags() != null && !descriptor.getTags().isEmpty()) {
                functionsBuilder.append(",\"tags\":[");
                for (int i = 0; i < descriptor.getTags().size(); i++) {
                    if (i > 0) {
                        functionsBuilder.append(",");
                    }
                    functionsBuilder.append("\"").append(escapeJson(descriptor.getTags().get(i))).append("\"");
                }
                functionsBuilder.append("]");
            }
            if (!isNullOrEmpty(descriptor.getSummary())) {
                functionsBuilder.append(",\"summary\":\"").append(escapeJson(descriptor.getSummary())).append("\"");
            }
            if (!isNullOrEmpty(descriptor.getDescription())) {
                functionsBuilder.append(",\"description\":\"").append(escapeJson(descriptor.getDescription())).append("\"");
            }
            if (!isNullOrEmpty(descriptor.getOperationId())) {
                functionsBuilder.append(",\"operationId\":\"").append(escapeJson(descriptor.getOperationId())).append("\"");
            }
            if (descriptor.isDeprecated()) {
                functionsBuilder.append(",\"deprecated\":true");
            }
            if (!isNullOrEmpty(descriptor.getInputSchema())) {
                functionsBuilder.append(",\"inputSchema\":\"").append(escapeJson(descriptor.getInputSchema())).append("\"");
            }
            if (!isNullOrEmpty(descriptor.getOutputSchema())) {
                functionsBuilder.append(",\"outputSchema\":\"").append(escapeJson(descriptor.getOutputSchema())).append("\"");
            }
            if (!isNullOrEmpty(descriptor.getResource())) {
                functionsBuilder.append(",\"resource\":\"").append(escapeJson(descriptor.getResource())).append("\"");
            }
            if (!isNullOrEmpty(descriptor.getRisk())) {
                functionsBuilder.append(",\"risk\":\"").append(escapeJson(descriptor.getRisk())).append("\"");
            }
            if (!isNullOrEmpty(descriptor.getOperation())) {
                functionsBuilder.append(",\"operation\":\"").append(escapeJson(descriptor.getOperation())).append("\"");
            }
            if (!isNullOrEmpty(descriptor.getCapability())) {
                functionsBuilder.append(",\"capability\":\"").append(escapeJson(descriptor.getCapability())).append("\"");
            }
            if (!isNullOrEmpty(descriptor.getExecution())) {
                functionsBuilder.append(",\"execution\":\"").append(escapeJson(descriptor.getExecution())).append("\"");
            }
            if (!isNullOrEmpty(descriptor.getPermission())) {
                functionsBuilder.append(",\"permission\":\"").append(escapeJson(descriptor.getPermission())).append("\"");
            }
            if (descriptor.isEnabled()) {
                functionsBuilder.append(",\"enabled\":true");
            }
            functionsBuilder.append("}");
        }
        if (!first) {
            functionsBuilder.append("]");
            builder.append(",\"functions\":").append(functionsBuilder);
        }

        builder.append("}");
        return builder.toString().getBytes(StandardCharsets.UTF_8);
    }

    /**
     * Get gzipped manifest.
     */
    public byte[] getManifestGzipped() throws IOException {
        return gzip(buildManifest());
    }

    private String toJson(Map<String, String> map) {
        StringBuilder sb = new StringBuilder("{");
        boolean first = true;
        for (Map.Entry<String, String> entry : map.entrySet()) {
            if (!first) sb.append(",");
            sb.append("\"").append(escapeJson(entry.getKey())).append("\":");
            sb.append("\"").append(escapeJson(entry.getValue())).append("\"");
            first = false;
        }
        sb.append("}");
        return sb.toString();
    }

    private String escapeJson(String value) {
        if (value == null) {
            return "";
        }
        StringBuilder out = new StringBuilder(value.length() + 16);
        for (int i = 0; i < value.length(); i++) {
            char ch = value.charAt(i);
            switch (ch) {
                case '"': out.append("\\\""); break;
                case '\\': out.append("\\\\"); break;
                case '\b': out.append("\\b"); break;
                case '\f': out.append("\\f"); break;
                case '\n': out.append("\\n"); break;
                case '\r': out.append("\\r"); break;
                case '\t': out.append("\\t"); break;
                default:
                    if (ch < 0x20) {
                        out.append(String.format("\\u%04x", (int) ch));
                    } else {
                        out.append(ch);
                    }
            }
        }
        return out.toString();
    }

    private String defaultValue(String value, String fallback) {
        return isNullOrEmpty(value) ? fallback : value;
    }

    private String defaultVersion(String version) {
        return isNullOrEmpty(version) ? "1.0.0" : version;
    }

    private byte[] gzip(byte[] payload) throws IOException {
        ByteArrayOutputStream output = new ByteArrayOutputStream();
        try (GZIPOutputStream gzip = new GZIPOutputStream(output)) {
            gzip.write(payload);
        }
        return output.toByteArray();
    }

    private boolean isNullOrEmpty(String value) {
        return value == null || value.trim().isEmpty();
    }

    private static final class LocalTaskState {
        private final String taskId;
        private final CopyOnWriteArrayList<TaskEventInfo> events = new CopyOnWriteArrayList<>();
        private final AtomicBoolean done = new AtomicBoolean(false);
        private final AtomicBoolean cancelled = new AtomicBoolean(false);
        private volatile Thread worker;

        private LocalTaskState(String taskId) {
            this.taskId = taskId;
        }

        private TaskEventInfo latest() {
            if (events.isEmpty()) {
                return TaskEventInfo.builder()
                    .type("started")
                    .taskId(taskId)
                    .message("Task started")
                    .progress(0)
                    .done(false)
                    .build();
            }
            return events.get(events.size() - 1);
        }
    }
}
