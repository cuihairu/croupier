/**
 * L3 Invoker — independent caller-side module for the JS/TS SDK.
 *
 * Per the SDK feature matrix (sdks/SDK_FEATURE_MATRIX.md §四), an Invoker is
 * an INDEPENDENT capability: it must not share the Provider Client's config
 * entry, and it calls the Server over HTTP rather than a Provider session.
 *
 * Minimal canonical API:
 *   - Invoker(config)
 *   - invoke(functionId, payload, options)        -> payload | error
 *   - startTask(functionId, payload, options)     -> taskId
 *   - streamTask(taskId)                          -> AsyncIterable<TaskEvent>
 *   - cancelTask(taskId)                          -> void
 *
 * Naming is canonical (*Task*); no Job aliases.
 */

import { EventEmitter } from "events";

/** Configuration for the L3 Invoker. Independent from the Provider Client. */
export interface InvokerConfig {
  /** Server API URL, Server root URL, or host:port; normalized to /api/v1. */
  baseUrl: string;
  /** JWT bearer token (Authorization: Bearer <token>). */
  token?: string;
  /** Game scope, sent as X-Game-ID on scoped requests. */
  gameId?: string;
  /** Environment scope, sent as X-Env on scoped requests. */
  env?: string;
  /** Request timeout in milliseconds (default 30000). */
  timeout?: number;
  /** Extra fetch headers applied to every request. */
  headers?: Record<string, string>;
}

/** Options for a single invocation. */
export interface InvokeTaskOptions {
  idempotencyKey?: string;
  timeout?: number;
  headers?: Record<string, string>;
}

/** A task lifecycle event streamed from the server. */
export interface TaskEvent {
  seq: number;
  type: string; // started | progress | log | completed | failed | cancelled | cancel_requested
  progress?: number;
  message?: string;
  payload?: unknown;
  createdAt?: string;
}

/** Server-persisted task state returned by GET /tasks/:id. */
export interface TaskStatus {
  id: string;
  functionId?: string;
  status?: string;
  progress?: number;
  message?: string;
  result?: unknown;
  error?: string;
  gameId?: string;
  env?: string;
  agentId?: string;
  actor?: string;
  traceId?: string;
  startedAt?: string;
  finishedAt?: string;
  createdAt?: string;
  updatedAt?: string;
}

/** Result of a synchronous invoke. */
export interface InvokeResult {
  payload: unknown;
}

/** Error thrown when the server returns a non-2xx response. */
export class InvokerError extends Error {
  status: number;
  code?: string;
  details?: unknown;
  constructor(message: string, status: number, code?: string, details?: unknown) {
    super(message);
    this.name = "InvokerError";
    this.status = status;
    this.code = code;
    this.details = details;
  }
}

const DEFAULT_TIMEOUT = 30_000;

function buildHeaders(
  config: InvokerConfig,
  options?: InvokeTaskOptions,
  extra?: Record<string, string>,
): Record<string, string> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(config.headers || {}),
    ...(options?.headers || {}),
    ...(extra || {}),
  };
  if (config.token) headers.Authorization = `Bearer ${config.token}`;
  if (config.gameId) headers["X-Game-ID"] = config.gameId;
  if (config.env) headers["X-Env"] = config.env;
  return headers;
}

async function parseError(res: Response): Promise<InvokerError> {
  let body: any = null;
  try {
    body = await res.json();
  } catch {
    try {
      body = { message: await res.text() };
    } catch {
      /* ignore */
    }
  }
  const message = body?.message || res.statusText || "request failed";
  return new InvokerError(message, res.status, body?.error, body?.details);
}

function withTimeout(ms: number): { signal: AbortSignal; cancel: () => void } {
  const ctrl = new AbortController();
  const timer = setTimeout(() => ctrl.abort(), ms);
  return { signal: ctrl.signal, cancel: () => clearTimeout(timer) };
}

/**
 * Invoker is the L3 caller-side SDK. It talks to the Server REST API over
 * HTTP and is fully independent from the Provider Client.
 */
export class Invoker {
  private readonly config: InvokerConfig;

  constructor(config: InvokerConfig) {
    if (!config || !config.baseUrl) {
      throw new Error("Invoker requires a baseUrl");
    }
    const rawBaseUrl = config.baseUrl.trim();
    if (/^tcp:\/\//i.test(rawBaseUrl)) {
      throw new Error("Invoker baseUrl must be an HTTP(S) Server address");
    }
    const withScheme = rawBaseUrl.includes("://") ? rawBaseUrl : `http://${rawBaseUrl}`;
    const parsed = new URL(withScheme);
    if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
      throw new Error("Invoker baseUrl must be an HTTP(S) Server address");
    }
    const path = parsed.pathname.replace(/\/+$/, "");
    parsed.pathname = path.endsWith("/api/v1") ? path : `${path || ""}/api/v1`;
    parsed.search = "";
    parsed.hash = "";
    this.config = {
      timeout: DEFAULT_TIMEOUT,
      ...config,
      baseUrl: parsed.toString().replace(/\/$/, ""),
    };
  }

  /**
   * Synchronously invoke a function and return its payload.
   * POST /functions/:id/invoke
   */
  async invoke(
    functionId: string,
    payload?: unknown,
    options?: InvokeTaskOptions,
  ): Promise<InvokeResult> {
    if (!functionId) throw new Error("invoke requires a functionId");
    const timeout = options?.timeout ?? this.config.timeout ?? DEFAULT_TIMEOUT;
    const { signal, cancel } = withTimeout(timeout);
    try {
      const res = await fetch(`${this.config.baseUrl}/functions/${encodeURIComponent(functionId)}/invoke`, {
        method: "POST",
        headers: buildHeaders(this.config, options, options?.idempotencyKey ? { "Idempotency-Key": options.idempotencyKey } : undefined),
        body: JSON.stringify({ params: payload ?? {} }),
        signal,
      });
      if (!res.ok) throw await parseError(res);
      const data: unknown = await res.json();
      if (!data || typeof data !== "object" || !("result" in data)) {
        throw new InvokerError("server did not return a result", 502, "no_result", data);
      }
      return { payload: (data as { result: unknown }).result };
    } finally {
      cancel();
    }
  }

  /**
   * Start an asynchronous task and return its task id.
   * POST /tasks
   */
  async startTask(
    functionId: string,
    payload?: unknown,
    options?: InvokeTaskOptions,
  ): Promise<string> {
    if (!functionId) throw new Error("startTask requires a functionId");
    const timeout = options?.timeout ?? this.config.timeout ?? DEFAULT_TIMEOUT;
    const { signal, cancel } = withTimeout(timeout);
    try {
      const body: Record<string, unknown> = {
        functionId,
        params: payload ?? {},
      };
      const res = await fetch(`${this.config.baseUrl}/tasks`, {
        method: "POST",
        headers: buildHeaders(this.config, options, options?.idempotencyKey ? { "Idempotency-Key": options.idempotencyKey } : undefined),
        body: JSON.stringify(body),
        signal,
      });
      if (!res.ok) throw await parseError(res);
      const data: any = await res.json();
      const taskId = data?.taskId;
      if (!taskId) throw new InvokerError("server did not return a taskId", 502, "no_task_id", data);
      return taskId as string;
    } finally {
      cancel();
    }
  }

  /** Get the current Server-persisted task state. GET /tasks/:id */
  async getTaskStatus(taskId: string, options?: InvokeTaskOptions): Promise<TaskStatus> {
    if (!taskId) throw new Error("getTaskStatus requires a taskId");
    const timeout = options?.timeout ?? this.config.timeout ?? DEFAULT_TIMEOUT;
    const { signal, cancel } = withTimeout(timeout);
    try {
      const res = await fetch(`${this.config.baseUrl}/tasks/${encodeURIComponent(taskId)}`, {
        method: "GET",
        headers: buildHeaders(this.config, options),
        signal,
      });
      if (!res.ok) throw await parseError(res);
      const data: unknown = await res.json();
      if (!data || typeof data !== "object") {
        throw new InvokerError("server task status response must be an object", 502, "invalid_task_status", data);
      }
      const status = data as Omit<TaskStatus, "id"> & { id?: unknown };
      return { ...status, id: typeof status.id === "string" && status.id ? status.id : taskId };
    } finally {
      cancel();
    }
  }

  /**
   * Stream task lifecycle events as an async iterable. Polls /tasks/:id/events
   * with an increasing afterSeq cursor until the task reaches a terminal state.
   *
   * pollIntervalMs overrides the default 500ms poll cadence (useful in tests).
   */
  async *streamTask(taskId: string, options?: InvokeTaskOptions & { pollIntervalMs?: number }): AsyncIterable<TaskEvent> {
    if (!taskId) throw new Error("streamTask requires a taskId");
    const timeout = this.config.timeout ?? DEFAULT_TIMEOUT;
    let afterSeq = 0;
    const terminal = new Set(["completed", "failed", "cancelled", "timed_out"]);
    // eslint-disable-next-line no-constant-condition
    while (true) {
      const url = new URL(`${this.config.baseUrl}/tasks/${encodeURIComponent(taskId)}/events`);
      url.searchParams.set("after_seq", String(afterSeq));
      const { signal, cancel } = withTimeout(timeout);
      let res: Response;
      try {
        res = await fetch(url.toString(), {
          method: "GET",
          headers: buildHeaders(this.config, options),
          signal,
        });
      } finally {
        cancel();
      }
      if (!res.ok) throw await parseError(res);
      const data: any = await res.json();
      const items: TaskEvent[] = data?.items ?? [];
      let done = Boolean(data?.done);
      for (const ev of items) {
        if (ev.seq > afterSeq) afterSeq = ev.seq;
        yield ev;
        if (terminal.has(ev.type)) done = true;
      }
      if (done) return;
      // Poll interval: 500ms default, unless caller overrides via pollIntervalMs.
      const interval = options?.pollIntervalMs ?? 500;
      if (interval > 0) await new Promise((r) => setTimeout(r, interval));
    }
  }

  /**
   * Cancel a running task.
   * POST /tasks/:id/cancel
   */
  async cancelTask(taskId: string, options?: InvokeTaskOptions): Promise<void> {
    if (!taskId) throw new Error("cancelTask requires a taskId");
    const timeout = options?.timeout ?? this.config.timeout ?? DEFAULT_TIMEOUT;
    const { signal, cancel } = withTimeout(timeout);
    try {
      const res = await fetch(`${this.config.baseUrl}/tasks/${encodeURIComponent(taskId)}/cancel`, {
        method: "POST",
        headers: buildHeaders(this.config, options),
        body: "{}",
        signal,
      });
      if (!res.ok) throw await parseError(res);
    } finally {
      cancel();
    }
  }
}

/** Convenience constructor. */
export function createInvoker(config: InvokerConfig): Invoker {
  return new Invoker(config);
}

// EventEmitter kept for backward compatibility with callers that subscribe
// instead of using async iteration. Prefer `for await (const ev of streamTask())`.
export class InvokerEventSource extends EventEmitter {
  private cancelled = false;
  constructor(private invoker: Invoker, private taskId: string) {
    super();
  }
  async run(): Promise<void> {
    try {
      for await (const ev of this.invoker.streamTask(this.taskId)) {
        if (this.cancelled) break;
        this.emit("event", ev);
        if (["completed", "failed", "cancelled", "timed_out"].includes(ev.type)) {
          this.emit("done", ev);
          return;
        }
      }
    } catch (err) {
      this.emit("error", err);
    }
  }
  cancel(): void {
    this.cancelled = true;
  }
}
