/**
 * Tests for OpenAPI 3 import (Go RegisterFromOpenAPI parity), Draft-07 JSON
 * Schema validation and invoker retry semantics.
 */

import { BasicClient, FunctionDescriptor } from "./index";
import { registerFromOpenAPI, ImportOptions } from "./openapi";
import { Invoker, InvokerError, RetryConfig } from "./invoker";

// ---------------------------------------------------------------------------
// fetch mocking helpers
// ---------------------------------------------------------------------------

type FetchImpl = typeof fetch;
let originalFetch: FetchImpl | undefined;

function mockFetch(
  responder: (url: string, init?: RequestInit) => Promise<Response>,
): void {
  originalFetch = globalThis.fetch;
  globalThis.fetch = (((url: string | URL | Request, init?: RequestInit) => {
    const u = typeof url === "string" ? url : url.toString();
    return responder(u, init);
  }) as unknown) as FetchImpl;
}

function restoreFetch(): void {
  if (originalFetch) {
    globalThis.fetch = originalFetch;
    originalFetch = undefined;
  }
}

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

afterEach(() => restoreFetch());

// ---------------------------------------------------------------------------
// OpenAPI import
// ---------------------------------------------------------------------------

const SPEC = {
  openapi: "3.0.3",
  info: { title: "GM API", version: "1.0.0" },
  paths: {
    "/players/{id}/ban": {
      put: {
        operationId: "player_ban",
        summary: "Ban player",
        description: "Bans a player account",
        tags: ["gm", "risk"],
        "x-resource": "player",
        "x-operation": "ban",
        "x-permission": "player.ban",
        "x-risk": "high",
        requestBody: {
          content: {
            "application/json": {
              schema: {
                type: "object",
                required: ["playerId", "reason"],
                properties: {
                  playerId: { type: "string", description: "Player ID" },
                  reason: { type: "string" },
                },
              },
            },
          },
        },
        responses: {
          200: {
            content: {
              "application/json": {
                schema: { type: "object", properties: { ok: { type: "boolean" } } },
              },
            },
          },
        },
      },
    },
    "/players/search": {
      get: {
        tags: ["query"],
        responses: {
          200: { content: { "application/json": { schema: { type: "array" } } } },
        },
      },
    },
  },
};

function makeClient(): BasicClient {
  return new BasicClient();
}

function registeredDescriptors(client: BasicClient): Map<string, FunctionDescriptor> {
  return (client as any).descriptors as Map<string, FunctionDescriptor>;
}

describe("registerFromOpenAPI", () => {
  const handlers = new Map<string, any>([
    ["player_ban", async () => "{}"],
    ["players.search", async () => "[]"],
  ]);

  it("registers all operations and returns their ids", () => {
    const client = makeClient();
    const registered = registerFromOpenAPI(client, SPEC, undefined, undefined, handlers);
    expect(registered).toEqual(["player_ban", "players.search"]);
    expect(registeredDescriptors(client).size).toBe(2);
  });

  it("maps operation metadata onto descriptors", () => {
    const client = makeClient();
    registerFromOpenAPI(client, SPEC, undefined, undefined, handlers);
    const descriptor = registeredDescriptors(client).get("player_ban")!;

    expect(descriptor.summary).toBe("Ban player");
    expect(descriptor.description).toBe("Bans a player account");
    expect(descriptor.tags).toEqual(["gm", "risk"]);
    expect(descriptor.resource).toBe("player");
    expect(descriptor.operation).toBe("ban");
    expect(descriptor.permission).toBe("player.ban");
    expect(descriptor.risk).toBe("high");
  });

  it("converts request/response schemas", () => {
    const client = makeClient();
    registerFromOpenAPI(client, SPEC, undefined, undefined, handlers);
    const descriptor = registeredDescriptors(client).get("player_ban")!;

    expect(descriptor.inputSchema).toEqual({
      type: "object",
      required: ["playerId", "reason"],
      properties: {
        playerId: { type: "string", description: "Player ID" },
        reason: { type: "string" },
      },
    });
    expect(descriptor.outputSchema).toEqual({
      type: "object",
      properties: { ok: { type: "boolean" } },
    });
  });

  it("derives ids from paths when operationId is missing", () => {
    const client = makeClient();
    registerFromOpenAPI(client, SPEC, undefined, undefined, handlers);
    const descriptor = registeredDescriptors(client).get("players.search")!;
    expect(descriptor.id).toBe("players.search");
    expect(descriptor.summary).toBe("Players.search");
    expect(descriptor.risk).toBe("medium");
  });

  it("applies resource and tag prefixes", () => {
    const client = makeClient();
    const options: ImportOptions = { resourcePrefix: "game", tagPrefix: "svc-" };
    registerFromOpenAPI(client, SPEC, options, undefined, handlers);
    const descriptor = registeredDescriptors(client).get("player_ban")!;

    expect(descriptor.resource).toBe("game.player");
    expect(descriptor.tags).toEqual(["svc-gm", "svc-risk"]);
  });

  it("throws when a handler is missing", () => {
    const client = makeClient();
    expect(() =>
      registerFromOpenAPI(client, SPEC, undefined, undefined, new Map()),
    ).toThrow("no handler provided for function: player_ban");
  });

  it("skips missing handlers when continueOnError is set", () => {
    const client = makeClient();
    const registered = registerFromOpenAPI(
      client,
      SPEC,
      { continueOnError: true },
      undefined,
      new Map([["players.search", async () => "[]"]]),
    );
    expect(registered).toEqual(["players.search"]);
  });

  it("accepts a resolver callback", () => {
    const client = makeClient();
    const seen: string[] = [];
    const registered = registerFromOpenAPI(
      client,
      SPEC,
      undefined,
      (functionId) => {
        seen.push(functionId);
        return async () => "{}";
      },
    );
    expect(registered).toEqual(["player_ban", "players.search"]);
    expect(seen).toEqual(["player_ban", "players.search"]);
  });

  it("rejects invalid JSON specs", () => {
    expect(() =>
      registerFromOpenAPI(makeClient(), "{not json", undefined, undefined, new Map()),
    ).toThrow("load OpenAPI spec failed");
  });

  it("rejects specs without paths", () => {
    expect(() =>
      registerFromOpenAPI(makeClient(), { openapi: "3.0.3" }, undefined, undefined, new Map()),
    ).toThrow("paths");
  });

  it("handles empty paths objects", () => {
    const client = makeClient();
    expect(
      registerFromOpenAPI(client, { paths: {} }, undefined, undefined, new Map()),
    ).toEqual([]);
  });

  it("parses a JSON string spec", () => {
    const client = makeClient();
    const registered = registerFromOpenAPI(
      client,
      JSON.stringify(SPEC),
      undefined,
      undefined,
      handlers,
    );
    expect(registered).toEqual(["player_ban", "players.search"]);
  });
});

// ---------------------------------------------------------------------------
// Draft-07 schema validation
// ---------------------------------------------------------------------------

describe("Invoker.setSchema (Draft-07)", () => {
  it("validates payload types before sending", async () => {
    mockFetch(async () => jsonResponse({ result: {} }));
    const invoker = new Invoker({ baseUrl: "http://s:18780" });
    invoker.setSchema("fn", {
      type: "object",
      required: ["playerId"],
      properties: { playerId: { type: "string", minLength: 3 } },
    });

    await expect(invoker.invoke("fn", { playerId: "abc" })).resolves.toEqual({ payload: {} });

    await expect(invoker.invoke("fn", { playerId: "ab" })).rejects.toMatchObject({
      code: "schema_validation",
    });
    await expect(invoker.invoke("fn", { playerId: 42 })).rejects.toThrow(
      "payload validation failed",
    );
  });

  it("validates startTask payloads and skips network on failure", async () => {
    let calls = 0;
    mockFetch(async () => {
      calls += 1;
      return jsonResponse({ taskId: "t1" });
    });
    const invoker = new Invoker({ baseUrl: "http://s:18780" });
    invoker.setSchema("fn", { type: "object", required: ["a"] });

    await expect(invoker.startTask("fn", {})).rejects.toMatchObject({
      code: "schema_validation",
    });
    expect(calls).toBe(0);
  });

  it("recompiles schemas when setSchema is called again", async () => {
    mockFetch(async () => jsonResponse({ result: {} }));
    const invoker = new Invoker({ baseUrl: "http://s:18780" });

    invoker.setSchema("fn", { type: "object", required: ["a"] });
    await expect(invoker.invoke("fn", {})).rejects.toMatchObject({ code: "schema_validation" });

    invoker.setSchema("fn", { type: "object" });
    await expect(invoker.invoke("fn", {})).resolves.toEqual({ payload: {} });
  });

  it("clearSchema removes validation", async () => {
    mockFetch(async () => jsonResponse({ result: {} }));
    const invoker = new Invoker({ baseUrl: "http://s:18780" });
    invoker.setSchema("fn", { type: "object", required: ["a"] });
    invoker.clearSchema("fn");
    await expect(invoker.invoke("fn", {})).resolves.toEqual({ payload: {} });
  });

  it("setSchema guards against bad input", () => {
    const invoker = new Invoker({ baseUrl: "http://s:18780" });
    expect(() => invoker.setSchema("", {})).toThrow("functionId");
    expect(() => invoker.setSchema("fn", null as any)).toThrow("schema object");
  });

  it("treats undefined payloads as empty objects", async () => {
    mockFetch(async () => jsonResponse({ result: {} }));
    const invoker = new Invoker({ baseUrl: "http://s:18780" });
    invoker.setSchema("fn", { type: "object", required: ["a"] });
    await expect(invoker.invoke("fn", undefined)).rejects.toMatchObject({
      code: "schema_validation",
    });
  });
});

// ---------------------------------------------------------------------------
// Retry semantics
// ---------------------------------------------------------------------------

describe("Invoker retry", () => {
  it("retries retryable server failures until success", async () => {
    let calls = 0;
    mockFetch(async () => {
      calls += 1;
      if (calls < 3) return jsonResponse({ message: "flaky" }, 503);
      return jsonResponse({ result: { ok: true } });
    });
    const invoker = new Invoker({
      baseUrl: "http://s:18780",
      retry: { maxAttempts: 3, initialDelayMs: 1, jitterFactor: 0 },
    });

    const result = await invoker.invoke("fn", {});
    expect(result.payload).toEqual({ ok: true });
    expect(calls).toBe(3);
  });

  it("does not retry non-retryable client failures", async () => {
    let calls = 0;
    mockFetch(async () => {
      calls += 1;
      return jsonResponse({ message: "missing" }, 404);
    });
    const invoker = new Invoker({
      baseUrl: "http://s:18780",
      retry: { maxAttempts: 5, initialDelayMs: 1 },
    });

    await expect(invoker.invoke("fn", {})).rejects.toMatchObject({ status: 404 });
    expect(calls).toBe(1);
  });

  it("per-request retry overrides invoker retry", async () => {
    let calls = 0;
    mockFetch(async () => {
      calls += 1;
      return jsonResponse({ message: "down" }, 503);
    });
    const invoker = new Invoker({
      baseUrl: "http://s:18780",
      retry: { maxAttempts: 5, initialDelayMs: 1 },
    });

    await expect(
      invoker.invoke("fn", {}, { retry: { enabled: false } }),
    ).rejects.toMatchObject({ status: 503 });
    expect(calls).toBe(1);
  });

  it("retries startTask on 429", async () => {
    let calls = 0;
    mockFetch(async () => {
      calls += 1;
      if (calls === 1) return jsonResponse({ message: "rate" }, 429);
      return jsonResponse({ taskId: "t-9" });
    });
    const invoker = new Invoker({
      baseUrl: "http://s:18780",
      retry: { maxAttempts: 2, initialDelayMs: 1, jitterFactor: 0 },
    });

    await expect(invoker.startTask("fn", {})).resolves.toBe("t-9");
    expect(calls).toBe(2);
  });

  it("disabled retry executes exactly once", async () => {
    let calls = 0;
    mockFetch(async () => {
      calls += 1;
      return jsonResponse({ message: "boom" }, 500);
    });
    const invoker = new Invoker({
      baseUrl: "http://s:18780",
      retry: { enabled: false },
    });

    await expect(invoker.invoke("fn", {})).rejects.toMatchObject({ status: 500 });
    expect(calls).toBe(1);
  });

  it("extra retryable status codes are honored", async () => {
    let calls = 0;
    mockFetch(async () => {
      calls += 1;
      if (calls === 1) return jsonResponse({ message: "teapot" }, 418);
      return jsonResponse({ result: 1 });
    });
    const invoker = new Invoker({
      baseUrl: "http://s:18780",
      retry: {
        maxAttempts: 2,
        initialDelayMs: 1,
        jitterFactor: 0,
        retryableStatusCodes: [418],
      },
    });

    await expect(invoker.invoke("fn", {})).resolves.toEqual({ payload: 1 });
    expect(calls).toBe(2);
  });

  it("retry does not re-run schema validation errors", async () => {
    let calls = 0;
    mockFetch(async () => {
      calls += 1;
      return jsonResponse({ result: {} });
    });
    const invoker = new Invoker({
      baseUrl: "http://s:18780",
      retry: { maxAttempts: 3, initialDelayMs: 1 },
    });
    invoker.setSchema("fn", { type: "object", required: ["a"] });

    await expect(invoker.invoke("fn", {})).rejects.toMatchObject({
      code: "schema_validation",
    });
    expect(calls).toBe(0);
  });
});
