package io.github.cuihairu.croupier.sdk.invoker;

import io.github.cuihairu.croupier.sdk.transport.Protocol;
import io.github.cuihairu.croupier.sdk.transport.TCPTransport;
import io.github.cuihairu.croupier.sdk.transport.TransportClient;
import io.github.cuihairu.croupier.sdk.wire.SdkWireMessages;
import org.reactivestreams.Publisher;
import org.reactivestreams.Subscriber;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.nio.charset.StandardCharsets;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.CopyOnWriteArrayList;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicLong;
import java.util.function.BiFunction;

import static io.github.cuihairu.croupier.sdk.invoker.InvokerException.ErrorCode;

/**
 * Legacy Provider-TCP task helper used internally by {@code CroupierClientImpl}.
 *
 * <p>It is not the public L3 caller: {@link ServerHttpInvoker} is returned by
 * {@code CroupierSDK.createInvoker} and exclusively uses Server HTTP.</p>
 */
public class InvokerImpl implements Invoker {

    private static final Logger logger = LoggerFactory.getLogger(InvokerImpl.class);

    private final InvokerConfig config;
    private final Map<String, Map<String, Object>> schemas;
    private final Map<String, TaskState> activeTasks;
    private final BiFunction<String, Integer, TransportClient> transportFactory;
    private volatile TransportClient transport;
    private volatile boolean connected;

    public InvokerImpl(InvokerConfig config) {
        this(config, createTransportFactory(config));
    }

    private static BiFunction<String, Integer, TransportClient> createTransportFactory(InvokerConfig config) {
        // TCP transport: parse host:port
        return (address, timeout) -> {
            String[] parts = address.replace("tcp://", "").split(":");
            String host = parts[0];
            int port = parts.length > 1 ? Integer.parseInt(parts[1]) : 19090;
            return new TCPTransport(host, port, timeout);
        };
    }

    public InvokerImpl(InvokerConfig config, BiFunction<String, Integer, TransportClient> transportFactory) {
        this.config = config;
        this.schemas = new ConcurrentHashMap<>();
        this.activeTasks = new ConcurrentHashMap<>();
        this.transportFactory = transportFactory;
        this.connected = false;
    }

    @Override
    public void connect() throws InvokerException {
        if (connected) {
            return;
        }

        try {
            logger.info("Connecting to server/agent at: {}", config.getAddress());
            TransportClient nextTransport = transportFactory.apply(
                config.getAddress(),
                config.getTimeout()
            );
            nextTransport.connect();
            if (transport != null) {
                transport.close();
            }
            transport = nextTransport;
            connected = true;
            logger.info("Connected to: {}", config.getAddress());
        } catch (Exception e) {
            logger.error("Connection failed", e);
            throw new InvokerException(ErrorCode.CONNECTION_FAILED, "Connection failed: " + e.getMessage(), e);
        }
    }

    @Override
    public String invoke(String functionId, String payload) throws InvokerException {
        return invoke(functionId, payload, InvokeOptions.create());
    }

    @Override
    public String invoke(String functionId, String payload, InvokeOptions options) throws InvokerException {
        ensureConnected();
        InvokeOptions effectiveOptions = options != null ? options : InvokeOptions.create();
        return withRetry("Invoke", effectiveOptions.getRetry(), () -> invokeInternal(functionId, payload, effectiveOptions));
    }

    @Override
    public String startTask(String functionId, String payload) throws InvokerException {
        return startTask(functionId, payload, InvokeOptions.create());
    }

    @Override
    public String startTask(String functionId, String payload, InvokeOptions options) throws InvokerException {
        ensureConnected();
        InvokeOptions effectiveOptions = options != null ? options : InvokeOptions.create();
        return withRetry("StartTask", effectiveOptions.getRetry(), () -> startTaskInternal(functionId, payload, effectiveOptions));
    }

    @Override
    public Publisher<TaskEventInfo> streamTask(String taskId) {
        logger.info("Streaming events for task: {}", taskId);
        return new TaskEventPublisher(taskId);
    }

    @Override
    public void cancelTask(String taskId) throws InvokerException {
        TaskState taskState = activeTasks.get(taskId);

        if (taskState == null) {
            throw new InvokerException(ErrorCode.NOT_FOUND,
                "Task not found: " + taskId);
        }

        if (taskState.isDone()) {
            throw new InvokerException(ErrorCode.FAILED_PRECONDITION,
                "Task already finished: " + taskId + " (status: " + taskState.getStatus() + ")");
        }

        try {
            logger.info("Cancelling task: {}", taskId);
            requireTransport().request(
                Protocol.MSG_CANCEL_TASK_REQUEST,
                SdkWireMessages.encodeCancelTaskRequest(new SdkWireMessages.CancelTaskRequest(taskId))
            );
            publishTaskEvent(taskId, TaskEventInfo.builder()
                .type("cancelled")
                .taskId(taskId)
                .message("Task cancelled")
                .done(true)
                .build());
            logger.info("Task cancelled: {}", taskId);
        } catch (Exception e) {
            if (e instanceof InvokerException) {
                throw (InvokerException) e;
            }
            throw new InvokerException(ErrorCode.INTERNAL, "CancelTask failed: " + e.getMessage(), e);
        }
    }

    @Override
    public void setSchema(String functionId, Map<String, Object> schema) {
        schemas.put(functionId, schema);
        logger.debug("Set schema for function: {}", functionId);
    }

    @Override
    public void close() throws InvokerException {
        if (transport != null) {
            transport.close();
            transport = null;
        }
        for (TaskState state : activeTasks.values()) {
            state.stopPolling();
        }
        connected = false;
        schemas.clear();
        activeTasks.clear();
        logger.info("Invoker closed");
    }

    @Override
    public boolean isConnected() {
        return connected;
    }

    /**
     * Gets the number of active tasks.
     *
     * @return the count of active tasks
     */
    public int getActiveTaskCount() {
        return activeTasks.size();
    }

    /**
     * Checks if a task exists.
     *
     * @param taskId the task ID to check
     * @return true if the task exists, false otherwise
     */
    public boolean hasTask(String taskId) {
        return activeTasks.containsKey(taskId);
    }

    /**
     * Gets the status of a task.
     *
     * @param taskId the task ID
     * @return the task status, or null if not found
     */
    public TaskStatus getTaskStatus(String taskId) {
        TaskState state = activeTasks.get(taskId);
        return state != null ? state.getStatus() : null;
    }

    // Private helper methods

    private void ensureConnected() throws InvokerException {
        if (!connected) {
            connect();
        }
    }

    private String invokeInternal(String functionId, String payload, InvokeOptions options) throws InvokerException {
        try {
            logger.debug("Invoking function: {} with payload: {}", functionId, payload);
            SdkWireMessages.InvokeRequest request = new SdkWireMessages.InvokeRequest(
                functionId,
                options.getIdempotencyKey(),
                (payload == null ? "" : payload).getBytes(StandardCharsets.UTF_8),
                options.getHeaders()
            );
            byte[] responseBody = requireTransport().request(
                Protocol.MSG_INVOKE_REQUEST,
                SdkWireMessages.encodeInvokeRequest(request)
            );
            return SdkWireMessages.decodeInvokeResponse(responseBody).payloadUtf8();
        } catch (InvokerException e) {
            throw e;
        } catch (Exception e) {
            throw new InvokerException(ErrorCode.INTERNAL, "Invoke failed: " + e.getMessage(), e);
        }
    }

    private String startTaskInternal(String functionId, String payload, InvokeOptions options) throws InvokerException {
        try {
            logger.debug("Starting task for function: {}", functionId);
            SdkWireMessages.InvokeRequest request = new SdkWireMessages.InvokeRequest(
                functionId,
                options.getIdempotencyKey(),
                (payload == null ? "" : payload).getBytes(StandardCharsets.UTF_8),
                options.getHeaders()
            );
            byte[] responseBody = requireTransport().request(
                Protocol.MSG_START_TASK_REQUEST,
                SdkWireMessages.encodeInvokeRequest(request)
            );
            String taskId = SdkWireMessages.decodeStartTaskResponse(responseBody).taskId;
            if (taskId.isEmpty()) {
                throw new InvokerException(ErrorCode.INTERNAL, "StartTask response did not include task ID");
            }

            TaskState taskState = new TaskState(taskId, functionId, payload == null ? "" : payload, options);
            activeTasks.put(taskId, taskState);
            publishTaskEvent(taskId, TaskEventInfo.builder()
                .type("started")
                .taskId(taskId)
                .message("Task started")
                .done(false)
                .build());
            return taskId;
        } catch (InvokerException e) {
            throw e;
        } catch (Exception e) {
            throw new InvokerException(ErrorCode.INTERNAL, "StartTask failed: " + e.getMessage(), e);
        }
    }

    private <T> T withRetry(String operation, RetryConfig overrideRetry, CheckedSupplier<T> supplier) throws InvokerException {
        RetryConfig retryConfig = overrideRetry != null ? overrideRetry : config.getRetry();
        if (retryConfig == null || !retryConfig.isEnabled()) {
            return supplier.get();
        }

        InvokerException lastException = null;
        int maxAttempts = Math.max(retryConfig.getMaxAttempts(), 1);
        for (int attempt = 0; attempt < maxAttempts; attempt++) {
            try {
                return supplier.get();
            } catch (InvokerException e) {
                lastException = e;
                if (attempt >= maxAttempts - 1 || !isRetryable(e, retryConfig)) {
                    throw e;
                }
                try {
                    Thread.sleep(calculateRetryDelayMillis(attempt, retryConfig));
                } catch (InterruptedException interrupted) {
                    Thread.currentThread().interrupt();
                    throw new InvokerException(ErrorCode.CANCELLED, operation + " interrupted", interrupted);
                }
            }
        }

        throw lastException != null ? lastException : new InvokerException(ErrorCode.UNKNOWN, operation + " failed");
    }

    private boolean isRetryable(InvokerException exception, RetryConfig retryConfig) {
        return retryConfig.getRetryableStatusCodes().contains(toStatusCode(exception.getErrorCode()));
    }

    private int toStatusCode(ErrorCode errorCode) {
        return switch (errorCode) {
            case CANCELLED -> 1;
            case UNKNOWN, CONNECTION_FAILED, CONNECTION_TIMEOUT -> 2;
            case INVALID_ARGUMENT -> 3;
            case TIMEOUT -> 4;
            case NOT_FOUND -> 5;
            case ALREADY_EXISTS -> 6;
            case PERMISSION_DENIED -> 7;
            case RESOURCE_EXHAUSTED -> 8;
            case FAILED_PRECONDITION -> 9;
            case ABORTED -> 10;
            case OUT_OF_RANGE -> 11;
            case UNIMPLEMENTED -> 12;
            case INTERNAL -> 13;
            case UNAVAILABLE -> 14;
            case DATA_LOSS -> 15;
            case UNAUTHENTICATED -> 16;
        };
    }

    private long calculateRetryDelayMillis(int attempt, RetryConfig retryConfig) {
        double delay = retryConfig.getInitialDelayMs() * Math.pow(retryConfig.getBackoffMultiplier(), attempt);
        delay = Math.min(delay, retryConfig.getMaxDelayMs());
        double jitter = 1.0 + ((Math.random() * 2.0) - 1.0) * retryConfig.getJitterFactor();
        return Math.max(0L, Math.round(delay * jitter));
    }

    private TransportClient requireTransport() throws InvokerException {
        if (transport == null || !transport.isConnected()) {
            connected = false;
            throw new InvokerException(ErrorCode.UNAVAILABLE, "Transport is not connected");
        }
        return transport;
    }

    private void publishTaskEvent(String taskId, TaskEventInfo event) {
        TaskState state = activeTasks.get(taskId);
        if (state != null) {
            state.recordEvent(event);
        }
    }

    private TaskEventInfo fetchTaskEvent(String taskId) throws InvokerException {
        try {
            byte[] responseBody = requireTransport().request(
                Protocol.MSG_STREAM_TASK_REQUEST,
                SdkWireMessages.encodeTaskStreamRequest(new SdkWireMessages.TaskStreamRequest(taskId))
            );
            SdkWireMessages.TaskEvent event = SdkWireMessages.decodeTaskEvent(responseBody);
            String normalizedType = normalizeTaskEventType(event.type, event.message);
            boolean done = "completed".equals(normalizedType) ||
                "error".equals(normalizedType) ||
                "cancelled".equals(normalizedType);
            return TaskEventInfo.builder()
                .type(normalizedType)
                .taskId(taskId)
                .payload(event.payloadUtf8().isEmpty() ? null : event.payloadUtf8())
                .message(event.message)
                .progress(event.progress)
                .error(("error".equals(normalizedType) || "cancelled".equals(normalizedType)) ? event.message : null)
                .done(done)
                .build();
        } catch (InvokerException e) {
            throw e;
        } catch (Exception e) {
            throw new InvokerException(ErrorCode.INTERNAL, "StreamTask failed: " + e.getMessage(), e);
        }
    }

    private void startPolling(TaskState state) {
        if (!state.markPollingStarted()) {
            return;
        }

        Thread pollingThread = new Thread(() -> {
            try {
                while (!state.shouldStopPolling() && !state.isDone()) {
                    TaskEventInfo event = fetchTaskEvent(state.getTaskId());
                    state.recordEventIfNew(event);
                    if (event.isDone()) {
                        break;
                    }
                    Thread.sleep(500L);
                }
            } catch (InvokerException e) {
                state.fail(e);
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
            } finally {
                state.stopPolling();
            }
        }, "croupier-java-task-poller-" + state.getTaskId());
        pollingThread.setDaemon(true);
        state.setPollingThread(pollingThread);
        pollingThread.start();
    }

    private String normalizeTaskEventType(String type, String message) {
        String loweredType = type == null ? "" : type.toLowerCase();
        if ("done".equals(loweredType)) {
            return "completed";
        }
        if ("error".equals(loweredType) && message != null && message.toLowerCase().contains("cancel")) {
            return "cancelled";
        }
        return loweredType;
    }

    private TaskStatus toTaskStatus(TaskEventInfo event) {
        return switch (event.getType()) {
            case "completed" -> TaskStatus.COMPLETED;
            case "error" -> TaskStatus.ERROR;
            case "cancelled" -> TaskStatus.CANCELLED;
            case "progress" -> TaskStatus.PROGRESS;
            default -> TaskStatus.STARTED;
        };
    }

    private boolean isSameEvent(TaskEventInfo left, TaskEventInfo right) {
        return left != null && left.equals(right);
    }

    @FunctionalInterface
    private interface CheckedSupplier<T> {
        T get() throws InvokerException;
    }

    /**
     * Simulates task progress updates (for testing/future implementation).
     *
     * @param taskId the task ID to update
     * @param progress the progress percentage (0-100)
     * @param message the progress message
     */
    public void simulateTaskProgress(String taskId, int progress, String message) {
        TaskState state = activeTasks.get(taskId);
        if (state != null && !state.isDone()) {
            publishTaskEvent(taskId, TaskEventInfo.builder()
                .type("progress")
                .taskId(taskId)
                .progress(progress)
                .message(message)
                .done(false)
                .build());

            // If task is complete, mark as done
            if (progress >= 100) {
                publishTaskEvent(taskId, TaskEventInfo.builder()
                    .type("completed")
                    .taskId(taskId)
                    .message("Task completed")
                    .progress(100)
                    .done(true)
                    .build());
            }
        }
    }

    /**
     * Simulates task error (for testing/future implementation).
     *
     * @param taskId the task ID that failed
     * @param error the error message
     */
    public void simulateTaskError(String taskId, String error) {
        TaskState state = activeTasks.get(taskId);
        if (state != null) {
            publishTaskEvent(taskId, TaskEventInfo.builder()
                .type("error")
                .taskId(taskId)
                .error(error)
                .message("Task failed: " + error)
                .done(true)
                .build());
        }
    }

    // Inner classes

    /**
     * Task status enumeration.
     */
    public enum TaskStatus {
        STARTED,
        PROGRESS,
        COMPLETED,
        ERROR,
        CANCELLED
    }

    /**
     * Internal state for tracking active tasks.
     */
    private class TaskState {
        private final String taskId;
        private final String functionId;
        private final String payload;
        private final InvokeOptions options;
        private final CopyOnWriteArrayList<TaskEventInfo> events;
        private final CopyOnWriteArrayList<TaskEventSubscription> subscriptions;
        private final AtomicBoolean pollingStarted;
        private final AtomicBoolean stopPolling;
        private volatile TaskStatus status;
        private volatile InvokerException failure;
        private volatile Thread pollingThread;

        TaskState(String taskId, String functionId, String payload, InvokeOptions options) {
            this.taskId = taskId;
            this.functionId = functionId;
            this.payload = payload;
            this.options = options;
            this.events = new CopyOnWriteArrayList<>();
            this.subscriptions = new CopyOnWriteArrayList<>();
            this.pollingStarted = new AtomicBoolean(false);
            this.stopPolling = new AtomicBoolean(false);
            this.status = TaskStatus.STARTED;
        }

        String getTaskId() {
            return taskId;
        }

        String getFunctionId() {
            return functionId;
        }

        String getPayload() {
            return payload;
        }

        InvokeOptions getOptions() {
            return options;
        }

        TaskStatus getStatus() {
            return status;
        }

        void setStatus(TaskStatus status) {
            this.status = status;
        }

        boolean isDone() {
            return status == TaskStatus.COMPLETED ||
                   status == TaskStatus.ERROR ||
                   status == TaskStatus.CANCELLED;
        }

        boolean markPollingStarted() {
            return pollingStarted.compareAndSet(false, true);
        }

        boolean shouldStopPolling() {
            return stopPolling.get();
        }

        void setPollingThread(Thread pollingThread) {
            this.pollingThread = pollingThread;
        }

        void stopPolling() {
            stopPolling.set(true);
            if (pollingThread != null && pollingThread != Thread.currentThread()) {
                pollingThread.interrupt();
            }
        }

        void recordEvent(TaskEventInfo event) {
            events.add(event);
            status = toTaskStatus(event);
            for (TaskEventSubscription subscription : subscriptions) {
                subscription.emitAvailable();
            }
            if (event.isDone()) {
                for (TaskEventSubscription subscription : subscriptions) {
                    subscription.completeIfDone();
                }
            }
        }

        void recordEventIfNew(TaskEventInfo event) {
            if (events.isEmpty() || !isSameEvent(events.get(events.size() - 1), event)) {
                recordEvent(event);
            }
        }

        int eventCount() {
            return events.size();
        }

        TaskEventInfo eventAt(int index) {
            return events.get(index);
        }

        InvokerException getFailure() {
            return failure;
        }

        void fail(InvokerException error) {
            failure = error;
            for (TaskEventSubscription subscription : subscriptions) {
                subscription.emitFailure(error);
            }
        }

        void addSubscription(TaskEventSubscription subscription) {
            subscriptions.add(subscription);
        }

        void removeSubscription(TaskEventSubscription subscription) {
            subscriptions.remove(subscription);
        }
    }

    /**
     * Reactive publisher for task events.
     */
    private class TaskEventPublisher implements Publisher<TaskEventInfo> {
        private final String taskId;

        TaskEventPublisher(String taskId) {
            this.taskId = taskId;
        }

        @Override
        public void subscribe(Subscriber<? super TaskEventInfo> subscriber) {
            TaskState state = activeTasks.get(taskId);

            if (state == null) {
                subscriber.onError(new InvokerException(ErrorCode.NOT_FOUND,
                    "Task not found: " + taskId));
                return;
            }

            try {
                TaskEventSubscription subscription = new TaskEventSubscription(state, subscriber);
                state.addSubscription(subscription);
                subscriber.onSubscribe(subscription);
                subscription.emitAvailable();
                if (!state.isDone()) {
                    startPolling(state);
                }
            } catch (Exception e) {
                subscriber.onError(e);
            }
        }
    }

    /**
     * Subscription for task event streams.
     */
    private class TaskEventSubscription implements org.reactivestreams.Subscription {
        private final TaskState taskState;
        private final Subscriber<? super TaskEventInfo> subscriber;
        private final AtomicLong requested = new AtomicLong();
        private volatile boolean cancelled = false;
        private int nextIndex = 0;

        TaskEventSubscription(TaskState state, Subscriber<? super TaskEventInfo> subscriber) {
            this.taskState = state;
            this.subscriber = subscriber;
        }

        void emitAvailable() {
            synchronized (this) {
                if (cancelled) {
                    return;
                }

                try {
                    while (!cancelled && requested.get() > 0 && nextIndex < taskState.eventCount()) {
                        TaskEventInfo event = taskState.eventAt(nextIndex++);
                        subscriber.onNext(event);
                        if (requested.get() != Long.MAX_VALUE) {
                            requested.decrementAndGet();
                        }
                        if (event.isDone()) {
                            cancelInternal();
                            subscriber.onComplete();
                            return;
                        }
                    }
                    if (!cancelled && taskState.getFailure() != null && nextIndex >= taskState.eventCount()) {
                        cancelInternal();
                        subscriber.onError(taskState.getFailure());
                    }
                } catch (Exception e) {
                    cancelInternal();
                    subscriber.onError(e);
                }
            }
        }

        void completeIfDone() {
            synchronized (this) {
                if (!cancelled && taskState.isDone() && nextIndex >= taskState.eventCount()) {
                    cancelInternal();
                    subscriber.onComplete();
                }
            }
        }

        void emitFailure(InvokerException error) {
            synchronized (this) {
                if (!cancelled) {
                    cancelInternal();
                    subscriber.onError(error);
                }
            }
        }

        @Override
        public void request(long n) {
            if (n <= 0) {
                cancelInternal();
                subscriber.onError(new IllegalArgumentException("Number requested must be positive"));
                return;
            }

            requested.accumulateAndGet(n, (current, incoming) -> {
                if (current == Long.MAX_VALUE || incoming == Long.MAX_VALUE) {
                    return Long.MAX_VALUE;
                }
                long sum = current + incoming;
                return sum < 0 ? Long.MAX_VALUE : sum;
            });
            emitAvailable();
        }

        @Override
        public void cancel() {
            synchronized (this) {
                cancelInternal();
            }
        }

        private void cancelInternal() {
            cancelled = true;
            taskState.removeSubscription(this);
        }
    }
}
