/**
 * Second coverage boost: client lifecycle edges, protocol parity,
 * OpenAPI corner cases and invoker retry/schema corners.
 */

import * as protobuf from "protobufjs";
import { BasicClient } from "./index";
import { Invoker } from "./invoker";
import { registerFromOpenAPI, ImportOptions } from "./openapi";
import {
  getMsgID,
  isRequest,
  isResponse,
  msgIdString,
  newMessage,
  parseMessage,
  putMsgID,
  MSG_INVOKE_REQUEST,
  MSG_INVOKE_RESPONSE,
  MSG_PROVIDER_CONNECT_REQUEST,
  MSG_PROVIDER_DRAIN_REQUEST,
  MSG_PROVIDER_DRAIN_RESPONSE,
} from "./protocol";
import { TCPTransport } from "./tcp_transport";

const providerRoot = protobuf.parse(`
syntax = "proto3";
package croupier.sdk.v1;
message ProviderConnectResponse { string session_id = 1; }
`).root;
const ProviderConnectResponseMessage = providerRoot.lookupType(
  "croupier.sdk.v1.ProviderConnectResponse",
);

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

afterEach(() => {
  restoreFetch();
  jest.restoreAllMocks();
});

// ---------------------------------------------------------------------------
// Protocol helpers
// ---------------------------------------------------------------------------

describe("protocol helpers", () => {
  it("round-trips every header field", () => {
    const body = Buffer.from([1, 2, 3]);
    const message = newMessage(MSG_INVOKE_REQUEST, 0x00abcdef, body);
    const parsed = parseMessage(message);
    expect(parsed.version).toBe(1);
    expect(parsed.msgId).toBe(MSG_INVOKE_REQUEST);
    expect(parsed.reqId).toBe(0x00abcdef);
    expect(Buffer.from(parsed.body)).toEqual(body);
  });

  it("encodes 24-bit message IDs including boundaries", () => {
    expect(getMsgID(putMsgID(0))).toBe(0);
    expect(getMsgID(putMsgID(0xffffff))).toBe(0x00ff_ffff);
    expect(getMsgID(putMsgID(MSG_PROVIDER_DRAIN_REQUEST))).toBe(
      MSG_PROVIDER_DRAIN_REQUEST,
    );
  });

  it("classifies requests and responses by parity", () => {
    expect(isRequest(MSG_INVOKE_REQUEST)).toBe(true);
    expect(isResponse(MSG_INVOKE_RESPONSE)).toBe(true);
    expect(isRequest(MSG_INVOKE_RESPONSE)).toBe(false);
    expect(isResponse(MSG_INVOKE_REQUEST)).toBe(false);
  });

  it("names known IDs and falls back for unknown ones", () => {
    expect(msgIdString(MSG_PROVIDER_CONNECT_REQUEST)).toBe(
      "ProviderConnectRequest",
    );
    expect(msgIdString(MSG_PROVIDER_DRAIN_RESPONSE)).toBe(
      "ProviderDrainResponse",
    );
    expect(msgIdString(0x99aabb)).toContain("Unknown");
  });
});

// ---------------------------------------------------------------------------
// Client lifecycle edges
// ---------------------------------------------------------------------------

describe("BasicClient lifecycle edges", () => {
  it("rejects registration while connected", () => {
    const client = new BasicClient();
    client.registerFunction({ id: "a.fn", version: "1.0.0" }, async () => "ok");
    (client as any).connected = true;

    expect(() =>
      client.registerFunction({ id: "b.fn", version: "1.0.0" }, async () => "ok"),
    ).toThrow("connected");

    (client as any).connected = false;
  });

  it("rejects invoke for unknown functions", async () => {
    const client = new BasicClient();
    await expect(client.invoke("ghost.fn", "{}")).rejects.toThrow();
  });

  it("builds manifests with optional fields only when present", () => {
    const client = new BasicClient();
    client.registerFunction(
      {
        id: "full.fn",
        version: "2.0.0",
        resource: "player",
        operation: "ban",
        risk: "danger",
        permission: "player.ban",
        deprecated: true,
        input_schema: { type: "object" } as any,
      },
      async () => "ok",
    );
    client.registerFunction({ id: "bare.fn" as string, version: "1.0.0" }, async () => "ok");

    const manifest = (client as any).buildManifest();
    const full = manifest.functions.find((f: any) => f.id === "full.fn");
    const bare = manifest.functions.find((f: any) => f.id === "bare.fn");

    expect(full.input_schema).toEqual({ type: "object" });
    expect(full.risk).toBe("danger");
    expect(bare.input_schema).toBeUndefined();
    expect(manifest.provider.lang).toBe("node");
    expect(manifest.functions).toHaveLength(2);
  });

  it("disconnect is idempotent when never connected", async () => {
    const client = new BasicClient();
    await expect(client.disconnect()).resolves.not.toThrow();
    expect((client as any).connected).toBe(false);
  });

  it("connect registers every descriptor and stores the session id", async () => {
    const seen: Buffer[] = [];
    jest.spyOn(TCPTransport.prototype, "connect").mockImplementation(async () => {});
    jest
      .spyOn(TCPTransport.prototype, "call")
      .mockImplementation(async (msgType: number, data: Buffer) => {
        if (msgType === MSG_PROVIDER_CONNECT_REQUEST) {
          seen.push(Buffer.from(data));
          return [
            msgType + 1,
            Buffer.from(
              ProviderConnectResponseMessage.encode(
                ProviderConnectResponseMessage.create({ sessionId: "sess-42" }),
              ).finish(),
            ),
          ];
        }
        if (msgType === 0x050103) {
          return [msgType + 1, Buffer.alloc(0)]; // heartbeat
        }
        throw new Error(`unexpected msgType ${msgType}`);
      });
    jest.spyOn(TCPTransport.prototype, "close").mockImplementation(() => {});

    const client = new BasicClient({ heartbeatIntervalSeconds: 60 });
    client.registerFunction({ id: "one.fn", version: "1.0.0" }, async () => "1");
    client.registerFunction({ id: "two.fn", version: "1.0.0" }, async () => "2");

    await client.connect();

    expect(seen).toHaveLength(1);
    expect((client as any).sessionId).toBe("sess-42");
    expect((client as any).connected).toBe(true);

    await client.disconnect();
    expect((client as any).connected).toBe(false);
  });

  it("reconnect settings are normalized with defaults", () => {
    const client = new BasicClient({});
    const reconnect = (client as any).config.reconnect;
    expect(reconnect.maxAttempts).toBeDefined();
    expect(reconnect.initialDelayMs).toBeGreaterThan(0);

    const custom = new BasicClient({
      reconnect: { initialDelayMs: 250, maxAttempts: 7 },
    } as any);
    expect((custom as any).config.reconnect.initialDelayMs).toBe(250);
    expect((custom as any).config.reconnect.maxAttempts).toBe(7);
  });

  it("startTask/streamTask/cancelTask work locally without a connection", async () => {
    const client = new BasicClient();
    let release: () => void = () => {};
    const gate = new Promise<void>((resolve) => { release = resolve; });
    client.registerFunction(
      { id: "job.fn", version: "1.0.0" },
      async () => {
        await gate;
        return "done-payload";
      },
    );

    const taskId = client.startTask("job.fn", "{}");
    expect(typeof taskId).toBe("string");

    // Stream in the background and cancel once the task has started.
    const events: string[] = [];
    const consumer = (async () => {
      for await (const event of client.streamTask(taskId)) {
        events.push(event.type);
      }
    })();
    await new Promise((resolve) => {
      const check = () => {
        if (events.includes("started")) resolve(undefined);
        else setTimeout(check, 10);
      };
      check();
    });

    expect(client.cancelTask(taskId)).toBe(true);
    release();
    await consumer;

    expect(events).toContain("cancelled");
    expect(client.cancelTask("never-existed")).toBe(false);
  });

  it("streamTask surfaces handler failures as error events", async () => {
    const client = new BasicClient();
    client.registerFunction(
      { id: "bad.fn", version: "1.0.0" },
      async () => {
        throw new Error("handler exploded");
      },
    );

    const taskId = client.startTask("bad.fn", "{}");
    const types: string[] = [];
    for await (const event of client.streamTask(taskId)) {
      types.push(event.type);
    }
    expect(types).toContain("error");
  });
});

// ---------------------------------------------------------------------------
// OpenAPI corner cases
// ---------------------------------------------------------------------------

describe("registerFromOpenAPI corner cases", () => {
  const resolver = async () => "{}";

  it("ignores non-string tags", () => {
    const client = new BasicClient();
    registerFromOpenAPI(
      client,
      { paths: { "/a": { post: { operationId: "a_post", tags: ["ok", 5, null] } } } },
      undefined,
      undefined,
      new Map([["a_post", resolver]]),
    );
    const descriptor = ((client as any).descriptors as Map<string, any>).get("a_post")!;
    expect(descriptor.tags).toEqual(["ok"]);
  });

  it("skips non-object path items", () => {
    const client = new BasicClient();
    const registered = registerFromOpenAPI(
      client,
      { paths: { "/bad": "nope", "/ok": { get: { operationId: "ok_get" } } } },
      undefined,
      undefined,
      new Map([["ok_get", resolver]]),
    );
    expect(registered).toEqual(["ok_get"]);
  });

  it("treats non-object specs as invalid", () => {
    const client = new BasicClient();
    expect(() =>
      registerFromOpenAPI(client, "[1,2,3]", undefined, undefined, new Map()),
    ).toThrow("paths");
  });

  it("empty prefixes are no-ops", () => {
    const client = new BasicClient();
    const options: ImportOptions = { resourcePrefix: "", tagPrefix: "" };
    registerFromOpenAPI(
      client,
      {
        paths: {
          "/a": { get: { operationId: "a_get", "x-resource": "thing", tags: ["t"] } },
        },
      },
      options,
      undefined,
      new Map([["a_get", resolver]]),
    );
    const descriptor = ((client as any).descriptors as Map<string, any>).get("a_get")!;
    expect(descriptor.resource).toBe("thing");
    expect(descriptor.tags).toEqual(["t"]);
  });

  it("handles specs with empty operations per path", () => {
    const client = new BasicClient();
    const registered = registerFromOpenAPI(
      client,
      { paths: { "/empty": {} } },
      undefined,
      undefined,
      new Map(),
    );
    expect(registered).toEqual([]);
  });

  it("derives ids deterministically for repeated paths", () => {
    const client = new BasicClient();
    const seen: string[] = [];
    registerFromOpenAPI(
      client,
      { paths: { "/x/y": { get: {}, post: {} } } },
      undefined,
      (functionId) => {
        seen.push(functionId);
        return resolver;
      },
    );
    // Both methods derive the same id; both resolve through the resolver.
    expect(seen.length).toBe(2);
  });
});

// ---------------------------------------------------------------------------
// Invoker retry and schema corners
// ---------------------------------------------------------------------------

describe("Invoker retry corners", () => {
  it("retries network-level failures (TypeError)", async () => {
    let calls = 0;
    mockFetch(async () => {
      calls += 1;
      if (calls === 1) {
        throw new TypeError("fetch failed");
      }
      return jsonResponse({ result: "recovered" });
    });
    const invoker = new Invoker({
      baseUrl: "http://s:18780",
      retry: { maxAttempts: 3, initialDelayMs: 1, jitterFactor: 0 },
    });

    await expect(invoker.invoke("fn", {})).resolves.toEqual({
      payload: "recovered",
    });
    expect(calls).toBe(2);
  });

  it("getTaskStatus also honors retry configuration", async () => {
    let calls = 0;
    mockFetch(async () => {
      calls += 1;
      if (calls === 1) return jsonResponse({ message: "boom" }, 502);
      return jsonResponse({ id: "t1", status: "running" });
    });
    const invoker = new Invoker({
      baseUrl: "http://s:18780",
      retry: { maxAttempts: 2, initialDelayMs: 1, jitterFactor: 0 },
    });

    await expect(invoker.getTaskStatus("t1")).resolves.toMatchObject({
      id: "t1",
      status: "running",
    });
    expect(calls).toBe(2);
  });

  it("cancelTask fails fast on non-retryable statuses", async () => {
    let calls = 0;
    mockFetch(async () => {
      calls += 1;
      return jsonResponse({ message: "nope" }, 403);
    });
    const invoker = new Invoker({
      baseUrl: "http://s:18780",
      retry: { maxAttempts: 5, initialDelayMs: 1 },
    });

    await expect(invoker.cancelTask("t1")).rejects.toMatchObject({ status: 403 });
    expect(calls).toBe(1);
  });

  it("validates schemas with local $ref before any network call", async () => {
    let calls = 0;
    mockFetch(async () => {
      calls += 1;
      return jsonResponse({ result: {} });
    });
    const invoker = new Invoker({ baseUrl: "http://s:18780" });
    invoker.setSchema("fn", {
      definitions: { name: { type: "string", minLength: 2 } },
      type: "object",
      properties: { name: { $ref: "#/definitions/name" } },
      required: ["name"],
    });

    await expect(invoker.invoke("fn", { name: "ab" })).resolves.toBeDefined();
    await expect(invoker.invoke("fn", { name: "x" })).rejects.toMatchObject({
      code: "schema_validation",
    });
    await expect(invoker.invoke("fn", {})).rejects.toMatchObject({
      code: "schema_validation",
    });
    expect(calls).toBe(1); // only the valid call reached the network
  });

  it("validates type unions in schemas", async () => {
    mockFetch(async () => jsonResponse({ result: {} }));
    const invoker = new Invoker({ baseUrl: "http://s:18780" });
    invoker.setSchema("fn", {
      type: "object",
      properties: { value: { type: ["string", "null"] } },
    });

    await expect(invoker.invoke("fn", { value: "x" })).resolves.toBeDefined();
    await expect(invoker.invoke("fn", { value: null })).resolves.toBeDefined();
    await expect(invoker.invoke("fn", { value: 3 })).rejects.toMatchObject({
      code: "schema_validation",
    });
  });
});
