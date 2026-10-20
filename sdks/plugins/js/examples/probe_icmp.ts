#!/usr/bin/env node
/**
 * Reference ICMP probe plugin. Requires CAP_NET_RAW on Linux; missing
 * capability is returned as a normal failed CheckResult.
 *
 * Payload: {"host": "127.0.0.1", "count": 1}
 * PROBE_ICMP_FORCE_CAPABILITY_ERROR=1 provides deterministic CI coverage.
 */
import dgram from "node:dgram";
import { Buffer } from "node:buffer";
import { randomBytes } from "node:crypto";
import { performance } from "node:perf_hooks";
import { isIP } from "node:net";
import { lookup } from "node:dns/promises";
import { serve, passResult, failResult, type ProbePlugin, type ProbeCall } from "../src";

const PLUGIN_NAME = "probe-icmp";
const ICMP_ECHO = 8;
const ICMP_ECHO_REPLY = 0;

function checksum(buffer: Buffer): number {
  let total = 0;
  for (let index = 0; index < buffer.length - 1; index += 2) {
    total += buffer.readUInt16BE(index);
  }
  if (buffer.length % 2) total += buffer[buffer.length - 1] << 8;
  while (total >> 16) total = (total & 0xffff) + (total >> 16);
  return (~total & 0xffff);
}

function createPacket(identifier: number, sequence: number): Buffer {
  const header = Buffer.alloc(8);
  header.writeUInt8(ICMP_ECHO, 0);
  header.writeUInt8(0, 1);
  const payload = randomBytes(8);
  header.writeUInt16BE(checksum(Buffer.concat([header, payload])), 2);
  header.writeUInt16BE(identifier, 4);
  header.writeUInt16BE(sequence, 6);
  return Buffer.concat([header, payload]);
}

function pingOnce(
  address: string,
  identifier: number,
  sequence: number,
  timeoutMs: number,
): Promise<void> {
  return new Promise((resolve, reject) => {
    const socket = dgram.createSocket(isIP(address) === 6 ? "udp6" : "udp4");
    const packet = createPacket(identifier, sequence);
    const done = (error?: Error) => {
      socket.removeAllListeners("message");
      socket.removeAllListeners("error");
      clearTimeout(timer);
      socket.close();
      if (error) reject(error);
      else resolve();
    };
    const timer = setTimeout(() => done(new Error("timeout: no ICMP reply")), timeoutMs);
    socket.on("message", (buffer) => {
      const offset = address.includes(":") ? 4 : 20;
      if (buffer.length < offset + 8) return;
      const icmp = buffer.subarray(offset);
      if (
        icmp.readUInt8(0) === ICMP_ECHO_REPLY &&
        icmp.readUInt16BE(4) === identifier &&
        icmp.readUInt16BE(6) === sequence
      ) {
        done();
      }
    });
    socket.on("error", done);
    socket.send(packet, 0, packet.length, 0, address, (error) => {
      if (error) done(error);
    });
  });
}

const probeICMP: ProbePlugin = {
  describe: () => ({ name: PLUGIN_NAME, version: "1.0.0" }),
  check: async (call: ProbeCall) => {
    const started = performance.now();
    let host: unknown;
    let count = 1;
	let forceCapabilityError = false;
    try {
		const payload = JSON.parse(call.payloadJson) as {
			host?: unknown;
			count?: unknown;
			force_capability_error?: unknown;
		};
      host = payload.host;
      count = Number(payload.count ?? 1);
		forceCapabilityError = payload.force_capability_error === true;
    } catch {
      host = undefined;
    }
    if (typeof host !== "string" || !host || !Number.isInteger(count) || (count as number) < 1) {
      return failResult("configuration: payload must contain host", { latencyMs: elapsed(started) });
    }
    let address: string;
    try {
      address = isIP(host) ? host : (await lookup(host)).address;
    } catch (error) {
      return failResult(`dns: ${text(error)}`, { latencyMs: elapsed(started) });
    }

    const identifier = process.pid & 0xffff;
	if (forceCapabilityError) {
      return failResult("capability: CAP_NET_RAW is required for ICMP: forced by test", {
        latencyMs: elapsed(started),
      });
    }
    for (let attempt = 0; attempt < (count as number); attempt++) {
      const remaining = Math.max(1, call.remainingMs());
      try {
        await pingOnce(address, identifier, attempt + 1, remaining);
        return passResult({
          status: 200,
          latencyMs: elapsed(started),
          detail: `probe-icmp received reply from ${host}`,
        });
      } catch (error) {
        const reason = text(error);
        if (!reason.startsWith("timeout:")) {
          return failResult(reason, { latencyMs: elapsed(started) });
        }
      }
    }
    return failResult("timeout: no ICMP reply", { latencyMs: elapsed(started) });
  },
};

function elapsed(started: number): number {
  return Math.floor(performance.now() - started);
}

function text(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

void serve(probeICMP).then(
  () => undefined,
  (error: unknown) => {
    process.stderr.write(`${text(error)}\n`);
    process.exit(1);
  },
);
