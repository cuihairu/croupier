import React, { useCallback, useMemo, useRef, useState } from 'react';
import { App, Button, Col, Empty, Input, Row, Space, Typography } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { ArrowLeftOutlined, SaveOutlined } from '@ant-design/icons';
import { history, request } from '@umijs/max';
import type { FunctionDescriptor } from '@/services/api/functions';
import { SortableList } from '@/components/SortableList';
import FunctionPanel from './FunctionPanel';
import SectionCard from './SectionCard';
import Inspector from './Inspector';
import TryRunPanel from './TryRunPanel';
import { defaultView, derivePageKey, type SectionDraft } from './types';
import { extractErrorMessage } from '@/utils/errors';

const { Text } = Typography;

/**
 * 组合页编辑器（全页工作台）：
 * 左 = 函数面板（点击加入） / 中 = 画布（拖拽排序 + 点选配置） /
 * 右 = 属性面板（形态/宽度/联动）/ 底 = 试跑面板（真实调用）。
 * 保存产出 PageProposal，走既有接受/发布/菜单链路。
 */
export default function CompositeEditorPage() {
  const { message, modal } = App.useApp();
  const [sections, setSections] = useState<SectionDraft[]>([]);
  const [selectedKey, setSelectedKey] = useState<string | null>(null);
  const [pageKey, setPageKey] = useState('');
  const [keyTouched, setKeyTouched] = useState(false);

  const fnById = useRef(new Map<string, FunctionDescriptor>());
  const registerFn = useCallback((fn: FunctionDescriptor) => {
    fnById.current.set(fn.id, fn);
  }, []);

  // pageKey 自动预填（用户改过则不覆盖）
  React.useEffect(() => {
    if (!keyTouched && sections.length > 0) {
      const derived = derivePageKey(sections);
      setPageKey((prev) => (prev === derived ? prev : derived));
    }
  }, [sections, keyTouched]);

  const addSection = useCallback(
    (fn: FunctionDescriptor) => {
      registerFn(fn);
      setSections((prev) => {
        if (prev.some((s) => s.functionId === fn.id)) {
          message.warning(`函数 ${fn.id} 已在画布中`);
          return prev;
        }
        const sec: SectionDraft = {
          key: fn.id,
          functionId: fn.id,
          view: defaultView(fn),
          title: fn.summary?.['zh-CN'] || fn.id,
          span: 24,
          autoRun: defaultView(fn) === 'table' || defaultView(fn) === 'fields',
          dependsOn: [],
        };
        setSelectedKey(sec.key);
        return [...prev, sec];
      });
    },
    [message, registerFn],
  );

  const patchSection = useCallback((key: string, patch: Partial<SectionDraft>) => {
    setSections((prev) => prev.map((s) => (s.key === key ? { ...s, ...patch } : s)));
  }, []);

  const removeSection = useCallback((key: string) => {
    setSections((prev) =>
      prev
        .filter((s) => s.key !== key)
        .map((s) => ({ ...s, dependsOn: s.dependsOn.filter((d) => d !== key) })),
    );
    setSelectedKey((k) => (k === key ? null : k));
  }, []);

  const selected = useMemo(
    () => sections.find((s) => s.key === selectedKey),
    [sections, selectedKey],
  );

  const depCounts = useMemo(() => {
    const downstream = new Map<string, number>();
    for (const s of sections) {
      for (const d of s.dependsOn) downstream.set(d, (downstream.get(d) ?? 0) + 1);
    }
    return downstream;
  }, [sections]);

  const [saving, setSaving] = useState(false);
  const save = useCallback(async () => {
    const key = pageKey.trim();
    if (!key) {
      message.warning('请填写页面 Key');
      return;
    }
    if (sections.length < 2) {
      message.warning('组合页至少需要 2 个区块');
      return;
    }
    setSaving(true);
    try {
      const body = {
        pageKey: key,
        sections: sections.map((s) => ({
          functionId: s.functionId,
          view: s.view,
          title: s.title,
          span: s.span,
          autoRun: s.autoRun,
          refreshOn: s.dependsOn,
        })),
      };
      const resp = (await request('/api/v1/versioning/pages/composite', {
        method: 'POST',
        data: body,
      })) as { proposalKey?: unknown };
      modal.success({
        title: '提案已创建',
        content: `组合页提案 ${String(resp?.proposalKey ?? '')} 已进入提案收件箱，接受并发布后生效。`,
        onOk: () => history.push('/functions/pages'),
      });
    } catch (err) {
      message.error(extractErrorMessage(err, '创建提案失败'));
    } finally {
      setSaving(false);
    }
  }, [pageKey, sections, message, modal]);

  return (
    <PageContainer
      header={{
        title: '组合页编辑器',
        onBack: () => history.push('/functions/pages'),
        backIcon: <ArrowLeftOutlined />,
        extra: [
          <Button
            key="save"
            type="primary"
            icon={<SaveOutlined />}
            loading={saving}
            onClick={() => void save()}
          >
            保存为提案
          </Button>,
        ],
      }}
    >
      {/* 顶部：页面标识 */}
      <Space wrap style={{ marginBottom: 12 }}>
        <Text strong>页面 Key</Text>
        <Input
          placeholder="按区块自动生成，可修改"
          value={pageKey}
          onChange={(e) => {
            setKeyTouched(true);
            setPageKey(e.target.value);
          }}
          style={{ width: 320 }}
          suffix={
            keyTouched ? (
              <Button
                size="small"
                type="link"
                style={{ padding: 0 }}
                onClick={() => {
                  setKeyTouched(false);
                  setPageKey(derivePageKey(sections));
                }}
              >
                恢复自动
              </Button>
            ) : null
          }
        />
        <Text type="secondary">
          {sections.length} 个区块 ·{' '}
          {depCounts.size > 0
            ? `${[...depCounts.values()].reduce((a, b) => a + b, 0)} 处联动`
            : '无联动'}
        </Text>
      </Space>

      <Row gutter={12}>
        {/* 左：函数面板 */}
        <Col flex="280px">
          <FunctionPanel
            addedIds={useMemo(() => new Set(sections.map((s) => s.functionId)), [sections])}
            onAdd={addSection}
          />
        </Col>

        {/* 中：画布 */}
        <Col flex="auto" style={{ minWidth: 420 }}>
          <div
            style={{
              border: '1px solid #f0f0f0',
              borderRadius: 8,
              minHeight: 'calc(100vh - 240px)',
              padding: 12,
              background: '#fafafa',
            }}
          >
            {sections.length === 0 ? (
              <Empty
                style={{ marginTop: 120 }}
                description="从左侧点击函数，开始搭建工作台页面"
                image={Empty.PRESENTED_IMAGE_SIMPLE}
              />
            ) : (
              <Row gutter={[12, 12]}>
                <SortableList
                  items={sections}
                  getKey={(s) => s.key}
                  onReorder={setSections}
                  externalDnd
                >
                  {(sec, _idx, dragHandleProps) => (
                    <Col span={sec.span && sec.span < 24 ? sec.span : 24} key={sec.key}>
                      <SectionCard
                        section={sec}
                        fn={fnById.current.get(sec.functionId)}
                        selected={selectedKey === sec.key}
                        onSelect={() => setSelectedKey(sec.key)}
                        onDelete={() => removeSection(sec.key)}
                        dragHandleProps={dragHandleProps}
                        depCount={sec.dependsOn.length}
                        downstreamCount={depCounts.get(sec.key) ?? 0}
                      />
                    </Col>
                  )}
                </SortableList>
              </Row>
            )}
          </div>
        </Col>

        {/* 右：属性面板 */}
        <Col flex="360px">
          <Inspector
            section={selected}
            fn={selected ? fnById.current.get(selected.functionId) : undefined}
            sections={sections}
            fnById={fnById.current}
            onChange={(patch) => selected && patchSection(selected.key, patch)}
            onDelete={() => selected && removeSection(selected.key)}
          />
        </Col>
      </Row>

      {/* 底：试跑面板 */}
      <div style={{ position: 'sticky', bottom: 0, marginTop: 12 }}>
        <TryRunPanel
          section={selected}
          fn={selected ? fnById.current.get(selected.functionId) : undefined}
        />
      </div>
    </PageContainer>
  );
}
