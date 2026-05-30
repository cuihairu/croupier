import { history, useAccess, useLocation } from '@umijs/max';
import {
  DASHBOARD_PAGE_TOKENS,
  PageStatePanel,
  StandardFilterBar,
  StandardListSection,
  SummaryOverview,
} from '@/components';
import {
  Alert,
  AutoComplete,
  Badge,
  Button,
  Card,
  Collapse,
  Dropdown,
  Empty,
  Form,
  Input,
  List,
  Modal,
  Progress,
  Select,
  Space,
  Tooltip,
  Typography,
  message,
} from 'antd';
import { useEffect, useMemo, useState } from 'react';
import {
  CopyOutlined,
  DeleteOutlined,
  EditOutlined,
  GlobalOutlined,
  MoreOutlined,
  PlusOutlined,
  ReloadOutlined,
  StopOutlined,
} from '@ant-design/icons';
import {
  cloneWorkspaceConfig,
  deleteWorkspaceConfig,
  listWorkspaceConfigs,
  listWorkspaceVersions,
  publishWorkspaceConfig,
  unpublishWorkspaceConfig,
} from '@/services/workspaceConfig';
import type { WorkspaceConfig } from '@/types/workspace';
import { listDescriptors } from '@/services/api/functions';
import { trackWorkspaceEvent } from '@/services/workspace/telemetry';
import { buildWorkspaceQualityReport } from '@/services/workspace/quality';
import {
  getWorkspaceErrorMessage,
  parseWorkspaceError,
  type WorkspaceErrorCode,
} from '@/services/workspace/errors';

function resolveWorkspaceStatus(config: WorkspaceConfig): 'draft' | 'published' | 'archived' {
  if (config.status) return config.status;
  return config.published ? 'published' : 'draft';
}

function buildPublishChangeSummary(
  draftConfig?: WorkspaceConfig,
  publishedConfig?: WorkspaceConfig,
): string {
  if (!draftConfig) return '缺少当前草稿配置';
  if (!publishedConfig) return '当前无已发布版本，本次发布将作为首个发布版本';

  const changes: string[] = [];
  if ((draftConfig.title || '') !== (publishedConfig.title || '')) {
    changes.push('标题已变更');
  }
  if ((draftConfig.description || '') !== (publishedConfig.description || '')) {
    changes.push('描述已变更');
  }
  if ((draftConfig.layout?.type || '-') !== (publishedConfig.layout?.type || '-')) {
    changes.push(
      `布局 ${publishedConfig.layout?.type || '-'} -> ${draftConfig.layout?.type || '-'}`,
    );
  }
  const draftTabs = draftConfig.layout?.tabs?.length || 0;
  const publishedTabs = publishedConfig.layout?.tabs?.length || 0;
  if (draftTabs !== publishedTabs) {
    changes.push(`标签页数 ${publishedTabs} -> ${draftTabs}`);
  }
  return changes.length > 0 ? changes.join('；') : '与当前发布版本结构一致';
}

type CreateWorkspaceFormValues = {
  objectKey: string;
};

type CloneWorkspaceFormValues = {
  sourceKey: string;
  targetKey: string;
  targetTitle: string;
};

function buildSuggestedCloneKey(sourceKey: string, existingKeys: string[]): string {
  if (!sourceKey) return '';
  const keySet = new Set(existingKeys);
  const baseKey = `${sourceKey}-copy`;
  if (!keySet.has(baseKey)) return baseKey;
  let index = 2;
  while (keySet.has(`${baseKey}-${index}`)) {
    index += 1;
  }
  return `${baseKey}-${index}`;
}

function summarizeWorkspaceReadiness(config: WorkspaceConfig, availableFunctions: any[] = []): {
  readyCount: number;
  pendingCount: number;
  summary: string;
} {
  const quality = buildWorkspaceQualityReport(config, availableFunctions);
  if (quality.tabs.length === 0) {
    return {
      readyCount: 0,
      pendingCount: 0,
      summary: '还没有页面',
    };
  }
  return {
    readyCount: quality.readyTabCount,
    pendingCount: quality.pendingTabCount,
    summary:
      quality.blockingCount > 0
        ? `${quality.blockingCount} 个阻塞项待处理`
        : quality.warningCount > 0
        ? `${quality.warningCount} 个风险提示待确认`
        : `全部 ${quality.readyTabCount} 个页面已具备基础条件`,
  };
}

function getWorkspaceCompletion(readiness: {
  readyCount: number;
  pendingCount: number;
  summary: string;
}) {
  const total = readiness.readyCount + readiness.pendingCount;
  if (total <= 0) {
    return {
      total: 0,
      percent: 0,
      label: '未开始',
      status: 'exception' as const,
    };
  }

  const percent = Math.round((readiness.readyCount / total) * 100);
  if (readiness.pendingCount === 0) {
    return {
      total,
      percent: 100,
      label: '可发布',
      status: 'success' as const,
    };
  }

  return {
    total,
    percent,
    label: '待完善',
    status: 'active' as const,
  };
}

function getWorkspaceStatusMeta(status: 'draft' | 'published' | 'archived') {
  if (status === 'published') {
    return {
      badgeStatus: 'success' as const,
      badgeText: '已发布',
      color: '#52c41a',
    };
  }
  if (status === 'archived') {
    return {
      badgeStatus: 'warning' as const,
      badgeText: '已归档',
      color: '#faad14',
    };
  }
  return {
    badgeStatus: 'default' as const,
    badgeText: '草稿',
    color: '#8c8c8c',
  };
}

function getWorkspacePriorityHint(params: {
  status: 'draft' | 'published' | 'archived';
  readiness: { readyCount: number; pendingCount: number; summary: string };
  qualityScore?: number;
  blockingCount?: number;
  warningCount?: number;
}) {
  const { status, readiness, qualityScore = 0, blockingCount = 0, warningCount = 0 } = params;
  if (status === 'archived') {
    return {
      tone: '#8c8c8c',
      text: '已归档，当前不参与发布流转。',
    };
  }
  if (blockingCount > 0) {
    return {
      tone: '#cf1322',
      text: `当前发布评分 ${qualityScore}/100，仍有 ${blockingCount} 个硬阻塞，先补齐后再发布。`,
    };
  }
  if (warningCount > 0) {
    return {
      tone: '#d46b08',
      text: `当前发布评分 ${qualityScore}/100，暂无硬阻塞，但仍有 ${warningCount} 个风险提示待确认。`,
    };
  }
  if (readiness.readyCount === 0 && readiness.pendingCount === 0) {
    return {
      tone: '#d46b08',
      text: '还没有页面骨架，建议先生成首个 Tab。',
    };
  }
  if (readiness.pendingCount > 0) {
    return {
      tone: '#d46b08',
      text: `还有 ${readiness.pendingCount} 个页面待补齐，优先继续完善页面骨架。`,
    };
  }
  if (status === 'published') {
    return {
      tone: '#389e0d',
      text: '页面已齐备且已发布，可直接去控制台验证运行结果。',
    };
  }
  return {
    tone: '#1677ff',
    text: '页面已齐备，下一步建议发布到运行控制台。',
  };
}

type WorkspaceStarter = {
  objectKey: string;
  label: string;
  functionCount: number;
};

type PublishReviewState = {
  objectKey: string;
  title: string;
  tabCount: number;
  score: number;
  readinessSummary: string;
  qualitySummary: string;
  warningCount: number;
  changeSummary: string;
};

function buildWorkspaceStepSummary(params: {
  hasRegisteredFunctions: boolean;
  hasWorkspaceDraft: boolean;
  hasRenderablePages: boolean;
}) {
  const { hasRegisteredFunctions, hasWorkspaceDraft, hasRenderablePages } = params;

  if (!hasRegisteredFunctions) {
    return {
      type: 'warning' as const,
      title: '1. 先挑主函数',
      description: '先在函数目录把可用函数注册好，再回到这里进入页面编排。',
    };
  }

  if (!hasWorkspaceDraft) {
    return {
      type: 'info' as const,
      title: '2. 起页面骨架',
      description: '函数已经有了，下一步直接进入对象工作台，为对象创建首个页面草稿。',
    };
  }

  if (!hasRenderablePages) {
    return {
      type: 'warning' as const,
      title: '2. 补页面骨架',
      description: '工作台草稿已经存在，但还有页面缺主函数、核心函数或基础结构。',
    };
  }

  return {
    type: 'success' as const,
    title: '3. 看预览并微调',
    description: '页面已经具备基础条件，可以去预览检查效果，再决定是否发布到控制台。',
  };
}

export default function WorkspacesIndexPage() {
  const access = useAccess() as any;
  const location = useLocation();
  const [loading, setLoading] = useState(false);
  const [configs, setConfigs] = useState<WorkspaceConfig[]>([]);
  const [error, setError] = useState('');
  const [errorCode, setErrorCode] = useState<WorkspaceErrorCode | undefined>();
  const [actionKey, setActionKey] = useState('');
  const [keyword, setKeyword] = useState('');
  const [statusFilter, setStatusFilter] = useState<'all' | 'published' | 'draft' | 'archived'>(
    'all',
  );
  const [sortBy, setSortBy] = useState<'updated_desc' | 'title_asc'>('updated_desc');
  const [createVisible, setCreateVisible] = useState(false);
  const [cloneVisible, setCloneVisible] = useState(false);
  const [createSubmitting, setCreateSubmitting] = useState(false);
  const [cloneSubmitting, setCloneSubmitting] = useState(false);
  const [qualityInspectKey, setQualityInspectKey] = useState('');
  const [publishReview, setPublishReview] = useState<PublishReviewState | null>(null);
  const [objectOptions, setObjectOptions] = useState<{ label: string; value: string }[]>([]);
  const [workspaceStarters, setWorkspaceStarters] = useState<WorkspaceStarter[]>([]);
  const [availableFunctions, setAvailableFunctions] = useState<any[]>([]);
  const [objectOptionsLoading, setObjectOptionsLoading] = useState(false);
  const [createForm] = Form.useForm<CreateWorkspaceFormValues>();
  const [cloneForm] = Form.useForm<CloneWorkspaceFormValues>();
  const starterContext = useMemo(() => {
    const search = new URLSearchParams(location.search);
    const objectKey = search.get('objectKey')?.trim() || '';
    const functionId = search.get('functionId')?.trim() || '';
    const from = search.get('from')?.trim() || '';
    return {
      objectKey,
      functionId,
      from,
    };
  }, [location.search]);

  const load = async () => {
    setLoading(true);
    setError('');
    setErrorCode(undefined);
    try {
      const rows = await listWorkspaceConfigs();
      trackWorkspaceEvent('workspace_load', {
        scope: 'workspaces_page',
        count: Array.isArray(rows) ? rows.length : 0,
      });
      setConfigs(Array.isArray(rows) ? rows : []);
    } catch (err: any) {
      const parsedError = parseWorkspaceError(err);
      trackWorkspaceEvent('workspace_load_error', {
        scope: 'workspaces_page',
        error: err?.message || String(err),
        code: parsedError.code,
      });
      setErrorCode(parsedError.code);
      setError(getWorkspaceErrorMessage(err, '加载对象工作台失败'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    trackWorkspaceEvent('workspace_page_open', {
      page: 'workspaces_index',
    });
    load().catch(() => {});
    loadObjectOptions().catch(() => {});
  }, []);

  const summary = useMemo(() => {
    const publishedCount = configs.filter(
      (item) => resolveWorkspaceStatus(item) === 'published',
    ).length;
    const draftCount = configs.filter((item) => resolveWorkspaceStatus(item) === 'draft').length;
    const archivedCount = configs.filter(
      (item) => resolveWorkspaceStatus(item) === 'archived',
    ).length;
    return {
      total: configs.length,
      publishedCount,
      draftCount,
      archivedCount,
    };
  }, [configs]);

  const visibleConfigs = useMemo(() => {
    const normalizedKeyword = keyword.trim().toLowerCase();
    const filtered = configs.filter((config) => {
      const status = resolveWorkspaceStatus(config);
      if (statusFilter === 'published' && status !== 'published') return false;
      if (statusFilter === 'draft' && status !== 'draft') return false;
      if (statusFilter === 'archived' && status !== 'archived') return false;

      if (!normalizedKeyword) return true;
      const title = (config.title || '').toLowerCase();
      const objectKey = (config.objectKey || '').toLowerCase();
      const description = (config.description || '').toLowerCase();
      return (
        title.includes(normalizedKeyword) ||
        objectKey.includes(normalizedKeyword) ||
        description.includes(normalizedKeyword)
      );
    });

    const sortable = [...filtered];
    if (sortBy === 'title_asc') {
      sortable.sort((a, b) => (a.title || '').localeCompare(b.title || ''));
      return sortable;
    }

    sortable.sort((a, b) => {
      const aTime = new Date(a.meta?.updatedAt || 0).getTime();
      const bTime = new Date(b.meta?.updatedAt || 0).getTime();
      return bTime - aTime;
    });
    return sortable;
  }, [configs, keyword, statusFilter, sortBy]);

  const unconfiguredStarters = useMemo(() => {
    const existingKeys = new Set(configs.map((item) => item.objectKey));
    return workspaceStarters.filter((item) => !existingKeys.has(item.objectKey));
  }, [configs, workspaceStarters]);

  const starterContextMeta = useMemo(() => {
    if (!starterContext.objectKey) return null;
    const existingConfig = configs.find((item) => item.objectKey === starterContext.objectKey) || null;
    const starter = workspaceStarters.find((item) => item.objectKey === starterContext.objectKey) || null;
    return {
      ...starterContext,
      existingConfig,
      starter,
    };
  }, [configs, starterContext, workspaceStarters]);

  const qualityInspectTarget = useMemo(() => {
    if (!qualityInspectKey) return null;
    const target = configs.find((item) => item.objectKey === qualityInspectKey);
    if (!target) return null;
    return {
      config: target,
      report: buildWorkspaceQualityReport(target, availableFunctions),
    };
  }, [availableFunctions, configs, qualityInspectKey]);

  const workspaceStepSummary = useMemo(
    () =>
      buildWorkspaceStepSummary({
        hasRegisteredFunctions: workspaceStarters.length > 0,
        hasWorkspaceDraft: configs.length > 0,
        hasRenderablePages: configs.some(
          (item) => summarizeWorkspaceReadiness(item, availableFunctions).readyCount > 0,
        ),
      }),
    [configs, workspaceStarters, availableFunctions],
  );

  const loadObjectOptions = async () => {
    if (objectOptions.length > 0 && workspaceStarters.length > 0) return;
    setObjectOptionsLoading(true);
    try {
      const descriptors = await listDescriptors();
      const keyMeta = new Map<string, WorkspaceStarter>();
      descriptors.forEach((descriptor: any) => {
        const id = String(descriptor?.id || '');
        const entity = String(descriptor?.entity || '').trim();
        const objectKey = entity || (id.includes('.') ? id.split('.')[0] : '');
        if (!objectKey) return;
        const label = String(
          descriptor?.entityDisplay?.zh ||
            descriptor?.entityDisplay?.en ||
            descriptor?.displayName?.zh ||
            descriptor?.displayName?.en ||
            objectKey,
        ).trim();
        const current = keyMeta.get(objectKey);
        if (current) {
          current.functionCount += 1;
          return;
        }
        keyMeta.set(objectKey, {
          objectKey,
          label,
          functionCount: 1,
        });
      });
      const starterList = Array.from(keyMeta.values()).sort((a, b) =>
        a.objectKey.localeCompare(b.objectKey),
      );
      setAvailableFunctions(descriptors);
      setWorkspaceStarters(starterList);
      setObjectOptions(
        starterList.map((item) => ({
          label:
            item.label === item.objectKey ? item.objectKey : `${item.label} (${item.objectKey})`,
          value: item.objectKey,
        })),
      );
    } catch {
      setAvailableFunctions([]);
      setObjectOptions([]);
      setWorkspaceStarters([]);
    } finally {
      setObjectOptionsLoading(false);
    }
  };

  const handlePublish = async (objectKey: string) => {
    if (!access?.canWorkspacePublish) {
      message.error('无发布权限');
      return;
    }
    const target = configs.find((item) => item.objectKey === objectKey);
    const readiness = target ? summarizeWorkspaceReadiness(target, availableFunctions) : undefined;
    const quality = target ? buildWorkspaceQualityReport(target, availableFunctions) : undefined;
    let publishChangeSummary = '发布后会在控制台向有权限用户可见。';
    try {
      const versions = await listWorkspaceVersions(objectKey);
      const currentPublished = versions.find((item) => item.isCurrentPublished)?.config;
      publishChangeSummary = buildPublishChangeSummary(target, currentPublished);
    } catch {
      publishChangeSummary = '版本差异摘要暂不可用，本次将按当前草稿发布。';
    }
    if (quality && quality.blockingCount > 0) {
      Modal.warning({
        title: '当前还不能发布',
        content: (
          <div>
            <div>{`对象: ${target?.objectKey || objectKey}`}</div>
            <div>{`发布评分: ${quality.score}/100`}</div>
            <div>{`阻塞项: ${quality.blockingCount}`}</div>
            <div>{`风险项: ${quality.warningCount}`}</div>
            <div style={{ marginTop: 8 }}>{quality.summary}</div>
            <div style={{ marginTop: 8, color: '#cf1322' }}>
              {quality.items
                .filter((item) => item.level === 'blocking')
                .slice(0, 4)
                .map((item) => item.detail)
                .join('；')}
            </div>
          </div>
        ),
      });
      return;
    }

    setPublishReview({
      objectKey,
      title: target?.title || '-',
      tabCount: target?.layout?.tabs?.length || 0,
      score: quality?.score || 0,
      readinessSummary: readiness?.summary || '未知',
      qualitySummary: quality?.summary || '当前质量摘要暂不可用',
      warningCount: quality?.warningCount || 0,
      changeSummary: publishChangeSummary,
    });
  };

  const handleConfirmPublish = async () => {
    if (!publishReview) return;
    const objectKey = publishReview.objectKey;
    setActionKey(objectKey);
    try {
      await publishWorkspaceConfig(objectKey);
      trackWorkspaceEvent('workspace_publish', { objectKey });
      setPublishReview(null);
      await load();
      message.success('发布成功，已出现在控制台菜单');
    } catch (err: any) {
      trackWorkspaceEvent('workspace_publish_error', {
        objectKey,
        error: err?.message || String(err),
      });
      message.error(getWorkspaceErrorMessage(err, '发布失败'));
    } finally {
      setActionKey('');
    }
  };

  const handleUnpublish = async (objectKey: string) => {
    if (!access?.canWorkspacePublish) {
      message.error('无发布权限');
      return;
    }
    const target = configs.find((item) => item.objectKey === objectKey);
    Modal.confirm({
      title: '确认取消发布',
      content: (
        <div>
          <div>{`对象: ${target?.objectKey || objectKey}`}</div>
          <div>{`标题: ${target?.title || '-'}`}</div>
          <div style={{ marginTop: 8, color: '#cf1322' }}>取消发布后将从控制台菜单移除。</div>
        </div>
      ),
      onOk: async () => {
        setActionKey(objectKey);
        try {
          await unpublishWorkspaceConfig(objectKey);
          trackWorkspaceEvent('workspace_unpublish', { objectKey });
          await load();
          message.success('已取消发布');
        } catch (err: any) {
          trackWorkspaceEvent('workspace_unpublish_error', {
            objectKey,
            error: err?.message || String(err),
          });
          message.error(getWorkspaceErrorMessage(err, '取消发布失败'));
        } finally {
          setActionKey('');
        }
      },
    });
  };

  const handleDelete = async (objectKey: string) => {
    if (!access?.canWorkspaceDelete) {
      message.error('无删除权限');
      return;
    }
    const target = configs.find((item) => item.objectKey === objectKey);
    const targetStatus = target ? resolveWorkspaceStatus(target) : '-';
    Modal.confirm({
      title: '确认删除',
      content: (
        <div>
          <div>{`对象: ${target?.objectKey || objectKey}`}</div>
          <div>{`标题: ${target?.title || '-'}`}</div>
          <div>{`状态: ${targetStatus}`}</div>
          <div style={{ marginTop: 8, color: '#cf1322' }}>删除后不可恢复，请谨慎操作。</div>
        </div>
      ),
      okButtonProps: { danger: true },
      okText: '确认删除',
      cancelText: '取消',
      onOk: async () => {
        setActionKey(objectKey);
        try {
          await deleteWorkspaceConfig(objectKey);
          trackWorkspaceEvent('workspace_delete', { objectKey });
          await load();
          message.success('删除成功');
        } catch (err: any) {
          trackWorkspaceEvent('workspace_delete_error', {
            objectKey,
            error: err?.message || String(err),
          });
          message.error(getWorkspaceErrorMessage(err, '删除失败'));
        } finally {
          setActionKey('');
        }
      },
    });
  };

  const handleOpenCreate = async () => {
    if (!access?.canWorkspaceEdit) {
      message.error('无创建权限');
      return;
    }
    await loadObjectOptions();
    createForm.resetFields();
    if (starterContext.objectKey) {
      createForm.setFieldsValue({ objectKey: starterContext.objectKey });
    }
    setCreateVisible(true);
  };

  const handleCreate = async () => {
    if (!access?.canWorkspaceEdit) {
      message.error('无创建权限');
      return;
    }
    const values = await createForm.validateFields();
    const objectKey = values.objectKey.trim();
    setCreateSubmitting(true);
    try {
      const existingConfig = configs.find((item) => item.objectKey === objectKey);
      if (existingConfig) {
        setCreateVisible(false);
        history.push(`/system/functions/workspace-editor/${encodeURIComponent(objectKey)}`);
        message.info('该 objectKey 已存在，已直接打开现有工作台');
        return;
      }
      trackWorkspaceEvent('workspace_create_entry', { objectKey, source: 'workspaces_index' });
      setCreateVisible(false);
      history.push(`/system/functions/workspace-editor/${encodeURIComponent(objectKey)}`);
      message.success('已进入工作台编辑器');
    } finally {
      setCreateSubmitting(false);
    }
  };

  const handleOpenClone = () => {
    if (!access?.canWorkspaceEdit) {
      message.error('无复制权限');
      return;
    }
    const sourceKey = visibleConfigs[0]?.objectKey || configs[0]?.objectKey || '';
    const suggestedTargetKey = buildSuggestedCloneKey(
      sourceKey,
      configs.map((item) => item.objectKey),
    );
    cloneForm.setFieldsValue({
      sourceKey,
      targetKey: suggestedTargetKey,
      targetTitle: sourceKey ? `${sourceKey} 副本` : '',
    });
    setCloneVisible(true);
  };

  const handleClone = async () => {
    if (!access?.canWorkspaceEdit) {
      message.error('无复制权限');
      return;
    }
    const values = await cloneForm.validateFields();
    setCloneSubmitting(true);
    try {
      const config = await cloneWorkspaceConfig(
        values.sourceKey.trim(),
        values.targetKey.trim(),
        values.targetTitle.trim(),
      );
      trackWorkspaceEvent('workspace_clone', {
        sourceKey: values.sourceKey.trim(),
        targetKey: values.targetKey.trim(),
      });
      message.success('复制成功');
      setCloneVisible(false);
      await load();
      history.push(`/system/functions/workspace-editor/${encodeURIComponent(config.objectKey)}`);
    } catch (err: any) {
      message.error(getWorkspaceErrorMessage(err, '复制失败'));
    } finally {
      setCloneSubmitting(false);
    }
  };

  if (!access?.canWorkspaceRead) {
    return (
      <PageStatePanel
        tone="error"
        badgeText="权限受限"
        title="无法进入对象工作台"
        description="你没有查看对象工作台的权限，需要先具备读取权限后才能继续页面装配。"
      />
    );
  }
  if (loading) return <Card loading />;
  if (errorCode === 'forbidden') {
    return (
      <PageStatePanel
        tone="error"
        badgeText="权限受限"
        title="无法进入对象工作台"
        description="当前账号没有查看对象工作台的权限，现有页面装配配置暂时不可见。"
      />
    );
  }
  if (error) {
    return (
      <PageStatePanel
        tone="warning"
        badgeText="加载异常"
        title="对象工作台列表加载失败"
        description={error}
      />
    );
  }

  return (
    <Space direction="vertical" size={16} style={{ width: '100%' }}>
      <SummaryOverview
        title="对象工作台"
        description="这里是页面装配主台。你需要在这里把函数能力组织成对象页面、补齐骨架、完成预览，并决定是否发布到控制台。"
        items={[
          { color: '#1677ff', text: `总数 ${summary.total}` },
          { color: '#52c41a', text: `已发布 ${summary.publishedCount}` },
          { color: '#d9d9d9', text: `草稿 ${summary.draftCount}` },
          { color: '#faad14', text: `已归档 ${summary.archivedCount}` },
        ]}
        hint="函数目录负责能力供给；对象工作台负责页面装配；运行控制台负责发布结果验证。"
      />

      <Alert
        type={workspaceStepSummary.type}
        showIcon
        message={`${workspaceStepSummary.title} · 当前建议动作`}
        action={
          unconfiguredStarters.length > 0 ? (
            <Button
              size="small"
              type="primary"
              onClick={() =>
                history.push(
                  `/system/functions/workspace-editor/${encodeURIComponent(
                    unconfiguredStarters[0].objectKey,
                  )}`,
                )
              }
            >
              {`去做 ${unconfiguredStarters[0].label} 的首个工作台页面`}
            </Button>
          ) : configs.length > 0 ? (
            <Button
              size="small"
              type="primary"
              onClick={() =>
                history.push(
                  `/system/functions/workspace-editor/${encodeURIComponent(
                    visibleConfigs[0]?.objectKey || configs[0].objectKey,
                  )}`,
                )
              }
            >
              进入最近工作台
            </Button>
          ) : null
        }
        description={
          <Space wrap size={[8, 8]}>
            <Badge color="blue" text="1. 先挑主函数" />
            <Badge
              color={configs.length > 0 || unconfiguredStarters.length > 0 ? 'green' : 'blue'}
              text="2. 起页面骨架"
            />
            <Badge
              color={
                configs.some((item) => summarizeWorkspaceReadiness(item, availableFunctions).readyCount > 0)
                  ? 'green'
                  : 'default'
              }
              text="3. 看预览并微调"
            />
            <Typography.Text type="secondary">{workspaceStepSummary.description}</Typography.Text>
            <Typography.Text type="secondary">
              函数目录只负责注册和表单定义，真正把函数拼成业务页面要在这里进入对象工作台。
            </Typography.Text>
          </Space>
        }
      />

      {starterContextMeta ? (
        <Alert
          type="info"
          showIcon
          message={`已带入函数上下文${starterContextMeta.functionId ? ` · ${starterContextMeta.functionId}` : ''}`}
          description={
            starterContextMeta.existingConfig
              ? `当前函数建议落到对象 ${starterContextMeta.objectKey}。这个对象已有工作台草稿，可以直接继续补页面骨架、预览和发布。`
              : `当前函数建议落到对象 ${starterContextMeta.objectKey}。这个对象还没有工作台草稿，现在可以直接创建首个页面并进入编排。`
          }
          action={
            starterContextMeta.existingConfig ? (
              <Button
                type="primary"
                onClick={() =>
                  history.push(
                    `/system/functions/workspace-editor/${encodeURIComponent(
                      starterContextMeta.objectKey,
                    )}`,
                  )
                }
              >
                打开对象工作台
              </Button>
            ) : (
              <Button type="primary" onClick={() => handleOpenCreate().catch(() => {})}>
                为该对象创建首个工作台
              </Button>
            )
          }
        />
      ) : null}

      {unconfiguredStarters.length > 0 && (
        <Card
          title="待开始的页面对象"
          extra={
            <Typography.Text type="secondary">
              {`已注册函数但还没进入对象工作台的对象 ${unconfiguredStarters.length} 个`}
            </Typography.Text>
          }
        >
          <Space direction="vertical" size={12} style={{ width: '100%' }}>
            <Typography.Text type="secondary">
              下面这些对象已经注册了函数，但还没有页面草稿。现在最直接的动作就是进入对象工作台，先起一个能跑的首屏骨架。
            </Typography.Text>
            <List
              grid={{ gutter: 12, xs: 1, sm: 2, md: 2, lg: 3, xl: 3 }}
              dataSource={unconfiguredStarters.slice(0, 6)}
              renderItem={(item, index) => (
                <List.Item>
                  <Card
                    size="small"
                    styles={{
                      body: {
                        padding: DASHBOARD_PAGE_TOKENS.cardPadding,
                        borderRadius: DASHBOARD_PAGE_TOKENS.compactRadius,
                      },
                    }}
                  >
                    <Space direction="vertical" size={8} style={{ width: '100%' }}>
                      <Space wrap size={[8, 8]}>
                        <Typography.Text strong>{item.label}</Typography.Text>
                        <Typography.Text code>{item.objectKey}</Typography.Text>
                      </Space>
                      <Space wrap size={[8, 8]}>
                        <Badge color="processing" text={`已注册函数 ${item.functionCount}`} />
                        <Badge color="orange" text="还没建页面草稿" />
                      </Space>
                      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                        下一步：进入对象工作台，先挑主函数，再生成首个页面骨架。
                      </Typography.Text>
                      <Button
                        type={index === 0 ? 'primary' : 'default'}
                        onClick={() =>
                          history.push(
                            `/system/functions/workspace-editor/${encodeURIComponent(
                              item.objectKey,
                            )}`,
                          )
                        }
                      >
                        {`去做 ${item.label} 的首个工作台页面`}
                      </Button>
                    </Space>
                  </Card>
                </List.Item>
              )}
            />
            {unconfiguredStarters.length > 6 && (
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                {`其余 ${unconfiguredStarters.length - 6} 个对象可通过“新建工作台”继续进入。`}
              </Typography.Text>
            )}
          </Space>
        </Card>
      )}

      <StandardListSection
        title="对象工作台列表"
        extra={
          <Space wrap>
            <Button icon={<ReloadOutlined />} onClick={() => load().catch(() => {})}>
              刷新
            </Button>
            <Button icon={<CopyOutlined />} onClick={handleOpenClone} disabled={!configs.length}>
              复制现有工作台
            </Button>
            <Button type="primary" icon={<PlusOutlined />} onClick={handleOpenCreate}>
              新建对象工作台
            </Button>
          </Space>
        }
      >
        <StandardFilterBar
          resultText={`当前结果 ${visibleConfigs.length} 个`}
          controls={
            <>
              <Space.Compact style={{ width: 320 }}>
                <Input
                  allowClear
                  placeholder="搜索标题 / objectKey / 描述"
                  value={keyword}
                  onChange={(e) => setKeyword(e.target.value)}
                  onPressEnter={() => {}}
                />
                <Button icon={<ReloadOutlined />} title="搜索" />
              </Space.Compact>
              <Select
                value={statusFilter}
                onChange={(val) => setStatusFilter(val)}
                style={{ width: 150 }}
                options={[
                  { label: '全部状态', value: 'all' },
                  { label: '已发布', value: 'published' },
                  { label: '草稿', value: 'draft' },
                  { label: '已归档', value: 'archived' },
                ]}
              />
              <Select
                value={sortBy}
                onChange={(val) => setSortBy(val)}
                style={{ width: 180 }}
                options={[
                  { label: '按更新时间降序', value: 'updated_desc' },
                  { label: '按标题升序', value: 'title_asc' },
                ]}
              />
            </>
          }
        />
        {configs.length === 0 ? (
          <Empty description="还没有对象工作台">
            <Space wrap>
              <Button type="primary" icon={<PlusOutlined />} onClick={handleOpenCreate}>
                新建第一个页面
              </Button>
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                输入对象键后会直接进入对象工作台编排器，首次保存时生成草稿配置。
              </Typography.Text>
            </Space>
          </Empty>
        ) : (
          <>
            {visibleConfigs.length === 0 && (
              <Alert
                style={{ marginBottom: 16 }}
                type="info"
                showIcon
                message="没有匹配的工作台"
                description="请调整筛选条件或搜索关键字。"
              />
            )}

            <List
              grid={{ gutter: 16, xs: 1, sm: 2, md: 2, lg: 3, xl: 3, xxl: 4 }}
              dataSource={visibleConfigs}
              renderItem={(config) => {
                const status = resolveWorkspaceStatus(config);
                const readiness = summarizeWorkspaceReadiness(config, availableFunctions);
                const quality = buildWorkspaceQualityReport(config, availableFunctions);
                const completion = getWorkspaceCompletion(readiness);
                const statusMeta = getWorkspaceStatusMeta(status);
                const priorityHint = getWorkspacePriorityHint({
                  status,
                  readiness,
                  qualityScore: quality.score,
                  blockingCount: quality.blockingCount,
                  warningCount: quality.warningCount,
                });
                const isArchived = status === 'archived';
                const needsCompletion = readiness.pendingCount > 0;
                const editDisabledReason = !access?.canWorkspaceEdit
                  ? '无编辑权限'
                  : isArchived
                  ? '归档状态不允许编辑'
                  : '';
                const publishDisabledReason = !access?.canWorkspacePublish
                  ? '无发布权限'
                  : isArchived
                  ? '归档状态不允许发布操作'
                  : quality.blockingCount > 0
                  ? `当前仍有 ${quality.blockingCount} 个阻塞项，需先回编辑器修复`
                  : '';
                const deleteDisabledReason = !access?.canWorkspaceDelete ? '无删除权限' : '';
                return (
                  <List.Item>
                    <Card
                      hoverable
                      styles={{ body: { padding: 18 } }}
                      extra={
                        <Dropdown
                          trigger={['click']}
                          menu={{
                            items: [
                              config.published
                                ? {
                                    key: 'unpublish',
                                    label: '从控制台下线',
                                    danger: true,
                                    disabled: Boolean(publishDisabledReason),
                                    icon: <StopOutlined />,
                                  }
                                : {
                                    key: 'publish',
                                    label: needsCompletion
                                      ? `发布到控制台（仍有 ${readiness.pendingCount} 个页面待完善）`
                                      : '发布到控制台',
                                    disabled: Boolean(publishDisabledReason),
                                    icon: <GlobalOutlined />,
                                  },
                              {
                                key: 'delete',
                                label: '删除工作台',
                                danger: true,
                                disabled: Boolean(deleteDisabledReason),
                                icon: <DeleteOutlined />,
                              },
                            ],
                            onClick: ({ key }) => {
                              if (key === 'publish') handlePublish(config.objectKey);
                              if (key === 'unpublish') handleUnpublish(config.objectKey);
                              if (key === 'delete') handleDelete(config.objectKey);
                            },
                          }}
                        >
                          <Button
                            type="text"
                            icon={<MoreOutlined />}
                            loading={actionKey === config.objectKey}
                          />
                        </Dropdown>
                      }
                    >
                      <Card.Meta
                        title={
                          <Space direction="vertical" size={8} style={{ width: '100%' }}>
                            <Space wrap size={[8, 8]}>
                              <Typography.Text strong style={{ fontSize: 16 }}>
                                {config.title}
                              </Typography.Text>
                              <Badge status={statusMeta.badgeStatus} text={statusMeta.badgeText} />
                            </Space>
                            <Space wrap size={[8, 6]}>
                              <Typography.Text code>{config.objectKey}</Typography.Text>
                              {typeof config.version === 'number' ? (
                                <Badge color="blue" text={`v${config.version}`} />
                              ) : null}
                              {config.layout?.tabs ? (
                                <Badge
                                  color="default"
                                  text={`${config.layout.tabs.length} 个标签页`}
                                />
                              ) : null}
                            </Space>
                          </Space>
                        }
                        description={
                          <Space direction="vertical" size={12} style={{ width: '100%' }}>
                            <Typography.Text type="secondary">
                              {config.description || '暂无描述'}
                            </Typography.Text>
                            <div
                              style={{
                                padding: 12,
                                borderRadius: 12,
                                background: 'rgba(0,0,0,0.025)',
                              }}
                            >
                              <Space direction="vertical" size={8} style={{ width: '100%' }}>
                                <Space
                                  align="center"
                                  style={{ width: '100%', justifyContent: 'space-between' }}
                                >
                                  <Typography.Text strong>页面完成度</Typography.Text>
                                  <Typography.Text style={{ color: statusMeta.color }}>
                                    {completion.label}
                                  </Typography.Text>
                                </Space>
                                <Progress
                                  percent={completion.percent}
                                  status={completion.status}
                                  strokeColor={statusMeta.color}
                                  size="small"
                                />
                                <Space wrap size={[8, 6]}>
                                  <Badge color="#52c41a" text={`已就绪 ${readiness.readyCount}`} />
                                  <Badge
                                    color="#faad14"
                                    text={`待完善 ${readiness.pendingCount}`}
                                  />
                                  <Badge
                                    color={quality.blockingCount > 0 ? '#ff4d4f' : '#1677ff'}
                                    text={`评分 ${quality.score}`}
                                  />
                                  {quality.warningCount > 0 ? (
                                    <Badge color="#faad14" text={`风险 ${quality.warningCount}`} />
                                  ) : null}
                                  <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                                    {readiness.summary}
                                  </Typography.Text>
                                </Space>
                              </Space>
                            </div>
                            <div
                              style={{
                                padding: '10px 12px',
                                borderRadius: 10,
                                background: 'rgba(250,173,20,0.08)',
                              }}
                            >
                              <Typography.Text style={{ fontSize: 12, color: priorityHint.tone }}>
                                {priorityHint.text}
                              </Typography.Text>
                            </div>
                            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                              {config.meta?.updatedAt
                                ? `更新于 ${new Date(config.meta.updatedAt).toLocaleString(
                                    'zh-CN',
                                  )}`
                                : '暂无更新时间'}
                            </Typography.Text>
                            <Space wrap size={[8, 8]}>
                              <Tooltip title={editDisabledReason}>
                                <span>
                                  <Button
                                    type="primary"
                                    icon={<EditOutlined />}
                                    disabled={Boolean(editDisabledReason)}
                                    onClick={() =>
                                      history.push(
                                        `/system/functions/workspace-editor/${encodeURIComponent(
                                          config.objectKey,
                                        )}`,
                                      )
                                    }
                                  >
                                    {needsCompletion ? '继续完善' : '查看并编辑'}
                                  </Button>
                                </span>
                              </Tooltip>
                              <Tooltip title={publishDisabledReason}>
                                <span>
                                  <Button
                                    icon={config.published ? <StopOutlined /> : <GlobalOutlined />}
                                    disabled={Boolean(publishDisabledReason)}
                                    onClick={() =>
                                      config.published
                                        ? handleUnpublish(config.objectKey)
                                        : handlePublish(config.objectKey)
                                    }
                                  >
                                    {config.published ? '下线' : '发布'}
                                  </Button>
                                </span>
                              </Tooltip>
                              <Tooltip
                                title={!config.published ? '需要先发布后才能进入控制台' : ''}
                              >
                                <span>
                                  <Button
                                    disabled={!config.published}
                                    onClick={() =>
                                      history.push(
                                        `/console/${encodeURIComponent(config.objectKey)}`,
                                      )
                                    }
                                  >
                                    查看控制台
                                  </Button>
                                </span>
                              </Tooltip>
                              <Button onClick={() => setQualityInspectKey(config.objectKey)}>
                                质量检查
                              </Button>
                            </Space>
                          </Space>
                        }
                      />
                    </Card>
                  </List.Item>
                );
              }}
            />
          </>
        )}
      </StandardListSection>

      <Modal
        title={
          qualityInspectTarget
            ? `质量检查 · ${qualityInspectTarget.config.title || qualityInspectTarget.config.objectKey}`
            : '质量检查'
        }
        open={Boolean(qualityInspectTarget)}
        footer={
          qualityInspectTarget
            ? [
                <Button key="close" onClick={() => setQualityInspectKey('')}>
                  关闭
                </Button>,
                <Button
                  key="edit"
                  type="primary"
                  onClick={() => {
                    history.push(
                      `/system/functions/workspace-editor/${encodeURIComponent(
                        qualityInspectTarget.config.objectKey,
                      )}`,
                    );
                    setQualityInspectKey('');
                  }}
                >
                  去编辑器修复
                </Button>,
              ]
            : null
        }
        width={760}
        onCancel={() => setQualityInspectKey('')}
      >
        {qualityInspectTarget ? (
          <Space direction="vertical" size={12} style={{ width: '100%' }}>
            <Card
              size="small"
              styles={{
                body: {
                  padding: 16,
                  background:
                    qualityInspectTarget.report.blockingCount > 0
                      ? 'linear-gradient(180deg, rgba(255,77,79,0.08) 0%, rgba(255,77,79,0.02) 100%)'
                      : qualityInspectTarget.report.warningCount > 0
                      ? 'linear-gradient(180deg, rgba(250,173,20,0.08) 0%, rgba(250,173,20,0.02) 100%)'
                      : 'linear-gradient(180deg, rgba(82,196,26,0.08) 0%, rgba(82,196,26,0.02) 100%)',
                },
              }}
            >
              <Space direction="vertical" size={8} style={{ width: '100%' }}>
                <Space wrap size={[8, 8]}>
                  <Typography.Text code>{qualityInspectTarget.config.objectKey}</Typography.Text>
                  <Badge
                    status={
                      qualityInspectTarget.report.blockingCount > 0
                        ? 'error'
                        : qualityInspectTarget.report.warningCount > 0
                        ? 'warning'
                        : 'success'
                    }
                    text={qualityInspectTarget.report.headline}
                  />
                </Space>
                <Typography.Text strong>{`当前评分 ${qualityInspectTarget.report.score}/100`}</Typography.Text>
                <Typography.Text type="secondary">
                  {qualityInspectTarget.report.summary}
                </Typography.Text>
                <Space wrap size={[8, 8]}>
                  <Badge
                    status={qualityInspectTarget.report.blockingCount > 0 ? 'error' : 'default'}
                    text={`阻塞项 ${qualityInspectTarget.report.blockingCount}`}
                  />
                  <Badge
                    status={qualityInspectTarget.report.warningCount > 0 ? 'warning' : 'default'}
                    text={`风险项 ${qualityInspectTarget.report.warningCount}`}
                  />
                  <Badge
                    status="processing"
                    text={`通过项 ${qualityInspectTarget.report.readyCount}`}
                  />
                </Space>
              </Space>
            </Card>

            {qualityInspectTarget.report.items.length > 0 ? (
              <Card size="small" title="全局结果">
                <Space direction="vertical" size={10} style={{ width: '100%' }}>
                  {qualityInspectTarget.report.items.map((item, index) => (
                    <Alert
                      key={`${index}-${item.title}`}
                      type={
                        item.level === 'blocking'
                          ? 'error'
                          : item.level === 'warning'
                          ? 'warning'
                          : 'success'
                      }
                      showIcon
                      message={item.title}
                      description={item.detail}
                    />
                  ))}
                </Space>
              </Card>
            ) : null}

            <Card size="small" title="逐页结果">
              <Collapse
                ghost
                items={qualityInspectTarget.report.tabs.map((tab) => ({
                  key: tab.key,
                  label: (
                    <Space wrap size={[8, 8]}>
                      <Typography.Text strong>{tab.title}</Typography.Text>
                      <Tag>{tab.layoutType}</Tag>
                      <Badge
                        status={
                          tab.level === 'blocking'
                            ? 'error'
                            : tab.level === 'warning'
                            ? 'warning'
                            : 'success'
                        }
                        text={tab.summary}
                      />
                    </Space>
                  ),
                  children: (
                    <Space direction="vertical" size={10} style={{ width: '100%' }}>
                      {tab.items.map((item, index) => (
                        <Alert
                          key={`${tab.key}-${index}-${item.title}`}
                          type={
                            item.level === 'blocking'
                              ? 'error'
                              : item.level === 'warning'
                              ? 'warning'
                              : 'success'
                          }
                          showIcon
                          message={item.title}
                          description={item.detail}
                        />
                      ))}
                    </Space>
                  ),
                }))}
              />
            </Card>
          </Space>
        ) : null}
      </Modal>

      <Modal
        title={publishReview ? `发布前摘要 · ${publishReview.title}` : '发布前摘要'}
        open={Boolean(publishReview)}
        onCancel={() => setPublishReview(null)}
        onOk={() => handleConfirmPublish().catch(() => {})}
        confirmLoading={Boolean(publishReview && actionKey === publishReview.objectKey)}
        okText="确认发布"
        cancelText="取消"
        width={720}
      >
        {publishReview ? (
          <Space direction="vertical" size={12} style={{ width: '100%' }}>
            <Card
              size="small"
              styles={{
                body: {
                  padding: 16,
                  background:
                    publishReview.warningCount > 0
                      ? 'linear-gradient(180deg, rgba(250,173,20,0.08) 0%, rgba(250,173,20,0.02) 100%)'
                      : 'linear-gradient(180deg, rgba(82,196,26,0.08) 0%, rgba(82,196,26,0.02) 100%)',
                },
              }}
            >
              <Space direction="vertical" size={8} style={{ width: '100%' }}>
                <Space wrap size={[8, 8]}>
                  <Typography.Text code>{publishReview.objectKey}</Typography.Text>
                  <Badge
                    status={publishReview.warningCount > 0 ? 'warning' : 'success'}
                    text={publishReview.warningCount > 0 ? '可发布，但有风险提示' : '已具备发布条件'}
                  />
                </Space>
                <Typography.Text strong>{`当前评分 ${publishReview.score}/100`}</Typography.Text>
                <Typography.Text type="secondary">{publishReview.qualitySummary}</Typography.Text>
                <Space wrap size={[8, 8]}>
                  <Badge status="processing" text={`标签页 ${publishReview.tabCount}`} />
                  <Badge status="success" text={`页面准备度 ${publishReview.readinessSummary}`} />
                  {publishReview.warningCount > 0 ? (
                    <Badge status="warning" text={`风险提示 ${publishReview.warningCount}`} />
                  ) : null}
                </Space>
              </Space>
            </Card>

            <Card size="small" title="本次发布摘要">
              <Space direction="vertical" size={10} style={{ width: '100%' }}>
                <Alert
                  type="info"
                  showIcon
                  message="变更摘要"
                  description={publishReview.changeSummary}
                />
                <Alert
                  type={publishReview.warningCount > 0 ? 'warning' : 'success'}
                  showIcon
                  message={publishReview.warningCount > 0 ? '发布前提醒' : '可以直接发布'}
                  description={
                    publishReview.warningCount > 0
                      ? '当前没有硬阻塞，但仍建议先检查预览、空态、真实数据绑定和控制台展示结果。'
                      : '当前草稿已通过质量检查，可以直接进入发布。'
                  }
                />
                <Alert
                  type="error"
                  showIcon
                  message="发布影响"
                  description="发布后会在控制台向有权限用户可见，控制台入口会立即切到当前草稿版本。"
                />
              </Space>
            </Card>

            <Space wrap size={[8, 8]}>
              <Button
                onClick={() => {
                  history.push(
                    `/system/functions/workspace-editor/${encodeURIComponent(
                      publishReview.objectKey,
                    )}`,
                  );
                  setPublishReview(null);
                }}
              >
                去编辑器复查
              </Button>
              <Button
                onClick={() => {
                  setQualityInspectKey(publishReview.objectKey);
                  setPublishReview(null);
                }}
              >
                查看质量检查
              </Button>
            </Space>
          </Space>
        ) : null}
      </Modal>

      <Modal
        title="新建对象工作台"
        open={createVisible}
        onCancel={() => setCreateVisible(false)}
        onOk={() => handleCreate().catch(() => {})}
        confirmLoading={createSubmitting}
        okText="进入对象工作台"
        cancelText="取消"
      >
        <Form form={createForm} layout="vertical">
          <Form.Item
            name="objectKey"
            label="对象键"
            extra="如果该对象键已经存在，将直接打开现有对象工作台；如果不存在，会以新草稿进入对象工作台编排器。"
            rules={[
              { required: true, message: '请输入对象键' },
              { pattern: /^[a-zA-Z0-9._-]+$/, message: '仅支持字母、数字、点、下划线和中划线' },
            ]}
          >
            <AutoComplete
              placeholder="输入或选择对象键"
              options={objectOptions}
              onFocus={() => loadObjectOptions().catch(() => {})}
              filterOption={(inputValue, option) =>
                String(option?.value || '')
                  .toLowerCase()
                  .includes(inputValue.toLowerCase())
              }
            >
              <Input suffix={objectOptionsLoading ? '加载中' : undefined} />
            </AutoComplete>
          </Form.Item>
          <Alert
            type="info"
            showIcon
            message="如果这个对象还没有现成页面，会以默认草稿直接进入对象工作台编排器。"
          />
        </Form>
      </Modal>

      <Modal
        title="复制现有工作台"
        open={cloneVisible}
        onCancel={() => setCloneVisible(false)}
        onOk={() => handleClone().catch(() => {})}
        confirmLoading={cloneSubmitting}
        okText="复制并编辑"
        cancelText="取消"
      >
        <Form form={cloneForm} layout="vertical">
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 16 }}
            message="复制会基于源工作台生成一个新草稿"
            description="系统会自动建议新的 objectKey 和标题。你可以直接微调，然后进入编辑器继续修改。"
          />
          <Form.Item
            name="sourceKey"
            label="源工作台"
            rules={[{ required: true, message: '请选择源工作台' }]}
          >
            <Select
              showSearch
              onChange={(nextSourceKey) => {
                const sourceItem = configs.find((item) => item.objectKey === nextSourceKey);
                cloneForm.setFieldsValue({
                  targetKey: buildSuggestedCloneKey(
                    nextSourceKey,
                    configs.map((item) => item.objectKey),
                  ),
                  targetTitle: sourceItem
                    ? `${sourceItem.title || sourceItem.objectKey} 副本`
                    : `${nextSourceKey} 副本`,
                });
              }}
              options={configs.map((item) => ({
                label: `${item.title || item.objectKey} (${item.objectKey})`,
                value: item.objectKey,
              }))}
            />
          </Form.Item>
          <Form.Item
            name="targetKey"
            label="新 objectKey"
            rules={[
              { required: true, message: '请输入新的 objectKey' },
              { pattern: /^[a-zA-Z0-9._-]+$/, message: '仅支持字母、数字、点、下划线和中划线' },
              {
                validator: async (_, value) => {
                  const normalized = String(value || '').trim();
                  if (!normalized) return;
                  if (configs.some((item) => item.objectKey === normalized)) {
                    throw new Error('该 objectKey 已存在，请更换一个新的标识');
                  }
                },
              },
            ]}
          >
            <Input placeholder="例如 player-ops-copy" />
          </Form.Item>
          <Form.Item
            name="targetTitle"
            label="新标题"
            rules={[{ required: true, message: '请输入新标题' }]}
          >
            <Input placeholder="请输入新工作台标题" />
          </Form.Item>
        </Form>
      </Modal>
    </Space>
  );
}
