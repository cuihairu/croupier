/**
 * Integration tests for Croupier JavaScript SDK
 *
 * These tests require a running croupier-agent on localhost:19090.
 * They test real TCP connection, function registration, and heartbeat.
 */

import { BasicClient, FunctionDescriptor } from "./index";

// Check if agent is available
const AGENT_ADDR = process.env.CROUPIER_AGENT_ADDR || "tcp://127.0.0.1:19090";
const INTEGRATION_TEST_TIMEOUT = 15000; // 15 seconds

async function isAgentAvailable(): Promise<boolean> {
  try {
    const client = new BasicClient({ agentAddr: AGENT_ADDR });
    // Try to connect with minimal timeout
    const timeoutPromise = new Promise<boolean>((_resolve, reject) => {
      setTimeout(() => reject(new Error("timeout")), 2000);
    });
    const connectPromise = client.connect().then(() => true, () => false);
    const result = await Promise.race([connectPromise, timeoutPromise]);
    await client.disconnect().catch(() => {});
    return result === true;
  } catch {
    return false;
  }
}

describe("Integration Tests (requires croupier-agent)", () => {
  let agentAvailable = false;

  beforeAll(async () => {
    agentAvailable = await isAgentAvailable();
    if (!agentAvailable) {
      console.warn(
        "⚠️  croupier-agent not available, skipping integration tests",
      );
      console.warn(`   Expected agent at: ${AGENT_ADDR}`);
    }
  });

  test(
    "connect to agent and register function",
    async () => {
      if (!agentAvailable) {
        console.warn("Skipping test: agent not available");
        return;
      }

      const client = new BasicClient({
        agentAddr: AGENT_ADDR,
        serviceId: "js-integration-test",
        serviceVersion: "1.0.0",
        heartbeatIntervalSeconds: 30,
      });

      // Register a test function
      const descriptor: FunctionDescriptor = {
        id: "test.ping",
        version: "1.0.0",
        name: "Ping",
        description: "A simple ping function",
      };

      const handler = async (_ctx: string, payload: string) => {
        return `pong: ${payload}`;
      };

      client.registerFunction(descriptor, handler);

      // Connect to agent
      await expect(client.connect()).resolves.not.toThrow();

      // Verify we are connected
      expect((client as any).connected).toBe(true);
      expect((client as any).sessionId).toBeTruthy();

      // Clean up
      await client.disconnect();
      expect((client as any).connected).toBe(false);
    },
    INTEGRATION_TEST_TIMEOUT,
  );

  test(
    "connect fails with invalid agent address",
    async () => {
      const client = new BasicClient({
        agentAddr: "tcp://127.0.0.1:9999", // Non-existent port
        serviceId: "js-integration-test",
        heartbeatIntervalSeconds: 30,
      });

      const descriptor: FunctionDescriptor = {
        id: "test.ping",
        version: "1.0.0",
      };

      client.registerFunction(descriptor, async () => "ok");

      await expect(client.connect()).rejects.toThrow();
    },
    INTEGRATION_TEST_TIMEOUT,
  );

  test(
    "connect requires at least one function",
    async () => {
      if (!agentAvailable) {
        console.warn("Skipping test: agent not available");
        return;
      }

      const client = new BasicClient({
        agentAddr: AGENT_ADDR,
      });

      // No functions registered - should fail
      await expect(client.connect()).rejects.toThrow(
        /Register at least one function/i,
      );
    },
    INTEGRATION_TEST_TIMEOUT,
  );

  test(
    "heartbeat is sent periodically",
    async () => {
      if (!agentAvailable) {
        console.warn("Skipping test: agent not available");
        return;
      }

      const client = new BasicClient({
        agentAddr: AGENT_ADDR,
        serviceId: "js-integration-test-heartbeat",
        serviceVersion: "1.0.0",
        heartbeatIntervalSeconds: 2, // Short interval for testing
      });

      client.registerFunction(
        { id: "test.hb", version: "1.0.0" },
        async () => "ok",
      );

      await client.connect();
      expect((client as any).sessionId).toBeTruthy();

      // Wait for a few heartbeats
      await new Promise((resolve) => setTimeout(resolve, 5000));

      // Should still be connected
      expect((client as any).connected).toBe(true);

      await client.disconnect();
    },
    INTEGRATION_TEST_TIMEOUT,
  );

  test(
    "disconnect is idempotent",
    async () => {
      if (!agentAvailable) {
        console.warn("Skipping test: agent not available");
        return;
      }

      const client = new BasicClient({
        agentAddr: AGENT_ADDR,
        serviceId: "js-integration-test-disconnect",
      });

      client.registerFunction(
        { id: "test.disconnect", version: "1.0.0" },
        async () => "ok",
      );

      await client.connect();

      // Disconnect multiple times - should not throw
      await client.disconnect();
      await client.disconnect();
      await client.disconnect();

      expect((client as any).connected).toBe(false);
    },
    INTEGRATION_TEST_TIMEOUT,
  );

  test(
    "reconnect after disconnect",
    async () => {
      if (!agentAvailable) {
        console.warn("Skipping test: agent not available");
        return;
      }

      const client = new BasicClient({
        agentAddr: AGENT_ADDR,
        serviceId: "js-integration-test-reconnect",
      });

      client.registerFunction(
        { id: "test.reconnect", version: "1.0.0" },
        async () => "ok",
      );

      // First connection
      await client.connect();
      const sessionId1 = (client as any).sessionId;
      expect(sessionId1).toBeTruthy();

      await client.disconnect();

      // Second connection
      await client.connect();
      const sessionId2 = (client as any).sessionId;
      expect(sessionId2).toBeTruthy();

      // Session IDs should be different
      expect(sessionId2).not.toBe(sessionId1);

      await client.disconnect();
    },
    INTEGRATION_TEST_TIMEOUT,
  );
});
