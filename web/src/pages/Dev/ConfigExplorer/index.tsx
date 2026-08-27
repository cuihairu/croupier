import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  App,
  Button,
  Empty,
  Input,
  Modal,
  Popconfirm,
  Select,
  Space,
  Spin,
  Table,
  Tag,
  Tree,
  Typography,
} from 'antd';
import type { DataNode } from 'antd/es/tree';
import {
  DatabaseOutlined,
  FileTextOutlined,
  FolderOutlined,
  GithubOutlined,
  ReloadOutlined,
  SaveOutlined,
  SettingOutlined,
} from '@ant-design/icons';
import { PageContainer } from '@ant-design/pro-components';
import * as XLSX from 'xlsx';
import { CodeEditor } from '@/components/MonacoDynamic';
import {
  listConfigSources,
  listConfigTree,
  readConfigFile,
  writeConfigFile,
  type ConfigExplorerFile,
  type ConfigSourceBinding,
} from '@/services/api/configExplorer';
import { listGamesMeta, type Game } from '@/services/api/games';
import { useAccess } from '@umijs/max';
import SourceManageModal from './SourceManageModal';

const { Text } = Typography;

// 各数据源类型的展示元信息（图标/说明/配置模板）
const SOURCE_TYPE_META: Record<
  ConfigSourceBinding['type'],
  { label: string; icon: React.ReactNode; desc: string }
> = {
  git: { label: 'Git 仓库', icon: <GithubOutlined />, desc: '只读浏览分支目录' },
  redis: { label: 'Redis', icon: <DatabaseOutlined />, desc: 'key 前缀目录（skynet 惯例）' },
  nacos: { label: 'Nacos', icon: <DatabaseOutlined />, desc: 'dataId 即路径' },
  db: { label: '数据库', icon: <DatabaseOutlined />, desc: '表即文件（CSV 视图）' },
  croupier: { label: 'Croupier', icon: <DatabaseOutlined />, desc: 'ConfigVersion 版本库' },
};

// Monaco 语言映射（文本格式）
function langOf(format: string): string {
  switch (format) {
    case 'json':
    case 'yaml':
    case 'xml':
    case 'ini':
    case 'lua':
    case 'python':
      return format;
    case 'yml':
      return 'yaml';
    case 'py':
      return 'python';
    default:
      return 'plaintext';
  }
}

// 文件大小展示
function humanSize(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / 1024 / 1024).toFixed(1)} MB`;
}

type XlsxPreviewData = { columns: string[]; rows: string[][] };

export default function ConfigExplorer() {
  const { message } = App.useApp();
  const access = useAccess();
  const [games, setGames] = useState<Game[]>([]);
  const [game, setGame] = useState<string>('');
  const [env, setEnv] = useState<string>('');
  const [sources, setSources] = useState<ConfigSourceBinding[]>([]);
  const [sourceId, setSourceId] = useState<number | undefined>();
  const [treeData, setTreeData] = useState<DataNode[]>([]);
  const [treeLoading, setTreeLoading] = useState(false);
  const [file, setFile] = useState<ConfigExplorerFile | null>(null);
  const [fileLoading, setFileLoading] = useState(false);
  const [editText, setEditText] = useState<string>('');
  const [saveOpen, setSaveOpen] = useState(false);
  const [reason, setReason] = useState('');
  const [saving, setSaving] = useState(false);
  const [manageOpen, setManageOpen] = useState(false);
  const [xlsx, setXlsx] = useState<XlsxPreviewData | null>(null);

  const envs = useMemo(() => {
    const g = games.find((x) => x.name === game);
    return (g?.envs || []).map((v) => ({ label: v, value: v }));
  }, [games, game]);

  const currentSource = useMemo(() => sources.find((s) => s.id === sourceId), [sources, sourceId]);

  // 加载游戏列表
  useEffect(() => {
    listGamesMeta().then(({ games: list }) => {
      setGames(list);
      if (list.length > 0 && list[0]?.name) setGame(list[0].name);
    });
  }, []);

  // 加载数据源列表
  const loadSources = useCallback(
    async (g: string, e: string) => {
      if (!g || !e) {
        setSources([]);
        return;
      }
      try {
        const { items } = await listConfigSources({ gameId: g, env: e });
        setSources(items);
        setSourceId((prev) => (items.some((s) => s.id === prev) ? prev : items[0]?.id));
      } catch {
        message.error('加载数据源失败');
      }
    },
    [message],
  );

  useEffect(() => {
    if (game && env) void loadSources(game, env);
  }, [game, env, loadSources]);

  // 目录树：懒加载子节点
  const loadDir = useCallback(
    async (dir: string): Promise<DataNode[]> => {
      if (!sourceId) return [];
      const { items } = await listConfigTree(sourceId, dir);
      return items.map((e) => ({
        key: e.path,
        title: e.dir ? (
          <Space size={4}>
            <FolderOutlined />
            <span>{e.name}</span>
          </Space>
        ) : (
          <Space size={4}>
            <FileTextOutlined />
            <span>{e.name}</span>
          </Space>
        ),
        isLeaf: !e.dir,
        dir: e.dir,
      }));
    },
    [sourceId],
  );

  const reloadTree = useCallback(async () => {
    if (!sourceId) {
      setTreeData([]);
      return;
    }
    setTreeLoading(true);
    try {
      setTreeData(await loadDir(''));
    } catch {
      message.error('加载目录失败');
    } finally {
      setTreeLoading(false);
    }
  }, [loadDir, message, sourceId]);

  useEffect(() => {
    setTreeData([]);
    setFile(null);
    void reloadTree();
  }, [reloadTree]);

  // 打开文件
  const openFile = useCallback(
    async (path: string) => {
      if (!sourceId) return;
      setFileLoading(true);
      setXlsx(null);
      try {
        const resp = await readConfigFile(sourceId, path);
        setFile(resp);
        setEditText(resp.text ?? '');
        if (resp.base64 && resp.format === 'xlsx') {
          const bin = Uint8Array.from(atob(resp.base64), (c) => c.charCodeAt(0));
          const wb = XLSX.read(bin, { type: 'array' });
          const sheet = wb.Sheets[wb.SheetNames[0]];
          if (sheet) {
            const rows = XLSX.utils.sheet_to_json<unknown[]>(sheet, {
              header: 1,
              blankrows: false,
            });
            const columns = (rows[0] || []).map((_, i) => `列 ${i + 1}`);
            const dataRows = rows.map((r) => (r || []).map((c) => String(c ?? '')));
            setXlsx({ columns, rows: dataRows });
          }
        }
      } catch {
        message.error('读取文件失败');
      } finally {
        setFileLoading(false);
      }
    },
    [message, sourceId],
  );

  // 应急保存
  const doSave = async () => {
    if (!sourceId || !file) return;
    if (!reason.trim()) {
      message.warning('应急原因必填（将记入审计）');
      return;
    }
    setSaving(true);
    try {
      await writeConfigFile({
        sourceId,
        path: file.path,
        content: editText,
        reason: reason.trim(),
      });
      message.success('已写回');
      setSaveOpen(false);
      setReason('');
      await openFile(file.path);
    } catch (err) {
      const msg = err instanceof Error ? err.message : '写回失败';
      message.error(msg);
    } finally {
      setSaving(false);
    }
  };

  const canManage = access.canDevManage;

  return (
    <PageContainer
      extra={[
        <Button
          key="manage"
          icon={<SettingOutlined />}
          disabled={!canManage || !game || !env}
          onClick={() => setManageOpen(true)}
        >
          管理数据源
        </Button>,
        <Button key="reload" icon={<ReloadOutlined />} onClick={() => void reloadTree()}>
          刷新
        </Button>,
      ]}
    >
      <Space style={{ marginBottom: 12 }} wrap>
        <Select
          style={{ width: 180 }}
          placeholder="选择游戏"
          value={game || undefined}
          onChange={setGame}
          options={games.map((g) => ({
            label: g.displayName || g.name || '',
            value: g.name || '',
          }))}
        />
        <Select
          style={{ width: 120 }}
          placeholder="环境"
          value={env || undefined}
          onChange={setEnv}
          options={envs}
        />
        <Select
          style={{ width: 240 }}
          placeholder="数据源"
          value={sourceId}
          onChange={setSourceId}
          options={sources.map((s) => ({
            label: (
              <Space>
                {SOURCE_TYPE_META[s.type].icon}
                <span>{s.name}</span>
                <Tag color={s.writable ? 'orange' : 'default'}>{s.writable ? '可写' : '只读'}</Tag>
              </Space>
            ),
            value: s.id,
          }))}
        />
        {currentSource && (
          <Text type="secondary">
            {SOURCE_TYPE_META[currentSource.type].label} ·{' '}
            {SOURCE_TYPE_META[currentSource.type].desc}
          </Text>
        )}
      </Space>

      <div style={{ display: 'flex', gap: 12, minHeight: 480 }}>
        <div style={{ width: 280, flexShrink: 0, overflow: 'auto' }}>
          <Spin spinning={treeLoading}>
            {sources.length === 0 ? (
              <Empty description="暂无数据源，请先管理数据源添加" />
            ) : (
              <Tree
                treeData={treeData}
                loadData={async (node) => {
                  const children = await loadDir(String(node.key));
                  setTreeData((prev) => updateChildren(prev, String(node.key), children));
                }}
                onSelect={(keys) => {
                  const key = Array.isArray(keys) ? keys[0] : keys;
                  if (typeof key === 'string') void openFile(key);
                }}
                defaultExpandAll={false}
              />
            )}
          </Spin>
        </div>

        <div style={{ flex: 1, minWidth: 0 }}>
          <Spin spinning={fileLoading}>
            {!file ? (
              <Empty description="选择左侧文件查看在线配置" style={{ marginTop: 160 }} />
            ) : xlsx ? (
              <Table
                size="small"
                rowKey={(_, i) => String(i)}
                columns={xlsx.columns.map((c) => ({ title: c, dataIndex: c, ellipsis: true }))}
                dataSource={xlsx.rows.map((r) => {
                  const row: Record<string, string> = {};
                  xlsx.columns.forEach((c, i) => {
                    row[c] = r[i] ?? '';
                  });
                  return row;
                })}
                pagination={{ pageSize: 20, showSizeChanger: false }}
                scroll={{ x: 'max-content' }}
              />
            ) : (
              <Space direction="vertical" style={{ width: '100%' }}>
                <Space>
                  <Text strong>{file.path}</Text>
                  <Tag>{file.format}</Tag>
                  <Text type="secondary">{humanSize(file.size)}</Text>
                  {file.writable && canManage && (
                    <Popconfirm
                      title="应急编辑并写回？"
                      description="改动会直接写回配置中心（各项目配置流程不变），原因将记入审计。"
                      onConfirm={() => setSaveOpen(true)}
                    >
                      <Button type="primary" danger icon={<SaveOutlined />}>
                        应急编辑
                      </Button>
                    </Popconfirm>
                  )}
                </Space>
                <CodeEditor
                  value={editText}
                  onChange={setEditText}
                  language={langOf(file.format)}
                  height={520}
                  readOnly={!file.writable || !canManage}
                  options={{
                    minimap: { enabled: false },
                    wordWrap: 'on',
                    readOnly: !file.writable || !canManage,
                  }}
                />
              </Space>
            )}
          </Spin>
        </div>
      </div>

      <Modal
        open={saveOpen}
        title="应急写回"
        okText="写回"
        okButtonProps={{ danger: true, loading: saving }}
        onCancel={() => setSaveOpen(false)}
        onOk={doSave}
        destroyOnHidden
      >
        <Space direction="vertical" style={{ width: '100%' }}>
          <Text type="secondary">
            将写回 <Text code>{currentSource?.name}</Text>（{currentSource?.type}
            ）的 <Text code>{file?.path}</Text>；各项目配置流程不变，原因必填并记入审计。
          </Text>
          <Input.TextArea
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            maxLength={200}
            placeholder="应急原因（必填，如：线上活动奖励配置错误紧急修正）"
            rows={3}
          />
        </Space>
      </Modal>

      {manageOpen && (
        <SourceManageModal
          open={manageOpen}
          gameId={game}
          env={env}
          onClose={() => setManageOpen(false)}
          onChanged={() => void loadSources(game, env)}
        />
      )}
    </PageContainer>
  );
}

// 递归挂载子节点
function updateChildren(list: DataNode[], key: string, children: DataNode[]): DataNode[] {
  return list.map((node) => {
    if (String(node.key) === key) {
      return { ...node, children };
    }
    if (node.children) {
      return { ...node, children: updateChildren(node.children, key, children) };
    }
    return node;
  });
}
