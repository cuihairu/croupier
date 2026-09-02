/**
 * Provider drain 处理：确认帧、幂等、drain 期间拒绝新 Invoke、恢复后清除状态。
 */
import { Buffer } from "node:buffer";
import { BasicClient, createClient } from "./index";
import { MSG_PROVIDER_DRAIN_REQUEST } from "./protocol";

const drainBody = () => Buffer.alloc(0);

describe("F-provider-drain: drain request handling", () => {
  test("确认帧为空且置位 draining（幂等）", async () => {
    const client = createClient({ autoReconnect: false }) as BasicClient;
    const anyClient = client as unknown as Record<string, unknown> & {
      handleDrainRequest: (b: Buffer) => Buffer;
      draining: boolean;
      drainAndRecover: () => Promise<void>;
      config: { autoReconnect: boolean; reconnect: { enabled: boolean } };
    };

    // 制造在途调用：恢复流程必须等待，draining 保持 true
    (anyClient as unknown as { inflightCalls: number }).inflightCalls = 1;

    const resp = anyClient.handleDrainRequest(drainBody());
    expect(Buffer.isBuffer(resp)).toBe(true);
    expect(resp.length).toBe(0);
    expect(anyClient.draining).toBe(true);

    // 幂等：重复 drain 不重复进入恢复流程
    expect(anyClient.handleDrainRequest(drainBody()).length).toBe(0);
    expect(anyClient.draining).toBe(true);

    // 恢复（autoReconnect 关闭 → 清理连接并清除状态）
    anyClient.config.autoReconnect = false;
    anyClient.config.reconnect.enabled = false;
    (anyClient as unknown as { inflightCalls: number }).inflightCalls = 0;
    await new Promise((r) => setTimeout(r, 200));
    expect(anyClient.draining).toBe(false);
  });

  test("派发表路由 drain 且 drain 期间新 Invoke 被拒", async () => {
    const client = createClient({ autoReconnect: false }) as BasicClient;
    const anyClient = client as unknown as Record<string, unknown> & {
      handleInboundRequest: (
        msgId: number,
        reqId: number,
        body: Buffer,
      ) => Promise<Buffer>;
      draining: boolean;
      handlers: Map<string, (ctx: string, payload: string) => Promise<string>>;
    };
    anyClient.handlers.set("demo.fn", async () => JSON.stringify({ ok: true }));
    (anyClient as unknown as { inflightCalls: number }).inflightCalls = 1;

    const resp = await anyClient.handleInboundRequest(
      MSG_PROVIDER_DRAIN_REQUEST,
      1,
      drainBody(),
    );
    expect(resp.length).toBe(0);
    expect(anyClient.draining).toBe(true);

    // drain 期间：Invoke 被拒（此用例无需等待恢复），返回错误 payload，handler 不执行
    const invokeBody = Buffer.from(
      JSON.stringify({ functionId: "demo.fn", payload: [] }),
    );
    const invokeResp = await anyClient.handleInboundRequest(
      0x030101, // MSG_INVOKE_REQUEST
      2,
      invokeBody,
    );
    expect(invokeResp.toString()).toContain("provider is draining");
  });
});
