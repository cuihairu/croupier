/**
 * 实例元数据（ProviderConnectRequest.metadata）序列化回归：
 *
 * BasicClient 新增 instanceMetadata 配置（用户自定义多 KV，如 serverId），
 * 序列化时必须随帧携带（field 13，map<string,string>）。旧 SDK 不发该字段
 * 依旧兼容；agent 对保留键（sdkLanguage 等）剥离开告警。
 */

import * as protobuf from "protobufjs";
import { BasicClient } from "./index";

const ROUND_TRIP_PROTO = `
syntax = "proto3";
package croupier.sdk.v1;

message ProviderConnectRequest {
  string service_id = 1;
  string version = 2;
  string sdk_language = 4;
  string sdk_version = 5;
  string sdk_name = 6;
  string game_id = 11;
  string env = 12;
  map<string, string> metadata = 13;
}
`;

const root = protobuf.parse(ROUND_TRIP_PROTO).root;
const ConnectRequest = root.lookupType("croupier.sdk.v1.ProviderConnectRequest");

interface DecodedRequest {
  serviceId: string;
  metadata: Record<string, string>;
}

describe("provider instance metadata protobuf round-trip", () => {
  it("carries user metadata key-values through the register frame", () => {
    const client = new BasicClient({
      insecure: true,
      serviceId: "game-demo",
      instanceMetadata: { serverId: "s1", pod: "game-7c4d" },
    });

    const serializer = client as unknown as {
      serializeProviderConnectProtobufRequest: (req: unknown) => Buffer;
    };
    const buf = serializer.serializeProviderConnectProtobufRequest(
      client.getRegisterRequest(),
    );
    expect(buf.length).toBeGreaterThan(0);

    const decoded = ConnectRequest.toObject(ConnectRequest.decode(buf), {
      defaults: true,
    }) as DecodedRequest;
    expect(decoded.serviceId).toBe("game-demo");
    expect(decoded.metadata).toEqual({ serverId: "s1", pod: "game-7c4d" });
  });

  it("omits metadata when instanceMetadata is not configured", () => {
    const client = new BasicClient({ insecure: true });

    const serializer = client as unknown as {
      serializeProviderConnectProtobufRequest: (req: unknown) => Buffer;
    };
    const buf = serializer.serializeProviderConnectProtobufRequest(
      client.getRegisterRequest(),
    );

    const decoded = ConnectRequest.toObject(ConnectRequest.decode(buf), {
      defaults: true,
    }) as DecodedRequest;
    expect(decoded.metadata).toEqual({});
  });
});
