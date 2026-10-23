/**
 * Reference probe plugin: probe-echo (todo.md:40).
 *
 * Mirrors the Python example so the probe contract has a cross-language
 * proof: the real synthetics controller drives this process through the
 * real go-plugin handshake in the SDK test suite (hostdriver scenarios).
 *
 * Semantics: a target named "fail..." always reports ok=false with the
 * synthetic reason; anything else passes with status 200. An optional
 * {"sleep_ms": N} payload makes the check wait first (bounded by the RPC
 * deadline), which is what the host-side deadline scenario drives.
 *
 * Run (the host supplies the handshake env; direct execution is refused):
 *     node dist/examples/probe_echo.js
 */

import { performance } from "node:perf_hooks";
import {
  failResult,
  passResult,
  serve,
  type CheckOutcome,
  type ProbeCall,
  type ProbePlugin,
} from "../src";

const PLUGIN_NAME = "probe-echo";
const PLUGIN_VERSION = "1.0.0";

interface EchoPayload {
  sleep_ms?: unknown;
}

const probeEcho: ProbePlugin = {
  describe: () => ({ name: PLUGIN_NAME, version: PLUGIN_VERSION }),

  check: async (call: ProbeCall): Promise<CheckOutcome> => {
    const started = performance.now();

    let sleepMs = 0;
    if (call.payloadJson !== "") {
      try {
        const payload = JSON.parse(call.payloadJson) as EchoPayload;
        if (
          typeof payload === "object" &&
          payload !== null &&
          Number.isInteger(payload.sleep_ms)
        ) {
          sleepMs = payload.sleep_ms as number;
        }
      } catch {
        // payload is plugin-specific; unparseable means "no payload"
      }
    }
    const deadline = started + Math.max(0, sleepMs);
    while (performance.now() < deadline) {
      if (call.isCancelled()) {
        return failResult("check cancelled", { latencyMs: elapsed(started) });
      }
      // Timer-based wait: unlike the Python SDK there is no worker pool
      // here, so blocking the event loop would stall every concurrent RPC.
      await sleep(Math.min(20, deadline - performance.now()));
    }

    const latencyMs = elapsed(started);
    // Credential-boundary targets (todo.md:36), mirrored by the Go
    // echoprobe and the Python probe_echo so hostdriver can prove the
    // cross-language path: "cred..." reports how many credentials
    // arrived (never their values); "leak..." echoes a value back so
    // hosts can prove their redaction scrubs it.
    if (call.target.startsWith("cred")) {
      return passResult({
        status: 200,
        latencyMs,
        detail: `probe-echo received ${Object.keys(call.credentials).length} credential(s)`,
      });
    }
    if (call.target.startsWith("leak")) {
      const secret = Object.values(call.credentials)[0] ?? "";
      const msg = `auth failed token=${secret}`;
      return failResult(msg, {
        latencyMs,
        detail: `probe-echo ${msg}`,
      });
    }
    if (call.target.startsWith("fail")) {
      return failResult("synthetic probe failure", {
        status: 500,
        latencyMs,
        detail: `probe-echo checked ${call.target}`,
      });
    }
    return passResult({
      status: 200,
      latencyMs,
      detail: `probe-echo checked ${call.target}`,
    });
  },
};

function elapsed(started: number): number {
  return Math.floor(performance.now() - started);
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => {
    setTimeout(resolve, Math.max(0, Math.floor(ms)));
  });
}

void serve(probeEcho).then(
  () => undefined,
  (err: unknown) => {
    // Handshake refusals print the reason the process cannot serve.
    process.stderr.write(`${err instanceof Error ? err.message : String(err)}\n`);
    process.exit(1);
  },
);
