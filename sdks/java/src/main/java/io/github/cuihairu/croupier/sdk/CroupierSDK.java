package io.github.cuihairu.croupier.sdk;

import io.github.cuihairu.croupier.sdk.invoker.InvokeOptions;
import io.github.cuihairu.croupier.sdk.invoker.Invoker;
import io.github.cuihairu.croupier.sdk.invoker.InvokerConfig;
import io.github.cuihairu.croupier.sdk.invoker.InvokerImpl;
import io.github.cuihairu.croupier.sdk.invoker.ServerHttpInvoker;

/**
 * Factory class for creating Croupier SDK instances
 */
public class CroupierSDK {

    /**
     * Create a new Croupier client with the provided configuration
     *
     * @param config Client configuration
     * @return CroupierClient instance
     */
    public static CroupierClient createClient(ClientConfig config) {
        return new CroupierClientImpl(config);
    }

    /**
     * Create a new Croupier client with default configuration
     *
     * @param gameId Game identifier
     * @param serviceId Service identifier
     * @return CroupierClient instance
     * @throws NullPointerException if gameId or serviceId is null
     */
    public static CroupierClient createClient(String gameId, String serviceId) {
        if (gameId == null) {
            throw new NullPointerException("gameId cannot be null");
        }
        if (serviceId == null) {
            throw new NullPointerException("serviceId cannot be null");
        }
        ClientConfig config = new ClientConfig(gameId, serviceId);
        return new CroupierClientImpl(config);
    }

    /**
     * Create a new Croupier client with minimal configuration
     *
     * @param gameId Game identifier
     * @param serviceId Service identifier
     * @param agentAddr Agent address
     * @return CroupierClient instance
     * @throws NullPointerException if gameId or serviceId is null
     */
    public static CroupierClient createClient(String gameId, String serviceId, String agentAddr) {
        if (gameId == null) {
            throw new NullPointerException("gameId cannot be null");
        }
        if (serviceId == null) {
            throw new NullPointerException("serviceId cannot be null");
        }
        ClientConfig config = new ClientConfig(gameId, serviceId);
        config.setAgentAddr(agentAddr);
        return new CroupierClientImpl(config);
    }

    /**
     * Create a new function descriptor builder
     *
     * @param id Function ID
     * @param version Function version
     * @return FunctionDescriptorBuilder instance
     */
    public static FunctionDescriptorBuilder functionDescriptor(String id, String version) {
        return new FunctionDescriptorBuilder(id, version);
    }

    // ========== Invoker Factory Methods ==========

    /**
     * Create a new Invoker with the provided configuration.
     *
     * @param config Invoker configuration
     * @return Invoker instance
     * @throws NullPointerException if config is null
     */
    public static Invoker createInvoker(InvokerConfig config) {
        if (config == null) {
            throw new NullPointerException("config cannot be null");
        }
        return new ServerHttpInvoker(config);
    }

    /**
     * Create a new Invoker with default configuration.
     *
     * @return Invoker instance with default config
     */
    public static Invoker createInvoker() {
        return new ServerHttpInvoker(InvokerConfig.createDefault());
    }

    /**
     * Create a new Invoker with a custom server address.
     *
     * @param address the Server HTTP API address, root URL, or "host:port"
     * @return Invoker instance
     * @throws NullPointerException if address is null
     * @throws IllegalArgumentException if address is empty
     */
    public static Invoker createInvoker(String address) {
        if (address == null) {
            throw new NullPointerException("address cannot be null");
        }
        if (address.isEmpty()) {
            throw new IllegalArgumentException("address cannot be empty");
        }
        InvokerConfig config = InvokerConfig.builder()
            .address(address)
            .build();
        return new ServerHttpInvoker(config);
    }

    /**
     * Create a new InvokeOptions builder.
     *
     * @return InvokeOptions builder
     */
    public static InvokeOptions.Builder invokeOptions() {
        return InvokeOptions.builder();
    }

    /**
     * Builder for function descriptors
     */
    public static class FunctionDescriptorBuilder {
        private final FunctionDescriptor descriptor;

        private FunctionDescriptorBuilder(String id, String version) {
            this.descriptor = new FunctionDescriptor(id, version);
        }

        public FunctionDescriptorBuilder resource(String resource) {
            descriptor.setResource(resource);
            return this;
        }

        public FunctionDescriptorBuilder tags(java.util.List<String> tags) {
            descriptor.setTags(tags);
            return this;
        }

        public FunctionDescriptorBuilder summary(String summary) {
            descriptor.setSummary(summary);
            return this;
        }

        public FunctionDescriptorBuilder description(String description) {
            descriptor.setDescription(description);
            return this;
        }

        public FunctionDescriptorBuilder operationId(String operationId) {
            descriptor.setOperationId(operationId);
            return this;
        }

        public FunctionDescriptorBuilder deprecated(boolean deprecated) {
            descriptor.setDeprecated(deprecated);
            return this;
        }

        public FunctionDescriptorBuilder inputSchema(String inputSchema) {
            descriptor.setInputSchema(inputSchema);
            return this;
        }

        public FunctionDescriptorBuilder outputSchema(String outputSchema) {
            descriptor.setOutputSchema(outputSchema);
            return this;
        }

        public FunctionDescriptorBuilder risk(String risk) {
            descriptor.setRisk(risk);
            return this;
        }

        public FunctionDescriptorBuilder operation(String operation) {
            descriptor.setOperation(operation);
            return this;
        }

        public FunctionDescriptorBuilder capability(String capability) {
            descriptor.setCapability(capability);
            return this;
        }

        public FunctionDescriptorBuilder execution(String execution) {
            descriptor.setExecution(execution);
            return this;
        }

        public FunctionDescriptorBuilder permission(String permission) {
            descriptor.setPermission(permission);
            return this;
        }

        public FunctionDescriptorBuilder enabled(boolean enabled) {
            descriptor.setEnabled(enabled);
            return this;
        }

        public FunctionDescriptor build() {
            // Return a copy to ensure immutability
            return new FunctionDescriptor(descriptor);
        }
    }
}
