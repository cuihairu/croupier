/**
 * gameOpsDemoRuntime - 模板预览进程内共享 demo 数据源
 *
 * 所有游戏运营业务模板读取同一份内存实例，保证玩家/任务/封禁/订单/活动
 * 的跨页关联一致；重置按钮调用 resetGameOpsDemoRuntime 后恢复初始 seed。
 */

import type { PageExecuteFn } from '@/types/dashboard';
import { createGameOpsDemoStore, type GameOpsDemoStore } from './gameOpsDemoData';

export const gameOpsDemoRuntimeStore = createGameOpsDemoStore();

export function resetGameOpsDemoRuntime(): void {
  gameOpsDemoRuntimeStore.reset();
}

export function createGameOpsDemoExecutor(
  createExecutor: (store: GameOpsDemoStore) => PageExecuteFn,
): PageExecuteFn {
  return createExecutor(gameOpsDemoRuntimeStore);
}
