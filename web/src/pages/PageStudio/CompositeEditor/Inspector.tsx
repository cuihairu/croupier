import React from 'react';
import { Alert, Card, Empty, Input, Select, Slider, Space, Switch, Tag, Typography } from 'antd';
import { LinkOutlined, WarningOutlined } from '@ant-design/icons';
import type { FunctionDescriptor } from '@/services/api/functions';
import {
  VIEW_META,
  linkageCheck,
  sectionParams,
  sectionOutputFields,
  type CompositeView,
  type SectionDraft,
} from './types';

const { Text } = Typography;

export default function Inspector({
  section,
  fn,
  sections,
  fnById,
  onChange,
  onDelete,
}: {
  section: SectionDraft | undefined;
  fn: FunctionDescriptor | undefined;
  sections: SectionDraft[];
  fnById: Map<string, FunctionDescriptor>;
  onChange: (patch: Partial<SectionDraft>) => void;
  onDelete: () => void;
}) {
  if (!section) {
    return (
      <Card size="small" title="属性" styles={{ body: { padding: 16 } }}>
        <Empty description="点击画布中的区块进行配置" image={Empty.PRESENTED_IMAGE_SIMPLE} />
      </Card>
    );
  }

  const others = sections.filter((s) => s.key !== section.key);
  const params = sectionParams(fn);
  const check = linkageCheck(
    others
      .filter((s) => section.dependsOn.includes(s.key))
      .map((s) => ({
        key: s.key,
        title: s.title || s.functionId,
        outputs: sectionOutputFields(fnById.get(s.functionId)),
      })),
    params,
  );
  const downstream = sections.filter((s) => s.dependsOn.includes(section.key));
  const myOutputs = sectionOutputFields(fn);

  return (
    <Card
      size="small"
      title={<Text strong>区块属性</Text>}
      styles={{ body: { maxHeight: 'calc(100vh - 200px)', overflow: 'auto' } }}
    >
      <Space direction="vertical" size={14} style={{ width: '100%' }}>
        <div>
          <Text type="secondary" style={{ fontSize: 11 }}>
            函数
          </Text>
          <div>
            <Text code style={{ fontSize: 12 }}>
              {section.functionId}
            </Text>
          </div>
        </div>

        <div>
          <Text type="secondary" style={{ fontSize: 11, display: 'block', marginBottom: 4 }}>
            标题
          </Text>
          <Input
            size="small"
            value={section.title}
            onChange={(e) => onChange({ title: e.target.value })}
            placeholder={section.functionId}
          />
        </div>

        <div>
          <Text type="secondary" style={{ fontSize: 11, display: 'block', marginBottom: 4 }}>
            展示形态
          </Text>
          <Select<CompositeView>
            size="small"
            style={{ width: '100%' }}
            value={section.view}
            onChange={(v) => onChange({ view: v })}
            options={Object.entries(VIEW_META).map(([value, meta]) => ({
              value: value as CompositeView,
              label: `${meta.label} — ${meta.hint}`,
            }))}
          />
        </div>

        <div>
          <Text type="secondary" style={{ fontSize: 11, display: 'block', marginBottom: 4 }}>
            宽度（{section.span || 24}/24 栅格）
          </Text>
          <Slider
            min={4}
            max={24}
            step={4}
            value={section.span || 24}
            onChange={(v) => onChange({ span: v })}
            marks={{ 8: '1/3', 12: '半宽', 24: '整行' }}
          />
        </div>

        <div>
          <Space size={8}>
            <Switch
              size="small"
              checked={section.autoRun}
              onChange={(v) => onChange({ autoRun: v })}
            />
            <Text style={{ fontSize: 12 }}>进入页面自动执行</Text>
          </Space>
          <div>
            <Text type="secondary" style={{ fontSize: 11 }}>
              查询类区块建议开启（表格/字段卡）
            </Text>
          </div>
        </div>

        <div>
          <Text type="secondary" style={{ fontSize: 11, display: 'block', marginBottom: 4 }}>
            <LinkOutlined /> 联动依赖（上游产出变化时自动重跑本区块）
          </Text>
          {others.length === 0 ? (
            <Text type="secondary" style={{ fontSize: 12 }}>
              暂无其他区块可依赖
            </Text>
          ) : (
            <Select
              mode="multiple"
              size="small"
              style={{ width: '100%' }}
              placeholder="选择上游区块"
              value={section.dependsOn}
              onChange={(v) => onChange({ dependsOn: v })}
              options={others.map((s) => ({
                value: s.key,
                label: s.title || s.functionId,
              }))}
            />
          )}

          {check.map((c) => (
            <div key={c.depKey} style={{ marginTop: 6 }}>
              <Text style={{ fontSize: 11 }}>← {c.depTitle}</Text>
              <div>
                {c.matched.length > 0 && (
                  <Text type="success" style={{ fontSize: 11 }}>
                    同名字段自动传入：{c.matched.join(', ')}
                  </Text>
                )}
                {c.unmatchedParams.length > 0 && (
                  <div style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
                    <WarningOutlined style={{ color: '#faad14', fontSize: 11 }} />
                    <Text type="warning" style={{ fontSize: 11 }}>
                      必填参数 {c.unmatchedParams.join(', ')} 在上游输出中无同名字段，运行时需手填
                    </Text>
                  </div>
                )}
              </div>
            </div>
          ))}
        </div>

        {downstream.length > 0 && (
          <div>
            <Text type="secondary" style={{ fontSize: 11, display: 'block' }}>
              被下游引用
            </Text>
            <Space wrap size={4} style={{ marginTop: 4 }}>
              {downstream.map((s) => (
                <Tag key={s.key} color="blue" style={{ fontSize: 11 }}>
                  {s.title || s.functionId}
                </Tag>
              ))}
            </Space>
          </div>
        )}

        {myOutputs.length > 0 && (
          <div>
            <Text type="secondary" style={{ fontSize: 11, display: 'block', marginBottom: 4 }}>
              输出字段（供下游同名匹配）
            </Text>
            <Space wrap size={4}>
              {myOutputs.map((f) => (
                <Tag key={f} style={{ fontSize: 11 }}>
                  {f}
                </Tag>
              ))}
            </Space>
          </div>
        )}

        {params.length > 0 && (
          <Alert
            type="info"
            showIcon={false}
            style={{ fontSize: 11, padding: '6px 10px' }}
            message={
              <>
                <Text strong style={{ fontSize: 11 }}>
                  输入参数
                </Text>
                <div>
                  {params.map((p) => (
                    <Tag key={p.name} style={{ fontSize: 11 }}>
                      {p.name}
                      {p.required ? ' *' : ''}
                    </Tag>
                  ))}
                </div>
              </>
            }
          />
        )}
      </Space>
    </Card>
  );
}
