import React from 'react';
import { Alert, Button, Card, Form, Input, Modal, Select, Space } from 'antd';
import type { FormInstance } from 'antd';
import { FormattedMessage, useIntl } from '@umijs/max';
import type {
  CapabilityKind,
  FunctionInfo,
  UpdateResourceSemanticsRequest,
} from '@/types/dashboard';
import { capabilityLabels } from './shared';

/** 编辑语义弹窗：identity/collection/lifecycle 绑定 + 动作/任务/报表三组 Form.List。
 * Form 实例由主页持有（预填在主页完成），函数选项来自当前资源的 functions。 */
const EditSemanticsModal: React.FC<{
  open: boolean;
  form: FormInstance<UpdateResourceSemanticsRequest>;
  functions: FunctionInfo[];
  onOk: () => void;
  onCancel: () => void;
}> = ({ open, form, functions, onOk, onCancel }) => {
  const intl = useIntl();

  const renderFunctionSelect = (capability: CapabilityKind, placeholder: string) => (
    <Select<number>
      allowClear
      placeholder={placeholder}
      showSearch
      optionFilterProp="label"
      options={(functions || [])
        .filter((fn) => fn.capability === capability)
        .map((fn) => ({
          value: fn.id,
          label: `${fn.functionId} #${fn.id}`,
          disabled: !fn.enabled,
        }))}
    />
  );

  const renderFunctionIdSelect = (
    placeholder: string,
    capability?: CapabilityKind,
    width = 260,
  ) => (
    <Select<string>
      allowClear
      placeholder={placeholder}
      showSearch
      optionFilterProp="label"
      style={{ width }}
      options={(functions || [])
        .filter((fn) => !capability || fn.capability === capability)
        .map((fn) => ({
          value: fn.functionId,
          label: `${fn.functionId} #${fn.id} / ${capabilityLabels[fn.capability] || fn.capability}`,
          disabled: !fn.enabled,
        }))}
    />
  );

  return (
    <Modal
      title={intl.formatMessage({
        id: 'pages.resourceCatalog.editSemantics.title',
        defaultMessage: '编辑语义',
      })}
      open={open}
      onOk={onOk}
      onCancel={onCancel}
      okText={intl.formatMessage({
        id: 'pages.resourceCatalog.editSemantics.button.save',
        defaultMessage: '保存',
      })}
      cancelText={intl.formatMessage({
        id: 'pages.resourceCatalog.editSemantics.button.cancel',
        defaultMessage: '取消',
      })}
      width={760}
    >
      <Alert
        type="warning"
        showIcon
        style={{ marginBottom: 16 }}
        message={intl.formatMessage({
          id: 'pages.resourceCatalog.editSemantics.alert.message',
          defaultMessage: '这里只补充能力语义，不编辑页面 UI',
        })}
        description={intl.formatMessage({
          id: 'pages.resourceCatalog.editSemantics.alert.description',
          defaultMessage:
            '选择当前资源下的函数数据库 ID 作为 lifecycle binding；保存后会记录 platform_review 来源、创建语义版本，并触发相关 Proposal 重算。',
        })}
      />
      <Form form={form} layout="vertical">
        <Form.Item
          label={intl.formatMessage({
            id: 'pages.resourceCatalog.editSemantics.form.identityField.label',
            defaultMessage: 'Identity 字段',
          })}
          name="identityField"
        >
          <Input
            placeholder={intl.formatMessage({
              id: 'pages.resourceCatalog.editSemantics.form.identityField.placeholder',
              defaultMessage: '例如: id, player_id',
            })}
          />
        </Form.Item>
        <Form.Item
          label={intl.formatMessage({
            id: 'pages.resourceCatalog.editSemantics.form.identityType.label',
            defaultMessage: 'Identity 类型',
          })}
          name="identityFieldType"
        >
          <Select
            options={[
              { value: 'string', label: 'string' },
              { value: 'number', label: 'number' },
              { value: 'integer', label: 'integer' },
              { value: 'boolean', label: 'boolean' },
            ]}
          />
        </Form.Item>
        <Form.Item
          label={intl.formatMessage({
            id: 'pages.resourceCatalog.editSemantics.form.identityPath.label',
            defaultMessage: 'Identity 路径',
          })}
          name="identityPath"
        >
          <Input
            placeholder={intl.formatMessage({
              id: 'pages.resourceCatalog.editSemantics.form.identityPath.placeholder',
              defaultMessage: '例如: id 或 /data/id',
            })}
          />
        </Form.Item>
        <Form.Item label="Collection Query" name="collectionQueryId">
          {renderFunctionSelect(
            'collection_query',
            intl.formatMessage({
              id: 'pages.resourceCatalog.editSemantics.form.collectionQuery.placeholder',
              defaultMessage: '选择列表查询函数',
            }),
          )}
        </Form.Item>
        <Form.Item
          label={intl.formatMessage({
            id: 'pages.resourceCatalog.editSemantics.form.collectionPath.label',
            defaultMessage: 'Collection 路径',
          })}
          name="collectionPath"
        >
          <Input
            placeholder={intl.formatMessage({
              id: 'pages.resourceCatalog.editSemantics.form.collectionPath.placeholder',
              defaultMessage: '例如: /players',
            })}
          />
        </Form.Item>
        <Form.Item
          label={intl.formatMessage({
            id: 'pages.resourceCatalog.editSemantics.form.pageFieldName.label',
            defaultMessage: '分页字段',
          })}
          name="pageFieldName"
        >
          <Input
            placeholder={intl.formatMessage({
              id: 'pages.resourceCatalog.editSemantics.form.pageFieldName.placeholder',
              defaultMessage: '默认 page',
            })}
          />
        </Form.Item>
        <Form.Item
          label={intl.formatMessage({
            id: 'pages.resourceCatalog.editSemantics.form.pageSizeFieldName.label',
            defaultMessage: '分页大小字段',
          })}
          name="pageSizeFieldName"
        >
          <Input
            placeholder={intl.formatMessage({
              id: 'pages.resourceCatalog.editSemantics.form.pageSizeFieldName.placeholder',
              defaultMessage: '默认 page_size',
            })}
          />
        </Form.Item>
        <Form.Item
          label={intl.formatMessage({
            id: 'pages.resourceCatalog.editSemantics.form.itemsFieldName.label',
            defaultMessage: 'Items 字段',
          })}
          name="itemsFieldName"
        >
          <Input
            placeholder={intl.formatMessage({
              id: 'pages.resourceCatalog.editSemantics.form.itemsFieldName.placeholder',
              defaultMessage: '默认 items',
            })}
          />
        </Form.Item>
        <Form.Item
          label={intl.formatMessage({
            id: 'pages.resourceCatalog.editSemantics.form.totalFieldName.label',
            defaultMessage: 'Total 字段',
          })}
          name="totalFieldName"
        >
          <Input
            placeholder={intl.formatMessage({
              id: 'pages.resourceCatalog.editSemantics.form.totalFieldName.placeholder',
              defaultMessage: '默认 total',
            })}
          />
        </Form.Item>
        <Form.Item label="Item Query" name="itemQueryId">
          {renderFunctionSelect(
            'item_query',
            intl.formatMessage({
              id: 'pages.resourceCatalog.editSemantics.form.itemQuery.placeholder',
              defaultMessage: '选择详情查询函数',
            }),
          )}
        </Form.Item>
        <Form.Item
          label={intl.formatMessage({
            id: 'pages.resourceCatalog.editSemantics.form.itemPath.label',
            defaultMessage: 'Item 路径',
          })}
          name="itemPath"
        >
          <Input
            placeholder={intl.formatMessage({
              id: 'pages.resourceCatalog.editSemantics.form.itemPath.placeholder',
              defaultMessage: "例如: /players/'{'player_id'}'",
            })}
          />
        </Form.Item>
        <Form.Item label="Create" name="createId">
          {renderFunctionSelect(
            'create',
            intl.formatMessage({
              id: 'pages.resourceCatalog.editSemantics.form.create.placeholder',
              defaultMessage: '选择创建函数',
            }),
          )}
        </Form.Item>
        <Form.Item label="Update" name="updateId">
          {renderFunctionSelect(
            'update',
            intl.formatMessage({
              id: 'pages.resourceCatalog.editSemantics.form.update.placeholder',
              defaultMessage: '选择更新函数',
            }),
          )}
        </Form.Item>
        <Form.Item label="Delete" name="deleteId">
          {renderFunctionSelect(
            'delete',
            intl.formatMessage({
              id: 'pages.resourceCatalog.editSemantics.form.delete.placeholder',
              defaultMessage: '选择删除函数',
            }),
          )}
        </Form.Item>
        <Form.List name="actions">
          {(fields, { add, remove }) => (
            <Card
              size="small"
              title={intl.formatMessage({
                id: 'pages.resourceCatalog.editSemantics.actionList.title',
                defaultMessage: '资源动作语义',
              })}
              style={{ marginBottom: 16 }}
              extra={
                <Button size="small" onClick={() => add({ subject: 'resource_item' })}>
                  <FormattedMessage
                    id="pages.resourceCatalog.editSemantics.actionList.add"
                    defaultMessage="添加动作"
                  />
                </Button>
              }
            >
              <Alert
                type="info"
                showIcon
                style={{ marginBottom: 12 }}
                message={intl.formatMessage({
                  id: 'pages.resourceCatalog.editSemantics.actionList.alert.message',
                  defaultMessage: '这里只描述动作需要的资源上下文',
                })}
                description={intl.formatMessage({
                  id: 'pages.resourceCatalog.editSemantics.actionList.alert.description',
                  defaultMessage:
                    'subject 决定动作针对单行、选中集合或整个资源；按钮位置由 PageProposal 生成器决定，不在这里配置。',
                })}
              />
              {fields.map((field) => (
                <Space
                  key={field.key}
                  align="baseline"
                  wrap
                  style={{ display: 'flex', marginBottom: 8 }}
                >
                  <Form.Item
                    {...field}
                    label={intl.formatMessage({
                      id: 'pages.resourceCatalog.editSemantics.actionList.function.label',
                      defaultMessage: '函数',
                    })}
                    name={[field.name, 'functionId']}
                    rules={[
                      {
                        required: true,
                        message: intl.formatMessage({
                          id: 'pages.resourceCatalog.editSemantics.actionList.function.required',
                          defaultMessage: '请选择 action 函数',
                        }),
                      },
                    ]}
                  >
                    <Select<string>
                      style={{ width: 260 }}
                      placeholder={intl.formatMessage({
                        id: 'pages.resourceCatalog.editSemantics.actionList.function.placeholder',
                        defaultMessage: '选择 action 函数',
                      })}
                      showSearch
                      optionFilterProp="label"
                      options={(functions || [])
                        .filter((fn) => fn.capability === 'action')
                        .map((fn) => ({
                          value: fn.functionId,
                          label: `${fn.functionId} #${fn.id}`,
                          disabled: !fn.enabled,
                        }))}
                    />
                  </Form.Item>
                  <Form.Item
                    {...field}
                    label="Subject"
                    name={[field.name, 'subject']}
                    rules={[
                      {
                        required: true,
                        message: intl.formatMessage({
                          id: 'pages.resourceCatalog.editSemantics.actionList.subject.required',
                          defaultMessage: '请选择 subject',
                        }),
                      },
                    ]}
                  >
                    <Select
                      style={{ width: 180 }}
                      options={[
                        {
                          value: 'resource_item',
                          label: intl.formatMessage({
                            id: 'pages.resourceCatalog.editSemantics.actionList.subject.resourceItem',
                            defaultMessage: '单个资源对象',
                          }),
                        },
                        {
                          value: 'resource_selection',
                          label: intl.formatMessage({
                            id: 'pages.resourceCatalog.editSemantics.actionList.subject.resourceSelection',
                            defaultMessage: '选中资源集合',
                          }),
                        },
                        {
                          value: 'none',
                          label: intl.formatMessage({
                            id: 'pages.resourceCatalog.editSemantics.actionList.subject.none',
                            defaultMessage: '整个资源',
                          }),
                        },
                      ]}
                    />
                  </Form.Item>
                  <Form.Item
                    {...field}
                    label="Identity Input"
                    name={[field.name, 'identityInput']}
                    tooltip={intl.formatMessage({
                      id: 'pages.resourceCatalog.editSemantics.actionList.identityInput.tooltip',
                      defaultMessage:
                        'resource_item/resource_selection 必填，例如 /playerId 或 /playerIds；none 可留空',
                    })}
                  >
                    <Input
                      style={{ width: 200 }}
                      placeholder={intl.formatMessage({
                        id: 'pages.resourceCatalog.editSemantics.actionList.identityInput.placeholder',
                        defaultMessage: '/playerId 或 /playerIds',
                      })}
                    />
                  </Form.Item>
                  <Button danger onClick={() => remove(field.name)}>
                    <FormattedMessage
                      id="pages.resourceCatalog.editSemantics.actionList.remove"
                      defaultMessage="删除"
                    />
                  </Button>
                </Space>
              ))}
            </Card>
          )}
        </Form.List>
        <Form.List name="tasks">
          {(fields, { add, remove }) => (
            <Card
              size="small"
              title={intl.formatMessage({
                id: 'pages.resourceCatalog.editSemantics.taskList.title',
                defaultMessage: '任务语义',
              })}
              style={{ marginBottom: 16 }}
              extra={
                <Button
                  size="small"
                  onClick={() =>
                    add({
                      taskId: { resultPath: '/taskId', valueType: 'string' },
                      status: { taskIdInput: '/taskId', statePath: '/status' },
                    })
                  }
                >
                  <FormattedMessage
                    id="pages.resourceCatalog.editSemantics.taskList.add"
                    defaultMessage="添加任务"
                  />
                </Button>
              }
            >
              <Alert
                type="info"
                showIcon
                style={{ marginBottom: 12 }}
                message={intl.formatMessage({
                  id: 'pages.resourceCatalog.editSemantics.taskList.alert.message',
                  defaultMessage: '这里只描述任务生命周期能力',
                })}
                description={intl.formatMessage({
                  id: 'pages.resourceCatalog.editSemantics.taskList.alert.description',
                  defaultMessage:
                    'start 必须是 task 能力函数；status/events/result/cancel 只声明真实函数和 taskId 输入路径，不配置页面按钮位置。当前平台没有 retry runtime，因此不提供重试语义录入。',
                })}
              />
              {fields.map((field) => (
                <Card key={field.key} size="small" style={{ marginBottom: 12 }}>
                  <Space align="baseline" wrap style={{ display: 'flex' }}>
                    <Form.Item
                      label={intl.formatMessage({
                        id: 'pages.resourceCatalog.editSemantics.taskList.start.label',
                        defaultMessage: 'Start 函数',
                      })}
                      name={[field.name, 'start', 'functionId']}
                      rules={[
                        {
                          required: true,
                          message: intl.formatMessage({
                            id: 'pages.resourceCatalog.editSemantics.taskList.start.required',
                            defaultMessage: '请选择 task start 函数',
                          }),
                        },
                      ]}
                    >
                      {renderFunctionIdSelect(
                        intl.formatMessage({
                          id: 'pages.resourceCatalog.editSemantics.taskList.taskFunction.placeholder',
                          defaultMessage: '选择 task 函数',
                        }),
                        'task',
                        300,
                      )}
                    </Form.Item>
                    <Form.Item
                      label="TaskID Result Path"
                      name={[field.name, 'taskId', 'resultPath']}
                      rules={[
                        {
                          required: true,
                          message: intl.formatMessage({
                            id: 'pages.resourceCatalog.editSemantics.taskList.taskIdResultPath.required',
                            defaultMessage: '请输入 taskId 输出路径',
                          }),
                        },
                      ]}
                    >
                      <Input
                        style={{ width: 180 }}
                        placeholder={intl.formatMessage({
                          id: 'pages.resourceCatalog.editSemantics.taskList.taskIdResultPath.placeholder',
                          defaultMessage: '/taskId 或空根路径',
                        })}
                      />
                    </Form.Item>
                    <Form.Item
                      label={intl.formatMessage({
                        id: 'pages.resourceCatalog.editSemantics.taskList.taskIdType.label',
                        defaultMessage: 'TaskID 类型',
                      })}
                      name={[field.name, 'taskId', 'valueType']}
                      rules={[
                        {
                          required: true,
                          message: intl.formatMessage({
                            id: 'pages.resourceCatalog.editSemantics.taskList.taskIdType.required',
                            defaultMessage: '请选择 taskId 类型',
                          }),
                        },
                      ]}
                    >
                      <Select
                        style={{ width: 140 }}
                        options={[
                          { value: 'string', label: 'string' },
                          { value: 'number', label: 'number' },
                          { value: 'integer', label: 'integer' },
                          { value: 'boolean', label: 'boolean' },
                        ]}
                      />
                    </Form.Item>
                  </Space>
                  <Space align="baseline" wrap style={{ display: 'flex' }}>
                    <Form.Item
                      label={intl.formatMessage({
                        id: 'pages.resourceCatalog.editSemantics.taskList.status.label',
                        defaultMessage: 'Status 函数',
                      })}
                      name={[field.name, 'status', 'function', 'functionId']}
                      rules={[
                        {
                          required: true,
                          message: intl.formatMessage({
                            id: 'pages.resourceCatalog.editSemantics.taskList.status.required',
                            defaultMessage: '请选择 status 函数',
                          }),
                        },
                      ]}
                    >
                      {renderFunctionIdSelect(
                        intl.formatMessage({
                          id: 'pages.resourceCatalog.editSemantics.taskList.statusFunction.placeholder',
                          defaultMessage: '选择 status 函数',
                        }),
                        undefined,
                        300,
                      )}
                    </Form.Item>
                    <Form.Item
                      label="Status TaskID Input"
                      name={[field.name, 'status', 'taskIdInput']}
                      rules={[
                        {
                          required: true,
                          message: intl.formatMessage({
                            id: 'pages.resourceCatalog.editSemantics.taskList.statusTaskIdInput.required',
                            defaultMessage: '请输入 status taskId 输入路径',
                          }),
                        },
                      ]}
                    >
                      <Input style={{ width: 180 }} placeholder="/taskId" />
                    </Form.Item>
                    <Form.Item
                      label="State Path"
                      name={[field.name, 'status', 'statePath']}
                      rules={[
                        {
                          required: true,
                          message: intl.formatMessage({
                            id: 'pages.resourceCatalog.editSemantics.taskList.statePath.required',
                            defaultMessage: '请输入状态输出路径',
                          }),
                        },
                      ]}
                    >
                      <Input style={{ width: 160 }} placeholder="/status" />
                    </Form.Item>
                  </Space>
                  <Space align="baseline" wrap style={{ display: 'flex' }}>
                    <Form.Item
                      label={intl.formatMessage({
                        id: 'pages.resourceCatalog.editSemantics.taskList.events.label',
                        defaultMessage: 'Events 函数',
                      })}
                      name={[field.name, 'events', 'function', 'functionId']}
                    >
                      {renderFunctionIdSelect(
                        intl.formatMessage({
                          id: 'pages.resourceCatalog.editSemantics.taskList.eventsFunction.placeholder',
                          defaultMessage: '选择 events 函数',
                        }),
                      )}
                    </Form.Item>
                    <Form.Item
                      label="Events TaskID Input"
                      name={[field.name, 'events', 'taskIdInput']}
                    >
                      <Input style={{ width: 160 }} placeholder="/taskId" />
                    </Form.Item>
                    <Form.Item label="Events Path" name={[field.name, 'events', 'eventsPath']}>
                      <Input style={{ width: 160 }} placeholder="/events" />
                    </Form.Item>
                  </Space>
                  <Space align="baseline" wrap style={{ display: 'flex' }}>
                    <Form.Item
                      label={intl.formatMessage({
                        id: 'pages.resourceCatalog.editSemantics.taskList.result.label',
                        defaultMessage: 'Result 函数',
                      })}
                      name={[field.name, 'result', 'function', 'functionId']}
                    >
                      {renderFunctionIdSelect(
                        intl.formatMessage({
                          id: 'pages.resourceCatalog.editSemantics.taskList.resultFunction.placeholder',
                          defaultMessage: '选择 result 函数',
                        }),
                      )}
                    </Form.Item>
                    <Form.Item
                      label="Result TaskID Input"
                      name={[field.name, 'result', 'taskIdInput']}
                    >
                      <Input style={{ width: 160 }} placeholder="/taskId" />
                    </Form.Item>
                    <Form.Item label="Result Path" name={[field.name, 'result', 'resultPath']}>
                      <Input style={{ width: 160 }} placeholder="/result" />
                    </Form.Item>
                  </Space>
                  <Space align="baseline" wrap style={{ display: 'flex' }}>
                    <Form.Item
                      label={intl.formatMessage({
                        id: 'pages.resourceCatalog.editSemantics.taskList.cancel.label',
                        defaultMessage: 'Cancel 函数',
                      })}
                      name={[field.name, 'cancel', 'function', 'functionId']}
                    >
                      {renderFunctionIdSelect(
                        intl.formatMessage({
                          id: 'pages.resourceCatalog.editSemantics.taskList.cancelFunction.placeholder',
                          defaultMessage: '选择 cancel 函数',
                        }),
                      )}
                    </Form.Item>
                    <Form.Item
                      label="Cancel TaskID Input"
                      name={[field.name, 'cancel', 'taskIdInput']}
                    >
                      <Input style={{ width: 160 }} placeholder="/taskId" />
                    </Form.Item>
                  </Space>
                  <Button danger onClick={() => remove(field.name)}>
                    <FormattedMessage
                      id="pages.resourceCatalog.editSemantics.taskList.remove"
                      defaultMessage="删除任务语义"
                    />
                  </Button>
                </Card>
              ))}
            </Card>
          )}
        </Form.List>
        <Form.List name="reports">
          {(fields, { add, remove }) => (
            <Card
              size="small"
              title={intl.formatMessage({
                id: 'pages.resourceCatalog.editSemantics.reportList.title',
                defaultMessage: '报表语义',
              })}
              style={{ marginBottom: 16 }}
              extra={
                <Button
                  size="small"
                  onClick={() => add({ datasetPath: '/dataset', dimensions: [], metrics: [] })}
                >
                  <FormattedMessage
                    id="pages.resourceCatalog.editSemantics.reportList.add"
                    defaultMessage="添加报表"
                  />
                </Button>
              }
            >
              <Alert
                type="info"
                showIcon
                style={{ marginBottom: 12 }}
                message={intl.formatMessage({
                  id: 'pages.resourceCatalog.editSemantics.reportList.alert.message',
                  defaultMessage: '这里只描述报表数据集',
                })}
                description={intl.formatMessage({
                  id: 'pages.resourceCatalog.editSemantics.reportList.alert.description',
                  defaultMessage:
                    'datasetPath 指向查询结果中的数组；dimensions/metrics 是相对 dataset item 的 JSON Pointer。图表类型和表格展示属于 Page Proposal/Page Studio。',
                })}
              />
              {fields.map((field) => (
                <Space
                  key={field.key}
                  align="baseline"
                  wrap
                  style={{ display: 'flex', marginBottom: 8 }}
                >
                  <Form.Item
                    label={intl.formatMessage({
                      id: 'pages.resourceCatalog.editSemantics.reportList.query.label',
                      defaultMessage: 'Query 函数',
                    })}
                    name={[field.name, 'query', 'functionId']}
                    rules={[
                      {
                        required: true,
                        message: intl.formatMessage({
                          id: 'pages.resourceCatalog.editSemantics.reportList.query.required',
                          defaultMessage: '请选择 report 函数',
                        }),
                      },
                    ]}
                  >
                    {renderFunctionIdSelect(
                      intl.formatMessage({
                        id: 'pages.resourceCatalog.editSemantics.reportList.query.placeholder',
                        defaultMessage: '选择 report 函数',
                      }),
                      'report',
                      300,
                    )}
                  </Form.Item>
                  <Form.Item
                    label="Dataset Path"
                    name={[field.name, 'datasetPath']}
                    tooltip={intl.formatMessage({
                      id: 'pages.resourceCatalog.editSemantics.reportList.datasetPath.tooltip',
                      defaultMessage: '根数组可留空；对象字段数组示例 /dataset 或 /data/items',
                    })}
                  >
                    <Input
                      style={{ width: 180 }}
                      placeholder={intl.formatMessage({
                        id: 'pages.resourceCatalog.editSemantics.reportList.datasetPath.placeholder',
                        defaultMessage: '/dataset 或空根路径',
                      })}
                    />
                  </Form.Item>
                  <Form.Item
                    label="Dimensions"
                    name={[field.name, 'dimensions']}
                    rules={[
                      {
                        required: true,
                        message: intl.formatMessage({
                          id: 'pages.resourceCatalog.editSemantics.reportList.dimensions.required',
                          defaultMessage: '至少填写一个维度指针',
                        }),
                      },
                    ]}
                  >
                    <Select<string>
                      mode="tags"
                      style={{ width: 260 }}
                      tokenSeparators={[',']}
                      placeholder="/date, /channel"
                    />
                  </Form.Item>
                  <Form.Item
                    label="Metrics"
                    name={[field.name, 'metrics']}
                    rules={[
                      {
                        required: true,
                        message: intl.formatMessage({
                          id: 'pages.resourceCatalog.editSemantics.reportList.metrics.required',
                          defaultMessage: '至少填写一个指标指针',
                        }),
                      },
                    ]}
                  >
                    <Select<string>
                      mode="tags"
                      style={{ width: 260 }}
                      tokenSeparators={[',']}
                      placeholder="/payAmount, /userCount"
                    />
                  </Form.Item>
                  <Button danger onClick={() => remove(field.name)}>
                    <FormattedMessage
                      id="pages.resourceCatalog.editSemantics.reportList.remove"
                      defaultMessage="删除"
                    />
                  </Button>
                </Space>
              ))}
            </Card>
          )}
        </Form.List>
        <Form.Item
          label={intl.formatMessage({
            id: 'pages.resourceCatalog.editSemantics.form.changeReason.label',
            defaultMessage: '变更原因',
          })}
          name="changeReason"
        >
          <Input.TextArea
            placeholder={intl.formatMessage({
              id: 'pages.resourceCatalog.editSemantics.form.changeReason.placeholder',
              defaultMessage: '说明变更原因',
            })}
          />
        </Form.Item>
      </Form>
    </Modal>
  );
};

export default EditSemanticsModal;
