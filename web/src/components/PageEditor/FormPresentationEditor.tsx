import { Button, Card, Col, Form, Input, Row, Select, Space, Switch, Tag, Typography } from 'antd';
import { DeleteOutlined, HolderOutlined } from '@ant-design/icons';
import { FormattedMessage, useIntl } from '@umijs/max';
import type {
  FormFieldSpec,
  FormGroupSpec,
  FormPresentationSpec,
  FormWidget,
} from '@/types/dashboard';
import { widgetCapabilityHint } from '@/utils/widgetCapabilities';
import LocalizedTextEditor from '@/components/LocalizedTextEditor';
import { SortableList } from '@/components/SortableList';
import FormVariantsEditor from './FormVariantsEditor';

const { Text } = Typography;

export interface FormPresentationEditorProps {
  value: FormPresentationSpec;
  onChange: (value: FormPresentationSpec) => void;
  readonly?: boolean;
}

function updateField(
  fields: FormFieldSpec[] | undefined,
  index: number,
  updates: Partial<FormFieldSpec>,
): FormFieldSpec[] {
  return (fields || []).map((field, currentIndex) =>
    currentIndex === index ? { ...field, ...updates } : field,
  );
}

/** 基础字段与风格覆盖（SUI-034）共用的控件候选集。 */
export function formWidgetChoices(
  intl: ReturnType<typeof useIntl>,
): Array<{ value: FormWidget; label: string }> {
  return [
    {
      value: 'Input',
      label: intl.formatMessage({
        id: 'component.pageEditor.formPresentation.widget.input',
        defaultMessage: '输入框',
      }),
    },
    {
      value: 'TextArea',
      label: intl.formatMessage({
        id: 'component.pageEditor.formPresentation.widget.textArea',
        defaultMessage: '多行文本',
      }),
    },
    {
      value: 'InputNumber',
      label: intl.formatMessage({
        id: 'component.pageEditor.formPresentation.widget.inputNumber',
        defaultMessage: '数字',
      }),
    },
    {
      value: 'Password',
      label: intl.formatMessage({
        id: 'component.pageEditor.formPresentation.widget.password',
        defaultMessage: '密码',
      }),
    },
    {
      value: 'Select',
      label: intl.formatMessage({
        id: 'component.pageEditor.formPresentation.widget.select',
        defaultMessage: '选择',
      }),
    },
    {
      value: 'MultiSelect',
      label: intl.formatMessage({
        id: 'component.pageEditor.formPresentation.widget.multiSelect',
        defaultMessage: '多选',
      }),
    },
    {
      value: 'Switch',
      label: intl.formatMessage({
        id: 'component.pageEditor.formPresentation.widget.switch',
        defaultMessage: '开关',
      }),
    },
    {
      value: 'DatePicker',
      label: intl.formatMessage({
        id: 'component.pageEditor.formPresentation.widget.datePicker',
        defaultMessage: '日期',
      }),
    },
    {
      value: 'DateRange',
      label: intl.formatMessage({
        id: 'component.pageEditor.formPresentation.widget.dateRange',
        defaultMessage: '日期范围',
      }),
    },
    {
      value: 'Upload',
      label: intl.formatMessage({
        id: 'component.pageEditor.formPresentation.widget.upload',
        defaultMessage: '上传',
      }),
    },
    {
      value: 'Array',
      label: intl.formatMessage({
        id: 'component.pageEditor.formPresentation.widget.array',
        defaultMessage: '数组',
      }),
    },
    {
      value: 'Object',
      label: intl.formatMessage({
        id: 'component.pageEditor.formPresentation.widget.object',
        defaultMessage: '对象',
      }),
    },
  ];
}

/** Edits only presentation metadata; JSON Schema and binding selectors remain immutable here.
 *  拖拽直接重排 fields 数组——渲染端按数组顺序生成 ui:order，保存后顺序即生效。 */
export default function FormPresentationEditor({
  value,
  onChange,
  readonly = false,
}: FormPresentationEditorProps) {
  const intl = useIntl();
  const fields = value.fields || [];
  const groups = value.groups || [];
  /** SUI-029：新建分组的 key 在当前草稿内唯一；已在用则顺延编号，避免
   *  服务端 groups.key 重复校验拦截。 */
  const nextGroupKey = (existing: FormGroupSpec[]): string => {
    const used = new Set(existing.map((group) => group.key));
    let index = existing.length + 1;
    let candidate = `group-${index}`;
    while (used.has(candidate)) {
      index += 1;
      candidate = `group-${index}`;
    }
    return candidate;
  };
  const updateGroup = (index: number, updates: Partial<FormGroupSpec>): FormGroupSpec[] =>
    groups.map((group, currentIndex) =>
      currentIndex === index ? { ...group, ...updates } : group,
    );
  const widgetOptions = formWidgetChoices(intl).map((option) => ({
    ...option,
    // SUI-020：原生 title 提示（无需额外弹层），值仍保持原有 12 项不变。
    title: widgetCapabilityHint(option.value),
  }));
  const applyField = (index: number, updates: Partial<FormFieldSpec>) =>
    onChange({ ...value, fields: updateField(value.fields, index, updates) });

  return (
    <Space orientation="vertical" size={12} style={{ width: '100%' }}>
      <Text type="secondary">
        <FormattedMessage
          id="component.pageEditor.formPresentation.hint"
          defaultMessage="字段来自函数 JSON Schema；这里只调整展示（拖动 ⠿ 调整顺序），不改变输入结构、binding 或 selector。"
        />
      </Text>
      <Form layout="vertical" disabled={readonly}>
        <Form.Item
          label={intl.formatMessage({
            id: 'component.pageEditor.formPresentation.layout',
            defaultMessage: '布局',
          })}
          style={{ marginBottom: 0 }}
        >
          <Select
            value={value.layout || 'vertical'}
            onChange={(layout) => onChange({ ...value, layout })}
            style={{ maxWidth: 200 }}
            options={[
              {
                value: 'vertical',
                label: intl.formatMessage({
                  id: 'component.pageEditor.formPresentation.layoutOption.vertical',
                  defaultMessage: '纵向',
                }),
              },
              {
                value: 'horizontal',
                label: intl.formatMessage({
                  id: 'component.pageEditor.formPresentation.layoutOption.horizontal',
                  defaultMessage: '横向',
                }),
              },
              {
                value: 'inline',
                label: intl.formatMessage({
                  id: 'component.pageEditor.formPresentation.layoutOption.inline',
                  defaultMessage: '行内',
                }),
              },
              {
                value: 'grid',
                label: intl.formatMessage({
                  id: 'component.pageEditor.formPresentation.layoutOption.grid',
                  defaultMessage: '网格',
                }),
              },
            ]}
          />
        </Form.Item>
      </Form>
      {fields.length === 0 ? (
        <Tag>
          <FormattedMessage
            id="component.pageEditor.formPresentation.noFields"
            defaultMessage="Schema 未生成可配置字段"
          />
        </Tag>
      ) : (
        <SortableList
          items={fields}
          getKey={(field) => field.key}
          onReorder={(next) => onChange({ ...value, fields: next })}
        >
          {(field, index, dragHandleProps) => (
            <Card
              size="small"
              title={
                <Space size={12} wrap>
                  {!readonly && (
                    <span {...dragHandleProps}>
                      <HolderOutlined />
                    </span>
                  )}
                  <Text code>{field.key}</Text>
                  <Select
                    size="small"
                    allowClear
                    placeholder={intl.formatMessage({
                      id: 'component.pageEditor.formPresentation.widgetPlaceholder',
                      defaultMessage: '组件',
                    })}
                    value={field.widget}
                    style={{ width: 110 }}
                    options={widgetOptions}
                    onChange={(widget) => applyField(index, { widget })}
                  />
                  <Space size={4}>
                    <Text type="secondary">
                      <FormattedMessage
                        id="component.pageEditor.formPresentation.field.visible"
                        defaultMessage="可见"
                      />
                    </Text>
                    <Switch
                      size="small"
                      checked={field.visible !== false}
                      onChange={(visible) => applyField(index, { visible })}
                    />
                  </Space>
                  <Space size={4}>
                    <Text type="secondary">
                      <FormattedMessage
                        id="component.pageEditor.formPresentation.field.disabled"
                        defaultMessage="禁用"
                      />
                    </Text>
                    <Switch
                      size="small"
                      checked={Boolean(field.disabled)}
                      onChange={(disabled) => applyField(index, { disabled })}
                    />
                  </Space>
                </Space>
              }
            >
              <Form layout="vertical" disabled={readonly} style={{ marginBottom: 0 }}>
                <Row gutter={12}>
                  <Col span={12}>
                    <Form.Item
                      label={intl.formatMessage({
                        id: 'component.pageEditor.formPresentation.field.label',
                        defaultMessage: '标签',
                      })}
                      style={{ marginBottom: 0 }}
                    >
                      <LocalizedTextEditor
                        value={field.label}
                        placeholder={intl.formatMessage({
                          id: 'component.pageEditor.formPresentation.field.labelPlaceholder',
                          defaultMessage: '字段标签',
                        })}
                        onChange={(label) => applyField(index, { label })}
                      />
                    </Form.Item>
                  </Col>
                  <Col span={12}>
                    <Form.Item
                      label={intl.formatMessage({
                        id: 'component.pageEditor.formPresentation.field.placeholder',
                        defaultMessage: '占位',
                      })}
                      style={{ marginBottom: 0 }}
                    >
                      <LocalizedTextEditor
                        value={field.placeholder}
                        placeholder={intl.formatMessage({
                          id: 'component.pageEditor.formPresentation.field.placeholderHint',
                          defaultMessage: '占位提示',
                        })}
                        onChange={(placeholder) => applyField(index, { placeholder })}
                      />
                    </Form.Item>
                  </Col>
                </Row>
              </Form>
            </Card>
          )}
        </SortableList>
      )}
      {/* SUI-029：分组编辑——创建/重命名/移动/删除、字段归属和折叠行为。
       * 分组保存到 FormPresentationSpec.groups，草稿重开原样回读（顺序即数组顺序）。 */}
      {fields.length > 0 && (
        <Card
          size="small"
          title={
            <FormattedMessage
              id="component.pageEditor.formPresentation.groups.title"
              defaultMessage="表单分组"
            />
          }
          extra={
            !readonly && (
              <Button
                size="small"
                type="link"
                data-testid="group-add"
                onClick={() =>
                  onChange({
                    ...value,
                    groups: [...groups, { key: nextGroupKey(groups), fields: [] }],
                  })
                }
              >
                <FormattedMessage
                  id="component.pageEditor.formPresentation.groups.add"
                  defaultMessage="新增分组"
                />
              </Button>
            )
          }
        >
          {groups.length === 0 ? (
            <Text type="secondary">
              <FormattedMessage
                id="component.pageEditor.formPresentation.groups.empty"
                defaultMessage="未配置分组；字段按表单顺序平铺"
              />
            </Text>
          ) : (
            <SortableList
              items={groups}
              getKey={(group) => group.key}
              onReorder={(next) => onChange({ ...value, groups: next })}
            >
              {(group, index, dragHandleProps) => (
                <Card
                  size="small"
                  style={{ marginBottom: 8 }}
                  title={
                    <Space size={8} wrap>
                      {!readonly && (
                        <span {...dragHandleProps}>
                          <HolderOutlined />
                        </span>
                      )}
                      <Text code data-testid={`group-key-${index}`}>
                        {group.key}
                      </Text>
                      <Switch
                        size="small"
                        checked={Boolean(group.collapsible)}
                        data-testid={`group-collapsible-${index}`}
                        onChange={(collapsible) =>
                          onChange({ ...value, groups: updateGroup(index, { collapsible }) })
                        }
                      />
                      <Text type="secondary">
                        <FormattedMessage
                          id="component.pageEditor.formPresentation.groups.collapsible"
                          defaultMessage="可折叠"
                        />
                      </Text>
                      {group.collapsible && (
                        <>
                          <Switch
                            size="small"
                            checked={Boolean(group.collapsed)}
                            data-testid={`group-collapsed-${index}`}
                            onChange={(collapsed) =>
                              onChange({ ...value, groups: updateGroup(index, { collapsed }) })
                            }
                          />
                          <Text type="secondary">
                            <FormattedMessage
                              id="component.pageEditor.formPresentation.groups.collapsed"
                              defaultMessage="默认折叠"
                            />
                          </Text>
                        </>
                      )}
                    </Space>
                  }
                  extra={
                    !readonly && (
                      <Button
                        size="small"
                        type="text"
                        danger
                        icon={<DeleteOutlined />}
                        data-testid={`group-delete-${index}`}
                        onClick={() =>
                          onChange({ ...value, groups: groups.filter((_, i) => i !== index) })
                        }
                      />
                    )
                  }
                >
                  <Form layout="vertical" disabled={readonly} style={{ marginBottom: 0 }}>
                    <Row gutter={12}>
                      <Col span={12}>
                        <Form.Item
                          label={intl.formatMessage({
                            id: 'component.pageEditor.formPresentation.groups.key',
                            defaultMessage: '分组标识',
                          })}
                          style={{ marginBottom: 0 }}
                        >
                          <Input
                            size="small"
                            value={group.key}
                            data-testid={`group-key-input-${index}`}
                            onChange={(event) =>
                              onChange({
                                ...value,
                                groups: updateGroup(index, { key: event.target.value }),
                              })
                            }
                          />
                        </Form.Item>
                      </Col>
                      <Col span={12}>
                        <Form.Item
                          label={intl.formatMessage({
                            id: 'component.pageEditor.formPresentation.groups.titleLabel',
                            defaultMessage: '分组标题（多语言）',
                          })}
                          style={{ marginBottom: 0 }}
                        >
                          <LocalizedTextEditor
                            value={group.title}
                            size="small"
                            placeholder={intl.formatMessage({
                              id: 'component.pageEditor.formPresentation.groups.titlePlaceholder',
                              defaultMessage: '分组标题',
                            })}
                            onChange={(title) =>
                              onChange({ ...value, groups: updateGroup(index, { title }) })
                            }
                          />
                        </Form.Item>
                      </Col>
                    </Row>
                    <Form.Item
                      label={intl.formatMessage({
                        id: 'component.pageEditor.formPresentation.groups.fields',
                        defaultMessage: '分组字段（按选择顺序，字段只属一个分组）',
                      })}
                      style={{ marginBottom: 0 }}
                    >
                      <Select
                        mode="multiple"
                        size="small"
                        data-testid={`group-fields-${index}`}
                        value={group.fields}
                        options={fields.map((field) => ({
                          value: field.key,
                          label: field.key,
                          // 已被其他分组认领的字段不可再选（服务端 form_group_field_conflict）
                          disabled: groups.some(
                            (other, otherIndex) =>
                              otherIndex !== index && other.fields.includes(field.key),
                          ),
                        }))}
                        onChange={(keys: string[]) =>
                          onChange({ ...value, groups: updateGroup(index, { fields: keys }) })
                        }
                      />
                    </Form.Item>
                  </Form>
                </Card>
              )}
            </SortableList>
          )}
        </Card>
      )}
      {/* SUI-034：命名展示风格管理——新增/改名/删除/设默认 + 逐风格预览。
          风格是纯展示覆盖（列宽/控件/布局），与绑定 selector（如「从选中行
          取 playerId」）是两类操作；重开草稿随 FormPresentationSpec 原样回读。 */}
      {fields.length > 0 && (
        <FormVariantsEditor value={value} onChange={onChange} readonly={readonly} />
      )}
    </Space>
  );
}
