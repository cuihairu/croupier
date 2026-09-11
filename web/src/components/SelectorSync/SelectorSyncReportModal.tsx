import { FormattedMessage, useIntl } from '@umijs/max';
import { Alert, Button, Modal, Popconfirm, Space, Spin, Tag, Typography, message } from 'antd';
import { useCallback, useEffect, useState } from 'react';
import { getPageDraft, syncPageSelectors } from '@/services/api/pages';
import type {
  BindingSelectorSyncReport,
  Diagnostic,
  SelectorSyncAction,
  SelectorSyncConfidence,
} from '@/types/dashboard';

const { Text } = Typography;

/** action → Tag 颜色：与计划约定一致（manual_required 用火山色警示） */
const ACTION_COLORS: Record<SelectorSyncAction, string> = {
  kept: 'default',
  renamed: 'blue',
  removed: 'red',
  added: 'green',
  type_changed: 'orange',
  shape_updated: 'purple',
  manual_required: 'volcano',
};

function isSeverityError(diag: Diagnostic): boolean {
  return diag.severity === 'error';
}

function ConfidenceTag({ confidence }: { confidence?: SelectorSyncConfidence }) {
  if (!confidence) return null;
  return (
    <Tag color={confidence === 'high' ? 'geekblue' : 'gold'}>
      <FormattedMessage
        id={`component.selectorSync.confidence.${confidence}`}
        defaultMessage={confidence === 'high' ? '精确匹配' : '启发式'}
      />
    </Tag>
  );
}

function ActionTag({ action }: { action: SelectorSyncAction }) {
  const labels: Record<SelectorSyncAction, { id: string; fallback: string }> = {
    kept: { id: 'component.selectorSync.action.kept', fallback: '保留' },
    renamed: { id: 'component.selectorSync.action.renamed', fallback: '重映射' },
    removed: { id: 'component.selectorSync.action.removed', fallback: '摘除' },
    added: { id: 'component.selectorSync.action.added', fallback: '补齐' },
    type_changed: { id: 'component.selectorSync.action.type_changed', fallback: '类型变化' },
    shape_updated: { id: 'component.selectorSync.action.shape_updated', fallback: '形状更新' },
    manual_required: {
      id: 'component.selectorSync.action.manual_required',
      fallback: '需人工处理',
    },
  };
  const { id, fallback } = labels[action];
  return (
    <Tag color={ACTION_COLORS[action]}>
      <FormattedMessage id={id} defaultMessage={fallback} />
    </Tag>
  );
}

/**
 * Selector 一键同步报告弹窗：打开即 dry-run 展示同步计划（按 binding
 * 分组、逐条标注动作/置信度/原因），确认后 apply 到草稿（不自动发布）。
 * revision 由组件内 getPageDraft 获取，409 冲突时提示刷新。
 */
export default function SelectorSyncReportModal({
  open,
  pageKey,
  bindingIds,
  onClose,
  onApplied,
}: {
  open: boolean;
  pageKey: string;
  bindingIds?: string[];
  onClose: () => void;
  onApplied?: (revision: number) => void;
}) {
  const intl = useIntl();
  const [loading, setLoading] = useState(false);
  const [applying, setApplying] = useState(false);
  const [errorText, setErrorText] = useState('');
  const [report, setReport] = useState<BindingSelectorSyncReport[] | null>(null);
  const [remaining, setRemaining] = useState<Diagnostic[]>([]);
  const [appliedRevision, setAppliedRevision] = useState<number | null>(null);
  const [draftRevision, setDraftRevision] = useState<number | null>(null);

  const loadPlan = useCallback(async () => {
    if (!open || !pageKey) return;
    setLoading(true);
    setErrorText('');
    try {
      const draft = await getPageDraft(pageKey);
      const revision = draft.draftRevision;
      setDraftRevision(revision);
      setAppliedRevision(null);
      const resp = await syncPageSelectors(pageKey, {
        draftRevision: revision,
        dryRun: true,
        bindingIds,
      });
      setReport(resp.syncedBindings || []);
      setRemaining(resp.remainingDiagnostics || []);
    } catch (err: unknown) {
      setErrorText(err instanceof Error ? err.message : String(err));
      setReport(null);
    } finally {
      setLoading(false);
    }
  }, [open, pageKey, bindingIds]);

  useEffect(() => {
    if (open) {
      loadPlan();
    }
  }, [open, loadPlan]);

  const handleApply = async () => {
    if (draftRevision == null) return;
    setApplying(true);
    try {
      const resp = await syncPageSelectors(pageKey, {
        draftRevision,
        dryRun: false,
        bindingIds,
      });
      setReport(resp.syncedBindings || []);
      setRemaining(resp.remainingDiagnostics || []);
      setAppliedRevision(resp.draftRevision);
      message.success(
        intl.formatMessage(
          {
            id: 'component.selectorSync.applied',
            defaultMessage: '已应用到草稿（版本 {revision}），请检查后手动发布',
          },
          { revision: resp.draftRevision },
        ),
      );
      onApplied?.(resp.draftRevision);
    } catch (err: unknown) {
      message.error(
        `${intl.formatMessage({
          id: 'component.selectorSync.applyFailed',
          defaultMessage: '应用同步失败',
        })}：${err instanceof Error ? err.message : String(err)}`,
      );
    } finally {
      setApplying(false);
    }
  };

  const remainingErrors = remaining.filter(isSeverityError);
  const changedCount = (report || []).filter((item) => item.changed).length;

  return (
    <Modal
      title={intl.formatMessage(
        { id: 'component.selectorSync.title', defaultMessage: '一键同步 Selector（{pageKey}）' },
        { pageKey },
      )}
      open={open}
      onCancel={onClose}
      width={760}
      footer={
        <Space>
          <Button onClick={onClose}>
            <FormattedMessage id="component.selectorSync.close" defaultMessage="关闭" />
          </Button>
          {!appliedRevision && report && report.length > 0 ? (
            <Popconfirm
              title={intl.formatMessage({
                id: 'component.selectorSync.applyConfirmTitle',
                defaultMessage: '应用同步到草稿？',
              })}
              description={intl.formatMessage({
                id: 'component.selectorSync.applyConfirmDescription',
                defaultMessage:
                  '将按以上计划更新草稿并生成新版本（不自动发布）；未受影响的映射与定制全部保留。',
              })}
              onConfirm={() => {
                handleApply();
              }}
            >
              <Button type="primary" loading={applying}>
                <FormattedMessage
                  id="component.selectorSync.apply"
                  defaultMessage="应用同步到草稿"
                />
              </Button>
            </Popconfirm>
          ) : null}
        </Space>
      }
    >
      {loading ? (
        <div style={{ textAlign: 'center', padding: '48px 0' }}>
          <Spin
            tip={intl.formatMessage({
              id: 'component.selectorSync.loading',
              defaultMessage: '正在获取同步计划…',
            })}
          >
            <div style={{ minHeight: 80 }} />
          </Spin>
        </div>
      ) : errorText ? (
        <Alert
          type="error"
          showIcon
          message={intl.formatMessage({
            id: 'component.selectorSync.loadFailed',
            defaultMessage: '加载同步计划失败',
          })}
          description={
            <Space direction="vertical">
              <Text>{errorText}</Text>
              <Button
                size="small"
                onClick={() => {
                  loadPlan();
                }}
              >
                <FormattedMessage id="component.selectorSync.retry" defaultMessage="重试" />
              </Button>
            </Space>
          }
        />
      ) : (
        <Space direction="vertical" size={12} style={{ width: '100%' }}>
          {appliedRevision != null ? (
            <Alert
              type="success"
              showIcon
              message={intl.formatMessage(
                {
                  id: 'component.selectorSync.applied',
                  defaultMessage: '已应用到草稿（版本 {revision}），请检查后手动发布',
                },
                { revision: appliedRevision },
              )}
            />
          ) : null}
          {report && report.length === 0 ? (
            <Alert
              type="info"
              showIcon
              message={intl.formatMessage({
                id: 'component.selectorSync.noBindings',
                defaultMessage: '没有需要同步的 binding',
              })}
            />
          ) : null}
          {report && changedCount > 0 ? (
            <Text type="secondary">
              <FormattedMessage
                id="component.selectorSync.changedCount"
                defaultMessage="{count} 个 binding 将被修改；其余保留原样"
                values={{ count: changedCount }}
              />
            </Text>
          ) : null}
          {(report || []).map((binding) => (
            <div
              key={binding.bindingId}
              style={{
                border: '1px solid #f0f0f0',
                borderRadius: 8,
                padding: '8px 12px',
              }}
            >
              <Space wrap style={{ marginBottom: 4 }}>
                <Tag color={binding.changed ? 'processing' : 'default'}>{binding.bindingId}</Tag>
                {binding.functionId ? <Text code>{binding.functionId}</Text> : null}
                {binding.changed ? (
                  <Tag color="cyan">
                    <FormattedMessage
                      id="component.selectorSync.bindingChanged"
                      defaultMessage="有变更"
                    />
                  </Tag>
                ) : (
                  <Tag>
                    <FormattedMessage
                      id="component.selectorSync.bindingUnchanged"
                      defaultMessage="无变更"
                    />
                  </Tag>
                )}
                {binding.executionModeFixed ? (
                  <Tag color="geekblue">
                    <FormattedMessage
                      id="component.selectorSync.executionModeFixed"
                      defaultMessage="执行模式已修正"
                    />
                  </Tag>
                ) : null}
              </Space>
              {(binding.input || []).length > 0 ? (
                <div>
                  <Text type="secondary">
                    <FormattedMessage
                      id="component.selectorSync.inputSection"
                      defaultMessage="输入映射"
                    />
                  </Text>
                  <ul style={{ margin: '4px 0 8px', paddingLeft: 18 }}>
                    {(binding.input || []).map((entry, i) => (
                      <li key={`${entry.target}:${i}`}>
                        <Space size={4} wrap>
                          <ActionTag action={entry.action} />
                          <Text code>{entry.target}</Text>
                          {entry.newTarget ? (
                            <>
                              <span>→</span>
                              <Text code>{entry.newTarget}</Text>
                            </>
                          ) : null}
                          <ConfidenceTag confidence={entry.confidence} />
                        </Space>
                        <div>
                          <Text type="secondary" style={{ fontSize: 12 }}>
                            {entry.reason}
                          </Text>
                        </div>
                      </li>
                    ))}
                  </ul>
                </div>
              ) : null}
              {(binding.output || []).length > 0 ? (
                <div>
                  <Text type="secondary">
                    <FormattedMessage
                      id="component.selectorSync.outputSection"
                      defaultMessage="输出映射"
                    />
                  </Text>
                  <ul style={{ margin: '4px 0 8px', paddingLeft: 18 }}>
                    {(binding.output || []).map((entry, i) => (
                      <li key={`${entry.stateKey}:${i}`}>
                        <Space size={4} wrap>
                          <ActionTag action={entry.action} />
                          <Text code>{entry.stateKey}</Text>
                          <span>←</span>
                          <Text code>{entry.source || '""'}</Text>
                          {entry.newSource ? (
                            <>
                              <span>→</span>
                              <Text code>{entry.newSource || '""'}</Text>
                            </>
                          ) : null}
                          {entry.required ? (
                            <Tag color="red">
                              <FormattedMessage
                                id="component.selectorSync.requiredOutput"
                                defaultMessage="必需"
                              />
                            </Tag>
                          ) : null}
                          <ConfidenceTag confidence={entry.confidence} />
                        </Space>
                        <div>
                          <Text type="secondary" style={{ fontSize: 12 }}>
                            {entry.reason}
                          </Text>
                        </div>
                      </li>
                    ))}
                  </ul>
                </div>
              ) : null}
              {(binding.manual || []).length > 0 ? (
                <div>
                  <Text type="warning">
                    <FormattedMessage
                      id="component.selectorSync.manualSection"
                      defaultMessage="需人工处理"
                    />
                  </Text>
                  <ul style={{ margin: '4px 0 0', paddingLeft: 18 }}>
                    {(binding.manual || []).map((diag, i) => (
                      <li key={`${diag.code}:${i}`}>
                        <Tag color="volcano">{diag.code}</Tag>
                        <Text>{diag.message}</Text>
                      </li>
                    ))}
                  </ul>
                </div>
              ) : null}
            </div>
          ))}
          {remainingErrors.length > 0 ? (
            <Alert
              type="warning"
              showIcon
              message={intl.formatMessage(
                {
                  id: 'component.selectorSync.remainingErrors',
                  defaultMessage:
                    '同步后仍有 {count} 条错误级诊断，发布会继续被阻断；请处理「需人工处理」项后重试',
                },
                { count: remainingErrors.length },
              )}
              description={
                <ul style={{ margin: 0, paddingLeft: 18 }}>
                  {remainingErrors.slice(0, 8).map((diag, i) => (
                    <li key={`${diag.code}:${i}`}>
                      <Tag color="red">{diag.code}</Tag>
                      <Text>{diag.message}</Text>
                    </li>
                  ))}
                </ul>
              }
            />
          ) : report && report.length > 0 ? (
            <Alert
              type="success"
              showIcon
              message={intl.formatMessage({
                id: 'component.selectorSync.remainingClean',
                defaultMessage: '同步后无错误级诊断，可尝试发布',
              })}
            />
          ) : null}
        </Space>
      )}
    </Modal>
  );
}
