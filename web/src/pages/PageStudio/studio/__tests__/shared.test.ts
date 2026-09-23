/**
 * studio/shared 纯函数：statusColor / formatDate / pageTypeLabel 与
 * jsdom 侧 currentFocusPageKey 正例（SSR 守卫负例见 sharedNode.test.ts）。
 */
import {
  clearMountParam,
  currentFocusPageKey,
  currentMountFlag,
  formatDate,
  pageTypeLabel,
  statusColor,
} from '../shared';

jest.mock('@umijs/max', () => ({
  __esModule: true,
  // shared.ts 模块级调用 getIntl（无组件上下文的 label 求值先例）
  getIntl: () => ({
    locale: 'zh-CN',
    formatMessage: ({ defaultMessage }: { defaultMessage: string }) => defaultMessage,
  }),
}));

describe('statusColor', () => {
  it('published 绿、archived 灰、其余（含未知值）蓝', () => {
    expect(statusColor('published')).toBe('green');
    expect(statusColor('archived')).toBe('default');
    expect(statusColor('draft')).toBe('blue');
    expect(statusColor('whatever' as never)).toBe('blue');
  });
});

describe('formatDate', () => {
  it('空值返回 -', () => {
    expect(formatDate(undefined)).toBe('-');
    expect(formatDate('')).toBe('-');
  });

  it('非法时间串原样返回', () => {
    expect(formatDate('not-a-date')).toBe('not-a-date');
  });

  it('合法 ISO 时间本地化渲染', () => {
    const out = formatDate('2026-09-19T00:00:00Z');
    expect(out).not.toBe('-');
    expect(Number.isNaN(new Date(out).getTime())).toBe(false);
  });
});

describe('pageTypeLabel', () => {
  it('四种页面类型走词典，未知类型回退原值', () => {
    expect(pageTypeLabel('resource')).toBe('资源页面');
    expect(pageTypeLabel('operation')).toBe('操作页面');
    expect(pageTypeLabel('task')).toBe('任务页面');
    expect(pageTypeLabel('report')).toBe('报表页面');
    expect(pageTypeLabel('mystery' as never)).toBe('mystery');
  });
});

describe('currentFocusPageKey / currentMountFlag / clearMountParam（jsdom）', () => {
  const cleanUrl = () => window.history.replaceState(null, '', '/page-studio');

  afterEach(cleanUrl);

  it('读取 ?focus 与 ?mount=1', () => {
    window.history.replaceState(null, '', '/page-studio?focus=demo--home&mount=1');
    expect(currentFocusPageKey()).toBe('demo--home');
    expect(currentMountFlag()).toBe(true);
  });

  it('无参数时 focus 空串、mount false', () => {
    expect(currentFocusPageKey()).toBe('');
    expect(currentMountFlag()).toBe(false);
  });

  it('clearMountParam 消费后移除 mount、保留 focus，且二次调用幂等', () => {
    window.history.replaceState(null, '', '/page-studio?focus=demo--home&mount=1');
    clearMountParam();
    expect(window.location.search).toBe('?focus=demo--home');
    expect(() => clearMountParam()).not.toThrow();
    expect(window.location.search).toBe('?focus=demo--home');
  });
});
