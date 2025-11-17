import React, { useEffect, useState } from 'react';
import { Card, Form, Input, Button } from 'antd';
import { getMessage } from '@/utils/antdApp';
import { getMyProfile, updateMyProfile } from '@/services/croupier';

export default function AccountCenter() {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    (async () => {
      try {
        const p = await getMyProfile();
        form.setFieldsValue({ display_name: p.display_name, email: p.email, phone: p.phone });
      } catch (e) {
        // handled globally
      }
    })();
  }, []);

  const onSubmit = async () => {
    try {
      const v = await form.validateFields();
      setLoading(true);
      await updateMyProfile(v);
      getMessage()?.success('个人资料已更新');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{ padding: 24 }}>
      <Card title="个人资料">
        <Form form={form} layout="vertical" style={{ maxWidth: 520 }}>
          <Form.Item label="显示名称" name="display_name">
            <Input placeholder="请输入显示名称" />
          </Form.Item>
          <Form.Item label="邮箱" name="email" rules={[{ type: 'email', message: '请输入合法邮箱' }]}> 
            <Input placeholder="name@example.com" />
          </Form.Item>
          <Form.Item label="手机号" name="phone">
            <Input placeholder="可选" />
          </Form.Item>
          <Form.Item>
            <Button type="primary" onClick={onSubmit} loading={loading}>保存</Button>
          </Form.Item>
        </Form>
      </Card>
    </div>
  );
}

