import {
  Button,
  Card,
  Col,
  Form,
  Input,
  InputNumber,
  Row,
  Select,
  Space,
  Tag,
  Typography,
} from 'antd';
import { DeleteOutlined, EyeOutlined } from '@ant-design/icons';
import { FormattedMessage, useIntl } from '@umijs/max';
import { useState } from 'react';
import type {
  FormPresentationSpec,
  FormVariantFieldSpec,
  FormVariantSpec,
  FormWidget,
} from '@/types/dashboard';
import { resolveVariantPresentation } from '@/utils/formVariants';
import LocalizedTextEditor from '@/components/LocalizedTextEditor';
import SchemaFormRenderer from '@/components/SchemaFormRenderer';
import { formWidgetChoices } from './FormPresentationEditor';

const { Text } = Typography;

export interface FormVariantsEditorProps {
  value: FormPresentationSpec;
  onChange: (value: FormPresentationSpec) => void;
  readonly?: boolean;
}

/** SUI-034：命名展示风格管理。新增/改名/删除/设默认 + 逐风格预览。
 *  风格只覆盖纯展示属性（布局、列宽、控件、文案），绑定与 selector 不在
 *  风格里配置——「从选中行取 playerId」属于绑定操作，走 bindings.selectors。 */
export default function FormVariantsEditor({
  value,
  onChange,
  readonly = false,
}: FormVariantsEditorProps) {
  const intl = useIntl();
  const [previewKey, setPreviewKey] = useState<string>('');
  const variants = value.variants || [];
  const baseFields = value.fields || [];
  const widgetOptions = formWidgetChoices(intl);

  const setVariants = (next: FormVariantSpec[]) => onChange({ ...value, variants: next });
  const updateVariant = (index: number, updates: Partial<FormVariantSpec>) =>
    setVariants(variants.map((item, i) => (i === index ? { ...item, ...updates } : item)));

  /** 新建风格 key 在当前表单内唯一（SUI-033 form_variant_key_conflict 前置）。 */
  const nextVariantKey = (): string => {
    const used = new Set(variants.map((item) => item.key));
    let index = variants.length + 1;
    let candidate = `variant-${index}`;
    while (used.has(candidate)) {
      index += 1;
      candidate = `variant-${index}`;
    }
    return candidate;
  };

  const addVariant = () => setVariants([...variants, { key: nextVariantKey(), fields: [] }]);

  const removeVariant = (index: number) => {
    const removed = variants[index];
    const next = variants.filter((_, i) => i !== index);
    const nextDefault = value.defaultVariant === removed?.key ? '' : value.defaultVariant;
    onChange({ ...value, variants: next, defaultVariant: nextDefault || undefined });
  };

  /** 改名同步 defaultVariant——默认项永远解析到已声明 key（SUI-033）。 */
  const renameVariant = (index: number, key: string) => {
    const previous = variants[index]?.key;
    const wasDefault = previous !== undefined && value.defaultVariant === previous;
    onChange({
      ...value,
      variants: variants.map((item, i) => (i === index ? { ...item, key } : item)),
      defaultVariant: wasDefault ? key || undefined : value.defaultVariant,
    });
  };

  const toggleDefault = (index: number) => {
    const key = variants[index]?.key ?? '';
    const next = value.defaultVariant === key ? '' : key;
    onChange({ ...value, defaultVariant: next || undefined });
  };

  /** 逐字段展示覆盖：width/widget。清空 = 继承基础（显式删除键，不留
   *  undefined 值属性——merge spread 会以 undefined 覆盖基础值）；条目
   *  失去全部覆盖属性时整条移除（key-only 空条目对协议合法但无编辑意义）。
   *  API 导入的变体可能携带 label 等其他展示覆盖，保留、不丢。 */
  const updateVariantField = (
    variantIndex: number,
    fieldKey: string,
    updates: Partial<FormVariantFieldSpec>,
  ) => {
    const variant = variants[variantIndex];
    if (!variant) return;
    const fields = [...(variant.fields || [])];
    const at = fields.findIndex((item) => item.key === fieldKey);
    const patch = (target: FormVariantFieldSpec): FormVariantFieldSpec => {
      const next: Record<string, unknown> = { ...target };
      for (const [prop, val] of Object.entries(updates)) {
        if (val === undefined) delete next[prop];
        else next[prop] = val;
      }
      return next as unknown as FormVariantFieldSpec;
    };
    if (at < 0) {
      fields.push(patch({ key: fieldKey }));
    } else {
      fields[at] = patch(fields[at]);
    }
    updateVariant(variantIndex, {
      fields: fields.filter((item) => Object.keys(item).some((prop) => prop !== 'key')),
    });
  };

  const clearVariantFieldOverride = (variantIndex: number, fieldKey: string) => {
    const variant = variants[variantIndex];
    if (!variant) return;
    updateVariant(variantIndex, {
      fields: (variant.fields || []).filter((item) => item.key !== fieldKey),
    });
  };

  const variantField = (
    variant: FormVariantSpec,
    fieldKey: string,
  ): FormVariantFieldSpec | undefined =>
    (variant.fields || []).find((item) => item.key === fieldKey);

  return (
    <Card
      size="small"
      title={
        <FormattedMessage
          id="component.pageEditor.formPresentation.variants.title"
          defaultMessage="命名展示风格"
        />
      }
      extra={
        !readonly && (
          <Button size="small" type="link" data-testid="variant-add" onClick={addVariant}>
            <FormattedMessage
              id="component.pageEditor.formPresentation.variants.add"
              defaultMessage="新增风格"
            />
          </Button>
        )
      }
    >
      <Space orientation="vertical" size={8} style={{ width: '100%' }}>
        <Text type="secondary">
          <FormattedMessage
            id="component.pageEditor.formPresentation.variants.hint"
            defaultMessage="风格只调整展示（布局、列宽、控件、文案）；取数与提交结构由绑定决定（如从选中行取 playerId 在绑定 selector 里配置），任何风格都不改变调用。"
          />
        </Text>
        {variants.length === 0 ? (
          <Text type="secondary">
            <FormattedMessage
              id="component.pageEditor.formPresentation.variants.empty"
              defaultMessage="未配置风格；表单按基础呈现渲染"
            />
          </Text>
        ) : (
          variants.map((variant, index) => {
            const isDefault = value.defaultVariant === variant.key;
            const merged = resolveVariantPresentation(value, variant.key);
            return (
              <Card
                size="small"
                key={variant.key || index}
                style={{ marginBottom: 8 }}
                title={
                  <Space size={8} wrap>
                    <Text code data-testid={`variant-key-${index}`}>
                      {variant.key}
                    </Text>
                    {isDefault && (
                      <Tag color="blue" data-testid={`variant-default-tag-${index}`}>
                        <FormattedMessage
                          id="component.pageEditor.formPresentation.variants.defaultTag"
                          defaultMessage="默认"
                        />
                      </Tag>
                    )}
                  </Space>
                }
                extra={
                  <Space size={4}>
                    {!readonly && (
                      <>
                        <Button
                          size="small"
                          type="link"
                          data-testid={`variant-set-default-${index}`}
                          onClick={() => toggleDefault(index)}
                        >
                          {isDefault ? (
                            <FormattedMessage
                              id="component.pageEditor.formPresentation.variants.unsetDefault"
                              defaultMessage="取消默认"
                            />
                          ) : (
                            <FormattedMessage
                              id="component.pageEditor.formPresentation.variants.setDefault"
                              defaultMessage="设为默认"
                            />
                          )}
                        </Button>
                        <Button
                          size="small"
                          type="text"
                          danger
                          icon={<DeleteOutlined />}
                          data-testid={`variant-delete-${index}`}
                          onClick={() => removeVariant(index)}
                        />
                      </>
                    )}
                    <Button
                      size="small"
                      type="text"
                      icon={<EyeOutlined />}
                      data-testid={`variant-preview-${index}`}
                      onClick={() => setPreviewKey(previewKey === variant.key ? '' : variant.key)}
                    />
                  </Space>
                }
              >
                <Form layout="vertical" disabled={readonly} style={{ marginBottom: 0 }}>
                  <Row gutter={12}>
                    <Col span={12}>
                      <Form.Item
                        label={intl.formatMessage({
                          id: 'component.pageEditor.formPresentation.variants.key',
                          defaultMessage: '风格标识',
                        })}
                        style={{ marginBottom: 0 }}
                      >
                        <Input
                          size="small"
                          value={variant.key}
                          data-testid={`variant-key-input-${index}`}
                          onChange={(event) => renameVariant(index, event.target.value)}
                        />
                      </Form.Item>
                    </Col>
                    <Col span={12}>
                      <Form.Item
                        label={intl.formatMessage({
                          id: 'component.pageEditor.formPresentation.variants.titleLabel',
                          defaultMessage: '风格名称（多语言）',
                        })}
                        style={{ marginBottom: 0 }}
                      >
                        <LocalizedTextEditor
                          value={variant.title}
                          size="small"
                          placeholder={intl.formatMessage({
                            id: 'component.pageEditor.formPresentation.variants.titlePlaceholder',
                            defaultMessage: '风格名称',
                          })}
                          onChange={(title) => updateVariant(index, { title })}
                        />
                      </Form.Item>
                    </Col>
                  </Row>
                  <Form.Item
                    label={intl.formatMessage({
                      id: 'component.pageEditor.formPresentation.variants.layout',
                      defaultMessage: '布局（未选继承基础）',
                    })}
                    style={{ marginBottom: 0 }}
                  >
                    <Select
                      size="small"
                      allowClear
                      data-testid={`variant-layout-${index}`}
                      value={variant.layout}
                      style={{ maxWidth: 200 }}
                      placeholder={intl.formatMessage({
                        id: 'component.pageEditor.formPresentation.variants.layoutPlaceholder',
                        defaultMessage: '继承基础',
                      })}
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
                      onChange={(layout) => updateVariant(index, { layout: layout || undefined })}
                    />
                  </Form.Item>
                  {baseFields.length > 0 && (
                    <Form.Item
                      label={intl.formatMessage({
                        id: 'component.pageEditor.formPresentation.variants.fields',
                        defaultMessage: '字段展示覆盖（列宽 1-12；清空 = 继承基础）',
                      })}
                      style={{ marginBottom: 0 }}
                    >
                      <Space orientation="vertical" size={4} style={{ width: '100%' }}>
                        {baseFields.map((field) => {
                          const override = variantField(variant, field.key);
                          return (
                            <Space key={field.key} size={8} wrap>
                              <Text code style={{ minWidth: 80 }}>
                                {field.key}
                              </Text>
                              <InputNumber
                                size="small"
                                min={1}
                                max={12}
                                data-testid={`variant-field-width-${index}-${field.key}`}
                                placeholder={intl.formatMessage({
                                  id: 'component.pageEditor.formPresentation.variants.widthPlaceholder',
                                  defaultMessage: '列宽',
                                })}
                                value={override?.width}
                                style={{ width: 90 }}
                                onChange={(width) =>
                                  updateVariantField(index, field.key, {
                                    width: typeof width === 'number' ? width : undefined,
                                  })
                                }
                              />
                              <Select
                                size="small"
                                allowClear
                                data-testid={`variant-field-widget-${index}-${field.key}`}
                                placeholder={intl.formatMessage({
                                  id: 'component.pageEditor.formPresentation.variants.widgetPlaceholder',
                                  defaultMessage: '控件',
                                })}
                                value={override?.widget}
                                style={{ width: 110 }}
                                options={widgetOptions}
                                onChange={(widget) =>
                                  updateVariantField(index, field.key, {
                                    widget: (widget as FormWidget | undefined) || undefined,
                                  })
                                }
                              />
                              {override && (
                                <Button
                                  size="small"
                                  type="link"
                                  data-testid={`variant-field-clear-${index}-${field.key}`}
                                  onClick={() => clearVariantFieldOverride(index, field.key)}
                                >
                                  <FormattedMessage
                                    id="component.pageEditor.formPresentation.variants.clearOverride"
                                    defaultMessage="还原"
                                  />
                                </Button>
                              )}
                            </Space>
                          );
                        })}
                      </Space>
                    </Form.Item>
                  )}
                </Form>
                {previewKey === variant.key && (
                  <div data-testid={`variant-preview-pane-${index}`} style={{ marginTop: 12 }}>
                    <SchemaFormRenderer spec={merged} disabled hideSubmit />
                  </div>
                )}
              </Card>
            );
          })
        )}
      </Space>
    </Card>
  );
}
