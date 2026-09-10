import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { App, Col, Modal, Row, Space, Switch, Tag, Typography } from 'antd';
import type { FunctionDescriptor } from '@/services/api/functions';
import type { JSONValue } from '@/types/dashboard';
import { invokeFunction } from '@/services/api/functions';
import { resolveStepParams } from '@/components/PageRenderer';
import { parseAction, type ActionSpec, type ActionStep } from './actions';
import type { PageNode } from './model';
import { extractErrorMessage } from '@/utils/errors';
import { generateMockResponse } from './mockData';
import PreviewNode, { ModalForm } from './PreviewNode';
import { findIn, payloadOf, type JSONRecord, type StepLike } from './previewShared';

const { Text } = Typography;

/**
 * 预览运行时（= 发布后行为的编辑器内等价物）：
 * autoRun 自动执行；button.onClick 动作（打开弹窗/执行/刷新）；
 * fnForm 提交（行内或弹窗）成功后触发 onSuccessRefresh；表格渲染真实数据。
 * 数据流对齐发布端 CompositeRenderer：函数输出归一为 {data} 页面状态、
 * 表单当前值写入 values、inputAssignments 显式映射求值、失败保持弹窗打开。
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
  // 模拟数据空态已警示的节点（每节点一次，避免 autoRun 反复弹提示）
  const mockWarnedRef = useRef(new Set<string>());
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
   * 无参动作打开同一弹窗时不得残留上一次的预填；
   * 且必须无条件打开（toggle 语义会让「弹窗开着时再触发动作」误关闭弹窗）。 */
  const openDialogPrefill = useCallback((modalId: string, inputs: JSONRecord) => {
    const modalNode = findIn(treeRef.current, modalId);
    const form = modalNode?.children?.find((c) => c.type === 'fnForm');
    if (form) {
      setDialogInputs((prev) => ({ ...prev, [form.id]: inputs }));
    }
    setDialogId(modalId);
  }, []);

  /** 显式参数映射求值（对齐发布端服务端 inputAssignment 语义）：
   * page_state → 页面状态 var[key] 按字段路径取值（编译产物 /a/b JSON Pointer
   * 的前端等价物）；literal → 原值（单表达式 {{var.path}} 在预览内就地求值，
   * 与编译端「表达式字面量编译为 page_state」规则呼应）。 */
  const evalInputAssignments = useCallback((node: PageNode): JSONRecord => {
    const raw = Array.isArray(node.props.inputAssignments)
      ? (node.props.inputAssignments as Array<Record<string, unknown>>)
      : [];
    const out: JSONRecord = {};
    for (const m of raw) {
      const param = String(m.param ?? '').replace(/^\//, '');
      if (!param) continue;
      if (m.kind === 'page_state') {
        const srcNode = findIn(treeRef.current, String(m.sourceNodeId ?? ''));
        const varName =
          typeof srcNode?.props.sectionKey === 'string' && srcNode.props.sectionKey.trim()
            ? srcNode.props.sectionKey.trim()
            : String(m.sourceNodeId ?? '');
        let cur: unknown = stateByVarRef.current[varName];
        for (const seg of String(m.field ?? '')
          .split('/')
          .filter(Boolean)) {
          cur = cur && typeof cur === 'object' ? (cur as JSONRecord)[seg] : undefined;
          if (cur === undefined) break;
        }
        out[param] = cur;
      } else {
        const v = m.value;
        out[param] =
          typeof v === 'string' && v.startsWith('{{')
            ? resolveStepParams({ [param]: v }, stateByVarRef.current)?.[param]
            : v;
      }
    }
    return out;
  }, []);

  const runNode = useCallback(
    async (node: PageNode, params: JSONRecord = {}): Promise<boolean> => {
      const fid = String(node.props.functionId ?? '');
      if (!fid) return false;
      setRunning((r) => ({ ...r, [node.id]: true }));
      try {
        // 输入合并（对齐发布端 runSection 的 {...sectionInputs, ...overrides}）：
        // refreshOn 级联同名字段 < inputAssignments 显式映射 < 动作显式参数。
        const merged: JSONRecord = {
          ...(cascadeInputsRef.current[node.id] ?? {}),
          ...evalInputAssignments(node),
          ...params,
        };
        if (mockRef.current) {
          // 模拟模式：按 outputSchema 合成假数据（跳过真实调用与错误提示）
          const mockResp = generateMockResponse(fnRef.current.get(fid));
          // 无 schema/顶层结构不支持 → 模拟数据为空（发布端真实调用同样为空，
          // 不伪造数据造成预览/发布分叉）——每节点提示一次即可
          if (!mockResp && !mockWarnedRef.current.has(node.id)) {
            mockWarnedRef.current.add(node.id);
            message.warning(
              `「${String(node.props.title ?? fid)}」无可用 outputSchema（或顶层结构不支持），模拟数据为空`,
            );
          }
          setResults((r) => ({ ...r, [node.id]: mockResp ?? { data: {} } }));
        } else {
          // 真实响应归一为 {data: payload}：FunctionInvokeResponse.result 才是
          // 函数输出——与发布端 {{var.data.x}} / 级联 .data 的读取形态同构
          // （直接存原始响应会让两者恒 undefined）。
          const resp = (await invokeFunction(fid, merged as JSONValue)) as JSONRecord;
          if (typeof resp?.error === 'string' && resp.error) return false;
          setResults((r) => ({ ...r, [node.id]: { data: payloadOf(resp) } }));
        }
        // fnForm 成功 → 刷新下游（失败路径不触发）。两条来源（编译产物已去重，
        // 预览侧再防御性去重）：props.onSuccess（规范路径）+ 遗留 props.onSuccessRefresh。
        if (node.type === 'fnForm') {
          const fired = new Set<string>();
          const fireRefresh = (raw: unknown) => {
            const act = parseAction(raw);
            if (act?.kind === 'refreshNode' && !fired.has(act.target)) {
              fired.add(act.target);
              const target = findIn(treeRef.current, act.target);
              if (target) void runNode(target, {});
            }
          };
          fireRefresh(node.props.onSuccess);
          fireRefresh(node.props.onSuccessRefresh);
        }
        return true;
      } catch (err) {
        message.error(extractErrorMessage(err, `${String(node.props.title ?? fid)} 执行失败`));
        // 真实调用失败（无 agent 在线/契约缺失）→ 引导开启模拟数据
        if (!mockRef.current) {
          message.warning('可开启顶部「模拟数据」安全体验完整流程（不触发真实操作）');
        }
        return false;
      } finally {
        setRunning((r) => ({ ...r, [node.id]: false }));
      }
    },
    [message, evalInputAssignments],
  );

  const runRef = useRef(runNode);
  runRef.current = runNode;

  /** 单步动作执行（主动作与链步骤共用）：按 kind 分派——
   * openModal/closeModal/navigate/showMessage 立即生效；
   * runBinding/refreshNode 执行目标函数组件（参数表达式求值）。
   * 链步骤不再只找节点执行——否则 openModal/navigate 等无目标步骤静默丢弃。 */
  const runStep = useCallback(
    (step: StepLike, ctx?: JSONRecord) => {
      const kind = String(step.kind ?? 'refreshNode');
      const target = String(step.target ?? '');
      switch (kind) {
        case 'openModal': {
          const node = findIn(treeRef.current, target);
          if (!node) {
            message.warning('动作目标不存在（可能已删除）');
            return;
          }
          // V5：params 表达式求值 → 弹窗表单预填
          openDialogPrefill(target, resolveStepParams(step.params, stateByVarRef.current, ctx));
          return;
        }
        case 'closeModal':
          // 无目标=关当前打开的弹窗；有目标=仅当前打开的是它才关
          setDialogId((cur) => (cur && (!target || cur === target) ? null : cur));
          return;
        case 'navigate': {
          const url = String(step.params?.url ?? '');
          if (url) window.open(url, '_blank', 'noopener');
          return;
        }
        case 'showMessage':
          message.info(String(step.params?.message ?? ''));
          return;
        default: {
          // runBinding / refreshNode → 执行目标函数组件（V5：参数表达式求值）
          const node = findIn(treeRef.current, target);
          if (!node) {
            message.warning('动作目标不存在（可能已删除）');
            return;
          }
          void runRef.current(node, resolveStepParams(step.params, stateByVarRef.current, ctx));
        }
      }
    },
    [message, openDialogPrefill],
  );

  const runStepRef = useRef(runStep);
  runStepRef.current = runStep;

  const handleAction = useCallback(
    (raw: unknown, ctx?: JSONRecord) => {
      const act = parseAction(raw) as (ActionSpec & StepLike) | null;
      if (!act) return;
      runStep(act, ctx);
      // 动作链：后续步骤按 kind 分派（同上下文求值，对齐发布端 runChain）
      const chain = (raw as { chain?: ActionStep[] | undefined })?.chain ?? [];
      for (const step of chain) runStep(step, ctx);
    },
    [runStep],
  );

  /** 行操作点击（V5）：行字段映射（row.x / {{row.x}} / 裸字段）→ 预填弹窗，
   * 再执行行操作链（对齐发布端 openDialog 后 runChain，行数据作求值上下文）。 */
  const handleRowAction = useCallback(
    (raw: unknown, row: JSONRecord) => {
      const ra = raw as {
        label?: unknown;
        targetSection?: unknown;
        params?: Record<string, string>;
        danger?: boolean;
        chain?: ActionStep[];
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
        if (field.startsWith('{{row.') && field.endsWith('}}')) field = field.slice(6, -2);
        else if (field.startsWith('row.')) field = field.slice(4);
        inputs[param] = row[field];
      }
      const openWithChain = () => {
        openDialogPrefill(String(ra.targetSection), inputs);
        for (const step of ra.chain ?? []) runStepRef.current(step, row);
      };
      if (ra.danger) {
        modal.confirm({
          title: `确认执行「${String(ra.label ?? '操作')}」`,
          onOk: openWithChain,
        });
        return;
      }
      openWithChain();
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

  /** 表单当前值写入页面状态 results[id].values（{{var.values.x}} 求值来源）——
   * 对齐发布端 valuesMergeRef 防抖合并（预览规模小，直写即可）。 */
  const handleFormValues = useCallback((formNodeId: string, values: JSONRecord) => {
    setResults((r) => {
      const cur = (r[formNodeId] ?? {}) as JSONRecord;
      return { ...r, [formNodeId]: { ...cur, values } };
    });
  }, []);

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
              cascadeInputs={cascadeInputsRef.current[node.id]}
              onAction={handleAction}
              onRowAction={handleRowAction}
              onSubmit={(params) => void runNode(node, params)}
              onFormValues={handleFormValues}
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
                  cascadeInputs={cascadeInputsRef.current[child.id]}
                  onAction={handleAction}
                  onRowAction={handleRowAction}
                  onSubmit={(params) => void runNode(child, params)}
                  onFormValues={handleFormValues}
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
                  onValuesChange={(values) => handleFormValues(form.id, values)}
                  onSubmit={async (params) => {
                    // 失败：保持弹窗打开（可改参数重试），无成功提示——
                    // 对齐发布端「runSection 异常时不关弹窗」的行为。
                    const ok = await runNode(form, params);
                    if (!ok) return;
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

// 常量表单实时态供模板预览等外部消费（历史导出路径不变，测试零改动）
export { StaticFormLive } from './PreviewNode';
