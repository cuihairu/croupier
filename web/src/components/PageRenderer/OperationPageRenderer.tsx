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
import { App, Card, Button, Modal, Result, Alert, Space, Typography, Descriptions } from 'antd';
import {
  CheckCircleOutlined,
  ClockCircleOutlined,
  CloseCircleOutlined,
  SyncOutlined,
} from '@ant-design/icons';
import SchemaFormRenderer from '@/components/SchemaFormRenderer';
import ResultViewRenderer, { renderJSONValueSummary } from './ResultViewRenderer';
import type {
  OperationPageSpec,
  PageFunctionBinding,
  PageExecuteFn,
  PageExecutionResult,
  ApprovalStatusResult,
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
  /** 页面标题 */
  title?: string;
}

const OperationPageRenderer: React.FC<OperationPageRendererProps> = ({
  spec,
  bindings,
  onExecute,
  preview = false,
  onQueryApprovalStatus,
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
      setResult(null);
      setApprovalStatus(null);

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

        if (spec.resultView?.errorMessage) {
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

    setConfirmVisible(false);
    setLoading(true);
    setError(null);
    setResult(null);
    setApprovalStatus(null);

    try {
      const response = await onExecute(mainBinding.id, { form: pendingValues });
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
      message.error(
        intlRef.current.formatMessage({
          id: 'component.pageRenderer.operationPage.message.failed',
          defaultMessage: '操作失败',
        }),
      );
    } finally {
      setLoading(false);
      setPendingValues(null);
    }
  }, [message, mainBinding, pendingValues, onExecute, preview]);

  // 重置
  const handleReset = useCallback(() => {
    setResult(null);
    setApprovalStatus(null);
    setError(null);
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

  const renderApprovedContinuation = () => {
    if (!approvalStatus || approvalStatus.status !== 'approved' || !approvalStatus.continuation) {
      return null;
    }
    if (approvalStatus.resultKind === 'task' && approvalStatus.taskId) {
      return (
        <Alert
          type="info"
          showIcon
          message={intl.formatMessage({
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
            <Descriptions
              bordered
              column={1}
              size="small"
              style={{ marginTop: 12 }}
              items={Object.entries(pendingValues).map(([key, value]) => ({
                key,
                label: <Typography.Text strong>{key}</Typography.Text>,
                children: renderJSONValueSummary(value),
              }))}
            />
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
          {error ? (
            <Result
              status="error"
              title={intl.formatMessage({
                id: 'component.pageRenderer.operationPage.message.failed',
                defaultMessage: '操作失败',
              })}
              subTitle={error}
              icon={<CloseCircleOutlined />}
            />
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
                    message={intl.formatMessage({
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
                        message={intl.formatMessage(
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
              extra={<Typography.Text code>{result.taskId || result.requestId}</Typography.Text>}
            />
          ) : (
            <Result
              status="success"
              title={intl.formatMessage({
                id: 'component.pageRenderer.operationPage.message.success',
                defaultMessage: '操作成功',
              })}
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
