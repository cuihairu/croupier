/** PropsPanel 自定义 propSchema 路径（registry 隔离直渲）：
 * 内置组件的 propSchema 恒产 properties，无法覆盖「无 properties 的分桶空侧」
 * 与「分桶标题 ?? key 兜底」——本文件 reset 后重注册同名类型（'text'/'button'）
 * 用自定义 schema 精确触发这两条兜底链。 */
import React from 'react';
import { render, screen } from '@testing-library/react';
import { App } from 'antd';
import PropsPanel from '../PropsPanel';
import { registerComponent, resetRegistryForTest, type PreviewProps } from '../registry';
import type { PageNode } from '../model';
import type { JSONSchema } from '@/types/dashboard';

const stubPreview = ({ node }: PreviewProps) => <div data-testid={`preview-${node.id}`} />;

beforeAll(() => {
  resetRegistryForTest();
  // 裸 text：propSchema 无 properties → 分桶全空（Empty 兜底路径）
  registerComponent({
    type: 'text',
    name: '裸文本',
    icon: null,
    category: 'basic',
    propSchema: () => ({ type: 'object' }) as JSONSchema,
    scaffold: () => ({}),
    Preview: stubPreview,
  });
  // 裸 button：staticSchema/condition 字段均无 title → 分桶标题回退 key
  registerComponent({
    type: 'button',
    name: '裸按钮',
    icon: null,
    category: 'basic',
    propSchema: () =>
      ({
        type: 'object',
        properties: {
          rawStatic: { type: 'string', format: 'staticSchema' },
          rawCond: { type: 'object', format: 'condition' },
        },
      }) as JSONSchema,
    scaffold: () => ({}),
    Preview: stubPreview,
  });
});

function renderPanel(node: PageNode) {
  render(
    <App>
      <PropsPanel
        node={node}
        nodes={[node]}
        allFns={[]}
        fnById={new Map()}
        onPatch={jest.fn()}
        onDelete={jest.fn()}
      />
    </App>,
  );
}

describe('schema 无 properties：分桶全空 → Empty 兜底', () => {
  it('properties 缺省：渲染「无配置字段」提示', () => {
    renderPanel({ id: 't-empty', type: 'text', props: {} });
    expect(screen.getByText('无配置字段')).toBeInTheDocument();
  });
});

describe('分桶标题兜底（字段无 title → 显示 key）', () => {
  it('staticSchema / condition 字段标题回退 key；非全空不出 Empty', async () => {
    renderPanel({ id: 'b-fallback', type: 'button', props: {} });
    expect(await screen.findByText('rawStatic')).toBeInTheDocument();
    expect(screen.getByText('rawCond')).toBeInTheDocument();
    // plainKeys 空 + staticSchemaKeys 非空 → Empty 条件不成立
    expect(screen.queryByText('无配置字段')).not.toBeInTheDocument();
  });
});
