/** 保存为组件弹窗「更新已有模板」通道（V3）：
 * 1. 默认另存模式提交 POST（key 自动生成 custom-- 前缀）；
 * 2. 更新模式拉取列表后下拉只列自定义模板（builtin 不列），
 *    选中后表单预填，提交走 PUT /:key 且 tree 以当前画布选择覆盖；
 * 3. 更新模式未选模板点保存被拦（无 PUT，出警告文案）。 */
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { App } from 'antd';
import { request } from '@umijs/max';
import SaveComponentModal from '../SaveComponentModal';
import type { PageNode } from '../model';
import type { ComponentTemplateDTO } from '../ComponentLibrary';

const mockedRequest = request as unknown as jest.Mock;

// antd Modal + Select + 远端 options 链路在 jsdom 偏慢，放宽预算（同 componentLibraryPanel）
jest.setTimeout(20000);
const FIND = { timeout: 5000 } as const;

// 名称展示与 ComponentLibrary 同源（localizedText zh-CN 优先）
const CUSTOM_NAME = '玩家 CRUD';
const BUILTIN_NAME = '内置资源列表';

const customTpl: ComponentTemplateDTO = {
  key: 'crud--player',
  name: { 'zh-CN': '玩家 CRUD', 'en-US': 'Player CRUD' },
  requiredFunctions: ['player.list'],
  tree: [{ id: 'old', type: 'fnTable', props: {} }] as PageNode[],
  builtin: false,
};
const builtinTpl: ComponentTemplateDTO = {
  key: 'resource-manage',
  name: { 'zh-CN': '内置资源列表', 'en-US': 'Builtin resource list' },
  requiredFunctions: [],
  tree: [],
  builtin: true,
};

const modalState = {
  fnIds: ['player.list'],
  selectedNodes: [
    { id: 'n1', type: 'fnTable', props: { functionId: 'player.list', title: '列表' } },
    { id: 'n2', type: 'button', props: { title: '发邮件' } },
  ] as PageNode[],
  paramCandidates: [],
};

function renderModal(onClose = () => undefined, state = modalState) {
  return render(
    <App>
      <SaveComponentModal state={state} onClose={onClose} />
    </App>,
  );
}

function fillName(value: string) {
  fireEvent.change(screen.getByLabelText('组件名称'), { target: { value } });
}

/** Modal 底部确认按钮（antd 中文两字按钮渲染为「保 存」，正则容忍空格）。 */
function clickOk() {
  fireEvent.click(screen.getByRole('button', { name: /保\s*存/ }));
}

beforeEach(() => {
  // 清跨用例累积的调用记录（用例 3 断言「无 PUT」须不看见用例 2 的调用）
  mockedRequest.mockClear();
  mockedRequest.mockImplementation(async (url: string) => {
    if (typeof url === 'string' && url.includes('/api/v1/component-templates')) {
      return { items: [customTpl, builtinTpl] };
    }
    return {};
  });
});

describe('SaveComponentModal 保存方式两模式（V3 更新通道）', () => {
  it('默认另存新模板：POST /component-templates 且 key 自动生成', async () => {
    const onClose = jest.fn();
    renderModal(onClose);
    fillName('新模板');
    clickOk();
    await screen.findByText(/已保存/, undefined, FIND);
    const post = mockedRequest.mock.calls.find(
      (c) => typeof c[1] === 'object' && c[1]?.method === 'POST',
    );
    expect(post?.[0]).toBe('/api/v1/component-templates');
    expect(String(post?.[1]?.data?.key)).toMatch(/^custom--/);
    expect(post?.[1]?.data?.tree).toEqual(modalState.selectedNodes);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('更新模式：下拉只列自定义模板，选中预填后 PUT /:key 覆盖 tree', async () => {
    renderModal();
    // findByRole 等初始渲染稳定（antd Form 的 initialValues 在 effect 中写入，
    // render 后同步点击 radio 发生在 form store 挂载前，mode 切换丢失）
    fireEvent.click(await screen.findByRole('radio', { name: '更新已有模板' }, FIND));
    // 目标模板下拉（区别于分类下拉：按 antd Form 字段 id 定位）；
    // Radio 切换 → Form.useWatch 重渲染是异步的，先等 Select 出现
    const combo = await waitFor(
      () => {
        const el = document.querySelector('#targetKey');
        expect(el).toBeInTheDocument();
        return el as HTMLElement;
      },
      { timeout: 5000 },
    );
    fireEvent.mouseDown(combo);
    const option = await screen.findByText(CUSTOM_NAME, undefined, FIND);
    // builtin 模板不出现在更新下拉
    expect(screen.queryByText(BUILTIN_NAME)).not.toBeInTheDocument();
    fireEvent.click(option);
    // 预填发生在选中后（异步 effect），等待 input 反映模板现名
    await waitFor(
      () =>
        expect((screen.getByLabelText('组件名称') as HTMLInputElement).value).toMatch(CUSTOM_NAME),
      { timeout: 5000 },
    );
    clickOk();
    await screen.findByText(/已更新/, undefined, FIND);
    const put = mockedRequest.mock.calls.find(
      (c) => typeof c[1] === 'object' && c[1]?.method === 'PUT',
    );
    expect(put?.[0]).toBe('/api/v1/component-templates/crud--player');
    // body 携带 key（后端 Update 复用 CreateRequest 绑定，key 为 required）
    expect(put?.[1]?.data?.key).toBe('crud--player');
    // tree/requiredFunctions 以当前画布选择覆盖（不是模板旧 tree）
    expect(put?.[1]?.data?.tree).toEqual(modalState.selectedNodes);
    expect(put?.[1]?.data?.requiredFunctions).toEqual(['player.list']);
  });

  it('更新模式未选模板：点保存被拦（无 PUT，出警告）', async () => {
    renderModal();
    fireEvent.click(await screen.findByRole('radio', { name: '更新已有模板' }, FIND));
    fillName('改个名');
    clickOk();
    await screen.findByText('请选择要更新的模板', undefined, FIND);
    expect(mockedRequest.mock.calls.some((c) => c[1]?.method === 'PUT')).toBe(false);
  });

  it('名称为空：点保存被拦（无 POST，出警告）', async () => {
    renderModal();
    clickOk();
    await screen.findByText('请填写组件名称', undefined, FIND);
    expect(mockedRequest.mock.calls.some((c) => c[1]?.method === 'POST')).toBe(false);
  });

  it('参数化候选：渲染勾选组，勾选后 body 携带 params 定义', async () => {
    const withParams = {
      ...modalState,
      paramCandidates: [
        {
          key: 'tb1.title',
          nodeId: 'tb1',
          prop: 'title',
          propLabel: '标题',
          nodeTitle: '玩家列表',
          current: '旧标题',
        },
      ],
    };
    renderModal(undefined, withParams);
    expect(screen.getByText('参数化（勾选后拖入组件时可在弹窗中快速配置）')).toBeInTheDocument();
    const box = screen.getByRole('checkbox', { name: /玩家列表·标题/ });
    fireEvent.click(box);
    fillName('带参模板');
    clickOk();
    await screen.findByText(/已保存/, undefined, FIND);
    const post = mockedRequest.mock.calls.find(
      (c) => typeof c[1] === 'object' && c[1]?.method === 'POST',
    );
    expect(post?.[1]?.data?.params).toEqual([
      {
        key: 'tb1.title',
        label: { 'zh-CN': '玩家列表·标题' },
        nodeId: 'tb1',
        prop: 'title',
        default: '旧标题',
      },
    ]);
  });

  it('保存请求失败：出错误提示且不关闭弹窗', async () => {
    mockedRequest.mockImplementation(async (url: string) => {
      if (typeof url === 'string' && url.includes('/api/v1/component-templates')) {
        return { items: [] };
      }
      return {};
    });
    mockedRequest.mockRejectedValueOnce(new Error('boom')).mockRejectedValueOnce(new Error('boom'));
    const onClose = jest.fn();
    renderModal(onClose);
    fillName('会失败');
    clickOk();
    await screen.findByText('保存失败', undefined, FIND);
    expect(onClose).not.toHaveBeenCalled();
  });

  it('更新模式拉取列表失败：警告 + 下拉空（不阻断表单）', async () => {
    mockedRequest.mockImplementation(async (url: string) => {
      if (typeof url === 'string' && url.includes('/api/v1/component-templates')) {
        throw new Error('boom');
      }
      return {};
    });
    renderModal();
    fireEvent.click(await screen.findByRole('radio', { name: '更新已有模板' }, FIND));
    await screen.findByText('拉取自定义模板失败', undefined, FIND);
    // 拉取失败回退空列表：下拉可打开（无崩溃），保存仍被「未选模板」拦截
    expect(screen.queryByRole('option')).toBeNull();
  });

  it('列表响应兼容裸数组与无 items 信封（归一空表）', async () => {
    // 裸数组形态
    mockedRequest.mockImplementation(async () => [customTpl]);
    const { unmount } = renderModal();
    fireEvent.click(await screen.findByRole('radio', { name: '更新已有模板' }, FIND));
    const combo = await waitFor(() => {
      const el = document.querySelector('#targetKey');
      expect(el).toBeInTheDocument();
      return el as HTMLElement;
    }, FIND);
    fireEvent.mouseDown(combo);
    expect(await screen.findByText(CUSTOM_NAME, undefined, FIND)).toBeInTheDocument();
    unmount();

    // 信封无 items 键 → 空表
    mockedRequest.mockImplementation(async () => ({ foo: 1 }));
    renderModal();
    fireEvent.click(await screen.findByRole('radio', { name: '更新已有模板' }, FIND));
    await waitFor(() => expect(document.querySelector('#targetKey')).toBeInTheDocument(), FIND);
    expect(screen.queryByText(CUSTOM_NAME)).not.toBeInTheDocument();
  });

  it('state 置 null（弹窗关闭）：重置拉取态，重开时重新拉取', async () => {
    const listCalls = () =>
      mockedRequest.mock.calls.filter(
        (c) => typeof c[0] === 'string' && c[0] === '/api/v1/component-templates' && !c[1]?.method,
      ).length;
    const first = renderModal();
    fireEvent.click(await screen.findByRole('radio', { name: '更新已有模板' }, FIND));
    await waitFor(() => expect(document.querySelector('#targetKey')).toBeInTheDocument(), FIND);
    const afterOpen = listCalls();

    // 关闭：state null（effect 重置 customTemplates），再打开重新拉取
    first.rerender(
      <App>
        <SaveComponentModal state={null} onClose={() => undefined} />
      </App>,
    );
    first.rerender(
      <App>
        <SaveComponentModal state={modalState} onClose={() => undefined} />
      </App>,
    );
    fireEvent.click(await screen.findByRole('radio', { name: '更新已有模板' }, FIND));
    await waitFor(() => expect(document.querySelector('#targetKey')).toBeInTheDocument(), FIND);
    await waitFor(() => expect(listCalls()).toBeGreaterThan(afterOpen), FIND);
  });

  it('无依赖函数：摘要文案省略函数段（fns 空侧）', () => {
    renderModal(undefined, { ...modalState, fnIds: [] });
    expect(
      screen.getByText(/包含 2 个节点。保存后在组件库 Tab 拖入任意组合页复用。/),
    ).toBeInTheDocument();
  });
});
