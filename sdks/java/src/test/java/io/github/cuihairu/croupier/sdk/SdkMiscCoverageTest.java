package io.github.cuihairu.croupier.sdk;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.lang.reflect.Constructor;

import static org.junit.jupiter.api.Assertions.*;

/**
 * 零散未覆盖成员补测：ClientConfig.inboundWorkers、CroupierSDK 隐式构造器、
 * sdk 包 ReconnectConfig.equals 守卫分支。
 */
@DisplayName("Misc uncovered members")
class SdkMiscCoverageTest {

    @Test
    @DisplayName("ClientConfig inboundWorkers getter/setter")
    void clientConfigInboundWorkers() {
        ClientConfig config = new ClientConfig("game-1", "svc-1");
        assertEquals(0, config.getInboundWorkers());
        config.setInboundWorkers(4);
        assertEquals(4, config.getInboundWorkers());
        config.setInboundWorkers(1);
        assertEquals(1, config.getInboundWorkers());
    }

    @Test
    @DisplayName("CroupierSDK 隐式构造器可实例化")
    void croupierSdkDefaultConstructor() throws Exception {
        Constructor<CroupierSDK> constructor = CroupierSDK.class.getDeclaredConstructor();
        constructor.setAccessible(true);
        assertNotNull(constructor.newInstance());
    }

    @Test
    @DisplayName("sdk ReconnectConfig equals 守卫分支")
    void sdkReconnectConfigEqualsGuards() {
        ReconnectConfig reconnect = ReconnectConfig.builder().build();
        assertEquals(reconnect, reconnect);
        assertNotEquals(null, reconnect);
        assertNotEquals("other", reconnect);
        assertEquals(reconnect, ReconnectConfig.builder().build());
        assertNotEquals(reconnect, ReconnectConfig.builder().maxAttempts(9).build());
        assertNotEquals(reconnect, ReconnectConfig.builder().enabled(false).build());
        assertNotEquals(reconnect, ReconnectConfig.builder().initialDelayMs(11).build());
        assertNotEquals(reconnect, ReconnectConfig.builder().maxDelayMs(22).build());
        assertNotEquals(reconnect, ReconnectConfig.builder().backoffMultiplier(3.0).build());
        assertNotEquals(reconnect, ReconnectConfig.builder().jitterFactor(0.9).build());
    }
}
