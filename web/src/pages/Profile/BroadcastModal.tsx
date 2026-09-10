import { useCallback, useState } from 'react';
import { Form, Input, Modal, Select, message } from 'antd';
import { broadcastMessage } from '@/services/api/messages';
import { extractErrorMessage } from '@/utils/errors';

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
  const [sendForm] = Form.useForm();
  const [sending, setSending] = useState(false);

  const handleSend = useCallback(
    async (values: {
      title: string;
      content: string;
      audience: 'all' | 'users';
      toUser?: string;
    }) => {
      setSending(true);
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
        onClose();
        sendForm.resetFields();
        onSent();
      } catch (error) {
        message.error(extractErrorMessage(error, '发送失败'));
      } finally {
        setSending(false);
      }
    },
    [onClose, onSent, sendForm],
  );

  return (
    <Modal
      title="发送站内消息"
      open={open}
      confirmLoading={sending}
      onCancel={onClose}
      onOk={() => {
        sendForm.validateFields().then(async (values) => {
          await handleSend(values);
        });
      }}
      destroyOnClose
    >
      <Form form={sendForm} layout="vertical">
        <Form.Item name="audience" label="接收范围" initialValue="all" rules={[{ required: true }]}>
          <Select
            options={[
              { value: 'all', label: '全部管理员/用户（广播）' },
              { value: 'users', label: '指定用户' },
            ]}
            onChange={(value) => {
              if (value === 'all') sendForm.setFieldsValue({ toUser: undefined });
            }}
          />
        </Form.Item>
        {sendForm.getFieldValue('audience') === 'users' && (
          <Form.Item
            name="toUser"
            label="接收用户名"
            rules={[{ required: true, message: '请输入接收用户名' }]}
          >
            <Input placeholder="用户名" />
          </Form.Item>
        )}
        <Form.Item name="title" label="标题" rules={[{ required: true, message: '请输入标题' }]}>
          <Input placeholder="消息标题" maxLength={100} />
        </Form.Item>
        <Form.Item name="content" label="内容" rules={[{ required: true, message: '请输入内容' }]}>
          <Input.TextArea rows={4} placeholder="消息内容" maxLength={2000} showCount />
        </Form.Item>
      </Form>
    </Modal>
  );
}
