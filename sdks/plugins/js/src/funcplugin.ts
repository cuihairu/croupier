/**
 * Sidecar function adapter plugin contract for Node.js (todo.md:38/40).
 *
 * Mirrors sdks/plugins/python/croupier_plugin/funcplugin.py: the same
 * magic cookie gate, the same croupier.funcplugin.v1.FunctionPlugin
 * service and the same closed-set error envelope, so an adapter authored
 * in either SDK is interchangeable behind the sidecar host config.
 *
 * A function plugin is a resident subprocess the sidecar launches and
 * maps function calls onto the game's existing HTTP/TCP/admin interface.
 * The host keeps only the call; the plugin holds no Croupier credentials
 * and never learns the server address.
 *
 * Business failures ride the closed-set InvokeResponse envelope and must
 * not be raised — a raised exception means the plugin process itself is
 * broken, which the host classifies as a plugin_* degradation cause.
 */

import * as grpc from "@grpc/grpc-js";
import { loadProtos } from "./proto";
import type {
  FuncDescriptorShape,
  InvokeRequestShape,
  InvokeResponseShape,
} from "./proto";
import { servePlugin } from "./serve";
import { normalizeDeadline } from "./probeplugin";
import type { CallSurface } from "./probeplugin";

/** Magic cookie gate: set by the sidecar function plugin host only. */
export const COOKIE_KEY = "CROUPIER_SIDECAR_FUNC_PLUGIN";
/** Cookie value identifying a sidecar function plugin binary. */
export const COOKIE_VALUE = "croupier-sidecar-function-plugin";
/** Protocol version this contract requests. */
export const PROTOCOL_VERSION = 1;

// Closed set for InvokeResult.errorCode: the host refuses anything else
// and classifies the call as a plugin contract violation instead of
// letting an unknown code leak into the caller's error taxonomy.
export const TARGET_UNAVAILABLE = "target_unavailable";
export const TARGET_REJECTED = "target_rejected";
export const BAD_RESPONSE = "bad_response";
export const ERROR_CODES: readonly string[] = [
  TARGET_UNAVAILABLE,
  TARGET_REJECTED,
  BAD_RESPONSE,
];

// Closed set for FunctionDescriptor.riskLevel; empty means low.
export const RISK_LOW = "low";
export const RISK_MEDIUM = "medium";
export const RISK_HIGH = "high";
export const RISK_DANGER = "danger";
export const RISK_LEVELS: readonly string[] = [
  RISK_LOW,
  RISK_MEDIUM,
  RISK_HIGH,
  RISK_DANGER,
];

/** One declared function (the author-facing form; descriptorToProto maps it). */
export interface FunctionDescriptor {
  /** id must live under "plugin.<plugin-name>.<function>" (host-checked). */
  id: string;
  /** Short technical summary (not a UI label). */
  summary: string;
  /** JSON Schema (Draft 2020-12) for the request object; top-level object. */
  paramSchema: string;
  /** JSON Schema for the success result object; top-level object. */
  resultSchema: string;
  /** Optional JSON Schema describing the error envelope shape. */
  errorSchema?: string;
  /** Closed set: low|medium|high|danger (empty = low). No plugin bypass. */
  riskLevel?: string;
}

export function descriptorToProto(d: FunctionDescriptor): FuncDescriptorShape {
  return {
    id: d.id,
    summary: d.summary,
    paramSchema: d.paramSchema,
    resultSchema: d.resultSchema,
    errorSchema: d.errorSchema ?? "",
    riskLevel: d.riskLevel ?? RISK_LOW,
  };
}

/** One host->plugin call. */
export interface InvokeCall {
  /** The registered function id (namespace-checked by the host). */
  functionId: string;
  /** JSON-encoded params object conforming to the declared paramSchema. */
  paramsJson: string;
  /** Remaining host-side deadline in milliseconds. */
  timeoutMs: number;
  /** True once the host deadline fires or the caller cancels. */
  isCancelled: () => boolean;
  /** Remaining ms until the RPC deadline, Infinity when unset. */
  remainingMs: () => number;
}

/** One call outcome: ok carries resultJson, !ok carries the closed set. */
export interface InvokeOutcome {
  ok: boolean;
  resultJson?: string;
  errorCode?: string;
  errorMessage?: string;
  atUnixMs?: number;
}

/** A function adapter plugin definition. */
export interface FunctionPlugin {
  /** (plugin name, version); the name must match the sidecar's config. */
  describe: () => { name: string; version: string };
  /** Functions this plugin offers right now (re-read per reconcile). */
  listFunctions: () => FunctionDescriptor[];
  /** Map one call onto the game interface. */
  invoke: (call: InvokeCall) => InvokeOutcome | Promise<InvokeOutcome>;
}

export function nowMs(): number {
  return Math.floor(Date.now());
}

/** Build a success envelope. */
export function okResult(
  resultJson: string,
  atUnixMs?: number,
): InvokeOutcome {
  return { ok: true, resultJson, atUnixMs };
}

/** Build a business failure envelope; code must be in the closed set. */
export function errorResult(
  code: string,
  message: string,
  atUnixMs?: number,
): InvokeOutcome {
  if (!ERROR_CODES.includes(code)) {
    throw new Error(
      `error_code ${JSON.stringify(code)} is outside the closed set ` +
        `${ERROR_CODES.join("|")}`,
    );
  }
  return { ok: false, errorCode: code, errorMessage: message, atUnixMs };
}

/** Map an InvokeOutcome onto the wire response (atUnixMs defaults to now). */
export function outcomeToProto(outcome: InvokeOutcome): InvokeResponseShape {
  return {
    ok: outcome.ok,
    resultJson: outcome.resultJson ?? "",
    errorCode: outcome.errorCode ?? "",
    errorMessage: outcome.errorMessage ?? "",
    atUnixMs: outcome.atUnixMs ?? nowMs(),
  };
}

/** Wrap the gRPC call into the plugin-facing InvokeCall view. */
export function makeInvokeCall(
  request: InvokeRequestShape,
  call: CallSurface,
): InvokeCall {
  const deadlineMs = normalizeDeadline(
    typeof call.getDeadline === "function" ? call.getDeadline() : undefined,
  );
  const reached = (): boolean => Date.now() >= deadlineMs;
  return {
    functionId: request.functionId ?? "",
    paramsJson: request.paramsJson ?? "",
    timeoutMs: Number(request.timeoutMs) || 0,
    isCancelled: () => call.cancelled === true || reached(),
    remainingMs: () => Math.max(0, deadlineMs - Date.now()),
  };
}

/** Handler map implementing croupier.funcplugin.v1.FunctionPlugin. */
export function funcHandlers(plugin: FunctionPlugin): grpc.UntypedServiceImplementation {
  return {
    Describe: (
      _call: grpc.ServerUnaryCall<unknown, unknown>,
      callback: grpc.sendUnaryData<{ name: string; version: string }>,
    ) => {
      const { name, version } = plugin.describe();
      callback(null, { name, version });
    },
    ListFunctions: (
      _call: grpc.ServerUnaryCall<unknown, unknown>,
      callback: grpc.sendUnaryData<{ functions: FuncDescriptorShape[] }>,
    ) => {
      callback(null, {
        functions: plugin.listFunctions().map(descriptorToProto),
      });
    },
    Invoke: (
      call: grpc.ServerUnaryCall<InvokeRequestShape, unknown>,
      callback: grpc.sendUnaryData<InvokeResponseShape>,
    ) => {
      const request = (call.request ?? {}) as InvokeRequestShape;
      Promise.resolve(plugin.invoke(makeInvokeCall(request, call))).then(
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

/** Options for serveFunctionPlugin. */
export interface ServeFuncOptions {
  /** Override the handshake protocol version request. */
  protocolVersion?: number;
  /** Injectable env for tests. */
  env?: Record<string, string | undefined>;
  /** Extra stop hooks. */
  onShutdown?: () => Promise<void> | void;
}

/** Run one function adapter plugin until the host shuts it down. */
export async function serveFunctionPlugin(
  plugin: FunctionPlugin,
  options: ServeFuncOptions = {},
): Promise<void> {
  await servePlugin({
    cookieKey: COOKIE_KEY,
    cookieValue: COOKIE_VALUE,
    protocolVersionsKey: "PLUGIN_PROTOCOL_VERSIONS",
    multiplexKey: "PLUGIN_MULTIPLEX_GRPC",
    bindEnv: "CROUPIER_PLUGIN_BIND",
    protocolVersion: options.protocolVersion ?? PROTOCOL_VERSION,
    register: (server) => server.addService(loadProtos().funcService, funcHandlers(plugin)),
    onShutdown: options.onShutdown,
    env: options.env,
  });
}
