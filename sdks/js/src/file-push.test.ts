/**
 * F：文件下发接收（hotpatch P1 传输层）——JS 实现测试。
 */
import { Buffer } from "node:buffer";
import protobuf from "protobufjs";
import { mkdtempSync, readFileSync, existsSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { createHash } from "node:crypto";
import { BasicClient, type ClientConfig, type FunctionDescriptor } from "./index";

const proto = `
syntax = "proto3";
package croupier.sdk.v1;
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
`;
const root = protobuf.parse(proto).root;
const FilePushRequestMessage = root.lookupType("croupier.sdk.v1.FilePushRequest");
const FilePushResponseMessage = root.lookupType("croupier.sdk.v1.FilePushResponse");

type DecodedPushResponse = { ok?: boolean; error?: string; storedPath?: string };

function makeClient(
  configOverride: Partial<ClientConfig>,
): { client: BasicClient; dispatch: (body: Buffer) => Promise<Buffer> } {
  const config: ClientConfig = {
    autoReconnect: false,
    enableFileTransfer: true,
    fileStagingDir: mkdtempSync(join(tmpdir(), "croupier-push-")),
    ...configOverride,
  };
  const client = new BasicClient(config);
  const descriptor: FunctionDescriptor = { id: "player.ban", version: "1.0.0" };
  client.registerFunction(descriptor, () => "ok");
  const anyClient = client as unknown as {
    handleInboundRequest: (m: number, r: number, b: Buffer) => Promise<Buffer>;
  };
  return {
    client,
    dispatch: (body: Buffer) => anyClient.handleInboundRequest(0x050109, 1, body),
  };
}

function buildPushBody(
  transferId: string,
  fileName: string,
  sha256: string,
  data: Buffer,
): Buffer {
  return Buffer.from(
    FilePushRequestMessage.encode(
      FilePushRequestMessage.create({ transferId, fileName, contentSha256: sha256, data }),
    ).finish(),
  );
}

function decodeResponse(response: Buffer): DecodedPushResponse {
  return FilePushResponseMessage.toObject(FilePushResponseMessage.decode(response), {
    defaults: true,
  }) as DecodedPushResponse;
}

describe("F：文件下发接收", () => {
  const staging = mkdtempSync(join(tmpdir(), "croupier-push-"));
  const data = Buffer.from("print('hotfix')");
  const sha256 = createHash("sha256").update(data).digest("hex");

  test("合法文件落盘暂存目录并回 ok+storedPath", async () => {
    const { dispatch } = makeClient({ fileStagingDir: staging });
    const response = await dispatch(buildPushBody("t-1", "hotfix.lua", sha256, data));
    const decoded = decodeResponse(response);
    expect(decoded.ok).toBe(true);
    expect(decoded.storedPath).toContain("hotfix.lua");
    expect(readFileSync(decoded.storedPath!).toString()).toBe("print('hotfix')");
  });

  test("开关关闭拒绝", async () => {
    const { dispatch } = makeClient({ enableFileTransfer: false, fileStagingDir: staging });
    const response = await dispatch(buildPushBody("t-2", "hotfix.lua", sha256, data));
    const decoded = decodeResponse(response);
    expect(decoded.ok).toBe(false);
    expect(decoded.error).toContain("file transfer is disabled");
  });

  test("checksum 不匹配拒绝", async () => {
    const { dispatch } = makeClient({ fileStagingDir: staging });
    const response = await dispatch(
      buildPushBody("t-3", "hotfix.lua", "deadbeef".repeat(8), data),
    );
    const decoded = decodeResponse(response);
    expect(decoded.ok).toBe(false);
    expect(decoded.error).toContain("checksum mismatch");
  });

  test("路径穿越拒绝（../ 与子目录）", async () => {
    const { dispatch } = makeClient({ fileStagingDir: staging });
    for (const evil of ["../evil.lua", "sub/dir/evil.lua"]) {
      const response = await dispatch(buildPushBody("t-4", evil, sha256, data));
      const decoded = decodeResponse(response);
      expect(decoded.ok).toBe(false);
      expect(decoded.error).toContain("bare basename");
    }
    // 恶意文件不存在于暂存目录之外
    expect(existsSync(join(staging, "..", "evil.lua"))).toBe(false);
  });

  test("超限拒绝", async () => {
    const { dispatch } = makeClient({ fileStagingDir: staging, maxFileSize: 4 });
    const response = await dispatch(buildPushBody("t-5", "big.lua", sha256, data));
    const decoded = decodeResponse(response);
    expect(decoded.ok).toBe(false);
    expect(decoded.error).toContain("exceeds max");
  });
});
