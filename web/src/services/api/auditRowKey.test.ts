/**
 * 审计列表 rowKey 回归（docs/BUGS.md BUG-011）。
 *
 * 现象：`/admin/login-logs` 每页刷十余条
 * `Encountered two children with the same key`。
 *
 * 根因：两个列表页都用 `rowKey={(r) => r.hash}`，但 `/api/v1/audit` 在当前部署
 * **不回填 `hash`**（实测 19 条记录 `hash` 全为空）。`normalizeAuditEvent` 又把
 * 缺失的 hash 兜底成 `''`，于是整页每一行的 rowKey 都是空串——React 无法区分
 * 各行，可能重复/漏渲染。
 *
 * 修复：归一化时保留服务端唯一 `id`，并提供 `auditRowKey`：
 * id → hash → time|actor|kind|target → 行序号，保证同页内唯一。
 */
import { request } from '@umijs/max';
import { auditRowKey, listAudit, type AuditEvent } from './audit';

jest.mock('@umijs/max', () => ({ request: jest.fn() }));

const mockedRequest = request as jest.MockedFunction<typeof request>;

/** 构造一条服务端原始审计记录。 */
function rawItem(id: string, hash?: string) {
  return {
    id,
    action: 'auth.login',
    userId: 'admin',
    target: '',
    createdAt: `2026-09-25T22:00:00Z`,
    ...(hash === undefined ? {} : { hash }),
  };
}

function event(overrides: Partial<AuditEvent> = {}): AuditEvent {
  return {
    id: '',
    time: '',
    kind: '',
    actor: '',
    target: '',
    meta: {},
    hash: '',
    prev: '',
    ...overrides,
  };
}

describe('listAudit 归一化保留服务端唯一 id', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('把 id 带进 view-model（此前被丢弃，导致 rowKey 无从取得）', async () => {
    mockedRequest.mockResolvedValue({
      items: [rawItem('audit_1_abc')],
      total: 1,
      page: 1,
      pageSize: 20,
    });
    const res = await listAudit();
    expect(res.events[0].id).toBe('audit_1_abc');
  });

  it('id 缺失时兜底为空串，不产生 undefined', async () => {
    mockedRequest.mockResolvedValue({
      // 模拟类型漂移：服务端没给 id
      items: [{ ...rawItem('x'), id: undefined as unknown as string }],
      total: 1,
      page: 1,
      pageSize: 20,
    });
    const res = await listAudit();
    expect(res.events[0].id).toBe('');
  });
});

describe('auditRowKey 保证同页内唯一（BUG-011）', () => {
  it('首选服务端唯一 id', () => {
    expect(auditRowKey(event({ id: 'audit_1' }), 0)).toBe('audit_1');
    expect(auditRowKey(event({ id: 'audit_2' }), 1)).toBe('audit_2');
  });

  it('id 缺失时退回审计链 hash', () => {
    expect(auditRowKey(event({ hash: 'h1' }), 0)).toBe('h1');
  });

  it('id 与 hash 都缺失时用组合字段（当前部署的真实形态）', () => {
    const a = auditRowKey(
      event({ time: '2026-09-25T22:00:01Z', actor: 'admin', kind: 'auth.login' }),
      0,
    );
    const b = auditRowKey(
      event({ time: '2026-09-25T22:00:02Z', actor: 'admin', kind: 'auth.login' }),
      1,
    );
    expect(a).not.toBe(b);
    expect(a).toContain('auth.login');
  });

  it('字段全空时用行序号兜底，仍唯一', () => {
    expect(auditRowKey(event(), 0)).toBe('audit-row-0');
    expect(auditRowKey(event(), 1)).toBe('audit-row-1');
    expect(auditRowKey(event(), 0)).not.toBe(auditRowKey(event(), 1));
  });

  it('还原真实响应：整页 key 必须互不相同（修复前全是空串）', async () => {
    // 服务端返回 19 条、hash 全部缺失——与实测的 /api/v1/audit 响应同形
    mockedRequest.mockResolvedValue({
      items: Array.from({ length: 19 }, (_, i) => rawItem(`audit_${i}_x`)),
      total: 19,
      page: 1,
      pageSize: 20,
    });
    const res = await listAudit();
    const keys = res.events.map((e, i) => auditRowKey(e, i));
    expect(new Set(keys).size).toBe(keys.length);
    expect(keys.every((k) => k !== '')).toBe(true);
  });
});
