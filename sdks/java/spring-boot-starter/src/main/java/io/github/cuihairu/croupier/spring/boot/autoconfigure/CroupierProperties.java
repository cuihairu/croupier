package io.github.cuihairu.croupier.spring.boot.autoconfigure;

import org.springframework.boot.context.properties.ConfigurationProperties;

/**
 * Croupier SDK configuration properties for Spring Boot.
 *
 * <p>These properties can be configured in application.yml or application.properties:
 *
 * <pre>
 * croupier:
 *   game-id: my-game
 *   service-id: my-service
 *   agent-address: 127.0.0.1:19090
 *   local-listen: 127.0.0.1:0
 *   env: production
 *   insecure: false
 * </pre>
 */
@ConfigurationProperties(prefix = "croupier")
public class CroupierProperties {

    /**
     * Game identifier for tenant isolation (required).
     */
    private String gameId;

    /**
     * Service identifier (required).
     */
    private String serviceId;

    /**
     * Agent gRPC address.
     */
    private String agentAddress = "127.0.0.1:19090";

    /**
     * Environment: development, staging, or production.
     */
    private String env = "development";

    /**
     * Service version for compatibility tracking.
     */
    private String serviceVersion = "1.0.0";

    /**
     * Agent identifier for load balancing.
     */
    private String agentId;

    /**
     * Local gRPC listener address.
     */
    private String localListen = "127.0.0.1:0";

    /**
     * Optional control-plane address for manifest upload.
     */
    private String controlAddr;

    /**
     * Connection timeout in seconds.
     */
    private int timeoutSeconds = 30;

    /**
     * Use insecure gRPC connection (for development).
     */
    private boolean insecure = true;

    /**
     * CA certificate file path (when not insecure).
     */
    private String caFile;

    /**
     * Client certificate file path.
     */
    private String certFile;

    /**
     * Client private key file path.
     */
    private String keyFile;

    /**
     * Provider language identifier.
     */
    private String providerLang = "java";

    /**
     * Provider SDK identifier.
     */
    private String providerSdk = "croupier-java-sdk";

    public String getGameId() {
        return gameId;
    }

    public void setGameId(String gameId) {
        this.gameId = gameId;
    }

    public String getServiceId() {
        return serviceId;
    }

    public void setServiceId(String serviceId) {
        this.serviceId = serviceId;
    }

    public String getAgentAddress() {
        return agentAddress;
    }

    public void setAgentAddress(String agentAddress) {
        this.agentAddress = agentAddress;
    }

    public String getEnv() {
        return env;
    }

    public void setEnv(String env) {
        this.env = env;
    }

    public String getServiceVersion() {
        return serviceVersion;
    }

    public void setServiceVersion(String serviceVersion) {
        this.serviceVersion = serviceVersion;
    }

    public String getAgentId() {
        return agentId;
    }

    public void setAgentId(String agentId) {
        this.agentId = agentId;
    }

    public String getLocalListen() {
        return localListen;
    }

    public void setLocalListen(String localListen) {
        this.localListen = localListen;
    }

    public String getControlAddr() {
        return controlAddr;
    }

    public void setControlAddr(String controlAddr) {
        this.controlAddr = controlAddr;
    }

    public int getTimeoutSeconds() {
        return timeoutSeconds;
    }

    public void setTimeoutSeconds(int timeoutSeconds) {
        this.timeoutSeconds = timeoutSeconds;
    }

    public boolean isInsecure() {
        return insecure;
    }

    public void setInsecure(boolean insecure) {
        this.insecure = insecure;
    }

    public String getCaFile() {
        return caFile;
    }

    public void setCaFile(String caFile) {
        this.caFile = caFile;
    }

    public String getCertFile() {
        return certFile;
    }

    public void setCertFile(String certFile) {
        this.certFile = certFile;
    }

    public String getKeyFile() {
        return keyFile;
    }

    public void setKeyFile(String keyFile) {
        this.keyFile = keyFile;
    }

    public String getProviderLang() {
        return providerLang;
    }

    public void setProviderLang(String providerLang) {
        this.providerLang = providerLang;
    }

    public String getProviderSdk() {
        return providerSdk;
    }

    public void setProviderSdk(String providerSdk) {
        this.providerSdk = providerSdk;
    }
}
