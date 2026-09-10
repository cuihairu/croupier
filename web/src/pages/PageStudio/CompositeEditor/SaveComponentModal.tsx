import { App, Checkbox, Form, Input, Modal, Select, Typography } from 'antd';
import { request } from '@umijs/max';
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
      message.warning('请填写组件名称');
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
      message.success(`「${name}」已保存——组件库中可拖入复用`);
      onClose();
    } catch {
      message.error('保存失败');
    }
  };

  return (
    <Modal
      title="保存为组件模板"
      open={state !== null}
      onCancel={onClose}
      onOk={() => void confirmSave()}
      okText="保存"
      cancelText="取消"
      destroyOnHidden
    >
      <Form form={form} layout="vertical" preserve={false}>
        <Form.Item
          name="name"
          label="组件名称"
          rules={[{ required: true, message: '组件名称必填' }]}
        >
          <Input placeholder="如：玩家数值下拉查询" maxLength={40} />
        </Form.Item>
        <Form.Item name="category" label="分类" initialValue="自定义">
          <Select
            options={[
              { label: '自定义', value: '自定义' },
              { label: '查询表单', value: '查询表单' },
              { label: '操作面板', value: '操作面板' },
              { label: '监控展示', value: '监控展示' },
            ]}
          />
        </Form.Item>
        <Form.Item name="description" label="描述">
          <Input.TextArea rows={2} placeholder="用途说明（可选）" maxLength={200} />
        </Form.Item>
        {state && state.paramCandidates.length > 0 && (
          <Form.Item
            name="paramKeys"
            label="参数化（勾选后拖入组件时可在弹窗中快速配置）"
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
            包含 {state.selectedNodes.length} 个节点
            {state.fnIds.length > 0 ? `，依赖函数：${state.fnIds.join('、')}` : ''}。保存后在组件库
            Tab 拖入任意组合页复用。
          </Text>
        )}
      </Form>
    </Modal>
  );
}
