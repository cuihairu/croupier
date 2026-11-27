/**
 * TCP Transport Layer for Croupier JavaScript SDK
 *
 * Implements bidirectional multiplexed TCP transport for communication
 * with Croupier Agent using a single TCP connection.
 *
 * Wire Protocol:
 *   Frame:   [4-byte length prefix (big-endian)][payload]
 *   Payload: [8-byte header][protobuf body]
 *   Header:  Version(1B) + MsgID(3B) + RequestID(4B)
 *
 * Request messages have odd MsgID, Response messages have even MsgID.
 * Multiple concurrent request/response pairs multiplex on the same connection.
 */

import os from "os";
import { readFileSync } from "fs";
import { Socket, createConnection, isIP, TcpSocketConnectOpts } from "net";
import { connect as tlsConnect, ConnectionOptions, TLSSocket } from "tls";
import { getResponseMsgId, isControlRequest } from "./protocol";

/** Frame constants */
const FRAME_HEADER_BYTES = 4; // 4-byte big-endian length prefix
const MAX_FRAME_BYTES = 32 * 1024 * 1024; // 32 MB

/** Extract a message from an unknown thrown value */
function errMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

/**
 * Read exactly n bytes from socket.
 *
 * 挂起的读必须可被连接关闭/错误唤醒（reject），否则 close() 后读协程
 * 永久悬挂；resolve/reject 路径都要清掉挂着的 readable/error/close/end
 * 监听，防止每帧泄漏 2+ 个 listener（MaxListenersExceededWarning）。
 */
function readExact(socket: Socket, n: number): Promise<Buffer> {
  return new Promise((resolve, reject) => {
    const data = Buffer.allocUnsafe(n);
    let offset = 0;
    let settled = false;

    const cleanup = () => {
      socket.removeListener("readable", readMore);
      socket.removeListener("error", onError);
      socket.removeListener("close", onClosed);
      socket.removeListener("end", onClosed);
    };

    const finish = (err: Error | null) => {
      if (settled) {
        return;
      }
      settled = true;
      cleanup();
      if (err) {
        reject(err);
      } else {
        resolve(data);
      }
    };

    const onError = (err: Error) => finish(err);
    const onClosed = () => finish(new Error("connection closed"));

    const readMore = () => {
      if (settled) {
        return;
      }
      const chunk = socket.read(n - offset);
      if (chunk === null) {
        socket.once("readable", readMore);
        return;
      }
      chunk.copy(data, offset);
      offset += chunk.length;
      if (offset >= n) {
        finish(null);
      } else {
        readMore();
      }
    };

    socket.once("error", onError);
    socket.once("close", onClosed);
    socket.once("end", onClosed);
    readMore();
  });
}

/** Write a length-prefixed frame to socket */
function writeFrame(socket: Socket, payload: Buffer): void {
  const header = Buffer.allocUnsafe(FRAME_HEADER_BYTES);
  header.writeUInt32BE(payload.length, 0);
  socket.write(Buffer.concat([header, payload]));
}

/** Read a length-prefixed frame from socket */
async function readFrame(socket: Socket): Promise<Buffer> {
  const header = await readExact(socket, FRAME_HEADER_BYTES);
  const size = header.readUInt32BE(0);
  if (size === 0) {
    return Buffer.allocUnsafe(0);
  }
  if (size > MAX_FRAME_BYTES) {
    throw new Error(`frame too large: ${size} > ${MAX_FRAME_BYTES}`);
  }
  return readExact(socket, size);
}

/** Pending request tracker */
interface PendingCall {
  resolve: () => void;
  respMsgId: number;
  respBody: Buffer;
  error: Error | null;
}

/** TCP Transport configuration */
export interface TCPTransportConfig {
  address?: string;
  timeoutMs?: number;
  connectTimeoutMs?: number; // Connection timeout (separate from request timeout)
  // 入站处理并发（固定 worker 池，防读循环头部阻塞——一个慢 handler
  // 卡住整条连接的所有请求）。0/未设 = os.cpus().length；队列容量
  // = workers * 4，满时立即回 busy 错误响应（Agent failover 接管）。
  inboundWorkers?: number;
  tlsEnabled?: boolean;
  tlsCertFile?: string;
  tlsKeyFile?: string;
  tlsCaFile?: string;
  tlsServerName?: string;
  tlsInsecureSkipVerify?: boolean;
}

/** Inbound request handler */
export type RequestHandler = (
  msgId: number,
  reqId: number,
  body: Buffer,
) => Promise<Buffer> | Buffer;

/**
 * Bidirectional multiplexed TCP transport.
 *
 * A single TCP connection supports:
 * - **Outbound requests** via `call()` (send request, wait for response)
 * - **Inbound requests** via a handler callback (agent pushes invoke/task)
 */
export class TCPTransport {
  private address: string;
  private timeoutMs: number;
  private connectTimeoutMs: number;
  private tlsEnabled: boolean;
  private tlsCertFile: string;
  private tlsKeyFile: string;
  private tlsCaFile: string;
  private tlsServerName: string;
  private tlsInsecureSkipVerify: boolean;

  private socket: Socket | null = null;
  private connected = false;
  private requestId = 0;
  private writeLock = Promise.resolve();

  // 连接代次（AUD-060）：nextEpoch 是分配器（connect 取号、close 前移），
  // epoch 是当前生效连接的代次。在途拨号、读循环、pending、入站队列都绑定
  // 自己的代次——close 或重连后旧代次的尾巴（迟到会话/读循环/队列残留）
  // 只影响所属连接，不再污染新连接。
  private epoch = 0;
  private nextEpoch = 0;

  // pending request_id -> PendingCall
  private pending: Map<number, PendingCall> = new Map();

  private running = false;

  // inbound request handler
  private handler: RequestHandler | null = null;
  // 入站 worker 池：读循环只投递，handler 由固定并发消费。
  private inboundQueue: Array<{ msgId: number; reqId: number; body: Buffer }> =
    [];
  private inboundWorkersRunning = 0;
  private inboundWorkerLimit = 0;
  // 控制车道：心跳/注册/drain 独立队列 + 单并发（双车道，对齐 Go MuxConn）
  private controlQueue: Array<{ msgId: number; reqId: number; body: Buffer }> =
    [];
  private controlWorkerRunning = false;

  constructor(config: TCPTransportConfig = {}) {
    this.inboundWorkerLimit =
      config.inboundWorkers && config.inboundWorkers > 0
        ? config.inboundWorkers
        : Math.max(2, os.cpus().length);
    this.address = config.address ?? "127.0.0.1:19091";
    this.timeoutMs = config.timeoutMs ?? 30000;
    this.connectTimeoutMs = config.connectTimeoutMs ?? 5000;
    this.tlsEnabled = config.tlsEnabled ?? false;
    this.tlsCertFile = config.tlsCertFile ?? "";
    this.tlsKeyFile = config.tlsKeyFile ?? "";
    this.tlsCaFile = config.tlsCaFile ?? "";
    this.tlsServerName = config.tlsServerName ?? "";
    this.tlsInsecureSkipVerify = config.tlsInsecureSkipVerify ?? false;
  }

  /** Set handler for inbound requests from the remote peer */
  setHandler(handler: RequestHandler): void {
    this.handler = handler;
  }

  /** Set the connection timeout (separate from request timeout) */
  setConnectTimeout(timeoutMs: number): void {
    this.connectTimeoutMs = timeoutMs;
  }

  /** Connect to the remote endpoint with timeout */
  async connect(): Promise<void> {
    if (this.connected) {
      return;
    }

    const addr = this.stripScheme(this.address);
    const [host, portStr] = addr.split(":");
    const port = parseInt(portStr, 10);

    // 本次拨号自成一代：并发/先后拨号各自取号，close 或更新拨号会使
    // 旧代次过期（AUD-060）。
    const gen = ++this.nextEpoch;

    // Create a promise that resolves when connection is established
    let socket: Socket | null = null;
    let timeout: NodeJS.Timeout | null = null;
    const cleanup = () => {
      if (timeout) {
        clearTimeout(timeout);
        timeout = null;
      }
      socket?.removeListener(
        this.tlsEnabled ? "secureConnect" : "connect",
        onConnect,
      );
      socket?.removeListener("error", onError);
    };
    const onConnect = () => {
      cleanup();
      // istanbul ignore next -- connect 事件触发时闭包 socket 必已赋值
      if (!socket) {
        return;
      }
      // TLS 模式等的是 secureConnect（握手完成后才允许会话建立）。
      // 兜底核验：rejectUnauthorized=false 放行时 authorized 可为 false，
      // 显式跳过校验才允许建立会话（AUD-074）。
      if (socket instanceof TLSSocket && !this.tlsInsecureSkipVerify) {
        if (!socket.authorized) {
          socket.destroy();
          rejectConnect?.(
            new Error(
              `Failed to connect to ${this.address}: TLS certificate validation failed: ${socket.authorizationError}`,
            ),
          );
          return;
        }
      }
      if (gen !== this.nextEpoch) {
        // 代次过期：拨号在途期间发生过 close() 或更新拨号——迟到的会话
        // 直接废弃，不建立、不接管当前连接（AUD-060）。挂起的 connect()
        // 立即报错，不等拨号超时。
        socket.destroy();
        rejectConnect?.(new Error("connection closed"));
        return;
      }
      this.epoch = gen;
      this.socket = socket;
      this.connected = true;
      this.running = true;
      this.startReader();
      resolveConnect?.();
    };
    const onError = (err: Error) => {
      cleanup();
      socket?.destroy();
      // TLS 握手/证书校验失败必须有明确错误（AUD-074）：阻断会话建立。
      const reason = this.tlsEnabled
        ? `TLS handshake failed: ${err.message}`
        : err.message;
      rejectConnect?.(
        new Error(`Failed to connect to ${this.address}: ${reason}`),
      );
    };
    let resolveConnect: (() => void) | null = null;
    let rejectConnect: ((reason?: Error) => void) | null = null;
    // 拨号超时先建：同步拨号失败（如证书文件读不到）也能清掉计时器，
    // 不给事件循环留 5s 悬挂 handle。
    const timeoutPromise = new Promise<void>((_, reject) => {
      timeout = setTimeout(() => {
        cleanup();
        socket?.destroy();
        reject(
          new Error(
            `Connection timeout to ${this.address} after ${this.connectTimeoutMs}ms`,
          ),
        );
      }, this.connectTimeoutMs);
    });

    const connectionPromise = new Promise<void>((resolve, reject) => {
      resolveConnect = resolve;
      rejectConnect = reject;
      try {
        socket = this.dialSocket(host, port);
      } catch (err) {
        cleanup();
        rejectConnect?.(
          new Error(
            `Failed to connect to ${this.address}: ${errMessage(err)}`,
          ),
        );
        return;
      }
      // TLS：等 secureConnect（握手完成）；明文：等 connect。
      socket.once(this.tlsEnabled ? "secureConnect" : "connect", onConnect);
      socket.once("error", onError);
      socket.setTimeout(this.timeoutMs);
    });

    await Promise.race([connectionPromise, timeoutPromise]);
  }

  /**
   * 按配置拨号（AUD-074）：tlsEnabled=true 走真 TLS 拨号（此前 TLS 选项
   * 只被存下、拨号恒普通 TCP），否则普通 TCP。
   * 语义对齐 Go SDK transport（sdks/go/pkg/croupier/transport/config.go
   * createTLSConfig）：CA / 客户端证书+私钥从文件读入；serverName 显式
   * 优先，缺省时非 IP 主机名默认 SNI=host，IP 直连不发 SNI；证书校验
   * 失败在握手阶段报错，阻断会话建立。
   */
  private dialSocket(host: string, port: number): Socket {
    if (!this.tlsEnabled) {
      return createConnection({ host, port, family: 4 } as TcpSocketConnectOpts);
    }

    // family 走 net.connect 透传（tls.ConnectionOptions 类型未列，运行时支持）。
    const opts: ConnectionOptions & { family?: number } = {
      host,
      port,
      family: 4,
      rejectUnauthorized: !this.tlsInsecureSkipVerify,
    };
    const servername = this.tlsServerName || (isIP(host) ? "" : host);
    if (servername) {
      opts.servername = servername;
    }
    if (this.tlsCaFile) {
      try {
        opts.ca = readFileSync(this.tlsCaFile);
      } catch (err) {
        throw new Error(
          `failed to read TLS CA file "${this.tlsCaFile}": ${errMessage(err)}`,
        );
      }
    }
    if (this.tlsCertFile && this.tlsKeyFile) {
      try {
        opts.cert = readFileSync(this.tlsCertFile);
        opts.key = readFileSync(this.tlsKeyFile);
      } catch (err) {
        throw new Error(
          `failed to read TLS client certificate/key "${this.tlsCertFile}", "${this.tlsKeyFile}": ${errMessage(err)}`,
        );
      }
    }
    return tlsConnect(opts);
  }

  /** Close the connection and release resources */
  close(): void {
    // 前移代次必须无条件执行：即使"未连接"，也可能有在途拨号——旧代码
    // 在 !connected 时提前返回，close 后迟到会话照样建立（AUD-060）。
    // 幂等：重复 close 只是无害空转。
    this.nextEpoch += 1;
    this.epoch = 0;
    this.running = false;
    this.connected = false;

    if (this.socket) {
      try {
        // destroy 可能异步触发 error 事件；读循环的 error 监听已随读退出
        // 清理，挂一个空监听避免 uncaught exception。
        this.socket.once("error", () => {});
        this.socket.destroy();
      } catch {
        // ignore
      }
      this.socket = null;
    }

    // 入站/控制队列只属于已死连接：清空，旧连接的消息不跨代处理（AUD-060）。
    this.inboundQueue.length = 0;
    this.controlQueue.length = 0;

    // fail all pending calls
    for (const p of this.pending.values()) {
      p.error = new Error("connection closed");
      p.resolve();
    }
    this.pending.clear();
  }

  /** Check if connected */
  isConnected(): boolean {
    return this.connected;
  }

  /**
   * Send a request and block until the matching response arrives.
   * Returns (response_msg_type, response_body)
   */
  async call(msgType: number, data: Buffer): Promise<[number, Buffer]> {
    const sock = this.socket;
    if (!this.connected || !sock) {
      throw new Error("Not connected");
    }

    this.requestId = (this.requestId + 1) & 0xffffffff;
    const reqId = this.requestId;

    // istanbul ignore next -- 占位 resolve 恒被 donePromise executor 覆盖
    const placeholderResolve: () => void = () => {};
    const pending: PendingCall = {
      resolve: placeholderResolve,
      respMsgId: 0,
      respBody: Buffer.allocUnsafe(0),
      error: null,
    };
    const donePromise = new Promise<void>((resolve) => {
      pending.resolve = resolve;
    });
    this.pending.set(reqId, pending);

    try {
      const message = newMessage(msgType, reqId, data);
      await this.writeLock;
      // 写前核验：close/重连后旧代次调用不再向新连接写帧（AUD-060）。
      if (!this.connected || this.socket !== sock || sock.destroyed) {
        throw new Error("connection closed");
      }
      this.writeLock = (async () => {
        writeFrame(sock, message);
      })();

      // wait for response
      let timeout: NodeJS.Timeout | null = null;
      const timeoutPromise = new Promise<void>((_, reject) => {
        timeout = setTimeout(
          () => reject(new Error("timeout")),
          this.timeoutMs,
        );
      });

      try {
        await Promise.race([donePromise, timeoutPromise]);
      } finally {
        if (timeout) {
          clearTimeout(timeout);
        }
      }

      if (pending.error) {
        throw pending.error;
      }

      return [pending.respMsgId, pending.respBody];
    } finally {
      this.pending.delete(reqId);
    }
  }

  /** Send a response frame (used by inbound-request handler) */
  sendResponse(msgType: number, reqId: number, data: Buffer): void {
    if (!this.connected || !this.socket) {
      return;
    }
    const message = newMessage(msgType, reqId, data);
    writeFrame(this.socket, message);
  }

  /** Start background reader loop */
  private startReader(): void {
    // 读循环绑定自己的连接代次与插座（AUD-060）：只读捕获的 sock，退出
    // 条件含代次核验——close 后重连时旧读循环即使还挂在本代读上，也不会
    // 接管新连接的 socket 或误清新连接的 pending（多读者竞争 socket.read
    // 会把帧解析彻底打乱）。读循环直接阻塞在 readFrame 上（单读者），空闲
    // 连接不产生任何并发读——此前 readFrameWithTimeout 用 1s 空转超时退出
    // 等待，但挂起的 readFrame→readExact 不可取消，空闲每秒多挂一个并发
    // 读；数据帧到达时多个 readMore 竞争 socket.read(n)，一个读走 4 字节
    // 长度前缀、其余把 payload 前 4 字节当长度前缀，帧解析彻底错乱（探针/
    // 心跳响应丢失、agent keepalive 判死会话的根因）。
    const gen = this.epoch;
    // startReader 仅由 onConnect 在 this.socket 赋值后调用（既有测试经
    // 内部句柄直呼亦同代次在位），此处非空断言成立。
    const sock = this.socket as Socket;
    const readLoop = async () => {
      try {
        while (this.running && this.socket === sock && this.epoch === gen) {
          const frame = await readFrame(sock);
          if (frame.length < 8) {
            continue;
          }

          const [version, msgId, reqId, body] = parseMessage(frame);

          if (isResponse(msgId)) {
            const pending = this.pending.get(reqId);
            if (pending) {
              pending.respMsgId = msgId;
              pending.respBody = body;
              pending.resolve();
            }
          } else if (isRequest(msgId)) {
            this.dispatchInbound(msgId, reqId, body);
          }
        }
      } catch {
        // 读失败（对端关闭/网络错误/超限帧）：退出读循环，交给 finally 善后。
      } finally {
        // 非主动 close 的退出（对端断开/读异常）：本代连接已不可用。置断开
        // 并唤醒所有挂起调用，让上层心跳失败触发重连——否则 pending
        // 只能等各自的请求超时才发现连接已死。代次已前移（close/重连接管）
        // 时不善后：pending 与状态归当前代次所有（AUD-060）。
        if (this.epoch === gen && this.running) {
          this.connected = false;
          this.running = false;
          for (const p of this.pending.values()) {
            p.error = new Error("connection closed");
            p.resolve();
          }
          this.pending.clear();
        }
      }
    };

    readLoop().catch(() => {});
  }

  /** 读循环只投递：双车道派发（对齐 Go MuxConn）。

   * - 控制消息（心跳/注册/drain）→ 独立车道，永不拒绝
   * - 业务消息 → 固定并发消费 handler，队列满立即回 busy（failover）
   */
  private dispatchInbound(msgId: number, reqId: number, body: Buffer): void {
    if (isControlRequest(msgId)) {
      this.controlQueue.push({ msgId, reqId, body });
      if (!this.controlWorkerRunning) {
        this.controlWorkerRunning = true;
        void this.controlWorkerLoop();
      }
      return;
    }
    const capacity = this.inboundWorkerLimit * 4;
    if (this.inboundQueue.length >= capacity) {
      // 队列满：立即回空/错误响应，Agent 侧 failover 接管。
      this.sendResponse(getResponseMsgId(msgId), reqId, Buffer.alloc(0));
      return;
    }
    this.inboundQueue.push({ msgId, reqId, body });
    if (this.inboundWorkersRunning < this.inboundWorkerLimit) {
      this.inboundWorkersRunning += 1;
      void this.inboundWorkerLoop();
    }
  }

  private async controlWorkerLoop(): Promise<void> {
    for (;;) {
      const task = this.controlQueue.shift();
      if (!task) {
        this.controlWorkerRunning = false;
        return;
      }
      await this.handleInbound(task.msgId, task.reqId, task.body);
    }
  }

  private async inboundWorkerLoop(): Promise<void> {
    for (;;) {
      const task = this.inboundQueue.shift();
      if (!task) {
        this.inboundWorkersRunning -= 1;
        return;
      }
      await this.handleInbound(task.msgId, task.reqId, task.body);
    }
  }

  private async handleInbound(
    msgId: number,
    reqId: number,
    body: Buffer,
  ): Promise<void> {
    if (!this.handler) {
      return;
    }
    try {
      const respBody = await this.handler(msgId, reqId, body);
      const respMsgId = getResponseMsgId(msgId);
      this.sendResponse(respMsgId, reqId, respBody);
    } catch {
      // error, ignore
    }
  }

  private stripScheme(address: string): string {
    if (address.includes("://")) {
      return address.split("://")[1];
    }
    return address;
  }
}

/** Protocol constants */
const VERSION = 0x01;
const HEADER_SIZE = 8;

/** Create a new message: [version(1) | msg_id(3) | req_id(4)][body] */
function newMessage(msgId: number, reqId: number, body: Buffer): Buffer {
  const header = Buffer.allocUnsafe(HEADER_SIZE);
  header.writeUInt8(VERSION, 0);
  header.writeUIntBE(msgId, 1, 3);
  header.writeUInt32BE(reqId, 4);
  return Buffer.concat([header, body]);
}

/** Parse message: [version(1) | msg_id(3) | req_id(4)][body] */
function parseMessage(frame: Buffer): [number, number, number, Buffer] {
  const version = frame.readUInt8(0);
  const msgId = frame.readUIntBE(1, 3);
  const reqId = frame.readUInt32BE(4);
  const body = frame.subarray(HEADER_SIZE);
  return [version, msgId, reqId, body];
}

/** Check if message ID is a response (even number) */
function isResponse(msgId: number): boolean {
  return (msgId & 0x01) === 0;
}

/** Check if message ID is a request (odd number) */
function isRequest(msgId: number): boolean {
  return (msgId & 0x01) === 0x01;
}
