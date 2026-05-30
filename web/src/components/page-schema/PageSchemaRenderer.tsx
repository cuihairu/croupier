import React from 'react';
import { Card, Col, Row, Statistic, Tabs, Button } from 'antd';
import type { TabsProps } from 'antd';

export type SchemaPermission = 'read' | 'write';

export type SchemaStat = {
  key: string;
  title: string;
  icon: string;
  color?: string;
};

export type SchemaAction = {
  key: string;
  label: string;
  icon: string;
  permission?: SchemaPermission;
  disabledWhen?: string[];
  loadingWhen?: string;
};

export type SchemaTab = {
  key: string;
  labelTemplate: string;
  permission?: SchemaPermission;
  visibleWhen?: string;
  component: string;
};

type RenderCtx = {
  canWrite: boolean;
  flags: Record<string, boolean>;
};

type ActionRenderCtx = RenderCtx & {
  onAction: (key: string) => void;
  renderIcon: (icon: string) => React.ReactNode;
};

type Props = RenderCtx & {
  stats: SchemaStat[];
  actions: SchemaAction[];
  tabs: SchemaTab[];
  statValues: Record<string, number>;
  templateValues: Record<string, string | number>;
  activeTab: string;
  onTabChange: (key: string) => void;
  renderIcon: (icon: string) => React.ReactNode;
  renderTab: (component: string) => React.ReactNode;
};

const formatTemplate = (template: string, values: Record<string, string | number>) =>
  template.replace(/\{(\w+)\}/g, (_, key) => String(values[key] ?? ''));

const allowByPermission = (permission: SchemaPermission | undefined, canWrite: boolean) =>
  permission !== 'write' || canWrite;

const isDisabled = (rules: string[] | undefined, flags: Record<string, boolean>) =>
  !!rules?.some((rule) => !!flags[rule]);

const isVisible = (rule: string | undefined, flags: Record<string, boolean>) =>
  !rule || !!flags[rule];

export const renderSchemaActions = (
  { canWrite, flags, onAction, renderIcon }: ActionRenderCtx,
  actions: SchemaAction[],
) =>
  actions
    .filter((action) => allowByPermission(action.permission, canWrite))
    .map((action) => (
      <Button
        key={action.key}
        icon={renderIcon(action.icon)}
        disabled={isDisabled(action.disabledWhen, flags)}
        loading={action.loadingWhen ? !!flags[action.loadingWhen] : false}
        onClick={() => onAction(action.key)}
      >
        {action.label}
      </Button>
    ));

export default function PageSchemaRenderer({
  canWrite,
  flags,
  stats,
  tabs,
  statValues,
  templateValues,
  activeTab,
  onTabChange,
  renderIcon,
  renderTab,
}: Props) {
  const tabItems: TabsProps['items'] = tabs
    .filter((tab) => allowByPermission(tab.permission, canWrite))
    .filter((tab) => isVisible(tab.visibleWhen, flags))
    .map((tab) => ({
      key: tab.key,
      label: formatTemplate(tab.labelTemplate, templateValues),
      children: renderTab(tab.component),
    }));

  return (
    <>
      <Row gutter={16} style={{ marginBottom: 16 }}>
        {stats.map((stat) => (
          <Col key={stat.key} span={6}>
            <Card>
              <Statistic
                title={stat.title}
                value={statValues[stat.key]}
                valueStyle={stat.color ? { color: stat.color } : undefined}
                prefix={renderIcon(stat.icon)}
              />
            </Card>
          </Col>
        ))}
      </Row>

      <Card>
        <Tabs activeKey={activeTab} onChange={onTabChange} items={tabItems} />
      </Card>
    </>
  );
}
