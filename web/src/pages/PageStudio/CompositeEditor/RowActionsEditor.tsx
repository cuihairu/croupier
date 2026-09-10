import React, { useState } from 'react';
import { Button, Input, Select, Space, Switch, Typography } from 'antd';
import { DeleteOutlined, PlusOutlined } from '@ant-design/icons';
import type { PageNode } from './model';
import type { FunctionDescriptor } from '@/services/api/functions';
import { schemaProperties } from './types';
import ExpressionInput from './ExpressionInput';

const { Text } = Typography;

type RowActionDraft = {
  label: string;
  targetSection: string;
  params: Record<string, string>;
  danger: boolean;
};

/** 行操作编辑器：行尾按钮 → 打开弹窗表单，行字段映射进表单参数。 */
export default function RowActionsEditor({
  value,
  nodes,
  fnById,
  rowFields,
  onChange,
}: {
  value: unknown;
  nodes: PageNode[];
  fnById: Map<string, FunctionDescriptor>;
  rowFields: string[];
  onChange: (v: RowActionDraft[] | null) => void;
}) {
  const actions = (Array.isArray(value) ? value : []) as unknown as RowActionDraft[];
  const modals = nodes.filter(
    (n) => n.type === 'modal' && n.children?.some((c) => c.type === 'fnForm'),
  );
  const modalOptions = modals.map((m) => {
    const form = m.children!.find((c) => c.type === 'fnForm')!;
    return {
      value: m.id,
      label: `${String(m.props.title ?? '弹窗')}（${String(form.props.functionId ?? '')}）`,
    };
  });

  const patch = (i: number, p: Partial<RowActionDraft>) =>
    onChange(actions.map((a, idx) => (idx === i ? { ...a, ...p } : a)));

  return (
    <Space orientation="vertical" size={8} style={{ width: '100%' }}>
      {actions.map((a, i) => {
        const targetModal = modals.find((m) => m.id === a.targetSection);
        const formFnId = String(
          targetModal?.children?.find((c) => c.type === 'fnForm')?.props.functionId ?? '',
        );
        const targetParamFields = getParamFields(targetModal, fnById);
        return (
          <div key={i} style={{ border: '1px solid #f0f0f0', borderRadius: 6, padding: 8 }}>
            <Space orientation="vertical" size={6} style={{ width: '100%' }}>
              <Input
                size="small"
                placeholder="按钮文案（如：发邮件）"
                value={a.label}
                onChange={(e) => patch(i, { label: e.target.value })}
              />
              <Select
                size="small"
                style={{ width: '100%' }}
                placeholder="打开弹窗"
                value={a.targetSection || undefined}
                onChange={(v) => patch(i, { targetSection: v, params: {} })}
                options={modalOptions}
              />
              {targetModal && (
                <ParamMapping
                  mapping={a.params}
                  rowFields={rowFields}
                  paramFields={targetParamFields}
                  onChange={(params) => patch(i, { params })}
                />
              )}
              <Space size={8}>
                <Switch
                  size="small"
                  checked={a.danger}
                  onChange={(v) => patch(i, { danger: v })}
                  checkedChildren="危险"
                  unCheckedChildren="普通"
                />
                <Text type="secondary" style={{ fontSize: 11 }}>
                  危险=红字+二次确认
                </Text>
                <Button
                  size="small"
                  type="link"
                  danger
                  icon={<DeleteOutlined />}
                  onClick={() => onChange(actions.filter((_, idx) => idx !== i))}
                >
                  删除
                </Button>
              </Space>
            </Space>
          </div>
        );
      })}
      <Button
        size="small"
        type="dashed"
        block
        icon={<PlusOutlined />}
        disabled={modalOptions.length === 0}
        onClick={() =>
          onChange([...actions, { label: '', targetSection: '', params: {}, danger: false }])
        }
      >
        添加行操作{modalOptions.length === 0 ? '（先创建含表单的弹窗）' : ''}
      </Button>
    </Space>
  );
}

/** 从 modal 的 fnForm 契约提取表单参数字段名。 */
function getParamFields(
  modal: PageNode | undefined,
  fnById: Map<string, FunctionDescriptor>,
): string[] {
  const fid = modal?.children?.find((c) => c.type === 'fnForm')?.props.functionId;
  if (typeof fid !== 'string' || !fid) return [];
  return schemaProperties(fnById.get(fid)?.inputSchema);
}

/** 参数映射编辑：行字段 → 弹窗表单参数。paramFields 为空时参数名手输。 */
function ParamMapping({
  mapping,
  rowFields,
  paramFields,
  onChange,
}: {
  mapping: Record<string, string>;
  rowFields: string[];
  paramFields: string[];
  onChange: (m: Record<string, string>) => void;
}) {
  // 参数名手输草稿（按已提交参数名键控）：失焦校验（非空/防撞）后保位改名
  const [nameDrafts, setNameDrafts] = useState<Record<string, string>>({});
  const entries = Object.entries(mapping);

  /** 保位改名：trim 后非空、不与其他参数撞名才生效（条目顺序不变）。 */
  const renameParam = (from: string, raw: string) => {
    const to = raw.trim();
    if (!to || to === from || mapping[to] !== undefined) return;
    onChange(Object.fromEntries(entries.map(([k, v]) => [k === from ? to : k, v])));
  };

  const commitRename = (param: string) => {
    const draft = nameDrafts[param];
    if (draft === undefined) return;
    renameParam(param, draft);
    setNameDrafts((prev) => {
      const next = { ...prev };
      delete next[param];
      return next;
    });
  };

  return (
    <div style={{ fontSize: 11 }}>
      <Text type="secondary">参数带入（表单参数 ← 行字段）</Text>
      {entries.map(([param, source], idx) => {
        const draft = nameDrafts[param];
        const draftInvalid =
          draft !== undefined &&
          (draft.trim() === '' || (draft.trim() !== param && mapping[draft.trim()] !== undefined));
        return (
          <Space key={idx} size={4} style={{ display: 'flex', marginBottom: 4 }}>
            {paramFields.length ? (
              <Select
                size="small"
                style={{ width: 110 }}
                value={param}
                onChange={(np) => renameParam(param, np)}
                options={paramFields.map((f) => ({ value: f, label: f }))}
              />
            ) : (
              <Input
                size="small"
                style={{ width: 110 }}
                status={draftInvalid ? 'error' : undefined}
                value={draft ?? param}
                placeholder="参数名"
                onChange={(e) => setNameDrafts((prev) => ({ ...prev, [param]: e.target.value }))}
                onBlur={() => commitRename(param)}
                onPressEnter={() => commitRename(param)}
              />
            )}
            <span>←</span>
            <ExpressionInput
              size="small"
              style={{ width: 150 }}
              value={source}
              onChange={(v) => onChange({ ...mapping, [param]: v })}
              variables={[]}
              rootsOf={() => []}
              rowFields={rowFields}
              placeholder="行字段，或 {{ row. }}"
            />
            <Button
              size="small"
              type="text"
              danger
              style={{ padding: 0 }}
              onClick={() => {
                const next = { ...mapping };
                delete next[param];
                onChange(next);
              }}
            >
              ×
            </Button>
          </Space>
        );
      })}
      <Button
        size="small"
        type="link"
        style={{ padding: 0, fontSize: 11 }}
        disabled={rowFields.length === 0}
        onClick={() => {
          const used = new Set(Object.keys(mapping));
          const free = rowFields.find((f) => !used.has(f)) ?? '';
          if (free) onChange({ ...mapping, [free]: free });
        }}
      >
        + 添加映射
      </Button>
    </div>
  );
}
