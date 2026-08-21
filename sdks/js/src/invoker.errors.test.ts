/**
 * Invoker edge-case tests: error parsing, construction validation, timeout
 * aborts, task lifecycle polling and InvokerEventSource behavior.
 */

import {
  Invoker,
  InvokerError,
  InvokerEventSource,
  createInvoker,
} from "./invoker";

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

afterEach(() => restoreFetch());

describe("Invoker construction (edge cases)", () => {
  it("rejects non-HTTP schemes", () => {
    expect(() => new Invoker({ baseUrl: "ftp://server:21" })).toThrow("HTTP(S)");
    expect(() => new Invoker({ baseUrl: "ws://server" })).toThrow("HTTP(S)");
  });

  it("appends /api/v1 to a base path that lacks it", () => {
    const inv = new Invoker({ baseUrl: "https://h/base/" });
    expect((inv as any).config.baseUrl).toBe("https://h/base/api/v1");
  });

  it("keeps a baseUrl that already ends with /api/v1", () => {
    const inv = new Invoker({ baseUrl: "https://h/api/v1" });
    expect((inv as any).config.baseUrl).toBe("https://h/api/v1");
  });

  it("strips query and fragment from baseUrl", () => {
    const inv = new Invoker({ baseUrl: "https://h/api/v1?x=1#frag" });
    expect((inv as any).config.baseUrl).toBe("https://h/api/v1");
  });
});

describe("Invoker error parsing", () => {
  it("falls back to statusText when the body is not JSON", async () => {
    mockFetch(async () =>
      new Response("boom", { status: 418, statusText: "teapot" }),
    );
    const inv = new Invoker({ baseUrl: "https://h/api/v1" });
    const err = await inv.invoke("f").catch((e) => e);
    expect(err).toBeInstanceOf(InvokerError);
    expect(err.message).toBe("teapot");
    expect(err.status).toBe(418);
  });

  it("falls back to 'request failed' when the body is unusable and statusText empty", async () => {
    mockFetch(async () => new Response("not json", { status: 502 }));
    const inv = new Invoker({ baseUrl: "https://h/api/v1" });
    const err = await inv.startTask("f").catch((e) => e);
    expect(err).toBeInstanceOf(InvokerError);
    expect(err.message).toBe("request failed");
    expect(err.status).toBe(502);
    expect(err.code).toBeUndefined();
  });

  it("propagates structured error details", async () => {
    mockFetch(async () =>
      jsonResponse(
        {
          error: "validation_failed",
          message: "invalid",
          details: { gameId: "required" },
        },
        400,
      ),
    );
    const inv = new Invoker({ baseUrl: "https://h/api/v1" });
    const err = await inv.cancelTask("t-1").catch((e) => e);
    expect(err).toBeInstanceOf(InvokerError);
    expect(err.code).toBe("validation_failed");
    expect(err.details).toEqual({ gameId: "required" });
  });
});

describe("Invoker.invoke (edge cases)", () => {
  it("requires a functionId", async () => {
    const inv = new Invoker({ baseUrl: "https://h/api/v1" });
    await expect(inv.invoke("")).rejects.toThrow("functionId");
  });

  it("throws no_result when the payload lacks a result field", async () => {
    mockFetch(async () => jsonResponse({ ok: true }));
    const inv = new Invoker({ baseUrl: "https://h/api/v1" });
    await expect(inv.invoke("f")).rejects.toMatchObject({
      status: 502,
      code: "no_result",
    });
  });

  it("aborts the request when the per-call timeout elapses", async () => {
    mockFetch(
      (_url, init) =>
        new Promise<Response>((_resolve, reject) => {
          init?.signal?.addEventListener("abort", () =>
            reject(new DOMException("aborted", "AbortError")),
          );
        }),
    );
    const inv = new Invoker({ baseUrl: "https://h/api/v1" });
    await expect(inv.invoke("f", {}, { timeout: 50 })).rejects.toThrow(
      /aborted/i,
    );
  });

  it("encodes function ids in the URL", async () => {
    let url = "";
    mockFetch(async (u) => {
      url = u;
      return jsonResponse({ result: 1 });
    });
    const inv = new Invoker({ baseUrl: "https://h/api/v1" });
    await inv.invoke("player/ban all");
    expect(url).toBe("https://h/api/v1/functions/player%2Fban%20all/invoke");
  });

  it("per-call headers override configured headers", async () => {
    let headers: Record<string, string> = {};
    mockFetch(async (_u, init) => {
      headers = (init?.headers as Record<string, string>) || {};
      return jsonResponse({ result: 1 });
    });
    const inv = new Invoker({
      baseUrl: "https://h/api/v1",
      headers: { "X-Trace": "base", "X-Extra": "keep" },
    });
    await inv.invoke("f", {}, { headers: { "X-Trace": "call" } });
    expect(headers["X-Trace"]).toBe("call");
    expect(headers["X-Extra"]).toBe("keep");
  });
});

describe("Invoker.startTask (edge cases)", () => {
  it("requires a functionId", async () => {
    const inv = new Invoker({ baseUrl: "https://h/api/v1" });
    await expect(inv.startTask("")).rejects.toThrow("functionId");
  });

  it("attaches Idempotency-Key when provided", async () => {
    let headers: Record<string, string> = {};
    mockFetch(async (_u, init) => {
      headers = (init?.headers as Record<string, string>) || {};
      return jsonResponse({ taskId: "t" });
    });
    const inv = new Invoker({ baseUrl: "https://h/api/v1" });
    await inv.startTask("f", {}, { idempotencyKey: "idem-7" });
    expect(headers["Idempotency-Key"]).toBe("idem-7");
  });

  it("throws when the server returns a falsy taskId", async () => {
    mockFetch(async () => jsonResponse({ taskId: 0 }));
    const inv = new Invoker({ baseUrl: "https://h/api/v1" });
    await expect(inv.startTask("f")).rejects.toMatchObject({
      status: 502,
      code: "no_task_id",
    });
  });
});

describe("Invoker.getTaskStatus (edge cases)", () => {
  it("requires a taskId", async () => {
    const inv = new Invoker({ baseUrl: "https://h/api/v1" });
    await expect(inv.getTaskStatus("")).rejects.toThrow("taskId");
  });

  it("rejects non-object status payloads", async () => {
    mockFetch(async () => jsonResponse(123));
    const inv = new Invoker({ baseUrl: "https://h/api/v1" });
    await expect(inv.getTaskStatus("t-1")).rejects.toMatchObject({
      status: 502,
      code: "invalid_task_status",
    });
  });

  it("substitutes the requested taskId when the payload omits id", async () => {
    mockFetch(async () => jsonResponse({ status: "running" }));
    const inv = new Invoker({ baseUrl: "https://h/api/v1" });
    const task = await inv.getTaskStatus("requested-id");
    expect(task.id).toBe("requested-id");
    expect(task.status).toBe("running");
  });

  it("encodes task ids in the URL", async () => {
    let url = "";
    mockFetch(async (u) => {
      url = u;
      return jsonResponse({ id: "a b" });
    });
    const inv = new Invoker({ baseUrl: "https://h/api/v1" });
    await inv.getTaskStatus("a b");
    expect(url).toBe("https://h/api/v1/tasks/a%20b");
  });
});

describe("Invoker.streamTask (edge cases)", () => {
  it("requires a taskId", async () => {
    const inv = new Invoker({ baseUrl: "https://h/api/v1" });
    await expect(async () => {
      for await (const _ev of inv.streamTask("")) {
        // unreachable
      }
    }).rejects.toThrow("taskId");
  });

  it("stops immediately when done=true with no events", async () => {
    let polls = 0;
    mockFetch(async () => {
      polls += 1;
      return jsonResponse({ items: [], done: true });
    });
    const inv = new Invoker({ baseUrl: "https://h/api/v1" });
    const events: unknown[] = [];
    for await (const ev of inv.streamTask("t-1", { pollIntervalMs: 0 })) {
      events.push(ev);
    }
    expect(events).toEqual([]);
    expect(polls).toBe(1);
  });

  it("stops at a terminal event even when done=false", async () => {
    const batches = [
      { items: [{ seq: 1, type: "progress", progress: 10 }], done: false },
      { items: [{ seq: 2, type: "failed", message: "boom" }], done: false },
      { items: [{ seq: 3, type: "progress", progress: 99 }], done: true },
    ];
    let call = 0;
    mockFetch(async () => jsonResponse(batches[Math.min(call++, batches.length - 1)]));
    const inv = new Invoker({ baseUrl: "https://h/api/v1" });
    const types: string[] = [];
    for await (const ev of inv.streamTask("t-1", { pollIntervalMs: 1 })) {
      types.push(ev.type);
    }
    expect(types).toEqual(["progress", "failed"]);
  });

  it("advances the after_seq cursor between polls", async () => {
    const seenUrls: string[] = [];
    const batches = [
      { items: [{ seq: 5, type: "started" }], done: false },
      { items: [{ seq: 6, type: "completed" }], done: true },
    ];
    let call = 0;
    mockFetch(async (url) => {
      seenUrls.push(url);
      return jsonResponse(batches[Math.min(call++, batches.length - 1)]);
    });
    const inv = new Invoker({ baseUrl: "https://h/api/v1" });
    for await (const _ev of inv.streamTask("t-1", { pollIntervalMs: 1 })) {
      // drain
    }
    expect(seenUrls[0]).toContain("after_seq=0");
    expect(seenUrls[1]).toContain("after_seq=5");
  });

  it("surfaces mid-stream errors after events were yielded", async () => {
    let call = 0;
    mockFetch(async () => {
      call += 1;
      if (call === 1) {
        return jsonResponse({ items: [{ seq: 1, type: "started" }], done: false });
      }
      return jsonResponse({ error: "server_error", message: "kaboom" }, 500);
    });
    const inv = new Invoker({ baseUrl: "https://h/api/v1" });
    const types: string[] = [];
    let caught: unknown = null;
    try {
      for await (const ev of inv.streamTask("t-1", { pollIntervalMs: 1 })) {
        types.push(ev.type);
      }
    } catch (e) {
      caught = e;
    }
    expect(types).toEqual(["started"]);
    expect(caught).toBeInstanceOf(InvokerError);
    expect((caught as InvokerError).status).toBe(500);
  });

  it("sleeps between polls with the default cadence when overridden", async () => {
    const batches = [
      { items: [{ seq: 1, type: "started" }], done: false },
      { items: [{ seq: 2, type: "completed" }], done: true },
    ];
    let call = 0;
    mockFetch(async () => jsonResponse(batches[Math.min(call++, batches.length - 1)]));
    const inv = new Invoker({ baseUrl: "https://h/api/v1" });
    const started = Date.now();
    for await (const _ev of inv.streamTask("t-1", { pollIntervalMs: 25 })) {
      // drain
    }
    expect(Date.now() - started).toBeGreaterThanOrEqual(20);
  });
});

describe("InvokerEventSource", () => {
  function eventBatches() {
    return [
      { items: [{ seq: 1, type: "started" }], done: false },
      { items: [{ seq: 2, type: "completed" }], done: true },
    ];
  }

  it("emits events and done for a terminal event", async () => {
    const batches = eventBatches();
    let call = 0;
    mockFetch(async () => jsonResponse(batches[Math.min(call++, batches.length - 1)]));
    const inv = createInvoker({ baseUrl: "https://h/api/v1" });
    const src = new InvokerEventSource(inv, "t-1");

    const events: string[] = [];
    const doneEvents: string[] = [];
    src.on("event", (ev) => events.push(ev.type));
    src.on("done", (ev) => doneEvents.push(ev.type));

    await src.run();

    expect(events).toEqual(["started", "completed"]);
    expect(doneEvents).toEqual(["completed"]);
  });

  it("stops iterating once cancelled", async () => {
    const batches = eventBatches();
    let call = 0;
    mockFetch(async () => jsonResponse(batches[Math.min(call++, batches.length - 1)]));
    const inv = createInvoker({ baseUrl: "https://h/api/v1" });
    const src = new InvokerEventSource(inv, "t-1");

    const events: string[] = [];
    let doneEmitted = false;
    src.on("event", (ev) => {
      events.push(ev.type);
      if (ev.type === "started") {
        src.cancel();
      }
    });
    src.on("done", () => {
      doneEmitted = true;
    });

    await src.run();

    expect(events).toEqual(["started"]);
    expect(doneEmitted).toBe(false);
  });

  it("emits error when the stream fails", async () => {
    mockFetch(async () => jsonResponse({ error: "not_found", message: "nope" }, 404));
    const inv = createInvoker({ baseUrl: "https://h/api/v1" });
    const src = new InvokerEventSource(inv, "t-404");

    let caught: unknown = null;
    src.on("error", (err) => {
      caught = err;
    });
    await src.run();

    expect(caught).toBeInstanceOf(InvokerError);
    expect((caught as InvokerError).status).toBe(404);
  });
});
