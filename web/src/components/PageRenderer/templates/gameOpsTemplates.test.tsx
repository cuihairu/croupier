/**
 * gameOpsTemplates 注册表 + Console/TemplateGameOps 壳覆盖。
 *
 * 模板组件重量级且有自己的行为（由各自测试覆盖），这里 mock 后验证：
 * - 注册表覆盖五个默认菜单分类、key 唯一、PageSpec 归属一致；
 * - 壳按路径渲染对应模板、未知 key 兜底列表不猜默认模板。
 */
import React from 'react';
import { render, screen, fireEvent } from '@testing-library/react';
import {
  findGameOpsTemplate,
  gameOpsDefaultMenuKeys,
  gameOpsTemplateRoutePrefix,
  gameOpsTemplates,
} from './gameOpsTemplates';
import TemplateGameOpsPage, { templateKeyFromPath } from '@/pages/Console/TemplateGameOps';

const mockPush = jest.fn();
let mockPathname = '/console/templates/player-manage';

jest.mock('@umijs/max', () => ({
  useIntl: () => ({
    formatMessage: ({ defaultMessage }: { defaultMessage: string }) => defaultMessage,
  }),
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => (
    <span>{defaultMessage}</span>
  ),
  history: {
    // 工厂在变量初始化前被提升调用，这里用 getter 延迟读取外层可变状态
    get location() {
      return { pathname: mockPathname };
    },
    get push() {
      return mockPush;
    },
  },
}));

jest.mock('./playerManage', () => ({
  __esModule: true,
  default: () => <div data-testid="tpl-player" />,
  playerManagePageSpec: { pageKey: 'template--player-manage' },
}));
jest.mock('./mailCompensation', () => ({
  __esModule: true,
  default: () => <div data-testid="tpl-mail" />,
  mailCompensationPageSpec: { pageKey: 'template--mail-compensation' },
}));
jest.mock('./banManagement', () => ({
  __esModule: true,
  default: () => <div data-testid="tpl-ban" />,
  banManagementPageSpec: { pageKey: 'template--ban-management' },
}));
jest.mock('./orderReview', () => ({
  __esModule: true,
  default: () => <div data-testid="tpl-order" />,
  orderReviewPageSpec: { pageKey: 'template--order-review' },
}));
jest.mock('./activityConfig', () => ({
  __esModule: true,
  default: () => <div data-testid="tpl-activity" />,
  activityConfigPageSpec: { pageKey: 'template--activity-config' },
}));

describe('gameOpsTemplates 注册表', () => {
  it('恰好五个条目，menuKey 与默认菜单前五类一一对应且顺序一致', () => {
    expect(gameOpsTemplates.map((entry) => entry.menuKey)).toEqual([...gameOpsDefaultMenuKeys]);
  });

  it('key 唯一且与 PageSpec pageKey / 路由段一致', () => {
    const keys = gameOpsTemplates.map((entry) => entry.key);
    expect(new Set(keys).size).toBe(keys.length);
    for (const entry of gameOpsTemplates) {
      expect(entry.pageSpec.pageKey).toBe(`template--${entry.key}`);
      expect(entry.title['zh-CN']).toBeTruthy();
      expect(entry.title['en-US']).toBeTruthy();
      expect(entry.component).toBeInstanceOf(Function);
    }
  });

  it('findGameOpsTemplate 命中/未命中', () => {
    expect(findGameOpsTemplate('ban-management')?.menuKey).toBe('ban');
    expect(findGameOpsTemplate('nope')).toBeUndefined();
  });

  it('templateKeyFromPath 提取路由段', () => {
    expect(templateKeyFromPath('/console/templates/order-review')).toBe('order-review');
    expect(templateKeyFromPath('/console/home')).toBe('');
  });
});

describe('Console/TemplateGameOps 壳', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it.each([
    ['player-manage', 'tpl-player'],
    ['mail-compensation', 'tpl-mail'],
    ['ban-management', 'tpl-ban'],
    ['order-review', 'tpl-order'],
    ['activity-config', 'tpl-activity'],
  ])('渲染 %s 模板并携带示例数据横幅', (key, testId) => {
    mockPathname = `${gameOpsTemplateRoutePrefix}/${key}`;
    render(<TemplateGameOpsPage />);
    expect(screen.getByTestId(testId)).toBeInTheDocument();
    expect(screen.getByText(/默认菜单分类/)).toBeInTheDocument();
  });

  it('未知 key 兜底：不渲染模板，列出可用模板并可跳转', () => {
    mockPathname = '/console/templates/nope';
    render(<TemplateGameOpsPage />);
    expect(screen.queryByTestId('tpl-player')).not.toBeInTheDocument();
    expect(screen.getByText('ban-management')).toBeInTheDocument();
    fireEvent.click(screen.getByText('order-review'));
    expect(mockPush).toHaveBeenCalledWith('/console/templates/order-review');
  });
});
