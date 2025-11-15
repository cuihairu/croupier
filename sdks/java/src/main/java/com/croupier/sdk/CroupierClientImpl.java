package com.croupier.sdk;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.stream.Collectors;

/**
 * Default implementation of CroupierClient
 */
public class CroupierClientImpl implements CroupierClient {
    private static final Logger logger = LoggerFactory.getLogger(CroupierClientImpl.class);

    private final ClientConfig config;
    private final Map<String, FunctionHandler> handlers = new ConcurrentHashMap<>();
    private final Map<String, FunctionDescriptor> descriptors = new ConcurrentHashMap<>();

    private final AtomicBoolean connected = new AtomicBoolean(false);
    private final AtomicBoolean serving = new AtomicBoolean(false);

    private GrpcManager grpcManager;
    private String sessionId;
    private String localAddress;

    public CroupierClientImpl(ClientConfig config) {
        this.config = config;
        validateConfig();
        logger.info("Initialized CroupierClient for game '{}' in '{}' environment",
                   config.getGameId(), config.getEnv());
    }

    @Override
    public void registerFunction(FunctionDescriptor descriptor, FunctionHandler handler) throws CroupierException {
        if (serving.get()) {
            throw new CroupierException("Cannot register functions while client is serving");
        }

        validateFunctionDescriptor(descriptor);

        handlers.put(descriptor.getId(), handler);
        descriptors.put(descriptor.getId(), descriptor);

        logger.info("Registered function: {} (version: {})", descriptor.getId(), descriptor.getVersion());
    }

    @Override
    public CompletableFuture<Void> connect() {
        if (connected.get()) {
            return CompletableFuture.completedFuture(null);
        }

        return CompletableFuture.supplyAsync(() -> {
            try {
                logger.info("🔌 Connecting to Croupier Agent: {}", config.getAgentAddr());

                // Initialize gRPC manager
                grpcManager = new GrpcManager(config, handlers);

                // Connect to agent
                grpcManager.connect();

                // Register functions with agent
                List<LocalFunctionDescriptor> localFunctions = convertToLocalFunctions();
                sessionId = grpcManager.registerWithAgent(
                    config.getServiceId(),
                    config.getServiceVersion(),
                    localFunctions
                );

                // Start local server
                localAddress = grpcManager.startLocalServer();

                connected.set(true);

                logger.info("✅ Successfully connected and registered with Agent");
                logger.info("📍 Local service address: {}", localAddress);
                logger.info("🔑 Session ID: {}", sessionId);

                return null;
            } catch (Exception e) {
                logger.error("❌ Connection failed", e);
                throw new RuntimeException(new CroupierException("Connection failed", e));
            }
        });
    }

    @Override
    public void serve() throws CroupierException {
        serveAsync().join();
    }

    @Override
    public CompletableFuture<Void> serveAsync() {
        if (!connected.get()) {
            return connect().thenCompose(v -> doServe());
        } else {
            return doServe();
        }
    }

    private CompletableFuture<Void> doServe() {
        return CompletableFuture.supplyAsync(() -> {
            try {
                serving.set(true);
                logger.info("🚀 Croupier client service started");
                logger.info("📍 Local service address: {}", localAddress);
                logger.info("🎯 Registered functions: {}", handlers.size());
                logger.info("💡 Use stop() method to stop the service");
                logger.info("===============================================");

                // Keep serving until stopped
                while (serving.get()) {
                    if (!grpcManager.isConnected()) {
                        logger.warn("⚠️ Connection to agent lost");
                        break;
                    }

                    try {
                        Thread.sleep(100); // Check every 100ms
                    } catch (InterruptedException e) {
                        Thread.currentThread().interrupt();
                        break;
                    }
                }

                serving.set(false);
                logger.info("🛑 Service has stopped");
                return null;
            } catch (Exception e) {
                serving.set(false);
                throw new RuntimeException(new CroupierException("Serving failed", e));
            }
        });
    }

    @Override
    public void stop() {
        serving.set(false);
        connected.set(false);

        logger.info("🛑 Stopping Croupier client...");

        if (grpcManager != null) {
            grpcManager.disconnect();
        }

        logger.info("✅ Client stopped successfully");
    }

    @Override
    public void close() {
        stop();
        handlers.clear();
        descriptors.clear();
    }

    @Override
    public String getLocalAddress() {
        return localAddress;
    }

    @Override
    public boolean isConnected() {
        return connected.get();
    }

    @Override
    public boolean isServing() {
        return serving.get();
    }

    private void validateConfig() {
        if (config.getGameId() == null || config.getGameId().trim().isEmpty()) {
            logger.warn("Warning: gameId is required for proper backend separation");
        }

        String env = config.getEnv();
        if (!"development".equals(env) && !"staging".equals(env) && !"production".equals(env)) {
            logger.warn("Warning: Unknown environment '{}'. Valid values: development, staging, production", env);
        }
    }

    private void validateFunctionDescriptor(FunctionDescriptor descriptor) throws CroupierException {
        if (descriptor.getId() == null || descriptor.getId().trim().isEmpty()) {
            throw new CroupierException("Function ID cannot be empty");
        }
        if (descriptor.getVersion() == null || descriptor.getVersion().trim().isEmpty()) {
            throw new CroupierException("Function version cannot be empty");
        }
    }

    private List<LocalFunctionDescriptor> convertToLocalFunctions() {
        return descriptors.values().stream()
                .map(desc -> new LocalFunctionDescriptor(desc.getId(), desc.getVersion()))
                .collect(Collectors.toList());
    }
}