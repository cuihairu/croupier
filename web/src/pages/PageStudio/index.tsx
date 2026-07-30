import React, { useEffect, useMemo, useState } from 'react';
import { PageContainer, ProTable, type ProColumns } from '@ant-design/pro-components';
import {
  Alert,
  App,
  Button,
  Card,
  Drawer,
  Empty,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Select,
  Space,
  Switch,
  Tag,
  Tabs,
  Typography,
} from 'antd';
import {
  CheckCircleOutlined,
  CodeOutlined,
  EyeOutlined,
  HistoryOutlined,
  ReloadOutlined,
  RetweetOutlined,
  RocketOutlined,
  StopOutlined,
} from '@ant-design/icons';
import FormilyPageRenderer from '@/components/FormilyPageRenderer';
import PageSchemaEditor from './PageSchemaEditor';
import {
  getPageDraft,
  getPageVersion,
  listPageDrafts,
  listPageVersions,
  previewPageDraft,
  publishPageDraft,
  regeneratePageDraft,
  rollbackPageDraft,
  savePageDraft,
  unpublishPage,
  validatePageDraft,
} from '@/services/api/pages';
import { listGeneratedPages, listResources } from '@/services/api/resources';
import { parseOptionalJSONObject } from '@/utils/dashboardJson';
import type {
  Diagnostic,
  GeneratedPageSpec,
  JSONValue,
  PageBindingUsage,
  PageExecutionMode,
  PageExecutionResult,
  PageFunctionBinding,
  PageSpec,
  PageSpecDraft,
  PageSpecDraftSummary,
  PageType,
  PageVersionItem,
  ResourceSpec,
} from '@/types/dashboard';

type JsonParseResult =
  | { ok: true; value: PageSpec }
  | { ok: false; message: string };

type BindingMappingField = 'inputMapping' | 'outputMapping';
type BindingMappingTexts = Record<string, string>;

type TextDiffLine = {
  key: string;
  marker: 'same' | 'added' | 'removed';
  text: string;
};

type ApiErrorLike = {
  response?: {
    status?: number;
    data?: {
      message?: string;
      details?: Record<string, unknown>;
    };
  };
  message?: string;
};

const PAGE_TYPE_OPTIONS: Array<{ label: string; value: PageType }> = [
  { label: 'Entity Page', value: 'entity' },
  { label: 'Operation Page', value: 'operation' },
  { label: 'Task Page', value: 'task' },
  { label: 'Report Page', value: 'report' },
];

const BINDING_USAGE_OPTIONS: Array<{ label: string; value: PageBindingUsage }> = [
  { label: 'query', value: 'query' },
  { label: 'detail', value: 'detail' },
  { label: 'action', value: 'action' },
  { label: 'task', value: 'task' },
  { label: 'report', value: 'report' },
];

const EXECUTION_MODE_OPTIONS: Array<{ label: string; value: PageExecutionMode }> = [
  { label: 'sync', value: 'sync' },
  { label: 'task', value: 'task' },
];

function localizedText(text: Record<string, string> | undefined, fallback: string): string {
  if (!text) return fallback;
  return text['zh-CN'] || text['en-US'] || Object.values(text).find((value) => value.trim()) || fallback;
}

function statusColor(status: PageSpecDraftSummary['status']) {
  if (status === 'published') return 'green';
  if (status === 'archived') return 'default';
  return 'blue';
}

function generatedQualityColor(quality: GeneratedPageSpec['quality']) {
  if (quality === 'ready') return 'green';
  if (quality === 'basic') return 'blue';
  if (quality === 'needs_review') return 'orange';
  return 'red';
}

function canPublishGeneratedPage(quality: GeneratedPageSpec['quality']) {
  return quality === 'ready' || quality === 'basic';
}

function diagnosticColor(severity: Diagnostic['severity']) {
  if (severity === 'error') return 'red';
  if (severity === 'warning') return 'orange';
  return 'blue';
}

function bindingFreshnessDiagnostics(page: PageSpecDraft | null): Diagnostic[] {
  return (page?.bindingFreshness || []).map((item) => item.diagnostic);
}

function formatDate(value?: string): string {
  if (!value) return '-';
  const time = new Date(value);
  return Number.isNaN(time.getTime()) ? value : time.toLocaleString();
}

function stringifyPage(page: PageSpec): string {
  return JSON.stringify(page, null, 2);
}

function compactLocalizedText(value: Record<string, string> | undefined): Record<string, string> | undefined {
  if (!value) return undefined;
  const next = Object.fromEntries(
    Object.entries(value)
      .map(([locale, text]) => [locale, text.trim()])
      .filter(([, text]) => text),
  );
  return Object.keys(next).length > 0 ? next : undefined;
}

function mappingToText(value: JSONValue | undefined): string {
  if (value === undefined) return '';
  return JSON.stringify(value, null, 2);
}

function bindingMappingTextKey(bindingIndex: number, field: BindingMappingField): string {
  return `${bindingIndex}:${field}`;
}

function mappingTextsFromPage(page: PageSpec | null): BindingMappingTexts {
  if (!page) return {};
  return page.bindings.reduce<BindingMappingTexts>((result, binding, index) => {
    result[bindingMappingTextKey(index, 'inputMapping')] = mappingToText(binding.inputMapping);
    result[bindingMappingTextKey(index, 'outputMapping')] = mappingToText(binding.outputMapping);
    return result;
  }, {});
}

function parsePageSpec(raw: string): JsonParseResult {
  try {
    const value = JSON.parse(raw) as PageSpec;
    if (!value || typeof value !== 'object') {
      return { ok: false, message: 'PageSpec 必须是对象' };
    }
    if (!value.pageKey || typeof value.pageKey !== 'string') {
      return { ok: false, message: 'PageSpec.pageKey 必须是字符串' };
    }
    if (!value.type || typeof value.type !== 'string') {
      return { ok: false, message: 'PageSpec.type 必须是字符串' };
    }
    if (!value.title || typeof value.title !== 'object') {
      return { ok: false, message: 'PageSpec.title 必须是多语言对象' };
    }
    if (!value.category || typeof value.category !== 'object') {
      return { ok: false, message: 'PageSpec.category 必须是对象' };
    }
    if (!value.schema || typeof value.schema !== 'object') {
      return { ok: false, message: 'PageSpec.schema 必须是 Formily JSON Schema 对象' };
    }
    if (!Array.isArray(value.bindings)) {
      return { ok: false, message: 'PageSpec.bindings 必须是数组' };
    }
    return { ok: true, value };
  } catch (error) {
    return {
      ok: false,
      message: error instanceof Error ? error.message : 'PageSpec JSON 解析失败',
    };
  }
}

function pageSpecFromCandidate(candidate: GeneratedPageSpec): PageSpec {
  return {
    pageKey: candidate.pageKey,
    type: candidate.type,
    resourceKey: candidate.resourceKey,
    title: candidate.title,
    description: candidate.description,
    category: candidate.category,
    order: candidate.order,
    icon: candidate.icon,
    schema: candidate.schema,
    bindings: candidate.bindings,
    metadata: candidate.metadata,
  };
}

function errorMessage(error: unknown, fallback: string): string {
  if (error instanceof Error && error.message) {
    return error.message;
  }
  const apiError = error as ApiErrorLike;
  if (apiError?.response?.data?.message) {
    return apiError.response.data.message;
  }
  return fallback;
}

function diagnosticsFromApiError(error: unknown): Diagnostic[] {
  const apiError = error as ApiErrorLike;
  const details = apiError?.response?.data?.details;
  if (!details) return [];
  return Object.entries(details).map(([field, message]) => ({
    code: 'server_validation_error',
    severity: 'error',
    field,
    message: typeof message === 'string' ? message : JSON.stringify(message),
  }));
}

function diffText(before: string, after: string): TextDiffLine[] {
  const beforeLines = before.split('\n');
  const afterLines = after.split('\n');
  const max = Math.max(beforeLines.length, afterLines.length);
  const result: TextDiffLine[] = [];
  for (let index = 0; index < max; index += 1) {
    const beforeLine = beforeLines[index];
    const afterLine = afterLines[index];
    if (beforeLine === afterLine) {
      result.push({ key: `${index}:same`, marker: 'same', text: `  ${beforeLine || ''}` });
      continue;
    }
    if (beforeLine !== undefined) {
      result.push({ key: `${index}:removed`, marker: 'removed', text: `- ${beforeLine}` });
    }
    if (afterLine !== undefined) {
      result.push({ key: `${index}:added`, marker: 'added', text: `+ ${afterLine}` });
    }
  }
  return result;
}

function isRevisionConflict(error: unknown): error is ApiErrorLike {
  const apiError = error as ApiErrorLike;
  return apiError?.response?.status === 409;
}

function conflictDescription(error: unknown, localRevision?: number): string {
  const apiError = error as ApiErrorLike;
  const details = apiError?.response?.data?.details;
  const current = details?.current ?? details?.expected;
  const provided = details?.provided ?? localRevision;
  if (current !== undefined || provided !== undefined) {
    return `服务端当前 revision=${String(current ?? '-')}, 本地提交 revision=${String(provided ?? '-')}`;
  }
  return errorMessage(error, '草稿已被其他人或其他窗口更新，请加载最新草稿后再保存。');
}

function mockExecute(bindingId: string, payload: JSONValue): PageExecutionResult {
  return {
    kind: 'sync',
    requestId: `preview-${bindingId}`,
    data: {
      bindingId,
      payload,
      preview: true,
      items: [],
      total: 0,
    },
  };
}

export default function PageStudio() {
  const { message, modal } = App.useApp();
  const [loading, setLoading] = useState(false);
  const [rows, setRows] = useState<PageSpecDraftSummary[]>([]);
  const [selectedKey, setSelectedKey] = useState('');
  const [detailLoading, setDetailLoading] = useState(false);
  const [draft, setDraft] = useState<PageSpecDraft | null>(null);
  const [jsonText, setJsonText] = useState('');
  const [dirty, setDirty] = useState(false);
  const [diagnostics, setDiagnostics] = useState<Diagnostic[]>([]);
  const [preview, setPreview] = useState<PageSpec | null>(null);
  const [versions, setVersions] = useState<PageVersionItem[]>([]);
  const [versionOpen, setVersionOpen] = useState(false);
  const [versionJsonOpen, setVersionJsonOpen] = useState(false);
  const [versionJsonTitle, setVersionJsonTitle] = useState('');
  const [versionJsonText, setVersionJsonText] = useState('');
  const [versionDiffOpen, setVersionDiffOpen] = useState(false);
  const [versionDiffTitle, setVersionDiffTitle] = useState('');
  const [versionDiffLines, setVersionDiffLines] = useState<TextDiffLine[]>([]);
  const [query, setQuery] = useState('');
  const [resources, setResources] = useState<ResourceSpec[]>([]);
  const [selectedResourceKey, setSelectedResourceKey] = useState<string>();
  const [candidateLoading, setCandidateLoading] = useState(false);
  const [candidates, setCandidates] = useState<GeneratedPageSpec[]>([]);
  const [bindingMappingTexts, setBindingMappingTexts] = useState<BindingMappingTexts>({});

  const loadDrafts = async () => {
    setLoading(true);
    try {
      setRows(await listPageDrafts());
    } finally {
      setLoading(false);
    }
  };

  const loadDetail = async (pageKey: string) => {
    setSelectedKey(pageKey);
    setDetailLoading(true);
    setDiagnostics([]);
    setPreview(null);
    try {
      const nextDraft = await getPageDraft(pageKey);
      setDraft(nextDraft);
      setJsonText(stringifyPage(nextDraft));
      setBindingMappingTexts(mappingTextsFromPage(nextDraft));
      setDiagnostics(bindingFreshnessDiagnostics(nextDraft));
      setDirty(false);
    } finally {
      setDetailLoading(false);
    }
  };

  const loadResources = async () => {
    setResources(await listResources());
  };

  const loadCandidates = async () => {
    if (!selectedResourceKey) {
      message.warning('请选择资源');
      return;
    }
    setCandidateLoading(true);
    try {
      setCandidates(await listGeneratedPages(selectedResourceKey));
    } finally {
      setCandidateLoading(false);
    }
  };

  const saveCandidateAsDraft = async (candidate: GeneratedPageSpec) => {
    try {
      const page = pageSpecFromCandidate(candidate);
      await savePageDraft({
        ...page,
        draftRevision: 0,
      });
      message.success(`已创建草稿：${candidate.pageKey}`);
      await loadDrafts();
      await loadDetail(candidate.pageKey);
    } catch (error) {
      message.error(errorMessage(error, '创建草稿失败；如果草稿已存在，请从列表打开后编辑'));
    }
  };

  const publishCandidateDirectly = async (candidate: GeneratedPageSpec) => {
    if (!canPublishGeneratedPage(candidate.quality)) {
      message.warning('只有 ready/basic 默认页可以直接发布');
      return;
    }
    try {
      const page = pageSpecFromCandidate(candidate);
      const saved = await savePageDraft({
        ...page,
        draftRevision: 0,
      });
      const published = await publishPageDraft(candidate.pageKey, saved.draftRevision);
      message.success(`默认页已发布：version ${published.publishedVersion}`);
      await loadDrafts();
      await loadDetail(candidate.pageKey);
    } catch (error) {
      message.error(errorMessage(error, '直接发布失败；如果草稿已存在，请打开草稿后发布'));
    }
  };

  useEffect(() => {
    loadDrafts();
    loadResources();
  }, []);

  const filteredRows = useMemo(() => {
    const keyword = query.trim().toLowerCase();
    if (!keyword) return rows;
    return rows.filter((row) => {
      const title = localizedText(row.title, row.pageKey).toLowerCase();
      return (
        row.pageKey.toLowerCase().includes(keyword) ||
        title.includes(keyword) ||
        (row.resourceKey || '').toLowerCase().includes(keyword) ||
        row.category.key.toLowerCase().includes(keyword)
      );
    });
  }, [query, rows]);

  const currentJson = useMemo(() => parsePageSpec(jsonText), [jsonText]);
  const currentPage = currentJson.ok ? currentJson.value : null;
  const currentJsonErrorMessage = currentJson.ok ? '' : currentJson.message;

  useEffect(() => {
    setBindingMappingTexts(mappingTextsFromPage(currentPage));
  }, [jsonText]);

  const updateCurrentPage = (updater: (page: PageSpec) => PageSpec) => {
    if (!currentJson.ok) {
      message.error(currentJson.message);
      return;
    }
    const nextPage = updater(currentJson.value);
    setJsonText(stringifyPage(nextPage));
    setBindingMappingTexts(mappingTextsFromPage(nextPage));
    setDirty(true);
    setPreview(null);
  };

  const updateLocalizedField = (
    target: 'title' | 'description',
    locale: 'zh-CN' | 'en-US',
    value: string,
  ) => {
    updateCurrentPage((page) => ({
      ...page,
      [target]: compactLocalizedText({
        ...(page[target] || {}),
        [locale]: value,
      }),
    }));
  };

  const updateCategoryLabel = (locale: 'zh-CN' | 'en-US', value: string) => {
    updateCurrentPage((page) => ({
      ...page,
      category: {
        ...page.category,
        labels: compactLocalizedText({
          ...(page.category.labels || {}),
          [locale]: value,
        }) || {},
      },
    }));
  };

  const updateBinding = (bindingIndex: number, updater: (binding: PageFunctionBinding) => PageFunctionBinding) => {
    updateCurrentPage((page) => ({
      ...page,
      bindings: page.bindings.map((binding, index) => (index === bindingIndex ? updater(binding) : binding)),
    }));
  };

  const updateBindingMapping = (
    bindingIndex: number,
    key: BindingMappingField,
    raw: string,
  ) => {
    try {
      const parsed = parseOptionalJSONObject(raw, key);
      setBindingMappingTexts((previous) => ({
        ...previous,
        [bindingMappingTextKey(bindingIndex, key)]: mappingToText(parsed),
      }));
      updateBinding(bindingIndex, (binding) => ({
        ...binding,
        [key]: parsed,
      }));
    } catch (error) {
      message.error(errorMessage(error, `${key} 必须是合法 JSON`));
    }
  };

  const updatePageSchema = (schema: PageSpec['schema']) => {
    updateCurrentPage((page) => ({
      ...page,
      schema,
    }));
  };

  const addBinding = () => {
    updateCurrentPage((page) => ({
      ...page,
      bindings: [
        ...page.bindings,
        {
          id: `binding.${page.bindings.length + 1}`,
          functionId: '',
          usage: 'action',
          execution: { mode: 'sync' },
        },
      ],
    }));
  };

  const removeBinding = (bindingIndex: number) => {
    updateCurrentPage((page) => ({
      ...page,
      bindings: page.bindings.filter((_, index) => index !== bindingIndex),
    }));
  };

  const mappingText = (bindingIndex: number, field: BindingMappingField): string => {
    return bindingMappingTexts[bindingMappingTextKey(bindingIndex, field)] || '';
  };

  const updateMappingText = (bindingIndex: number, field: BindingMappingField, value: string) => {
    setBindingMappingTexts((previous) => ({
      ...previous,
      [bindingMappingTextKey(bindingIndex, field)]: value,
    }));
  };

  const saveCurrentDraft = async () => {
    if (!draft) return;
    if (!currentJson.ok) {
      message.error(currentJson.message);
      return;
    }
    const page = currentJson.value;
    if (page.pageKey !== draft.pageKey) {
      message.error('不允许通过编辑 JSON 修改 pageKey；请新建独立 PageSpec');
      return;
    }
    try {
      const result = await savePageDraft({
        ...page,
        draftRevision: draft.draftRevision,
      });
      message.success(`草稿已保存：revision ${result.draftRevision}`);
      await loadDetail(page.pageKey);
      await loadDrafts();
    } catch (error) {
      if (isRevisionConflict(error)) {
        modal.confirm({
          title: '草稿 revision 冲突',
          content: conflictDescription(error, draft.draftRevision),
          okText: '加载最新草稿',
          cancelText: '保留本地内容',
          onOk: async () => {
            await loadDetail(draft.pageKey);
            await loadDrafts();
          },
        });
        return;
      }
      const nextDiagnostics = diagnosticsFromApiError(error);
      if (nextDiagnostics.length > 0) {
        setDiagnostics(nextDiagnostics);
        message.error('保存草稿失败，请查看诊断');
        return;
      }
      message.error(errorMessage(error, '保存草稿失败'));
    }
  };

  const validateCurrentDraft = async () => {
    if (!draft) return;
    const result = await validatePageDraft(draft.pageKey);
    setDiagnostics(result.diagnostics || []);
    if (result.valid) {
      message.success('PageSpec 校验通过');
      return;
    }
    message.warning('PageSpec 校验未通过，请查看诊断');
  };

  const previewCurrentDraft = async () => {
    if (!draft) return;
    try {
      const page = await previewPageDraft(draft.pageKey);
      setPreview(page);
      message.success('预览已生成');
    } catch (error) {
      const nextDiagnostics = diagnosticsFromApiError(error);
      if (nextDiagnostics.length > 0) {
        setDiagnostics(nextDiagnostics);
        message.error('预览失败，请查看诊断');
        return;
      }
      message.error(errorMessage(error, '预览失败'));
    }
  };

  const publishCurrentDraft = async () => {
    if (!draft) return;
    if (dirty) {
      message.warning('请先保存草稿，再发布');
      return;
    }
    try {
      const result = await publishPageDraft(draft.pageKey, draft.draftRevision);
      message.success(`已发布版本 ${result.publishedVersion}`);
      await loadDetail(draft.pageKey);
      await loadDrafts();
    } catch (error) {
      if (isRevisionConflict(error)) {
        modal.confirm({
          title: '发布 revision 冲突',
          content: conflictDescription(error, draft.draftRevision),
          okText: '加载最新草稿',
          cancelText: '取消',
          onOk: async () => {
            await loadDetail(draft.pageKey);
            await loadDrafts();
          },
        });
        return;
      }
      const nextDiagnostics = diagnosticsFromApiError(error);
      if (nextDiagnostics.length > 0) {
        setDiagnostics(nextDiagnostics);
        message.error('发布失败，请查看诊断');
        return;
      }
      message.error(errorMessage(error, '发布失败'));
    }
  };

  const regenerateCurrentDraft = async () => {
    if (!draft) return;
    if (dirty) {
      message.warning('请先保存或放弃当前未保存修改，再重新生成默认草稿');
      return;
    }
    try {
      const result = await regeneratePageDraft(draft.pageKey, draft.draftRevision);
      message.success(`已重新生成默认草稿：revision ${result.draftRevision}`);
      setDiagnostics(result.diagnostics || []);
      await loadDetail(result.pageKey);
      await loadDrafts();
    } catch (error) {
      if (isRevisionConflict(error)) {
        modal.confirm({
          title: '重生成 revision 冲突',
          content: conflictDescription(error, draft.draftRevision),
          okText: '加载最新草稿',
          cancelText: '取消',
          onOk: async () => {
            await loadDetail(draft.pageKey);
            await loadDrafts();
          },
        });
        return;
      }
      const nextDiagnostics = diagnosticsFromApiError(error);
      if (nextDiagnostics.length > 0) {
        setDiagnostics(nextDiagnostics);
        message.error('重新生成失败，请查看诊断');
        return;
      }
      message.error(errorMessage(error, '重新生成默认草稿失败'));
    }
  };

  const unpublishCurrentPage = async () => {
    if (!draft) return;
    await unpublishPage(draft.pageKey);
    message.success('已取消发布');
    await loadDetail(draft.pageKey);
    await loadDrafts();
  };

  const openVersions = async () => {
    if (!draft) return;
    const result = await listPageVersions(draft.pageKey);
    setVersions(result.items || []);
    setVersionOpen(true);
  };

  const viewVersion = async (version: number) => {
    if (!draft) return;
    const detail = await getPageVersion(draft.pageKey, version);
    setVersionJsonTitle(`${draft.pageKey} / version ${version}`);
    setVersionJsonText(stringifyPage(detail.page));
    setVersionJsonOpen(true);
  };

  const diffVersion = async (version: number) => {
    if (!draft) return;
    if (!currentJson.ok) {
      message.error(currentJson.message);
      return;
    }
    const detail = await getPageVersion(draft.pageKey, version);
    setVersionDiffTitle(`${draft.pageKey} / version ${version} -> 当前草稿`);
    setVersionDiffLines(diffText(stringifyPage(detail.page), stringifyPage(currentJson.value)));
    setVersionDiffOpen(true);
  };

  const rollbackVersion = async (version: number) => {
    if (!draft) return;
    const result = await rollbackPageDraft(draft.pageKey, version);
    message.success(`已回滚为新草稿 revision ${result.draftRevision}`);
    setVersionOpen(false);
    await loadDetail(draft.pageKey);
    await loadDrafts();
  };

  const columns: ProColumns<PageSpecDraftSummary>[] = [
    {
      title: '页面',
      dataIndex: 'pageKey',
      render: (_, record) => (
        <Space direction="vertical" size={0}>
          <Typography.Text strong>{localizedText(record.title, record.pageKey)}</Typography.Text>
          <Typography.Text code>{record.pageKey}</Typography.Text>
        </Space>
      ),
    },
    {
      title: '分类',
      dataIndex: ['category', 'key'],
      width: 170,
      render: (_, record) => (
        <Space size={4} wrap>
          <Tag color="blue">{localizedText(record.category.labels, record.category.key)}</Tag>
          <Typography.Text code>{record.category.key}</Typography.Text>
        </Space>
      ),
    },
    {
      title: '类型',
      dataIndex: 'type',
      width: 110,
      render: (_, record) => <Tag>{record.type}</Tag>,
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 120,
      render: (_, record) => <Tag color={statusColor(record.status)}>{record.status}</Tag>,
    },
    {
      title: '版本',
      dataIndex: 'draftRevision',
      width: 120,
      render: (_, record) => (
        <Space size={4}>
          <Tag>{`draft ${record.draftRevision}`}</Tag>
          {record.publishedVersion ? <Tag color="green">{`pub ${record.publishedVersion}`}</Tag> : null}
        </Space>
      ),
    },
    {
      title: '更新',
      dataIndex: 'updatedAt',
      width: 180,
      render: (_, record) => formatDate(record.updatedAt),
    },
    {
      title: '操作',
      valueType: 'option',
      width: 120,
      render: (_, record) => [
        <Button key="open" type="link" size="small" onClick={() => loadDetail(record.pageKey)}>
          打开
        </Button>,
      ],
    },
  ];

  const candidateColumns: ProColumns<GeneratedPageSpec>[] = [
    {
      title: '候选页面',
      dataIndex: 'pageKey',
      render: (_, record) => (
        <Space direction="vertical" size={0}>
          <Typography.Text strong>{localizedText(record.title, record.pageKey)}</Typography.Text>
          <Typography.Text code>{record.pageKey}</Typography.Text>
        </Space>
      ),
    },
    {
      title: '资源',
      dataIndex: 'resourceKey',
      width: 180,
      render: (_, record) => record.resourceKey ? <Typography.Text code>{record.resourceKey}</Typography.Text> : '-',
    },
    {
      title: '分类',
      dataIndex: ['category', 'key'],
      width: 170,
      render: (_, record) => (
        <Space size={4} wrap>
          <Tag color="blue">{localizedText(record.category.labels, record.category.key)}</Tag>
          <Typography.Text code>{record.category.key}</Typography.Text>
        </Space>
      ),
    },
    {
      title: '质量',
      dataIndex: 'quality',
      width: 130,
      render: (_, record) => <Tag color={generatedQualityColor(record.quality)}>{record.quality}</Tag>,
    },
    {
      title: '诊断',
      dataIndex: 'diagnostics',
      render: (_, record) => {
        const items = record.diagnostics || [];
        if (items.length === 0) return '-';
        return (
          <Space direction="vertical" size={2}>
            {items.slice(0, 3).map((item) => (
              <Typography.Text
                key={`${record.pageKey}:${item.code}:${item.field || ''}`}
                type={item.severity === 'error' ? 'danger' : 'secondary'}
              >
                {`${item.severity}: ${item.message}`}
              </Typography.Text>
            ))}
            {items.length > 3 ? <Typography.Text type="secondary">{`还有 ${items.length - 3} 条`}</Typography.Text> : null}
          </Space>
        );
      },
    },
    {
      title: '操作',
      valueType: 'option',
      width: 190,
      render: (_, record) => [
        <Popconfirm
          key="publish"
          title="直接发布默认页？"
          description="ready/basic 默认页会先物化为草稿，再生成发布快照并进入运行控制台。"
          disabled={!canPublishGeneratedPage(record.quality)}
          onConfirm={() => publishCandidateDirectly(record)}
        >
          <Button type="link" size="small" disabled={!canPublishGeneratedPage(record.quality)}>
            直接发布
          </Button>
        </Popconfirm>,
        <Popconfirm
          key="save"
          title="保存为 PageSpec 草稿？"
          description="保存后可预览、编辑、校验并发布。"
          onConfirm={() => saveCandidateAsDraft(record)}
        >
          <Button type="link" size="small">
            保存为草稿
          </Button>
        </Popconfirm>,
      ],
    },
  ];

  return (
    <PageContainer
      title="Page Studio"
      subTitle="PageSpec 是唯一页面编排协议；运行菜单、分类和多语言标题只来自已发布 PageSpec。"
      extra={[
        <Button key="reload" icon={<ReloadOutlined />} onClick={loadDrafts} loading={loading}>
          刷新
        </Button>,
      ]}
    >
      <Space direction="vertical" size={16} style={{ width: '100%' }}>
        <Alert
          type="info"
          showIcon
          message="边界说明"
          description="函数注册只提供 resource/operation/input/output/risk 等能力契约；页面布局、菜单分类、标题、多语言和 binding 映射必须在 Page Studio 保存并发布。"
        />

        <Card title="从资源生成页面候选">
          <Space direction="vertical" size={12} style={{ width: '100%' }}>
            <Space wrap>
              <Select
                showSearch
                allowClear
                placeholder="选择资源"
                value={selectedResourceKey}
                onChange={setSelectedResourceKey}
                style={{ minWidth: 280 }}
                optionFilterProp="label"
                options={resources.map((resource) => ({
                  label: `${localizedText(resource.labels, resource.key)} (${resource.key})`,
                  value: resource.key,
                }))}
              />
              <Button onClick={loadResources}>刷新资源</Button>
              <Button type="primary" onClick={loadCandidates} loading={candidateLoading}>
                加载候选
              </Button>
              {selectedResourceKey ? <Typography.Text code>{selectedResourceKey}</Typography.Text> : null}
            </Space>
            <ProTable<GeneratedPageSpec>
              rowKey="pageKey"
              loading={candidateLoading}
              dataSource={candidates}
              columns={candidateColumns}
              search={false}
              pagination={{ pageSize: 5 }}
              tableAlertRender={false}
              options={false}
              locale={{ emptyText: '请选择资源并加载候选' }}
            />
          </Space>
        </Card>

        <Card>
          <Space direction="vertical" size={12} style={{ width: '100%' }}>
            <Input.Search
              allowClear
              placeholder="搜索 pageKey、标题、资源或分类"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              style={{ maxWidth: 420 }}
            />
            <ProTable<PageSpecDraftSummary>
              rowKey="pageKey"
              loading={loading}
              dataSource={filteredRows}
              columns={columns}
              search={false}
              pagination={{ pageSize: 20, showSizeChanger: true }}
              tableAlertRender={false}
              options={false}
            />
          </Space>
        </Card>
      </Space>

      <Drawer
        title={draft ? localizedText(draft.title, draft.pageKey) : 'PageSpec'}
        open={!!draft}
        onClose={() => {
          setDraft(null);
          setSelectedKey('');
          setPreview(null);
          setDiagnostics([]);
        }}
        width="86vw"
        destroyOnClose
        extra={
          draft ? (
            <Space wrap>
              <Tag color={statusColor(draft.status)}>{draft.status}</Tag>
              <Tag>{`draft ${draft.draftRevision}`}</Tag>
              {dirty ? <Tag color="orange">未保存</Tag> : <Tag color="green">已同步</Tag>}
            </Space>
          ) : null
        }
      >
        {detailLoading ? (
          <Card loading />
        ) : draft ? (
          <Space direction="vertical" size={16} style={{ width: '100%' }}>
            {draft.bindingFreshness && draft.bindingFreshness.length > 0 ? (
              <Alert
                type="error"
                showIcon
                message="当前已发布页面的函数契约已变化"
                action={
                  <Popconfirm
                    title="重新生成默认草稿？"
                    description="这会用最新 FunctionSpec 覆盖当前 PageSpec 草稿，但不会自动发布。"
                    onConfirm={regenerateCurrentDraft}
                  >
                    <Button danger size="small" icon={<RetweetOutlined />}>
                      重新生成草稿
                    </Button>
                  </Popconfirm>
                }
                description={
                  <Space direction="vertical" size={4}>
                    <Typography.Text>
                      运行控制台会拒绝执行这些 binding。请同步 Function Form、重新生成或修正 PageSpec，并重新发布新快照。
                    </Typography.Text>
                    {draft.bindingFreshness.map((item) => (
                      <Space key={`${item.bindingId}:${item.status}:${item.diagnostic.code}`} wrap>
                        <Tag color="red">{item.status}</Tag>
                        <Typography.Text code>{item.bindingId}</Typography.Text>
                        {item.functionId ? <Typography.Text code>{item.functionId}</Typography.Text> : null}
                        <Typography.Text>{item.diagnostic.message}</Typography.Text>
                      </Space>
                    ))}
                  </Space>
                }
              />
            ) : null}
            <Card size="small">
              <Space wrap>
                <Button type="primary" icon={<CodeOutlined />} onClick={saveCurrentDraft} disabled={!dirty}>
                  保存草稿
                </Button>
                <Button icon={<CheckCircleOutlined />} onClick={validateCurrentDraft}>
                  校验
                </Button>
                <Button icon={<EyeOutlined />} onClick={previewCurrentDraft}>
                  预览
                </Button>
                <Button icon={<HistoryOutlined />} onClick={openVersions}>
                  版本
                </Button>
                <Popconfirm title="确认发布当前草稿？" onConfirm={publishCurrentDraft}>
                  <Button type="primary" icon={<RocketOutlined />} disabled={dirty}>
                    发布
                  </Button>
                </Popconfirm>
                <Popconfirm title="确认取消发布？运行控制台菜单会移除此页面。" onConfirm={unpublishCurrentPage}>
                  <Button danger icon={<StopOutlined />} disabled={draft.status !== 'published'}>
                    取消发布
                  </Button>
                </Popconfirm>
              </Space>
            </Card>

            <Tabs
              items={[
                {
                  key: 'basic',
                  label: '基础信息与绑定',
                  children: currentPage ? (
                    <Space direction="vertical" size={16} style={{ width: '100%' }}>
                      <Alert
                        type="info"
                        showIcon
                        message="结构化编辑只修改 PageSpec 强类型字段"
                        description="这里不会生成旧 layout，也不会新增 UI 协议；页面组件结构仍以 PageSpec.schema 的 Formily JSON Schema 为准。"
                      />
                      <Card size="small" title="页面身份">
                        <Space direction="vertical" size={12} style={{ width: '100%' }}>
                          <Space wrap>
                            <Typography.Text code>{currentPage.pageKey}</Typography.Text>
                            <Select<PageType>
                              value={currentPage.type}
                              options={PAGE_TYPE_OPTIONS}
                              style={{ width: 180 }}
                              onChange={(value) => updateCurrentPage((page) => ({ ...page, type: value }))}
                            />
                            <Input
                              addonBefore="resourceKey"
                              value={currentPage.resourceKey || ''}
                              style={{ width: 280 }}
                              onChange={(event) =>
                                updateCurrentPage((page) => ({
                                  ...page,
                                  resourceKey: event.target.value.trim() || undefined,
                                }))
                              }
                            />
                            <InputNumber
                              addonBefore="order"
                              value={currentPage.order}
                              style={{ width: 180 }}
                              onChange={(value) =>
                                updateCurrentPage((page) => ({
                                  ...page,
                                  order: typeof value === 'number' ? value : undefined,
                                }))
                              }
                            />
                          </Space>
                          <Space wrap>
                            <Input
                              addonBefore="标题 zh-CN"
                              value={currentPage.title?.['zh-CN'] || ''}
                              style={{ width: 360 }}
                              onChange={(event) => updateLocalizedField('title', 'zh-CN', event.target.value)}
                            />
                            <Input
                              addonBefore="Title en-US"
                              value={currentPage.title?.['en-US'] || ''}
                              style={{ width: 360 }}
                              onChange={(event) => updateLocalizedField('title', 'en-US', event.target.value)}
                            />
                            <Input
                              addonBefore="icon"
                              value={currentPage.icon || ''}
                              style={{ width: 220 }}
                              onChange={(event) =>
                                updateCurrentPage((page) => ({ ...page, icon: event.target.value.trim() || undefined }))
                              }
                            />
                          </Space>
                          <Space wrap>
                            <Input
                              addonBefore="分类 key"
                              value={currentPage.category.key}
                              style={{ width: 260 }}
                              onChange={(event) =>
                                updateCurrentPage((page) => ({
                                  ...page,
                                  category: { ...page.category, key: event.target.value.trim() },
                                }))
                              }
                            />
                            <Input
                              addonBefore="分类 zh-CN"
                              value={currentPage.category.labels?.['zh-CN'] || ''}
                              style={{ width: 320 }}
                              onChange={(event) => updateCategoryLabel('zh-CN', event.target.value)}
                            />
                            <Input
                              addonBefore="Category en-US"
                              value={currentPage.category.labels?.['en-US'] || ''}
                              style={{ width: 320 }}
                              onChange={(event) => updateCategoryLabel('en-US', event.target.value)}
                            />
                            <InputNumber
                              addonBefore="分类排序"
                              value={currentPage.category.order}
                              style={{ width: 180 }}
                              onChange={(value) =>
                                updateCurrentPage((page) => ({
                                  ...page,
                                  category: {
                                    ...page.category,
                                    order: typeof value === 'number' ? value : undefined,
                                  },
                                }))
                              }
                            />
                          </Space>
                        </Space>
                      </Card>
                      <Card
                        size="small"
                        title="Function Bindings"
                        extra={
                          <Button size="small" onClick={addBinding}>
                            新增 binding
                          </Button>
                        }
                      >
                        <Space direction="vertical" size={12} style={{ width: '100%' }}>
                          {currentPage.bindings.length === 0 ? <Empty description="暂无 binding" /> : null}
                          {currentPage.bindings.map((binding, index) => (
                            <Card
                              key={`${binding.id}:${index}`}
                              size="small"
                              type="inner"
                              title={binding.id || `binding ${index + 1}`}
                              extra={
                                <Popconfirm title="删除此 binding？" onConfirm={() => removeBinding(index)}>
                                  <Button danger size="small" type="link">
                                    删除
                                  </Button>
                                </Popconfirm>
                              }
                            >
                              <Space direction="vertical" size={12} style={{ width: '100%' }}>
                                <Space wrap>
                                  <Input
                                    addonBefore="bindingId"
                                    value={binding.id}
                                    style={{ width: 280 }}
                                    onChange={(event) =>
                                      updateBinding(index, (item) => ({ ...item, id: event.target.value.trim() }))
                                    }
                                  />
                                  <Input
                                    addonBefore="functionId"
                                    value={binding.functionId}
                                    style={{ width: 320 }}
                                    onChange={(event) =>
                                      updateBinding(index, (item) => ({ ...item, functionId: event.target.value.trim() }))
                                    }
                                  />
                                  <Select<PageBindingUsage>
                                    value={binding.usage}
                                    options={BINDING_USAGE_OPTIONS}
                                    style={{ width: 140 }}
                                    onChange={(value) => updateBinding(index, (item) => ({ ...item, usage: value }))}
                                  />
                                  <Select<PageExecutionMode>
                                    value={binding.execution.mode}
                                    options={EXECUTION_MODE_OPTIONS}
                                    style={{ width: 120 }}
                                    onChange={(value) =>
                                      updateBinding(index, (item) => ({
                                        ...item,
                                        execution: { ...item.execution, mode: value },
                                      }))
                                    }
                                  />
                                  <Space>
                                    <Typography.Text type="secondary">确认</Typography.Text>
                                    <Switch
                                      checked={!!binding.execution.requireConfirm}
                                      onChange={(checked) =>
                                        updateBinding(index, (item) => ({
                                          ...item,
                                          execution: { ...item.execution, requireConfirm: checked || undefined },
                                        }))
                                      }
                                    />
                                  </Space>
                                </Space>
                                <Space align="start" wrap>
                                  <Input.TextArea
                                    value={mappingText(index, 'inputMapping')}
                                    onChange={(event) => updateMappingText(index, 'inputMapping', event.target.value)}
                                    onBlur={(event) => updateBindingMapping(index, 'inputMapping', event.target.value)}
                                    rows={5}
                                    placeholder="inputMapping JSON，例如 {&quot;playerId&quot;:&quot;row.id&quot;}"
                                    style={{ width: 420, fontFamily: 'monospace' }}
                                  />
                                  <Input.TextArea
                                    value={mappingText(index, 'outputMapping')}
                                    onChange={(event) => updateMappingText(index, 'outputMapping', event.target.value)}
                                    onBlur={(event) => updateBindingMapping(index, 'outputMapping', event.target.value)}
                                    rows={5}
                                    placeholder="outputMapping JSON，例如 {&quot;items&quot;:&quot;data.items&quot;}"
                                    style={{ width: 420, fontFamily: 'monospace' }}
                                  />
                                </Space>
                              </Space>
                            </Card>
                          ))}
                        </Space>
                      </Card>
                    </Space>
                  ) : (
                    <Alert type="error" showIcon message="PageSpec JSON 无效" description={currentJsonErrorMessage} />
                  ),
                },
                {
                  key: 'schema',
                  label: '页面组件',
                  children: currentPage ? (
                    <PageSchemaEditor
                      schema={currentPage.schema}
                      bindings={currentPage.bindings}
                      onChange={updatePageSchema}
                    />
                  ) : (
                    <Alert type="error" showIcon message="PageSpec JSON 无效" description={currentJsonErrorMessage} />
                  ),
                },
                {
                  key: 'json',
                  label: 'PageSpec JSON',
                  children: (
                    <Space direction="vertical" size={12} style={{ width: '100%' }}>
                      {!currentJson.ok ? (
                        <Alert type="error" showIcon message="JSON 无效" description={currentJson.message} />
                      ) : null}
                      <Input.TextArea
                        value={jsonText}
                        onChange={(event) => {
                          setJsonText(event.target.value);
                          setDirty(true);
                        }}
                        rows={28}
                        spellCheck={false}
                        style={{ fontFamily: 'monospace' }}
                      />
                    </Space>
                  ),
                },
                {
                  key: 'diagnostics',
                  label: `诊断 ${diagnostics.length}`,
                  children:
                    diagnostics.length > 0 ? (
                      <Space direction="vertical" size={8} style={{ width: '100%' }}>
                        {diagnostics.map((item) => (
                          <Alert
                            key={`${item.code}:${item.field || ''}:${item.message}`}
                            type={item.severity === 'error' ? 'error' : item.severity === 'warning' ? 'warning' : 'info'}
                            showIcon
                            message={
                              <Space wrap>
                                <Tag color={diagnosticColor(item.severity)}>{item.severity}</Tag>
                                <Typography.Text code>{item.code}</Typography.Text>
                                {item.field ? <Typography.Text code>{item.field}</Typography.Text> : null}
                              </Space>
                            }
                            description={item.message}
                          />
                        ))}
                      </Space>
                    ) : (
                      <Empty description="暂无诊断。点击校验后会显示服务端结果。" />
                    ),
                },
                {
                  key: 'preview',
                  label: '预览',
                  children: preview ? (
                    <FormilyPageRenderer page={preview} onExecute={mockExecute} />
                  ) : (
                    <Empty description="点击预览后渲染当前服务端草稿快照" />
                  ),
                },
              ]}
            />
          </Space>
        ) : (
          <Empty description={selectedKey ? '页面加载失败' : '请选择页面'} />
        )}
      </Drawer>

      <Modal
        title={draft ? `${draft.pageKey} 版本历史` : '版本历史'}
        open={versionOpen}
        onCancel={() => setVersionOpen(false)}
        footer={null}
        width={760}
      >
        <ProTable<PageVersionItem>
          rowKey="version"
          dataSource={versions}
          search={false}
          options={false}
          pagination={false}
          columns={[
            {
              title: '版本',
              dataIndex: 'version',
              width: 90,
              render: (_, record) => (
                <Space size={4}>
                  <Typography.Text strong>{record.version}</Typography.Text>
                  {record.isCurrentDraft ? <Tag color="blue">draft</Tag> : null}
                  {record.isCurrentPublished ? <Tag color="green">published</Tag> : null}
                </Space>
              ),
            },
            {
              title: '状态',
              dataIndex: 'status',
              width: 120,
              render: (_, record) => <Tag>{record.status}</Tag>,
            },
            {
              title: '说明',
              dataIndex: 'message',
              render: (_, record) => record.message || '-',
            },
            {
              title: '时间',
              dataIndex: 'createdAt',
              width: 180,
              render: (_, record) => formatDate(record.createdAt),
            },
            {
              title: '操作',
              valueType: 'option',
              width: 180,
              render: (_, record) => [
                <Button key="view" size="small" type="link" onClick={() => viewVersion(record.version)}>
                  查看 JSON
                </Button>,
                <Button key="diff" size="small" type="link" onClick={() => diffVersion(record.version)}>
                  对比当前
                </Button>,
                <Popconfirm
                  key="rollback"
                  title={`回滚到版本 ${record.version}？`}
                  description="回滚会创建新的草稿 revision，不会直接发布到运行控制台。"
                  onConfirm={() => rollbackVersion(record.version)}
                >
                  <Button size="small" type="link" disabled={record.isCurrentDraft}>
                    回滚
                  </Button>
                </Popconfirm>,
              ],
            },
          ]}
        />
      </Modal>
      <Modal
        title={versionJsonTitle}
        open={versionJsonOpen}
        onCancel={() => setVersionJsonOpen(false)}
        footer={null}
        width={920}
      >
        <Input.TextArea value={versionJsonText} rows={30} readOnly spellCheck={false} style={{ fontFamily: 'monospace' }} />
      </Modal>
      <Modal
        title={versionDiffTitle}
        open={versionDiffOpen}
        onCancel={() => setVersionDiffOpen(false)}
        footer={null}
        width={980}
      >
        <Space direction="vertical" size={2} style={{ width: '100%' }}>
          {versionDiffLines.map((line) => (
            <Typography.Text
              key={line.key}
              code
              type={line.marker === 'added' ? 'success' : line.marker === 'removed' ? 'danger' : 'secondary'}
              style={{ whiteSpace: 'pre-wrap', display: 'block' }}
            >
              {line.text}
            </Typography.Text>
          ))}
        </Space>
      </Modal>
    </PageContainer>
  );
}
