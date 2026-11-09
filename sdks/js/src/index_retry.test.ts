import { BasicClient } from "./index";

describe("BasicClient", () => {
  describe("RetryConfig and ReconnectConfig", () => {
    test("constructor includes default retry config", () => {
      const client = new BasicClient();
      const config = (client as any).config;

      expect(config.retry).toBeDefined();
      expect(config.retry.enabled).toBe(true);
      expect(config.retry.maxAttempts).toBe(3);
      expect(config.retry.initialDelayMs).toBe(100);
      expect(config.retry.maxDelayMs).toBe(5000);
      expect(config.retry.backoffMultiplier).toBe(2.0);
      expect(config.retry.jitterFactor).toBe(0.1);
      expect(config.retry.retryableStatusCodes).toEqual([14, 13, 2, 10, 4]);
    });

    test("constructor includes default reconnect config", () => {
      const client = new BasicClient();
      const config = (client as any).config;

      expect(config.reconnect).toBeDefined();
      expect(config.reconnect.enabled).toBe(true);
      expect(config.reconnect.maxAttempts).toBe(0); // Infinite
      expect(config.reconnect.initialDelayMs).toBe(1000);
      expect(config.reconnect.maxDelayMs).toBe(30000);
      expect(config.reconnect.backoffMultiplier).toBe(2.0);
      expect(config.reconnect.jitterFactor).toBe(0.2);
    });

    test("constructor merges user retry config", () => {
      const client = new BasicClient({
        retry: {
          maxAttempts: 5,
          initialDelayMs: 200,
        },
      });
      const config = (client as any).config;

      expect(config.retry.maxAttempts).toBe(5);
      expect(config.retry.initialDelayMs).toBe(200);
      // Other defaults should still apply
      expect(config.retry.maxDelayMs).toBe(5000);
      expect(config.retry.backoffMultiplier).toBe(2.0);
    });

    test("constructor merges user reconnect config", () => {
      const client = new BasicClient({
        reconnect: {
          maxAttempts: 10,
          initialDelayMs: 500,
        },
      });
      const config = (client as any).config;

      expect(config.reconnect.maxAttempts).toBe(10);
      expect(config.reconnect.initialDelayMs).toBe(500);
      // Other defaults should still apply
      expect(config.reconnect.maxDelayMs).toBe(30000);
      expect(config.reconnect.backoffMultiplier).toBe(2.0);
    });

    test("invoke accepts InvokeOptions with retry config", async () => {
      const client = new BasicClient();
      const handler = jest.fn().mockResolvedValue("result");
      client.registerFunction({ id: "test.fn", version: "1.0.0" }, handler);

      const invokeOptions = {
        retry: {
          maxAttempts: 3,
          initialDelayMs: 100,
        },
        headers: {
          "x-custom": "value",
        },
      };

      const result = await client.invoke("test.fn", "payload", invokeOptions);

      expect(result).toBe("result");
      expect(handler).toHaveBeenCalledWith(
        expect.stringContaining("x-custom"),
        "payload",
      );
    });

    test("invoke accepts Record<string, string> as metadata (backward compatibility)", async () => {
      const client = new BasicClient();
      const handler = jest.fn().mockResolvedValue("result");
      client.registerFunction({ id: "test.fn", version: "1.0.0" }, handler);

      const metadata = {
        "x-custom": "value",
      };

      const result = await client.invoke("test.fn", "payload", metadata);

      expect(result).toBe("result");
      expect(handler).toHaveBeenCalledWith(
        expect.stringContaining("x-custom"),
        "payload",
      );
    });

    test("startTask accepts InvokeOptions", async () => {
      const client = new BasicClient();
      let receivedContext = "";
      client.registerFunction(
        { id: "test.fn", version: "1.0.0" },
        async (ctx) => {
          receivedContext = ctx;
          return "done";
        },
      );

      const invokeOptions = {
        retry: {
          maxAttempts: 5,
        },
        headers: {
          "x-custom": "value",
        },
      };

      const taskId = client.startTask("test.fn", "payload", invokeOptions);

      // Wait for task to complete
      for await (const _ of client.streamTask(taskId)) {
        // consume events
      }

      expect(receivedContext).toContain("x-custom");
      expect(receivedContext).toContain("value");
    });

    test("RetryConfig can be disabled", () => {
      const client = new BasicClient({
        retry: {
          enabled: false,
        },
      });
      const config = (client as any).config;

      expect(config.retry.enabled).toBe(false);
    });

    test("ReconnectConfig can be disabled", () => {
      const client = new BasicClient({
        reconnect: {
          enabled: false,
        },
      });
      const config = (client as any).config;

      expect(config.reconnect.enabled).toBe(false);
    });
  });
});
