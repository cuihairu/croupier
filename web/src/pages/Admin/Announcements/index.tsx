import React, { useCallback, useEffect, useRef, useState } from 'react';
import {
  App,
  Button,
  Card,
  DatePicker,
  Form,
  Input,
  Modal,
  Popconfirm,
  Select,
  Space,
  Switch,
  Table,
  Tag,
  Typography,
} from 'antd';
import type { Dayjs } from 'dayjs';
import { PageContainer } from '@ant-design/pro-components';
import { PlusOutlined } from '@ant-design/icons';
import { useIntl } from '@umijs/max';
import {
  createAnnouncement,
  deleteAnnouncement,
  listAnnouncements,
  updateAnnouncement,
  type AdminAnnouncement,
} from '@/services/api/announcements';
import { extractErrorMessage } from '@/utils/errors';
import { formatDateTime } from '@/utils/format';

const { Text } = Typography;

type FormValues = {
  title: string;
  contentMd: string;
  audience: 'all' | 'role';
  role?: string;
  popup: boolean;
  active: boolean;
  range?: [Dayjs, Dayjs];
};

/**
 * 系统公告（广播）管理页 —— admin-only。
 *
 * 此前「发送消息 / 广播」入口挂在**个人中心**的消息通知页顶部（`NotificationsTab`
 * 的 primary 按钮 + `BroadcastModal`），而个人中心是所有用户都有的页面，语义
 * 不对（docs/BUGS.md BUG-021）。后端 `/api/v1/admin/announcements` 这套接口一直
 * 存在却**没有任何前端入口**，等于公告功能此前不可用；本页就是它的落地面。
 *
 * 权限：路由 `access: 'canAdmin'`，与后端 admin 路由组的鉴权口径一致。
 */
export default function AnnouncementsPage() {
  const intl = useIntl();
  const { message } = App.useApp();
  // 测试环境的 intl mock 每次渲染返回新引用；若 t 依赖它，load 也会每次变，
  // 于是 useEffect([load]) 反复触发 → 无限重拉。经 ref 转发让 t 引用稳定。
  const intlRef = useRef(intl);
  intlRef.current = intl;
  const t = useCallback(
    (id: string, fallback: string) => intlRef.current.formatMessage({ id, defaultMessage: fallback }),
    [],
  );

  const [rows, setRows] = useState<AdminAnnouncement[]>([]);
  const [loading, setLoading] = useState(false);
  const [editing, setEditing] = useState<AdminAnnouncement | null>(null);
  const [open, setOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [form] = Form.useForm<FormValues>();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const r = await listAnnouncements();
      setRows(r.items);
    } catch (error) {
      message.error(
        extractErrorMessage(error, t('pages.announcements.error.load', '加载公告失败')),
      );
    } finally {
      setLoading(false);
    }
  }, [message, t]);

  useEffect(() => {
    void load();
  }, [load]);

  const openCreate = () => {
    setEditing(null);
    form.resetFields();
    form.setFieldsValue({ audience: 'all', popup: false, active: true });
    setOpen(true);
  };

  const openEdit = (row: AdminAnnouncement) => {
    setEditing(row);
    form.resetFields();
    form.setFieldsValue({
      title: row.title,
      contentMd: row.contentMd,
      audience: row.audience === 'role' ? 'role' : 'all',
      role: row.role,
      popup: row.popup,
      active: row.active,
    });
    setOpen(true);
  };

  const submit = async () => {
    // validateFields 失败时会 reject；不接住就是 unhandled rejection
    //（表单错误提示由 antd 自己渲染，这里只需安静返回）。
    let values: FormValues;
    try {
      values = await form.validateFields();
    } catch {
      return;
    }
    const draft = {
      title: values.title,
      contentMd: values.contentMd,
      audience: values.audience,
      // audience=all 时清掉 role，否则后端可能残留上一次的 role 过滤
      role: values.audience === 'role' ? values.role : undefined,
      popup: values.popup,
      active: values.active,
      startAt: values.range?.[0]?.toISOString(),
      endAt: values.range?.[1]?.toISOString(),
    };
    setSaving(true);
    try {
      if (editing) {
        await updateAnnouncement(editing.id, draft);
        message.success(t('pages.announcements.update.success', '公告已更新'));
      } else {
        await createAnnouncement(draft);
        message.success(t('pages.announcements.create.success', '公告已发布'));
      }
      setOpen(false);
      await load();
    } catch (error) {
      message.error(
        extractErrorMessage(error, t('pages.announcements.error.save', '保存公告失败')),
      );
    } finally {
      setSaving(false);
    }
  };

  const remove = async (row: AdminAnnouncement) => {
    try {
      await deleteAnnouncement(row.id);
      message.success(t('pages.announcements.delete.success', '公告已删除'));
      await load();
    } catch (error) {
      message.error(
        extractErrorMessage(error, t('pages.announcements.error.delete', '删除公告失败')),
      );
    }
  };

  return (
    <PageContainer
      title={t('pages.announcements.title', '系统公告 / 广播')}
      subTitle={t(
        'pages.announcements.subtitle',
        '面向全体用户或指定角色的公告。公告走用户侧 /announcements/active，弹窗型公告在用户登录后提示确认。',
      )}
      extra={[
        <Button
          key="create"
          type="primary"
          icon={<PlusOutlined />}
          onClick={openCreate}
          data-testid="announcement-create"
        >
          {t('pages.announcements.create', '发布公告')}
        </Button>,
      ]}
    >
      <Card>
        <Table<AdminAnnouncement>
          rowKey="id"
          loading={loading}
          dataSource={rows}
          pagination={{ pageSize: 20, hideOnSinglePage: true }}
          locale={{
            emptyText: t('pages.announcements.empty', '还没有公告'),
          }}
          columns={[
            {
              title: t('pages.announcements.column.title', '标题'),
              dataIndex: 'title',
              render: (v: string, row) => (
                <Space orientation="vertical" size={0}>
                  <Text strong>{v}</Text>
                  <Text type="secondary" style={{ fontSize: 12 }}>
                    {row.contentMd?.slice(0, 80)}
                  </Text>
                </Space>
              ),
            },
            {
              title: t('pages.announcements.column.audience', '受众'),
              dataIndex: 'audience',
              width: 160,
              render: (v: string, row) =>
                v === 'role' ? (
                  <Tag color="purple">
                    {t('pages.announcements.audience.role', '角色')}：{row.role || '-'}
                  </Tag>
                ) : (
                  <Tag color="blue">{t('pages.announcements.audience.all', '全体用户')}</Tag>
                ),
            },
            {
              title: t('pages.announcements.column.flags', '标记'),
              width: 200,
              render: (_, row) => (
                <Space wrap size={4}>
                  <Tag color={row.active ? 'success' : 'default'}>
                    {row.active
                      ? t('pages.announcements.flag.active', '生效中')
                      : t('pages.announcements.flag.inactive', '已停用')}
                  </Tag>
                  {row.popup && (
                    <Tag color="orange">{t('pages.announcements.flag.popup', '弹窗提示')}</Tag>
                  )}
                </Space>
              ),
            },
            {
              title: t('pages.announcements.column.updatedAt', '更新时间'),
              dataIndex: 'updatedAt',
              width: 180,
              render: (v?: string) => (v ? formatDateTime(v) : '-'),
            },
            {
              title: t('pages.announcements.column.actions', '操作'),
              width: 160,
              render: (_, row) => (
                <Space>
                  <Button
                    type="link"
                    size="small"
                    onClick={() => openEdit(row)}
                    data-testid={`announcement-edit-${row.id}`}
                  >
                    {t('pages.announcements.action.edit', '编辑')}
                  </Button>
                  <Popconfirm
                    title={t('pages.announcements.delete.confirm', '确认删除该公告？')}
                    okText={t('pages.announcements.delete.ok', '确认')}
                    cancelText={t('pages.announcements.delete.cancel', '取消')}
                    okButtonProps={{ 'data-testid': `announcement-delete-confirm-${row.id}` } as never}
                    onConfirm={() => void remove(row)}
                  >
                    <Button
                      type="link"
                      size="small"
                      danger
                      data-testid={`announcement-delete-${row.id}`}
                    >
                      {t('pages.announcements.action.delete', '删除')}
                    </Button>
                  </Popconfirm>
                </Space>
              ),
            },
          ]}
        />
      </Card>

      <Modal
        open={open}
        title={
          editing
            ? t('pages.announcements.modal.edit', '编辑公告')
            : t('pages.announcements.modal.create', '发布公告')
        }
        onCancel={() => setOpen(false)}
        onOk={() => void submit()}
        confirmLoading={saving}
        okText={t('pages.announcements.modal.ok', '保存')}
        cancelText={t('pages.announcements.modal.cancel', '取消')}
        // 用 testid 而非文案定位：antd 6 的按钮文本可能被拆进子节点
        okButtonProps={{ 'data-testid': 'announcement-save' } as never}
        cancelButtonProps={{ 'data-testid': 'announcement-cancel' } as never}
        destroyOnHidden
        width={640}
      >
        <Form form={form} layout="vertical" preserve={false}>
          <Form.Item
            name="title"
            label={t('pages.announcements.field.title', '标题')}
            rules={[{ required: true, message: t('pages.announcements.field.title.required', '请填写标题') }]}
          >
            <Input
              placeholder={t('pages.announcements.field.title.placeholder', '一句话说清公告主题')}
              data-testid="announcement-title"
            />
          </Form.Item>
          <Form.Item
            name="contentMd"
            label={t('pages.announcements.field.content', '正文（Markdown）')}
            rules={[
              { required: true, message: t('pages.announcements.field.content.required', '请填写正文') },
            ]}
          >
            <Input.TextArea rows={6} data-testid="announcement-content" />
          </Form.Item>
          <Space size="large" wrap align="start">
            <Form.Item
              name="audience"
              label={t('pages.announcements.field.audience', '受众')}
              rules={[{ required: true }]}
            >
              <Select
                style={{ width: 160 }}
                data-testid="announcement-audience"
                options={[
                  { value: 'all', label: t('pages.announcements.audience.all', '全体用户') },
                  { value: 'role', label: t('pages.announcements.audience.role', '指定角色') },
                ]}
              />
            </Form.Item>
            {/* audience=all 时隐藏：留着会让管理员以为 role 仍生效 */}
            <Form.Item
              noStyle
              shouldUpdate={(prev, next) => prev.audience !== next.audience}
            >
              {({ getFieldValue }) =>
                getFieldValue('audience') === 'role' ? (
                  <Form.Item
                    name="role"
                    label={t('pages.announcements.field.role', '角色名')}
                    rules={[
                      {
                        required: true,
                        message: t('pages.announcements.field.role.required', '请填写角色名'),
                      },
                    ]}
                  >
                    <Input
                      style={{ width: 160 }}
                      placeholder="admin"
                      data-testid="announcement-role"
                    />
                  </Form.Item>
                ) : null
              }
            </Form.Item>
            <Form.Item
              name="popup"
              label={t('pages.announcements.field.popup', '登录后弹窗提示')}
              valuePropName="checked"
            >
              <Switch data-testid="announcement-popup" />
            </Form.Item>
            <Form.Item
              name="active"
              label={t('pages.announcements.field.active', '立即生效')}
              valuePropName="checked"
            >
              <Switch data-testid="announcement-active" />
            </Form.Item>
          </Space>
          <Form.Item name="range" label={t('pages.announcements.field.range', '生效时间区间（可选）')}>
            <DatePicker.RangePicker showTime />
          </Form.Item>
        </Form>
      </Modal>
    </PageContainer>
  );
}
