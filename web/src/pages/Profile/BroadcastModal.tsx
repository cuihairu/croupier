import { useRef } from 'react';
import { Form, Input, Select, message } from 'antd';
import type { FormInstance } from 'antd';
import { ModalForm } from '@ant-design/pro-components';
import { broadcastMessage } from '@/services/api/messages';
import { extractErrorMessage } from '@/utils/errors';

type BroadcastFormValues = {
  audience: 'all' | 'users';
  toUser?: string;
  title: string;
  content: string;
};

/** 管理员群发站内消息弹窗（open 受控；发送成功后回调刷新通知列表）。 */
export default function BroadcastModal({
  open,
  onClose,
  onSent,
}: {
  open: boolean;
  onClose: () => void;
  onSent: () => void;
}) {
  // 切回「广播」时清空 toUser 需要命令式写回；formRef 指向当前挂载的表单实例
  // （destroyOnHidden 下每次打开都是新实例，不会持有陈旧 store）
  const formRef = useRef<FormInstance<BroadcastFormValues> | undefined>(undefined);

  return (
    <ModalForm<BroadcastFormValues>
      title="发送站内消息"
      open={open}
      onOpenChange={(v) => {
        if (!v) onClose();
      }}
      formRef={formRef}
      modalProps={{ destroyOnHidden: true }}
      width={520}
      submitter={{ searchConfig: { submitText: '确定' } }}
      initialValues={{ audience: 'all' }}
      onFinish={async (values) => {
        try {
          const isAll = values.audience === 'all';
          const res = await broadcastMessage({
            audience: isAll ? 'all' : 'users',
            usernames: isAll ? undefined : [values.toUser || ''].filter(Boolean),
            type: 'system',
            title: values.title,
            content: values.content,
          });
          message.success(`已发送给 ${res.sent} 位用户`);
          onSent();
          return true;
        } catch (error) {
          message.error(extractErrorMessage(error, '发送失败'));
          return false;
        }
      }}
    >
      <Form.Item name="audience" label="接收范围" rules={[{ required: true }]}>
        <Select
          options={[
            { value: 'all', label: '全部管理员/用户（广播）' },
            { value: 'users', label: '指定用户' },
          ]}
          onChange={(value) => {
            if (value === 'all') formRef.current?.setFieldsValue({ toUser: undefined });
          }}
        />
      </Form.Item>
      {/* shouldUpdate 订阅 audience 变化实时显隐：原实现在父组件渲染期读
          getFieldValue，切换「指定用户」后输入框不会立即出现（陈旧渲染缺陷），
          随本次迁移修复 */}
      <Form.Item noStyle shouldUpdate={(prev, next) => prev.audience !== next.audience}>
        {({ getFieldValue }) =>
          getFieldValue('audience') === 'users' ? (
            <Form.Item
              name="toUser"
              label="接收用户名"
              rules={[{ required: true, message: '请输入接收用户名' }]}
            >
              <Input placeholder="用户名" />
            </Form.Item>
          ) : null
        }
      </Form.Item>
      <Form.Item name="title" label="标题" rules={[{ required: true, message: '请输入标题' }]}>
        <Input placeholder="消息标题" maxLength={100} />
      </Form.Item>
      <Form.Item name="content" label="内容" rules={[{ required: true, message: '请输入内容' }]}>
        <Input.TextArea rows={4} placeholder="消息内容" maxLength={2000} showCount />
      </Form.Item>
    </ModalForm>
  );
}
