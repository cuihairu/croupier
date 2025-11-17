import React, { useState } from 'react';
import { Card, Form, Input, Button } from 'antd';
import { getMessage } from '@/utils/antdApp';
import { changeMyPassword } from '@/services/croupier';

export default function AccountSettings() {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);

  const onSubmit = async () => {
    try {
      const v = await form.validateFields();
      if (v.password !== v.confirm) {
        getMessage()?.warning('两次输入的新密码不一致');
        return;
      }
      setLoading(true);
      await changeMyPassword({ current: v.current || '', password: v.password });
      form.resetFields(['current', 'password', 'confirm']);
      getMessage()?.success('密码已更新');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{ padding: 24 }}>
      <Card title="修改密码" style={{ maxWidth: 520 }}>
        <Form form={form} layout="vertical">
          <Form.Item label="当前密码" name="current" rules={[{ required: true, message: '请输入当前密码' }]}> 
            <Input.Password placeholder="当前密码" />
          </Form.Item>
          <Form.Item label="新密码" name="password" rules={[{ required: true, message: '请输入新密码' }, { min: 6, message: '至少 6 位' }]}>
            <Input.Password placeholder="新密码" />
          </Form.Item>
          <Form.Item label="确认新密码" name="confirm" rules={[{ required: true, message: '请再次输入新密码' }]}>
            <Input.Password placeholder="再次输入新密码" />
          </Form.Item>
          <Form.Item>
            <Button type="primary" onClick={onSubmit} loading={loading}>保存</Button>
          </Form.Item>
        </Form>
      </Card>
    </div>
  );
}

