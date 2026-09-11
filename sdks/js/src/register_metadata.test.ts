/**
 * 注册元数据透传回归（2026-09-11 线上 tags 丢失根因）：
 *
 * serializeProviderConnectProtobufRequest 的函数字段映射此前漏了
 * tags / operationId / deprecated——getRegisterRequest 里有、proto 契约里
 * 有（tags=3 / operation_id=6 / deprecated=7），但序列化时被丢弃。线上
 * 表现：JS demo 与 go/cpp demo 共享注册同一函数空间时，JS 重注册把
 * function_contracts 的 tags 抹成 null（agent funcMeta 最后注册者覆盖）。
 */

import * as protobuf from "protobufjs";
import { BasicClient } from "./index";

const ROUND_TRIP_PROTO = `
syntax = "proto3";
package croupier.sdk.v1;

message ProviderFunctionDescriptor {
  string id = 1;
  string version = 2;
  repeated string tags = 3;
  string summary = 4;
  string operation_id = 6;
  bool deprecated = 7;
  string resource = 10;
  string operation = 11;
}

message ProviderConnectRequest {
  string service_id = 1;
  string version = 2;
  repeated ProviderFunctionDescriptor functions = 3;
}
`;

const root = protobuf.parse(ROUND_TRIP_PROTO).root;
const ConnectRequest = root.lookupType("croupier.sdk.v1.ProviderConnectRequest");

interface DecodedFunction {
  id: string;
  tags: string[];
  summary: string;
  operationId: string;
  deprecated: boolean;
}

describe("register metadata protobuf round-trip", () => {
  it("carries tags, operationId and deprecated through the register frame", () => {
    const client = new BasicClient({ insecure: true });
    client.registerFunction(
      {
        id: "player.get",
        version: "1.0.0",
        tags: ["player", "get"],
        summary: "player get",
        description: "demo",
        operationId: "player.get",
        resource: "player",
        operation: "get",
        capability: "item_query",
        risk: "safe",
      },
      async () => "ok",
    );

    const serializer = client as unknown as {
      serializeProviderConnectProtobufRequest: (req: unknown) => Buffer;
    };
    const buf = serializer.serializeProviderConnectProtobufRequest(
      client.getRegisterRequest(),
    );
    expect(buf.length).toBeGreaterThan(0);

    const decoded = ConnectRequest.toObject(ConnectRequest.decode(buf), {
      defaults: true,
    }) as { functions: DecodedFunction[] };
    expect(decoded.functions).toHaveLength(1);
    const fn = decoded.functions[0];
    expect(fn.tags).toEqual(["player", "get"]);
    expect(fn.summary).toBe("player get");
    expect(fn.operationId).toBe("player.get");
    expect(fn.deprecated).toBe(false);
  });
});
