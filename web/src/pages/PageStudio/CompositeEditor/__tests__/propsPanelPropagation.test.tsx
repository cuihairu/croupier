/** P0 回归锁定：配置表单（SchemaFormRenderer）的编辑必须真实传播到
 * onPatch——SchemaFormRenderer 的 onValuesChange 契约第一参恒为 {}，
 * 消费端取第二参。若取错参数，编辑视觉生效但从不落树（保存全丢）。 */
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import PropsPanel from '../PropsPanel';
import { registerBuiltinComponents } from '../components/builtin';
import type { PageNode } from '../model';

beforeAll(() => registerBuiltinComponents());

const textNode: PageNode = {
  id: 'text-1',
  type: 'text',
  props: { content: '说明文本', level: 'p', span: 24 },
};

function renderPanel(node: PageNode, onPatch: jest.Mock) {
  return render(
    <App>
      <PropsPanel
        node={node}
        nodes={[node]}
        allFns={[]}
        fnById={new Map()}
        onPatch={onPatch}
        onDelete={() => undefined}
      />
    </App>,
  );
}

describe('PropsPanel 配置表单编辑传播（P0 回归）', () => {
  it('修改「内容」后 onPatch 收到真实变更（非空对象、含新值）', async () => {
    const onPatch = jest.fn();
    renderPanel(textNode, onPatch);

    const input = await screen.findByLabelText(/内容/);
    expect(input).toBeTruthy();

    fireEvent.change(input, { target: { value: '新文案' } });

    await waitFor(() => expect(onPatch).toHaveBeenCalled());
    const last = onPatch.mock.calls[onPatch.mock.calls.length - 1][0] as Record<string, unknown>;
    // 关键断言：不是空对象，且含修改后的值
    expect(Object.keys(last).length).toBeGreaterThan(0);
    expect(last.content).toBe('新文案');
  });

  it('onPatch 只发发生变化的键（未编辑的 span/level 不在 patch 中）', async () => {
    const onPatch = jest.fn();
    renderPanel(textNode, onPatch);

    const input = await screen.findByLabelText(/内容/);
    fireEvent.change(input, { target: { value: 'x' } });

    await waitFor(() => expect(onPatch).toHaveBeenCalled());
    const last = onPatch.mock.calls[onPatch.mock.calls.length - 1][0] as Record<string, unknown>;
    expect(last.span).toBeUndefined();
    expect(last.level).toBeUndefined();
  });
});
