/**
 * Integration tests for Croupier JavaScript SDK
 *
 * These tests require a running croupier-agent local SDK gateway on localhost:19091.
 * They test real TCP connection, function registration, and heartbeat.
 */

import net from "node:net";

import { BasicClient, FunctionDescriptor } from "./index";

const RUN_INTEGRATION_TESTS = process.env.CROUPIER_RUN_INTEGRATION_TESTS === "1";
const AGENT_ADDR = process.env.CROUPIER_AGENT_ADDR || "tcp://127.0.0.1:19091";
const INTEGRATION_TEST_TIMEOUT = 15000; // 15 seconds
const integrationDescribe = RUN_INTEGRATION_TESTS ? describe : describe.skip;

async function isAgentAvailable(): Promise<boolean> {
  const address = AGENT_ADDR.replace(/^[a-z+]+:\/\//i, "");
  const lastColon = address.lastIndexOf(":");
  if (lastColon <= 0 || lastColon === address.length - 1) {
    return false;
  }

  const host = address.slice(0, lastColon).replace(/^\[|\]$/g, "");
  const port = Number(address.slice(lastColon + 1));
  if (!Number.isInteger(port) || port < 1 || port > 65535) {
    return false;
  }

  return new Promise((resolve) => {
    const socket = net.createConnection({ host, port });
    const finish = (available: boolean) => {
      socket.removeAllListeners();
      socket.destroy();
      resolve(available);
    };
    socket.setTimeout(2000);
    socket.once("connect", () => finish(true));
    socket.once("timeout", () => finish(false));
    socket.once("error", () => finish(false));
  });
}

integrationDescribe("Integration Tests (requires croupier-agent)", () => {
  beforeAll(async () => {
    if (!(await isAgentAvailable())) {
      throw new Error(
        `croupier-agent local SDK gateway is unavailable at ${AGENT_ADDR}`,
      );
    }
  });

  test(
    "connect to agent and register function",
    async () => {
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
