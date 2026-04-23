package io.github.cuihairu.croupier.sdk.invoker;

import java.util.Objects;

/**
 * Configuration for the Invoker connection to the Croupier server/agent.
 *
 * <p>Use {@link #builder()} to create a new configuration with custom settings,
 * or use {@link #createDefault()} for standard development settings.</p>
 *
 * <p>Example usage:</p>
 * <pre>{@code
 * InvokerConfig config = InvokerConfig.builder()
 *     .address("127.0.0.1:19090")
 *     .timeout(30000)
 *     .insecure(true)
 *     .build();
 * }</pre>
 */
public class InvokerConfig {

    private final String address;
    private final int timeout;
    private final boolean insecure;
    private final String caFile;
    private final String certFile;
    private final String keyFile;
    private final String serverName;
    private final ReconnectConfig reconnect;
    private final RetryConfig retry;

    private InvokerConfig(Builder builder) {
        this.address = builder.address;
        this.timeout = builder.timeout;
        this.insecure = builder.insecure;
        this.caFile = builder.caFile;
        this.certFile = builder.certFile;
        this.keyFile = builder.keyFile;
        this.serverName = builder.serverName;
        this.reconnect = builder.reconnect != null ? builder.reconnect : ReconnectConfig.createDefault();
        this.retry = builder.retry != null ? builder.retry : RetryConfig.createDefault();
    }

    /**
     * Creates a new builder for constructing InvokerConfig instances.
     *
     * @return a new Builder instance
     */
    public static Builder builder() {
        return new Builder();
    }

    /**
     * Creates a default configuration for development.
     * <ul>
     *   <li>address: 127.0.0.1:19090</li>
     *   <li>timeout: 30000ms</li>
     *   <li>insecure: true</li>
     * </ul>
     *
     * @return a default InvokerConfig
     */
    public static InvokerConfig createDefault() {
        return builder().build();
    }

    /**
     * Gets the server/agent address.
     *
     * @return the address in "host:port" format
     */
    public String getAddress() {
        return address;
    }

    /**
     * Gets the timeout for operations in milliseconds.
     *
     * @return timeout in milliseconds
     */
    public int getTimeout() {
        return timeout;
    }

    /**
     * Checks if the connection should use insecure (plaintext) transport.
     *
     * @return true if insecure, false if TLS should be used
     */
    public boolean isInsecure() {
        return insecure;
    }

    /**
     * Gets the CA certificate file path for TLS verification.
     *
     * @return CA file path, or empty string if not set
     */
    public String getCaFile() {
        return caFile;
    }

    /**
     * Gets the client certificate file path for mTLS.
     *
     * @return certificate file path, or empty string if not set
     */
    public String getCertFile() {
        return certFile;
    }

    /**
     * Gets the client private key file path for mTLS.
     *
     * @return key file path, or empty string if not set
     */
    public String getKeyFile() {
        return keyFile;
    }

    /**
     * Gets the server name for TLS verification (SNI).
     *
     * @return server name, or empty string if not set
     */
    public String getServerName() {
        return serverName;
    }

    /**
     * Gets the reconnection configuration.
     *
     * @return the reconnection configuration
     */
    public ReconnectConfig getReconnect() {
        return reconnect;
    }

    /**
     * Gets the retry configuration.
     *
     * @return the retry configuration
     */
    public RetryConfig getRetry() {
        return retry;
    }

    @Override
    public boolean equals(Object o) {
        if (this == o) return true;
        if (o == null || getClass() != o.getClass()) return false;
        InvokerConfig that = (InvokerConfig) o;
        return timeout == that.timeout &&
               insecure == that.insecure &&
               Objects.equals(address, that.address) &&
               Objects.equals(caFile, that.caFile) &&
               Objects.equals(certFile, that.certFile) &&
               Objects.equals(keyFile, that.keyFile) &&
               Objects.equals(serverName, that.serverName) &&
               Objects.equals(reconnect, that.reconnect) &&
               Objects.equals(retry, that.retry);
    }

    @Override
    public int hashCode() {
        return Objects.hash(address, timeout, insecure, caFile, certFile, keyFile, serverName, reconnect, retry);
    }

    @Override
    public String toString() {
        return "InvokerConfig{" +
               "address='" + address + '\'' +
               ", timeout=" + timeout +
               ", insecure=" + insecure +
               ", caFile='" + caFile + '\'' +
               ", certFile='" + certFile + '\'' +
               ", keyFile='" + keyFile + '\'' +
               ", serverName='" + serverName + '\'' +
               ", reconnect=" + reconnect +
               ", retry=" + retry +
               '}';
    }

    /**
     * Builder for creating InvokerConfig instances.
     */
    public static class Builder {
        private String address = "127.0.0.1:19090";
        private int timeout = 30000;
        private boolean insecure = true;
        private String caFile = "";
        private String certFile = "";
        private String keyFile = "";
        private String serverName = "";
        private ReconnectConfig reconnect = null;  // Null will use default in constructor
        private RetryConfig retry = null;  // Null will use default in constructor

        /**
         * Sets the server/agent address.
         *
         * @param address the address in "host:port" format
         * @return this builder
         */
        public Builder address(String address) {
            this.address = address;
            return this;
        }

        /**
         * Sets the timeout for operations.
         *
         * @param timeout timeout in milliseconds
         * @return this builder
         */
        public Builder timeout(int timeout) {
            this.timeout = timeout;
            return this;
        }

        /**
         * Sets whether to use insecure (plaintext) transport.
         *
         * @param insecure true for plaintext, false for TLS
         * @return this builder
         */
        public Builder insecure(boolean insecure) {
            this.insecure = insecure;
            return this;
        }

        /**
         * Sets the CA certificate file path for TLS verification.
         *
         * @param caFile path to CA certificate file
         * @return this builder
         */
        public Builder caFile(String caFile) {
            this.caFile = caFile != null ? caFile : "";
            return this;
        }

        /**
         * Sets the client certificate file path for mTLS.
         *
         * @param certFile path to client certificate file
         * @return this builder
         */
        public Builder certFile(String certFile) {
            this.certFile = certFile != null ? certFile : "";
            return this;
        }

        /**
         * Sets the client private key file path for mTLS.
         *
         * @param keyFile path to client private key file
         * @return this builder
         */
        public Builder keyFile(String keyFile) {
            this.keyFile = keyFile != null ? keyFile : "";
            return this;
        }

        /**
         * Sets the server name for TLS verification (SNI).
         *
         * @param serverName the expected server name
         * @return this builder
         */
        public Builder serverName(String serverName) {
            this.serverName = serverName != null ? serverName : "";
            return this;
        }

        /**
         * Sets the reconnection configuration.
         *
         * @param reconnect the reconnection configuration
         * @return this builder
         */
        public Builder reconnect(ReconnectConfig reconnect) {
            this.reconnect = reconnect;
            return this;
        }

        /**
         * Sets the retry configuration.
         *
         * @param retry the retry configuration
         * @return this builder
         */
        public Builder retry(RetryConfig retry) {
            this.retry = retry;
            return this;
        }

        /**
         * Builds the InvokerConfig instance.
         *
         * @return a new InvokerConfig
         */
        public InvokerConfig build() {
            return new InvokerConfig(this);
        }
    }
}
