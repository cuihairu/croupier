package com.croupier.sdk;

import com.croupier.control.v1.ControlServiceGrpc;
import com.croupier.control.v1.ProviderMeta;
import com.croupier.control.v1.RegisterCapabilitiesRequest;
import com.google.protobuf.ByteString;
import io.grpc.ManagedChannel;
import io.grpc.netty.shaded.io.grpc.netty.NettyChannelBuilder;
import io.grpc.netty.shaded.io.netty.handler.ssl.SslContext;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.stream.Collectors;
import java.util.zip.GZIPOutputStream;

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

                // Start local server
                localAddress = grpcManager.startLocalServer();

                // Register functions with agent
                List<LocalFunctionDescriptor> localFunctions = convertToLocalFunctions();
                sessionId = grpcManager.registerWithAgent(
                    config.getServiceId(),
                    config.getServiceVersion(),
                    localFunctions
                );

                connected.set(true);

                logger.info("✅ Successfully connected and registered with Agent");
                logger.info("📍 Local service address: {}", localAddress);
                logger.info("🔑 Session ID: {}", sessionId);

                if (!isNullOrEmpty(config.getControlAddr())) {
                    try {
                        registerCapabilitiesWithControlPlane();
                    } catch (Exception e) {
                        logger.warn("⚠️ Failed to upload provider capabilities", e);
                    }
                }

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

    private void registerCapabilitiesWithControlPlane() throws Exception {
        byte[] manifest = buildManifest();
        byte[] compressed = gzip(manifest);

        NettyChannelBuilder builder = NettyChannelBuilder.forTarget(config.getControlAddr());
        if (config.isInsecure()) {
            builder.usePlaintext();
        } else {
            if (grpcManager == null) {
                throw new CroupierException("gRPC manager not initialized, cannot configure TLS");
            }
            SslContext sslContext = grpcManager.buildClientSslContext();
            builder.sslContext(sslContext);
        }

        builder.keepAliveTime(30, TimeUnit.SECONDS)
               .keepAliveTimeout(5, TimeUnit.SECONDS)
               .keepAliveWithoutCalls(true);

        ManagedChannel controlChannel = builder.build();
        try {
            ControlServiceGrpc.ControlServiceBlockingStub stub = ControlServiceGrpc.newBlockingStub(controlChannel);
            if (config.getTimeoutSeconds() > 0) {
                stub = stub.withDeadlineAfter(config.getTimeoutSeconds(), TimeUnit.SECONDS);
            }

            RegisterCapabilitiesRequest request = RegisterCapabilitiesRequest.newBuilder()
                    .setProvider(ProviderMeta.newBuilder()
                            .setId(defaultValue(config.getServiceId(), "java-service"))
                            .setVersion(defaultVersion(config.getServiceVersion()))
                            .setLang(defaultValue(config.getProviderLang(), "java"))
                            .setSdk(defaultValue(config.getProviderSdk(), "croupier-java-sdk"))
                            .build())
                    .setManifestJsonGz(ByteString.copyFrom(compressed))
                    .build();

            stub.registerCapabilities(request);
            logger.info("📤 Uploaded provider capabilities manifest ({} functions)", descriptors.size());
        } finally {
            controlChannel.shutdown();
            try {
                if (!controlChannel.awaitTermination(5, TimeUnit.SECONDS)) {
                    controlChannel.shutdownNow();
                }
            } catch (InterruptedException e) {
                controlChannel.shutdownNow();
                Thread.currentThread().interrupt();
                throw new CroupierException("Interrupted while closing control channel", e);
            }
        }
    }

    private byte[] buildManifest() {
        StringBuilder builder = new StringBuilder();
        builder.append("{\"provider\":{");
        builder.append("\"id\":\"").append(escapeJson(defaultValue(config.getServiceId(), "java-service"))).append("\",");
        builder.append("\"version\":\"").append(escapeJson(defaultVersion(config.getServiceVersion()))).append("\",");
        builder.append("\"lang\":\"").append(escapeJson(defaultValue(config.getProviderLang(), "java"))).append("\",");
        builder.append("\"sdk\":\"").append(escapeJson(defaultValue(config.getProviderSdk(), "croupier-java-sdk"))).append("\"}");

        List<FunctionDescriptor> snapshot = descriptors.values().stream().collect(Collectors.toList());
        StringBuilder functionsBuilder = new StringBuilder();
        boolean first = true;
        for (FunctionDescriptor descriptor : snapshot) {
            if (descriptor == null || isNullOrEmpty(descriptor.getId())) {
                continue;
            }
            if (first) {
                functionsBuilder.append("[");
                first = false;
            } else {
                functionsBuilder.append(",");
            }
            functionsBuilder.append("{");
            functionsBuilder.append("\"id\":\"").append(escapeJson(descriptor.getId())).append("\"");
            functionsBuilder.append(",\"version\":\"").append(escapeJson(defaultVersion(descriptor.getVersion()))).append("\"");
            if (!isNullOrEmpty(descriptor.getCategory())) {
                functionsBuilder.append(",\"category\":\"").append(escapeJson(descriptor.getCategory())).append("\"");
            }
            if (!isNullOrEmpty(descriptor.getRisk())) {
                functionsBuilder.append(",\"risk\":\"").append(escapeJson(descriptor.getRisk())).append("\"");
            }
            if (!isNullOrEmpty(descriptor.getEntity())) {
                functionsBuilder.append(",\"entity\":\"").append(escapeJson(descriptor.getEntity())).append("\"");
            }
            if (!isNullOrEmpty(descriptor.getOperation())) {
                functionsBuilder.append(",\"operation\":\"").append(escapeJson(descriptor.getOperation())).append("\"");
            }
            if (descriptor.isEnabled()) {
                functionsBuilder.append(",\"enabled\":true");
            }
            functionsBuilder.append("}");
        }
        if (!first) {
            functionsBuilder.append("]");
            builder.append(",\"functions\":").append(functionsBuilder);
        }

        builder.append("}");
        return builder.toString().getBytes(StandardCharsets.UTF_8);
    }

    private String escapeJson(String value) {
        if (value == null) {
            return "";
        }
        StringBuilder out = new StringBuilder(value.length() + 16);
        for (int i = 0; i < value.length(); i++) {
            char ch = value.charAt(i);
            switch (ch) {
                case '"':
                    out.append("\\\"");
                    break;
                case '\\':
                    out.append("\\\\");
                    break;
                case '\b':
                    out.append("\\b");
                    break;
                case '\f':
                    out.append("\\f");
                    break;
                case '\n':
                    out.append("\\n");
                    break;
                case '\r':
                    out.append("\\r");
                    break;
                case '\t':
                    out.append("\\t");
                    break;
                default:
                    if (ch < 0x20) {
                        out.append(String.format("\\u%04x", (int) ch));
                    } else {
                        out.append(ch);
                    }
            }
        }
        return out.toString();
    }

    private String defaultValue(String value, String fallback) {
        return isNullOrEmpty(value) ? fallback : value;
    }

    private String defaultVersion(String version) {
        return isNullOrEmpty(version) ? "1.0.0" : version;
    }

    private byte[] gzip(byte[] payload) throws IOException {
        ByteArrayOutputStream output = new ByteArrayOutputStream();
        try (GZIPOutputStream gzip = new GZIPOutputStream(output)) {
            gzip.write(payload);
        }
        return output.toByteArray();
    }

    private boolean isNullOrEmpty(String value) {
        return value == null || value.trim().isEmpty();
    }
}
