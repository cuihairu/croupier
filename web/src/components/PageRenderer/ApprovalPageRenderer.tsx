/**
 * ApprovalPageRenderer - 审批页面渲染器
 *
 * 渲染审批流程页面，包括：
 * - 审批状态展示（pending/approved/rejected/expired）
 * - 审批表单
 * - 审批操作（批准/拒绝）
 * - 审批历史
 *
 * @module components/PageRenderer/ApprovalPageRenderer
 */

import React, { useState, useCallback, useEffect } from 'react';
import {
  Card,
  Button,
  Space,
  Modal,
  message,
  Result,
  Typography,
  Steps,
  Tag,
  Timeline,
  Descriptions,
  Alert,
  Spin,
  Divider,
  Input,
} from 'antd';
import {
  CheckCircleOutlined,
  CloseCircleOutlined,
  ClockCircleOutlined,
  ExclamationCircleOutlined,
  HourglassOutlined,
  UserOutlined,
  FileTextOutlined,
} from '@ant-design/icons';
import SchemaFormRenderer from '@/components/SchemaFormRenderer';
import type {
  OperationPageSpec,
  PageFunctionBinding,
  PageExecuteFn,
  FormValues,
} from '@/types/dashboard';

const { Text, Title, Paragraph } = Typography;

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

/** 审批状态 */
type ApprovalStatus = 'pending' | 'approved' | 'rejected' | 'expired' | 'cancelled';

/** 审批记录 */
interface ApprovalRecord {
  id: string;
  status: ApprovalStatus;
  approver?: string;
  comment?: string;
  timestamp: string;
}

/** 审批详情 */
interface ApprovalDetail {
  id: string;
  status: ApprovalStatus;
  title: string;
  description?: string;
  submitter: string;
  submitTime: string;
  approver?: string;
  approveTime?: string;
  rejectReason?: string;
  expireTime?: string;
  records: ApprovalRecord[];
}

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

export interface ApprovalPageRendererProps {
  /** 操作页面规格 */
  spec: OperationPageSpec;
  /** 页面绑定 */
  bindings: PageFunctionBinding[];
  /** 执行绑定函数 */
  onExecute: PageExecuteFn;
  /** 查询审批状态 */
  onQueryStatus?: (approvalId: string) => Promise<ApprovalDetail>;
  /** 审批操作 */
  onApprove?: (approvalId: string, comment?: string) => Promise<void>;
  /** 拒绝操作 */
  onReject?: (approvalId: string, reason: string) => Promise<void>;
  /** 页面标题 */
  title?: string;
}

// ---------------------------------------------------------------------------
// 状态配置
// ---------------------------------------------------------------------------

const STATUS_CONFIG: Record<ApprovalStatus, {
  color: string;
  icon: React.ReactNode;
  label: string;
  description: string;
}> = {
  pending: {
    color: 'processing',
    icon: <HourglassOutlined />,
    label: '待审批',
    description: '等待审批人处理',
  },
  approved: {
    color: 'success',
    icon: <CheckCircleOutlined />,
    label: '已批准',
    description: '审批已通过',
  },
  rejected: {
    color: 'error',
    icon: <CloseCircleOutlined />,
    label: '已拒绝',
    description: '审批未通过',
  },
  expired: {
    color: 'warning',
    icon: <ClockCircleOutlined />,
    label: '已过期',
    description: '审批已超时',
  },
  cancelled: {
    color: 'default',
    icon: <ExclamationCircleOutlined />,
    label: '已取消',
    description: '审批已取消',
  },
};

// ---------------------------------------------------------------------------
// ApprovalPageRenderer 组件
// ---------------------------------------------------------------------------

const ApprovalPageRenderer: React.FC<ApprovalPageRendererProps> = ({
  spec,
  bindings,
  onExecute,
  onQueryStatus,
  onApprove,
  onReject,
  title,
}) => {
  const [approvalDetail, setApprovalDetail] = useState<ApprovalDetail | null>(null);
  const [loading, setLoading] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [showRejectModal, setShowRejectModal] = useState(false);
  const [rejectReason, setRejectReason] = useState('');

  // 查询审批状态
  const fetchApprovalStatus = useCallback(async () => {
    if (!onQueryStatus) return;
    setLoading(true);
    try {
      // TODO: 从页面状态获取 approvalId
      const approvalId = 'current';
      const detail = await onQueryStatus(approvalId);
      setApprovalDetail(detail);
    } catch (error) {
      message.error('查询审批状态失败');
    } finally {
      setLoading(false);
    }
  }, [onQueryStatus]);

  useEffect(() => {
    fetchApprovalStatus();
  }, [fetchApprovalStatus]);

  // 提交审批申请
  const handleSubmit = useCallback(async (values: FormValues) => {
    const mainBinding = bindings.find(b => b.usage === 'action') || bindings[0];
    if (!mainBinding) {
      message.error('未找到执行绑定');
      return;
    }

    setSubmitting(true);
    try {
      await onExecute(mainBinding.id, values);
      message.success('审批申请已提交');
      await fetchApprovalStatus();
    } catch (error) {
      message.error('提交失败');
    } finally {
      setSubmitting(false);
    }
  }, [bindings, onExecute, fetchApprovalStatus]);

  // 批准操作
  const handleApprove = useCallback(async (comment?: string) => {
    if (!onApprove || !approvalDetail) return;

    setSubmitting(true);
    try {
      await onApprove(approvalDetail.id, comment);
      message.success('已批准');
      await fetchApprovalStatus();
    } catch (error) {
      message.error('批准操作失败');
    } finally {
      setSubmitting(false);
    }
  }, [onApprove, approvalDetail, fetchApprovalStatus]);

  // 拒绝操作
  const handleReject = useCallback(async () => {
    if (!onReject || !approvalDetail) return;

    if (!rejectReason.trim()) {
      message.warning('请填写拒绝原因');
      return;
    }

    setSubmitting(true);
    try {
      await onReject(approvalDetail.id, rejectReason);
      message.success('已拒绝');
      setShowRejectModal(false);
      setRejectReason('');
      await fetchApprovalStatus();
    } catch (error) {
      message.error('拒绝操作失败');
    } finally {
      setSubmitting(false);
    }
  }, [onReject, approvalDetail, rejectReason, fetchApprovalStatus]);

  // 渲染状态卡片
  const renderStatusCard = () => {
    if (!approvalDetail) return null;

    const config = STATUS_CONFIG[approvalDetail.status];
    const currentStep = approvalDetail.status === 'pending' ? 1
      : approvalDetail.status === 'approved' ? 2
      : approvalDetail.status === 'rejected' ? 2
      : 0;

    return (
      <Card style={{ marginBottom: 16 }}>
        <Space direction="vertical" style={{ width: '100%' }}>
          <Space>
            <Tag icon={config.icon} color={config.color} style={{ fontSize: 14, padding: '4px 12px' }}>
              {config.label}
            </Tag>
            <Text type="secondary">{config.description}</Text>
          </Space>

          <Steps
            current={currentStep}
            size="small"
            items={[
              {
                title: '提交申请',
                description: approvalDetail.submitter,
                icon: <UserOutlined />,
              },
              {
                title: '审批处理',
                description: approvalDetail.approver || '待处理',
                icon: <FileTextOutlined />,
              },
              {
                title: '完成',
                description: approvalDetail.status === 'approved' ? '已通过'
                  : approvalDetail.status === 'rejected' ? '已拒绝'
                  : '待完成',
                icon: approvalDetail.status === 'approved'
                  ? <CheckCircleOutlined />
                  : approvalDetail.status === 'rejected'
                  ? <CloseCircleOutlined />
                  : <ClockCircleOutlined />,
              },
            ]}
          />

          {approvalDetail.status === 'rejected' && approvalDetail.rejectReason && (
            <Alert
              type="error"
              showIcon
              message="拒绝原因"
              description={approvalDetail.rejectReason}
            />
          )}

          {approvalDetail.status === 'expired' && (
            <Alert
              type="warning"
              showIcon
              message="审批已过期"
              description="审批已超过有效期，请重新提交"
            />
          )}
        </Space>
      </Card>
    );
  };

  // 渲染审批表单
  const renderApprovalForm = () => {
    // 如果已有审批详情且状态不是 pending，显示结果
    if (approvalDetail && approvalDetail.status !== 'pending') {
      return renderResultView();
    }

    // 如果有待审批状态，显示审批操作
    if (approvalDetail && approvalDetail.status === 'pending') {
      return renderApprovalActions();
    }

    // 否则显示提交表单
    return renderSubmitForm();
  };

  // 渲染提交表单
  const renderSubmitForm = () => {
    const { form: formSpec } = spec;
    if (!formSpec) {
      return (
        <Result
          status="warning"
          title="配置错误"
          subTitle="审批页面缺少表单配置"
        />
      );
    }

    return (
      <Card title={title || '提交审批申请'}>
        <SchemaFormRenderer
          spec={formSpec}
          onFinish={handleSubmit}
          disabled={submitting}
        />
      </Card>
    );
  };

  // 渲染审批操作
  const renderApprovalActions = () => {
    return (
      <Card title="审批处理">
        <Space direction="vertical" style={{ width: '100%' }}>
          {approvalDetail?.description && (
            <Paragraph>{approvalDetail.description}</Paragraph>
          )}

          <Divider />

          <Space>
            <Button
              type="primary"
              icon={<CheckCircleOutlined />}
              loading={submitting}
              onClick={() => handleApprove()}
            >
              批准
            </Button>
            <Button
              danger
              icon={<CloseCircleOutlined />}
              loading={submitting}
              onClick={() => setShowRejectModal(true)}
            >
              拒绝
            </Button>
            <Button
              onClick={fetchApprovalStatus}
              loading={loading}
            >
              刷新状态
            </Button>
          </Space>
        </Space>
      </Card>
    );
  };

  // 渲染结果视图
  const renderResultView = () => {
    if (!approvalDetail) return null;

    const config = STATUS_CONFIG[approvalDetail.status];

    return (
      <Card title="审批结果">
        <Result
          status={approvalDetail.status === 'approved' ? 'success'
            : approvalDetail.status === 'rejected' ? 'error'
            : 'warning'}
          title={`审批${config.label}`}
          subTitle={approvalDetail.status === 'approved'
            ? '审批已通过，操作将被执行'
            : approvalDetail.status === 'rejected'
            ? `审批未通过：${approvalDetail.rejectReason || '无原因说明'}`
            : '审批状态异常'}
          extra={[
            <Button
              key="refresh"
              onClick={fetchApprovalStatus}
              loading={loading}
            >
              刷新状态
            </Button>,
          ]}
        />
      </Card>
    );
  };

  // 渲染审批历史
  const renderApprovalHistory = () => {
    if (!approvalDetail?.records?.length) return null;

    return (
      <Card title="审批历史" style={{ marginTop: 16 }}>
        <Timeline
          items={approvalDetail.records.map(record => {
            const config = STATUS_CONFIG[record.status];
            return {
              color: config.color === 'processing' ? 'blue'
                : config.color === 'success' ? 'green'
                : config.color === 'error' ? 'red'
                : config.color === 'warning' ? 'orange'
                : 'gray',
              children: (
                <Space direction="vertical" size={4}>
                  <Space>
                    <Tag color={config.color}>{config.label}</Tag>
                    {record.approver && <Text>{record.approver}</Text>}
                    <Text type="secondary">{record.timestamp}</Text>
                  </Space>
                  {record.comment && <Text type="secondary">{record.comment}</Text>}
                </Space>
              ),
            };
          })}
        />
      </Card>
    );
  };

  // 加载中状态
  if (loading && !approvalDetail) {
    return (
      <Card>
        <div style={{ textAlign: 'center', padding: '50px 0' }}>
          <Spin size="large" tip="加载审批状态..." />
        </div>
      </Card>
    );
  }

  return (
    <div>
      {renderStatusCard()}
      {renderApprovalForm()}
      {renderApprovalHistory()}

      {/* 拒绝原因弹窗 */}
      <Modal
        title="拒绝审批"
        open={showRejectModal}
        onOk={handleReject}
        onCancel={() => {
          setShowRejectModal(false);
          setRejectReason('');
        }}
        confirmLoading={submitting}
        okText="确认拒绝"
        cancelText="取消"
      >
        <Space direction="vertical" style={{ width: '100%' }}>
          <Text>请填写拒绝原因：</Text>
          <Input.TextArea
            value={rejectReason}
            onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) => setRejectReason(e.target?.value || '')}
            placeholder="请输入拒绝原因"
            rows={4}
          />
        </Space>
      </Modal>
    </div>
  );
};

export default ApprovalPageRenderer;
