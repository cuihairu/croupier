package io.github.cuihairu.croupier.sdk.invoker;

import java.util.HashMap;
import java.util.Map;
import java.util.Objects;

/**
 * Options for function invocation.
 *
 * <p>Use {@link #builder()} to create a new instance with custom settings.</p>
 *
 * <p>Example usage:</p>
 * <pre>{@code
 * InvokeOptions options = InvokeOptions.builder()
 *     .idempotencyKey("unique-key-123")
 *     .timeout(5000)
 *     .header("X-Request-ID", "req-456")
 *     .header("X-Game-ID", "my-game")
 *     .build();
 * }</pre>
 */
public class InvokeOptions {

    private final String idempotencyKey;
    private final Integer timeout;
    private final Map<String, String> headers;
    private final RetryConfig retry;

    private InvokeOptions(Builder builder) {
        this.idempotencyKey = builder.idempotencyKey;
        this.timeout = builder.timeout;
        this.headers = builder.headers != null ? Map.copyOf(builder.headers) : Map.of();
        this.retry = builder.retry;
    }

    /**
     * Creates a new builder for constructing InvokeOptions instances.
     *
     * @return a new Builder instance
     */
    public static Builder builder() {
        return new Builder();
    }

    /**
     * Creates empty invoke options.
     *
     * @return empty InvokeOptions
     */
    public static InvokeOptions create() {
        return builder().build();
    }

    /**
     * Gets the idempotency key for this invocation.
     * <p>Requests with the same idempotency key will return the same result.</p>
     *
     * @return the idempotency key, or null if not set
     */
    public String getIdempotencyKey() {
        return idempotencyKey;
    }

    /**
     * Gets the timeout for this specific invocation.
     * <p>If null, the default timeout from InvokerConfig is used.</p>
     *
     * @return timeout in milliseconds, or null if not set
     */
    public Integer getTimeout() {
        return timeout;
    }

    /**
     * Gets the headers to include with this invocation.
     *
     * @return an unmodifiable map of headers
     */
    public Map<String, String> getHeaders() {
        return headers;
    }

    /**
     * Gets the retry configuration override for this invocation.
     *
     * @return the retry configuration, or null if not set
     */
    public RetryConfig getRetry() {
        return retry;
    }

    @Override
    public boolean equals(Object o) {
        if (this == o) return true;
        if (o == null || getClass() != o.getClass()) return false;
        InvokeOptions that = (InvokeOptions) o;
        return Objects.equals(idempotencyKey, that.idempotencyKey) &&
               Objects.equals(timeout, that.timeout) &&
               Objects.equals(headers, that.headers) &&
               Objects.equals(retry, that.retry);
    }

    @Override
    public int hashCode() {
        return Objects.hash(idempotencyKey, timeout, headers, retry);
    }

    @Override
    public String toString() {
        return "InvokeOptions{" +
               "idempotencyKey='" + idempotencyKey + '\'' +
               ", timeout=" + timeout +
               ", headers=" + headers +
               ", retry=" + retry +
               '}';
    }

    /**
     * Builder for creating InvokeOptions instances.
     */
    public static class Builder {
        private String idempotencyKey;
        private Integer timeout;
        private Map<String, String> headers;
        private RetryConfig retry;

        /**
         * Sets the idempotency key for this invocation.
         *
         * @param idempotencyKey a unique key for idempotency
         * @return this builder
         */
        public Builder idempotencyKey(String idempotencyKey) {
            this.idempotencyKey = idempotencyKey;
            return this;
        }

        /**
         * Sets the timeout for this invocation.
         *
         * @param timeout timeout in milliseconds
         * @return this builder
         */
        public Builder timeout(int timeout) {
            this.timeout = timeout;
            return this;
        }

        /**
         * Adds a header to include with this invocation.
         *
         * @param key the header name
         * @param value the header value
         * @return this builder
         */
        public Builder header(String key, String value) {
            if (this.headers == null) {
                this.headers = new HashMap<>();
            }
            this.headers.put(key, value);
            return this;
        }

        /**
         * Sets all headers to include with this invocation.
         *
         * @param headers a map of headers
         * @return this builder
         */
        public Builder headers(Map<String, String> headers) {
            this.headers = headers != null ? new HashMap<>(headers) : null;
            return this;
        }

        /**
         * Sets the retry configuration override for this invocation.
         *
         * @param retry the retry configuration
         * @return this builder
         */
        public Builder retry(RetryConfig retry) {
            this.retry = retry;
            return this;
        }

        /**
         * Builds the InvokeOptions instance.
         *
         * @return a new InvokeOptions
         */
        public InvokeOptions build() {
            return new InvokeOptions(this);
        }
    }
}
