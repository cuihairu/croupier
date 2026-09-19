/** ComponentTemplates 页（T7）：requiredFunctions 命中 unbound 契约的模板
 * 卡片标注「未绑定」Tag（不置灰，依赖函数 tooltip 说明）；bound 依赖与
 * 无依赖模板不标注。
 *
 * 补充覆盖（v8 branch/function 缺口）：
 * - 列表加载形态（裸数组遗留形态 / 缺 items 降级 / 失败 catch 提示）与刷新按钮
 * - 手动重建响应缺 regenerated 字段回显 0
 * - 分类排序兜底（未收录分类间 localeCompare、规范分类整体靠前）
 * - stale 模板「已过期」Tag
 * - 预览弹窗（标题/结构摘要 treeSummary/界面预览/JSON 切换/关闭重开/null tree 遗留数据）
 * - 导入常量弹窗页内联动（onSaved 提示+刷新 / onCancel 关闭并清空） */
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
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

// ==================== 覆盖率缺口补充（v8 branch/function）====================

/** 新增用例的模板 fixture（可覆写字段） */
const pageTpl = (key: string, overrides: Record<string, unknown> = {}) => ({
  key,
  name: `模板-${key}`,
  tree: [],
  builtin: true,
  ...overrides,
});

/** request stub：GET 返回固定 items，带 method 的写请求返回空对象 */
function stubTemplates(items: Array<Record<string, unknown>>) {
  mockedRequest
    .mockReset()
    .mockImplementation(async (url: string, init: { method?: string } = {}) => {
      if (init.method) return {};
      if (typeof url === 'string' && url.includes('/api/v1/component-templates')) {
        return { items };
      }
      return {};
    });
}

/** 统计列表 GET 请求次数（无 method 的 component-templates 调用） */
const pageGets = () =>
  mockedRequest.mock.calls.filter(
    ([url, init]) =>
      typeof url === 'string' &&
      url.includes('/api/v1/component-templates') &&
      !(init as { method?: string } | undefined)?.method,
  );

describe('列表加载形态与失败（load 兜底分支）', () => {
  it('响应为裸数组（遗留形态）按数组直接渲染', async () => {
    mockedRequest.mockReset().mockResolvedValue([pageTpl('t-arr-a'), pageTpl('t-arr-b')]);
    render(
      <App>
        <ComponentTemplatesPage />
      </App>,
    );
    expect(await screen.findByText('模板-t-arr-a', undefined, FIND)).toBeInTheDocument();
    expect(screen.getByText('模板-t-arr-b')).toBeInTheDocument();
  });

  it('响应缺 items 字段降级为空列表（Empty 兜底）', async () => {
    mockedRequest.mockReset().mockResolvedValue({ total: 0 });
    render(
      <App>
        <ComponentTemplatesPage />
      </App>,
    );
    expect(await screen.findByText(/暂无组件模板/, undefined, FIND)).toBeInTheDocument();
  });

  it('响应体为空（204 形态，resp nullish）按空列表降级', async () => {
    mockedRequest.mockReset().mockResolvedValue(undefined);
    render(
      <App>
        <ComponentTemplatesPage />
      </App>,
    );
    expect(await screen.findByText(/暂无组件模板/, undefined, FIND)).toBeInTheDocument();
  });

  it('加载失败 catch 提示「加载组件模板失败」', async () => {
    mockedRequest.mockReset().mockRejectedValue(new Error('boom'));
    render(
      <App>
        <ComponentTemplatesPage />
      </App>,
    );
    await waitFor(() => expect(screen.getByText('加载组件模板失败')).toBeInTheDocument(), FIND);
  });

  it('顶部刷新按钮（图标按钮）重新拉取列表', async () => {
    stubTemplates([pageTpl('t-reload')]);
    const { container } = render(
      <App>
        <ComponentTemplatesPage />
      </App>,
    );
    await waitFor(() => expect(screen.getByText('模板-t-reload')).toBeInTheDocument(), FIND);
    expect(pageGets()).toHaveLength(1);

    // 纯图标按钮无可访问名，按图标类定位
    const reloadBtn = container.querySelector('.anticon-reload')?.closest('button');
    expect(reloadBtn).not.toBeNull();
    fireEvent.click(reloadBtn as HTMLButtonElement);
    await waitFor(() => expect(pageGets()).toHaveLength(2), FIND);
  });
});

describe('手动重建计数字段缺省', () => {
  it('regenerate 响应缺 regenerated 字段 → 回显 0 个契约', async () => {
    stubTemplates([pageTpl('t-regen')]);
    render(
      <App>
        <ComponentTemplatesPage />
      </App>,
    );
    await waitFor(() => expect(screen.getByText('模板-t-regen')).toBeInTheDocument(), FIND);

    fireEvent.click(screen.getByRole('button', { name: /手动重建（兜底）/ }));
    // stubTemplates 对带 method 的请求（POST regenerate）返回 {} → count 兜底 0
    await waitFor(
      () => expect(screen.getByText('已从 0 个契约重新生成内置组件')).toBeInTheDocument(),
      FIND,
    );
  });
});

describe('分类排序兜底分支', () => {
  it('未收录分类之间按 localeCompare，整体排在全部规范分类之后', async () => {
    const cat = (key: string, category: string) => ({
      key,
      name: `模板-${key}`,
      tree: [],
      builtin: false,
      category,
    });
    // 乱序给两个未收录分类 + 两个规范分类，迫使比较器双向比较（已收录 vs 未收录）
    stubTemplates([
      cat('c-zeta', 'Zeta类'),
      cat('b-fn', '函数组件'),
      cat('c-alpha', 'Alpha类'),
      cat('b-const', '常量'),
    ]);
    const { container } = render(
      <App>
        <ComponentTemplatesPage />
      </App>,
    );
    await waitFor(() => expect(screen.getByText('模板-c-alpha')).toBeInTheDocument(), FIND);
    const titles = Array.from(container.querySelectorAll('h5')).map((h) => h.textContent ?? '');
    const order = titles.map((t) => t.replace(/（\d+）$/, ''));
    // 规范分类按 CATEGORY_ORDER（函数组件 < 常量），未收录分类按字母序垫后
    expect(order).toEqual(['函数组件', '常量', 'Alpha类', 'Zeta类']);
  });
});

describe('模板卡片标注（stale）', () => {
  it('stale 模板渲染「已过期」Tag', async () => {
    stubTemplates([pageTpl('t-stale', { stale: true })]);
    render(
      <App>
        <ComponentTemplatesPage />
      </App>,
    );
    expect(await screen.findByText('已过期', undefined, FIND)).toBeInTheDocument();
  });
});

describe('预览弹窗', () => {
  /** 覆盖 treeSummary 全部分支：functionId / title 回退 / 无 type 与无 fn 节点 */
  const schemaJson = JSON.stringify({
    type: 'object',
    properties: { 阵营: { type: 'string', title: '阵营', enum: ['联盟', '部落'] } },
  });
  const previewTreeNodes = [
    { id: 'n-fn', type: 'fnTable', props: { functionId: 'player.list' } },
    { id: 'n-sf', type: 'staticForm', props: { title: '阵营', staticSchema: schemaJson } },
    // 遗留脏数据：缺 type → treeSummary 输出 '?'
    { id: 'n-bare', props: {} },
  ];

  it('标题/结构摘要/界面预览，切 JSON 页签展示模板树原文，关闭后可重开', async () => {
    stubTemplates([pageTpl('t-preview', { tree: previewTreeNodes })]);
    render(
      <App>
        <ComponentTemplatesPage />
      </App>,
    );
    await waitFor(() => expect(screen.getByText('模板-t-preview')).toBeInTheDocument(), FIND);

    fireEvent.click(screen.getByRole('button', { name: /预\s*览/ }));
    // 弹窗标题取模板名（区分卡片同名文本：限定 .ant-modal-title）
    await screen.findByText('模板-t-preview', { selector: '.ant-modal-title' }, FIND);
    // FormattedMessage mock 不插值，结构摘要文本为字面量（treeSummary 已被求值调用）
    expect(screen.getByText('结构：{structure}')).toBeInTheDocument();
    expect(screen.getByText('界面预览')).toBeInTheDocument();
    // 界面预览 tab：staticForm 实例化后渲染真实下拉
    expect(screen.getByRole('combobox')).toBeInTheDocument();

    // 切 JSON 页签（Segmented onChange）→ 展示模板树 JSON 原文
    fireEvent.click(screen.getByText('JSON'));
    await waitFor(() => {
      const pre = document.querySelector('.ant-modal pre');
      expect(pre?.textContent).toContain('"functionId": "player.list"');
      expect(pre?.textContent).toContain('"title": "阵营"');
    }, FIND);

    // 右上角 X 关闭（Modal onCancel → setPreviewKey(null)）→ 可再次打开
    // Modal 渲染在 body portal，须从 document 查找（仓库既有先例同款）
    const closeIcon = document.querySelector('.ant-modal-close') as HTMLElement | null;
    expect(closeIcon).not.toBeNull();
    fireEvent.click(closeIcon as HTMLElement);
    fireEvent.click(screen.getByRole('button', { name: /预\s*览/ }));
    await screen.findByText('模板-t-preview', { selector: '.ant-modal-title' }, FIND);
  });

  it('tree 为 null 的遗留模板可预览（非数组守卫 + JSON 页为 null，不崩溃）', async () => {
    stubTemplates([pageTpl('t-null-tree', { tree: null })]);
    render(
      <App>
        <ComponentTemplatesPage />
      </App>,
    );
    await waitFor(() => expect(screen.getByText('模板-t-null-tree')).toBeInTheDocument(), FIND);

    fireEvent.click(screen.getByRole('button', { name: /预\s*览/ }));
    await screen.findByText('模板-t-null-tree', { selector: '.ant-modal-title' }, FIND);
    expect(screen.getByText('结构：{structure}')).toBeInTheDocument();

    fireEvent.click(screen.getByText('JSON'));
    await waitFor(
      () => expect(document.querySelector('.ant-modal pre')?.textContent).toContain('null'),
      FIND,
    );
  });
});

describe('导入常量弹窗（页内联动）', () => {
  /** 打开页内导入常量弹窗并等待上传控件就绪 */
  async function openImportModal() {
    fireEvent.click(screen.getByRole('button', { name: /导入常量/ }));
    await waitFor(() => expect(document.querySelector('input[type=file]')).not.toBeNull(), FIND);
    return document.querySelector('input[type=file]') as HTMLInputElement;
  }

  it('打开 → 上传 JSON → 全部保存：onSaved 提示并重新拉取列表', async () => {
    const posts: Array<Record<string, unknown>> = [];
    mockedRequest
      .mockReset()
      .mockImplementation(
        async (url: string, init: { method?: string; data?: Record<string, unknown> } = {}) => {
          if (init.method === 'POST' && url.includes('/api/v1/component-templates')) {
            posts.push(init.data ?? {});
            return {};
          }
          if (typeof url === 'string' && url.includes('/api/v1/component-templates')) {
            return { items: [pageTpl('t-import')] };
          }
          return {};
        },
      );
    render(
      <App>
        <ComponentTemplatesPage />
      </App>,
    );
    await waitFor(() => expect(screen.getByText('模板-t-import')).toBeInTheDocument(), FIND);
    expect(pageGets()).toHaveLength(1);

    const fileInput = await openImportModal();
    fireEvent.change(fileInput, {
      target: {
        files: [
          new File(['{"阵营":["联盟","部落"],"稀有度":["传说"]}'], 'consts.json', {
            type: 'application/json',
          }),
        ],
      },
    });
    const saveBtn = await screen.findByRole('button', { name: /全部保存（2 个组件）/ }, FIND);
    fireEvent.click(saveBtn);

    // 页面 onSaved 回调：成功提示 + 重新 load（GET 计数 +1）
    await waitFor(
      () =>
        expect(
          screen.getByText('常量模板已保存——组合页编辑器组件库中可拖入使用'),
        ).toBeInTheDocument(),
      FIND,
    );
    await waitFor(() => expect(pageGets()).toHaveLength(2), FIND);
    // 每个常量独立 POST，key 前缀 consts--
    expect(posts).toHaveLength(2);
    expect(String(posts[0]?.key)).toMatch(/^consts--/);
  });

  it('取消：onCancel 关闭弹窗，重开后弹窗内状态已清空', async () => {
    stubTemplates([pageTpl('t-cancel')]);
    render(
      <App>
        <ComponentTemplatesPage />
      </App>,
    );
    await waitFor(() => expect(screen.getByText('模板-t-cancel')).toBeInTheDocument(), FIND);

    const fileInput = await openImportModal();
    fireEvent.change(fileInput, {
      target: {
        files: [new File(['{"阵营":["联盟"]}'], 'consts.json', { type: 'application/json' })],
      },
    });
    expect(
      await screen.findByRole('button', { name: /全部保存（1 个组件）/ }, FIND),
    ).toBeInTheDocument();

    // 弹窗底部取消 → 弹窗内部 reset + 页面 onCancel（importOpen 关闭）
    fireEvent.click(screen.getByRole('button', { name: /取\s*消/ }));
    // 重新打开：保存计数归零证明取消链路（reset + onCancel）完整执行
    fireEvent.click(screen.getByRole('button', { name: /导入常量/ }));
    expect(
      await screen.findByRole('button', { name: /全部保存（0 个组件）/ }, FIND),
    ).toBeInTheDocument();
  });
});
