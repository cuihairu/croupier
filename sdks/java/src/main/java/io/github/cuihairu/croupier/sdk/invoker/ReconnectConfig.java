package io.github.cuihairu.croupier.sdk.invoker;

import java.util.Objects;

/**
 * Configuration for automatic reconnection with exponential backoff.
 *
 * <p>When enabled, the Invoker will automatically attempt to reconnect
 * after connection failures using an exponential backoff strategy with
 * jitter to prevent thundering herd problems.</p>
 *
 * <p>Example usage:</p>
 * <pre>{@code
 * ReconnectConfig config = ReconnectConfig.builder()
 *     .enabled(true)
 *     .maxAttempts(0)  // Infinite retries
 *     .initialDelayMs(1000)  // 1 second
 *     .maxDelayMs(30000)  // 30 seconds
 *     .backoffMultiplier(2.0)
 *     .jitterFactor(0.2)
 *     .build();
 * }</pre>
 */
public class ReconnectConfig {

    private final boolean enabled;
    private final int maxAttempts;
    private final int initialDelayMs;
    private final int maxDelayMs;
    private final double backoffMultiplier;
    private final double jitterFactor;

    private ReconnectConfig(Builder builder) {
        this.enabled = builder.enabled;
        this.maxAttempts = builder.maxAttempts;
        this.initialDelayMs = builder.initialDelayMs;
        this.maxDelayMs = builder.maxDelayMs;
        this.backoffMultiplier = builder.backoffMultiplier;
        this.jitterFactor = builder.jitterFactor;
    }

    /**
     * Creates a new builder for constructing ReconnectConfig instances.
     *
     * @return a new Builder instance
     */
    public static Builder builder() {
        return new Builder();
    }

    /**
     * Creates a default reconnection configuration.
     * <ul>
     *   <li>enabled: true</li>
     *   <li>maxAttempts: 0 (infinite)</li>
     *   <li>initialDelayMs: 1000 (1 second)</li>
     *   <li>maxDelayMs: 30000 (30 seconds)</li>
     *   <li>backoffMultiplier: 2.0</li>
     *   <li>jitterFactor: 0.2</li>
     * </ul>
     *
     * @return a default ReconnectConfig
     */
    public static ReconnectConfig createDefault() {
        return builder().build();
    }

    /**
     * Checks if automatic reconnection is enabled.
     *
     * @return true if reconnection is enabled
     */
    public boolean isEnabled() {
        return enabled;
    }

    /**
     * Gets the maximum number of reconnection attempts.
     *
     * @return max attempts (0 = infinite retries)
     */
    public int getMaxAttempts() {
        return maxAttempts;
    }

    /**
     * Gets the initial reconnection delay in milliseconds.
     *
     * @return initial delay in milliseconds
     */
    public int getInitialDelayMs() {
        return initialDelayMs;
    }

    /**
     * Gets the maximum reconnection delay in milliseconds.
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

    @Override
    public boolean equals(Object o) {
        if (this == o) return true;
        if (o == null || getClass() != o.getClass()) return false;
        ReconnectConfig that = (ReconnectConfig) o;
        return enabled == that.enabled &&
               maxAttempts == that.maxAttempts &&
               initialDelayMs == that.initialDelayMs &&
               maxDelayMs == that.maxDelayMs &&
               Double.compare(that.backoffMultiplier, backoffMultiplier) == 0 &&
               Double.compare(that.jitterFactor, jitterFactor) == 0;
    }

    @Override
    public int hashCode() {
        return Objects.hash(enabled, maxAttempts, initialDelayMs, maxDelayMs,
                           backoffMultiplier, jitterFactor);
    }

    @Override
    public String toString() {
        return "ReconnectConfig{" +
               "enabled=" + enabled +
               ", maxAttempts=" + maxAttempts +
               ", initialDelayMs=" + initialDelayMs +
               ", maxDelayMs=" + maxDelayMs +
               ", backoffMultiplier=" + backoffMultiplier +
               ", jitterFactor=" + jitterFactor +
               '}';
    }

    /**
     * Builder for creating ReconnectConfig instances.
     */
    public static class Builder {
        private boolean enabled = true;
        private int maxAttempts = 0;  // 0 = infinite retries
        private int initialDelayMs = 1000;  // 1 second
        private int maxDelayMs = 30000;  // 30 seconds
        private double backoffMultiplier = 2.0;
        private double jitterFactor = 0.2;

        /**
         * Sets whether reconnection is enabled.
         *
         * @param enabled true to enable reconnection
         * @return this builder
         */
        public Builder enabled(boolean enabled) {
            this.enabled = enabled;
            return this;
        }

        /**
         * Sets the maximum number of reconnection attempts.
         *
         * @param maxAttempts maximum attempts (0 = infinite)
         * @return this builder
         */
        public Builder maxAttempts(int maxAttempts) {
            this.maxAttempts = maxAttempts;
            return this;
        }

        /**
         * Sets the initial reconnection delay.
         *
         * @param initialDelayMs initial delay in milliseconds
         * @return this builder
         */
        public Builder initialDelayMs(int initialDelayMs) {
            this.initialDelayMs = initialDelayMs;
            return this;
        }

        /**
         * Sets the maximum reconnection delay.
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
         * Builds the ReconnectConfig instance.
         *
         * @return a new ReconnectConfig
         */
        public ReconnectConfig build() {
            return new ReconnectConfig(this);
        }
    }
}
