/**
 * CR-FIX-05 跨 SDK 行为级契约 golden 向量测试（JavaScript SDK 面）。
 *
 * 读取 proto/contracts/golden_vectors.json（语言中立字节向量），断言本 SDK 的
 * newMessage 产出与向量 frame 逐字节一致，且 parseMessage 能还原出
 * (version, msgId, reqId, body)。任一 wire 语义漂移即失败。
 */
import { readFileSync } from 'fs';
import { resolve } from 'path';
import { newMessage, parseMessage, VERSION_1, MSG_REGISTER_REQUEST, MSG_HEARTBEAT_REQUEST, MSG_TASK_EVENT } from './protocol';

interface GoldenVec {
  msgId: string;
  reqId: string;
  body: string;
  frame: string;
}

interface GoldenFile {
  version: number;
  vectors: Record<string, GoldenVec>;
}

function loadGolden(): GoldenFile {
  // 测试位于 sdks/js/src/，向上 3 层到仓库根。
  const p = resolve(__dirname, '..', '..', '..', 'proto', 'contracts', 'golden_vectors.json');
  return JSON.parse(readFileSync(p, 'utf8')) as GoldenFile;
}

function hex(s: string): Buffer {
  return Buffer.from(s, 'hex');
}

function msgIdFromHex(s: string): number {
  return parseInt(s, 16);
}

describe('golden vectors wire frames', () => {
  const gf = loadGolden();

  for (const [name, vec] of Object.entries(gf.vectors)) {
    it(`encodes and decodes ${name}`, () => {
      const body = hex(vec.body);
      const wantFrame = hex(vec.frame);
      const msgId = msgIdFromHex(vec.msgId);
      const reqId = parseInt(vec.reqId, 16);

      const got = newMessage(msgId, reqId, body);
      expect(got).toEqual(wantFrame);

      const parsed = parseMessage(wantFrame);
      expect(parsed.version).toBe(VERSION_1);
      expect(parsed.msgId).toBe(msgId);
      expect(parsed.reqId).toBe(reqId);
      expect(parsed.body).toEqual(body);
    });
  }

  it('uses known msgIds from the contract surface', () => {
    const v = gf.vectors;
    expect(msgIdFromHex(v.register_request.msgId)).toBe(MSG_REGISTER_REQUEST);
    expect(msgIdFromHex(v.heartbeat_request.msgId)).toBe(MSG_HEARTBEAT_REQUEST);
    expect(msgIdFromHex(v.task_event.msgId)).toBe(MSG_TASK_EVENT);
  });
});
