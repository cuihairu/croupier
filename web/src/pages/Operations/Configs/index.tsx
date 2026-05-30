import React from 'react';
import {
  Card,
  Table,
  Space,
  Tag,
  Select,
  Input,
  Button,
  Modal,
  Form,
  Input as AntInput,
} from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { CodeEditor, DiffEditor as MonacoDiff } from '@/components/MonacoDynamic';
import useConfigsPage from './useConfigsPage';
import { DiffView, hasMonaco, langOf } from './diff';
import {
  buildConfigColumns,
  buildVersionColumns,
  CONFIG_FORMAT_OPTIONS,
  CONFIGS_TOOLBAR_SCHEMA,
  type ConfigToolbarActionKey,
} from './schema';

export default function OperationsConfigsPage() {
  const {
    loading,
    rows,
    game,
    setGame,
    env,
    setEnv,
    format,
    setFormat,
    q,
    setQ,
    cur,
    setCur,
    verOpen,
    setVerOpen,
    versions,
    saveOpen,
    setSaveOpen,
    saveMsg,
    setSaveMsg,
    diffOpen,
    setDiffOpen,
    diffLeft,
    diffRight,
    games,
    envs,
    load,
    openItem,
    validate,
    doSave,
    openVersions,
    viewVersion,
    diffWithVersion,
    rollbackTo,
  } = useConfigsPage();

  const cols = buildConfigColumns(openItem);
  const versionCols = buildVersionColumns(viewVersion, diffWithVersion, rollbackTo);

  const runToolbarAction = (key: ConfigToolbarActionKey) => {
    if (key === 'query') return load();
    setGame('');
    setEnv('');
    setFormat('');
    setQ('');
    return load();
  };

  const csvPreview = (txt: string) => {
    const lines = txt
      .replace(/\r\n/g, '\n')
      .split('\n')
      .filter((l) => l.trim().length > 0);
    const rows = lines.map((l) => l.split(','));
    return (
      <div style={{ maxHeight: 280, overflow: 'auto', border: '1px solid #f0f0f0' }}>
        <table style={{ width: '100%', borderCollapse: 'collapse' }}>
          <tbody>
            {rows.map((r, i) => (
              <tr key={i}>
                {r.map((c, j) => (
                  <td key={j} style={{ border: '1px solid #eee', padding: '4px 6px' }}>
                    {c}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    );
  };

  return (
    <PageContainer>
      <Card
        title="配置管理"
        extra={
          <Space>
            <Select
              allowClear
              placeholder={CONFIGS_TOOLBAR_SCHEMA.filters[0].placeholder}
              value={game}
              onChange={setGame as any}
              style={{ width: CONFIGS_TOOLBAR_SCHEMA.filters[0].width }}
              options={games}
            />
            <Select
              allowClear
              placeholder={CONFIGS_TOOLBAR_SCHEMA.filters[1].placeholder}
              value={env}
              onChange={setEnv as any}
              style={{ width: CONFIGS_TOOLBAR_SCHEMA.filters[1].width }}
              options={envs}
            />
            <Select
              allowClear
              placeholder={CONFIGS_TOOLBAR_SCHEMA.filters[2].placeholder}
              value={format}
              onChange={setFormat as any}
              style={{ width: CONFIGS_TOOLBAR_SCHEMA.filters[2].width }}
              options={CONFIG_FORMAT_OPTIONS}
            />
            <Space.Compact style={{ width: CONFIGS_TOOLBAR_SCHEMA.filters[3].width }}>
              <Input
                allowClear
                placeholder={CONFIGS_TOOLBAR_SCHEMA.filters[3].placeholder}
                value={q}
                onChange={(e) => setQ(e.target.value)}
                onPressEnter={() => runToolbarAction('query')}
              />
              {CONFIGS_TOOLBAR_SCHEMA.actions.map((action) => (
                <Button
                  key={action.key}
                  type={action.primary ? 'primary' : 'default'}
                  onClick={() => runToolbarAction(action.key)}
                >
                  {action.label}
                </Button>
              ))}
            </Space.Compact>
          </Space>
        }
      >
        <Table
          rowKey={(r) => `${r.gameId}|${r.env}|${r.id}`}
          dataSource={rows}
          columns={cols}
          loading={loading}
          size="small"
          pagination={{ pageSize: 10 }}
        />
      </Card>

      <Modal
        open={!!cur}
        onCancel={() => setCur(null)}
        width={960}
        footer={null}
        title={cur ? `${cur.id} (${cur.format}) v${cur.version || ''}` : ''}
        destroyOnHidden
      >
        {cur && (
          <Space direction="vertical" style={{ width: '100%' }}>
            <Space>
              <Select
                value={cur.format}
                onChange={(v) => setCur({ ...cur, format: v })}
                options={CONFIG_FORMAT_OPTIONS}
              />
              <Button onClick={validate}>校验</Button>
              <Button onClick={openVersions}>历史版本</Button>
              <Button type="primary" onClick={() => setSaveOpen(true)}>
                保存新版本
              </Button>
            </Space>
            {cur.format === 'csv' && csvPreview(cur.content)}
            <CodeEditor
              value={cur.content}
              onChange={(v) => setCur({ ...cur, content: v })}
              language={langOf(cur.format)}
              height={420}
            />
          </Space>
        )}
      </Modal>

      <Modal
        open={saveOpen}
        title="保存版本"
        onCancel={() => setSaveOpen(false)}
        onOk={doSave}
        destroyOnHidden
      >
        <Form layout="vertical">
          <Form.Item label="版本说明">
            <AntInput
              value={saveMsg}
              onChange={(e) => setSaveMsg(e.target.value)}
              maxLength={200}
              placeholder="本次修改原因（必填）"
            />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        open={verOpen}
        title="历史版本"
        onCancel={() => setVerOpen(false)}
        footer={null}
        destroyOnHidden
      >
        <Table
          size="small"
          rowKey={(r) => String(r.version)}
          dataSource={versions}
          columns={versionCols}
        />
      </Modal>

      <Modal
        open={diffOpen}
        title="版本对比"
        onCancel={() => setDiffOpen(false)}
        footer={null}
        width={980}
        destroyOnHidden
      >
        <div>
          <MonacoDiff
            left={diffLeft}
            right={diffRight}
            language={langOf(cur?.format || '')}
            height={420}
          />
          <div style={{ display: 'none' }} />
        </div>
        {!hasMonaco() && <DiffView left={diffLeft} right={diffRight} />}
      </Modal>
    </PageContainer>
  );
}
