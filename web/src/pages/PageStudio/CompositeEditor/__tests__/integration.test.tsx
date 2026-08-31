import { render, screen, fireEvent } from '@testing-library/react';
import { App } from 'antd';
import PropsPanel from '../PropsPanel';
import { registerBuiltinComponents } from '../components/builtin';
import { getComponent } from '../registry';
import type { PageNode } from '../model';

beforeAll(() => registerBuiltinComponents());

const nodes: PageNode[] = [
  { id: 'btn-1', type: 'button', props: { title: '测试按钮', btnStyle: 'default', span: 6 } },
  {
    id: 'fn-1',
    type: 'fnTable',
    props: { functionId: 'player.list', title: '玩家列表', span: 24 },
  },
];

describe('完整用户流程集成测试：选中按钮→属性面板→动作Tab', () => {
  it('选中按钮后属性面板显示「配置」和「动作」两个 Tab', () => {
    const btn = nodes[0];
    render(
      <App>
        <PropsPanel
          node={btn}
          nodes={nodes}
          allFns={[]}
          fnById={new Map()}
          onPatch={() => undefined}
          onDelete={() => undefined}
        />
      </App>,
    );
    // 应该有两个 Tab
    expect(screen.getByText('配置')).toBeInTheDocument();
    expect(screen.getByText('动作')).toBeInTheDocument();
  });

  it('选中按钮后自动切到「动作」Tab，显示「点击」事件', () => {
    const btn = nodes[0];
    render(
      <App>
        <PropsPanel
          node={btn}
          nodes={nodes}
          allFns={[]}
          fnById={new Map()}
          onPatch={() => undefined}
          onDelete={() => undefined}
        />
      </App>,
    );
    // useEffect 自动切到 actions tab 后，应显示事件的标签
    expect(screen.getByText(/点击/)).toBeInTheDocument();
  });

  it('选中表格后也有「动作」Tab（行点击/行选中）', () => {
    const table = nodes[1];
    render(
      <App>
        <PropsPanel
          node={table}
          nodes={nodes}
          allFns={[]}
          fnById={new Map()}
          onPatch={() => undefined}
          onDelete={() => undefined}
        />
      </App>,
    );
    expect(screen.getByText('动作')).toBeInTheDocument();
  });

  it('fnFields 组件也有动作 Tab（点击事件）', () => {
    const fields: PageNode = {
      id: 'f-1',
      type: 'fnFields',
      props: { functionId: 'x', title: '详情' },
    };
    render(
      <App>
        <PropsPanel
          node={fields}
          nodes={nodes}
          allFns={[]}
          fnById={new Map()}
          onPatch={() => undefined}
          onDelete={() => undefined}
        />
      </App>,
    );
    expect(screen.getByText('动作')).toBeInTheDocument();
  });
});
