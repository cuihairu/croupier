import React, { useRef, useState } from 'react';
import {
  Button,
  Form,
  Input,
  InputNumber,
  Modal,
  Select,
  Space,
  Switch,
  Tag,
  Typography,
} from 'antd';
import {
  PageContainer,
  ProTable,
  type ActionType,
  type ProColumns,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { DeleteOutlined, EditOutlined, PlusOutlined } from '@ant-design/icons';
import {
  createIncidentCategory,
  deleteIncidentCategory,
  fetchIncidentCategories,
  fetchIncidentCategoryUsage,
  updateIncidentCategory,
  type CategoryUpsertPayload,
  type IncidentCategory,
} from '@/services/api/incident';

type FormValues = {
  name: string;
  slug?: string;
  sort?: number;
  leader?: string;
  subcategories?: string[];
  audienceRoles?: string[];
  audienceUsers?: string[];
  enabled: boolean;
};

const SLUG_RE = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;

export default function IncidentCategoriesPage() {
  const intl = useIntl();
  const actionRef = useRef<ActionType | undefined>(undefined);
  const [editorOpen, setEditorOpen] = useState(false);
  const [editing, setEditing] = useState<IncidentCategory | undefined>(undefined);
  const [saving, setSaving] = useState(false);

  const text = (id: string, defaultMessage: string) => intl.formatMessage({ id, defaultMessage });

  const openCreate = () => {
    setEditing(undefined);
    setEditorOpen(true);
  };

  const openEdit = (row: IncidentCategory) => {
    setEditing(row);
    setEditorOpen(true);
  };

  const handleDelete = (row: IncidentCategory) => {
    if (row.builtin) {
      Modal.warning({
        title: text('pages.incidentCategories.delete.builtinTitle', '内建类别不可删除'),
        content: text(
          'pages.incidentCategories.delete.builtinContent',
          '内建六类与未分类是体系基座，只能停用不能删除。',
        ),
      });
      return;
    }
    Modal.confirm({
      title: text('pages.incidentCategories.delete.title', '删除类别'),
      okButtonProps: { danger: true },
      okText: text('pages.incidentCategories.delete.ok', '删除并归并'),
      content: text('pages.incidentCategories.delete.loading', '正在统计该类别下的存量数据…'),
      onOk: async () => {
        const usage = await fetchIncidentCategoryUsage(row.id);
        return new Promise<void>((resolve, reject) => {
          Modal.confirm({
            title: text('pages.incidentCategories.delete.confirmTitle', '确认删除并归并？'),
            okButtonProps: { danger: true },
            content: text(
              'pages.incidentCategories.delete.confirmContent',
              `该类别下有 ${usage.incidents} 条事故、${usage.bugs} 个缺陷，删除后将归并到「未分类」。`,
            ),
            onOk: async () => {
              try {
                await deleteIncidentCategory(row.id);
                actionRef.current?.reload();
                resolve();
              } catch (e) {
                reject(e);
              }
            },
            onCancel: () => reject(),
          });
        });
      },
    });
  };

  const handleSubmit = async (values: FormValues) => {
    setSaving(true);
    try {
      const payload: CategoryUpsertPayload = {
        name: values.name,
        sort: values.sort,
        leader: values.leader,
        subcategories: values.subcategories,
        enabled: values.enabled,
        audience: {
          roles: values.audienceRoles || [],
          users: values.audienceUsers || [],
        },
      };
      if (editing) {
        await updateIncidentCategory(editing.id, payload);
      } else {
        await createIncidentCategory({ ...payload, slug: values.slug });
      }
      setEditorOpen(false);
      actionRef.current?.reload();
    } finally {
      setSaving(false);
    }
  };

  const columns: ProColumns<IncidentCategory>[] = [
    { title: text('pages.incidentCategories.col.id', 'ID'), dataIndex: 'id', width: 64 },
    {
      title: text('pages.incidentCategories.col.name', '名称'),
      dataIndex: 'name',
      render: (_, row) => (
        <Space size={4}>
          <Typography.Text strong>{row.name}</Typography.Text>
          {row.builtin && <Tag>{text('pages.incidentCategories.builtin', '内建')}</Tag>}
          {!row.enabled && (
            <Tag color="default">{text('pages.incidentCategories.disabled', '已停用')}</Tag>
          )}
        </Space>
      ),
    },
    { title: 'Slug', dataIndex: 'slug', width: 140 },
    { title: text('pages.incidentCategories.col.sort', '排序'), dataIndex: 'sort', width: 72 },
    {
      title: text('pages.incidentCategories.col.leader', '负责人'),
      dataIndex: 'leader',
      render: (v) => v || <Typography.Text type="secondary">-</Typography.Text>,
    },
    {
      title: text('pages.incidentCategories.col.subcategories', '子类白名单'),
      dataIndex: 'subcategories',
      render: (_, row) =>
        row.subcategories.length === 0 ? (
          <Typography.Text type="secondary">
            {text('pages.incidentCategories.subcategories.any', '不限')}
          </Typography.Text>
        ) : (
          row.subcategories.map((s) => <Tag key={s}>{s}</Tag>)
        ),
    },
    {
      title: text('pages.incidentCategories.col.audience', '可见范围'),
      dataIndex: 'audience',
      render: (_, row) => {
        const roles = row.audience?.roles || [];
        const users = row.audience?.users || [];
        if (roles.length === 0 && users.length === 0) {
          return (
            <Typography.Text type="secondary">
              {text('pages.incidentCategories.audience.all', '全员')}
            </Typography.Text>
          );
        }
        return (
          <Space size={4} wrap>
            {roles.map((r) => (
              <Tag key={`r-${r}`} color="blue">
                {r}
              </Tag>
            ))}
            {users.map((u) => (
              <Tag key={`u-${u}`}>{u}</Tag>
            ))}
          </Space>
        );
      },
    },
    {
      title: text('pages.incidentCategories.col.enabled', '启用'),
      dataIndex: 'enabled',
      width: 80,
      render: (_, row) => <Switch checked={row.enabled} disabled size="small" />,
    },
    {
      title: text('pages.incidentCategories.col.actions', '操作'),
      valueType: 'option',
      width: 120,
      render: (_, row) => [
        <a key="edit" onClick={() => openEdit(row)}>
          <EditOutlined /> {text('pages.incidentCategories.action.edit', '编辑')}
        </a>,
        <a key="del" onClick={() => handleDelete(row)}>
          <DeleteOutlined /> {text('pages.incidentCategories.action.delete', '删除')}
        </a>,
      ],
    },
  ];

  return (
    <PageContainer>
      <ProTable<IncidentCategory>
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        search={false}
        pagination={false}
        toolBarRender={() => [
          <Button key="new" type="primary" icon={<PlusOutlined />} onClick={openCreate}>
            {text('pages.incidentCategories.action.new', '新建类别')}
          </Button>,
        ]}
        request={async () => {
          const resp = await fetchIncidentCategories(false);
          return { data: resp.items || [], success: true, total: resp.total };
        }}
      />
      <CategoryEditor
        open={editorOpen}
        editing={editing}
        saving={saving}
        onCancel={() => setEditorOpen(false)}
        onSubmit={handleSubmit}
      />
    </PageContainer>
  );
}

function CategoryEditor({
  open,
  editing,
  saving,
  onCancel,
  onSubmit,
}: {
  open: boolean;
  editing?: IncidentCategory;
  saving: boolean;
  onCancel: () => void;
  onSubmit: (values: FormValues) => Promise<void>;
}) {
  const intl = useIntl();
  const text = (id: string, defaultMessage: string) =>
    intl.formatMessage({ id }, { defaultMessage });
  return (
    <Modal
      open={open}
      title={
        editing
          ? text('pages.incidentCategories.editor.editTitle', '编辑类别')
          : text('pages.incidentCategories.editor.createTitle', '新建类别')
      }
      onCancel={onCancel}
      footer={null}
      destroyOnClose
    >
      <EditorForm editing={editing} saving={saving} onCancel={onCancel} onSubmit={onSubmit} />
    </Modal>
  );
}

function EditorForm({
  editing,
  saving,
  onCancel,
  onSubmit,
}: {
  editing?: IncidentCategory;
  saving: boolean;
  onCancel: () => void;
  onSubmit: (values: FormValues) => Promise<void>;
}) {
  const intl = useIntl();
  const text = (id: string, defaultMessage: string) => intl.formatMessage({ id, defaultMessage });
  const [form] = Form.useForm<FormValues>();
  return (
    <Form
      form={form}
      layout="vertical"
      preserve={false}
      initialValues={
        editing
          ? {
              name: editing.name,
              sort: editing.sort,
              leader: editing.leader,
              subcategories: editing.subcategories,
              audienceRoles: editing.audience?.roles || [],
              audienceUsers: editing.audience?.users || [],
              enabled: editing.enabled,
            }
          : { enabled: true, sort: 100 }
      }
      onFinish={onSubmit}
    >
      <Form.Item
        name="name"
        label={text('pages.incidentCategories.editor.name', '名称')}
        rules={[
          {
            required: true,
            message: text('pages.incidentCategories.editor.nameRequired', '请输入名称'),
          },
        ]}
      >
        <Input />
      </Form.Item>
      {!editing && (
        <Form.Item
          name="slug"
          label="Slug"
          rules={[
            {
              required: true,
              message: text('pages.incidentCategories.editor.slugRequired', '请输入 slug'),
            },
            {
              pattern: SLUG_RE,
              message: text('pages.incidentCategories.editor.slugPattern', '小写字母/数字/中划线'),
            },
          ]}
          extra={text('pages.incidentCategories.editor.slugExtra', '创建后不可修改')}
        >
          <Input />
        </Form.Item>
      )}
      <Space size="middle" style={{ display: 'flex' }}>
        <Form.Item name="sort" label={text('pages.incidentCategories.editor.sort', '排序')}>
          <InputNumber min={0} style={{ width: 120 }} />
        </Form.Item>
        <Form.Item
          name="leader"
          label={text('pages.incidentCategories.editor.leader', '负责人')}
          extra={text('pages.incidentCategories.editor.leaderExtra', '报表分片按此定向')}
        >
          <Input style={{ width: 200 }} />
        </Form.Item>
      </Space>
      <Form.Item
        name="subcategories"
        label={text('pages.incidentCategories.editor.subcategories', '子类白名单')}
        extra={text('pages.incidentCategories.editor.subcategoriesExtra', '留空表示登记时不限子类')}
      >
        <Select
          mode="tags"
          open={false}
          suffixIcon={null}
          placeholder={text(
            'pages.incidentCategories.editor.subcategoriesPlaceholder',
            '输入后回车',
          )}
        />
      </Form.Item>
      <Space size="middle" style={{ display: 'flex' }}>
        <Form.Item
          name="audienceRoles"
          label={text('pages.incidentCategories.editor.audienceRoles', '可见角色')}
        >
          <Select mode="tags" open={false} suffixIcon={null} style={{ minWidth: 200 }} />
        </Form.Item>
        <Form.Item
          name="audienceUsers"
          label={text('pages.incidentCategories.editor.audienceUsers', '可见账号')}
        >
          <Select mode="tags" open={false} suffixIcon={null} style={{ minWidth: 200 }} />
        </Form.Item>
      </Space>
      <Form.Item
        name="enabled"
        label={text('pages.incidentCategories.editor.enabled', '启用')}
        valuePropName="checked"
      >
        <Switch />
      </Form.Item>
      <Space style={{ display: 'flex', justifyContent: 'flex-end' }}>
        <Button onClick={onCancel}>
          <FormattedMessage id="pages.incidentCategories.editor.cancel" defaultMessage="取消" />
        </Button>
        <Button type="primary" loading={saving} htmlType="submit">
          <FormattedMessage id="pages.incidentCategories.editor.save" defaultMessage="保存" />
        </Button>
      </Space>
    </Form>
  );
}
