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

import {
  Socket,
  createConnection,
  TcpSocketConnectOpts,
} from "net";

/** Frame constants */
const FRAME_HEADER_BYTES = 4; // 4-byte big-endian length prefix
const MAX_FRAME_BYTES = 32 * 1024 * 1024; // 32 MB

/** Read exactly n bytes from socket */
function readExact(socket: Socket, n: number): Promise<Buffer> {
  return new Promise((resolve, reject) => {
    const data = Buffer.allocUnsafe(n);
    let offset = 0;

    const readMore = () => {
      const chunk = socket.read(n - offset);
      if (chunk === null) {
        socket.once("readable", readMore);
        return;
      }
      chunk.copy(data, offset);
      offset += chunk.length;
      if (offset >= n) {
        resolve(data);
      } else {
        readMore();
      }
    };

    socket.once("error", reject);
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
  connectTimeoutMs?: number;  // Connection timeout (separate from request timeout)
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

  // pending request_id -> PendingCall
  private pending: Map<number, PendingCall> = new Map();

  private readerTimer: NodeJS.Timeout | null = null;
  private running = false;

  // inbound request handler
  private handler: RequestHandler | null = null;

  constructor(config: TCPTransportConfig = {}) {
    this.address = config.address ?? "127.0.0.1:19090";
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

    // Create a promise that resolves when connection is established
    const connectionPromise = new Promise<void>((resolve, reject) => {
      const socket = createConnection(
        { host, port, family: 4 } as TcpSocketConnectOpts,
        () => {
          this.socket = socket;
          this.connected = true;
          this.running = true;
          this.startReader();
          resolve();
        },
      );

      // Handle connection errors
      socket.once("error", (err: Error) => {
        reject(new Error(`Failed to connect to ${this.address}: ${err.message}`));
      });

      // Set read timeout (not connection timeout)
      socket.setTimeout(this.timeoutMs);
    });

    // Create a timeout promise
    const timeoutPromise = new Promise<never>((_, reject) => {
      setTimeout(() => {
        reject(new Error(`Connection timeout to ${this.address} after ${this.connectTimeoutMs}ms`));
      }, this.connectTimeoutMs);
    });

    // Race between connection and timeout
    await Promise.race([connectionPromise, timeoutPromise]);
  }

  /** Close the connection and release resources */
  close(): void {
    if (!this.connected && !this.running) {
      return;
    }

    this.running = false;
    this.connected = false;

    if (this.readerTimer) {
      clearTimeout(this.readerTimer);
      this.readerTimer = null;
    }

    if (this.socket) {
      try {
        this.socket.destroy();
      } catch {
        // ignore
      }
      this.socket = null;
    }

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
    if (!this.connected || !this.socket) {
      throw new Error("Not connected");
    }

    this.requestId = (this.requestId + 1) & 0xffffffff;
    const reqId = this.requestId;

    const pending: PendingCall = {
      resolve: () => {},
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
      this.writeLock = (async () => {
        writeFrame(this.socket!, message);
      })();

      // wait for response
      const timeoutPromise = new Promise<void>((_, reject) => {
        setTimeout(() => reject(new Error("timeout")), this.timeoutMs);
      });

      await Promise.race([donePromise, timeoutPromise]);

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
    const readLoop = async () => {
      while (this.running && this.socket) {
        try {
          const frame = await this.readFrameWithTimeout();
          if (!frame || frame.length < 8) {
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
            await this.handleInbound(msgId, reqId, body);
          }
        } catch (err) {
          if (this.running) {
            // error, continue
          }
        }
      }
    };

    readLoop().catch(() => {});
    this.readerTimer = setInterval(() => {}, 1000);
  }

  private async readFrameWithTimeout(): Promise<Buffer> {
    return new Promise((resolve) => {
      if (!this.socket) {
        resolve(Buffer.allocUnsafe(0));
        return;
      }

      const timeout = setTimeout(() => {
        resolve(Buffer.allocUnsafe(0));
      }, 1000);

      setImmediate(async () => {
        try {
          const frame = await readFrame(this.socket!);
          clearTimeout(timeout);
          resolve(frame);
        } catch {
          clearTimeout(timeout);
          resolve(Buffer.allocUnsafe(0));
        }
      });
    });
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

/** Get response message ID from request message ID */
function getResponseMsgId(msgId: number): number {
  return msgId | 0x01;
}
