/** 编辑器预览栏（B1）：预览消费与「保存→发布」同一份生成链产出的 PageSpec，
 *
 * 经正式 PageRenderer/CompositeRenderer 渲染，**只替换执行适配器**——模拟执行器
 * 按绑定契约的 outputSchema 合成样例数据，页面结构、区块形态、联动/动作链、分页
 * 与真实页面同源，不再由 PreviewRuntime 另起一套渲染。
 *
 * 预览请求不落库（无提案/草稿/审计），故不需要 pages:publish 权限。
 * 无法无损编译为 PageSpec 的画布（文本节点、非卡片容器等）或预览请求失败时，
 * 回退到旧 PreviewRuntime 渲染树结构，保证预览始终可用。
 */
import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Alert, Spin, Tag, Typography } from 'antd';
import { FormattedMessage, request, useIntl } from '@umijs/max';
import PageRenderer from '@/components/PageRenderer';
import ErrorDetailList from '@/components/ErrorDetailList';
import { extractErrorMessage, hasErrorDetails } from '@/utils/errors';
import type {
  BindingExecutionContext,
  ComponentTemplateUsage,
  Diagnostic,
  JSONValue,
  PageExecutionResult,
  PageSpec,
} from '@/types/dashboard';
import type { FunctionDescriptor } from '@/services/api/functions';
import { compileTree, type CompiledSection } from './compiler';
import { generateMockResponse } from './mockData';
import PreviewRuntime from './PreviewRuntime';
import type { PageNode } from './model';

const { Paragraph, Text } = Typography;

/** 预览端点响应：pageSpec 为完整生成产物（GeneratedPageSpec），
 * 含 quality 与 diagnostics 生成元信息（渲染用 PageSpec 为其子集）。 */
type PreviewCompositeResponse = {
  pageKey: string;
  pageSpec: PageSpec & { quality?: string; diagnostics?: Diagnostic[] };
};

/** 生成质量文案：quality 为服务端机器码，展示须本地化而非英文码值。 */
const QUALITY_LABEL: Record<string, { id: string; defaultMessage: string }> = {
  ready: {
    id: 'pages.pageStudio.preview.quality.ready',
    defaultMessage: '可直接发布',
  },
  basic: {
    id: 'pages.pageStudio.preview.quality.basic',
    defaultMessage: '基础可用',
  },
  needs_review: {
    id: 'pages.pageStudio.preview.quality.needsReview',
    defaultMessage: '需复核',
  },
};

interface PreviewPaneProps {
  tree: PageNode[];
  fnById: Map<string, FunctionDescriptor>;
  pageKey: string;
  componentTemplates?: ComponentTemplateUsage[];
}

export default function PreviewPane({
  tree,
  fnById,
  pageKey,
  componentTemplates,
}: PreviewPaneProps) {
  const intl = useIntl();
  const fmt = useCallback(
    (id: string, defaultMessage: string) => intl.formatMessage({ id, defaultMessage }),
    [intl],
  );
  // fmt 随 intl 重建（Provider 重渲染/语言切换）。直接进 effect 依赖会形成
  // setState → 重渲染 → 新 fmt → effect 重跑 → setState 的循环，故用 ref 取最新值。
  const fmtRef = useRef(fmt);
  fmtRef.current = fmt;

  const [phase, setPhase] = useState<'loading' | 'ready' | 'blocked'>('loading');
  const [spec, setSpec] = useState<PreviewCompositeResponse | null>(null);
  // error：预览请求被服务端拒绝时的原始错误对象；400 + validation_failed 的
  // details 由 ErrorDetailList 逐条内联呈现（与保存/发布弹窗共用同一组件），
  // 而不止一条笼统 message 让前端无从定位。
  const [failure, setFailure] = useState<{
    message: string;
    blockers: string[];
    error?: unknown;
  } | null>(null);

  const compiled = useMemo(() => compileTree(tree), [tree]);

  // sections/blockers 每次 compileTree 都是新数组引用，effect 依赖须用序列化形态比较。
  const requestKey = JSON.stringify({
    pageKey,
    sections: compiled.sections,
    blockers: compiled.blockers,
    componentTemplates,
  });

  useEffect(() => {
    const body = JSON.parse(requestKey) as {
      pageKey: string;
      sections: CompiledSection[];
      blockers: string[];
      componentTemplates?: ComponentTemplateUsage[];
    };
    const key = body.pageKey.trim();

    if (body.blockers.length > 0) {
      setPhase('blocked');
      setFailure({ message: '', blockers: body.blockers });
      setSpec(null);
      return;
    }
    if (!key) {
      setPhase('blocked');
      setFailure({
        message: fmtRef.current('pages.pageStudio.editor.save.keyRequired', '请填写页面 Key'),
        blockers: [],
      });
      setSpec(null);
      return;
    }

    const controller = new AbortController();
    setPhase('loading');
    setSpec(null);
    setFailure(null);

    request<PreviewCompositeResponse>('/api/v1/versioning/pages/composite/preview', {
      method: 'POST',
      data: {
        pageKey: key,
        sections: body.sections,
        ...(body.componentTemplates && body.componentTemplates.length > 0
          ? { componentTemplates: body.componentTemplates }
          : {}),
      },
      signal: controller.signal,
    })
      .then((resp) => {
        if (!resp || !resp.pageSpec) {
          setFailure({
            message: fmtRef.current(
              'pages.pageStudio.preview.badResponse',
              '预览响应缺少 pageSpec',
            ),
            blockers: [],
          });
          setPhase('blocked');
          return;
        }
        setSpec(resp);
        setPhase('ready');
      })
      .catch((error: unknown) => {
        // 新一次预览或退出预览取消了本次请求：不落报错
        if (controller.signal.aborted) return;
        setFailure({
          message: extractErrorMessage(
            error,
            fmtRef.current('pages.pageStudio.preview.fetchFailed', '预览生成失败'),
          ),
          blockers: [],
          error,
        });
        setPhase('blocked');
      });

    return () => controller.abort();
  }, [requestKey]);

  /** 模拟执行器：按绑定契约的 outputSchema 合成样例数据。
   * 返回真实 kind 语义（sync），使联动/动作链/分页/成功回调按正式路径走通。 */
  const mockExecute = useCallback(
    async (bindingId: string, _context: BindingExecutionContext): Promise<PageExecutionResult> => {
      const binding = spec?.pageSpec.bindings.find((b) => b.id === bindingId);
      const fn = binding ? fnById.get(binding.functionId) : undefined;
      const mock = binding ? generateMockResponse(fn, binding.functionId) : undefined;
      const data: JSONValue = mock?.data ?? {};
      return { kind: 'sync', requestId: `preview-${bindingId}`, data };
    },
    [spec, fnById],
  );

  // 执行器引用稳定：PageRenderer 的 executeWithPageState 依赖其引用，
  // 频繁重建会打断在途执行
  const onExecuteRef = useRef(mockExecute);
  onExecuteRef.current = mockExecute;
  const onExecute = useCallback(
    (bindingId: string, context: BindingExecutionContext): Promise<PageExecutionResult> =>
      onExecuteRef.current(bindingId, context),
    [],
  );

  // 预览模式不接导出（模板导出走发布页）
  const onExport = useCallback(async () => undefined, []);

  if (phase === 'loading') {
    return (
      <div
        style={{
          display: 'flex',
          justifyContent: 'center',
          alignItems: 'center',
          minHeight: 240,
        }}
      >
        <Spin>
          <FormattedMessage
            id="pages.pageStudio.preview.generating"
            defaultMessage="正在生成预览"
          />
        </Spin>
      </div>
    );
  }

  const fallback = <PreviewRuntime tree={tree} fnById={fnById} />;

  if (phase === 'blocked') {
    const blockers = failure?.blockers ?? [];
    // 服务端结构化拒绝（400 + validation_failed）的字段级原因
    const hasDetails = failure ? hasErrorDetails(failure.error) : false;
    return (
      <div>
        {failure && (
          <Alert
            type="warning"
            showIcon
            style={{ marginBottom: 12 }}
            title={
              blockers.length > 0
                ? fmt(
                    'pages.pageStudio.preview.fallback.blocked',
                    '存在无法无损编译为页面规格的内容，已回退到画布结构预览',
                  )
                : failure.message
            }
            description={
              blockers.length > 0 || hasDetails ? (
                <div>
                  {blockers.length > 0 && (
                    <ul style={{ margin: 0, paddingLeft: 16 }}>
                      {blockers.map((b, i) => (
                        <li key={`b-${i}`}>{b}</li>
                      ))}
                    </ul>
                  )}
                  {hasDetails && <ErrorDetailList error={failure.error} />}
                  {blockers.length > 0 && (
                    <Paragraph type="secondary" style={{ fontSize: 12, marginBottom: 0 }}>
                      {fmt(
                        'pages.pageStudio.preview.fallbackHint',
                        '修复上述内容后重新进入预览，即可获得与发布页一致的渲染。',
                      )}
                    </Paragraph>
                  )}
                </div>
              ) : undefined
            }
          />
        )}
        {fallback}
      </div>
    );
  }

  const previewSpec = spec?.pageSpec;
  if (!previewSpec) return null;

  const diagnostics = previewSpec.diagnostics ?? [];
  const severity = diagnostics.some((d) => d.severity === 'error') ? 'error' : 'warning';
  const qualityKey = previewSpec.quality ?? '';

  return (
    <div>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 12 }}>
        <Text strong>{fmt('pages.pageStudio.preview.spec', '页面规格预览')}</Text>
        {qualityKey && (
          <Tag color="blue">
            {fmt(
              QUALITY_LABEL[qualityKey]?.id ?? 'pages.pageStudio.preview.quality.other',
              QUALITY_LABEL[qualityKey]?.defaultMessage ?? qualityKey,
            )}
          </Tag>
        )}
        <Text type="secondary" style={{ fontSize: 12 }}>
          {fmt(
            'pages.pageStudio.preview.specHint',
            '与保存/发布同一份生成链产物；执行由模拟适配器提供样例数据',
          )}
        </Text>
      </div>
      {diagnostics.length > 0 && (
        <Alert
          type={severity}
          showIcon
          style={{ marginBottom: 12 }}
          title={fmt('pages.pageStudio.preview.diagnostics', '生成诊断')}
          description={
            <ul style={{ margin: 0, paddingLeft: 16 }}>
              {diagnostics.map((d, i) => (
                <li key={`${d.code}-${i}`}>{d.message}</li>
              ))}
            </ul>
          }
        />
      )}
      <div
        style={{
          border: '1px solid #f0f0f0',
          borderRadius: 8,
          padding: 12,
          background: '#fafafa',
        }}
      >
        <PageRenderer
          pageSpec={previewSpec}
          onExecute={onExecute}
          preview={false}
          onExport={onExport}
        />
      </div>
      <Paragraph type="secondary" style={{ fontSize: 12, marginBottom: 0 }}>
        {fmt(
          'pages.pageStudio.preview.mockNotice',
          '模拟模式：点击执行返回按契约 schema 合成的样例数据，不产生真实调用；发布后按快照执行。',
        )}
      </Paragraph>
    </div>
  );
}
