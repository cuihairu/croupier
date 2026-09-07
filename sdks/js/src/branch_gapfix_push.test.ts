/**
 * Branch coverage gap fix for index.ts:1463 — the FilePush unmarshal error
 * path's `String(error)` arm.
 *
 * protobufjs decode only ever throws Error instances for malformed wire data,
 * so the non-Error arm of
 * `unmarshal FilePushRequest: ${error instanceof Error ? error.message : String(error)}`
 * cannot be reached through public inputs. This file module-mocks protobufjs
 * (fault injection, same spirit as the node:fs mock in coverage_final.test.ts)
 * so the provider's FilePushRequest.decode throws a plain string, proving the
 * provider replies with a well-formed error frame instead of crashing.
 *
 * Kept in its own test file because the mock applies to every protobuf call
 * made by modules loaded in this file.
 */

import { Buffer } from "node:buffer";
import protobuf from "protobufjs";
import { BasicClient, type ClientConfig } from "./index";

jest.mock("protobufjs", () => {
  const actual = jest.requireActual("protobufjs") as typeof import("protobufjs");
  const realParse = actual.parse.bind(actual);
  return {
    ...actual,
    parse: ((proto: string, options?: { keepCase?: boolean; alternateCommentMode?: boolean }) => {
      const parsed = realParse(proto, options);
      const realLookup = parsed.root.lookupType.bind(parsed.root);
      parsed.root.lookupType = ((name: string) => {
        const type = realLookup(name);
        if (name === "croupier.sdk.v1.FilePushRequest") {
          type.decode = (() => {
            throw "wire corruption";
          }) as typeof type.decode;
        }
        return type;
      }) as typeof parsed.root.lookupType;
      return parsed;
    }) as typeof actual.parse,
  };
});

const responseProto = `
syntax = "proto3";
package gapfix.test;
message FilePushResponse {
  string transfer_id = 1;
  bool ok = 2;
  string stored_path = 3;
  string error = 4;
}
`;
const FilePushResponseMessage = protobuf.parse(responseProto).root.lookupType(
  "gapfix.test.FilePushResponse",
);

describe("file push unmarshal failure stringification", () => {
  it("stringifies non-Error decode failures in the error response", async () => {
    const config: ClientConfig = {
      autoReconnect: false,
      enableFileTransfer: true,
    };
    const client = new BasicClient(config);
    client.registerFunction({ id: "player.ban", version: "1.0.0" }, () => "ok");

    const response = await (
      client as unknown as {
        handleInboundRequest: (
          msgId: number,
          reqId: number,
          body: Buffer,
        ) => Promise<Buffer>;
      }
    ).handleInboundRequest(0x050109, 1, Buffer.from([0x0a, 0x02, 0x74, 0x31]));

    const decoded = FilePushResponseMessage.toObject(
      FilePushResponseMessage.decode(response),
      { defaults: true },
    ) as { ok?: boolean; error?: string };
    expect(decoded.ok).toBe(false);
    expect(decoded.error).toBe("unmarshal FilePushRequest: wire corruption");
  });
});
