package io.github.cuihairu.croupier.sdk.invoker;

import java.util.List;
import java.util.Objects;

/**
 * Configuration for retrying failed invocations with exponential backoff.
 *
 * <p>When enabled, the Invoker will automatically retry failed invocations
 * using an exponential backoff strategy with jitter to prevent thundering
 * herd problems.</p>
 *
 * <p>Example usage:</p>
 * <pre>{@code
 * RetryConfig config = RetryConfig.builder()
 *     .enabled(true)
 *     .maxAttempts(3)
 *     .initialDelayMs(100)
 *     .maxDelayMs(5000)
 *     .backoffMultiplier(2.0)
 *     .jitterFactor(0.1)
 *     .build();
 * }</pre>
 */
public class RetryConfig {

    private final boolean enabled;
    private final int maxAttempts;
    private final int initialDelayMs;
    private final int maxDelayMs;
    private final double backoffMultiplier;
    private final double jitterFactor;
    private final List<Integer> retryableStatusCodes;

    private RetryConfig(Builder builder) {
        this.enabled = builder.enabled;
        this.maxAttempts = builder.maxAttempts;
        this.initialDelayMs = builder.initialDelayMs;
        this.maxDelayMs = builder.maxDelayMs;
        this.backoffMultiplier = builder.backoffMultiplier;
        this.jitterFactor = builder.jitterFactor;
        this.retryableStatusCodes = List.copyOf(builder.retryableStatusCodes);
    }

    /**
     * Creates a new builder for constructing RetryConfig instances.
     *
     * @return a new Builder instance
     */
    public static Builder builder() {
        return new Builder();
    }

    /**
     * Creates a default retry configuration.
     * <ul>
     *   <li>enabled: true</li>
     *   <li>maxAttempts: 3</li>
     *   <li>initialDelayMs: 100 (100ms)</li>
     *   <li>maxDelayMs: 5000 (5 seconds)</li>
     *   <li>backoffMultiplier: 2.0</li>
     *   <li>jitterFactor: 0.1</li>
     * </ul>
     *
     * @return a default RetryConfig
     */
    public static RetryConfig createDefault() {
        return builder().build();
    }

    /**
     * Checks if retry is enabled.
     *
     * @return true if retry is enabled
     */
    public boolean isEnabled() {
        return enabled;
    }

    /**
     * Gets the maximum number of retry attempts.
     *
     * @return max attempts
     */
    public int getMaxAttempts() {
        return maxAttempts;
    }

    /**
     * Gets the initial retry delay in milliseconds.
     *
     * @return initial delay in milliseconds
     */
    public int getInitialDelayMs() {
        return initialDelayMs;
    }

    /**
     * Gets the maximum retry delay in milliseconds.
     *
     * @return maximum delay in milliseconds
     */
    public int getMaxDelayMs() {
        return maxDelayMs;
    }

    /**
     * Gets the backoff multiplier for exponential backoff.
     *
     * @return backoff multiplier
     */
    public double getBackoffMultiplier() {
        return backoffMultiplier;
    }

    /**
     * Gets the jitter factor (0-1) for adding randomness to delays.
     *
     * @return jitter factor
     */
    public double getJitterFactor() {
        return jitterFactor;
    }

    /**
     * Gets the list of status codes that should trigger a retry.
     * Status codes follow standard error code conventions.
     *
     * @return list of retryable status codes
     */
    public List<Integer> getRetryableStatusCodes() {
        return retryableStatusCodes;
    }

    @Override
    public boolean equals(Object o) {
        if (this == o) return true;
        if (o == null || getClass() != o.getClass()) return false;
        RetryConfig that = (RetryConfig) o;
        return enabled == that.enabled &&
               maxAttempts == that.maxAttempts &&
               initialDelayMs == that.initialDelayMs &&
               maxDelayMs == that.maxDelayMs &&
               Double.compare(that.backoffMultiplier, backoffMultiplier) == 0 &&
               Double.compare(that.jitterFactor, jitterFactor) == 0 &&
               Objects.equals(retryableStatusCodes, that.retryableStatusCodes);
    }

    @Override
    public int hashCode() {
        return Objects.hash(enabled, maxAttempts, initialDelayMs, maxDelayMs,
                           backoffMultiplier, jitterFactor, retryableStatusCodes);
    }

    @Override
    public String toString() {
        return "RetryConfig{" +
               "enabled=" + enabled +
               ", maxAttempts=" + maxAttempts +
               ", initialDelayMs=" + initialDelayMs +
               ", maxDelayMs=" + maxDelayMs +
               ", backoffMultiplier=" + backoffMultiplier +
               ", jitterFactor=" + jitterFactor +
               ", retryableStatusCodes=" + retryableStatusCodes +
               '}';
    }

    /**
     * Builder for creating RetryConfig instances.
     */
    public static class Builder {
        private boolean enabled = true;
        private int maxAttempts = 3;
        private int initialDelayMs = 100;
        private int maxDelayMs = 5000;
        private double backoffMultiplier = 2.0;
        private double jitterFactor = 0.1;
        private List<Integer> retryableStatusCodes = List.of(
            14,  // UNAVAILABLE
            13,  // INTERNAL
            2,   // UNKNOWN
            10,  // ABORTED
            4    // DEADLINE_EXCEEDED
        );

        /**
         * Sets whether retry is enabled.
         *
         * @param enabled true to enable retry
         * @return this builder
         */
        public Builder enabled(boolean enabled) {
            this.enabled = enabled;
            return this;
        }

        /**
         * Sets the maximum number of retry attempts.
         *
         * @param maxAttempts maximum attempts
         * @return this builder
         */
        public Builder maxAttempts(int maxAttempts) {
            this.maxAttempts = maxAttempts;
            return this;
        }

        /**
         * Sets the initial retry delay.
         *
         * @param initialDelayMs initial delay in milliseconds
         * @return this builder
         */
        public Builder initialDelayMs(int initialDelayMs) {
            this.initialDelayMs = initialDelayMs;
            return this;
        }

        /**
         * Sets the maximum retry delay.
         *
         * @param maxDelayMs maximum delay in milliseconds
         * @return this builder
         */
        public Builder maxDelayMs(int maxDelayMs) {
            this.maxDelayMs = maxDelayMs;
            return this;
        }

        /**
         * Sets the backoff multiplier for exponential backoff.
         *
         * @param backoffMultiplier the multiplier (e.g., 2.0 for doubling)
         * @return this builder
         */
        public Builder backoffMultiplier(double backoffMultiplier) {
            this.backoffMultiplier = backoffMultiplier;
            return this;
        }

        /**
         * Sets the jitter factor for adding randomness to delays.
         *
         * @param jitterFactor jitter factor (0-1)
         * @return this builder
         */
        public Builder jitterFactor(double jitterFactor) {
            this.jitterFactor = jitterFactor;
            return this;
        }

        /**
         * Sets the status codes that should trigger a retry.
         * Status codes follow standard error code conventions.
         *
         * @param retryableStatusCodes list of status codes
         * @return this builder
         */
        public Builder retryableStatusCodes(List<Integer> retryableStatusCodes) {
            this.retryableStatusCodes = retryableStatusCodes != null
                ? List.copyOf(retryableStatusCodes)
                : List.of();
            return this;
        }

        /**
         * Builds the RetryConfig instance.
         *
         * @return a new RetryConfig
         */
        public RetryConfig build() {
            return new RetryConfig(this);
        }
    }
}
