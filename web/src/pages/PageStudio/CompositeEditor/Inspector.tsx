import React from 'react';
import {
  Alert,
  Button,
  Card,
  Empty,
  Input,
  Select,
  Slider,
  Space,
  Switch,
  Tag,
  Typography,
} from 'antd';
import { LinkOutlined, PlusOutlined, WarningOutlined } from '@ant-design/icons';
import type { FunctionDescriptor } from '@/services/api/functions';
import {
  VIEW_META,
  linkageCheck,
  sectionParams,
  sectionOutputFields,
  type ActionDraft,
  type CompositeView,
  type SectionDraft,
} from './types';

const { Text } = Typography;

export default function Inspector({
  section,
  fn,
  sections,
  fnById,
  onChange,
  onDelete,
}: {
  section: SectionDraft | undefined;
  fn: FunctionDescriptor | undefined;
  sections: SectionDraft[];
  fnById: Map<string, FunctionDescriptor>;
  onChange: (patch: Partial<SectionDraft>) => void;
  onDelete: () => void;
}) {
  if (!section) {
    return (
      <Card size="small" title="属性" styles={{ body: { padding: 16 } }}>
        <Empty description="点击画布中的区块进行配置" image={Empty.PRESENTED_IMAGE_SIMPLE} />
      </Card>
    );
  }

  const others = sections.filter((s) => s.key !== section.key);
  const params = sectionParams(fn);
  const check = linkageCheck(
    others
      .filter((s) => section.dependsOn.includes(s.key))
      .map((s) => ({
        key: s.key,
        title: s.title || s.functionId,
        outputs: sectionOutputFields(fnById.get(s.functionId)),
      })),
    params,
  );
  const downstream = sections.filter((s) => s.dependsOn.includes(section.key));
  const myOutputs = sectionOutputFields(fn);

  return (
    <Card
      size="small"
      title={<Text strong>区块属性</Text>}
      styles={{ body: { maxHeight: 'calc(100vh - 200px)', overflow: 'auto' } }}
    >
      <Space direction="vertical" size={14} style={{ width: '100%' }}>
        <div>
          <Text type="secondary" style={{ fontSize: 11 }}>
            函数
          </Text>
          <div>
            <Text code style={{ fontSize: 12 }}>
              {section.functionId}
            </Text>
          </div>
        </div>

        <div>
          <Text type="secondary" style={{ fontSize: 11, display: 'block', marginBottom: 4 }}>
            标题
          </Text>
          <Input
            size="small"
            value={section.title}
            onChange={(e) => onChange({ title: e.target.value })}
            placeholder={section.functionId}
          />
        </div>

        <div>
          <Text type="secondary" style={{ fontSize: 11, display: 'block', marginBottom: 4 }}>
            展示形态
          </Text>
          <Select<CompositeView>
            size="small"
            style={{ width: '100%' }}
            value={section.view}
            onChange={(v) => onChange({ view: v })}
            options={Object.entries(VIEW_META).map(([value, meta]) => ({
              value: value as CompositeView,
              label: `${meta.label} — ${meta.hint}`,
            }))}
          />
        </div>

        <div>
          <Text type="secondary" style={{ fontSize: 11, display: 'block', marginBottom: 4 }}>
            宽度（{section.span || 24}/24 栅格）
          </Text>
          <Slider
            min={4}
            max={24}
            step={4}
            value={section.span || 24}
            onChange={(v) => onChange({ span: v })}
            marks={{ 8: '1/3', 12: '半宽', 24: '整行' }}
          />
        </div>

        <div>
          <Space size={8}>
            <Switch
              size="small"
              checked={section.autoRun}
              onChange={(v) => onChange({ autoRun: v })}
            />
            <Text style={{ fontSize: 12 }}>进入页面自动执行</Text>
          </Space>
          <div>
            <Text type="secondary" style={{ fontSize: 11 }}>
              查询类区块建议开启（表格/字段卡）
            </Text>
          </div>
        </div>

        <div>
          <Text type="secondary" style={{ fontSize: 11, display: 'block', marginBottom: 4 }}>
            <LinkOutlined /> 联动依赖（上游产出变化时自动重跑本区块）
          </Text>
          {others.length === 0 ? (
            <Text type="secondary" style={{ fontSize: 12 }}>
              暂无其他区块可依赖
            </Text>
          ) : (
            <Select
              mode="multiple"
              size="small"
              style={{ width: '100%' }}
              placeholder="选择上游区块"
              value={section.dependsOn}
              onChange={(v) => onChange({ dependsOn: v })}
              options={others.map((s) => ({
                value: s.key,
                label: s.title || s.functionId,
              }))}
            />
          )}

          {check.map((c) => (
            <div key={c.depKey} style={{ marginTop: 6 }}>
              <Text style={{ fontSize: 11 }}>← {c.depTitle}</Text>
              <div>
                {c.matched.length > 0 && (
                  <Text type="success" style={{ fontSize: 11 }}>
                    同名字段自动传入：{c.matched.join(', ')}
                  </Text>
                )}
                {c.unmatchedParams.length > 0 && (
                  <div style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
                    <WarningOutlined style={{ color: '#faad14', fontSize: 11 }} />
                    <Text type="warning" style={{ fontSize: 11 }}>
                      必填参数 {c.unmatchedParams.join(', ')} 在上游输出中无同名字段，运行时需手填
                    </Text>
                  </div>
                )}
              </div>
            </div>
          ))}
        </div>

        {/* ---- 组合能力：弹窗 / 行操作 / 顶部按钮 / 成功刷新 ---- */}

        {section.view === 'form' && (
          <div>
            <Text type="secondary" style={{ fontSize: 11, display: 'block', marginBottom: 4 }}>
              展示方式
            </Text>
            <Select
              size="small"
              style={{ width: '100%' }}
              value={section.display}
              onChange={(v) => onChange({ display: v })}
              options={[
                { value: 'inline', label: '行内 — 嵌在页面栅格中' },
                { value: 'dialog', label: '弹窗 — 由按钮/行操作触发打开' },
              ]}
            />
            {section.display === 'dialog' && (
              <Text type="secondary" style={{ fontSize: 11 }}>
                配置表格区块的行操作或顶部按钮来触发此弹窗
              </Text>
            )}
          </div>
        )}

        {(section.view === 'form' || section.view === 'actions') && others.length > 0 && (
          <div>
            <Text type="secondary" style={{ fontSize: 11, display: 'block', marginBottom: 4 }}>
              成功后自动刷新
            </Text>
            <Select
              mode="multiple"
              size="small"
              style={{ width: '100%' }}
              placeholder="选择操作成功后重跑的区块"
              value={section.onSuccessRefresh}
              onChange={(v) => onChange({ onSuccessRefresh: v })}
              options={others.map((s) => ({ value: s.key, label: s.title || s.functionId }))}
            />
          </div>
        )}

        {section.view === 'table' && (
          <ActionsEditor
            title="顶部按钮（打开弹窗操作）"
            actions={section.toolbarActions}
            dialogOptions={others
              .filter((s) => s.display === 'dialog')
              .map((s) => ({ value: s.key, label: s.title || s.functionId }))}
            rowFields={sectionOutputFields(fn)}
            paramOptions={sectionParams(fnById.get(section.functionId)).map((p) => p.name)}
            onChange={(actions) => onChange({ toolbarActions: actions })}
            mode="toolbar"
          />
        )}

        {section.view === 'table' && (
          <ActionsEditor
            title="行操作（行尾按钮，行字段带入弹窗参数）"
            actions={section.rowActions}
            dialogOptions={others
              .filter((s) => s.display === 'dialog')
              .map((s) => ({ value: s.key, label: s.title || s.functionId }))}
            rowFields={sectionOutputFields(fn)}
            paramOptions={[]}
            onChange={(actions) => onChange({ rowActions: actions })}
            mode="row"
          />
        )}

        {downstream.length > 0 && (
          <div>
            <Text type="secondary" style={{ fontSize: 11, display: 'block' }}>
              被下游引用
            </Text>
            <Space wrap size={4} style={{ marginTop: 4 }}>
              {downstream.map((s) => (
                <Tag key={s.key} color="blue" style={{ fontSize: 11 }}>
                  {s.title || s.functionId}
                </Tag>
              ))}
            </Space>
          </div>
        )}

        {myOutputs.length > 0 && (
          <div>
            <Text type="secondary" style={{ fontSize: 11, display: 'block', marginBottom: 4 }}>
              输出字段（供下游同名匹配）
            </Text>
            <Space wrap size={4}>
              {myOutputs.map((f) => (
                <Tag key={f} style={{ fontSize: 11 }}>
                  {f}
                </Tag>
              ))}
            </Space>
          </div>
        )}

        {params.length > 0 && (
          <Alert
            type="info"
            showIcon={false}
            style={{ fontSize: 11, padding: '6px 10px' }}
            message={
              <>
                <Text strong style={{ fontSize: 11 }}>
                  输入参数
                </Text>
                <div>
                  {params.map((p) => (
                    <Tag key={p.name} style={{ fontSize: 11 }}>
                      {p.name}
                      {p.required ? ' *' : ''}
                    </Tag>
                  ))}
                </div>
              </>
            }
          />
        )}
      </Space>
    </Card>
  );
}

/** 按钮动作编辑器（行操作 / 顶部按钮）：目标弹窗 + 参数映射 + 危险标记。 */
function ActionsEditor({
  title,
  actions,
  dialogOptions,
  rowFields,
  paramOptions,
  onChange,
  mode,
}: {
  title: string;
  actions: ActionDraft[];
  dialogOptions: { value: string; label: string }[];
  rowFields: string[];
  paramOptions: string[];
  onChange: (actions: ActionDraft[]) => void;
  mode: 'toolbar' | 'row';
}) {
  const patch = (i: number, p: Partial<ActionDraft>) =>
    onChange(actions.map((a, idx) => (idx === i ? { ...a, ...p } : a)));

  return (
    <div>
      <Text type="secondary" style={{ fontSize: 11, display: 'block', marginBottom: 4 }}>
        {title}
      </Text>
      {dialogOptions.length === 0 && actions.length === 0 ? (
        <Text type="secondary" style={{ fontSize: 11 }}>
          先把某个 form 区块的展示方式设为「弹窗」
        </Text>
      ) : null}
      {actions.map((a, i) => (
        <div
          key={i}
          style={{ border: '1px solid #f0f0f0', borderRadius: 6, padding: 8, marginBottom: 8 }}
        >
          <Space direction="vertical" size={6} style={{ width: '100%' }}>
            <Input
              size="small"
              placeholder="按钮文案"
              value={a.label}
              onChange={(e) => patch(i, { label: e.target.value })}
              style={{ width: '100%' }}
            />
            <Select
              size="small"
              style={{ width: '100%' }}
              placeholder="打开弹窗"
              value={a.targetSection || undefined}
              onChange={(v) => patch(i, { targetSection: v })}
              options={dialogOptions}
            />
            {a.targetSection && (
              <ParamsMapping
                mapping={a.params}
                rowFields={rowFields}
                paramOptions={paramOptions}
                onChange={(params) => patch(i, { params })}
              />
            )}
            <Space size={8}>
              <label style={{ fontSize: 11 }}>
                <input
                  type="checkbox"
                  checked={a.danger}
                  onChange={(e) => patch(i, { danger: e.target.checked })}
                />{' '}
                危险操作（红字+二次确认）
              </label>
              <Button
                size="small"
                type="link"
                danger
                style={{ padding: 0 }}
                onClick={() => onChange(actions.filter((_, idx) => idx !== i))}
              >
                删除
              </Button>
            </Space>
          </Space>
        </div>
      ))}
      <Button
        size="small"
        type="dashed"
        block
        icon={<PlusOutlined />}
        disabled={dialogOptions.length === 0}
        onClick={() =>
          onChange([...actions, { label: '', targetSection: '', params: {}, danger: false }])
        }
      >
        添加{mode === 'row' ? '行操作' : '按钮'}
      </Button>
    </div>
  );
}

/** 参数映射编辑：行字段 → 弹窗表单参数（row 模式）；静态初值（toolbar 模式）。 */
function ParamsMapping({
  mapping,
  rowFields,
  paramOptions,
  onChange,
}: {
  mapping: Record<string, string>;
  rowFields: string[];
  paramOptions: string[];
  onChange: (m: Record<string, string>) => void;
}) {
  const entries = Object.entries(mapping);
  return (
    <div style={{ fontSize: 11 }}>
      <Text type="secondary">参数带入（行字段 → 表单参数）</Text>
      {entries.map(([param, source]) => (
        <Space key={param} size={4} style={{ display: 'flex', marginBottom: 4 }}>
          <Select
            size="small"
            style={{ width: 120 }}
            value={param}
            onChange={(np) => {
              const next = { ...mapping };
              delete next[param];
              next[np] = source;
              onChange(next);
            }}
            options={(paramOptions.length ? paramOptions : rowFields).map((f) => ({
              value: f,
              label: f,
            }))}
          />
          <span>←</span>
          <Select
            size="small"
            style={{ width: 120 }}
            value={source}
            onChange={(v) => onChange({ ...mapping, [param]: v })}
            options={rowFields.map((f) => ({ value: f, label: `行.${f}` }))}
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
      ))}
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
