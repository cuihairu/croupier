import React, { useState } from 'react';
import { App, Button, Card, Form, Input } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { FormattedMessage, history, useIntl } from '@umijs/max';
import { upsertGame } from '@/services/api/games';
import GameIcon, { GAME_ICON_SIZE } from '@/components/GameIcon';
import { notifyGamesChanged } from '@/utils/gamesChanged';

type CreateFormValues = {
  name: string;
  aliasName?: string;
  icon?: string;
  description?: string;
};

/** 新增游戏独立页面（games:write）：标识/显示名称/图标，icon 不指定走默认骰子。 */
export default function GameCreatePage() {
  const { message } = App.useApp();
  const intl = useIntl();
  const [form] = Form.useForm<CreateFormValues>();
  const [submitting, setSubmitting] = useState(false);
  const iconPreview = Form.useWatch('icon', form);

  const onFinish = async (values: CreateFormValues) => {
    setSubmitting(true);
    try {
      await upsertGame(values);
      message.success(
        intl.formatMessage({
          id: 'pages.gamesManage.create.success',
          defaultMessage: '游戏已创建',
        }),
      );
      notifyGamesChanged();
      history.push('/system/games');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <PageContainer
      subTitle={intl.formatMessage({
        id: 'pages.gamesManage.create.subTitle',
        defaultMessage: '创建后可在列表页继续编辑显示名称与图标',
      })}
      onBack={() => history.push('/system/games')}
    >
      <Card style={{ maxWidth: 560 }}>
        <Form<CreateFormValues> form={form} layout="vertical" onFinish={onFinish}>
          <Form.Item
            name="name"
            label={
              <FormattedMessage id="pages.gamesManage.form.name" defaultMessage="游戏标识 (Name)" />
            }
            rules={[
              {
                required: true,
                message: intl.formatMessage({
                  id: 'pages.gamesManage.form.nameRequired',
                  defaultMessage: '请输入游戏标识（字母、数字和 _ - @）',
                }),
              },
              {
                pattern: /^[A-Za-z0-9_\-@]+$/,
                message: intl.formatMessage({
                  id: 'pages.gamesManage.form.namePattern',
                  defaultMessage: '仅支持字母、数字和 _ - @',
                }),
              },
            ]}
            extra={
              <FormattedMessage
                id="pages.gamesManage.form.nameExtra"
                defaultMessage="创建后作为业务标识使用，不再修改"
              />
            }
          >
            <Input placeholder="e.g. demo_game" maxLength={128} />
          </Form.Item>
          <Form.Item
            name="aliasName"
            label={
              <FormattedMessage id="pages.gamesManage.form.aliasName" defaultMessage="显示名称" />
            }
            extra={
              <FormattedMessage
                id="pages.gamesManage.form.aliasNameExtra"
                defaultMessage="控制台各处展示用；留空时使用游戏标识"
              />
            }
          >
            <Input maxLength={64} />
          </Form.Item>
          <Form.Item
            name="icon"
            label={<FormattedMessage id="pages.gamesManage.form.icon" defaultMessage="图标" />}
            extra={
              <FormattedMessage
                id="pages.gamesManage.form.iconExtra"
                defaultMessage="图片地址；不指定时显示默认骰子图标"
              />
            }
          >
            <Input
              placeholder="https://..."
              allowClear
              prefix={
                <GameIcon
                  icon={iconPreview}
                  name={iconPreview || 'game'}
                  size={GAME_ICON_SIZE.sm}
                />
              }
            />
          </Form.Item>
          <Form.Item
            name="description"
            label={
              <FormattedMessage id="pages.gamesManage.form.description" defaultMessage="描述" />
            }
          >
            <Input.TextArea rows={3} maxLength={500} />
          </Form.Item>
          <Form.Item style={{ marginBottom: 0 }}>
            <Button
              type="primary"
              htmlType="submit"
              loading={submitting}
              data-testid="game-create-submit"
            >
              <FormattedMessage id="pages.gamesManage.form.submit" defaultMessage="创建" />
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </PageContainer>
  );
}
