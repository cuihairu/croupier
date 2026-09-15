/** T9 编辑器内绑定抽屉：画布 unbound 组件就地完成 provider 绑定。
 *
 * 打开时把组件引用的 unbound functionId 溯源回 (sourceId, operationId)
 * （unboundTrace 复刻服务端确定性映射），调用现有 CreateBinding API
 * （kind=provider）保存；函数候选 = bound 描述符 + 运行时 provider 独有
 * 函数（与 CreateBinding 的 registeredFunctionMetaInScope 校验源一致）。
 * 同名绑定走 T6 原地翻转（刷新契约即消标记）；不同名绑定由父组件引导
 * 切换组件函数引用（decideBindOutcome）。
 * OpenAPISources 页的 BindingModal 由此下沉复用，源管理页保留上传入口
 * 与诊断/绑定状态总览。 */
import React, { useEffect, useMemo, useState } from 'react';
import { Alert, App, Button, Drawer, Input, Select, Space, Spin, Tag, Typography } from 'antd';
import { useIntl } from '@umijs/max';
import {
  bindOpenAPISourceProvider,
  getOpenAPISource,
  listOpenAPISources,
  listRuntimeSources,
  type OpenAPISourceOperation,
  type RuntimeProviderItem,
} from '@/services/api/openapi';
import type { FunctionDescriptor } from '@/services/api/functions';
import { unboundFunctionIdForOperation } from './unboundTrace';

const { Text } = Typography;

/** 溯源命中的来源（含全部 operations，供未命中时手动选择）。 */
interface SourceTrace {
  sourceId: string;
  sourceName: string;
  operations: OpenAPISourceOperation[];
  /** unboundId 命中的 operationId 集合 */
  matchedOperationIds: Set<string>;
}

/** 绑定结果分类：同名 → bound 契约原地翻转（T6 语义），刷新即可；
 * 不同名 → bound 契约落在运行时函数名下，组件需切换函数引用才可执行。 */
export function decideBindOutcome(
  unboundFunctionId: string | undefined,
  boundFunctionId: string,
): 'refresh' | 'swap' {
  return boundFunctionId === unboundFunctionId ? 'refresh' : 'swap';
}

/** umi ResponseError 的 body.message → Error.message → fallback。 */
function bindErrorMessage(err: unknown, fallback: string): string {
  if (err && typeof err === 'object') {
    const data = (err as { data?: { message?: unknown } }).data;
    if (data && typeof data.message === 'string' && data.message) return data.message;
    const msg = (err as { message?: unknown }).message;
    if (typeof msg === 'string' && msg) return msg;
  }
  return fallback;
}

export default function BindingDrawer({
  open,
  functionId,
  allFns,
  onClose,
  onBound,
}: {
  open: boolean;
  /** 画布选中组件引用的 unbound 函数 id（上传物料，执行被阻断） */
  functionId?: string;
  /** 当前 scope 描述符（bound 候选来源；绑定后刷新由父组件负责） */
  allFns: FunctionDescriptor[];
  onClose: () => void;
  /** 绑定保存成功：参数为实际绑定的运行时函数 id */
  onBound: (boundFunctionId: string) => void | Promise<void>;
}) {
  const intl = useIntl();
  const { message } = App.useApp();
  const [tracing, setTracing] = useState(false);
  const [sources, setSources] = useState<SourceTrace[]>([]);
  const [runtimeSources, setRuntimeSources] = useState<RuntimeProviderItem[]>([]);
  const [sourceId, setSourceId] = useState<string>();
  const [operationId, setOperationId] = useState<string>();
  const [fnId, setFnId] = useState<string>();
  const [providerId, setProviderId] = useState('');
  const [saving, setSaving] = useState(false);

  // 打开即溯源：拉全部 source 详情按 unboundId 匹配；候选运行时函数并行拉取
  useEffect(() => {
    if (!open || !functionId) return;
    let cancelled = false;
    setTracing(true);
    setSources([]);
    setRuntimeSources([]);
    setSourceId(undefined);
    setOperationId(undefined);
    setFnId(undefined);
    setProviderId('');
    void (async () => {
      try {
        const [listResp, runtimeResp] = await Promise.all([
          listOpenAPISources(),
          listRuntimeSources().catch(() => ({ items: [] as RuntimeProviderItem[], total: 0 })),
        ]);
        if (cancelled) return;
        setRuntimeSources(runtimeResp.items ?? []);
        const details = await Promise.all(
          (listResp.items ?? []).map(async (summary) => {
            try {
              const resp = await getOpenAPISource(summary.sourceId);
              return { summary, operations: resp.source.operations ?? [] };
            } catch {
              return { summary, operations: [] as OpenAPISourceOperation[] };
            }
          }),
        );
        if (cancelled) return;
        const traced: SourceTrace[] = details.map(({ summary, operations }) => {
          const matched = new Set<string>();
          for (const op of operations) {
            if (unboundFunctionIdForOperation(op.operationId, op.path) === functionId) {
              matched.add(op.operationId);
            }
          }
          return {
            sourceId: summary.sourceId,
            sourceName: summary.name,
            operations,
            matchedOperationIds: matched,
          };
        });
        setSources(traced);
        // 单一命中直接预填；多命中取首个（用户可改选）
        const hit = traced.find((t) => t.matchedOperationIds.size > 0);
        if (hit) {
          setSourceId(hit.sourceId);
          setOperationId([...hit.matchedOperationIds][0]);
        }
      } finally {
        if (!cancelled) setTracing(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [open, functionId]);

  // 函数候选：bound 描述符 + 运行时 provider 独有函数（去重，描述符优先）
  const functionOptions = useMemo(() => {
    const descriptorIds = new Set(allFns.map((f) => f.id));
    const runtimeOptions = runtimeSources.flatMap((provider) =>
      provider.functions
        .filter((fn) => !descriptorIds.has(fn))
        .map((fn) => ({
          label: `${fn}${intl.formatMessage(
            {
              id: 'pages.openapiSources.bindingModal.function.runtimeAgent',
              defaultMessage: '（运行时导入 · {agent}）',
            },
            { agent: provider.agentId },
          )}`,
          value: fn,
        })),
    );
    return [
      ...allFns
        .filter((f) => f.executionState !== 'unbound')
        .map((f) => ({ label: f.id, value: f.id })),
      ...runtimeOptions,
    ];
  }, [allFns, runtimeSources, intl]);

  const currentSource = sources.find((s) => s.sourceId === sourceId);
  const currentOperation = currentSource?.operations.find((op) => op.operationId === operationId);
  const matchedSourceIds = new Set(
    sources.filter((s) => s.matchedOperationIds.size > 0).map((s) => s.sourceId),
  );

  const submit = async () => {
    if (!sourceId || !operationId || !fnId) {
      message.warning(
        intl.formatMessage({
          id: 'pages.pageStudio.editor.binding.incomplete',
          defaultMessage: '请选择来源操作与运行时函数',
        }),
      );
      return;
    }
    setSaving(true);
    try {
      await bindOpenAPISourceProvider(sourceId, {
        operationId,
        functionId: fnId,
        providerId: providerId.trim() || undefined,
        bindingId: operationId,
      });
      message.success(
        intl.formatMessage({
          id: 'pages.pageStudio.editor.binding.successToast',
          defaultMessage: '绑定成功，契约状态已刷新',
        }),
      );
      await onBound(fnId);
      onClose();
    } catch (err) {
      message.error(
        bindErrorMessage(
          err,
          intl.formatMessage({
            id: 'pages.pageStudio.editor.binding.saveFailed',
            defaultMessage: '保存 binding 失败',
          }),
        ),
      );
    } finally {
      setSaving(false);
    }
  };

  return (
    <Drawer
      width={480}
      open={open}
      onClose={onClose}
      title={
        <Space size={8} wrap>
          <span>
            {intl.formatMessage({
              id: 'pages.pageStudio.editor.binding.title',
              defaultMessage: '绑定运行时执行器',
            })}
          </span>
          {functionId ? <Text code>{functionId}</Text> : null}
        </Space>
      }
      footer={
        <Space style={{ float: 'right' }}>
          <Button onClick={onClose}>
            {intl.formatMessage({ id: 'app.cancel', defaultMessage: '取消' })}
          </Button>
          <Button type="primary" loading={saving} onClick={submit}>
            {intl.formatMessage({
              id: 'pages.pageStudio.editor.binding.submit',
              defaultMessage: '保存绑定',
            })}
          </Button>
        </Space>
      }
    >
      <Space direction="vertical" size={12} style={{ width: '100%' }}>
        <Alert
          type="info"
          showIcon
          message={intl.formatMessage({
            id: 'pages.pageStudio.editor.binding.hint.title',
            defaultMessage: '该函数来自上传物料，尚未绑定运行时执行器',
          })}
          description={intl.formatMessage({
            id: 'pages.pageStudio.editor.binding.hint.desc',
            defaultMessage:
              '绑定后 operation 的 bound 契约落在所选运行时函数名下；与物料同名时原地翻转（组件无需改动），不同名时可在绑定后切换组件函数引用。',
          })}
        />
        {tracing ? (
          <Spin
            tip={intl.formatMessage({
              id: 'pages.pageStudio.editor.binding.tracing',
              defaultMessage: '正在溯源来源操作…',
            })}
          >
            <div style={{ height: 60 }} />
          </Spin>
        ) : (
          <>
            {sources.length > 0 && matchedSourceIds.size === 0 && (
              <Alert
                type="warning"
                showIcon
                message={intl.formatMessage({
                  id: 'pages.pageStudio.editor.binding.traceFailed',
                  defaultMessage: '未在 OpenAPI Sources 中找到该函数的来源操作',
                })}
                description={intl.formatMessage({
                  id: 'pages.pageStudio.editor.binding.traceFailedDesc',
                  defaultMessage: '可手动选择来源与操作，或前往 OpenAPI Sources 页核查上传物料。',
                })}
              />
            )}
            <div>
              <Text type="secondary" style={{ fontSize: 11, display: 'block', marginBottom: 4 }}>
                {intl.formatMessage({
                  id: 'pages.pageStudio.editor.binding.sourceLabel',
                  defaultMessage: '来源 Source',
                })}
              </Text>
              <Select
                style={{ width: '100%' }}
                placeholder={intl.formatMessage({
                  id: 'pages.pageStudio.editor.binding.sourcePlaceholder',
                  defaultMessage: '选择 OpenAPI Source',
                })}
                value={sourceId}
                onChange={(v) => {
                  setSourceId(v);
                  setOperationId(undefined);
                }}
                options={sources.map((s) => ({
                  value: s.sourceId,
                  label: matchedSourceIds.has(s.sourceId)
                    ? `${s.sourceName}（${s.sourceId}·${intl.formatMessage({
                        id: 'pages.pageStudio.editor.binding.sourceMatched',
                        defaultMessage: '来源匹配',
                      })}）`
                    : `${s.sourceName}（${s.sourceId}）`,
                }))}
              />
            </div>
            <div>
              <Text type="secondary" style={{ fontSize: 11, display: 'block', marginBottom: 4 }}>
                {intl.formatMessage({
                  id: 'pages.pageStudio.editor.binding.operationLabel',
                  defaultMessage: '操作（operationId）',
                })}
              </Text>
              <Select
                style={{ width: '100%' }}
                showSearch
                placeholder={intl.formatMessage({
                  id: 'pages.pageStudio.editor.binding.operationPlaceholder',
                  defaultMessage: '选择 operation',
                })}
                value={operationId}
                onChange={setOperationId}
                options={(currentSource?.operations ?? []).map((op) => ({
                  value: op.operationId,
                  label: currentSource?.matchedOperationIds.has(op.operationId)
                    ? `${op.operationId}（${intl.formatMessage({
                        id: 'pages.pageStudio.editor.binding.sourceMatched',
                        defaultMessage: '来源匹配',
                      })}）`
                    : op.operationId,
                }))}
                notFoundContent={
                  currentSource
                    ? undefined
                    : intl.formatMessage({
                        id: 'pages.pageStudio.editor.binding.sourceFirst',
                        defaultMessage: '先选择来源 Source',
                      })
                }
              />
            </div>
            {currentOperation && (
              <Space size={8} wrap>
                <Tag>
                  {currentOperation.method.toUpperCase()} {currentOperation.path}
                </Tag>
                {currentOperation.bound && currentOperation.functionId ? (
                  <Tag color="green">
                    {intl.formatMessage(
                      {
                        id: 'pages.pageStudio.editor.binding.boundTag',
                        defaultMessage: '已绑定 → {functionId}',
                      },
                      { functionId: currentOperation.functionId },
                    )}
                  </Tag>
                ) : (
                  <Tag color="gold">
                    {intl.formatMessage({
                      id: 'pages.pageStudio.editor.binding.unboundOpTag',
                      defaultMessage: '未绑定',
                    })}
                  </Tag>
                )}
              </Space>
            )}
            <div>
              <Text type="secondary" style={{ fontSize: 11, display: 'block', marginBottom: 4 }}>
                {intl.formatMessage({
                  id: 'pages.pageStudio.editor.binding.functionLabel',
                  defaultMessage: '运行时函数',
                })}
              </Text>
              <Select
                style={{ width: '100%' }}
                showSearch
                placeholder={
                  functionOptions.length > 0
                    ? intl.formatMessage({
                        id: 'pages.pageStudio.editor.binding.functionPlaceholder',
                        defaultMessage: '选择当前 scope 已注册的运行时函数',
                      })
                    : intl.formatMessage({
                        id: 'pages.pageStudio.editor.binding.noCandidates',
                        defaultMessage:
                          '当前 scope 暂无已注册运行时函数（请先在 Agent/SDK 侧注册）',
                      })
                }
                value={fnId}
                onChange={setFnId}
                options={functionOptions}
              />
            </div>
            <div>
              <Text type="secondary" style={{ fontSize: 11, display: 'block', marginBottom: 4 }}>
                {intl.formatMessage({
                  id: 'pages.pageStudio.editor.binding.providerIdLabel',
                  defaultMessage: 'providerId（可选，多 provider 注册同名函数时用于路由）',
                })}
              </Text>
              <Input
                value={providerId}
                onChange={(e) => setProviderId(e.target.value)}
                placeholder="provider:xxx"
                allowClear
              />
            </div>
          </>
        )}
      </Space>
    </Drawer>
  );
}
