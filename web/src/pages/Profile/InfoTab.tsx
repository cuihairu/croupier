import { useCallback, useEffect } from 'react';
import { Button, Card, Col, Descriptions, Form, Input, Row, Space } from 'antd';
import {
  EditOutlined,
  HistoryOutlined,
  MailOutlined,
  PhoneOutlined,
  RocketOutlined,
  UserOutlined,
} from '@ant-design/icons';
import { useIntl } from '@umijs/max';
import type { FormInstance } from 'antd';
import { formatDateTime } from '@/utils/format';
import type { ProfileData } from './shared';

/** 基本资料 Tab：查看态 Descriptions / 编辑态表单（表单实例由主页持有——hero
 * 编辑按钮与取消回填共用）。 */
export default function InfoTab({
  profile,
  editing,
  form,
  loading,
  latestLoginIP,
  onEdit,
  onCancelEdit,
  onSubmit,
}: {
  profile: ProfileData;
  editing: boolean;
  form: FormInstance;
  loading: boolean;
  latestLoginIP: string;
  onEdit: () => void;
  onCancelEdit: () => void;
  onSubmit: (values: ProfileData) => void;
}) {
  const intl = useIntl();
  const formatMessage = useCallback((id: string) => intl.formatMessage({ id }), [intl]);
  const notSet = formatMessage('profile.info.notSet');

  // 资料到位后回填表单。
  //
  // 刻意放在这里而不是数据层：<Form> 只在 InfoTab 挂载后才存在，而 InfoTab 位于
  // Tabs 的「资料」面板内、antd 惰性渲染。若由数据层（在任何标签下都会跑）调用
  // setFieldsValue，用户停在其它标签时实例未连接，antd 每次都会告警
  // "not connected to any Form element"（docs/BUGS.md BUG-008）。
  //
  // 依赖只取三个字段：编辑期间用户输入的值不应被后台刷新覆盖。
  const { displayName, email, phone } = profile || {};
  useEffect(() => {
    form.setFieldsValue({
      displayName: displayName || profile?.nickname,
      email,
      phone,
    });
  }, [form, displayName, email, phone, profile?.nickname]);

  const infoItems = [
    {
      title: formatMessage('profile.info.user.id'),
      value: profile?.id ?? notSet,
      icon: <UserOutlined />,
    },
    {
      title: formatMessage('profile.info.username'),
      value: profile?.username ?? notSet,
      icon: <UserOutlined />,
    },
    {
      title: formatMessage('profile.info.email'),
      value: profile?.email ?? notSet,
      icon: <MailOutlined />,
    },
    {
      title: formatMessage('profile.info.phone'),
      value: profile?.phone || notSet,
      icon: <PhoneOutlined />,
    },
    {
      title: formatMessage('profile.info.joined'),
      value: profile?.createdAt ? formatDateTime(String(profile.createdAt)) : notSet,
      icon: <RocketOutlined />,
    },
    {
      title: formatMessage('profile.info.last.login'),
      value: profile?.lastLoginAt ? formatDateTime(String(profile.lastLoginAt)) : notSet,
      icon: <HistoryOutlined />,
    },
    {
      title: formatMessage('profile.info.last.login.ip'),
      value: latestLoginIP,
      icon: <HistoryOutlined />,
    },
  ];

  return (
    <Row gutter={[24, 24]}>
      <Col xs={24}>
        <Card
          title={
            editing
              ? formatMessage('profile.section.profileForm')
              : formatMessage('profile.account.info')
          }
          extra={
            editing ? (
              <Space>
                <Button onClick={onCancelEdit}>{formatMessage('profile.edit.cancel')}</Button>
                <Button type="primary" loading={loading} onClick={() => form.submit()}>
                  {formatMessage('profile.save')}
                </Button>
              </Space>
            ) : (
              <Button type="primary" icon={<EditOutlined />} onClick={onEdit}>
                {formatMessage('profile.hero.edit')}
              </Button>
            )
          }
        >
          {!editing ? (
            <Descriptions column={1} size="middle">
              {infoItems.map((item) => (
                <Descriptions.Item key={item.title} label={item.title}>
                  <Space>
                    {item.icon}
                    <span>{item.value || formatMessage('profile.info.notSet')}</span>
                  </Space>
                </Descriptions.Item>
              ))}
            </Descriptions>
          ) : null}
          {/*
            表单始终挂载、非编辑态仅隐藏：`form` 实例由父级 useForm 创建并被
            复用（进入编辑前 setFieldsValue 回填、保存时 submit）。若只在编辑态
            渲染 <Form>，未编辑时实例处于「未连接」状态，antd 每次渲染都会告警
            "Instance created by `useForm` is not connected to any Form element"。
            用 hidden 保留挂载（display:none 下不参与布局），视觉与之前一致。
          */}
          <div hidden={!editing}>
            <Form form={form} layout="vertical" onFinish={onSubmit}>
              <Form.Item
                name="displayName"
                label={formatMessage('profile.info.display.name')}
                rules={[
                  {
                    required: true,
                    message: formatMessage('profile.display.name.required'),
                  },
                  {
                    max: 50,
                    message: formatMessage('profile.display.name.max.length'),
                  },
                ]}
              >
                <Input placeholder={formatMessage('profile.display.name.placeholder')} />
              </Form.Item>

              <Form.Item
                name="email"
                label={formatMessage('profile.info.email')}
                rules={[{ type: 'email', message: formatMessage('profile.email.invalid') }]}
              >
                <Input placeholder={formatMessage('profile.email.placeholder')} />
              </Form.Item>

              <Form.Item
                name="phone"
                label={formatMessage('profile.info.phone')}
                rules={[
                  { max: 20, message: formatMessage('profile.phone.max.length') },
                  {
                    pattern: /^1[3-9]\d{9}$/,
                    message: formatMessage('profile.phone.invalid'),
                  },
                ]}
              >
                <Input placeholder={formatMessage('profile.phone.placeholder')} />
              </Form.Item>
            </Form>
          </div>
        </Card>
      </Col>
    </Row>
  );
}
