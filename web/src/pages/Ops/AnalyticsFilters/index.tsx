import React, { useEffect, useMemo, useState } from 'react';
import { App, Card, Space, Select, Button, Switch, Tag, InputNumber, Tooltip } from 'antd';
import { FormattedMessage, useIntl } from '@umijs/max';
import { fetchAnalyticsFilters, saveAnalyticsFilters } from '@/services/api/analytics';
import { listGamesMeta } from '@/services/api/games';

type GameOption = {
  gameId: string;
  label: string;
  envs: string[];
};

export default function AnalyticsFiltersPage() {
  const { message } = App.useApp();
  const intl = useIntl();
  const [games, setGames] = useState<GameOption[]>([]);
  const [gameId, setGameId] = useState<string>('');
  const [envs, setEnvs] = useState<string[]>([]);
  const [env, setEnv] = useState<string>('');
  const [events, setEvents] = useState<string[]>([]);
  const [paymentsEnabled, setPaymentsEnabled] = useState<boolean>(true);
  const [sampleGlobal, setSampleGlobal] = useState<number>(100);
  const [loading, setLoading] = useState(false);

  // load games list once
  useEffect(() => {
    (async () => {
      try {
        const r = await listGamesMeta();
        const arr = (r?.games || [])
          .map((g) => ({
            gameId: String(g.name || ''),
            label: g.displayName || g.aliasName || g.name || 'Unknown',
            envs: Array.isArray(g.envs)
              ? g.envs
              : Array.isArray(g.envMeta)
                ? g.envMeta.map((env) => env?.env).filter(Boolean)
                : [],
          }))
          .filter((g: GameOption) => Boolean(g.gameId));
        setGames(arr);
      } catch {}
    })();
  }, []);

  // load envs when game changes
  useEffect(() => {
    setEnv('');
    const selectedGame = games.find((item) => item.gameId === gameId);
    setEnvs(selectedGame?.envs || []);
  }, [gameId, games]);

  const canQuery = useMemo(() => !!gameId && env !== '', [gameId, env]);

  const load = async () => {
    if (!canQuery) return;
    setLoading(true);
    try {
      const r = await fetchAnalyticsFilters({ gameId, env });
      setEvents(r.events || []);
      setPaymentsEnabled(r.paymentsEnabled !== false);
      setSampleGlobal(r.sampleGlobal ?? 100);
    } catch {
      message.error(
        intl.formatMessage({
          id: 'pages.opsAnalyticsFilters.error.loadFailed',
          defaultMessage: '加载失败',
        }),
      );
    } finally {
      setLoading(false);
    }
  };

  const save = async () => {
    if (!canQuery) {
      message.warning(
        intl.formatMessage({
          id: 'pages.opsAnalyticsFilters.warning.selectGameEnv',
          defaultMessage: '请选择游戏与环境',
        }),
      );
      return;
    }
    setLoading(true);
    try {
      await saveAnalyticsFilters({
        gameId,
        env,
        events,
        paymentsEnabled,
        sampleGlobal,
      });
      message.success(
        intl.formatMessage({
          id: 'pages.opsAnalyticsFilters.success.saved',
          defaultMessage: '已保存',
        }),
      );
    } catch {
      message.error(
        intl.formatMessage({
          id: 'pages.opsAnalyticsFilters.error.saveFailed',
          defaultMessage: '保存失败（需要 analytics:manage 权限）',
        }),
      );
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{ padding: 24 }}>
      <Card
        title={intl.formatMessage({
          id: 'pages.opsAnalyticsFilters.card.title',
          defaultMessage: '采样控制（Server 下发）',
        })}
        extra={
          <Space wrap>
            <Select
              placeholder={intl.formatMessage({
                id: 'pages.opsAnalyticsFilters.select.gamePlaceholder',
                defaultMessage: '游戏',
              })}
              value={gameId}
              onChange={setGameId}
              style={{ minWidth: 200 }}
              options={games.map((g) => ({ label: g.label, value: g.gameId }))}
            />
            <Select
              placeholder={intl.formatMessage({
                id: 'pages.opsAnalyticsFilters.select.envPlaceholder',
                defaultMessage: '环境',
              })}
              value={env}
              onChange={setEnv}
              style={{ minWidth: 160 }}
              options={envs.map((e) => ({ label: e, value: e }))}
            />
            <Button onClick={load} disabled={!canQuery} loading={loading}>
              <FormattedMessage id="pages.opsAnalyticsFilters.action.load" defaultMessage="加载" />
            </Button>
            <Button type="primary" onClick={save} disabled={!canQuery} loading={loading}>
              <FormattedMessage id="pages.opsAnalyticsFilters.action.save" defaultMessage="保存" />
            </Button>
          </Space>
        }
      >
        <Space orientation="vertical" style={{ width: '100%' }}>
          <div>
            <b>
              <FormattedMessage
                id="pages.opsAnalyticsFilters.sample.label"
                defaultMessage="全局采样："
              />
            </b>
            <InputNumber
              min={0}
              max={100}
              value={sampleGlobal}
              onChange={(v) => setSampleGlobal(Number(v || 0))}
            />
            <span style={{ marginLeft: 8 }}>%</span>
            <Tooltip
              title={intl.formatMessage({
                id: 'pages.opsAnalyticsFilters.hint.sampleTooltip',
                defaultMessage: '按百分比随机采样，100 表示全量，0 表示全部丢弃。',
              })}
            >
              <Tag style={{ marginLeft: 8 }}>
                <FormattedMessage
                  id="pages.opsAnalyticsFilters.hint.explainTag"
                  defaultMessage="说明"
                />
              </Tag>
            </Tooltip>
          </div>
          <div>
            <b>
              <FormattedMessage
                id="pages.opsAnalyticsFilters.payments.label"
                defaultMessage="支付埋点："
              />
            </b>
            <Switch checked={paymentsEnabled} onChange={setPaymentsEnabled} />
            <span style={{ marginLeft: 8, color: '#888' }}>
              {paymentsEnabled
                ? intl.formatMessage({
                    id: 'pages.opsAnalyticsFilters.payments.enabled',
                    defaultMessage: '允许上报',
                  })
                : intl.formatMessage({
                    id: 'pages.opsAnalyticsFilters.payments.disabled',
                    defaultMessage: '禁用（全部丢弃）',
                  })}
            </span>
          </div>
          <div>
            <b>
              <FormattedMessage
                id="pages.opsAnalyticsFilters.events.label"
                defaultMessage="事件白名单："
              />
            </b>
            <Select
              mode="tags"
              style={{ width: '100%', maxWidth: 720 }}
              value={events}
              onChange={(v) => setEvents(v)}
              placeholder={intl.formatMessage({
                id: 'pages.opsAnalyticsFilters.events.placeholder',
                defaultMessage: '输入允许的事件名，留空=全部允许',
              })}
            />
          </div>
          <div style={{ color: '#888' }}>
            <FormattedMessage
              id="pages.opsAnalyticsFilters.hint.sampleRule"
              defaultMessage="说明：留空事件列表=允许全部；设置为空列表（删除所有 tag）=全部丢弃。Agent 端将基于白名单与采样率丢弃事件；支付上报可独立开关。"
            />
          </div>
          <div>
            <FormattedMessage
              id="pages.opsAnalyticsFilters.summary.current"
              defaultMessage="当前："
            />
            {gameId || '-'} | {env || '-'}
            {events?.length ? (
              <span>
                {' '}
                <FormattedMessage
                  id="pages.opsAnalyticsFilters.summary.eventsCount"
                  defaultMessage="· 事件数"
                />{' '}
                <Tag color="blue">{events.length}</Tag>
              </span>
            ) : (
              <span>
                {' '}
                ·{' '}
                <Tag>
                  <FormattedMessage
                    id="pages.opsAnalyticsFilters.events.allAllowed"
                    defaultMessage="全部允许"
                  />
                </Tag>
              </span>
            )}
            <span>
              {' '}
              <FormattedMessage
                id="pages.opsAnalyticsFilters.summary.sample"
                defaultMessage="· 采样"
              />
              <Tag color={sampleGlobal < 100 ? 'gold' : 'green'}>{sampleGlobal}%</Tag>
            </span>
            <span>
              {' '}
              <FormattedMessage
                id="pages.opsAnalyticsFilters.summary.payments"
                defaultMessage="· 支付"
              />
              <Tag color={paymentsEnabled ? 'green' : 'red'}>
                {paymentsEnabled
                  ? intl.formatMessage({
                      id: 'pages.opsAnalyticsFilters.payments.stateEnabled',
                      defaultMessage: '启用',
                    })
                  : intl.formatMessage({
                      id: 'pages.opsAnalyticsFilters.payments.stateDisabled',
                      defaultMessage: '禁用',
                    })}
              </Tag>
            </span>
          </div>
        </Space>
      </Card>
    </div>
  );
}
