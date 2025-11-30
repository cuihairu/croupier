import React, { useState, useEffect } from 'react';
import {
  Card,
  Form,
  Input,
  Button,
  Avatar,
  Upload,
  Tabs,
  Divider,
  Row,
  Col,
  Badge,
  Space,
  Typography,
  message,
  Descriptions,
  Modal,
  Alert,
} from 'antd';
import {
  UserOutlined,
  LockOutlined,
  MailOutlined,
  PhoneOutlined,
  UploadOutlined,
  SafetyOutlined,
  SettingOutlined,
} from '@ant-design/icons';
import { PageContainer } from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { getMyProfile, updateMyProfile, changeMyPassword } from '@/services/croupier/me';

const { Title, Text } = Typography;

export default function Profile() {
  const intl = useIntl();
  const formatMessage = (id: string) => intl.formatMessage({ id });
  const [form] = Form.useForm();
  const [profile, setProfile] = useState<any>(null);
  const [loading, setLoading] = useState(false);
  const [passwordLoading, setPasswordLoading] = useState(false);
  const [activeTab, setActiveTab] = useState('info');

  useEffect(() => {
    loadProfile();
  }, []);

  const loadProfile = async () => {
    try {
      const p = await getMyProfile();
      setProfile(p);
      form.setFieldsValue({
        display_name: p.display_name,
        email: p.email,
        phone: p.phone,
      });
    } catch (error) {
      message.error(formatMessage('profile.load.error'));
    }
  };

  const handleProfileSubmit = async (values: any) => {
    setLoading(true);
    try {
      await updateMyProfile({
        display_name: values.display_name,
        email: values.email,
        phone: values.phone,
      });
      message.success(formatMessage('profile.update.success'));
      loadProfile();
    } catch (error) {
      message.error(formatMessage('profile.update.error'));
    } finally {
      setLoading(false);
    }
  };

  const handlePasswordChange = async (values: any) => {
    setPasswordLoading(true);
    try {
      await changeMyPassword({
        current: values.current,
        password: values.password,
      });
      Modal.destroyAll();
      message.success(formatMessage('profile.password.success'));
    } catch (error) {
      message.error(formatMessage('profile.password.error'));
    } finally {
      setPasswordLoading(false);
    }
  };

  const showPasswordModal = () => {
    Modal.confirm({
      title: formatMessage('profile.password.modal.title'),
      content: (
        <div>
          <Alert
            message={formatMessage('profile.password.modal.warning')}
            type="warning"
            showIcon
            style={{ marginBottom: 16 }}
          />
          <Form layout="vertical" onFinish={handlePasswordChange}>
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
                    if (!value || getFieldValue('password') !== value) {
                      return formatMessage('profile.password.mismatch');
                    }
                    return Promise.resolve();
                  },
                }),
              ]}
            >
              <Input.Password placeholder={formatMessage('profile.password.confirm.placeholder')} />
            </Form.Item>
          </Form>
        </div>
      ),
      width: 480,
      okText: formatMessage('profile.password.modal.submit'),
      confirmLoading: passwordLoading,
    });
  };

  const handleAvatarChange = (info: any) => {
    if (info.file.status === 'uploading') {
      message.info(formatMessage('profile.avatar.uploading'));
      return;
    }
    if (info.file.status === 'done') {
      message.success(formatMessage('profile.avatar.success'));
      loadProfile();
    }
  };

  const getInitials = (name: string, email?: string) => {
    if (name) {
      return name.split(' ').map(word => word.charAt(0)).join('').toUpperCase();
    }
    if (email) {
      return email.charAt(0).toUpperCase();
    }
    return '';
  };

  const getStatusBadge = (status?: boolean) => {
    return (
      <Badge
        status={status ? 'success' : 'default'}
        text={status ? formatMessage('profile.status.active') : formatMessage('profile.status.inactive')}
      />
    );
  };

  const infoItems = [
    {
      title: formatMessage('profile.info.username'),
      value: profile?.username,
      icon: <UserOutlined />,
    },
    {
      title: formatMessage('profile.info.joined'),
      value: profile?.created_at ? new Date(profile.created_at).toLocaleDateString() : 'N/A',
      icon: <UserOutlined />,
    },
    {
      title: formatMessage('profile.info.last.login'),
      value: profile?.last_login_at ? new Date(profile.last_login_at).toLocaleString() : 'N/A',
      icon: <UserOutlined />,
    },
  ];

  if (!profile) {
    return (
      <PageContainer>
        <div style={{ textAlign: 'center', padding: '48px 0' }}>
          <div style={{ maxWidth: 400 }}>
            <Card>
              <Space size="large" direction="vertical">
                <div style={{ fontSize: '48px', color: '#1890ff', marginBottom: '16px' }}>
                  <UserOutlined style={{ fontSize: '48px' }} />
                </div>
                <Title level={4}>{formatMessage('profile.loading')}</Title>
              </Space>
            </Card>
          </div>
        </div>
      </PageContainer>
    );
  }

  return (
    <PageContainer>
      <Row gutter={[24, 16]}>
        {/* 左侧个人信息卡片 */}
        <Col xs={24} lg={8}>
          <Card
            title={formatMessage('profile.info.title')}
            extra={getStatusBadge(profile?.active)}
            loading={!profile}
          >
            <Space size="large" direction="vertical" style={{ width: '100%' }}>
              {/* 头像 */}
              <div style={{ textAlign: 'center', marginBottom: '24px' }}>
                <Upload
                  name="avatar"
                  listType="picture-card"
                  className="avatar-uploader"
                  showUploadList={false}
                  action="/api/v1/me/avatar"
                  beforeUpload={() => false}
                  onChange={handleAvatarChange}
                  headers={{
                    Authorization: `Bearer ${localStorage.getItem('token')}`,
                  }}
                >
                  {profile?.avatar ? (
                    <Avatar
                      size={96}
                      src={profile.avatar}
                      style={{
                        border: '3px solid #1890ff',
                        boxShadow: '0 2px 8px rgba(0,0,0,0.15)',
                        backgroundColor: '#fff',
                      }}
                    />
                  ) : (
                    <Avatar
                      size={96}
                      icon={<UserOutlined />}
                      style={{
                        border: '3px solid #1890ff',
                        boxShadow: '0 2px 8px rgba(0,0,0,0.15)',
                        backgroundColor: '#e6f4ff',
                        color: '#1677ff',
                      }}
                    />
                  )}
                  <div style={{ marginTop: '12px' }}>
                    <Button
                      icon={<UploadOutlined />}
                      type="link"
                      onClick={() => document.querySelector('.avatar-uploader input')?.click()}
                    >
                      {formatMessage('profile.avatar.change')}
                    </Button>
                  </div>
                </Upload>
              </div>

              {/* 基本信息 */}
              <Descriptions column={1} size="small">
                <Descriptions.Item
                  label={formatMessage('profile.info.username')}
                  labelStyle={{ width: '80px' }}
                >
                  <Space>
                    <Text strong>{profile?.username}</Text>
                    <Text type="secondary" style={{ marginLeft: '8px' }}>
                      ({profile?.active ? 'Active' : 'Inactive'})
                    </Text>
                  </Space>
                </Descriptions.Item>
                <Descriptions.Item
                  label={formatMessage('profile.info.display.name')}
                  labelStyle={{ width: '80px' }}
                >
                  {profile?.display_name || 'N/A'}
                </Descriptions.Item>
                <Descriptions.Item
                  label={formatMessage('profile.info.email')}
                  labelStyle={{ width: '80px' }}
                >
                  <Space>
                    <Text>{profile?.email || 'N/A'}</Text>
                    {profile?.email && (
                      <Button
                        type="link"
                        size="small"
                        onClick={() => navigator.clipboard.writeText(profile?.email)}
                      >
                        {formatMessage('profile.copy')}
                      </Button>
                    )}
                  </Space>
                </Descriptions.Item>
                <Descriptions.Item
                  label={formatMessage('profile.info.phone')}
                  labelStyle={{ width: '80px' }}
                >
                  {profile?.phone || 'N/A'}
                </Descriptions.Item>
              </Descriptions>

              <Divider />

              {/* 账户信息 */}
              <div style={{ marginBottom: '16px' }}>
                <Title level={5}>{formatMessage('profile.account.info')}</Title>
              </div>
              <Descriptions column={2} size="small">
                <Descriptions.Item
                  label={formatMessage('profile.info.joined')}
                  labelStyle={{ width: '100px' }}
                >
                  {profile?.created_at ? new Date(profile.created_at).toLocaleDateString() : 'N/A'}
                </Descriptions.Item>
                <Descriptions.Item
                  label={formatMessage('profile.info.last.login')}
                  labelStyle={{ width: '100px' }}
                >
                  {profile?.last_login_at ? new Date(profile.last_login_at).toLocaleString() : 'N/A'}
                </Descriptions.Item>
              </Descriptions>
            </Space>

            <div style={{ marginTop: '16px' }}>
              <Button type="primary" block onClick={() => setActiveTab('settings')}>
                {formatMessage('profile.security.settings')}
              </Button>
            </div>
          </Card>
        </Col>

        {/* 右侧标签页内容 */}
        <Col xs={24} lg={16}>
          <Card>
            <Tabs
              activeKey={activeTab}
              onChange={setActiveTab}
              items={[
                {
                  key: 'info',
                  label: (
                    <Space>
                      <UserOutlined />
                      {formatMessage('profile.tab.info')}
                    </Space>
                  ),
                  children: (
                    <Form
                      form={form}
                      layout="vertical"
                      onFinish={handleProfileSubmit}
                      style={{ maxWidth: '600px', margin: '0 auto' }}
                    >
                      <Form.Item
                        name="display_name"
                        label={formatMessage('profile.info.display.name')}
                        rules={[
                          { required: true, message: formatMessage('profile.display.name.required') },
                          { max: 50, message: formatMessage('profile.display.name.max.length') },
                        ]}
                      >
                        <Input placeholder={formatMessage('profile.display.name.placeholder')} />
                      </Form.Item>

                      <Form.Item
                        name="email"
                        label={formatMessage('profile.info.email')}
                        rules={[
                          { type: 'email', message: formatMessage('profile.email.invalid') },
                        ]}
                      >
                        <Input placeholder={formatMessage('profile.email.placeholder')} />
                      </Form.Item>

                      <Form.Item
                        name="phone"
                        label={formatMessage('profile.info.phone')}
                        rules={[
                          { max: 20, message: formatMessage('profile.phone.max.length') },
                          { pattern: /^1[3-9]\d{9}$/, message: formatMessage('profile.phone.invalid') },
                        ]}
                      >
                        <Input placeholder={formatMessage('profile.phone.placeholder')} />
                      </Form.Item>

                      <Form.Item>
                        <Button type="primary" htmlType="submit" loading={loading} block>
                          {formatMessage('profile.save')}
                        </Button>
                      </Form.Item>
                    </Form>
                  ),
                },
                {
                  key: 'settings',
                  label: (
                    <Space>
                      <SafetyOutlined />
                      {formatMessage('profile.tab.security')}
                    </Space>
                  ),
                  children: (
                    <Space direction="vertical" size="large" style={{ width: '100%' }}>
                      {/* 修改密码 */}
                      <Card
                        title={
                          <Space>
                            <LockOutlined />
                            {formatMessage('profile.password.change.title')}
                          </Space>
                        }
                        extra={
                          <Button type="primary" onClick={showPasswordModal}>
                            {formatMessage('profile.password.change.btn')}
                          </Button>
                        }
                        style={{ marginBottom: '16px' }}
                      >
                        <Text type="secondary">
                          {formatMessage('profile.password.description')}
                        </Text>
                      </Card>

                      {/* 其他安全设置 */}
                      <Card
                        title={
                          <Space>
                            <SettingOutlined />
                            {formatMessage('profile.security.settings.title')}
                          </Space>
                        }
                      >
                        <Space direction="vertical" style={{ width: '100%' }}>
                          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '12px 0', borderBottom: '1px solid #f0f0f0' }}>
                            <Text>{formatMessage('profile.two.factor.auth')}</Text>
                            <Badge status="default" text={formatMessage('profile.not.enabled')} />
                          </div>
                          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '12px 0' }}>
                            <Text>{formatMessage('profile.login.notification')}</Text>
                            <Badge status="default" text={formatMessage('profile.enabled')} />
                          </div>
                          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '12px 0' }}>
                            <Text>{formatMessage('profile.session.management')}</Text>
                            <Badge status="default" text={formatMessage('profile.view.sessions')} />
                          </div>
                        </Space>
                      </Card>
                    </Space>
                  ),
                },
              ]}
            />
          </Card>
        </Col>
      </Row>
    </PageContainer>
  );
}
