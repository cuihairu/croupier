import React, { useMemo, useState } from 'react';
import { Alert, Modal, Select, Space, Tag, Typography } from 'antd';
import { useIntl } from '@umijs/max';
import type { DanglingTemplateRef, TemplateRefFix } from './ComponentLibrary';

const { Text } = Typography;

/** U7：模板实例化跨模板联动悬空引用提示 + 快捷重连。
 * 悬空项可逐条选择画布区块重连（按旧引用值替换为目标节点 id），
 * 不选择即保持断开（保存时编译警告兜底）。 */
export default function DanglingRefsModal({
  open,
  refs,
  candidateNodes,
  onClose,
  onApply,
}: {
  open: boolean;
  refs: DanglingTemplateRef[];
  /** 画布节点（含新插入实例）作为重连候选。 */
  candidateNodes: Array<{ id: string; title: string; type: string }>;
  onClose: () => void;
  onApply: (fixes: TemplateRefFix[]) => void;
}) {
  const intl = useIntl();
  const [choices, setChoices] = useState<Record<number, string>>({});

  const kindLabel = (kind: DanglingTemplateRef['kind']): string => {
    const ids: Record<DanglingTemplateRef['kind'], string> = {
      action: 'pages.pageStudio.editor.dangling.kind.action',
      refresh: 'pages.pageStudio.editor.dangling.kind.refresh',
      assignment: 'pages.pageStudio.editor.dangling.kind.assignment',
      rowAction: 'pages.pageStudio.editor.dangling.kind.rowAction',
    };
    return intl.formatMessage({ id: ids[kind], defaultMessage: kind });
  };
  const detailLabel = (d?: string): string => {
    const m = /^(chain|assignment|rowAction) (\d+)$/.exec(d ?? '');
    if (!m) {
      return intl.formatMessage({
        id: 'pages.pageStudio.editor.dangling.detail.main',
        defaultMessage: '主动作目标',
      });
    }
    const [, kind, n] = m;
    const id =
      kind === 'chain'
        ? 'pages.pageStudio.editor.dangling.detail.chain'
        : kind === 'assignment'
          ? 'pages.pageStudio.editor.dangling.detail.assignment'
          : 'pages.pageStudio.editor.dangling.detail.rowAction';
    const def =
      kind === 'chain'
        ? '链第 {n} 步'
        : kind === 'assignment'
          ? '第 {n} 个映射'
          : '第 {n} 个行操作';
    return intl.formatMessage({ id, defaultMessage: def }, { n: Number(n) });
  };

  const options = useMemo(
    () =>
      candidateNodes.map((n) => ({
        value: n.id,
        label: `${n.title}（${n.type}）`,
      })),
    [candidateNodes],
  );

  const handleOk = () => {
    const fixes: TemplateRefFix[] = [];
    refs.forEach((ref, i) => {
      const target = choices[i];
      if (target && target !== ref.nodeId) {
        fixes.push({ nodeId: ref.nodeId, kind: ref.kind, prop: ref.prop, ref: ref.ref, target });
      }
    });
    onApply(fixes);
  };

  return (
    <Modal
      open={open}
      onCancel={onClose}
      onOk={handleOk}
      okText={intl.formatMessage({
        id: 'pages.pageStudio.editor.dangling.apply',
        defaultMessage: '应用重连',
      })}
      cancelText={intl.formatMessage({ id: 'app.cancel', defaultMessage: '取消' })}
      title={intl.formatMessage({
        id: 'pages.pageStudio.editor.dangling.title',
        defaultMessage: '模板联动断链提示',
      })}
      width={520}
      destroyOnHidden
    >
      <Alert
        type="warning"
        showIcon
        message={intl.formatMessage(
          {
            id: 'pages.pageStudio.editor.dangling.intro',
            defaultMessage:
              '以下 {count} 处联动指向模板外区块（模板保存时画布上的其他节点），实例化后已断开。可选择画布区块重连，或保持断开（保存时仍会警告）。',
          },
          { count: refs.length },
        )}
        style={{ marginBottom: 12 }}
      />
      <Space direction="vertical" size={8} style={{ width: '100%' }}>
        {refs.map((ref, i) => (
          <Space
            key={`${ref.nodeId}:${ref.prop}:${ref.ref}:${ref.detail ?? ''}`}
            style={{ width: '100%' }}
          >
            <Space size={4} wrap>
              <Text strong style={{ fontSize: 12 }}>
                {ref.nodeTitle}
              </Text>
              <Tag style={{ marginInlineEnd: 0 }}>{kindLabel(ref.kind)}</Tag>
              <Text type="secondary" style={{ fontSize: 12 }}>
                {detailLabel(ref.detail)} · {ref.prop}
              </Text>
            </Space>
            <Select
              size="small"
              style={{ minWidth: 180 }}
              allowClear
              placeholder={intl.formatMessage({
                id: 'pages.pageStudio.editor.dangling.keep',
                defaultMessage: '保持断开',
              })}
              options={options.filter((o) => o.value !== ref.nodeId)}
              value={choices[i]}
              onChange={(v) => setChoices((prev) => ({ ...prev, [i]: v ?? '' }))}
            />
          </Space>
        ))}
      </Space>
    </Modal>
  );
}
