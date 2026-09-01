/**
 * 双车道派发（控制优先级）：业务洪峰下心跳依然可达。
 *
 * 回归背景：业务/控制共队列时，业务 handler 洪峰打满队列 → 心跳被
 * fail-fast 拒绝 → Agent 判定会话死亡 → 过载升级为连接雪崩。
 * 双车道后控制消息（心跳/注册/drain）走独立车道，永不拒绝。
 */

import { TCPTransport, TCPTransportConfig } from "./tcp_transport";
import {
  MSG_INVOKE_REQUEST,
  MSG_INVOKE_RESPONSE,
  MSG_PROVIDER_HEARTBEAT_REQUEST,
  MSG_PROVIDER_HEARTBEAT_RESPONSE,
  MSG_PROVIDER_DRAIN_REQUEST,
  MSG_PROVIDER_CONNECT_REQUEST,
  MSG_REGISTER_REQUEST,
  MSG_REGISTER_CAPABILITIES_REQ,
  isControlRequest,
} from "./protocol";

const VERSION = 0x01;

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

describe("dual-lane inbound dispatch", () => {
  it("classifies control vs business messages", () => {
    for (const id of [
      MSG_PROVIDER_HEARTBEAT_REQUEST,
      MSG_PROVIDER_CONNECT_REQUEST,
      MSG_PROVIDER_DRAIN_REQUEST,
      MSG_REGISTER_REQUEST,
      MSG_REGISTER_CAPABILITIES_REQ,
    ]) {
      expect(isControlRequest(id)).toBe(true);
    }
    for (const id of [
      MSG_INVOKE_REQUEST,
      MSG_INVOKE_RESPONSE,
      0x030103, // start task
      0x030107, // cancel task
    ]) {
      expect(isControlRequest(id)).toBe(false);
    }
  });

  it("answers heartbeats while the business lane is saturated", async () => {
    const server = await startFakeServer();
    const { address, sockets, send, nextFrame, close } = server;

    const t = new TCPTransport({
      address,
      inboundWorkers: 1,
    });
    let businessStarted = false;
    t.setHandler((msgId, _reqId, _body) => {
      if (msgId === MSG_PROVIDER_HEARTBEAT_REQUEST) {
        return Buffer.from("pong");
      }
      businessStarted = true;
      return new Promise<Buffer>((resolve) => {
        // 占住唯一业务 worker 3 秒：队列随即打满
        setTimeout(() => resolve(Buffer.alloc(0)), 3000);
      });
    });
    await t.connect();

    try {
      // 打满业务车道（worker 1 + 队列 4 → 连发 6 个业务请求）
      for (let reqId = 101; reqId <= 106; reqId++) {
        send(MSG_INVOKE_REQUEST, reqId, Buffer.from("x"));
      }
      await waitFor(() => businessStarted);

      // 洪峰中的心跳：控制车道应立即处理
      const start = Date.now();
      send(MSG_PROVIDER_HEARTBEAT_REQUEST, 900, Buffer.alloc(0));
      const resp = await nextFrame(
        (f) => f.msgId === MSG_PROVIDER_HEARTBEAT_RESPONSE,
        2000,
      );
      expect(resp).not.toBeNull();
      expect(resp?.reqId).toBe(900);
      expect(resp?.body.toString()).toBe("pong");
      expect(Date.now() - start).toBeLessThan(2000);

      // 对照：业务队列满 → 新业务请求被 fail-fast 回空帧
      send(MSG_INVOKE_REQUEST, 901, Buffer.from("x"));
      const busy = await nextFrame(
        (f) => f.msgId === MSG_INVOKE_RESPONSE && f.reqId === 901,
        2000,
      );
      expect(busy).not.toBeNull();
      expect(busy?.body.length).toBe(0);
    } finally {
      t.close();
      close();
    }
    expect(sockets.length).toBeGreaterThanOrEqual(0);
  });
});

import { createServer, Server, Socket } from "net";

interface DecodedFrame {
  msgId: number;
  reqId: number;
  body: Buffer;
}

type FakeServer = {
  address: string;
  sockets: Socket[];
  send: (msgId: number, reqId: number, body: Buffer) => void;
  nextFrame: (
    predicate: (f: DecodedFrame) => boolean,
    timeoutMs: number,
  ) => Promise<DecodedFrame | null>;
  close: () => Promise<void>;
};

function startFakeServer(): Promise<FakeServer> {
  return new Promise((resolve) => {
    const server: Server = createServer();
    const sockets: Socket[] = [];
    const frames: DecodedFrame[] = [];
    const waiters: Array<{
      predicate: (f: DecodedFrame) => boolean;
      resolve: (f: DecodedFrame | null) => void;
      timer: ReturnType<typeof setTimeout>;
    }> = [];
    let buffer = Buffer.alloc(0);
    let address = "";

    server.on("connection", (socket) => {
      sockets.push(socket);
      socket.on("data", (chunk: Buffer) => {
        buffer = Buffer.concat([buffer, chunk]);
        for (;;) {
          if (buffer.length < 4) return;
          const size = buffer.readUInt32BE(0);
          if (buffer.length < 4 + size) return;
          const payload = buffer.subarray(4, 4 + size);
          buffer = buffer.subarray(4 + size);
          if (payload.length >= 8) {
            const decoded: DecodedFrame = {
              msgId: payload.readUIntBE(1, 3),
              reqId: payload.readUInt32BE(4),
              body: Buffer.from(payload.subarray(8)),
            };
            const idx = waiters.findIndex((w) => w.predicate(decoded));
            if (idx >= 0) {
              const [w] = waiters.splice(idx, 1);
              clearTimeout(w.timer);
              w.resolve(decoded);
            } else {
              frames.push(decoded);
            }
          }
        }
      });
    });

    server.listen(0, "127.0.0.1", () => {
      const addr = server.address();
      if (addr && typeof addr === "object") {
        address = `127.0.0.1:${addr.port}`;
      }
      resolve({
        address,
        sockets,
        send: (msgId, reqId, body) => {
          const raw = frame(encodeMessage(msgId, reqId, body));
          for (const s of sockets) s.write(raw);
        },
        nextFrame: (predicate, timeoutMs) => {
          const idx = frames.findIndex(predicate);
          if (idx >= 0) {
            const [f] = frames.splice(idx, 1);
            return Promise.resolve(f);
          }
          return new Promise((resolve) => {
            const timer = setTimeout(() => {
              const i = waiters.findIndex((w) => w.resolve === resolve);
              if (i >= 0) waiters.splice(i, 1);
              resolve(null);
            }, timeoutMs);
            waiters.push({ predicate, resolve, timer });
          });
        },
        close: () =>
          new Promise((done) => {
            for (const s of sockets) s.destroy();
            server.close(() => done());
          }),
      });
    });
  });
}

function waitFor(cond: () => boolean, timeoutMs = 2000): Promise<void> {
  const start = Date.now();
  return new Promise((resolve, reject) => {
    const tick = () => {
      if (cond()) return resolve();
      if (Date.now() - start > timeoutMs) {
        return reject(new Error("condition not met in time"));
      }
      setTimeout(tick, 10);
    };
    tick();
  });
}
