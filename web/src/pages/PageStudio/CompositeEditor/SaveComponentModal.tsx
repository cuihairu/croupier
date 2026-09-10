import { App, Checkbox, Form, Input, Modal, Select, Typography } from 'antd';
import { request, useIntl } from '@umijs/max';
import type { PageNode } from './model';
import type { ParamCandidate } from './types';

const { Text } = Typography;

/** 「保存为组件模板」弹窗：命名/分类/描述 + 参数化候选勾选（U6）。
 * 收集逻辑（多选集合 → 候选扫描）在编辑器主页，此处仅承载表单与提交。 */
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
    name: string;
    description?: string;
    category?: string;
    paramKeys?: string[];
  }>();

  const confirmSave = async () => {
    if (!state) return;
    const name = (form.getFieldValue('name') || '').trim();
    if (!name) {
      message.warning(
        intl.formatMessage({
          id: 'pages.pageStudio.saveModal.nameRequired',
          defaultMessage: '请填写组件名称',
        }),
      );
      return;
    }
    try {
      const key = `custom--${Date.now().toString(36)}`;
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
      await request('/api/v1/component-templates', {
        method: 'POST',
        data: {
          key,
          name: { 'zh-CN': name, 'en-US': name },
          description: {
            'zh-CN': (form.getFieldValue('description') || '').trim(),
            'en-US': (form.getFieldValue('description') || '').trim(),
          },
          category: form.getFieldValue('category') || '自定义',
          icon: 'AppstoreOutlined',
          requiredFunctions: state.fnIds,
          ...(params.length ? { params } : {}),
          tree: state.selectedNodes,
        },
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
      </Form>
    </Modal>
  );
}
