/**
 * AUD-060: connection-generation semantics for concurrent dial / close /
 * reconnect. Each test drives fake sockets through jest.mock("net") so the
 * interleavings (close during an in-flight dial, a stale dial completing
 * late, an old reader loop waking up after a reconnect) are deterministic.
 */

import { EventEmitter } from "events";

class FakeSocket extends EventEmitter {
  destroyed = false;
  writes: Buffer[] = [];
  private buf = Buffer.alloc(0);

  setTimeout(_ms: number, _fn?: () => void): void {
    // no-op: dial timeout is driven by the transport's own timer
  }

  read(n?: number): Buffer | null {
    if (this.buf.length === 0) {
      return null;
    }
    const take = Math.min(n ?? this.buf.length, this.buf.length);
    const out = Buffer.from(this.buf.subarray(0, take));
    this.buf = this.buf.subarray(take);
    return out;
  }

  write(data: Buffer): boolean {
    this.writes.push(data);
    return true;
  }

  destroy(): void {
    this.destroyed = true;
  }

  push(chunk: Buffer): void {
    this.buf = Buffer.concat([this.buf, chunk]);
  }
}

const mockSockets: FakeSocket[] = [];

jest.mock("net", () => ({
  __esModule: true,
  createConnection: () => {
    const s = new FakeSocket();
    mockSockets.push(s);
    return s;
  },
}));

import { TCPTransport, TCPTransportConfig } from "./tcp_transport";
import {
  MSG_INVOKE_REQUEST,
  MSG_PROVIDER_HEARTBEAT_REQUEST,
  MSG_PROVIDER_HEARTBEAT_RESPONSE,
} from "./protocol";

const VERSION = 0x01;

function cfg(): TCPTransportConfig {
  return { address: "127.0.0.1:19091", connectTimeoutMs: 200, timeoutMs: 200 };
}

function frame(msgId: number, reqId: number, body: Buffer): Buffer {
  const header = Buffer.alloc(8);
  header.writeUInt8(VERSION, 0);
  header.writeUIntBE(msgId, 1, 3);
  header.writeUInt32BE(reqId, 4);
  const payload = Buffer.concat([header, body]);
  const prefix = Buffer.alloc(4);
  prefix.writeUInt32BE(payload.length, 0);
  return Buffer.concat([prefix, payload]);
}

function flushMicrotasks(): Promise<void> {
  return new Promise((resolve) => setImmediate(resolve));
}

beforeEach(() => {
  mockSockets.length = 0;
});

test("close during an in-flight dial aborts the late session", async () => {
  const t = new TCPTransport(cfg());
  const connecting = t.connect();
  expect(mockSockets).toHaveLength(1);
  const dial = mockSockets[0];

  // 拨号在途时关闭：代次前移，迟到的 connect 事件不得建立会话
  t.close();
  dial.emit("connect");

  await expect(connecting).rejects.toThrow("connection closed");
  expect(dial.destroyed).toBe(true);
  expect(t.isConnected()).toBe(false);
});

test("concurrent dials: only the latest generation wins", async () => {
  const t = new TCPTransport(cfg());
  const p1 = t.connect();
  const p2 = t.connect();
  expect(mockSockets).toHaveLength(2);
  const [s1, s2] = mockSockets;

  s1.emit("connect"); // 旧代次 → 废弃
  await expect(p1).rejects.toThrow("connection closed");
  expect(s1.destroyed).toBe(true);

  s2.emit("connect"); // 当前代次 → 生效
  await expect(p2).resolves.toBeUndefined();
  expect(t.isConnected()).toBe(true);
});

test("old reader loop does not clobber the new generation after reconnect", async () => {
  const t = new TCPTransport(cfg());
  const first = t.connect();
  mockSockets[0].emit("connect");
  await first;
  const oldSocket = mockSockets[0];

  const firstCall = t.call(MSG_PROVIDER_HEARTBEAT_REQUEST, Buffer.alloc(0));
  t.close(); // 旧连接死亡：firstCall 失败、代次前移
  await expect(firstCall).rejects.toThrow("connection closed");

  // 重连：新代次接管
  const reconnecting = t.connect();
  const newSocket = mockSockets[mockSockets.length - 1];
  expect(newSocket).not.toBe(oldSocket);
  newSocket.emit("connect");
  await reconnecting;

  const secondCall = t.call(MSG_PROVIDER_HEARTBEAT_REQUEST, Buffer.alloc(0));

  // 旧读循环此刻才醒来（destroy 的 close 事件到达）：不得误清新连接状态
  oldSocket.emit("close");
  await flushMicrotasks();
  expect(t.isConnected()).toBe(true);

  // 新连接照常处理响应：secondCall 正常完成
  const reqId = 2; // 第一次 call 占用 reqId 1（已随旧连接失败）
  newSocket.push(frame(MSG_PROVIDER_HEARTBEAT_RESPONSE, reqId, Buffer.from("ok")));
  newSocket.emit("readable");
  const [respMsgId, body] = await secondCall;
  expect(respMsgId).toBe(MSG_PROVIDER_HEARTBEAT_RESPONSE);
  expect(body.toString()).toBe("ok");
});

test("close clears inbound and control queues of the dead connection", async () => {
  // 单 worker：第一个入站请求占住唯一 worker，第二个滞留队列
  const t = new TCPTransport({ ...cfg(), inboundWorkers: 1 });
  const connecting = t.connect();
  const s1 = mockSockets[0];
  s1.emit("connect");
  await connecting;

  // handler 永不返回：第一个入站请求占住 worker，第二个滞留队列
  t.setHandler(() => new Promise<Buffer>(() => {}));
  const MSG_INVOKE = MSG_INVOKE_REQUEST; // 业务车道 invoke 请求
  s1.push(frame(MSG_INVOKE, 1, Buffer.from("a")));
  s1.emit("readable");
  await flushMicrotasks();
  s1.push(frame(MSG_INVOKE, 2, Buffer.from("b")));
  s1.emit("readable");
  await flushMicrotasks();

  const queued = t as unknown as { inboundQueue: unknown[]; controlQueue: unknown[] };
  expect(queued.inboundQueue.length).toBeGreaterThan(0);

  t.close();
  expect(queued.inboundQueue).toHaveLength(0);
  expect(queued.controlQueue).toHaveLength(0);
});

test("stale call does not write frames to the reconnected socket", async () => {
  const t = new TCPTransport(cfg());
  const first = t.connect();
  mockSockets[0].emit("connect");
  await first;
  const s1 = mockSockets[0];

  const staleCall = t.call(MSG_PROVIDER_HEARTBEAT_REQUEST, Buffer.alloc(0));
  t.close();
  const reconnecting = t.connect();
  const s2 = mockSockets[mockSockets.length - 1];
  s2.emit("connect");
  await reconnecting;

  // 旧代次 call 的写路径醒来：代次/插座核验拦截，不向新连接写帧
  await expect(staleCall).rejects.toThrow("connection closed");
  await flushMicrotasks();
  expect(s2.writes).toHaveLength(0);
});
