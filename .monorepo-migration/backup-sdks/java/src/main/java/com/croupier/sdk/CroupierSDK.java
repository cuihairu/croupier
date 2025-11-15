package com.croupier.sdk;

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