package io.github.cuihairu.croupier.spring.boot.autoconfigure;

import io.github.cuihairu.croupier.sdk.ClientConfig;
import org.junit.jupiter.api.Test;
import org.springframework.boot.autoconfigure.AutoConfigurations;
import org.springframework.boot.test.context.runner.ApplicationContextRunner;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNull;

/**
 * AUD-107: the Spring entry must map the same TLS fields the default TCP
 * transport consumes (CA / client cert / key / server name / key-store
 * password) onto the hand-written ClientConfig.
 */
class CroupierSecurityPropertiesBindingTest {

    private final ApplicationContextRunner runner = new ApplicationContextRunner()
        .withConfiguration(AutoConfigurations.of(CroupierAutoConfiguration.class));

    @Test
    void securityDefaultsStayUnset() {
        runner.withPropertyValues(
            "croupier.game-id=game-1",
            "croupier.service-id=svc-1").run(context -> {
            ClientConfig config = context.getBean(ClientConfig.class);
            assertNull(config.getServerName());
            assertNull(config.getKeyStorePassword());
            assertNull(config.getCaFile());
        });
    }

    @Test
    void explicitSecurityFieldsBindOntoClientConfig() {
        runner.withPropertyValues(
            "croupier.game-id=game-1",
            "croupier.service-id=svc-1",
            "croupier.insecure=false",
            "croupier.ca-file=/etc/croupier/ca.pem",
            "croupier.cert-file=/etc/croupier/client.pem",
            "croupier.key-file=/etc/croupier/client-key.pem",
            "croupier.server-name=agent.internal",
            "croupier.key-store-password=changeit").run(context -> {
            ClientConfig config = context.getBean(ClientConfig.class);
            assertEquals(Boolean.FALSE, config.isInsecure(), "relaxed binding croupier.insecure");
            assertEquals("/etc/croupier/ca.pem", config.getCaFile());
            assertEquals("/etc/croupier/client.pem", config.getCertFile());
            assertEquals("/etc/croupier/client-key.pem", config.getKeyFile());
            assertEquals("agent.internal", config.getServerName(),
                "relaxed binding croupier.server-name must map to ClientConfig.serverName");
            assertEquals("changeit", config.getKeyStorePassword(),
                "relaxed binding croupier.key-store-password must map to ClientConfig.keyStorePassword");
        });
    }
}
