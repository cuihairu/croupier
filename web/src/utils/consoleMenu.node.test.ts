/**
 * @jest-environment node
 */
import { requestConsoleMenuRefresh } from './consoleMenu';

describe('requestConsoleMenuRefresh（SSR：无 window）', () => {
  it('无 window 时安全 no-op（不派发事件、不抛错）', () => {
    expect(() => requestConsoleMenuRefresh()).not.toThrow();
  });
});
