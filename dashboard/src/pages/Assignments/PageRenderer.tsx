import React from 'react';
import { Button } from 'antd';
import type { ProColumns } from '@ant-design/pro-components';
import PageSchemaRenderer, {
  renderSchemaActions,
} from '@/components/page-schema/PageSchemaRenderer';
import { resolveSchemaIcon } from '@/components/page-schema/icons';
import type { AssignmentGroup, AssignmentItem } from './types';
import type { AssignmentPageSchema } from './pageSchema';
import ListTab from './ListTab';
import CategoryTab from './CategoryTab';
import RouteTab from './RouteTab';

type RenderCtx = {
  schema: AssignmentPageSchema;
  stats: { total: number; active: number; inactive: number; categories: number };
  groupedAssignments: AssignmentGroup[];
  selected: string[];
  loading: boolean;
  canWrite: boolean;
  hasScope: boolean;
  activeTab: string;
  onTabChange: (key: string) => void;
  columns: ProColumns<AssignmentItem>[];
  categoryColumns: ProColumns<AssignmentGroup>[];
  routeColumns: ProColumns<AssignmentItem>[];
  onSelectAll: () => void;
  onClearAll: () => void;
  onBatchAssign: (category: string, assign: boolean) => void;
  onSave: () => void;
  onReload: () => void;
  onSelectionChange: (keys: React.Key[]) => void;
  onOpenHistory: () => void;
  onOpenClone: () => void;
};

const renderIcon = (icon: string) => resolveSchemaIcon(icon);

const runListToolbarAction = (
  key: 'selectAll' | 'clearAll' | 'save' | 'reload',
  ctx: RenderCtx,
) => {
  if (key === 'selectAll') return ctx.onSelectAll();
  if (key === 'clearAll') return ctx.onClearAll();
  if (key === 'save') return ctx.onSave();
  return ctx.onReload();
};

const renderListToolbarActions = (ctx: RenderCtx) =>
  ctx.schema.listToolbar
    .filter((action) => (action.permission === 'write' ? ctx.canWrite : true))
    .map((action) => (
      <Button
        key={action.key}
        type={action.type}
        icon={renderIcon(action.icon)}
        disabled={
          !!action.disabledWhen?.some((rule) => {
            if (rule === 'noScope') return !ctx.hasScope;
            if (rule === 'noSelection') return ctx.selected.length === 0;
            if (rule === 'loading') return ctx.loading;
            return false;
          })
        }
        loading={action.loadingWhen ? ctx.loading : false}
        onClick={() => runListToolbarAction(action.key, ctx)}
      >
        {action.label}
      </Button>
    ));

const renderCategoryActions = (
  category: string,
  ctx: RenderCtx,
  size: 'small' | 'middle' = 'small',
) =>
  ctx.schema.categoryActions.map((action) => (
    <Button
      key={`${category}-${action.key}`}
      size={size}
      onClick={() => ctx.onBatchAssign(category, action.key === 'enable')}
    >
      {action.label}
    </Button>
  ));

const renderTab = (component: string, ctx: RenderCtx) => {
  if (component === 'ListTab') {
    return (
      <ListTab
        groupedAssignments={ctx.groupedAssignments}
        selected={ctx.selected}
        columns={ctx.columns}
        toolbarActions={renderListToolbarActions(ctx)}
        renderCategoryActions={(category, size) => renderCategoryActions(category, ctx, size)}
        onSelectionChange={ctx.onSelectionChange}
      />
    );
  }
  if (component === 'CategoryTab') {
    return <CategoryTab data={ctx.groupedAssignments} columns={ctx.categoryColumns} />;
  }
  return (
    <RouteTab
      data={ctx.groupedAssignments
        .flatMap((g) => g.items)
        .filter((item) => ctx.selected.includes(item.id))}
      columns={ctx.routeColumns}
    />
  );
};

export const renderPageActions = (ctx: RenderCtx) =>
  renderSchemaActions(
    {
      canWrite: ctx.canWrite,
      flags: {
        noScope: !ctx.hasScope,
        noSelection: ctx.selected.length === 0,
        loading: ctx.loading,
      },
      onAction: (key) => (key === 'history' ? ctx.onOpenHistory() : ctx.onOpenClone()),
      renderIcon,
    },
    ctx.schema.actions,
  );

export default function PageRenderer(ctx: RenderCtx) {
  return (
    <PageSchemaRenderer
      canWrite={ctx.canWrite}
      flags={{ hasGroups: ctx.groupedAssignments.length > 0 }}
      stats={ctx.schema.stats}
      actions={ctx.schema.actions}
      tabs={ctx.schema.tabs}
      statValues={ctx.stats}
      templateValues={{
        active: ctx.stats.active,
        total: ctx.stats.total,
        selectedCount: ctx.selected.length,
      }}
      activeTab={ctx.activeTab}
      onTabChange={ctx.onTabChange}
      renderIcon={renderIcon}
      renderTab={(component) => renderTab(component, ctx)}
    />
  );
}
