package io.github.cuihairu.croupier.spring.boot.autoconfigure;

import io.github.cuihairu.croupier.sdk.ClientConfig;
import org.junit.jupiter.api.Test;
import org.springframework.boot.autoconfigure.AutoConfigurations;
import org.springframework.boot.test.context.runner.ApplicationContextRunner;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertSame;

/**
 * AUD-105: the Spring starter must bind the provider handshake token and map
 * it onto the same hand-written {@link ClientConfig} field the core SDK uses.
 */
class CroupierAuthTokenBindingTest {

    private final ApplicationContextRunner runner = new ApplicationContextRunner()
        .withConfiguration(AutoConfigurations.of(CroupierAutoConfiguration.class));

    @Test
    void omittedTokenLeavesClientConfigDefault() {
        runner.withPropertyValues(
            "croupier.game-id=game-1",
            "croupier.service-id=svc-1").run(context -> {
            ClientConfig config = context.getBean(ClientConfig.class);
            assertNull(config.getAuthToken(), "omitted token must keep the ClientConfig default");
        });
    }

    @Test
    void explicitTokenBindsOntoClientConfig() {
        runner.withPropertyValues(
            "croupier.game-id=game-1",
            "croupier.service-id=svc-1",
            "croupier.auth-token=Bearer spring-token").run(context -> {
            ClientConfig config = context.getBean(ClientConfig.class);
            assertEquals("Bearer spring-token", config.getAuthToken(),
                "relaxed binding croupier.auth-token must map to ClientConfig.authToken");
        });
    }

    @Test
    void userDefinedClientConfigBeanWins() {
        ClientConfig custom = new ClientConfig("my-game", "my-service");
        custom.setAuthToken("Bearer custom-token");
        runner.withBean(ClientConfig.class, () -> custom).run(context -> {
            assertSame(custom, context.getBean(ClientConfig.class),
                "@ConditionalOnMissingBean must back off for a user-supplied ClientConfig");
            assertEquals("Bearer custom-token", context.getBean(ClientConfig.class).getAuthToken());
        });
    }
}
