/**
 * capture.gates 结构化表单编辑器（风控规则页，todo 4297）：三段闸 + 事件
 * 过滤的分区编辑面。定位是 JSON 生产者——所有编辑经 onChange 回传整份
 * GatesFormState，由页面序列化回 wire JSON 走既有校验/发布链；本组件不
 * 感知发布语义。控件锚点用 aria-label（`<段>[<行>].<字段>`）供单测定位。
 */
import React from 'react';
import { Button, Card, Input, InputNumber, Select, Space, Typography } from 'antd';
import { DeleteOutlined, PlusOutlined } from '@ant-design/icons';
import { FormattedMessage } from '@umijs/max';
import {
  GATE_OPS,
  GATE_SEVERITIES,
  type GatesBurstRule,
  type GatesFormState,
  type GatesLineageRule,
  type GatesOffHoursRule,
  type GatesProbeRule,
  type GatesReconcileRule,
} from './gatesForm';

const { Text } = Typography;

type Row = Record<string, string>;

/** 字段标签（id 进 locales，defaultMessage 兜底）。 */
const FIELD: Row = {
  name: 'pages.opsCapture.form.field.name',
  itemTable: 'pages.opsCapture.form.field.itemTable',
  ledgerTable: 'pages.opsCapture.form.field.ledgerTable',
  itemKey: 'pages.opsCapture.form.field.itemKey',
  ledgerKey: 'pages.opsCapture.form.field.ledgerKey',
  table: 'pages.opsCapture.form.field.table',
  ops: 'pages.opsCapture.form.field.ops',
  deltaField: 'pages.opsCapture.form.field.deltaField',
  keyField: 'pages.opsCapture.form.field.keyField',
  threshold: 'pages.opsCapture.form.field.threshold',
  mode: 'pages.opsCapture.form.field.mode',
  windowSec: 'pages.opsCapture.form.field.windowSec',
  maxEvents: 'pages.opsCapture.form.field.maxEvents',
  maxPending: 'pages.opsCapture.form.field.maxPending',
  smallBelow: 'pages.opsCapture.form.field.smallBelow',
  count: 'pages.opsCapture.form.field.count',
  maxTracked: 'pages.opsCapture.form.field.maxTracked',
  severity: 'pages.opsCapture.form.field.severity',
  exemptKeys: 'pages.opsCapture.form.field.exemptKeys',
  allowAccounts: 'pages.opsCapture.form.field.allowAccounts',
  windows: 'pages.opsCapture.form.field.windows',
  tzOffsetMin: 'pages.opsCapture.form.field.tzOffsetMin',
  startMin: 'pages.opsCapture.form.field.startMin',
  endMin: 'pages.opsCapture.form.field.endMin',
};

function F({ k }: { k: string }) {
  return <FormattedMessage id={FIELD[k]} defaultMessage={k} />;
}

interface EditorProps {
  value: GatesFormState;
  onChange: (next: GatesFormState) => void;
  disabled: boolean;
}

function RuleBox({
  label,
  onRemove,
  children,
}: {
  label: string;
  onRemove: () => void;
  children: React.ReactNode;
}) {
  return (
    <div
      style={{
        border: '1px solid #d9d9d9',
        borderRadius: 6,
        padding: 12,
        marginBottom: 8,
        position: 'relative',
      }}
    >
      <Button
        aria-label={`${label}.remove`}
        type="text"
        size="small"
        danger
        icon={<DeleteOutlined />}
        onClick={onRemove}
        style={{ position: 'absolute', top: 4, right: 4 }}
      />
      <Space wrap size={8}>
        {children}
      </Space>
    </div>
  );
}

function opsOptions(): { value: string; label: React.ReactNode }[] {
  return GATE_OPS.map((op) => ({ value: op, label: op }));
}

function severityOptions(): { value: string; label: React.ReactNode }[] {
  return [...GATE_SEVERITIES.map((s) => ({ value: s, label: s })), { value: '', label: '-' }];
}

export function GatesFormEditor({ value, onChange, disabled }: EditorProps) {
  const patch = <K extends keyof GatesFormState>(key: K, next: GatesFormState[K]) =>
    onChange({ ...value, [key]: next });

  // 行内补丁：返回新数组（不可变更新），调用点持具体段类型。
  const patchRuleOf = <T,>(list: T[], index: number, pf: Partial<T>): T[] =>
    list.map((r, i) => (i === index ? { ...r, ...pf } : r));

  const addReconcile = () =>
    patch('reconcile', [
      ...value.reconcile,
      {
        name: '',
        itemTable: '',
        ledgerTable: '',
        itemKey: '',
        ledgerKey: '',
        windowSec: 300,
        severity: '',
        exemptKeys: [],
        maxPending: 0,
      },
    ]);

  return (
    <Space orientation="vertical" style={{ width: '100%' }} size={12}>
      <Card
        size="small"
        title={
          <FormattedMessage id="pages.opsCapture.form.section.reconcile" defaultMessage="对账闸" />
        }
        extra={
          <Button
            aria-label="reconcile.add"
            size="small"
            icon={<PlusOutlined />}
            disabled={disabled}
            onClick={addReconcile}
          />
        }
      >
        {value.reconcile.length === 0 && (
          <Text type="secondary">
            <FormattedMessage
              id="pages.opsCapture.form.empty"
              defaultMessage="无规则（只采不限）"
            />
          </Text>
        )}
        {value.reconcile.map((r: GatesReconcileRule, i) => (
          <RuleBox
            key={`reconcile-${i}`}
            label={`reconcile[${i}]`}
            onRemove={() =>
              patch(
                'reconcile',
                value.reconcile.filter((_, j) => j !== i),
              )
            }
          >
            <Input
              aria-label={`reconcile[${i}].name`}
              addonBefore={<F k="name" />}
              style={{ width: 200 }}
              value={r.name}
              disabled={disabled}
              onChange={(e) =>
                patch('reconcile', patchRuleOf(value.reconcile, i, { name: e.target.value }))
              }
            />
            <Input
              aria-label={`reconcile[${i}].itemTable`}
              addonBefore={<F k="itemTable" />}
              style={{ width: 240 }}
              placeholder="db.table"
              value={r.itemTable}
              disabled={disabled}
              onChange={(e) =>
                patch('reconcile', patchRuleOf(value.reconcile, i, { itemTable: e.target.value }))
              }
            />
            <Input
              aria-label={`reconcile[${i}].ledgerTable`}
              addonBefore={<F k="ledgerTable" />}
              style={{ width: 240 }}
              placeholder="db.table"
              value={r.ledgerTable}
              disabled={disabled}
              onChange={(e) =>
                patch('reconcile', patchRuleOf(value.reconcile, i, { ledgerTable: e.target.value }))
              }
            />
            <Input
              aria-label={`reconcile[${i}].itemKey`}
              addonBefore={<F k="itemKey" />}
              style={{ width: 140 }}
              value={r.itemKey}
              disabled={disabled}
              onChange={(e) =>
                patch('reconcile', patchRuleOf(value.reconcile, i, { itemKey: e.target.value }))
              }
            />
            <Input
              aria-label={`reconcile[${i}].ledgerKey`}
              addonBefore={<F k="ledgerKey" />}
              style={{ width: 140 }}
              value={r.ledgerKey}
              disabled={disabled}
              onChange={(e) =>
                patch('reconcile', patchRuleOf(value.reconcile, i, { ledgerKey: e.target.value }))
              }
            />
            <InputNumber
              aria-label={`reconcile[${i}].windowSec`}
              addonBefore={<F k="windowSec" />}
              min={0}
              style={{ width: 150 }}
              value={r.windowSec}
              disabled={disabled}
              onChange={(v) =>
                patch('reconcile', patchRuleOf(value.reconcile, i, { windowSec: v ?? 0 }))
              }
            />
            <InputNumber
              aria-label={`reconcile[${i}].maxPending`}
              addonBefore={<F k="maxPending" />}
              min={0}
              style={{ width: 150 }}
              value={r.maxPending}
              disabled={disabled}
              onChange={(v) =>
                patch('reconcile', patchRuleOf(value.reconcile, i, { maxPending: v ?? 0 }))
              }
            />
            <Select
              aria-label={`reconcile[${i}].severity`}
              style={{ width: 130 }}
              value={r.severity}
              options={severityOptions()}
              disabled={disabled}
              onChange={(v) =>
                patch(
                  'reconcile',
                  patchRuleOf(value.reconcile, i, {
                    severity: v as GatesReconcileRule['severity'],
                  }),
                )
              }
            />
            <Select
              aria-label={`reconcile[${i}].exemptKeys`}
              mode="tags"
              style={{ minWidth: 220 }}
              value={r.exemptKeys}
              placeholder={<F k="exemptKeys" />}
              disabled={disabled}
              onChange={(v) =>
                patch('reconcile', patchRuleOf(value.reconcile, i, { exemptKeys: v }))
              }
            />
          </RuleBox>
        ))}
      </Card>

      <Card
        size="small"
        title={
          <FormattedMessage
            id="pages.opsCapture.form.section.cepOffHours"
            defaultMessage="CEP 时段闸"
          />
        }
        extra={
          <Button
            aria-label="cepOffHours.add"
            size="small"
            icon={<PlusOutlined />}
            disabled={disabled}
            onClick={() =>
              patch('cepOffHours', [
                ...value.cepOffHours,
                { name: '', table: '', ops: [], windows: [], tzOffsetMin: 480, severity: '' },
              ])
            }
          />
        }
      >
        {value.cepOffHours.length === 0 && (
          <Text type="secondary">
            <FormattedMessage
              id="pages.opsCapture.form.empty"
              defaultMessage="无规则（只采不限）"
            />
          </Text>
        )}
        {value.cepOffHours.map((r: GatesOffHoursRule, i) => (
          <RuleBox
            key={`offhours-${i}`}
            label={`cepOffHours[${i}]`}
            onRemove={() =>
              patch(
                'cepOffHours',
                value.cepOffHours.filter((_, j) => j !== i),
              )
            }
          >
            <Input
              aria-label={`cepOffHours[${i}].name`}
              addonBefore={<F k="name" />}
              style={{ width: 200 }}
              value={r.name}
              disabled={disabled}
              onChange={(e) =>
                patch('cepOffHours', patchRuleOf(value.cepOffHours, i, { name: e.target.value }))
              }
            />
            <Input
              aria-label={`cepOffHours[${i}].table`}
              addonBefore={<F k="table" />}
              style={{ width: 240 }}
              placeholder="db.table"
              value={r.table}
              disabled={disabled}
              onChange={(e) =>
                patch('cepOffHours', patchRuleOf(value.cepOffHours, i, { table: e.target.value }))
              }
            />
            <Select
              aria-label={`cepOffHours[${i}].ops`}
              mode="multiple"
              style={{ minWidth: 200 }}
              value={r.ops}
              options={opsOptions()}
              placeholder={<F k="ops" />}
              disabled={disabled}
              onChange={(v) => patch('cepOffHours', patchRuleOf(value.cepOffHours, i, { ops: v }))}
            />
            <InputNumber
              aria-label={`cepOffHours[${i}].tzOffsetMin`}
              addonBefore={<F k="tzOffsetMin" />}
              min={-720}
              max={840}
              style={{ width: 160 }}
              value={r.tzOffsetMin}
              disabled={disabled}
              onChange={(v) =>
                patch('cepOffHours', patchRuleOf(value.cepOffHours, i, { tzOffsetMin: v ?? 0 }))
              }
            />
            <Select
              aria-label={`cepOffHours[${i}].severity`}
              style={{ width: 130 }}
              value={r.severity}
              options={severityOptions()}
              disabled={disabled}
              onChange={(v) =>
                patch(
                  'cepOffHours',
                  patchRuleOf(value.cepOffHours, i, {
                    severity: v as GatesOffHoursRule['severity'],
                  }),
                )
              }
            />
            <Space wrap size={4}>
              <Text type="secondary">
                <F k="windows" />
              </Text>
              {r.windows.map((w, wi) => (
                <Space key={`win-${wi}`} size={4}>
                  <InputNumber
                    aria-label={`cepOffHours[${i}].windows[${wi}].startMin`}
                    min={0}
                    max={1439}
                    value={w.startMin}
                    disabled={disabled}
                    onChange={(v) =>
                      patch(
                        'cepOffHours',
                        patchRuleOf(value.cepOffHours, i, {
                          windows: r.windows.map((x, xj) =>
                            xj === wi ? { startMin: v ?? 0, endMin: x.endMin } : x,
                          ),
                        }),
                      )
                    }
                  />
                  <InputNumber
                    aria-label={`cepOffHours[${i}].windows[${wi}].endMin`}
                    min={0}
                    max={1439}
                    value={w.endMin}
                    disabled={disabled}
                    onChange={(v) =>
                      patch(
                        'cepOffHours',
                        patchRuleOf(value.cepOffHours, i, {
                          windows: r.windows.map((x, xj) =>
                            xj === wi ? { startMin: x.startMin, endMin: v ?? 0 } : x,
                          ),
                        }),
                      )
                    }
                  />
                  <Button
                    aria-label={`cepOffHours[${i}].windows[${wi}].remove`}
                    type="text"
                    size="small"
                    danger
                    icon={<DeleteOutlined />}
                    disabled={disabled}
                    onClick={() =>
                      patch(
                        'cepOffHours',
                        patchRuleOf(value.cepOffHours, i, {
                          windows: r.windows.filter((_, xj) => xj !== wi),
                        }),
                      )
                    }
                  />
                </Space>
              ))}
              <Button
                aria-label={`cepOffHours[${i}].windows.add`}
                size="small"
                icon={<PlusOutlined />}
                disabled={disabled}
                onClick={() =>
                  patch(
                    'cepOffHours',
                    patchRuleOf(value.cepOffHours, i, {
                      windows: [...r.windows, { startMin: 0, endMin: 0 }],
                    }),
                  )
                }
              />
            </Space>
          </RuleBox>
        ))}
      </Card>

      <Card
        size="small"
        title={
          <FormattedMessage
            id="pages.opsCapture.form.section.cepBurst"
            defaultMessage="CEP 突变闸"
          />
        }
        extra={
          <Button
            aria-label="cepBurst.add"
            size="small"
            icon={<PlusOutlined />}
            disabled={disabled}
            onClick={() =>
              patch('cepBurst', [
                {
                  name: '',
                  table: '',
                  ops: [],
                  deltaField: '',
                  threshold: 0,
                  mode: 'single',
                  windowSec: 0,
                  maxEvents: 0,
                  severity: '',
                },
              ])
            }
          />
        }
      >
        {value.cepBurst.length === 0 && (
          <Text type="secondary">
            <FormattedMessage
              id="pages.opsCapture.form.empty"
              defaultMessage="无规则（只采不限）"
            />
          </Text>
        )}
        {value.cepBurst.map((r: GatesBurstRule, i) => (
          <RuleBox
            key={`burst-${i}`}
            label={`cepBurst[${i}]`}
            onRemove={() =>
              patch(
                'cepBurst',
                value.cepBurst.filter((_, j) => j !== i),
              )
            }
          >
            <Input
              aria-label={`cepBurst[${i}].name`}
              addonBefore={<F k="name" />}
              style={{ width: 200 }}
              value={r.name}
              disabled={disabled}
              onChange={(e) =>
                patch('cepBurst', patchRuleOf(value.cepBurst, i, { name: e.target.value }))
              }
            />
            <Input
              aria-label={`cepBurst[${i}].table`}
              addonBefore={<F k="table" />}
              style={{ width: 240 }}
              placeholder="db.table"
              value={r.table}
              disabled={disabled}
              onChange={(e) =>
                patch('cepBurst', patchRuleOf(value.cepBurst, i, { table: e.target.value }))
              }
            />
            <Select
              aria-label={`cepBurst[${i}].ops`}
              mode="multiple"
              style={{ minWidth: 200 }}
              value={r.ops}
              options={opsOptions()}
              placeholder={<F k="ops" />}
              disabled={disabled}
              onChange={(v) => patch('cepBurst', patchRuleOf(value.cepBurst, i, { ops: v }))}
            />
            <Input
              aria-label={`cepBurst[${i}].deltaField`}
              addonBefore={<F k="deltaField" />}
              style={{ width: 150 }}
              value={r.deltaField}
              disabled={disabled}
              onChange={(e) =>
                patch('cepBurst', patchRuleOf(value.cepBurst, i, { deltaField: e.target.value }))
              }
            />
            <InputNumber
              aria-label={`cepBurst[${i}].threshold`}
              addonBefore={<F k="threshold" />}
              style={{ width: 150 }}
              value={r.threshold}
              disabled={disabled}
              onChange={(v) =>
                patch('cepBurst', patchRuleOf(value.cepBurst, i, { threshold: v ?? 0 }))
              }
            />
            <Select
              aria-label={`cepBurst[${i}].mode`}
              style={{ width: 140 }}
              value={r.mode}
              options={[
                { value: 'single', label: 'single' },
                { value: 'window', label: 'window' },
                { value: '', label: '-' },
              ]}
              disabled={disabled}
              onChange={(v) =>
                patch(
                  'cepBurst',
                  patchRuleOf(value.cepBurst, i, { mode: v as GatesBurstRule['mode'] }),
                )
              }
            />
            <InputNumber
              aria-label={`cepBurst[${i}].windowSec`}
              addonBefore={<F k="windowSec" />}
              min={0}
              style={{ width: 150 }}
              value={r.windowSec}
              disabled={disabled}
              onChange={(v) =>
                patch('cepBurst', patchRuleOf(value.cepBurst, i, { windowSec: v ?? 0 }))
              }
            />
            <InputNumber
              aria-label={`cepBurst[${i}].maxEvents`}
              addonBefore={<F k="maxEvents" />}
              min={0}
              style={{ width: 150 }}
              value={r.maxEvents}
              disabled={disabled}
              onChange={(v) =>
                patch('cepBurst', patchRuleOf(value.cepBurst, i, { maxEvents: v ?? 0 }))
              }
            />
            <Select
              aria-label={`cepBurst[${i}].severity`}
              style={{ width: 130 }}
              value={r.severity}
              options={severityOptions()}
              disabled={disabled}
              onChange={(v) =>
                patch(
                  'cepBurst',
                  patchRuleOf(value.cepBurst, i, { severity: v as GatesBurstRule['severity'] }),
                )
              }
            />
          </RuleBox>
        ))}
      </Card>

      <Card
        size="small"
        title={
          <FormattedMessage
            id="pages.opsCapture.form.section.cepProbing"
            defaultMessage="CEP 试探闸"
          />
        }
        extra={
          <Button
            aria-label="cepProbing.add"
            size="small"
            icon={<PlusOutlined />}
            disabled={disabled}
            onClick={() =>
              patch('cepProbing', [
                {
                  name: '',
                  table: '',
                  ops: [],
                  keyField: '',
                  deltaField: '',
                  smallBelow: 0,
                  count: 0,
                  windowSec: 0,
                  maxTracked: 0,
                  severity: '',
                },
              ])
            }
          />
        }
      >
        {value.cepProbing.length === 0 && (
          <Text type="secondary">
            <FormattedMessage
              id="pages.opsCapture.form.empty"
              defaultMessage="无规则（只采不限）"
            />
          </Text>
        )}
        {value.cepProbing.map((r: GatesProbeRule, i) => (
          <RuleBox
            key={`probing-${i}`}
            label={`cepProbing[${i}]`}
            onRemove={() =>
              patch(
                'cepProbing',
                value.cepProbing.filter((_, j) => j !== i),
              )
            }
          >
            <Input
              aria-label={`cepProbing[${i}].name`}
              addonBefore={<F k="name" />}
              style={{ width: 200 }}
              value={r.name}
              disabled={disabled}
              onChange={(e) =>
                patch('cepProbing', patchRuleOf(value.cepProbing, i, { name: e.target.value }))
              }
            />
            <Input
              aria-label={`cepProbing[${i}].table`}
              addonBefore={<F k="table" />}
              style={{ width: 240 }}
              placeholder="db.table"
              value={r.table}
              disabled={disabled}
              onChange={(e) =>
                patch('cepProbing', patchRuleOf(value.cepProbing, i, { table: e.target.value }))
              }
            />
            <Select
              aria-label={`cepProbing[${i}].ops`}
              mode="multiple"
              style={{ minWidth: 200 }}
              value={r.ops}
              options={opsOptions()}
              placeholder={<F k="ops" />}
              disabled={disabled}
              onChange={(v) => patch('cepProbing', patchRuleOf(value.cepProbing, i, { ops: v }))}
            />
            <Input
              aria-label={`cepProbing[${i}].keyField`}
              addonBefore={<F k="keyField" />}
              style={{ width: 150 }}
              value={r.keyField}
              disabled={disabled}
              onChange={(e) =>
                patch('cepProbing', patchRuleOf(value.cepProbing, i, { keyField: e.target.value }))
              }
            />
            <Input
              aria-label={`cepProbing[${i}].deltaField`}
              addonBefore={<F k="deltaField" />}
              style={{ width: 150 }}
              value={r.deltaField}
              disabled={disabled}
              onChange={(e) =>
                patch(
                  'cepProbing',
                  patchRuleOf(value.cepProbing, i, { deltaField: e.target.value }),
                )
              }
            />
            <InputNumber
              aria-label={`cepProbing[${i}].smallBelow`}
              addonBefore={<F k="smallBelow" />}
              style={{ width: 150 }}
              value={r.smallBelow}
              disabled={disabled}
              onChange={(v) =>
                patch('cepProbing', patchRuleOf(value.cepProbing, i, { smallBelow: v ?? 0 }))
              }
            />
            <InputNumber
              aria-label={`cepProbing[${i}].count`}
              addonBefore={<F k="count" />}
              min={0}
              style={{ width: 130 }}
              value={r.count}
              disabled={disabled}
              onChange={(v) =>
                patch('cepProbing', patchRuleOf(value.cepProbing, i, { count: v ?? 0 }))
              }
            />
            <InputNumber
              aria-label={`cepProbing[${i}].windowSec`}
              addonBefore={<F k="windowSec" />}
              min={0}
              style={{ width: 150 }}
              value={r.windowSec}
              disabled={disabled}
              onChange={(v) =>
                patch('cepProbing', patchRuleOf(value.cepProbing, i, { windowSec: v ?? 0 }))
              }
            />
            <InputNumber
              aria-label={`cepProbing[${i}].maxTracked`}
              addonBefore={<F k="maxTracked" />}
              min={0}
              style={{ width: 150 }}
              value={r.maxTracked}
              disabled={disabled}
              onChange={(v) =>
                patch('cepProbing', patchRuleOf(value.cepProbing, i, { maxTracked: v ?? 0 }))
              }
            />
            <Select
              aria-label={`cepProbing[${i}].severity`}
              style={{ width: 130 }}
              value={r.severity}
              options={severityOptions()}
              disabled={disabled}
              onChange={(v) =>
                patch(
                  'cepProbing',
                  patchRuleOf(value.cepProbing, i, { severity: v as GatesProbeRule['severity'] }),
                )
              }
            />
          </RuleBox>
        ))}
      </Card>

      <Card
        size="small"
        title={
          <FormattedMessage id="pages.opsCapture.form.section.lineage" defaultMessage="血缘闸" />
        }
        extra={
          <Button
            aria-label="lineage.add"
            size="small"
            icon={<PlusOutlined />}
            disabled={disabled}
            onClick={() =>
              patch('lineage', [{ name: '', table: '', ops: [], allowAccounts: [], severity: '' }])
            }
          />
        }
      >
        {value.lineage.length === 0 && (
          <Text type="secondary">
            <FormattedMessage
              id="pages.opsCapture.form.empty"
              defaultMessage="无规则（只采不限）"
            />
          </Text>
        )}
        {value.lineage.map((r: GatesLineageRule, i) => (
          <RuleBox
            key={`lineage-${i}`}
            label={`lineage[${i}]`}
            onRemove={() =>
              patch(
                'lineage',
                value.lineage.filter((_, j) => j !== i),
              )
            }
          >
            <Input
              aria-label={`lineage[${i}].name`}
              addonBefore={<F k="name" />}
              style={{ width: 200 }}
              value={r.name}
              disabled={disabled}
              onChange={(e) =>
                patch('lineage', patchRuleOf(value.lineage, i, { name: e.target.value }))
              }
            />
            <Input
              aria-label={`lineage[${i}].table`}
              addonBefore={<F k="table" />}
              style={{ width: 240 }}
              placeholder="db.table"
              value={r.table}
              disabled={disabled}
              onChange={(e) =>
                patch('lineage', patchRuleOf(value.lineage, i, { table: e.target.value }))
              }
            />
            <Select
              aria-label={`lineage[${i}].ops`}
              mode="multiple"
              style={{ minWidth: 200 }}
              value={r.ops}
              options={opsOptions()}
              placeholder={<F k="ops" />}
              disabled={disabled}
              onChange={(v) => patch('lineage', patchRuleOf(value.lineage, i, { ops: v }))}
            />
            <Select
              aria-label={`lineage[${i}].allowAccounts`}
              mode="tags"
              style={{ minWidth: 220 }}
              value={r.allowAccounts}
              placeholder={<F k="allowAccounts" />}
              disabled={disabled}
              onChange={(v) =>
                patch('lineage', patchRuleOf(value.lineage, i, { allowAccounts: v }))
              }
            />
            <Select
              aria-label={`lineage[${i}].severity`}
              style={{ width: 130 }}
              value={r.severity}
              options={severityOptions()}
              disabled={disabled}
              onChange={(v) =>
                patch(
                  'lineage',
                  patchRuleOf(value.lineage, i, { severity: v as GatesLineageRule['severity'] }),
                )
              }
            />
          </RuleBox>
        ))}
      </Card>

      <Card
        size="small"
        title={
          <FormattedMessage
            id="pages.opsCapture.form.section.eventOps"
            defaultMessage="事件过滤（操作白名单，空=全放行）"
          />
        }
      >
        <Select
          aria-label="eventOps"
          mode="multiple"
          style={{ minWidth: 300 }}
          value={value.eventOps}
          options={opsOptions()}
          placeholder={<F k="ops" />}
          disabled={disabled}
          onChange={(v) => patch('eventOps', v)}
        />
      </Card>
    </Space>
  );
}
