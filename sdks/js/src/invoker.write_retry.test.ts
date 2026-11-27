/**
 * AUD-068: write requests must not be replayed automatically. A write may
 * have taken effect before the response was lost, so the default policy is
 * single-shot for non-GET requests and the final failure is marked as a
 * result-unknown outcome. GET requests keep the previous retry behavior.
 */
import { Invoker, InvokerError, isResultUnknown } from "./invoker";
import type { RetryConfig } from "./invoker";

// Minimal fetch mock (mirrors invoker.test.ts).
type FetchResponder = (
  url: string,
  init?: RequestInit,
) => Promise<Response>;

let originalFetch: typeof globalThis.fetch | undefined;
function mockFetch(responder: FetchResponder): void {
  originalFetch = globalThis.fetch;
  globalThis.fetch = (((
    url: string | URL | Request,
    init?: RequestInit,
  ) => {
    const u = typeof url === "string" ? url : url.toString();
    return responder(u, init);
  }) as unknown) as typeof globalThis.fetch;
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

function fastRetry(extra: Partial<RetryConfig> = {}): RetryConfig {
  return {
    enabled: true,
    maxAttempts: 3,
    initialDelayMs: 1,
    maxDelayMs: 2,
    jitterFactor: 0,
    ...extra,
  };
}

afterEach(() => restoreFetch());

// 首个 POST 已执行后网络层失败（fetch 抛 TypeError）：默认单发 + result-unknown。
test("write is not retried by default and failure is marked result-unknown", async () => {
  const invoker = new Invoker({
    baseUrl: "http://server.test",
    retry: fastRetry(),
  });
  let calls = 0;
  mockFetch(async () => {
    calls += 1;
    throw new TypeError("fetch failed: connection reset");
  });

  await expect(invoker.invoke("player.ban", {})).rejects.toThrow(
    /result unknown after write/,
  );
  expect(calls).toBe(1);
});

// 可重试 5xx 同样默认单发（门控在 isRetryableFailure 之前），失败带标记。
test("write with retryable 503 is single-shot by default", async () => {
  const invoker = new Invoker({
    baseUrl: "http://server.test",
    retry: fastRetry(),
  });
  let calls = 0;
  mockFetch(async () => {
    calls += 1;
    return jsonResponse({ error: "unavailable", message: "try later" }, 503);
  });

  const err = await invoker.invoke("player.ban", {}).catch((e: unknown) => e);
  expect(calls).toBe(1);
  expect(isResultUnknown(err)).toBe(true);
  expect((err as InvokerError).status).toBe(503);
});

// retryWrites=true 时写请求恢复自动重试；耗尽仍标 result-unknown。
test("retryWrites opts the write back into retries", async () => {
  const invoker = new Invoker({
    baseUrl: "http://server.test",
    retry: fastRetry({ retryWrites: true }),
  });
  let calls = 0;
  mockFetch(async () => {
    calls += 1;
    if (calls < 2) {
      return jsonResponse({ error: "unavailable" }, 503);
    }
    return jsonResponse({ result: { ok: true } });
  });

  await expect(invoker.invoke("player.ban", {})).resolves.toEqual({
    payload: { ok: true },
  });
  expect(calls).toBe(2);

  // startTask 同一策略（POST /tasks）。
  calls = 0;
  mockFetch(async () => {
    calls += 1;
    if (calls < 2) {
      return jsonResponse({ error: "unavailable" }, 503);
    }
    return jsonResponse({ taskId: "t1" });
  });
  await expect(invoker.startTask("player.ban", {})).resolves.toBe("t1");
  expect(calls).toBe(2);
});

// 明确拒绝（4xx）不重试也不标 result-unknown。
test("explicit 4xx rejection is neither retried nor marked unknown", async () => {
  const invoker = new Invoker({
    baseUrl: "http://server.test",
    retry: fastRetry({ retryWrites: true }),
  });
  let calls = 0;
  mockFetch(async () => {
    calls += 1;
    return jsonResponse({ error: "permission_denied", message: "no" }, 403);
  });

  const err = await invoker.invoke("player.ban", {}).catch((e: unknown) => e);
  expect(calls).toBe(1);
  expect(isResultUnknown(err)).toBe(false);
  expect((err as InvokerError).status).toBe(403);
});

// GET 仍按原行为退避重试；读失败不标 result-unknown。
test("GET keeps retrying and read failures are not marked unknown", async () => {
  const invoker = new Invoker({
    baseUrl: "http://server.test",
    retry: fastRetry(),
  });
  let calls = 0;
  mockFetch(async () => {
    calls += 1;
    if (calls < 3) {
      return jsonResponse({ error: "unavailable" }, 503);
    }
    return jsonResponse({ id: "t1", status: "running" });
  });

  const status = await invoker.getTaskStatus("t1");
  expect(calls).toBe(3);
  expect(status.status).toBe("running");

  // 持续 5xx 的读：重试耗尽，普通 InvokerError，无标记。
  calls = 0;
  mockFetch(async () => {
    calls += 1;
    return jsonResponse({ error: "unavailable" }, 503);
  });
  const err = await invoker.getTaskStatus("t1").catch((e: unknown) => e);
  expect(calls).toBe(3);
  expect(isResultUnknown(err)).toBe(false);
});

// startTask 写失败：默认单发 + 标记。
test("startTask write is single-shot by default and marked unknown", async () => {
  const invoker = new Invoker({
    baseUrl: "http://server.test",
    retry: fastRetry(),
  });
  let calls = 0;
  mockFetch(async () => {
    calls += 1;
    return jsonResponse({ error: "unavailable" }, 503);
  });

  await expect(invoker.startTask("player.ban", {})).rejects.toThrow(
    /result unknown after write/,
  );
  expect(calls).toBe(1);
});
