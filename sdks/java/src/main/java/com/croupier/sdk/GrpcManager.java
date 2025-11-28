package com.croupier.sdk;

import croupier.agent.local.v1.Local;
import croupier.agent.local.v1.LocalControlServiceGrpc;
import io.grpc.ManagedChannel;
import io.grpc.Server;
import io.grpc.netty.shaded.io.grpc.netty.GrpcSslContexts;
import io.grpc.netty.shaded.io.grpc.netty.NettyChannelBuilder;
import io.grpc.netty.shaded.io.grpc.netty.NettyServerBuilder;
import io.grpc.netty.shaded.io.netty.handler.ssl.ClientAuth;
import io.grpc.netty.shaded.io.netty.handler.ssl.SslContext;
import java.io.File;
import java.io.IOException;
import java.net.InetSocketAddress;
import java.util.List;
import java.util.Map;
import java.util.concurrent.Executors;
import java.util.concurrent.ScheduledExecutorService;
import java.util.concurrent.ScheduledFuture;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicBoolean;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * gRPC manager for handling agent communication and local server
 */
public class GrpcManager {
    private static final Logger logger = LoggerFactory.getLogger(GrpcManager.class);

    private final ClientConfig config;
    private final Map<String, FunctionHandler> handlers;

    private ManagedChannel channel;
    private LocalControlServiceGrpc.LocalControlServiceBlockingStub localControl;
    private Server localServer;
    private FunctionServiceImpl functionService;
    private final AtomicBoolean connected = new AtomicBoolean(false);
    private ScheduledExecutorService heartbeatExecutor;
    private ScheduledFuture<?> heartbeatTask;
    private String sessionId;
    private String localAddress;
    private String currentServiceId;

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
                channelBuilder.sslContext(createClientSslContext());
            }

            // Set timeouts and other options
            channelBuilder
                .keepAliveTime(30, TimeUnit.SECONDS)
                .keepAliveTimeout(5, TimeUnit.SECONDS)
                .keepAliveWithoutCalls(true)
                .maxInboundMessageSize(4 * 1024 * 1024) // 4MB
                .maxInboundMetadataSize(8 * 1024);      // 8KB

            channel = channelBuilder.build();
            channel.getState(true); // request connection
            localControl = LocalControlServiceGrpc.newBlockingStub(channel);

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
        if (localControl == null) {
            throw new CroupierException("Local control client unavailable");
        }
        if (localAddress == null || localAddress.isEmpty()) {
            throw new CroupierException("Local server address missing; start server before registering");
        }

        try {
            Local.RegisterLocalRequest.Builder builder = Local.RegisterLocalRequest.newBuilder()
                .setServiceId(serviceId)
                .setVersion(serviceVersion)
                .setRpcAddr(localAddress);
            if (functions != null) {
                for (LocalFunctionDescriptor func : functions) {
                    if (func.getId() == null || func.getId().isEmpty()) {
                        continue;
                    }
                    builder.addFunctions(Local.LocalFunctionDescriptor.newBuilder()
                        .setId(func.getId())
                        .setVersion(func.getVersion() == null ? "" : func.getVersion()));
                }
            }

            Local.RegisterLocalResponse resp = localControl.registerLocal(builder.build());
            String newSessionId = resp.getSessionId();
            if (newSessionId == null || newSessionId.isEmpty()) {
                throw new CroupierException("Agent returned empty session ID");
            }

            this.sessionId = newSessionId;
            this.currentServiceId = serviceId;
            startHeartbeat();

            logger.info("📡 Registered service with Agent");
            logger.info("   Service ID: {}", serviceId);
            logger.info("   Version: {}", serviceVersion);
            logger.info("   Local Address: {}", localAddress);
            logger.info("   Functions: {}", builder.getFunctionsCount());
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
            if (localServer != null) {
                return localAddress;
            }

            // Parse listen address
            String listenAddr = config.getLocalListen();
            InetSocketAddress socketAddress = parseListenAddress(listenAddr);
            String advertisedHost = socketAddress.getHostString();
            if (advertisedHost == null || advertisedHost.equals("0.0.0.0")) {
                advertisedHost = "127.0.0.1";
            }

            // Create server builder
            NettyServerBuilder serverBuilder = NettyServerBuilder.forAddress(socketAddress);
            functionService = new FunctionServiceImpl(handlers);

            if (!config.isInsecure()) {
                serverBuilder.sslContext(createServerSslContext());
            }

            serverBuilder.addService(functionService);

            localServer = serverBuilder.build();
            localServer.start();

            // Get actual port
            int actualPort = localServer.getPort();
            localAddress = String.format("%s:%d", advertisedHost, actualPort);

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
            if (functionService != null) {
                functionService.shutdown();
                functionService = null;
            }
            logger.info("🛑 Local gRPC server stopped");
        }
    }

    /**
     * Disconnect from agent
     */
    public void disconnect() {
        connected.set(false);

        stopHeartbeat();
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
            localControl = null;
            sessionId = null;
            currentServiceId = null;
            localAddress = null;
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

    /**
     * Expose the TLS client context so other components (e.g. control-plane dialers)
     * can reuse the same certificate loading logic.
     */
    public SslContext buildClientSslContext() throws CroupierException {
        return createClientSslContext();
    }

    private void startHeartbeat() {
        stopHeartbeat();
        if (heartbeatExecutor == null) {
            heartbeatExecutor = Executors.newSingleThreadScheduledExecutor(r -> {
                Thread t = new Thread(r, "croupier-heartbeat");
                t.setDaemon(true);
                return t;
            });
        }
        heartbeatTask = heartbeatExecutor.scheduleAtFixedRate(() -> {
            try {
                if (localControl != null && sessionId != null && currentServiceId != null) {
                    localControl.heartbeat(
                        Local.HeartbeatRequest.newBuilder()
                            .setServiceId(currentServiceId)
                            .setSessionId(sessionId)
                            .build()
                    );
                }
            } catch (Exception e) {
                logger.warn("Heartbeat failed: {}", e.toString());
            }
        }, 5, 25, TimeUnit.SECONDS);
    }

    private void stopHeartbeat() {
        if (heartbeatTask != null) {
            heartbeatTask.cancel(true);
            heartbeatTask = null;
        }
        if (heartbeatExecutor != null) {
            heartbeatExecutor.shutdownNow();
            heartbeatExecutor = null;
        }
    }

    private InetSocketAddress parseListenAddress(String listenAddr) {
        if (listenAddr == null || listenAddr.isEmpty()) {
            return new InetSocketAddress("0.0.0.0", 0);
        }
        String host = "0.0.0.0";
        int port = 0;
        String[] parts = listenAddr.split(":");
        if (parts.length == 2) {
            host = parts[0].isEmpty() ? "0.0.0.0" : parts[0];
            port = Integer.parseInt(parts[1]);
        } else if (parts.length == 1) {
            port = Integer.parseInt(parts[0]);
        }
        return new InetSocketAddress(host, port);
    }

    private SslContext createClientSslContext() throws CroupierException {
        try {
            io.grpc.netty.shaded.io.netty.handler.ssl.SslContextBuilder builder = GrpcSslContexts.forClient();

            if (!isNullOrEmpty(config.getCaFile())) {
                builder.trustManager(new File(config.getCaFile()));
            }

            if (!isNullOrEmpty(config.getCertFile()) && !isNullOrEmpty(config.getKeyFile())) {
                builder.keyManager(new File(config.getCertFile()), new File(config.getKeyFile()));
            }

            return builder.build();
        } catch (Exception e) {
            throw new CroupierException("Failed to create gRPC client TLS context", e);
        }
    }

    private SslContext createServerSslContext() throws CroupierException {
        if (isNullOrEmpty(config.getCertFile()) || isNullOrEmpty(config.getKeyFile())) {
            throw new CroupierException("Server TLS requires certFile and keyFile");
        }

        try {
            io.grpc.netty.shaded.io.netty.handler.ssl.SslContextBuilder builder =
                GrpcSslContexts.forServer(new File(config.getCertFile()), new File(config.getKeyFile()));

            if (!isNullOrEmpty(config.getCaFile())) {
                builder.trustManager(new File(config.getCaFile()));
                builder.clientAuth(ClientAuth.REQUIRE);
            } else {
                builder.clientAuth(ClientAuth.NONE);
            }

            return builder.build();
        } catch (Exception e) {
            throw new CroupierException("Failed to create gRPC server TLS context", e);
        }
    }

    private boolean isNullOrEmpty(String value) {
        return value == null || value.isEmpty();
    }
}
