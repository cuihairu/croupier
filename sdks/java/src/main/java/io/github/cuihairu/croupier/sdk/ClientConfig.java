package io.github.cuihairu.croupier.sdk;

import java.util.HashMap;
import java.util.Map;

/**
 * Configuration for Croupier client
 */
public class ClientConfig {
    // ========== Agent Connection Settings ==========
    private String agentAddr = "127.0.0.1:19090"; // Agent TCP address
    private String agentId;                       // Agent unique identifier (auto-generated if empty)

    // ========== Service Identification (multi-tenant support) ==========
    private String gameId;          // game identifier for tenant isolation
    private String env = "development"; // environment: "development"|"staging"|"production"
    private String serviceId;       // unique service identifier
    private String serviceVersion = "1.0.0"; // service version for compatibility

    // ========== Control Plane Settings ==========
    private String controlAddr;     // optional control-plane addr for manifest upload

    // ========== Connection Settings ==========
    private int timeoutSeconds = 30; // connection timeout in seconds
    private boolean insecure = true; // use insecure connection (for development)

    // ========== Heartbeat Configuration ==========
    private int heartbeatInterval = 60; // heartbeat interval in seconds

    // ========== TLS Settings (when not insecure) ==========
    private String caFile;   // CA certificate file path
    private String certFile; // client certificate file path
    private String keyFile;  // client private key file path
    private String serverName; // Server name for TLS verification

    // ========== Authentication ==========
    private String authToken;                      // Bearer token for authentication
    private Map<String, String> headers = new HashMap<>();  // Additional headers

    // ========== Provider Metadata ==========
    private String providerLang = "java";
    private String providerSdk = "croupier-java-sdk";

    // ========== Reconnection Configuration ==========
    private ReconnectConfig reconnect; // Reconnection config with exponential backoff

    // ========== File Transfer Configuration ==========
    private boolean enableFileTransfer = false;  // Enable file transfer functionality (default: false)
    private int maxFileSize = 10485760;         // Max file size in bytes (default: 10MB)

    // ========== Logging Configuration ==========
    private boolean disableLogging = false;  // Disable all logging
    private boolean debugLogging = false;    // Enable debug level logging
    private String logLevel = "INFO";       // Log level: "DEBUG", "INFO", "WARN", "ERROR", "OFF"

    public ClientConfig() {}

    public ClientConfig(String gameId, String serviceId) {
        this.gameId = gameId;
        this.serviceId = serviceId;
    }

    // Getters and setters
    public String getAgentAddr() { return agentAddr; }
    public void setAgentAddr(String agentAddr) { this.agentAddr = agentAddr; }

    public String getAgentId() { return agentId; }
    public void setAgentId(String agentId) { this.agentId = agentId; }

    public String getGameId() { return gameId; }
    public void setGameId(String gameId) { this.gameId = gameId; }

    public String getEnv() { return env; }
    public void setEnv(String env) { this.env = env; }

    public String getServiceId() { return serviceId; }
    public void setServiceId(String serviceId) { this.serviceId = serviceId; }

    public String getServiceVersion() { return serviceVersion; }
    public void setServiceVersion(String serviceVersion) { this.serviceVersion = serviceVersion; }

    public String getControlAddr() { return controlAddr; }
    public void setControlAddr(String controlAddr) { this.controlAddr = controlAddr; }

    public int getTimeoutSeconds() { return timeoutSeconds; }
    public void setTimeoutSeconds(int timeoutSeconds) { this.timeoutSeconds = timeoutSeconds; }

    public boolean isInsecure() { return insecure; }
    public void setInsecure(boolean insecure) { this.insecure = insecure; }

    public int getHeartbeatInterval() { return heartbeatInterval; }
    public void setHeartbeatInterval(int heartbeatInterval) { this.heartbeatInterval = heartbeatInterval; }

    public String getCaFile() { return caFile; }
    public void setCaFile(String caFile) { this.caFile = caFile; }

    public String getCertFile() { return certFile; }
    public void setCertFile(String certFile) { this.certFile = certFile; }

    public String getKeyFile() { return keyFile; }
    public void setKeyFile(String keyFile) { this.keyFile = keyFile; }

    public String getServerName() { return serverName; }
    public void setServerName(String serverName) { this.serverName = serverName; }

    public String getAuthToken() { return authToken; }
    public void setAuthToken(String authToken) { this.authToken = authToken; }

    public Map<String, String> getHeaders() { return headers; }
    public void setHeaders(Map<String, String> headers) { this.headers = headers; }

    public String getProviderLang() { return providerLang; }
    public void setProviderLang(String providerLang) { this.providerLang = providerLang; }

    public String getProviderSdk() { return providerSdk; }
    public void setProviderSdk(String providerSdk) { this.providerSdk = providerSdk; }

    public ReconnectConfig getReconnect() { return reconnect; }
    public void setReconnect(ReconnectConfig reconnect) { this.reconnect = reconnect; }

    public boolean isEnableFileTransfer() { return enableFileTransfer; }
    public void setEnableFileTransfer(boolean enableFileTransfer) { this.enableFileTransfer = enableFileTransfer; }

    public int getMaxFileSize() { return maxFileSize; }
    public void setMaxFileSize(int maxFileSize) { this.maxFileSize = maxFileSize; }

    public boolean isDisableLogging() { return disableLogging; }
    public void setDisableLogging(boolean disableLogging) { this.disableLogging = disableLogging; }

    public boolean isDebugLogging() { return debugLogging; }
    public void setDebugLogging(boolean debugLogging) { this.debugLogging = debugLogging; }

    public String getLogLevel() { return logLevel; }
    public void setLogLevel(String logLevel) { this.logLevel = logLevel; }

    @Override
    public String toString() {
        return String.format("ClientConfig{agentAddr='%s', gameId='%s', env='%s', serviceId='%s', serviceVersion='%s', insecure=%s, reconnect=%s, enableFileTransfer=%s}",
                agentAddr, gameId, env, serviceId, serviceVersion, insecure, reconnect, enableFileTransfer);
    }
}
