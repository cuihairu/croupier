import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { App, Button, Card, Col, Descriptions, Modal, Row, Space, Table, Typography } from 'antd';
import type { FunctionDescriptor } from '@/services/api/functions';
import type { FormPresentationSpec, JSONSchema } from '@/types/dashboard';
import { invokeFunction } from '@/services/api/functions';
import SchemaFormRenderer, { type SchemaFormRendererProps } from '@/components/SchemaFormRenderer';
import { derivePresentationSpec } from '@/utils/schemaHints';
import { parseAction } from './actions';
import type { PageNode } from './model';
import { schemaProperties } from './types';
import { extractErrorMessage } from '@/utils/errors';

const { Text } = Typography;

type JSONRecord = Record<string, unknown>;

function payloadOf(resp: unknown): JSONRecord {
  const r = resp as JSONRecord | undefined;
  if (!r) return {};
  const inner = (r.result ?? r.data) as JSONRecord | undefined;
  return inner && typeof inner === 'object' ? inner : r;
}

function itemsOf(payload: JSONRecord): JSONRecord[] {
  const items = payload.items;
  return Array.isArray(items) ? (items as JSONRecord[]) : [];
}

/**
 * 预览运行时（= 发布后行为的编辑器内等价物）：
 * autoRun 自动执行；button.onClick 动作（打开弹窗/执行/刷新）；
 * fnForm 提交（行内或弹窗）成功后触发 onSuccessRefresh；表格渲染真实数据。
 */
export default function PreviewRuntime({
  tree,
  fnById,
}: {
  tree: PageNode[];
  fnById: Map<string, FunctionDescriptor>;
}) {
  const { message } = App.useApp();
  const [results, setResults] = useState<Record<string, unknown>>({});
  const [running, setRunning] = useState<Record<string, boolean>>({});
  const [dialogId, setDialogId] = useState<string | null>(null);
  // refreshOnNode 级联的同名字段合并输入（与发布运行时 sectionInputs 同语义）。
  const cascadeInputsRef = useRef<Record<string, JSONRecord>>({});
  const runningRef = useRef<Record<string, boolean>>({});
  runningRef.current = running;

  // staticForm 值：StaticFormLive 内防抖后并入 results（与发布运行时的
  // 值缓冲一致），驱动 refreshOnNode 联动。
  const handleStaticChange = useCallback((nodeId: string, values: JSONRecord) => {
    setResults((r) => ({ ...r, [nodeId]: { data: values } }));
  }, []);

  const treeRef = useRef(tree);
  treeRef.current = tree;
  const fnRef = useRef(fnById);
  fnRef.current = fnById;

  const runNode = useCallback(
    async (node: PageNode, params: JSONRecord = {}) => {
      const fid = String(node.props.functionId ?? '');
      if (!fid) return;
      setRunning((r) => ({ ...r, [node.id]: true }));
      try {
        const resp = await invokeFunction(fid, params as never);
        setResults((r) => ({ ...r, [node.id]: resp }));
        // fnForm 成功 → onSuccessRefresh 动作
        if (node.type === 'fnForm') {
          const act = parseAction(node.props.onSuccessRefresh);
          if (act?.kind === 'refreshNode') {
            const target = findIn(treeRef.current, act.target);
            if (target) void runNode(target, {});
          }
        }
      } catch (err) {
        message.error(extractErrorMessage(err, `${String(node.props.title ?? fid)} 执行失败`));
      } finally {
        setRunning((r) => ({ ...r, [node.id]: false }));
      }
    },
    [message],
  );

  const runRef = useRef(runNode);
  runRef.current = runNode;

  const handleAction = useCallback(
    (raw: unknown) => {
      const act = parseAction(raw);
      if (!act) return;
      switch (act.kind) {
        case 'openModal': {
          const target = findIn(treeRef.current, act.target);
          if (!target) {
            message.warning('动作目标不存在（可能已删除）');
            return;
          }
          // toggle 语义：重复点击已打开的弹窗 → 关闭（显示/隐藏直觉）
          setDialogId((cur) => (cur === act.target ? null : act.target));
          break;
        }
        case 'closeModal':
          // 无目标=关当前打开的弹窗；有目标=仅当前打开的是它才关
          setDialogId((cur) => (cur && (!act.target || cur === act.target) ? null : cur));
          break;
        case 'navigate': {
          const url = String(act.params?.url ?? '');
          if (url) window.open(url, '_blank', 'noopener');
          break;
        }
        case 'showMessage':
          message.info(String(act.params?.message ?? ''));
          break;
        default: {
          // runBinding / refreshNode → 执行目标函数组件
          const target = findIn(treeRef.current, act.target);
          if (!target) {
            message.warning('动作目标不存在（可能已删除）');
            return;
          }
          void runRef.current(target);
        }
      }
      // 动作链：后续步骤按序执行
      const chain = (raw as { chain?: Array<{ kind: string; target: string }> })?.chain ?? [];
      for (const step of chain) {
        const node = findIn(treeRef.current, step.target);
        if (node) void runRef.current(node);
      }
    },
    [message],
  );

  // autoRun（进入预览时一次）
  useEffect(() => {
    for (const n of tree) {
      if (n.props.autoRun === true && n.type !== 'modal') void runNode(n);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // refreshOnNode 级联：上游（含 staticForm 值）产出即重跑下游 + 同名字段
  // 合并进输入——语义对齐发布运行时 CompositeRenderer。
  const resultsRef = useRef(results);
  resultsRef.current = results;
  useEffect(() => {
    const nodes = treeRef.current;
    for (const node of nodes) {
      const deps = Array.isArray(node.props.refreshOnNode)
        ? (node.props.refreshOnNode as unknown[]).map(String)
        : [];
      if (deps.length === 0) continue;
      const merged = { ...(cascadeInputsRef.current[node.id] ?? {}) };
      for (const dep of deps) {
        const depData = (resultsRef.current[dep] as { data?: JSONRecord } | undefined)?.data;
        if (depData) Object.assign(merged, depData);
      }
      cascadeInputsRef.current[node.id] = merged;
    }
    for (const node of nodes) {
      const deps = Array.isArray(node.props.refreshOnNode)
        ? (node.props.refreshOnNode as unknown[]).map(String)
        : [];
      if (deps.length === 0) continue;
      if (String(node.props.display ?? '') === 'dialog') continue;
      const depChanged = deps.some((dep) => dep in resultsRef.current);
      if (depChanged && !runningRef.current[node.id]) {
        void runRef.current(node, cascadeInputsRef.current[node.id] ?? {});
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [Object.keys(results).join(',')]);

  const inline = tree.filter((n) => n.type !== 'modal');
  const modals = tree.filter((n) => n.type === 'modal');
  const openModal = modals.find((m) => m.id === dialogId);
  const openForms = (openModal?.children ?? []).filter((c) => c.type === 'fnForm');

  return (
    <>
      <Row gutter={[12, 12]}>
        {inline.map((node) => (
          <Col key={node.id} span={Number(node.props.span ?? 24) || 24}>
            <PreviewNode
              node={node}
              fn={node.props.functionId ? fnById.get(String(node.props.functionId)) : undefined}
              data={results[node.id]}
              running={running[node.id] || false}
              onAction={handleAction}
              onSubmit={(params) => void runNode(node, params)}
              onStaticChange={handleStaticChange}
              renderChild={(child) => (
                <PreviewNode
                  node={child}
                  fn={
                    child.props.functionId ? fnById.get(String(child.props.functionId)) : undefined
                  }
                  data={results[child.id]}
                  running={running[child.id] || false}
                  onAction={handleAction}
                  onSubmit={(params) => void runNode(child, params)}
                  onStaticChange={handleStaticChange}
                />
              )}
            />
          </Col>
        ))}
      </Row>

      {openModal && (
        <Modal
          title={String(openModal.props.title ?? '弹窗')}
          open
          onCancel={() => setDialogId(null)}
          footer={null}
          destroyOnHidden
          width={
            openModal.props.width === 'narrow' ? 420 : openModal.props.width === 'wide' ? 720 : 560
          }
        >
          {openForms.length > 0 ? (
            <Space orientation="vertical" size={16} style={{ width: '100%' }}>
              {openForms.map((form) => (
                <ModalForm
                  key={form.id}
                  fn={fnById.get(String(form.props.functionId))}
                  running={running[form.id] || false}
                  onSubmit={async (params) => {
                    await runNode(form, params);
                    setDialogId(null);
                    message.success(`${String(openModal.props.title ?? '操作')} 执行成功`);
                  }}
                />
              ))}
            </Space>
          ) : (
            <Text type="secondary">弹窗没有内容——编辑态拖入函数表单</Text>
          )}
        </Modal>
      )}
    </>
  );
}

function findIn(nodes: PageNode[], id: string): PageNode | undefined {
  for (const n of nodes) {
    if (n.id === id) return n;
    if (n.children) {
      const hit = findIn(n.children, id);
      if (hit) return hit;
    }
  }
  return undefined;
}

/** 行内节点渲染（预览态，无编辑装饰）。 */
function PreviewNode({
  node,
  fn,
  data,
  running,
  onAction,
  onSubmit,
  onStaticChange,
  renderChild,
}: {
  node: PageNode;
  fn: FunctionDescriptor | undefined;
  data: unknown;
  running: boolean;
  onAction: (raw: unknown) => void;
  onSubmit: (params: JSONRecord) => void;
  /** staticForm 值变化（防抖后）→ 预览页面状态。 */
  onStaticChange?: (nodeId: string, values: JSONRecord) => void;
  /** 容器子节点渲染回调（由主组件注入执行上下文）。 */
  renderChild?: (child: PageNode) => React.ReactNode;
}) {
  const payload = useMemo(() => payloadOf(data), [data]);
  const items = useMemo(() => itemsOf(payload), [payload]);
  const title = String(node.props.title ?? node.type);

  if (node.type === 'text') {
    const level = String(node.props.level ?? 'p');
    const content = String(node.props.content ?? '');
    const inner =
      level === 'h2' ? (
        <Typography.Title level={4}>{content}</Typography.Title>
      ) : level === 'h3' ? (
        <Typography.Title level={5}>{content}</Typography.Title>
      ) : (
        <Text>{content}</Text>
      );
    return node.props.onClick ? (
      <span style={{ cursor: 'pointer' }} onClick={() => onAction(node.props.onClick)}>
        {inner}
      </span>
    ) : (
      inner
    );
  }

  if (node.type === 'button') {
    return (
      <Button
        type={node.props.btnStyle === 'primary' ? 'primary' : 'default'}
        danger={node.props.btnStyle === 'danger'}
        onClick={() => onAction(node.props.onClick)}
      >
        {title}
      </Button>
    );
  }

  return (
    <Card
      size="small"
      title={title}
      loading={running}
      extra={
        (node.type === 'fnTable' || node.type === 'fnFields') && node.props.autoRun !== true ? (
          <Button size="small" onClick={() => onSubmit({})}>
            执行
          </Button>
        ) : null
      }
    >
      {node.type === 'fnTable' ? (
        <Table
          size="small"
          rowKey={(_, i) => String(i)}
          pagination={{ pageSize: 10, showSizeChanger: false }}
          columns={(Array.isArray(node.props.columns) && node.props.columns.length
            ? (node.props.columns as string[])
            : schemaProperties(fn?.outputSchema)
          )
            .slice(0, 8)
            .map((c) => ({
              title: c,
              dataIndex: c,
              ellipsis: true,
            }))}
          dataSource={items}
        />
      ) : node.type === 'fnFields' ? (
        <Descriptions size="small" column={2} bordered>
          {Object.entries(payload)
            .filter(([k]) => k !== 'items' && k !== 'total')
            .slice(0, 10)
            .map(([k, v]) => (
              <Descriptions.Item key={k} label={k}>
                {typeof v === 'object' ? JSON.stringify(v) : String(v ?? '-')}
              </Descriptions.Item>
            ))}
        </Descriptions>
      ) : node.type === 'fnForm' ? (
        <ModalForm fn={fn} running={running} onSubmit={onSubmit} inline />
      ) : node.type === 'staticForm' ? (
        <StaticFormLive
          node={node}
          initialValues={undefined}
          onChange={(values) => onStaticChange?.(node.id, values)}
        />
      ) : node.type === 'container' ? (
        <Space orientation="vertical" size={8} style={{ width: '100%' }}>
          {(node.children ?? []).map((c) => (
            <React.Fragment key={c.id}>
              {renderChild?.(c) ?? <Text type="secondary">{c.type}</Text>}
            </React.Fragment>
          ))}
          {(node.children ?? []).length === 0 && <Text type="secondary">空容器</Text>}
        </Space>
      ) : null}
    </Card>
  );
}

/** 表单（弹窗/行内共用）：复用 SchemaFormRenderer（与发布渲染器同一 RJSF
 * 运行时），控件与校验行为和 Invoke/操作页一致。 */
function ModalForm({
  fn,
  running,
  onSubmit,
  inline,
}: {
  fn: FunctionDescriptor | undefined;
  running: boolean;
  onSubmit: (params: JSONRecord) => void | Promise<void>;
  inline?: boolean;
}) {
  const raw = fn?.inputSchema;
  const schema = raw && typeof raw === 'object' && !Array.isArray(raw) ? (raw as JSONSchema) : null;
  const spec = useMemo(() => derivePresentationSpec(schema), [schema]);
  const properties = schema?.properties;
  if (!fn || !properties || typeof properties !== 'object') {
    return (
      <Button type="primary" block loading={running} onClick={() => void onSubmit({})}>
        确认执行
      </Button>
    );
  }
  return (
    <SchemaFormRenderer
      spec={spec}
      disabled={running}
      onFinish={async (values) => {
        await onSubmit(values as JSONRecord);
      }}
    />
  );
}

/** 常量表单预览态：与发布渲染同一 rjsf 运行时（真实控件可交互），
 * 值防抖并入预览页面状态（驱动预览内 refreshOn/动作链消费）。 */
export function StaticFormLive({
  node,
  initialValues,
  onChange,
  debounceMs = 300,
}: {
  node: PageNode;
  initialValues?: JSONRecord;
  onChange?: (values: JSONRecord) => void;
  debounceMs?: number;
}) {
  const raw = node.props.staticSchema;
  const spec = useMemo<FormPresentationSpec | null>(() => {
    try {
      const schema =
        typeof raw === 'string' ? (JSON.parse(raw) as JSONSchema) : (raw as unknown as JSONSchema);
      if (!schema || typeof schema !== 'object') return null;
      return derivePresentationSpec(schema);
    } catch {
      return null;
    }
  }, [raw]);

  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(
    () => () => {
      if (timerRef.current) clearTimeout(timerRef.current);
    },
    [],
  );

  if (!spec) {
    return <Text type="warning">字段定义 JSON 无效</Text>;
  }
  const handleValuesChange: SchemaFormRendererProps['onValuesChange'] = (_changed, all) => {
    if (timerRef.current) clearTimeout(timerRef.current);
    timerRef.current = setTimeout(() => {
      onChange?.(all as JSONRecord);
    }, debounceMs);
  };
  return (
    <SchemaFormRenderer
      spec={spec}
      initialValues={initialValues as SchemaFormRendererProps['initialValues']}
      hideSubmit
      disabled={false}
      onValuesChange={handleValuesChange}
    />
  );
}
