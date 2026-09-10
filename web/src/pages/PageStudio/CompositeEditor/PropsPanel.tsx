import React, { useEffect, useMemo, useState } from 'react';
import { Button, Card, Empty, Input, Space, Tabs, Tooltip, Typography } from 'antd';
import { DeleteOutlined } from '@ant-design/icons';
import SchemaFormRenderer from '@/components/SchemaFormRenderer';
import type { FunctionDescriptor } from '@/services/api/functions';
import { getComponent } from './registry';
import type { PageNode } from './model';
import type { JSONSchema } from '@/types/dashboard';
import { collectVarNames, isValidVarName } from './varname';
import ActionEditor from './ActionEditor';
import RowActionsEditor from './RowActionsEditor';
import ParamMappingEditor from './ParamMappingEditor';
import ConstantFieldsEditor from './ConstantFieldsEditor';
import { schemaProperties } from './types';

const { Text } = Typography;

/** 属性面板：读选中组件 propSchema → rjsf 渲染（amis panelControls 的形）。
 * Appsmith 式分区：配置 / 动作 两个 Tab；选中按钮自动切到「动作」。 */
export default function PropsPanel({
  node,
  nodes,
  allFns,
  fnById,
  onPatch,
  onRenameVariable,
  onDelete,
  onCreateModal,
}: {
  node: PageNode | undefined;
  nodes: PageNode[];
  allFns: FunctionDescriptor[];
  fnById: Map<string, FunctionDescriptor>;
  onPatch: (patch: Record<string, unknown>) => void;
  /** V5：变量改名（同步重写树内引用）；非法/冲突由本组件先校验拦截。 */
  onRenameVariable?: (newName: string) => void;
  onDelete: () => void;
  /** 无弹窗时按钮动作内联创建（建弹窗+装表单+绑定）。 */
  onCreateModal?: (fn: FunctionDescriptor) => void;
}) {
  const def = node ? getComponent(node.type) : undefined;
  const [activeTab, setActiveTab] = useState<string>('config');

  // 选中按钮时自动切到「动作」Tab（用户选按钮通常就是为了配事件）
  useEffect(() => {
    if (node?.type === 'button') setActiveTab('actions');
    else setActiveTab('config');
  }, [node?.id, node?.type]);

  const schema = useMemo(
    () =>
      def
        ? def.propSchema({
            nodes,
            fnById,
            allFns,
            fn: node?.props.functionId ? fnById.get(String(node.props.functionId)) : undefined,
          })
        : undefined,
    [def, nodes, fnById, allFns, node?.props.functionId],
  );

  const fn = node?.props.functionId ? fnById.get(String(node.props.functionId)) : undefined;

  if (!node || !def || !schema) {
    return (
      <Card size="small" title={<Text strong>属性</Text>} styles={{ body: { padding: 16 } }}>
        <Empty description="点击画布组件进行配置" image={Empty.PRESENTED_IMAGE_SIMPLE} />
      </Card>
    );
  }

  const props = (schema.properties ?? {}) as Record<string, JSONSchema>;
  const fmt = (k: string) => (props[k] as { format?: string })?.format;
  const staticSchemaKeys = Object.keys(props).filter((k) => fmt(k) === 'staticSchema');
  const plainKeys = Object.keys(props).filter((k) => !fmt(k));
  const rowActionsKeys = Object.keys(props).filter((k) => fmt(k) === 'rowActions');
  const events = def.events ?? [];
  const hasActions = rowActionsKeys.length > 0 || events.length > 0;
  const plainSchema: JSONSchema = {
    ...schema,
    properties: Object.fromEntries(plainKeys.map((k) => [k, props[k]])),
  };

  return (
    <Card
      size="small"
      title={
        <Space size={6}>
          {def.icon}
          <Text strong>{def.name}</Text>
        </Space>
      }
      extra={
        <Button size="small" type="text" danger icon={<DeleteOutlined />} onClick={onDelete} />
      }
      styles={{ body: { padding: 8, height: 'calc(100vh - 220px)', overflow: 'hidden' } }}
    >
      <Tabs
        size="small"
        activeKey={hasActions ? activeTab : 'config'}
        onChange={setActiveTab}
        style={{ height: '100%' }}
        items={[
          {
            key: 'config',
            label: '配置',
            forceRender: true,
            children: (
              <div style={{ maxHeight: 'calc(100vh - 300px)', overflow: 'auto', paddingRight: 4 }}>
                {staticSchemaKeys.map((key) => (
                  <div key={key} style={{ marginBottom: 12 }}>
                    <Typography.Text
                      type="secondary"
                      style={{ fontSize: 11, display: 'block', marginBottom: 4 }}
                    >
                      {(props[key] as { title?: string })?.title ?? key}
                    </Typography.Text>
                    <ConstantFieldsEditor
                      value={
                        typeof node.props[key] === 'string'
                          ? (node.props[key] as string)
                          : JSON.stringify(node.props[key] ?? {}, null, 2)
                      }
                      onChange={(v) => onPatch({ [key]: v })}
                    />
                  </div>
                ))}
                {(def.category === 'function' ||
                  node.type.startsWith('fn') ||
                  node.type === 'staticForm') && (
                  <VarNameInput
                    node={node}
                    nodes={nodes}
                    onPatch={onPatch}
                    onRename={onRenameVariable}
                  />
                )}
                {(def.category === 'function' || node.type.startsWith('fn')) && (
                  <div style={{ marginBottom: 12 }}>
                    <Typography.Text
                      type="secondary"
                      style={{ fontSize: 11, display: 'block', marginBottom: 4 }}
                    >
                      参数映射
                    </Typography.Text>
                    <ParamMappingEditor
                      fn={fn}
                      nodes={nodes}
                      selfId={node.id}
                      fnById={fnById}
                      value={(node.props.inputAssignments as never[]) ?? []}
                      onChange={(v) =>
                        onPatch({ inputAssignments: v } as unknown as Record<string, unknown>)
                      }
                    />
                  </div>
                )}
                {plainKeys.length > 0 && (
                  <SchemaFormRenderer
                    spec={{ jsonSchema: plainSchema, layout: 'vertical' }}
                    initialValues={node.props as Record<string, never>}
                    hideSubmit
                    onValuesChange={(_, all) => {
                      // SchemaFormRenderer 契约：(changedValues, allValues) 且第一参
                      // 恒为 {}（见其 handleChangeEvent）——必须取第二参，否则编辑
                      // 从不落树。只发与现值不同的键，避免每次击键把整个 props
                      // 全量写入（换绑分支按 functionId 变化判断，全量回写无碍，
                      // 但 diff 后语义更精确）。
                      const next = all as Record<string, unknown>;
                      const patch: Record<string, unknown> = {};
                      for (const k of plainKeys) {
                        if (!Object.is(node.props[k], next[k])) patch[k] = next[k];
                      }
                      if (Object.keys(patch).length > 0) onPatch(patch);
                    }}
                  />
                )}
                {plainKeys.length === 0 && staticSchemaKeys.length === 0 && (
                  <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="无配置字段" />
                )}
              </div>
            ),
          },
          ...(hasActions
            ? [
                {
                  key: 'actions',
                  label: '动作',
                  forceRender: true,
                  children: (
                    <div
                      style={{
                        maxHeight: 'calc(100vh - 300px)',
                        overflow: 'auto',
                        paddingRight: 4,
                      }}
                    >
                      {events.map((ev) => (
                        <div key={ev.name} style={{ marginBottom: 16 }}>
                          <Typography.Text
                            type="secondary"
                            style={{ fontSize: 11, display: 'block', marginBottom: 4 }}
                          >
                            {ev.label}（{ev.name}）
                          </Typography.Text>
                          <ActionEditor
                            value={node.props[ev.name]}
                            nodes={nodes}
                            allFns={allFns}
                            fnById={fnById}
                            onCreateModal={onCreateModal}
                            onChange={(v) => onPatch({ [ev.name]: v ?? undefined })}
                          />
                        </div>
                      ))}
                      {rowActionsKeys.map((key) => (
                        <div key={key} style={{ marginBottom: 12 }}>
                          <Typography.Text
                            type="secondary"
                            style={{ fontSize: 11, display: 'block', marginBottom: 4 }}
                          >
                            行操作（行尾按钮打开弹窗表单）
                          </Typography.Text>
                          <RowActionsEditor
                            value={node.props[key]}
                            nodes={nodes}
                            fnById={fnById}
                            rowFields={schemaProperties(
                              node.props.functionId
                                ? fnById.get(String(node.props.functionId))?.outputSchema
                                : undefined,
                            )}
                            onChange={(v) => onPatch({ [key]: v ?? [] })}
                          />
                        </div>
                      ))}
                    </div>
                  ),
                },
              ]
            : []),
        ]}
      />
    </Card>
  );
}

/** V5 变量名输入框（§3.2）：格式 + 唯一性校验，提交改名时同步重写树内引用。
 * 旧页面回读的遗留 key（如 player.list，非 camelCase）展示琥珀提示，不强制改写。 */
function VarNameInput({
  node,
  nodes,
  onPatch,
  onRename,
}: {
  node: PageNode;
  nodes: PageNode[];
  onPatch: (patch: Record<string, unknown>) => void;
  onRename?: (newName: string) => void;
}) {
  const declared = typeof node.props.sectionKey === 'string' ? node.props.sectionKey : '';
  const [draft, setDraft] = useState(declared);
  const [touched, setTouched] = useState(false);
  useEffect(() => {
    setDraft(declared);
    setTouched(false);
  }, [node.id, declared]);

  const others = useMemo(() => {
    const names = collectVarNames(nodes);
    names.delete(declared);
    return names;
  }, [nodes, declared]);

  const edited = touched && draft !== declared;
  const formatError = edited && draft !== '' && !isValidVarName(draft);
  const conflictError = edited && draft !== '' && others.has(draft);
  const legacy = !!declared && !isValidVarName(declared);
  const invalid = formatError || conflictError;

  const commit = () => {
    setTouched(false);
    const next = draft.trim();
    if (next === declared || invalid || next === '') return;
    // 有 onRenameVariable 走引用同步重写；否则退化为直接 patch（不改引用）
    if (onRename) onRename(next);
    else onPatch({ sectionKey: next });
  };

  return (
    <div style={{ marginBottom: 12 }}>
      <Typography.Text type="secondary" style={{ fontSize: 11, display: 'block', marginBottom: 4 }}>
        变量名（页面内唯一；表达式/refreshOn/参数映射按此引用）
      </Typography.Text>
      <Input
        size="small"
        allowClear
        placeholder="留空自动分配"
        value={draft}
        onChange={(e) => {
          setDraft(e.target.value);
          setTouched(true);
        }}
        onBlur={commit}
        onPressEnter={(e) => (e.target as HTMLInputElement).blur()}
        status={invalid ? 'error' : undefined}
        suffix={
          edited && !invalid ? (
            <Text type="secondary" style={{ fontSize: 11 }}>
              回车确认改名（同步重写引用）
            </Text>
          ) : undefined
        }
      />
      {formatError && (
        <Text type="danger" style={{ fontSize: 11, display: 'block', marginTop: 2 }}>
          格式：小写字母开头的 camelCase（仅 ASCII 字母数字）
        </Text>
      )}
      {conflictError && (
        <Text type="danger" style={{ fontSize: 11, display: 'block', marginTop: 2 }}>
          与其他组件的变量名冲突
        </Text>
      )}
      {legacy && !edited && (
        <Tooltip title="旧页面区块 key，保留原样即可继续被引用；改名后将同步重写全部引用">
          <Text type="warning" style={{ fontSize: 11, display: 'block', marginTop: 2 }}>
            旧 key（非 camelCase）——表达式仍可引用
          </Text>
        </Tooltip>
      )}
    </div>
  );
}
