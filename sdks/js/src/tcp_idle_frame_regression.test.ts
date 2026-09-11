/**
 * 空闲连接撕帧回归（2026-09-11 线上 node demo 90s 重连循环根因）：
 *
 * 读循环此前用 readFrameWithTimeout（1s 空转超时）包装 readFrame——超时
 * 只是让本轮等待退出，挂起的 readFrame→readExact 不可取消，空闲连接每秒
 * 多挂一个并发读；数据帧到达时多个 readMore 竞争 socket.read(n)，一个读走
 * 4 字节长度前缀、其余把 payload 前 4 字节当长度前缀，帧解析彻底错乱。
 * 后果：agent keepalive 探针/心跳响应丢失 → 会话被判死 → SDK 重连 →
 * 90s 循环重注册。
 *
 * 修复：读循环单读者直接阻塞在 readFrame 上；readExact 挂起读可被
 * close/error 唤醒且 resolve 路径清理监听。以下用例在修复前失败、修复后通过。
 */

import { createServer, Server, Socket } from "net";
import { TCPTransport } from "./tcp_transport";
import {
  MSG_PROVIDER_HEARTBEAT_REQUEST,
  MSG_PROVIDER_HEARTBEAT_RESPONSE,
  getResponseMsgId,
} from "./protocol";

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

/** 真 TCP fake agent：只做帧协议解析/回写，不掺业务语义 */
class FrameAgent {
  private server: Server;
  private socket: Socket | null = null;
  private frames: DecodedFrame[] = [];
  private waiters: Array<{
    predicate: (f: DecodedFrame) => boolean;
    resolve: (f: DecodedFrame) => void;
  }> = [];
  private buffer = Buffer.alloc(0);
  address = "";

  constructor() {
    this.server = createServer((socket) => {
      this.socket = socket;
      socket.on("data", (chunk: Buffer) => this.feed(chunk));
    });
  }

  private feed(chunk: Buffer): void {
    this.buffer = Buffer.concat([this.buffer, chunk]);
    for (;;) {
      if (this.buffer.length < 4) {
        return;
      }
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
    this.socket?.write(frame(encodeMessage(msgId, reqId, body)));
  }

  /** 模拟 agent keepalive 探针：发心跳请求并等 pong */
  async probe(reqId: number): Promise<DecodedFrame> {
    this.send(MSG_PROVIDER_HEARTBEAT_REQUEST, reqId, Buffer.alloc(0));
    return this.waitFrame(
      (f) => f.msgId === MSG_PROVIDER_HEARTBEAT_RESPONSE && f.reqId === reqId,
      2000,
    );
  }

  destroyClientConnection(): void {
    this.socket?.destroy();
  }

  async close(): Promise<void> {
    this.socket?.destroy();
    await new Promise<void>((resolve) => {
      this.server.close(() => resolve());
    });
  }
}

describe("TCPTransport idle-connection frame integrity", () => {
  let agent: FrameAgent;
  let transport: TCPTransport;

  beforeEach(async () => {
    agent = new FrameAgent();
    const address = await agent.start();
    transport = new TCPTransport({ address, timeoutMs: 5000 });
    await transport.connect();
  });

  afterEach(async () => {
    transport.close();
    await agent.close();
  });

  it("answers a keepalive probe after an idle period (no concurrent-read tearing)", async () => {
    transport.setHandler(() => Buffer.alloc(0));

    // 空闲 3s：修复前会挂 2-3 个并发 readExact（1s 超时各泄漏一个），
    // 探针帧到达即被并发读撕裂；修复后读循环单读者阻塞，帧完整。
    await new Promise((r) => setTimeout(r, 3000));

    const pong = await agent.probe(9001);
    expect(pong.msgId).toBe(MSG_PROVIDER_HEARTBEAT_RESPONSE);
    expect(pong.reqId).toBe(9001);
  });

  it("round-trips a call after an idle period", async () => {
    // 空闲后再发起请求-响应：读路径仍完整、pending 路由不受空闲影响。
    await new Promise((r) => setTimeout(r, 1500));

    const callPromise = transport.call(
      MSG_PROVIDER_HEARTBEAT_REQUEST,
      Buffer.from("ping"),
    );
    const req = await agent.waitFrame(
      (f) => f.msgId === MSG_PROVIDER_HEARTBEAT_REQUEST && f.body.length > 0,
    );
    agent.send(MSG_PROVIDER_HEARTBEAT_RESPONSE, req.reqId, Buffer.from("pong"));

    const [respMsgId, respBody] = await callPromise;
    expect(respMsgId).toBe(getResponseMsgId(MSG_PROVIDER_HEARTBEAT_REQUEST));
    expect(respBody.toString()).toBe("pong");
  });

  it("fails pending calls promptly when the peer drops the connection", async () => {
    // 远端断开时读循环必须立即退出并唤醒挂起调用（而不是等到请求超时），
    // 上层心跳才能快速触发重连。
    const callPromise = transport.call(
      MSG_PROVIDER_HEARTBEAT_REQUEST,
      Buffer.alloc(0),
    );
    await agent.waitFrame(
      (f) => f.msgId === MSG_PROVIDER_HEARTBEAT_REQUEST,
    );
    agent.destroyClientConnection();

    await expect(callPromise).rejects.toThrow(/connection closed/);
    expect(transport.isConnected()).toBe(false);
  });
});
