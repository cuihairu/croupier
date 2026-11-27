/**
 * AUD-074: TCPTransport TLS tests.
 *
 * Before AUD-074 the TLS options were stored but connect() always dialed
 * plain TCP (net.createConnection). These tests drive REAL TLS handshakes
 * against a local TLS server using pinned self-signed test certificates
 * (src/__fixtures__/tls/, validity 100 years):
 *   ca.pem / ca2.pem       — two independent self-signed CAs
 *   server(2).pem + keys   — server certs signed by ca.pem / ca2.pem
 *                            (SAN: DNS:localhost, IP:127.0.0.1)
 *   client.pem + key       — client cert signed by ca.pem (mTLS)
 */

import { createServer as createNetServer, Server, Socket } from "net";
import {
  createServer as createTLSServer,
  PeerCertificate,
  TlsOptions,
  TLSSocket,
} from "tls";
import { readFileSync } from "fs";
import { join } from "path";
import { TCPTransport, TCPTransportConfig } from "./tcp_transport";
import {
  MSG_INVOKE_REQUEST,
  MSG_INVOKE_RESPONSE,
} from "./protocol";

const VERSION = 0x01;
const FIXTURES = join(__dirname, "__fixtures__", "tls");
const CA_FILE = join(FIXTURES, "ca.pem");
const CA2_FILE = join(FIXTURES, "ca2.pem");
const SERVER_CERT = join(FIXTURES, "server.pem");
const SERVER_KEY = join(FIXTURES, "server-key.pem");
const SERVER2_CERT = join(FIXTURES, "server2.pem");
const SERVER2_KEY = join(FIXTURES, "server2-key.pem");
const CLIENT_CERT = join(FIXTURES, "client.pem");
const CLIENT_KEY = join(FIXTURES, "client-key.pem");

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

/** Wire-protocol frame feeder shared by the plain and TLS fake agents. */
abstract class FrameFeed {
  protected sockets: Array<Socket | TLSSocket> = [];
  protected frames: DecodedFrame[] = [];
  protected waiters: Array<(f: DecodedFrame) => void> = [];
  protected buffer = Buffer.alloc(0);
  address = "";

  protected feed(chunk: Buffer): void {
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

  protected listen(server: Server): Promise<void> {
    return new Promise((resolve) => {
      server.listen(0, "127.0.0.1", () => {
        const addr = server.address();
        if (addr && typeof addr === "object") {
          this.address = `127.0.0.1:${addr.port}`;
        }
        resolve();
      });
    });
  }

  nextFrame(timeoutMs = 3000): Promise<DecodedFrame> {
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

  reply(reqId: number, respMsgId: number, body: Buffer): void {
    for (const s of this.sockets) {
      s.write(frame(encodeMessage(respMsgId, reqId, body)));
    }
  }

  async close(): Promise<void> {
    for (const s of this.sockets) {
      s.destroy();
    }
    this.sockets = [];
  }
}

/** Plain TCP fake agent (explicit-plaintext path). */
class FakePlainAgent extends FrameFeed {
  private server: Server;

  constructor() {
    super();
    this.server = createNetServer((socket) => {
      this.sockets.push(socket);
      socket.on("data", (chunk: Buffer) => this.feed(chunk));
    });
  }

  start(): Promise<void> {
    return this.listen(this.server);
  }

  async close(): Promise<void> {
    await super.close();
    await new Promise<void>((resolve) => this.server.close(() => resolve()));
  }
}

interface TLSAgentOptions {
  certFile: string;
  keyFile: string;
  /** CA for verifying the CLIENT certificate (mTLS). */
  clientCaFile?: string;
  requestCert?: boolean;
}

/** TLS fake agent: real TLS handshake + same wire protocol. */
class FakeTLSAgent extends FrameFeed {
  private server: ReturnType<typeof createTLSServer>;
  private closed = false;
  private handshakeCount = 0;
  private handshakeWaiters: Array<() => void> = [];
  /** SNI advertised by the most recent client handshake. */
  servername: string | false | null = null;
  /** Client certificate seen by the most recent handshake (empty when none). */
  peerCert: PeerCertificate | Record<string, unknown> = {};

  constructor(opts: TLSAgentOptions) {
    super();
    const tlsOpts: TlsOptions = {
      cert: readFileSync(opts.certFile),
      key: readFileSync(opts.keyFile),
    };
    if (opts.requestCert) {
      tlsOpts.requestCert = true;
      tlsOpts.rejectUnauthorized = true;
      if (opts.clientCaFile) {
        tlsOpts.ca = [readFileSync(opts.clientCaFile)];
      }
    }
    this.server = createTLSServer(tlsOpts, (socket) => {
      this.sockets.push(socket);
      this.servername = socket.servername ?? null;
      this.peerCert =
        socket.getPeerCertificate(false) as PeerCertificate | Record<string, unknown>;
      socket.on("data", (chunk: Buffer) => this.feed(chunk));
      this.handshakeCount += 1;
      const waiters = this.handshakeWaiters;
      this.handshakeWaiters = [];
      for (const w of waiters) {
        w();
      }
    });
  }

  start(): Promise<void> {
    return this.listen(this.server);
  }

  /**
   * TLS 1.3 中客户端 secureConnect 早于服务端 secureConnection 触发：
   * 断言服务端捕获（SNI / 客户端证书）前必须等本端握手记录落袋。
   */
  waitHandshake(timeoutMs = 3000): Promise<void> {
    if (this.handshakeCount > 0) {
      return Promise.resolve();
    }
    return new Promise((resolve, reject) => {
      const timer = setTimeout(
        () => reject(new Error("timed out waiting for TLS handshake")),
        timeoutMs,
      );
      this.handshakeWaiters.push(() => {
        clearTimeout(timer);
        resolve();
      });
    });
  }

  async close(): Promise<void> {
    if (this.closed) {
      return;
    }
    this.closed = true;
    await super.close();
    await new Promise<void>((resolve) => this.server.close(() => resolve()));
  }
}

describe("TCPTransport TLS (AUD-074)", () => {
  let transports: TCPTransport[];
  let agents: Array<FakeTLSAgent | FakePlainAgent>;

  beforeEach(() => {
    transports = [];
    agents = [];
  });

  afterEach(async () => {
    for (const t of transports) {
      t.close();
    }
    for (const a of agents) {
      await a.close();
    }
  });

  function makeTransport(
    address: string,
    config: TCPTransportConfig = {},
  ): TCPTransport {
    const t = new TCPTransport({
      address,
      timeoutMs: 5000,
      connectTimeoutMs: 3000,
      ...config,
    });
    transports.push(t);
    return t;
  }

  async function startTLSAgent(opts: TLSAgentOptions): Promise<FakeTLSAgent> {
    const agent = new FakeTLSAgent(opts);
    agents.push(agent);
    await agent.start();
    return agent;
  }

  async function startPlainAgent(): Promise<FakePlainAgent> {
    const agent = new FakePlainAgent();
    agents.push(agent);
    await agent.start();
    return agent;
  }

  function internalSocket(t: TCPTransport): Socket | null {
    return (t as unknown as { socket: Socket | null }).socket;
  }

  it("dials TLS when enabled: real handshake, CA pinning, SNI, authorized socket, frame round-trip", async () => {
    const agent = await startTLSAgent({
      certFile: SERVER_CERT,
      keyFile: SERVER_KEY,
    });

    const t = makeTransport(agent.address, {
      tlsEnabled: true,
      tlsCaFile: CA_FILE,
      tlsServerName: "localhost",
    });
    await t.connect();

    expect(t.isConnected()).toBe(true);
    const sock = internalSocket(t);
    expect(sock).toBeInstanceOf(TLSSocket);
    // 校验真的发生了：证书被 CA 钉住并通过校验（真握手而非明文）。
    expect((sock as TLSSocket).authorized).toBe(true);
    // serverName 配置作为 SNI 发出（TLS 1.3 下服务端记录晚于客户端
    // secureConnect，先等服务端握手记录落袋再断言）。
    await agent.waitHandshake();
    expect(agent.servername).toBe("localhost");

    const pending = t.call(MSG_INVOKE_REQUEST, Buffer.from("ping-tls"));
    const req = await agent.nextFrame();
    expect(req.msgId).toBe(MSG_INVOKE_REQUEST);
    expect(req.body.toString()).toBe("ping-tls");
    agent.reply(req.reqId, MSG_INVOKE_RESPONSE, Buffer.from("pong-tls"));
    const [, resp] = await pending;
    expect(resp.toString()).toBe("pong-tls");
  });

  it("blocks establishment with a clear error when the CA does not validate the server cert", async () => {
    // server2 由 ca2 签发；客户端只钉 ca1 —— 校验必须失败并阻断。
    const agent = await startTLSAgent({
      certFile: SERVER2_CERT,
      keyFile: SERVER2_KEY,
    });

    const t = makeTransport(agent.address, {
      tlsEnabled: true,
      tlsCaFile: CA_FILE,
      tlsServerName: "localhost",
    });
    await expect(t.connect()).rejects.toThrow(
      /Failed to connect to .*TLS handshake failed:/,
    );
    expect(t.isConnected()).toBe(false);
    expect(internalSocket(t)).toBeNull();
  });

  it("blocks establishment with a clear error on server name mismatch", async () => {
    const agent = await startTLSAgent({
      certFile: SERVER_CERT,
      keyFile: SERVER_KEY,
    });

    const t = makeTransport(agent.address, {
      tlsEnabled: true,
      tlsCaFile: CA_FILE,
      tlsServerName: "wrong.example.com",
    });
    let message = "";
    await t.connect().catch((err: Error) => {
      message = err.message;
    });
    expect(message).toMatch(/Failed to connect to /);
    expect(message).toMatch(/TLS handshake failed:/);
    expect(t.isConnected()).toBe(false);
  });

  it("keeps plain TCP when plaintext is explicitly configured", async () => {
    const agent = await startPlainAgent();

    const t = makeTransport(agent.address, {
      tlsEnabled: false,
    });
    await t.connect();

    expect(t.isConnected()).toBe(true);
    const sock = internalSocket(t);
    expect(sock).not.toBeNull();
    expect(sock instanceof TLSSocket).toBe(false);

    const pending = t.call(MSG_INVOKE_REQUEST, Buffer.from("ping-plain"));
    const req = await agent.nextFrame();
    expect(req.body.toString()).toBe("ping-plain");
    agent.reply(req.reqId, MSG_INVOKE_RESPONSE, Buffer.from("pong-plain"));
    const [, resp] = await pending;
    expect(resp.toString()).toBe("pong-plain");
  });

  it("presents the configured client certificate for mTLS", async () => {
    const agent = await startTLSAgent({
      certFile: SERVER_CERT,
      keyFile: SERVER_KEY,
      requestCert: true,
      clientCaFile: CA_FILE,
    });

    const t = makeTransport(agent.address, {
      tlsEnabled: true,
      tlsCaFile: CA_FILE,
      tlsServerName: "localhost",
      tlsCertFile: CLIENT_CERT,
      tlsKeyFile: CLIENT_KEY,
    });
    await t.connect();

    expect(t.isConnected()).toBe(true);
    await agent.waitHandshake();
    const cert = agent.peerCert as PeerCertificate;
    expect(cert.subject?.CN).toBe("croupier-js-test-client");
  });

  it("rejects with a clear error when the CA file is unreadable", async () => {
    const t = makeTransport("127.0.0.1:19091", {
      connectTimeoutMs: 500,
      tlsEnabled: true,
      tlsCaFile: join(FIXTURES, "does-not-exist.pem"),
    });
    await expect(t.connect()).rejects.toThrow(/failed to read TLS CA file/);
    expect(t.isConnected()).toBe(false);
    expect(internalSocket(t)).toBeNull();
  });

  it("allows connecting with tlsInsecureSkipVerify against an untrusted cert", async () => {
    const agent = await startTLSAgent({
      certFile: SERVER2_CERT,
      keyFile: SERVER2_KEY,
    });

    const t = makeTransport(agent.address, {
      tlsEnabled: true,
      tlsServerName: "localhost",
      tlsInsecureSkipVerify: true,
    });
    await t.connect();

    expect(t.isConnected()).toBe(true);
    const sock = internalSocket(t) as TLSSocket;
    expect(sock).toBeInstanceOf(TLSSocket);
    expect(sock.authorized).toBe(false);
  });
});
