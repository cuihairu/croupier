package com.croupier.sdk;

/**
 * Configuration for Croupier client
 */
public class ClientConfig {
    // Agent connection settings
    private String agentAddr = "localhost:19090"; // Agent gRPC address

    // Service identification (multi-tenant support)
    private String gameId;          // game identifier for tenant isolation
    private String env = "development"; // environment: "development"|"staging"|"production"
    private String serviceId;       // unique service identifier
    private String serviceVersion = "1.0.0"; // service version for compatibility
    private String agentId;         // agent identifier for load balancing

    // Local server settings
    private String localListen;     // local gRPC listener address

    // Connection settings
    private int timeoutSeconds = 30; // connection timeout in seconds
    private boolean insecure = true; // use insecure gRPC (for development)

    // TLS settings (when not insecure)
    private String caFile;   // CA certificate file path
    private String certFile; // client certificate file path
    private String keyFile;  // client private key file path

    public ClientConfig() {}

    public ClientConfig(String gameId, String serviceId) {
        this.gameId = gameId;
        this.serviceId = serviceId;
    }

    // Getters and setters
    public String getAgentAddr() { return agentAddr; }
    public void setAgentAddr(String agentAddr) { this.agentAddr = agentAddr; }

    public String getGameId() { return gameId; }
    public void setGameId(String gameId) { this.gameId = gameId; }

    public String getEnv() { return env; }
    public void setEnv(String env) { this.env = env; }

    public String getServiceId() { return serviceId; }
    public void setServiceId(String serviceId) { this.serviceId = serviceId; }

    public String getServiceVersion() { return serviceVersion; }
    public void setServiceVersion(String serviceVersion) { this.serviceVersion = serviceVersion; }

    public String getAgentId() { return agentId; }
    public void setAgentId(String agentId) { this.agentId = agentId; }

    public String getLocalListen() { return localListen; }
    public void setLocalListen(String localListen) { this.localListen = localListen; }

    public int getTimeoutSeconds() { return timeoutSeconds; }
    public void setTimeoutSeconds(int timeoutSeconds) { this.timeoutSeconds = timeoutSeconds; }

    public boolean isInsecure() { return insecure; }
    public void setInsecure(boolean insecure) { this.insecure = insecure; }

    public String getCaFile() { return caFile; }
    public void setCaFile(String caFile) { this.caFile = caFile; }

    public String getCertFile() { return certFile; }
    public void setCertFile(String certFile) { this.certFile = certFile; }

    public String getKeyFile() { return keyFile; }
    public void setKeyFile(String keyFile) { this.keyFile = keyFile; }

    @Override
    public String toString() {
        return String.format("ClientConfig{agentAddr='%s', gameId='%s', env='%s', serviceId='%s', serviceVersion='%s', insecure=%s}",
                agentAddr, gameId, env, serviceId, serviceVersion, insecure);
    }
}