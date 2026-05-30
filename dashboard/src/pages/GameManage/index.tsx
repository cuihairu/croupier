import React, { useEffect, useState } from 'react';
import {
  Card,
  Table,
  Form,
  Input,
  Button,
  Space,
  AutoComplete,
  Modal,
  Popconfirm,
  Tag,
  message,
} from 'antd';
import { listGamesMeta, upsertGame, deleteGame, updateGame, type Game } from '@/services/api';

const GAME_ID_PATTERN = /^[A-Za-z0-9_@-]+$/;
const notifyGamesChanged = () => {
  if (typeof window === 'undefined') return;
  window.dispatchEvent(new CustomEvent('games:changed'));
};

export default function GameManagePage() {
  const [data, setData] = useState<Game[]>([]);
  const [loading, setLoading] = useState(false);
  const [form] = Form.useForm();
  const [editForm] = Form.useForm();
  const [submitting, setSubmitting] = useState(false);
  const [editing, setEditing] = useState<Game | null>(null);

  const reload = async () => {
    setLoading(true);
    try {
      const res = await listGamesMeta();
      setData(res.games || []);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    reload();
  }, []);

  const onAdd = async () => {
    try {
      const v = await form.validateFields();
      setSubmitting(true);
      await upsertGame({
        name: String(v.game_id || '').trim(),
        aliasName: String(v.alias_name || '').trim() || undefined,
        description: String(v.description || '').trim() || undefined,
      });
      form.resetFields();
      await reload();
      notifyGamesChanged();
    } catch (_) {
      // handled by global request interceptor
    } finally {
      setSubmitting(false);
    }
  };

  const onEdit = (row: Game) => {
    setEditing(row);
    editForm.setFieldsValue({
      alias_name: row.aliasName || '',
      description: row.description || '',
    });
  };

  const onSaveEdit = async () => {
    if (!editing?.id) return;
    const v = await editForm.validateFields();
    await updateGame(editing.id, {
      name: editing.name,
      aliasName: String(v.alias_name || '').trim(),
      description: String(v.description || '').trim(),
    });
    message.success('已保存');
    setEditing(null);
    await reload();
    notifyGamesChanged();
  };

  return (
    <Card title="Game Management">
      <Space direction="vertical" style={{ width: '100%' }}>
        <Form form={form} layout="inline">
          <Form.Item
            name="game_id"
            label="game_id"
            rules={[
              { required: true, message: '请输入 game_id' },
              { pattern: GAME_ID_PATTERN, message: '仅支持字母、数字和 _ - @' },
              {
                validator: async (_, value) => {
                  const next = String(value || '')
                    .trim()
                    .toLowerCase();
                  if (!next) return;
                  const exists = (data || []).some(
                    (item) =>
                      String(item.name || '')
                        .trim()
                        .toLowerCase() === next,
                  );
                  if (exists) {
                    throw new Error('game_id 已存在');
                  }
                },
              },
            ]}
          >
            {/* Dropdown suggestions + free input */}
            <AutoComplete
              style={{ width: 240 }}
              placeholder="e.g. default | mygame"
              options={[...new Set((data || []).map((d) => d.name).filter(Boolean))].map((g) => ({
                value: g!,
              }))}
              filterOption={(inputValue, option) =>
                (option?.value || '').toLowerCase().includes(inputValue.toLowerCase())
              }
            />
          </Form.Item>
          <Form.Item name="alias_name" label="alias">
            <Input style={{ width: 180 }} placeholder="显示名（可选）" />
          </Form.Item>
          <Form.Item name="description" label="desc">
            <Input style={{ width: 260 }} placeholder="描述（可选）" />
          </Form.Item>
          <Form.Item>
            <Button type="primary" onClick={onAdd} loading={submitting}>
              Add
            </Button>
          </Form.Item>
        </Form>
        <Table
          rowKey={(r) => String(r.id || r.name)}
          loading={loading}
          dataSource={data}
          pagination={false}
          columns={[
            { title: 'ID', dataIndex: 'id', width: 80 },
            {
              title: 'game_id',
              dataIndex: 'name',
              render: (v: string) => <Tag color="blue">{v}</Tag>,
            },
            { title: 'alias', dataIndex: 'aliasName' },
            { title: 'description', dataIndex: 'description' },
            {
              title: 'actions',
              width: 120,
              render: (_, row) => (
                <Space>
                  <Button size="small" onClick={() => onEdit(row)}>
                    Edit
                  </Button>
                  <Popconfirm
                    title={`Delete game "${row.name}"?`}
                    onConfirm={async () => {
                      if ((data || []).length <= 1) {
                        Modal.warning({
                          title: '无法删除',
                          content: '至少需要保留一个游戏，当前仅剩 1 个。',
                        });
                        return;
                      }
                      if (!row.id) {
                        Modal.warning({
                          title: '无法删除',
                          content: '该游戏缺少 ID，无法调用删除接口。',
                        });
                        return;
                      }
                      await deleteGame(row.id);
                      await reload();
                      notifyGamesChanged();
                    }}
                  >
                    <Button danger size="small" disabled={(data || []).length <= 1}>
                      Delete
                    </Button>
                  </Popconfirm>
                </Space>
              ),
            },
          ]}
        />
      </Space>
      <Modal
        title={editing ? `Edit ${editing.name}` : 'Edit Game'}
        open={Boolean(editing)}
        onCancel={() => setEditing(null)}
        onOk={onSaveEdit}
        okText="Save"
      >
        <Form form={editForm} layout="vertical">
          <Form.Item name="alias_name" label="alias">
            <Input placeholder="显示名（可选）" />
          </Form.Item>
          <Form.Item name="description" label="desc">
            <Input.TextArea rows={3} placeholder="描述（可选）" />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  );
}
