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
 * Implementation of the Invoker interface with job management support.
 *
 * <p>This implementation uses the shared SDK wire protocol over transport abstractions.</p>
 */
public class InvokerImpl implements Invoker {

    private static final Logger logger = LoggerFactory.getLogger(InvokerImpl.class);

    private final InvokerConfig config;
    private final Map<String, Map<String, Object>> schemas;
    private final Map<String, JobState> activeJobs;
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
        this.activeJobs = new ConcurrentHashMap<>();
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
    public String startJob(String functionId, String payload) throws InvokerException {
        return startJob(functionId, payload, InvokeOptions.create());
    }

    @Override
    public String startJob(String functionId, String payload, InvokeOptions options) throws InvokerException {
        ensureConnected();
        InvokeOptions effectiveOptions = options != null ? options : InvokeOptions.create();
        return withRetry("StartJob", effectiveOptions.getRetry(), () -> startJobInternal(functionId, payload, effectiveOptions));
    }

    @Override
    public Publisher<JobEventInfo> streamJob(String jobId) {
        logger.info("Streaming events for job: {}", jobId);
        return new JobEventPublisher(jobId);
    }

    @Override
    public void cancelJob(String jobId) throws InvokerException {
        JobState jobState = activeJobs.get(jobId);

        if (jobState == null) {
            throw new InvokerException(ErrorCode.NOT_FOUND,
                "Job not found: " + jobId);
        }

        if (jobState.isDone()) {
            throw new InvokerException(ErrorCode.FAILED_PRECONDITION,
                "Job already finished: " + jobId + " (status: " + jobState.getStatus() + ")");
        }

        try {
            logger.info("Cancelling job: {}", jobId);
            requireTransport().request(
                Protocol.MSG_CANCEL_JOB_REQUEST,
                SdkWireMessages.encodeCancelJobRequest(new SdkWireMessages.CancelJobRequest(jobId))
            );
            publishJobEvent(jobId, JobEventInfo.builder()
                .type("cancelled")
                .jobId(jobId)
                .message("Job cancelled")
                .done(true)
                .build());
            logger.info("Job cancelled: {}", jobId);
        } catch (Exception e) {
            if (e instanceof InvokerException) {
                throw (InvokerException) e;
            }
            throw new InvokerException(ErrorCode.INTERNAL, "CancelJob failed: " + e.getMessage(), e);
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
        for (JobState state : activeJobs.values()) {
            state.stopPolling();
        }
        connected = false;
        schemas.clear();
        activeJobs.clear();
        logger.info("Invoker closed");
    }

    @Override
    public boolean isConnected() {
        return connected;
    }

    /**
     * Gets the number of active jobs.
     *
     * @return the count of active jobs
     */
    public int getActiveJobCount() {
        return activeJobs.size();
    }

    /**
     * Checks if a job exists.
     *
     * @param jobId the job ID to check
     * @return true if the job exists, false otherwise
     */
    public boolean hasJob(String jobId) {
        return activeJobs.containsKey(jobId);
    }

    /**
     * Gets the status of a job.
     *
     * @param jobId the job ID
     * @return the job status, or null if not found
     */
    public JobStatus getJobStatus(String jobId) {
        JobState state = activeJobs.get(jobId);
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

    private String startJobInternal(String functionId, String payload, InvokeOptions options) throws InvokerException {
        try {
            logger.debug("Starting job for function: {}", functionId);
            SdkWireMessages.InvokeRequest request = new SdkWireMessages.InvokeRequest(
                functionId,
                options.getIdempotencyKey(),
                (payload == null ? "" : payload).getBytes(StandardCharsets.UTF_8),
                options.getHeaders()
            );
            byte[] responseBody = requireTransport().request(
                Protocol.MSG_START_JOB_REQUEST,
                SdkWireMessages.encodeInvokeRequest(request)
            );
            String jobId = SdkWireMessages.decodeStartJobResponse(responseBody).jobId;
            if (jobId.isEmpty()) {
                throw new InvokerException(ErrorCode.INTERNAL, "StartJob response did not include job ID");
            }

            JobState jobState = new JobState(jobId, functionId, payload == null ? "" : payload, options);
            activeJobs.put(jobId, jobState);
            publishJobEvent(jobId, JobEventInfo.builder()
                .type("started")
                .jobId(jobId)
                .message("Job started")
                .done(false)
                .build());
            return jobId;
        } catch (InvokerException e) {
            throw e;
        } catch (Exception e) {
            throw new InvokerException(ErrorCode.INTERNAL, "StartJob failed: " + e.getMessage(), e);
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

    private void publishJobEvent(String jobId, JobEventInfo event) {
        JobState state = activeJobs.get(jobId);
        if (state != null) {
            state.recordEvent(event);
        }
    }

    private JobEventInfo fetchJobEvent(String jobId) throws InvokerException {
        try {
            byte[] responseBody = requireTransport().request(
                Protocol.MSG_STREAM_JOB_REQUEST,
                SdkWireMessages.encodeJobStreamRequest(new SdkWireMessages.JobStreamRequest(jobId))
            );
            SdkWireMessages.JobEvent event = SdkWireMessages.decodeJobEvent(responseBody);
            String normalizedType = normalizeJobEventType(event.type, event.message);
            boolean done = "completed".equals(normalizedType) ||
                "error".equals(normalizedType) ||
                "cancelled".equals(normalizedType);
            return JobEventInfo.builder()
                .type(normalizedType)
                .jobId(jobId)
                .payload(event.payloadUtf8().isEmpty() ? null : event.payloadUtf8())
                .message(event.message)
                .progress(event.progress)
                .error(("error".equals(normalizedType) || "cancelled".equals(normalizedType)) ? event.message : null)
                .done(done)
                .build();
        } catch (InvokerException e) {
            throw e;
        } catch (Exception e) {
            throw new InvokerException(ErrorCode.INTERNAL, "StreamJob failed: " + e.getMessage(), e);
        }
    }

    private void startPolling(JobState state) {
        if (!state.markPollingStarted()) {
            return;
        }

        Thread pollingThread = new Thread(() -> {
            try {
                while (!state.shouldStopPolling() && !state.isDone()) {
                    JobEventInfo event = fetchJobEvent(state.getJobId());
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
        }, "croupier-java-job-poller-" + state.getJobId());
        pollingThread.setDaemon(true);
        state.setPollingThread(pollingThread);
        pollingThread.start();
    }

    private String normalizeJobEventType(String type, String message) {
        String loweredType = type == null ? "" : type.toLowerCase();
        if ("done".equals(loweredType)) {
            return "completed";
        }
        if ("error".equals(loweredType) && message != null && message.toLowerCase().contains("cancel")) {
            return "cancelled";
        }
        return loweredType;
    }

    private JobStatus toJobStatus(JobEventInfo event) {
        return switch (event.getType()) {
            case "completed" -> JobStatus.COMPLETED;
            case "error" -> JobStatus.ERROR;
            case "cancelled" -> JobStatus.CANCELLED;
            case "progress" -> JobStatus.PROGRESS;
            default -> JobStatus.STARTED;
        };
    }

    private boolean isSameEvent(JobEventInfo left, JobEventInfo right) {
        return left != null && left.equals(right);
    }

    @FunctionalInterface
    private interface CheckedSupplier<T> {
        T get() throws InvokerException;
    }

    /**
     * Simulates job progress updates (for testing/future implementation).
     *
     * @param jobId the job ID to update
     * @param progress the progress percentage (0-100)
     * @param message the progress message
     */
    public void simulateJobProgress(String jobId, int progress, String message) {
        JobState state = activeJobs.get(jobId);
        if (state != null && !state.isDone()) {
            publishJobEvent(jobId, JobEventInfo.builder()
                .type("progress")
                .jobId(jobId)
                .progress(progress)
                .message(message)
                .done(false)
                .build());

            // If job is complete, mark as done
            if (progress >= 100) {
                publishJobEvent(jobId, JobEventInfo.builder()
                    .type("completed")
                    .jobId(jobId)
                    .message("Job completed")
                    .progress(100)
                    .done(true)
                    .build());
            }
        }
    }

    /**
     * Simulates job error (for testing/future implementation).
     *
     * @param jobId the job ID that failed
     * @param error the error message
     */
    public void simulateJobError(String jobId, String error) {
        JobState state = activeJobs.get(jobId);
        if (state != null) {
            publishJobEvent(jobId, JobEventInfo.builder()
                .type("error")
                .jobId(jobId)
                .error(error)
                .message("Job failed: " + error)
                .done(true)
                .build());
        }
    }

    // Inner classes

    /**
     * Job status enumeration.
     */
    public enum JobStatus {
        STARTED,
        PROGRESS,
        COMPLETED,
        ERROR,
        CANCELLED
    }

    /**
     * Internal state for tracking active jobs.
     */
    private class JobState {
        private final String jobId;
        private final String functionId;
        private final String payload;
        private final InvokeOptions options;
        private final CopyOnWriteArrayList<JobEventInfo> events;
        private final CopyOnWriteArrayList<JobEventSubscription> subscriptions;
        private final AtomicBoolean pollingStarted;
        private final AtomicBoolean stopPolling;
        private volatile JobStatus status;
        private volatile InvokerException failure;
        private volatile Thread pollingThread;

        JobState(String jobId, String functionId, String payload, InvokeOptions options) {
            this.jobId = jobId;
            this.functionId = functionId;
            this.payload = payload;
            this.options = options;
            this.events = new CopyOnWriteArrayList<>();
            this.subscriptions = new CopyOnWriteArrayList<>();
            this.pollingStarted = new AtomicBoolean(false);
            this.stopPolling = new AtomicBoolean(false);
            this.status = JobStatus.STARTED;
        }

        String getJobId() {
            return jobId;
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

        JobStatus getStatus() {
            return status;
        }

        void setStatus(JobStatus status) {
            this.status = status;
        }

        boolean isDone() {
            return status == JobStatus.COMPLETED ||
                   status == JobStatus.ERROR ||
                   status == JobStatus.CANCELLED;
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

        void recordEvent(JobEventInfo event) {
            events.add(event);
            status = toJobStatus(event);
            for (JobEventSubscription subscription : subscriptions) {
                subscription.emitAvailable();
            }
            if (event.isDone()) {
                for (JobEventSubscription subscription : subscriptions) {
                    subscription.completeIfDone();
                }
            }
        }

        void recordEventIfNew(JobEventInfo event) {
            if (events.isEmpty() || !isSameEvent(events.get(events.size() - 1), event)) {
                recordEvent(event);
            }
        }

        int eventCount() {
            return events.size();
        }

        JobEventInfo eventAt(int index) {
            return events.get(index);
        }

        InvokerException getFailure() {
            return failure;
        }

        void fail(InvokerException error) {
            failure = error;
            for (JobEventSubscription subscription : subscriptions) {
                subscription.emitFailure(error);
            }
        }

        void addSubscription(JobEventSubscription subscription) {
            subscriptions.add(subscription);
        }

        void removeSubscription(JobEventSubscription subscription) {
            subscriptions.remove(subscription);
        }
    }

    /**
     * Reactive publisher for job events.
     */
    private class JobEventPublisher implements Publisher<JobEventInfo> {
        private final String jobId;

        JobEventPublisher(String jobId) {
            this.jobId = jobId;
        }

        @Override
        public void subscribe(Subscriber<? super JobEventInfo> subscriber) {
            JobState state = activeJobs.get(jobId);

            if (state == null) {
                subscriber.onError(new InvokerException(ErrorCode.NOT_FOUND,
                    "Job not found: " + jobId));
                return;
            }

            try {
                JobEventSubscription subscription = new JobEventSubscription(state, subscriber);
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
     * Subscription for job event streams.
     */
    private class JobEventSubscription implements org.reactivestreams.Subscription {
        private final JobState jobState;
        private final Subscriber<? super JobEventInfo> subscriber;
        private final AtomicLong requested = new AtomicLong();
        private volatile boolean cancelled = false;
        private int nextIndex = 0;

        JobEventSubscription(JobState state, Subscriber<? super JobEventInfo> subscriber) {
            this.jobState = state;
            this.subscriber = subscriber;
        }

        void emitAvailable() {
            synchronized (this) {
                if (cancelled) {
                    return;
                }

                try {
                    while (!cancelled && requested.get() > 0 && nextIndex < jobState.eventCount()) {
                        JobEventInfo event = jobState.eventAt(nextIndex++);
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
                    if (!cancelled && jobState.getFailure() != null && nextIndex >= jobState.eventCount()) {
                        cancelInternal();
                        subscriber.onError(jobState.getFailure());
                    }
                } catch (Exception e) {
                    cancelInternal();
                    subscriber.onError(e);
                }
            }
        }

        void completeIfDone() {
            synchronized (this) {
                if (!cancelled && jobState.isDone() && nextIndex >= jobState.eventCount()) {
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
            jobState.removeSubscription(this);
        }
    }
}
