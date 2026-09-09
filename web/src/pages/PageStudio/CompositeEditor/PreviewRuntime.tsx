import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  App,
  Button,
  Card,
  Col,
  Descriptions,
  Modal,
  Row,
  Space,
  Switch,
  Table,
  Tag,
  Typography,
} from 'antd';
import type { FunctionDescriptor } from '@/services/api/functions';
import type { FormPresentationSpec, JSONSchema } from '@/types/dashboard';
import { invokeFunction } from '@/services/api/functions';
import { resolveStepParams } from '@/components/PageRenderer';
import SchemaFormRenderer, { type SchemaFormRendererProps } from '@/components/SchemaFormRenderer';
import { derivePresentationSpec } from '@/utils/schemaHints';
import { parseAction } from './actions';
import type { PageNode } from './model';
import { schemaProperties } from './types';
import { extractErrorMessage } from '@/utils/errors';
import { generateMockResponse } from './mockData';

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
  const { message, modal } = App.useApp();
  const [results, setResults] = useState<Record<string, unknown>>({});
  const [running, setRunning] = useState<Record<string, boolean>>({});
  const [dialogId, setDialogId] = useState<string | null>(null);
  // V5：弹窗表单预填初值（行操作/带参动作求值结果，按表单节点 id 键控）
  const [dialogInputs, setDialogInputs] = useState<Record<string, JSONRecord>>({});
  // 模拟数据模式（默认关）：开启后按函数 outputSchema 合成假数据，不调用真实
  // 函数——组装/联动验证无需真实 agent 在线；默认保持真实调用（预览=发布行为）。
  const [mock, setMock] = useState(false);
  const mockRef = useRef(mock);
  mockRef.current = mock;
  // refreshOnNode 级联的同名字段合并输入（与发布运行时 sectionInputs 同语义）。
  const cascadeInputsRef = useRef<Record<string, JSONRecord>>({});
  const runningRef = useRef<Record<string, boolean>>({});
  runningRef.current = running;

  // staticForm 值：StaticFormLive 内防抖后并入 results（与发布运行时的
  // 值缓冲一致），驱动 refreshOnNode 联动。
  const handleStaticChange = useCallback((nodeId: string, values: JSONRecord) => {
    setResults((r) => ({ ...r, [nodeId]: { data: values } }));
  }, []);

  // V5 表格选中行：写入 results[id]（selectedRow/selectedRows；与发布运行时
  // 同一状态形态，不触发 refreshOnNode 联动——refreshOnNode 只看 data 更新）。
  const handleSelectionChange = useCallback((nodeId: string, rows: JSONRecord[]) => {
    setResults((r) => {
      const cur = (r[nodeId] ?? {}) as Record<string, unknown>;
      return { ...r, [nodeId]: { ...cur, selectedRow: rows[0], selectedRows: rows } };
    });
  }, []);
  const treeRef = useRef(tree);
  treeRef.current = tree;
  const fnRef = useRef(fnById);
  fnRef.current = fnById;

  // V5：表达式求值空间——sectionKey → 该节点运行时状态（与发布运行时同构）
  const stateByVar = useMemo(() => {
    const state: Record<string, unknown> = {};
    const walk = (list: PageNode[]) => {
      for (const n of list) {
        const name = typeof n.props.sectionKey === 'string' ? n.props.sectionKey.trim() : '';
        if (name) state[name] = results[n.id];
        if (n.children) walk(n.children);
      }
    };
    walk(tree);
    return state;
  }, [tree, results]);
  const stateByVarRef = useRef(stateByVar);
  stateByVarRef.current = stateByVar;
  // 运行时状态快照 ref（事件帧内求值/执行读取，避免 setState 异步导致同帧读旧值）
  const resultsRef = useRef(results);

  /** 打开弹窗（modal 节点 id）并预填其函数表单初值。
   * inputs 无条件覆盖（替换语义，与发布端 openDialog 一致）——
   * 无参动作打开同一弹窗时不得残留上一次的预填。 */
  const openDialogPrefill = useCallback((modalId: string, inputs: JSONRecord) => {
    const modalNode = findIn(treeRef.current, modalId);
    const form = modalNode?.children?.find((c) => c.type === 'fnForm');
    if (form) {
      setDialogInputs((prev) => ({ ...prev, [form.id]: inputs }));
    }
    setDialogId((cur) => (cur === modalId ? null : modalId));
  }, []);

  const runNode = useCallback(
    async (node: PageNode, params: JSONRecord = {}) => {
      const fid = String(node.props.functionId ?? '');
      if (!fid) return;
      setRunning((r) => ({ ...r, [node.id]: true }));
      try {
        // 模拟模式：按 outputSchema 合成假数据（跳过真实调用与错误提示）
        if (mockRef.current) {
          const mockResp = generateMockResponse(fnRef.current.get(fid));
          setResults((r) => ({ ...r, [node.id]: mockResp ?? { data: {} } }));
        } else {
          const resp = await invokeFunction(fid, params as never);
          setResults((r) => ({ ...r, [node.id]: resp }));
        }
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
        // 真实调用失败（无 agent 在线/契约缺失）→ 引导开启模拟数据
        if (!mockRef.current) {
          message.warning('可开启顶部「模拟数据」安全体验完整流程（不触发真实操作）');
        }
      } finally {
        setRunning((r) => ({ ...r, [node.id]: false }));
      }
    },
    [message],
  );

  const runRef = useRef(runNode);
  runRef.current = runNode;

  const handleAction = useCallback(
    (raw: unknown, ctx?: JSONRecord) => {
      const act = parseAction(raw);
      if (!act) return;
      switch (act.kind) {
        case 'openModal': {
          const target = findIn(treeRef.current, act.target);
          if (!target) {
            message.warning('动作目标不存在（可能已删除）');
            return;
          }
          // V5：params 表达式求值 → 弹窗表单预填
          openDialogPrefill(act.target, resolveStepParams(act.params, stateByVarRef.current, ctx));
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
          // runBinding / refreshNode → 执行目标函数组件（V5：参数表达式求值）
          const target = findIn(treeRef.current, act.target);
          if (!target) {
            message.warning('动作目标不存在（可能已删除）');
            return;
          }
          const resolved = resolveStepParams(act.params, stateByVarRef.current, ctx);
          void runRef.current(target, resolved);
        }
      }
      // 动作链：后续步骤按序执行（同上下文求值）
      const chain =
        (
          raw as {
            chain?: Array<{ kind: string; target: string; params?: Record<string, string> }>;
          }
        )?.chain ?? [];
      for (const step of chain) {
        const node = findIn(treeRef.current, step.target);
        if (node) {
          void runRef.current(node, resolveStepParams(step.params, stateByVarRef.current, ctx));
        }
      }
    },
    [message, openDialogPrefill],
  );

  /** 行操作点击（V5）：行字段映射（row.x / {{row.x}} / 裸字段）→ 预填弹窗。 */
  const handleRowAction = useCallback(
    (raw: unknown, row: JSONRecord) => {
      const ra = raw as {
        label?: unknown;
        targetSection?: unknown;
        params?: Record<string, string>;
        danger?: boolean;
      } | null;
      if (!ra?.targetSection) return;
      const modalNode = findIn(treeRef.current, String(ra.targetSection));
      if (!modalNode) {
        message.warning('动作目标不存在（可能已删除）');
        return;
      }
      const inputs: JSONRecord = {};
      for (const [param, src] of Object.entries(ra.params ?? {})) {
        let field = String(src);
        // 兼容编辑器三种形态：row.x（编译产物）/ {{row.x}}（表达式输入原文）/ 裸字段名
        if (field.startsWith('{{row.') && field.endsWith('}}')) field = field.slice(5, -2);
        else if (field.startsWith('row.')) field = field.slice(4);
        inputs[param] = row[field];
      }
      const open = () => openDialogPrefill(String(ra.targetSection), inputs);
      if (ra.danger) {
        modal.confirm({
          title: `确认执行「${String(ra.label ?? '操作')}」`,
          onOk: open,
        });
        return;
      }
      open();
    },
    [message, modal, openDialogPrefill],
  );

  /** 选中行变化（V5）：同步写入 results/变量快照（同帧事件求值可见）→
   * 触发 onRowSelected 事件动作（选中行作为 row 上下文）。 */
  const handleSelection = useCallback(
    (node: PageNode, rows: JSONRecord[]) => {
      const cur = (resultsRef.current[node.id] ?? {}) as Record<string, unknown>;
      const entry = { ...cur, selectedRow: rows[0], selectedRows: rows };
      resultsRef.current = { ...resultsRef.current, [node.id]: entry };
      setResults(resultsRef.current);
      const name = typeof node.props.sectionKey === 'string' ? node.props.sectionKey.trim() : '';
      if (name) {
        stateByVarRef.current = { ...stateByVarRef.current, [name]: entry };
      }
      if (rows[0]) handleAction(node.props.onRowSelected, rows[0]);
    },
    [handleAction],
  );

  // autoRun（进入预览时一次）
  useEffect(() => {
    for (const n of tree) {
      if (n.props.autoRun === true && n.type !== 'modal') void runNode(n);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  /** 模拟/真实模式切换：同步刷新 ref（同帧生效）→ 清空状态 → 重跑 autoRun 区块。 */
  const applyMockMode = useCallback((v: boolean) => {
    mockRef.current = v;
    setMock(v);
    setResults({});
    cascadeInputsRef.current = {};
    for (const n of treeRef.current) {
      if (n.props.autoRun === true && n.type !== 'modal') void runRef.current(n);
    }
  }, []);

  // refreshOnNode 级联：上游（含 staticForm 值）产出即重跑下游 + 同名字段
  // 合并进输入——语义对齐发布运行时 CompositeRenderer。
  useEffect(() => {
    resultsRef.current = results;
  }, [results]);
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
      {/* 预览工具条：模式常驻可见——模拟=安全探索（无副作用），真实=发布行为 */}
      <div style={{ marginBottom: 12, display: 'flex', alignItems: 'center', gap: 8 }}>
        <Tag color="orange" style={{ marginRight: 0 }}>
          预览
        </Tag>
        <Text type="secondary" style={{ fontSize: 12 }}>
          数据来源
        </Text>
        <Switch
          size="small"
          checked={mock}
          onChange={(v) => {
            applyMockMode(v);
            message.info(
              v
                ? '模拟数据：按 outputSchema 生成假数据，不触发真实操作'
                : '真实调用：将实际执行函数（注意操作类函数有真实副作用）',
            );
          }}
        />
        <Tag
          color={mock ? 'orange' : 'default'}
          style={{ marginRight: 0 }}
          data-mock-state={mock ? 'mock' : 'real'}
        >
          {mock ? '模拟中' : '真实调用'}
        </Tag>
        <Text type="secondary" style={{ fontSize: 11 }}>
          {mock
            ? '假数据按函数 outputSchema 动态生成，可安全验证绑定/联动/弹窗预填'
            : '实际执行函数——操作类函数（如发邮件）将产生真实副作用'}
        </Text>
      </div>
      <Row gutter={[12, 12]}>
        {inline.map((node) => (
          <Col key={node.id} span={Number(node.props.span ?? 24) || 24}>
            <PreviewNode
              node={node}
              fn={node.props.functionId ? fnById.get(String(node.props.functionId)) : undefined}
              data={results[node.id]}
              running={running[node.id] || false}
              onAction={handleAction}
              onRowAction={handleRowAction}
              onSubmit={(params) => void runNode(node, params)}
              onStaticChange={handleStaticChange}
              onSelectionChange={handleSelection}
              renderChild={(child) => (
                <PreviewNode
                  node={child}
                  fn={
                    child.props.functionId ? fnById.get(String(child.props.functionId)) : undefined
                  }
                  data={results[child.id]}
                  running={running[child.id] || false}
                  onAction={handleAction}
                  onRowAction={handleRowAction}
                  onSubmit={(params) => void runNode(child, params)}
                  onStaticChange={handleStaticChange}
                  onSelectionChange={handleSelection}
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
                  initialValues={dialogInputs[form.id]}
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
  onRowAction,
  onSubmit,
  onStaticChange,
  onSelectionChange,
  renderChild,
}: {
  node: PageNode;
  fn: FunctionDescriptor | undefined;
  data: unknown;
  running: boolean;
  onAction: (raw: unknown, ctx?: JSONRecord) => void;
  /** V5：行操作点击（行字段映射 → 弹窗预填），由父级处理目标与求值。 */
  onRowAction?: (raw: unknown, row: JSONRecord) => void;
  onSubmit: (params: JSONRecord) => void;
  /** staticForm 值变化（防抖后）→ 预览页面状态。 */
  onStaticChange?: (nodeId: string, values: JSONRecord) => void;
  /** V5：表格选中行变化 → 预览页面状态（selectedRow/selectedRows）+ 选中事件。 */
  onSelectionChange?: (node: PageNode, rows: JSONRecord[]) => void;
  /** 容器子节点渲染回调（由主组件注入执行上下文）。 */
  renderChild?: (child: PageNode) => React.ReactNode;
}) {
  const payload = useMemo(() => payloadOf(data), [data]);
  const items = useMemo(() => itemsOf(payload), [payload]);
  const title = String(node.props.title ?? node.type);
  // V5：列 = 声明列/schema 字段 + 行操作列（发布行为的预览等价物）
  const rowActionDrafts = Array.isArray(node.props.rowActions)
    ? (node.props.rowActions as Array<Record<string, unknown>>)
    : [];
  const previewColumns = useMemo(() => {
    const base = (
      Array.isArray(node.props.columns) && node.props.columns.length
        ? (node.props.columns as string[])
        : schemaProperties(fn?.outputSchema)
    )
      .slice(0, 8)
      .map((c) => ({ title: c, dataIndex: c, ellipsis: true }));
    if (!rowActionDrafts.length) return base;
    return [
      ...base,
      {
        title: '操作',
        key: '__preview_row_actions',
        render: (_: unknown, row: JSONRecord) => (
          <Space size={4}>
            {rowActionDrafts.map((ra, i) => (
              <Button
                key={i}
                size="small"
                type="link"
                danger={ra.danger === true}
                onClick={() => onRowAction?.(ra, row)}
              >
                {String(ra.label ?? '操作')}
              </Button>
            ))}
          </Space>
        ),
      },
    ];
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [node.props.columns, fn?.outputSchema, rowActionDrafts, onRowAction]);

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
          rowSelection={{
            type: 'radio',
            onChange: (_keys, rows) => {
              // V5：选中行写状态（同步）+ 触发 onRowSelected 事件动作
              onSelectionChange?.(node, rows as JSONRecord[]);
            },
          }}
          columns={previewColumns}
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
  initialValues,
  onSubmit,
  inline,
}: {
  fn: FunctionDescriptor | undefined;
  running: boolean;
  /** V5：弹窗预填初值（行操作/带参 openModal 的求值结果）。 */
  initialValues?: JSONRecord;
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
      initialValues={(initialValues ?? {}) as never}
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
