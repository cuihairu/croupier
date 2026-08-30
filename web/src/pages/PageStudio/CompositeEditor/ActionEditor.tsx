import React from 'react';
import { Button, Select, Space, Typography } from 'antd';
import { CloseOutlined } from '@ant-design/icons';
import {
  ACTIONS,
  nodeSummary,
  parseAction,
  type ActionKind,
  type ActionSpec,
  type ActionStep,
} from './actions';
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
      {action && (
        <div style={{ borderTop: '1px dashed #e8e8e8', paddingTop: 6 }}>
          <Text type="secondary" style={{ fontSize: 11, display: 'block', marginBottom: 4 }}>
            后续动作（主动作完成后按序执行）
          </Text>
          {(action.chain ?? []).map((step: ActionStep, i: number) => {
            const stepTargets = ACTIONS[step.kind].targetFilter(nodes);
            return (
              <Space.Compact key={i} style={{ width: '100%', marginBottom: 4 }}>
                <Select
                  size="small"
                  style={{ width: 90 }}
                  value={step.kind}
                  onChange={(k) => {
                    const cand = ACTIONS[k].targetFilter(nodes);
                    onChange({
                      ...action,
                      chain: (action.chain ?? []).map((s2, j) =>
                        j === i ? { kind: k as ActionStep['kind'], target: cand[0]?.id ?? '' } : s2,
                      ),
                    });
                  }}
                  options={[
                    { value: 'runBinding', label: '执行' },
                    { value: 'refreshNode', label: '刷新' },
                  ]}
                />
                <Select
                  size="small"
                  style={{ width: 150 }}
                  value={stepTargets.some((t) => t.id === step.target) ? step.target : undefined}
                  placeholder="目标"
                  onChange={(t) =>
                    onChange({
                      ...action,
                      chain: (action.chain ?? []).map((s2, j) =>
                        j === i ? { ...s2, target: t } : s2,
                      ),
                    })
                  }
                  options={stepTargets.map((t) => ({ value: t.id, label: nodeSummary(t) }))}
                  notFoundContent={<Text type="secondary">无</Text>}
                />
                <Button
                  size="small"
                  icon={<CloseOutlined />}
                  onClick={() =>
                    onChange({ ...action, chain: (action.chain ?? []).filter((_, j) => j !== i) })
                  }
                />
              </Space.Compact>
            );
          })}
          <Button
            size="small"
            type="link"
            style={{ padding: 0, fontSize: 11 }}
            onClick={() => {
              const cand = ACTIONS.refreshNode.targetFilter(nodes);
              onChange({
                ...action,
                chain: [
                  ...(action.chain ?? []),
                  { kind: 'refreshNode', target: cand[0]?.id ?? '' },
                ],
              });
            }}
          >
            + 添加后续动作
          </Button>
        </div>
      )}
    </Space>
  );
}
