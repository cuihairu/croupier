import dashboardMock from './dashboard';
import type { ConsoleMenuSpec } from '@/types/dashboard';

describe('dashboard mock contract', () => {
  test('console menu matches ConsoleMenuSpec and published mock pages', () => {
    const handlers = dashboardMock as Record<
      string,
      (req: unknown, res: { send: jest.Mock }) => void
    >;
    const menuResponse = { send: jest.fn() };
    const pagesResponse = { send: jest.fn() };

    handlers['GET /api/v1/console/menu']({}, menuResponse);
    handlers['GET /api/v1/console/pages']({}, pagesResponse);

    const menu = menuResponse.send.mock.calls[0][0] as ConsoleMenuSpec;
    const pages = pagesResponse.send.mock.calls[0][0] as { items: Array<{ pageKey: string }> };
    expect(Array.isArray(menu.items)).toBe(true);
    expect(menu.items.length).toBeGreaterThan(0);

    const pageKeys = new Set(pages.items.map((page) => page.pageKey));
    menu.items.forEach((category) => {
      expect(category.locale).toBe(false);
      expect(category.path).toBe(`/console/${encodeURIComponent(category.key)}`);
      expect(category.children).toHaveLength(1);
      category.children?.forEach((page) => {
        expect(pageKeys.has(page.key)).toBe(true);
        expect(page.path).toBe(`${category.path}/${encodeURIComponent(page.key)}`);
        expect(page.locale).toBe(false);
      });
    });
  });
});
