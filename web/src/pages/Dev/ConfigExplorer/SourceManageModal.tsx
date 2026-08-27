import React, { useEffect, useState } from 'react';
import { App, Button, Form, Input, Modal, Select, Space, Table, Tag } from 'antd';
import {
  deleteConfigSource,
  listConfigSources,
  upsertConfigSource,
  type ConfigSourceBinding,
} from '@/services/api/configExplorer';

export type SourceManageModalProps = {
  open: boolean;
  gameId: string;
  env: string;
  onClose: () => void;
  onChanged: () => void;
};

// 各类型的连接配置模板（JSON）——与后端适配器 schema 对齐
const CONFIG_TEMPLATES: Record<ConfigSourceBinding['type'], string> = {
  git: JSON.stringify(
    { repoUrl: '', branch: 'main', subPath: '', username: '', password: '' },
    null,
    2,
  ),
  redis: JSON.stringify(
    { addr: '127.0.0.1:6379', password: '', db: 0, prefix: 'cfg:', delimiter: '/' },
    null,
    2,
  ),
  nacos: JSON.stringify(
    {
      endpoint: 'http://127.0.0.1:8848',
      namespaceId: '',
      group: 'DEFAULT_GROUP',
      username: '',
      password: '',
    },
    null,
    2,
  ),
  db: JSON.stringify({ dsn: 'user:pass@tcp(127.0.0.1:3306)/game_db', tables: [] }, null, 2),
  croupier: JSON.stringify({ namespaces: [] }, null, 2),
};

const TYPE_OPTIONS = [
  { label: 'Git 仓库（只读）', value: 'git' },
  { label: 'Redis（skynet 配置总线）', value: 'redis' },
  { label: 'Nacos 配置中心', value: 'nacos' },
  { label: '数据库（表快照，只读）', value: 'db' },
  { label: 'Croupier ConfigVersion', value: 'croupier' },
];

export default function SourceManageModal({
  open,
  gameId,
  env,
  onClose,
  onChanged,
}: SourceManageModalProps) {
  const { message } = App.useApp();
  const [rows, setRows] = useState<ConfigSourceBinding[]>([]);
  const [editing, setEditing] = useState<ConfigSourceBinding | null>(null);
  const [formOpen, setFormOpen] = useState(false);
  const [form] = Form.useForm();

  const load = async () => {
    try {
      const { items } = await listConfigSources({ gameId, env });
      setRows(items);
    } catch {
      message.error('加载数据源失败');
    }
  };

  useEffect(() => {
    if (open && gameId && env) void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, gameId, env]);

  const openCreate = () => {
    form.resetFields();
    form.setFieldsValue({
      name: '',
      type: 'git',
      config: CONFIG_TEMPLATES.git,
    });
    setEditing(null);
    setFormOpen(true);
  };

  const openEdit = (row: ConfigSourceBinding) => {
    form.resetFields();
    // 编辑时展示脱敏 config；保存时空值字段会被后端忽略已有凭据？
    // —— 后端 Update 逻辑：config 为空则保留原值；脱敏值需用户重填或清空保留
    form.setFieldsValue({ name: row.name, type: row.type, config: row.config });
    setEditing(row);
    setFormOpen(true);
  };

  const doSave = async () => {
    const values = await form.validateFields();
    try {
      await upsertConfigSource({
        id: editing?.id,
        gameId,
        env,
        name: values.name,
        type: values.type,
        config: values.config,
      });
      message.success('已保存');
      setFormOpen(false);
      await load();
      onChanged();
    } catch (err) {
      const msg = err instanceof Error ? err.message : '保存失败';
      message.error(msg);
    }
  };

  const doDelete = async (row: ConfigSourceBinding) => {
    try {
      await deleteConfigSource(row.id);
      message.success('已删除');
      await load();
      onChanged();
    } catch {
      message.error('删除失败');
    }
  };

  return (
    <Modal
      open={open}
      title={`管理数据源（${gameId} / ${env}）`}
      width={720}
      footer={null}
      onCancel={onClose}
      destroyOnHidden
    >
      <Space direction="vertical" style={{ width: '100%' }}>
        <Button type="primary" onClick={openCreate}>
          添加数据源
        </Button>
        <Table<ConfigSourceBinding>
          size="small"
          rowKey="id"
          dataSource={rows}
          pagination={false}
          columns={[
            { title: '名称', dataIndex: 'name' },
            { title: '类型', dataIndex: 'type', width: 100 },
            {
              title: '读写',
              dataIndex: 'writable',
              width: 80,
              render: (v: boolean) => (
                <Tag color={v ? 'orange' : 'default'}>{v ? '可写' : '只读'}</Tag>
              ),
            },
            {
              title: '操作',
              width: 140,
              render: (_, row) => (
                <Space>
                  <Button size="small" onClick={() => openEdit(row)}>
                    编辑
                  </Button>
                  <Button size="small" danger onClick={() => void doDelete(row)}>
                    删除
                  </Button>
                </Space>
              ),
            },
          ]}
        />
      </Space>

      <Modal
        open={formOpen}
        title={editing ? `编辑：${editing.name}` : '添加数据源'}
        okText="保存"
        onOk={doSave}
        onCancel={() => setFormOpen(false)}
        destroyOnHidden
        width={640}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="名称" rules={[{ required: true, message: '必填' }]}>
            <Input placeholder="如：数值表仓库 / skynet 配置总线" maxLength={64} />
          </Form.Item>
          <Form.Item name="type" label="类型" rules={[{ required: true }]}>
            <Select
              options={TYPE_OPTIONS}
              onChange={(v: ConfigSourceBinding['type']) =>
                form.setFieldValue('config', CONFIG_TEMPLATES[v])
              }
              disabled={!!editing}
            />
          </Form.Item>
          <Form.Item
            name="config"
            label="连接配置（JSON）"
            rules={[
              { required: true },
              {
                validator: (_, value) => {
                  if (!value) return Promise.resolve();
                  try {
                    JSON.parse(value);
                    return Promise.resolve();
                  } catch {
                    return Promise.reject(new Error('必须是合法 JSON'));
                  }
                },
              },
            ]}
          >
            <Input.TextArea rows={10} style={{ fontFamily: 'monospace' }} />
          </Form.Item>
        </Form>
        {editing && (
          <span style={{ color: '#999', fontSize: 12 }}>
            凭据字段已脱敏显示；无需更换凭据时保持 ****** 原样提交即可沿用旧值。
          </span>
        )}
      </Modal>
    </Modal>
  );
}
