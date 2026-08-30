import React, { useCallback, useMemo, useRef, useState } from 'react';
import {
  Alert,
  App,
  Button,
  Col,
  Empty,
  Input,
  Row,
  Segmented,
  Space,
  Tag,
  Typography,
} from 'antd';
import { ArrowLeftOutlined, EyeOutlined, SaveOutlined, EditOutlined } from '@ant-design/icons';
import { history, request } from '@umijs/max';
import type { FunctionDescriptor } from '@/services/api/functions';
import { invokeFunction } from '@/services/api/functions';
import { PageContainer } from '@ant-design/pro-components';
import { SortableList } from '@/components/SortableList';
import { CompositeRenderer } from '@/components/PageRenderer';
import type {
  BindingExecutionContext,
  ColumnSpec,
  CompositeSection as SpecSection,
  LocalizedText,
  PageExecuteFn,
  PageExecutionResult,
} from '@/types/dashboard';
import FunctionPanel from './FunctionPanel';
import SectionCard from './SectionCard';
import Inspector from './Inspector';
import TryRunPanel from './TryRunPanel';
import { defaultView, derivePageKey, sectionOutputFields, type SectionDraft } from './types';
import { extractErrorMessage } from '@/utils/errors';

const { Text } = Typography;

type JSONRecord = Record<string, unknown>;

function payloadOf(resp: unknown): JSONRecord {
  const r = resp as JSONRecord | undefined;
  if (!r) return {};
  return (r.data as JSONRecord) ?? r;
}

/**
 * 组合页编辑器：编辑态（左函数/中画布/右属性/底试跑）与预览态
 * （隐藏编辑装饰，autoRun 自动执行 + 联动重跑，即发布后形态）。
 * 试跑与预览执行的结果直接渲染进画布区块。
 */
export default function CompositeEditorPage() {
  const { message, modal } = App.useApp();
  const [sections, setSections] = useState<SectionDraft[]>([]);
  const [selectedKey, setSelectedKey] = useState<string | null>(null);
  const [pageKey, setPageKey] = useState('');
  const [keyTouched, setKeyTouched] = useState(false);
  const [mode, setMode] = useState<'edit' | 'preview'>('edit');

  const fnById = useRef(new Map<string, FunctionDescriptor>());
  const canvasRef = useRef<HTMLDivElement>(null);

  // 执行结果与表单参数（试跑/预览共用，画布渲染真实数据）
  const [runData, setRunData] = useState<Record<string, unknown>>({});
  const [running, setRunning] = useState<Record<string, boolean>>({});
  const formValues = useRef<Record<string, JSONRecord>>({});
  const sectionsRef = useRef<SectionDraft[]>([]);
  sectionsRef.current = sections;

  // pageKey 自动预填
  React.useEffect(() => {
    if (!keyTouched && sections.length > 0) {
      setPageKey((prev) => (prev === derivePageKey(sections) ? prev : derivePageKey(sections)));
    }
  }, [sections, keyTouched]);

  const registerFn = useCallback((fn: FunctionDescriptor) => {
    fnById.current.set(fn.id, fn);
  }, []);

  const addSection = useCallback(
    (fn: FunctionDescriptor) => {
      registerFn(fn);
      setSections((prev) => {
        if (prev.some((s) => s.functionId === fn.id)) {
          message.warning(`函数 ${fn.id} 已在画布中`);
          return prev;
        }
        const view = defaultView(fn);
        const sec: SectionDraft = {
          key: fn.id,
          functionId: fn.id,
          view,
          title: fn.summary?.['zh-CN'] || fn.id,
          span: 24,
          autoRun: view === 'table' || view === 'fields',
          dependsOn: [],
          display: 'inline',
          rowActions: [],
          toolbarActions: [],
          onSuccessRefresh: [],
        };
        setSelectedKey(sec.key);
        return [...prev, sec];
      });
    },
    [message, registerFn],
  );

  const patchSection = useCallback((key: string, patch: Partial<SectionDraft>) => {
    setSections((prev) => prev.map((s) => (s.key === key ? { ...s, ...patch } : s)));
  }, []);

  const removeSection = useCallback((key: string) => {
    setSections((prev) =>
      prev
        .filter((s) => s.key !== key)
        .map((s) => ({ ...s, dependsOn: s.dependsOn.filter((d) => d !== key) })),
    );
    setSelectedKey((k) => (k === key ? null : k));
    setRunData((d) => {
      const next = { ...d };
      delete next[key];
      return next;
    });
  }, []);

  /** 执行区块：结果进画布；refreshOn 联动（上游 data 同名合并进下游输入）自动重跑。 */
  const executeSection = useCallback(
    async (key: string, params: JSONRecord) => {
      const sec = sectionsRef.current.find((s) => s.key === key);
      if (!sec) return;
      setRunning((r) => ({ ...r, [key]: true }));
      try {
        const resp = await invokeFunction(sec.functionId, params as never);
        setRunData((d) => ({ ...d, [key]: resp }));
        const payload = payloadOf(resp);
        for (const down of sectionsRef.current) {
          if (down.dependsOn.includes(key)) {
            const merged = { ...(formValues.current[down.key] ?? {}), ...payload };
            void executeSection(down.key, merged);
          }
        }
      } catch (err) {
        message.error(extractErrorMessage(err, `${sec.title || sec.functionId} 执行失败`));
      } finally {
        setRunning((r) => ({ ...r, [key]: false }));
      }
    },
    [message],
  );

  const handleFormExecute = useCallback(
    (key: string, params: JSONRecord) => {
      formValues.current[key] = params;
      void executeSection(key, params);
    },
    [executeSection],
  );

  // 预览模式：autoRun 区块自动执行
  const autoRunFired = useRef(false);
  React.useEffect(() => {
    if (mode === 'preview' && !autoRunFired.current) {
      autoRunFired.current = true;
      for (const sec of sections) {
        if (sec.autoRun) void executeSection(sec.key, {});
      }
    }
    if (mode === 'edit') autoRunFired.current = false;
  }, [mode, sections, executeSection]);

  const selected = useMemo(
    () => sections.find((s) => s.key === selectedKey),
    [sections, selectedKey],
  );

  const dialogSections = useMemo(() => sections.filter((s) => s.display === 'dialog'), [sections]);

  const depCounts = useMemo(() => {
    const downstream = new Map<string, number>();
    for (const s of sections) {
      for (const d of s.dependsOn) downstream.set(d, (downstream.get(d) ?? 0) + 1);
    }
    return downstream;
  }, [sections]);

  const [saving, setSaving] = useState(false);
  const save = useCallback(async () => {
    const key = pageKey.trim();
    if (!key) {
      message.warning('请填写页面 Key');
      return;
    }
    if (sections.length < 2) {
      message.warning('组合页至少需要 2 个区块');
      return;
    }
    setSaving(true);
    try {
      const body = {
        pageKey: key,
        sections: sections.map((s) => ({
          functionId: s.functionId,
          view: s.view,
          title: s.title,
          span: s.span,
          autoRun: s.autoRun,
          refreshOn: s.dependsOn,
          display: s.display,
          rowActions: s.rowActions,
          toolbarActions: s.toolbarActions,
          onSuccessRefresh: s.onSuccessRefresh,
        })),
      };
      const resp = (await request('/api/v1/versioning/pages/composite', {
        method: 'POST',
        data: body,
      })) as { proposalKey?: unknown };
      modal.success({
        title: '提案已创建',
        content: `组合页提案 ${String(resp?.proposalKey ?? '')} 已进入提案收件箱，接受并发布后生效。`,
        onOk: () => history.push('/functions/pages'),
      });
    } catch (err) {
      message.error(extractErrorMessage(err, '创建提案失败'));
    } finally {
      setSaving(false);
    }
  }, [pageKey, sections, message, modal]);

  const preview = mode === 'preview';

  return (
    <PageContainer
      header={{
        title: '组合页编辑器',
        onBack: () => history.push('/functions/pages'),
        backIcon: <ArrowLeftOutlined />,
        extra: [
          <Segmented
            key="mode"
            value={mode}
            onChange={(v) => setMode(v as 'edit' | 'preview')}
            options={[
              { value: 'edit', label: '编辑', icon: <EditOutlined /> },
              { value: 'preview', label: '预览', icon: <EyeOutlined /> },
            ]}
          />,
          <Button
            key="save"
            type="primary"
            icon={<SaveOutlined />}
            loading={saving}
            onClick={() => void save()}
            disabled={preview}
          >
            保存为提案
          </Button>,
        ],
      }}
    >
      {/* 顶部：页面标识 */}
      {!preview && (
        <Space wrap style={{ marginBottom: 12 }}>
          <Text strong>页面 Key</Text>
          <Input
            placeholder="按区块自动生成，可修改"
            value={pageKey}
            onChange={(e) => {
              setKeyTouched(true);
              setPageKey(e.target.value);
            }}
            style={{ width: 320 }}
            suffix={
              keyTouched ? (
                <Button
                  size="small"
                  type="link"
                  style={{ padding: 0 }}
                  onClick={() => {
                    setKeyTouched(false);
                    setPageKey(derivePageKey(sections));
                  }}
                >
                  恢复自动
                </Button>
              ) : null
            }
          />
          <Text type="secondary">
            {sections.length} 个区块 ·{' '}
            {depCounts.size > 0
              ? `${[...depCounts.values()].reduce((a, b) => a + b, 0)} 处联动`
              : '无联动'}
          </Text>
        </Space>
      )}

      {preview && (
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message="预览态：autoRun 区块已自动执行，操作表单可真实提交，联动自动重跑——即发布后形态。切回编辑继续调整。"
        />
      )}

      <Row gutter={12}>
        {/* 左：函数面板（预览隐藏） */}
        {!preview && (
          <Col flex="280px">
            <FunctionPanel
              addedIds={new Set(sections.map((s) => s.functionId))}
              onAdd={addSection}
            />
          </Col>
        )}

        {/* 中：画布（编辑=真实渲染+装饰；预览=发布后形态） */}
        <Col flex="auto" style={{ minWidth: 420 }}>
          <div
            ref={canvasRef}
            style={{
              border: preview ? 'none' : '1px solid #f0f0f0',
              borderRadius: 8,
              minHeight: preview ? 'calc(100vh - 220px)' : 'calc(100vh - 300px)',
              padding: 12,
              background: preview ? '#fff' : '#fafafa',
            }}
          >
            {preview ? (
              <PreviewRunner sections={sections} fnById={fnById.current} />
            ) : sections.length === 0 ? (
              <Empty
                style={{ marginTop: 120 }}
                description="从左侧点击函数，开始搭建工作台页面"
                image={Empty.PRESENTED_IMAGE_SIMPLE}
              />
            ) : (
              <Row gutter={[12, 12]}>
                <SortableList
                  items={sections.filter((x) => x.display !== 'dialog')}
                  getKey={(s) => s.key}
                  onReorder={(next) =>
                    setSections((prev) => [...next, ...prev.filter((x) => x.display === 'dialog')])
                  }
                  externalDnd
                >
                  {(sec, _idx, dragHandleProps) => (
                    <Col
                      span={sec.span && sec.span < 24 ? sec.span : 24}
                      key={sec.key}
                      style={{ display: preview ? 'block' : undefined }}
                    >
                      <SectionCard
                        section={sec}
                        fn={fnById.current.get(sec.functionId)}
                        selected={selectedKey === sec.key}
                        preview={preview}
                        data={runData[sec.key]}
                        running={running[sec.key]}
                        onSelect={() => setSelectedKey(sec.key)}
                        onDelete={() => removeSection(sec.key)}
                        onExecute={(params) => handleFormExecute(sec.key, params)}
                        onSpanChange={(span) => patchSection(sec.key, { span })}
                        dragHandleProps={dragHandleProps}
                        depCount={sec.dependsOn.length}
                        downstreamCount={depCounts.get(sec.key) ?? 0}
                        canvasWidthRef={canvasRef}
                      />
                    </Col>
                  )}
                </SortableList>
              </Row>
            )}

            {/* 弹窗操作库：display=dialog 的区块（由按钮/行操作触发） */}
            {dialogSections.length > 0 && (
              <div
                style={{
                  marginTop: 12,
                  borderTop: '1px dashed #d9d9d9',
                  paddingTop: 8,
                }}
              >
                <Text strong style={{ fontSize: 12 }}>
                  弹窗操作（{dialogSections.length}）
                </Text>
                <Text type="secondary" style={{ fontSize: 11, marginLeft: 8 }}>
                  由表格行操作 / 顶部按钮触发打开
                </Text>
                <Row gutter={[8, 8]} style={{ marginTop: 8 }}>
                  {dialogSections.map((sec) => (
                    <Col key={sec.key} span={8}>
                      <div
                        onClick={() => setSelectedKey(sec.key)}
                        style={{
                          border: selectedKey === sec.key ? '1px solid #1677ff' : '1px dashed #bbb',
                          borderRadius: 6,
                          padding: '6px 10px',
                          cursor: 'pointer',
                          background: '#fff',
                        }}
                      >
                        <Space size={6}>
                          <Tag color="purple" style={{ marginRight: 0 }}>
                            弹窗
                          </Tag>
                          <Text strong style={{ fontSize: 12 }}>
                            {sec.title || sec.key}
                          </Text>
                        </Space>
                        <div>
                          <Text type="secondary" style={{ fontSize: 11 }}>
                            {sec.functionId}
                          </Text>
                        </div>
                      </div>
                    </Col>
                  ))}
                </Row>
              </div>
            )}
          </div>
        </Col>

        {/* 右：属性面板（预览隐藏） */}
        {!preview && (
          <Col flex="360px">
            <Inspector
              section={selected}
              fn={selected ? fnById.current.get(selected.functionId) : undefined}
              sections={sections}
              fnById={fnById.current}
              onChange={(patch) => selected && patchSection(selected.key, patch)}
              onDelete={() => selected && removeSection(selected.key)}
            />
          </Col>
        )}
      </Row>

      {/* 底：试跑（预览隐藏；结果直接进画布） */}
      {!preview && (
        <div style={{ position: 'sticky', bottom: 0, marginTop: 12 }}>
          <TryRunPanel
            section={selected}
            fn={selected ? fnById.current.get(selected.functionId) : undefined}
            onExecute={(params: Record<string, unknown>) =>
              selected && handleFormExecute(selected.key, params)
            }
            running={selected ? running[selected.key] : false}
          />
        </div>
      )}
    </PageContainer>
  );
}

/** 预览态：复用发布渲染器 CompositeRenderer（弹窗/行操作/工具栏/联动
 * 全部真实可用），执行经 invokeFunction 适配——所见即发布后所得。 */
function PreviewRunner({
  sections,
  fnById,
}: {
  sections: SectionDraft[];
  fnById: Map<string, FunctionDescriptor>;
}) {
  const specSections = useMemo<SpecSection[]>(
    () =>
      sections.map((s) => {
        const fn = fnById.get(s.functionId);
        const title: LocalizedText = { 'zh-CN': s.title || s.functionId };
        const base: SpecSection = {
          key: s.key,
          bindingId: s.key,
          title,
          view: s.view,
          span: s.span,
          autoRun: s.autoRun,
          refreshOn: s.dependsOn,
          display: s.display,
          onSuccessRefresh: s.onSuccessRefresh,
        };
        if (s.view === 'table') {
          const cols: ColumnSpec[] = sectionOutputFields(fn).map((f) => ({
            key: f,
            title: { 'zh-CN': f },
            dataType: 'string',
          }));
          base.table = {
            columns: cols,
            rowActions: s.rowActions.map((a) => ({
              label: { 'zh-CN': a.label },
              targetSection: a.targetSection,
              params: a.params,
              danger: a.danger,
            })),
          };
          if (s.toolbarActions.length) {
            base.toolbar = {
              actions: s.toolbarActions.map((a) => ({
                label: { 'zh-CN': a.label },
                targetSection: a.targetSection,
                params: a.params,
                danger: a.danger,
              })),
            };
          }
        }
        if (s.view === 'form' || s.view === 'actions') {
          base.form = { jsonSchema: (fn?.inputSchema ?? {}) as never };
        }
        return base;
      }),
    [sections, fnById],
  );

  const onExecute = useCallback<PageExecuteFn>(
    async (bindingId, context) => {
      const sec = sections.find((s) => s.key === bindingId);
      if (!sec) throw new Error(`unknown binding ${String(bindingId)}`);
      const form = ((context as BindingExecutionContext).form ?? {}) as Record<string, unknown>;
      const resp = await invokeFunction(sec.functionId, form as never);
      return { kind: 'sync', data: resp.result ?? resp } as unknown as PageExecutionResult;
    },
    [sections],
  );

  return (
    <CompositeRenderer
      sections={specSections}
      bindings={[]}
      onExecute={onExecute}
      preview={false}
    />
  );
}
