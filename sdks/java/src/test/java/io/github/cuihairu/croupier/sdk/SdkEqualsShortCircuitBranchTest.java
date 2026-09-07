package io.github.cuihairu.croupier.sdk;

import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotEquals;

/** Short-circuit-safe equals coverage for sdk-level value objects. */
class SdkEqualsShortCircuitBranchTest {

    private static ProviderFunctionDescriptor like(ProviderFunctionDescriptor base, String id,
        String version, String capability, String execution) {
        return new ProviderFunctionDescriptor(
            id != null ? id : base.getId(),
            version != null ? version : base.getVersion(),
            capability != null ? capability : base.getCapability(),
            execution != null ? execution : base.getExecution());
    }

    @Test
    void providerFunctionDescriptorEveryFieldDivergenceIsCompared() {
        ProviderFunctionDescriptor base =
            new ProviderFunctionDescriptor("fn-1", "v1", "http", "async");

        assertEquals(base, like(base, null, null, null, null));

        assertNotEquals(base, like(base, "other", null, null, null));
        assertNotEquals(base, like(base, null, "v2", null, null));
        assertNotEquals(base, like(base, null, null, "grpc", null));
        assertNotEquals(base, like(base, null, null, null, "sync"));
    }

    @Test
    void sdkReconnectConfigEveryFieldDivergenceIsCompared() {
        ReconnectConfig base = ReconnectConfig.builder()
            .enabled(true).maxAttempts(4).initialDelayMs(11).maxDelayMs(111)
            .backoffMultiplier(2.5).jitterFactor(0.25)
            .build();

        assertEquals(base, ReconnectConfig.builder()
            .enabled(true).maxAttempts(4).initialDelayMs(11).maxDelayMs(111)
            .backoffMultiplier(2.5).jitterFactor(0.25)
            .build());

        assertNotEquals(base, ReconnectConfig.builder()
            .enabled(true).maxAttempts(4).initialDelayMs(11).maxDelayMs(111)
            .backoffMultiplier(2.5).jitterFactor(0.25).enabled(false).build());
        assertNotEquals(base, ReconnectConfig.builder()
            .enabled(true).maxAttempts(5).initialDelayMs(11).maxDelayMs(111)
            .backoffMultiplier(2.5).jitterFactor(0.25).build());
        assertNotEquals(base, ReconnectConfig.builder()
            .enabled(true).maxAttempts(4).initialDelayMs(12).maxDelayMs(111)
            .backoffMultiplier(2.5).jitterFactor(0.25).build());
        assertNotEquals(base, ReconnectConfig.builder()
            .enabled(true).maxAttempts(4).initialDelayMs(11).maxDelayMs(112)
            .backoffMultiplier(2.5).jitterFactor(0.25).build());
        assertNotEquals(base, ReconnectConfig.builder()
            .enabled(true).maxAttempts(4).initialDelayMs(11).maxDelayMs(111)
            .backoffMultiplier(2.6).jitterFactor(0.25).build());
        assertNotEquals(base, ReconnectConfig.builder()
            .enabled(true).maxAttempts(4).initialDelayMs(11).maxDelayMs(111)
            .backoffMultiplier(2.5).jitterFactor(0.26).build());
    }
}
