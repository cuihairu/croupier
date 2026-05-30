import type { TabConfig } from '@/types/workspace';
import type { FunctionDescriptor } from '@/services/api/functions';
import { createDefaultLayout } from './orchestrationUtils';

export function healTabLayoutWithTemplate(
  tab: TabConfig,
  descriptors: FunctionDescriptor[],
): TabConfig {
  const functions = Array.isArray(tab.functions) ? tab.functions : [];
  const primaryFunctionId = functions[0] || '';
  const primaryDescriptor = descriptors.find((d) => d.id === primaryFunctionId);
  const layout: any = tab.layout || { type: 'form' };
  const type = layout.type || 'form';
  const template = createDefaultLayout(type, primaryFunctionId, primaryDescriptor);
  const nextLayout = { ...layout };

  if (type === 'list') {
    if (!nextLayout.listFunction) nextLayout.listFunction = template.listFunction;
    if (!Array.isArray(nextLayout.columns) || nextLayout.columns.length === 0) {
      nextLayout.columns = template.columns;
    }
  } else if (type === 'form') {
    if (!nextLayout.submitFunction) nextLayout.submitFunction = template.submitFunction;
    if (!Array.isArray(nextLayout.fields) || nextLayout.fields.length === 0) {
      nextLayout.fields = template.fields;
    }
  } else if (type === 'detail') {
    if (!nextLayout.detailFunction) nextLayout.detailFunction = template.detailFunction;
    if (!Array.isArray(nextLayout.sections) || nextLayout.sections.length === 0) {
      nextLayout.sections = template.sections;
    }
  } else if (type === 'form-detail') {
    if (!nextLayout.queryFunction) nextLayout.queryFunction = template.queryFunction;
    if (!Array.isArray(nextLayout.queryFields) || nextLayout.queryFields.length === 0) {
      nextLayout.queryFields = template.queryFields;
    }
    if (!Array.isArray(nextLayout.detailSections) || nextLayout.detailSections.length === 0) {
      nextLayout.detailSections = template.detailSections;
    }
  } else if (type === 'kanban') {
    if (!nextLayout.dataFunction) nextLayout.dataFunction = template.dataFunction;
    if (!Array.isArray(nextLayout.columns) || nextLayout.columns.length === 0) {
      nextLayout.columns = template.columns;
    }
  } else if (type === 'timeline') {
    if (!nextLayout.dataFunction) nextLayout.dataFunction = template.dataFunction;
    if (typeof nextLayout.showFilter !== 'boolean') nextLayout.showFilter = true;
    if (typeof nextLayout.reverse !== 'boolean') nextLayout.reverse = true;
  } else if (type === 'split') {
    if (!Array.isArray(nextLayout.panels) || nextLayout.panels.length === 0) {
      nextLayout.panels = template.panels;
    } else {
      nextLayout.panels = nextLayout.panels.map((panel: any, index: number) => {
        const templatePanel = template.panels?.[index] || {};
        const templateComp = templatePanel.component || {};
        const panelComp = panel?.component || {};
        const panelCfg = panelComp.config || {};
        const templateCfg = templateComp.config || {};
        const cfg = { ...panelCfg };
        if (!cfg.listFunction && templateCfg.listFunction)
          cfg.listFunction = templateCfg.listFunction;
        if (!cfg.detailFunction && templateCfg.detailFunction)
          cfg.detailFunction = templateCfg.detailFunction;
        if (!Array.isArray(cfg.columns) || cfg.columns.length === 0)
          cfg.columns = templateCfg.columns || [];
        if (!Array.isArray(cfg.sections) || cfg.sections.length === 0)
          cfg.sections = templateCfg.sections || [];
        return {
          ...panel,
          component: {
            ...panelComp,
            type: panelComp.type || templateComp.type,
            config: cfg,
          },
        };
      });
    }
    if (!nextLayout.direction) nextLayout.direction = 'horizontal';
  } else if (type === 'wizard') {
    if (!Array.isArray(nextLayout.steps) || nextLayout.steps.length === 0) {
      nextLayout.steps = template.steps;
    } else {
      nextLayout.steps = nextLayout.steps.map((step: any, index: number) => {
        const templateStep = template.steps?.[index] || {};
        const templateComp = templateStep.component || {};
        const stepComp = step?.component || {};
        const stepCfg = stepComp.config || {};
        const templateCfg = templateComp.config || {};
        const cfg = { ...stepCfg };
        if (!cfg.submitFunction && templateCfg.submitFunction)
          cfg.submitFunction = templateCfg.submitFunction;
        if (!cfg.detailFunction && templateCfg.detailFunction)
          cfg.detailFunction = templateCfg.detailFunction;
        if (!Array.isArray(cfg.fields) || cfg.fields.length === 0)
          cfg.fields = templateCfg.fields || [];
        if (!Array.isArray(cfg.sections) || cfg.sections.length === 0)
          cfg.sections = templateCfg.sections || [];
        return {
          ...step,
          component: {
            ...stepComp,
            type: stepComp.type || templateComp.type,
            config: cfg,
          },
        };
      });
    }
  } else if (type === 'dashboard') {
    if (!Array.isArray(nextLayout.stats) || nextLayout.stats.length === 0) {
      nextLayout.stats = template.stats;
    }
    if (!Array.isArray(nextLayout.panels) || nextLayout.panels.length === 0) {
      nextLayout.panels = template.panels;
    } else {
      nextLayout.panels = nextLayout.panels.map((panel: any, index: number) => {
        const templatePanel = template.panels?.[index] || {};
        const templateComp = templatePanel.component || {};
        const panelComp = panel?.component || {};
        const panelCfg = panelComp.config || {};
        const templateCfg = templateComp.config || {};
        const cfg = { ...panelCfg };
        if (!cfg.listFunction && templateCfg.listFunction)
          cfg.listFunction = templateCfg.listFunction;
        if (!cfg.detailFunction && templateCfg.detailFunction)
          cfg.detailFunction = templateCfg.detailFunction;
        if (!Array.isArray(cfg.columns) || cfg.columns.length === 0)
          cfg.columns = templateCfg.columns || [];
        if (!Array.isArray(cfg.sections) || cfg.sections.length === 0)
          cfg.sections = templateCfg.sections || [];
        return {
          ...panel,
          component: {
            ...panelComp,
            type: panelComp.type || templateComp.type,
            config: cfg,
          },
        };
      });
    }
  } else if (type === 'grid') {
    if (!nextLayout.columns) nextLayout.columns = template.columns;
    if (!Array.isArray(nextLayout.items) || nextLayout.items.length === 0) {
      nextLayout.items = template.items;
    } else {
      nextLayout.items = nextLayout.items.map((item: any, index: number) => {
        const templateItem = template.items?.[index] || {};
        const templateComp = templateItem.component || {};
        const itemComp = item?.component || {};
        const itemCfg = itemComp.config || {};
        const templateCfg = templateComp.config || {};
        const cfg = { ...itemCfg };
        if (!cfg.listFunction && templateCfg.listFunction)
          cfg.listFunction = templateCfg.listFunction;
        if (!cfg.detailFunction && templateCfg.detailFunction)
          cfg.detailFunction = templateCfg.detailFunction;
        if (!Array.isArray(cfg.columns) || cfg.columns.length === 0)
          cfg.columns = templateCfg.columns || [];
        if (!Array.isArray(cfg.sections) || cfg.sections.length === 0)
          cfg.sections = templateCfg.sections || [];
        return {
          ...item,
          component: {
            ...itemComp,
            type: itemComp.type || templateComp.type,
            config: cfg,
          },
        };
      });
    }
    if (!Array.isArray(nextLayout.gutter) || nextLayout.gutter.length !== 2) {
      nextLayout.gutter = template.gutter;
    }
  } else if (type === 'custom') {
    if (!nextLayout.component) nextLayout.component = template.component;
    if (!nextLayout.props || typeof nextLayout.props !== 'object') nextLayout.props = {};
  }

  return {
    ...tab,
    layout: nextLayout,
  };
}
