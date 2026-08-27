/**
 * Third coverage boost: openapi.ts edge branches (id derivation fallbacks,
 * schema conversion guards, extension value forms, missing-resolver path,
 * registration failures), plus client/invoker behaviour corners.
 */

import { BasicClient } from "./index";
import { Invoker } from "./invoker";
import { registerFromOpenAPI } from "./openapi";

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

function descriptorsOf(client: BasicClient): Map<string, any> {
  return (client as any).descriptors as Map<string, any>;
}

afterEach(() => {
  restoreFetch();
  jest.restoreAllMocks();
});

// ---------------------------------------------------------------------------
// deriveOperationId fallbacks (via full import path)
// ---------------------------------------------------------------------------

const makeHandler = () => (_ctx: string, _payload: string) => "{}";

describe("openapi id/summary derivation fallbacks", () => {
  it("falls back to unknown.function when operationId and path are missing", () => {
    const client = new BasicClient();
    // Empty path segments: path "/" yields no segments.
    const registered = registerFromOpenAPI(
      client,
      { paths: { "/": { get: { tags: [] } } } },
      undefined,
      (id) => (id === "unknown.function" ? async () => "{}" : undefined),
    );
    expect(registered).toEqual(["unknown.function"]);
    expect(descriptorsOf(client).get("unknown.function").summary).toBe(
      "Unnamed Function",
    );
  });

  it("derives the summary from the path when operationId is absent", () => {
    const client = new BasicClient();
    registerFromOpenAPI(
      client,
      { paths: { "/player/ban": { put: {} } } },
      undefined,
      () => makeHandler(),
    );
    expect(descriptorsOf(client).get("player.ban").summary).toBe("Player.ban");
  });
});

// ---------------------------------------------------------------------------
// schema conversion guards
// ---------------------------------------------------------------------------

describe("openapi schema conversion guards", () => {
  it("omits schemas for empty or malformed request bodies", () => {
    const client = new BasicClient();
    registerFromOpenAPI(
      client,
      {
        paths: {
          "/a": {
            post: {
              requestBody: { content: { "application/json": { schema: {} } } },
              responses: {},
            },
          },
          "/b": {
            post: {
              requestBody: { content: { "application/json": {} } },
              responses: {},
            },
          },
          "/c": {
            post: {
              requestBody: "not-an-object",
              responses: {},
            },
          },
        },
      },
      undefined,
      () => makeHandler(),
    );
    for (const id of ["a", "b", "c"]) {
      expect(descriptorsOf(client).get(id).inputSchema).toBeUndefined();
    }
  });

  it("keeps descriptions and defaults missing property types to object", () => {
    const client = new BasicClient();
    registerFromOpenAPI(
      client,
      {
        paths: {
          "/d": {
            post: {
              requestBody: {
                content: {
                  "application/json": {
                    schema: {
                      description: "request shape",
                      properties: {
                        typed: { type: "integer", description: "a number" },
                        untyped: {},
                        scalar: "not-a-schema",
                      },
                    },
                  },
                },
              },
              responses: {},
            },
          },
        },
      },
      undefined,
      () => makeHandler(),
    );
    const schema = descriptorsOf(client).get("d").inputSchema;
    expect(schema.description).toBe("request shape");
    expect(schema.properties.typed).toEqual({
      type: "integer",
      description: "a number",
    });
    expect(schema.properties.untyped).toEqual({ type: "object" });
    expect(schema.properties.scalar).toBeUndefined();
    expect(schema.required).toBeUndefined();
  });

  it("ignores empty descriptions and empty required arrays", () => {
    const client = new BasicClient();
    registerFromOpenAPI(
      client,
      {
        paths: {
          "/e": {
            post: {
              requestBody: {
                content: {
                  "application/json": {
                    schema: { description: "", required: [], type: "object" },
                  },
                },
              },
              responses: {},
            },
          },
        },
      },
      undefined,
      () => makeHandler(),
    );
    const schema = descriptorsOf(client).get("e").inputSchema;
    expect(schema.type).toBe("object");
    expect(schema.description).toBeUndefined();
    expect(schema.required).toBeUndefined();
  });
});

// ---------------------------------------------------------------------------
// extension value forms and risk mapping
// ---------------------------------------------------------------------------

describe("openapi extension extraction", () => {
  const spec = (extensions: Record<string, unknown>) => ({
    paths: { "/x": { post: { operationId: "x_post", responses: {}, ...extensions } } },
  });

  it("stringifies numeric and object extension values", () => {
    const client = new BasicClient();
    registerFromOpenAPI(
      client,
      spec({ "x-resource": 42, "x-permission": { role: "gm" } }),
      undefined,
      () => makeHandler(),
    );
    const descriptor = descriptorsOf(client).get("x_post");
    expect(descriptor.resource).toBe("42");
    expect(descriptor.permission).toBe('{"role":"gm"}');
  });

  it("maps every x-risk alias to the v2 vocabulary", () => {
    const cases: Array<[unknown, string]> = [
      ["safe", "safe"],
      ["LOW", "safe"],
      ["medium", "warning"],
      ["MODERATE", "warning"],
      ["high", "high"],
      ["critical", "danger"],
      ["DANGER", "danger"],
    ];
    for (const [risk, expected] of cases) {
      const client = new BasicClient();
      registerFromOpenAPI(client, spec({ "x-risk": risk }), undefined, () => makeHandler());
      expect(descriptorsOf(client).get("x_post").risk).toBe(expected);
    }
  });

  it("rejects unknown x-risk values", () => {
    const client = new BasicClient();
    const action = () =>
      registerFromOpenAPI(client, spec({ "x-risk": "unknown-word" }), undefined, () => makeHandler());
    expect(action).toThrow("invalid x-risk");
  });
});

// ---------------------------------------------------------------------------
// resolver / registration failure paths
// ---------------------------------------------------------------------------

describe("openapi resolver and registration failures", () => {
  it("throws when no resolver or handlers are provided", () => {
    const client = new BasicClient();
    const action = () =>
      registerFromOpenAPI(client, { paths: { "/a": { get: {} } } }, undefined, undefined, undefined as any);
    expect(action).toThrow("no handler provided for function: a");
  });

  it("reports registration failures with the function id", () => {
    const client = new BasicClient();
    (client as any).connected = true; // BasicClient rejects registration while connected

    const action = () =>
      registerFromOpenAPI(
        client,
        { paths: { "/a": { get: {} } } },
        undefined,
        () => makeHandler(),
      );
    expect(action).toThrow("register function a failed");

    (client as any).connected = false;
  });

  it("continues past registration failures when continueOnError is set", () => {
    const client = new BasicClient();
    const original = client.registerFunction.bind(client);
    jest.spyOn(client, "registerFunction").mockImplementation((descriptor, handler) => {
      if (descriptor.id === "a") throw new Error("duplicate");
      return original(descriptor, handler);
    });

    const registered = registerFromOpenAPI(
      client,
      { paths: { "/a": { get: {} }, "/b": { get: {} } } },
      { continueOnError: true },
      () => makeHandler(),
    );
    expect(registered).toEqual(["b"]);
  });
});

// ---------------------------------------------------------------------------
// client & invoker corners
// ---------------------------------------------------------------------------

describe("client and invoker corners", () => {
  it("uploadFiles reports failures for unreadable files", async () => {
    const client = new BasicClient({ enableFileTransfer: true });
    const batch = await client.uploadFiles([
      { filePath: "/definitely-missing-dir-xyz/nope.bin" },
      { filePath: "/also-missing/dir-abc/empty.txt" },
    ]);
    expect(batch.total).toBe(2);
    expect(batch.succeeded).toBe(0);
    expect(batch.failed).toBe(2);
    expect(batch.items.every((item) => !item.ok)).toBe(true);
  }, 15000);

  it("uploadFileStream rejects when transfer is disabled", async () => {
    const client = new BasicClient();
    const { Readable } = await import("node:stream");
    await expect(
      client.uploadFileStream({
        filePath: "x.bin",
        stream: Readable.from(["data"]),
      }),
    ).rejects.toThrow();
  });

  it("invoker streamTask stops when server marks done without events", async () => {
    let calls = 0;
    mockFetch(async () => {
      calls += 1;
      return jsonResponse({ items: [], done: true });
    });
    const invoker = new Invoker({ baseUrl: "http://s:18780" });

    const events: unknown[] = [];
    for await (const event of invoker.streamTask("t1")) {
      events.push(event);
    }
    expect(events).toEqual([]);
    expect(calls).toBe(1);
  });

  it("invoker emits progress events with payloads", async () => {
    mockFetch(async () =>
      jsonResponse({
        items: [
          { seq: 1, type: "progress", progress: 10, payload: { step: 1 } },
          { seq: 2, type: "progress", progress: 90, payload: { step: 2 } },
          { seq: 3, type: "completed", payload: { ok: true } },
        ],
        done: true,
      }),
    );
    const invoker = new Invoker({ baseUrl: "http://s:18780" });

    const types: string[] = [];
    const progresses: number[] = [];
    for await (const event of invoker.streamTask("t1")) {
      types.push(event.type);
      if (event.progress !== undefined) progresses.push(event.progress);
    }
    expect(types).toEqual(["progress", "progress", "completed"]);
    expect(progresses).toEqual([10, 90]);
  });

  it("invoker getTaskStatus fills the id when the server omits it", async () => {
    mockFetch(async () => jsonResponse({ status: "queued", progress: 0 }));
    const invoker = new Invoker({ baseUrl: "http://s:18780" });

    const status = await invoker.getTaskStatus("requested-id");
    expect(status.id).toBe("requested-id");
    expect(status.status).toBe("queued");
  });

  it("invoker getTaskStatus rejects non-object responses", async () => {
    mockFetch(async () => jsonResponse("just-a-string"));
    const invoker = new Invoker({ baseUrl: "http://s:18780" });

    await expect(invoker.getTaskStatus("t1")).rejects.toMatchObject({
      code: "invalid_task_status",
    });
  });

  it("invoker honors per-request headers over config headers", async () => {
    let auth: string | undefined;
    mockFetch(async (url, init) => {
      const headers = init?.headers as Record<string, string>;
      auth = headers?.Authorization;
      return jsonResponse({ result: {} });
    });
    const invoker = new Invoker({
      baseUrl: "http://s:18780",
      token: "config-token",
    });

    await invoker.invoke("fn", {}, { headers: { Authorization: "Bearer request-token" } });
    expect(auth).toBe("Bearer request-token");
  });

  it("invoker constructs with minimum config and normalizes root URLs", () => {
    const invoker = new Invoker({ baseUrl: "server.example" });
    expect((invoker as any).config.baseUrl).toBe("http://server.example/api/v1");
  });
});
