/**
 * Reference TCP probe plugin for non-HTTP destinations (todo.md:40).
 *
 * Host payload: {"host":"127.0.0.1","port":5432}. A successful TCP connect
 * proves network reachability only; game-protocol plugins own any stronger
 * login/business semantic.
 */

import * as net from "node:net";
import { performance } from "node:perf_hooks";
import {
  failResult,
  passResult,
  serve,
  type CheckOutcome,
  type ProbeCall,
  type ProbePlugin,
} from "../src";

const PLUGIN_NAME = "probe-tcp";
const PLUGIN_VERSION = "1.0.0";

interface TCPPayload {
  host?: unknown;
  port?: unknown;
  sleep_ms?: unknown;
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, Math.max(0, ms)));
}

async function waitForDeadline(call: ProbeCall, ms: number): Promise<boolean> {
  if (!Number.isInteger(ms) || ms <= 0) return false;
  const deadline = performance.now() + ms;
  while (performance.now() < deadline) {
    if (call.isCancelled()) return true;
    await sleep(Math.min(20, deadline - performance.now()));
  }
  return call.isCancelled();
}

function connect(host: string, port: number, timeoutMs: number): Promise<void> {
  return new Promise((resolve, reject) => {
    const socket = net.connect({ host, port });
    const done = (error?: Error) => {
      socket.removeAllListeners("connect");
      socket.removeAllListeners("error");
      socket.removeAllListeners("timeout");
      socket.destroy();
      if (error) reject(error);
      else resolve();
    };
    socket.setTimeout(timeoutMs, () => done(new Error("connect timeout")));
    socket.once("connect", () => done());
    socket.once("error", (error: Error) => done(error));
  });
}

const probeTCP: ProbePlugin = {
  describe: () => ({ name: PLUGIN_NAME, version: PLUGIN_VERSION }),

  check: async (call: ProbeCall): Promise<CheckOutcome> => {
    const started = performance.now();
    let payload: TCPPayload;
    try {
      payload = JSON.parse(call.payloadJson) as TCPPayload;
    } catch {
      payload = {};
    }
    if (await waitForDeadline(call, Number(payload.sleep_ms))) {
      return failResult("check cancelled", { latencyMs: elapsed(started) });
    }

    const { host, port } = payload;
    if (
      typeof host !== "string" ||
      !host ||
      !Number.isInteger(port) ||
      (port as number) < 1 ||
      (port as number) > 65535
    ) {
      return failResult("configuration: payload must contain host and port", {
        latencyMs: elapsed(started),
      });
    }

    try {
      await connect(host, port as number, Math.max(1, call.timeoutMs));
    } catch (error) {
      const reason = error instanceof Error ? error.message : String(error);
      return failResult(`tcp: ${reason}`, {
        latencyMs: elapsed(started),
        detail: `probe-tcp checked ${host}:${port}`,
      });
    }
    return passResult({
      status: 200,
      latencyMs: elapsed(started),
      detail: `probe-tcp connected to ${host}:${port}`,
    });
  },
};

function elapsed(started: number): number {
  return Math.floor(performance.now() - started);
}

void serve(probeTCP).then(
  () => undefined,
  (err: unknown) => {
    process.stderr.write(
      `${err instanceof Error ? err.message : String(err)}\n`,
    );
    process.exit(1);
  },
);
