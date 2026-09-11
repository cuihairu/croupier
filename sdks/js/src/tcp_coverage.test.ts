/**
 * TCPTransport coverage boost: partial-chunk frame reassembly (readExact
 * recursion), inbound queue saturation with a failing socket write (read-loop
 * catch path), and the idle read timeout resolution that keeps the reader
 * loop alive between frames.
 */

import { createServer, Server, Socket } from "net";
import { TCPTransport } from "./tcp_transport";
import { MSG_INVOKE_REQUEST, MSG_INVOKE_RESPONSE } from "./protocol";

const VERSION = 0x01;

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

/** Minimal peer: records decoded frames, never auto-replies. */
class RecordingPeer {
  private server: Server;
  private sockets: Socket[] = [];
  private frames: DecodedFrame[] = [];
  private waiters: Array<{
    predicate: (f: DecodedFrame) => boolean;
    resolve: (f: DecodedFrame) => void;
  }> = [];
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
    for (;;) {
      if (this.buffer.length < 4) return;
      const size = this.buffer.readUInt32BE(0);
      if (this.buffer.length < 4 + size) return;
      const payload = this.buffer.subarray(4, 4 + size);
      this.buffer = this.buffer.subarray(4 + size);
      if (payload.length >= 8) {
        const decoded: DecodedFrame = {
          msgId: payload.readUIntBE(1, 3),
          reqId: payload.readUInt32BE(4),
          body: Buffer.from(payload.subarray(8)),
        };
        const idx = this.waiters.findIndex((w) => w.predicate(decoded));
        if (idx >= 0) {
          const [w] = this.waiters.splice(idx, 1);
          w.resolve(decoded);
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

  waitFrame(
    predicate: (f: DecodedFrame) => boolean,
    timeoutMs = 3000,
  ): Promise<DecodedFrame> {
    const idx = this.frames.findIndex(predicate);
    if (idx >= 0) {
      const [f] = this.frames.splice(idx, 1);
      return Promise.resolve(f);
    }
    return new Promise((resolve, reject) => {
      const timer = setTimeout(
        () => reject(new Error("timed out waiting for frame")),
        timeoutMs,
      );
      this.waiters.push({
        predicate,
        resolve: (f) => {
          clearTimeout(timer);
          resolve(f);
        },
      });
    });
  }

  send(msgId: number, reqId: number, body: Buffer): void {
    const raw = frame(encodeMessage(msgId, reqId, body));
    for (const s of this.sockets) s.write(raw);
  }

  /** Reply with a response frame split across two TCP writes. */
  sendSplitResponse(reqId: number, body: Buffer, firstBytes: number, gapMs: number): void {
    const raw = frame(encodeMessage(MSG_INVOKE_RESPONSE, reqId, body));
    for (const s of this.sockets) {
      s.write(raw.subarray(0, firstBytes));
      setTimeout(() => s.write(raw.subarray(firstBytes)), gapMs);
    }
  }

  async close(): Promise<void> {
    for (const s of this.sockets) s.destroy();
    await new Promise<void>((resolve) => {
      this.server.close(() => resolve());
    });
  }
}

describe("TCPTransport coverage corners", () => {
  let peer: RecordingPeer;
  let transports: TCPTransport[];

  beforeEach(async () => {
    peer = new RecordingPeer();
    await peer.start();
    transports = [];
  });

  afterEach(async () => {
    for (const t of transports) t.close();
    await peer.close();
  });

  function makeTransport(config: Record<string, unknown> = {}): TCPTransport {
    const t = new TCPTransport({ address: peer.address, timeoutMs: 5000, ...config });
    transports.push(t);
    return t;
  }

  it("reassembles a response frame delivered as partial TCP writes", async () => {
    const t = makeTransport();
    await t.connect();

    const pending = t.call(MSG_INVOKE_REQUEST, Buffer.from("split"));
    const req = await peer.waitFrame((f) => f.msgId === MSG_INVOKE_REQUEST);
    // 2 bytes first (< the 4-byte length prefix), remainder after a gap:
    // readExact must recurse on the short chunk.
    peer.sendSplitResponse(req.reqId, Buffer.from("chunked"), 2, 40);

    const [respMsgId, respBody] = await pending;
    expect(respMsgId).toBe(MSG_INVOKE_RESPONSE);
    expect(respBody.toString()).toBe("chunked");
  });

  it("survives a saturated inbound queue with a failing socket write", async () => {
    const t = makeTransport({ inboundWorkers: 1, timeoutMs: 2000 });
    let release: () => void = () => {};
    const gate = new Promise<void>((r) => {
      release = r;
    });
    t.setHandler(async () => {
      await gate;
      return Buffer.from("late");
    });
    await t.connect();

    // Break socket writes so the fail-fast response on queue overflow throws
    // synchronously inside the reader loop.
    const socket = (t as unknown as { socket: Socket }).socket;
    const originalWrite = socket.write.bind(socket);
    socket.write = (() => {
      throw new Error("simulated write failure");
    }) as typeof socket.write;

    // capacity = 1 worker * 4 queue slots: frames 1-5 occupy worker+queue,
    // frame 6+ hits the fail-fast branch whose sendResponse throws.
    for (let i = 0; i < 7; i++) {
      peer.send(MSG_INVOKE_REQUEST, 100 + i, Buffer.from("x"));
    }
    await new Promise((r) => setTimeout(r, 100));
    // 写失败意味着连接已坏：读循环退出、连接显式断开（旧实现吞掉错误
    // 假装存活，响应实际再也发不出去）。
    expect(t.isConnected()).toBe(false);

    // 恢复写并放行 gated handler：进程不崩溃、worker 车道正常收尾。
    socket.write = originalWrite;
    release();
    await new Promise((r) => setTimeout(r, 50));
  }, 10000);

  it("fails pending calls when a truncated frame hits EOF", async () => {
    // Peer sends a partial length prefix (2 bytes) then half-closes: at EOF
    // the pending read is woken by the end event, the reader exits and the
    // in-flight call fails immediately instead of parking until timeout.
    const server = createServer((socket) => {
      socket.once("data", () => {
        socket.write(Buffer.from([0x00, 0x00]));
        socket.end();
      });
    });
    const address = await new Promise<string>((resolve) => {
      server.listen(0, "127.0.0.1", () => {
        const addr = server.address();
        resolve(`127.0.0.1:${addr && typeof addr === "object" ? addr.port : 0}`);
      });
    });

    const t = new TCPTransport({ address, timeoutMs: 5000 });
    transports.push(t);
    await t.connect();
    // Trigger a request so the peer's connection handler fires; the reply
    // never arrives as a complete frame.
    const pending = t.call(MSG_INVOKE_REQUEST, Buffer.from("x"));
    await expect(pending).rejects.toThrow(/connection closed/);
    expect(t.isConnected()).toBe(false);

    t.close();
    await new Promise<void>((resolve) => {
      server.close(() => resolve());
    });
  }, 10000);

  it("keeps the connection up through an idle period", async () => {
    const t = makeTransport();
    await t.connect();

    // Stay silent: the reader blocks on a single pending read (no polling
    // timeout) and the connection must stay healthy across the idle window.
    await new Promise((r) => setTimeout(r, 1200));
    expect(t.isConnected()).toBe(true);
  }, 10000);
});
