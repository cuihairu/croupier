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

import React, { useState, useCallback } from 'react';
import {
  Card,
  Button,
  Modal,
  message,
  Result,
  Alert,
  Space,
  Typography,
} from 'antd';
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
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<PageExecutionResult | null>(null);
  const [approvalStatus, setApprovalStatus] = useState<ApprovalStatusResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [confirmVisible, setConfirmVisible] = useState(false);
  const [pendingValues, setPendingValues] = useState<FormValues | null>(null);

  // 查找主绑定
  const mainBinding = bindings.find((b) => b.usage === 'action') || bindings[0];

  // 处理表单提交
  const handleSubmit = useCallback(
    async (values: FormValues) => {
      if (!mainBinding) {
        message.error('未配置操作绑定');
        return;
      }
      if (preview) {
        message.info('预览模式不执行操作');
        return;
      }

      // 如果需要确认
      if (spec.confirm) {
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
          message.info('操作已提交审批');
        } else if (response.kind === 'task') {
          message.success('任务已提交');
        } else if (spec.resultView?.successMessage) {
          message.success(
            spec.resultView.successMessage['zh-CN'] || '操作成功'
          );
        } else {
          message.success('操作成功');
        }
      } catch (err) {
        const msg = err instanceof Error ? err.message : '操作失败';
        setError(msg);

        if (spec.resultView?.errorMessage) {
          message.error(
            spec.resultView.errorMessage['zh-CN'] || '操作失败'
          );
        } else {
          message.error('操作失败');
        }
      } finally {
        setLoading(false);
      }
    },
    [mainBinding, spec.confirm, spec.resultView, onExecute, preview]
  );

  // 处理确认后执行
  const handleConfirm = useCallback(async () => {
    if (!mainBinding || !pendingValues) {
      return;
    }
    if (preview) {
      message.info('预览模式不执行操作');
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
        message.info('操作已提交审批');
      } else if (response.kind === 'task') {
        message.success('任务已提交');
      } else {
        message.success('操作成功');
      }
    } catch (err) {
      const msg = err instanceof Error ? err.message : '操作失败';
      setError(msg);
      message.error('操作失败');
    } finally {
      setLoading(false);
      setPendingValues(null);
    }
  }, [mainBinding, pendingValues, onExecute, preview]);

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
      const msg = err instanceof Error ? err.message : '审批状态查询失败';
      message.error(msg);
    }
  }, [onQueryApprovalStatus, result?.approvalId]);

  const renderApprovedContinuation = () => {
    if (!approvalStatus || approvalStatus.status !== 'approved' || !approvalStatus.continuation) {
      return null;
    }
    if (approvalStatus.resultKind === 'task' && approvalStatus.taskId) {
      return (
        <Alert
          type="info"
          showIcon
          message="审批已通过，任务已启动"
          description={<Typography.Text code>{approvalStatus.taskId}</Typography.Text>}
        />
      );
    }
    if (approvalStatus.resultKind === 'sync') {
      return (
        <ResultViewRenderer
          data={approvalStatus.result}
          resultView={spec.resultView}
          emptyTitle="审批后执行结果视图未配置"
        />
      );
    }
    return null;
  };

  return (
    <div>
      {/* 表单 */}
      <Card title={title || '执行操作'}>
        <SchemaFormRenderer
          spec={spec.form}
          onFinish={handleSubmit}
          disabled={loading || preview}
        />
        <Button style={{ marginTop: 12 }} onClick={handleReset}>
          重置结果
        </Button>
      </Card>

      {/* 确认对话框 */}
      {spec.confirm && (
        <Modal
          title={spec.confirm.title['zh-CN'] || '确认操作'}
          open={confirmVisible}
          onOk={handleConfirm}
          onCancel={() => {
            setConfirmVisible(false);
            setPendingValues(null);
          }}
          okText={spec.confirm.confirmText['zh-CN'] || '确定'}
          cancelText={spec.confirm.cancelText?.['zh-CN'] || '取消'}
          confirmLoading={loading}
        >
          {spec.confirm.description && (
            <p>{spec.confirm.description['zh-CN']}</p>
          )}
          {pendingValues && (
            <Space direction="vertical" style={{ width: '100%' }}>
              {Object.entries(pendingValues).map(([key, value]) => (
                <Space key={key} align="start">
                  <Typography.Text strong>{key}</Typography.Text>
                  {renderJSONValueSummary(value)}
                </Space>
              ))}
            </Space>
          )}
        </Modal>
      )}

      {/* 结果展示 */}
      {(result || error) && (
        <Card title="执行结果" style={{ marginTop: 16 }}>
          {error ? (
            <Result
              status="error"
              title="操作失败"
              subTitle={error}
              icon={<CloseCircleOutlined />}
            />
          ) : result?.kind === 'approval' ? (
            <Result
              status="info"
              title="等待审批"
              subTitle="审批通过后才会继续执行，请在审批中心查看状态。"
              icon={<ClockCircleOutlined />}
              extra={
                <Space direction="vertical">
                  <Alert
                    type="info"
                    showIcon
                    message="操作尚未完成"
                    description="当前返回的是 approvalId，不代表业务执行成功。"
                  />
                  <Typography.Text code>{result.approvalId || result.requestId}</Typography.Text>
                  {approvalStatus ? (
                    <>
                      <Alert
                        type={approvalStatus.status === 'rejected' ? 'error' : approvalStatus.status === 'approved' ? 'success' : 'info'}
                        showIcon
                        message={`审批状态：${approvalStatus.status}`}
                        description={approvalStatus.reason || approvalStatus.updatedAt || undefined}
                      />
                      {renderApprovedContinuation()}
                    </>
                  ) : null}
                  {result.approvalId && onQueryApprovalStatus ? (
                    <Button onClick={refreshApproval}>刷新审批状态</Button>
                  ) : null}
                </Space>
              }
            />
          ) : result?.kind === 'task' ? (
            <Result
              status="info"
              title="任务已提交"
              subTitle="异步任务仍在执行，请在任务中心或任务页面查看进度。"
              icon={<SyncOutlined spin />}
              extra={<Typography.Text code>{result.taskId || result.requestId}</Typography.Text>}
            />
          ) : (
            <Result
              status="success"
              title="操作成功"
              icon={<CheckCircleOutlined />}
              extra={
                <ResultViewRenderer
                  data={result?.data}
                  resultView={spec.resultView}
                  emptyTitle="操作结果视图未配置"
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
