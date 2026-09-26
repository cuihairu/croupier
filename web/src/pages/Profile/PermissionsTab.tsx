import SimpleList from '@/components/SimpleList';
import { useCallback, useRef, useState } from 'react';
import { App, Alert, Button, Card, Form, Input, Space, Tag, Typography } from 'antd';
import type { FormInstance } from 'antd';
import { ModalForm } from '@ant-design/pro-components';
import { CopyOutlined } from '@ant-design/icons';
import { useIntl, useNavigate } from '@umijs/max';
import { createFeedback } from '@/services/api/support';
import PermissionTreeView from './PermissionTreeView';
import type { PermissionCatalogEntry, RoleGrant } from './permissionTree';
import type { PermissionApplyItem } from './shared';

const { Text } = Typography;

type ApplyFormValues = { reason: string };

/**
 * 权限 Tab：权限树（角色 → 资源 → 操作）+ 可申请权限列表 + 申请弹窗。
 *
 * 原先是单层 `SimpleList` 平铺「已授权」，既没有资源/操作分层，也没有
 * 已授权/未授权的两态区分（docs/BUGS.md BUG-020）。现在树结构由
 * `PermissionTreeView` 渲染，汇总列表保留在下方作为紧凑视图。
 */
export default function PermissionsTab({
  groups,
  candidates,
  catalogAvailable,
  roleGrants,
  catalog,
  fullAccess,
  username,
}: {
  groups: { resource: string; actions: string[]; scope?: string }[];
  candidates: PermissionApplyItem[];
  catalogAvailable: boolean;
  /** 逐角色授权明细；缺失时树退化为「仅已授权」单层结构 */
  roleGrants?: RoleGrant[];
  /** 全量权限目录，用于渲染未授权项 */
  catalog?: PermissionCatalogEntry[];
  /** 账号持通配权限 */
  fullAccess?: boolean;
  username?: string;
}) {
  const { message } = App.useApp();
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
        <PermissionTreeView
          roles={roleGrants || []}
          catalog={catalog || []}
          fullAccess={fullAccess === true}
        />
        <Card title={formatMessage('profile.permissions.summary.title')}>
          <SimpleList
            dataSource={groups}
            locale={{ emptyText: formatMessage('profile.permissions.empty') }}
            renderItem={(item) => (
              <SimpleList.Item>
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
              </SimpleList.Item>
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
          <SimpleList
            dataSource={candidates}
            locale={{ emptyText: formatMessage('profile.permissions.apply.empty') }}
            renderItem={(item) => (
              <SimpleList.Item
                actions={[
                  <Button key="apply" type="link" onClick={() => handleOpenApply(item)}>
                    {formatMessage('profile.permissions.apply.action')}
                  </Button>,
                ]}
              >
                <SimpleList.Item.Meta
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
              </SimpleList.Item>
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
              title={selected.name}
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
