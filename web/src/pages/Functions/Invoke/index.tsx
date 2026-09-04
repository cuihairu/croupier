/** 函数调用工作台：编排状态和调用，展示逻辑由子组件承担。 */
import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { PageContainer } from '@ant-design/pro-components';
import { Alert, App, Button, Card, Col, Row, Select, Space, Tabs, Tag, Typography } from 'antd';
import {
  CloudServerOutlined,
  HistoryOutlined,
  ReloadOutlined,
  SendOutlined,
} from '@ant-design/icons';
import { getLocale, history, useLocation } from '@umijs/max';
import { type SchemaFormRendererHandle } from '@/components/SchemaFormRenderer';
import {
  invokeFunction,
  listDescriptors,
  type FunctionDescriptor,
  type InvokeFunctionOptions,
} from '@/services/api';
import { extractErrorDetails, extractErrorMessage } from '@/utils/errors';
import { deriveSchemaDefaults, parseInputSchema, type JSONSchemaType } from '@/utils/json';
import { derivePresentationSpec } from '@/utils/schemaHints';
import { isScopeReady, subscribeScope } from '@/stores/scope';
import { queryApprovalStatus } from '@/services/console';
import type { ApiErrorDetail } from '@/utils/errors';
import type { FormValues, JSONSchema, JSONValue } from '@/types/dashboard';
import ExecutionOptions from './ExecutionOptions';
import InvocationResponse from './InvocationResponse';
import TaskProgressPanel from './TaskProgressPanel';
import RequestBodyEditor from './RequestBodyEditor';
import RequestHistory from './RequestHistory';
import ServerHistoryPanel from './ServerHistory';
import { startApprovalPolling } from './approvalPolling';
import type { FormSchemaState, RequestHistoryItem } from './types';
import { localizedText } from '@/utils/localizedText';

const { Text } = Typography;
const HISTORY_KEY = 'croupier.function-invoke.history.v1';
const EMPTY_FORM_STATE: FormSchemaState = { status: 'idle' };
const APPROVAL_POLL_INTERVAL_MS = 10000;

/** 审批中状态（A4）：invoke 返回 approvalRequired 时进入轮询直到终态。 */
type PendingApproval = {
  approvalId: string;
  functionId: string;
  status: 'pending' | 'approved' | 'rejected' | 'expired';
  reason?: string;
};

function displayName(descriptor: FunctionDescriptor, locale: string) {
  return (
    localizedText(descriptor.displayName, locale, '') ||
    localizedText(descriptor.summary, locale, '') ||
    descriptor.id
  );
}

function resolveOutputSchema(descriptor: FunctionDescriptor): JSONSchema | null {
  const raw = descriptor.outputSchema;
  if (typeof raw === 'string') {
    return parseInputSchema(raw) as JSONSchema | null;
  }
  if (raw && typeof raw === 'object' && !Array.isArray(raw)) return raw as JSONSchema;
  return null;
}

function resolveSchema(descriptor: FunctionDescriptor): JSONSchemaType | null {
  for (const value of [descriptor.inputSchema, descriptor.schema, descriptor.params]) {
    if (typeof value === 'string') {
      const schema = parseInputSchema(value);
      if (schema) return schema;
    } else if (value && typeof value === 'object' && !Array.isArray(value))
      return value as JSONSchemaType;
  }
  return null;
}

function loadHistory(): RequestHistoryItem[] {
  try {
    const historyItems = JSON.parse(localStorage.getItem(HISTORY_KEY) || '[]');
    return Array.isArray(historyItems) ? historyItems.slice(0, 50) : [];
  } catch {
    return [];
  }
}

export default function FunctionInvokePage() {
  const { message } = App.useApp();
  const locale = getLocale();
  const fid = new URLSearchParams(useLocation().search).get('fid') || '';
  const formRef = useRef<SchemaFormRendererHandle | null>(null);
  const [loading, setLoading] = useState(false);
  const [executing, setExecuting] = useState(false);
  const [descriptors, setDescriptors] = useState<FunctionDescriptor[]>([]);
  const [formState, setFormState] = useState<FormSchemaState>(EMPTY_FORM_STATE);
  const [formValues, setFormValues] = useState<FormValues>({});
  const [rawJson, setRawJson] = useState('{}');
  const [inputMode, setInputMode] = useState<'form' | 'json'>('json');
  const [route, setRoute] = useState<NonNullable<InvokeFunctionOptions['route']>>('lb');
  const [targetServiceId, setTargetServiceId] = useState('');
  const [hashKey, setHashKey] = useState('');
  const [asyncMode, setAsyncMode] = useState(false);
  const [response, setResponse] = useState<JSONValue>();
  const [traceId, setTraceId] = useState('');
  const [error, setError] = useState('');
  const [errorDetails, setErrorDetails] = useState<ApiErrorDetail[]>([]);
  const [activeTaskId, setActiveTaskId] = useState('');
  const [pendingApproval, setPendingApproval] = useState<PendingApproval | null>(null);
  const [duration, setDuration] = useState(0);
  const [historyItems, setHistoryItems] = useState<RequestHistoryItem[]>(loadHistory);
  const [showHistory, setShowHistory] = useState(false);
  const [scopeKey, setScopeKey] = useState('');
  const selected = useMemo(() => descriptors.find((item) => item.id === fid), [descriptors, fid]);

  // 订阅 scope 变化，scope 变更时重新加载函数列表
  useEffect(() => {
    const off = subscribeScope((scope) => {
      setScopeKey(`${scope.gameId || ''}:${scope.env || ''}`);
    });
    return off;
  }, []);

  const refresh = useCallback(async () => {
    // 等待 scope 就绪后再加载
    if (!isScopeReady()) return;
    setLoading(true);
    try {
      setDescriptors(await listDescriptors());
    } catch (err) {
      message.error(extractErrorMessage(err, '加载函数列表失败'));
    } finally {
      setLoading(false);
    }
  }, [message]);
  useEffect(() => {
    refresh();
  }, [refresh, scopeKey]);
  useEffect(() => {
    if (!selected) return setFormState(EMPTY_FORM_STATE);
    const schema = resolveSchema(selected);
    setFormState(
      schema
        ? { status: 'ready', spec: derivePresentationSpec(schema as JSONSchema) }
        : { status: 'unavailable', error: '该函数未声明输入 Schema；请使用原始 JSON 调用。' },
    );
    // 按 Schema 类型派生默认值：default > example > enum 首项 > 类型占位值，
    // 让表单/JSON 编辑器开箱即得完整参数骨架。
    const defaults = deriveSchemaDefaults(schema) as FormValues;
    setFormValues(defaults);
    setRawJson(JSON.stringify(defaults, null, 2));
    setResponse(undefined);
    setError('');
    setErrorDetails([]);
    setActiveTaskId('');
    setPendingApproval(null);
  }, [selected]);
  useEffect(() => {
    try {
      localStorage.setItem(HISTORY_KEY, JSON.stringify(historyItems.slice(0, 50)));
    } catch {
      /* 存储失败不阻断调用 */
    }
  }, [historyItems]);

  const execute = useCallback(async () => {
    if (!selected) return;
    if (route === 'targeted' && !targetServiceId.trim()) {
      setError('指定实例路由需要填写 service_id');
      return;
    }
    if (route === 'hash' && !hashKey.trim()) {
      setError('哈希路由需要填写 hash key');
      return;
    }
    let payload: JSONValue;
    try {
      payload =
        inputMode === 'form' && formState.status === 'ready'
          ? (JSON.parse(JSON.stringify(formRef.current?.getValues() || formValues)) as JSONValue)
          : (JSON.parse(rawJson) as JSONValue);
      if (inputMode === 'form' && formState.status === 'ready' && !formRef.current?.validate())
        throw new Error('表单校验失败');
    } catch (err) {
      setError(`请求体不是有效 JSON：${err instanceof Error ? err.message : String(err)}`);
      return;
    }
    const options: InvokeFunctionOptions = {
      route,
      ...(route === 'targeted' && targetServiceId.trim()
        ? { targetServiceId: targetServiceId.trim() }
        : {}),
      ...(route === 'hash' && hashKey.trim() ? { hashKey: hashKey.trim() } : {}),
      ...(asyncMode ? { mode: 'async' } : {}),
    };
    setExecuting(true);
    setError('');
    setResponse(undefined);
    setTraceId('');
    setDuration(0);
    const startedAt = Date.now();
    try {
      const result = await invokeFunction(selected.id, payload, options);
      setTraceId(result?.traceId || '');
      // 审批流：不写成功历史、不展示结果面板，进入轮询等待审批结论
      if (result.approvalRequired && result.approvalId) {
        setPendingApproval({
          approvalId: result.approvalId,
          functionId: selected.id,
          status: 'pending',
        });
        message.info('该操作需要审批，已提交审批流程');
        return;
      }
      setPendingApproval(null);
      const item: RequestHistoryItem = {
        id: `${startedAt}`,
        functionId: selected.id,
        timestamp: new Date().toISOString(),
        duration: Date.now() - startedAt,
        status: 'success',
        request: payload,
        options,
        response: (result.result ?? result) as JSONValue,
      };
      setDuration(item.duration);
      setResponse(item.response);
      setHistoryItems((items) => [item, ...items].slice(0, 50));
      if (asyncMode && result.taskId) {
        setActiveTaskId(result.taskId);
        message.success(`任务已创建：${result.taskId}`);
      } else {
        message.success('调用成功');
      }
    } catch (err) {
      const detail = extractErrorMessage(err, '调用失败');
      const elapsed = Date.now() - startedAt;
      const item: RequestHistoryItem = {
        id: `${startedAt}`,
        functionId: selected.id,
        timestamp: new Date().toISOString(),
        duration: elapsed,
        status: 'error',
        request: payload,
        options,
        error: detail,
      };
      setDuration(elapsed);
      setError(detail);
      setErrorDetails(extractErrorDetails(err));
      setHistoryItems((items) => [item, ...items].slice(0, 50));
      message.error(detail);
    } finally {
      setExecuting(false);
    }
  }, [
    asyncMode,
    formState.status,
    formValues,
    hashKey,
    inputMode,
    message,
    rawJson,
    route,
    selected,
    targetServiceId,
  ]);
  useEffect(() => {
    const handler = (event: KeyboardEvent) => {
      if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') {
        event.preventDefault();
        execute();
      }
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [execute]);

  // 审批轮询：pending 时立即查询一次并按间隔轮询，到达终态停止
  useEffect(() => {
    if (!pendingApproval || pendingApproval.status !== 'pending') return;
    return startApprovalPolling(
      pendingApproval.approvalId,
      (update) => {
        setPendingApproval((prev) =>
          prev ? { ...prev, status: update.status, reason: update.reason } : prev,
        );
      },
      async (id) => {
        const st = await queryApprovalStatus(id);
        return { status: st.status, reason: st.reason };
      },
      APPROVAL_POLL_INTERVAL_MS,
    );
  }, [pendingApproval]);

  const restore = (item: RequestHistoryItem) => {
    history.push(`/functions/invoke?fid=${encodeURIComponent(item.functionId)}`);
    setRawJson(JSON.stringify(item.request, null, 2));
    setInputMode('json');
    setRoute(item.options.route || 'lb');
    setTargetServiceId(item.options.targetServiceId || '');
    setHashKey(item.options.hashKey || '');
    setResponse(item.response);
    setError(item.error || '');
    setErrorDetails([]);
    setActiveTaskId('');
    setPendingApproval(null);
    setDuration(item.duration);
  };
  const responseRaw = response === undefined ? '' : JSON.stringify(response, null, 2);
  return (
    <PageContainer
      title="函数调用工作台"
      subTitle="构造请求、选择路由并直接查看真实执行结果"
      extra={[
        <Button key="refresh" icon={<ReloadOutlined />} onClick={refresh} loading={loading}>
          刷新函数
        </Button>,
        <Button
          key="history"
          icon={<HistoryOutlined />}
          onClick={() => setShowHistory((value) => !value)}
        >
          历史记录
        </Button>,
      ]}
    >
      <Row gutter={16}>
        <Col xs={24} xl={showHistory ? 17 : 24}>
          <Space direction="vertical" size={16} style={{ width: '100%' }}>
            <Card size="small" bodyStyle={{ padding: 12 }}>
              <Space.Compact style={{ width: '100%' }}>
                <Button style={{ width: 88 }} disabled>
                  POST
                </Button>
                <Select
                  showSearch
                  loading={loading}
                  value={selected?.id}
                  placeholder="选择已注册函数"
                  style={{ width: '100%' }}
                  optionFilterProp="label"
                  onChange={(id) => history.push(`/functions/invoke?fid=${encodeURIComponent(id)}`)}
                  options={descriptors.map((item) => ({
                    value: item.id,
                    label: `${item.id}  ·  ${displayName(item, locale)}`,
                  }))}
                />
                <Button
                  type="primary"
                  icon={<SendOutlined />}
                  loading={executing}
                  disabled={
                    !selected ||
                    (route === 'targeted' && !targetServiceId.trim()) ||
                    (route === 'hash' && !hashKey.trim())
                  }
                  onClick={execute}
                >
                  发送
                </Button>
              </Space.Compact>
              {selected ? (
                <Space wrap style={{ marginTop: 8 }}>
                  <Text strong>{displayName(selected, locale)}</Text>
                  {selected.resource ? <Tag color="blue">{selected.resource}</Tag> : null}
                  <Text type="secondary">{localizedText(selected.description, locale, '')}</Text>
                </Space>
              ) : !loading ? (
                <Alert
                  style={{ marginTop: 12 }}
                  type="info"
                  showIcon
                  message="请选择一个已注册函数后再发送请求"
                />
              ) : null}
            </Card>
            <Card size="small" title="执行选项">
              <ExecutionOptions
                route={route}
                targetServiceId={targetServiceId}
                hashKey={hashKey}
                asyncMode={asyncMode}
                onRouteChange={setRoute}
                onTargetServiceIdChange={setTargetServiceId}
                onHashKeyChange={setHashKey}
                onAsyncModeChange={setAsyncMode}
              />
            </Card>
            <RequestBodyEditor
              mode={inputMode}
              rawJson={rawJson}
              formState={formState}
              formValues={formValues}
              formRef={formRef as React.RefObject<SchemaFormRendererHandle>}
              onModeChange={setInputMode}
              onRawJsonChange={(json) => {
                setRawJson(json);
                // JSON → 表单回写：合法对象时同步表单初值，切回表单模式所见一致；
                // 输入未完成的 JSON 时保留旧表单值
                if (formState.status === 'ready') {
                  try {
                    const parsed: unknown = JSON.parse(json);
                    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
                      setFormValues(parsed as FormValues);
                    }
                  } catch {
                    /* 半成品 JSON 忽略 */
                  }
                }
              }}
              onFormValuesChange={(values) => {
                setFormValues(values);
                setRawJson(JSON.stringify(values, null, 2));
              }}
              onFormat={() => {
                try {
                  setRawJson(JSON.stringify(JSON.parse(rawJson), null, 2));
                } catch {
                  message.error('请求体不是有效 JSON');
                }
              }}
            />
            {pendingApproval ? (
              <Alert
                type={
                  pendingApproval.status === 'rejected'
                    ? 'error'
                    : pendingApproval.status === 'approved'
                      ? 'success'
                      : 'warning'
                }
                showIcon
                message={
                  pendingApproval.status === 'pending'
                    ? '审批中：该操作需要审批通过后才会执行'
                    : pendingApproval.status === 'approved'
                      ? '审批已通过：可重新发起调用'
                      : pendingApproval.status === 'rejected'
                        ? `审批已拒绝${pendingApproval.reason ? `：${pendingApproval.reason}` : ''}`
                        : '审批已过期'
                }
                description={
                  <Space direction="vertical" size={4}>
                    <Text type="secondary">
                      审批单号：{pendingApproval.approvalId}（自动刷新中）
                    </Text>
                    <Space size={8}>
                      <a
                        href={`/approvals?approvalId=${encodeURIComponent(pendingApproval.approvalId)}`}
                        target="_blank"
                        rel="noreferrer"
                      >
                        前往审批中心查看
                      </a>
                      {pendingApproval.status === 'approved' && (
                        <Button size="small" type="primary" onClick={() => void execute()}>
                          重新调用
                        </Button>
                      )}
                    </Space>
                  </Space>
                }
              />
            ) : null}
            {activeTaskId ? (
              <TaskProgressPanel
                taskId={activeTaskId}
                onCompleted={(result) => {
                  setResponse(result ?? undefined);
                  setRawJson(JSON.stringify(result ?? null, null, 2));
                }}
              />
            ) : null}
            <InvocationResponse
              responseRaw={responseRaw}
              error={error}
              errorDetails={errorDetails}
              response={response}
              outputSchema={selected ? resolveOutputSchema(selected) : null}
              duration={duration}
              traceId={traceId}
              onCopy={(value) =>
                navigator.clipboard.writeText(value).then(() => message.success('已复制'))
              }
            />
          </Space>
        </Col>
        {showHistory ? (
          <Col xs={24} xl={7}>
            <Card size="small" title="历史记录">
              <Tabs
                items={[
                  {
                    key: 'local',
                    label: '本地草稿',
                    children: (
                      <RequestHistory
                        items={historyItems}
                        onClear={() => setHistoryItems([])}
                        onSelect={restore}
                      />
                    ),
                  },
                  {
                    key: 'server',
                    label: '服务端记录',
                    children: <ServerHistoryPanel />,
                  },
                ]}
              />
            </Card>
          </Col>
        ) : null}
      </Row>
    </PageContainer>
  );
}
