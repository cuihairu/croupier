package io.github.cuihairu.croupier.spring.boot.autoconfigure;

import io.github.cuihairu.croupier.sdk.ClientConfig;
import io.github.cuihairu.croupier.sdk.CroupierClient;
import io.github.cuihairu.croupier.sdk.CroupierClientImpl;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.boot.autoconfigure.AutoConfiguration;
import org.springframework.boot.autoconfigure.condition.ConditionalOnMissingBean;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.boot.context.properties.EnableConfigurationProperties;
import org.springframework.context.annotation.Bean;

/**
 * Auto-configuration for Croupier SDK in Spring Boot applications.
 *
 * <p>This configuration creates the following beans:
 * <ul>
 *   <li>{@link ClientConfig} - configured from application properties</li>
 *   <li>{@link CroupierClient} - the main gRPC client</li>
 * </ul>
 *
 * <p>To disable auto-configuration, set:
 * <pre>
 * spring.autoconfigure.exclude=io.github.cuihairu.croupier.spring.boot.autoconfigure.CroupierAutoConfiguration
 * </pre>
 *
 * <p>Or use the property:
 * <pre>
 * croupier.enabled=false
 * </pre>
 */
@AutoConfiguration
@EnableConfigurationProperties(CroupierProperties.class)
@ConditionalOnProperty(prefix = "croupier", name = "enabled", havingValue = "true", matchIfMissing = true)
public class CroupierAutoConfiguration {

    private static final Logger log = LoggerFactory.getLogger(CroupierAutoConfiguration.class);

    /**
     * Creates a {@link ClientConfig} bean from {@link CroupierProperties}.
     */
    @Bean
    @ConditionalOnMissingBean
    public ClientConfig croupierClientConfig(CroupierProperties properties) {
        log.info("Configuring Croupier client with agent address: {}", properties.getAgentAddress());

        ClientConfig config = new ClientConfig(properties.getGameId(), properties.getServiceId());
        config.setAgentAddr(properties.getAgentAddress());
        config.setEnv(properties.getEnv());
        config.setServiceVersion(properties.getServiceVersion());
        config.setAgentId(properties.getAgentId());
        config.setLocalListen(properties.getLocalListen());
        config.setControlAddr(properties.getControlAddr());
        config.setTimeoutSeconds(properties.getTimeoutSeconds());
        config.setInsecure(properties.isInsecure());
        config.setCaFile(properties.getCaFile());
        config.setCertFile(properties.getCertFile());
        config.setKeyFile(properties.getKeyFile());
        config.setProviderLang(properties.getProviderLang());
        config.setProviderSdk(properties.getProviderSdk());

        return config;
    }

    /**
     * Creates a {@link CroupierClient} bean.
     */
    @Bean
    @ConditionalOnMissingBean
    public CroupierClient croupierClient(ClientConfig clientConfig) {
        log.info("Creating Croupier client bean");
        return new CroupierClientImpl(clientConfig);
    }
}
