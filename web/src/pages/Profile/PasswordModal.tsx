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
  // defaultMessage 与 zh-CN locale 同值：缺 key 时兜底可读文案（与其余页面
  // formatMessage 写法一致），测试环境 intl mock 同样依赖 defaultMessage。
  const formatMessage = useCallback(
    (id: string, defaultMessage: string) => intl.formatMessage({ id, defaultMessage }),
    [intl],
  );

  return (
    <ModalForm<PasswordValues>
      open={open}
      onOpenChange={(v) => {
        if (!v) onClose();
      }}
      title={formatMessage('profile.password.modal.title', '修改密码')}
      modalProps={{ destroyOnHidden: true }}
      width={520}
      submitter={{
        searchConfig: { submitText: formatMessage('profile.password.modal.submit', '确认修改') },
      }}
      // destroyOnHidden 下每次打开都是全新表单实例，等价于原实现打开即 resetFields
      onFinish={async (values) => {
        try {
          await changeMyPassword({
            current: values.current,
            password: values.password,
          });
          message.success(formatMessage('profile.password.success', '密码更新成功'));
          return true;
        } catch {
          // 原实现本地 toast 后 rethrow；此处按 ModalForm 语义失败保持弹窗开启
          message.error(formatMessage('profile.password.error', '更新密码失败'));
          return false;
        }
      }}
    >
      <Alert
        title={formatMessage('profile.password.modal.warning', '继续之前请确保记住新密码。')}
        type="warning"
        showIcon
        style={{ marginBottom: 16 }}
      />
      <Form.Item
        name="current"
        label={formatMessage('profile.password.current', '当前密码')}
        rules={[{ required: true }]}
      >
        <Input.Password
          placeholder={formatMessage('profile.password.current.placeholder', '请输入当前密码')}
        />
      </Form.Item>
      <Form.Item
        name="password"
        label={formatMessage('profile.password.new', '新密码')}
        rules={[
          { required: true },
          { min: 6, message: formatMessage('profile.password.min.length', '至少 6 位字符') },
        ]}
      >
        <Input.Password
          placeholder={formatMessage('profile.password.new.placeholder', '请输入新密码')}
        />
      </Form.Item>
      <Form.Item
        name="confirm"
        label={formatMessage('profile.password.confirm', '确认密码')}
        rules={[
          { required: true },
          ({ getFieldValue }) => ({
            validator(_, value) {
              if (!value || getFieldValue('password') === value) {
                return Promise.resolve();
              }
              return Promise.reject(
                new Error(formatMessage('profile.password.mismatch', '两次输入的密码不一致')),
              );
            },
          }),
        ]}
      >
        <Input.Password
          placeholder={formatMessage('profile.password.confirm.placeholder', '再次输入新密码')}
        />
      </Form.Item>
    </ModalForm>
  );
}
