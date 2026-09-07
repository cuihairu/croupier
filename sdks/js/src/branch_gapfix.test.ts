/**
 * Branch coverage gap fixes (Branch 91.39% -> >=98%).
 *
 * Each describe block targets specific uncovered branch paths in
 * src/index.ts, src/invoker.ts and src/openapi.ts. Fault injection is used
 * only for defensive branches that cannot be triggered through public
 * library behavior (Ajv always provides an errors array; JSON.parse only
 * throws SyntaxError); each of those tests documents the reason inline.
 *
 * Structurally unreachable defensive branches intentionally NOT covered:
 * - tcp_transport.ts:194  `if (!socket)` inside onConnect — `socket` is
 *   assigned synchronously inside the Promise executor before `once("connect")`
 *   is attached, and Node emits "connect" only after the current tick, so the
 *   null case can never fire.
 * - index.ts:1465-1468/1566/1567 `decoded.x ?? default` — decode uses
   protobufjs `toObject({defaults:true})` which always materializes every
 *   field (string -> "", bytes -> empty buffer, map -> {}), so the nullish
 *   side is dead code.
 * - openapi.ts:276 `if (!isRecord(paths))` — its only caller
 *   (registerFromOpenAPI) pre-validates `isRecord(document.paths)` and throws
 *   earlier, so iterOperations can never receive a non-record paths value.
 */

import { Buffer } from "node:buffer";
import * as fs from "node:fs";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { createHash } from "node:crypto";
import { runInNewContext } from "node:vm";
import * as protobuf from "protobufjs";
import {
  BasicClient,
  setFieldHint,
  type ClientConfig,
  type FunctionDescriptor,
  type FunctionHandler,
} from "./index";
import { TCPTransport } from "./tcp_transport";
import { MSG_PROVIDER_CONNECT_REQUEST } from "./protocol";
import { Invoker, InvokerError } from "./invoker";
import { registerFromOpenAPI, type RegistrationTarget } from "./openapi";
import type { ValidateFunction } from "ajv";

// node:fs exports are non-configurable in modern Node, so a plain jest.spyOn
// cannot redefine writeFileSync. Wrap it in a jest.fn via a module mock
// (default behaviour stays the real implementation) — same pattern as
// coverage_final.test.ts. Needed to make the staging write throw a non-Error.
jest.mock("node:fs", () => {
  const actual = jest.requireActual("node:fs") as typeof import("node:fs");
  return { ...actual, writeFileSync: jest.fn(actual.writeFileSync) };
});

// ---------------------------------------------------------------------------
// Shared protobuf helpers (mirror of the wire messages in index.ts)
// ---------------------------------------------------------------------------

const helperProto = `
syntax = "proto3";
package croupier.sdk.v1;
message ProviderConnectRequest {
  string service_id = 1;
  string version = 2;
  string sdk_language = 4;
  string transport_security_mode = 9;
}
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
`;
const helperRoot = protobuf.parse(helperProto).root;
const ProviderConnectRequestMessage = helperRoot.lookupType(
  "croupier.sdk.v1.ProviderConnectRequest",
);
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

type DecodedPushResponse = {
  ok?: boolean;
  error?: string;
  storedPath?: string;
};
type InboundDispatch = {
  handleInboundRequest: (msgId: number, reqId: number, body: Buffer) => Promise<Buffer>;
};

function inboundOf(client: BasicClient): InboundDispatch {
  return client as unknown as InboundDispatch;
}

function decodeInvokePayload(response: Buffer): string {
  const decoded = InvokeResponseMessage.toObject(
    InvokeResponseMessage.decode(response),
    { defaults: true },
  ) as { payload?: Uint8Array };
  return new TextDecoder().decode(decoded.payload ?? new Uint8Array());
}

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

afterEach(() => {
  jest.restoreAllMocks();
});

// ---------------------------------------------------------------------------
// index.ts getFunctionDescriptor: schema-less descriptors (789-794 falsy side)
// ---------------------------------------------------------------------------

describe("getFunctionDescriptor schema mapping", () => {
  it("serializes declared schemas to JSON strings", () => {
    const client = new BasicClient();
    client.registerFunction(
      {
        id: "with.schemas",
        version: "1.0.0",
        inputSchema: { type: "object", properties: { a: { type: "string" } } },
        outputSchema: { type: "object" },
      },
      () => "ok",
    );
    const descriptor = client.getFunctionDescriptor("with.schemas");
    expect(descriptor).toBeDefined();
    expect(JSON.parse(descriptor?.inputSchema ?? "")).toMatchObject({
      type: "object",
    });
    expect(JSON.parse(descriptor?.outputSchema ?? "")).toEqual({
      type: "object",
    });
  });

  it("returns undefined schemas for schema-less descriptors", () => {
    const client = new BasicClient();
    client.registerFunction({ id: "bare.fn", version: "1.0.0" }, () => "ok");
    const descriptor = client.getFunctionDescriptor("bare.fn");
    expect(descriptor).toBeDefined();
    expect(descriptor?.inputSchema).toBeUndefined();
    expect(descriptor?.outputSchema).toBeUndefined();
  });
});

// ---------------------------------------------------------------------------
// index.ts startTask: nullish handler result and non-Error rejections
// (953 `result ?? ""`, 959 `instanceof Error ? : "Handler failed"`)
// ---------------------------------------------------------------------------

describe("startTask handler edge results", () => {
  type TaskEventShape = {
    type: string;
    message?: string;
    progress?: number;
    payload?: Uint8Array;
  };

  async function collectEvents(client: BasicClient, taskId: string): Promise<TaskEventShape[]> {
    const events: TaskEventShape[] = [];
    const stream = client.streamTask(taskId) as AsyncIterable<TaskEventShape>;
    for await (const event of stream) {
      events.push(event);
    }
    return events;
  }

  it("encodes an empty payload when the handler resolves without a value", async () => {
    const client = new BasicClient({ autoReconnect: false });
    const handler: FunctionHandler = async () => undefined as unknown as string;
    client.registerFunction({ id: "task.empty", version: "1.0.0" }, handler);

    const taskId = client.startTask("task.empty", "{}");
    const events = await collectEvents(client, taskId);

    expect(events.map((event) => event.type)).toEqual(["started", "completed"]);
    expect(events[1].progress).toBe(100);
    expect(new TextDecoder().decode(events[1].payload ?? new Uint8Array())).toBe("");
  });

  it("reports 'Handler failed' when the handler rejects with a non-Error", async () => {
    const client = new BasicClient({ autoReconnect: false });
    const handler: FunctionHandler = () => Promise.reject("kaput") as unknown as Promise<string>;
    client.registerFunction({ id: "task.nonerror", version: "1.0.0" }, handler);

    const taskId = client.startTask("task.nonerror", "{}");
    const events = await collectEvents(client, taskId);

    expect(events.map((event) => event.type)).toEqual(["started", "error"]);
    expect(events[1].message).toBe("Handler failed");
    expect(events[1].progress).toBe(0);
  });
});

// ---------------------------------------------------------------------------
// index.ts uploadFile: cross-realm Uint8Array content (1179) and
// extension-less file names (1281 `<none>`)
// ---------------------------------------------------------------------------

describe("uploadFile content and extension edges", () => {
  it("copies cross-realm Uint8Array content that fails the instanceof check", async () => {
    const crossRealm = runInNewContext("new Uint8Array([104, 105])") as Uint8Array;
    // The whole point of this input: a real Uint8Array from another context
    // whose `instanceof Uint8Array` is false, so the copy branch runs.
    expect(crossRealm instanceof Uint8Array).toBe(false);

    const client = new BasicClient({ enableFileTransfer: true });
    const result = await client.uploadFile({
      filePath: "functions/greeting.bin",
      content: crossRealm,
    });

    expect(result.size).toBe(2);
    expect(result.sha256).toBe(
      createHash("sha256").update(Buffer.from([104, 105])).digest("hex"),
    );
  });

  it("reports `<none>` for files without an extension when extensions are restricted", async () => {
    const client = new BasicClient({
      enableFileTransfer: true,
      allowedExtensions: [".js"],
    });
    await expect(
      client.uploadFile({ filePath: "config", content: "x" }),
    ).rejects.toThrow("File extension <none> is not allowed.");
  });
});

// ---------------------------------------------------------------------------
// index.ts FilePush: missing-field rejections, config fallbacks (1474/1486)
// and non-Error staging write failure (1502)
// ---------------------------------------------------------------------------

describe("file push field validation", () => {
  const staging = mkdtempSync(join(tmpdir(), "croupier-gapfix-push-"));
  const payload = Buffer.from("print('gapfix')");
  const sha256 = createHash("sha256").update(payload).digest("hex");

  function pushClient(override: Partial<ClientConfig> = {}) {
    const client = new BasicClient({
      autoReconnect: false,
      enableFileTransfer: true,
      fileStagingDir: staging,
      ...override,
    });
    client.registerFunction({ id: "player.ban", version: "1.0.0" }, () => "ok");
    return {
      client,
      push: (body: Buffer) =>
        inboundOf(client).handleInboundRequest(0x050109, 1, body),
    };
  }

  function pushBody(fields: {
    transferId?: string;
    fileName?: string;
    contentSha256?: string;
    data?: Buffer;
  }): Buffer {
    return Buffer.from(
      FilePushRequestMessage.encode(FilePushRequestMessage.create(fields)).finish(),
    );
  }

  function decode(response: Buffer): DecodedPushResponse {
    return FilePushResponseMessage.toObject(
      FilePushResponseMessage.decode(response),
      { defaults: true },
    ) as DecodedPushResponse;
  }

  it("rejects a push without transferId", async () => {
    const { push } = pushClient();
    const decoded = decode(
      await push(pushBody({ fileName: "a.lua", data: payload, contentSha256: sha256 })),
    );
    expect(decoded.ok).toBe(false);
    expect(decoded.error).toBe("transferId is required");
  });

  it("rejects a push without a file name (bare basename rule)", async () => {
    const { push } = pushClient();
    const decoded = decode(
      await push(pushBody({ transferId: "t-1", data: payload, contentSha256: sha256 })),
    );
    expect(decoded.ok).toBe(false);
    expect(decoded.error).toContain("bare basename");
  });

  it("rejects a push with empty data", async () => {
    const { push } = pushClient();
    const decoded = decode(
      await push(pushBody({ transferId: "t-2", fileName: "b.lua", contentSha256: sha256 })),
    );
    expect(decoded.ok).toBe(false);
    expect(decoded.error).toBe("file payload is empty");
  });

  it("rejects a push without a checksum", async () => {
    const { push } = pushClient();
    const decoded = decode(
      await push(pushBody({ transferId: "t-3", fileName: "c.lua", data: payload })),
    );
    expect(decoded.ok).toBe(false);
    expect(decoded.error).toBe("contentSha256 is required");
  });

  it("falls back to the built-in 10MB limit when maxFileSize is unset", async () => {
    const { push } = pushClient({ maxFileSize: undefined });
    const big = Buffer.alloc(10 * 1024 * 1024 + 1, 1);
    const decoded = decode(
      await push(pushBody({ transferId: "t-4", fileName: "d.bin", data: big })),
    );
    expect(decoded.ok).toBe(false);
    // 10 * 1024 * 1024 === 10485760 proves the `?? 10 * 1024 * 1024` default
    // was used rather than a configured value.
    expect(decoded.error).toContain("exceeds max 10485760");
  });

  it("falls back to ./croupier-staging when fileStagingDir is unset", async () => {
    const workingDir = mkdtempSync(join(tmpdir(), "croupier-gapfix-cwd-"));
    const originalCwd = process.cwd();
    process.chdir(workingDir);
    try {
      const { push } = pushClient({
        fileStagingDir: undefined,
        maxFileSize: 1000000,
      });
      const decoded = decode(
        await push(
          pushBody({
            transferId: "t-5",
            fileName: "e.lua",
            data: payload,
            contentSha256: sha256,
          }),
        ),
      );
      expect(decoded.ok).toBe(true);
      // path.join normalizes "./croupier-staging" into a relative path, which
      // resolves against the chdir'ed working directory.
      expect(decoded.storedPath).toBe(join("croupier-staging", "e.lua"));
      const storedAbsolute = join(workingDir, "croupier-staging", "e.lua");
      expect(fs.existsSync(storedAbsolute)).toBe(true);
      expect(fs.readFileSync(storedAbsolute).toString()).toBe("print('gapfix')");
    } finally {
      process.chdir(originalCwd);
      rmSync(workingDir, { recursive: true, force: true });
    }
  });

  it("stringifies non-Error staging write failures", async () => {
    // Fault injection: node:fs is module-mocked above; make the next
    // writeFileSync throw a plain string. protobuf decode errors are always
    // Error instances, but the staging write calls into user-land FS code
    // where non-Error throwables are possible (e.g. EMFILE from a wrapper).
    (fs.writeFileSync as unknown as ReturnType<typeof jest.fn>).mockImplementationOnce(
      () => {
        throw "EIO: disk on fire";
      },
    );
    const { push } = pushClient();
    const decoded = decode(
      await push(
        pushBody({
          transferId: "t-6",
          fileName: "f.lua",
          data: payload,
          contentSha256: sha256,
        }),
      ),
    );
    expect(decoded.ok).toBe(false);
    expect(decoded.error).toBe("write staging file: EIO: disk on fire");
  });
});

// ---------------------------------------------------------------------------
// index.ts decodeInvokeRequest: falsy functionId (1564 `|| ""`)
// ---------------------------------------------------------------------------

describe("inbound invoke with missing function id", () => {
  it("answers with an empty payload for an unknown function", async () => {
    const client = new BasicClient({ autoReconnect: false });
    client.registerFunction({ id: "known.fn", version: "1.0.0" }, () => "ok");

    const response = await inboundOf(client).handleInboundRequest(
      0x030101,
      1,
      encodeInvoke("", "{}"),
    );
    // Unknown function -> empty payload so the agent fails over.
    expect(decodeInvokePayload(response)).toBe("");
  });
});

// ---------------------------------------------------------------------------
// index.ts validateInboundPayload: schema-missing / schema-not-object (1602)
// and empty payload defaulting to "{}" (1615)
// ---------------------------------------------------------------------------

describe("inbound payload validation fallbacks", () => {
  async function dispatchInbound(
    client: BasicClient,
    functionId: string,
    payload: string,
  ): Promise<string> {
    const response = await (
      client as unknown as {
        handleInboundInvoke: (body: Buffer) => Promise<Buffer>;
      }
    ).handleInboundInvoke(encodeInvoke(functionId, payload));
    return decodeInvokePayload(response);
  }

  it("skips validation when the function declares no input schema", async () => {
    const handler = jest.fn(() => "no-schema-ok");
    const client = new BasicClient({
      autoReconnect: false,
      validateInputPayloads: true,
    });
    client.registerFunction({ id: "noschema.fn", version: "1.0.0" }, handler);

    expect(await dispatchInbound(client, "noschema.fn", '{"a":1}')).toBe(
      "no-schema-ok",
    );
    expect(handler).toHaveBeenCalledTimes(1);
  });

  it("skips validation when the declared schema is not an object", async () => {
    const handler = jest.fn(() => "weird-schema-ok");
    const client = new BasicClient({
      autoReconnect: false,
      validateInputPayloads: true,
    });
    client.registerFunction(
      {
        id: "weirdschema.fn",
        version: "1.0.0",
        inputSchema: "not-a-schema" as unknown as Record<string, unknown>,
      },
      handler,
    );

    expect(await dispatchInbound(client, "weirdschema.fn", '{"a":1}')).toBe(
      "weird-schema-ok",
    );
    expect(handler).toHaveBeenCalledTimes(1);
  });

  it("validates an empty payload against the default empty object", async () => {
    const handler = jest.fn(() => "empty-payload-ok");
    const client = new BasicClient({
      autoReconnect: false,
      validateInputPayloads: true,
    });
    client.registerFunction(
      {
        id: "emptypayload.fn",
        version: "1.0.0",
        inputSchema: { type: "object" },
      },
      handler,
    );

    expect(await dispatchInbound(client, "emptypayload.fn", "")).toBe(
      "empty-payload-ok",
    );
    expect(handler).toHaveBeenCalledWith(expect.any(String), "");
  });

  it("stringifies non-Error JSON.parse failures (fault injection)", async () => {
    // JSON.parse only ever throws SyntaxError (an Error), so the
    // `String(error)` arm of index.ts:1618 is unreachable through public
    // behaviour. Inject a single non-Error throw to prove the error payload
    // still renders a readable message instead of "undefined".
    const handler = jest.fn(() => "never");
    const client = new BasicClient({
      autoReconnect: false,
      validateInputPayloads: true,
    });
    client.registerFunction(
      { id: "jsonparse.fn", version: "1.0.0", inputSchema: { type: "object" } },
      handler,
    );

    const parseSpy = jest
      .spyOn(JSON, "parse")
      .mockImplementationOnce(() => {
        throw "bad input stream";
      });
    const response = await (
      client as unknown as {
        handleInboundInvoke: (body: Buffer) => Promise<Buffer>;
      }
    ).handleInboundInvoke(encodeInvoke("jsonparse.fn", "x"));

    const parsed = JSON.parse(decodeInvokePayload(response)) as { error?: string };
    expect(parsed.error).toBe("payload must be valid JSON: bad input stream");
    expect(handler).not.toHaveBeenCalled();
    parseSpy.mockRestore();
  });

  it("tolerates validators that fail without error details (fault injection)", async () => {
    // Ajv compiled validators always set `errors` to an array when validation
    // fails, so the `?? []` / per-item fallback arms of index.ts:1623-1624 are
    // defensive only. Inject a bare failing validator (returns false, errors
    // undefined) and one whose error items lack instancePath/message to prove
    // the error payload degrades gracefully to "/" and "".
    const handler = jest.fn(() => "never");
    const client = new BasicClient({
      autoReconnect: false,
      validateInputPayloads: true,
    });
    client.registerFunction(
      { id: "silentvalidator.fn", version: "1.0.0", inputSchema: { type: "object" } },
      handler,
    );

    const cache = (
      client as unknown as {
        inboundSchemaCache: Map<string, ValidateFunction | null>;
      }
    ).inboundSchemaCache;
    const schemaKey = JSON.stringify({ type: "object" });
    cache.set(schemaKey, null);
    // First dispatch compiles-and-caches is bypassed by seeding the cache
    // with a validator that fails with no errors array at all.
    const silentValidator = Object.assign(() => false, {}) as unknown as ValidateFunction;
    cache.set(schemaKey, silentValidator);
    let response = await (
      client as unknown as {
        handleInboundInvoke: (body: Buffer) => Promise<Buffer>;
      }
    ).handleInboundInvoke(encodeInvoke("silentvalidator.fn", "{}"));
    let parsed = JSON.parse(decodeInvokePayload(response)) as { error?: string };
    expect(parsed.error).toBe("payload validation failed: ");
    expect(handler).not.toHaveBeenCalled();

    // Second shape: errors present but items missing instancePath/message.
    const sparseValidator = Object.assign(() => false, {
      errors: [{}],
    }) as unknown as ValidateFunction;
    cache.set(schemaKey, sparseValidator);
    response = await (
      client as unknown as {
        handleInboundInvoke: (body: Buffer) => Promise<Buffer>;
      }
    ).handleInboundInvoke(encodeInvoke("silentvalidator.fn", "{}"));
    parsed = JSON.parse(decodeInvokePayload(response)) as { error?: string };
    // "/ " + "" is trimmed to "/" by the message builder.
    expect(parsed.error).toBe("payload validation failed: /");
  });
});

// ---------------------------------------------------------------------------
// index.ts invokeInbound: undefined handler result (1650 `result ?? ""`) and
// non-Error handler rejection (1652)
// ---------------------------------------------------------------------------

describe("inbound invoke handler edge results", () => {
  it("encodes an empty payload when the handler resolves undefined", async () => {
    const client = new BasicClient({ autoReconnect: false });
    const handler: FunctionHandler = async () => undefined as unknown as string;
    client.registerFunction({ id: "undef.fn", version: "1.0.0" }, handler);

    const response = await (
      client as unknown as {
        handleInboundInvoke: (body: Buffer) => Promise<Buffer>;
      }
    ).handleInboundInvoke(encodeInvoke("undef.fn", "{}"));
    expect(decodeInvokePayload(response)).toBe("");
  });

  it("reports 'Handler failed' when the handler rejects with a non-Error", async () => {
    const client = new BasicClient({ autoReconnect: false });
    const handler: FunctionHandler = () => Promise.reject("boom") as unknown as Promise<string>;
    client.registerFunction({ id: "str.fn", version: "1.0.0" }, handler);

    const response = await (
      client as unknown as {
        handleInboundInvoke: (body: Buffer) => Promise<Buffer>;
      }
    ).handleInboundInvoke(encodeInvoke("str.fn", "{}"));
    const parsed = JSON.parse(decodeInvokePayload(response)) as { error?: string };
    expect(parsed.error).toBe("Handler failed");
  });
});

// ---------------------------------------------------------------------------
// index.ts reconnectLoop: `maxAttempts ?? 0` fallback (1757)
// ---------------------------------------------------------------------------

describe("reconnect loop with undefined maxAttempts", () => {
  it("keeps retrying past any attempt cap until disconnect", async () => {
    const connectSpy = jest
      .spyOn(TCPTransport.prototype, "connect")
      .mockImplementation(async () => {
        throw new Error("agent unreachable");
      });
    const closeSpy = jest
      .spyOn(TCPTransport.prototype, "close")
      .mockImplementation(() => {});

    const client = new BasicClient({
      autoReconnect: false,
      // maxAttempts: undefined must fall back to 0 ("endless with backoff"),
      // otherwise the loop would throw "Max reconnect attempts reached".
      reconnect: {
        enabled: true,
        maxAttempts: undefined,
        initialDelayMs: 1,
        maxDelayMs: 2,
        backoffMultiplier: 1,
        jitterFactor: 0,
      },
    });
    client.registerFunction({ id: "re.fn", version: "1.0.0" }, () => "ok");

    const loop = (
      client as unknown as { reconnectLoop: () => Promise<void> }
    ).reconnectLoop();

    await new Promise((resolve) => setTimeout(resolve, 40));
    const attemptsWhileRunning = connectSpy.mock.calls.length;
    expect(attemptsWhileRunning).toBeGreaterThanOrEqual(3);

    await client.disconnect();
    await loop;
    expect(connectSpy.mock.calls.length).toBeGreaterThanOrEqual(
      attemptsWhileRunning,
    );

    connectSpy.mockRestore();
    closeSpy.mockRestore();
  });
});

// ---------------------------------------------------------------------------
// index.ts ProviderConnectRequest serialization: providerLang fallback
// (1811 `|| "javascript"`) and TLS security mode (1815 "tls" side)
// ---------------------------------------------------------------------------

describe("provider connect request flags", () => {
  it("defaults sdkLanguage to javascript and reports tls when not insecure", async () => {
    const registerBodies: Buffer[] = [];
    const connectSpy = jest
      .spyOn(TCPTransport.prototype, "connect")
      .mockImplementation(async () => {});
    const callSpy = jest
      .spyOn(TCPTransport.prototype, "call")
      .mockImplementation(async (msgType: number, data: Buffer) => {
        if (msgType === MSG_PROVIDER_CONNECT_REQUEST) {
          registerBodies.push(Buffer.from(data));
          return [
            msgType + 1,
            Buffer.from(
              ProviderConnectResponseMessage.encode(
                ProviderConnectResponseMessage.create({ sessionId: "sess-gapfix" }),
              ).finish(),
            ),
          ];
        }
        return [msgType + 1, Buffer.alloc(0)];
      });
    const closeSpy = jest
      .spyOn(TCPTransport.prototype, "close")
      .mockImplementation(() => {});

    const client = new BasicClient({
      autoReconnect: false,
      heartbeatIntervalSeconds: 3600,
      providerLang: "",
      insecure: false,
    });
    client.registerFunction({ id: "flags.fn", version: "1.0.0" }, () => "ok");

    await client.connect();

    expect(registerBodies.length).toBe(1);
    const request = ProviderConnectRequestMessage.toObject(
      ProviderConnectRequestMessage.decode(registerBodies[0]),
      { defaults: true },
    ) as { sdkLanguage?: string; transportSecurityMode?: string };
    expect(request.sdkLanguage).toBe("javascript");
    expect(request.transportSecurityMode).toBe("tls");
    expect((client as unknown as { sessionId: string }).sessionId).toBe(
      "sess-gapfix",
    );

    await client.disconnect();
    connectSpy.mockRestore();
    callSpy.mockRestore();
    closeSpy.mockRestore();
  });
});

// ---------------------------------------------------------------------------
// index.ts parseProviderConnectResponse: JSON body without session_id (1842)
// ---------------------------------------------------------------------------

describe("parseProviderConnectResponse JSON fallback", () => {
  it("returns an empty session id when the JSON body lacks session_id", () => {
    const client = new BasicClient();
    const parsed = (
      client as unknown as {
        parseProviderConnectResponse: (data: Buffer) => { sessionId: string };
      }
    ).parseProviderConnectResponse(Buffer.from('{"ok":true}'));
    expect(parsed.sessionId).toBe("");
  });
});

// ---------------------------------------------------------------------------
// index.ts normalizeHintKey: blank hint (1957) and x_ underscore alias (1959)
// ---------------------------------------------------------------------------

describe("setFieldHint key normalization", () => {
  const base: FunctionDescriptor = { id: "hint.fn", version: "1.0.0" };

  it("rejects a hint that is only whitespace", () => {
    expect(() => setFieldHint(base, "level", "   ", "slider")).toThrow(
      'hint "   " must be an x- extension key (e.g. x-widget)',
    );
  });

  it("normalizes an x_ prefixed hint to the x- canonical form", () => {
    const withHint = setFieldHint(base, "level", "x_widget", "slider");
    const schema = withHint.inputSchema as Record<string, unknown>;
    const properties = schema.properties as Record<string, Record<string, unknown>>;
    expect(properties.level["x-widget"]).toBe("slider");
    // Original descriptor is not mutated.
    expect(base.inputSchema).toBeUndefined();
  });
});

// ---------------------------------------------------------------------------
// invoker.ts mergeRetry clamps (81, 82)
// ---------------------------------------------------------------------------

type FetchImpl = typeof fetch;

function withMockedFetch(
  responder: (url: string, init?: RequestInit) => Promise<Response>,
): { fetchSpy: jest.Mock; restore: () => void } {
  const original = globalThis.fetch;
  const fetchSpy = jest.fn(((url: string | URL | Request, init?: RequestInit) =>
    responder(url.toString(), init)) as unknown as FetchImpl);
  globalThis.fetch = fetchSpy as unknown as FetchImpl;
  return { fetchSpy, restore: () => { globalThis.fetch = original; } };
}

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

describe("invoker retry config clamping", () => {
  it("clamps maxAttempts below 1 to a single attempt", async () => {
    const { fetchSpy, restore } = withMockedFetch(async () =>
      jsonResponse({ error: "internal", message: "boom" }, 500),
    );
    try {
      const invoker = new Invoker({
        baseUrl: "https://h/api/v1",
        retry: { maxAttempts: 0, initialDelayMs: 1, jitterFactor: 0 },
      });
      await expect(invoker.invoke("fn", {})).rejects.toMatchObject({
        name: "InvokerError",
        status: 500,
      });
      expect(fetchSpy).toHaveBeenCalledTimes(1);
    } finally {
      restore();
    }
  });

  it("clamps a non-positive backoffMultiplier and still retries with backoff", async () => {
    let calls = 0;
    const { fetchSpy, restore } = withMockedFetch(async () => {
      calls += 1;
      if (calls === 1) return jsonResponse({ message: "transient" }, 502);
      return jsonResponse({ result: "recovered" });
    });
    try {
      const invoker = new Invoker({
        baseUrl: "https://h/api/v1",
        retry: {
          maxAttempts: 2,
          backoffMultiplier: 0,
          initialDelayMs: 1,
          jitterFactor: 0,
        },
      });
      await expect(invoker.invoke("fn", {})).resolves.toMatchObject({
        payload: "recovered",
      });
      expect(fetchSpy).toHaveBeenCalledTimes(2);
    } finally {
      restore();
    }
  });
});

// ---------------------------------------------------------------------------
// invoker.ts: status-0 InvokerError is retryable (97) and schema_validation
// errors from the operation are rethrown untouched (339)
// ---------------------------------------------------------------------------

describe("invoker status-0 retry and schema_validation passthrough", () => {
  it("retries status-0 InvokerErrors and rethrows the same error instance", async () => {
    // Fault injection: the operation rejects with the SDK's own InvokerError
    // (status 0, schema_validation). This matches the class of failures the
    // retry layer must treat as network-level (status 0) while invoke()
    // guarantees schema_validation errors surface untouched.
    const synthetic = new InvokerError("synthetic schema failure", 0, "schema_validation");
    const { fetchSpy, restore } = withMockedFetch(() => Promise.reject(synthetic));
    try {
      const invoker = new Invoker({
        baseUrl: "https://h/api/v1",
        retry: { maxAttempts: 2, initialDelayMs: 1, jitterFactor: 0 },
      });
      await expect(invoker.invoke("fn", {})).rejects.toBe(synthetic);
      expect(fetchSpy).toHaveBeenCalledTimes(2);
    } finally {
      restore();
    }
  });
});

// ---------------------------------------------------------------------------
// invoker.ts timeout fallback chains (309/355/387/421/461 `?? DEFAULT_TIMEOUT`)
// ---------------------------------------------------------------------------

describe("invoker default timeout fallback", () => {
  it("uses the built-in default when config.timeout is explicitly undefined", async () => {
    const seenUrls: string[] = [];
    const { restore } = withMockedFetch(async (url) => {
      seenUrls.push(url);
      // Note: the events URL carries a query string (?after_seq=...), so
      // match with includes() before the generic /tasks/ branch.
      if (url.includes("/events")) {
        return jsonResponse({ items: [{ seq: 1, type: "completed" }], done: true });
      }
      if (url.endsWith("/cancel")) return jsonResponse({});
      if (url.includes("/tasks/")) {
        return jsonResponse({ id: "t-gapfix", status: "running" });
      }
      if (url.endsWith("/tasks")) return jsonResponse({ taskId: "t-gapfix" });
      return jsonResponse({ result: { ok: true } });
    });
    try {
      const invoker = new Invoker({
        baseUrl: "https://h/api/v1",
        timeout: undefined,
      });

      await expect(invoker.invoke("fn", {})).resolves.toMatchObject({
        payload: { ok: true },
      });
      await expect(invoker.startTask("fn", {})).resolves.toBe("t-gapfix");
      await expect(invoker.getTaskStatus("t-gapfix")).resolves.toMatchObject({
        id: "t-gapfix",
        status: "running",
      });
      const events: string[] = [];
      for await (const event of invoker.streamTask("t-gapfix", { pollIntervalMs: 0 })) {
        events.push(event.type);
      }
      expect(events).toEqual(["completed"]);
      await expect(invoker.cancelTask("t-gapfix")).resolves.toBeUndefined();

      expect(seenUrls.length).toBe(5);
    } finally {
      restore();
    }
  });
});

// ---------------------------------------------------------------------------
// invoker.ts streamTask: null items list (441 `data?.items ?? []`)
// ---------------------------------------------------------------------------

describe("invoker streamTask null items", () => {
  it("treats a null items array as empty and finishes on done", async () => {
    const { restore } = withMockedFetch(async () =>
      jsonResponse({ items: null, done: true }),
    );
    try {
      const invoker = new Invoker({ baseUrl: "https://h/api/v1" });
      const events: string[] = [];
      for await (const event of invoker.streamTask("t-none", { pollIntervalMs: 1 })) {
        events.push(event.type);
      }
      expect(events).toEqual([]);
    } finally {
      restore();
    }
  });

  it("tolerates validators that fail without error details (fault injection)", async () => {
    // Ajv always attaches an errors array on failure, so invoker.ts:286's
    // `validate.errors || []` fallback arm is defensive only. Seed the private
    // validator cache with a bare failing function to prove validation errors
    // still produce a readable InvokerError instead of crashing.
    const invoker = new Invoker({ baseUrl: "https://h/api/v1" });
    invoker.setSchema("fn", { type: "object", required: ["a"] });
    const validators = (
      invoker as unknown as { validators: Map<string, ValidateFunction> }
    ).validators;
    validators.set("fn", Object.assign(() => false, {}) as unknown as ValidateFunction);

    await expect(invoker.invoke("fn", {})).rejects.toMatchObject({
      name: "InvokerError",
      code: "schema_validation",
      message: "payload validation failed: ",
    });
  });
});

// ---------------------------------------------------------------------------
// openapi.ts: toTitleCase empty segments (99), unrecognized schema keys (144),
// non-record media objects (152), boolean extension values (160)
// ---------------------------------------------------------------------------

class RecordingTarget implements RegistrationTarget {
  readonly descriptors: FunctionDescriptor[] = [];

  registerFunction(descriptor: FunctionDescriptor, _handler: FunctionHandler): void {
    this.descriptors.push(descriptor);
  }
}

const openApiHandler: FunctionHandler = () => "{}";

describe("openapi import edge cases", () => {
  it("keeps empty snake_case segments as empty words when deriving names", () => {
    const target = new RecordingTarget();
    const registered = registerFromOpenAPI(
      target,
      {
        paths: {
          "/x": {
            post: {
              operationId: "demo__ping",
              responses: { 200: { description: "ok" } },
            },
          },
        },
      },
      undefined,
      () => openApiHandler,
    );
    expect(registered).toEqual(["demo__ping"]);
    expect(target.descriptors[0].name).toBe("Demo  Ping");
    expect(target.descriptors[0].summary).toBe("Demo  Ping");
  });

  it("drops request schemas that contain no recognized JSON-Schema keys", () => {
    const target = new RecordingTarget();
    registerFromOpenAPI(
      target,
      {
        paths: {
          "/y": {
            post: {
              operationId: "y_post",
              requestBody: {
                content: {
                  "application/json": { schema: { "x-vendor-extension": 1 } },
                },
              },
              responses: { 200: { description: "ok" } },
            },
          },
        },
      },
      undefined,
      () => openApiHandler,
    );
    expect(target.descriptors[0].inputSchema).toBeUndefined();
    expect(target.descriptors[0].outputSchema).toBeUndefined();
  });

  it("drops request schemas whose media object is not a record", () => {
    const target = new RecordingTarget();
    registerFromOpenAPI(
      target,
      {
        paths: {
          "/z": {
            post: {
              operationId: "z_post",
              requestBody: {
                content: { "application/json": "text/plain" },
              },
              responses: { 200: { description: "ok" } },
            },
          },
        },
      },
      undefined,
      () => openApiHandler,
    );
    expect(target.descriptors[0].inputSchema).toBeUndefined();
  });

  it("stringifies boolean extension values", () => {
    const target = new RecordingTarget();
    registerFromOpenAPI(
      target,
      {
        paths: {
          "/b": {
            post: {
              operationId: "b_post",
              "x-resource": true,
              "x-permission": false,
              responses: { 200: { description: "ok" } },
            },
          },
        },
      },
      undefined,
      () => openApiHandler,
    );
    expect(target.descriptors[0].resource).toBe("true");
    expect(target.descriptors[0].permission).toBe("false");
  });
});
