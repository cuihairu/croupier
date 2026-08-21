/**
 * Coverage tests for BasicClient edge paths: task state lifecycle, serve()
 * signal handling, remote invocation, upload validation, heartbeat guards,
 * reconnect loop and (de)serialization fallbacks.
 */

import * as protobuf from "protobufjs";
import { Readable } from "node:stream";
import { BasicClient } from "./index";
import { TCPTransport } from "./tcp_transport";
import { MSG_INVOKE_REQUEST, MSG_PROVIDER_CONNECT_REQUEST } from "./protocol";

const providerRoot = protobuf.parse(`
syntax = "proto3";
package croupier.sdk.v1;
message ProviderConnectResponse { string session_id = 1; }
`).root;
const ProviderConnectResponseMessage = providerRoot.lookupType(
  "croupier.sdk.v1.ProviderConnectResponse",
);

function makeClient(config = {}): BasicClient {
  const client = new BasicClient(config);
  client.registerFunction(
    { id: "test.fn", version: "1.0.0" },
    async () => "ok",
  );
  return client;
}

function encodeConnectResponse(sessionId: string): Buffer {
  const response = ProviderConnectResponseMessage.create({ sessionId });
  return Buffer.from(ProviderConnectResponseMessage.encode(response).finish());
}

afterEach(() => {
  jest.restoreAllMocks();
});

describe("TaskState edge cases", () => {
  it("resolves a waiting consumer with null when the task is cancelled", async () => {
    const client = new BasicClient();
    client.registerFunction(
      { id: "slow.fn", version: "1.0.0" },
      () => new Promise<string>(() => {}),
    );

    const taskId = client.startTask("slow.fn", "payload");
    const events: string[] = [];
    const consumer = (async () => {
      for await (const ev of client.streamTask(taskId)) {
        events.push(ev.type);
      }
    })();

    // Wait until the consumer has drained "started" and is parked.
    await new Promise((r) => setTimeout(r, 20));
    expect(client.cancelTask(taskId)).toBe(true);

    await consumer;
    expect(events).toEqual(["started", "cancelled"]);
  });

  it("closing an already-closed task state is a no-op", async () => {
    const client = new BasicClient();
    client.registerFunction(
      { id: "slow.fn", version: "1.0.0" },
      () => new Promise<string>(() => {}),
    );

    const taskId = client.startTask("slow.fn", "payload");
    const state = (client as any).taskStates.get(taskId);

    // Park a consumer so close() must resolve it with null.
    const events: string[] = [];
    const consumer = (async () => {
      for await (const ev of client.streamTask(taskId)) {
        events.push(ev.type);
      }
    })();
    await new Promise((r) => setTimeout(r, 20));

    state.close();
    state.close(); // second close is a no-op
    await consumer;

    expect(events).toEqual(["started"]);
    expect(state.closed).toBe(true);
  });
});

describe("serve() signal handling", () => {
  it("resolves on SIGINT and logs disconnect failures", async () => {
    const client = makeClient();
    const connectSpy = jest
      .spyOn(client, "connect")
      .mockImplementation(async () => {});
    const disconnectSpy = jest
      .spyOn(client, "disconnect")
      .mockRejectedValue(new Error("boom"));
    const logSpy = jest.spyOn(console, "log").mockImplementation(() => {});
    const errSpy = jest.spyOn(console, "error").mockImplementation(() => {});

    const servePromise = client.serve();
    await new Promise((r) => setTimeout(r, 20));

    process.emit("SIGINT");
    await servePromise;
    await new Promise((r) => setImmediate(r));

    expect(disconnectSpy).toHaveBeenCalled();
    expect(errSpy).toHaveBeenCalledWith(
      "Error during disconnect:",
      expect.objectContaining({ message: "boom" }),
    );

    logSpy.mockRestore();
    errSpy.mockRestore();
    disconnectSpy.mockRestore();
    connectSpy.mockRestore();

    // Flush the SIGTERM handler registered by this serve() call so later
    // tests can register their own.
    process.emit("SIGTERM");
    await new Promise((r) => setImmediate(r));
  });

  it("resolves on SIGTERM and disconnects cleanly", async () => {
    const client = makeClient();
    const connectSpy = jest
      .spyOn(client, "connect")
      .mockImplementation(async () => {
        (client as any).connected = true;
      });
    const disconnectSpy = jest
      .spyOn(client, "disconnect")
      .mockResolvedValue(undefined);
    const logSpy = jest.spyOn(console, "log").mockImplementation(() => {});

    const servePromise = client.serve();
    await new Promise((r) => setTimeout(r, 20));

    process.emit("SIGTERM");
    await servePromise;

    expect(disconnectSpy).toHaveBeenCalled();

    logSpy.mockRestore();
    disconnectSpy.mockRestore();
    connectSpy.mockRestore();

    // Flush the SIGINT handler registered by this serve() call.
    process.emit("SIGINT");
    await new Promise((r) => setImmediate(r));
  });
});

describe("remote invocation path", () => {
  it("sends the invoke request over the transport and returns the response body", async () => {
    const client = makeClient({
      authToken: "tok",
      gameId: "game-1",
      env: "prod",
    });

    const calls: Array<{ msgType: number; data: Buffer }> = [];
    (client as any).transport = {
      call: async (msgType: number, data: Buffer) => {
        calls.push({ msgType, data });
        return [msgType + 1, Buffer.from('"remote-result"')];
      },
    };
    (client as any).connected = true;

    const result = await client.invoke("test.fn", "the-arg", {
      idempotencyKey: "key-9",
      timeout: 1234,
    });

    expect(result).toBe('"remote-result"');
    expect(calls).toHaveLength(1);
    expect(calls[0].msgType).toBe(MSG_INVOKE_REQUEST);

    const request = JSON.parse(calls[0].data.toString());
    expect(request.function_id).toBe("test.fn");
    expect(request.idempotency_key).toBe("key-9");
    expect(request.timeout).toBe(1234);
    expect(request.metadata.Authorization).toBe("Bearer tok");
    expect(request.metadata["X-Game-ID"]).toBe("game-1");
    expect(request.metadata["X-Env"]).toBe("prod");
  });
});

describe("isInvokeOptions normalization", () => {
  it("treats null options as empty metadata", async () => {
    const client = new BasicClient();
    const handler = jest.fn().mockResolvedValue("done");
    client.registerFunction({ id: "f", version: "1.0.0" }, handler);

    const result = await client.invoke("f", "payload", null as any);
    expect(result).toBe("done");
    expect(handler).toHaveBeenCalledWith(
      expect.any(String),
      "payload",
    );
  });
});

describe("upload validation edge cases", () => {
  it("rejects a blank filePath", async () => {
    const client = new BasicClient({ enableFileTransfer: true });
    await expect(
      client.uploadFile({ filePath: "   ", content: "x" }),
    ).rejects.toThrow(/non-empty filePath/);
  });

  it("accepts Buffer content", async () => {
    const client = new BasicClient({ enableFileTransfer: true });
    const result = await client.uploadFile({
      filePath: "a/b/data.bin",
      content: Buffer.from("buffer-bytes"),
    });
    expect(result.size).toBe("buffer-bytes".length);
    expect(result.mimeType).toBe("application/octet-stream");
    expect(result.status).toBe("validated");
  });

  it("rejects empty upload content", async () => {
    const client = new BasicClient({ enableFileTransfer: true });
    await expect(
      client.uploadFile({ filePath: "empty.txt", content: "" }),
    ).rejects.toThrow(/cannot be empty/);
  });

  it("rejects inline content larger than maxFileSize", async () => {
    const client = new BasicClient({
      enableFileTransfer: true,
      maxFileSize: 4,
    });
    await expect(
      client.uploadFile({ filePath: "big.txt", content: "way-too-long" }),
    ).rejects.toThrow(/maxFileSize limit/);
  });

  it("rejects disallowed MIME types", async () => {
    const client = new BasicClient({
      enableFileTransfer: true,
      allowedMimeTypes: ["application/json"],
    });
    await expect(
      client.uploadFile({ filePath: "note.txt", content: "plain text" }),
    ).rejects.toThrow("MIME type text/plain is not allowed");
  });

  it("guesses MIME types for known extensions", async () => {
    const client = new BasicClient({ enableFileTransfer: true });
    const cases: Array<[string, string]> = [
      ["f.json", "application/json"],
      ["f.js", "text/javascript"],
      ["f.ts", "text/typescript"],
      ["f.txt", "text/plain"],
      ["f.md", "text/plain"],
      ["f.zip", "application/zip"],
      ["f.png", "image/png"],
      ["f.jpg", "image/jpeg"],
      ["f.jpeg", "image/jpeg"],
      ["f.unknown", "application/octet-stream"],
    ];
    for (const [filePath, expectedMime] of cases) {
      const result = await client.uploadFile({ filePath, content: "x" });
      expect(result.mimeType).toBe(expectedMime);
    }
  });

  it("treats an unparsable stream size hint as unknown total", async () => {
    const client = new BasicClient({
      enableFileTransfer: true,
      uploadTimeout: 100, // readStreamWithTimeout leaks its timer; keep it short
    });
    const percents: Array<number | undefined> = [];
    const result = await client.uploadFileStream({
      filePath: "blob.bin",
      stream: Readable.from(["chunk"]),
      metadata: { size: "not-a-number" },
      onProgress: (p) => percents.push(p.percent),
    });
    expect(result.size).toBe("chunk".length);
    expect(percents).toEqual([undefined, undefined]);
  });

  it("aborts a stream that exceeds maxFileSize mid-transfer", async () => {
    const client = new BasicClient({
      enableFileTransfer: true,
      maxFileSize: 4,
      uploadTimeout: 50, // readStreamWithTimeout leaks its timer; keep it short
    });
    const stream = new Readable({
      read() {
        this.push("aaaaaaaaaa"); // 10 bytes > 4 limit
      },
    });
    await expect(
      client.uploadFileStream({ filePath: "big.bin", stream }),
    ).rejects.toThrow(/maxFileSize limit/);
    expect(stream.destroyed).toBe(true);
  });

  it("times out a stream that never ends", async () => {
    const client = new BasicClient({
      enableFileTransfer: true,
      uploadTimeout: 50,
    });
    const stream = new Readable({ read() {} });
    stream.push("data"); // a single chunk, then silence forever
    await expect(
      client.uploadFileStream({ filePath: "stuck.bin", stream }),
    ).rejects.toThrow(/timed out/);
    stream.destroy();
  });
});

describe("connect and registration failures", () => {
  it("rejects when ProviderConnect returns an empty session id", async () => {
    const connectSpy = jest
      .spyOn(TCPTransport.prototype, "connect")
      .mockImplementation(async () => {});
    const callSpy = jest
      .spyOn(TCPTransport.prototype, "call")
      .mockImplementation(async (msgType: number) => [
        msgType + 1,
        encodeConnectResponse(""),
      ]);
    const closeSpy = jest
      .spyOn(TCPTransport.prototype, "close")
      .mockImplementation(() => {});

    const client = makeClient();
    await expect(client.connect()).rejects.toThrow(/empty session_id/);
    expect(closeSpy).toHaveBeenCalled();

    connectSpy.mockRestore();
    callSpy.mockRestore();
    closeSpy.mockRestore();
  });

  it("closes the previous transport when reconnecting", async () => {
    const connectSpy = jest
      .spyOn(TCPTransport.prototype, "connect")
      .mockImplementation(async () => {});
    const callSpy = jest
      .spyOn(TCPTransport.prototype, "call")
      .mockImplementation(async (msgType: number) => [
        msgType + 1,
        encodeConnectResponse("session-2"),
      ]);

    const client = makeClient();
    const oldTransport = { close: jest.fn() };
    (client as any).transport = oldTransport;

    await client.connect();

    expect(oldTransport.close).toHaveBeenCalledTimes(1);
    expect((client as any).sessionId).toBe("session-2");

    // connect() starts the heartbeat interval; stop it so jest can exit.
    await client.disconnect();

    connectSpy.mockRestore();
    callSpy.mockRestore();
  });

  it("closes the transport and rethrows when the connect call fails", async () => {
    const connectSpy = jest
      .spyOn(TCPTransport.prototype, "connect")
      .mockImplementation(async () => {});
    const callSpy = jest
      .spyOn(TCPTransport.prototype, "call")
      .mockImplementation(async (msgType: number) => {
        if (msgType === MSG_PROVIDER_CONNECT_REQUEST) {
          throw new Error("agent said no");
        }
        return [msgType + 1, Buffer.alloc(0)];
      });
    const closeSpy = jest
      .spyOn(TCPTransport.prototype, "close")
      .mockImplementation(() => {});

    const client = makeClient();
    await expect(client.connect()).rejects.toThrow("agent said no");
    expect(closeSpy).toHaveBeenCalled();
    expect((client as any).transport).toBeNull();

    connectSpy.mockRestore();
    callSpy.mockRestore();
    closeSpy.mockRestore();
  });
});

describe("heartbeat and reconnect internals", () => {
  it("sendHeartbeat rejects when the client is not registered", async () => {
    const client = makeClient();
    await expect((client as any).sendHeartbeat()).rejects.toThrow(
      "Client is not registered",
    );
  });

  it("scheduleReconnect is a no-op when stop was requested", async () => {
    const client = makeClient();
    (client as any).stopRequested = true;
    await expect((client as any).scheduleReconnect()).resolves.toBeUndefined();
    expect((client as any).reconnectPromise).toBeNull();
  });

  it("scheduleReconnect is a no-op when autoReconnect is disabled", async () => {
    const client = makeClient({ autoReconnect: false });
    await expect((client as any).scheduleReconnect()).resolves.toBeUndefined();
    expect((client as any).reconnectPromise).toBeNull();
  });

  it("scheduleReconnect is a no-op when reconnect is disabled in config", async () => {
    const client = makeClient({ reconnect: { enabled: false } });
    await expect((client as any).scheduleReconnect()).resolves.toBeUndefined();
    expect((client as any).reconnectPromise).toBeNull();
  });

  it("scheduleReconnect reuses the in-flight reconnect promise", async () => {
    const client = makeClient();
    const existing = Promise.resolve();
    (client as any).reconnectPromise = existing;
    await (client as any).scheduleReconnect();
    expect((client as any).reconnectPromise).toBe(existing);
  });

  it("reconnectLoop stops after maxAttempts failures", async () => {
    const connectSpy = jest
      .spyOn(TCPTransport.prototype, "connect")
      .mockImplementation(async () => {
        throw new Error("unreachable");
      });
    const closeSpy = jest
      .spyOn(TCPTransport.prototype, "close")
      .mockImplementation(() => {});

    const client = makeClient({
      reconnect: {
        maxAttempts: 2,
        initialDelayMs: 1,
        maxDelayMs: 1,
        backoffMultiplier: 1,
        jitterFactor: 0,
      },
    });

    await expect((client as any).reconnectLoop()).rejects.toThrow(
      "Max reconnect attempts reached",
    );

    connectSpy.mockRestore();
    closeSpy.mockRestore();
  });

  it("reconnectLoop returns once reconnection succeeds", async () => {
    let attempts = 0;
    const connectSpy = jest
      .spyOn(TCPTransport.prototype, "connect")
      .mockImplementation(async () => {
        attempts += 1;
        if (attempts === 1) {
          throw new Error("first try fails");
        }
      });
    const callSpy = jest
      .spyOn(TCPTransport.prototype, "call")
      .mockImplementation(async (msgType: number) => [
        msgType + 1,
        encodeConnectResponse("session-ok"),
      ]);
    const closeSpy = jest
      .spyOn(TCPTransport.prototype, "close")
      .mockImplementation(() => {});

    const client = makeClient({
      reconnect: {
        maxAttempts: 5,
        initialDelayMs: 1,
        maxDelayMs: 1,
        backoffMultiplier: 1,
        jitterFactor: 0,
      },
    });

    await expect((client as any).reconnectLoop()).resolves.toBeUndefined();
    expect(attempts).toBe(2);
    expect((client as any).sessionId).toBe("session-ok");

    // reconnectLoop restarts the heartbeat interval; stop it so jest can exit.
    await client.disconnect();

    connectSpy.mockRestore();
    callSpy.mockRestore();
    closeSpy.mockRestore();
  });

  it("calculateReconnectDelay applies jitter within bounds", () => {
    const client = makeClient({
      reconnect: {
        initialDelayMs: 100,
        maxDelayMs: 1000,
        backoffMultiplier: 2,
        jitterFactor: 0.5,
      },
    });
    for (let attempt = 1; attempt <= 5; attempt += 1) {
      const delay = (client as any).calculateReconnectDelay(attempt);
      expect(delay).toBeGreaterThanOrEqual(0);
      expect(delay).toBeLessThanOrEqual(1500);
    }
  });

  it("delay resolves after the given milliseconds", async () => {
    const client = makeClient();
    await expect((client as any).delay(1)).resolves.toBeUndefined();
  });
});

describe("serialization fallbacks", () => {
  it("parseProviderConnectResponse parses a JSON body", () => {
    const client = makeClient();
    const parsed = (client as any).parseProviderConnectResponse(
      Buffer.from(JSON.stringify({ session_id: "json-session" })),
    );
    expect(parsed).toEqual({ sessionId: "json-session" });
  });

  it("parseProviderConnectResponse falls back to protobuf for binary bodies", () => {
    const client = makeClient();
    const parsed = (client as any).parseProviderConnectResponse(
      encodeConnectResponse("pb-session"),
    );
    expect(parsed).toEqual({ sessionId: "pb-session" });
  });

  it("parseProviderConnectProtobufResponse decodes a protobuf body", () => {
    const client = makeClient();
    const parsed = (client as any).parseProviderConnectProtobufResponse(
      encodeConnectResponse("direct-pb"),
    );
    expect(parsed).toEqual({ sessionId: "direct-pb" });
  });
});
