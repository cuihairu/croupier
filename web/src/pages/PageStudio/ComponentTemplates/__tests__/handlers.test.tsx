/**
 * ComponentTemplates 页操作链路（在 unbound 标注用例之外）：
 * 手动重建（成功计数回显 / 失败提示）、自定义模板删除（成功 / 失败）、
 * 旧版合并模板检测与一键清理（单条失败不阻断）、
 * 生成示例常量（新建 POST / 已存在幂等 info / 失败提示）、
 * 搜索过滤按名称/key/分类。
 */
import React from 'react';
import { render, screen, fireEvent, waitFor, configure } from '@testing-library/react';
import { App } from 'antd';
import { request, history } from '@umijs/max';
import ComponentTemplatesPage from '../index';
import { fieldsToSchemaJson } from '../../CompositeEditor/constants';
import { listDescriptors } from '@/services/api/functions';

jest.mock('@/services/api/functions', () => ({
  ...jest.requireActual('@/services/api/functions'),
  listDescriptors: jest.fn(),
}));

const mockedList = listDescriptors as jest.Mock;
const mockedRequest = request as unknown as jest.Mock;

jest.setTimeout(20000);
const FIND = { timeout: 5000 } as const;
configure({ asyncUtilTimeout: 5000 });

function staticFormNode(schemaJson: string) {
  return {
    id: 'n1',
    type: 'staticForm',
    props: { title: 't', span: 12, staticSchema: schemaJson },
  };
}

/** 旧版合并模板：一个 staticForm 塞两个常量 */
const legacyTree = [
  staticFormNode(
    fieldsToSchemaJson([
      { key: 'a', title: 'A', options: [{ value: 'a1' }] },
      { key: 'b', title: 'B', options: [{ value: 'b1' }] },
    ]),
  ),
];

const tpl = (key: string, overrides: Record<string, unknown> = {}) => ({
  key,
  name: `模板-${key}`,
  tree: [],
  builtin: false,
  ...overrides,
});

/** request mock：按 url + method 分发 */
function stubRequest(handler: (url: string, init: { method?: string }) => unknown) {
  mockedRequest
    .mockReset()
    .mockImplementation(async (url: string, init: { method?: string } = {}) => handler(url, init));
}

async function renderPage(items: Array<Record<string, unknown>>) {
  stubRequest((url) => {
    if (url.includes('/api/v1/component-templates/regenerate')) return { regenerated: 3 };
    if (url.includes('/api/v1/component-templates')) return { items };
    return {};
  });
  const utils = render(
    <App>
      <ComponentTemplatesPage />
    </App>,
  );
  await waitFor(() => expect(screen.getByText(`模板-${items[0].key}`)).toBeInTheDocument(), FIND);
  return utils;
}

beforeEach(() => {
  jest.clearAllMocks();
  mockedList.mockReset().mockResolvedValue([]);
});

describe('手动重建', () => {
  it('成功后回显重建数量', async () => {
    await renderPage([tpl('t-1')]);
    stubRequest((url) => {
      if (url.includes('/regenerate')) return { regenerated: 3 };
      if (url.includes('/api/v1/component-templates')) return { items: [tpl('t-1')] };
      return {};
    });
    // renderPage 内 stubRequest 已被再次覆盖前先点按钮
    fireEvent.click(screen.getByRole('button', { name: /手动重建（兜底）/ }));

    await waitFor(() =>
      expect(screen.getByText('已从 3 个契约重新生成内置组件')).toBeInTheDocument(),
    );
  });

  it('失败提示', async () => {
    await renderPage([tpl('t-1')]);
    stubRequest((url) => {
      if (url.includes('/regenerate')) throw new Error('boom');
      if (url.includes('/api/v1/component-templates')) return { items: [tpl('t-1')] };
      return {};
    });
    fireEvent.click(screen.getByRole('button', { name: /手动重建（兜底）/ }));

    await waitFor(() => expect(screen.getByText('重新生成失败')).toBeInTheDocument());
  });
});

describe('删除自定义模板', () => {
  it('Popconfirm 确认后 DELETE 并刷新', async () => {
    await renderPage([tpl('t-custom', { builtin: false })]);
    stubRequest((url, init) => {
      if (init.method === 'DELETE') return {};
      if (url.includes('/api/v1/component-templates')) return { items: [] };
      return {};
    });

    fireEvent.click(screen.getByRole('button', { name: /删\s*除/ }));
    fireEvent.click(screen.getByRole('button', { name: /OK|确\s*定/ }));
    await waitFor(() => expect(screen.getByText('已删除 t-custom')).toBeInTheDocument());
  });

  it('删除失败提示（内置组件不可删除）', async () => {
    await renderPage([tpl('t-custom', { builtin: false })]);
    stubRequest((url, init) => {
      if (init.method === 'DELETE') throw new Error('builtin');
      if (url.includes('/api/v1/component-templates')) return { items: [tpl('t-custom')] };
      return {};
    });

    fireEvent.click(screen.getByRole('button', { name: /删\s*除/ }));
    fireEvent.click(screen.getByRole('button', { name: /OK|确\s*定/ }));
    await waitFor(() =>
      expect(screen.getByText('删除失败（内置组件不可删除）')).toBeInTheDocument(),
    );
  });
});

describe('旧版合并模板清理', () => {
  it('检测到旧模板显示 Alert，一键清理逐条 DELETE', async () => {
    stubRequest((url, init) => {
      if (init.method === 'DELETE') return {};
      if (url.includes('/api/v1/component-templates')) {
        return { items: [tpl('consts--merged', { tree: legacyTree })] };
      }
      return {};
    });
    render(
      <App>
        <ComponentTemplatesPage />
      </App>,
    );

    await waitFor(() =>
      expect(screen.getByText(/检测到 1 个旧版合并常量模板/)).toBeInTheDocument(),
    );
    fireEvent.click(screen.getByRole('button', { name: /一键清理旧模板/ }));

    await waitFor(() => expect(screen.getByText(/已清理 1 个旧版合并模板/)).toBeInTheDocument());
    const deleteCalls = mockedRequest.mock.calls.filter(
      ([, init]) => (init as { method?: string }).method === 'DELETE',
    );
    expect(deleteCalls).toHaveLength(1);
  });

  it('清理中单条失败不阻断其余（removed 计数只含成功）', async () => {
    stubRequest((url, init) => {
      if (init.method === 'DELETE') {
        const key = String(url).split('/').pop();
        if (key === 'consts--merged-a') throw new Error('builtin');
        return {};
      }
      if (url.includes('/api/v1/component-templates')) {
        return {
          items: [
            tpl('consts--merged-a', { tree: legacyTree }),
            tpl('consts--merged-b', { tree: legacyTree }),
          ],
        };
      }
      return {};
    });
    render(
      <App>
        <ComponentTemplatesPage />
      </App>,
    );

    await waitFor(() =>
      expect(screen.getByText(/检测到 2 个旧版合并常量模板/)).toBeInTheDocument(),
    );
    fireEvent.click(screen.getByRole('button', { name: /一键清理旧模板/ }));

    await waitFor(() => expect(screen.getByText(/已清理 1 个旧版合并模板/)).toBeInTheDocument());
  });
});

describe('生成示例常量', () => {
  it('逐个 POST consts--demo-* 模板并回显数量', async () => {
    await renderPage([tpl('t-1')]);
    const posts: Array<Record<string, unknown>> = [];
    stubRequest((url, init) => {
      if (init.method === 'POST' && url.includes('/api/v1/component-templates')) {
        posts.push((init as { data?: Record<string, unknown> }).data ?? {});
        return {};
      }
      if (url.includes('/api/v1/component-templates')) return { items: [tpl('t-1')] };
      return {};
    });

    fireEvent.click(screen.getByRole('button', { name: /生成示例常量/ }));
    await waitFor(() => expect(screen.getByText(/已生成 4 个示例常量组件/)).toBeInTheDocument());
    expect(posts.map((p) => p.key)).toEqual([
      'consts--demo-ban-reason',
      'consts--demo-vip-level',
      'consts--demo-server-status',
      'consts--demo-pay-channel',
    ]);
  });

  it('已全部存在时幂等提示，不发 POST', async () => {
    const demoKeys = [
      'consts--demo-ban-reason',
      'consts--demo-vip-level',
      'consts--demo-server-status',
      'consts--demo-pay-channel',
    ];
    stubRequest((url) => {
      if (url.includes('/api/v1/component-templates')) {
        return { items: demoKeys.map((k) => tpl(k)) };
      }
      return {};
    });
    render(
      <App>
        <ComponentTemplatesPage />
      </App>,
    );

    fireEvent.click(
      await waitFor(() => screen.getByRole('button', { name: /生成示例常量/ }), FIND),
    );
    await waitFor(() => expect(screen.getByText('示例常量模板已存在')).toBeInTheDocument());
    expect(
      mockedRequest.mock.calls.filter(
        ([, init]) => (init as { method?: string }).method === 'POST',
      ),
    ).toHaveLength(0);
  });

  it('POST 失败提示', async () => {
    await renderPage([tpl('t-1')]);
    stubRequest((url, init) => {
      if (init.method === 'POST') throw new Error('boom');
      if (url.includes('/api/v1/component-templates')) return { items: [tpl('t-1')] };
      return {};
    });

    fireEvent.click(screen.getByRole('button', { name: /生成示例常量/ }));
    await waitFor(() => expect(screen.getByText('生成示例常量模板失败')).toBeInTheDocument());
  });
});

describe('搜索过滤', () => {
  it('按 key 过滤（名称/key/分类）', async () => {
    stubRequest((url) => {
      if (url.includes('/api/v1/component-templates')) {
        return {
          items: [
            tpl('player-card', { name: '玩家卡片' }),
            tpl('order-table', { name: '订单表格' }),
          ],
        };
      }
      return {};
    });
    render(
      <App>
        <ComponentTemplatesPage />
      </App>,
    );
    await waitFor(() => expect(screen.getByText('玩家卡片')).toBeInTheDocument(), FIND);

    fireEvent.change(screen.getByPlaceholderText('搜索组件名 / key / 分类'), {
      target: { value: 'order' },
    });
    await waitFor(() => expect(screen.queryByText('玩家卡片')).not.toBeInTheDocument());
    expect(screen.getByText('订单表格')).toBeInTheDocument();
  });
});

describe('创建组合页入口', () => {
  it('按钮跳转组合页编辑器', async () => {
    await renderPage([tpl('t-1')]);
    fireEvent.click(screen.getByRole('button', { name: /创建组合页/ }));
    expect(history.push).toHaveBeenCalledWith('/functions/pages/composite-editor');
  });
});
