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
