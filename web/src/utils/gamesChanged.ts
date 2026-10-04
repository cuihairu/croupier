/**
 * 游戏资料变更广播：新增/编辑游戏后派发 games:changed，
 * GameSelector 监听该事件立即重拉授权列表，让变更即时可见。
 */
export const GAMES_CHANGED_EVENT = 'games:changed';

export function notifyGamesChanged(): void {
  if (typeof window === 'undefined') return;
  window.dispatchEvent(new Event(GAMES_CHANGED_EVENT));
}
