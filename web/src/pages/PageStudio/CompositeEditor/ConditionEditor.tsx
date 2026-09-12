/** U10 区块级条件显示编辑器：表达式（页面变量路径，复用 ExpressionInput
 * 补全/校验）+ 运算符（等于/不等于/有值）+ 比较值。
 * 编辑态 prop visibleWhen: { expr, op, value }；编译期 parseExpression 拆
 * key/path 组装 wire 条件（compile.ts compileVisibleWhen）。 */
import React, { useCallback, useMemo } from 'react';
import { Button, Input, Select, Space, Typography } from 'antd';
import { DeleteOutlined } from '@ant-design/icons';
import { useIntl } from '@umijs/max';
import type { FunctionDescriptor } from '@/services/api/functions';
import type { PageNode } from './model';
import ExpressionInput from './ExpressionInput';
import { buildExprVariables, buildPathRoots, type ExprPathNode } from './exprVariables';

const { Text } = Typography;

export type VisibleWhenProp = {
  expr?: string;
  op?: 'equals' | 'notEquals' | 'exists';
  value?: string;
};

const ConditionEditor: React.FC<{
  value: VisibleWhenProp | undefined;
  onChange: (v: VisibleWhenProp | undefined) => void;
  nodes: PageNode[];
  /** 排除自身防自引用（显示条件依赖自身状态无意义）。 */
  selfId: string;
  fnById: Map<string, FunctionDescriptor>;
}> = ({ value, onChange, nodes, selfId, fnById }) => {
  const intl = useIntl();
  // 表达式补全上下文（页面变量 + 路径树；排除自身）
  const exprVariables = useMemo(
    () => buildExprVariables(nodes.filter((n) => n.id !== selfId)),
    [nodes, selfId],
  );
  const nodeByVar = useMemo(() => {
    const map = new Map<string, PageNode>();
    const walk = (list: PageNode[]) => {
      for (const n of list) {
        const name = typeof n.props.sectionKey === 'string' ? n.props.sectionKey.trim() : '';
        if (name && !map.has(name)) map.set(name, n);
        if (n.children) walk(n.children);
      }
    };
    walk(nodes);
    return map;
  }, [nodes]);
  const rootsOf = useCallback(
    (name: string): ExprPathNode[] => buildPathRoots(nodeByVar.get(name), fnById),
    [nodeByVar, fnById],
  );

  if (!value) {
    return (
      <Button
        size="small"
        type="dashed"
        block
        onClick={() => onChange({ expr: '', op: 'equals', value: '' })}
      >
        {intl.formatMessage({
          id: 'pages.pageStudio.editor.condition.add',
          defaultMessage: '+ 设置显示条件',
        })}
      </Button>
    );
  }

  const op = value.op ?? 'equals';
  const patch = (next: Partial<VisibleWhenProp>) => onChange({ ...value, ...next });

  return (
    <Space direction="vertical" size={6} style={{ width: '100%' }}>
      <Text type="secondary" style={{ fontSize: 11 }}>
        {intl.formatMessage({
          id: 'pages.pageStudio.editor.condition.hint',
          defaultMessage: '条件不满足时本区块不渲染（执行/联动不受影响）',
        })}
      </Text>
      <ExpressionInput
        size="small"
        value={value.expr ?? ''}
        onChange={(v) => patch({ expr: v })}
        variables={exprVariables}
        rootsOf={rootsOf}
        placeholder="{{var.values.字段}}"
      />
      <Space.Compact style={{ width: '100%' }}>
        <Select
          size="small"
          style={{ width: 110 }}
          value={op}
          onChange={(v) => patch({ op: v })}
          options={[
            {
              label: intl.formatMessage({
                id: 'pages.pageStudio.editor.condition.op.equals',
                defaultMessage: '等于',
              }),
              value: 'equals',
            },
            {
              label: intl.formatMessage({
                id: 'pages.pageStudio.editor.condition.op.notEquals',
                defaultMessage: '不等于',
              }),
              value: 'notEquals',
            },
            {
              label: intl.formatMessage({
                id: 'pages.pageStudio.editor.condition.op.exists',
                defaultMessage: '有值',
              }),
              value: 'exists',
            },
          ]}
        />
        {op !== 'exists' ? (
          <Input
            size="small"
            allowClear
            placeholder={intl.formatMessage({
              id: 'pages.pageStudio.editor.condition.valuePlaceholder',
              defaultMessage: '比较值',
            })}
            value={value.value ?? ''}
            onChange={(e) => patch({ value: e.target.value })}
          />
        ) : null}
      </Space.Compact>
      <Button
        size="small"
        type="link"
        danger
        icon={<DeleteOutlined />}
        style={{ alignSelf: 'flex-start', padding: 0 }}
        onClick={() => onChange(undefined)}
      >
        {intl.formatMessage({
          id: 'pages.pageStudio.editor.condition.remove',
          defaultMessage: '移除条件',
        })}
      </Button>
    </Space>
  );
};

export default ConditionEditor;
