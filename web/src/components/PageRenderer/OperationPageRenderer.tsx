import { localizedText } from '@/utils/localizedText';
/**
 * OperationPageRenderer - 操作页面渲染器
 *
 * 渲染独立操作页面，包括：
 * - 表单输入
 * - 确认对话框
 * - 结果展示
 *
 * @module components/PageRenderer/OperationPageRenderer
 */

import React, { useState, useCallback, useRef } from 'react';
import { FormattedMessage, useIntl } from '@umijs/max';
import { App, Card, Button, Modal, Progress, Result, Alert, Space, Tag, Typography } from 'antd';
import {
  CheckCircleOutlined,
  ClockCircleOutlined,
  CloseCircleOutlined,
  ExclamationCircleOutlined,
  SyncOutlined,
} from '@ant-design/icons';
import SchemaFormRenderer from '@/components/SchemaFormRenderer';
import ResultViewRenderer from './ResultViewRenderer';
import ExecutorUnboundAlert, { EXECUTOR_UNBOUND_CODE } from './ExecutorUnboundAlert';
import OperationStageBar from './OperationStageBar';
import { describeConfirmEntries } from './confirmContent';
import { renderConfirmEntries } from './compositeRuntime';
import { getStatusColor, getStatusIcon } from './TaskPageRenderer';
import { extractApiErrorCode, isResultUnknownError } from '@/utils/apiError';
import type {
  OperationPageSpec,
  PageFunctionBinding,
  PageExecuteFn,
  PageExecutionResult,
  ApprovalStatusResult,
  TaskStatusResult,
  FormValues,
} from '@/types/dashboard';

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

export interface OperationPageRendererProps {
  /** 操作页面规格 */
  spec: OperationPageSpec;
  /** 页面绑定 */
  bindings: PageFunctionBinding[];
  /** 执行绑定函数 */
  onExecute: PageExecuteFn;
  /** 预览模式只展示页面结构，禁止触发真实函数执行 */
  preview?: boolean;
  /** 查询审批状态 */
  onQueryApprovalStatus?: (approvalId: string) => Promise<ApprovalStatusResult>;
  /** 查询异步任务状态，供任务结果面板直接查询进度（无任务中心路由，操作页内闭环） */
  onQueryTaskStatus?: (taskId: string) => Promise<TaskStatusResult>;
  /** 页面标题 */
  title?: string;
}

const OperationPageRenderer: React.FC<OperationPageRendererProps> = ({
  spec,
  bindings,
  onExecute,
  preview = false,
  onQueryApprovalStatus,
  onQueryTaskStatus,
  title,
}) => {
  const { message } = App.useApp();
  const intl = useIntl();
  // useIntl 在测试 mock 下每次渲染返回新引用，直接进 useCallback 依赖会让
  // 回调链每渲染重建；经 ref 转发后回调依赖稳定，执行时仍读取最新实例
  const intlRef = useRef(intl);
  intlRef.current = intl;
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<PageExecutionResult | null>(null);
  const [approvalStatus, setApprovalStatus] = useState<ApprovalStatusResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  // T8：稳定错误码（executor_unbound 等）驱动结构化空态分支
  const [errorCode, setErrorCode] = useState<string>('');
  // D4：超时/网络中断=结果未知（请求可能已在服务端执行），不按「已失败」呈现
  const [errorUnknown, setErrorUnknown] = useState(false);
  // D4：任务结果面板可直接查询执行状态（无任务中心路由，操作页内闭环）
  const [taskStatus, setTaskStatus] = useState<TaskStatusResult | null>(null);
  const [taskStatusLoading, setTaskStatusLoading] = useState(false);
  const [confirmVisible, setConfirmVisible] = useState(false);
  const [pendingValues, setPendingValues] = useState<FormValues | null>(null);

  // 查找主绑定
  const mainBinding = bindings.find((b) => b.usage === 'action');
  const requiresConfirm = Boolean(spec.confirm || mainBinding?.execution.requireConfirm);

  // 处理表单提交
  const handleSubmit = useCallback(
    async (values: FormValues) => {
      if (!mainBinding) {
        message.error(
          intlRef.current.formatMessage({
            id: 'component.pageRenderer.operationPage.error.missingBinding',
            defaultMessage: '未配置操作绑定',
          }),
        );
        return;
      }
      if (preview) {
        message.info(
          intlRef.current.formatMessage({
            id: 'component.pageRenderer.operationPage.preview.blocked',
            defaultMessage: '预览模式不执行操作',
          }),
        );
        return;
      }

      // 如果需要确认
      if (requiresConfirm) {
        setPendingValues(values);
        setConfirmVisible(true);
        return;
      }

      // 直接执行
      setLoading(true);
      setError(null);
      setErrorCode('');
      setErrorUnknown(false);
      setResult(null);
      setApprovalStatus(null);
      setTaskStatus(null);

      try {
        const response = await onExecute(mainBinding.id, { form: values });
        setResult(response);

        if (response.kind === 'approval') {
          message.info(
            intlRef.current.formatMessage({
              id: 'component.pageRenderer.operationPage.message.submittedApproval',
              defaultMessage: '操作已提交审批',
            }),
          );
        } else if (response.kind === 'task') {
          message.success(
            intlRef.current.formatMessage({
              id: 'component.pageRenderer.operationPage.message.taskSubmitted',
              defaultMessage: '任务已提交',
            }),
          );
        } else if (response.replayed) {
          // D4：幂等重放——结果取自服务端已落地记录，明确告知未重复执行
          message.success(
            intlRef.current.formatMessage({
              id: 'component.pageRenderer.operationPage.message.replayed',
              defaultMessage: '操作已在先前提交过，本次返回服务端记录的结果（未重复执行）',
            }),
          );
        } else if (spec.resultView?.successMessage) {
          message.success(
            localizedText(
              spec.resultView.successMessage,
              'zh-CN',
              intlRef.current.formatMessage({
                id: 'component.pageRenderer.operationPage.message.success',
                defaultMessage: '操作成功',
              }),
            ),
          );
        } else {
          message.success(
            intlRef.current.formatMessage({
              id: 'component.pageRenderer.operationPage.message.success',
              defaultMessage: '操作成功',
            }),
          );
        }
      } catch (err) {
        const msg =
          err instanceof Error
            ? err.message
            : intlRef.current.formatMessage({
                id: 'component.pageRenderer.operationPage.message.failed',
                defaultMessage: '操作失败',
              });
        setError(msg);
        setErrorCode(extractApiErrorCode(err));
        // D4：超时/网络中断=结果未知，toast 用 warning 措辞而非「操作失败」
        const unknown = isResultUnknownError(err);
        setErrorUnknown(unknown);

        if (unknown) {
          message.warning(
            intlRef.current.formatMessage({
              id: 'component.pageRenderer.operationPage.message.unknownOutcome',
              defaultMessage: '操作结果未知',
            }),
          );
        } else if (spec.resultView?.errorMessage) {
          message.error(
            localizedText(
              spec.resultView.errorMessage,
              'zh-CN',
              intlRef.current.formatMessage({
                id: 'component.pageRenderer.operationPage.message.failed',
                defaultMessage: '操作失败',
              }),
            ),
          );
        } else {
          message.error(
            intlRef.current.formatMessage({
              id: 'component.pageRenderer.operationPage.message.failed',
              defaultMessage: '操作失败',
            }),
          );
        }
      } finally {
        setLoading(false);
      }
    },
    [message, mainBinding, requiresConfirm, spec.resultView, onExecute, preview],
  );

  // 处理确认后执行
  const handleConfirm = useCallback(async () => {
    if (!mainBinding || !pendingValues) {
      return;
    }
    if (preview) {
      message.info(
        intlRef.current.formatMessage({
          id: 'component.pageRenderer.operationPage.preview.blocked',
          defaultMessage: '预览模式不执行操作',
        }),
      );
      return;
    }

    // Close confirm dialog and clear pending only on success; on failure,
    // keep dialog open and values so user can review and retry.
    setConfirmVisible(false);
    setLoading(true);
    setError(null);
    setErrorCode('');
    setErrorUnknown(false);
    setResult(null);
    setApprovalStatus(null);
    setTaskStatus(null);

    try {
      const response = await onExecute(mainBinding.id, { form: pendingValues });
      setResult(response);
      setPendingValues(null);
      if (response.kind === 'approval') {
        message.info(
          intlRef.current.formatMessage({
            id: 'component.pageRenderer.operationPage.message.submittedApproval',
            defaultMessage: '操作已提交审批',
          }),
        );
      } else if (response.kind === 'task') {
        message.success(
          intlRef.current.formatMessage({
            id: 'component.pageRenderer.operationPage.message.taskSubmitted',
            defaultMessage: '任务已提交',
          }),
        );
      } else if (response.replayed) {
        // D4：幂等重放——确认路径与直接执行同口径，替换成功文案
        message.success(
          intlRef.current.formatMessage({
            id: 'component.pageRenderer.operationPage.message.replayed',
            defaultMessage: '操作已在先前提交过，本次返回服务端记录的结果（未重复执行）',
          }),
        );
      } else {
        message.success(
          intlRef.current.formatMessage({
            id: 'component.pageRenderer.operationPage.message.success',
            defaultMessage: '操作成功',
          }),
        );
      }
    } catch (err) {
      const msg =
        err instanceof Error
          ? err.message
          : intlRef.current.formatMessage({
              id: 'component.pageRenderer.operationPage.message.failed',
              defaultMessage: '操作失败',
            });
      setError(msg);
      setErrorCode(extractApiErrorCode(err));
      // D4：超时/网络中断=结果未知，toast 用 warning 措辞而非「操作失败」
      const unknown = isResultUnknownError(err);
      setErrorUnknown(unknown);
      // Re-open confirm dialog on failure so user can retry
      setConfirmVisible(true);
      if (unknown) {
        message.warning(
          intlRef.current.formatMessage({
            id: 'component.pageRenderer.operationPage.message.unknownOutcome',
            defaultMessage: '操作结果未知',
          }),
        );
      } else {
        message.error(
          intlRef.current.formatMessage({
            id: 'component.pageRenderer.operationPage.message.failed',
            defaultMessage: '操作失败',
          }),
        );
      }
    } finally {
      setLoading(false);
    }
  }, [message, mainBinding, pendingValues, onExecute, preview]);

  // 重置
  const handleReset = useCallback(() => {
    setResult(null);
    setApprovalStatus(null);
    setError(null);
    setErrorCode('');
    setErrorUnknown(false);
    setTaskStatus(null);
  }, []);

  const refreshApproval = useCallback(async () => {
    const approvalId = result?.approvalId;
    if (!approvalId || !onQueryApprovalStatus) {
      return;
    }
    try {
      setApprovalStatus(await onQueryApprovalStatus(approvalId));
    } catch (err) {
      const msg =
        err instanceof Error
          ? err.message
          : intlRef.current.formatMessage({
              id: 'component.pageRenderer.operationPage.approval.queryFailed',
              defaultMessage: '审批状态查询失败',
            });
      message.error(msg);
    }
  }, [message, onQueryApprovalStatus, result?.approvalId]);

  // D4：任务结果面板「查询执行状态」——无任务中心路由，操作页内闭环查进度
  const refreshTaskStatus = useCallback(async () => {
    const taskId = result?.taskId || result?.requestId;
    if (!taskId || !onQueryTaskStatus) {
      return;
    }
    setTaskStatusLoading(true);
    try {
      setTaskStatus(await onQueryTaskStatus(taskId));
    } catch (err) {
      const msg =
        err instanceof Error
          ? err.message
          : intlRef.current.formatMessage({
              id: 'component.pageRenderer.operationPage.task.queryFailed',
              defaultMessage: '任务状态查询失败',
            });
      message.error(msg);
    } finally {
      setTaskStatusLoading(false);
    }
  }, [message, onQueryTaskStatus, result?.taskId, result?.requestId]);

  const renderApprovedContinuation = () => {
    if (!approvalStatus || approvalStatus.status !== 'approved' || !approvalStatus.continuation) {
      return null;
    }
    if (approvalStatus.resultKind === 'task' && approvalStatus.taskId) {
      return (
        <Alert
          type="info"
          showIcon
          title={intl.formatMessage({
            id: 'component.pageRenderer.operationPage.approval.taskStarted',
            defaultMessage: '审批已通过，任务已启动',
          })}
          description={<Typography.Text code>{approvalStatus.taskId}</Typography.Text>}
        />
      );
    }
    if (approvalStatus.resultKind === 'sync') {
      return (
        <ResultViewRenderer
          data={approvalStatus.result}
          resultView={spec.resultView}
          emptyTitle={intl.formatMessage({
            id: 'component.pageRenderer.operationPage.approval.resultViewMissing',
            defaultMessage: '审批后执行结果视图未配置',
          })}
        />
      );
    }
    return null;
  };

  // 多阶段反馈（D1 状态反馈）：与结果卡同源派生，追加在结果卡顶部供扫读。
  // 审批状态未查询时按进行中呈现；续跑类别仅在审批通过后有意义。
  const stageInput = {
    loading,
    // D4：结果未知（超时/网络中断）不是「已失败」，阶段条不落 submit error
    hasError: !!error && !errorUnknown,
    resultKind: result?.kind ?? null,
    approvalStatus: approvalStatus?.status ?? null,
    continuationKind: approvalStatus?.continuation ? (approvalStatus.resultKind ?? null) : null,
    // D4：任务路径查询到终态后阶段条随任务状态收口（completed 完成/failed·cancelled error）
    taskStatus: taskStatus?.status ?? null,
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16, maxWidth: 880 }}>
      {/* 表单 */}
      <Card
        title={
          title ||
          intl.formatMessage({
            id: 'component.pageRenderer.operationPage.form.title',
            defaultMessage: '执行操作',
          })
        }
      >
        <SchemaFormRenderer
          spec={spec.form}
          onFinish={handleSubmit}
          disabled={loading || preview}
        />
        {(result || error) && (
          <Button style={{ marginTop: 16 }} onClick={handleReset}>
            <FormattedMessage
              id="component.pageRenderer.operationPage.button.resetResult"
              defaultMessage="重置结果"
            />
          </Button>
        )}
      </Card>

      {/* 确认对话框 */}
      {requiresConfirm && (
        <Modal
          title={localizedText(
            spec.confirm?.title,
            'zh-CN',
            intl.formatMessage({
              id: 'component.pageRenderer.operationPage.confirm.title',
              defaultMessage: '确认操作',
            }),
          )}
          open={confirmVisible}
          onOk={handleConfirm}
          onCancel={() => {
            setConfirmVisible(false);
            setPendingValues(null);
          }}
          okText={localizedText(
            spec.confirm?.confirmText,
            'zh-CN',
            intl.formatMessage({
              id: 'component.pageRenderer.operationPage.confirm.ok',
              defaultMessage: '确定',
            }),
          )}
          cancelText={localizedText(
            spec.confirm?.cancelText,
            'zh-CN',
            intl.formatMessage({
              id: 'component.pageRenderer.operationPage.confirm.cancel',
              defaultMessage: '取消',
            }),
          )}
          confirmLoading={loading}
        >
          {spec.confirm?.description && (
            <p>{localizedText(spec.confirm.description, 'zh-CN', '')}</p>
          )}
          {pendingValues && (
            <div style={{ marginTop: 12 }}>
              {/* D2：确认内容按字段标题/枚举文案/单位/语义强调渲染，不显示原始字段 key */}
              {renderConfirmEntries(describeConfirmEntries(spec.form, pendingValues))}
            </div>
          )}
        </Modal>
      )}

      {/* 结果展示 */}
      {(result || error) && (
        <Card
          title={intl.formatMessage({
            id: 'component.pageRenderer.operationPage.result.title',
            defaultMessage: '执行结果',
          })}
        >
          <OperationStageBar input={stageInput} />
          {error ? (
            // T8：unbound 阻断渲染结构化空态 + 去绑定入口（无伪数据兜底）
            errorCode === EXECUTOR_UNBOUND_CODE ? (
              <ExecutorUnboundAlert functionId={mainBinding?.functionId} />
            ) : errorUnknown ? (
              // D4：超时/网络中断——请求可能已在服务端执行，结果未知；
              // 不按「已失败」提示盲目重复提交，只能经幂等键安全重试。
              <Result
                status="warning"
                title={intl.formatMessage({
                  id: 'component.pageRenderer.operationPage.message.unknownOutcome',
                  defaultMessage: '操作结果未知',
                })}
                subTitle={error}
                icon={<ExclamationCircleOutlined />}
                extra={
                  <Alert
                    type="warning"
                    showIcon
                    title={intl.formatMessage({
                      id: 'component.pageRenderer.operationPage.message.unknownOutcomeHint',
                      defaultMessage:
                        '请求可能已在服务端执行，请勿盲目重复提交；可在服务端幂等保证下安全重试，或稍后查询执行状态。',
                    })}
                  />
                }
              />
            ) : (
              <Result
                status="error"
                title={intl.formatMessage({
                  id: 'component.pageRenderer.operationPage.message.failed',
                  defaultMessage: '操作失败',
                })}
                subTitle={error}
                icon={<CloseCircleOutlined />}
              />
            )
          ) : result?.kind === 'approval' ? (
            <Result
              status="info"
              title={intl.formatMessage({
                id: 'component.pageRenderer.operationPage.approval.pendingTitle',
                defaultMessage: '等待审批',
              })}
              subTitle={intl.formatMessage({
                id: 'component.pageRenderer.operationPage.approval.pendingSubtitle',
                defaultMessage: '审批通过后才会继续执行，请在审批中心查看状态。',
              })}
              icon={<ClockCircleOutlined />}
              extra={
                <Space orientation="vertical">
                  <Alert
                    type="info"
                    showIcon
                    title={intl.formatMessage({
                      id: 'component.pageRenderer.operationPage.approval.incompleteTitle',
                      defaultMessage: '操作尚未完成',
                    })}
                    description={intl.formatMessage({
                      id: 'component.pageRenderer.operationPage.approval.incompleteDescription',
                      defaultMessage: '当前返回的是 approvalId，不代表业务执行成功。',
                    })}
                  />
                  <Typography.Text code>{result.approvalId || result.requestId}</Typography.Text>
                  {approvalStatus ? (
                    <>
                      <Alert
                        type={
                          approvalStatus.status === 'rejected'
                            ? 'error'
                            : approvalStatus.status === 'approved'
                              ? 'success'
                              : 'info'
                        }
                        showIcon
                        title={intl.formatMessage(
                          {
                            id: 'component.pageRenderer.operationPage.approval.statusLabel',
                            defaultMessage: `审批状态：${approvalStatus.status}`,
                          },
                          { status: approvalStatus.status },
                        )}
                        description={approvalStatus.reason || approvalStatus.updatedAt || undefined}
                      />
                      {renderApprovedContinuation()}
                    </>
                  ) : null}
                  {result.approvalId && onQueryApprovalStatus ? (
                    <Button onClick={refreshApproval}>
                      <FormattedMessage
                        id="component.pageRenderer.operationPage.button.refreshApproval"
                        defaultMessage="刷新审批状态"
                      />
                    </Button>
                  ) : null}
                </Space>
              }
            />
          ) : result?.kind === 'task' ? (
            <Result
              status="info"
              title={intl.formatMessage({
                id: 'component.pageRenderer.operationPage.message.taskSubmitted',
                defaultMessage: '任务已提交',
              })}
              subTitle={intl.formatMessage({
                id: 'component.pageRenderer.operationPage.task.runningSubtitle',
                defaultMessage: '异步任务仍在执行，请在任务中心或任务页面查看进度。',
              })}
              icon={<SyncOutlined spin />}
              extra={
                <Space orientation="vertical">
                  <Typography.Text code>{result.taskId || result.requestId}</Typography.Text>
                  {/* D4：查询到任务状态后在面板内展示（无任务中心路由，操作页内闭环） */}
                  {taskStatus && (
                    <Space orientation="vertical" size={4}>
                      <Tag
                        color={getStatusColor(taskStatus.status)}
                        icon={getStatusIcon(taskStatus.status)}
                      >
                        {taskStatus.status.toUpperCase()}
                      </Tag>
                      {taskStatus.progress !== undefined && (
                        <Progress
                          percent={taskStatus.progress}
                          status={
                            taskStatus.status === 'failed' || taskStatus.status === 'cancelled'
                              ? 'exception'
                              : taskStatus.status === 'completed'
                                ? 'success'
                                : 'active'
                          }
                        />
                      )}
                      {taskStatus.message && (
                        <Alert type="info" showIcon title={taskStatus.message} />
                      )}
                      {taskStatus.error && <Alert type="error" showIcon title={taskStatus.error} />}
                    </Space>
                  )}
                  {onQueryTaskStatus && (
                    <Button onClick={refreshTaskStatus} loading={taskStatusLoading}>
                      <FormattedMessage
                        id="component.pageRenderer.operationPage.button.queryTaskStatus"
                        defaultMessage="查询执行状态"
                      />
                    </Button>
                  )}
                </Space>
              }
            />
          ) : (
            <Result
              status="success"
              title={intl.formatMessage(
                result?.replayed
                  ? {
                      id: 'component.pageRenderer.operationPage.message.replayed',
                      defaultMessage: '操作已在先前提交过，本次返回服务端记录的结果（未重复执行）',
                    }
                  : {
                      id: 'component.pageRenderer.operationPage.message.success',
                      defaultMessage: '操作成功',
                    },
              )}
              icon={<CheckCircleOutlined />}
              extra={
                <ResultViewRenderer
                  data={result?.data}
                  resultView={spec.resultView}
                  emptyTitle={intl.formatMessage({
                    id: 'component.pageRenderer.operationPage.result.viewMissing',
                    defaultMessage: '操作结果视图未配置',
                  })}
                />
              }
            />
          )}
        </Card>
      )}
    </div>
  );
};

export default OperationPageRenderer;
