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
  gameOpsTemplateCategories,
  gameOpsTemplateCrudRoles,
  gameOpsTemplateRoutePrefix,
  gameOpsTemplates,
  missingTemplateCrudRoles,
  templateFunctionRoles,
} from './gameOpsTemplates';
import TemplateGameOpsPage, { templateKeyFromPath } from '@/pages/Console/TemplateGameOps';
import { gameOpsDemoRuntimeStore } from './gameOpsDemoRuntime';

const mockPush = jest.fn();
let mockPathname = '/console/templates/player-manage';

jest.mock('@umijs/max', () => {
  // formatMessage 按 {key} 替换 values（与生产 Intl 行为一致，未声明的键原样输出）
  const formatMessage = (
    descriptor: { defaultMessage?: string },
    values?: Record<string, unknown>,
  ): string =>
    Object.entries(values ?? {}).reduce(
      (msg, [key, val]) => msg.split(`{${key}}`).join(String(val)),
      descriptor.defaultMessage ?? '',
    );
  return {
    useIntl: () => ({ formatMessage }),
    FormattedMessage: ({
      defaultMessage,
      values,
    }: {
      defaultMessage?: string;
      values?: Record<string, unknown>;
    }) => <span>{formatMessage({ defaultMessage }, values)}</span>,
    history: {
      // 工厂在变量初始化前被提升调用，这里用 getter 延迟读取外层可变状态
      get location() {
        return { pathname: mockPathname };
      },
      get push() {
        return mockPush;
      },
    },
  };
});

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

  it('每个模板声明能力分类、函数绑定与能力说明', () => {
    const categoryKeys = gameOpsTemplateCategories.map((c) => c.key);
    for (const entry of gameOpsTemplates) {
      expect(entry.categories.length).toBeGreaterThan(0);
      for (const category of entry.categories) {
        expect(categoryKeys).toContain(category);
      }
      expect(entry.functions.length).toBeGreaterThan(0);
      for (const fn of entry.functions) {
        expect(fn.id).toBeTruthy();
        expect(fn.role).toBeTruthy();
      }
      expect(entry.capabilitiesNote['zh-CN']).toBeTruthy();
      expect(entry.capabilitiesNote['en-US']).toBeTruthy();
    }
    // CRUD 能力轴固定为新建/编辑/删除，缺口查询不越界
    expect(gameOpsTemplateCrudRoles).toEqual(['create', 'update', 'delete']);
    for (const entry of gameOpsTemplates) {
      const roles = templateFunctionRoles(entry);
      for (const missing of missingTemplateCrudRoles(entry)) {
        expect(roles).not.toContain(missing);
        expect(gameOpsTemplateCrudRoles).toContain(missing);
      }
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

  it('兜底列表按四类能力分组，展示函数绑定与能力缺口提示', () => {
    mockPathname = '/console/templates/nope';
    render(<TemplateGameOpsPage />);

    // 四类能力分类标题齐备（任务监控由邮件与补偿模板承接）
    for (const category of gameOpsTemplateCategories) {
      expect(screen.getAllByText(new RegExp(category.label['zh-CN'])).length).toBeGreaterThan(0);
    }
    // 函数绑定以标签展示，玩家工作台与订单核查各出现一次
    expect(screen.getByText('player.list')).toBeInTheDocument();
    expect(screen.getByText('order.recredit')).toBeInTheDocument();
    // 能力缺口显式提示（不提供：… + 模板能力说明）
    expect(screen.getAllByText(/不提供：/).length).toBeGreaterThan(0);
    expect(screen.getByText(/不提供：create \/ update \/ delete/)).toBeInTheDocument();
  });

  it('示例数据重置入口恢复共享 store 初始状态', () => {
    mockPathname = `${gameOpsTemplateRoutePrefix}/player-manage`;
    gameOpsDemoRuntimeStore.players = [];
    gameOpsDemoRuntimeStore.opRecords = [];
    render(<TemplateGameOpsPage />);

    fireEvent.click(screen.getByRole('button', { name: '重置示例数据' }));

    expect(gameOpsDemoRuntimeStore.players).toHaveLength(26);
    expect(gameOpsDemoRuntimeStore.opRecords).toHaveLength(3);
  });
});
