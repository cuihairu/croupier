/**
 * Synthetics probe plugin contract for Node.js.
 *
 * Mirrors sdks/plugins/python/croupier_plugin/probeplugin.py: the same magic
 * cookie gate, the same croupier.probeplugin.v1.ProbePlugin service, and the
 * same CheckInput / CheckResult mapping so a probe authored in either SDK is
 * interchangeable in a synconfig plugin list.
 *
 * The plugin sees a ProbeCall (target, timeoutMs, payloadJson, cancellation
 * view) and returns a CheckOutcome; the SDK maps it onto the wire result,
 * defaulting at_unix_ms to now (unix ms) like the Python SDK.
 */

import * as grpc from "@grpc/grpc-js";
import { loadProtos } from "./proto";
import type { CheckResultShape, CheckRequestShape } from "./proto";
import { servePlugin } from "./serve";

/** Magic cookie gate: set by the synthetics plugin host only. */
export const COOKIE_KEY = "CROUPIER_SYNTHETICS_PLUGIN";
/** Cookie value identifying a synthetics probe plugin binary. */
export const COOKIE_VALUE = "croupier-synthetics-probe-plugin";
/** Protocol version this contract requests. */
export const PROTOCOL_VERSION = 1;
/** Env var holding the host's supported protocol version list. */
export const PROTOCOL_VERSIONS_KEY = "PLUGIN_PROTOCOL_VERSIONS";
/** Env var enabling multiplex gRPC (handshake field 7). */
export const MULTIPLEX_KEY = "PLUGIN_MULTIPLEX_GRPC";
/** Env var overriding the TCP bind address. */
export const BIND_ENV = "CROUPIER_PLUGIN_BIND";

/** Cancellation + deadline surface of the host's RPC context. */
export interface ProbeCall {
  /** Target as configured in the synconfig probe definition. */
  target: string;
  /** Per-check timeout in ms (informational; the host also sets a deadline). */
  timeoutMs: number;
  /** Optional per-check payload, encoded as a JSON string ("" if none). */
  payloadJson: string;
  /** Target-scoped credentials the host resolved from its environment
   * (todo.md:36 credential boundary). Use them in-process; never echo
   * them into error/detail — the host scrubs known values, but a plugin
   * that echoes secrets leaks them into logs first. */
  credentials: Record<string, string>;
  /** True once the host cancels or the RPC deadline is reached. */
  isCancelled: () => boolean;
  /** Remaining ms until the RPC deadline, Infinity when the host set none. */
  remainingMs: () => number;
}

/** One probe outcome; every field except ok is optional (0 / "" defaults). */
export interface CheckOutcome {
  ok: boolean;
  status?: number;
  latencyMs?: number;
  error?: string;
  detail?: string;
  atUnixMs?: number;
}

/** Options for a passing outcome. */
export interface PassOptions {
  status?: number;
  latencyMs?: number;
  detail?: string;
  atUnixMs?: number;
}

/** Options for a failing outcome. */
export interface FailOptions {
  status?: number;
  latencyMs?: number;
  detail?: string;
  atUnixMs?: number;
}

/** A probe plugin definition. */
export interface ProbePlugin {
  /** Self-description reported to the host at registration. */
  describe: () => { name: string; version: string };
  /** Run one check against call.target. */
  check: (call: ProbeCall) => CheckOutcome | Promise<CheckOutcome>;
}

export function nowMs(): number {
  return Math.floor(Date.now());
}

/** Build a passing outcome (ok=true, error cleared). */
export function passResult(options: PassOptions = {}): CheckOutcome {
  return {
    ok: true,
    status: options.status ?? 0,
    latencyMs: options.latencyMs ?? 0,
    error: "",
    detail: options.detail ?? "",
    atUnixMs: options.atUnixMs,
  };
}

/** Build a failing outcome (ok=false, error required). */
export function failResult(
  error: string,
  options: FailOptions = {},
): CheckOutcome {
  return {
    ok: false,
    status: options.status ?? 0,
    latencyMs: options.latencyMs ?? 0,
    error,
    detail: options.detail ?? "",
    atUnixMs: options.atUnixMs,
  };
}

/** Map a CheckOutcome onto the wire CheckResult (at_unix_ms defaults to now). */
export function outcomeToProto(outcome: CheckOutcome): CheckResultShape {
  return {
    ok: outcome.ok,
    status: outcome.status ?? 0,
    latencyMs: outcome.latencyMs ?? 0,
    error: outcome.error ?? "",
    detail: outcome.detail ?? "",
    atUnixMs: outcome.atUnixMs ?? nowMs(),
  };
}

/** Minimal gRPC call surface makeProbeCall needs (easy to fake in tests). */
export interface CallSurface {
  getDeadline?: () => grpc.Deadline;
  cancelled?: boolean;
}

/** Normalize grpc-js's Deadline (Date | number, Infinity when unset) to ms. */
export function normalizeDeadline(raw: grpc.Deadline | undefined): number {
  if (raw === undefined) {
    return Number.POSITIVE_INFINITY;
  }
  if (raw instanceof Date) {
    return raw.getTime();
  }
  return raw;
}

/** Wrap the gRPC call into the plugin-facing ProbeCall view. */
export function makeProbeCall(
  request: CheckRequestShape,
  call: CallSurface,
): ProbeCall {
  const deadlineMs = normalizeDeadline(
    typeof call.getDeadline === "function" ? call.getDeadline() : undefined,
  );
  const reached = (): boolean => Date.now() >= deadlineMs;
  return {
    target: request.target ?? "",
    timeoutMs: Number(request.timeoutMs) || 0,
    payloadJson: request.payloadJson ?? "",
    credentials: { ...(request.credentials ?? {}) },
    isCancelled: () => call.cancelled === true || reached(),
    remainingMs: () => Math.max(0, deadlineMs - Date.now()),
  };
}

/** Handler map implementing croupier.probeplugin.v1.ProbePlugin. */
export function probeHandlers(plugin: ProbePlugin): grpc.UntypedServiceImplementation {
  return {
    Describe: (
      _call: grpc.ServerUnaryCall<unknown, unknown>,
      callback: grpc.sendUnaryData<{ name: string; version: string }>,
    ) => {
      const { name, version } = plugin.describe();
      callback(null, { name, version });
    },
    Check: (
      call: grpc.ServerUnaryCall<CheckRequestShape, unknown>,
      callback: grpc.sendUnaryData<CheckResultShape>,
    ) => {
      const request = (call.request ?? {}) as CheckRequestShape;
      Promise.resolve(plugin.check(makeProbeCall(request, call))).then(
        (outcome) => callback(null, outcomeToProto(outcome)),
        (err: unknown) =>
          callback(
            err instanceof Error
              ? (Object.assign(err, { code: grpc.status.UNKNOWN }) as grpc.ServerErrorResponse)
              : { code: grpc.status.UNKNOWN, message: String(err) },
          ),
      );
    },
  };
}

/** Options for serve. */
export interface ServeProbeOptions {
  /** Override the handshake protocol version request. */
  protocolVersion?: number;
  /** Injectable env for tests. */
  env?: Record<string, string | undefined>;
  /** Extra stop hooks. */
  onShutdown?: () => Promise<void> | void;
}

/** Serve one probe plugin until the host shuts it down or a signal arrives. */
export async function serve(
  plugin: ProbePlugin,
  options: ServeProbeOptions = {},
): Promise<void> {
  await servePlugin({
    cookieKey: COOKIE_KEY,
    cookieValue: COOKIE_VALUE,
    protocolVersionsKey: PROTOCOL_VERSIONS_KEY,
    multiplexKey: MULTIPLEX_KEY,
    bindEnv: BIND_ENV,
    protocolVersion: options.protocolVersion ?? PROTOCOL_VERSION,
    register: (server) => server.addService(loadProtos().probeService, probeHandlers(plugin)),
    onShutdown: options.onShutdown,
    env: options.env,
  });
}
