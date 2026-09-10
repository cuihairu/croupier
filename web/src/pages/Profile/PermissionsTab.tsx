import { useCallback, useRef, useState } from 'react';
import { Alert, Button, Card, Form, Input, List, Space, Tag, Typography, message } from 'antd';
import type { FormInstance } from 'antd';
import { ModalForm } from '@ant-design/pro-components';
import { CopyOutlined } from '@ant-design/icons';
import { useIntl, useNavigate } from '@umijs/max';
import { createFeedback } from '@/services/api/support';
import type { PermissionApplyItem } from './shared';

const { Text } = Typography;

type ApplyFormValues = { reason: string };

/** 权限 Tab：已有权限汇总 + 可申请权限列表 + 申请弹窗（复制申请文案/提交反馈工单）。 */
export default function PermissionsTab({
  groups,
  candidates,
  catalogAvailable,
  username,
}: {
  groups: { resource: string; actions: string[]; scope?: string }[];
  candidates: PermissionApplyItem[];
  catalogAvailable: boolean;
  username?: string;
}) {
  const intl = useIntl();
  const navigate = useNavigate();
  const formatMessage = useCallback((id: string) => intl.formatMessage({ id }), [intl]);
  // 「复制申请文案」按钮在弹窗外定义、只做校验+取值不提交；formRef 指向当前
  // 挂载的表单实例（destroyOnHidden 下每次打开都是新实例）
  const formRef = useRef<FormInstance<ApplyFormValues> | undefined>(undefined);
  const [modalVisible, setModalVisible] = useState(false);
  const [selected, setSelected] = useState<PermissionApplyItem | null>(null);

  const handleOpenApply = (item: PermissionApplyItem) => {
    setSelected(item);
    setModalVisible(true);
  };

  const buildApplyContent = (reason: string) => {
    if (!selected) return '';
    return [
      `${formatMessage('profile.permissions.apply.content.applicant')}: ${username || '-'}`,
      `${formatMessage('profile.permissions.apply.content.permission')}: ${selected.name}`,
      `${formatMessage('profile.permissions.apply.content.permission.key')}: ${selected.resource}:${selected.action}`,
      `${formatMessage('profile.permissions.apply.content.permission.id')}: ${selected.id}`,
      `${formatMessage('profile.permissions.apply.content.reason')}: ${reason || '-'}`,
    ].join('\n');
  };

  const handleCopyApplyContent = async () => {
    const form = formRef.current;
    if (!form) return;
    const values = await form.validateFields();
    const content = buildApplyContent(String(values.reason ?? ''));
    await navigator.clipboard.writeText(content);
    message.success(formatMessage('profile.permissions.apply.copy.success'));
  };

  const handleFinishApply = async (values: ApplyFormValues) => {
    const content = buildApplyContent(String(values.reason ?? ''));
    try {
      await createFeedback({
        category: 'permission_request',
        content,
        priority: 'normal',
        source: 'profile_permission_apply',
      });
      message.success(formatMessage('profile.permissions.apply.submit.success'));
      return true;
    } catch {
      // 部分环境可能未开放反馈写入，降级为复制文案+人工提交流程
      await navigator.clipboard.writeText(content);
      message.warning(formatMessage('profile.permissions.apply.submit.fallback'));
      return false;
    }
  };

  return (
    <>
      <Space orientation="vertical" size={16} style={{ width: '100%' }}>
        <Card title={formatMessage('profile.permissions.summary.title')}>
          <List
            dataSource={groups}
            locale={{ emptyText: formatMessage('profile.permissions.empty') }}
            renderItem={(item) => (
              <List.Item>
                <div style={{ width: '100%' }}>
                  <Space>
                    <Text strong>{item.resource}</Text>
                    {item.scope && <Tag color="purple">{item.scope}</Tag>}
                  </Space>
                  <div style={{ marginTop: 8 }}>
                    <Space wrap>
                      {item.actions.map((action) => (
                        <Tag key={action} color="cyan">
                          {action}
                        </Tag>
                      ))}
                    </Space>
                  </div>
                </div>
              </List.Item>
            )}
          />
        </Card>
        <Card
          title={formatMessage('profile.permissions.apply.title')}
          extra={
            !catalogAvailable ? (
              <Tag color="gold">{formatMessage('profile.permissions.apply.catalog.fallback')}</Tag>
            ) : (
              <Tag color="green">{formatMessage('profile.permissions.apply.catalog.live')}</Tag>
            )
          }
        >
          <List
            dataSource={candidates}
            locale={{ emptyText: formatMessage('profile.permissions.apply.empty') }}
            renderItem={(item) => (
              <List.Item
                actions={[
                  <Button key="apply" type="link" onClick={() => handleOpenApply(item)}>
                    {formatMessage('profile.permissions.apply.action')}
                  </Button>,
                ]}
              >
                <List.Item.Meta
                  title={
                    <Space>
                      <Text strong>{item.name}</Text>
                      <Tag>
                        {item.resource}:{item.action}
                      </Tag>
                      {item.category && <Tag color="blue">{item.category}</Tag>}
                    </Space>
                  }
                  description={
                    item.description || formatMessage('profile.permissions.apply.no.description')
                  }
                />
              </List.Item>
            )}
          />
        </Card>
      </Space>
      <ModalForm<ApplyFormValues>
        open={modalVisible}
        onOpenChange={setModalVisible}
        formRef={formRef}
        title={formatMessage('profile.permissions.apply.modal.title')}
        modalProps={{ destroyOnHidden: true }}
        width={520}
        submitter={{
          searchConfig: { submitText: formatMessage('profile.permissions.apply.modal.submit') },
        }}
        onFinish={handleFinishApply}
      >
        <Space orientation="vertical" style={{ width: '100%' }} size={12}>
          {selected && (
            <Alert
              showIcon
              type="info"
              message={selected.name}
              description={`${selected.resource}:${selected.action}`}
            />
          )}
          <Form.Item
            name="reason"
            label={formatMessage('profile.permissions.apply.reason')}
            rules={[
              {
                required: true,
                message: formatMessage('profile.permissions.apply.reason.required'),
              },
            ]}
          >
            <Input.TextArea
              rows={4}
              placeholder={formatMessage('profile.permissions.apply.reason.placeholder')}
            />
          </Form.Item>
          <Space>
            <Button
              icon={<CopyOutlined />}
              onClick={() => handleCopyApplyContent().catch(() => {})}
            >
              {formatMessage('profile.permissions.apply.copy')}
            </Button>
            <Button
              onClick={() => {
                navigate('/support/feedback');
              }}
            >
              {formatMessage('profile.permissions.apply.goto.feedback')}
            </Button>
          </Space>
        </Space>
      </ModalForm>
    </>
  );
}
