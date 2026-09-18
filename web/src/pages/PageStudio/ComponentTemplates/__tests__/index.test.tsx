/** ComponentTemplates 页（T7）：requiredFunctions 命中 unbound 契约的模板
 * 卡片标注「未绑定」Tag（不置灰，依赖函数 tooltip 说明）；bound 依赖与
 * 无依赖模板不标注。 */
import React from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import { request } from '@umijs/max';
import ComponentTemplatesPage from '../index';
import { listDescriptors, type FunctionDescriptor } from '@/services/api/functions';

jest.mock('@/services/api/functions', () => ({
  ...jest.requireActual('@/services/api/functions'),
  listDescriptors: jest.fn(),
}));

const mockedList = listDescriptors as jest.Mock;
const mockedRequest = request as unknown as jest.Mock;

// 全页渲染（PageContainer + 组件库 fetch），放宽预算
jest.setTimeout(20000);
const FIND = { timeout: 5000 } as const;

const tpl = (key: string, requiredFunctions?: string[]) => ({
  key,
  name: `模板-${key}`,
  tree: [],
  builtin: true,
  requiredFunctions,
});

beforeEach(() => {
  mockedList.mockReset().mockResolvedValue([
    { id: 'player.get', executionState: 'unbound' },
    { id: 'player.list', executionState: 'bound' },
  ] as FunctionDescriptor[]);
  mockedRequest.mockReset().mockImplementation(async (url: string) => {
    if (typeof url === 'string' && url.includes('/api/v1/component-templates')) {
      return {
        items: [tpl('t-unbound', ['player.get']), tpl('t-bound', ['player.list']), tpl('t-plain')],
      };
    }
    return {};
  });
});

describe('ComponentTemplates unbound 标注（T7）', () => {
  it('依赖命中 unbound 契约 → 卡片「未绑定」；bound/无依赖不标注', async () => {
    render(
      <App>
        <ComponentTemplatesPage />
      </App>,
    );
    // fnById 异步加载完成后标注出现（仅依赖 player.get 的模板）
    await waitFor(() => expect(screen.getAllByText('未绑定')).toHaveLength(1), FIND);
    // 依赖行两卡都渲染（FormattedMessage mock 不插值，{fns} 为字面量）
    expect(screen.getAllByText('依赖：{fns}')).toHaveLength(2);
    // bound 依赖与无依赖卡片正常渲染、无标注
    expect(screen.getByText('模板-t-plain')).toBeInTheDocument();
    expect(screen.getByText('模板-t-bound')).toBeInTheDocument();
  });

  it('descriptors 拉取失败（空集降级）：卡片正常渲染、无 unbound 标注', async () => {
    mockedList.mockRejectedValue(new Error('boom'));
    render(
      <App>
        <ComponentTemplatesPage />
      </App>,
    );
    expect(await screen.findByText('模板-t-unbound', undefined, FIND)).toBeInTheDocument();
    expect(screen.queryByText('未绑定')).not.toBeInTheDocument();
  });
});

describe('ComponentTemplates 创建入口与分类体系', () => {
  it('「新建组合组件」主按钮 → 跳编辑器并带 createComponent 引导标记', async () => {
    const { history } = await import('@umijs/max');
    (history.push as jest.Mock).mockClear();
    render(
      <App>
        <ComponentTemplatesPage />
      </App>,
    );
    const btn = await screen.findByText('新建组合组件', undefined, FIND);
    btn.click();
    expect(history.push).toHaveBeenCalledWith(
      '/functions/pages/composite-editor?createComponent=1',
    );
    // 「创建组合页」仍保留（建页面，非组件），不再承担主入口
    expect(screen.getByText('创建组合页')).toBeInTheDocument();
  });

  it('分组按规范分类顺序展示（内置函数/查询/资源 → 组合组件 → 常量），未收录分类排尾', async () => {
    const catTpl = (key: string, category?: string) => ({
      key,
      name: `模板-${key}`,
      tree: [],
      builtin: !category,
      ...(category ? { category } : {}),
    });
    mockedRequest.mockReset().mockImplementation(async (url: string) => {
      if (typeof url === 'string' && url.includes('/api/v1/component-templates')) {
        return {
          items: [
            // 乱序给：常量/组合组件/未收录/内置各类，断言渲染分组顺序稳定
            catTpl('c-const', '常量'),
            catTpl('c-composite', '组合组件'),
            catTpl('c-other', '自由分类'),
            { ...catTpl('b-fn'), category: '函数组件' },
            { ...catTpl('b-crud'), category: '资源管理' },
            { ...catTpl('b-query'), category: '查询组合' },
          ],
        };
      }
      return {};
    });
    const { container } = render(
      <App>
        <ComponentTemplatesPage />
      </App>,
    );
    await waitFor(() => expect(screen.getByText('模板-c-other')).toBeInTheDocument(), FIND);
    const titles = Array.from(container.querySelectorAll('h5')).map((h) => h.textContent ?? '');
    // 计数后缀（0）来自 groupCount 插值 mock；只比对分组名顺序
    const order = titles.map((t) => t.replace(/（\d+）$/, ''));
    expect(order).toEqual(['函数组件', '查询组合', '资源管理', '组合组件', '常量', '自由分类']);
  });
});
