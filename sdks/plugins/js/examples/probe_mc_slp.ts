/**
 * Reference Minecraft Java Server List Ping (SLP) probe plugin.
 *
 * The first game-protocol reference probe (todo PROBE-001): SLP is the
 * public read-only status query (handshake state=1 -> status request ->
 * status response JSON) per wiki.vg, implemented with node:net and
 * standard buffers only. It never touches the login/encryption chain,
 * so it verifies any Java server with zero accounts — and a pass here is
 * server-status evidence only, never player-login evidence (binding
 * clause from mc-online-login-survey-2026-11-28).
 *
 * Host payload: {"host": "127.0.0.1", "port": 25565} — port optional,
 * defaulting to Minecraft Java's 25565.
 *
 * The internal read budget is 80% of the host deadline so a silent
 * non-SLP target surfaces as a plugin business failure before the
 * host-side degradation deadline races it.
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

const PLUGIN_NAME = "probe-mc-slp";
const PLUGIN_VERSION = "1.0.0";

const DEFAULT_PORT = 25565;
const MAX_STATUS_BYTES = 2 << 20;
// Handshake declares 1.8.9; servers do not validate the version on the
// status path (version negotiation happens in the login flow).
const DECLARED_PROTOCOL = 47;

interface SLPPayload {
  host?: unknown;
  port?: unknown;
}

interface SLPVersion {
  name?: unknown;
  protocol?: unknown;
}

interface SLPPlayers {
  online?: unknown;
  max?: unknown;
}

interface SLPStatus {
  version?: unknown;
  players?: unknown;
}

class SLPError extends Error {}

function varint(value: number): Buffer {
  const out: number[] = [];
  let v = value >>> 0;
  for (;;) {
    if ((v & ~0x7f) === 0) {
      out.push(v);
      return Buffer.from(out);
    }
    out.push((v & 0x7f) | 0x80);
    v >>>= 7;
  }
}

function frame(packetID: number, payload: Buffer): Buffer {
  const data = Buffer.concat([Buffer.from([packetID]), payload]);
  return Buffer.concat([varint(data.length), data]);
}

function handshake(host: string, port: number): Buffer {
  const hostBytes = Buffer.from(host, "utf8");
  const portBytes = Buffer.alloc(2);
  portBytes.writeUInt16BE(port, 0);
  const payload = Buffer.concat([
    varint(DECLARED_PROTOCOL),
    varint(hostBytes.length),
    hostBytes,
    portBytes,
    varint(1), // nextState: 1 = status
  ]);
  return frame(0x00, payload);
}

function statusRequest(): Buffer {
  return frame(0x00, Buffer.alloc(0));
}

function readVarIntAt(
  buf: Buffer,
  offset: number,
): { value: number; next: number } | null {
  let result = 0;
  for (let i = 0; i < 5; i++) {
    if (offset + i >= buf.length) return null;
    const byte = buf[offset + i];
    result |= (byte & 0x7f) << (7 * i);
    if ((byte & 0x80) === 0) return { value: result >>> 0, next: offset + i + 1 };
  }
  throw new SLPError("varint too long");
}

interface SLPAnswer {
  version: string;
  protocol: number;
  players: string | null;
}

/** One SLP round trip; resolves with the parsed status summary. */
function slpQuery(
  host: string,
  port: number,
  budgetMs: number,
  call: ProbeCall,
): Promise<SLPAnswer> {
  return new Promise((resolve, reject) => {
    const sock = net.connect({ host, port });
    let buf = Buffer.alloc(0);
    let frameLen = -1;
    let settled = false;

    const done = (error: Error, answer?: SLPAnswer) => {
      if (settled) return;
      settled = true;
      clearTimeout(timer);
      clearInterval(cancelPoll);
      sock.removeAllListeners();
      sock.destroy();
      if (answer) resolve(answer);
      else reject(error);
    };

    // 80% read budget: answer before the host deadline (see module doc).
    const timer = setTimeout(
      () => done(new SLPError("timed out waiting for status response")),
      Math.max(1, budgetMs),
    );
    const cancelPoll = setInterval(() => {
      if (call.isCancelled()) done(new SLPError("check cancelled"));
    }, 20);

    sock.on("error", (error: Error) => done(error));
    sock.on("connect", () => {
      sock.write(Buffer.concat([handshake(host, port), statusRequest()]));
    });
    sock.on("close", () => done(new SLPError("connection closed before status response completed")));
    sock.on("data", (chunk: Buffer) => {
      buf = Buffer.concat([buf, chunk]);
      try {
        if (frameLen < 0) {
          const head = readVarIntAt(buf, 0);
          if (head === null) return;
          if (head.value <= 0 || head.value > MAX_STATUS_BYTES) {
            done(new SLPError(`invalid frame length ${head.value}`));
            return;
          }
          frameLen = head.value;
          buf = buf.subarray(head.next);
        }
        if (buf.length < frameLen) return;

        const packetID = readVarIntAt(buf, 0);
        if (packetID === null) return;
        if (packetID.value !== 0) {
          done(new SLPError(`unexpected packet id 0x${packetID.value.toString(16)}`));
          return;
        }
        const strLen = readVarIntAt(buf, packetID.next);
        if (strLen === null) return;
        if (strLen.value > buf.length - strLen.next) {
          done(new SLPError(`string length ${strLen.value} exceeds payload`));
          return;
        }
        const status = JSON.parse(
          buf.subarray(strLen.next, strLen.next + strLen.value).toString("utf8"),
        ) as SLPStatus;
        if (typeof status.version !== "object" || status.version === null) {
          done(new SLPError("response missing version"));
          return;
        }
        const version = status.version as SLPVersion;
        const answer: SLPAnswer = {
          version: typeof version.name === "string" ? version.name : "",
          protocol: typeof version.protocol === "number" ? version.protocol : 0,
          players: null,
        };
        if (typeof status.players === "object" && status.players !== null) {
          const players = status.players as SLPPlayers;
          answer.players = `${typeof players.online === "number" ? players.online : 0}/${typeof players.max === "number" ? players.max : 0}`;
        }
        done(new Error("unreachable"), answer);
      } catch (error) {
        done(error instanceof Error ? error : new SLPError(String(error)));
      }
    });
  });
}

const probeMCSLP: ProbePlugin = {
  describe: () => ({ name: PLUGIN_NAME, version: PLUGIN_VERSION }),

  check: async (call: ProbeCall): Promise<CheckOutcome> => {
    const started = performance.now();
    let payload: SLPPayload;
    try {
      payload = JSON.parse(call.payloadJson) as SLPPayload;
    } catch {
      payload = {};
    }
    const { host, port } = payload;
    if (typeof host !== "string" || !host) {
      return failResult(
        "configuration: payload must contain host (port optional, default 25565)",
        { latencyMs: elapsed(started) },
      );
    }
    const resolvedPort =
      port === undefined || port === null ? DEFAULT_PORT : port;
    if (
      !Number.isInteger(resolvedPort) ||
      (resolvedPort as number) < 1 ||
      (resolvedPort as number) > 65535
    ) {
      return failResult("configuration: port must be an integer in [1, 65535]", {
        latencyMs: elapsed(started),
      });
    }

    try {
      const answer = await slpQuery(
        host,
        resolvedPort as number,
        Math.max(50, call.timeoutMs * 0.8),
        call,
      );
      let detail = `probe-mc-slp checked ${host}:${resolvedPort} version=${answer.version} protocol=${answer.protocol}`;
      if (answer.players !== null) detail += ` players=${answer.players}`;
      return passResult({ status: 200, latencyMs: elapsed(started), detail });
    } catch (error) {
      const reason = error instanceof Error ? error.message : String(error);
      return failResult(`minecraft slp: ${reason}`, {
        latencyMs: elapsed(started),
        detail: `probe-mc-slp checked ${host}:${resolvedPort}`,
      });
    }
  },
};

function elapsed(started: number): number {
  return Math.floor(performance.now() - started);
}

void serve(probeMCSLP).then(
  () => undefined,
  (err: unknown) => {
    process.stderr.write(
      `${err instanceof Error ? err.message : String(err)}\n`,
    );
    process.exit(1);
  },
);
