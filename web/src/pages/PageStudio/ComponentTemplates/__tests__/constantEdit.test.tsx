/** 单条常量编辑（OPEN-ISSUES #6）：
 * - ConstantEditModal 编辑模式：staticSchema 回填、改选项/显示名 → PUT 更新、
 *   失败降级提示、空模板校验
 * - 新增模式：默认一行常量、逐字段拆分 POST（与导入通道同语义）、多字段拆分
 * - 页面集成：仅「用户自建常量模板」卡片出「编辑」；内置/组合组件不出；
 *   顶栏「新增常量」入口可打开弹窗 */
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import { request } from '@umijs/max';
import ComponentTemplatesPage from '../index';
import ConstantEditModal from '../ConstantEditModal';
import { listDescriptors, type FunctionDescriptor } from '@/services/api/functions';

jest.mock('@/services/api/functions', () => ({
  ...jest.requireActual('@/services/api/functions'),
  listDescriptors: jest.fn(),
}));

const mockedList = listDescriptors as jest.Mock;
const mockedRequest = request as unknown as jest.Mock;

jest.setTimeout(20000);
const FIND = { timeout: 5000 } as const;

const schema = JSON.stringify({
  type: 'object',
  properties: { 阵营: { type: 'string', title: '阵营', enum: ['联盟', '部落'] } },
});

const constTpl = {
  key: 'consts--z1',
  name: { 'zh-CN': '阵营', 'en-US': '阵营' },
  builtin: false,
  category: '常量',
  tree: [
    {
      id: 'sf1',
      type: 'staticForm',
      props: { title: '阵营', span: 12, staticSchema: schema },
    },
  ],
};

beforeEach(() => {
  mockedList.mockReset().mockResolvedValue([] as FunctionDescriptor[]);
  mockedRequest.mockReset().mockResolvedValue({});
});

describe('ConstantEditModal 编辑模式', () => {
  it('回填 staticSchema → 改选项 → 保存走 PUT 且 name/树同步', async () => {
    const onSaved = jest.fn();
    render(
      <App>
        <ConstantEditModal open template={constTpl} onCancel={jest.fn()} onSaved={onSaved} />
      </App>,
    );
    // 选项编辑区回填既有选项
    const options = screen.getByPlaceholderText(/每行一个选项/);
    expect((options as HTMLTextAreaElement).value).toContain('联盟');
    expect((options as HTMLTextAreaElement).value).toContain('部落');

    fireEvent.change(options, { target: { value: '联盟\n部落\n中立' } });
    fireEvent.click(screen.getByRole('button', { name: /保\s*存/ }));

    await waitFor(() => expect(onSaved).toHaveBeenCalled(), FIND);
    const put = mockedRequest.mock.calls.find(
      ([url, init]) =>
        typeof url === 'string' &&
        url.includes('/api/v1/component-templates/consts--z1') &&
        (init as { method?: string })?.method === 'PUT',
    );
    expect(put).toBeTruthy();
    const data = (
      put as unknown as [
        string,
        {
          data: { name: Record<string, string>; tree: Array<{ props: { staticSchema: string } }> };
        },
      ]
    )[1].data;
    expect(data.name['zh-CN']).toBe('阵营');
    expect(JSON.stringify(data.tree)).toContain('中立');
  });

  it('保存失败 → 弹窗内错误提示，不回调 onSaved', async () => {
    mockedRequest.mockRejectedValue(new Error('boom'));
    const onSaved = jest.fn();
    render(
      <App>
        <ConstantEditModal open template={constTpl} onCancel={jest.fn()} onSaved={onSaved} />
      </App>,
    );
    fireEvent.click(screen.getByRole('button', { name: /保\s*存/ }));
    await waitFor(() => expect(screen.getByText('boom')).toBeInTheDocument(), FIND);
    expect(onSaved).not.toHaveBeenCalled();
  });

  it('删除唯一常量后保存 → 空模板校验拦截', async () => {
    const onSaved = jest.fn();
    render(
      <App>
        <ConstantEditModal open template={constTpl} onCancel={jest.fn()} onSaved={onSaved} />
      </App>,
    );
    const delBtn = document.querySelector('.anticon-delete')?.closest('button');
    expect(delBtn).not.toBeNull();
    fireEvent.click(delBtn as HTMLButtonElement);
    fireEvent.click(screen.getByRole('button', { name: /保\s*存/ }));
    expect(await screen.findByText(/请至少保留一个常量/, undefined, FIND)).toBeInTheDocument();
    expect(onSaved).not.toHaveBeenCalled();
  });
});

describe('ConstantEditModal 新增模式', () => {
  it('默认一行常量 → 保存 POST consts--*，name 取显示名', async () => {
    const onSaved = jest.fn();
    render(
      <App>
        <ConstantEditModal open template={null} onCancel={jest.fn()} onSaved={onSaved} />
      </App>,
    );
    fireEvent.click(screen.getByRole('button', { name: /保\s*存/ }));
    await waitFor(() => expect(onSaved).toHaveBeenCalledWith('已创建 1 个常量组件'), FIND);
    const post = mockedRequest.mock.calls.find(
      ([url, init]) =>
        typeof url === 'string' &&
        url === '/api/v1/component-templates' &&
        (init as { method?: string })?.method === 'POST',
    );
    expect(post).toBeTruthy();
    const data = (
      post as unknown as [string, { data: { key: string; name: Record<string, string> } }]
    )[1].data;
    expect(data.key).toMatch(/^consts--/);
    expect(data.name['zh-CN']).toBe('新常量');
  });

  it('多字段拆分逐条 POST（一种常量一个组件，与导入同语义）', async () => {
    const onSaved = jest.fn();
    render(
      <App>
        <ConstantEditModal open template={null} onCancel={jest.fn()} onSaved={onSaved} />
      </App>,
    );
    fireEvent.click(screen.getByRole('button', { name: /添加常量/ }));
    fireEvent.click(screen.getByRole('button', { name: /保\s*存/ }));
    await waitFor(() => expect(onSaved).toHaveBeenCalledWith('已创建 2 个常量组件'), FIND);
    const posts = mockedRequest.mock.calls.filter(
      ([url, init]) =>
        typeof url === 'string' &&
        url === '/api/v1/component-templates' &&
        (init as { method?: string })?.method === 'POST',
    );
    expect(posts).toHaveLength(2);
  });
});

describe('页面集成：编辑入口与新增入口', () => {
  function stubPage(items: Array<Record<string, unknown>>) {
    mockedRequest.mockImplementation(async (url: string, init: { method?: string } = {}) => {
      if (init.method) return {};
      if (typeof url === 'string' && url.includes('/api/v1/component-templates')) {
        return { items };
      }
      return {};
    });
  }

  it('仅用户自建常量卡片出「编辑」；内置/组合组件不出；顶栏「新增常量」打开弹窗', async () => {
    stubPage([
      constTpl,
      { key: 'b-const', name: '内置常量', builtin: true, category: '常量', tree: [] },
      { key: 'u-comp', name: '组合组件', builtin: false, category: '组合组件', tree: [] },
    ]);
    render(
      <App>
        <ComponentTemplatesPage />
      </App>,
    );
    expect(await screen.findByText('阵营', undefined, FIND)).toBeInTheDocument();
    // 编辑按钮恰好一个（内置常量/组合组件卡片无）
    expect(screen.getAllByRole('button', { name: /编辑$/ })).toHaveLength(1);

    // 顶栏新增常量 → 弹窗以新增态打开
    fireEvent.click(screen.getByRole('button', { name: /新增常量/ }));
    expect(
      await screen.findByText('新增常量', { selector: '.ant-modal-title' }, FIND),
    ).toBeInTheDocument();
  });

  it('卡片「编辑」→ 弹窗编辑态回填该模板', async () => {
    stubPage([constTpl]);
    render(
      <App>
        <ComponentTemplatesPage />
      </App>,
    );
    await waitFor(() => expect(screen.getByText('阵营')).toBeInTheDocument(), FIND);
    fireEvent.click(screen.getByRole('button', { name: /编辑$/ }));
    await screen.findByText('编辑常量', { selector: '.ant-modal-title' }, FIND);
    expect((screen.getByPlaceholderText(/每行一个选项/) as HTMLTextAreaElement).value).toContain(
      '部落',
    );
  });
});

describe('ConstantEditModal 边界与关闭路径（round-7 覆盖率巡检）', () => {
  it('编辑模板树无 staticForm → constantSchemaOf 回退空 → 保存被空模板校验拦下且不发请求', async () => {
    const noFormTpl = {
      key: 'consts--nf',
      name: { 'zh-CN': '无表单模板', 'en-US': 'no-form' },
      builtin: false,
      tree: [],
    };
    render(
      <App>
        <ConstantEditModal open template={noFormTpl} onCancel={jest.fn()} onSaved={jest.fn()} />
      </App>,
    );
    // 无 staticForm 可回填 → fields 空 → 保存触发空模板校验（不发 PUT/POST）
    fireEvent.click(screen.getByRole('button', { name: /保\s*存/ }));
    expect(
      await screen.findByText('请至少保留一个常量（空模板无法渲染下拉）', undefined, FIND),
    ).toBeInTheDocument();
    expect(mockedRequest).not.toHaveBeenCalled();
  });

  it('legacy 多常量模板整组回写：描述为「常量组件（N 个常量）」、name 取模板显示名', async () => {
    const schema2 = JSON.stringify({
      type: 'object',
      properties: {
        阵营: { type: 'string', title: '阵营', enum: ['联盟', '部落'] },
        段位: { type: 'string', title: '段位', enum: ['青铜', '王者'] },
      },
    });
    const legacyTpl = {
      key: 'consts--legacy',
      name: { 'zh-CN': '旧合并模板', 'en-US': 'legacy-bundle' },
      builtin: false,
      tree: [
        {
          id: 'sf1',
          type: 'staticForm',
          props: { title: '旧合并模板', span: 12, staticSchema: schema2 },
        },
      ],
    };
    render(
      <App>
        <ConstantEditModal open template={legacyTpl} onCancel={jest.fn()} onSaved={jest.fn()} />
      </App>,
    );
    fireEvent.click(screen.getByRole('button', { name: /保\s*存/ }));
    await waitFor(
      () =>
        expect(screen.getByRole('button', { name: /保\s*存/ })).not.toHaveClass('ant-btn-loading'),
      FIND,
    );
    const put = mockedRequest.mock.calls.find(
      ([url, init]) =>
        typeof url === 'string' &&
        url.includes('/api/v1/component-templates/consts--legacy') &&
        (init as { method?: string })?.method === 'PUT',
    );
    expect(put).toBeTruthy();
    const data = (
      put as unknown as [
        string,
        {
          data: {
            name: Record<string, string>;
            description: Record<string, string>;
            tree: unknown[];
          };
        },
      ]
    )[1].data;
    // 多常量：displayName 回落模板显示名（localizedText 取 zh-CN），且 payloadBase
    // 将同一 displayName 写入 zh-CN/en-US 两键
    expect(data.name['zh-CN']).toBe('旧合并模板');
    expect(data.name['en-US']).toBe('旧合并模板');
    expect(data.description['zh-CN']).toBe('常量组件（2 个常量）');
    expect(data.description['en-US']).toBe('Constant component (2 constants)');
  });

  it('保存请求 reject 非 Error 值 → 显示「保存失败」兜底文案', async () => {
    mockedRequest.mockRejectedValueOnce('boom');
    render(
      <App>
        <ConstantEditModal open template={constTpl} onCancel={jest.fn()} onSaved={jest.fn()} />
      </App>,
    );
    fireEvent.click(screen.getByRole('button', { name: /保\s*存/ }));
    // reject 的是字符串而非 Error → e instanceof Error 为假 → intl 兜底文案
    expect(await screen.findByText('保存失败', undefined, FIND)).toBeInTheDocument();
    expect(screen.queryByText(/consts--z1/)).not.toBeInTheDocument();
  });

  it('footer「取 消」与弹窗右上角 X 均复位并回调 onCancel；重开后重新回填', async () => {
    const onCancel = jest.fn();
    const view = render(
      <App>
        <ConstantEditModal open template={constTpl} onCancel={onCancel} onSaved={jest.fn()} />
      </App>,
    );
    const options = () => screen.getByPlaceholderText(/每行一个选项/) as HTMLTextAreaElement;
    await screen.findByText('编辑常量', { selector: '.ant-modal-title' }, FIND);
    // 篡改脏状态 → footer 取消 → onCancel + 复位
    fireEvent.change(options(), { target: { value: '脏数据' } });
    fireEvent.click(screen.getByRole('button', { name: /取\s*消/ }));
    expect(onCancel).toHaveBeenCalledTimes(1);

    // 关闭后重开 → effect 重新按模板回填（证明 reset 生效）
    view.rerender(
      <App>
        <ConstantEditModal
          open={false}
          template={constTpl}
          onCancel={onCancel}
          onSaved={jest.fn()}
        />
      </App>,
    );
    view.rerender(
      <App>
        <ConstantEditModal open template={constTpl} onCancel={onCancel} onSaved={jest.fn()} />
      </App>,
    );
    await waitFor(() => expect(options().value).toContain('部落'), FIND);

    // X 关闭（scope 到可见弹窗根，避免命中残留弹窗壳）
    const modalRoot = screen
      .getByText('编辑常量', { selector: '.ant-modal-title' })
      .closest('.ant-modal');
    fireEvent.click(modalRoot?.querySelector('.ant-modal-close') as HTMLElement);
    expect(onCancel).toHaveBeenCalledTimes(2);
  });
});
