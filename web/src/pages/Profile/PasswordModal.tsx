import { useCallback } from 'react';
import { App, Alert, Form, Input } from 'antd';
import { ModalForm } from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { changeMyPassword } from '@/services/api/me';
import type { PasswordValues } from './shared';

/** 修改密码弹窗（表单/提交/重置自包含；open 受控）。 */
export default function PasswordModal({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { message } = App.useApp();
  const intl = useIntl();
  const formatMessage = useCallback((id: string) => intl.formatMessage({ id }), [intl]);

  return (
    <ModalForm<PasswordValues>
      open={open}
      onOpenChange={(v) => {
        if (!v) onClose();
      }}
      title={formatMessage('profile.password.modal.title')}
      modalProps={{ destroyOnHidden: true }}
      width={520}
      submitter={{ searchConfig: { submitText: formatMessage('profile.password.modal.submit') } }}
      // destroyOnHidden 下每次打开都是全新表单实例，等价于原实现打开即 resetFields
      onFinish={async (values) => {
        try {
          await changeMyPassword({
            current: values.current,
            password: values.password,
          });
          message.success(formatMessage('profile.password.success'));
          return true;
        } catch {
          // 原实现本地 toast 后 rethrow；此处按 ModalForm 语义失败保持弹窗开启
          message.error(formatMessage('profile.password.error'));
          return false;
        }
      }}
    >
      <Alert
        message={formatMessage('profile.password.modal.warning')}
        type="warning"
        showIcon
        style={{ marginBottom: 16 }}
      />
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
    </ModalForm>
  );
}
