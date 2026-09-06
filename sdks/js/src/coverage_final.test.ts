/**
 * Final coverage boost: previously uncovered defensive paths in index.ts
 * (capabilities-registration failure logging, unknown inbound dispatch,
 * FilePush unmarshal/staging failures, drain timeout & reconnect recovery,
 * inbound payload validation corners, inbound start-task failure,
 * setFieldWidget guard), openapi.ts x-approval type validation,
 * invoker.ts retry fallthrough and trace.ts non-string field branch.
 */

import * as fs from "node:fs";
import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { createHash } from "node:crypto";
import * as protobuf from "protobufjs";
import { BasicClient, type ClientConfig, type FunctionDescriptor } from "./index";
import { setFieldWidget } from "./index";
import { TCPTransport } from "./tcp_transport";
import { Invoker } from "./invoker";
import { registerFromOpenAPI } from "./openapi";
import { traceParentFromContext, traceIdFromContext } from "./trace";
import {
  MSG_PROVIDER_CONNECT_REQUEST,
  MSG_REGISTER_CAPABILITIES_REQ,
} from "./protocol";

// node:fs exports are non-configurable in modern Node, so a plain
// jest.spyOn cannot redefine writeFileSync. Wrap it in a jest.fn via a
// module mock (default behaviour stays the real implementation).
jest.mock("node:fs", () => {
  const actual =
    jest.requireActual("node:fs") as typeof import("node:fs");
  return { ...actual, writeFileSync: jest.fn(actual.writeFileSync) };
});

// ---------------------------------------------------------------------------
// Shared protobuf helpers
// ---------------------------------------------------------------------------

const helperProto = `
syntax = "proto3";
package croupier.sdk.v1;
message ProviderConnectResponse { string session_id = 1; }
message FilePushRequest {
  string transfer_id = 1;
  string file_name = 2;
  string content_sha256 = 3;
  bytes data = 4;
}
message FilePushResponse {
  string transfer_id = 1;
  bool ok = 2;
  string stored_path = 3;
  string error = 4;
}
message InvokeRequest {
  string function_id = 1;
  string idempotencyKey = 2;
  bytes payload = 3;
  map<string, string> metadata = 4;
}
message InvokeResponse { bytes payload = 1; }
message StartTaskResponse { string task_id = 1; }
`;
const helperRoot = protobuf.parse(helperProto).root;
const ProviderConnectResponseMessage = helperRoot.lookupType(
  "croupier.sdk.v1.ProviderConnectResponse",
);
const FilePushRequestMessage = helperRoot.lookupType(
  "croupier.sdk.v1.FilePushRequest",
);
const FilePushResponseMessage = helperRoot.lookupType(
  "croupier.sdk.v1.FilePushResponse",
);
const InvokeRequestMessage = helperRoot.lookupType(
  "croupier.sdk.v1.InvokeRequest",
);
const InvokeResponseMessage = helperRoot.lookupType(
  "croupier.sdk.v1.InvokeResponse",
);
const StartTaskResponseMessage = helperRoot.lookupType(
  "croupier.sdk.v1.StartTaskResponse",
);

function encodeConnectResponse(sessionId: string): Buffer {
  return Buffer.from(
    ProviderConnectResponseMessage.encode(
      ProviderConnectResponseMessage.create({ sessionId }),
    ).finish(),
  );
}

function makeClient(config: ClientConfig = {}): BasicClient {
  const client = new BasicClient(config);
  client.registerFunction(
    { id: "test.fn", version: "1.0.0" },
    async () => "ok",
  );
  return client;
}

type InboundDispatch = {
  handleInboundRequest: (msgId: number, reqId: number, body: Buffer) => Promise<Buffer>;
};

function inboundOf(client: BasicClient): InboundDispatch {
  return client as unknown as InboundDispatch;
}

afterEach(() => {
  jest.restoreAllMocks();
});

// ---------------------------------------------------------------------------
// index.ts: capabilities registration failure logging (fire-and-forget)
// ---------------------------------------------------------------------------

describe("capabilities registration failure logging", () => {
  it("logs a warning when the control-plane transport fails to connect", async () => {
    let transportConnects = 0;
    const connectSpy = jest
      .spyOn(TCPTransport.prototype, "connect")
      .mockImplementation(async () => {
        transportConnects += 1;
        if (transportConnects > 1) {
          throw new Error("control plane unreachable");
        }
      });
    const callSpy = jest
      .spyOn(TCPTransport.prototype, "call")
      .mockImplementation(async (msgType: number) => {
        if (msgType === MSG_PROVIDER_CONNECT_REQUEST) {
          return [msgType + 1, encodeConnectResponse("sess-cap-fail")];
        }
        return [msgType + 1, Buffer.alloc(0)];
      });
    const closeSpy = jest
      .spyOn(TCPTransport.prototype, "close")
      .mockImplementation(() => {});
    const warnSpy = jest.spyOn(console, "warn").mockImplementation(() => {});

    const client = makeClient({
      controlAddr: "tcp://127.0.0.1:19199",
      autoReconnect: false,
      heartbeatIntervalSeconds: 3600,
    });
    await client.connect();
    // maybeRegisterCapabilities is fire-and-forget; give it a tick to settle.
    await new Promise((r) => setTimeout(r, 50));

    expect(transportConnects).toBeGreaterThanOrEqual(2);
    expect(warnSpy).toHaveBeenCalledWith(
      "Failed to register capabilities:",
      expect.objectContaining({ message: "control plane unreachable" }),
    );

    await client.disconnect();

    connectSpy.mockRestore();
    callSpy.mockRestore();
    closeSpy.mockRestore();
  });

  it("never sends a capabilities request when controlAddr is empty", async () => {
    const connectSpy = jest
      .spyOn(TCPTransport.prototype, "connect")
      .mockImplementation(async () => {});
    const callSpy = jest
      .spyOn(TCPTransport.prototype, "call")
      .mockImplementation(async (msgType: number) => [
        msgType + 1,
        encodeConnectResponse("sess-no-control"),
      ]);
    const closeSpy = jest
      .spyOn(TCPTransport.prototype, "close")
      .mockImplementation(() => {});

    const client = makeClient({ autoReconnect: false, heartbeatIntervalSeconds: 3600 });
    await client.connect();
    await new Promise((r) => setTimeout(r, 20));

    const requestedTypes = (callSpy.mock.calls as unknown as Array<[number]>).map(
      ([type]) => type,
    );
    expect(requestedTypes).not.toContain(MSG_REGISTER_CAPABILITIES_REQ);

    await client.disconnect();

    connectSpy.mockRestore();
    callSpy.mockRestore();
    closeSpy.mockRestore();
  });
});

// ---------------------------------------------------------------------------
// index.ts: unknown inbound message / FilePush failure paths
// ---------------------------------------------------------------------------

describe("inbound dispatch edges", () => {
  it("replies with an empty buffer for unknown inbound message ids", async () => {
    const client = makeClient({ autoReconnect: false });
    const response = await inboundOf(client).handleInboundRequest(
      0x0e0e0e,
      1,
      Buffer.alloc(0),
    );
    expect(Buffer.isBuffer(response)).toBe(true);
    expect(response.length).toBe(0);
  });
});

describe("file push failure paths", () => {
  const staging = mkdtempSync(join(tmpdir(), "croupier-final-push-"));
  const payload = Buffer.from("print('final')");
  const sha256 = createHash("sha256").update(payload).digest("hex");

  function makePushClient(): { client: BasicClient; dispatch: (b: Buffer) => Promise<Buffer> } {
    const config: ClientConfig = {
      autoReconnect: false,
      enableFileTransfer: true,
      fileStagingDir: staging,
    };
    const client = new BasicClient(config);
    client.registerFunction(
      { id: "player.ban", version: "1.0.0" },
      () => "ok",
    );
    return {
      client,
      dispatch: (body: Buffer) =>
        inboundOf(client).handleInboundRequest(0x050109, 1, body),
    };
  }

  function buildPushBody(transferId: string, fileName: string): Buffer {
    return Buffer.from(
      FilePushRequestMessage.encode(
        FilePushRequestMessage.create({
          transferId,
          fileName,
          contentSha256: sha256,
          data: payload,
        }),
      ).finish(),
    );
  }

  function decodePushResponse(response: Buffer): { ok?: boolean; error?: string } {
    return FilePushResponseMessage.toObject(
      FilePushResponseMessage.decode(response),
      { defaults: true },
    ) as { ok?: boolean; error?: string };
  }

  it("rejects a body that fails protobuf unmarshalling", async () => {
    const { dispatch } = makePushClient();
    // Field 4 (bytes) with a truncated length varint -> RangeError on decode.
    const response = await dispatch(Buffer.from([0x22, 0x83]));
    const decoded = decodePushResponse(response);
    expect(decoded.ok).toBe(false);
    expect(decoded.error).toContain("unmarshal FilePushRequest");
    expect(decoded.error).toContain("index out of range");
  });

  it("reports staging write failures without crashing", async () => {
    const writeSpy = jest
      .spyOn(fs, "writeFileSync")
      .mockImplementation(() => {
        throw new Error("EACCES: permission denied");
      });
    const { dispatch } = makePushClient();
    const response = await dispatch(buildPushBody("t-write-fail", "hotfix.lua"));
    const decoded = decodePushResponse(response);
    expect(decoded.ok).toBe(false);
    expect(decoded.error).toContain("write staging file: EACCES: permission denied");
    writeSpy.mockRestore();
  });
});

// ---------------------------------------------------------------------------
// index.ts: drain timeout warning and reconnect recovery
// ---------------------------------------------------------------------------

describe("drain recovery paths", () => {
  it("warns when in-flight calls do not drain within the deadline", async () => {
    const client = makeClient({ autoReconnect: false });
    const warnSpy = jest.spyOn(console, "warn").mockImplementation(() => {});
    (client as unknown as { inflightCalls: number }).inflightCalls = 2;

    // First Date.now() computes the deadline; every later call is past it.
    let nowCalls = 0;
    jest.spyOn(Date, "now").mockImplementation(() =>
      nowCalls++ === 0 ? 1_000 : 32_000,
    );

    const response = (client as unknown as { handleDrainRequest: (b: Buffer) => Buffer })
      .handleDrainRequest(Buffer.alloc(0));
    expect(response.length).toBe(0);
    // With the clock already past the deadline the recovery completes
    // synchronously and clears the draining flag immediately.
    expect((client as unknown as { draining: boolean }).draining).toBe(false);

    await new Promise((r) => setTimeout(r, 50));

    expect(warnSpy).toHaveBeenCalledWith(
      expect.stringContaining("Drain timeout with 2 in-flight call(s)"),
    );
    expect((client as unknown as { draining: boolean }).draining).toBe(false);
    expect((client as unknown as { inflightCalls: number }).inflightCalls).toBe(2);
  });

  it("schedules a reconnect after draining when autoReconnect is enabled", async () => {
    const connectSpy = jest
      .spyOn(TCPTransport.prototype, "connect")
      .mockImplementation(async () => {});
    const callSpy = jest
      .spyOn(TCPTransport.prototype, "call")
      .mockImplementation(async (msgType: number) => [
        msgType + 1,
        encodeConnectResponse("sess-drain-reconnect"),
      ]);
    const closeSpy = jest
      .spyOn(TCPTransport.prototype, "close")
      .mockImplementation(() => {});

    const client = makeClient({
      autoReconnect: true,
      reconnect: {
        enabled: true,
        maxAttempts: 2,
        initialDelayMs: 1,
        maxDelayMs: 1,
        backoffMultiplier: 1,
        jitterFactor: 0,
      },
    });

    const response = (client as unknown as { handleDrainRequest: (b: Buffer) => Buffer })
      .handleDrainRequest(Buffer.alloc(0));
    expect(response.length).toBe(0);

    await new Promise((r) => setTimeout(r, 100));

    expect((client as unknown as { sessionId: string }).sessionId).toBe(
      "sess-drain-reconnect",
    );
    expect((client as unknown as { draining: boolean }).draining).toBe(false);

    await client.disconnect();

    connectSpy.mockRestore();
    callSpy.mockRestore();
    closeSpy.mockRestore();
  });
});

// ---------------------------------------------------------------------------
// index.ts: inbound payload validation corners
// ---------------------------------------------------------------------------

function encodeInvoke(functionId: string, payload: string): Buffer {
  return Buffer.from(
    InvokeRequestMessage.encode(
      InvokeRequestMessage.create({
        functionId,
        payload: Buffer.from(payload),
      }),
    ).finish(),
  );
}

function decodeInvokePayload(response: Buffer): string {
  const decoded = InvokeResponseMessage.toObject(
    InvokeResponseMessage.decode(response),
    { defaults: true },
  ) as { payload?: Uint8Array };
  return new TextDecoder().decode(decoded.payload ?? new Uint8Array());
}

async function dispatchInboundInvoke(
  client: BasicClient,
  functionId: string,
  payload: string,
): Promise<string> {
  const response = await (
    client as unknown as { handleInboundInvoke: (b: Buffer) => Promise<Buffer> }
  ).handleInboundInvoke(encodeInvoke(functionId, payload));
  return decodeInvokePayload(response);
}

describe("inbound payload validation corners", () => {
  it("skips validation when the schema fails to compile", async () => {
    const handler = jest.fn(() => "compiled-no-more");
    const client = new BasicClient({
      autoReconnect: false,
      validateInputPayloads: true,
    });
    const descriptor: FunctionDescriptor = {
      id: "bad.schema",
      version: "1.0.0",
      inputSchema: {
        type: "object",
        properties: { x: { type: "banana" } },
      } as Record<string, unknown>,
    };
    client.registerFunction(descriptor, handler);

    const response = await dispatchInboundInvoke(client, "bad.schema", "{}");
    expect(response).toBe("compiled-no-more");
    expect(handler).toHaveBeenCalledTimes(1);

    // Second hit uses the cached null validator (still skipped).
    const again = await dispatchInboundInvoke(client, "bad.schema", '{"x":1}');
    expect(again).toBe("compiled-no-more");
    expect(handler).toHaveBeenCalledTimes(2);
  });

  it("rejects payloads that are not valid JSON", async () => {
    const handler = jest.fn(() => "never");
    const client = new BasicClient({
      autoReconnect: false,
      validateInputPayloads: true,
    });
    client.registerFunction(
      {
        id: "json.fn",
        version: "1.0.0",
        inputSchema: { type: "object" },
      },
      handler,
    );

    const response = await dispatchInboundInvoke(client, "json.fn", "not-json{{");
    const parsed = JSON.parse(response) as { error?: string };
    expect(parsed.error).toContain("payload must be valid JSON");
    expect(handler).not.toHaveBeenCalled();
  });

  it("answers inbound StartTask with an empty task id when starting fails", async () => {
    const client = new BasicClient({ autoReconnect: false });
    client.registerFunction(
      { id: "task.fn", version: "1.0.0" },
      () => "ok",
    );
    jest.spyOn(client, "startTask").mockImplementation(() => {
      throw new Error("task slot exhausted");
    });

    const response = await inboundOf(client).handleInboundRequest(
      0x030103, // MSG_START_TASK_REQUEST
      9,
      encodeInvoke("task.fn", "x"),
    );
    const decoded = StartTaskResponseMessage.toObject(
      StartTaskResponseMessage.decode(response),
      { defaults: true },
    ) as { taskId?: string };
    expect(decoded.taskId ?? "").toBe("");
  });
});

// ---------------------------------------------------------------------------
// index.ts: setFieldWidget guard
// ---------------------------------------------------------------------------

describe("setFieldWidget guard", () => {
  it("rejects a blank widget", () => {
    expect(() =>
      setFieldWidget({ id: "f", version: "1.0.0" }, "field", "   "),
    ).toThrow("widget is required for setFieldWidget");
  });
});

// ---------------------------------------------------------------------------
// openapi.ts: x-approval type validation
// ---------------------------------------------------------------------------

describe("openapi x-approval type validation", () => {
  const spec = (approval: unknown) => ({
    paths: {
      "/x": {
        post: {
          operationId: "x_post",
          "x-approval": approval,
          responses: { 200: { description: "ok" } },
        },
      },
    },
  });
  const handler = (_ctx: string, _payload: string) => "{}";

  it("rejects a non-boolean x-approval.required", () => {
    const client = new BasicClient();
    expect(() =>
      registerFromOpenAPI(
        client,
        spec({ required: "yes", policyKey: "pk" }),
        undefined,
        () => handler,
      ),
    ).toThrow("x-approval.required for x_post must be a boolean");
  });

  it("rejects a non-string x-approval.policyKey", () => {
    const client = new BasicClient();
    expect(() =>
      registerFromOpenAPI(
        client,
        spec({ required: true, policyKey: 42 }),
        undefined,
        () => handler,
      ),
    ).toThrow("x-approval.policyKey for x_post must be a string");
  });
});

// ---------------------------------------------------------------------------
// invoker.ts: retry fallthrough with a NaN maxAttempts
// ---------------------------------------------------------------------------

describe("invoker retry fallthrough", () => {
  it("rejects immediately when maxAttempts is NaN without any fetch", async () => {
    const originalFetch = globalThis.fetch;
    const fetchSpy = jest.fn();
    globalThis.fetch = fetchSpy as unknown as typeof fetch;
    try {
      const invoker = new Invoker({ baseUrl: "http://s:18780" });
      let caught: unknown;
      let returned = false;
      try {
        await invoker.invoke("fn", {}, { retry: { maxAttempts: Number.NaN } });
        returned = true;
      } catch (error) {
        caught = error;
      }
      expect(returned).toBe(false);
      expect(caught).toBeUndefined();
      expect(fetchSpy).not.toHaveBeenCalled();
    } finally {
      globalThis.fetch = originalFetch;
    }
  });
});

// ---------------------------------------------------------------------------
// trace.ts: non-string trace field values
// ---------------------------------------------------------------------------

describe("trace helper non-string values", () => {
  it("returns an empty string when trace fields are not strings", () => {
    const ctx = JSON.stringify({ traceparent: 1234, traceId: { deep: true } });
    expect(traceParentFromContext(ctx)).toBe("");
    expect(traceIdFromContext(ctx)).toBe("");
  });

  it("trims whitespace around string trace fields", () => {
    const ctx = JSON.stringify({ traceparent: "  00-abc-def-01  ", traceId: " abc " });
    expect(traceParentFromContext(ctx)).toBe("00-abc-def-01");
    expect(traceIdFromContext(ctx)).toBe("abc");
  });
});
