package io.github.cuihairu.croupier.sdk;

import io.github.cuihairu.croupier.sdk.invoker.InvokeOptions;
import io.github.cuihairu.croupier.sdk.invoker.Invoker;
import io.github.cuihairu.croupier.sdk.invoker.InvokerConfig;
import io.github.cuihairu.croupier.sdk.invoker.InvokerImpl;

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
     */
    public static CroupierClient createClient(String gameId, String serviceId) {
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
     */
    public static CroupierClient createClient(String gameId, String serviceId, String agentAddr) {
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
     */
    public static Invoker createInvoker(InvokerConfig config) {
        return new InvokerImpl(config);
    }

    /**
     * Create a new Invoker with default configuration.
     *
     * @return Invoker instance with default config
     */
    public static Invoker createInvoker() {
        return new InvokerImpl(InvokerConfig.createDefault());
    }

    /**
     * Create a new Invoker with a custom server address.
     *
     * @param address the server address in "host:port" format
     * @return Invoker instance
     */
    public static Invoker createInvoker(String address) {
        InvokerConfig config = InvokerConfig.builder()
            .address(address)
            .build();
        return new InvokerImpl(config);
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

        public FunctionDescriptorBuilder category(String category) {
            descriptor.setCategory(category);
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

        public FunctionDescriptorBuilder entity(String entity) {
            descriptor.setEntity(entity);
            return this;
        }

        public FunctionDescriptorBuilder operation(String operation) {
            descriptor.setOperation(operation);
            return this;
        }

        public FunctionDescriptorBuilder enabled(boolean enabled) {
            descriptor.setEnabled(enabled);
            return this;
        }

        public FunctionDescriptor build() {
            return descriptor;
        }
    }
}
