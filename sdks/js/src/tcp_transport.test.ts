/**
 * TCPTransport tests driven against a real local TCP server that emulates
 * the Croupier Agent wire protocol:
 *   Frame:   [4-byte length prefix (BE)][payload]
 *   Payload: [version(1B)][msgId(3B)][reqId(4B)][body]
 */

import { createServer, Server, Socket } from "net";
import { TCPTransport, TCPTransportConfig } from "./tcp_transport";
import {
  MSG_INVOKE_REQUEST,
  MSG_INVOKE_RESPONSE,
  MSG_PROVIDER_HEARTBEAT_REQUEST,
  MSG_PROVIDER_HEARTBEAT_RESPONSE,
} from "./protocol";

const VERSION = 0x01;
const MAX_FRAME_BYTES = 32 * 1024 * 1024;

interface DecodedFrame {
  msgId: number;
  reqId: number;
  body: Buffer;
}

function encodeMessage(msgId: number, reqId: number, body: Buffer): Buffer {
  const header = Buffer.alloc(8);
  header.writeUInt8(VERSION, 0);
  header.writeUIntBE(msgId, 1, 3);
  header.writeUInt32BE(reqId, 4);
  return Buffer.concat([header, body]);
}

function frame(payload: Buffer): Buffer {
  const prefix = Buffer.alloc(4);
  prefix.writeUInt32BE(payload.length, 0);
  return Buffer.concat([prefix, payload]);
}

/** Minimal in-process agent: accepts one or more connections, decodes frames. */
class FakeAgent {
  private server: Server;
  private sockets: Socket[] = [];
  private frames: DecodedFrame[] = [];
  private waiters: Array<(f: DecodedFrame) => void> = [];
  private buffer = Buffer.alloc(0);
  address = "";

  constructor() {
    this.server = createServer((socket) => {
      this.sockets.push(socket);
      socket.on("data", (chunk: Buffer) => this.feed(chunk));
    });
  }

  private feed(chunk: Buffer): void {
    this.buffer = Buffer.concat([this.buffer, chunk]);
    while (this.buffer.length >= 4) {
      const size = this.buffer.readUInt32BE(0);
      if (this.buffer.length < 4 + size) {
        return;
      }
      const payload = this.buffer.subarray(4, 4 + size);
      this.buffer = this.buffer.subarray(4 + size);
      if (payload.length >= 8) {
        const decoded: DecodedFrame = {
          msgId: payload.readUIntBE(1, 3),
          reqId: payload.readUInt32BE(4),
          body: Buffer.from(payload.subarray(8)),
        };
        const waiter = this.waiters.shift();
        if (waiter) {
          waiter(decoded);
        } else {
          this.frames.push(decoded);
        }
      }
    }
  }

  async start(): Promise<string> {
    return new Promise((resolve) => {
      this.server.listen(0, "127.0.0.1", () => {
        const addr = this.server.address();
        if (addr && typeof addr === "object") {
          this.address = `127.0.0.1:${addr.port}`;
        }
        resolve(this.address);
      });
    });
  }

  nextFrame(timeoutMs = 2000): Promise<DecodedFrame> {
    const existing = this.frames.shift();
    if (existing) {
      return Promise.resolve(existing);
    }
    return new Promise((resolve, reject) => {
      const timer = setTimeout(
        () => reject(new Error("timed out waiting for frame")),
        timeoutMs,
      );
      this.waiters.push((f) => {
        clearTimeout(timer);
        resolve(f);
      });
    });
  }

  send(msgId: number, reqId: number, body: Buffer): void {
    const raw = frame(encodeMessage(msgId, reqId, body));
    for (const s of this.sockets) {
      s.write(raw);
    }
  }

  writeRaw(data: Buffer): void {
    for (const s of this.sockets) {
      s.write(data);
    }
  }

  destroySockets(): void {
    for (const s of this.sockets) {
      s.destroy();
    }
  }

  async close(): Promise<void> {
    this.destroySockets();
    await new Promise<void>((resolve) => {
      this.server.close(() => resolve());
    });
  }
}

describe("TCPTransport", () => {
  let agent: FakeAgent;
  let transports: TCPTransport[];

  beforeEach(async () => {
    agent = new FakeAgent();
    await agent.start();
    transports = [];
  });

  afterEach(async () => {
    for (const t of transports) {
      t.close();
    }
    await agent.close();
  });

  function makeTransport(
    config: TCPTransportConfig = {},
    track = true,
  ): TCPTransport {
    const t = new TCPTransport({
      address: agent.address,
      timeoutMs: 5000,
      ...config,
    });
    if (track) {
      transports.push(t);
    }
    return t;
  }

  describe("connection management", () => {
    it("connects to a local TCP server", async () => {
      const t = makeTransport();
      await t.connect();
      expect(t.isConnected()).toBe(true);
    });

    it("connect is idempotent when already connected", async () => {
      const t = makeTransport();
      await t.connect();
      await expect(t.connect()).resolves.toBeUndefined();
      expect(t.isConnected()).toBe(true);
    });

    it("strips tcp:// scheme from the address", async () => {
      const t = makeTransport({ address: `tcp://${agent.address}` });
      await t.connect();
      expect(t.isConnected()).toBe(true);
    });

    it("is not connected before connect and after close", async () => {
      const t = makeTransport();
      expect(t.isConnected()).toBe(false);
      await t.connect();
      expect(t.isConnected()).toBe(true);
      t.close();
      expect(t.isConnected()).toBe(false);
    });

    it("rejects when the server refuses the connection", async () => {
      const hold = new FakeAgent();
      await hold.start();
      const addr = hold.address;
      await hold.close();

      const t = makeTransport({ address: addr, connectTimeoutMs: 2000 });
      await expect(t.connect()).rejects.toThrow(/Failed to connect/);
      expect(t.isConnected()).toBe(false);
    });

    it("times out when the host is unreachable", async () => {
      const t = makeTransport({
        address: "10.255.255.1:9999",
        connectTimeoutMs: 300,
      });
      await expect(t.connect()).rejects.toThrow(
        /Connection timeout|Failed to connect/,
      );
    });

    it("setConnectTimeout updates the connect timeout", () => {
      const t = makeTransport();
      t.setConnectTimeout(1234);
      expect((t as unknown as { connectTimeoutMs: number }).connectTimeoutMs).toBe(1234);
    });

    it("close before connect is a no-op", () => {
      const t = new TCPTransport();
      expect(() => t.close()).not.toThrow();
    });

    it("still reports connected after remote disconnect (documented behavior)", async () => {
      const t = makeTransport();
      await t.connect();
      agent.destroySockets();
      await new Promise((r) => setTimeout(r, 100));
      // Current implementation has no disconnect detection; it keeps
      // reporting connected until close() is called explicitly.
      expect(t.isConnected()).toBe(true);
    });
  });

  describe("request/response frames", () => {
    it("round-trips a request and response frame", async () => {
      const t = makeTransport();
      await t.connect();

      const pending = t.call(MSG_INVOKE_REQUEST, Buffer.from("ping"));
      const req = await agent.nextFrame();
      expect(req.msgId).toBe(MSG_INVOKE_REQUEST);
      expect(req.body.toString()).toBe("ping");
      expect(req.reqId).toBeGreaterThan(0);

      agent.send(MSG_INVOKE_RESPONSE, req.reqId, Buffer.from("pong"));
      const [respMsgId, respBody] = await pending;
      expect(respMsgId).toBe(MSG_INVOKE_RESPONSE);
      expect(respBody.toString()).toBe("pong");
    });

    it("increments request ids across calls", async () => {
      const t = makeTransport();
      await t.connect();

      const first = t.call(MSG_INVOKE_REQUEST, Buffer.from("a"));
      const req1 = await agent.nextFrame();
      agent.send(MSG_INVOKE_RESPONSE, req1.reqId, Buffer.from("1"));
      await first;

      const second = t.call(MSG_INVOKE_REQUEST, Buffer.from("b"));
      const req2 = await agent.nextFrame();
      agent.send(MSG_INVOKE_RESPONSE, req2.reqId, Buffer.from("2"));
      await second;

      expect(req2.reqId).toBe(req1.reqId + 1);
    });

    it("throws when calling before connect", async () => {
      const t = makeTransport();
      await expect(
        t.call(MSG_INVOKE_REQUEST, Buffer.from("x")),
      ).rejects.toThrow("Not connected");
    });

    it("times out when no response arrives", async () => {
      const t = makeTransport({ timeoutMs: 200 });
      await t.connect();

      const pending = t.call(MSG_INVOKE_REQUEST, Buffer.from("x"));
      const req = await agent.nextFrame();
      expect(req.body.toString()).toBe("x");
      // Server intentionally never responds.
      await expect(pending).rejects.toThrow("timeout");
    });

    it("multiplexes concurrent calls and matches responses by reqId", async () => {
      const t = makeTransport();
      await t.connect();

      const p1 = t.call(MSG_INVOKE_REQUEST, Buffer.from("one"));
      const p2 = t.call(MSG_PROVIDER_HEARTBEAT_REQUEST, Buffer.from("two"));
      const p3 = t.call(MSG_INVOKE_REQUEST, Buffer.from("three"));

      const f1 = await agent.nextFrame();
      const f2 = await agent.nextFrame();
      const f3 = await agent.nextFrame();
      expect([f1.body.toString(), f2.body.toString(), f3.body.toString()]).toEqual([
        "one",
        "two",
        "three",
      ]);

      // Respond in reverse order.
      agent.send(MSG_INVOKE_RESPONSE, f3.reqId, Buffer.from("r3"));
      agent.send(MSG_PROVIDER_HEARTBEAT_RESPONSE, f2.reqId, Buffer.from("r2"));
      agent.send(MSG_INVOKE_RESPONSE, f1.reqId, Buffer.from("r1"));

      expect((await p1)[1].toString()).toBe("r1");
      expect((await p2)[1].toString()).toBe("r2");
      expect((await p3)[1].toString()).toBe("r3");
    });

    it("ignores responses for unknown request ids", async () => {
      const t = makeTransport();
      await t.connect();

      const pending = t.call(MSG_INVOKE_REQUEST, Buffer.from("x"));
      const req = await agent.nextFrame();

      // Stale response for a request id nobody waits on.
      agent.send(MSG_INVOKE_RESPONSE, req.reqId + 1000, Buffer.from("stale"));
      agent.send(MSG_INVOKE_RESPONSE, req.reqId, Buffer.from("fresh"));

      const [, body] = await pending;
      expect(body.toString()).toBe("fresh");
    });

    it("skips zero-length frames", async () => {
      const t = makeTransport();
      await t.connect();

      const pending = t.call(MSG_INVOKE_REQUEST, Buffer.from("x"));
      const req = await agent.nextFrame();

      agent.writeRaw(Buffer.alloc(4)); // length prefix of 0
      agent.send(MSG_INVOKE_RESPONSE, req.reqId, Buffer.from("ok"));

      const [, body] = await pending;
      expect(body.toString()).toBe("ok");
    });

    it("swallows oversized frame headers without crashing the reader", async () => {
      const t = makeTransport({ timeoutMs: 300 });
      await t.connect();

      const pending = t.call(MSG_INVOKE_REQUEST, Buffer.from("x"));
      await agent.nextFrame();

      const oversized = Buffer.alloc(4);
      oversized.writeUInt32BE(MAX_FRAME_BYTES + 1, 0);
      agent.writeRaw(oversized);
      agent.writeRaw(Buffer.from("garbage-that-is-not-a-frame"));

      // The reader discards the error; the call eventually times out
      // and the transport stays usable from the caller's perspective.
      await expect(pending).rejects.toThrow("timeout");
      expect(t.isConnected()).toBe(true);
    });

    it("reassembles frames delivered byte by byte", async () => {
      const t = makeTransport();
      await t.connect();

      const pending = t.call(MSG_INVOKE_REQUEST, Buffer.from("x"));
      const req = await agent.nextFrame();

      const raw = frame(encodeMessage(MSG_INVOKE_RESPONSE, req.reqId, Buffer.from("chunked")));
      for (const b of raw) {
        agent.writeRaw(Buffer.from([b]));
      }

      const [, body] = await pending;
      expect(body.toString()).toBe("chunked");
    });

    it("fails pending calls with connection closed on close()", async () => {
      const t = makeTransport();
      await t.connect();

      const pending = t.call(MSG_INVOKE_REQUEST, Buffer.from("x"));
      await agent.nextFrame();

      t.close();
      await expect(pending).rejects.toThrow("connection closed");
    });
  });

  describe("inbound request handling", () => {
    it("delivers inbound requests to the handler and auto-replies", async () => {
      const t = makeTransport();
      t.setHandler((msgId, reqId, body) => {
        expect(msgId).toBe(MSG_INVOKE_REQUEST);
        expect(reqId).toBe(4242);
        return Buffer.from(`echo:${body.toString()}`);
      });
      await t.connect();

      agent.send(MSG_INVOKE_REQUEST, 4242, Buffer.from("hi"));

      const resp = await agent.nextFrame();
      expect(resp.msgId).toBe(MSG_INVOKE_RESPONSE);
      expect(resp.reqId).toBe(4242);
      expect(resp.body.toString()).toBe("echo:hi");
    });

    it("awaits async handlers before replying", async () => {
      const t = makeTransport();
      t.setHandler(async (_msgId, _reqId, body) => {
        await new Promise((r) => setTimeout(r, 30));
        return Buffer.from(`async:${body.toString()}`);
      });
      await t.connect();

      agent.send(MSG_PROVIDER_HEARTBEAT_REQUEST, 7, Buffer.from("b"));
      const resp = await agent.nextFrame();
      expect(resp.msgId).toBe(MSG_PROVIDER_HEARTBEAT_RESPONSE);
      expect(resp.reqId).toBe(7);
      expect(resp.body.toString()).toBe("async:b");
    });

    it("drops inbound requests when no handler is registered", async () => {
      const t = makeTransport();
      await t.connect();

      agent.send(MSG_INVOKE_REQUEST, 11, Buffer.from("nobody-home"));
      await expect(agent.nextFrame(150)).rejects.toThrow(/timed out/);
    });

    it("drops inbound requests when the handler throws", async () => {
      const t = makeTransport();
      t.setHandler(() => {
        throw new Error("handler exploded");
      });
      await t.connect();

      agent.send(MSG_INVOKE_REQUEST, 12, Buffer.from("boom"));
      await expect(agent.nextFrame(150)).rejects.toThrow(/timed out/);
    });

    it("sendResponse is a no-op when not connected", () => {
      const t = makeTransport();
      expect(() =>
        t.sendResponse(MSG_INVOKE_RESPONSE, 1, Buffer.from("x")),
      ).not.toThrow();
    });
  });
});
