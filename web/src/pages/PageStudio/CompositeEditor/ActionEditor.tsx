import React, { useCallback, useMemo, useState } from 'react';
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
import ExpressionInput from './ExpressionInput';
import { buildExprVariables, buildPathRoots, type ExprPathNode } from './exprVariables';

const { Text } = Typography;

/** 动作编辑器（Appsmith onClick 式）：动作类型下拉 + 目标节点下拉。
 * allowedKinds 限定可选动作（如 onSuccess 只允许刷新）。 */
export default function ActionEditor({
  value,
  nodes,
  allowedKinds,
  allFns,
  fnById,
  onCreateModal,
  onChange,
}: {
  value: unknown;
  nodes: PageNode[];
  allowedKinds?: ActionKind[];
  allFns?: FunctionDescriptor[];
  /** V5：表达式补全上下文所需的函数契约映射。 */
  fnById?: Map<string, FunctionDescriptor>;
  /** 内联创建弹窗（无可用弹窗时一步完成：建弹窗+装表单+绑定本按钮）。 */
  onCreateModal?: (fn: FunctionDescriptor) => void;
  onChange: (v: ActionSpec | null) => void;
}) {
  const [newFnId, setNewFnId] = useState<string | undefined>();
  const action = parseAction(value);
  const kinds = allowedKinds ?? (Object.keys(ACTIONS) as ActionKind[]);

  // 原始 kind（parseAction 会把「需目标但目标为空」判 null——那是运行期保护；
  // 编辑期刚选 openModal 还没建弹窗时 target 为空，必须保留 kind 才能渲染引导框）
  const rawValue = value as { kind?: unknown } | null | undefined;
  const rawKind =
    rawValue &&
    typeof rawValue === 'object' &&
    typeof rawValue.kind === 'string' &&
    rawValue.kind in ACTIONS
      ? (rawValue.kind as ActionKind)
      : undefined;

  const effKind = action?.kind ?? rawKind;
  const targets = effKind ? ACTIONS[effKind].targetFilter(nodes) : [];
  const needModal = effKind === 'openModal' && targets.length === 0;

  // V5：表达式补全上下文（页面变量 + 各变量路径树）
  const exprVariables = useMemo(() => buildExprVariables(nodes), [nodes]);
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
    (name: string): ExprPathNode[] => buildPathRoots(nodeByVar.get(name), fnById ?? new Map()),
    [nodeByVar, fnById],
  );

  return (
    <Space orientation="vertical" size={6} style={{ width: '100%' }}>
      <Select
        size="small"
        style={{ width: '100%' }}
        placeholder="选择动作"
        value={effKind}
        onChange={(kind) => {
          const def = ACTIONS[kind];
          if (!def.needsTarget) {
            onChange({ kind, target: '', params: {} });
            return;
          }
          const candidates = def.targetFilter(nodes);
          // 无候选也保留 kind（target 留空）——openModal 时由引导框接手创建弹窗，
          // 而非把动作清成 null 导致引导框永不出现
          onChange({ kind, target: candidates.length ? candidates[0].id : '' });
        }}
        options={kinds.map((k) => ({ value: k, label: ACTIONS[k].label }))}
        allowClear
        onClear={() => onChange(null)}
      />
      {effKind &&
        (ACTIONS[effKind].paramFields ?? []).map((pf) => (
          <Input
            key={pf.key}
            size="small"
            addonBefore={<span style={{ fontSize: 11 }}>{pf.label}</span>}
            placeholder={pf.placeholder}
            value={String(action?.params?.[pf.key] ?? '')}
            onChange={(e) =>
              onChange({
                kind: effKind,
                target: action?.target ?? '',
                params: { ...(action?.params ?? {}), [pf.key]: e.target.value },
              })
            }
          />
        ))}
      {effKind && !needModal && ACTIONS[effKind].needsTarget && (
        <Space.Compact style={{ width: '100%' }}>
          <Select
            size="small"
            style={{ width: '100%' }}
            value={targets.some((t) => t.id === action?.target) ? action?.target : undefined}
            placeholder="选择目标"
            onChange={(target) => onChange({ kind: effKind, target })}
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
                  <div style={{ marginTop: 4 }}>
                    {Object.entries(step.params ?? {}).map(([pk, pv]) => (
                      <Space key={pk} size={4} style={{ display: 'flex', marginBottom: 4 }}>
                        <Input
                          size="small"
                          style={{ width: 90 }}
                          value={pk}
                          placeholder="参数名"
                          onChange={(e) => {
                            const nextName = e.target.value;
                            const params: Record<string, string> = {};
                            for (const [k, v] of Object.entries(step.params ?? {})) {
                              params[k === pk ? nextName || k : k] = v;
                            }
                            onChange({
                              ...action,
                              chain: (action.chain ?? []).map((s2, j) =>
                                j === i ? { ...s2, params } : s2,
                              ),
                            });
                          }}
                        />
                        <Text type="secondary" style={{ fontSize: 11 }}>
                          =
                        </Text>
                        <ExpressionInput
                          size="small"
                          style={{ flex: 1, minWidth: 140 }}
                          value={String(pv)}
                          onChange={(v) =>
                            onChange({
                              ...action,
                              chain: (action.chain ?? []).map((s2, j) =>
                                j === i
                                  ? { ...s2, params: { ...(step.params ?? {}), [pk]: v } }
                                  : s2,
                              ),
                            })
                          }
                          variables={exprVariables}
                          rootsOf={rootsOf}
                        />
                        <Button
                          size="small"
                          type="text"
                          danger
                          icon={<CloseOutlined />}
                          onClick={() => {
                            const params = { ...(step.params ?? {}) };
                            delete params[pk];
                            onChange({
                              ...action,
                              chain: (action.chain ?? []).map((s2, j) =>
                                j === i ? { ...s2, params } : s2,
                              ),
                            });
                          }}
                        />
                      </Space>
                    ))}
                    <Button
                      size="small"
                      type="link"
                      style={{ padding: 0, fontSize: 11 }}
                      onClick={() => {
                        const used = new Set(Object.keys(step.params ?? {}));
                        let name = 'param';
                        for (let n = 1; used.has(name); n += 1) name = `param${n}`;
                        onChange({
                          ...action,
                          chain: (action.chain ?? []).map((s2, j) =>
                            j === i
                              ? { ...s2, params: { ...(step.params ?? {}), [name]: '' } }
                              : s2,
                          ),
                        });
                      }}
                    >
                      + 添加参数
                    </Button>
                  </div>
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
