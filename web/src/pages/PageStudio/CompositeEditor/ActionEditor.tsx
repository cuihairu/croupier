import React from 'react';
import { Button, Select, Space, Typography } from 'antd';
import { CloseOutlined } from '@ant-design/icons';
import { ACTIONS, nodeSummary, parseAction, type ActionKind, type ActionSpec } from './actions';
import type { PageNode } from './model';

const { Text } = Typography;

/** 动作编辑器（Appsmith onClick 式）：动作类型下拉 + 目标节点下拉。
 * allowedKinds 限定可选动作（如 onSuccess 只允许刷新）。 */
export default function ActionEditor({
  value,
  nodes,
  allowedKinds,
  onChange,
}: {
  value: unknown;
  nodes: PageNode[];
  allowedKinds?: ActionKind[];
  onChange: (v: ActionSpec | null) => void;
}) {
  const action = parseAction(value);
  const kinds = allowedKinds ?? (Object.keys(ACTIONS) as ActionKind[]);

  const targets = action ? ACTIONS[action.kind].targetFilter(nodes) : [];

  return (
    <Space direction="vertical" size={6} style={{ width: '100%' }}>
      <Select
        size="small"
        style={{ width: '100%' }}
        placeholder="选择动作"
        value={action?.kind}
        onChange={(kind) => {
          const candidates = ACTIONS[kind].targetFilter(nodes);
          onChange(candidates.length ? { kind, target: candidates[0].id } : null);
        }}
        options={kinds.map((k) => ({ value: k, label: ACTIONS[k].label }))}
        allowClear
        onClear={() => onChange(null)}
      />
      {action && (
        <Space.Compact style={{ width: '100%' }}>
          <Select
            size="small"
            style={{ width: '100%' }}
            value={targets.some((t) => t.id === action.target) ? action.target : undefined}
            placeholder="选择目标"
            onChange={(target) => onChange({ kind: action.kind, target })}
            options={targets.map((t) => ({ value: t.id, label: nodeSummary(t) }))}
            notFoundContent={<Text type="secondary">无可用目标</Text>}
          />
          <Button size="small" icon={<CloseOutlined />} onClick={() => onChange(null)} />
        </Space.Compact>
      )}
      {action && !targets.some((t) => t.id === action.target) && (
        <Text type="danger" style={{ fontSize: 11 }}>
          目标节点已被删除——请重新选择
        </Text>
      )}
    </Space>
  );
}
