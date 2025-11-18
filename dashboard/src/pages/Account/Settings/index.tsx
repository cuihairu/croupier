import React, { useState } from 'react';
import { Card, Form, Input, Button, Divider, Tabs } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { LockOutlined, SafetyCertificateOutlined, AppstoreOutlined } from '@ant-design/icons';
import { useIntl } from '@umijs/max';
import { getMessage } from '@/utils/antdApp';
import { changeMyPassword } from '@/services/croupier';

export default function AccountSettings() {
  const intl = useIntl();
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);

  const onSubmit = async () => {
    try {
      const v = await form.validateFields();
      if (v.password !== v.confirm) {
        getMessage()?.warning(intl.formatMessage({ id: 'pages.account.settings.password.mismatch' }));
        return;
      }
      setLoading(true);
      await changeMyPassword({ current: v.current || '', password: v.password });
      form.resetFields(['current', 'password', 'confirm']);
      getMessage()?.success(intl.formatMessage({ id: 'pages.account.settings.password.updated' }));
    } finally {
      setLoading(false);
    }
  };

  const passwordTab = (
    <Card title={intl.formatMessage({ id: 'pages.account.settings.password.title' })} style={{ maxWidth: 520 }}>
      <Form form={form} layout="vertical">
        <Form.Item label={intl.formatMessage({ id: 'pages.account.settings.password.current' })} name="current" rules={[{ required: true, message: intl.formatMessage({ id: 'pages.account.settings.password.current.required' }) }]}> 
          <Input.Password placeholder={intl.formatMessage({ id: 'pages.account.settings.password.current.placeholder' })} />
        </Form.Item>
        <Form.Item label={intl.formatMessage({ id: 'pages.account.settings.password.new' })} name="password" rules={[{ required: true, message: intl.formatMessage({ id: 'pages.account.settings.password.new.required' }) }, { min: 6, message: intl.formatMessage({ id: 'pages.account.settings.password.min.length' }) }]}>
          <Input.Password placeholder={intl.formatMessage({ id: 'pages.account.settings.password.new.placeholder' })} />
        </Form.Item>
        <Form.Item label={intl.formatMessage({ id: 'pages.account.settings.password.confirm' })} name="confirm" rules={[{ required: true, message: intl.formatMessage({ id: 'pages.account.settings.password.confirm.required' }) }]}>
          <Input.Password placeholder={intl.formatMessage({ id: 'pages.account.settings.password.confirm.placeholder' })} />
        </Form.Item>
        <Form.Item>
          <Button type="primary" onClick={onSubmit} loading={loading}>{intl.formatMessage({ id: 'pages.account.settings.save' })}</Button>
        </Form.Item>
      </Form>
    </Card>
  );

  // 安全设置和应用设置作为占位内容，后续可以实现具体功能
  const securityTab = (
    <Card title={intl.formatMessage({ id: 'pages.account.settings.security.title' })}>
      <p>{intl.formatMessage({ id: 'pages.account.settings.security.content' })}</p>
    </Card>
  );

  const applicationTab = (
    <Card title={intl.formatMessage({ id: 'pages.account.settings.application.title' })}>
      <p>{intl.formatMessage({ id: 'pages.account.settings.application.content' })}</p>
    </Card>
  );

  const items = [
    {
      key: 'password',
      label: (
        <span>
          <LockOutlined />
          {intl.formatMessage({ id: 'pages.account.settings.password' })}
        </span>
      ),
      children: passwordTab,
    },
    {
      key: 'security',
      label: (
        <span>
          <SafetyCertificateOutlined />
          {intl.formatMessage({ id: 'pages.account.settings.security' })}
        </span>
      ),
      children: securityTab,
    },
    {
      key: 'application',
      label: (
        <span>
          <AppstoreOutlined />
          {intl.formatMessage({ id: 'pages.account.settings.application' })}
        </span>
      ),
      children: applicationTab,
    },
  ];

  return (
    <PageContainer>
      <Card>
        <Tabs defaultActiveKey="password" items={items} />
      </Card>
    </PageContainer>
  );
}

