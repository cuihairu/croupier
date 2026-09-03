package io.github.cuihairu.croupier.sdk;

import java.util.HashMap;
import java.util.Map;

/**
 * Configuration for Croupier client
 */
public class ClientConfig {
    // ========== Agent Connection Settings ==========
    private String agentAddr = "127.0.0.1:19091"; // Agent local SDK gateway address
    private String agentId;                       // Agent unique identifier (auto-generated if empty)

    // ========== Service Identification (single-company, multi-game scope) ==========
    private String gameId;          // game identifier for business scope isolation
    private String env = "development"; // logical environment: "development"|"staging"|"production"
    private String serviceId;       // unique service identifier
    private String serviceVersion = "1.0.0"; // service version for compatibility

    // ========== Control Plane Settings ==========
    private String controlAddr;     // optional control-plane addr for manifest upload

    // ========== Connection Settings ==========
    private int timeoutSeconds = 30; // connection timeout in seconds
    private boolean insecure = true; // use insecure connection (for development)
    // 入站调用 worker 数。0（默认）= max(2, CPU)；1 = 串行模式：handler
    // 顺序执行互不并发——单线程游戏服务器应设为 1。
    private int inboundWorkers = 0;

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
    // Defaults to an enabled config (infinite attempts, 1s→30s backoff) so a
    // plain `new ClientConfig()` recovers after the agent restarts.
    private ReconnectConfig reconnect = ReconnectConfig.builder().build();

    // ========== File Transfer Configuration ==========
    private boolean enableFileTransfer = false;  // Enable file transfer functionality (default: false)
    private boolean validateInputPayloads = false; // F：provider 侧入站校验（按函数声明 input schema），默认关闭
    private int maxFileSize = 10485760;         // Max file size in bytes (default: 10MB)
    private String fileStagingDir = "./croupier-staging"; // F：下发文件仅落盘至此（不自动应用）

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
    public int getInboundWorkers() { return inboundWorkers; }
    public void setInboundWorkers(int inboundWorkers) { this.inboundWorkers = inboundWorkers; }
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

    public boolean isValidateInputPayloads() { return validateInputPayloads; }
    public void setValidateInputPayloads(boolean validateInputPayloads) { this.validateInputPayloads = validateInputPayloads; }

    public int getMaxFileSize() { return maxFileSize; }
    public void setMaxFileSize(int maxFileSize) { this.maxFileSize = maxFileSize; }

    public String getFileStagingDir() { return fileStagingDir; }
    public void setFileStagingDir(String fileStagingDir) { this.fileStagingDir = fileStagingDir; }

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
