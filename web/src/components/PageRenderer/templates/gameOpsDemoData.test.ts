/**
 * gameOpsDemoData 覆盖：仓库完整性（数据量、交叉引用、唯一性）与边界用例。
 */
import { createGameOpsDemoStore, createRequestIdFactory, asRecord } from './gameOpsDemoData';

describe('gameOpsDemoData', () => {
  const store = createGameOpsDemoStore();

  it('数据量足够多页：玩家 26 / 订单 30 / 封禁 12 / 活动 8', () => {
    expect(store.players).toHaveLength(26);
    expect(store.orders).toHaveLength(30);
    expect(store.bans).toHaveLength(12);
    expect(store.activities).toHaveLength(8);
    expect(store.servers).toHaveLength(3);
    expect(store.tasks.length).toBeGreaterThanOrEqual(4);
    expect(store.opRecords.length).toBeGreaterThanOrEqual(3);
  });

  it('交叉引用完整：封禁/订单/区服引用均真实存在', () => {
    const playerIds = new Set(store.players.map((player) => player.id));
    const serverIds = new Set(store.servers.map((server) => server.id));
    for (const ban of store.bans) {
      expect(playerIds.has(ban.playerId)).toBe(true);
      expect(serverIds.has(ban.serverId)).toBe(true);
    }
    for (const order of store.orders) {
      expect(playerIds.has(order.playerId)).toBe(true);
    }
    for (const player of store.players) {
      expect(serverIds.has(player.serverId)).toBe(true);
    }
    for (const task of store.tasks) {
      if (task.targetServerId) {
        expect(serverIds.has(task.targetServerId)).toBe(true);
      }
      if (task.targetPlayerId) {
        expect(playerIds.has(task.targetPlayerId)).toBe(true);
      }
      for (const pid of task.targetPlayerIds ?? []) {
        expect(playerIds.has(pid)).toBe(true);
      }
      if (task.targetOrderNo) {
        expect(store.orders.some((order) => order.orderNo === task.targetOrderNo)).toBe(true);
      }
    }
  });

  it('边界用例在案：长昵称、三种封禁状态、三类对账结论、四态发货', () => {
    expect(store.players.some((player) => player.nickname.length > 30)).toBe(true);
    expect(new Set(store.bans.map((ban) => ban.status))).toEqual(
      new Set(['active', 'expired', 'lifted']),
    );
    expect(new Set(store.orders.map((order) => order.reconciliation))).toEqual(
      new Set(['match', 'missing_credit', 'missing_order']),
    );
    expect(new Set(store.orders.map((order) => order.gameCredit))).toEqual(
      new Set(['credited', 'pending', 'missing', 'not_needed']),
    );
    // 补偿任务带每人数量与总量且审批号在案
    const compensation = store.tasks.find((task) => task.type === 'compensation');
    expect(compensation?.perAmount).toBeGreaterThan(0);
    expect(compensation?.totalAmount).toBe(
      (compensation?.perAmount ?? 0) * compensation!.targetCount,
    );
    expect(compensation?.approvalId).toBeTruthy();
  });

  it('id 唯一（同仓内不重复）', () => {
    const uniq = (values: string[]) => new Set(values).size === values.length;
    expect(uniq(store.players.map((player) => player.id))).toBe(true);
    expect(uniq(store.orders.map((order) => order.orderNo))).toBe(true);
    expect(uniq(store.bans.map((ban) => ban.id))).toBe(true);
    expect(uniq(store.activities.map((activity) => activity.id))).toBe(true);
  });

  it('pushOpRecord 追加记录并返回 operationId；活动均带版本历史', () => {
    const before = store.opRecords.length;
    const operationId = store.pushOpRecord({
      operator: 'tester',
      action: 'test.action',
      target: 't1',
      result: 'success',
    });
    expect(store.opRecords).toHaveLength(before + 1);
    expect(store.opRecords[0].operationId).toBe(operationId);
    for (const activity of store.activities) {
      expect(activity.versions.length).toBeGreaterThanOrEqual(1);
      expect(activity.currentVersion).toBe(activity.versions.length);
    }
  });

  it('每次调用返回全新实例（测试间互不污染）', () => {
    const a = createGameOpsDemoStore();
    const b = createGameOpsDemoStore();
    expect(a).not.toBe(b);
    expect(a.players[0]).not.toBe(b.players[0]);
  });

  it('reset 原地恢复整组 seed，已有引用读到新数据', () => {
    const shared = createGameOpsDemoStore();
    const players = shared.players;
    shared.players = [];
    shared.opRecords = [];
    shared.reset();

    expect(shared.players).toHaveLength(26);
    expect(shared.opRecords).toHaveLength(3);
    expect(shared.players).not.toBe(players);
    expect(shared.tasks.every((task) => task.targetServerId !== undefined)).toBe(true);
  });

  it('createRequestIdFactory 递增；asRecord 非对象兜底空对象', () => {
    const next = createRequestIdFactory();
    expect(next()).toBe('gameops-demo-1');
    expect(next()).toBe('gameops-demo-2');
    expect(asRecord(undefined)).toEqual({});
    expect(asRecord([1])).toEqual({});
    expect(asRecord({ a: 1 })).toEqual({ a: 1 });
  });
});
