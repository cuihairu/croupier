package com.croupier.sdk;

import io.grpc.*;
import io.grpc.netty.shaded.io.grpc.netty.NettyChannelBuilder;
import io.grpc.netty.shaded.io.grpc.netty.NettyServerBuilder;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.io.IOException;
import java.net.InetSocketAddress;
import java.util.List;
import java.util.Map;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicBoolean;

/**
 * gRPC manager for handling agent communication and local server
 */
public class GrpcManager {
    private static final Logger logger = LoggerFactory.getLogger(GrpcManager.class);

    private final ClientConfig config;
    private final Map<String, FunctionHandler> handlers;

    private ManagedChannel channel;
    private Server localServer;
    private final AtomicBoolean connected = new AtomicBoolean(false);

    private String localAddress;

    public GrpcManager(ClientConfig config, Map<String, FunctionHandler> handlers) {
        this.config = config;
        this.handlers = handlers;
    }

    /**
     * Connect to the agent
     */
    public void connect() throws CroupierException {
        if (connected.get()) {
            return;
        }

        try {
            logger.info("Creating gRPC channel to agent: {}", config.getAgentAddr());

            // Create channel builder
            NettyChannelBuilder channelBuilder = NettyChannelBuilder.forTarget(config.getAgentAddr());

            if (config.isInsecure()) {
                channelBuilder.usePlaintext();
            } else {
                // TODO: Implement TLS credentials
                throw new CroupierException("TLS not implemented yet");
            }

            // Set timeouts and other options
            channelBuilder
                .keepAliveTime(30, TimeUnit.SECONDS)
                .keepAliveTimeout(5, TimeUnit.SECONDS)
                .keepAliveWithoutCalls(true)
                .maxInboundMessageSize(4 * 1024 * 1024) // 4MB
                .maxInboundMetadataSize(8 * 1024);      // 8KB

            channel = channelBuilder.build();

            // Wait for channel to be ready
            if (!channel.awaitTermination(config.getTimeoutSeconds(), TimeUnit.SECONDS)) {
                // Channel is ready
            }

            connected.set(true);
            logger.info("✅ Successfully connected to Agent: {}", config.getAgentAddr());

        } catch (Exception e) {
            throw new CroupierException("Failed to connect to agent", e);
        }
    }

    /**
     * Register functions with agent
     */
    public String registerWithAgent(String serviceId, String serviceVersion, List<LocalFunctionDescriptor> functions) throws CroupierException {
        if (!connected.get()) {
            throw new CroupierException("Not connected to agent");
        }

        try {
            // This is a mock implementation - in a real implementation, this would:
            // 1. Use generated gRPC client stub from local.proto
            // 2. Call the LocalControlService.RegisterLocal RPC
            // 3. Handle the actual proto message marshaling

            String sessionId = String.format("mock_session_%s_%d", serviceId, System.currentTimeMillis());

            logger.info("📡 Registering service with Agent:");
            logger.info("   Service ID: {}", serviceId);
            logger.info("   Version: {}", serviceVersion);
            logger.info("   Local Address: {}", localAddress);
            logger.info("   Functions: {}", functions.size());
            for (LocalFunctionDescriptor func : functions) {
                logger.info("     - {} (v{})", func.getId(), func.getVersion());
            }
            logger.info("   Session ID: {}", sessionId);

            return sessionId;

        } catch (Exception e) {
            throw new CroupierException("Failed to register with agent", e);
        }
    }

    /**
     * Start local gRPC server
     */
    public String startLocalServer() throws CroupierException {
        try {
            // Parse listen address
            String listenAddr = config.getLocalListen();
            int port = 0; // Auto-assign port

            if (listenAddr != null && !listenAddr.isEmpty()) {
                String[] parts = listenAddr.split(":");
                if (parts.length == 2) {
                    port = Integer.parseInt(parts[1]);
                }
            }

            // Create server builder
            NettyServerBuilder serverBuilder = NettyServerBuilder.forPort(port);

            if (!config.isInsecure()) {
                // TODO: Implement TLS credentials
                throw new CroupierException("Server TLS not implemented yet");
            }

            // Add function service
            // In a real implementation, this would register the generated gRPC service
            // from function.proto: serverBuilder.addService(new FunctionServiceImpl(handlers));

            localServer = serverBuilder.build();
            localServer.start();

            // Get actual port
            int actualPort = localServer.getPort();
            localAddress = String.format("localhost:%d", actualPort);

            logger.info("🚀 Local gRPC server started on: {}", localAddress);

            // Add shutdown hook
            Runtime.getRuntime().addShutdownHook(new Thread(() -> {
                logger.info("Shutting down gRPC server");
                stopLocalServer();
            }));

            return localAddress;

        } catch (IOException e) {
            throw new CroupierException("Failed to start local server", e);
        }
    }

    /**
     * Stop local server
     */
    public void stopLocalServer() {
        if (localServer != null) {
            localServer.shutdown();
            try {
                if (!localServer.awaitTermination(5, TimeUnit.SECONDS)) {
                    localServer.shutdownNow();
                }
            } catch (InterruptedException e) {
                localServer.shutdownNow();
                Thread.currentThread().interrupt();
            }
            localServer = null;
            logger.info("🛑 Local gRPC server stopped");
        }
    }

    /**
     * Disconnect from agent
     */
    public void disconnect() {
        connected.set(false);

        stopLocalServer();

        if (channel != null) {
            channel.shutdown();
            try {
                if (!channel.awaitTermination(5, TimeUnit.SECONDS)) {
                    channel.shutdownNow();
                }
            } catch (InterruptedException e) {
                channel.shutdownNow();
                Thread.currentThread().interrupt();
            }
            channel = null;
            logger.info("📴 Disconnected from Agent");
        }
    }

    /**
     * Check if connected to agent
     */
    public boolean isConnected() {
        return connected.get() && channel != null && !channel.isShutdown();
    }

    /**
     * Get local server address
     */
    public String getLocalAddress() {
        return localAddress;
    }
}