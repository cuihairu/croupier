/**
 * AUD-080 — task event polling: the caller's timeout budget covers the
 * response body read/parse, and the polling-interval sleep honors the
 * caller's cancellation signal (no further request after a cancel).
 */

import { Invoker, InvokerEventSource, TaskEvent } from "./invoker";

type FetchImpl = typeof fetch;
let originalFetch: FetchImpl | undefined;

function mockFetch(
  responder: (url: string, init?: RequestInit) => Promise<Response>,
): void {
  originalFetch = globalThis.fetch;
  globalThis.fetch = ((url: string | URL | Request, init?: RequestInit) => {
    const u = typeof url === "string" ? url : url.toString();
    return responder(u, init);
  }) as unknown as FetchImpl;
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

/**
 * A response whose headers arrive immediately but whose body never settles
 * unless the fetch signal aborts — the undici behavior under a stalled body.
 */
function stalledBodyResponse(
  signal: AbortSignal | null | undefined,
): Response {
  const abortError = () =>
    new DOMException("This operation was aborted", "AbortError");
  return {
    ok: true,
    status: 200,
    statusText: "OK",
    json: () =>
      new Promise<never>((_resolve, reject) => {
        if (signal?.aborted) {
          reject(abortError());
          return;
        }
        signal?.addEventListener("abort", () => reject(abortError()));
      }),
  } as unknown as Response;
}

afterEach(() => restoreFetch());

describe("streamTask polling budget and cancellation (AUD-080)", () => {
  it("reads batches sequentially until done across poll intervals", async () => {
    const batches = [
      { items: [{ seq: 1, type: "started" }], done: false },
      { items: [{ seq: 2, type: "progress", progress: 40 }], done: false },
      { items: [{ seq: 3, type: "progress", progress: 80 }], done: false },
      { items: [{ seq: 4, type: "completed" }], done: true },
    ];
    const seenUrls: string[] = [];
    let call = 0;
    mockFetch(async (url) => {
      seenUrls.push(url);
      const batch = batches[call];
      call += 1;
      return jsonResponse(batch);
    });
    const inv = new Invoker({ baseUrl: "https://h/api/v1" });
    const types: string[] = [];
    for await (const ev of inv.streamTask("t-a", { pollIntervalMs: 5 })) {
      types.push(ev.type);
    }
    expect(types).toEqual(["started", "progress", "progress", "completed"]);
    expect(call).toBe(4);
    expect(seenUrls.map((u) => /after_seq=(\d+)/.exec(u)?.[1])).toEqual([
      "0",
      "1",
      "2",
      "3",
    ]);
  });

  it("times out when response headers arrive but the body stalls", async () => {
    let calls = 0;
    mockFetch(async (_url, init) => {
      calls += 1;
      return stalledBodyResponse(init?.signal);
    });
    const inv = new Invoker({ baseUrl: "https://h/api/v1" });
    let caught: unknown = null;
    const started = Date.now();
    try {
      for await (const _ev of inv.streamTask("t-b", {
        pollIntervalMs: 1,
        timeout: 80,
      })) {
        // unreachable: the body never settles
      }
    } catch (err) {
      caught = err;
    }
    expect(caught).not.toBeNull();
    expect((caught as Error).name).toBe("AbortError");
    // The caller timeout (80ms) fired during the body read — well before the
    // jest watchdog — and no second poll was issued after the abort.
    expect(Date.now() - started).toBeLessThan(5000);
    expect(calls).toBe(1);
  });

  it("cancellation during the polling interval stops the sleep and sends no further request", async () => {
    let calls = 0;
    mockFetch(async () => {
      calls += 1;
      if (calls === 1) {
        return jsonResponse({
          items: [{ seq: 1, type: "progress", progress: 10 }],
          done: false,
        });
      }
      return jsonResponse({ items: [], done: true });
    });
    const inv = new Invoker({ baseUrl: "https://h/api/v1" });
    const ctrl = new AbortController();
    const it = inv
      .streamTask("t-c", { pollIntervalMs: 1500, signal: ctrl.signal })
      [Symbol.asyncIterator]();
    const first = await it.next();
    expect(first.done).toBe(false);
    expect((first.value as TaskEvent).type).toBe("progress");
    // The generator is now sleeping in the poll interval (1500ms); cancel
    // well before that interval elapses.
    setTimeout(() => ctrl.abort(new Error("consumer stopped")), 50);
    const started = Date.now();
    await expect(it.next()).rejects.toThrow("consumer stopped");
    expect(Date.now() - started).toBeLessThan(1400); // sleep did not run out
    expect(calls).toBe(1); // no request was sent after the cancel
    await it.return?.();
  });

  it("cancellation before the first poll sends no request at all", async () => {
    let calls = 0;
    mockFetch(async () => {
      calls += 1;
      return jsonResponse({ items: [], done: true });
    });
    const inv = new Invoker({ baseUrl: "https://h/api/v1" });
    const ctrl = new AbortController();
    ctrl.abort();
    await expect(async () => {
      for await (const _ev of inv.streamTask("t-d", { signal: ctrl.signal })) {
        // unreachable
      }
    }).rejects.toThrow(/abort/i);
    expect(calls).toBe(0);
  });

  it("InvokerEventSource.cancel wakes the polling sleep and stops polling", async () => {
    let calls = 0;
    mockFetch(async () => {
      calls += 1;
      if (calls === 1) {
        return jsonResponse({
          items: [{ seq: 1, type: "started" }],
          done: false,
        });
      }
      return jsonResponse({ items: [], done: true });
    });
    const inv = new Invoker({ baseUrl: "https://h/api/v1" });
    const src = new InvokerEventSource(inv, "t-e");
    const errors: unknown[] = [];
    src.on("error", (err) => errors.push(err));
    const events: string[] = [];
    src.on("event", (ev: TaskEvent) => {
      events.push(ev.type);
      // cancel while the generator is sleeping in the poll interval
      setTimeout(() => src.cancel(), 10);
    });
    const started = Date.now();
    await src.run();
    expect(events).toEqual(["started"]);
    expect(errors).toEqual([]); // cancel() is not a stream failure
    expect(calls).toBe(1); // the cancelled poll sent no further request
    expect(Date.now() - started).toBeLessThan(450); // default 500ms sleep cut short
  });
});
