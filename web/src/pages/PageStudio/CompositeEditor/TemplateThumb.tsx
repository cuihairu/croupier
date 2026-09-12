import React from 'react';
import type { PageNode } from './model';

/**
 * 模板结构缩略图：PageNode[] → 轻量线框（纯 div，类名 tpl-thumb-* 供测试定位）。
 * 设计取舍：不实例化真实组件（数十张卡片实时渲染 antd 代价过高），
 * 结构语义即可传达模板形态——表格=表头+行线、表单=标签/输入行、
 * 按钮=圆角小块、弹窗=紫色框+内部表单行、容器=嵌套框。宽度按 span 占比。
 */

const BOX_GAP = 3;

function spanPct(node: PageNode): number {
  const raw = Number(node.props.span ?? 24);
  const span = Number.isFinite(raw) && raw >= 4 && raw <= 24 ? raw : 24;
  return (span / 24) * 100;
}

function Rows({ count, className }: { count: number; className: string }) {
  return (
    <>
      {Array.from({ length: count }, (_, i) => (
        <div key={i} className={className} style={{ height: 3, borderRadius: 1 }} />
      ))}
    </>
  );
}

function NodeThumb({ node }: { node: PageNode }) {
  const common: React.CSSProperties = {
    borderRadius: 3,
    border: '1px solid #d9d9d9',
    background: '#fff',
    padding: 3,
    display: 'flex',
    flexDirection: 'column',
    gap: 2,
  };

  if (node.type === 'button') {
    return (
      <div
        className="tpl-thumb__btn"
        style={{ width: 36, height: 12, borderRadius: 6, background: '#1677ff', opacity: 0.75 }}
      />
    );
  }
  if (node.type === 'text') {
    return (
      <div
        className="tpl-thumb__text"
        style={{ height: 4, borderRadius: 2, background: '#bfbfbf' }}
      />
    );
  }
  if (node.type === 'modal') {
    return (
      <div
        className="tpl-thumb__modal"
        style={{ ...common, width: `${spanPct(node)}%`, border: '1px solid #b37feb' }}
      >
        <div style={{ height: 4, borderRadius: 2, background: '#d3b8f5' }} />
        <div style={{ height: 3, borderRadius: 1, background: '#f0f0f0' }} />
        <div style={{ height: 3, borderRadius: 1, background: '#f0f0f0' }} />
      </div>
    );
  }
  if (node.type === 'container' && node.children?.length) {
    return (
      <div
        className="tpl-thumb__container"
        style={{ ...common, width: `${spanPct(node)}%`, gap: BOX_GAP }}
      >
        {node.children.slice(0, 3).map((c) => (
          <NodeThumb key={c.id} node={c} />
        ))}
      </div>
    );
  }
  // fnTable（表头+行线）/ fnForm、staticForm、fnFields（标签/输入行）
  const isTable = node.type === 'fnTable';
  const cls = isTable
    ? 'tpl-thumb__table'
    : node.type === 'fnFields'
      ? 'tpl-thumb__fields'
      : 'tpl-thumb__form';
  return (
    <div className={cls} style={{ ...common, width: `${spanPct(node)}%` }}>
      {isTable && <div style={{ height: 4, borderRadius: 1, background: '#91caff' }} />}
      <Rows count={isTable ? 3 : 2} className={isTable ? 'tpl-thumb__trow' : 'tpl-thumb__frow'} />
    </div>
  );
}

/** 缩略图容器：根级节点按序铺开，超过 4 个显示 +N 溢出标记。 */
export default function TemplateThumb({ tree }: { tree: PageNode[] }) {
  if (!tree.length) return null;
  return (
    <div
      className="tpl-thumb"
      style={{
        display: 'flex',
        flexWrap: 'wrap',
        gap: BOX_GAP,
        padding: '5px 4px',
        background: '#f5f5f5',
        borderRadius: 4,
        marginBottom: 4,
      }}
    >
      {tree.slice(0, 4).map((n) => (
        <NodeThumb key={n.id} node={n} />
      ))}
      {tree.length > 4 && (
        <div
          className="tpl-thumb__more"
          style={{ fontSize: 10, color: '#999', alignSelf: 'center' }}
        >
          +{tree.length - 4}
        </div>
      )}
    </div>
  );
}
