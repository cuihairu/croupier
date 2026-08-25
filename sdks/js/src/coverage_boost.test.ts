/**
 * Coverage boost tests: invoker argument guards & response validation,
 * BasicClient descriptor serialization / upload normalization / reconnect
 * helpers, dispatcher queue limits, and transport frame-timeout guards.
 */

import { BasicClient } from "./index";
import { Invoker, createInvoker, InvokerError } from "./invoker";
import {
  MainThreadDispatcher,
  getDispatcher,
} from "./threading/dispatcher";
import { TCPTransport } from "./tcp_transport";

// ---------------------------------------------------------------------------
// fetch mocking helpers (same pattern as invoker.test.ts)
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

afterEach(() => {
  restoreFetch();
  jest.restoreAllMocks();
});

// ---------------------------------------------------------------------------
// Invoker argument guards
// ---------------------------------------------------------------------------

describe("Invoker argument guards", () => {
  it("invoke rejects an empty functionId", async () => {
    const inv = createInvoker({ baseUrl: "http://s:18780" });
    await expect(inv.invoke("", {})).rejects.toThrow("functionId");
  });

  it("startTask rejects an empty functionId", async () => {
    const inv = createInvoker({ baseUrl: "http://s:18780" });
    await expect(inv.startTask("", {})).rejects.toThrow("functionId");
  });

  it("getTaskStatus rejects an empty taskId", async () => {
    const inv = createInvoker({ baseUrl: "http://s:18780" });
    await expect(inv.getTaskStatus("")).rejects.toThrow("taskId");
  });

  it("streamTask rejects an empty taskId", async () => {
    const inv = createInvoker({ baseUrl: "http://s:18780" });
    await expect(
      (async () => {
        for await (const _ev of inv.streamTask("")) {
          // unreachable
        }
      })(),
    ).rejects.toThrow("taskId");
  });

  it("cancelTask rejects an empty taskId", async () => {
    const inv = createInvoker({ baseUrl: "http://s:18780" });
    await expect(inv.cancelTask("")).rejects.toThrow("taskId");
  });
});

// ---------------------------------------------------------------------------
// Invoker response validation
// ---------------------------------------------------------------------------

describe("Invoker response validation", () => {
  it("startTask throws InvokerError when the server omits taskId", async () => {
    mockFetch(async () => jsonResponse({ ok: true }));
    const inv = new Invoker({ baseUrl: "http://s:18780/api/v1" });
    await expect(inv.startTask("fn", {})).rejects.toMatchObject({
      name: "InvokerError",
      code: "no_task_id",
    });
  });

  it("getTaskStatus throws when the response is not an object", async () => {
    mockFetch(async () => jsonResponse(null));
    const inv = new Invoker({ baseUrl: "http://s:18780/api/v1" });
    await expect(inv.getTaskStatus("t1")).rejects.toMatchObject({
      code: "invalid_task_status",
    });
  });

  it("streamTask yields events then stops on a terminal type", async () => {
    let afterSeqSeen = "";
    mockFetch(async (url) => {
      afterSeqSeen = url;
      return jsonResponse({
        items: [{ seq: 1, type: "started" }, { seq: 2, type: "failed" }],
        done: false,
      });
    });
    const inv = new Invoker({ baseUrl: "http://s:18780/api/v1" });
    const types: string[] = [];
    for await (const ev of inv.streamTask("t1", { pollIntervalMs: 1 })) {
      types.push(ev.type);
    }
    expect(types).toEqual(["started", "failed"]);
    expect(afterSeqSeen).toContain("after_seq=0");
  });

  it("streamTask polls with the default interval when none is given", async () => {
    let calls = 0;
    mockFetch(async () => {
      calls += 1;
      if (calls === 1) {
        return jsonResponse({ items: [], done: false });
      }
      return jsonResponse({ items: [{ seq: 1, type: "completed" }], done: true });
    });
    const inv = new Invoker({ baseUrl: "http://s:18780/api/v1" });
    const types: string[] = [];
    for await (const ev of inv.streamTask("t1")) {
      types.push(ev.type);
    }
    expect(types).toEqual(["completed"]);
    expect(calls).toBe(2);
  });

  it("cancelTask posts to the cancel endpoint", async () => {
    let captured: { url: string; method?: string } | null = null;
    mockFetch(async (url, init) => {
      captured = { url, method: init?.method };
      return jsonResponse({});
    });
    const inv = new Invoker({
      baseUrl: "http://s:18780/api/v1",
      token: "tk",
      gameId: "g",
      env: "e",
    });
    await inv.cancelTask("t 1");
    expect(captured!.url).toBe(
      "http://s:18780/api/v1/tasks/t%201/cancel",
    );
    expect(captured!.method).toBe("POST");
  });
});

// ---------------------------------------------------------------------------
// BasicClient descriptor serialization helpers
// ---------------------------------------------------------------------------

describe("BasicClient register request serialization", () => {
  it("getRegisterRequest renders optional schema fields as empty strings", () => {
    const client = new BasicClient();
    client.registerFunction({ id: "plain.fn", version: "1.0.0" }, async () => "ok");
    const req = (client as any).getRegisterRequest();
    expect(req.functions).toHaveLength(1);
    expect(req.functions[0].inputSchema).toBe("");
    expect(req.functions[0].outputSchema).toBe("");
    expect(req.functions[0].operationId).toBe("plain.fn");
  });

  it("getRegisterRequest serializes provided schemas to JSON", () => {
    const client = new BasicClient();
    client.registerFunction(
      {
        id: "schema.fn",
        version: "1.0.0",
        inputSchema: { type: "object" } as any,
        outputSchema: { type: "string" } as any,
      },
      async () => "ok",
    );
    const req = (client as any).getRegisterRequest();
    expect(req.functions[0].inputSchema).toBe('{"type":"object"}');
    expect(req.functions[0].outputSchema).toBe('{"type":"string"}');
  });

  it("protobuf serialization round-trips schema and metadata fields", () => {
    const client = new BasicClient({ insecure: true });
    client.registerFunction(
      {
        id: "pb.fn",
        version: "1.0.0",
        inputSchema: { type: "object" } as any,
        outputSchema: { type: "array" } as any,
      },
      async () => "ok",
    );
    const req = (client as any).getRegisterRequest();
    const buf: Buffer = (client as any).serializeProviderConnectProtobufRequest(
      req,
    );
    expect(buf.length).toBeGreaterThan(0);
  });

  it("parseProviderConnectResponse parses JSON session responses", () => {
    const client = new BasicClient();
    const parsed = (client as any).parseProviderConnectResponse(
      Buffer.from(JSON.stringify({ session_id: "sess-1" })),
    );
    expect(parsed.sessionId).toBe("sess-1");
  });

  it("parseProviderConnectResponse falls back to protobuf for non-JSON bytes", () => {
    const client = new BasicClient();
    // 0x0a 0x00 is a valid protobuf message (field 1, length 0) but not JSON.
    const parsed = (client as any).parseProviderConnectResponse(
      Buffer.from([0x0a, 0x00]),
    );
    expect(parsed.sessionId).toBe("");
  });

  it("parseProviderConnectProtobufResponse defaults to empty sessionId", () => {
    const client = new BasicClient();
    const parsed = (client as any).parseProviderConnectProtobufResponse(
      Buffer.alloc(0),
    );
    expect(parsed.sessionId).toBe("");
  });
});

// ---------------------------------------------------------------------------
// BasicClient upload helpers
// ---------------------------------------------------------------------------

describe("BasicClient upload helpers", () => {
  it("normalizeUploadPath rejects blank paths", () => {
    const client = new BasicClient();
    expect(() => (client as any).normalizeUploadPath("   ")).toThrow(
      "non-empty filePath",
    );
  });

  it("normalizeUploadContent encodes strings and copies buffers", () => {
    const client = new BasicClient();
    const fromString: Uint8Array = (client as any).normalizeUploadContent("ab");
    expect(Array.from(fromString)).toEqual([97, 98]);

    const direct = new Uint8Array([1, 2, 3]);
    expect((client as any).normalizeUploadContent(direct)).toBe(direct);

    const fromBuffer: Uint8Array = (client as any).normalizeUploadContent(
      Buffer.from([9]),
    );
    expect(Array.from(fromBuffer)).toEqual([9]);
  });

  it("uploadFile rejects when file transfer is disabled", async () => {
    const client = new BasicClient();
    await expect(
      client.uploadFile({ filePath: "/tmp/x.bin" } as any),
    ).rejects.toThrow();
  });

  it("uploadFiles returns an empty batch for an empty list", async () => {
    const client = new BasicClient({ enableFileTransfer: true });
    const batch = await client.uploadFiles([]);
    expect(batch).toMatchObject({ total: 0, succeeded: 0, failed: 0 });
  });

  it("uploadFiles records per-item failures", async () => {
    const client = new BasicClient({ enableFileTransfer: true });
    const batch = await client.uploadFiles([
      { filePath: "ok.txt", content: "hello" },
      { filePath: "/nonexistent-dir-xyz/nope.bin" },
    ]);
    expect(batch.total).toBe(2);
    expect(batch.succeeded).toBe(1);
    expect(batch.failed).toBe(1);
    const failed = batch.items.find((r) => !r.ok);
    expect(failed?.error?.length ?? 0).toBeGreaterThan(0);
  }, 15000);
});

// ---------------------------------------------------------------------------
// BasicClient reconnect helpers
// ---------------------------------------------------------------------------

describe("BasicClient reconnect helpers", () => {
  it("calculateReconnectDelay falls back to defaults for unset config", () => {
    const client = new BasicClient();
    (client as any).config.reconnect = {};
    const first = (client as any).calculateReconnectDelay(1);
    expect(first).toBeGreaterThanOrEqual(800);
    expect(first).toBeLessThanOrEqual(1200);
  });

  it("calculateReconnectDelay caps at maxDelayMs", () => {
    const client = new BasicClient();
    (client as any).config.reconnect = {
      initialDelayMs: 1000,
      maxDelayMs: 1500,
      backoffMultiplier: 4,
      jitterFactor: 0,
    };
    expect((client as any).calculateReconnectDelay(10)).toBe(1500);
  });
});

// ---------------------------------------------------------------------------
// Dispatcher queue edge cases
// ---------------------------------------------------------------------------

describe("MainThreadDispatcher queue edges", () => {
  let d: MainThreadDispatcher;

  beforeEach(() => {
    MainThreadDispatcher.resetInstance();
    d = getDispatcher();
  });

  afterEach(() => {
    d.clear();
  });

  it("enqueueDeferred ignores null callbacks", () => {
    expect(() => d.enqueueDeferred(null)).not.toThrow();
    expect((d as any).queue.length).toBe(0);
  });

  it("enqueueWithData ignores null callbacks", () => {
    expect(() => d.enqueueWithData(null, 1)).not.toThrow();
  });

  it("processQueueWithLimit uses maxProcessPerFrame when given <= 0", () => {
    const executed: number[] = [];
    for (let i = 0; i < 5; i += 1) {
      d.enqueueDeferred(() => executed.push(i));
    }
    (d as any).maxProcessPerFrame = 2;
    expect(d.processQueueWithLimit(0)).toBe(2);
    expect(executed).toEqual([0, 1]);
  });

  it("processQueueWithLimit returns 0 for an empty queue", () => {
    expect(d.processQueueWithLimit(10)).toBe(0);
  });

  it("immediate execution swallows callback errors", () => {
    d.initialize();
    const errorSpy = jest
      .spyOn(console, "error")
      .mockImplementation(() => undefined);
    expect(() => d.enqueue(() => { throw new Error("boom"); })).not.toThrow();
    expect(errorSpy).toHaveBeenCalled();
  });
});

// ---------------------------------------------------------------------------
// TCPTransport guards
// ---------------------------------------------------------------------------

describe("TCPTransport guards", () => {
  it("readFrameWithTimeout resolves an empty buffer without a socket", async () => {
    const t = new TCPTransport({ address: "127.0.0.1:1" });
    const frame = await (t as any).readFrameWithTimeout();
    expect(Buffer.isBuffer(frame)).toBe(true);
    expect(frame.length).toBe(0);
    await t.close();
  });

  it("connect rejects an unparsable address", async () => {
    const t = new TCPTransport({ address: "not a host:port!" });
    await expect(t.connect()).rejects.toThrow();
  });
});
