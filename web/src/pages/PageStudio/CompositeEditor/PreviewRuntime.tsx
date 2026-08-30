import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { App, Button, Card, Col, Descriptions, Modal, Row, Space, Table, Typography } from 'antd';
import type { FunctionDescriptor } from '@/services/api/functions';
import { invokeFunction } from '@/services/api/functions';
import { parseAction } from './actions';
import type { PageNode } from './model';
import { schemaProperties } from './types';
import { getComponent } from './registry';
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
      const target = findIn(treeRef.current, act.target);
      if (!target) {
        message.warning('动作目标不存在（可能已删除）');
        return;
      }
      if (act.kind === 'openModal') {
        setDialogId(act.target);
      } else {
        void runRef.current(target);
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
            <Space direction="vertical" size={16} style={{ width: '100%' }}>
              {openForms.map((form) => (
                <ModalForm
                  key={form.id}
                  node={form}
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
  renderChild,
}: {
  node: PageNode;
  fn: FunctionDescriptor | undefined;
  data: unknown;
  running: boolean;
  onAction: (raw: unknown) => void;
  onSubmit: (params: JSONRecord) => void;
  /** 容器子节点渲染回调（由主组件注入执行上下文）。 */
  renderChild?: (child: PageNode) => React.ReactNode;
}) {
  const payload = useMemo(() => payloadOf(data), [data]);
  const items = useMemo(() => itemsOf(payload), [payload]);
  const title = String(node.props.title ?? node.type);

  if (node.type === 'text') {
    const level = String(node.props.level ?? 'p');
    if (level === 'h2')
      return <Typography.Title level={4}>{String(node.props.content ?? '')}</Typography.Title>;
    if (level === 'h3')
      return <Typography.Title level={5}>{String(node.props.content ?? '')}</Typography.Title>;
    return <Text>{String(node.props.content ?? '')}</Text>;
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
        <ModalForm node={node} fn={fn} running={running} onSubmit={onSubmit} inline />
      ) : node.type === 'container' ? (
        <Space direction="vertical" size={8} style={{ width: '100%' }}>
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

/** 表单（弹窗/行内共用）：按 inputSchema 生成字段，提交执行。 */
function ModalForm({
  node,
  fn,
  running,
  onSubmit,
  inline,
}: {
  node: PageNode;
  fn: FunctionDescriptor | undefined;
  running: boolean;
  onSubmit: (params: JSONRecord) => void | Promise<void>;
  inline?: boolean;
}) {
  const [values, setValues] = useState<JSONRecord>({});
  const schema = (fn?.inputSchema ?? {}) as {
    properties?: Record<string, Record<string, unknown>>;
    required?: string[];
  };
  const names = Object.keys(schema.properties ?? {});
  const required = new Set(schema.required ?? []);

  if (names.length === 0) {
    return (
      <Button type="primary" block loading={running} onClick={() => void onSubmit({})}>
        确认执行
      </Button>
    );
  }

  return (
    <Space direction="vertical" size={10} style={{ width: '100%' }}>
      {names.map((n) => {
        const p = schema.properties?.[n] ?? {};
        const type = typeof p.type === 'string' ? p.type : 'string';
        const label = typeof p.title === 'string' ? p.title : n;
        const v = values[n];
        return (
          <div key={n}>
            <div style={{ fontSize: 12, marginBottom: 4 }}>
              {label}
              {required.has(n) && <span style={{ color: '#ff4d4f' }}> *</span>}
            </div>
            <input
              style={{
                width: '100%',
                height: 30,
                padding: '0 8px',
                border: '1px solid #d9d9d9',
                borderRadius: 6,
              }}
              type={type === 'number' || type === 'integer' ? 'number' : 'text'}
              value={v === undefined || v === null ? '' : String(v)}
              onChange={(e) =>
                setValues((s) => ({
                  ...s,
                  [n]:
                    type === 'number' || type === 'integer'
                      ? Number(e.target.value)
                      : e.target.value,
                }))
              }
            />
          </div>
        );
      })}
      <Button
        type="primary"
        block
        loading={running}
        onClick={() => {
          for (const r of required) {
            if (values[r] === undefined || values[r] === '') {
              return; // 必填缺失：浏览器原生约束兜底，简单静默
            }
          }
          void onSubmit(values);
        }}
      >
        {inline ? '执行' : '提交'}
      </Button>
    </Space>
  );
}

// 保持 registry 引用（PreviewNode 未直接用 def.Preview，预览态独立渲染真实数据）
void getComponent;
