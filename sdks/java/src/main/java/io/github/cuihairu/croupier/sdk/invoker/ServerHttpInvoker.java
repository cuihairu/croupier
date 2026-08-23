package io.github.cuihairu.croupier.sdk.invoker;

import org.reactivestreams.Publisher;
import org.reactivestreams.Subscriber;
import org.reactivestreams.Subscription;

import java.io.IOException;
import java.net.URI;
import java.net.URLEncoder;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Locale;
import java.util.Map;
import java.util.Objects;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicLong;

import static io.github.cuihairu.croupier.sdk.invoker.InvokerException.ErrorCode;

/**
 * Public L3 Invoker backed only by the Croupier Server HTTP API.
 *
 * <p>It has no dependency on Provider TCP transport: Server authorization,
 * scope, audit, routing and task persistence therefore remain authoritative.</p>
 */
public final class ServerHttpInvoker implements TaskStatusInvoker {
    public static final String DEFAULT_SERVER_API_URL = "http://127.0.0.1:18780/api/v1";

    private final InvokerConfig config;
    private final HttpClient client;
    private final String baseUrl;
    private final Map<String, Map<String, Object>> schemas = new java.util.concurrent.ConcurrentHashMap<>();
    private volatile boolean connected;

    public ServerHttpInvoker(InvokerConfig config) {
        this(Objects.requireNonNull(config, "config"), createHttpClient(config));
    }

    /** Constructor exposed to mock HTTP contract tests. */
    public ServerHttpInvoker(InvokerConfig config, HttpClient client) {
        this.config = Objects.requireNonNull(config, "config");
        this.client = Objects.requireNonNull(client, "client");
        this.baseUrl = normalizeBaseUrl(config.getAddress());
    }

    private static HttpClient createHttpClient(InvokerConfig config) {
        return HttpClient.newBuilder()
            .connectTimeout(Duration.ofMillis(Objects.requireNonNull(config, "config").getTimeout()))
            .build();
    }

    @Override
    public void connect() {
        connected = true;
    }

    @Override
    public String invoke(String functionId, String payload) throws InvokerException {
        return invoke(functionId, payload, InvokeOptions.create());
    }

    @Override
    public String invoke(String functionId, String payload, InvokeOptions options) throws InvokerException {
        validateIdentifier("function ID", functionId);
        Map<String, Object> body = new LinkedHashMap<>();
        body.put("params", parseAndValidatePayload(functionId, payload));
        Map<String, Object> response = objectResponse(request("POST", List.of("functions", functionId, "invoke"), body, options, Map.of()), "invoke");
        if (!response.containsKey("result")) {
            throw new InvokerException(ErrorCode.INTERNAL, "server invoke response does not contain result");
        }
        return Json.stringify(response.get("result"));
    }

    @Override
    public String startTask(String functionId, String payload) throws InvokerException {
        return startTask(functionId, payload, InvokeOptions.create());
    }

    @Override
    public String startTask(String functionId, String payload, InvokeOptions options) throws InvokerException {
        validateIdentifier("function ID", functionId);
        Map<String, Object> body = new LinkedHashMap<>();
        body.put("functionId", functionId);
        body.put("params", parseAndValidatePayload(functionId, payload));
        String taskId = stringValue(objectResponse(request("POST", List.of("tasks"), body, options, Map.of()), "start task").get("taskId"));
        if (taskId == null || taskId.isBlank()) {
            throw new InvokerException(ErrorCode.INTERNAL, "server start task response does not contain taskId");
        }
        return taskId;
    }

    @Override
    public TaskStatusInfo getTaskStatus(String taskId) throws InvokerException {
        validateIdentifier("task ID", taskId);
        Map<String, Object> response = objectResponse(request("GET", List.of("tasks", taskId), null, null, Map.of()), "task status");
        Object result = response.get("result");
        return new TaskStatusInfo(
            defaultValue(stringValue(response.get("id")), taskId), stringValue(response.get("functionId")),
            defaultValue(stringValue(response.get("status")), "unknown"), integerValue(response.get("progress")),
            stringValue(response.get("message")), result == null ? null : Json.stringify(result), stringValue(response.get("error")),
            stringValue(response.get("startedAt")), stringValue(response.get("finishedAt")),
            stringValue(response.get("createdAt")), stringValue(response.get("updatedAt"))
        );
    }

    @Override
    public Publisher<TaskEventInfo> streamTask(String taskId) {
        try {
            validateIdentifier("task ID", taskId);
        } catch (InvokerException error) {
            return subscriber -> { subscriber.onSubscribe(NoopSubscription.INSTANCE); subscriber.onError(error); };
        }
        return subscriber -> subscriber.onSubscribe(new EventsSubscription(subscriber, taskId));
    }

    @Override
    public void cancelTask(String taskId) throws InvokerException {
        validateIdentifier("task ID", taskId);
        request("POST", List.of("tasks", taskId, "cancel"), Map.of(), null, Map.of());
    }

    @Override
    public void setSchema(String functionId, Map<String, Object> schema) {
        if (functionId == null || functionId.isBlank()) {
            throw new IllegalArgumentException("function ID cannot be empty");
        }
        schemas.put(functionId, schema == null ? Map.of() : Map.copyOf(schema));
    }

    @Override
    public void close() {
        connected = false;
        schemas.clear();
    }

    @Override
    public boolean isConnected() {
        return connected;
    }

    public String getBaseUrl() {
        return baseUrl;
    }

    private Object parseAndValidatePayload(String functionId, String payload) throws InvokerException {
        Object value;
        try {
            value = Json.parse(payload == null || payload.isBlank() ? "{}" : payload);
        } catch (IllegalArgumentException exception) {
            throw new InvokerException(ErrorCode.INVALID_ARGUMENT, "payload must be valid JSON: " + exception.getMessage(), exception);
        }
        Map<String, Object> schema = schemas.get(functionId);
        if (schema == null || schema.isEmpty()) return value;
        List<String> errors = JsonSchemaValidator.validate(schema, value);
        if (!errors.isEmpty()) {
            throw new InvokerException(ErrorCode.INVALID_ARGUMENT, "payload validation failed: " + String.join("; ", errors));
        }
        return value;
    }

    private Object request(String method, List<String> segments, Map<String, Object> body, InvokeOptions options, Map<String, String> query) throws InvokerException {
        InvokeOptions effective = options == null ? InvokeOptions.create() : options;
        RetryConfig retry = effective.getRetry() == null ? config.getRetry() : effective.getRetry();
        int attempts = retry != null && retry.isEnabled() ? Math.max(1, retry.getMaxAttempts()) : 1;
        InvokerException previous = null;
        for (int attempt = 0; attempt < attempts; attempt++) {
            try {
                return requestOnce(method, segments, body, effective, query);
            } catch (InvokerException error) {
                previous = error;
                if (attempt == attempts - 1 || !isRetryable(error)) throw error;
                try {
                    Thread.sleep(retryDelay(attempt, retry));
                } catch (InterruptedException interrupted) {
                    Thread.currentThread().interrupt();
                    throw new InvokerException(ErrorCode.CANCELLED, "Server HTTP request interrupted", interrupted);
                }
            }
        }
        throw previous == null ? new InvokerException(ErrorCode.UNKNOWN, "Server HTTP request failed") : previous;
    }

    private Object requestOnce(String method, List<String> segments, Map<String, Object> body, InvokeOptions options, Map<String, String> query) throws InvokerException {
        String payload = body == null ? null : Json.stringify(body);
        HttpRequest.Builder request = HttpRequest.newBuilder(endpoint(segments, query))
            .timeout(Duration.ofMillis(options.getTimeout() == null ? config.getTimeout() : options.getTimeout()));
        for (Map.Entry<String, String> header : headers(options).entrySet()) {
            request.header(header.getKey(), header.getValue());
        }
        if (payload != null) request.header("Content-Type", "application/json");
        request.method(method, payload == null ? HttpRequest.BodyPublishers.noBody() : HttpRequest.BodyPublishers.ofString(payload));
        try {
            HttpResponse<String> response = client.send(request.build(), HttpResponse.BodyHandlers.ofString());
            if (response.statusCode() < 200 || response.statusCode() >= 300) {
                throw InvokerException.fromHttpStatus(response.statusCode(), serverErrorMessage(response.body()));
            }
            return response.body().isBlank() ? Map.of() : Json.parse(response.body());
        } catch (InvokerException exception) {
            throw exception;
        } catch (java.net.http.HttpTimeoutException exception) {
            throw new InvokerException(ErrorCode.TIMEOUT, "Server HTTP request timed out", exception);
        } catch (IOException exception) {
            throw new InvokerException(ErrorCode.UNAVAILABLE, "send Server HTTP request: " + exception.getMessage(), exception);
        } catch (InterruptedException exception) {
            Thread.currentThread().interrupt();
            throw new InvokerException(ErrorCode.CANCELLED, "Server HTTP request interrupted", exception);
        } catch (IllegalArgumentException exception) {
            throw new InvokerException(ErrorCode.INTERNAL, "server returned invalid JSON: " + exception.getMessage(), exception);
        }
    }

    private Map<String, String> headers(InvokeOptions options) throws InvokerException {
        Map<String, String> headers = new LinkedHashMap<>();
        for (Map.Entry<String, String> header : options.getHeaders().entrySet()) {
            if (header.getKey() == null || header.getValue() == null || header.getKey().contains("\r") || header.getKey().contains("\n") || header.getValue().contains("\r") || header.getValue().contains("\n")) {
                throw new InvokerException(ErrorCode.INVALID_ARGUMENT, "HTTP headers cannot contain null, CR or LF");
            }
            headers.put(header.getKey(), header.getValue());
        }
        if (options.getIdempotencyKey() != null && !options.getIdempotencyKey().isBlank() && !containsHeader(headers, "Idempotency-Key")) headers.put("Idempotency-Key", options.getIdempotencyKey());
        if (!config.getGameId().isBlank() && !containsHeader(headers, "X-Game-ID")) headers.put("X-Game-ID", config.getGameId());
        if (!config.getEnv().isBlank() && !containsHeader(headers, "X-Env")) headers.put("X-Env", config.getEnv());
        if (!config.getAuthToken().isBlank() && !containsHeader(headers, "Authorization")) {
            String token = config.getAuthToken().trim();
            headers.put("Authorization", token.regionMatches(true, 0, "Bearer ", 0, 7) ? token : "Bearer " + token);
        }
        return headers;
    }

    private URI endpoint(List<String> segments, Map<String, String> query) {
        StringBuilder value = new StringBuilder(baseUrl);
        for (String segment : segments) value.append('/').append(encode(segment));
        if (!query.isEmpty()) {
            value.append('?'); boolean first = true;
            for (Map.Entry<String, String> entry : query.entrySet()) {
                if (!first) value.append('&');
                value.append(encode(entry.getKey())).append('=').append(encode(entry.getValue())); first = false;
            }
        }
        return URI.create(value.toString());
    }

    private static String normalizeBaseUrl(String address) {
        String candidate = address == null || address.isBlank() ? DEFAULT_SERVER_API_URL : address.trim();
        if (!candidate.contains("://")) candidate = "http://" + candidate;
        URI uri = URI.create(candidate);
        if (!("http".equalsIgnoreCase(uri.getScheme()) || "https".equalsIgnoreCase(uri.getScheme())) || uri.getHost() == null) {
            throw new IllegalArgumentException("InvokerConfig.address must be an HTTP(S) Server address");
        }
        String path = uri.getPath() == null ? "" : uri.getPath().replaceAll("/+$", "");
        if (!path.endsWith("/api/v1")) path = path.isEmpty() ? "/api/v1" : path + "/api/v1";
        return uri.getScheme() + "://" + uri.getAuthority() + path;
    }

    private static void validateIdentifier(String name, String value) throws InvokerException {
        if (value == null || value.isBlank()) throw new InvokerException(ErrorCode.INVALID_ARGUMENT, name + " cannot be empty");
    }
    private static boolean containsHeader(Map<String, String> headers, String expected) { return headers.keySet().stream().anyMatch(key -> key.equalsIgnoreCase(expected)); }
    private static boolean isRetryable(InvokerException exception) { return exception.getErrorCode() == ErrorCode.UNAVAILABLE || exception.getErrorCode() == ErrorCode.RESOURCE_EXHAUSTED || exception.getErrorCode() == ErrorCode.TIMEOUT; }
    private static long retryDelay(int attempt, RetryConfig retry) { double delay = retry.getInitialDelayMs() * Math.pow(retry.getBackoffMultiplier() > 0 ? retry.getBackoffMultiplier() : 2, attempt); if (retry.getMaxDelayMs() > 0) delay = Math.min(delay, retry.getMaxDelayMs()); if (retry.getJitterFactor() > 0) delay += delay * retry.getJitterFactor() * (Math.random() * 2 - 1); return Math.max(0L, Math.round(delay)); }
    @SuppressWarnings("unchecked") private static Map<String, Object> objectResponse(Object response, String operation) throws InvokerException { if (!(response instanceof Map<?, ?>)) throw new InvokerException(ErrorCode.INTERNAL, "server " + operation + " response must be an object"); return (Map<String, Object>) response; }
    private static String stringValue(Object value) { return value instanceof String string ? string : null; }
    private static Integer integerValue(Object value) { return value instanceof Number number ? number.intValue() : null; }
    private static String defaultValue(String value, String fallback) { return value == null || value.isBlank() ? fallback : value; }
    private static String encode(String value) { return URLEncoder.encode(value, StandardCharsets.UTF_8).replace("+", "%20"); }
    private static String serverErrorMessage(String body) { try { Object value = Json.parse(body); if (value instanceof Map<?, ?> map) { String message = stringValue(map.get("message")); if (message != null && !message.isBlank()) return message; String error = stringValue(map.get("error")); if (error != null && !error.isBlank()) return error; } } catch (IllegalArgumentException ignored) {} return body == null || body.isBlank() ? "empty response body" : body; }

    private enum NoopSubscription implements Subscription { INSTANCE; public void request(long ignored) {} public void cancel() {} }

    private final class EventsSubscription implements Subscription, Runnable {
        private final Subscriber<? super TaskEventInfo> subscriber;
        private final String taskId;
        private final AtomicBoolean started = new AtomicBoolean();
        private final AtomicBoolean cancelled = new AtomicBoolean();
        private final AtomicLong demand = new AtomicLong();
        private EventsSubscription(Subscriber<? super TaskEventInfo> subscriber, String taskId) { this.subscriber = Objects.requireNonNull(subscriber, "subscriber"); this.taskId = taskId; }
        @Override public void request(long count) { if (count <= 0) { cancel(); subscriber.onError(new IllegalArgumentException("request count must be positive")); return; } demand.accumulateAndGet(count, (current, incoming) -> current > Long.MAX_VALUE - incoming ? Long.MAX_VALUE : current + incoming); if (started.compareAndSet(false, true)) { Thread worker = new Thread(this, "croupier-server-task-events-" + taskId); worker.setDaemon(true); worker.start(); } }
        @Override public void cancel() { cancelled.set(true); }
        @Override public void run() {
            long afterSeq = 0;
            try {
                while (!cancelled.get()) {
                    Map<String, Object> response = objectResponse(ServerHttpInvoker.this.request("GET", List.of("tasks", taskId, "events"), null, null, Map.of("after_seq", Long.toString(afterSeq))), "task events");
                    if (!(response.get("items") instanceof List<?> items)) throw new InvokerException(ErrorCode.INTERNAL, "server task events items must be an array");
                    boolean emitted = false;
                    for (Object raw : items) {
                        if (!(raw instanceof Map<?, ?> rawMap)) throw new InvokerException(ErrorCode.INTERNAL, "server task event must be an object");
                        @SuppressWarnings("unchecked") Map<String, Object> item = (Map<String, Object>) rawMap;
                        Integer seq = integerValue(item.get("seq")); if (seq != null) afterSeq = Math.max(afterSeq, seq.longValue());
                        awaitDemand(); if (cancelled.get()) return; subscriber.onNext(taskEvent(taskId, item)); emitted = true;
                    }
                    if (Boolean.TRUE.equals(response.get("done"))) { if (!cancelled.get()) subscriber.onComplete(); return; }
                    if (!emitted) Thread.sleep(Math.max(1, config.getTaskPollIntervalMs()));
                }
            } catch (InterruptedException interrupted) { Thread.currentThread().interrupt(); } catch (Exception error) { if (!cancelled.get()) subscriber.onError(error); }
        }
        private void awaitDemand() throws InterruptedException { while (!cancelled.get()) { long available = demand.get(); if (available > 0 && demand.compareAndSet(available, available - 1)) return; Thread.sleep(1); } }
    }

    private static TaskEventInfo taskEvent(String taskId, Map<String, Object> item) {
        String type = defaultValue(stringValue(item.get("type")), "unknown"); if ("done".equals(type)) type = "completed";
        String message = stringValue(item.get("message")); Object payload = item.get("payload"); boolean done = List.of("completed", "failed", "error", "cancelled", "timed_out").contains(type);
        return TaskEventInfo.builder().type(type).taskId(taskId).payload(payload == null ? null : Json.stringify(payload)).message(message).progress(integerValue(item.get("progress"))).error(List.of("failed", "error", "cancelled", "timed_out").contains(type) ? message : null).done(done).build();
    }

}
