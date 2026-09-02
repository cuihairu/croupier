/**
 * F15 验收测试：provider 侧入站 payload 校验
 */
import { Buffer } from "node:buffer";
import protobuf from "protobufjs";
import { BasicClient, type ClientConfig, type FunctionDescriptor } from "./index";

const proto = `
syntax = "proto3";
package croupier.sdk.v1;
message InvokeRequest {
  string function_id = 1;
  string idempotencyKey = 2;
  bytes payload = 3;
  map<string, string> metadata = 4;
}
message InvokeResponse {
  bytes payload = 1;
}
`;
const root = protobuf.parse(proto).root;
const InvokeRequestMessage = root.lookupType("croupier.sdk.v1.InvokeRequest");
const InvokeResponseMessage = root.lookupType("croupier.sdk.v1.InvokeResponse");

function encodeInvoke(functionId: string, payload: string): Buffer {
  return Buffer.from(
    InvokeRequestMessage.encode(
      InvokeRequestMessage.create({ functionId, payload: Buffer.from(payload) }),
    ).finish(),
  );
}

function makeClient(validate: boolean, handlerPayload?: string): BasicClient {
  const config: ClientConfig = {
    autoReconnect: false,
    validateInputPayloads: validate,
  };
  const client = new BasicClient(config);
  const descriptor: FunctionDescriptor = {
    id: "player.ban",
    version: "1.0.0",
    inputSchema: {
      type: "object",
      properties: { id: { type: "string" } },
      required: ["id"],
    },
  };
  client.registerFunction(descriptor, () => handlerPayload ?? "ok");
  return client;
}

function invoke(client: BasicClient, functionId: string, payload: string): Promise<string> {
  const anyClient = client as unknown as {
    handleInboundInvoke: (body: Buffer) => Promise<Buffer>;
  };
  return anyClient.handleInboundInvoke(encodeInvoke(functionId, payload)).then((response) => {
    const decoded = InvokeResponseMessage.decode(response) as { payload?: Uint8Array };
    return new TextDecoder().decode(decoded.payload ?? new Uint8Array());
  });
}

describe("F15: 入站 payload 校验", () => {
  test("合法 payload 通过并调用 handler", async () => {
    const client = makeClient(true);
    const response = await invoke(client, "player.ban", JSON.stringify({ id: "p1" }));
    expect(response).toBe("ok");
  });

  test("缺 required 字段回错误 payload，handler 不被调用", async () => {
    const handler = jest.fn(() => "ok");
    const client = new BasicClient({ autoReconnect: false, validateInputPayloads: true });
    client.registerFunction(
      {
        id: "player.ban",
        version: "1.0.0",
        inputSchema: { type: "object", properties: { id: { type: "string" } }, required: ["id"] },
      },
      handler,
    );
    const response = await invoke(client, "player.ban", "{}");
    const parsed = JSON.parse(response) as { error?: string };
    expect(parsed.error).toContain("payload validation failed");
    expect(handler).not.toHaveBeenCalled();
  });

  test("类型不符回错误 payload", async () => {
    const client = makeClient(true);
    const response = await invoke(client, "player.ban", JSON.stringify({ id: 123 }));
    const parsed = JSON.parse(response) as { error?: string };
    expect(parsed.error).toContain("payload validation failed");
  });

  test("开关关闭跳过校验（兼容旧行为）", async () => {
    const handler = jest.fn(() => "ok");
    const client = new BasicClient({ autoReconnect: false });
    client.registerFunction(
      {
        id: "player.ban",
        version: "1.0.0",
        inputSchema: { type: "object", properties: { id: { type: "string" } }, required: ["id"] },
      },
      handler,
    );
    const response = await invoke(client, "player.ban", "{}");
    expect(response).toBe("ok");
    expect(handler).toHaveBeenCalled();
  });
});
