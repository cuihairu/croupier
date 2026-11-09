import { BasicClient, type InvokeOptions } from "./index";

describe("BasicClient", () => {
  describe("InvokeOptions - idempotencyKey and timeout", () => {
    test("invoke accepts idempotencyKey in InvokeOptions", async () => {
      const client = new BasicClient();
      let receivedContext = "";
      const handler = jest.fn().mockImplementation(async (ctx) => {
        receivedContext = ctx;
        return "result";
      });
      client.registerFunction({ id: "test.fn", version: "1.0.0" }, handler);

      const invokeOptions = {
        idempotencyKey: "unique-key-123",
        headers: {
          "x-custom": "value",
        },
      };

      const result = await client.invoke("test.fn", "payload", invokeOptions);

      expect(result).toBe("result");
      expect(handler).toHaveBeenCalled();
      const context = JSON.parse(receivedContext);
      expect(context.idempotencyKey).toBe("unique-key-123");
      expect(context["x-custom"]).toBe("value");
    });

    test("invoke accepts timeout in InvokeOptions", async () => {
      const client = new BasicClient();
      let receivedContext = "";
      const handler = jest.fn().mockImplementation(async (ctx) => {
        receivedContext = ctx;
        return "result";
      });
      client.registerFunction({ id: "test.fn", version: "1.0.0" }, handler);

      const invokeOptions = {
        timeout: 10000,
        headers: {},
      };

      const result = await client.invoke("test.fn", "payload", invokeOptions);

      expect(result).toBe("result");
      const context = JSON.parse(receivedContext);
      expect(context.timeout).toBe(10000);
    });

    test("invoke uses client-level timeout when not specified in options", async () => {
      const client = new BasicClient({ timeout: 5000 });
      let receivedContext = "";
      const handler = jest.fn().mockImplementation(async (ctx) => {
        receivedContext = ctx;
        return "result";
      });
      client.registerFunction({ id: "test.fn", version: "1.0.0" }, handler);

      const result = await client.invoke("test.fn", "payload", {});

      expect(result).toBe("result");
      const context = JSON.parse(receivedContext);
      expect(context.timeout).toBe(5000);
    });

    test("invoke timeout overrides client-level timeout", async () => {
      const client = new BasicClient({ timeout: 5000 });
      let receivedContext = "";
      const handler = jest.fn().mockImplementation(async (ctx) => {
        receivedContext = ctx;
        return "result";
      });
      client.registerFunction({ id: "test.fn", version: "1.0.0" }, handler);

      const invokeOptions = {
        timeout: 10000,
      };

      const result = await client.invoke("test.fn", "payload", invokeOptions);

      expect(result).toBe("result");
      const context = JSON.parse(receivedContext);
      expect(context.timeout).toBe(10000); // Override should work
    });

    test("startTask accepts idempotencyKey in InvokeOptions", async () => {
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
        idempotencyKey: "job-key-456",
        headers: {
          "x-job-id": "123",
        },
      };

      const taskId = client.startTask("test.fn", "payload", invokeOptions);

      // Wait for task to complete
      for await (const _ of client.streamTask(taskId)) {
        // consume events
      }

      const context = JSON.parse(receivedContext);
      expect(context.idempotencyKey).toBe("job-key-456");
      expect(context["x-job-id"]).toBe("123");
    });

    test("startTask accepts timeout in InvokeOptions", async () => {
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
        timeout: 15000,
      };

      const taskId = client.startTask("test.fn", "payload", invokeOptions);

      // Wait for task to complete
      for await (const _ of client.streamTask(taskId)) {
        // consume events
      }

      const context = JSON.parse(receivedContext);
      expect(context.timeout).toBe(15000);
    });

    test("invoke with complete InvokeOptions", async () => {
      const client = new BasicClient();
      let receivedContext = "";
      const handler = jest.fn().mockImplementation(async (ctx) => {
        receivedContext = ctx;
        return "result";
      });
      client.registerFunction({ id: "test.fn", version: "1.0.0" }, handler);

      const invokeOptions: InvokeOptions = {
        idempotencyKey: "complete-key",
        timeout: 20000,
        headers: {
          "x-request-id": "req-789",
          "x-game-id": "game-123",
        },
        retry: {
          maxAttempts: 5,
          initialDelayMs: 200,
        },
      };

      const result = await client.invoke("test.fn", "payload", invokeOptions);

      expect(result).toBe("result");
      const context = JSON.parse(receivedContext);
      expect(context.idempotencyKey).toBe("complete-key");
      expect(context.timeout).toBe(20000);
      expect(context["x-request-id"]).toBe("req-789");
      expect(context["x-game-id"]).toBe("game-123");
    });

    test("isInvokeOptions correctly identifies InvokeOptions with idempotencyKey", async () => {
      const client = new BasicClient();
      const handler = jest.fn().mockResolvedValue("result");
      client.registerFunction({ id: "test.fn", version: "1.0.0" }, handler);

      // InvokeOptions with idempotencyKey
      const options1 = { idempotencyKey: "key-123" };
      const result1 = await client.invoke("test.fn", "payload", options1);
      expect(result1).toBe("result");

      // InvokeOptions with timeout
      const options2 = { timeout: 5000 };
      const result2 = await client.invoke("test.fn", "payload", options2);
      expect(result2).toBe("result");

      // Plain Record<string, string>
      const metadata = { "x-custom": "value" };
      const result3 = await client.invoke("test.fn", "payload", metadata);
      expect(result3).toBe("result");
    });

    test("InvokeOptions without idempotencyKey or timeout still works", async () => {
      const client = new BasicClient();
      const handler = jest.fn().mockResolvedValue("result");
      client.registerFunction({ id: "test.fn", version: "1.0.0" }, handler);

      const invokeOptions = {
        headers: {
          "x-custom": "value",
        },
      };

      const result = await client.invoke("test.fn", "payload", invokeOptions);

      expect(result).toBe("result");
      expect(handler).toHaveBeenCalled();
    });

    test("invoke merges client-level headers, auth token, gameId, and env", async () => {
      const client = new BasicClient({
        authToken: "secret-token",
        gameId: "game-123",
        env: "staging",
        headers: {
          "X-Client": "sdk",
        },
      });
      let receivedContext = "";
      client.registerFunction(
        { id: "test.fn", version: "1.0.0" },
        async (ctx) => {
          receivedContext = ctx;
          return "result";
        },
      );

      const result = await client.invoke("test.fn", "payload", {
        headers: {
          "X-Request-ID": "req-1",
        },
      });

      expect(result).toBe("result");
      const context = JSON.parse(receivedContext);
      expect(context.Authorization).toBe("Bearer secret-token");
      expect(context["X-Client"]).toBe("sdk");
      expect(context["X-Request-ID"]).toBe("req-1");
      expect(context["X-Game-ID"]).toBe("game-123");
      expect(context["X-Env"]).toBe("staging");
    });

    test("per-call headers override client-level metadata defaults", async () => {
      const client = new BasicClient({
        authToken: "secret-token",
        gameId: "game-default",
        env: "development",
        headers: {
          Authorization: "Bearer config-token",
          "X-Game-ID": "game-config",
          "X-Env": "staging",
        },
      });
      let receivedContext = "";
      client.registerFunction(
        { id: "test.fn", version: "1.0.0" },
        async (ctx) => {
          receivedContext = ctx;
          return "result";
        },
      );

      await client.invoke("test.fn", "payload", {
        headers: {
          Authorization: "Bearer override-token",
          "X-Game-ID": "game-override",
          "X-Env": "production",
        },
      });

      const context = JSON.parse(receivedContext);
      expect(context.Authorization).toBe("Bearer override-token");
      expect(context["X-Game-ID"]).toBe("game-override");
      expect(context["X-Env"]).toBe("production");
    });

    test("startTask uses merged client-level metadata", async () => {
      const client = new BasicClient({
        authToken: "job-token",
        gameId: "job-game",
        env: "staging",
        headers: {
          "X-Client": "job-sdk",
        },
      });
      let receivedContext = "";
      client.registerFunction(
        { id: "test.fn", version: "1.0.0" },
        async (ctx) => {
          receivedContext = ctx;
          return "done";
        },
      );

      const taskId = client.startTask("test.fn", "payload", {
        headers: {
          "X-Request-ID": "job-1",
        },
      });

      for await (const _ of client.streamTask(taskId)) {
        // consume events
      }

      const context = JSON.parse(receivedContext);
      expect(context.Authorization).toBe("Bearer job-token");
      expect(context["X-Client"]).toBe("job-sdk");
      expect(context["X-Request-ID"]).toBe("job-1");
      expect(context["X-Game-ID"]).toBe("job-game");
      expect(context["X-Env"]).toBe("staging");
    });
  });
});
