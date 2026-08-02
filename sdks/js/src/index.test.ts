import * as protobuf from "protobufjs";
import { writeFileSync, unlinkSync } from "node:fs";
import { join } from "node:path";
import { Readable } from "node:stream";
import { TextDecoder } from "node:util";
import {
  BasicClient,
  createClient,
  FunctionDescriptor,
  InvokeOptions,
} from "./index";
import { TCPTransport } from "./tcp_transport";
import {
  MSG_PROVIDER_HEARTBEAT_REQUEST,
  MSG_REGISTER_CAPABILITIES_REQ,
  MSG_PROVIDER_CONNECT_REQUEST,
} from "./protocol";

const decoder = new TextDecoder();

const providerRoot = protobuf.parse(`
syntax = "proto3";
package croupier.sdk.v1;
message ProviderConnectResponse { string session_id = 1; }
message HeartbeatResponse {}
message LocalFunctionDescriptor {
  string id = 1;
  string version = 2;
  repeated string tags = 3;
  string summary = 4;
  string description = 5;
  string operation_id = 6;
  bool deprecated = 7;
  string input_schema = 8;
  string output_schema = 9;
  string resource = 10;
  string operation = 11;
  string risk = 12;
  bool enabled = 13;
  string permission = 14;
}
message ProviderConnectRequest {
  string service_id = 1;
  string version = 2;
  repeated LocalFunctionDescriptor functions = 3;
  string sdk_language = 4;
  string sdk_version = 5;
  string sdk_name = 6;
  string protocol_version = 7;
  repeated string supported_capabilities = 8;
  string transport_security_mode = 9;
  repeated string supported_transports = 10;
  string game_id = 11;
  string env = 12;
}
message HeartbeatRequest {
  string service_id = 1;
  string session_id = 2;
}
message ProviderMeta {
  string id = 1;
  string version = 2;
  string lang = 3;
  string sdk = 4;
}
message RegisterCapabilitiesRequest {
  ProviderMeta provider = 1;
  bytes manifest_json_gz = 2;
}
message RegisterCapabilitiesResponse {}
`).root;
const ProviderConnectResponseMessage = providerRoot.lookupType(
  "croupier.sdk.v1.ProviderConnectResponse",
);
const ProviderConnectRequestMessage = providerRoot.lookupType(
  "croupier.sdk.v1.ProviderConnectRequest",
);
const HeartbeatRequestMessage = providerRoot.lookupType(
  "croupier.sdk.v1.HeartbeatRequest",
);
const RegisterCapabilitiesRequestMessage = providerRoot.lookupType(
  "croupier.sdk.v1.RegisterCapabilitiesRequest",
);

describe("BasicClient", () => {
  test("uploadFile rejects when file transfer is disabled", async () => {
    const client = new BasicClient();

    await expect(
      client.uploadFile({
        filePath: "functions/test.js",
        content: "console.log('hi')",
      }),
    ).rejects.toThrow(/File transfer is disabled/i);
  });

  test("uploadFile validates content and returns metadata", async () => {
    const client = new BasicClient({
      enableFileTransfer: true,
      allowedExtensions: [".js"],
      allowedMimeTypes: ["text/javascript"],
      maxFileSize: 1024,
    });

    const result = await client.uploadFile({
      filePath: "functions/test.js",
      content: "console.log('hi')",
      metadata: { version: "1.0.0" },
    });

    expect(result.status).toBe("validated");
    expect(result.fileName).toBe("test.js");
    expect(result.mimeType).toBe("text/javascript");
    expect(result.metadata).toEqual({ version: "1.0.0" });
    expect(result.size).toBeGreaterThan(0);
    expect(result.sha256).toMatch(/^[a-f0-9]{64}$/);
  });

  test("uploadFile reports progress for inline content", async () => {
    const client = new BasicClient({
      enableFileTransfer: true,
      allowedExtensions: [".js"],
    });
    const progressEvents: Array<{
      filePath: string;
      loaded: number;
      total?: number;
      percent?: number;
    }> = [];

    const result = await client.uploadFile({
      filePath: "functions/test.js",
      content: "console.log('hi')",
      onProgress: (progress) => progressEvents.push(progress),
    });

    expect(progressEvents).toEqual([
      {
        filePath: "functions/test.js",
        loaded: result.size,
        total: result.size,
        percent: 100,
      },
    ]);
  });

  test("uploadFile reads content from disk when content is omitted", async () => {
    const filePath = join(process.cwd(), "tmp-js-sdk-upload.json");
    writeFileSync(filePath, JSON.stringify({ ok: true }), "utf8");

    try {
      const client = new BasicClient({
        enableFileTransfer: true,
        allowedExtensions: [".json"],
      });
      const progressEvents: Array<{
        filePath: string;
        loaded: number;
        total?: number;
        percent?: number;
      }> = [];

      const result = await client.uploadFile({
        filePath,
        onProgress: (progress) => progressEvents.push(progress),
      });
      expect(result.filePath).toBe(filePath);
      expect(result.mimeType).toBe("application/json");
      expect(result.size).toBeGreaterThan(0);
      expect(progressEvents).toEqual([
        {
          filePath,
          loaded: result.size,
          total: result.size,
          percent: 100,
        },
      ]);
    } finally {
      unlinkSync(filePath);
    }
  });

  test("uploadFile rejects disallowed file extension", async () => {
    const client = new BasicClient({
      enableFileTransfer: true,
      allowedExtensions: [".json"],
    });

    await expect(
      client.uploadFile({
        filePath: "functions/test.js",
        content: "console.log('hi')",
      }),
    ).rejects.toThrow(/not allowed/i);
  });

  test("uploadFileStream validates stream content", async () => {
    const client = new BasicClient({
      enableFileTransfer: true,
      allowedExtensions: [".txt"],
      allowedMimeTypes: ["text/plain"],
      maxFileSize: 1024,
    });

    const result = await client.uploadFileStream({
      filePath: "assets/readme.txt",
      stream: Readable.from(["hello ", "world"]),
    });

    expect(result.status).toBe("validated");
    expect(result.fileName).toBe("readme.txt");
    expect(result.mimeType).toBe("text/plain");
    expect(result.size).toBe(11);
  });

  test("uploadFileStream reports progress for stream content", async () => {
    const client = new BasicClient({
      enableFileTransfer: true,
      allowedExtensions: [".txt"],
      allowedMimeTypes: ["text/plain"],
      maxFileSize: 1024,
    });
    const progressEvents: Array<{
      filePath: string;
      loaded: number;
      total?: number;
      percent?: number;
    }> = [];

    const result = await client.uploadFileStream({
      filePath: "assets/readme.txt",
      stream: Readable.from(["hello ", "world"]),
      metadata: { size: "11" },
      onProgress: (progress) => progressEvents.push(progress),
    });

    expect(result.size).toBe(11);
    expect(progressEvents).toEqual([
      {
        filePath: "assets/readme.txt",
        loaded: 6,
        total: 11,
        percent: expect.closeTo(54.545454, 5),
      },
      {
        filePath: "assets/readme.txt",
        loaded: 11,
        total: 11,
        percent: 100,
      },
      {
        filePath: "assets/readme.txt",
        loaded: 11,
        total: 11,
        percent: 100,
      },
    ]);
  });

  test("uploadFiles validates batch requests", async () => {
    const client = new BasicClient({
      enableFileTransfer: true,
      allowedExtensions: [".js"],
      parallelUploads: 2,
    });

    const result = await client.uploadFiles([
      {
        filePath: "functions/a.js",
        content: "export const a = 1;",
      },
      {
        filePath: "functions/b.js",
        content: "export const b = 2;",
      },
    ]);

    expect(result.total).toBe(2);
    expect(result.succeeded).toBe(2);
    expect(result.failed).toBe(0);
    expect(result.results).toHaveLength(2);
    expect(result.items).toHaveLength(2);
    expect(result.items[0].ok).toBe(true);
    expect(result.items[1].ok).toBe(true);
    expect(result.results[0].fileName).toBe("a.js");
    expect(result.results[1].fileName).toBe("b.js");
  });

  test("uploadFiles reports per-item failures without aborting the batch", async () => {
    const client = new BasicClient({
      enableFileTransfer: true,
      allowedExtensions: [".js"],
      parallelUploads: 2,
    });

    const result = await client.uploadFiles([
      {
        filePath: "functions/a.js",
        content: "export const a = 1;",
      },
      {
        filePath: "functions/b.txt",
        content: "not allowed",
      },
      {
        filePath: "functions/c.js",
        content: "export const c = 3;",
      },
    ]);

    expect(result.total).toBe(3);
    expect(result.succeeded).toBe(2);
    expect(result.failed).toBe(1);
    expect(result.results).toHaveLength(2);
    expect(result.results[0].fileName).toBe("a.js");
    expect(result.results[1].fileName).toBe("c.js");
    expect(result.items).toEqual([
      expect.objectContaining({
        ok: true,
        filePath: "functions/a.js",
      }),
      {
        ok: false,
        filePath: "functions/b.txt",
        error: expect.stringMatching(/not allowed/i),
      },
      expect.objectContaining({
        ok: true,
        filePath: "functions/c.js",
      }),
    ]);
  });

  test("uploadFiles returns empty batch result for empty input", async () => {
    const client = new BasicClient({
      enableFileTransfer: true,
    });

    await expect(client.uploadFiles([])).resolves.toEqual({
      total: 0,
      succeeded: 0,
      failed: 0,
      results: [],
      items: [],
    });
  });

  test("connect requires at least one function", async () => {
    const client = new BasicClient({ agentAddr: "tcp://127.0.0.1:19090" });
    await expect(client.connect()).rejects.toThrow(
      /Register at least one function/i,
    );
  });

  test("registerFunction validates descriptor", () => {
    const client = new BasicClient();
    const handler = async () => "";

    expect(() =>
      client.registerFunction({ id: "", version: "1.0.0" }, handler),
    ).toThrow(/id and version/i);
    expect(() =>
      client.registerFunction({ id: "f1", version: "" }, handler),
    ).toThrow(/id and version/i);
  });

  test("buildManifest includes provider and functions", () => {
    const client = new BasicClient({
      serviceId: "svc-1",
      serviceVersion: "sv1",
      providerLang: "node",
      providerSdk: "croupier-js-sdk",
    });

    client.registerFunction(
      { id: "f1", version: "1.2.3", name: "category-a", description: "desc" },
      async () => "ok",
    );

    const manifest = (client as any).buildManifest();
    expect(manifest.provider).toEqual({
      id: "svc-1",
      version: "sv1",
      lang: "node",
      sdk: "croupier-js-sdk",
    });
    expect(manifest.functions).toHaveLength(1);
    expect(manifest.functions[0].id).toBe("f1");
    expect(manifest.functions[0].version).toBe("1.2.3");
  });

  test("startTask/streamTask produces started then completed", async () => {
    const client = new BasicClient();
    client.registerFunction(
      { id: "f1", version: "1.0.0" },
      async (_ctx, payload) => {
        await new Promise((resolve) => setTimeout(resolve, 25));
        return `ok:${payload}`;
      },
    );

    const taskId = client.startTask("f1", "hi");
    const iterable = client.streamTask(taskId);
    const events: any[] = [];

    for await (const evt of iterable) {
      events.push(evt);
    }

    expect(events[0].type).toBe("started");
    expect(events[events.length - 1].type).toBe("completed");
    expect(new TextDecoder().decode(events[events.length - 1].payload)).toBe(
      "ok:hi",
    );
  });

  test("cancelTask closes stream with cancelled event", async () => {
    const client = new BasicClient();
    client.registerFunction({ id: "f1", version: "1.0.0" }, async () => {
      await new Promise((resolve) => setTimeout(resolve, 200));
      return "late";
    });

    const taskId = client.startTask("f1", "");
    const iterable = client.streamTask(taskId);

    // Get the first event (started)
    const reader = iterable[Symbol.asyncIterator]();
    const first = await reader.next();
    expect(first.value.type).toBe("started");

    // Cancel the task
    client.cancelTask(taskId);

    // Continue streaming
    const events: any[] = [first.value];
    for await (const evt of iterable) {
      events.push(evt);
    }

    expect(events.some((e) => e.type === "cancelled")).toBe(true);
  });

  test("constructor applies default config", () => {
    const client = new BasicClient();
    const config = (client as any).config;

    expect(config.agentAddr).toBe("tcp://127.0.0.1:19090");
    expect(config.timeout).toBe(30000);
    expect(config.agentId).toBe("");
    expect(config.env).toBe("development");
    expect(config.serviceVersion).toBe("1.0.0");
    expect(config.heartbeatIntervalSeconds).toBe(60);
    expect(config.providerLang).toBe("node");
    expect(config.providerSdk).toBe("croupier-js-sdk");
    expect(config.insecure).toBe(true);
    expect(config.serverName).toBe("");
  });

  test("constructor merges user config", () => {
    const client = new BasicClient({
      agentAddr: "tcp://192.168.1.1:9999",
      serviceId: "my-service",
      timeout: 5000,
    });
    const config = (client as any).config;

    expect(config.agentAddr).toBe("tcp://192.168.1.1:9999");
    expect(config.serviceId).toBe("my-service");
    expect(config.timeout).toBe(5000);
    // Defaults should still apply for unspecified values
    expect(config.serviceVersion).toBe("1.0.0");
  });

  test("invoke calls registered handler locally", async () => {
    const client = new BasicClient();
    const handler = jest.fn().mockResolvedValue("test-result");
    client.registerFunction({ id: "test.fn", version: "1.0.0" }, handler);

    const result = await client.invoke("test.fn", "input");

    // Context now includes timeout from client config
    expect(handler).toHaveBeenCalledWith(
      expect.stringContaining("timeout"),
      "input",
    );
    expect(result).toBe("test-result");
  });

  test("invoke throws for unregistered function", async () => {
    const client = new BasicClient();

    await expect(client.invoke("unknown.fn", "")).rejects.toThrow(/not found/i);
  });

  test("invoke propagates handler errors", async () => {
    const client = new BasicClient();
    const handler = jest.fn().mockRejectedValue(new Error("Handler error"));
    client.registerFunction({ id: "failing.fn", version: "1.0.0" }, handler);

    await expect(client.invoke("failing.fn", "")).rejects.toThrow(
      /Handler error/,
    );
  });

  test("registerFunction throws when connected", () => {
    const client = new BasicClient();
    client.registerFunction(
      {
        id: "f1",
        version: "1.0.0",
        resource: "player",
        operation: "ban",
        risk: "danger",
        permission: "player:ban",
      },
      async () => "ok",
    );

    // Manually set connected to true (since we can't actually connect in tests)
    (client as any).connected = true;

    expect(() =>
      client.registerFunction({ id: "f2", version: "1.0.0" }, async () => "ok"),
    ).toThrow(/Cannot register new functions while connected/);
  });

  test("streamTask throws for missing taskId", () => {
    const client = new BasicClient();

    expect(() => client.streamTask("")).toThrow();
  });

  test("startTask creates task state and returns taskId", () => {
    const client = new BasicClient();
    client.registerFunction(
      { id: "async.fn", version: "1.0.0" },
      async () => "done",
    );

    const taskId = client.startTask("async.fn", "");

    expect(taskId).toBeDefined();
    expect(taskId).toContain("async.fn-");
  });

  test("startTask emits error event on handler failure", async () => {
    const client = new BasicClient();
    client.registerFunction(
      { id: "failing.fn", version: "1.0.0" },
      async () => {
        throw new Error("Async error");
      },
    );

    const taskId = client.startTask("failing.fn", "");
    const events: any[] = [];

    for await (const evt of client.streamTask(taskId)) {
      events.push(evt);
    }

    expect(events[0].type).toBe("started");
    expect(events[1].type).toBe("error");
    expect(events[1].message).toBe("Async error");
  });

  test("buildManifest includes all descriptor fields", () => {
    const client = new BasicClient();

    client.registerFunction(
      {
        id: "full.fn",
        version: "2.0.0",
        resource: "full-resource",
        operation: "full-operation",
        risk: "safe",
        permission: "full:invoke",
        description: "A full function descriptor",
        input_schema: { type: "object" },
        output_schema: { type: "string" },
      },
      async () => "ok",
    );

    const manifest = (client as any).buildManifest();
    const fn = manifest.functions.find((f: any) => f.id === "full.fn");

    expect(fn).toBeDefined();
    expect(fn.version).toBe("2.0.0");
    expect(fn.resource).toBe("full-resource");
    expect(fn.operation).toBe("full-operation");
    expect(fn.risk).toBe("safe");
    expect(fn.permission).toBe("full:invoke");
    expect(fn.description).toBe("A full function descriptor");
    expect(fn.input_schema).toEqual({ type: "object" });
    expect(fn.output_schema).toEqual({ type: "string" });
  });

  test("cancelTask does nothing for unknown task", () => {
    const client = new BasicClient();

    // Should return false (not throw)
    const result = client.cancelTask("unknown-task");
    expect(result).toBe(false);
  });

  test("buildManifest defaults version to 1.0.0", () => {
    const client = new BasicClient();

    // Register with empty version (though registerFunction would reject this)
    (client as any).descriptors.set("test", { id: "test", version: "" });

    const manifest = (client as any).buildManifest();
    const fn = manifest.functions.find((f: any) => f.id === "test");

    expect(fn.version).toBe("1.0.0");
  });

  test("disconnect clears state", async () => {
    const client = new BasicClient();
    client.registerFunction(
      {
        id: "f1",
        version: "1.0.0",
        resource: "player",
        operation: "ban",
        risk: "danger",
        permission: "player:ban",
      },
      async () => "ok",
    );

    // Set some state
    (client as any).connected = true;

    await client.disconnect();

    expect((client as any).connected).toBe(false);
  });

  test("invoke with empty payload uses default", async () => {
    const client = new BasicClient();
    const handler = jest.fn().mockResolvedValue("result");
    client.registerFunction({ id: "test.fn", version: "1.0.0" }, handler);

    await client.invoke("test.fn", "");

    expect(handler).toHaveBeenCalledWith(expect.any(String), "");
  });

  test("connect is idempotent - multiple calls are safe", async () => {
    const client = new BasicClient();

    // First connect would fail without mocking, so we test the idempotency check
    (client as any).connected = true;

    // Should not throw, should just return
    await expect(client.connect()).resolves.not.toThrow();
  });

  test("connect sends ProviderConnectRequest and stores session id", async () => {
    const registerCalls: Buffer[] = [];
    const connectSpy = jest
      .spyOn(TCPTransport.prototype, "connect")
      .mockImplementation(async () => {});
    const callSpy = jest
      .spyOn(TCPTransport.prototype, "call")
      .mockImplementation(async (msgType, data) => {
        if (msgType === MSG_PROVIDER_CONNECT_REQUEST) {
          registerCalls.push(Buffer.from(data));
          const response = ProviderConnectResponseMessage.create({
            sessionId: "session-1",
          });
          return [
            msgType + 1,
            Buffer.from(ProviderConnectResponseMessage.encode(response).finish()),
          ];
        }
        if (msgType === MSG_PROVIDER_HEARTBEAT_REQUEST) {
          return [msgType + 1, Buffer.alloc(0)];
        }
        throw new Error(`Unexpected msgType ${msgType}`);
      });
    const closeSpy = jest
      .spyOn(TCPTransport.prototype, "close")
      .mockImplementation(() => {});

    const client = new BasicClient({
      serviceId: "test-service",
      heartbeatIntervalSeconds: 60,
    });
    client.registerFunction(
      { id: "test.fn", version: "1.0.0" },
      async () => "ok",
    );

    await client.connect();

    const request = ProviderConnectRequestMessage.decode(registerCalls[0]) as any;
    expect(request.serviceId).toBe("test-service");
    expect(request.sdkLanguage).toBe("node");
    expect(request.transportSecurityMode).toBe("plaintext");
    expect((client as any).sessionId).toBe("session-1");

    await client.disconnect();
    connectSpy.mockRestore();
    callSpy.mockRestore();
    closeSpy.mockRestore();
  });

  test("connect uploads capabilities when controlAddr is configured", async () => {
    const capabilityCalls: Buffer[] = [];
    const connectSpy = jest
      .spyOn(TCPTransport.prototype, "connect")
      .mockImplementation(async () => {});
    const callSpy = jest
      .spyOn(TCPTransport.prototype, "call")
      .mockImplementation(async (msgType, data) => {
        if (msgType === MSG_PROVIDER_CONNECT_REQUEST) {
          const response = ProviderConnectResponseMessage.create({
            sessionId: "session-1",
          });
          return [
            msgType + 1,
            Buffer.from(ProviderConnectResponseMessage.encode(response).finish()),
          ];
        }
        if (msgType === MSG_REGISTER_CAPABILITIES_REQ) {
          capabilityCalls.push(Buffer.from(data));
          return [msgType + 1, Buffer.alloc(0)];
        }
        if (msgType === MSG_PROVIDER_HEARTBEAT_REQUEST) {
          return [msgType + 1, Buffer.alloc(0)];
        }
        throw new Error(`Unexpected msgType ${msgType}`);
      });
    const closeSpy = jest
      .spyOn(TCPTransport.prototype, "close")
      .mockImplementation(() => {});

    const client = new BasicClient({
      serviceId: "test-service",
      serviceVersion: "2.0.0",
      providerLang: "node",
      providerSdk: "croupier-js-sdk",
      controlAddr: "tcp://127.0.0.1:19100",
    });
    client.registerFunction(
      { id: "test.fn", version: "1.0.0" },
      async () => "ok",
    );

    await client.connect();

    expect(capabilityCalls).toHaveLength(1);
    const decoded = RegisterCapabilitiesRequestMessage.decode(
      capabilityCalls[0],
    );
    const request = RegisterCapabilitiesRequestMessage.toObject(decoded, {
      defaults: true,
    }) as {
      provider: { id: string; version: string; lang: string; sdk: string };
      manifestJsonGz: Uint8Array;
    };
    expect(request.provider).toEqual({
      id: "test-service",
      version: "2.0.0",
      lang: "node",
      sdk: "croupier-js-sdk",
    });
    expect(Buffer.from(request.manifestJsonGz).length).toBeGreaterThan(0);

    await client.disconnect();
    connectSpy.mockRestore();
    callSpy.mockRestore();
    closeSpy.mockRestore();
  });

  test("connect ignores capability upload failures", async () => {
    const warnSpy = jest.spyOn(console, "warn").mockImplementation(() => {});
    const connectSpy = jest
      .spyOn(TCPTransport.prototype, "connect")
      .mockImplementation(async () => {});
    const callSpy = jest
      .spyOn(TCPTransport.prototype, "call")
      .mockImplementation(async (msgType) => {
        if (msgType === MSG_PROVIDER_CONNECT_REQUEST) {
          const response = ProviderConnectResponseMessage.create({
            sessionId: "session-1",
          });
          return [
            msgType + 1,
            Buffer.from(ProviderConnectResponseMessage.encode(response).finish()),
          ];
        }
        if (msgType === MSG_REGISTER_CAPABILITIES_REQ) {
          throw new Error("capabilities failed");
        }
        if (msgType === MSG_PROVIDER_HEARTBEAT_REQUEST) {
          return [msgType + 1, Buffer.alloc(0)];
        }
        throw new Error(`Unexpected msgType ${msgType}`);
      });
    const closeSpy = jest
      .spyOn(TCPTransport.prototype, "close")
      .mockImplementation(() => {});

    const client = new BasicClient({
      serviceId: "test-service",
      controlAddr: "tcp://127.0.0.1:19100",
    });
    client.registerFunction(
      { id: "test.fn", version: "1.0.0" },
      async () => "ok",
    );

    await expect(client.connect()).resolves.not.toThrow();
    expect((client as any).sessionId).toBe("session-1");
    expect(warnSpy).toHaveBeenCalled();

    await client.disconnect();
    warnSpy.mockRestore();
    connectSpy.mockRestore();
    callSpy.mockRestore();
    closeSpy.mockRestore();
  });

  test("heartbeat failure triggers reconnect and re-register", async () => {
    jest.useFakeTimers();

    let registerCount = 0;
    let heartbeatCount = 0;
    const connectSpy = jest
      .spyOn(TCPTransport.prototype, "connect")
      .mockImplementation(async () => {});
    const callSpy = jest
      .spyOn(TCPTransport.prototype, "call")
      .mockImplementation(async (msgType, data) => {
        if (msgType === MSG_PROVIDER_CONNECT_REQUEST) {
          registerCount += 1;
          const response = ProviderConnectResponseMessage.create({
            sessionId: `session-${registerCount}`,
          });
          return [
            msgType + 1,
            Buffer.from(ProviderConnectResponseMessage.encode(response).finish()),
          ];
        }
        if (msgType === MSG_PROVIDER_HEARTBEAT_REQUEST) {
          heartbeatCount += 1;
          if (heartbeatCount === 1) {
            throw new Error("heartbeat failed");
          }
          const decoded = HeartbeatRequestMessage.decode(data);
          const request = HeartbeatRequestMessage.toObject(decoded, {
            defaults: true,
          }) as {
            sessionId: string;
          };
          expect(request.sessionId).toBe("session-2");
          return [msgType + 1, Buffer.alloc(0)];
        }
        throw new Error(`Unexpected msgType ${msgType}`);
      });
    const closeSpy = jest
      .spyOn(TCPTransport.prototype, "close")
      .mockImplementation(() => {});

    const client = new BasicClient({
      serviceId: "test-service",
      heartbeatIntervalSeconds: 1,
      reconnect: {
        initialDelayMs: 10,
        maxDelayMs: 10,
        backoffMultiplier: 1,
        jitterFactor: 0,
      },
    });
    client.registerFunction(
      { id: "test.fn", version: "1.0.0" },
      async () => "ok",
    );

    await client.connect();
    expect(registerCount).toBe(1);
    expect((client as any).sessionId).toBe("session-1");

    await jest.advanceTimersByTimeAsync(1000);
    await jest.advanceTimersByTimeAsync(20);

    expect(registerCount).toBeGreaterThanOrEqual(2);
    expect((client as any).sessionId).toBe("session-2");

    await client.disconnect();
    jest.useRealTimers();
    connectSpy.mockRestore();
    callSpy.mockRestore();
    closeSpy.mockRestore();
  });

  test("startTask throws for unregistered function", () => {
    const client = new BasicClient();

    expect(() => client.startTask("unknown.fn", "")).toThrow(/not found/i);
  });

  test("streamTask throws for unknown task", () => {
    const client = new BasicClient();

    expect(() => client.streamTask("unknown-task-id")).toThrow(/not found/i);
  });

  test("buildManifest omits resource when descriptor has none", () => {
    const client = new BasicClient();

    client.registerFunction(
      {
        id: "simple.fn",
        version: "1.0.0",
        // no resource specified
      },
      async () => "ok",
    );

    const manifest = (client as any).buildManifest();
    const fn = manifest.functions.find((f: any) => f.id === "simple.fn");

    expect(fn).toBeDefined();
    expect(fn.resource).toBeUndefined();
  });

  test("createClient exports a factory function", () => {
    const client = createClient({ serviceId: "test-service" });

    expect(client).toBeInstanceOf(BasicClient);
  });

  test("startTask creates task with correct taskId format", () => {
    const client = new BasicClient();
    client.registerFunction(
      { id: "test.fn", version: "1.0.0" },
      async () => "ok",
    );

    const taskId = client.startTask("test.fn", "");

    expect(taskId).toContain("test.fn-");
    expect(typeof taskId).toBe("string");
  });

  test("invoke with metadata", async () => {
    const client = new BasicClient();
    const handler = jest.fn().mockResolvedValue("result");
    client.registerFunction({ id: "test.fn", version: "1.0.0" }, handler);

    const metadata = { "x-custom-header": "value" };
    const result = await client.invoke("test.fn", "", metadata);

    expect(result).toBe("result");
    // Handler should receive metadata as JSON string in context
    expect(handler).toHaveBeenCalledWith(
      expect.stringContaining("x-custom-header"),
      "",
    );
  });

  test("getFunctionDescriptor returns correct descriptor", () => {
    const client = new BasicClient();
    client.registerFunction(
      {
        id: "test.fn",
        version: "2.0.0",
        tags: ["test"],
        summary: "Test function",
        description: "Detailed test function description",
        input_schema: { type: "object", properties: { id: { type: "string" } } },
        output_schema: { type: "object", properties: { ok: { type: "boolean" } } },
        resource: "player",
        risk: "low",
        operation: "ban",
        capability: "action",
        execution: "sync",
        permission: "player:ban",
      },
      async () => "ok",
    );

    const desc = client.getFunctionDescriptor("test.fn");

    expect(desc).toEqual({
      id: "test.fn",
      version: "2.0.0",
      tags: ["test"],
      summary: "Test function",
      description: "Detailed test function description",
      operation_id: "test.fn",
      deprecated: undefined,
      input_schema: JSON.stringify({ type: "object", properties: { id: { type: "string" } } }),
      output_schema: JSON.stringify({ type: "object", properties: { ok: { type: "boolean" } } }),
      resource: "player",
      risk: "low",
      operation: "ban",
      capability: "action",
      execution: "sync",
      permission: "player:ban",
      enabled: undefined,
    });
  });

  test("getFunctionDescriptor returns undefined for unknown function", () => {
    const client = new BasicClient();

    const desc = client.getFunctionDescriptor("unknown.fn");

    expect(desc).toBeUndefined();
  });

  test("getRegisterRequest builds correct request", () => {
    const client = new BasicClient({
      serviceId: "test-svc",
      serviceVersion: "1.0.0",
    });
    client.registerFunction(
      {
        id: "f1",
        version: "1.0.0",
        resource: "player",
        operation: "ban",
        capability: "action",
        execution: "sync",
        risk: "danger",
        permission: "player:ban",
      },
      async () => "ok",
    );

    const req = client.getRegisterRequest();

    expect(req.serviceId).toBe("test-svc");
    expect(req.version).toBe("1.0.0");
    expect(req.functions).toHaveLength(1);
    expect(req.functions[0].id).toBe("f1");
    expect(req.functions[0].resource).toBe("player");
    expect(req.functions[0].operation).toBe("ban");
    expect(req.functions[0].capability).toBe("action");
    expect(req.functions[0].execution).toBe("sync");
    expect(req.functions[0].risk).toBe("danger");
    expect(req.functions[0].permission).toBe("player:ban");
    expect(req.functions[0]).not.toHaveProperty("category_display");
    expect(req.functions[0]).not.toHaveProperty("entity_display");
    expect(req.functions[0]).not.toHaveProperty("operation_display");
    expect(req.functions[0]).not.toHaveProperty("operation_kind");
    expect(req.functions[0]).not.toHaveProperty("placement");
    expect(req.functions[0]).not.toHaveProperty("page_hint");
    expect(req.functions[0]).not.toHaveProperty("extensions");
  });

  test("getManifestGzipped returns gzipped manifest", () => {
    const client = new BasicClient();
    client.registerFunction({ id: "f1", version: "1.0.0" }, async () => "ok");

    const gzipped = client.getManifestGzipped();

    expect(gzipped).toBeInstanceOf(Buffer);
    expect(gzipped.length).toBeGreaterThan(0);
  });

  test("startTask with metadata passes to handler", async () => {
    const client = new BasicClient();
    let receivedContext = "";
    client.registerFunction(
      { id: "test.fn", version: "1.0.0" },
      async (ctx) => {
        receivedContext = ctx;
        return "done";
      },
    );

    const taskId = client.startTask("test.fn", "payload", { key: "value" });

    // Wait for task to complete
    for await (const _ of client.streamTask(taskId)) {
      // just consume events
    }

    expect(receivedContext).toContain("key");
    expect(receivedContext).toContain("value");
  });

  test("cancelTask returns true when task exists", async () => {
    const client = new BasicClient();
    client.registerFunction({ id: "test.fn", version: "1.0.0" }, async () => {
      await new Promise((resolve) => setTimeout(resolve, 100));
      return "done";
    });

    const taskId = client.startTask("test.fn", "");

    // Wait for started event
    const reader = client.streamTask(taskId)[Symbol.asyncIterator]();
    await reader.next();

    const result = client.cancelTask(taskId);
    expect(result).toBe(true);
  });

  test("disconnect without transport", async () => {
    const client = new BasicClient();
    client.registerFunction({ id: "f1", version: "1.0.0" }, async () => "ok");

    // No transport set, just set connected
    (client as any).connected = true;

    // Should not throw
    await client.disconnect();
    expect((client as any).connected).toBe(false);
  });

  test("task handler with byte array result", async () => {
    const client = new BasicClient();
    client.registerFunction(
      { id: "byte.fn", version: "1.0.0" },
      async () => "binary-data" as string,
    );

    const result = await client.invoke("byte.fn", "");
    expect(result).toBe("binary-data");
  });

  test("invoke with function returning number", async () => {
    const client = new BasicClient();
    client.registerFunction(
      { id: "number.fn", version: "1.0.0" },
      async () => "42",
    );

    const result = await client.invoke("number.fn", "");
    expect(result).toBe("42");
  });

  test("getRegisterRequest returns function list", () => {
    const client = new BasicClient({
      serviceId: "test-svc",
      serviceVersion: "1.0.0",
    });
    client.registerFunction({ id: "f1", version: "1.0.0" }, async () => "ok");

    const req = client.getRegisterRequest();

    expect(req.functions).toHaveLength(1);
    expect(req.functions[0].id).toBe("f1");
  });

  test("startTask with multiple rapid cancels", async () => {
    const client = new BasicClient();
    client.registerFunction({ id: "test.fn", version: "1.0.0" }, async () => {
      await new Promise((resolve) => setTimeout(resolve, 500));
      return "done";
    });

    const taskId = client.startTask("test.fn", "");

    // Cancel multiple times - should be idempotent
    const result1 = client.cancelTask(taskId);
    const result2 = client.cancelTask(taskId);

    expect(result1).toBe(true);
    expect(result2).toBe(false); // Second cancel returns false since task is already cancelled
  });

  test("function descriptor with all optional fields", () => {
    const client = new BasicClient();
    client.registerFunction(
      {
        id: "complete.fn",
        version: "1.0.0",
        name: "Complete Function",
        description: "A complete function descriptor",
        resource: "system",
        risk: "low",
        operation: "health",
        permission: "system:health",
        input_schema: { type: "object" },
        output_schema: { type: "string" },
      },
      async () => "ok",
    );

    const desc = client.getFunctionDescriptor("complete.fn");

    expect(desc?.id).toBe("complete.fn");
    expect(desc?.version).toBe("1.0.0");
    expect(desc?.resource).toBe("system");
    expect(desc?.risk).toBe("low");
    expect(desc?.operation).toBe("health");
    expect(desc?.permission).toBe("system:health");
    expect(desc?.input_schema).toBe(JSON.stringify({ type: "object" }));
    expect(desc?.output_schema).toBe(JSON.stringify({ type: "string" }));
  });

  test("invoke handler that returns Uint8Array", async () => {
    const client = new BasicClient();
    client.registerFunction(
      { id: "uint8.fn", version: "1.0.0" },
      async () => "binary-data" as string,
    );

    const result = await client.invoke("uint8.fn", "");
    expect(result).toBeDefined();
  });

  test("task error event includes progress", async () => {
    const client = new BasicClient();
    client.registerFunction({ id: "error.fn", version: "1.0.0" }, async () => {
      throw new Error("Test error");
    });

    const taskId = client.startTask("error.fn", "");
    const events: any[] = [];

    for await (const evt of client.streamTask(taskId)) {
      events.push(evt);
    }

    expect(events[1].type).toBe("error");
    expect(events[1].progress).toBe(0);
  });

  test("invoke with empty metadata object", async () => {
    const client = new BasicClient();
    const handler = jest.fn().mockResolvedValue("result");
    client.registerFunction({ id: "test.fn", version: "1.0.0" }, handler);

    await client.invoke("test.fn", "payload", {});

    // Context now includes timeout from client config
    expect(handler).toHaveBeenCalledWith(
      expect.stringContaining("timeout"),
      "payload",
    );
  });

  describe("serve()", () => {
    test("serve() requires at least one function", async () => {
      const client = new BasicClient({ agentAddr: "tcp://127.0.0.1:19090" });

      // Should reject because no functions registered
      await expect(client.serve()).rejects.toThrow();
    });

    test("serve() connects and waits indefinitely", async () => {
      const client = new BasicClient();
      client.registerFunction(
        { id: "serve.fn", version: "1.0.0" },
        async () => "ok",
      );

      // Mock the connect method to avoid actual connection
      const connectSpy = jest
        .spyOn(client, "connect")
        .mockImplementation(async () => {
          (client as any).connected = true;
        });

      // serve() should return a Promise
      const servePromise = client.serve();
      expect(servePromise).toBeInstanceOf(Promise);
      expect(connectSpy).toHaveBeenCalled();

      // It should not resolve immediately
      let resolved = false;
      servePromise.then(() => {
        resolved = true;
      });
      await new Promise((resolve) => setTimeout(resolve, 50));
      expect(resolved).toBe(false);

      // Cleanup - restore connect
      connectSpy.mockRestore();
      (client as any).connected = false;
    });

    test("serveAsync is alias for serve", async () => {
      const client = new BasicClient();
      client.registerFunction(
        { id: "serve.fn", version: "1.0.0" },
        async () => "ok",
      );

      // Mock connect to avoid actual connection
      jest.spyOn(client, "connect").mockImplementation(async () => {
        (client as any).connected = true;
      });

      // Both methods should return promises
      const serve1 = client.serve();
      const serve2 = client.serveAsync();

      // Both should be Promise instances
      expect(serve1).toBeInstanceOf(Promise);
      expect(serve2).toBeInstanceOf(Promise);

      // Cleanup
      (client.connect as jest.Mock).mockRestore();
      (client as any).connected = false;
    });

    test("serve() connects if not connected", async () => {
      const client = new BasicClient();
      client.registerFunction(
        { id: "serve.fn", version: "1.0.0" },
        async () => "ok",
      );

      // Mock the connect method BEFORE calling serve
      const connectSpy = jest
        .spyOn(client, "connect")
        .mockImplementation(async () => {
          (client as any).connected = true;
        });
      (client as any).connected = false;

      const servePromise = client.serve();

      // Give it a moment to call connect
      await new Promise((resolve) => setTimeout(resolve, 50));
      expect(connectSpy).toHaveBeenCalled();

      // Cleanup
      connectSpy.mockRestore();
      (client as any).connected = false;
    });
  });

  describe("RetryConfig and ReconnectConfig", () => {
    test("constructor includes default retry config", () => {
      const client = new BasicClient();
      const config = (client as any).config;

      expect(config.retry).toBeDefined();
      expect(config.retry.enabled).toBe(true);
      expect(config.retry.maxAttempts).toBe(3);
      expect(config.retry.initialDelayMs).toBe(100);
      expect(config.retry.maxDelayMs).toBe(5000);
      expect(config.retry.backoffMultiplier).toBe(2.0);
      expect(config.retry.jitterFactor).toBe(0.1);
      expect(config.retry.retryableStatusCodes).toEqual([14, 13, 2, 10, 4]);
    });

    test("constructor includes default reconnect config", () => {
      const client = new BasicClient();
      const config = (client as any).config;

      expect(config.reconnect).toBeDefined();
      expect(config.reconnect.enabled).toBe(true);
      expect(config.reconnect.maxAttempts).toBe(0); // Infinite
      expect(config.reconnect.initialDelayMs).toBe(1000);
      expect(config.reconnect.maxDelayMs).toBe(30000);
      expect(config.reconnect.backoffMultiplier).toBe(2.0);
      expect(config.reconnect.jitterFactor).toBe(0.2);
    });

    test("constructor merges user retry config", () => {
      const client = new BasicClient({
        retry: {
          maxAttempts: 5,
          initialDelayMs: 200,
        },
      });
      const config = (client as any).config;

      expect(config.retry.maxAttempts).toBe(5);
      expect(config.retry.initialDelayMs).toBe(200);
      // Other defaults should still apply
      expect(config.retry.maxDelayMs).toBe(5000);
      expect(config.retry.backoffMultiplier).toBe(2.0);
    });

    test("constructor merges user reconnect config", () => {
      const client = new BasicClient({
        reconnect: {
          maxAttempts: 10,
          initialDelayMs: 500,
        },
      });
      const config = (client as any).config;

      expect(config.reconnect.maxAttempts).toBe(10);
      expect(config.reconnect.initialDelayMs).toBe(500);
      // Other defaults should still apply
      expect(config.reconnect.maxDelayMs).toBe(30000);
      expect(config.reconnect.backoffMultiplier).toBe(2.0);
    });

    test("invoke accepts InvokeOptions with retry config", async () => {
      const client = new BasicClient();
      const handler = jest.fn().mockResolvedValue("result");
      client.registerFunction({ id: "test.fn", version: "1.0.0" }, handler);

      const invokeOptions = {
        retry: {
          maxAttempts: 3,
          initialDelayMs: 100,
        },
        headers: {
          "x-custom": "value",
        },
      };

      const result = await client.invoke("test.fn", "payload", invokeOptions);

      expect(result).toBe("result");
      expect(handler).toHaveBeenCalledWith(
        expect.stringContaining("x-custom"),
        "payload",
      );
    });

    test("invoke accepts Record<string, string> as metadata (backward compatibility)", async () => {
      const client = new BasicClient();
      const handler = jest.fn().mockResolvedValue("result");
      client.registerFunction({ id: "test.fn", version: "1.0.0" }, handler);

      const metadata = {
        "x-custom": "value",
      };

      const result = await client.invoke("test.fn", "payload", metadata);

      expect(result).toBe("result");
      expect(handler).toHaveBeenCalledWith(
        expect.stringContaining("x-custom"),
        "payload",
      );
    });

    test("startTask accepts InvokeOptions", async () => {
      const client = new BasicClient();
      let receivedContext = "";
      client.registerFunction(
        { id: "test.fn", version: "1.0.0" },
        async (ctx) => {
          receivedContext = ctx;
          return "done";
        },
      );

      const invokeOptions = {
        retry: {
          maxAttempts: 5,
        },
        headers: {
          "x-custom": "value",
        },
      };

      const taskId = client.startTask("test.fn", "payload", invokeOptions);

      // Wait for task to complete
      for await (const _ of client.streamTask(taskId)) {
        // consume events
      }

      expect(receivedContext).toContain("x-custom");
      expect(receivedContext).toContain("value");
    });

    test("RetryConfig can be disabled", () => {
      const client = new BasicClient({
        retry: {
          enabled: false,
        },
      });
      const config = (client as any).config;

      expect(config.retry.enabled).toBe(false);
    });

    test("ReconnectConfig can be disabled", () => {
      const client = new BasicClient({
        reconnect: {
          enabled: false,
        },
      });
      const config = (client as any).config;

      expect(config.reconnect.enabled).toBe(false);
    });
  });

  describe("InvokeOptions - idempotencyKey and timeout", () => {
    test("invoke accepts idempotencyKey in InvokeOptions", async () => {
      const client = new BasicClient();
      let receivedContext = "";
      const handler = jest.fn().mockImplementation(async (ctx) => {
        receivedContext = ctx;
        return "result";
      });
      client.registerFunction({ id: "test.fn", version: "1.0.0" }, handler);

      const invokeOptions = {
        idempotencyKey: "unique-key-123",
        headers: {
          "x-custom": "value",
        },
      };

      const result = await client.invoke("test.fn", "payload", invokeOptions);

      expect(result).toBe("result");
      expect(handler).toHaveBeenCalled();
      const context = JSON.parse(receivedContext);
      expect(context.idempotency_key).toBe("unique-key-123");
      expect(context["x-custom"]).toBe("value");
    });

    test("invoke accepts timeout in InvokeOptions", async () => {
      const client = new BasicClient();
      let receivedContext = "";
      const handler = jest.fn().mockImplementation(async (ctx) => {
        receivedContext = ctx;
        return "result";
      });
      client.registerFunction({ id: "test.fn", version: "1.0.0" }, handler);

      const invokeOptions = {
        timeout: 10000,
        headers: {},
      };

      const result = await client.invoke("test.fn", "payload", invokeOptions);

      expect(result).toBe("result");
      const context = JSON.parse(receivedContext);
      expect(context.timeout).toBe(10000);
    });

    test("invoke uses client-level timeout when not specified in options", async () => {
      const client = new BasicClient({ timeout: 5000 });
      let receivedContext = "";
      const handler = jest.fn().mockImplementation(async (ctx) => {
        receivedContext = ctx;
        return "result";
      });
      client.registerFunction({ id: "test.fn", version: "1.0.0" }, handler);

      const result = await client.invoke("test.fn", "payload", {});

      expect(result).toBe("result");
      const context = JSON.parse(receivedContext);
      expect(context.timeout).toBe(5000);
    });

    test("invoke timeout overrides client-level timeout", async () => {
      const client = new BasicClient({ timeout: 5000 });
      let receivedContext = "";
      const handler = jest.fn().mockImplementation(async (ctx) => {
        receivedContext = ctx;
        return "result";
      });
      client.registerFunction({ id: "test.fn", version: "1.0.0" }, handler);

      const invokeOptions = {
        timeout: 10000,
      };

      const result = await client.invoke("test.fn", "payload", invokeOptions);

      expect(result).toBe("result");
      const context = JSON.parse(receivedContext);
      expect(context.timeout).toBe(10000); // Override should work
    });

    test("startTask accepts idempotencyKey in InvokeOptions", async () => {
      const client = new BasicClient();
      let receivedContext = "";
      client.registerFunction(
        { id: "test.fn", version: "1.0.0" },
        async (ctx) => {
          receivedContext = ctx;
          return "done";
        },
      );

      const invokeOptions = {
        idempotencyKey: "job-key-456",
        headers: {
          "x-job-id": "123",
        },
      };

      const taskId = client.startTask("test.fn", "payload", invokeOptions);

      // Wait for task to complete
      for await (const _ of client.streamTask(taskId)) {
        // consume events
      }

      const context = JSON.parse(receivedContext);
      expect(context.idempotency_key).toBe("job-key-456");
      expect(context["x-job-id"]).toBe("123");
    });

    test("startTask accepts timeout in InvokeOptions", async () => {
      const client = new BasicClient();
      let receivedContext = "";
      client.registerFunction(
        { id: "test.fn", version: "1.0.0" },
        async (ctx) => {
          receivedContext = ctx;
          return "done";
        },
      );

      const invokeOptions = {
        timeout: 15000,
      };

      const taskId = client.startTask("test.fn", "payload", invokeOptions);

      // Wait for task to complete
      for await (const _ of client.streamTask(taskId)) {
        // consume events
      }

      const context = JSON.parse(receivedContext);
      expect(context.timeout).toBe(15000);
    });

    test("invoke with complete InvokeOptions", async () => {
      const client = new BasicClient();
      let receivedContext = "";
      const handler = jest.fn().mockImplementation(async (ctx) => {
        receivedContext = ctx;
        return "result";
      });
      client.registerFunction({ id: "test.fn", version: "1.0.0" }, handler);

      const invokeOptions: InvokeOptions = {
        idempotencyKey: "complete-key",
        timeout: 20000,
        headers: {
          "x-request-id": "req-789",
          "x-game-id": "game-123",
        },
        retry: {
          maxAttempts: 5,
          initialDelayMs: 200,
        },
      };

      const result = await client.invoke("test.fn", "payload", invokeOptions);

      expect(result).toBe("result");
      const context = JSON.parse(receivedContext);
      expect(context.idempotency_key).toBe("complete-key");
      expect(context.timeout).toBe(20000);
      expect(context["x-request-id"]).toBe("req-789");
      expect(context["x-game-id"]).toBe("game-123");
    });

    test("isInvokeOptions correctly identifies InvokeOptions with idempotencyKey", async () => {
      const client = new BasicClient();
      const handler = jest.fn().mockResolvedValue("result");
      client.registerFunction({ id: "test.fn", version: "1.0.0" }, handler);

      // InvokeOptions with idempotencyKey
      const options1 = { idempotencyKey: "key-123" };
      const result1 = await client.invoke("test.fn", "payload", options1);
      expect(result1).toBe("result");

      // InvokeOptions with timeout
      const options2 = { timeout: 5000 };
      const result2 = await client.invoke("test.fn", "payload", options2);
      expect(result2).toBe("result");

      // Plain Record<string, string>
      const metadata = { "x-custom": "value" };
      const result3 = await client.invoke("test.fn", "payload", metadata);
      expect(result3).toBe("result");
    });

    test("InvokeOptions without idempotencyKey or timeout still works", async () => {
      const client = new BasicClient();
      const handler = jest.fn().mockResolvedValue("result");
      client.registerFunction({ id: "test.fn", version: "1.0.0" }, handler);

      const invokeOptions = {
        headers: {
          "x-custom": "value",
        },
      };

      const result = await client.invoke("test.fn", "payload", invokeOptions);

      expect(result).toBe("result");
      expect(handler).toHaveBeenCalled();
    });

    test("invoke merges client-level headers, auth token, gameId, and env", async () => {
      const client = new BasicClient({
        authToken: "secret-token",
        gameId: "game-123",
        env: "staging",
        headers: {
          "X-Client": "sdk",
        },
      });
      let receivedContext = "";
      client.registerFunction(
        { id: "test.fn", version: "1.0.0" },
        async (ctx) => {
          receivedContext = ctx;
          return "result";
        },
      );

      const result = await client.invoke("test.fn", "payload", {
        headers: {
          "X-Request-ID": "req-1",
        },
      });

      expect(result).toBe("result");
      const context = JSON.parse(receivedContext);
      expect(context.Authorization).toBe("Bearer secret-token");
      expect(context["X-Client"]).toBe("sdk");
      expect(context["X-Request-ID"]).toBe("req-1");
      expect(context["X-Game-ID"]).toBe("game-123");
      expect(context["X-Env"]).toBe("staging");
    });

    test("per-call headers override client-level metadata defaults", async () => {
      const client = new BasicClient({
        authToken: "secret-token",
        gameId: "game-default",
        env: "development",
        headers: {
          Authorization: "Bearer config-token",
          "X-Game-ID": "game-config",
          "X-Env": "staging",
        },
      });
      let receivedContext = "";
      client.registerFunction(
        { id: "test.fn", version: "1.0.0" },
        async (ctx) => {
          receivedContext = ctx;
          return "result";
        },
      );

      await client.invoke("test.fn", "payload", {
        headers: {
          Authorization: "Bearer override-token",
          "X-Game-ID": "game-override",
          "X-Env": "production",
        },
      });

      const context = JSON.parse(receivedContext);
      expect(context.Authorization).toBe("Bearer override-token");
      expect(context["X-Game-ID"]).toBe("game-override");
      expect(context["X-Env"]).toBe("production");
    });

    test("startTask uses merged client-level metadata", async () => {
      const client = new BasicClient({
        authToken: "job-token",
        gameId: "job-game",
        env: "staging",
        headers: {
          "X-Client": "job-sdk",
        },
      });
      let receivedContext = "";
      client.registerFunction(
        { id: "test.fn", version: "1.0.0" },
        async (ctx) => {
          receivedContext = ctx;
          return "done";
        },
      );

      const taskId = client.startTask("test.fn", "payload", {
        headers: {
          "X-Request-ID": "job-1",
        },
      });

      for await (const _ of client.streamTask(taskId)) {
        // consume events
      }

      const context = JSON.parse(receivedContext);
      expect(context.Authorization).toBe("Bearer job-token");
      expect(context["X-Client"]).toBe("job-sdk");
      expect(context["X-Request-ID"]).toBe("job-1");
      expect(context["X-Game-ID"]).toBe("job-game");
      expect(context["X-Env"]).toBe("staging");
    });
  });

  describe("Configuration Options - controlAddr", () => {
    test("controlAddr can be set", () => {
      const client = new BasicClient({
        controlAddr: "localhost:9090",
      });
      expect((client as any).config.controlAddr).toBe("localhost:9090");
    });

    test("controlAddr defaults to empty string", () => {
      const client = new BasicClient();
      expect((client as any).config.controlAddr).toBe("");
    });
  });

  describe("Configuration Options - agentId and serverName", () => {
    test("agentId can be set", () => {
      const client = new BasicClient({
        agentId: "agent-1",
      });
      expect((client as any).config.agentId).toBe("agent-1");
    });

    test("agentId defaults to empty string", () => {
      const client = new BasicClient();
      expect((client as any).config.agentId).toBe("");
    });

    test("serverName can be set", () => {
      const client = new BasicClient({
        serverName: "agent.example.com",
      });
      expect((client as any).config.serverName).toBe("agent.example.com");
    });

    test("serverName defaults to empty string", () => {
      const client = new BasicClient();
      expect((client as any).config.serverName).toBe("");
    });
  });

  describe("Configuration Options - Logging", () => {
    test("disableLogging can be set to true", () => {
      const client = new BasicClient({
        disableLogging: true,
      });
      expect((client as any).config.disableLogging).toBe(true);
    });

    test("disableLogging defaults to false", () => {
      const client = new BasicClient();
      expect((client as any).config.disableLogging).toBe(false);
    });

    test("debugLogging can be set to true", () => {
      const client = new BasicClient({
        debugLogging: true,
      });
      expect((client as any).config.debugLogging).toBe(true);
    });

    test("debugLogging defaults to false", () => {
      const client = new BasicClient();
      expect((client as any).config.debugLogging).toBe(false);
    });

    test("logLevel can be set to valid values", () => {
      const validLevels: Array<"DEBUG" | "INFO" | "WARN" | "ERROR" | "OFF"> = [
        "DEBUG",
        "INFO",
        "WARN",
        "ERROR",
        "OFF",
      ];

      for (const level of validLevels) {
        const client = new BasicClient({ logLevel: level });
        expect((client as any).config.logLevel).toBe(level);
      }
    });

    test("logLevel defaults to INFO", () => {
      const client = new BasicClient();
      expect((client as any).config.logLevel).toBe("INFO");
    });

    test("all logging options can be set together", () => {
      const client = new BasicClient({
        disableLogging: false,
        debugLogging: true,
        logLevel: "DEBUG",
      });
      expect((client as any).config.disableLogging).toBe(false);
      expect((client as any).config.debugLogging).toBe(true);
      expect((client as any).config.logLevel).toBe("DEBUG");
    });
  });
});
