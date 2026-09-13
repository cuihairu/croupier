/** PropsPanel 编辑器分桶与 VarNameInput 覆盖：
 * staticSchema 分桶（ConstantFieldsEditor 渲染+变更落 patch）、
 * condition 分桶（ConditionEditor 渲染）、VarNameInput（遗留 key 提示/
 * 合法改名走 onRenameVariable/无回调回退 onPatch/格式与冲突错误不提交/
 * 空串未变不提交/回车提交）、ParamMappingEditor 变更落 patch。 */
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import PropsPanel from '../PropsPanel';
import { registerBuiltinComponents } from '../components/builtin';
import type { PageNode } from '../model';
import type { FunctionDescriptor } from '@/services/api/functions';

beforeAll(() => registerBuiltinComponents());

const banFn: FunctionDescriptor = {
  id: 'player.ban',
  operation: 'update',
  resource: 'player',
  inputSchema: {
    type: 'object',
    properties: { playerId: { type: 'string' } },
    required: ['playerId'],
  },
};

const STATIC_SCHEMA = JSON.stringify({
  type: 'object',
  properties: { env: { type: 'string', title: '环境', enum: ['prod'] } },
});

interface PanelOpts {
  nodes?: PageNode[];
  fns?: FunctionDescriptor[];
  onPatch?: jest.Mock;
  onRenameVariable?: (name: string) => void;
}

function renderPanel(node: PageNode, opts: PanelOpts = {}) {
  const { nodes = [node], fns = [], onPatch = jest.fn(), onRenameVariable } = opts;
  const fnById = new Map(fns.map((f) => [f.id, f]));
  render(
    <App>
      <PropsPanel
        node={node}
        nodes={nodes}
        allFns={fns}
        fnById={fnById}
        onPatch={onPatch}
        onDelete={jest.fn()}
        onRenameVariable={onRenameVariable}
      />
    </App>,
  );
  return { onPatch };
}

describe('PropsPanel 空态', () => {
  it('无选中节点：空态提示（点击画布组件进行配置）', () => {
    render(
      <App>
        <PropsPanel
          node={undefined}
          nodes={[]}
          allFns={[]}
          fnById={new Map()}
          onPatch={jest.fn()}
          onDelete={jest.fn()}
        />
      </App>,
    );
    expect(screen.getByText('点击画布组件进行配置')).toBeInTheDocument();
  });
});

describe('PropsPanel staticSchema / condition 分桶', () => {
  const staticFormNode: PageNode = {
    id: 'sf-1',
    type: 'staticForm',
    props: { title: '筛选', span: 12, staticSchema: STATIC_SCHEMA },
  };

  it('staticSchema 分桶：ConstantFieldsEditor 渲染，添加常量落 patch', async () => {
    const { onPatch } = renderPanel(staticFormNode);
    // 分桶标题（区别于 plain 标题字段）
    expect(await screen.findByText('字段定义（JSON Schema）')).toBeInTheDocument();
    expect(await screen.findByDisplayValue('环境')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /添加常量/ }));
    await waitFor(() =>
      expect(
        onPatch.mock.calls.some(
          ([p]) => typeof (p as Record<string, unknown>).staticSchema === 'string',
        ),
      ).toBe(true),
    );
  });

  it('staticSchema prop 非字符串（遗留对象）：JSON 序列化兜底进编辑器', async () => {
    renderPanel({
      ...staticFormNode,
      props: {
        title: '筛选',
        span: 12,
        staticSchema: {
          type: 'object',
          properties: { env: { type: 'string', title: '环境', enum: ['prod'] } },
        } as unknown as string,
      },
    });
    expect(await screen.findByDisplayValue('环境')).toBeInTheDocument();
  });

  it('condition 分桶：ConditionEditor 渲染（显示条件标题）', async () => {
    renderPanel(staticFormNode);
    expect(await screen.findByText(/显示条件（按页面状态显隐本区块）/)).toBeInTheDocument();
  });

  it('condition 分桶：设置初始条件落 patch（visibleWhen）', async () => {
    const { onPatch } = renderPanel(staticFormNode);
    fireEvent.click(await screen.findByRole('button', { name: '+ 设置显示条件' }));
    await waitFor(() =>
      expect(
        onPatch.mock.calls.some(([p]) => 'visibleWhen' in (p as Record<string, unknown>)),
      ).toBe(true),
    );
  });

  it('staticForm 无 staticSchema prop：空对象序列化兜底进编辑器', async () => {
    renderPanel({
      id: 'sf-2',
      type: 'staticForm',
      props: { title: '筛选', span: 12 },
    });
    // 兜底 '{}' → 空常量列表 + 添加按钮仍可用
    expect(await screen.findByText(/暂无常量/)).toBeInTheDocument();
  });
});

describe('VarNameInput（变量名改名）', () => {
  const fnTableNode = (sectionKey?: string): PageNode => ({
    id: 'tb-1',
    type: 'fnTable',
    props: { functionId: 'player.list', ...(sectionKey ? { sectionKey } : {}) },
  });

  const fnOf = (id: string): FunctionDescriptor => ({ id, operation: 'list', resource: 'player' });
  const fns = [fnOf('player.list'), banFn];

  /** 变量名输入框（placeholder 定位，避免与其他输入撞名）。 */
  const varInput = () => screen.getByPlaceholderText('留空自动分配') as HTMLInputElement;

  it('遗留非 camelCase key：琥珀提示（未编辑态）', async () => {
    renderPanel(fnTableNode('player.list'), { fns });
    expect(await screen.findByText(/旧 key（非 camelCase）/)).toBeInTheDocument();
  });

  it('合法改名：hint 出现，回车提交走 onRenameVariable', async () => {
    const onRenameVariable = jest.fn();
    renderPanel(fnTableNode('playerList'), { fns, onRenameVariable });
    const input = await screen.findByPlaceholderText('留空自动分配');
    fireEvent.change(input, { target: { value: 'newName' } });
    expect(await screen.findByText(/回车确认改名（同步重写引用）/)).toBeInTheDocument();
    fireEvent.keyDown(input, { key: 'Enter' });
    fireEvent.blur(input); // jsdom 中原生 blur() 不派发 React 监听的 focusout，显式触发
    await waitFor(() => expect(onRenameVariable).toHaveBeenCalledWith('newName'));
  });

  it('无 onRenameVariable 回退：onPatch 直接改 sectionKey', async () => {
    const { onPatch } = renderPanel(fnTableNode('playerList'), { fns });
    const input = await screen.findByPlaceholderText('留空自动分配');
    fireEvent.change(input, { target: { value: 'newName' } });
    fireEvent.blur(input);
    await waitFor(() =>
      expect(onPatch).toHaveBeenCalledWith(expect.objectContaining({ sectionKey: 'newName' })),
    );
  });

  it('格式非法：错误提示 + 不提交', async () => {
    const onRenameVariable = jest.fn();
    const { onPatch } = renderPanel(fnTableNode('playerList'), { fns, onRenameVariable });
    const input = await screen.findByPlaceholderText('留空自动分配');
    fireEvent.change(input, { target: { value: '1abc' } });
    expect(await screen.findByText(/格式：小写字母开头的 camelCase/)).toBeInTheDocument();
    fireEvent.blur(input);
    expect(onRenameVariable).not.toHaveBeenCalled();
    expect(onPatch).not.toHaveBeenCalledWith(expect.objectContaining({ sectionKey: '1abc' }));
  });

  it('与他组件冲突：错误提示 + 不提交', async () => {
    const other: PageNode = {
      id: 'tb-2',
      type: 'fnTable',
      props: { functionId: 'player.list', sectionKey: 'takenName' },
    };
    const onRenameVariable = jest.fn();
    renderPanel(fnTableNode('playerList'), {
      nodes: [fnTableNode('playerList'), other],
      fns,
      onRenameVariable,
    });
    const input = await screen.findByPlaceholderText('留空自动分配');
    fireEvent.change(input, { target: { value: 'takenName' } });
    expect(await screen.findByText(/与其他组件的变量名冲突/)).toBeInTheDocument();
    fireEvent.blur(input);
    expect(onRenameVariable).not.toHaveBeenCalled();
  });

  it('清空/未修改：blur 不提交', async () => {
    const onRenameVariable = jest.fn();
    const { onPatch } = renderPanel(fnTableNode('playerList'), { fns, onRenameVariable });
    const input = await screen.findByPlaceholderText('留空自动分配');
    fireEvent.change(input, { target: { value: '  ' } });
    fireEvent.blur(input);
    fireEvent.change(varInput(), { target: { value: 'playerList' } }); // 改回原值 = 未变
    fireEvent.blur(varInput());
    expect(onRenameVariable).not.toHaveBeenCalled();
    expect(onPatch).not.toHaveBeenCalledWith(expect.objectContaining({ sectionKey: '  ' }));
  });
});

describe('动作 Tab 集成（按钮事件 + 行操作）', () => {
  it('选中按钮自动切动作 Tab；选择动作落 patch（onClick）', async () => {
    const node: PageNode = { id: 'btn-1', type: 'button', props: { title: '发邮件' } };
    const { onPatch } = renderPanel(node, { nodes: [node] });
    // 按钮自动激活动作 Tab：事件区直接可见
    expect(await screen.findByText(/onClick/)).toBeInTheDocument();
    fireEvent.mouseDown(screen.getByText('选择动作'));
    fireEvent.click(await screen.findByText('关闭弹窗'));
    await waitFor(() =>
      expect(onPatch.mock.calls.some(([p]) => 'onClick' in (p as Record<string, unknown>))).toBe(
        true,
      ),
    );
  });

  it('fnTable 行操作添加落 patch（rowActions）', async () => {
    const node: PageNode = {
      id: 'tb-9',
      type: 'fnTable',
      props: { functionId: 'player.list' },
    };
    // 添加行操作需要可用弹窗（含 fnForm 的 modal）
    const modal: PageNode = {
      id: 'm-1',
      type: 'modal',
      props: { title: '封禁弹窗' },
      children: [{ id: 'ff-in-modal', type: 'fnForm', props: { functionId: 'player.ban' } }],
    };
    const { onPatch } = renderPanel(node, { nodes: [node, modal], fns: [banFn] });
    // 按钮型组件才自动切动作 Tab；fnTable 默认配置 Tab → 手动切动作
    fireEvent.click(await screen.findByRole('tab', { name: '动作' }));
    fireEvent.click(await screen.findByRole('button', { name: /添加行操作/ }));
    await waitFor(() =>
      expect(onPatch.mock.calls.some(([p]) => 'rowActions' in (p as Record<string, unknown>))).toBe(
        true,
      ),
    );
  });
});

describe('ParamMappingEditor 集成（参数映射变更落 patch）', () => {
  it('切换参数来源 → onPatch({ inputAssignments })', async () => {
    const node: PageNode = {
      id: 'ff-1',
      type: 'fnForm',
      props: { functionId: 'player.ban' },
    };
    const { onPatch } = renderPanel(node, {
      nodes: [node],
      fns: [banFn],
    });
    // 参数映射区块渲染（PropsPanel 标题与编辑器 hint 双处）+ 参数名（required 星标）
    await waitFor(() => expect(screen.getAllByText(/参数映射/).length).toBeGreaterThan(0));
    expect(await screen.findByText('playerId *')).toBeInTheDocument();
    // 「自动」→ 切「固定值」触发 onChange
    fireEvent.mouseDown(screen.getAllByText('自动')[0]);
    const literal = await screen.findByText('固定值');
    fireEvent.click(literal);
    await waitFor(() =>
      expect(
        onPatch.mock.calls.some(([p]) =>
          Array.isArray((p as Record<string, unknown>).inputAssignments),
        ),
      ).toBe(true),
    );
  });
});
