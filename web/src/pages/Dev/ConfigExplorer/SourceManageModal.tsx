import React, { useEffect, useRef, useState } from 'react';
import { App, Button, Form, Input, Modal, Select, Space, Table, Tag } from 'antd';
import type { FormInstance } from 'antd';
import { ModalForm } from '@ant-design/pro-components';
import {
  deleteConfigSource,
  listConfigSources,
  upsertConfigSource,
  type ConfigSourceBinding,
} from '@/services/api/configExplorer';
import { FormattedMessage, useIntl } from '@umijs/max';

/** 数据源表单值：id/gameId/env 不在表单中，提交时由编辑行与弹窗上下文补齐 */
type SourceFormValues = {
  name: string;
  type: ConfigSourceBinding['type'];
  config: string;
};

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

// 类型选项：value 是后端契约枚举不动，label 文案经 intl 解析（textId/textDefault）
const TYPE_OPTIONS: {
  value: ConfigSourceBinding['type'];
  textId: string;
  textDefault: string;
}[] = [
  {
    value: 'git',
    textId: 'pages.devConfigExplorer.sourceModal.typeOption.git',
    textDefault: 'Git 仓库（只读）',
  },
  {
    value: 'redis',
    textId: 'pages.devConfigExplorer.sourceModal.typeOption.redis',
    textDefault: 'Redis（skynet 配置总线）',
  },
  {
    value: 'nacos',
    textId: 'pages.devConfigExplorer.sourceModal.typeOption.nacos',
    textDefault: 'Nacos 配置中心',
  },
  {
    value: 'db',
    textId: 'pages.devConfigExplorer.sourceModal.typeOption.db',
    textDefault: '数据库（表快照，只读）',
  },
  {
    value: 'croupier',
    textId: 'pages.devConfigExplorer.sourceModal.typeOption.croupier',
    textDefault: 'Croupier ConfigVersion',
  },
];

export default function SourceManageModal({
  open,
  gameId,
  env,
  onClose,
  onChanged,
}: SourceManageModalProps) {
  const { message } = App.useApp();
  const intl = useIntl();
  const [rows, setRows] = useState<ConfigSourceBinding[]>([]);
  const [editing, setEditing] = useState<ConfigSourceBinding | null>(null);
  const [formOpen, setFormOpen] = useState(false);
  // 类型切换 → 刷新 config 模板的字段联动需要写回表单实例，经 formRef 拿 ModalForm 托管实例
  const formRef = useRef<FormInstance<SourceFormValues>>(undefined);

  const load = async () => {
    try {
      const { items } = await listConfigSources({ gameId, env });
      setRows(items);
    } catch {
      message.error(
        intl.formatMessage({
          id: 'pages.devConfigExplorer.sourceModal.error.loadFailed',
          defaultMessage: '加载数据源失败',
        }),
      );
    }
  };

  useEffect(() => {
    if (open && gameId && env) void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, gameId, env]);

  // destroyOnHidden 使弹窗每次关闭即卸载表单，重开时按最新 initialValues
  // 重新挂载，新增/编辑切换不会残留上一次的预填值
  const openCreate = () => {
    setEditing(null);
    setFormOpen(true);
  };

  const openEdit = (row: ConfigSourceBinding) => {
    setEditing(row);
    setFormOpen(true);
  };

  const onFinish = async (values: SourceFormValues) => {
    try {
      await upsertConfigSource({
        id: editing?.id,
        gameId,
        env,
        ...values,
      });
      message.success(
        intl.formatMessage({
          id: 'pages.devConfigExplorer.sourceModal.success.saved',
          defaultMessage: '已保存',
        }),
      );
      void load();
      onChanged();
      return true;
    } catch (err) {
      const msg =
        err instanceof Error
          ? err.message
          : intl.formatMessage({
              id: 'pages.devConfigExplorer.sourceModal.error.saveFailed',
              defaultMessage: '保存失败',
            });
      message.error(msg);
      return false;
    }
  };

  const doDelete = async (row: ConfigSourceBinding) => {
    try {
      await deleteConfigSource(row.id);
      message.success(
        intl.formatMessage({
          id: 'pages.devConfigExplorer.sourceModal.success.deleted',
          defaultMessage: '已删除',
        }),
      );
      await load();
      onChanged();
    } catch {
      message.error(
        intl.formatMessage({
          id: 'pages.devConfigExplorer.sourceModal.error.deleteFailed',
          defaultMessage: '删除失败',
        }),
      );
    }
  };

  return (
    <Modal
      open={open}
      title={intl.formatMessage(
        {
          id: 'pages.devConfigExplorer.sourceModal.modal.manageTitle',
          defaultMessage: `管理数据源（${gameId} / ${env}）`,
        },
        { gameId, env },
      )}
      width={720}
      footer={null}
      onCancel={onClose}
      destroyOnHidden
    >
      <Space orientation="vertical" style={{ width: '100%' }}>
        <Button type="primary" onClick={openCreate}>
          <FormattedMessage
            id="pages.devConfigExplorer.sourceModal.action.add"
            defaultMessage="添加数据源"
          />
        </Button>
        <Table<ConfigSourceBinding>
          size="small"
          rowKey="id"
          dataSource={rows}
          pagination={false}
          columns={[
            {
              title: intl.formatMessage({
                id: 'pages.devConfigExplorer.sourceModal.column.name',
                defaultMessage: '名称',
              }),
              dataIndex: 'name',
            },
            {
              title: intl.formatMessage({
                id: 'pages.devConfigExplorer.sourceModal.column.type',
                defaultMessage: '类型',
              }),
              dataIndex: 'type',
              width: 100,
            },
            {
              title: intl.formatMessage({
                id: 'pages.devConfigExplorer.sourceModal.column.writable',
                defaultMessage: '读写',
              }),
              dataIndex: 'writable',
              width: 80,
              render: (v: boolean) => (
                <Tag color={v ? 'orange' : 'default'}>
                  {v
                    ? intl.formatMessage({
                        id: 'pages.devConfigExplorer.sourceModal.tag.writable',
                        defaultMessage: '可写',
                      })
                    : intl.formatMessage({
                        id: 'pages.devConfigExplorer.sourceModal.tag.readonly',
                        defaultMessage: '只读',
                      })}
                </Tag>
              ),
            },
            {
              title: intl.formatMessage({
                id: 'pages.devConfigExplorer.sourceModal.column.operations',
                defaultMessage: '操作',
              }),
              width: 140,
              render: (_, row) => (
                <Space>
                  <Button size="small" onClick={() => openEdit(row)}>
                    <FormattedMessage
                      id="pages.devConfigExplorer.sourceModal.action.edit"
                      defaultMessage="编辑"
                    />
                  </Button>
                  <Button size="small" danger onClick={() => void doDelete(row)}>
                    <FormattedMessage
                      id="pages.devConfigExplorer.sourceModal.action.delete"
                      defaultMessage="删除"
                    />
                  </Button>
                </Space>
              ),
            },
          ]}
        />
      </Space>

      <ModalForm<SourceFormValues>
        open={formOpen}
        title={
          editing
            ? intl.formatMessage(
                {
                  id: 'pages.devConfigExplorer.sourceModal.modal.editTitle',
                  defaultMessage: `编辑：${editing.name}`,
                },
                { name: editing.name },
              )
            : intl.formatMessage({
                id: 'pages.devConfigExplorer.sourceModal.action.add',
                defaultMessage: '添加数据源',
              })
        }
        onOpenChange={(v) => {
          if (!v) setFormOpen(false);
        }}
        modalProps={{ destroyOnHidden: true }}
        width={640}
        layout="vertical"
        submitter={{
          searchConfig: {
            submitText: intl.formatMessage({
              id: 'pages.devConfigExplorer.sourceModal.form.submit',
              defaultMessage: '保存',
            }),
          },
        }}
        // 编辑时展示脱敏 config；后端 Update 逻辑：config 为空则保留原值，
        // 脱敏值需用户重填或保持 ****** 原样提交沿用旧值
        initialValues={
          editing
            ? { name: editing.name, type: editing.type, config: editing.config }
            : { name: '', type: 'git', config: CONFIG_TEMPLATES.git }
        }
        formRef={formRef}
        onFinish={onFinish}
      >
        <Form.Item
          name="name"
          label={intl.formatMessage({
            id: 'pages.devConfigExplorer.sourceModal.column.name',
            defaultMessage: '名称',
          })}
          rules={[
            {
              required: true,
              message: intl.formatMessage({
                id: 'pages.devConfigExplorer.sourceModal.form.nameRequired',
                defaultMessage: '必填',
              }),
            },
          ]}
        >
          <Input
            placeholder={intl.formatMessage({
              id: 'pages.devConfigExplorer.sourceModal.form.namePlaceholder',
              defaultMessage: '如：数值表仓库 / skynet 配置总线',
            })}
            maxLength={64}
          />
        </Form.Item>
        <Form.Item
          name="type"
          label={intl.formatMessage({
            id: 'pages.devConfigExplorer.sourceModal.column.type',
            defaultMessage: '类型',
          })}
          rules={[{ required: true }]}
        >
          <Select
            options={TYPE_OPTIONS.map((o) => ({
              value: o.value,
              label: intl.formatMessage({ id: o.textId, defaultMessage: o.textDefault }),
            }))}
            onChange={(v: ConfigSourceBinding['type']) =>
              formRef.current?.setFieldValue('config', CONFIG_TEMPLATES[v])
            }
            disabled={!!editing}
          />
        </Form.Item>
        <Form.Item
          name="config"
          label={intl.formatMessage({
            id: 'pages.devConfigExplorer.sourceModal.form.configLabel',
            defaultMessage: '连接配置（JSON）',
          })}
          rules={[
            { required: true },
            {
              validator: (_, value) => {
                if (!value) return Promise.resolve();
                try {
                  JSON.parse(value);
                  return Promise.resolve();
                } catch {
                  return Promise.reject(
                    new Error(
                      intl.formatMessage({
                        id: 'pages.devConfigExplorer.sourceModal.form.configValidJson',
                        defaultMessage: '必须是合法 JSON',
                      }),
                    ),
                  );
                }
              },
            },
          ]}
        >
          <Input.TextArea rows={10} style={{ fontFamily: 'monospace' }} />
        </Form.Item>
        {editing && (
          <span style={{ color: '#999', fontSize: 12 }}>
            <FormattedMessage
              id="pages.devConfigExplorer.sourceModal.hint.maskedCredentials"
              defaultMessage="凭据字段已脱敏显示；无需更换凭据时保持 ****** 原样提交即可沿用旧值。"
            />
          </span>
        )}
      </ModalForm>
    </Modal>
  );
}
