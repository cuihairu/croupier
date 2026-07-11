import {
  Invoker,
  createInvoker,
  InvokerError,
} from "./invoker";

// Minimal global fetch mock. Each test installs its own responder.
type FetchImpl = typeof fetch;
let originalFetch: FetchImpl | undefined;

function mockFetch(responder: (url: string, init?: RequestInit) => Promise<Response>): void {
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

describe("Invoker construction", () => {
  it("requires a baseUrl", () => {
    expect(() => new Invoker({} as any)).toThrow("baseUrl");
  });

  it("strips trailing slash from baseUrl", () => {
    const inv = new Invoker({ baseUrl: "https://h:18780/api/v1///" });
    expect((inv as any).config.baseUrl).toBe("https://h:18780/api/v1");
  });
});

describe("Invoker.invoke", () => {
  it("posts to /functions/:id/invoke and returns payload", async () => {
    let captured: { url: string; body: string; headers: Record<string, string> } | null = null;
    mockFetch(async (url, init) => {
      captured = {
        url,
        body: init?.body as string,
        headers: (init?.headers as Record<string, string>) || {},
      };
      return jsonResponse({ ok: true, value: 42 });
    });

    const inv = createInvoker({
      baseUrl: "https://h/api/v1",
      token: "tok",
      gameId: "demo",
      env: "prod",
    });
    const res = await inv.invoke("player.ban", { userId: 1 });
    expect(res.payload).toEqual({ ok: true, value: 42 });
    expect(captured!.url).toBe("https://h/api/v1/functions/player.ban/invoke");
    expect(captured!.headers.Authorization).toBe("Bearer tok");
    expect(captured!.headers["X-Game-ID"]).toBe("demo");
    expect(captured!.headers["X-Env"]).toBe("prod");
    expect(JSON.parse(captured!.body)).toEqual({ userId: 1 });
  });

  it("attaches Idempotency-Key when provided", async () => {
    let headers: Record<string, string> = {};
    mockFetch(async (_url, init) => {
      headers = (init?.headers as Record<string, string>) || {};
      return jsonResponse({});
    });
    const inv = new Invoker({ baseUrl: "https://h/api/v1" });
    await inv.invoke("f", {}, { idempotencyKey: "k-1" });
    expect(headers["Idempotency-Key"]).toBe("k-1");
  });

  it("throws InvokerError on non-2xx", async () => {
    mockFetch(async () => jsonResponse({ error: "forbidden", message: "no perm" }, 403));
    const inv = new Invoker({ baseUrl: "https://h/api/v1" });
    await expect(inv.invoke("f")).rejects.toMatchObject({
      name: "InvokerError",
      status: 403,
      code: "forbidden",
    });
    await expect(inv.invoke("f")).rejects.toBeInstanceOf(InvokerError);
  });
});

describe("Invoker.startTask", () => {
  it("posts to /tasks and returns taskId", async () => {
    let body: string | null = null;
    mockFetch(async (_url, init) => {
      body = init?.body as string;
      return jsonResponse({ taskId: "task-xyz", status: "dispatching" });
    });
    const inv = new Invoker({ baseUrl: "https://h/api/v1", gameId: "g", env: "e" });
    const id = await inv.startTask("player.kick", { reason: "afk" });
    expect(id).toBe("task-xyz");
    expect(JSON.parse(body!)).toEqual({
      functionId: "player.kick",
      params: { reason: "afk" },
      gameId: "g",
      env: "e",
    });
  });

  it("throws when server omits taskId", async () => {
    mockFetch(async () => jsonResponse({ status: "dispatching" }));
    const inv = new Invoker({ baseUrl: "https://h/api/v1" });
    await expect(inv.startTask("f")).rejects.toMatchObject({ status: 502 });
  });
});

describe("Invoker.streamTask", () => {
  it("yields events and stops at terminal state", async () => {
    const eventBatches = [
      { items: [{ seq: 1, type: "started" }], done: false },
      { items: [{ seq: 2, type: "progress", progress: 50 }], done: false },
      { items: [{ seq: 3, type: "completed", progress: 100 }], done: true },
    ];
    let call = 0;
    mockFetch(async (url) => {
      expect(url).toContain("/tasks/t-1/events");
      const batch = eventBatches[Math.min(call, eventBatches.length - 1)];
      call += 1;
      return jsonResponse(batch);
    });
    const inv = new Invoker({ baseUrl: "https://h/api/v1" });
    const collected: string[] = [];
    for await (const ev of inv.streamTask("t-1", { pollIntervalMs: 0 })) {
      collected.push(ev.type);
    }
    expect(collected).toEqual(["started", "progress", "completed"]);
  });

  it("surfaces server errors as InvokerError", async () => {
    mockFetch(async () => jsonResponse({ error: "not_found", message: "missing" }, 404));
    const inv = new Invoker({ baseUrl: "https://h/api/v1" });
    let caught: any = null;
    try {
      for await (const _ev of inv.streamTask("t-x")) {
        // should throw before yielding
      }
    } catch (e) {
      caught = e;
    }
    expect(caught).toBeInstanceOf(InvokerError);
    expect(caught.status).toBe(404);
    expect(caught.code).toBe("not_found");
  });
});

describe("Invoker.cancelTask", () => {
  it("posts to /tasks/:id/cancel", async () => {
    let url = "";
    mockFetch(async (u) => {
      url = u;
      return jsonResponse({});
    });
    const inv = new Invoker({ baseUrl: "https://h/api/v1" });
    await inv.cancelTask("t-9");
    expect(url).toBe("https://h/api/v1/tasks/t-9/cancel");
  });
});
