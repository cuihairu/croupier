import { useCallback, useEffect, useState } from 'react';
import { Alert, Form, Input, Modal, message } from 'antd';
import { useIntl } from '@umijs/max';
import { changeMyPassword } from '@/services/api/me';
import type { PasswordValues } from './shared';

/** 修改密码弹窗（表单/提交/重置自包含；open 受控）。 */
export default function PasswordModal({ open, onClose }: { open: boolean; onClose: () => void }) {
  const intl = useIntl();
  const formatMessage = useCallback((id: string) => intl.formatMessage({ id }), [intl]);
  const [passwordForm] = Form.useForm();
  const [loading, setLoading] = useState(false);

  // 打开即重置（对齐原 showPasswordModal 的 resetFields→open 顺序语义）
  useEffect(() => {
    if (open) passwordForm.resetFields();
  }, [open, passwordForm]);

  const handleFinish = async (values: PasswordValues) => {
    setLoading(true);
    try {
      await changeMyPassword({
        current: values.current,
        password: values.password,
      });
      message.success(formatMessage('profile.password.success'));
      onClose();
      passwordForm.resetFields();
    } catch (error) {
      message.error(formatMessage('profile.password.error'));
      throw error;
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal
      open={open}
      forceRender
      title={formatMessage('profile.password.modal.title')}
      onCancel={() => {
        onClose();
        passwordForm.resetFields();
      }}
      onOk={() => passwordForm.submit()}
      okText={formatMessage('profile.password.modal.submit')}
      confirmLoading={loading}
    >
      <Alert
        message={formatMessage('profile.password.modal.warning')}
        type="warning"
        showIcon
        style={{ marginBottom: 16 }}
      />
      <Form layout="vertical" form={passwordForm} onFinish={handleFinish}>
        <Form.Item
          name="current"
          label={formatMessage('profile.password.current')}
          rules={[{ required: true }]}
        >
          <Input.Password placeholder={formatMessage('profile.password.current.placeholder')} />
        </Form.Item>
        <Form.Item
          name="password"
          label={formatMessage('profile.password.new')}
          rules={[
            { required: true },
            { min: 6, message: formatMessage('profile.password.min.length') },
          ]}
        >
          <Input.Password placeholder={formatMessage('profile.password.new.placeholder')} />
        </Form.Item>
        <Form.Item
          name="confirm"
          label={formatMessage('profile.password.confirm')}
          rules={[
            { required: true },
            ({ getFieldValue }) => ({
              validator(_, value) {
                if (!value || getFieldValue('password') === value) {
                  return Promise.resolve();
                }
                return Promise.reject(new Error(formatMessage('profile.password.mismatch')));
              },
            }),
          ]}
        >
          <Input.Password placeholder={formatMessage('profile.password.confirm.placeholder')} />
        </Form.Item>
      </Form>
    </Modal>
  );
}
