import React, { useEffect, useState } from 'react';
import { Card, Form, Input, Button, Avatar, Divider, Upload, message } from 'antd';
import { PageContainer, ProCard } from '@ant-design/pro-components';
import { UserOutlined, UploadOutlined } from '@ant-design/icons';
import { useIntl } from '@umijs/max';
import { getMessage } from '@/utils/antdApp';
import { getMyProfile, updateMyProfile } from '@/services/croupier';

export default function AccountCenter() {
  const intl = useIntl();
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [profile, setProfile] = useState<any>({});

  useEffect(() => {
    (async () => {
      try {
        const p = await getMyProfile();
        setProfile(p);
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
      message.success(intl.formatMessage({ id: 'pages.account.center.profile.updated' }));
    } finally {
      setLoading(false);
    }
  };

  const handleAvatarChange = (info: any) => {
    if (info.file.status === 'uploading') {
      return;
    }
    if (info.file.status === 'done') {
      message.success(intl.formatMessage({ id: 'pages.account.center.avatar.updated' }));
    }
  };

  return (
    <PageContainer>
      <ProCard direction="column" ghost gutter={[0, 16]}>
        <ProCard>
          <Card>
            <div style={{ display: 'flex', alignItems: 'center', marginBottom: 24 }}>
              <Upload 
                name="avatar" 
                showUploadList={false}
                action="/api/me/avatar"
                onChange={handleAvatarChange}
                headers={{ Authorization: `Bearer ${localStorage.getItem('token')}` }}
              >
                <Avatar size={120} src={profile.avatar} icon={<UserOutlined />}>
                  {profile.username?.charAt(0).toUpperCase() || profile.display_name?.charAt(0).toUpperCase()}
                </Avatar>
              </Upload>
              <div style={{ marginLeft: 24 }}>
                <h2>{profile.display_name || profile.username}</h2>
                <p style={{ color: '#8c8c8c' }}>@{profile.username}</p>
                <div style={{ marginTop: 12 }}>
                  <div>{intl.formatMessage({ id: 'pages.account.center.joined' })}: {profile.created_at ? new Date(profile.created_at).toLocaleDateString() : 'N/A'}</div>
                  <div>{intl.formatMessage({ id: 'pages.account.center.last.login' })}: {profile.last_login_at ? new Date(profile.last_login_at).toLocaleString() : 'N/A'}</div>
                </div>
              </div>
            </div>
            <Divider />
            <Card title={intl.formatMessage({ id: 'pages.account.center.profile.title' })}>
              <Form form={form} layout="vertical" style={{ maxWidth: 520 }}>
                <Form.Item label={intl.formatMessage({ id: 'pages.account.center.display.name' })} name="display_name">
                  <Input placeholder={intl.formatMessage({ id: 'pages.account.center.display.name.placeholder' })} />
                </Form.Item>
                <Form.Item label={intl.formatMessage({ id: 'pages.account.center.email' })} name="email" rules={[{ type: 'email', message: intl.formatMessage({ id: 'pages.account.center.email.error' }) }]}> 
                  <Input placeholder={intl.formatMessage({ id: 'pages.account.center.email.placeholder' })} />
                </Form.Item>
                <Form.Item label={intl.formatMessage({ id: 'pages.account.center.phone' })} name="phone">
                  <Input placeholder={intl.formatMessage({ id: 'pages.account.center.phone.placeholder' })} />
                </Form.Item>
                <Form.Item>
                  <Button type="primary" onClick={onSubmit} loading={loading}>{intl.formatMessage({ id: 'pages.account.center.save' })}</Button>
                </Form.Item>
              </Form>
            </Card>
          </Card>
        </ProCard>
      </ProCard>
    </PageContainer>
  );
}

