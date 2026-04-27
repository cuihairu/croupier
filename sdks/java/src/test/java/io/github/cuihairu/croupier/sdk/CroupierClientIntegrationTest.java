package io.github.cuihairu.croupier.sdk;

import static org.junit.jupiter.api.Assertions.assertDoesNotThrow;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.util.Map;
import java.util.concurrent.CompletionException;
import java.util.concurrent.TimeUnit;

import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.condition.EnabledIfSystemProperty;

/**
 * Integration tests for Croupier Java SDK.
 *
 * <p>These tests require a running croupier-agent on localhost:19090.
 * They test real TCP connection, function registration, and heartbeat.
 *
 * <p>To run these tests:
 * <ul>
 *   <li>Start the agent: {@code docker compose -f docker/docker-compose.ci.yml up -d}</li>
 *   <li>Run with: {@code mvn test -Dtest=CroupierClientIntegrationTest -Dintegration=true}</li>
 * </ul>
 */
@EnabledIfSystemProperty(named = "integration", matches = "true")
class CroupierClientIntegrationTest {

    private static final String AGENT_ADDR = System.getProperty("croupier.agent.addr", "tcp://127.0.0.1:19090");
    private static final int CONNECT_TIMEOUT_SECONDS = 10;

    @Test
    void connectToAgentAndRegisterFunction() throws Exception {
        ClientConfig config = new ClientConfig("game1", "java-integration-test");
        config.setAgentAddr(AGENT_ADDR);
        config.setServiceVersion("1.0.0");
        config.setHeartbeatInterval(30);
        config.setTimeoutSeconds(CONNECT_TIMEOUT_SECONDS);

        CroupierClientImpl client = new CroupierClientImpl(config);

        // Register a test function
        FunctionDescriptor descriptor = new FunctionDescriptor("test.ping", "1.0.0");
        FunctionHandler handler = (context, payload) -> "pong: " + payload;

        client.registerFunction(descriptor, handler);

        // Connect to agent
        client.connect().get(CONNECT_TIMEOUT_SECONDS, TimeUnit.SECONDS);

        // Verify we are connected
        assertTrue(client.isConnected());
        assertNotNull(client.getSessionId());

        // Clean up
        client.stop();
        assertFalse(client.isConnected());
    }

    @Test
    void connectFailsWithInvalidAgentAddress() {
        ClientConfig config = new ClientConfig("game1", "java-integration-test-invalid");
        config.setAgentAddr("tcp://127.0.0.1:9999"); // Non-existent port
        config.setTimeoutSeconds(5);

        CroupierClientImpl client = new CroupierClientImpl(config);

        FunctionDescriptor descriptor = new FunctionDescriptor("test.ping", "1.0.0");
        client.registerFunction(descriptor, (context, payload) -> "ok");

        // Should fail to connect
        assertThrowsCompletionException(
            () -> client.connect().get(10, TimeUnit.SECONDS)
        );
    }

    @Test
    void connectRequiresAtLeastOneFunction() {
        ClientConfig config = new ClientConfig("game1", "java-integration-test-no-func");
        config.setAgentAddr(AGENT_ADDR);

        CroupierClientImpl client = new CroupierClientImpl(config);

        // No functions registered - should fail
        assertThrowsCompletionException(
            () -> client.connect().get(10, TimeUnit.SECONDS)
        );
    }

    @Test
    void connectIsIdempotent() throws Exception {
        ClientConfig config = new ClientConfig("game1", "java-integration-test-idempotent");
        config.setAgentAddr(AGENT_ADDR);

        CroupierClientImpl client = new CroupierClientImpl(config);

        client.registerFunction(
            new FunctionDescriptor("test.idempotent", "1.0.0"),
            (context, payload) -> "ok"
        );

        // First connect
        client.connect().get(CONNECT_TIMEOUT_SECONDS, TimeUnit.SECONDS);
        String firstSessionId = client.getSessionId();
        assertNotNull(firstSessionId);

        // Second connect should be safe (idempotent)
        assertDoesNotThrow(() -> client.connect().get(CONNECT_TIMEOUT_SECONDS, TimeUnit.SECONDS));
        String secondSessionId = client.getSessionId();

        // Session IDs should be the same (no reconnect)
        assertTrue(secondSessionId.equals(firstSessionId));

        // Clean up
        client.stop();
    }

    @Test
    void reconnectAfterDisconnect() throws Exception {
        ClientConfig config = new ClientConfig("game1", "java-integration-test-reconnect");
        config.setAgentAddr(AGENT_ADDR);

        CroupierClientImpl client = new CroupierClientImpl(config);

        client.registerFunction(
            new FunctionDescriptor("test.reconnect", "1.0.0"),
            (context, payload) -> "ok"
        );

        // First connection
        client.connect().get(CONNECT_TIMEOUT_SECONDS, TimeUnit.SECONDS);
        String sessionId1 = client.getSessionId();
        assertNotNull(sessionId1);

        // Disconnect
        client.stop();
        assertFalse(client.isConnected());

        // Second connection
        client.connect().get(CONNECT_TIMEOUT_SECONDS, TimeUnit.SECONDS);
        String sessionId2 = client.getSessionId();
        assertNotNull(sessionId2);

        // Session IDs should be different (new session)
        assertFalse(sessionId2.equals(sessionId1));

        // Clean up
        client.stop();
    }

    @Test
    void stopIsIdempotent() throws Exception {
        ClientConfig config = new ClientConfig("game1", "java-integration-test-stop");
        config.setAgentAddr(AGENT_ADDR);

        CroupierClientImpl client = new CroupierClientImpl(config);

        client.registerFunction(
            new FunctionDescriptor("test.stop", "1.0.0"),
            (context, payload) -> "ok"
        );

        client.connect().get(CONNECT_TIMEOUT_SECONDS, TimeUnit.SECONDS);

        // Stop multiple times - should not throw
        assertDoesNotThrow(client::stop);
        assertDoesNotThrow(client::stop);
        assertDoesNotThrow(client::stop);

        assertFalse(client.isConnected());
    }

    @Test
    void registerMultipleFunctions() throws Exception {
        ClientConfig config = new ClientConfig("game1", "java-integration-test-multi");
        config.setAgentAddr(AGENT_ADDR);
        config.setHeartbeatInterval(30);

        CroupierClientImpl client = new CroupierClientImpl(config);

        // Register multiple functions
        client.registerFunction(
            new FunctionDescriptor("test.ping", "1.0.0"),
            (context, payload) -> "pong: " + payload
        );
        client.registerFunction(
            new FunctionDescriptor("test.echo", "1.0.0"),
            (context, payload) -> payload
        );
        client.registerFunction(
            new FunctionDescriptor("test.upper", "1.0.0"),
            (context, payload) -> payload.toUpperCase()
        );

        client.connect().get(CONNECT_TIMEOUT_SECONDS, TimeUnit.SECONDS);

        assertTrue(client.isConnected());
        assertNotNull(client.getSessionId());

        // Clean up
        client.stop();
    }

    private void assertThrowsCompletionException(ThrowingRunnable runnable) {
        try {
            runnable.run();
            throw new AssertionError("Expected CompletionException but none was thrown");
        } catch (Exception e) {
            if (!(e instanceof CompletionException)) {
                throw new AssertionError("Expected CompletionException but got: " + e.getClass().getName());
            }
        }
    }

    @FunctionalInterface
    private interface ThrowingRunnable {
        void run() throws Exception;
    }
}
