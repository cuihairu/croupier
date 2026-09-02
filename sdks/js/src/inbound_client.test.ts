/**
 * Client-level inbound dispatch tests: agent pushes InvokeRequest /
 * StartTaskRequest / CancelTaskRequest over a real TCP connection and the
 * BasicClient must route them to registered handlers and reply on the wire.
 *
 * Regression coverage for: first-connect wiring previously never attached an
 * inbound handler, so all agent pushes were silently dropped by the transport.
 */

import { createServer, Server, Socket } from "net";
import * as protobuf from "protobufjs";
import { createClient, CroupierClient } from "./index";
import {
  MSG_INVOKE_REQUEST,
  MSG_INVOKE_RESPONSE,
  MSG_START_TASK_REQUEST,
  MSG_START_TASK_RESPONSE,
  MSG_CANCEL_TASK_REQUEST,
  MSG_CANCEL_TASK_RESPONSE,
  MSG_PROVIDER_CONNECT_REQUEST,
  MSG_PROVIDER_CONNECT_RESPONSE,
} from "./protocol";

const VERSION = 0x01;

const TEST_PROTO = `
syntax = "proto3";
package croupier.sdk.v1;

message ProviderConnectResponse {
  string session_id = 1;
}

message InvokeRequest {
  string function_id = 1;
  string idempotencyKey = 2;
  bytes payload = 3;
  map<string, string> metadata = 4;
}

message InvokeResponse {
  bytes payload = 1;
}

message StartTaskResponse {
  string task_id = 1;
}

message CancelTaskRequest {
  string task_id = 1;
}
`;
const testRoot = protobuf.parse(TEST_PROTO).root;
const ProviderConnectResponseMessage = testRoot.lookupType(
  "croupier.sdk.v1.ProviderConnectResponse",
);
const InvokeRequestMessage = testRoot.lookupType(
  "croupier.sdk.v1.InvokeRequest",
);
const InvokeResponseMessage = testRoot.lookupType(
  "croupier.sdk.v1.InvokeResponse",
);
const StartTaskResponseMessage = testRoot.lookupType(
  "croupier.sdk.v1.StartTaskResponse",
);
const CancelTaskRequestMessage = testRoot.lookupType(
  "croupier.sdk.v1.CancelTaskRequest",
);

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

class FakeAgent {
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
        this.dispatch(decoded);
      }
    }
  }

  private dispatch(f: DecodedFrame): void {
    // Auto-answer the provider connect handshake.
    if (f.msgId === MSG_PROVIDER_CONNECT_REQUEST) {
      const body = Buffer.from(
        ProviderConnectResponseMessage.encode(
          ProviderConnectResponseMessage.create({ sessionId: "sess-1" }),
        ).finish(),
      );
      this.send(MSG_PROVIDER_CONNECT_RESPONSE, f.reqId, body);
    }
    const idx = this.waiters.findIndex((w) => w.predicate(f));
    if (idx >= 0) {
      const [w] = this.waiters.splice(idx, 1);
      w.resolve(f);
    } else {
      this.frames.push(f);
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
    for (const s of this.sockets) {
      s.write(raw);
    }
  }

  async close(): Promise<void> {
    for (const s of this.sockets) {
      s.destroy();
    }
    await new Promise<void>((resolve) => {
      this.server.close(() => resolve());
    });
  }
}

function encodeInvokeRequest(fields: {
  functionId: string;
  payload: string;
  idempotencyKey?: string;
  metadata?: Record<string, string>;
}): Buffer {
  return Buffer.from(
    InvokeRequestMessage.encode(
      InvokeRequestMessage.create({
        functionId: fields.functionId,
        idempotencyKey: fields.idempotencyKey ?? "",
        payload: new TextEncoder().encode(fields.payload),
        metadata: fields.metadata ?? {},
      }),
    ).finish(),
  );
}

function decodeInvokeResponse(body: Buffer): string {
  const decoded = InvokeResponseMessage.toObject(
    InvokeResponseMessage.decode(body),
    { defaults: true },
  ) as { payload?: Uint8Array };
  return new TextDecoder().decode(decoded.payload ?? new Uint8Array());
}

describe("BasicClient inbound dispatch", () => {
  let agent: FakeAgent;
  let client: CroupierClient;

  beforeEach(async () => {
    agent = new FakeAgent();
    const address = await agent.start();
    client = createClient({
      agentAddr: `tcp://${address}`,
      insecure: true,
      disableLogging: true,
      heartbeatIntervalSeconds: 3600,
    });
  });

  afterEach(async () => {
    await client.disconnect();
    await agent.close();
  });

  it("routes agent-pushed invoke to the registered handler and replies", async () => {
    const seen: { context?: string; payload?: string } = {};
    client.registerFunction(
      { id: "echo", version: "1.0.0" },
      (context: string, payload: string) => {
        seen.context = context;
        seen.payload = payload;
        return "echo:" + payload;
      },
    );
    await client.connect();

    agent.send(
      MSG_INVOKE_REQUEST,
      42,
      encodeInvokeRequest({
        functionId: "echo",
        payload: "hello",
        idempotencyKey: "idem-1",
        metadata: { k: "v" },
      }),
    );

    const resp = await agent.waitFrame((f) => f.msgId === MSG_INVOKE_RESPONSE);
    expect(resp.reqId).toBe(42);
    expect(decodeInvokeResponse(resp.body)).toBe("echo:hello");
    expect(seen.payload).toBe("hello");
    expect(JSON.parse(seen.context ?? "{}")).toMatchObject({
      k: "v",
      idempotencyKey: "idem-1",
    });
  });

  it("returns an empty payload for unknown functions", async () => {
    client.registerFunction({ id: "echo", version: "1.0.0" }, () => "ok");
    await client.connect();

    agent.send(
      MSG_INVOKE_REQUEST,
      43,
      encodeInvokeRequest({ functionId: "missing", payload: "x" }),
    );

    const resp = await agent.waitFrame((f) => f.msgId === MSG_INVOKE_RESPONSE);
    expect(resp.reqId).toBe(43);
    expect(decodeInvokeResponse(resp.body)).toBe("");
  });

  it("returns an error payload when the handler throws", async () => {
    client.registerFunction({ id: "boom", version: "1.0.0" }, () => {
      throw new Error("kaboom");
    });
    await client.connect();

    agent.send(
      MSG_INVOKE_REQUEST,
      44,
      encodeInvokeRequest({ functionId: "boom", payload: "x" }),
    );

    const resp = await agent.waitFrame((f) => f.msgId === MSG_INVOKE_RESPONSE);
    expect(resp.reqId).toBe(44);
    expect(JSON.parse(decodeInvokeResponse(resp.body))).toEqual({
      error: "kaboom",
    });
  });

  it("answers StartTaskRequest with a task id", async () => {
    client.registerFunction({ id: "long", version: "1.0.0" }, async () => {
      await new Promise((r) => setTimeout(r, 50));
      return "done";
    });
    await client.connect();

    agent.send(
      MSG_START_TASK_REQUEST,
      45,
      encodeInvokeRequest({ functionId: "long", payload: "x" }),
    );

    const resp = await agent.waitFrame(
      (f) => f.msgId === MSG_START_TASK_RESPONSE,
    );
    expect(resp.reqId).toBe(45);
    const decoded = StartTaskResponseMessage.toObject(
      StartTaskResponseMessage.decode(resp.body),
      { defaults: true },
    ) as { taskId?: string };
    expect(decoded.taskId).toMatch(/^long-/);
  });

  it("answers StartTaskRequest with empty task id for unknown functions", async () => {
    client.registerFunction({ id: "echo", version: "1.0.0" }, () => "ok");
    await client.connect();

    agent.send(
      MSG_START_TASK_REQUEST,
      46,
      encodeInvokeRequest({ functionId: "missing", payload: "x" }),
    );

    const resp = await agent.waitFrame(
      (f) => f.msgId === MSG_START_TASK_RESPONSE,
    );
    expect(resp.reqId).toBe(46);
    const decoded = StartTaskResponseMessage.toObject(
      StartTaskResponseMessage.decode(resp.body),
      { defaults: true },
    ) as { taskId?: string };
    expect(decoded.taskId ?? "").toBe("");
  });

  it("answers CancelTaskRequest and cancels the local task", async () => {
    let resolveHandler: (v: string) => void = () => undefined;
    client.registerFunction(
      { id: "long", version: "1.0.0" },
      () =>
        new Promise<string>((r) => {
          resolveHandler = r;
        }),
    );
    await client.connect();

    agent.send(
      MSG_START_TASK_REQUEST,
      47,
      encodeInvokeRequest({ functionId: "long", payload: "x" }),
    );
    const startResp = await agent.waitFrame(
      (f) => f.msgId === MSG_START_TASK_RESPONSE,
    );
    const { taskId } = StartTaskResponseMessage.toObject(
      StartTaskResponseMessage.decode(startResp.body),
      { defaults: true },
    ) as { taskId?: string };
    expect(taskId).toBeTruthy();

    agent.send(
      MSG_CANCEL_TASK_REQUEST,
      48,
      Buffer.from(
        CancelTaskRequestMessage.encode(
          CancelTaskRequestMessage.create({ taskId }),
        ).finish(),
      ),
    );
    const cancelResp = await agent.waitFrame(
      (f) => f.msgId === MSG_CANCEL_TASK_RESPONSE,
    );
    expect(cancelResp.reqId).toBe(48);

    // The cancelled task must no longer be streamable.
    expect(() => client.streamTask(taskId ?? "")).toThrow(/not found/);
    resolveHandler("late");
  });

  it("handles 16 concurrent inbound invokes without dropping", async () => {
    let count = 0;
    client.registerFunction({ id: "echo", version: "1.0.0" }, (_c, p) => {
      count += 1;
      return "echo:" + p;
    });
    await client.connect();

    for (let i = 0; i < 16; i++) {
      agent.send(
        MSG_INVOKE_REQUEST,
        100 + i,
        encodeInvokeRequest({ functionId: "echo", payload: `m${i}` }),
      );
    }

    const replies = await Promise.all(
      Array.from({ length: 16 }, (_, i) =>
        agent.waitFrame(
          (f) => f.msgId === MSG_INVOKE_RESPONSE && f.reqId === 100 + i,
        ),
      ),
    );
    expect(replies).toHaveLength(16);
    expect(count).toBe(16);
    for (const r of replies) {
      expect(decodeInvokeResponse(r.body)).toMatch(/^echo:m\d+$/);
    }
  });
});
