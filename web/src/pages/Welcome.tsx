import { PageContainer } from '@ant-design/pro-components';
import { history } from '@umijs/max';
import {
  AppstoreOutlined,
  ArrowRightOutlined,
  CheckCircleOutlined,
  DesktopOutlined,
  FunctionOutlined,
  RocketOutlined,
  SettingOutlined,
} from '@ant-design/icons';
import { Button, Card, Col, Row, Space, Tag, Typography, theme } from 'antd';
import React from 'react';

type EntryCardProps = {
  title: string;
  description: string;
  hint: string;
  icon: React.ReactNode;
  actionLabel: string;
  path: string;
  tone: string;
};

function EntryCard({ title, description, hint, icon, actionLabel, path, tone }: EntryCardProps) {
  return (
    <Card
      hoverable
      style={{ height: '100%' }}
      styles={{ body: { height: '100%' } }}
      onClick={() => history.push(path)}
    >
      <Space direction="vertical" size={16} style={{ width: '100%', height: '100%' }}>
        <Space size={12}>
          <div
            style={{
              width: 44,
              height: 44,
              borderRadius: 14,
              display: 'grid',
              placeItems: 'center',
              background: tone,
              color: '#fff',
              fontSize: 20,
            }}
          >
            {icon}
          </div>
          <Space direction="vertical" size={2}>
            <Typography.Title level={4} style={{ margin: 0 }}>
              {title}
            </Typography.Title>
            <Typography.Text type="secondary">{hint}</Typography.Text>
          </Space>
        </Space>
        <Typography.Paragraph type="secondary" style={{ marginBottom: 0, flex: 1 }}>
          {description}
        </Typography.Paragraph>
        <Button type="link" style={{ padding: 0 }} icon={<ArrowRightOutlined />}>
          {actionLabel}
        </Button>
      </Space>
    </Card>
  );
}

const Welcome: React.FC = () => {
  const { token } = theme.useToken();
  const journey = [
    {
      title: '能力确认',
      description: '先在函数目录确认 descriptor、schema 和实例质量，确保供给可靠。',
      actionLabel: '查看函数目录',
      path: '/system/functions/catalog',
    },
    {
      title: '页面装配',
      description: '在对象工作台完成主函数绑定、页面骨架生成、字段细化与发布。',
      actionLabel: '进入对象工作台',
      path: '/system/functions/workspaces',
    },
    {
      title: '运行验证',
      description: '在控制台检查发布结果、入口可见性与页面可读性，确认是否达到交付标准。',
      actionLabel: '打开运行控制台',
      path: '/console/home',
    },
  ];

  return (
    <PageContainer
      header={{
        title: false,
        breadcrumb: {},
      }}
    >
      <Space direction="vertical" size={20} style={{ width: '100%' }}>
        <Card
          bordered={false}
          style={{
            overflow: 'hidden',
            background: `linear-gradient(135deg, ${token.colorBgContainer} 0%, ${token.colorPrimaryBg} 100%)`,
          }}
        >
          <Row gutter={[24, 24]} align="middle">
            <Col xs={24} xl={16}>
              <Space direction="vertical" size={18} style={{ width: '100%' }}>
                <Space wrap>
                  <Tag color="blue">能力供给</Tag>
                  <Tag color="cyan">页面装配</Tag>
                  <Tag color="gold">运行交付</Tag>
                </Space>
                <Typography.Title level={2} style={{ margin: 0, maxWidth: 820 }}>
                  Croupier 把函数能力收敛成可发布、可治理、可运行的业务工作台。
                </Typography.Title>
                <Typography.Paragraph
                  type="secondary"
                  style={{ margin: 0, fontSize: 16, lineHeight: 1.8, maxWidth: 860 }}
                >
                  函数目录负责沉淀原子能力，对象工作台负责把函数组织成页面，控制台负责把已发布页面交付给运营和管理用户。当前稳定交付范围是
                  `tabs + list/form/detail/form-detail`。
                </Typography.Paragraph>
                <Space wrap size={[12, 12]}>
                  <Button
                    type="primary"
                    icon={<RocketOutlined />}
                    size="large"
                    onClick={() => history.push('/system/functions/workspaces')}
                  >
                    进入对象工作台
                  </Button>
                  <Button
                    size="large"
                    icon={<DesktopOutlined />}
                    onClick={() => history.push('/console/home')}
                  >
                    查看运行控制台
                  </Button>
                  <Button
                    size="large"
                    icon={<FunctionOutlined />}
                    onClick={() => history.push('/system/functions/catalog')}
                  >
                    浏览函数目录
                  </Button>
                </Space>
              </Space>
            </Col>
            <Col xs={24} xl={8}>
              <Card
                size="small"
                style={{ background: 'rgba(255,255,255,0.65)', borderColor: token.colorBorder }}
              >
                <Space direction="vertical" size={10} style={{ width: '100%' }}>
                  <Typography.Text strong>当前产品重心</Typography.Text>
                  <Space wrap size={[8, 8]}>
                    <Tag color="processing">正式能力：tabs + list/form/detail/form-detail</Tag>
                    <Tag color="default">Beta / 实验能力不占主路径</Tag>
                  </Space>
                  <Typography.Text type="secondary">
                    当前重点是把“函数确认、页面装配、发布验证”这条主链路打磨到上线标准，而不是继续堆更多布局类型。
                  </Typography.Text>
                  <Space wrap size={[8, 8]}>
                    <Tag icon={<CheckCircleOutlined />} color="success">
                      主路径已成型
                    </Tag>
                    <Tag color="orange">正在继续提升产品表达与可交付感</Tag>
                  </Space>
                </Space>
              </Card>
            </Col>
          </Row>
        </Card>

        <Row gutter={[16, 16]}>
          {journey.map((step, index) => (
            <Col xs={24} md={8} key={step.title}>
              <Card
                size="small"
                style={{
                  height: '100%',
                  background:
                    index === 1
                      ? 'linear-gradient(180deg, rgba(22,119,255,0.06) 0%, #ffffff 100%)'
                      : undefined,
                }}
              >
                <Space direction="vertical" size={10} style={{ width: '100%' }}>
                  <Space wrap size={[8, 8]}>
                    <Tag color={index === 0 ? 'green' : index === 1 ? 'blue' : 'gold'}>
                      {`步骤 ${index + 1}`}
                    </Tag>
                    {index === 1 ? <Tag color="processing">当前核心入口</Tag> : null}
                  </Space>
                  <Typography.Title level={5} style={{ margin: 0 }}>
                    {step.title}
                  </Typography.Title>
                  <Typography.Text type="secondary">{step.description}</Typography.Text>
                  <Button
                    type={index === 1 ? 'primary' : 'default'}
                    onClick={() => history.push(step.path)}
                  >
                    {step.actionLabel}
                  </Button>
                </Space>
              </Card>
            </Col>
          ))}
        </Row>

        <Row gutter={[16, 16]}>
          <Col xs={24} md={8}>
            <EntryCard
              title="对象工作台"
              hint="把函数组装成页面"
              description="从业务对象视角创建页面骨架、绑定函数、预览结构、发布到控制台。这是当前最核心的产品入口。"
              actionLabel="去做页面编排"
              path="/system/functions/workspaces"
              icon={<SettingOutlined />}
              tone="linear-gradient(135deg, #1668dc 0%, #69b1ff 100%)"
            />
          </Col>
          <Col xs={24} md={8}>
            <EntryCard
              title="函数目录"
              hint="管理原子能力"
              description="查看 descriptor、参数 schema、实例和告警。这里负责能力供给，不负责最终页面装配。"
              actionLabel="浏览函数能力"
              path="/system/functions/catalog"
              icon={<FunctionOutlined />}
              tone="linear-gradient(135deg, #0f766e 0%, #34d399 100%)"
            />
          </Col>
          <Col xs={24} md={8}>
            <EntryCard
              title="运行控制台"
              hint="验证发布结果"
              description="查看已经发布的对象工作台，确认运营入口、页面可读性和运行态反馈是否已经达到可交付标准。"
              actionLabel="进入运行入口"
              path="/console/home"
              icon={<AppstoreOutlined />}
              tone="linear-gradient(135deg, #ad6800 0%, #ffd666 100%)"
            />
          </Col>
        </Row>
      </Space>
    </PageContainer>
  );
};

export default Welcome;
