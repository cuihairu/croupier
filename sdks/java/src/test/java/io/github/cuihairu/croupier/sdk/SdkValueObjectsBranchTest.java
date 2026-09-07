package io.github.cuihairu.croupier.sdk;

import io.github.cuihairu.croupier.sdk.invoker.RetryConfig;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;

/** Branch-complete equals coverage for sdk-level value objects. */
class SdkValueObjectsBranchTest {

    @Test
    void providerFunctionDescriptorEqualsCoversEveryFieldBranch() {
        ProviderFunctionDescriptor base =
            new ProviderFunctionDescriptor("fn-1", "v1", "http", "async");

        assertEquals(base, base);
        assertNotEquals(base, null);
        assertNotEquals(base, new Object());
        assertEquals(base, new ProviderFunctionDescriptor("fn-1", "v1", "http", "async"));
        assertEquals(base.hashCode(), new ProviderFunctionDescriptor("fn-1", "v1", "http", "async").hashCode());
        assertNotNull(base.toString());
        assertNotNull(new ProviderFunctionDescriptor().toString());
        assertNotNull(new ProviderFunctionDescriptor("id", "version").toString());

        assertNotEquals(base, new ProviderFunctionDescriptor("other", "v1", "http", "async"));
        assertNotEquals(base, new ProviderFunctionDescriptor("fn-1", "v2", "http", "async"));
        assertNotEquals(base, new ProviderFunctionDescriptor("fn-1", "v1", "grpc", "async"));
        assertNotEquals(base, new ProviderFunctionDescriptor("fn-1", "v1", "http", "sync"));

        // default constructed vs default constructed equality path
        assertEquals(new ProviderFunctionDescriptor(), new ProviderFunctionDescriptor());
    }

    @Test
    void sdkReconnectConfigEqualsCoversEveryFieldBranch() {
        ReconnectConfig base = ReconnectConfig.builder()
            .enabled(true).maxAttempts(3).initialDelayMs(10).maxDelayMs(1000)
            .backoffMultiplier(1.5).jitterFactor(0.1)
            .build();

        assertEquals(base, base);
        assertNotEquals(base, null);
        assertNotEquals(base, new Object());
        assertEquals(base, ReconnectConfig.builder()
            .enabled(true).maxAttempts(3).initialDelayMs(10).maxDelayMs(1000)
            .backoffMultiplier(1.5).jitterFactor(0.1)
            .build());
        assertEquals(base.hashCode(), base.hashCode());

        assertNotEquals(base, ReconnectConfig.builder().enabled(false).build());
        assertNotEquals(base, ReconnectConfig.builder().maxAttempts(8).build());
        assertNotEquals(base, ReconnectConfig.builder().initialDelayMs(8).build());
        assertNotEquals(base, ReconnectConfig.builder().maxDelayMs(8).build());
        assertNotEquals(base, ReconnectConfig.builder().backoffMultiplier(8.8).build());
        assertNotEquals(base, ReconnectConfig.builder().jitterFactor(0.8).build());
    }

    @Test
    void retryDefaultsCoverDegenerateValues() {
        RetryConfig zeroed = RetryConfig.builder()
            .maxAttempts(0).initialDelayMs(0).maxDelayMs(0)
            .backoffMultiplier(0).jitterFactor(0)
            .build();
        assertNotNull(zeroed);
        assertEquals(0, zeroed.getMaxAttempts());
    }
}
