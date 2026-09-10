import React from 'react';
import {
  Alert,
  Button,
  Card,
  Col,
  Descriptions,
  Divider,
  Form,
  Input,
  Row,
  Select,
  Space,
  Switch,
  Tag,
} from 'antd';
import type { FormInstance } from 'antd/es/form';
import { CopyOutlined } from '@ant-design/icons';
import { CodeEditor } from '@/components/MonacoDynamic';
import { formatDateTime } from '@/utils/format';
import { FormattedMessage, useIntl } from '@umijs/max';

const { TextArea } = Input;

export type FunctionDetailData = {
  id: string;
  description?: string;
  resource?: string;
  operation?: string;
  version?: string;
  enabled: boolean;
  tags?: string[];
  createdAt: string;
  updatedAt: string;
  provider?: string;
  agentCount?: number;
  health?: 'healthy' | 'unhealthy' | 'unknown';
};

export function JsonViewer({
  data,
  onCopySuccess,
  onCopyError,
}: {
  data: unknown;
  onCopySuccess: () => void;
  onCopyError: () => void;
}) {
  const pretty = JSON.stringify(data || {}, null, 2);

  const beforeMount = (monaco: unknown) => {
    const m = monaco as {
      editor?: {
        getTheme?: () => string;
        defineTheme?: (name: string, data: unknown) => void;
      };
    };
    if (!m?.editor || m.editor.getTheme?.() === 'sublime-monokai') return;
    m.editor.defineTheme?.('sublime-monokai', {
      base: 'vs-dark',
      inherit: true,
      rules: [
        { token: 'string.key.json', foreground: '66D9EF' },
        { token: 'string.value.json', foreground: 'A6E22E' },
        { token: 'number', foreground: 'E6DB74' },
        { token: 'keyword', foreground: 'F92672' },
      ],
      colors: {
        'editor.background': '#272822',
        'editorLineNumber.foreground': '#75715E',
        'editorLineNumber.activeForeground': '#F8F8F2',
      },
    });
  };

  const copyJson = async () => {
    try {
      await navigator.clipboard.writeText(pretty);
      onCopySuccess();
    } catch {
      onCopyError();
    }
  };

  return (
    <div
      style={{
        border: '1px solid #f0f0f0',
        borderRadius: 8,
        overflow: 'hidden',
        background: '#fafafa',
      }}
    >
      <div
        style={{
          display: 'flex',
          justifyContent: 'flex-end',
          alignItems: 'center',
          padding: '8px 12px',
          borderBottom: '1px solid #f0f0f0',
          background: '#fff',
        }}
      >
        <Button size="small" icon={<CopyOutlined />} onClick={copyJson}>
          <FormattedMessage id="pages.functionsDetail.section.json.copy" defaultMessage="复制" />
        </Button>
      </div>
      <CodeEditor
        value={pretty}
        language="json"
        height={500}
        readOnly
        theme="sublime-monokai"
        beforeMount={beforeMount}
        options={{
          lineNumbers: 'on',
          renderLineHighlight: 'line',
          scrollBeyondLastLine: false,
          automaticLayout: true,
          minimap: { enabled: false },
        }}
      />
    </div>
  );
}

export function BasicInfoTab({
  functionDetail,
  effectiveResource,
  editing,
  onStatusToggle,
}: {
  functionDetail: FunctionDetailData | null;
  effectiveResource: string;
  editing: boolean;
  onStatusToggle: (enabled: boolean) => void;
}) {
  const intl = useIntl();
  return (
    <>
      <Descriptions bordered column={2}>
        <Descriptions.Item
          label={intl.formatMessage({
            id: 'pages.functionsDetail.section.basic.functionId',
            defaultMessage: '函数ID',
          })}
        >
          <code>{functionDetail?.id}</code>
        </Descriptions.Item>
        <Descriptions.Item
          label={intl.formatMessage({
            id: 'pages.functionsDetail.section.basic.version',
            defaultMessage: '版本',
          })}
        >
          <Tag>{functionDetail?.version || '1.0.0'}</Tag>
        </Descriptions.Item>
        <Descriptions.Item
          label={intl.formatMessage({
            id: 'pages.functionsDetail.section.basic.resource',
            defaultMessage: '资源',
          })}
        >
          <Tag color="blue">
            {effectiveResource ||
              intl.formatMessage({
                id: 'pages.functionsDetail.section.basic.notDeclared',
                defaultMessage: '未声明',
              })}
          </Tag>
        </Descriptions.Item>
        <Descriptions.Item
          label={intl.formatMessage({
            id: 'pages.functionsDetail.section.basic.operation',
            defaultMessage: '操作',
          })}
        >
          <Tag color="purple">
            {functionDetail?.operation ||
              intl.formatMessage({
                id: 'pages.functionsDetail.section.basic.notDeclared',
                defaultMessage: '未声明',
              })}
          </Tag>
        </Descriptions.Item>
        <Descriptions.Item
          label={intl.formatMessage({
            id: 'pages.functionsDetail.section.basic.status',
            defaultMessage: '状态',
          })}
        >
          <Space>
            <Switch checked={functionDetail?.enabled || false} onChange={onStatusToggle} />
            <span>
              {functionDetail?.enabled
                ? intl.formatMessage({
                    id: 'pages.functionsDetail.section.basic.enabled',
                    defaultMessage: '已启用',
                  })
                : intl.formatMessage({
                    id: 'pages.functionsDetail.section.basic.disabled',
                    defaultMessage: '已禁用',
                  })}
            </span>
          </Space>
        </Descriptions.Item>
        <Descriptions.Item label="Provider">{functionDetail?.provider || '-'}</Descriptions.Item>
        <Descriptions.Item
          label={intl.formatMessage({
            id: 'pages.functionsDetail.section.basic.health',
            defaultMessage: '健康状态',
          })}
        >
          <Tag
            color={
              functionDetail?.health === 'healthy'
                ? 'green'
                : functionDetail?.health === 'unhealthy'
                  ? 'red'
                  : 'gray'
            }
          >
            {functionDetail?.health === 'healthy'
              ? intl.formatMessage({
                  id: 'pages.functionsDetail.section.basic.healthHealthy',
                  defaultMessage: '健康',
                })
              : functionDetail?.health === 'unhealthy'
                ? intl.formatMessage({
                    id: 'pages.functionsDetail.section.basic.healthUnhealthy',
                    defaultMessage: '异常',
                  })
                : intl.formatMessage({
                    id: 'pages.functionsDetail.section.basic.healthUnknown',
                    defaultMessage: '未知',
                  })}
          </Tag>
        </Descriptions.Item>
        <Descriptions.Item
          label={intl.formatMessage({
            id: 'pages.functionsDetail.section.basic.agentCount',
            defaultMessage: 'Agent 数量',
          })}
        >
          {functionDetail?.agentCount || 0}
        </Descriptions.Item>
        <Descriptions.Item
          label={intl.formatMessage({
            id: 'pages.functionsDetail.section.basic.createdAt',
            defaultMessage: '创建时间',
          })}
        >
          {functionDetail?.createdAt ? formatDateTime(functionDetail.createdAt) : '-'}
        </Descriptions.Item>
        <Descriptions.Item
          label={intl.formatMessage({
            id: 'pages.functionsDetail.section.basic.updatedAt',
            defaultMessage: '更新时间',
          })}
        >
          {functionDetail?.updatedAt ? formatDateTime(functionDetail.updatedAt) : '-'}
        </Descriptions.Item>
      </Descriptions>

      {editing && (
        <>
          <Divider>
            <FormattedMessage
              id="pages.functionsDetail.section.edit.divider"
              defaultMessage="编辑信息"
            />
          </Divider>
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                label={intl.formatMessage({
                  id: 'pages.functionsDetail.section.edit.name.label',
                  defaultMessage: '函数名称',
                })}
                name="name"
                rules={[
                  {
                    required: true,
                    message: intl.formatMessage({
                      id: 'pages.functionsDetail.section.edit.name.required',
                      defaultMessage: '请输入函数名称',
                    }),
                  },
                ]}
              >
                <Input
                  placeholder={intl.formatMessage({
                    id: 'pages.functionsDetail.section.edit.name.placeholder',
                    defaultMessage: '请输入函数名称',
                  })}
                />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                label={intl.formatMessage({
                  id: 'pages.functionsDetail.section.edit.resource.label',
                  defaultMessage: '资源',
                })}
                name="resource"
              >
                <Input
                  placeholder={intl.formatMessage({
                    id: 'pages.functionsDetail.section.edit.resource.placeholder',
                    defaultMessage: '例如 player / mail / economy',
                  })}
                />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item
            label={intl.formatMessage({
              id: 'pages.functionsDetail.section.edit.description.label',
              defaultMessage: '描述',
            })}
            name="description"
          >
            <TextArea
              rows={3}
              placeholder={intl.formatMessage({
                id: 'pages.functionsDetail.section.edit.description.placeholder',
                defaultMessage: '请输入函数描述',
              })}
            />
          </Form.Item>
          <Form.Item
            label={intl.formatMessage({
              id: 'pages.functionsDetail.section.edit.tags.label',
              defaultMessage: '标签',
            })}
            name="tags"
          >
            <Input
              placeholder={intl.formatMessage({
                id: 'pages.functionsDetail.section.edit.tags.placeholder',
                defaultMessage: '请输入标签，多个标签用逗号分隔',
              })}
            />
          </Form.Item>
        </>
      )}

      {!editing && (
        <>
          <Divider>
            <FormattedMessage
              id="pages.functionsDetail.section.view.descriptionDivider"
              defaultMessage="描述"
            />
          </Divider>
          <p>
            {functionDetail?.description ||
              intl.formatMessage({
                id: 'pages.functionsDetail.section.view.noDescription',
                defaultMessage: '暂无描述',
              })}
          </p>
        </>
      )}

      {!editing && functionDetail?.tags && functionDetail.tags.length > 0 && (
        <>
          <Divider>
            <FormattedMessage
              id="pages.functionsDetail.section.view.tagsDivider"
              defaultMessage="标签"
            />
          </Divider>
          <Space wrap>
            {functionDetail.tags.map((tag) => (
              <Tag key={tag} color="geekblue">
                {tag}
              </Tag>
            ))}
          </Space>
        </>
      )}
    </>
  );
}

export function PermissionsTab({
  functionId,
  permError,
  permLoading,
  permSaving,
  permForm,
  onSave,
}: {
  functionId?: string;
  permError: string;
  permLoading: boolean;
  permSaving: boolean;
  permForm: FormInstance;
  onSave: () => Promise<void>;
}) {
  const intl = useIntl();
  return (
    <>
      <Alert
        message={intl.formatMessage({
          id: 'pages.functionsDetail.section.permissions.alertMessage',
          defaultMessage: '权限配置',
        })}
        description={intl.formatMessage({
          id: 'pages.functionsDetail.section.permissions.alertDescription',
          defaultMessage:
            '用于控制哪些角色可以调用该函数（actions 建议使用 invoke/execute；roles 填角色名）。',
        })}
        type="info"
        showIcon
      />

      {permError && (
        <Alert
          style={{ marginTop: 16 }}
          type="error"
          showIcon
          message={intl.formatMessage({
            id: 'pages.functionsDetail.section.permissions.errorTitle',
            defaultMessage: '无法读取权限',
          })}
          description={permError}
        />
      )}

      <Card
        style={{ marginTop: 16 }}
        loading={permLoading}
        size="small"
        title={intl.formatMessage({
          id: 'pages.functionsDetail.section.permissions.cardTitle',
          defaultMessage: '函数权限规则',
        })}
      >
        <Form form={permForm} layout="vertical">
          <Form.List name="items">
            {(fields, { add, remove }) => (
              <Space orientation="vertical" style={{ width: '100%' }} size="middle">
                {fields.map((field) => (
                  <Card
                    key={field.key}
                    size="small"
                    type="inner"
                    title={intl.formatMessage(
                      {
                        id: 'pages.functionsDetail.section.permissions.ruleTitle',
                        defaultMessage: '规则 #{index}',
                      },
                      { index: field.name + 1 },
                    )}
                    extra={
                      <Button danger size="small" onClick={() => remove(field.name)}>
                        <FormattedMessage
                          id="pages.functionsDetail.section.permissions.remove"
                          defaultMessage="删除"
                        />
                      </Button>
                    }
                  >
                    <Row gutter={16}>
                      <Col span={6}>
                        <Form.Item
                          {...field}
                          label="resource"
                          name={[field.name, 'resource']}
                          rules={[
                            {
                              required: true,
                              message: intl.formatMessage({
                                id: 'pages.functionsDetail.section.permissions.resourceRequired',
                                defaultMessage: 'resource 必填',
                              }),
                            },
                          ]}
                        >
                          <Input placeholder="function" />
                        </Form.Item>
                      </Col>
                      <Col span={6}>
                        <Form.Item
                          {...field}
                          label="actions"
                          name={[field.name, 'actions']}
                          rules={[
                            {
                              required: true,
                              message: intl.formatMessage({
                                id: 'pages.functionsDetail.section.permissions.actionsRequired',
                                defaultMessage: 'actions 必填',
                              }),
                            },
                          ]}
                        >
                          <Select mode="tags" placeholder="invoke / execute" />
                        </Form.Item>
                      </Col>
                      <Col span={6}>
                        <Form.Item
                          {...field}
                          label="roles"
                          name={[field.name, 'roles']}
                          rules={[
                            {
                              required: true,
                              message: intl.formatMessage({
                                id: 'pages.functionsDetail.section.permissions.rolesRequired',
                                defaultMessage: 'roles 必填（至少 1 个）',
                              }),
                            },
                          ]}
                        >
                          <Select
                            mode="tags"
                            placeholder={intl.formatMessage({
                              id: 'pages.functionsDetail.section.permissions.rolesPlaceholder',
                              defaultMessage: '例如：ops / admin / functions:manage',
                            })}
                          />
                        </Form.Item>
                      </Col>
                      <Col span={3}>
                        <Form.Item {...field} label="gameId" name={[field.name, 'gameId']}>
                          <Input placeholder="(all)" />
                        </Form.Item>
                      </Col>
                      <Col span={3}>
                        <Form.Item {...field} label="env" name={[field.name, 'env']}>
                          <Input placeholder="(all)" />
                        </Form.Item>
                      </Col>
                    </Row>
                  </Card>
                ))}

                <Space>
                  <Button
                    onClick={() => add({ resource: 'function', actions: ['invoke'], roles: [] })}
                  >
                    <FormattedMessage
                      id="pages.functionsDetail.section.permissions.addRule"
                      defaultMessage="添加规则"
                    />
                  </Button>
                  <Button
                    type="primary"
                    loading={permSaving}
                    disabled={!functionId}
                    onClick={onSave}
                  >
                    <FormattedMessage
                      id="pages.functionsDetail.section.permissions.save"
                      defaultMessage="保存权限"
                    />
                  </Button>
                </Space>
              </Space>
            )}
          </Form.List>
        </Form>
      </Card>
    </>
  );
}
