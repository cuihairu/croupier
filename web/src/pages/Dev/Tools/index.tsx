import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  App,
  Button,
  Card,
  Col,
  Dropdown,
  Empty,
  Form,
  Input,
  Modal,
  Popconfirm,
  Row,
  Select,
  Space,
  Switch,
  Tag,
  Typography,
} from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import {
  AppstoreOutlined,
  BookOutlined,
  CloudServerOutlined,
  DatabaseOutlined,
  DeploymentUnitOutlined,
  ExportOutlined,
  GithubOutlined,
  PlusOutlined,
  ReloadOutlined,
  SettingOutlined,
  ToolOutlined,
} from '@ant-design/icons';
import { useAccess, useIntl } from '@umijs/max';
import {
  createTool,
  deleteTool,
  listTools,
  toolCategoryLabels,
  toolCategoryOrder,
  updateTool,
  type ToolCategory,
  type ToolItem,
} from '@/services/api/tools';
import { extractErrorMessage } from '@/utils/errors';
import { getScope } from '@/stores/scope';

const { Paragraph, Text } = Typography;

function categoryIcon(category: string): React.ReactNode {
  switch (category) {
    case 'ci':
      return <DeploymentUnitOutlined />;
    case 'repo':
      return <GithubOutlined />;
    case 'monitor':
      return <CloudServerOutlined />;
    case 'docs':
      return <BookOutlined />;
    case 'artifact':
      return <DatabaseOutlined />;
    default:
      return <AppstoreOutlined />;
  }
}

export default function DevToolsPage() {
  const { message } = App.useApp();
  const access = useAccess();
  const intl = useIntl();
  const canManage = Boolean(access.canDevManage);

  const [tools, setTools] = useState<ToolItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<ToolItem | null>(null);
  const [saving, setSaving] = useState(false);
  const [form] = Form.useForm();

  const scope = getScope();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await listTools({
        gameId: scope?.gameId || undefined,
        env: scope?.env || undefined,
      });
      setTools(res.items || []);
    } catch (error) {
      message.error(extractErrorMessage(error, '加载工具列表失败'));
    } finally {
      setLoading(false);
    }
  }, [message, scope?.gameId, scope?.env]);

  useEffect(() => {
    load();
  }, [load]);

  const grouped = useMemo(() => {
    const map = new Map<string, ToolItem[]>();
    for (const t of tools) {
      const list = map.get(t.category) || [];
      list.push(t);
      map.set(t.category, list);
    }
    return toolCategoryOrder
      .filter((c) => (map.get(c) || []).length > 0)
      .map((c) => ({ category: c as ToolCategory, items: map.get(c)! }));
  }, [tools]);

  const openCreate = () => {
    setEditing(null);
    form.resetFields();
    setModalOpen(true);
  };

  const openEdit = (tool: ToolItem) => {
    setEditing(tool);
    form.setFieldsValue({
      name: tool.name,
      url: tool.url,
      description: tool.description,
      category: tool.category,
      gameId: tool.gameId,
      env: tool.env,
      sort: tool.sort,
      enabled: tool.enabled,
    });
    setModalOpen(true);
  };

  const submit = async () => {
    const v = await form.validateFields();
    setSaving(true);
    try {
      if (editing) {
        await updateTool(editing.id, v);
        message.success('工具已更新');
      } else {
        await createTool(v);
        message.success('工具已登记');
      }
      setModalOpen(false);
      load();
    } catch (error) {
      message.error(extractErrorMessage(error, '保存失败'));
    } finally {
      setSaving(false);
    }
  };

  const toggleEnabled = async (tool: ToolItem, enabled: boolean) => {
    try {
      await updateTool(tool.id, { enabled });
      load();
    } catch (error) {
      message.error(extractErrorMessage(error, '操作失败'));
    }
  };

  const remove = async (tool: ToolItem) => {
    try {
      await deleteTool(tool.id);
      message.success('已删除');
      load();
    } catch (error) {
      message.error(extractErrorMessage(error, '删除失败'));
    }
  };

  return (
    <PageContainer>
      <Card
        title={
          <Space>
            <ToolOutlined />
            工具箱
          </Space>
        }
        extra={
          <Space>
            <Button icon={<ReloadOutlined />} onClick={load} loading={loading}>
              刷新
            </Button>
            {canManage ? (
              <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
                登记工具
              </Button>
            ) : null}
          </Space>
        }
      >
        {tools.length === 0 && !loading ? (
          <Empty description="暂无工具。让管理员登记 Jenkins / GitLab / Grafana 等内部工具链接，即可在此集中访问。" />
        ) : (
          <Space direction="vertical" size={16} style={{ width: '100%' }}>
            {grouped.map(({ category, items }) => (
              <div key={category}>
                <Space style={{ marginBottom: 8 }}>
                  {categoryIcon(category)}
                  <Text strong>{toolCategoryLabels[category]}</Text>
                  <Tag>{items.length}</Tag>
                </Space>
                <Row gutter={[12, 12]}>
                  {items.map((tool) => (
                    <Col xs={24} sm={12} md={8} lg={6} key={tool.id}>
                      <Card
                        size="small"
                        hoverable
                        actions={
                          canManage
                            ? [
                                <ExportOutlined
                                  key="open"
                                  onClick={() => window.open(tool.url, '_blank', 'noreferrer')}
                                />,
                                <SettingOutlined key="edit" onClick={() => openEdit(tool)} />,
                                <Popconfirm
                                  key="del"
                                  title={`删除工具「${tool.name}」？`}
                                  onConfirm={() => remove(tool)}
                                >
                                  <Button type="text" size="small" danger>
                                    删除
                                  </Button>
                                </Popconfirm>,
                              ]
                            : undefined
                        }
                      >
                        <Card.Meta
                          avatar={categoryIcon(tool.category)}
                          title={
                            <a href={tool.url} target="_blank" rel="noreferrer">
                              {tool.name} <ExportOutlined />
                            </a>
                          }
                          description={
                            <Space direction="vertical" size={4}>
                              {tool.description ? (
                                <Paragraph ellipsis={{ rows: 2 }} style={{ marginBottom: 0 }}>
                                  {tool.description}
                                </Paragraph>
                              ) : null}
                              <Space size={4} wrap>
                                {tool.gameId ? (
                                  <Tag color="blue">
                                    {tool.gameId}/{tool.env}
                                  </Tag>
                                ) : (
                                  <Tag>全局</Tag>
                                )}
                                {canManage ? (
                                  <Switch
                                    size="small"
                                    checked={tool.enabled}
                                    onChange={(v) => toggleEnabled(tool, v)}
                                  />
                                ) : null}
                              </Space>
                            </Space>
                          }
                        />
                      </Card>
                    </Col>
                  ))}
                </Row>
              </div>
            ))}
          </Space>
        )}
      </Card>

      <Modal
        title={editing ? `编辑工具：${editing.name}` : '登记内部工具'}
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        footer={
          <Space>
            <Button onClick={() => setModalOpen(false)}>取消</Button>
            <Button type="primary" loading={saving} onClick={submit}>
              保存
            </Button>
          </Space>
        }
        destroyOnHidden
      >
        <Form form={form} layout="vertical" initialValues={{ category: 'ci', sort: 0 }}>
          <Form.Item name="name" label="名称" rules={[{ required: true, message: '请输入名称' }]}>
            <Input placeholder="如 Jenkins / GitLab / Grafana" />
          </Form.Item>
          <Form.Item
            name="url"
            label="地址"
            rules={[
              { required: true, message: '请输入地址' },
              {
                pattern: /^https?:\/\//,
                message: '必须以 http:// 或 https:// 开头',
              },
            ]}
          >
            <Input placeholder="https://ci.example.com" />
          </Form.Item>
          <Form.Item name="category" label="分类">
            <Select
              options={toolCategoryOrder.map((c) => ({ label: toolCategoryLabels[c], value: c }))}
            />
          </Form.Item>
          <Form.Item name="description" label="描述">
            <Input placeholder="可选" />
          </Form.Item>
          <Form.Item
            name="scopeMode"
            label="作用域"
            initialValue={editing?.gameId ? 'scoped' : 'global'}
          >
            <Select
              options={[
                { label: '全局（所有游戏可见）', value: 'global' },
                {
                  label: `当前游戏环境（${scope?.gameId || '-'}/${scope?.env || '-'}）`,
                  value: 'scoped',
                },
              ]}
              onChange={(mode) => {
                if (mode === 'global') {
                  form.setFieldsValue({ gameId: '', env: '' });
                } else {
                  form.setFieldsValue({ gameId: scope?.gameId, env: scope?.env });
                }
              }}
            />
          </Form.Item>
          <Form.Item noStyle shouldUpdate>
            {() =>
              form.getFieldValue('scopeMode') === 'scoped' ? (
                <Space>
                  <Form.Item name="gameId" label="gameId" style={{ marginBottom: 12 }}>
                    <Input style={{ width: 160 }} />
                  </Form.Item>
                  <Form.Item name="env" label="env" style={{ marginBottom: 12 }}>
                    <Input style={{ width: 120 }} />
                  </Form.Item>
                </Space>
              ) : null
            }
          </Form.Item>
          {editing ? (
            <Form.Item name="enabled" label="启用" valuePropName="checked">
              <Switch />
            </Form.Item>
          ) : null}
        </Form>
      </Modal>
    </PageContainer>
  );
}
