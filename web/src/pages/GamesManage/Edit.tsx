import React, { useEffect, useState } from 'react';
import { App, Button, Card, Form, Input, Spin } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { FormattedMessage, history, useParams, useIntl } from '@umijs/max';
import { getGame, updateGame } from '@/services/api/games';
import GameIcon, { GAME_ICON_SIZE } from '@/components/GameIcon';
import { notifyGamesChanged } from '@/utils/gamesChanged';

type EditFormValues = {
  name?: string;
  aliasName: string;
  icon?: string;
};

/** 编辑游戏独立页面（games:write）：显示名称与图标；标识只读，icon 留空即回默认。 */
export default function GameEditPage() {
  const { message } = App.useApp();
  const intl = useIntl();
  const { id } = useParams<{ id: string }>();
  const [form] = Form.useForm<EditFormValues>();
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [notFound, setNotFound] = useState(false);
  const iconPreview = Form.useWatch('icon', form);

  useEffect(() => {
    if (!id) return;
    let cancelled = false;
    (async () => {
      try {
        const resp = await getGame(id);
        const game = resp?.game;
        if (cancelled || !game) return;
        form.setFieldsValue({
          name: game.name,
          aliasName: game.aliasName || game.name || '',
          icon: game.icon,
        });
      } catch {
        if (!cancelled) setNotFound(true);
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [form, id]);

  const onFinish = async (values: EditFormValues) => {
    if (!id) return;
    setSubmitting(true);
    try {
      await updateGame(id, { aliasName: values.aliasName, icon: values.icon ?? '' });
      message.success(
        intl.formatMessage({
          id: 'pages.gamesManage.edit.success',
          defaultMessage: '游戏已更新',
        }),
      );
      notifyGamesChanged();
      history.push('/system/games');
    } finally {
      setSubmitting(false);
    }
  };

  if (loading) {
    return (
      <PageContainer onBack={() => history.push('/system/games')}>
        <div style={{ textAlign: 'center', padding: 48 }}>
          <Spin />
        </div>
      </PageContainer>
    );
  }

  if (notFound) {
    return (
      <PageContainer
        title={
          <FormattedMessage id="pages.gamesManage.edit.notFound" defaultMessage="游戏不存在" />
        }
        onBack={() => history.push('/system/games')}
      >
        <Card>
          <FormattedMessage
            id="pages.gamesManage.edit.notFoundDesc"
            defaultMessage="该游戏可能已被删除，请返回列表刷新。"
          />
        </Card>
      </PageContainer>
    );
  }

  return (
    <PageContainer
      subTitle={intl.formatMessage({
        id: 'pages.gamesManage.edit.subTitle',
        defaultMessage: '编辑显示名称与图标；图标清空即回默认骰子',
      })}
      onBack={() => history.push('/system/games')}
    >
      <Card style={{ maxWidth: 560 }}>
        <Form<EditFormValues> form={form} layout="vertical" onFinish={onFinish}>
          <Form.Item
            name="name"
            label={
              <FormattedMessage id="pages.gamesManage.form.name" defaultMessage="游戏标识 (Name)" />
            }
          >
            <Input disabled />
          </Form.Item>
          <Form.Item
            name="aliasName"
            label={
              <FormattedMessage id="pages.gamesManage.form.aliasName" defaultMessage="显示名称" />
            }
            rules={[
              {
                required: true,
                message: intl.formatMessage({
                  id: 'pages.gamesManage.form.aliasNameRequired',
                  defaultMessage: '显示名称不能为空',
                }),
              },
            ]}
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
          <Form.Item style={{ marginBottom: 0 }}>
            <Button
              type="primary"
              htmlType="submit"
              loading={submitting}
              data-testid="game-edit-submit"
            >
              <FormattedMessage id="pages.gamesManage.form.save" defaultMessage="保存" />
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </PageContainer>
  );
}
