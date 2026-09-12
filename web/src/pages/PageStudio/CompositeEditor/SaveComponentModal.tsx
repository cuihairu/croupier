import { useEffect, useState } from 'react';
import { App, Checkbox, Form, Input, Modal, Radio, Select, Typography } from 'antd';
import { request, useIntl } from '@umijs/max';
import { localizedText } from '@/utils/localizedText';
import type { PageNode } from './model';
import type { ParamCandidate } from './types';
import type { ComponentTemplateDTO } from './ComponentLibrary';

const { Text } = Typography;

/** 「保存为组件模板」弹窗：命名/分类/描述 + 参数化候选勾选（U6）+ 更新通道（V3）。
 * 收集逻辑（多选集合 → 候选扫描）在编辑器主页，此处仅承载表单与提交。
 * 保存方式两模式：另存新模板（POST 现状）/ 更新已有模板（PUT，builtin 不列——
 * 内置模板更新走「从契约重新生成」，且后端 Update 会强置 builtin=false）。 */
export default function SaveComponentModal({
  state,
  onClose,
}: {
  state: null | {
    fnIds: string[];
    selectedNodes: PageNode[];
    /** 参数化候选（U6）：白名单 prop 扫描结果。 */
    paramCandidates: ParamCandidate[];
  };
  onClose: () => void;
}) {
  const { message } = App.useApp();
  const intl = useIntl();
  const [form] = Form.useForm<{
    mode: 'create' | 'update';
    targetKey?: string;
    name: string;
    description?: string;
    category?: string;
    paramKeys?: string[];
  }>();
  const mode = Form.useWatch('mode', form) ?? 'create';
  const targetKey = Form.useWatch('targetKey', form);
  /** 当前 scope 的自定义模板（builtin 已滤除）；null=未加载。 */
  const [customTemplates, setCustomTemplates] = useState<ComponentTemplateDTO[] | null>(null);

  // 弹窗关闭时重置拉取态：下次打开重新拉（模板可能刚被保存/删除）
  useEffect(() => {
    if (state === null) setCustomTemplates(null);
  }, [state]);

  // 更新模式首次激活时拉取自定义模板列表
  useEffect(() => {
    if (!state || mode !== 'update' || customTemplates !== null) return;
    let cancelled = false;
    void (async () => {
      try {
        const resp = (await request('/api/v1/component-templates', {
          skipErrorHandler: true,
        })) as { items?: ComponentTemplateDTO[] } | ComponentTemplateDTO[];
        const items = Array.isArray(resp) ? resp : (resp?.items ?? []);
        if (!cancelled) setCustomTemplates(items.filter((t) => !t.builtin));
      } catch {
        if (!cancelled) {
          setCustomTemplates([]);
          message.warning(
            intl.formatMessage({
              id: 'pages.pageStudio.saveModal.loadFailed',
              defaultMessage: '拉取自定义模板失败',
            }),
          );
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [state, mode, customTemplates, message, intl]);

  // 选中目标模板后预填名称/分类/描述（结构 tree/params 始终以当前画布选择覆盖）
  useEffect(() => {
    if (mode !== 'update' || !targetKey || !customTemplates) return;
    const tpl = customTemplates.find((t) => t.key === targetKey);
    if (!tpl) return;
    form.setFieldsValue({
      name: localizedText(tpl.name, 'zh-CN', tpl.key),
      category: tpl.category || '自定义',
      description: localizedText(tpl.description, 'zh-CN', ''),
    });
  }, [mode, targetKey, customTemplates, form]);

  const confirmSave = async () => {
    if (!state) return;
    const name = (form.getFieldValue('name') || '').trim();
    const saveMode = (form.getFieldValue('mode') as 'create' | 'update' | undefined) ?? 'create';
    const updateKey = (form.getFieldValue('targetKey') as string | undefined) ?? '';
    if (!name) {
      message.warning(
        intl.formatMessage({
          id: 'pages.pageStudio.saveModal.nameRequired',
          defaultMessage: '请填写组件名称',
        }),
      );
      return;
    }
    if (saveMode === 'update' && !updateKey) {
      message.warning(
        intl.formatMessage({
          id: 'pages.pageStudio.saveModal.targetRequired',
          defaultMessage: '请选择要更新的模板',
        }),
      );
      return;
    }
    try {
      // 勾选的候选 → 参数定义（default=当前值；实例化时可覆盖）
      const picked = new Set((form.getFieldValue('paramKeys') as string[] | undefined) ?? []);
      const params = state.paramCandidates
        .filter((c) => picked.has(c.key))
        .map((c) => ({
          key: c.key,
          label: { 'zh-CN': `${c.nodeTitle}·${c.propLabel}` },
          nodeId: c.nodeId,
          prop: c.prop,
          default: c.current,
        }));
      const description = (form.getFieldValue('description') || '').trim();
      const body = {
        name: { 'zh-CN': name, 'en-US': name },
        description: { 'zh-CN': description, 'en-US': description },
        category: form.getFieldValue('category') || '自定义',
        icon: 'AppstoreOutlined',
        requiredFunctions: state.fnIds,
        ...(params.length ? { params } : {}),
        tree: state.selectedNodes,
      };
      if (saveMode === 'update') {
        // 更新通道：key 以 URL 为准（body 中同名重复，后端 Update 复用 CreateRequest
        // 绑定校验，key 为 required 字段），tree/params/requiredFunctions 以当前画布选择覆盖
        await request(`/api/v1/component-templates/${encodeURIComponent(updateKey)}`, {
          method: 'PUT',
          data: { key: updateKey, ...body },
          skipErrorHandler: true,
        });
        message.success(
          intl.formatMessage(
            {
              id: 'pages.pageStudio.saveModal.updateSuccess',
              defaultMessage: '「{name}」已更新——模板内容已按当前画布选择覆盖',
            },
            { name },
          ),
        );
      } else {
        await request('/api/v1/component-templates', {
          method: 'POST',
          data: { key: `custom--${Date.now().toString(36)}`, ...body },
          skipErrorHandler: true,
        });
        message.success(
          intl.formatMessage(
            {
              id: 'pages.pageStudio.saveModal.saveSuccess',
              defaultMessage: '「{name}」已保存——组件库中可拖入复用',
            },
            { name },
          ),
        );
      }
      onClose();
    } catch {
      message.error(
        intl.formatMessage({
          id: 'pages.pageStudio.saveModal.saveFailed',
          defaultMessage: '保存失败',
        }),
      );
    }
  };

  return (
    <Modal
      title={intl.formatMessage({
        id: 'pages.pageStudio.saveModal.title',
        defaultMessage: '保存为组件模板',
      })}
      open={state !== null}
      onCancel={onClose}
      onOk={() => void confirmSave()}
      okText={intl.formatMessage({
        id: 'pages.pageStudio.saveModal.okText',
        defaultMessage: '保存',
      })}
      cancelText={intl.formatMessage({
        id: 'pages.pageStudio.saveModal.cancelText',
        defaultMessage: '取消',
      })}
      destroyOnHidden
    >
      <Form form={form} layout="vertical" preserve={false}>
        <Form.Item
          name="mode"
          label={intl.formatMessage({
            id: 'pages.pageStudio.saveModal.modeLabel',
            defaultMessage: '保存方式',
          })}
          initialValue="create"
        >
          <Radio.Group
            options={[
              {
                label: intl.formatMessage({
                  id: 'pages.pageStudio.saveModal.modeCreate',
                  defaultMessage: '另存新模板',
                }),
                value: 'create',
              },
              {
                label: intl.formatMessage({
                  id: 'pages.pageStudio.saveModal.modeUpdate',
                  defaultMessage: '更新已有模板',
                }),
                value: 'update',
              },
            ]}
            optionType="button"
            buttonStyle="solid"
          />
        </Form.Item>
        {mode === 'update' && (
          <Form.Item
            name="targetKey"
            label={intl.formatMessage({
              id: 'pages.pageStudio.saveModal.targetLabel',
              defaultMessage: '选择模板',
            })}
          >
            <Select
              showSearch
              optionFilterProp="label"
              placeholder={intl.formatMessage({
                id: 'pages.pageStudio.saveModal.targetPlaceholder',
                defaultMessage: '选择要覆盖的自定义模板',
              })}
              notFoundContent={intl.formatMessage({
                id: 'pages.pageStudio.saveModal.noCustomTemplates',
                defaultMessage:
                  '暂无可更新的自定义模板（内置模板请在模板管理中「从契约重新生成」）',
              })}
              loading={customTemplates === null}
              options={(customTemplates ?? []).map((t) => ({
                label: localizedText(t.name, 'zh-CN', t.key),
                value: t.key,
              }))}
            />
          </Form.Item>
        )}
        <Form.Item
          name="name"
          label={intl.formatMessage({
            id: 'pages.pageStudio.saveModal.nameLabel',
            defaultMessage: '组件名称',
          })}
          rules={[
            {
              required: true,
              message: intl.formatMessage({
                id: 'pages.pageStudio.saveModal.nameRequiredRule',
                defaultMessage: '组件名称必填',
              }),
            },
          ]}
        >
          <Input
            placeholder={intl.formatMessage({
              id: 'pages.pageStudio.saveModal.namePlaceholder',
              defaultMessage: '如：玩家数值下拉查询',
            })}
            maxLength={40}
          />
        </Form.Item>
        <Form.Item
          name="category"
          label={intl.formatMessage({
            id: 'pages.pageStudio.saveModal.categoryLabel',
            defaultMessage: '分类',
          })}
          initialValue="自定义"
        >
          <Select
            options={[
              { label: '自定义', value: '自定义' },
              { label: '查询表单', value: '查询表单' },
              { label: '操作面板', value: '操作面板' },
              { label: '监控展示', value: '监控展示' },
            ]}
          />
        </Form.Item>
        <Form.Item
          name="description"
          label={intl.formatMessage({
            id: 'pages.pageStudio.saveModal.descriptionLabel',
            defaultMessage: '描述',
          })}
        >
          <Input.TextArea
            rows={2}
            placeholder={intl.formatMessage({
              id: 'pages.pageStudio.saveModal.descriptionPlaceholder',
              defaultMessage: '用途说明（可选）',
            })}
            maxLength={200}
          />
        </Form.Item>
        {state && state.paramCandidates.length > 0 && (
          <Form.Item
            name="paramKeys"
            label={intl.formatMessage({
              id: 'pages.pageStudio.saveModal.paramKeysLabel',
              defaultMessage: '参数化（勾选后拖入组件时可在弹窗中快速配置）',
            })}
            initialValue={[]}
          >
            <Checkbox.Group
              options={state.paramCandidates.map((c) => ({
                label: `${c.nodeTitle}·${c.propLabel}`,
                value: c.key,
              }))}
            />
          </Form.Item>
        )}
        {state && (
          <Text type="secondary" style={{ fontSize: 12 }}>
            {intl.formatMessage(
              {
                id: 'pages.pageStudio.saveModal.summary',
                defaultMessage: '包含 {count} 个节点{fns}。保存后在组件库 Tab 拖入任意组合页复用。',
              },
              {
                count: state.selectedNodes.length,
                fns:
                  state.fnIds.length > 0
                    ? intl.formatMessage(
                        {
                          id: 'pages.pageStudio.saveModal.summaryFunctions',
                          defaultMessage: '，依赖函数：{fns}',
                        },
                        { fns: state.fnIds.join('、') },
                      )
                    : '',
              },
            )}
          </Text>
        )}
        {state && mode === 'update' && (
          <Text type="warning" style={{ fontSize: 12, display: 'block', marginTop: 4 }}>
            {intl.formatMessage({
              id: 'pages.pageStudio.saveModal.updateHint',
              defaultMessage: '更新会以当前画布选择覆盖该模板的结构、参数与依赖函数。',
            })}
          </Text>
        )}
      </Form>
    </Modal>
  );
}
