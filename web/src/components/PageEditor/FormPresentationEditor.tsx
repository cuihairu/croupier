import { Card, Col, Form, Row, Select, Space, Switch, Tag, Typography } from 'antd';
import { HolderOutlined } from '@ant-design/icons';
import { FormattedMessage, useIntl } from '@umijs/max';
import type { FormFieldSpec, FormPresentationSpec, FormWidget } from '@/types/dashboard';
import LocalizedTextEditor from '@/components/LocalizedTextEditor';
import { SortableList } from '@/components/SortableList';

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

/** Edits only presentation metadata; JSON Schema and binding selectors remain immutable here.
 *  拖拽直接重排 fields 数组——渲染端按数组顺序生成 ui:order，保存后顺序即生效。 */
export default function FormPresentationEditor({
  value,
  onChange,
  readonly = false,
}: FormPresentationEditorProps) {
  const intl = useIntl();
  const fields = value.fields || [];
  const widgetOptions: Array<{ value: FormWidget; label: string }> = [
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
    </Space>
  );
}
