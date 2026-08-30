import React, { useState } from 'react';
import { Button, Input, Select, Space, Typography } from 'antd';
import { CloseOutlined, PlusOutlined } from '@ant-design/icons';
import {
  ACTIONS,
  nodeSummary,
  parseAction,
  type ActionKind,
  type ActionSpec,
  type ActionStep,
} from './actions';
import type { PageNode } from './model';
import type { FunctionDescriptor } from '@/services/api/functions';

const { Text } = Typography;

/** 动作编辑器（Appsmith onClick 式）：动作类型下拉 + 目标节点下拉。
 * allowedKinds 限定可选动作（如 onSuccess 只允许刷新）。 */
export default function ActionEditor({
  value,
  nodes,
  allowedKinds,
  allFns,
  onCreateModal,
  onChange,
}: {
  value: unknown;
  nodes: PageNode[];
  allowedKinds?: ActionKind[];
  allFns?: FunctionDescriptor[];
  /** 内联创建弹窗（无可用弹窗时一步完成：建弹窗+装表单+绑定本按钮）。 */
  onCreateModal?: (fn: FunctionDescriptor) => void;
  onChange: (v: ActionSpec | null) => void;
}) {
  const [newFnId, setNewFnId] = useState<string | undefined>();
  const action = parseAction(value);
  const kinds = allowedKinds ?? (Object.keys(ACTIONS) as ActionKind[]);

  const targets = action ? ACTIONS[action.kind].targetFilter(nodes) : [];
  const needModal = action?.kind === 'openModal' && targets.length === 0;

  return (
    <Space direction="vertical" size={6} style={{ width: '100%' }}>
      <Select
        size="small"
        style={{ width: '100%' }}
        placeholder="选择动作"
        value={action?.kind}
        onChange={(kind) => {
          const def = ACTIONS[kind];
          if (!def.needsTarget) {
            onChange({ kind, target: '', params: {} });
            return;
          }
          const candidates = def.targetFilter(nodes);
          onChange(candidates.length ? { kind, target: candidates[0].id } : null);
        }}
        options={kinds.map((k) => ({ value: k, label: ACTIONS[k].label }))}
        allowClear
        onClear={() => onChange(null)}
      />
      {action &&
        (ACTIONS[action.kind].paramFields ?? []).map((pf) => (
          <Input
            key={pf.key}
            size="small"
            addonBefore={<span style={{ fontSize: 11 }}>{pf.label}</span>}
            placeholder={pf.placeholder}
            value={String(action.params?.[pf.key] ?? '')}
            onChange={(e) =>
              onChange({
                ...action,
                params: { ...(action.params ?? {}), [pf.key]: e.target.value },
              })
            }
          />
        ))}
      {action && !needModal && ACTIONS[action.kind].needsTarget && (
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
      {needModal && (
        <div
          style={{
            border: '1px dashed #b37feb',
            borderRadius: 6,
            padding: 8,
            background: '#faf5ff',
          }}
        >
          <Text strong style={{ fontSize: 12, display: 'block', marginBottom: 6 }}>
            页面上还没有弹窗——选一个操作函数，一步创建并绑定：
          </Text>
          <Space.Compact style={{ width: '100%' }}>
            <Select
              size="small"
              style={{ width: '100%' }}
              showSearch
              optionFilterProp="label"
              placeholder="选操作函数（如 mail.send）"
              value={newFnId}
              onChange={setNewFnId}
              options={(allFns ?? []).map((f) => ({
                value: f.id,
                label: f.summary?.['zh-CN'] ? `${f.id}（${f.summary['zh-CN']}）` : f.id,
              }))}
            />
            <Button
              size="small"
              type="primary"
              icon={<PlusOutlined />}
              disabled={!newFnId || !allFns?.some((f) => f.id === newFnId)}
              onClick={() => {
                const fn = allFns?.find((f) => f.id === newFnId);
                if (fn) onCreateModal?.(fn);
              }}
            >
              创建弹窗并绑定
            </Button>
          </Space.Compact>
          <Text type="secondary" style={{ fontSize: 11 }}>
            自动完成：新建弹窗 → 装入该函数表单 → 绑定到本按钮
          </Text>
        </div>
      )}
      {action &&
        ACTIONS[action.kind].needsTarget &&
        !targets.some((t) => t.id === action.target) && (
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
            const stepTargets = ACTIONS[step.kind]?.targetFilter(nodes) ?? [];
            return (
              <div key={i} style={{ marginBottom: 4 }}>
                <Space.Compact style={{ width: '100%' }}>
                  <Select
                    size="small"
                    style={{ width: 90 }}
                    value={step.kind}
                    onChange={(k) => {
                      const cand = ACTIONS[k].targetFilter(nodes);
                      onChange({
                        ...action,
                        chain: (action.chain ?? []).map((s2, j) =>
                          j === i
                            ? { kind: k as ActionStep['kind'], target: cand[0]?.id ?? '' }
                            : s2,
                        ),
                      });
                    }}
                    options={[
                      { value: 'runBinding', label: '执行' },
                      { value: 'refreshNode', label: '刷新' },
                      { value: 'closeModal', label: '关弹窗' },
                      { value: 'navigate', label: '跳转' },
                      { value: 'showMessage', label: '提示' },
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
                {(step.kind === 'runBinding' || step.kind === 'refreshNode') && (
                  <Input
                    size="small"
                    style={{ fontSize: 11 }}
                    placeholder="参数来源（如 playerId=行.uid，多个逗号分隔；节点.字段/row.字段/字面量）"
                    value={Object.entries(step.params ?? {})
                      .map(([k, v]) => `${k}=${v}`)
                      .join(',')}
                    onChange={(e) => {
                      const params: Record<string, string> = {};
                      for (const pair of e.target.value.split(',')) {
                        const eq = pair.indexOf('=');
                        if (eq > 0) {
                          params[pair.slice(0, eq).trim()] = pair.slice(eq + 1).trim();
                        }
                      }
                      onChange({
                        ...action,
                        chain: (action.chain ?? []).map((s2, j) =>
                          j === i ? { ...s2, params } : s2,
                        ),
                      });
                    }}
                  />
                )}
              </div>
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
